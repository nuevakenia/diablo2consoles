//go:build dreamcast

package main

// DC-002.140 -- LA DEMO JUGABLE: EL POC SIN GUION, CON EL CONTROL EN LA MANO.
//
// QUE CAMBIA CONTRA TODO LO ANTERIOR. Cada tarjeta de este POC corre un GUION adentro del binario: la cruceta
// la empuja `guionPoc`/`inyectarJugador1`, el golpe lo aprieta `golpeAuto`, el panel lo abre `cuadroAbreElDialogo`
// y la quest avanza por NUMERO DE CUADRO (`guionDeLaQuest`). Eso sirve para medir, no para jugar. La demo es lo
// contrario: NADIE aprieta solo. El jugador camina con la cruceta, pega con Y, habla con X y la quest avanza
// porque el MOTOR ve los hechos (hablo con Akara, mato los bichos, vuelve a Akara), no porque llego a un cuadro.
//
// POR QUE UN ARCHIVO APARTE Y CON PALANCA (`demoKnob`). Con `demoKnob = "no"` este archivo no hace NADA: la
// corrida es la de la tarjeta que corresponda, bit a bit. Con `demoKnob = "si"`:
//
//  1. EL POC NO SE CORTA: el camino de siempre deja de dibujar en `cuadros+3` y devuelve al jugador a la celda
//     de arranque en los cuadros del cierre. La demo corre HASTA QUE SE SALE DEL EMULADOR (eso es jugar).
//  2. LA QUEST SALE DEL JUEGO, NO DEL GUION:
//     - hablar con Akara (boton X dentro del radio) ACEPTA la Den of Evil (`Activate` + `Hear(64)`, el camino
//       del motor de DC-002.128), sin tocar un solo cuadro del guion;
//     - cada bicho que MUERE a golpes de verdad avisa al motor (`SyncDenMonsters` + `EvMonsterKilled`), y el
//       contador de la cueva baja con las bajas;
//     - cuando no queda ningun bicho vivo, el reloj del motor (`Tick(8)`) pasa la quest a "volve con Akara";
//     - hablar otra vez con Akara, con la quest limpia, COBRA la recompensa (`Hear(76)`): el punto de habilidad.
//  3. LA SALIDA DEL PUEBLO (`demoMoor`). Cruzar el borde del campamento compone el BLOOD MOOR como nivel nuevo
//     por el mismo camino medido en DC-002.120 (soltar la cache y las casillas del nivel que se deja, componer el
//     siguiente, colgarlo del renderer) y avisa al motor el cambio de area (`EvAreaChanged`). Va con palanca
//     propia porque el nivel de afuera pide RAM y ese numero se mide antes de dejarlo encendido.
//  4. LOS CUADROS POR SEGUNDO, EN LA CONSOLA: cada `cuadrosDelInformeDemo` cuadros se imprime el ritmo REAL de
//     la corrida jugable (`DEMO140: FPS`). Sin ese numero la demo no se puede juzgar (pedido del usuario).
//
// LO QUE NO HACE: habilidades, guardado a disco, actos 2-5. Y no cambia ni una linea del camino de dibujo: los
// ganchos son tres llamadas guardadas por la palanca.

import (
	"runtime"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2automap"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2quest"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

// cuadrosDelInformeDemo: cada cuantos cuadros se imprime el ritmo de la corrida (30 = los mismos del informe
// de DC-002.132, para que los dos numeros se puedan leer juntos).
const cuadrosDelInformeDemo = 30

var (
	// la quest, con los nombres del criterio de la demo
	demoAceptada bool // Akara entrego la Den of Evil (Hear 64)
	demoLimpia   bool // no queda un bicho vivo y el motor paso la pagina
	demoCobrada  bool // Akara cobro y pago el punto de habilidad
	demoMuertes  int  // bajas que el MOTOR conto (no las que el guion dijo)
	demoPunto    int  // el punto de habilidad que pago la recompensa

	// el borde del boton X: una pulsacion = una accion (el flanco, como el golpe)
	demoBotonAntes bool

	// demoBotonesAntes: la mascara del control del cuadro anterior, para dejar en el log lo que el jugador apreto
	demoBotonesAntes uint32

	// el reloj de los cuadros por segundo
	demoCuadros   int
	demoMsIni     int64
	demoMsAntes   int64
	demoMsAcum    int64
	demoFpsMinimo float64
	demoFpsMaximo float64
	demoFpsSuma   float64
	demoFpsN      int

	// la salida del pueblo
	demoYaSalio      bool
	demoSalidaFallo  bool
	demoMsDeLaSalida int64
	demoCeldasSalida float64
	demoNivelActual  = "TownN1"
)

// arrancarLaDemo: el escenario de la demo y el cartel de las teclas. Se llama al final de `main`, cuando el mundo,
// los NPCs, la quest y los bichos ya estan cargados y antes de que el ciclo del PVR pida el primer cuadro.
func arrancarLaDemo() {
	if demoKnob != "si" {
		return
	}

	demoMsIni = int64(d2cMs())
	demoMsAntes = demoMsIni
	demoFpsMinimo = 0

	println("DEMO140: ARRANCA LA DEMO JUGABLE -- sin guion: el control es del jugador")
	println("DEMO140: TECLAS cruceta= mover  Y= golpear  X= hablar (Akara)  A= recoger  I= inventario")
	println("DEMO140: ESCENARIO jugador=", celdaDelJugadorX(), celdaDelJugadorY(), "bichos=", len(bichos),
		"npcs=", len(npcsDelPueblo), "quest=", questKnob, "nivel=", demoNivelActual,
		"salida(radio)=", radioDeLaSalidaDemo, "moor=", demoMoor,
		"guionPoc=", guionPoc, "inyectarJugador1=", inyectarJugador1, "golpeAuto=", golpeAuto,
		"hablarAuto=", hablarAuto, "recogerAuto=", recogerAuto, "inventarioAuto=", inventarioAuto)

	n := elNPCAkara()
	if n != nil {
		println("DEMO140: AKARA celda=", n.CeldaX, n.CeldaY, "distancia=",
			distanciaEntre(celdaDelJugadorX(), celdaDelJugadorY(), n.CeldaX, n.CeldaY),
			"radioDeHabla=", radioQuest)
	} else {
		println("DEMO140: AVISO no hay Akara en NPC_LISTA: la quest no se puede tomar")
	}

	if len(bichos) > 0 {
		for i, b := range bichos {
			x, y := b.Est.Celda()
			vida := 0
			if b.Vida != nil {
				vida = b.Vida.HP
			}

			println("DEMO140: BICHO", i, "celda=", x, y, "vida=", vida, "patrulla=", b.A[0], b.A[1], "->",
				b.B[0], b.B[1])
		}
	}
}

// pasoDeLaDemo: lo que la demo hace en ESTE cuadro. Se llama desde `actualizar` despues de los NPCs y la quest,
// asi que la distancia que decide si se le puede hablar es la de este cuadro.
func pasoDeLaDemo() {
	if demoKnob != "si" {
		return
	}

	demoCuadros++

	// ---- el reloj: los cuadros por segundo de la corrida JUGABLE ----
	ahora := int64(d2cMs())
	demoMsAcum += ahora - demoMsAntes
	demoMsAntes = ahora

	if demoCuadros%cuadrosDelInformeDemo == 0 && demoMsAcum > 0 {
		fps := float64(cuadrosDelInformeDemo) * 1000.0 / float64(demoMsAcum)
		msPorCuadro := float64(demoMsAcum) / float64(cuadrosDelInformeDemo)

		demoFpsSuma += fps
		demoFpsN++

		if demoFpsMinimo == 0 || fps < demoFpsMinimo {
			demoFpsMinimo = fps
		}

		if fps > demoFpsMaximo {
			demoFpsMaximo = fps
		}

		println("DEMO140: FPS cuadros=", demoCuadros, "msPorCuadro=", int(msPorCuadro), "fps=", int(fps*100+50)/100,
			"celda=", celdaDelJugadorX(), celdaDelJugadorY(), "nivel=", demoNivelActual)

		demoMsAcum = 0
	}

	// ---- el boton X: el flanco (una pulsacion = una accion) ----
	apretado := false

	if len(jugadores) > 0 && jugadores[0].Entrada != nil {
		apretado = jugadores[0].Entrada.IsKeyPressed(teclaHablarNPC)
	}

	// ---- la tabla del boton: lo que el jugador aprieta, cuando cambia (para poder leer la corrida despues) ----
	if len(jugadores) > 0 && jugadores[0].Botones != demoBotonesAntes {
		println("DEMO140: BOTON cuadro=", cuadro+1, "botones=", int(jugadores[0].Botones),
			"celda=", celdaDelJugadorX(), celdaDelJugadorY())

		demoBotonesAntes = jugadores[0].Botones
	}

	flanco := apretado && !demoBotonAntes
	demoBotonAntes = apretado

	// ---- 1. TOMAR LA DEN OF EVIL: hablar con Akara ----
	if questKnob == "si" && questJuego != nil && !demoAceptada && flanco && laAkaraEstaCerca() {
		demoAceptaLaQuest()
	}

	// ---- 2. LAS BAJAS: las que el MOTOR ve ----
	if questKnob == "si" && questJuego != nil && demoAceptada {
		demoContarLasBajas()

		if !demoLimpia && questBichosConVida() == 0 && len(bichos) > 0 {
			demoCierraLaCueva()
		}
	}

	// ---- 3. COBRAR: volver a Akara con la quest limpia ----
	if questKnob == "si" && questJuego != nil && demoLimpia && !demoCobrada && flanco && laAkaraEstaCerca() {
		demoCobraLaRecompensa()
	}

	// ---- 4. LA SALIDA DEL PUEBLO ----
	if demoMoor == "si" && !demoYaSalio && !demoSalidaFallo {
		demoMiraLaSalida()
	}

	// ---- 4b. EL BORDE DE LA REGION (DC-002.147) ----
	// Con el moor por regiones compuesto, cada cuadro se mira si el jugador se salio de la caja de la region que
	// esta puesta: si se salio, se compone la vecina sin volver a compilar. Va DESPUES del movimiento (la celda de
	// este cuadro) y ANTES de la camara (que la pone el cuadro siguiente con la region ya cambiada).
	if demoMoor == "si" && demoYaSalio && moorRegionActual != nil {
		demoMiraElBordeDeLaRegion()
	}

	// ---- 5. EL RESUMEN PERIODICO: la corrida termina cuando el jugador cierra el emulador, asi que los numeros
	// del cierre tienen que estar en el log ANTES de eso (cada 300 cuadros = entre 10 y 40 s de juego real).
	if demoCuadros%cuadrosDelResumenDemo == 0 {
		resumenDeLaDemo()
	}
}

// cuadrosDelResumenDemo: cada cuantos cuadros se deja el resumen completo en la consola serie.
const cuadrosDelResumenDemo = 300

// laAkaraEstaCerca: si el jugador 1 esta dentro del radio de habla de Akara (la misma cuenta de DC-002.127).
func laAkaraEstaCerca() bool {
	n := elNPCAkara()
	if n == nil {
		return false
	}

	d := distanciaEntre(celdaDelJugadorX(), celdaDelJugadorY(), n.CeldaX, n.CeldaY)

	return d <= radioQuest
}

// demoAceptaLaQuest: el camino del motor, el mismo de DC-002.128 (`Activate` + `Hear(64)`), pero disparado por la
// pulsacion del jugador. La funcion de la quest ya sabe hacerlo: se reusa tal cual y se anota el resultado.
func demoAceptaLaQuest() {
	println("DEMO140: HABLA CON AKARA (boton X) jugador=", celdaDelJugadorX(), celdaDelJugadorY(),
		"cuadro=", cuadro+1)

	questAceptaLaQuest()

	q := questJuego.Quest(d2quest.QuestDenOfEvil)
	if q != nil && questJuego.Rec.Get(q.Slot, d2quest.FlagStarted) {
		demoAceptada = true
		questAceptada = true

		// EL DEN, CON SUS BICHOS: el motor cuenta las bajas de la cueva pero no sabe cuantos bichos hay -- en el
		// juego lo lee del nivel; aca lo dice el POC (`SetDenMonsters`), que es lo mismo que hace DC-002.128 al
		// entrar a la cueva. Sin esto `EvMonsterKilled` no cuenta ni una baja.
		vivos := questBichosConVida()
		questJuego.SetDenMonsters(vivos)

		println("DEMO140: QUEST ACEPTADA la Den of Evil cuadro=", cuadro+1, "estado=", estadoDeLaQuest(q),
			"pagina=", questJuego.LogPage(q), "bichosVivos=", vivos, "quedaSegunElMotor=",
			questJuego.DenMonstersLeft())
	} else {
		println("DEMO140: QUEST NO-SE-ACEPTO el motor no marco el slot (Hear 64 no la entrego)")
	}
}

// demoContarLasBajas: los bichos que MURIERON a golpes, uno por uno, avisados al motor. El instrumento NO toca la
// vida: la baja la produjo el combate de DC-002.122 con el boton Y del jugador.
func demoContarLasBajas() {
	for i, b := range bichos {
		if b == nil || b.Vida == nil || b.Vida.HP > 0 {
			continue
		}

		if demoBajasContadas[i] {
			continue
		}

		demoBajasContadas[i] = true
		demoMuertes++
		questMuertes++
		questContadorAvanzo = true

		vivos := questBichosConVida()
		questJuego.SyncDenMonsters(vivos)

		efectos := questJuego.Dispatch(d2quest.Event{Kind: d2quest.EvMonsterKilled,
			Level: nivelDeLaCuevaQuest, Monster: claseReturnedQuest})

		q := questJuego.Quest(d2quest.QuestDenOfEvil)

		println("DEMO140: MATA bicho=", i, "bajas=", demoMuertes, "de", len(bichos), "vivos=", vivos,
			"quedaSegunElMotor=", questJuego.DenMonstersLeft(), "pagina=", questJuego.LogPage(q),
			"cuadro=", cuadro+1)

		questImprime("DEMO-MATA", efectos)
	}
}

// demoBajasContadas: que bicho ya se le conto al motor (4 = el tope de `maxBichosBicho`).
var demoBajasContadas [8]bool

// demoCierraLaCueva: no queda un bicho vivo. El reloj del motor (`Tick`) es el que pasa la pagina de la quest a
// "volve con Akara"; se le dan los mismos 8 cuadros de DC-002.128 y se anota lo que quedo.
func demoCierraLaCueva() {
	q := questJuego.Quest(d2quest.QuestDenOfEvil)

	questJuego.Tick(8)

	demoLimpia = q != nil && questJuego.Rec.Get(q.Slot, d2quest.FlagPrimaryGoal)
	questCompletada = demoLimpia

	println("DEMO140: QUEST LIMPIA bajas=", demoMuertes, "estado=", estadoDeLaQuest(q),
		"pagina=", questJuego.LogPage(q),
		"recompensaPendiente=", q != nil && questJuego.Rec.Get(q.Slot, d2quest.FlagRewardPending),
		"marcador=", questJuego.Marker(d2quest.NPCAkara), "cuadro=", cuadro+1)

	// EL PANEL DE LA RECOMPENSA (DC-002.141): la misma pagina del log que muestra el camino de DC-002.128 cuando la
	// cueva queda limpia (`questAbreElPanel`, el panel de DC-002.127 con el texto del `string.tbl`). Es lo que hace
	// que la captura de la corrida sea la del PANEL de la quest y no solo un numero en el log; el texto del panel
	// todavia no se pinta (defecto abierto de DC-002.127), pero la geometria y la pagina son las del motor.
	if demoLimpia && q != nil {
		questAbreElPanel("AKARA", questJuego.LogPage(q))
	}
}

// demoCobraLaRecompensa: Akara paga. `Hear(76)` es el mensaje del nodo A1Q1 que cierra la quest y emite el punto
// de habilidad (el camino de DC-002.128, disparado por el jugador).
func demoCobraLaRecompensa() {
	println("DEMO140: COBRA con Akara (boton X) jugador=", celdaDelJugadorX(), celdaDelJugadorY(),
		"cuadro=", cuadro+1)

	n := elNPCAkara()
	if n == nil {
		return
	}

	questJuego.Activate(n.ClaseExe)
	efectos := questJuego.Hear(n.ClaseExe, msgCobraLaReward)
	questImprime("DEMO-COBRA", efectos)

	for _, e := range efectos {
		if e.Kind == d2quest.EffectSkillPoint {
			demoPunto += e.Value
		}
	}

	q := questJuego.Quest(d2quest.QuestDenOfEvil)

	if q != nil {
		demoCobrada = questJuego.Rec.Get(q.Slot, d2quest.FlagRewardGranted)
		questCobrada = demoCobrada
		questPuntoDeHabilidad = demoPunto
	}

	println("DEMO140: QUEST COBRADA cumple=", demoCobrada, "puntoDeHabilidad=", demoPunto,
		"estado=", estadoDeLaQuest(q), "log=", estadoDelLogDeLaQuest(), "cuadro=", cuadro+1)

	// EL PANEL DE LA RECOMPENSA ya cobrada: la pagina del log que dejo el motor (`UPDATE_LOG_ADD 13`, la de
	// "COMPLETADA"), por el mismo camino de DC-002.128. La captura de este cuadro es el panel con la quest cerrada.
	if demoCobrada && q != nil {
		questAbreElPanel("AKARA", questJuego.LogPage(q))
	}
}

// ============================== LA SALIDA DEL PUEBLO (EL BLOOD MOOR) ==============================

// radioDeLaSalidaDemo: a que distancia del punto de arranque se considera que el jugador SALIO del campamento. El
// campamento es un cuadrado de casillas alrededor del punto de arranque; caminar lejos de el, en la direccion en
// la que el `.ds1` deja caminar, es lo que en el juego es cruzar la puerta. El numero se mide: con 6 celdas el
// jugador sale del claro de la plaza y el `.ds1` sigue teniendo piso (la linea `DEMO140: SALIDA` dice la celda y
// el nivel al que se llego).
const radioDeLaSalidaDemo = 6.0

// demoSalidaX/Y: el punto desde el que se mide la salida (el arranque del jugador).
var demoSalidaX, demoSalidaY float64

// demoMiraLaSalida: si el jugador se alejo lo suficiente, se cruza al nivel de afuera. El viaje lo hace
// `demoCruza`. UN SOLO INTENTO: si el cruce falla, el nivel que se dejo ya no esta (la cache y las casillas
// estan soltadas), asi que reintentarlo cada cuadro no puede terminar bien; se anota y se deja de intentar.
func demoMiraLaSalida() {
	if demoSalidaX == 0 && demoSalidaY == 0 {
		demoSalidaX, demoSalidaY = celdaDelJugadorX(), celdaDelJugadorY()
		demoCeldasSalida = 0

		return
	}

	d := distanciaEntre(celdaDelJugadorX(), celdaDelJugadorY(), demoSalidaX, demoSalidaY)
	if d < radioDeLaSalidaDemo {
		return
	}

	if demoCruzaAlMoor() {
		demoYaSalio = true
		demoMsDeLaSalida = int64(d2cMs())
		demoCeldasSalida = d

		println("DEMO140: SALIDA DEL PUEBLO jugador=", celdaDelJugadorX(), celdaDelJugadorY(), "recorrido=", d,
			"nivel=", demoNivelActual, "bichos=", len(bichos), "cuadro=", cuadro+1)
	} else {
		demoSalidaFallo = true

		println("DEMO140: SALIDA FALLIDA no se cruzo; no se reintenta (el nivel soltado ya no esta) recorrido=", d,
			"nivel=", demoNivelActual, "libre=", d2cMontonLibre(), "semiespacio=", int(d2cGcUsado()),
			"cuadro=", cuadro+1)
	}
}

// demoCruzaAlMoor: EL CAMBIO DE NIVEL, con el orden medido en DC-002.109/DC-002.120 para que el nivel nuevo entre
// en la RAM de la consola: primero se suelta lo del nivel que se deja (la cache de tiles del renderer, el arena de
// los `.dt1` y las casillas del motor) y recien despues se compone el que sigue. Devuelve si el nivel nuevo quedo
// colgado del renderer.
func demoCruzaAlMoor() bool {
	if motorDelMapa == nil || rendererDelMapa == nil || gestorDeAssets == nil {
		println("DEMO140: SALIDA SIN MOTOR (motor/renderer/gestor nulos): no se cruza")

		return false
	}

	t0 := int64(d2cMs())

	println("DEMO140: CRUZA al Blood Moor desde celda=", celdaDelJugadorX(), celdaDelJugadorY(),
		"libre=", d2cMontonLibre(), "semiespacio=", int(d2cGcUsado()))

	// ---- lo del nivel que se deja ----
	bytes := rendererDelMapa.SoltarLaCacheDelNivel(soltarBloqueDeLaCache)

	println("DEMO140: CRUZA cache del pueblo soltada bytes=", bytes, "libre=", d2cMontonLibre())

	gestorDeAssets.ReiniciarArenaDeTiles()
	motorDelMapa.SoltarLasCasillasDelNivelAnterior()

	runtime.GC()

	println("DEMO140: CRUZA casillas del pueblo soltadas libre=", d2cMontonLibre(), "semiespacio=", int(d2cGcUsado()))

	// ---- el nivel nuevo: EL BLOOD MOOR POR REGIONES CON EL DRLG (DC-002.145) ----
	// Con `demoMoorDRLG = "si"` el afuera NO es un `.ds1`: es una region del nivel 2 compuesta con el plan que
	// el DRLG calculo en el host (`moor_145.go`). El camino del preset (DC-002.140/.144b) queda intacto abajo,
	// para que el A/B sea una constante.
	if demoMoorDRLG == "si" {
		return demoComponeLaRegionDelMoor(t0)
	}

	// ---- el nivel nuevo: el BLOOD MOOR, por su preset ----
	preset := presetDelMoor()
	if preset < 0 {
		println("DEMO140: CRUZA SIN PRESET del Blood Moor en las tablas del motor: no se cruza")

		return false
	}

	stamp := motorDelMapa.LoadStamp(d2enum.RegionAct1Wilderness, preset, 0)
	if stamp == nil {
		println("DEMO140: CRUZA LoadStamp devolvio nil preset=", preset)

		return false
	}

	tamano := stamp.Size()
	ruta := stamp.RegionPath()

	presetRec := gestorDeAssets.Records.Level.Presets[preset]

	println("DEMO140: CRUZA DS1 del moor=", ruta, "ancho=", tamano.Width, "alto=", tamano.Height,
		"preset=", preset, "nombre=", presetRec.Name, "levelID=", presetRec.LevelID, "outdoors=", presetRec.Outdoors)

	// ---- LO QUE DECIDE ESTE CRUCE: EL ORDEN DE LOS PEDIDOS (DC-002.144b) ----
	//
	// EL NUMERO QUE DEJO DC-002.144: al cruzar quedaban 2.995.872 B libres y el nivel de afuera murio pidiendo
	// el buffer de lectura de un `.dt1` de 1.008.003 B (`act1/town/floor.dt1`, el mas grande de la pieza de
	// afuera) con `Out of memory. Requested sbrk_base ... diff 1007616`: el monton NO podia crecer mas y lo
	// libre estaba en trozos. El hueco GRANDE existe -- es el arena de bloques del pueblo que se acaba de
	// devolver (2.621.440 B, cinco tramos pedidos uno detras del otro) -- pero el camino de DC-002.111 pide el
	// buffer de cada `.dt1` DESPUES de los tramos del arena del nivel nuevo, asi que los tramos (2 x 524.288) y
	// los buffers chicos (318.414, 505.504, 439.569) se meten DENTRO de ese hueco y lo parten: cuando llega el
	// pedido de 1.008.003 B no queda un tramo libre de ese tamano.
	//
	// LA PALANCA (`moorScratchPrimero`): pedir el buffer del `.dt1` MAS GRANDE del nivel ANTES de los tramos
	// del arena, que es lo que hace `scratchPorArchivo = false` (el camino de DC-002.101, que DC-002.111 dejo
	// como A/B). El buffer cae en el hueco fresco y los tramos crecen detras. Se mide: el tamano del buffer, lo
	// que el monton puede servir EN ESE MOMENTO y la VRAM libre (el numero que necesita la palanca 2).
	buffer := d2asset.TamanoDelDT1MasGrande(gestorDeAssets.Loader, stamp.TilesDelDS1())

	if moorScratchPrimero == "si" {
		d2asset.UsarScratchPorArchivo(false)
	} else {
		d2asset.UsarScratchPorArchivo(true)
	}

	println("DEMO140: CRUZA BUFFER primero=", moorScratchPrimero == "si", "delDT1MasGrande=", buffer,
		"libre=", d2cMontonLibre(), "servible=", int(d2cServible(uint(buffer))), "vramLibre=", pvr.VramDisponible())

	motorDelMapa.ResetMapParaStamp(d2enum.RegionAct1Wilderness, tamano.Width, tamano.Height, stamp)
	motorDelMapa.PlaceStampClippedWhere(stamp, 0, 0, tamano.Width, tamano.Height, func(int) bool { return false }, false)

	stamp.SoltarLosTilesDelDS1()

	// ---- LO QUE PIDIO EL NIVEL DE AFUERA, Y EL BUFFER DE VUELTA ----
	//
	// Los tramos del arena de bloques del afuera SON del nivel (los `EncodedData` de cada tile apuntan ahi
	// hasta que la cache de imagenes esta armada), asi que no se tocan. El buffer de lectura, en cambio, ya no
	// lo mira nadie: los bloques que el nivel usa estan COPIADOS. Con `moorScratchPrimero` ese buffer es el del
	// `.dt1` mas grande del nivel (1.008.003 B), y devolverlo ACA es lo que deja monton para la cache de tiles
	// del nivel nuevo y para jugar.
	arena := d2asset.EstadoDeLosTiles()
	bufferSoltado := d2asset.SoltarPrestamoDeLecturaDeTiles()

	println("DEMO140: CRUZA ARENA DEL MOOR copiado=", arena.BloquesBytes, "tramos=", arena.BloquesTramos,
		"capacidad=", arena.BloquesCapacidad, "buffer=", arena.Scratch, "bufferSoltado=", bufferSoltado,
		"libre=", d2cMontonLibre(), "semiespacio=", int(d2cGcUsado()))

	rendererDelMapa.SetMapEngine(motorDelMapa)

	// EL NIVEL QUE QUEDO: el nombre sale del `.ds1` que se compuso, no de una constante (decir "BloodMoor"
	// cuando el archivo es `Wild1.ds1` seria otra fuente de verdad y una mentira en el log).
	demoAcomodaElCruce("Afueras:"+nombreBaseDeLaRuta(ruta), float64(tamano.Width)/2, float64(tamano.Height)/2, t0, true)

	return true
}

// demoAcomodaElCruce (DC-002.145): lo que se hace IGUAL despues de componer el nivel nuevo, sea un `.ds1`
// entero (el camino de DC-002.140b/.144b) o una region del afuera compuesta con el plan del DRLG (DC-002.145):
// el jugador y los bichos al nivel nuevo, la interfaz rearmada, el motor avisado del cambio de area y el cierre
// del cruce con los numeros. `nombre` es el del nivel que quedo y `cx, cy` la casilla donde arranca el jugador.
//
// `rearmarInterfaz` (DC-002.147) dice si el nivel que entra es OTRO nivel (el cruce del pueblo: el minimapa y el
// HUD se rearman para el nivel que llega) o la region VECINA del mismo nivel (el cambio en caliente: la tabla y el
// arte del minimapa son los mismos y el automapa es del nivel entero, asi que rearmarlos seria tirar las celdas
// ya reveladas y pagar 1,3 s de recarga para volver a lo mismo).
func demoAcomodaElCruce(nombre string, cx, cy float64, t0 int64, rearmarInterfaz bool) {
	demoNivelActual = nombre

	// ---- el jugador y los bichos, al nivel nuevo ----
	if len(jugadores) > 0 && jugadores[0].Mov != nil {
		jugadores[0].Mov.Est = NuevoEstado(cx, cy, false)
	}

	demoSalidaX, demoSalidaY = cx, cy

	demoMueveLosBichos(cx, cy)

	// ---- DC-002.144c: LA INTERFAZ SIGUE AL NIVEL NUEVO ----
	// El minimapa y el HUD son del pueblo hasta aca: el minimapa porque su nivel era una constante de paquete
	// (`nivelDelPueblo`) y el HUD porque su unica carga es la del pueblo. Al cruzar se rearman los dos para el
	// nivel que entro. Va DESPUES de mudar al jugador (el reveal y el marcador se calculan sobre SU celda nueva) y
	// ANTES del cierre del cruce, para que los numeros queden en el log entre las lineas del cruce.
	if rearmarInterfaz {
		demoRearmaLaInterfaz()
	}

	// EL MOTOR VE EL CAMBIO DE AREA: salir del campamento es, para la Den of Evil, entrar al exterior (el nivel
	// del motor que la quest espera). Es el mismo evento de DC-002.128 (`EvAreaChanged`), disparado por el paso
	// del jugador y no por un cuadro del guion. En un cambio de REGION el area no cambio: no se dispara.
	if rearmarInterfaz && questKnob == "si" && questJuego != nil && demoAceptada {
		questJuego.Dispatch(d2quest.Event{Kind: d2quest.EvAreaChanged,
			OldLevel: nivelDelPuebloQuest, NewLevel: nivelDeLaCuevaQuest})

		vivos := questBichosConVida()
		questJuego.SetDenMonsters(vivos)

		println("DEMO140: AREA pueblo->exterior (el motor ya cuenta las bajas de la Den) bichosVivos=", vivos)
	}

	runtime.GC()

	rx, ry := regionDelLog()

	println("DEMO140: CRUZA OK nivel=", nombre, "region=", rx, ry, "tiles=", len(*motorDelMapa.Tiles()),
		"jugador=", cx, cy, "ms=", int64(d2cMs())-t0, "libre=", d2cMontonLibre(),
		"libreAntes=", moorLibreAntes, "semiespacio=", int(d2cGcUsado()), "semiespacioAntes=", moorSemiAntes,
		"vramLibre=", pvr.VramDisponible())
}

// demoRearmaLaInterfaz: el MINIMAPA y el HUD del nivel que acaba de entrar (DC-002.144c).
//
// QUE SE ARREGLA. Las dos piezas de la interfaz nacian con el pueblo y no tenian forma de cambiar de nivel:
//
//   - el MINIMAPA tenia el nivel en una CONSTANTE de paquete (`nivelDelPueblo` = 1) metida en la tabla, en el
//     arte, en la semilla y en el filtro de las filas, y ninguna funcion recargaba el mapa para otro nivel. Al
//     cruzar, el modelo solo SUMABA las celdas nuevas sobre las 680 del pueblo (`celdas= 701`). Ahora
//     `recargarElMinimapaDelNivel` suelta el arte y el modelo del que se deja y arma los del que llega.
//   - el HUD ponia el nombre de la zona UNA vez, en la carga (`JUEGO125: HUD LISTO ... zona= Rogue Encampment`), con
//     el fallback del motor para el nivel 1. Ahora `rearmarLaZona` le pone el nombre del nivel nuevo y le pide al
//     font16 -que ya esta en el monton de C- los glifos que le falten.
//
// EL ID DEL NIVEL. Los dos se piden por el id del nivel de AFUERA del acto 1: `d2enum.RegionAct1Wilderness` es el
// tipo de nivel (LvlTypes) con el que se compuso el mapa y es tambien el id de las filas de `AutoMap.txt` ("1
// Wilderness", 142 filas / 75 cuadros de arte medidos en `console/dc/medidas/automap-moor`); el nombre del HUD sale
// de `levels.txt` para `nivelDelBloodMoor` (2), porque el fallback `DefaultLevelNames` no tiene esa fila.
func demoRearmaLaInterfaz() {
	tipoDelNivel := int(d2enum.RegionAct1Wilderness)

	minimapaOk := false
	celdasDelNivel := 0

	if elMinimapa != nil {
		println("DEMO140: REARME paso=1 minimapaDelNivel=", tipoDelNivel, "libre=", d2cMontonLibre())

		minimapaOk = elMinimapa.recargarElMinimapaDelNivel(tipoDelNivel, 0)
		celdasDelNivel = elMinimapa.Modelo.Count()

		println("DEMO140: REARME paso=2 minimapaListo=", minimapaOk, "libre=", d2cMontonLibre())
	} else {
		println("DEMO140: REARME sin minimapa en la corrida (MINIMAPA=no): no hay nada que recargar")
	}

	nombre, deDonde := nombreDeLaZonaDelNivel(nivelDelBloodMoor)

	hudOk := false

	if elHUD != nil {
		println("DEMO140: REARME paso=3 zona=", nombre, "de=", deDonde, "libre=", d2cMontonLibre())

		hudOk = elHUD.rearmarLaZona(nombre, deDonde)

		println("DEMO140: REARME paso=4 hudListo=", hudOk, "libre=", d2cMontonLibre())
	} else {
		println("DEMO140: REARME sin HUD en la corrida (HUD=no): no hay zona que rearmar")
	}

	println("DEMO140: REARME INTERFAZ nivel=", tipoDelNivel, d2automap.LevelName(tipoDelNivel),
		"minimapaOk=", minimapaOk, "celdasDelNivel=", celdasDelNivel,
		"hudOk=", hudOk, "zona=", nombre, "zonaDe=", deDonde,
		"libre=", d2cMontonLibre(), "semiespacio=", int(d2cGcUsado()))
}

// demoMueveLosBichos: los bichos del pueblo no existen en el `.ds1` del moor, asi que los de la corrida se mudan
// con el jugador: cada uno a su carril alrededor de la celda nueva, con la patrulla armada alrededor de ella.
func demoMueveLosBichos(cx, cy float64) {
	for i, b := range bichos {
		if b == nil {
			continue
		}

		d := float64(i) * separacionEntreBichos
		ax, ay := cx+1.0+d, cy+1.0+d

		b.Est = NuevoEstado(ax, ay, false)
		b.A = [2]float64{ax, ay}
		b.B = [2]float64{ax + patrullaDx*patrullaLargo, ay + patrullaDy*patrullaLargo}
		b.HaciaB = false

		println("DEMO140: BICHO", i, "mudado a", ax, ay, "patrulla=", b.A[0], b.A[1], "->", b.B[0], b.B[1])
	}
}

// presetDelMoor: el indice de preset (fila de lvlprest) del nivel de AFUERA con el que se cruza el borde. Se
// busca en las tablas que el motor tiene cargadas, en tres pasos, y se devuelve -1 si ninguno acierta.
//
// MEDIDO EN EL HOST (`go run ./console/dc/medidas/presets-scratch "<carpeta de los MPQ>"`, LvlPrest.txt de
// patch_d2.mpq, 1.091 presets): el UNICO preset con `LevelID > 0` es el del pueblo (preset 1, LevelID 1, con
// TownN1/TownE1/TownS1/TownW1). El Blood Moor NO tiene fila de preset y no puede tenerla: en el juego no es un
// preset, es un nivel de DRLG (56 x 96) armado con las piezas de afuera (`Act1/Outdoors/Bord*.ds1`,
// `Wild*.ds1`, `StnClf*.ds1`). O sea que los dos primeros criterios de esta funcion no pueden acertar nunca:
// lo que la corrida de esta tarjeta midio fue `DEMO140: CRUZA SIN PRESET del Blood Moor en las tablas del
// motor` en cada intento. El tercero es el que existe de verdad.
func presetDelMoor() int {
	if gestorDeAssets == nil || gestorDeAssets.Records == nil {
		return -1
	}

	presets := gestorDeAssets.Records.Level.Presets

	// 1. por el id del nivel del Blood Moor, y que sea un nivel de AFUERA (si algun dia la tabla lo trae)
	for i, p := range presets {
		if p.LevelID == nivelDelBloodMoor && p.Outdoors {
			return i
		}
	}

	// 2. por nombre
	for i, p := range presets {
		if contieneElMoor(p.Name) {
			return i
		}
	}

	// 3. LA PIEZA DE AFUERA DEL ACTO 1 (el que existe): el preset mas bajo cuyo `.ds1` vive en
	// `Act1/Outdoors/Wild*.ds1`, que es la pieza de wilderness con la que el generador arma el afuera. Se
	// compone con `RegionAct1Wilderness`, o sea con el tipo de nivel de afuera y sus `.dt1`.
	menor := -1

	for i, p := range presets {
		for _, ruta := range p.Files {
			if contieneTexto(ruta, "outdoors/wild") && (menor < 0 || i < menor) {
				menor = i
			}
		}
	}

	return menor
}

// nivelDelBloodMoor: el id del nivel del Blood Moor en levels.txt (el mismo que usa el generador del acto 1 para
// la wilderness del arranque).
const nivelDelBloodMoor = 2

// contieneElMoor: si el nombre de un `.ds1` del preset es el del Blood Moor (la carpeta de afuera del acto 1).
func contieneElMoor(ruta string) bool {
	return contieneTexto(ruta, "moor")
}

// contieneTexto: si `ruta` trae `buscado` (que va en minusculas), sin distinguir mayusculas. Se escribe a mano
// porque el paquete usa `strings` solo en los archivos que ya lo importaban (el de la demo tiene que compilar
// con el mismo toolchain de la consola y no se le agregan imports por un `Contains`).
func contieneTexto(ruta, buscado string) bool {
	n := 0

	for i := 0; i+len(buscado) <= len(ruta); i++ {
		igual := true

		for k := 0; k < len(buscado); k++ {
			c := ruta[i+k]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}

			if c != buscado[k] {
				igual = false

				break
			}
		}

		if igual {
			n++
		}
	}

	return n > 0
}

// nombreBaseDeLaRuta: el nombre del archivo de una ruta con barras (`Act1/Outdoors/Wild1.ds1` -> `Wild1.ds1`).
func nombreBaseDeLaRuta(ruta string) string {
	ini := 0

	for i := 0; i < len(ruta); i++ {
		if ruta[i] == '/' || ruta[i] == '\\' {
			ini = i + 1
		}
	}

	return ruta[ini:]
}

// resumenDeLaDemo: los numeros del cierre, una linea por cosa del criterio. Se imprime cuando el jugador sale del
// emulador (`SIGTERM`) o cuando la corrida se corta: la consola serie es la unica salida, y un resumen que no sale
// es un resumen que no existe.
func resumenDeLaDemo() {
	if demoKnob != "si" || demoCuadros == 0 {
		return
	}

	segundos := float64(int64(d2cMs())-demoMsIni) / 1000.0
	fpsMedio := 0.0

	if segundos > 0 {
		fpsMedio = float64(demoCuadros) / segundos
	}

	muertes := 0
	for _, b := range bichos {
		if b != nil && b.Vida != nil && b.Vida.HP <= 0 {
			muertes++
		}
	}

	println("DEMO140: CIERRE cuadros=", demoCuadros, "segundos=", int(segundos*100)/100, "fpsMedio=",
		int(fpsMedio*100+50)/100, "fpsMinimo=", int(demoFpsMinimo*100+50)/100, "fpsMaximo=",
		int(demoFpsMaximo*100+50)/100, "informes=", demoFpsN)
	println("DEMO140: CIERRE QUEST aceptada=", demoAceptada, "bajas=", muertes, "limpias=", demoLimpia,
		"cobrada=", demoCobrada, "puntoDeHabilidad=", demoPunto)
	println("DEMO140: CIERRE MUNDO nivel=", demoNivelActual, "salio=", demoYaSalio, "recorrido=",
		demoCeldasSalida, "celda=", celdaDelJugadorX(), celdaDelJugadorY(), "libre=", d2cMontonLibre(),
		"semiespacio=", int(d2cGcUsado()))
}
