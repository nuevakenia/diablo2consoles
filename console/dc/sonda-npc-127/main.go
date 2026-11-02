// DC-002.127 -- SONDA DE LOS NPCs Y EL DIALOGO, EN EL HOST: que hay que portar, cuanto pesa y con que se dibuja,
// medido ANTES de escribir una linea del POC (la regla del plan, docs/PLAN-JUEGO.md).
//
// POR QUE EXISTE. La tarjeta (E6) pide el pueblo con gente con la que hablar: Warriv, Charsi, Kashya dibujados en
// su celda y su dialogo en pantalla. Para no inventar nada, esta sonda contesta, con los archivos del juego
// delante:
//
//  1. EL PRESUPUESTO: cuanto suman `d2common/d2npc` (las reglas, 2.137 lineas) y el panel de dialogo del motor
//     (`d2game/d2player/npc_menu.go` + `d2core/d2ui`), en lineas, bytes y CIERRE de imports (`go list -deps`, el
//     mismo que usa el build para armar el enlace). Si el cierre se lleva ebiten/d2asset adentro, el panel NO se
//     porta tal cual: se redibuja con el pincel del POC, como el HUD de DC-002.125. Ademas: `d2npc` arma DOS
//     tablas globales con `init` (46 entradas de gossip y 314 claves de mensaje en un `map`), y eso se paga en el
//     arranque de la consola: se informa el numero de entradas y una estimacion del costo.
//  2. LOS NPCs DEL PUEBLO: el `.ds1` del pueblo del acto 1 son CUATRO archivos (E/N/W/S) y los NPCs no estan en
//     la lista de objetos: estan en la de MONSTRUOS (type = 1), con un ID que es el indice de `MonPreset.txt`.
//     La sonda recorre los cuatro y resuelve, para cada monstruo colocado: `MonPreset.txt` (Act + indice -> Place)
//     -> `monstats.txt` (Id == Place -> Code, NameStr). Ese `Code` es la CARPETA del MPQ y el token del COF.
//  3. LOS ARCHIVOS DEL NPC: el COF de "quieto" (`<code>NU<arma>.cof`), sus capas y las `.dcc` de la direccion 0,
//     con bytes y caja. Es lo que hay que componer en la consola: si un `.dcc` no esta, esa capa no se pinta y se
//     dice.
//  4. EL TEXTO: `string.tbl` (390 KB, el archivo que el HUD NO pudo cargar) y las lineas de cada NPC por el
//     camino del motor (`d2npc.EntryFor` -> `Record` -> `MessageKey` -> tabla). Se imprimen las primeras lineas de
//     cada uno: sin el texto no hay "dialogo en pantalla" que medir.
//  5. EL PANEL: la geometria del motor tal cual esta escrita en `npc_menu.go` (ancho = el texto mas ancho + 20,
//     alto = suma de avances + 15, titulo 0x15 por fila 0xf, centrado, recortado a 320x240) aplicada al texto
//     REAL de cada NPC con la fuente `font16` del juego. Eso da la caja exacta que el POC tiene que pintar.
//
// Uso:
//
//	go run ./console/dc/sonda-npc-127
//	go run ./console/dc/sonda-npc-127 -dir "~/.wine-d2/drive_c/Games/Diablo II" -npc 155
package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2font"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2tbl"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2npc"
)

const (
	tablaMonPreset = `data\global\excel\MonPreset.txt`
	tablaMonStats  = `data\global\excel\monstats.txt`
	tablaStrings   = `data\local\lng\ENG\string.tbl`
	tablaPatchStr  = `data\local\lng\ENG\patchstring.tbl`
	tablaExpStr    = `data\local\lng\ENG\expansionstring.tbl`
	rutaFuente     = `data\local\font\latin\font16`

	// el pueblo del acto 1 son cuatro `.ds1` (el mismo dato que usa el POC para componerlo)
	rutaDS1PuebloE = `data\global\tiles\ACT1\TOWN\townE1.ds1`
	rutaDS1PuebloN = `data\global\tiles\ACT1\TOWN\townN1.ds1`
	rutaDS1PuebloO = `data\global\tiles\ACT1\TOWN\townW1.ds1`
	rutaDS1PuebloS = `data\global\tiles\ACT1\TOWN\townS1.ds1`

	// base de los monstruos/NPCs en el MPQ (composite.go, monsterBasePath)
	baseMonstruos = "data/global/monsters"

	// el valor de capa por omision del motor cuando no hay equipo (composite.go): "LIT"
	valorCapa = "LIT"

	// el arma por omision de un NPC: no lleva arma, la clase del compositor es HTH
	armaNPC = "HTH"
)

// paquetes: lo que la tarjeta tiene que portar. d2npc son las reglas; el panel de dialogo del motor vive en
// d2game/d2player/npc_menu.go y usa d2core/d2ui; d2game/d2gamescreen/npc_town.go es el pegamento con la partida.
var paquetes = []string{
	"d2common/d2npc",
	"d2game/d2player",
	"d2game/d2gamescreen",
	"d2core/d2ui",
	"d2core/d2asset",
}

func main() {
	dir := flag.String("dir", "", "carpeta del juego (por omision, la instalacion de wine)")
	npc := flag.Int("npc", 0, "si no es 0, imprime el detalle de ese id de monstats (155 Warriv, 154 Charsi, 150 Kashya)")
	flag.Parse()

	base := *dir
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".wine-d2", "drive_c", "Games", "Diablo II")
	}

	fmt.Println("== 0. LOS MPQ")
	mpqs := map[string]*d2mpq.MPQ{}

	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq", "d2char.mpq"} {
		m, err := d2mpq.FromFile(filepath.Join(base, nombre))
		if err != nil {
			fmt.Printf("   %-14s NO ABRE: %v\n", nombre, err)

			continue
		}

		mpqs[nombre] = m
		fmt.Printf("   %-14s %12d B\n", nombre, m.Size())
	}

	if mpqs["d2data.mpq"] == nil {
		fmt.Println("   sin d2data.mpq no hay nada que medir")

		return
	}

	elPresupuesto()
	npsDelPueblo := losNPCsDelPueblo(mpqs)
	losArchivosDeCadaNPC(mpqs, npsDelPueblo)
	lasLineas(mpqs, npsDelPueblo)
	elPanel(mpqs, npsDelPueblo)

	if *npc != 0 {
		detalleDeUnNPC(mpqs, *npc)
	}
}

// ============================== 1. EL PRESUPUESTO ==============================

func elPresupuesto() {
	fmt.Println("\n== 1. EL PRESUPUESTO: que cuesta portar d2npc y el panel de dialogo (go list -deps)")

	for _, p := range paquetes {
		archivos, lineas, bytes := medirPaquete(p)
		deps := cierreDeImports(p)
		ebiten, assets, render := enElCierre(deps)

		fmt.Printf("   %-28s %2d archivos %6d lineas %8d B | cierre %3d paquetes ebiten=%-5v d2asset=%-5v d2render=%v\n",
			p, archivos, lineas, bytes, len(deps), ebiten, assets, render)
	}

	fmt.Println("   -- el cierre de d2npc (tiene que ser CORTO: son reglas sin assets)")

	for _, d := range filtrarDeps(cierreDeImports("d2common/d2npc")) {
		fmt.Println("      " + d)
	}

	fmt.Println("   -- lo que d2npc arma con `init` (se paga en el ARRANQUE de la consola)")
	fmt.Printf("      entradas de gossip: %d   claves de mensaje (map): %d\n",
		d2npc.EntryCount(), clavesDeMensaje())

	// el costo del map: una entrada de map[int]string son 16 B de bucket + la cadena (16 B de cabecera y los
	// bytes). El map en si pide un arreglo de buckets y otro de overflow al `make`.
	claves := clavesDeMensaje()
	fmt.Printf("      estimado del map (claves %d): buckets %.0f B + cadenas %d B = %.1f KB en el monton de Go\n",
		claves, float64(claves)*16*1.3, claves*16, (float64(claves)*16*1.3+float64(claves)*16)/1024)

	fmt.Println("   -- lineas del panel de dialogo del motor (d2game/d2player/npc_menu.go forma parte del paquete)")
	fmt.Println("      la geometria que se porta esta en el bloque de constantes de ese archivo:")
	fmt.Println("      ancho = texto mas ancho + 20, alto = titulo 0x15 + filas 0xf + 15, centrado, recorte 320x240")
}

func medirPaquete(p string) (archivos, lineas, bytes int) {
	dir := p
	if _, err := os.Stat(dir); err != nil {
		dir = filepath.Join("..", "..", p)
	}

	entradas, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, 0
	}

	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}

		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}

		archivos++
		bytes += len(b)
		lineas += strings.Count(string(b), "\n")
	}

	return archivos, lineas, bytes
}

func cierreDeImports(p string) []string {
	salida, err := exec.Command("go", "list", "-deps", "./"+p).Output()
	if err != nil {
		return nil
	}

	var deps []string

	for _, l := range strings.Split(string(salida), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			deps = append(deps, l)
		}
	}

	return deps
}

func enElCierre(deps []string) (ebiten, assets, render bool) {
	for _, d := range deps {
		if strings.Contains(d, "hajimehoshi/ebiten") {
			ebiten = true
		}

		if strings.HasSuffix(d, "d2core/d2asset") {
			assets = true
		}

		if strings.Contains(d, "d2core/d2render") {
			render = true
		}
	}

	return ebiten, assets, render
}

func filtrarDeps(deps []string) []string {
	var out []string

	for _, d := range deps {
		if strings.Contains(d, "OpenDiablo2") || strings.Contains(d, "hajimehoshi") {
			out = append(out, d)
		}
	}

	sort.Strings(out)

	return out
}

// clavesDeMensaje: cuantas claves tiene el map de mensajes de d2npc (se cuentan las que devuelven texto).
func clavesDeMensaje() int {
	n := 0

	// los ids van 0..~3200 con huecos: se prueban los que la tabla de gossip puede usar y se cuentan los vivos.
	for id := 0; id < 4000; id++ {
		if d2npc.MessageKey(id) != "" {
			n++
		}
	}

	return n
}

// ============================== 2. LOS NPCs DEL PUEBLO ==============================

// npcDelPueblo: un monstruo colocado en el .ds1 del pueblo, ya resuelto a su fila de monstats.
type npcDelPueblo struct {
	Archivo   string
	Indice    int
	PresetID  int
	Place     string
	Clase     string // la columna Id de monstats.txt
	Code      string // la carpeta del MPQ / el token del COF
	NameStr   string
	CeldaX    int
	CeldaY    int
	Flag      int
	ClaseExe  int
	TieneMenu bool
}

func losNPCsDelPueblo(mpqs map[string]*d2mpq.MPQ) []npcDelPueblo {
	fmt.Println("\n== 2. LOS NPCs DEL PUEBLO: los monstruos (type = 1) de los cuatro .ds1 y su fila de monstats")

	presets := leerMonPreset(mpqs)
	fmt.Printf("   MonPreset.txt: actos %d, indices del acto 1: %d\n", len(presets), len(presets[1]))

	stats := leerMonStats(mpqs)
	fmt.Printf("   monstats.txt: %d filas (Id -> Code, NameStr)\n", len(stats))

	var todos []*npcDelPueblo

	for _, cual := range []string{rutaDS1PuebloE, rutaDS1PuebloN, rutaDS1PuebloO, rutaDS1PuebloS} {
		datos := leerDeLosMPQ(mpqs, cual)
		if datos == nil {
			fmt.Println("   NO encuentro", cual)

			continue
		}

		ds, err := d2ds1.Unmarshal(datos)
		if err != nil {
			fmt.Printf("   %s NO PARSEA: %v\n", cual, err)

			continue
		}

		monstruos := 0

		w, h := ds.Width(), ds.Height()

		fmt.Printf("   %s en %s: %d B crc32=%d version=%d act=%d %dx%d objetos=%d\n",
			filepath.Base(cual), dondeEsta(mpqs, cual), len(datos), crc32.ChecksumIEEE(datos),
			ds.Version(), ds.Act, w, h, len(ds.Objects))

		for i := range ds.Objects {
			o := &ds.Objects[i]
			if o.Type != 1 { // d2enum.ObjectTypeCharacter
				continue
			}

			monstruos++

			n := &npcDelPueblo{Archivo: filepath.Base(cual), Indice: i, PresetID: o.ID,
				CeldaX: o.X, CeldaY: o.Y, Flag: o.Flags}

			if act, ok := presets[int(ds.Act)]; ok && o.ID >= 0 && o.ID < len(act) {
				n.Place = act[o.ID]
			}

			if fila, ok := stats[n.Place]; ok {
				n.Clase, n.Code, n.NameStr = fila[0], fila[1], fila[2]
			}

			todos = append(todos, n)
		}

		fmt.Printf("      monstruos colocados (type = 1): %d\n", monstruos)
	}

	fmt.Println("   -- los monstruos del pueblo, resueltos")
	fmt.Printf("   %-10s %3s %6s %-22s %-22s %-6s %8s %5s\n",
		"archivo", "obj", "preset", "Place", "Id (monstats)", "Code", "celda", "flag")

	for _, n := range todos {
		fmt.Printf("   %-10s %3d %6d %-22s %-22s %-6s %3d,%-3d %5d\n",
			n.Archivo, n.Indice, n.PresetID, recorte(n.Place, 22), recorte(n.Clase, 22), n.Code,
			n.CeldaX, n.CeldaY, n.Flag)
	}

	// cuales de esos tienen menu en las reglas del exe (d2npc): esos son los "NPCs del pueblo" de verdad
	for _, n := range todos {
		if n.Clase == "" {
			continue
		}

		if _, ok := d2npc.EntryFor(claseExe(n.Clase)); ok {
			n.TieneMenu = true
		}
	}

	fmt.Println("   -- los que las reglas del exe conocen (d2npc.EntryFor)")

	for _, n := range todos {
		if !n.TieneMenu {
			continue
		}

		fmt.Printf("      %-10s %-12s claseExe=%d lineas=%d\n", n.Archivo, n.Clase, claseExe(n.Clase), lineasDe(n.Clase))
	}

	return valorDe(todos)
}

func valorDe(l []*npcDelPueblo) []npcDelPueblo {
	out := make([]npcDelPueblo, 0, len(l))
	for _, n := range l {
		out = append(out, *n)
	}

	return out
}

// claseExe: el id de clase del exe para los NPCs (el `Id` de monstats es en minuscula: warriv1, charsi, kashya,
// akara, gheed). Es el numero que usan las reglas de d2npc (ClassKashya 150, ClassWarriv1 155, ...).
func claseExe(clase string) int {
	ids := map[string]int{
		"akara": 148, "kashya": 150, "charsi": 154, "warriv1": 155, "warriv": 155,
		"gheed": 147, "cain1": 146, "deckardcain": 146, "cain": 146, "meshif1": 210,
	}

	if id, ok := ids[clase]; ok {
		return id
	}

	return -1
}

func lineasDe(clase string) int {
	e, ok := d2npc.EntryFor(claseExe(clase))
	if !ok {
		return 0
	}

	return len(e.Records)
}

// ============================== 3. LOS ARCHIVOS DE CADA NPC ==============================

func losArchivosDeCadaNPC(mpqs map[string]*d2mpq.MPQ, npcs []npcDelPueblo) {
	fmt.Println("\n== 3. LOS ARCHIVOS DEL NPC: su COF de quieto (NU) y las .dcc de la direccion 0")

	vistos := map[string]bool{}

	for _, n := range npcs {
		if n.Code == "" || vistos[n.Code] {
			continue
		}

		vistos[n.Code] = true

		cof := RutaCOF(n.Code, "NU", armaNPC)
		datos := leerDeLosMPQ(mpqs, cof)
		if datos == nil {
			fmt.Printf("   %-12s SIN COF %s\n", n.Code, cof)

			continue
		}

		c, err := d2cof.Unmarshal(datos)
		if err != nil {
			fmt.Printf("   %-12s COF NO PARSEA (%s): %v\n", n.Code, cof, err)

			continue
		}

		fmt.Printf("   %-12s COF %s %d B crc32=%d capas=%d framesPorDir=%d dirs=%d\n",
			n.Code, cof, len(datos), crc32.ChecksumIEEE(datos), c.NumberOfLayers,
			c.FramesPerDirection, c.NumberOfDirections)

		total := 0

		for i := range c.CofLayers {
			capa := c.CofLayers[i]
			// OJO: el arma NO es la misma en todas las capas: el COF le pone a cada una la suya
			// (`CofLayer.WeaponClass`). Con un arma fija en todas las capas el archivo no aparece (medido).
			ruta := RutaDeLaCapa(n.Code, capa.Type.String(), capa.WeaponClass.String(), "NU")
			d := leerDeLosMPQ(mpqs, ruta)

			if d == nil {
				fmt.Printf("      capa %-4s SIN GRAFICO %s\n", capa.Type.String(), ruta)

				continue
			}

			total += len(d)
			fmt.Printf("      capa %-4s arma %-4s %-56s %8d B\n", capa.Type.String(),
				capa.WeaponClass.String(), ruta, len(d))
		}

		fmt.Printf("      TOTAL de la direccion 0 de NU: %d B (lo que se compone y se queda vivo)\n", total)
	}
}

// RutaCOF: el COF de un modo, como lo arma el motor (composite.go, createMode).
func RutaCOF(code, modo, arma string) string {
	return `data\global\monsters\` + code + `\COF\` + minusculas(code) + minusculas(modo) + minusculas(arma) + `.cof`
}

// RutaDeLaCapa: el grafico de una capa (composite.go: <base>/<code>/<capa>/<code><capa>LIT<modo><arma>.dcc).
// LAS BARRAS SON INVERTIDAS: el MPQ del juego indexa las rutas con `\`, y con `/` el archivo NO aparece (medido).
func RutaDeLaCapa(code, capa, arma, modo string) string {
	return `data\global\monsters\` + code + `\` + capa + `\` + code + capa + valorCapa + modo + arma + ".dcc"
}

func minusculas(s string) string {
	b := []byte(s)

	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}

	return string(b)
}

// ============================== 4. LAS LINEAS ==============================

func lasLineas(mpqs map[string]*d2mpq.MPQ, npcs []npcDelPueblo) {
	fmt.Println("\n== 4. EL TEXTO: string.tbl y las lineas de cada NPC por el camino de d2npc")

	tablas := map[string]d2tbl.TextDictionary{}

	for _, cual := range []string{tablaStrings, tablaPatchStr, tablaExpStr} {
		datos := leerDeLosMPQ(mpqs, cual)
		if datos == nil {
			fmt.Println("   NO encuentro", cual)

			continue
		}

		td, err := d2tbl.LoadTextDictionary(datos)
		if err != nil {
			fmt.Printf("   %s NO PARSEA: %v\n", cual, err)

			continue
		}

		tablas[cual] = td
		fmt.Printf("   %-40s %8d B %6d claves en %s\n", filepath.Base(cual), len(datos), len(td),
			dondeEsta(mpqs, cual))
	}

	buscar := func(clave string) (string, string) {
		for _, cual := range []string{tablaPatchStr, tablaExpStr, tablaStrings} {
			if td, ok := tablas[cual]; ok {
				if v, hay := td[clave]; hay && v != "" {
					return v, filepath.Base(cual)
				}
			}
		}

		return "", ""
	}

	vistos := map[int]bool{}

	for _, n := range npcs {
		if !n.TieneMenu || vistos[claseExe(n.Clase)] {
			continue
		}

		id := claseExe(n.Clase)
		vistos[id] = true

		e, _ := d2npc.EntryFor(id)

		fmt.Printf("   -- %s (claseExe %d, Code %s, NameStr %q): %d lineas, noIntro=%v\n",
			n.Clase, id, n.Code, n.NameStr, len(e.Records), e.NoIntro)

		// el nombre del NPC por la tabla (NameStr es una clave de string.tbl)
		if v, de := buscar(n.NameStr); v != "" {
			fmt.Printf("      nombre: %-22s %-30q (%s) %d px en font16\n", n.NameStr, v, de, anchoDeTexto(mpqs, v))
		}

		for i, r := range e.Records {
			if i >= 4 {
				break
			}

			clave := d2npc.MessageKey(r.Msg)

			v, de := buscar(clave)
			if v == "" {
				// las tablas de este disco usan "#<indice>": se prueba asi tambien
				v, de = buscar("#" + strconv.Itoa(r.Msg))
			}

			fmt.Printf("      [%d] msg=%-5d clave=%-34q clase=%d -> %-9q (%s) %d px\n",
				i, r.Msg, clave, r.Class, recorte(v, 60), de, anchoDeTexto(mpqs, v))
		}
	}

	// las filas del menu (las CLAVES de string.tbl que usa d2game/d2player/npc_menu_table.go)
	fmt.Println("   -- las filas del menu (claves de string.tbl de npc_menu_table.go)")

	for _, f := range []struct{ Nombre, Clave string }{
		{"Talk", "TalkMenu"}, {"Trade", "NPCMenuTrade"}, {"Trade/Repair", "NPCMenuTradeRepair"},
		{"Gamble", "gamble"}, {"Hire", "NPCMenuHire"}, {"Cancel", "Back"},
	} {
		v, de := buscar(f.Clave)
		fmt.Printf("      %-14s clave=%-20q -> %-24q (%s) %d px\n", f.Nombre, f.Clave, v, de, anchoDeTexto(mpqs, v))
	}
}

// ============================== 5. EL PANEL ==============================

func elPanel(mpqs map[string]*d2mpq.MPQ, npcs []npcDelPueblo) {
	fmt.Println("\n== 5. EL PANEL: la geometria de npc_menu.go aplicada al texto real, en la fuente del juego")

	f, err := fuente16(mpqs)
	if err != nil {
		fmt.Println("   NO puedo medir la fuente 16:", err)

		return
	}

	// las constantes del motor (npc_menu.go)
	const (
		tituloAvance = 0x15
		filaAvance   = 0xf
		anchoPad     = 20
		altoPad      = 15
		borde        = 10
		bordeAbajo   = 0x3a
	)

	fmt.Printf("   constantes del motor: titulo %d px, fila %d px, pad ancho %d alto %d, borde %d, abajo %d\n",
		tituloAvance, filaAvance, anchoPad, altoPad, borde, bordeAbajo)

	for _, n := range npcs {
		if !n.TieneMenu {
			continue
		}

		e, _ := d2npc.EntryFor(claseExe(n.Clase))

		// el peor caso del panel: el titulo (el nombre del NPC) y las filas de su menu
		ancho := anchoDeTexto(mpqs, nombreDeLaTabla(mpqs, n.NameStr))
		filas := 2 // Talk + Cancel como minimo

		if e != nil && len(e.Records) > 0 {
			v := textoDeLaTabla(mpqs, d2npc.MessageKey(e.Records[0].Msg), e.Records[0].Msg)
			if w := anchoDeTexto(mpqs, v); w > ancho {
				ancho = w
			}
		}

		alto := tituloAvance + filas*filaAvance + altoPad
		anchoFinal := acotar(ancho+anchoPad, borde, 320-borde)
		altoFinal := acotar(alto, borde, 240-bordeAbajo)

		fmt.Printf("   %-12s nombre=%-18q anchoTexto=%3d -> panel %3d x %3d (filas %d)\n",
			n.Clase, recorte(nombreDeLaTabla(mpqs, n.NameStr), 18), ancho, anchoFinal, altoFinal, filas)
	}

	// la fuente: lo que cuesta tenerla viva
	for _, cual := range []string{rutaFuente + ".tbl", rutaFuente + ".DC6"} {
		if datos := leerDeLosMPQ(mpqs, cual); datos != nil {
			fmt.Printf("   fuente %-24s %7d B en %s\n", filepath.Base(cual), len(datos), dondeEsta(mpqs, cual))
		}
	}

	_ = f
}

func nombreDeLaTabla(mpqs map[string]*d2mpq.MPQ, clave string) string {
	return textoDeLaTabla(mpqs, clave, 0)
}

func textoDeLaTabla(mpqs map[string]*d2mpq.MPQ, clave string, id int) string {
	for _, cual := range []string{tablaPatchStr, tablaExpStr, tablaStrings} {
		datos := leerDeLosMPQ(mpqs, cual)
		if datos == nil {
			continue
		}

		td, err := d2tbl.LoadTextDictionary(datos)
		if err != nil {
			continue
		}

		if clave != "" {
			if v, ok := td[clave]; ok && v != "" {
				return v
			}
		}

		if id != 0 {
			if v, ok := td["#"+strconv.Itoa(id)]; ok && v != "" {
				return v
			}
		}
	}

	return ""
}

// anchoDeTexto: el ancho de un texto en la fuente 16, con el AVANCE de la letra (`FontGlyph.Width`), que es lo que
// suma `Font.GetTextMetrics` y NO el ancho de la caja del cuadro (la leccion de DC-002.125).
func anchoDeTexto(mpqs map[string]*d2mpq.MPQ, s string) int {
	f, err := fuente16(mpqs)
	if err != nil || s == "" {
		return 0
	}

	w, _ := f.GetTextMetrics(s)

	return w
}

func fuente16(mpqs map[string]*d2mpq.MPQ) (*d2font.Font, error) {
	datos := leerDeLosMPQ(mpqs, rutaFuente+".tbl")
	if datos == nil {
		return nil, fmt.Errorf("no encuentro %s.tbl", rutaFuente)
	}

	return d2font.Load(datos)
}

// ============================== EL DETALLE DE UN NPC ==============================

func detalleDeUnNPC(mpqs map[string]*d2mpq.MPQ, id int) {
	fmt.Printf("\n== 6. EL DETALLE DE UN NPC: id de monstats %d\n", id)

	stats := leerMonStats(mpqs)

	for clase, fila := range stats {
		if fila[3] != strconv.Itoa(id) {
			continue
		}

		fmt.Printf("   Id=%-22q Code=%-6q NameStr=%-22q claseExe=%d\n", clase, fila[1], fila[2], claseExe(clase))

		cof := RutaCOF(fila[1], "NU", armaNPC)
		if datos := leerDeLosMPQ(mpqs, cof); datos != nil {
			fmt.Printf("   COF %s %d B\n", cof, len(datos))
		} else {
			fmt.Printf("   SIN COF %s\n", cof)
		}
	}
}

// ============================== LAS TABLAS ==============================

// leerMonPreset: MonPreset.txt, columnas Act y Place. Devuelve acto -> lista de Place en el ORDEN del archivo
// (el indice de la lista es el ID del objeto del .ds1, que es como lo lee el motor).
func leerMonPreset(mpqs map[string]*d2mpq.MPQ) map[int][]string {
	actos := map[int][]string{}

	datos := leerDeLosMPQ(mpqs, tablaMonPreset)
	if datos == nil {
		fmt.Println("   NO encuentro", tablaMonPreset)

		return actos
	}

	lineas := strings.Split(string(datos), "\n")
	if len(lineas) < 2 {
		return actos
	}

	cAct, cPlace := indiceDeColumna(lineas[0], "act"), indiceDeColumna(lineas[0], "place")
	if cAct < 0 || cPlace < 0 {
		fmt.Println("   MonPreset.txt no tiene las columnas Act/Place")

		return actos
	}

	for _, l := range lineas[1:] {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) == "" {
			continue
		}

		act, err := strconv.Atoi(strings.TrimSpace(campo(l, cAct)))
		if err != nil {
			continue
		}

		actos[act] = append(actos[act], strings.TrimSpace(campo(l, cPlace)))
	}

	return actos
}

// leerMonStats: monstats.txt -> Id -> {Id, Code, NameStr, "id" de la fila??}. El cuarto campo es el indice de la
// fila en el archivo, que es lo que el exe usa como clase; se informa por si hace falta.
func leerMonStats(mpqs map[string]*d2mpq.MPQ) map[string][4]string {
	filas := map[string][4]string{}

	datos := leerDeLosMPQ(mpqs, tablaMonStats)
	if datos == nil {
		fmt.Println("   NO encuentro", tablaMonStats)

		return filas
	}

	lineas := strings.Split(string(datos), "\n")
	if len(lineas) < 2 {
		return filas
	}

	cID := indiceDeColumna(lineas[0], "id")
	cCode := indiceDeColumna(lineas[0], "code")
	cName := indiceDeColumna(lineas[0], "namestr")
	if cID < 0 || cCode < 0 || cName < 0 {
		fmt.Println("   monstats.txt no tiene las columnas Id/Code/NameStr")

		return filas
	}

	n := 0

	for _, l := range lineas[1:] {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) == "" {
			continue
		}

		id := strings.TrimSpace(campo(l, cID))
		if id == "" {
			continue
		}

		filas[id] = [4]string{id, strings.TrimSpace(campo(l, cCode)), strings.TrimSpace(campo(l, cName)),
			strconv.Itoa(n)}
		n++
	}

	return filas
}

func indiceDeColumna(cabecera, nombre string) int {
	campos := strings.Split(strings.TrimRight(cabecera, "\r"), "\t")

	for i, c := range campos {
		if strings.EqualFold(strings.TrimSpace(c), nombre) {
			return i
		}
	}

	return -1
}

func campo(linea string, i int) string {
	campos := strings.Split(linea, "\t")
	if i < 0 || i >= len(campos) {
		return ""
	}

	return campos[i]
}

// ============================== LAS HERRAMIENTAS ==============================

func leerDeLosMPQ(mpqs map[string]*d2mpq.MPQ, ruta string) []byte {
	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq", "d2char.mpq"} {
		m := mpqs[nombre]
		if m == nil {
			continue
		}

		datos, err := m.ReadFile(ruta)
		if err == nil && len(datos) > 0 {
			return datos
		}
	}

	return nil
}

func dondeEsta(mpqs map[string]*d2mpq.MPQ, ruta string) string {
	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq", "d2char.mpq"} {
		m := mpqs[nombre]
		if m == nil {
			continue
		}

		if datos, err := m.ReadFile(ruta); err == nil && len(datos) > 0 {
			return nombre
		}
	}

	return "?"
}

func recorte(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[:n]
}

func acotar(v, lo, hi int) int {
	if v < lo {
		return lo
	}

	if v > hi {
		return hi
	}

	return v
}
