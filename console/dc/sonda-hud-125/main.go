// DC-002.125 -- SONDA DEL HUD, EN EL HOST: el presupuesto de `d2ui` y `d2automap` ANTES de portar nada, los
// graficos que el HUD de verdad usa (y en que MPQ estan), y los VALORES REALES del jugador con las tablas del
// motor.
//
// POR QUE EXISTE. La tarjeta pide cuatro cosas y las cuatro se contestan con numeros, no con opiniones:
//   1. EL PRESUPUESTO: cuanto suman `d2core/d2ui` y `d2common/d2automap` (lineas, bytes, cierre de imports y si
//      el cierre se lleva ebiten/d2asset adentro). Eso decide si el HUD se porta tal cual o se dibuja con el
//      pincel del POC. El cierre se mide con `go list -deps`, el mismo que usa el build para armar el enlace.
//   2. LOS GRAFICOS: las barras de vida y mana del motor NO son barras: son DOS globos de 80x80
//      (`hlthmana.DC6`) tapados por `overlap.DC6` y recortados por un `maskRect` (globeWidget.Render). El
//      cinturon es un CUADRO del panel (`ctrlpnl7.DC6`, frame 3). El nombre de la zona es `font16` (DC6 + .tbl)
//      con la paleta de unidades. El minimapa son los cuadros de `maximaps.DC6` sobre el modelo de `d2automap`.
//      La sonda dice, para cada uno: en que MPQ esta, cuanto mide, cuantos cuadros tiene y la caja de cada uno.
//      OJO: en esta instalacion (1.14b) el panel NO es `800ctrlpnl7.dc6` -- el nombre que tiene escrito el
//      motor (`d2resource.GamePanels`) -- sino `ctrlpnl7.DC6`. Eso se mide, no se supone.
//   3. LOS VALORES REALES: `charstats.txt` -> `d2statlist.ParseClasses` -> `Class.BaseMax(nivel, vit, ene)`, que
//      es el MISMO camino que usa el motor (`d2hero/hero_totals.go`). Imprime los siete heroes al nivel 1.
//   4. EL CINTURON: `belts.txt` (la columna `numboxes`) y la tabla del motor `d2inventory.BeltBoxes`, que es de
//      donde sale el 4 del cinturon sin cinturon (la fila `default`).
//
// Y de paso contesta si el MINIMAPA entra: el modelo (`d2automap.Model`) que se arma con el `.ds1` del pueblo,
// cuantas celdas son, cuanto ocupa y cuanto mide la tabla (`automap.bin`, la binaria que usa el motor).
//
// Uso:
//
//	go run ./console/dc/sonda-hud-125
//	go run ./console/dc/sonda-hud-125 -dir "~/.wine-d2/drive_c/Games/Diablo II" -clase Amazon -nivel 1
package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2automap"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2font"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2tbl"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2statlist"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2inventory"
)

const (
	tablaCharStats = `data\global\excel\charstats.txt`
	tablaBelts     = `data\global\excel\belts.txt`
	tablaLevels    = `data\global\excel\levels.txt`
	tablaMisc      = `data\global\excel\misc.txt`
	tablaStrings   = `data\local\lng\ENG\string.tbl`
	ds1Pueblo      = `data\global\tiles\ACT1\TOWN\townN1.ds1`
	binAutomap     = `data\global\excel\automap.bin`
	txtAutomap     = `data\global\excel\AutoMap.txt`
)

// graficos: los que el HUD usa, con el nombre que les da el MOTOR y el que TIENE el MPQ de esta instalacion.
var graficos = []struct{ que, ruta string }{
	{"panel (frame 0 = panel, 3 = cinturon)", `data\global\ui\PANEL\ctrlpnl7.DC6`},
	{"panel, el nombre que pide el motor", `data\global\ui\PANEL\800ctrlpnl7.dc6`},
	{"globos de vida y mana (frame 0 vida, 1 mana)", `data\global\ui\PANEL\hlthmana.DC6`},
	{"tapa de los globos (frame 0, 1)", `data\global\ui\PANEL\overlap.DC6`},
	{"cinturon del inventario", `data\global\ui\PANEL\inv_belt.DC6`},
	{"cinturon desplegable del panel", `data\global\ui\PANEL\ctrlpnl_popbelt.DC6`},
	{"pocion de vida (invfile de hpot)", `data\global\items\invhp1.dc6`},
	{"pocion de mana (invfile de mpot)", `data\global\items\invmp1.dc6`},
	{"minimapa (cuadros de 8x16)", `data\global\ui\automap\maximaps.dc6`},
	{"minimapa grande (cuadros de 16x32)", `data\global\ui\automap\maximap.dc6`},
	{"fuente 16 (nombre de la zona)", `data\local\font\latin\font16.DC6`},
	{"fuente 30 (el nombre grande de la zona)", `data\local\font\latin\font30.DC6`},
	{"fuente 8 (nombres del minimapa)", `data\local\font\latin\font8.DC6`},
	{"paleta de unidades", `data\global\palette\units\pal.dat`},
	{"paleta estatica", `data\global\palette\static\pal.dat`},
	{"paleta del cielo (la de los globos)", `data\global\palette\sky\pal.dat`},
	{"paleta del acto 1 (la del minimapa)", `data\global\palette\act1\pal.dat`},
}

// paquetes: lo que se mide del motor antes de portarlo.
var paquetes = []string{
	"d2core/d2ui",
	"d2common/d2automap",
	"d2common/d2statlist",
	"d2common/d2display",
	"d2common/d2fileformats/d2font",
	"d2common/d2inventory",
	"d2game/d2player",
	"d2core/d2map/d2mapengine",
}

func main() {
	dir := flag.String("dir", "", "carpeta del juego (por omision, la instalacion de wine)")
	clase := flag.String("clase", "Amazon", "clase del heroe del POC")
	nivel := flag.Int("nivel", 1, "nivel del heroe")
	png := flag.String("png", "", "si se dice una carpeta, vuelca los graficos del HUD a PNG con la paleta del juego")
	flag.Parse()

	base := *dir
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".wine-d2", "drive_c", "Games", "Diablo II")
	}

	fmt.Println("== 0. LOS MPQ")
	fmt.Printf("   carpeta: %s\n", base)

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

	costeDelMotor()
	losGraficos(mpqs)
	losValores(mpqs, *clase, *nivel)
	elCinturon(mpqs)
	elMinimapa(mpqs)

	if *png != "" {
		volcarPNG(mpqs, *png, *clase)
	}
}

// ============================== 1. EL PRESUPUESTO ==============================

// costeDelMotor: para cada paquete, lo que cuesta portarlo: sus lineas y bytes, y su CIERRE de imports (cuantos
// paquetes se lleva y si el cierre trae ebiten o el gestor de assets, que es lo que hoy no compila para SH-4).
func costeDelMotor() {
	fmt.Println("\n== 1. EL PRESUPUESTO: que cuesta portar d2ui y d2automap (medido con go list -deps)")

	for _, p := range paquetes {
		archivos, lineas, bytes := medirPaquete(p)

		deps := cierreDeImports(p)
		ebiten, assets, d2render := enElCierre(deps)

		fmt.Printf("   %-42s %2d archivos %6d lineas %8d B | cierre %3d paquetes ebiten=%-5v "+
			"d2asset=%-5v d2render=%v\n",
			p, archivos, lineas, bytes, len(deps), ebiten, assets, d2render)
	}

	fmt.Println("   -- el cierre de d2ui, uno por uno (los que NO son libreria estandar)")
	fmt.Println("      " + strings.Join(filtrarDeps(cierreDeImports("d2core/d2ui")), "\n      "))
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

// ============================== 2. LOS GRAFICOS ==============================

func losGraficos(mpqs map[string]*d2mpq.MPQ) {
	fmt.Println("\n== 2. LOS GRAFICOS DEL HUD: donde estan y cuanto miden")
	fmt.Println("   (de un DC6 se imprimen los cuadros con SU caja: el HUD los pega por cuadro, no por hoja)")

	for _, g := range graficos {
		medirGrafico(mpqs, g.que, g.ruta)
	}

	fmt.Println("   -- las fuentes: la tabla .tbl dice que cuadro es cada letra")
	for _, f := range []string{`data\local\font\latin\font16`, `data\local\font\latin\font30`, `data\local\font\latin\font8`} {
		medirFuente(mpqs, f)
	}
}

func medirGrafico(mpqs map[string]*d2mpq.MPQ, que, ruta string) {
	mpq, datos := buscarEnLosMPQ(mpqs, ruta)
	if datos == nil {
		fmt.Printf("   %-46s NO ESTA en ningun MPQ\n", que)

		return
	}

	if !strings.HasSuffix(strings.ToLower(ruta), ".dc6") {
		fmt.Printf("   %-46s %-13s %8d B crc32=%d\n", que, mpq, len(datos), crc32.ChecksumIEEE(datos))

		return
	}

	dc, err := d2dc6.Load(datos)
	if err != nil {
		fmt.Printf("   %-46s %-13s %8d B NO PARSEA: %v\n", que, mpq, len(datos), err)

		return
	}

	suma := 0

	for i := range dc.Frames {
		suma += len(dc.DecodeFrame(i))
	}

	fmt.Printf("   %-46s %-13s %8d B crc32=%d cuadros=%d indices=%d\n", que, mpq, len(datos),
		crc32.ChecksumIEEE(datos), len(dc.Frames), suma)

	// los primeros cuadros con su caja: es lo que decide que cuadro es el panel, cual el cinturon y cuales los
	// dos globos
	tope := len(dc.Frames)
	if tope > 6 {
		tope = 6
	}

	for i := 0; i < tope; i++ {
		fr := dc.Frames[i]
		fmt.Printf("        cuadro %d: %dx%d en %d,%d pixelesNoVacios=%d\n", i, fr.Width, fr.Height,
			fr.OffsetX, fr.OffsetY, noVacios(dc.DecodeFrame(i)))
	}
}

func medirFuente(mpqs map[string]*d2mpq.MPQ, base string) {
	_, tabla := buscarEnLosMPQ(mpqs, base+`.tbl`)
	_, sprite := buscarEnLosMPQ(mpqs, base+`.DC6`)

	if tabla == nil || sprite == nil {
		fmt.Printf("   %-46s NO ESTA (tabla=%v sprite=%v)\n", base, tabla != nil, sprite != nil)

		return
	}

	f, err := d2font.Load(tabla)
	if err != nil {
		fmt.Printf("   %-46s %8d B NO PARSEA: %v\n", base+".tbl", len(tabla), err)

		return
	}

	ancho, _ := f.GetTextMetrics("Rogue Encampment")

	dc, err := d2dc6.Load(sprite)
	if err != nil {
		fmt.Printf("   %-46s %8d B el sprite NO PARSEA: %v\n", base+".DC6", len(sprite), err)

		return
	}

	fmt.Printf("   %-46s tabla=%d B sprite=%d B cuadros=%d glifos=%d \"Rogue Encampment\"=%d px\n",
		base, len(tabla), len(sprite), len(dc.Frames), len(f.Glyphs), ancho)
}

// ============================== 3. LOS VALORES REALES ==============================

func losValores(mpqs map[string]*d2mpq.MPQ, clase string, nivel int) {
	fmt.Println("\n== 3. LOS VALORES REALES: charstats.txt -> d2statlist -> BaseMax")

	mpq, datos := buscarEnLosMPQ(mpqs, tablaCharStats)
	if datos == nil {
		fmt.Println("   no encuentro charstats.txt")

		return
	}

	clases, err := d2statlist.ParseClasses(datos)
	if err != nil {
		fmt.Println("   charstats.txt no parsea:", err)

		return
	}

	fmt.Printf("   charstats.txt en %s: %d B crc32=%d filas=%d\n", mpq, len(datos),
		crc32.ChecksumIEEE(datos), len(clases))

	nombres := make([]string, 0, len(clases))
	for n := range clases {
		nombres = append(nombres, n)
	}

	sort.Strings(nombres)

	for _, n := range nombres {
		c := clases[n]
		vida, mana, aguante := c.BaseMax(nivel, c.InitVit, c.InitEne)

		fmt.Printf("   %-12s nivel=%d vit=%d ene=%d hpadd=%d | vida=%d mana=%d aguante=%d\n",
			n, nivel, c.InitVit, c.InitEne, c.HpAdd, vida, mana, aguante)
	}

	c, ok := clases[clase]
	if !ok {
		fmt.Printf("   la clase %q no esta en la tabla\n", clase)

		return
	}

	vida, mana, aguante := c.BaseMax(nivel, c.InitVit, c.InitEne)

	fmt.Printf("   EL HEROE DEL POC: %s nivel=%d vida=%d mana=%d aguante=%d (PerVit=%d PerEne=%d PerLevel=%d/%d)\n",
		clase, nivel, vida, mana, aguante, c.LifePerVit, c.ManaPerEne, c.LifePerLevel, c.ManaPerLevel)
	fmt.Printf("   (la vida y el mana con los que nace: HpAdd+InitVit = %d, InitEne = %d)\n",
		c.HpAdd+c.InitVit, c.InitEne)

	// y de donde salen las pociones: misc.txt
	if _, misc := buscarEnLosMPQs(mpqs, tablaMisc); misc != nil {
		fmt.Printf("   misc.txt bytes=%d  (dato: la fila manda el grafico del inventario)\n", len(misc))

		for _, codigo := range []string{"hpot", "mpot"} {
			inv, tipo := columnasDeLaPocion(misc, codigo)
			fmt.Printf("   misc.txt %-5s invfile=%-10q tipo=%q\n", codigo, inv, tipo)
		}

		// las columnas que hablan del grafico y las filas de pocion que de verdad trae esta instalacion
		fmt.Printf("   misc.txt columnas-del-grafico=%v\n", columnasDelGrafico(misc))
		fmt.Printf("   misc.txt filas-de-pocion=%v\n", filasDePocion(misc))
	}
}

// columnasDeLaPocion: el invfile y el tipo de una fila de misc.txt (TSV con cabecera).
func columnasDeLaPocion(datos []byte, codigo string) (invfile, tipo string) {
	filas := strings.Split(string(datos), "\n")
	if len(filas) < 2 {
		return "", ""
	}

	col := map[string]int{}

	for i, c := range strings.Split(strings.TrimRight(filas[0], "\r"), "\t") {
		col[strings.ToLower(strings.TrimSpace(c))] = i
	}

	iCod, okCod := col["code"]
	iInv, okInv := col["invfile"]
	iTipo, okTipo := col["type"]

	if !okCod || !okInv || !okTipo {
		return "", ""
	}

	for _, l := range filas[1:] {
		l = strings.TrimRight(l, "\r")
		if l == "" {
			continue
		}

		campos := strings.Split(l, "\t")
		if iCod >= len(campos) || !strings.EqualFold(strings.TrimSpace(campos[iCod]), codigo) {
			continue
		}

		if iInv < len(campos) {
			invfile = strings.TrimSpace(campos[iInv])
		}

		if iTipo < len(campos) {
			tipo = strings.TrimSpace(campos[iTipo])
		}

		return invfile, tipo
	}

	return "", ""
}

// columnasDelGrafico: los nombres de las columnas de misc.txt que mandan un grafico (para no adivinar cual es).
func columnasDelGrafico(datos []byte) []string {
	filas := strings.Split(string(datos), "\n")
	if len(filas) < 2 {
		return nil
	}

	var out []string

	for _, c := range strings.Split(strings.TrimRight(filas[0], "\r"), "\t") {
		lc := strings.ToLower(strings.TrimSpace(c))
		if strings.Contains(lc, "inv") || strings.Contains(lc, "file") {
			out = append(out, lc)
		}
	}

	return out
}

// filasDePocion: los codigos de pocion con su invfile, tal como vienen en esta instalacion.
func filasDePocion(datos []byte) []string {
	filas := strings.Split(string(datos), "\n")
	if len(filas) < 2 {
		return nil
	}

	col := map[string]int{}

	for i, c := range strings.Split(strings.TrimRight(filas[0], "\r"), "\t") {
		col[strings.ToLower(strings.TrimSpace(c))] = i
	}

	iCod, okCod := col["code"]
	iInv, okInv := col["invfile"]

	if !okCod {
		return nil
	}

	var out []string

	for _, l := range filas[1:] {
		l = strings.TrimRight(l, "\r")
		campos := strings.Split(l, "\t")
		if iCod >= len(campos) {
			continue
		}

		cod := strings.TrimSpace(campos[iCod])
		if !strings.HasPrefix(cod, "hp") && !strings.HasPrefix(cod, "mp") {
			continue
		}

		inv := ""
		if okInv && iInv < len(campos) {
			inv = strings.TrimSpace(campos[iInv])
		}

		if inv == "" {
			out = append(out, cod+"->(vacio)")
		} else {
			out = append(out, cod+"->"+inv)
		}
	}

	return out
}

// ============================== 4. EL CINTURON ==============================

func elCinturon(mpqs map[string]*d2mpq.MPQ) {
	fmt.Println("\n== 4. EL CINTURON: belts.txt (numboxes) y la tabla del motor")

	mpq, datos := buscarEnLosMPQs(mpqs, tablaBelts)
	if datos == nil {
		fmt.Println("   no encuentro belts.txt")

		return
	}

	fmt.Printf("   belts.txt en %s: %d B crc32=%d\n", mpq, len(datos), crc32.ChecksumIEEE(datos))

	filas := strings.Split(string(datos), "\n")
	if len(filas) < 2 {
		return
	}

	col := map[string]int{}

	for i, c := range strings.Split(strings.TrimRight(filas[0], "\r"), "\t") {
		col[strings.ToLower(strings.TrimSpace(c))] = i
	}

	iNombre, okNombre := col["name"]
	iCajas, okCajas := col["numboxes"]
	if !okCajas {
		fmt.Println("   belts.txt no tiene la columna numboxes")

		return
	}

	for _, l := range filas[1:] {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) == "" {
			continue
		}

		campos := strings.Split(l, "\t")
		nombre := ""
		if okNombre && iNombre < len(campos) {
			nombre = strings.TrimSpace(campos[iNombre])
		}

		cajas := ""
		if iCajas < len(campos) {
			cajas = strings.TrimSpace(campos[iCajas])
		}

		fmt.Printf("   fila %-14q numboxes=%s\n", nombre, cajas)
	}

	fmt.Printf("   d2inventory: BeltDefaultBoxes=%d BeltBoxesByType=%v (el motor, no la tabla)\n",
		d2inventory.BeltDefaultBoxes, d2inventory.BeltBoxesByType)
	fmt.Printf("   el cinturon de 4 pociones = %d cajas = %d fila(s) de %d columnas\n",
		d2inventory.BeltDefaultBoxes, d2inventory.BeltRows(d2inventory.BeltDefaultBoxes), d2inventory.BeltColumns)
}

// ============================== 5. EL MINIMAPA ==============================

func elMinimapa(mpqs map[string]*d2mpq.MPQ) {
	fmt.Println("\n== 5. EL MINIMAPA: la tabla, el modelo y lo que cuesta")

	var celda d2automap.Cell

	fmt.Printf("   celda del modelo: %d B (x,y,cuadro,capa)\n", unsafe.Sizeof(celda))

	mpqBin, bin := buscarEnLosMPQs(mpqs, binAutomap)
	mpqTxt, txt := buscarEnLosMPQs(mpqs, txtAutomap)

	fmt.Printf("   automap.bin: %s %d B | AutoMap.txt: %s %d B\n", mpqBin, len(bin), mpqTxt, len(txt))

	var tabla *d2automap.Table

	if bin != nil {
		if t, err := d2automap.ParseBin(bin); err == nil {
			tabla = t
			fmt.Printf("   automap.bin PARSEA: %d filas\n", len(t.Rows()))
		} else {
			fmt.Println("   automap.bin NO parsea:", err)
		}
	}

	if tabla == nil && txt != nil {
		if t, err := d2automap.ParseTxt(strings.NewReader(string(txt))); err == nil {
			tabla = t
			fmt.Printf("   AutoMap.txt PARSEA: %d filas\n", len(t.Rows()))
		}
	}

	// el modelo: se arma con el .ds1 del pueblo y el mismo adaptador del motor (automap.go, tileSource)
	_, datosDS1 := buscarEnLosMPQs(mpqs, ds1Pueblo)

	ds, err := d2ds1.Unmarshal(datosDS1)
	if err != nil {
		fmt.Println("   el .ds1 del pueblo no parsea:", err)

		return
	}

	w, h := ds.Size()
	alto := 0
	pisos, paredes := 0, 0

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for i := 0; i < 8; i++ {
				if l := ds.GetFloor(i); l != nil {
					if t := l.Tile(x, y); t != nil && !t.Hidden() && t.Prop1 != 0 {
						pisos++
					}
				}

				if l := ds.GetWall(i); l != nil {
					if t := l.Tile(x, y); t != nil && !t.Hidden() && t.Prop1 != 0 {
						paredes++
						alto++
					}
				}
			}
		}
	}

	fmt.Printf("   DS1 del pueblo: %dx%d tiles; capas con grafico pisos=%d paredes=%d\n", w, h, pisos, paredes)

	if tabla == nil {
		fmt.Println("   sin tabla no se puede decir cuantas celdas saca el modelo (y el minimapa NO entra)")

		return
	}

	m := d2automap.NewModel()

	area := d2automap.ScreenArea(320, 240) // la mitad del split: el lienzo mas chico que puede tener el HUD
	nivel := 1
	semilla := uint32(nivel+1) * 2654435761

	agregadas := m.RevealTiles(tabla, fuenteDeTiles{ds}, nivel, semilla, 28, 20, area)

	fmt.Printf("   REVEAL alrededor del jugador (28,20) en un lienzo de 320x240: celdas=%d (piso=%d pared=%d objeto=%d)\n",
		agregadas, m.CountLayer(d2automap.LayerFloor), m.CountLayer(d2automap.LayerWall), m.CountLayer(d2automap.LayerObject))

	// y el techo: el pueblo ENTERO
	mTodo := d2automap.NewModel()
	areaTodo := d2automap.Area{HalfW: w * 8, HalfH: h * 4}

	todo := mTodo.RevealTiles(tabla, fuenteDeTiles{ds}, nivel, semilla, float64(w/2), float64(h/2), areaTodo)

	fmt.Printf("   REVEAL del pueblo entero: celdas=%d (piso=%d pared=%d) = %d B de celdas ordenadas\n",
		todo, mTodo.CountLayer(d2automap.LayerFloor), mTodo.CountLayer(d2automap.LayerWall),
		len(mTodo.Cells())*int(unsafe.Sizeof(celda)))

	cx, cy := d2automap.WorldCell(28, 20)
	lay := d2automap.ComputeLayoutOffsets(d2automap.SizeMini, 320, 240, cx, cy, d2automap.PanelNone, false, 0, 0)

	dibujadas := 0

	for _, c := range mTodo.Cells() {
		x, y := lay.ToScreen(c.X, c.Y)
		if x+8 <= lay.Clip.X0 || x >= lay.Clip.X1 || y <= lay.Clip.Y0 || y-16 >= lay.Clip.Y1 {
			continue
		}

		dibujadas++
	}

	fmt.Printf("   la caja del minimapa en 320x240: clip=(%d,%d)-(%d,%d); celdas que caen adentro=%d de %d\n",
		lay.Clip.X0, lay.Clip.Y0, lay.Clip.X1, lay.Clip.Y1, dibujadas, mTodo.Count())
}

// fuenteDeTiles: el MISMO adaptador que el motor (d2game/d2player/automap.go, tileSource).
type fuenteDeTiles struct{ ds *d2ds1.DS1 }

func (f fuenteDeTiles) Size() (w, h int) { return f.ds.Size() }

func (f fuenteDeTiles) Tiles(tx, ty int) []d2automap.TileRef {
	var refs []d2automap.TileRef

	for i := 0; i < 8; i++ {
		if l := f.ds.GetFloor(i); l != nil {
			if t := l.Tile(tx, ty); t != nil && !t.Hidden() && t.Prop1 != 0 {
				refs = append(refs, d2automap.TileRef{Layer: d2automap.LayerFloor, Type: 0,
					Style: int(t.Style), Sequence: int(t.Sequence)})
			}
		}

		if l := f.ds.GetWall(i); l != nil {
			if t := l.Tile(tx, ty); t != nil && !t.Hidden() && t.Prop1 != 0 {
				refs = append(refs, d2automap.TileRef{Layer: d2automap.LayerWall, Type: int(t.Type),
					Style: int(t.Style), Sequence: int(t.Sequence)})
			}
		}
	}

	return refs
}

// ============================== ayudas ==============================

func buscarEnLosMPQs(mpqs map[string]*d2mpq.MPQ, ruta string) (string, []byte) {
	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq", "d2char.mpq"} {
		m := mpqs[nombre]
		if m == nil {
			continue
		}

		if datos, err := m.ReadFile(ruta); err == nil && len(datos) > 0 {
			return nombre, datos
		}
	}

	return "", nil
}

func buscarEnLosMPQ(mpqs map[string]*d2mpq.MPQ, ruta string) (string, []byte) {
	return buscarEnLosMPQs(mpqs, ruta)
}

func noVacios(pix []byte) int {
	c := 0

	for _, v := range pix {
		if v != 0 {
			c++
		}
	}

	return c
}

// la tabla de strings se usa para el nombre de la zona: se deja medido aunque el POC lo lea de levels.txt.
var _ = d2tbl.LoadIDStrings
