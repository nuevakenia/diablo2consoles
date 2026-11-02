package main

// DC-002.127 -- LOS NPCs Y EL DIALOGO: HABLAR EN EL PUEBLO (etapa E6 del plan).
//
// QUE ES. El pueblo del Acto 1 tiene gente: Warriv, Charsi y Kashya se DIBUJAN en la celda en la que el `.ds1`
// los coloca (con su `.dcc` y su animacion de quieto), el jugador se les acerca y con un boton sale el PANEL DE
// DIALOGO con lo que dice el NPC, sacado de `string.tbl` (la tabla que el HUD no pudo cargar en DC-002.125).
//
// DE DONDE SALE CADA COSA (nada elegido a ojo; todo medido en `console/dc/sonda-npc-127`):
//
//   - LOS NPCs NO SON OBJETOS DEL `.ds1`: son MONSTRUOS (type = 1) de los cuatro `.ds1` del pueblo
//     (townE1/N1/W1/S1). Su `ID` es el indice de `MonPreset.txt` del acto (act -> lista de `Place`), y el `Place`
//     se busca en `monstats.txt` (`Id` == Place) para sacar el `Code`, que es la CARPETA del MPQ y el token del
//     COF. Medido: el pueblo trae 15 monstruos en townN1 y 18 en townE1; de esos son gente con la que se habla
//     `warriv1` (WA), `charsi` (CI), `kashya` (RC), `akara` (PS) y `gheed` (GH); el resto son `rogue1` (RG),
//     `chicken` (CK) y `cow` (CW).
//   - LA ANIMACION DE QUIETO: el COF `<code>NU<arma>.cof` con 8 direcciones (WA 16 cuadros por direccion, CI 13,
//     RC 12) y las `.dcc` de sus capas. OJO CON DOS COSAS MEDIDAS: (a) el arma NO es la misma en todas las capas
//     (la declara cada capa del COF, como en el DT del bicho de DC-002.126) y (b) la ruta de la capa en el MPQ va
//     con BARRAS INVERTIDAS: con `/` el archivo "no existe" (medido: las diez capas salian "SIN GRAFICO").
//   - EL TEXTO: las reglas del exe ya estan portadas en el motor (`d2common/d2npc`: 46 NPCs, 418 lineas, 314
//     claves de mensaje). Aca se usan tal cual: `d2npc.EntryFor(claseExe)` -> `Record.Eligible(claseDelHeroe)` ->
//     `d2npc.MessageKey(msg)` -> `string.tbl`. El heroe del POC es la amazona (clase 0 del exe).
//   - EL PANEL: la geometria es la del motor (`d2game/d2player/npc_menu.go`, UI_CreateDialog 0x4b47b0): ancho =
//     el texto mas ancho + 20, alto = el nombre 0x15 + cada fila 0xf + 15, recortado a 10 px de los bordes. En
//     640x480 el texto de una linea de gossip mide ~340 px a font16, asi que entra sin recortar (medido en la
//     sonda). El fondo es un rectangulo de indices oscuro y el texto va con los glifos de `font16`, con la paleta
//     de las unidades: los MISMOS colores que el HUD saca de la paleta, no un RGB inventado.
//
// LO QUE NO ENTRA (y es la tarjeta siguiente): las quests (DC-002.128) y los mercaderes (DC-002.129). Aca se
// habla: el panel muestra el nombre del NPC y lo que dice; el menu (Talk / Trade / ...) se compone con las reglas
// de `d2npc.Options` y se MUESTRA, pero elegir una fila que no sea Talk es de las otras dos tarjetas.
//
// LAS PALANCAS: `npcKnob` (si/no, la corrida entera con o sin NPCs y dialogo, para el A/B del costo),
// `npcLista` (que NPCs se dibujan: "warriv1,charsi,kashya"), `hablarAuto` (el instrumento que aprieta el boton
// cuando el jugador esta cerca: el emulador corre sin teclado), `cuadroAbreElDialogo` y `cerrarElDialogo`.

import (
	"hash/crc32"
	"runtime"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/fuenteconsola"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2font"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2npc"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

const (
	// las rutas del MPQ (con el nombre que tiene el archivo, medido en la sonda)
	rutaTablaMonPresetNPC = `data\global\excel\MonPreset.txt`
	rutaTablaMonStatsNPC  = `data\global\excel\monstats.txt`
	rutaTablaStringsNPC   = `data\local\lng\ENG\string.tbl`
	rutaTablaPatchNPC     = `data\local\lng\ENG\patchstring.tbl`
	rutaFuenteNPC         = `data\local\font\latin\font16`

	// base y variante de capa de los monstruos (composite.go: monsterBasePath + "LIT" cuando no hay equipo)
	valorCapaNPC = "LIT"
	modoQuietoNPC = "NU"
	msPorCuadroNPC = 200

	// LA GEOMETRIA DEL PANEL, tal cual esta en `npc_menu.go` (UI_CreateDialog 0x4b47b0)
	npcTituloAvance = 0x15 // 21 px la linea del nombre
	npcFilaAvance   = 0xf  // 15 px cada fila del menu
	npcAnchoPad     = 20
	npcAltoPad      = 15
	npcBorde        = 10
	npcBordeAbajo   = 0x3a

	// el radio de "se le puede hablar": la misma regla del alijo (radioDelAlijo = 2,0 celdas, DC-002.124).
	// En el motor el NPC se abre con el PUNTERO; en la consola no hay puntero: es proximidad + boton.
	radioDeHablaNPC = 2.0

	// los ids de clase del exe de los NPCs del pueblo (los que usa `d2npc`)
	claseWarriv1NPC = 155
	claseCharsiNPC  = 154
	claseKashyaNPC  = 150
	claseAkaraNPC   = 148
	claseGheedNPC   = 147

	// tecla del boton de hablar (el golpe usa KeyY; esta no la usa nadie mas)
	teclaHablarNPC = d2enum.KeyX
)

// npc: un NPC del pueblo, compuesto y en su celda.
type npc struct {
	Nombre   string
	Clase    string // la columna Id de monstats.txt ("warriv1")
	Code     string // la carpeta del MPQ ("WA")
	ClaseExe int
	CeldaX   float64 // en celdas (sub-tile / 5)
	CeldaY   float64

	Ciclo *CicloDelJugador // el COF NU dir 0, con sus cuadros
	Linea string           // lo que dice (ya sin el id ni el salto de la primera linea)
	Lineas []string        // el texto cortado en renglones que entran en el panel
	Filas  []string        // las filas del menu del motor (Talk, Trade, ...)

	// el dibujo del cuadro
	AnclaX, AnclaY int
	Caja           CajaDelJugador
	CajaCruda      CajaDelJugador
	CajaGuardada   []byte
	Pintados       int
	MsDibujo       int
	CuadroDelCiclo int
}

// panelDelDialogo: el panel con su fondo de indices armado una vez (cuando se abre).
type panelDelDialogo struct {
	Ancho, Alto int
	Fondo       []byte
	X0, Y0      int
	NPC         *npc
	Nombre      string
	Lineas      []string
	Filas       []string
	Bytes       int
}

var (
	npcsDelPueblo []*npc
	bytesDeLosNPCs int

	elDialogo        *panelDelDialogo
	dialogoAbierto   bool
	dialogoVeces     int
	cuadrosConDialogo int
	pxDelDialogo     int
	msDelDialogo     int
	crcConDialogo    uint32
	crcUltimoCuadroSinDialogo uint32
	lienzoTrasCerrar uint32
	distanciaAlNPC   float64
	npcQueHablo      string
	botonDelDialogo  bool
	botonAntes       bool
	elDialogoSeCerro bool
	renglonesQueSobran int
	cuadrosCerradosSinCambio int
	cuadrosAbiertosConCambio int
	glifosDelNPC     map[byte]*glifoDelHUD
	lineasDeLaTabla  int
	bytesDeLaTabla   int
)

// ============================== LA CARGA ==============================

// cargarLosNPCs: lee el `.ds1` del pueblo que el POC acaba de componer, saca SUS NPCs (los monstruos que las
// reglas del exe conocen) y compone la animacion de quieto de cada uno. Va con las fuentes abiertas y despues de
// componer el pueblo (paso 4f de main): el `.ds1`, los COF y las `.dcc` salen del `d2data.mpq`.
func cargarLosNPCs() {
	if npcKnob != "si" {
		println("JUEGO127: NPC apagado (npc=", npcKnob, "): la corrida es la de DC-002.125 sin NPCs")

		return
	}

	datos := fuenteDe(rutaDatos)
	if datos == nil {
		println("JUEGO127: NPC sin fuente de datos")

		return
	}

	t0 := int64(d2cMs())

	// los bytes del `.ds1` del pueblo: la ruta la dejo el compositor (`stamp.RegionPath()`), asi que no se
	// adivina cual de los cuatro `.ds1` se dibujo.
	println("JUEGO127: NPC DS1 del pueblo=", rutaDelPueblo)

	tramo, err := pedirElDS1DelPueblo(datos)
	if err != nil || tramo == nil || len(tramo.Datos) == 0 {
		println("JUEGO127: NPC SIN DS1", rutaDelPueblo)

		return
	}

	crudo := make([]byte, len(tramo.Datos))
	copy(crudo, tramo.Datos)

	ds, err := d2ds1.Unmarshal(crudo)
	tramo.Liberar()

	if err != nil {
		println("JUEGO127: NPC el DS1 no parsea:", err.Error())

		return
	}

	monstruos := 0

	for i := range ds.Objects {
		if ds.Objects[i].Type == 1 {
			monstruos++
		}
	}

	println("JUEGO127: NPC DS1 objetos=", len(ds.Objects), "monstruos=", monstruos, "acto=", ds.Act)

	parche := fuenteDe(rutaParche)

	presets := leerMonPresetNPC(datos, parche)
	stats := leerMonStatsNPC(datos, parche)

	var todos []*npc

	for i := range ds.Objects {
		o := &ds.Objects[i]
		if o.Type != 1 {
			continue
		}

		place := ""

		if act, ok := presets[int(ds.Act)]; ok && o.ID >= 0 && o.ID < len(act) {
			place = act[o.ID]
		}

		fila, ok := stats[place]
		if !ok {
			continue
		}

		clase, code := fila[0], fila[1]

		claseExe := claseExeDelNPC(clase)
		if claseExe < 0 {
			continue // rogue1, chicken, cow: no son gente con la que se hable
		}

		if !enLaListaDeNPCs(clase) {
			continue
		}

		n := &npc{Nombre: nombreDelNPC(clase), Clase: clase, Code: code, ClaseExe: claseExe,
			CeldaX: float64(o.X) / float64(subtilesPorCelda), CeldaY: float64(o.Y) / float64(subtilesPorCelda)}

		println("JUEGO127: NPC COLOCADO nombre=", n.Nombre, "clase=", clase, "code=", code, "claseExe=", claseExe,
			"celda=", n.CeldaX, n.CeldaY, "subtiles=", o.X, o.Y)

		todos = append(todos, n)
	}

	if len(todos) == 0 {
		println("JUEGO127: NPC NINGUNO de la lista", npcLista, "esta en el .ds1", rutaDelPueblo)

		return
	}

	// el texto: la tabla de strings, UNA vez, en un bloque del monton de C (390.570 B no entran en el
	// semiespacio del recolector: es lo que midio DC-002.125 con la zona)
	cargarLaTablaDeStrings()

	for _, n := range todos {
		cargarUnNPC(datos, n)
	}

	npcsDelPueblo = todos

	soltarLaTablaDeStrings()

	println("JUEGO127: NPC CARGADOS npcs=", len(npcsDelPueblo), "bytesDeIndices=", bytesDeLosNPCs,
		"tablaDeStrings=", bytesDeLaTabla, "lineasDelMotor=", lineasDeLaTabla, "ms=", int64(d2cMs())-t0)
}

// cargarUnNPC: compone el ciclo de quieto (NU dir 0) y busca lo que dice.
func cargarUnNPC(datos *fuenteconsola.Fuente, n *npc) {
	rutaCOF := `data\global\monsters\` + n.Code + `\COF\` + minusculas(n.Code) + minusculas(modoQuietoNPC) +
		eliminaArmaDelCOF(n.Code) + `.cof`

	tramo, err := datos.Pedir(rutaCOF)
	if err != nil || tramo == nil || len(tramo.Datos) == 0 {
		println("JUEGO127: NPC SIN COF", n.Nombre, rutaCOF)

		return
	}

	bytesCOF, crcCOF := len(tramo.Datos), crc32.ChecksumIEEE(tramo.Datos)

	cof, err := d2cof.Unmarshal(tramo.Datos)
	tramo.Liberar()

	if err != nil {
		println("JUEGO127: NPC COF-PARSE FALLO", n.Nombre, err.Error())

		return
	}

	println("JUEGO127: NPC COF nombre=", n.Nombre, "ruta=", rutaCOF, "bytes=", bytesCOF, "crc32=", int(crcCOF),
		"capas=", cof.NumberOfLayers, "framesPorDir=", cof.FramesPerDirection, "dirs=", cof.NumberOfDirections)

	arena := &arenaDCC{}
	conArena(arena)

	ciclo := componerCicloConRuta(cof, 0, modoQuietoNPC, msPorCuadroNPC, lectorDeLaFuente(datos),
		rutaDeLaCapaDelNPC(n.Code))

	arena.liberar()

	if ciclo == nil || len(ciclo.Cuadros) == 0 {
		println("JUEGO127: NPC CICLO VACIO", n.Nombre)

		return
	}

	// LOS CUADROS AL MONTON DE C: el semiespacio del recolector esta al 91 % con este POC, y un NPC son sus
	// cuadros compuestos (WA 16, CI 13, RC 12). Copiados a bloques del monton de C el recolector no los ve: el
	// mismo camino del HUD (DC-002.125).
	bytes := 0

	for f := range ciclo.Cuadros {
		cu := &ciclo.Cuadros[f]
		bloque := bloqueDeC(len(cu.Pixeles))
		copy(bloque, cu.Pixeles)

		cu.Pixeles = bloque
		bytes += len(bloque)
	}

	runtime.GC()

	n.Ciclo = ciclo
	bytesDeLosNPCs += bytes

	println("JUEGO127: NPC CICLO nombre=", n.Nombre, "cuadros=", len(ciclo.Cuadros), "bytes=", bytes,
		"primerCuadro=", ciclo.Cuadros[0].Ancho, "x", ciclo.Cuadros[0].Alto,
		"pixeles=", ciclo.Cuadros[0].PixelesNoVacios, "crc32=", int(ciclo.Cuadros[0].CRC))

	// LO QUE DICE: la linea del exe (record elegible para la clase del heroe) y el texto de string.tbl
	n.Linea, n.Lineas = laLineaDelNPC(n)
	n.Filas = lasFilasDelNPC(n)

	if len(n.Lineas) > 0 {
		println("JUEGO127: NPC DICE nombre=", n.Nombre, "renglones=", len(n.Lineas), "texto=", n.Lineas[0],
			"filas=", len(n.Filas))
	}
}

// pedirElDS1DelPueblo: el `.ds1` del pueblo en el MPQ. `stamp.RegionPath()` devuelve el nombre del MOTOR
// ("Act1/Town/TownN1.ds1"), no la ruta del archivo: el prefijo es `data/global/tiles/` (asset_manager.go). Se
// prueban las dos formas (con `/` y con `\\`) y se dice cual entro: sin eso, la sonda no puede auditar la ruta.
func pedirElDS1DelPueblo(datos *fuenteconsola.Fuente) (*fuenteconsola.Tramo, error) {
	crudo := rutaDelPueblo

	conBarras := `data\global\tiles\`
	for i := 0; i < len(crudo); i++ {
		if crudo[i] == '/' {
			conBarras += `\`
		} else {
			conBarras += string(crudo[i])
		}
	}

	var ultimo error

	for _, candidata := range []string{"data/global/tiles/" + crudo, conBarras} {
		tramo, err := datos.Pedir(candidata)
		if err == nil && tramo != nil && len(tramo.Datos) > 0 {
			println("JUEGO127: NPC DS1 ENCONTRADO ruta=", candidata, "bytes=", len(tramo.Datos))

			return tramo, nil
		}

		ultimo = err
	}

	return nil, ultimo
}

// eliminaArmaDelCOF: el COF de un NPC se llama `<code>nu<arma>.cof` con la clase de arma del monstruo. Para los
// NPCs del pueblo (WA, CI, RC, PS, GH) la clase es HTH y el archivo es `<code>nuhth.cof` (medido en la sonda).
func eliminaArmaDelCOF(code string) string { return "hth" }

// rutaDeLaCapaDelNPC: el grafico de una capa del NPC. El arma la declara CADA capa del COF (no es la misma en
// todas) y las barras son invertidas: es lo que midio la sonda (`sonda-npc-127`, seccion 3).
func rutaDeLaCapaDelNPC(code string) func(*Personaje, int, string, string) string {
	return func(p *Personaje, i int, modo, ext string) string {
		return `data\global\monsters\` + code + `\` + p.Claves[i] + `\` + code + p.Claves[i] + valorCapaNPC +
			modo + p.Armas[i] + "." + ext
	}
}

// claseExeDelNPC: el id de clase del exe (el que usan las reglas de d2npc), -1 si no es gente.
func claseExeDelNPC(clase string) int {
	switch clase {
	case "warriv1":
		return claseWarriv1NPC
	case "charsi":
		return claseCharsiNPC
	case "kashya":
		return claseKashyaNPC
	case "akara":
		return claseAkaraNPC
	case "gheed":
		return claseGheedNPC
	}

	return -1
}

func nombreDelNPC(clase string) string {
	switch clase {
	case "warriv1":
		return "Warriv"
	case "charsi":
		return "Charsi"
	case "kashya":
		return "Kashya"
	case "akara":
		return "Akara"
	case "gheed":
		return "Gheed"
	}

	return clase
}

// enLaListaDeNPCs: la palanca `npcLista` ("warriv1,charsi,kashya") dice que NPCs se dibujan.
func enLaListaDeNPCs(clase string) bool {
	if npcLista == "" {
		return true
	}

	ini := 0

	for i := 0; i <= len(npcLista); i++ {
		if i == len(npcLista) || npcLista[i] == ',' {
			if npcLista[ini:i] == clase {
				return true
			}

			ini = i + 1
		}
	}

	return false
}

// ============================== EL TEXTO ==============================

// laLineaDelNPC: lo que el NPC dice, por el camino del motor: el registro elegible para la clase del heroe
// (`d2npc.EntryFor` + `Record.Eligible`), su clave de mensaje y el texto de `string.tbl`.
func laLineaDelNPC(n *npc) (string, []string) {
	e, ok := d2npc.EntryFor(n.ClaseExe)
	if !ok {
		return "", nil
	}

	sinQuest := func(quest, bit int) bool { return false }

	clave := ""

	for i := range e.Records {
		r := e.Records[i]
		if !r.Eligible(claseDelHeroeExe(), sinQuest) {
			continue
		}

		clave = d2npc.MessageKey(r.Msg)
		if clave != "" {
			break
		}
	}

	if clave == "" {
		println("JUEGO127: NPC SIN CLAVE nombre=", n.Nombre)

		return "", nil
	}

	texto := textoDeStringTbl(clave)
	if texto == "" {
		println("JUEGO127: NPC SIN TEXTO nombre=", n.Nombre, "clave=", clave)

		return "", nil
	}

	return clave, cortarElTexto(texto)
}

// lasFilasDelNPC: las filas del menu del exe (`d2npc.Options`) con su texto, mas el Cancel implicito.
func lasFilasDelNPC(n *npc) []string {
	ctx := d2npc.Ctx{HeroLevel: 1}
	opciones, ok := d2npc.Options(n.ClaseExe, ctx)

	filas := make([]string, 0, 4)
	if ok {
		for _, o := range opciones {
			if t := textoDeStringTbl(claveDeLaFila(o)); t != "" {
				filas = append(filas, t)
			}
		}
	}

	if len(filas) == 0 {
		filas = append(filas, "talk")
	}

	return append(filas, "cancel")
}

// claveDeLaFila: la clave de string.tbl de una fila del menu (las de `npc_menu_table.go`).
func claveDeLaFila(o d2npc.Option) string {
	switch o.Kind {
	case d2npc.KindTrade:
		return "NPCMenuTrade"
	case d2npc.KindTradeRepair:
		return "NPCMenuTradeRepair"
	case d2npc.KindGamble:
		return "gamble"
	case d2npc.KindHire:
		return "NPCMenuHire"
	case d2npc.KindIdentify:
		return "NPCIdentify1"
	}

	return "TalkMenu"
}

// claseDelHeroeExe: la clase del heroe en la numeracion del exe (0 amazon).
func claseDelHeroeExe() int {
	switch claseDelHeroe {
	case "Amazon":
		return 0
	case "Sorceress":
		return 1
	case "Necromancer":
		return 2
	case "Paladin":
		return 3
	case "Barbarian":
		return 4
	case "Druid":
		return 5
	case "Assassin":
		return 6
	}

	return 0
}

// cortarElTexto: el texto de `string.tbl` viene con el id de la linea adelante y los saltos del original
// ("45\nGreetings, stranger..."). Se descarta la primera linea (el id) y se cortan los renglones por '\n'; cada
// renglon se parte ademas para que entre en el ancho del panel (el motor lo hace con el ancho del texto: aca, con
// el ancho de la pantalla).
func cortarElTexto(texto string) []string {
	var lineas []string

	ini := 0
	primera := true

	for i := 0; i <= len(texto); i++ {
		if i == len(texto) || texto[i] == '\n' {
			linea := recortaEspacios(texto[ini:i])

			if primera && esNumero(linea) {
				primera = false
				ini = i + 1

				continue
			}

			primera = false

			if linea != "" {
				lineas = append(lineas, linea)
			}

			ini = i + 1
		}
	}

	return lineas
}

func esNumero(s string) bool {
	if s == "" {
		return false
	}

	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}

func recortaEspacios(s string) string {
	ini, fin := 0, len(s)

	for ini < fin && (s[ini] == ' ' || s[ini] == '\t' || s[ini] == '\r') {
		ini++
	}

	for fin > ini && (s[fin-1] == ' ' || s[fin-1] == '\t' || s[fin-1] == '\r') {
		fin--
	}

	return s[ini:fin]
}

// ============================== LA TABLA DE STRINGS ==============================

// cargarLaTablaDeStrings: deja la tabla de strings LISTA para `textoDeStringTbl`. NO se copia entera al monton de
// C (390.570 B no entran: medido, `gc_external_alloc: OOM` con la tabla copiada): se lee el tramo PRESTADO de la
// fuente, se buscan las claves que hacen falta (las de los NPCs de la lista) y se copian SOLO esos textos. La
// tabla se suelta en el acto.
func cargarLaTablaDeStrings() {
	if tablaDeStrings != nil {
		return
	}

	datos := fuenteDe(rutaDatos)
	if datos == nil {
		return
	}

	tramo, err := datos.Pedir(rutaTablaStringsNPC)
	if err != nil || tramo == nil || len(tramo.Datos) == 0 {
		println("JUEGO127: NPC SIN string.tbl")

		return
	}

	bytesDeLaTabla = len(tramo.Datos)
	tablaDeStrings = tramo.Datos
	clavesDeLaTabla = int(leerU16(tablaDeStrings, 2))

	println("JUEGO127: NPC TABLA string.tbl bytes=", bytesDeLaTabla,
		"crc32=", int(crc32.ChecksumIEEE(tablaDeStrings)), "claves=", clavesDeLaTabla)
}

var (
	tablaDeStrings  []byte
	clavesDeLaTabla int
)

// textoDeStringTbl: busca una clave en la tabla ya cargada y devuelve su texto ("" si no esta). El formato:
// crc u16, numero de elementos u16, tamano de la tabla u32, version u8, tres u32, la lista de indices (u16 por
// elemento) y despues las entradas de 17 bytes: activa u8, indice u16, hash u32, offset del TEXTO u32, offset de
// la CLAVE u32, largo de la clave u16. Los offsets son absolutos desde el principio del archivo.
func textoDeStringTbl(clave string) string {
	if len(tablaDeStrings) < 21 {
		return ""
	}

	n := int(leerU16(tablaDeStrings, 2))
	entradas := int(leerU32(tablaDeStrings, 4))
	base := 21 + 2*n

	for i := 0; i < entradas; i++ {
		e := base + 17*i
		if e+17 > len(tablaDeStrings) {
			break
		}

		if tablaDeStrings[e] == 0 {
			continue
		}

		largoClave := int(leerU16(tablaDeStrings, e+15))
		if largoClave == 0 {
			continue
		}

		// LA CLAVE es la cadena terminada en cero que empieza en `indexString`; EL TEXTO es el que empieza en
		// `nameString` y mide `nameLength-1` (asi lo lee `d2tbl.loadHashEntry`: el texto NO lleva largo adelante).
		offClave := int(leerU32(tablaDeStrings, e+7))
		if offClave >= len(tablaDeStrings) {
			continue
		}

		fin := offClave
		for fin < len(tablaDeStrings) && tablaDeStrings[fin] != 0 {
			fin++
		}

		if string(tablaDeStrings[offClave:fin]) != clave {
			continue
		}

		offTexto := int(leerU32(tablaDeStrings, e+11))
		largo := int(leerU16(tablaDeStrings, e+15)) - 1
		if largo < 0 || offTexto+largo > len(tablaDeStrings) {
			return ""
		}

		lineasDeLaTabla++

		return string(tablaDeStrings[offTexto : offTexto+largo])
	}

	return ""
}

// soltarLaTablaDeStrings: la tabla prestada se devuelve en el acto: los textos que hacian falta ya son copias de
// Go (n.Lineas y n.Filas salen de `string(...)`, que copia). Lo que queda es el numero para la evidencia.
func soltarLaTablaDeStrings() {
	tablaDeStrings = nil

	runtime.GC()
}

func leerU16(b []byte, o int) uint16 {
	if o+2 > len(b) {
		return 0
	}

	return uint16(b[o]) | uint16(b[o+1])<<8
}

func leerU32(b []byte, o int) uint32 {
	if o+4 > len(b) {
		return 0
	}

	return uint32(b[o]) | uint32(b[o+1])<<8 | uint32(b[o+2])<<16 | uint32(b[o+3])<<24
}

// ============================== LAS TABLAS DE LOS NPCs ==============================

// leerMonPresetNPC: `MonPreset.txt` (Act + Place) -> acto -> lista de Place EN EL ORDEN DEL ARCHIVO (el indice de
// la lista es el ID del objeto del `.ds1`, que es como lo lee el motor).
func leerMonPresetNPC(datos, parche *fuenteconsola.Fuente) map[int][]string {
	actos := map[int][]string{}

	var t *tablaAbierta

	for _, fuente := range []*fuenteconsola.Fuente{parche, datos} {
		if fuente == nil {
			continue
		}

		if t = abrirTabla(fuente, rutaTablaMonPresetNPC, "Act", "Place"); t != nil {
			println("JUEGO127: NPC MonPreset.txt en", fuente.Path(), "bytes=", len(t.Bytes))

			break
		}
	}

	if t == nil {
		return actos
	}

	defer t.cerrar()

	cAct, cPlace := t.col("Act"), t.col("Place")

	t.porCadaFila(func(linea []byte) bool {
		act := enteroDe(linea, cAct)
		actos[act] = append(actos[act], texto(campo(linea, cPlace)))

		return false
	})

	return actos
}

// leerMonStatsNPC: `monstats.txt` (del PARCHE si esta: es el que tiene las clases nuevas) -> Place -> {Id, Code}.
func leerMonStatsNPC(datos, parche *fuenteconsola.Fuente) map[string][2]string {
	filas := map[string][2]string{}

	for _, ruta := range []string{rutaTablaMonStatsNPC} {
		for _, fuente := range []*fuenteconsola.Fuente{parche, datos} {
			if fuente == nil || len(filas) > 0 {
				continue
			}

			t := abrirTabla(fuente, ruta, "Id", "Code", "NameStr")
			if t == nil {
				continue
			}

			cID, cCode := t.col("Id"), t.col("Code")

			t.porCadaFila(func(linea []byte) bool {
				id := texto(campo(linea, cID))
				if id == "" {
					return false
				}

				filas[id] = [2]string{id, texto(campo(linea, cCode))}

				return false
			})

			t.cerrar()
		}
	}

	println("JUEGO127: NPC monstats filas=", len(filas))

	return filas
}

// ============================== EL DIBUJO DE LOS NPCs ==============================

// dibujarLosNPCs: cada NPC en su celda, con el cuadro de su animacion de quieto. Se guarda lo de abajo, se pinta,
// se cuentan los pixeles cambiados y se DEVUELVE el mapa (el crc32 del pueblo solo tiene que seguir siendo el del
// host). Despues `volverAPintarLosNPCs` deja el cuadro que se ve.
func dibujarLosNPCs(malla *pvr.Surface, pixeles []byte) (crcConNPCs uint32, msTotal, pintados int) {
	crcConNPCs = crc32De(pixeles)

	if len(npcsDelPueblo) == 0 {
		return crcConNPCs, 0, 0
	}

	listos := make([]*npc, 0, len(npcsDelPueblo))

	for _, n := range npcsDelPueblo {
		if prepararElCuadroDeUnNPC(n, pixeles) {
			listos = append(listos, n)
		}
	}

	for _, n := range listos {
		cu := n.Ciclo.Cuadros[n.CuadroDelCiclo]

		t1 := d2cMs()
		PintarJugador(malla, &cu, n.AnclaX, n.AnclaY, palJugador[:])
		n.MsDibujo = int(d2cMs() - t1)

		msTotal += n.MsDibujo
		n.Pintados = pixelesCambiadosDelNPC(n, pixeles)
		pintados += n.Pintados
	}

	crcConNPCs = crc32De(pixeles)

	for i := len(listos) - 1; i >= 0; i-- {
		restaurarCajaDelNPC(listos[i], pixeles)
	}

	return crcConNPCs, msTotal, pintados
}

// prepararElCuadroDeUnNPC: el cuadro del momento, el ancla (la MISMA proyeccion del jugador y del bicho) y la
// copia de lo que hay debajo de su caja.
func prepararElCuadroDeUnNPC(n *npc, pixeles []byte) bool {
	if n.Ciclo == nil || len(n.Ciclo.Cuadros) == 0 {
		return false
	}

	// el reloj de la animacion: un cuadro de la consola por `msPorCuadroNPC` del motor
	n.CuadroDelCiclo = indiceDeCuadroPorReloj(n)

	cu := &n.Ciclo.Cuadros[n.CuadroDelCiclo]

	n.AnclaX, n.AnclaY = AnclaDelJugador(rendererDelMapa.WorldToScreenF, n.CeldaX, n.CeldaY)
	n.CajaCruda = CajaDe(n.AnclaX, n.AnclaY, cu)
	n.Caja = recortarALaPantalla(n.CajaCruda)

	if n.Caja.Ancho() <= 0 || n.Caja.Alto() <= 0 {
		n.Pintados = 0

		return false
	}

	guardarCajaDelNPC(n, pixeles)

	return true
}

func indiceDeCuadroPorReloj(n *npc) int {
	cuantos := len(n.Ciclo.Cuadros)
	if cuantos <= 1 {
		return 0
	}

	return int(d2cMs()/uint(msPorCuadroNPC)) % cuantos
}

func guardarCajaDelNPC(n *npc, pixeles []byte) {
	necesario := n.Caja.Ancho() * n.Caja.Alto() * 4

	if n.CajaGuardada == nil || len(n.CajaGuardada) < necesario {
		n.CajaGuardada = make([]byte, necesario)
	}

	k := 0

	for y := n.Caja.Y0; y < n.Caja.Y1; y++ {
		for x := n.Caja.X0; x < n.Caja.X1; x++ {
			j := (y*ancho + x) * 4
			copy(n.CajaGuardada[k:k+4], pixeles[j:j+4])

			k += 4
		}
	}
}

func restaurarCajaDelNPC(n *npc, pixeles []byte) {
	k := 0

	for y := n.Caja.Y0; y < n.Caja.Y1; y++ {
		for x := n.Caja.X0; x < n.Caja.X1; x++ {
			j := (y*ancho + x) * 4
			copy(pixeles[j:j+4], n.CajaGuardada[k:k+4])

			k += 4
		}
	}
}

// pixelesCambiadosDelNPC: cuantos pixeles de la caja cambiaron al pintar el NPC (el instrumento de "esto pinto").
func pixelesCambiadosDelNPC(n *npc, pixeles []byte) int {
	k, cambiados := 0, 0

	for y := n.Caja.Y0; y < n.Caja.Y1; y++ {
		for x := n.Caja.X0; x < n.Caja.X1; x++ {
			j := (y*ancho + x) * 4

			if pixeles[j] != n.CajaGuardada[k] || pixeles[j+1] != n.CajaGuardada[k+1] ||
				pixeles[j+2] != n.CajaGuardada[k+2] {
				cambiados++
			}

			k += 4
		}
	}

	return cambiados
}

// volverAPintarLosNPCs: el cuadro que queda en pantalla.
func volverAPintarLosNPCs(malla *pvr.Surface) {
	for _, n := range npcsDelPueblo {
		if n.Ciclo == nil || len(n.Ciclo.Cuadros) == 0 || n.Caja.Ancho() <= 0 {
			continue
		}

		cu := &n.Ciclo.Cuadros[n.CuadroDelCiclo]
		PintarJugador(malla, cu, n.AnclaX, n.AnclaY, palJugador[:])
	}
}

func detalleDelDibujoDeLosNPCs() string {
	s := ""

	for i, n := range npcsDelPueblo {
		if i > 0 {
			s += ";"
		}

		s += n.Nombre + ":celda=" + ftoa(n.CeldaX) + "," + ftoa(n.CeldaY) + ",pixeles=" +
			itoa(n.Pintados) + ",cuadro=" + itoa(n.CuadroDelCiclo) + ",ancla=" + itoa(n.AnclaX) + "/" +
			itoa(n.AnclaY) + ",caja=" + itoa(n.Caja.X0) + "/" + itoa(n.Caja.Y0) + "/" + itoa(n.Caja.X1) + "/" +
			itoa(n.Caja.Y1)
	}

	return s
}

// ============================== EL BOTON Y EL PANEL ==============================

// pasoDelNPC: el cuadro del habla. Si un jugador esta dentro del radio y aprieta el boton (o lo aprieta el
// instrumento `hablarAuto`, porque el emulador corre sin teclado), el panel se abre con lo que dice ese NPC; si no,
// y si toca cerrar, se cierra.
func pasoDelNPC() {
	if npcKnob != "si" || len(npcsDelPueblo) == 0 {
		return
	}

	// ---- cerrar: dos cuadros despues de abrir (la palanca `cerrarElDialogo`) ----
	if dialogoAbierto {
		if cerrarElDialogo == "si" && cuadro+1 >= cuadroAbreElDialogo+2 {
			dialogoAbierto = false
			elDialogo = nil
			elDialogoSeCerro = true

			println("JUEGO127: DIALOGO CIERRA cuadro=", cuadro+1, "npc=", npcQueHablo,
				"veces=", dialogoVeces)
		}

		return
	}

	if dialogoVeces > 0 && !elDialogoSeCerro {
		return
	}

	// ---- abrir: el boton (o el instrumento) y la distancia ----
	apretado := elBotonDeHablar()

	if !apretado || botonAntes {
		botonAntes = apretado

		return
	}

	botonAntes = apretado

	if cuadro+1 < cuadroAbreElDialogo {
		return
	}

	n := elNPCMasCercano()
	if n == nil {
		return
	}

	ax, ay := 0.0, 0.0

	if len(jugadores) > 0 && jugadores[0].Mov != nil && jugadores[0].Mov.Est != nil {
		ax, ay = jugadores[0].Mov.Est.Celda()
	}

	dx, dy := ax-n.CeldaX, ay-n.CeldaY
	distanciaAlNPC = raizCuadrada(dx*dx + dy*dy)

	if distanciaAlNPC > radioDeHablaNPC {
		println("JUEGO127: NPC NO-HABLA npc=", n.Nombre, "jugador=", ax, ay, "distancia=", distanciaAlNPC,
			"radio=", radioDeHablaNPC, ": el jugador no esta cerca")

		return
	}

	if len(n.Lineas) == 0 {
		println("JUEGO127: NPC NO-HABLA npc=", n.Nombre, "sin texto en la tabla")

		return
	}

	armarElPanelDelDialogo(n)

	dialogoAbierto = true
	dialogoVeces++
	npcQueHablo = n.Nombre

	println("JUEGO127: DIALOGO ABRE npc=", n.Nombre, "claseExe=", n.ClaseExe, "jugador=", ax, ay,
		"distancia=", distanciaAlNPC, "radio=", radioDeHablaNPC, "cuadro=", cuadro+1,
		"panel=", elDialogo.Ancho, "x", elDialogo.Alto, "renglones=", len(elDialogo.Lineas),
		"filas=", len(elDialogo.Filas), "bytes=", elDialogo.Bytes, "boton=", botonDelDialogo)
}

// elBotonDeHablar: el boton X del puerto 1, o el instrumento (`hablarAuto`) que lo aprieta en el cuadro del
// criterio. Se dice cual de los dos fue: un boton del instrumento no es un boton del control.
func elBotonDeHablar() bool {
	if len(jugadores) > 0 && jugadores[0].Entrada != nil && jugadores[0].Entrada.IsKeyPressed(teclaHablarNPC) {
		botonDelDialogo = false

		return true
	}

	if hablarAuto == "si" && cuadro+1 == cuadroAbreElDialogo {
		botonDelDialogo = true

		return true
	}

	return false
}

// elNPCMasCercano: el NPC mas cercano al jugador 1.
func elNPCMasCercano() *npc {
	if len(npcsDelPueblo) == 0 {
		return nil
	}

	ax, ay := 0.0, 0.0

	if len(jugadores) > 0 && jugadores[0].Mov != nil && jugadores[0].Mov.Est != nil {
		ax, ay = jugadores[0].Mov.Est.Celda()
	}

	mejor := npcsDelPueblo[0]
	mejorD := 1e9

	for _, n := range npcsDelPueblo {
		dx, dy := ax-n.CeldaX, ay-n.CeldaY
		d := raizCuadrada(dx*dx + dy*dy)

		if d < mejorD {
			mejorD, mejor = d, n
		}
	}

	return mejor
}

// armarElPanelDelDialogo: el panel con la geometria del motor, aplicada a la pantalla del POC (640x480). El fondo
// es una imagen de indices: el indice oscuro de la paleta, con el borde claro y una linea del medio separando el
// nombre del texto, los MISMOS tres indices que la grilla del inventario (DC-002.124).
func armarElPanelDelDialogo(n *npc) {
	// LOS GLIFOS PRIMERO: el ancho del panel se mide con el avance de cada letra de `font16`, asi que sin los
	// glifos cargados la medida es el promedio de 8 px por letra y el panel sale del ancho equivocado (medido).
	cargarLosGlifosDelNPC(n)

	println("JUEGO127: NPC PANEL-MEDIDA nombre=", n.Nombre, "anchoDelNombre=", anchoDeUnTexto(n.Nombre),
		"anchoDelPrimerRenglon=", anchoDeUnTexto(n.Lineas[0]))

	// el ancho: el del texto mas ancho (el nombre o la linea mas larga del texto y de las filas), como el motor
	anchoTexto := anchoDeUnTexto(n.Nombre)

	for _, l := range n.Lineas {
		if w := anchoDeUnTexto(l); w > anchoTexto {
			anchoTexto = w
		}
	}

	for _, f := range n.Filas {
		if w := anchoDeUnTexto(f); w > anchoTexto {
			anchoTexto = w
		}
	}

	anchoPanel := anchoTexto + npcAnchoPad
	if anchoPanel > ancho-2*npcBorde {
		anchoPanel = ancho - 2*npcBorde
	}

	// el alto: el nombre (0x15), cada renglon del texto y cada fila del menu (0xf), mas el relleno
	altoPanel := npcTituloAvance + (len(n.Lineas)+len(n.Filas))*npcFilaAvance + npcAltoPad
	if altoPanel > alto-npcBordeAbajo-npcBorde {
		altoPanel = alto - npcBordeAbajo - npcBorde
	}

	// centrado arriba: el motor lo cuelga del NPC, aca va en el medio de la pantalla (no hay puntero)
	x0 := (ancho - anchoPanel) / 2
	y0 := npcBorde

	fondo, medio, claro := indicesDelPanel()

	imagen := make([]byte, anchoPanel*altoPanel)

	for i := range imagen {
		imagen[i] = fondo
	}

	for x := 0; x < anchoPanel; x++ {
		imagen[x] = claro
		imagen[(altoPanel-1)*anchoPanel+x] = claro
	}

	for y := 0; y < altoPanel; y++ {
		imagen[y*anchoPanel] = claro
		imagen[y*anchoPanel+anchoPanel-1] = claro
	}

	// la linea que separa el nombre del texto (el avance del titulo del motor)
	for x := 1; x < anchoPanel-1; x++ {
		imagen[npcTituloAvance*anchoPanel+x] = medio
	}

	// EL TEXTO QUE ENTRA: altoPanel deja el nombre (0x15), las filas del menu (0xf cada una) y el relleno; cada
	// renglon del texto mide 0xf. El gossip de Warriv trae 30 renglones y en 480 px de pantalla entran 12: los
	// que sobran NO se pintan (no hay paginas en este POC) y se cuentan en el log.
	entran := (altoPanel - npcTituloAvance - len(n.Filas)*npcFilaAvance - npcAltoPad) / npcFilaAvance
	lineasQueEntran := n.Lineas

	if entran >= 0 && len(lineasQueEntran) > entran {
		renglonesQueSobran = len(lineasQueEntran) - entran
		lineasQueEntran = lineasQueEntran[:entran]
	}

	elDialogo = &panelDelDialogo{Ancho: anchoPanel, Alto: altoPanel, Fondo: imagen, X0: x0, Y0: y0,
		NPC: n, Nombre: n.Nombre, Lineas: lineasQueEntran, Filas: n.Filas, Bytes: len(imagen)}
}

// dibujarElDialogo: el panel y su texto, con el mismo pincel del HUD. Con el panel CERRADO no se pinta un pixel:
// el cuadro del pueblo queda intacto (es la mitad del criterio y se mide cuadro a cuadro).
func dibujarElDialogo(malla *pvr.Surface, pixeles []byte) (pintados, ms int) {
	crcUltimoCuadroSinDialogo = crc32De(pixeles)

	if !dialogoAbierto || elDialogo == nil {
		return 0, 0
	}


	t0 := d2cMs()

	malla.PushTranslation(elDialogo.X0, elDialogo.Y0)
	malla.RenderIndicesTintado(elDialogo.Fondo, elDialogo.Ancho, elDialogo.Alto, palJugador[:])
	malla.Pop()

	oro := indiceDelOro()

	// el nombre, en la primera linea (donde el motor pone el titulo)
	y := elDialogo.Y0 + 4
	dibujarUnTexto(malla, elDialogo.Nombre, elDialogo.X0+8, y, oro)

	// el texto del NPC: cada renglon a su avance de fila
	y = elDialogo.Y0 + npcTituloAvance
	for _, l := range elDialogo.Lineas {
		dibujarUnTexto(malla, l, elDialogo.X0+8, y, 0)
		y += npcFilaAvance
	}

	// las filas del menu (Talk, Trade, ... y Cancel), sangradas
	for _, f := range elDialogo.Filas {
		dibujarUnTexto(malla, f, elDialogo.X0+16, y, 0)
		y += npcFilaAvance
	}

	ms = int(d2cMs() - t0)

	pintados = elDialogo.Ancho * elDialogo.Alto
	pxDelDialogo += pintados
	msDelDialogo = ms
	cuadrosConDialogo++
	crcConDialogo = crc32De(pixeles)

	return pintados, ms
}

// dibujarUnTexto: pinta un texto con los glifos de font16 (el avance es el de la letra, no el ancho de la caja).
func dibujarUnTexto(malla *pvr.Surface, s string, x, y int, indice uint8) {
	avance := 0

	for i := 0; i < len(s); i++ {
		g := glifosDelNPC[s[i]]
		if g == nil {
			continue
		}

		// el glifo es del color que le toque: si el texto va en el indice del oro, se copia con el color del HUD;
		// si no, con la paleta de las unidades tal cual
		malla.PushTranslation(x+avance, y)
		malla.RenderIndicesTintado(g.Indices, g.Ancho, g.Alto, palJugador[:])
		malla.Pop()

		avance += g.Paso
	}
}

// cargarLosGlifosDelNPC: los glifos de `font16` para TODO el texto del panel (nombre, renglones y filas). Se piden
// una sola vez; el panel se arma cuando se abre.
func cargarLosGlifosDelNPC(n *npc) {
	if glifosDelNPC == nil {
		glifosDelNPC = make(map[byte]*glifoDelHUD, 64)
	}

	datos := fuenteDe(rutaDatos)
	if datos == nil {
		return
	}

	tramoTabla, err := datos.Pedir(rutaFuenteNPC + `.tbl`)
	if err != nil || tramoTabla == nil {
		return
	}

	f, err := d2font.Load(tramoTabla.Datos)
	tramoTabla.Liberar()

	if err != nil {
		println("JUEGO127: NPC la tabla de la fuente no parsea:", err.Error())

		return
	}

	tramoSprite, err := datos.Pedir(rutaFuenteNPC + `.DC6`)
	if err != nil || tramoSprite == nil {
		return
	}

	dc, err := d2dc6.Load(tramoSprite.Datos)
	tramoSprite.Liberar()

	if err != nil {
		println("JUEGO127: NPC el sprite de la fuente no parsea:", err.Error())

		return
	}

	textos := append([]string{n.Nombre}, n.Lineas...)
	textos = append(textos, n.Filas...)

	for _, t := range textos {
		for i := 0; i < len(t); i++ {
			c := t[i]
			if _, listo := glifosDelNPC[c]; listo {
				continue
			}

			g, ok := f.Glyphs[rune(c)]
			if !ok {
				continue
			}

			cuadro := g.FrameIndex()
			if cuadro < 0 || cuadro >= len(dc.Frames) {
				continue
			}

			crudo := dc.DecodeFrame(cuadro)
			indices := bloqueDeC(len(crudo))
			copy(indices, crudo)

			glifosDelNPC[c] = &glifoDelHUD{Ancho: int(dc.Frames[cuadro].Width), Alto: int(dc.Frames[cuadro].Height),
				Paso: g.Width(), Indices: indices}
		}
	}

	println("JUEGO127: NPC GLIFOS del panel npc=", n.Nombre, "glifos=", len(glifosDelNPC))
}

// anchoDeUnTexto: el ancho con el AVANCE de cada letra (lo que suma `Font.GetTextMetrics`), no con el ancho de la
// caja del cuadro: es la leccion de DC-002.125.
func anchoDeUnTexto(s string) int {
	anchoTexto := 0

	for i := 0; i < len(s); i++ {
		if g := glifosDelNPC[s[i]]; g != nil {
			anchoTexto += g.Paso
		} else {
			anchoTexto += 8 // una letra sin glifo avanza el promedio de font16
		}
	}

	return anchoTexto
}

// indiceDelOro: el indice de la paleta mas parecido al dorado del titulo del motor (npc_menu.go: 255,215,0).
func indiceDelOro() uint8 {
	mejor, cercania := uint8(1), 1<<30

	for i := 1; i < 256; i++ {
		dr := int(palJugador[i][0]) - 255
		dg := int(palJugador[i][1]) - 215
		db := int(palJugador[i][2]) - 0

		if dr < 0 {
			dr = -dr
		}

		if dg < 0 {
			dg = -dg
		}

		if db < 0 {
			db = -db
		}

		if d := dr + dg + db; d < cercania {
			cercania, mejor = d, uint8(i)
		}
	}

	return mejor
}

// ============================== EL CIERRE ==============================

func resumenDeLosNPCs() {
	if npcKnob != "si" {
		println("JUEGO127: NPC RESUMEN apagado (npc=", npcKnob, ")")

		return
	}

	cuales := ""

	for i, n := range npcsDelPueblo {
		if i > 0 {
			cuales += ","
		}

		cuales += n.Nombre
	}

	println("JUEGO127: NPC RESUMEN npcs=", len(npcsDelPueblo), "cuales=", cuales, "lista=", npcLista,
		"bytesDeIndices=", bytesDeLosNPCs, "tablaDeStrings=", bytesDeLaTabla, "glifos=", len(glifosDelNPC))

	for _, n := range npcsDelPueblo {
		cu := 0
		if n.Ciclo != nil {
			cu = len(n.Ciclo.Cuadros)
		}

		println("JUEGO127: NPC DETALLE nombre=", n.Nombre, "clase=", n.Clase, "code=", n.Code,
			"celda=", n.CeldaX, n.CeldaY, "cuadros=", cu, "dice=", n.Linea, "ultimoCuadro=", n.CuadroDelCiclo,
			"ultimosPixeles=", n.Pintados, "ultimosMs=", n.MsDibujo)
	}

	println("JUEGO127: DIALOGO RESUMEN veces=", dialogoVeces, "cuadrosConPanel=", cuadrosConDialogo,
		"renglonesQueSobran=", renglonesQueSobran,
		"pixeles=", pxDelDialogo, "msUltimo=", msDelDialogo, "crcConPanel=", int(crcConDialogo),
		"crcSinPanel=", int(crcUltimoCuadroSinDialogo), "seCerro=", elDialogoSeCerro,
		"lienzoTrasCerrar=", int(lienzoTrasCerrar), "botonDelInstrumento=", botonDelDialogo)

	// EL CRITERIO. "El pueblo se restaura" NO se mide contra `crcMapa` (que es el mapa solo, sin jugador, sin
	// bicho y sin HUD: no puede dar igual nunca). Se mide asi: en los cuadros CON el panel abierto el lienzo
	// cambia, y en los cuadros con el panel CERRADO no cambia ni un pixel, aunque la llamada al panel se haga
	// igual. Eso es lo que se puede exigir, y es lo que pide la tarjeta.
	elPanelCambiaElLienzo := cuadrosAbiertosConCambio > 0
	conElPanelCerradoNoCambia := cuadrosCerradosSinCambio > 0

	cumple := len(npcsDelPueblo) > 0 && dialogoVeces > 0 && pxDelDialogo > 0 && elDialogoSeCerro &&
		elPanelCambiaElLienzo && conElPanelCerradoNoCambia

	println("JUEGO127: NPC CRITERIO losNPCsSeDibujan=", len(npcsDelPueblo) > 0,
		"seLeHablo=", dialogoVeces > 0, "elDialogoSaleEnPantalla=", pxDelDialogo > 0,
		"elPanelCambiaElLienzo=", elPanelCambiaElLienzo,
		"conElPanelCerradoNoCambia=", conElPanelCerradoNoCambia,
		"elDialogoSeCerro=", elDialogoSeCerro, "cumple=", cumple)
}
