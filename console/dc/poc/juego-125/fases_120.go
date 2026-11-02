//go:build dreamcast

// DC-002.120 -- EL CICLO CON LOS CUATRO: ENTRAR / TRANSICION / VOLVER CON CUATRO JUGADORES DENTRO.
//
// QUE MIDE, Y POR QUE ES UNA TARJETA Y NO UNA CORRIDA MAS. Las dos piezas grandes ya estaban: el ciclo del
// pueblo (las tres fases con el crc32 del host, DC-002.111, en `mapa-103`) y los cuatro jugadores dibujados y
// caminando en el mismo cuadro con su leash (DC-002.117, en `juego-117`). Nunca se corrieron JUNTAS. Lo que esta
// tarjeta mide es si el mundo puede CAMBIAR DE NIVEL con la gente adentro: que los cuatro sigan dibujados (y con
// el leash puesto) cuando el nivel se tira y se compone el que sigue, y a que precio -- el pico del monton de C y
// del semiespacio del recolector POR FASE, los ms por cuadro por fase y el crc32 del cuadro de la VUELTA.
//
// EL CUADRO DE CADA FASE ES EL DEL HOST, Y ESO OBLIGA A UNA COSA: la camara tiene que estar en la celda del POC
// (28,20) -- el crc32 1437022778 es el de la camara en (28,20) --, no en el centro del grupo (29,5;21,5), que es
// la camara de DC-002.114..117. Por eso, en los cuadros de fase, los cuatro se ponen en un CUADRADO DE 2x2 celdas
// alrededor de (28,20) (P1 28,20 / P2 29,20 / P3 28,21 / P4 29,21): con la camara en la celda del host esa
// formacion deja a los cuatro DENTRO del lienzo (anclas 320,283 / 400,323 / 240,323 / 320,363) y el mapa se mide
// contra el numero del host. La formacion se dice en el log (`FASE FORMACION`), no se da por sabida.
//
// LO QUE SE MIDE DE CADA FASE:
//
//	ENTRAR      el pueblo ya compuesto: se mueve la camara a (28,20) y los cuatro a la formacion, y se dibuja el
//	            cuadro. El crc32 del MAPA SOLO tiene que dar 1437022778 (el del host) y el del cuadro CON los
//	            cuatro es otro numero, que queda anotado.
//	TRANSICION  se compone OTRO `.ds1` del mismo nivel (preset 1 / archivo 1 = TownE1), se cuelga del renderer y
//	            se dibuja. crc32 del mapa solo = 584380401 (el del host, medido en DC-002.109).
//	VOLVER      se compone otra vez el pueblo (preset 1 / archivo 0) y se dibuja: el crc32 tiene que ser OTRA VEZ
//	            1437022778, que es el criterio de la tarjeta.
//
// EL ORDEN DEL CAMBIO DE NIVEL ES EL (a) DE DC-002.109, EL QUE SE MIDIO QUE GANA: soltar la cache del nivel que
// se deja ANTES de tocar el nivel nuevo. Se elige con `ordenDeLasFases` (knobs.go) y el (b) queda para el A/B.
//
// LOS DOS INSTRUMENTOS QUE HACEN FALTA (y que DC-002.111 dejo puestos): el pico del monton de C se muestrea en
// CADA paso de la fase (`muestraDeLaFase`) y ademas lleva el pico que mide la carga de los `.dt1`
// (`d2asset.PicoDelMontonDeC`); el semiespacio se censa en cada paso con `d2c_gc_usado/total/colecciones`. Donde
// se corta el log ES la respuesta, y si no se corta, el resumen dice con que numeros entro.
package main

import (
	"hash/crc32"
	"runtime"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapstamp"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

// reservarElArenaDeLasCasillas (DC-002.120): pide el tramo de las casillas compuestas al monton externo, UNA sola
// vez en toda la corrida, y lo deja para que se reuse en cada cambio de nivel.
//
// La cuenta sale del `.ds1` que esta en la mano: casillas x (capas de pared + pisos + sombras + sustituciones). En
// el pueblo son 2.337 x 5 = 11.685 tiles de ~40 B = ~470.000 B, que es lo que el semiespacio NO tiene. Si el arena
// no alcanza para un nivel (mas capas que el reservado) las casillas que no entran caen al camino de siempre y el
// log lo dice (`fallas=`).
func reservarElArenaDeLasCasillas(stamp *d2mapstamp.Stamp, ancho, alto int) {
	if !faseEncendida() || !reusarLasCasillas || stamp == nil {
		return
	}

	if cuantos, _, _ := d2mapengine.EstadoDelArenaDeCasillas(); cuantos > 0 {
		return // ya esta: se reusa
	}

	paredes, pisos, sombras, sustituciones := stamp.CuantasCapasDeLaCasilla()
	cuantos := ancho * alto * (paredes + pisos + sombras + sustituciones)
	entraron := d2mapengine.ReservarArenaDeCasillas(cuantos)

	println("JUEGO121: ARENA DE CASILLAS reserva=", entraron, "tiles=", cuantos, "casillas=", ancho*alto,
		"capasPorCasilla=", paredes+pisos+sombras+sustituciones, "montonLibre=", d2cMontonLibre())
}

// (los externs del recolector -- d2c_gc_usado / d2c_gc_total / d2c_gc_colecciones -- ya estan declarados en
// main.go: aca NO se repiten.)

var (
	// gestorDeAssets y motorDelMapa: las dos piezas que el ciclo de fases necesita volver a tocar. `main` las
	// guarda aca al crearlas (es el mismo objeto, no una copia).
	gestorDeAssets *d2asset.AssetManager
	motorDelMapa   *d2mapengine.MapEngine

	// picoEnLaFase / picoSemiespacioEnLaFase: el pico de lo que se esta midiendo, desde que arranco la fase.
	picoEnLaFase            int
	picoSemiespacioEnLaFase int

	// faseEnCurso: la fase que se esta dibujando (sus cuadros se suman ahi).
	faseEnCurso *faseDelCiclo

	// fasesMedidas: una entrada por fase, para el resumen del final (arreglo fijo: son tres y no hay `append`
	// en el camino caliente).
	fasesMedidas [3]faseDelCiclo
	fasesHechas  int

	// el crc32 del cuadro anterior de la fase, para la repetibilidad.
	crcDeLaFaseAnterior uint32
	nombreDelNivel      = "TownN1"
)

// maxCuadrosDeLaFase: cuantos cuadros por fase se guardan para el resumen (los ms de cada uno).
const maxCuadrosDeLaFase = 6

// faseDelCiclo: lo que se mide de una fase.
type faseDelCiclo struct {
	nombre string
	ds1    string

	// el cambio de nivel
	libreAlComponer   int
	picoMonton        int
	picoSemiespacio   int
	semiespacioAntes  int
	semiespacioDepues int
	msCompose         int

	// el cuadro (se suman los que la fase se queda en pantalla)
	cuadros       int
	crcMapa       uint32
	crcConJugador uint32
	pixeles       int
	pixelesJug    int
	msMapa        int
	msMapaTotal   int
	msJugadores   int
	msPorCuadro   [maxCuadrosDeLaFase]int

	esperado     uint32
	igual        bool
	conLosCuatro bool
	ok           bool
}

// faseEncendida: si esta corrida corre el ciclo de las tres fases (lo dice la palanca, no el codigo).
func faseEncendida() bool { return faseKnob == "ciclo" }

// cuadroDelEntrar / cuadroDeLaTransicion / cuadroDelVolver / cuadroDeLaRepeticion: el guion de cuadros del ciclo.
// Los cuadros 1..`cuadros` son el guion de los cuatro (el pueblo caminando, la unica fase que se compone al
// arrancar); despues vienen las fases, cada una con `cuadrosPorFase` cuadros en pantalla para que la captura la
// alcance, y al final el cuadro de la repetibilidad.
func cuadroDelEntrar() int      { return cuadros + 1 }
func cuadroDeLaTransicion() int { return cuadros + 2 }
func cuadroDelVolver() int      { return cuadros + 2 + cuadrosPorFase }
func cuadroDeLaRepeticion() int { return cuadros + 2 + 2*cuadrosPorFase }
func cuadroDelFinDelCiclo() int { return cuadroDeLaRepeticion() + 1 }

// muestraDeLaFase: se queda con el pico del monton de C y del semiespacio en el paso en el que se llama. Es lo
// que convierte "el log dice donde murio" en "el pico de la fase fue este numero".
func muestraDeLaFase() {
	usado := d2cMontonUsado()

	// DC-002.111: la sonda de d2asset mide el pico DENTRO de la carga de los `.dt1`, que es lo que un muestreo
	// entre pasos no ve.
	if pico := d2asset.PicoDelMontonDeC(); pico > usado {
		usado = pico
	}

	if usado > picoEnLaFase {
		picoEnLaFase = usado
	}

	if u := int(d2cGcUsado()); u > picoSemiespacioEnLaFase {
		picoSemiespacioEnLaFase = u
	}
}

// soltarBloqueDeLaCache (DC-002.109): de donde salen y a donde vuelven los bloques del tramo de la cache. Es el
// mismo reparto que arma el tramo: si los bloques se pidieron a la VRAM del PVR, se devuelven ahi
// (`pvr.VramSoltar`); si salieron del monton de C (`make` de mas de 64 KB), se devuelven con el puente del shim.
func soltarBloqueDeLaCache(bloque []byte) int {
	if len(bloque) == 0 {
		return 0
	}

	if tramoDeLaCache > 0 && tramoEnPedazos > 0 && cacheEnVram {
		return pvr.VramSoltar(bloque)
	}

	return d2cSoltarBloque(uintptr(unsafe.Pointer(&bloque[0])), len(bloque))
}

// ponerLaFormacion: los cuatro QUIETOS (NU dir 0, que es el cuadro de DC-002.112) en el cuadrado de 2x2 celdas
// alrededor de la celda del POC, y la camara en esa celda. Silenciosa: se llama en CADA cuadro de fase (el cuadro
// de la fase es quieto por definicion, ver `actualizar`).
func ponerLaFormacion() {
	formacionX := [maxJugadoresCoop]float64{0, 1, 0, 1}
	formacionY := [maxJugadoresCoop]float64{0, 0, 1, 1}

	for i := range jugadores {
		p := jugadores[i]

		p.Mov.Est = NuevoEstado(celdaX+formacionX[i], celdaY+formacionY[i], false)
		p.Mov.Direccion = dirQuieto
		p.Mov.Caminando = false
		p.Mov.MsAnim = 0
		p.Mov.Resto = 0
	}

	ponerLaCamaraEn(celdaX, celdaY)
}

// ponerALosCuatroEnLaCeldaDelHost: la formacion + la linea del log que dice porque se hace. Se llama UNA vez por
// fase (la primera): el cuadro de la fase es donde se mide el mapa contra el crc32 del host.
func ponerALosCuatroEnLaCeldaDelHost() {
	ponerLaFormacion()

	println("JUEGO121: FASE FORMACION jugadores=", len(jugadores), "camara=", celdaX, celdaY,
		"celdas=", celdaX, celdaY, celdaX+1, celdaY, celdaX, celdaY+1, celdaX+1, celdaY+1,
		"porQue= el crc32 del host (1437022778) es el de la camara en la celda del POC")
}

// ponerLaCamaraEn: la camara del renderer en una celda del mapa (la misma cuenta del motor, WorldToOrtho).
func ponerLaCamaraEn(cx, cy float64) {
	if rendererDelMapa == nil {
		return
	}

	rx, ry := rendererDelMapa.WorldToOrtho(cx, cy)
	pos := d2vector.NewPosition(rx, ry)

	rendererDelMapa.MoveCameraTo(&pos)

	camaraCeldaX, camaraCeldaY = cx, cy
}

// soltarLosCiclosDeCaminar (DC-002.120): LAS FASES SON QUIETAS, ASI QUE LOS CICLOS DE CAMINAR SE PUEDEN SOLTAR.
//
// El unico vivo que los cuatro le agregan al semiespacio del recolector son las 8 direcciones de TW compuestas
// (148.720 B de indices, medidos en DC-002.113). En los cuadros de fase los cuatro estan QUIETOS en la formacion
// (`ponerLaFormacion`), o sea que lo unico que se dibuja es el ciclo NU: los 8 ciclos de caminar no los mira nadie
// despues del ultimo cuadro del guion. Soltarlos ANTES de tirar el primer nivel es lo que hace que la VUELTA entre:
// sin esto, el pico de la VUELTA queda en el 99 % del semiespacio (medido en la primera corrida de esta tarjeta:
// `GC to-space overflow` con 1.760.000 de 1.835.008).
//
// `cicloDelJugador` ya tiene el camino de vuelta: si la direccion no esta en el mapa devuelve el ciclo QUIETO, asi
// que los cuatro se siguen dibujando igual (con el mismo cuadro NU, que es el de las fases).
func soltarLosCiclosDeCaminar() {
	if !soltarCaminandoEnElCiclo || len(jugadoresCaminando) == 0 {
		return
	}

	bytes := 0

	for _, c := range jugadoresCaminando {
		bytes += bytesDeLosCuadros(c)
	}

	jugadoresCaminando = nil
	direccionesCompuestas = nil

	runtime.GC()

	usado := int(d2cGcUsado())

	println("JUEGO121: CICLO SUELTA LOS CICLOS DE CAMINAR (las fases son quietas) bytes=", bytes,
		"semiespacioDespuesDeRecolectar=", usado, "total=", int(d2cGcTotal()), "colecciones=",
		int(d2cGcColecciones()), "libre=", d2cMontonLibre())
}

// dibujarElCiclo: los cuadros que van despues del guion cuando la palanca `faseKnob` esta en "ciclo". Se llama
// desde `dibujar` con el numero de cuadro ya contado, y devuelve sin tocar el camino de siempre.
func dibujarElCiclo(superficie d2interface.Surface) error {
	malla, esPvr := superficie.(*pvr.Surface)
	if !esPvr {
		println("JUEGO121: el Renderer no entrego una Surface del PVR")

		return nil
	}

	switch {
	case cuadro == cuadroDelEntrar():
		// ENTRAR: el pueblo ya esta compuesto. Lo unico que cambia es DONDE mira la camara y donde estan los
		// cuatro (la formacion de la celda del host).
		println("JUEGO121: CICLO cuadro=", cuadro, "ENTRAR (el pueblo compuesto al arrancar)")

		// las fases son quietas: los 8 ciclos de caminar ya no los mira nadie y son el unico vivo que los cuatro
		// le agregan al semiespacio. Se sueltan ANTES de tirar el primer nivel.
		soltarLosCiclosDeCaminar()

		ponerALosCuatroEnLaCeldaDelHost()

		f := abrirLaFase("ENTRAR", nombreDelNivel, crcHost)
		elCuadroDeLaFase(malla, f)

	case cuadro == cuadroDeLaTransicion():
		correrLaFaseDelNivel(malla, "TRANSICION", transicionPreset, transicionArchivo, crcHostTransicion,
			"TownE1")

	case cuadro < cuadroDelVolver():
		sostenerLaFase(malla, "TRANSICION")

	case cuadro == cuadroDelVolver():
		correrLaFaseDelNivel(malla, "VOLVER", presetA, archivoA, crcHost, nombreDelNivel)

	case cuadro < cuadroDeLaRepeticion():
		sostenerLaFase(malla, "VOLVER")

	case cuadro == cuadroDeLaRepeticion():
		// LA REPETIBILIDAD: el mismo estado otra vez y el mismo crc32. Se compara contra el cuadro anterior de
		// la fase (no contra ENTRAR) porque la cache de tiles es perezosa: los tiles que se cachearon al
		// caminar no estan en el cuadro 1.
		f := faseEnCurso
		elCuadroDeLaFase(malla, f)

		if f != nil {
			println("JUEGO121: CICLO REPETIBILIDAD fase=", f.nombre, "cuadro=", cuadro, "crc32=", int(f.crcMapa),
				"crc32DelCuadroAnterior=", int(crcDeLaFaseAnterior), "igual=", f.crcMapa == crcDeLaFaseAnterior,
				"jugadores=", len(jugadores))
		}

	case cuadro == cuadroDelFinDelCiclo():
		resumenDeLasFases()
		resumenDelHUD() // DC-002.125: las barras, el cinturon, la zona y su costo

	default:
		// ya no se dibuja mas: el ultimo cuadro queda en pantalla
		return nil
	}

	return nil
}

// abrirLaFase: arranca la ficha de una fase y se queda con los numeros del momento (antes de tocarla).
func abrirLaFase(nombre, ds1 string, esperado uint32) *faseDelCiclo {
	picoEnLaFase = d2cMontonUsado()

	if pico := d2asset.PicoDelMontonDeC(); pico > picoEnLaFase {
		picoEnLaFase = pico
	}

	picoSemiespacioEnLaFase = int(d2cGcUsado())

	if fasesHechas >= len(fasesMedidas) {
		return &fasesMedidas[len(fasesMedidas)-1]
	}

	f := &fasesMedidas[fasesHechas]
	fasesHechas++

	*f = faseDelCiclo{nombre: nombre, ds1: ds1, esperado: esperado, libreAlComponer: d2cMontonLibre(),
		semiespacioAntes: int(d2cGcUsado())}

	faseEnCurso = f

	return f
}

// elCuadroDeLaFase: limpia el lienzo (el del cuadro 1 arranco en cero y el crc32 solo se compara contra un lienzo
// en la misma condicion), dibuja el mapa, mide el crc32 del MAPA SOLO, pega a los cuatro encima, mide el crc32 del
// cuadro CON los cuatro, devuelve el mapa y comprueba que vuelva al mismo crc32.
func elCuadroDeLaFase(malla *pvr.Surface, f *faseDelCiclo) {
	// el cuadro de la fase es QUIETO y con la camara en la celda del host: se vuelve a poner en cada cuadro por si
	// el ciclo del PVR movio algo entre uno y otro (es idempotente y no cuesta nada con cuatro jugadores).
	ponerLaFormacion()

	malla.ClearRGBA(0, 0, 0, 0)

	t0 := d2cMs()
	rendererDelMapa.Render(malla)
	msMapa := int(d2cMs() - t0)

	pixeles := malla.Imagen().Pixeles()

	llenos := contarLlenos(pixeles)
	crcMapa := crc32.ChecksumIEEE(pixeles)

	msJugadores := 0
	crcConJugador := crcMapa
	pixelesJug := 0

	if jugadorOK && len(jugadores) > 0 {
		var msDibujo int

		crcConJugador, msDibujo = dibujarAJugadores(malla, pixeles)
		msJugadores = msDibujo

		crcRestaurado := crc32.ChecksumIEEE(pixeles)
		if crcRestaurado != crcMapa {
			println("JUEGO121: FASE", f.nombre, "MAPA-RESTAURADO DISTINTO crc32=", int(crcRestaurado),
				"mapaSolo=", int(crcMapa), "-- las cajas de los cuatro se pisaron mal")
		}

		leashDelCuadro(cuadro)

		volverAPintarAJugadores(malla)

		for _, p := range jugadores {
			pixelesJug += p.Cambiados
		}
	}

	// DC-002.125: EL HUD DE LA FASE. Se dibuja ULTIMO y DESPUES de que los crc32 de arriba ya estan medidos: los
	// numeros de la fase (crcMapa, crcConJugador, pixelesNoVacios) siguen siendo los del mapa y los cuatro SIN
	// HUD, asi que la comparacion contra el host de DC-002.120 queda igual. Sin esto el HUD solo se veria en los
	// cuadros del guion (1..6) y no en las fases, que es donde el POC pasa el resto de la corrida.
	if hudKnob == "si" && elHUD != nil {
		pxHUD, msHUD := elHUD.dibujar(malla, 0, ancho, alto)

		nombreDeLaFase := "SIN-FASE"
		if f != nil {
			nombreDeLaFase = f.nombre
		}

		println("JUEGO125: HUD FASE", nombreDeLaFase, "CUADRO", cuadro, "pixeles=", pxHUD, "ms=", msHUD,
			"crcSinHUD=", int(crcConJugador), "lienzoLleno=", contarLlenos(pixeles), "de", ancho*alto)
	}

	println("JUEGO121: FASE", f.nombre, "CUADRO", cuadro, "pixelesNoVacios=", llenos, "de", ancho*alto,
		"crc32=", int(crcMapa), "esperado=", int(f.esperado), "igualAlHost=", crcMapa == f.esperado,
		"crcConJugador=", int(crcConJugador), "pixelesDeLosCuatro=", pixelesJug, "jugadores=", len(jugadores),
		"msMapa=", msMapa, "msJugadores=", msJugadores, "libre=", d2cMontonLibre(),
		"semiespacio=", int(d2cGcUsado()), "camara=", camaraCeldaX, camaraCeldaY)

	if f == nil {
		return
	}

	if f.cuadros < maxCuadrosDeLaFase {
		f.msPorCuadro[f.cuadros] = msMapa
	}

	f.cuadros++
	f.crcMapa = crcMapa
	f.crcConJugador = crcConJugador
	f.pixeles = llenos
	f.pixelesJug = pixelesJug
	f.msMapa = msMapa
	f.msMapaTotal += msMapa
	f.msJugadores = msJugadores
	f.igual = crcMapa == f.esperado
	f.conLosCuatro = len(jugadores) > 0
	f.ok = true

	if f.picoMonton < picoEnLaFase {
		f.picoMonton = picoEnLaFase
	}

	if f.picoSemiespacio < picoSemiespacioEnLaFase {
		f.picoSemiespacio = picoSemiespacioEnLaFase
	}

	crcDeLaFaseAnterior = crcMapa

	muestraDeLaFase()
	memoria("fase-" + f.nombre + "-despues-del-cuadro")
}

// sostenerLaFase: los cuadros que la fase se queda en pantalla (para que la captura la alcance). No se recompone
// nada: es el mismo nivel otra vez, que es lo que cuesta un cuadro del juego.
func sostenerLaFase(malla *pvr.Surface, nombre string) {
	f := faseEnCurso
	if f == nil || f.nombre != nombre {
		return
	}

	println("JUEGO121: CICLO cuadro=", cuadro, "SOSTIENE", nombre, "(el nivel no se recompone)")

	elCuadroDeLaFase(malla, f)
}

// correrLaFaseDelNivel: el cambio de nivel de una fase, con el orden (a) de DC-002.109 y los instrumentos de
// DC-002.111. Termina dibujando el cuadro de la fase con los cuatro encima.
func correrLaFaseDelNivel(malla *pvr.Surface, nombre string, preset, archivo int, esperado uint32, ds1 string) {
	println("JUEGO121: CICLO cuadro=", cuadro, "COMPONE", nombre, "ds1=preset", preset, "archivo", archivo,
		"orden=", ordenDeLasFases, "jugadores=", len(jugadores))

	f := abrirLaFase(nombre, ds1, esperado)

	memoria(nombre + "-antes")
	semiespacio(nombre + "-antes")
	censoDelTramo(nombre + "-antes")
	vram(nombre + "-antes")
	muestraDeLaFase()

	// ============================== EL ORDEN (a o b) ==============================
	if ordenDeLasFases == "a" {
		bytes := rendererDelMapa.SoltarLaCacheDelNivel(soltarBloqueDeLaCache)

		println("JUEGO121: FASE", nombre, "ORDEN a -- cache del nivel que se deja SOLTADA ANTES bytes=", bytes,
			"libre=", d2cMontonLibre())
	} else {
		println("JUEGO121: FASE", nombre, "ORDEN b -- la cache del nivel que se deja QUEDA TOMADA hasta colgar "+
			"el nivel nuevo")
	}

	censoDelTramo(nombre + "-despues-de-soltar-la-cache")
	memoria(nombre + "-despues-de-soltar-la-cache")
	muestraDeLaFase()

	// ============================== EL NIVEL QUE SE VA ==============================
	// DC-002.111: la arena de los `.dt1` del nivel que se deja se reinicia y las casillas se limpian ANTES de
	// parsear el `.ds1` nuevo (si no, el nivel viejo ocupa el semiespacio que el nuevo necesita).
	gestorDeAssets.ReiniciarArenaDeTiles()
	motorDelMapa.SoltarLasCasillasDelNivelAnterior()

	memoria(nombre + "-despues-de-soltar-las-casillas")
	semiespacio(nombre + "-despues-de-soltar-las-casillas")
	muestraDeLaFase()

	runtime.GC()

	semiespacio(nombre + "-antes-de-componer")
	muestraDeLaFase()

	// ============================== COMPONER EL NIVEL NUEVO ==============================
	f.libreAlComponer = d2cMontonLibre()

	t0 := d2cMs()
	if _, _, _, ok := componerNivelDeLaFase(motorDelMapa, preset, archivo, nombre); !ok {
		println("JUEGO121: FASE", nombre, "NO COMPONE: LoadStamp devolvio nil")

		return
	}

	f.msCompose = int(d2cMs() - t0)

	semiespacio(nombre + "-despues-de-componer")
	memoria(nombre + "-despues-de-componer")
	muestraDeLaFase()

	// ============================== COLGAR EL NIVEL NUEVO ==============================
	// `SetMapEngine` es el camino del motor para un nivel nuevo: tira las imagenes y la paleta del nivel anterior
	// y reinicia el tramo de la cache desde el byte 0.
	rendererDelMapa.SetMapEngine(motorDelMapa)

	censoDelTramo(nombre + "-despues-de-colgar")
	memoria(nombre + "-despues-de-colgar")
	semiespacio(nombre + "-despues-de-colgar")
	muestraDeLaFase()

	nombreDelNivel = ds1

	// ============================== EL CUADRO DE LA FASE ==============================
	// La camara vuelve a la celda del host y los cuatro a la formacion: el nivel es otro y las casillas del mapa
	// no tienen por que ser las mismas.
	ponerALosCuatroEnLaCeldaDelHost()
	elCuadroDeLaFase(malla, f)

	f.semiespacioDepues = int(d2cGcUsado())

	println("JUEGO121: FASE", nombre, "FIN libre=", d2cMontonLibre(), "picoDeLaFase=", picoEnLaFase,
		"picoSemiespacio=", picoSemiespacioEnLaFase, "msCompose=", f.msCompose, "ok=", f.ok)
}

// componerNivelDeLaFase (DC-002.109): compone `preset/archivo` en el motor y devuelve el censo de la grilla.
//
// EL `Stamp` y su `.ds1` NO salen de aca: una variable local del que llama deja el `.ds1` clavado en la pila, y
// el barrido de libgodc es CONSERVADOR (medido en DC-002.109: 413.184 B vivos despues de una recoleccion
// completa). Por eso el `.ds1` vive en el monton de C (`TilesDelDS1EnArena`) y la recoleccion que lo tira corre
// DENTRO de esta funcion, con el marco ya reescrito por el nivel nuevo (`recolectarDespuesDeCargarElDs1`).
func componerNivelDeLaFase(engine *d2mapengine.MapEngine, preset, archivo int, fase string) (pisos, paredes int, crc uint32, ok bool) {
	t0 := d2cMs()

	stamp := engine.LoadStamp(regionPueblo, preset, archivo)
	if stamp == nil {
		return 0, 0, 0, false
	}

	tamano := stamp.Size()
	enArena := stamp.TilesDelDS1EnArena()
	bloque := stamp.BytesDeLaArenaDelDS1()

	println("JUEGO121: FASE", fase, "DS1", stamp.RegionPath(), "ancho=", tamano.Width, "alto=", tamano.Height,
		"enArena=", enArena, "bloque=", bloque)

	semiespacio(fase + "-despues-de-loadstamp")
	muestraDeLaFase()

	// EL TRAMO DE LAS CASILLAS: si todavia no esta, se pide aca (es la primera vez que hay un `.ds1` en la mano
	// para saber cuantas capas pide cada casilla). Despues se reusa en las otras fases.
	reservarElArenaDeLasCasillas(stamp, tamano.Width, tamano.Height)

	// EL RECOLECTOR, CON EL MARCO YA REESCRITO: el `Stamp` nuevo es la raiz de verdad, el `.ds1` vive en el
	// monton de C y todavia no se toco un solo `.dt1`, asi que nada se puede mover bajo los pies del parser.
	if recolectarDespuesDeCargarElDs1 {
		runtime.GC()

		semiespacio(fase + "-despues-de-recolectar")
		muestraDeLaFase()
	}

	engine.ResetMapParaStamp(regionPueblo, tamano.Width, tamano.Height, stamp)
	engine.PlaceStampClippedWhere(stamp, 0, 0, tamano.Width, tamano.Height, func(int) bool { return false }, false)

	semiespacio(fase + "-despues-de-colocar")

	cuantosArena, usadosArena, fallasArena := d2mapengine.EstadoDelArenaDeCasillas()
	println("JUEGO121: FASE", fase, "CASILLAS DEL ARENA tiles=", cuantosArena, "usados=", usadosArena,
		"fallas=", fallasArena, "montonLibre=", d2cMontonLibre())

	// el `.ds1` no lo mira nadie mas: las casillas del mapa son COPIAS de sus celdas
	soltados := stamp.SoltarLosTilesDelDS1()

	println("JUEGO121: FASE", fase, "DS1 SOLTADO", stamp.RegionPath(), "enArena=", enArena, "bloque=", bloque,
		"devueltos=", soltados, "montonLibre=", d2cMontonLibre())

	semiespacio(fase + "-despues-de-soltar-el-ds1")
	muestraDeLaFase()

	pisos, paredes, crc = censo(engine)

	println("JUEGO121: FASE", fase, "MAPA tiles=", len(*engine.Tiles()), "pisos=", pisos, "paredes=", paredes,
		"crcGrilla=", int(crc), "ms=", int(d2cMs()-t0))

	muestraDeLaFase()

	return pisos, paredes, crc, true
}

// resumenDeLasFases: los numeros del criterio, una linea por fase y una del ciclo.
func resumenDeLasFases() {
	semiespacio("ciclo-fin")
	memoria("ciclo-fin")

	println("JUEGO121: CICLO RESUMEN fases=", fasesHechas, "jugadores=", len(jugadores),
		"guionPoc=", guionEnUso, "leashCeldas=", leashEnCeldas, "leashDeGrupo=", leashDeGrupo,
		"split=", splitKnob, "cuadros=", cuadro)

	for i := 0; i < fasesHechas && i < len(fasesMedidas); i++ {
		f := &fasesMedidas[i]

		msMedio := 0
		if f.cuadros > 0 {
			msMedio = f.msMapaTotal / f.cuadros
		}

		println("JUEGO121: CICLO FASE", f.nombre, "ds1=", f.ds1, "ok=", f.ok, "cuadros=", f.cuadros,
			"crc32=", int(f.crcMapa), "esperado=", int(f.esperado), "igualAlHost=", f.igual,
			"crcConJugador=", int(f.crcConJugador), "pixelesNoVacios=", f.pixeles,
			"pixelesDeLosCuatro=", f.pixelesJug,
			"picoMonton=", f.picoMonton, "picoSemiespacio=", f.picoSemiespacio,
			"semiespacioAntes=", f.semiespacioAntes, "semiespacioDespues=", f.semiespacioDepues,
			"libreAlComponer=", f.libreAlComponer, "msCompose=", f.msCompose,
			"msMapaMedio=", msMedio, "msJugadores=", f.msJugadores,
			"conLosCuatro=", f.conLosCuatro)
	}

	// EL CRITERIO, EN UNA LINEA: las tres fases con los cuatro, y la vuelta con el crc32 del host.
	ok := fasesHechas == 3 && fasesMedidas[0].igual && fasesMedidas[1].igual && fasesMedidas[2].igual &&
		fasesMedidas[0].conLosCuatro && fasesMedidas[1].conLosCuatro && fasesMedidas[2].conLosCuatro

	println("JUEGO121: CICLO CRITERIO tresFasesConLosCuatro=", ok, "entrar=", int(fasesMedidas[0].crcMapa),
		"transicion=", int(fasesMedidas[1].crcMapa), "volver=", int(fasesMedidas[2].crcMapa),
		"esperado=", int(crcHost), "picoDelCiclo=", picoMonton, "jugadores=", len(jugadores))

	// el leash del camino: lo medido al arranque y la vigilancia de cuadro a cuadro (las fases tambien cuentan)
	resumenDelLeash()
}
