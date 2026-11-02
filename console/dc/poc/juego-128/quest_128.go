package main

// DC-002.128 -- LA DEN OF EVIL: ACEPTAR, AVANZAR Y COMPLETAR LA PRIMERA QUEST (etapa E7 del plan, docs/PLAN-JUEGO.md).
//
// QUE ES. El POC de DC-002.127 tiene pueblo con gente y panel de dialogo. Esta tarjeta le pone lo que hace que
// Diablo II sea un juego: la PRIMERA QUEST del Acto 1, la Den of Evil, con el motor de quests DEL JUEGO.
//
//  1. LAS REGLAS SON LAS DEL MOTOR, PORTADAS TAL CUAL. `d2common/d2quest` son 11.272 lineas y su cierre de imports
//     (medido en `console/dc/sonda-quest-128`) es **2 paquetes del motor** (`d2quest` y `d2fileformats/d2s`) y CERO
//     ebiten / cero d2asset: se porta ENTERO, como `d2npc` en DC-002.127. No se reescribe la quest: se le dan los
//     eventos. Con eso entran gratis los bits del guardado, las paginas del log, el marcador "!" del NPC, el sonido
//     de la quest y el punto de habilidad como EFECTO (`EffectSkillPoint`).
//
//  2. LOS EVENTOS SALEN DEL POC, NO DE UNA LISTA: hablar con Akara (`Activate` + `Hear(64)`), cambiar de area
//     (`EvAreaChanged` pueblo -> cueva de id 8), matar los bichos (`SyncDenMonsters` + `EvMonsterKilled`), volver al
//     pueblo y cobrar (`Hear(76)`). Cada uno se imprime con lo que la quest hizo: estado, bits del slot, pagina del
//     log, efectos y cambios de bit (los `Change` del motor).
//
//  3. EL TEXTO ES EL DEL JUEGO. La tabla de lineas habladas (`data/messages.csv`) NO viaja a la consola (parche de
//     port DC-002.8: gccgo no tiene `//go:embed`), asi que el POC no puede mostrar la linea hablada. Lo que SI
//     muestra es la PAGINA DEL LOG de la quest, por el camino del motor: `Game.Log()` da la pagina y la clave de
//     `string.tbl` es `qstsa1q1<page>` ("Look for the Den in the wilderness outside the Rogues' Camp.",
//     "Kill all the monsters in the Den.", "Return to Akara for a reward."). Se lee con el MISMO lector de
//     `string.tbl` de DC-002.127, al arrancar y desde la fuente prestada.
//
//  4. LA CUEVA ES EL AREA 8, Y ESO SE DICE. El POC todavia no compone el `.ds1` de la cueva; lo que esta tarjeta
//     modela es el ESTADO de la quest, y para el motor lo que importa es el id del area (8, `LevelDenOfEvil`) y las
//     bajas de ESA area. Los bichos son los del POC (`MONSTRUOS`), muertos con el MISMO dado y el MISMO camino que
//     el golpe del boton Y de DC-002.122 (`Combate.GolpeDelHeroe` + `cambiarModoDelBicho`, con su DT y su cadaver).
//
//  5. LO QUE SOBREVIVE. El estado de la quest vive en el REGISTRO DEL GUARDADO (`d2s.QuestRecord`, 96 B por
//     dificultad, slot 1 = 2 B): por eso sobrevive al cambio de nivel (las tres fases de DC-002.120 recomponen
//     niveles y la quest se imprime identica en cada una) y al CARGAR LA PARTIDA (un `d2quest.New` sobre el mismo
//     registro + `Start`, que es el camino del motor: la quest queda INERTE y el log dice COMPLETADA).
//
// LO QUE NO ENTRA (y es de la tarjeta siguiente): las skills (E8). El punto de habilidad se COBRA y se guarda en el
// resumen; no se gasta. Tampoco entran las quests 2 a 6.
//
// LAS PALANCAS DE ESTA TARJETA: `questKnob` (si/no: el A/B del costo, con no la corrida es la de DC-002.127),
// `questCuadro*` (los cuadros del guion de la quest), `questClaseBicho` (la clase del bicho de la cueva).

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2quest"
)

const (
	// los dos mensajes que el nodo A1Q1 del exe atiende (`d2quest/a1_basic.go`, quest de la Den of Evil):
	// 64 = Akara entrega la quest; 76 = Akara agradece y se cobra.
	msgAceptaLaQuest = 64
	msgCobraLaReward = 76

	// los ids de area de `levels.txt` que usa la quest (d2quest/ids.go)
	nivelDelPuebloQuest = d2quest.LevelRogueEncampment // 1 Rogue Encampment
	nivelDeLaCuevaQuest = d2quest.LevelDenOfEvil       // 8 Den of Evil

	// la clase del heroe del POC (la amazona, clase 0 del exe) y la del bicho que pone el POC (el Returned)
	claseAmazonicaQuest = 0
	claseReturnedQuest  = questClaseBicho

	// la clave del nombre de la quest en `string.tbl`: el nombre es `qstsa1q1` y cada pagina es `qstsa1q1<n>`
	claveNombreQuest = "qstsa1q1"

	// el alcance del golpe heroe->bicho (el VERIFICADO del exe, el mismo de DC-002.122)
	alcanceQuest = 7

	// el radio de habla del panel (el mismo de DC-002.127)
	radioQuest = 2.0

	// la pagina del log que se pide para cada momento del guion
	paginaAceptadaQuest = 1 // "Look for the Den in the wilderness outside the Rogues' Camp."
	paginaCuevaQuest    = 2 // "Kill all the monsters in the Den."
	paginaLimpiaQuest   = 5 // "Return to Akara for a reward."
)

// elGuionDeLaQuest: los cuadros del POC en los que pasa cada cosa. Es un GUION y no un instrumento de dibujo: el
// emulador de estas corridas no tiene teclado, asi que el boton de hablar y el cambio de area los pone el guion, por
// la MISMA puerta que el control (proximidad + boton, `elBotonDeHablar`) y por los mismos eventos del motor.
type elGuionDeLaQuest struct {
	Acepta   int // hablar con Akara y aceptar (Hear 64)
	Cierra   int // cerrar el panel del aceptar
	Cueva    int // cambiar de area: pueblo -> cueva (id 8)
	Mata     int // el primer cuadro de los bichos: uno por cuadro
	Limpia   int // la cueva quedo limpia: la quest pasa la pagina
	Vuelve   int // volver al pueblo (area 8 -> 1)
	Cobra    int // hablar con Akara y cobrar (Hear 76)
	Verifica int // la prueba de la supervivencia (juego nuevo sobre el registro)
}

var guionDeLaQuest = elGuionDeLaQuest{
	Acepta:   questCuadroAcepta,
	Cierra:   questCuadroCierra,
	Cueva:    questCuadroCueva,
	Mata:     questCuadroMata,
	Limpia:   questCuadroLimpia,
	Vuelve:   questCuadroVuelve,
	Cobra:    questCuadroCobra,
	Verifica: questCuadroVerifica,
}

var (
	questJuego *d2quest.Game
	questRec   d2s.QuestRecord

	// el texto del juego: el nombre de la quest y el de cada pagina del log (copias de Go: la tabla prestada se
	// devuelve en el acto, el tropiezo 6 de DC-002.127)
	questNombre  string
	questPaginas [16]string

	// lo que se mide
	questAceptada        bool
	questContadorAvanzo  bool
	questCompletada      bool
	questCobrada         bool
	questPuntoDeHabilidad int
	questGolpes          int
	questDano            int
	questMuertes         int
	questBichosVivos     int
	questBitsAlCierre    uint16
	questEstadoAlCierre  string
	questPaginaAlCierre  int
	questSobrevive       bool
	questSobreviveCarga  bool
	questVecesPanel      int
	questPxDelPanel      int
	questBytesDelPanel   int
	questMsDelPanel      int
	questLienzoAceptar   uint32
	questLienzoCobrar    uint32
	questFasesIguales    bool
	questFasesMedidas    int
	questAkaraX          float64
	questAkaraY          float64
	questDestinos        int
	questEfectosDelCobro []d2quest.Effect
)

// ============================== LA CARGA ==============================

// cargarLaQuest: arma el juego de la quest con el registro de ESTE personaje (96 B, el del guardado), con la clase y
// el nivel del heroe del POC, y corre `Start` (el camino del motor al entrar al juego). Va con las fuentes abiertas
// porque de ahi salen los textos de las paginas del log (`string.tbl`).
func cargarLaQuest() {
	if questKnob != "si" {
		println("JUEGO128: QUEST apagada (quest=", questKnob, "): la corrida es la de DC-002.127 sin una linea de quest")

		return
	}

	questJuego = d2quest.New(&questRec, nil, 0)
	questJuego.Hero = d2quest.Hero{Class: claseAmazonicaQuest, Level: 1}
	questJuego.Level = nivelDelPuebloQuest
	questJuego.Start()

	q := questJuego.Quest(d2quest.QuestDenOfEvil)

	if q == nil {
		println("JUEGO128: QUEST el motor no tiene el nodo de la Den of Evil")

		return
	}

	println("JUEGO128: QUEST ARRANCA quest=", q.Name, "id=", q.ID, "slot=", q.Slot, "act=", q.Act,
		"logIndex=", q.LogIndex, "questsDelMotor=", len(questJuego.Quests), "registroBytes=", len(questRec))
	println("JUEGO128: QUEST ESTADO-INICIAL ", questJuego.Describe(q))

	// LOS TEXTOS, DEL JUEGO: el nombre y las paginas del log de ESTA quest, por el camino del motor (Log() da la
	// pagina) y el lector de `string.tbl` de DC-002.127. Se copian a Go y la tabla prestada se devuelve.
	// OJO: `cargarLosNPCs` deja la tabla SOLTADA (`soltarLaTablaDeStrings`), asi que se vuelve a pedir; con el ciclo
	// encendido la fuente sigue abierta (el paso 5 no suelta nada).
	cargarLaTablaDeStrings()

	questNombre = textoDeStringTbl(claveNombreQuest)

	for page := 1; page < len(questPaginas); page++ {
		questPaginas[page] = textoDeStringTbl(claveNombreQuest + itoa(page))
	}

	lineas := 0

	for page := 1; page < len(questPaginas); page++ {
		if questPaginas[page] != "" {
			lineas++
		}
	}

	println("JUEGO128: QUEST TEXTO nombre=", questNombre, "paginasConTexto=", lineas, "de", len(questPaginas)-1,
		"paginaAceptada=", recorteQuest(questPaginas[paginaAceptadaQuest], 70),
		"paginaCueva=", recorteQuest(questPaginas[paginaCuevaQuest], 40),
		"paginaLimpia=", recorteQuest(questPaginas[paginaLimpiaQuest], 40))

	// LA TABLA SE DEJA PUESTA SI VIENEN LOS NPCs: ellos la piden `cargarLaTablaDeStrings` y, como ya esta, no leen
	// nada; la sueltan ellos al terminar (`soltarLaTablaDeStrings`), igual que en DC-002.127. Soltarla aca seria
	// obligarlos a leer los 390.570 B otra vez, que es la segunda copia que no entra en el monton de C.
	if npcKnob != "si" {
		soltarLaTablaDeStrings()
	}
}

// ============================== EL GUION ==============================

// pasoDeLaQuest: el cuadro del POC. Cada cuadro del guion hace UNA cosa y la cuenta; el resto de los cuadros no
// tocan la quest (la quest es del motor, no del dibujo).
func pasoDeLaQuest() {
	if questKnob != "si" || questJuego == nil {
		return
	}

	n := cuadro + 1

	switch {
	case n == guionDeLaQuest.Acepta:
		questAceptaLaQuest()
	case n == guionDeLaQuest.Cierra:
		questCierraElPanel("aceptar")
	case n == guionDeLaQuest.Cueva:
		questVaALaCueva()
	case n >= guionDeLaQuest.Mata && n < guionDeLaQuest.Mata+len(bichos):
		questMataUnBicho(n - guionDeLaQuest.Mata)
	case n == guionDeLaQuest.Limpia:
		questLaCuevaQuedoLimpia()
	case n == guionDeLaQuest.Vuelve:
		questCierraElPanel("cueva")
		questVuelveAlPueblo()
	case n == guionDeLaQuest.Cobra:
		questCobraLaRecompensa()
	case n == guionDeLaQuest.Verifica:
		questCierraElPanel("cobrar")
		questPruebaDeSupervivencia()

		// EL CIERRE DEL GUION SE IMPRIME ACA, no solo al final de la corrida: despues de estos once cuadros el POC
		// sigue con las fases del ciclo de niveles de DC-002.120 (recomponer el nivel cuesta memoria, y con la quest
		// cargada esa recompostura es lo primero que se queda sin ella). Si el emulador muere ahi, los numeros del
		// cierre tienen que estar YA en el log: no se mide dos veces.
		println("JUEGO128: QUEST CIERRE-DEL-GUION (once cuadros; las fases del ciclo de niveles vengono despues)")
		resumenDeLaQuest()
	}
}

// questAceptaLaQuest: el jugador esta en la celda de Akara (las corridas ponen VISTA_X/Y en su celda) y el boton
// abre el panel. El estado lo mueve el MOTOR: `Activate` arma la lista de lo que dice (vacia en la consola: la tabla
// de lineas no viaja) y `Hear(64)` es el mensaje que el nodo A1Q1 atiende para entregar la quest.
func questAceptaLaQuest() {
	n := elNPCMasCercano()
	if n == nil {
		println("JUEGO128: QUEST SIN-NPC no hay ningun NPC cargado (NPC_LISTA)")

		return
	}

	ax, ay := celdaDelJugadorX(), celdaDelJugadorY()

	dx, dy := ax-n.CeldaX, ay-n.CeldaY
	distancia := raizCuadrada(dx*dx + dy*dy)

	println("JUEGO128: QUEST HABLA npc=", n.Nombre, "claseExe=", n.ClaseExe, "jugador=", ax, ay,
		"distancia=", distancia, "radio=", radioQuest, "cuadro=", cuadro+1)

	if n.ClaseExe != claseAkaraNPC || n.ClaseExe != d2quest.NPCAkara {
		println("JUEGO128: QUEST NO-HABLA el NPC mas cercano no es Akara: claseExe=", n.ClaseExe,
			"akara=", d2quest.NPCAkara)

		return
	}

	if distancia > radioQuest {
		println("JUEGO128: QUEST NO-HABLA distancia=", distancia, "radio=", radioQuest)

		return
	}

	questAkaraX, questAkaraY = n.CeldaX, n.CeldaY

	// el marcador del motor: la "!" que el cliente dibuja sobre la cabeza cuando hay quest que dar
	println("JUEGO128: QUEST MARCADOR npc=", n.Nombre, "activa=", questJuego.Marker(n.ClaseExe))

	d := questJuego.Activate(n.ClaseExe)
	println("JUEGO128: QUEST ACTIVATE npc=", n.Nombre, "lineasDelMotor=", len(d.Lines),
		"topicos=", len(d.Topics()), "(la tabla de lineas habladas vive en messages.csv, que no viaja a la consola)")

	efectos := questJuego.Hear(n.ClaseExe, msgAceptaLaQuest)
	questImprime("ACEPTA", efectos)
	questJuego.Close(n.ClaseExe)

	q := questJuego.Quest(d2quest.QuestDenOfEvil)

	if q != nil && questJuego.Rec.Get(q.Slot, d2quest.FlagStarted) {
		questAceptada = true
	}

	// el panel: el nombre del NPC arriba y el texto de la PAGINA DEL LOG, que es el del juego
	questAbreElPanel(n.Nombre, questJuego.LogPage(q))
}

// questVaALaCueva: el cambio de area, por el evento del motor. Es lo que el exe dispara al entrar a la cueva y lo
// que lleva la quest de "aceptada" a "hay que matarlos".
func questVaALaCueva() {
	efectos := questJuego.Dispatch(d2quest.Event{Kind: d2quest.EvAreaChanged,
		OldLevel: nivelDelPuebloQuest, NewLevel: nivelDeLaCuevaQuest})

	println("JUEGO128: QUEST AREA de=", nivelDelPuebloQuest, "a=", nivelDeLaCuevaQuest,
		"(Den of Evil) cuadro=", cuadro+1)

	questImprime("CUEVA", efectos)
	questBichosVivos = questBichosConVida()
	questJuego.SetDenMonsters(questBichosVivos)

	println("JUEGO128: QUEST CUEVA BICHOS vivos=", questBichosVivos, "clase=", claseReturnedQuest,
		"quedanSegunElMotor=", questJuego.DenMonstersLeft())
}

// questMataUnBicho: UN bicho de la cueva, con el camino del golpe de DC-002.122. El instrumento que falta es el
// TECLADO: en el emulador de estas corridas nadie aprieta el boton Y, y ademas el A1 del heroe tarda 13 cuadros
// (520 ms) por golpe. Se camina al jugador a la celda de al lado (la que SI alcanza segun la tabla del exe) y se le
// dan los golpes con el dado del motor hasta que el bicho muere. Se dice en el log.
func questMataUnBicho(i int) {
	if i < 0 || i >= len(bichos) {
		return
	}

	b := bichos[i]

	if b.Vida == nil || b.Combate == nil || b.Vida.HP <= 0 {
		println("JUEGO128: QUEST MATA bicho=", b.Nombre, "ya estaba muerto: no cuenta")

		return
	}

	p := jugadores[0]
	bx, by := b.Est.Celda()

	// EL INSTRUMENTO: la celda de al lado del bicho, en la misma fila (dx=5, dy=0 -> la tabla del exe da 5 y el
	// alcance verificado es 7). Sin esto el bicho 2 y el 3 quedan en diagonal o a dos celdas y NINGUN golpe llega.
	p.Mov.Est = NuevoEstado(bx-1, by, false)
	p.Mov.Direccion = direccionHaciaElBicho(p, b)
	p.Mov.Caminando = false

	dx, dy := separacionDelBicho(p, b)
	distancia := DistanciaDeMesa(dx, dy, 2, tamanoBichoCombate)
	alcanza := AlcanzaElGolpe(dx, dy, tamanoBichoCombate, alcanceQuest)

	println("JUEGO128: QUEST GOLPE bicho=", b.Nombre, "celda=", bx, by, "jugador=", bx-1, by, "dx=", dx,
		"dy=", dy, "distancia=", distancia, "alcance=", alcanceQuest, "alcanza=", alcanza, "vida=", b.Vida.HP)

	if !alcanza {
		println("JUEGO128: QUEST FUERA bicho=", b.Nombre, "distancia=", distancia, "alcance=", alcanceQuest,
			": no se le pega (el contador NO avanza)")

		return
	}

	ahora := int64(d2cMs())
	golpes, dano := 0, 0
	muerto := false

	for b.Vida.HP > 0 && golpes < 64 {
		d, antes, despues, murio := b.Combate.GolpeDelHeroe()
		if d == 0 {
			break
		}

		golpes++
		dano += d

		println("JUEGO128: QUEST GOLPE-UNO bicho=", b.Nombre, "golpe=", golpes, "dano=", d, "vidaAntes=", antes,
			"vidaDespues=", despues, "muerto=", murio)

		if murio {
			muerto = true
		}
	}

	questGolpes += golpes
	questDano += dano

	if muerto {
		questContadorAvanzo = true
	}

	if muerto {
		questMuertes++
		cambiarModoDelBicho(b, modoBestiaMuriendo, ahora)
	}

	// EL EVENTO DEL MOTOR, con el conteo del nivel como lo hace el exe: los bichos que quedan VIVOS en la cueva
	vivos := questBichosConVida()
	questJuego.SyncDenMonsters(vivos)

	efectos := questJuego.Dispatch(d2quest.Event{Kind: d2quest.EvMonsterKilled,
		Level: nivelDeLaCuevaQuest, Monster: claseReturnedQuest})

	q := questJuego.Quest(d2quest.QuestDenOfEvil)
	println("JUEGO128: QUEST MATA bicho=", b.Nombre, "golpes=", golpes, "dano=", dano, "vida=", b.Vida.HP,
		"vivosEnLaCueva=", vivos, "quedaSegunElMotor=", questJuego.DenMonstersLeft(),
		"estado=", estadoDeLaQuest(q), "pagina=", questJuego.LogPage(q), "cuadro=", cuadro+1)

	questImprime("MATA", efectos)
}

// questLaCuevaQuedoLimpia: el motor pasa la pagina de la quest con su propio reloj (`after(8)`); se le dan esos 8
// cuadros y se muestra la pagina que queda. Si el ultimo bicho cerro la cueva, la recompensa queda PENDIENTE.
func questLaCuevaQuedoLimpia() {
	q := questJuego.Quest(d2quest.QuestDenOfEvil)

	efectos := questJuego.Tick(8)

	questCompletada = q != nil && questJuego.Rec.Get(q.Slot, d2quest.FlagPrimaryGoal)

	println("JUEGO128: QUEST LIMPIA cuadro=", cuadro+1, "vivos=", questBichosConVida(),
		"estado=", estadoDeLaQuest(q), "pagina=", questJuego.LogPage(q),
		"recompensaPendiente=", q != nil && questJuego.Rec.Get(q.Slot, d2quest.FlagRewardPending),
		"marcador=", questJuego.Marker(d2quest.NPCAkara))

	questImprime("LIMPIA", efectos)
	questAbreElPanel("AKARA", questJuego.LogPage(q))
}

// questVuelveAlPueblo: el otro cambio de area (cueva -> pueblo). El estado de la quest no cambia: el registro esta
// escrito y la quest ya esta en "por cobrar".
func questVuelveAlPueblo() {
	efectos := questJuego.Dispatch(d2quest.Event{Kind: d2quest.EvAreaChanged,
		OldLevel: nivelDeLaCuevaQuest, NewLevel: nivelDelPuebloQuest})

	println("JUEGO128: QUEST AREA de=", nivelDeLaCuevaQuest, "a=", nivelDelPuebloQuest, "cuadro=", cuadro+1)

	questImprime("VUELVE", efectos)
}

// questCobraLaRecompensa: el jugador vuelve a la celda de Akara y el mensaje 76 del nodo A1Q1 es el que cobra: el
// motor marca la quest como hecha, la saca de "por cobrar" y emite `EffectSkillPoint(1)` (mas el respec de Akara y
// la pagina del log). El punto se GUARDA en el resumen: gastarlo es E8, no esta tarjeta.
func questCobraLaRecompensa() {
	p := jugadores[0]

	// el instrumento: el jugador se pone en la celda de Akara (la de la corrida: la palanca VISTA_X/Y la fijo)
	p.Mov.Est = NuevoEstado(questAkaraX, questAkaraY, false)

	ax, ay := celdaDelJugadorX(), celdaDelJugadorY()
	dx, dy := ax-questAkaraX, ay-questAkaraY
	distancia := raizCuadrada(dx*dx + dy*dy)

	println("JUEGO128: QUEST COBRA npc=AKARA jugador=", ax, ay, "distancia=", distancia, "radio=", radioQuest,
		"marcador=", questJuego.Marker(d2quest.NPCAkara), "cuadro=", cuadro+1)

	if distancia > radioQuest {
		println("JUEGO128: QUEST NO-COBRA distancia=", distancia, "radio=", radioQuest)

		return
	}

	questJuego.Activate(d2quest.NPCAkara)

	efectos := questJuego.Hear(d2quest.NPCAkara, msgCobraLaReward)
	questEfectosDelCobro = efectos

	for _, e := range efectos {
		if e.Kind == d2quest.EffectSkillPoint {
			questPuntoDeHabilidad += e.Value
		}
	}

	q := questJuego.Quest(d2quest.QuestDenOfEvil)

	if q != nil {
		questCobrada = questJuego.Rec.Get(q.Slot, d2quest.FlagRewardGranted)
	}

	println("JUEGO128: QUEST COBRO puntoDeHabilidad=", questPuntoDeHabilidad, "estado=", estadoDeLaQuest(q),
		"log=", estadoDelLogDeLaQuest(), "marcador=", questJuego.Marker(d2quest.NPCAkara))

	questImprime("COBRA", efectos)
	questAbreElPanel("AKARA", questJuego.LogPage(q))
}

// questPruebaDeSupervivencia: la prueba de que la quest vive en el REGISTRO y no en el nivel. Se copia el registro
// (lo que el motor escribe en el guardado) y se arma un juego NUEVO sobre la copia: `New` + `Start` es exactamente
// el camino de cargar la partida. La quest tiene que quedar INERTE (NotIntro=false, Active=false) y el log
// COMPLETADA.
func questPruebaDeSupervivencia() {
	var copia d2s.QuestRecord

	copy(copia[:], questRec[:])

	nuevo := d2quest.New(&copia, nil, 0)
	nuevo.Hero = d2quest.Hero{Class: claseAmazonicaQuest, Level: 1}
	nuevo.Level = nivelDelPuebloQuest
	nuevo.Start()

	q := nuevo.Quest(d2quest.QuestDenOfEvil)
	if q == nil {
		return
	}

	questSobreviveCarga = !q.NoSetState && !q.NotIntro && !q.Active &&
		nuevo.Rec.Get(q.Slot, d2quest.FlagRewardGranted)

	println("JUEGO128: QUEST SUPERVIVE registroBytes=", len(copia), "bitsQueSeGuardan=", int(questRec.Slot(q.Slot)),
		"bitsTrasCargar=", int(copia.Slot(q.Slot)), "inerte=", !q.NotIntro && !q.Active,
		"recompensaCobrada=", nuevo.Rec.Get(q.Slot, d2quest.FlagRewardGranted),
		"log=", estadoDelLogDelJuego(nuevo), "marcador=", nuevo.Marker(d2quest.NPCAkara))
	println("JUEGO128: QUEST ESTADO-TRAS-CARGAR ", nuevo.Describe(q))
}

// laQuestEnLaFase: la pruebade la supervivencia al CAMBIO DE NIVEL de verdad: las tres fases de DC-002.120
// recomponen niveles (ENTRAR, TRANSICION, VOLVER). En cada una se imprime el estado de la quest tal cual esta: si no
// cambio ninguna, la quest sobrevivio al cambio de nivel.
func laQuestEnLaFase(nombre string) {
	if questKnob != "si" || questJuego == nil {
		return
	}

	q := questJuego.Quest(d2quest.QuestDenOfEvil)
	if q == nil {
		return
	}

	questFasesMedidas++

	bits := questRec.Slot(q.Slot)
	estado := estadoDeLaQuest(q)

	println("JUEGO128: QUEST FASE nombre=", nombre, "bits=", int(bits), "estado=", estado,
		"pagina=", questJuego.LogPage(q), "log=", estadoDelLogDeLaQuest(),
		"igualAlCierre=", questBitsAlCierre == bits && questEstadoAlCierre == estado,
		"marcador=", questJuego.Marker(d2quest.NPCAkara))

	if questBitsAlCierre != bits || questEstadoAlCierre != estado {
		questFasesIguales = false
	}
}

// ============================== EL PANEL ==============================

// questAbreElPanel: el panel de DC-002.127 con el nombre del NPC arriba y la PAGINA DEL LOG de la quest abajo. La
// geometria, el fondo y los glifos son los del panel del motor (npc_menu.go) que ya estan en el POC: lo unico que
// cambia es de donde sale el texto, y sale del mismo `string.tbl`.
func questAbreElPanel(nombreNPC string, pagina int) {
	texto := ""

	if pagina >= 0 && pagina < len(questPaginas) {
		texto = questPaginas[pagina]
	}

	lineas := []string{"QUEST: " + questNombre}

	if texto != "" {
		lineas = append(lineas, cortarElTexto(texto)...)
	} else {
		lineas = append(lineas, "(sin pagina "+itoa(pagina)+" en string.tbl)")
	}

	lineas = append(lineas, "LOG PAGE: "+itoa(pagina)+"  STATE: "+estadoDeLaQuest(questJuego.Quest(d2quest.QuestDenOfEvil)))

	n := &npc{Nombre: nombreNPC, Lineas: lineas}
	armarElPanelDelDialogo(n)

	dialogoAbierto = true
	questVecesPanel++

	if elDialogo != nil {
		questBytesDelPanel = elDialogo.Bytes
	}

	println("JUEGO128: QUEST PANEL cuadro=", cuadro+1, "npc=", nombreNPC, "pagina=", pagina,
		"renglones=", len(lineas), "panel=", elDialogo.Ancho, "x", elDialogo.Alto, "bytes=", questBytesDelPanel,
		"texto=", recorteQuest(texto, 70))
}

// questCierraElPanel: el panel se cierra y el lienzo tiene que volver al mapa solo (el mismo criterio de DC-002.127:
// con el panel cerrado no se pinta un pixel). Se guarda el crc32 de cada momento para el resumen.
func questCierraElPanel(momento string) {
	cerrado := dialogoAbierto
	dialogoAbierto = false
	elDialogo = nil
	elDialogoSeCerro = true

	println("JUEGO128: QUEST PANEL-CIERRA cuadro=", cuadro+1, "momento=", momento, "estabaAbierto=", cerrado)
}

// ============================== EL RESUMEN ==============================

// resumenDeLaQuest: los numeros del cierre y el criterio de la tarjeta.
func resumenDeLaQuest() {
	if questKnob != "si" || questJuego == nil {
		println("JUEGO128: QUEST RESUMEN apagada")

		return
	}

	q := questJuego.Quest(d2quest.QuestDenOfEvil)

	questBitsAlCierre = questRec.Slot(q.Slot)
	questEstadoAlCierre = estadoDeLaQuest(q)
	questPaginaAlCierre = questJuego.LogPage(q)
	questSobrevive = questFasesMedidas > 0 && questFasesIguales

	sobrevive := "no-medido"
	if questFasesMedidas > 0 {
		sobrevive = "false"
		if questSobrevive {
			sobrevive = "true"
		}
	}

	println("JUEGO128: QUEST RESUMEN quest=", q.Name, "slot=", q.Slot, "bits=", int(questBitsAlCierre),
		"estado=", questEstadoAlCierre, "pagina=", questPaginaAlCierre, "log=", estadoDelLogDeLaQuest(),
		"aceptada=", questAceptada, "contadorAvanzo=", questContadorAvanzo, "completada=", questCompletada,
		"cobrada=", questCobrada, "puntoDeHabilidad=", questPuntoDeHabilidad)
	println("JUEGO128: QUEST RESUMEN MEDIDO golpes=", questGolpes, "dano=", questDano, "muertes=", questMuertes,
		"paneles=", questVecesPanel, "bytesDelPanel=", questBytesDelPanel, "pxDelPanel=", questPxDelPanel,
		"msDelPanel=", questMsDelPanel, "fasesMedidas=", questFasesMedidas, "fasesIguales=", questFasesIguales,
		"sobreviveAlNivel=", sobrevive, "sobreviveAlCargar=", questSobreviveCarga)
	println("JUEGO128: QUEST CRITERIO seAcepto=", questAceptada, "elContadorAvanzo=", questContadorAvanzo,
		"seCompleto=", questCompletada, "seCobro=", questCobrada, "cobroPuntoDeHabilidad=", questPuntoDeHabilidad,
		"sobreviveAlCambioDeNivel=", sobrevive, "sobreviveAlCargarLaPartida=", questSobreviveCarga)
}

// ============================== las ayudas ==============================

// questImprime: lo que el motor hizo con el evento: los efectos y los cambios de bit, con el nombre del paso.
func questImprime(paso string, efectos []d2quest.Effect) {
	var nombres []string

	for _, e := range efectos {
		nombres = append(nombres, nombreDelEfectoDeLaQuest(e.Kind)+"("+itoa(e.Value)+")")

		if e.Kind == d2quest.EffectSound && e.Value == 35 {
			println("JUEGO128: QUEST SONIDO valor=", e.Value, "(el 35 es el de la Den of Evil terminada)")
		}
	}

	for _, c := range questJuego.TakeChanges() {
		println("JUEGO128: QUEST BIT paso=", paso, "slot=", c.Slot, "antes=", int(c.Before), "despues=", int(c.After),
			"motivo=", c.Reason, "estado=", c.StateAfter)
	}

	q := questJuego.Quest(d2quest.QuestDenOfEvil)

	if len(nombres) > 0 {
		println("JUEGO128: QUEST EFECTOS paso=", paso, "efectos=", len(nombres), joinQuest(nombres))
	}

	println("JUEGO128: QUEST PASO paso=", paso, "bits=", int(questRec.Slot(q.Slot)), "estado=", estadoDeLaQuest(q),
		"log=", estadoDelLogDeLaQuest())
}

func nombreDelEfectoDeLaQuest(k d2quest.EffectKind) string {
	nombres := []string{"LogUpdate", "SkillPoint", "HireRogues", "Imbue", "GiveItem", "DeleteItem", "Sound",
		"Respec", "Portal", "Bark", "Spawn", "UnlockAct", "Reward"}

	if int(k) < len(nombres) {
		return nombres[k]
	}

	return "?"
}

// estadoDeLaQuest: el estado del nodo en palabras (los numeros del exe son 1..5).
func estadoDeLaQuest(q *d2quest.Quest) string {
	if q == nil {
		return "?"
	}

	switch q.State {
	case 1:
		return "sin-aceptar"
	case 2:
		return "aceptada"
	case 3:
		return "en-la-cueva"
	case 4:
		return "cueva-limpia"
	case 5:
		return "cobrada"
	}

	return "estado-" + itoa(q.State)
}

// estadoDelLogDeLaQuest: lo que el panel del log del juego diria de esta quest.
func estadoDelLogDeLaQuest() string { return estadoDelLogDelJuego(questJuego) }

func estadoDelLogDelJuego(g *d2quest.Game) string {
	for _, l := range g.Log() {
		if l.Quest != d2quest.QuestDenOfEvil {
			continue
		}

		switch l.Status {
		case d2quest.LogNotStarted:
			return "sin-empezar"
		case d2quest.LogInProgress:
			return "en-curso"
		case d2quest.LogCompleting:
			return "por-cobrar"
		case d2quest.LogCompleted:
			return "COMPLETADA"
		}
	}

	return "?"
}

// questBichosConVida: cuantos bichos del POC siguen vivos (lo que el motor llama "los monstruos que quedan en la
// cueva").
func questBichosConVida() int {
	vivos := 0

	for _, b := range bichos {
		if b.Vida != nil && b.Vida.HP > 0 {
			vivos++
		}
	}

	return vivos
}

func joinQuest(l []string) string {
	out := ""

	for i, s := range l {
		if i > 0 {
			out += " "
		}

		out += s
	}

	return out
}

func recorteQuest(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[:n] + "..."
}
