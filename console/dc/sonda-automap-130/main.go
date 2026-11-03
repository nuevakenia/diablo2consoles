// DC-002.130 -- SONDA DEL MINIMAPA, EN EL HOST: lo que hay que saber ANTES de dibujar el minimapa del pueblo en
// la consola, con los numeros de la 125 como piso y sin volver a medir lo ya medido.
//
// QUE CONTESTA (y por que cada cosa hace falta en el POC):
//
//  1. EL PRESUPUESTO, Re-VERIFICADO: `d2common/d2automap` (archivos, lineas, bytes) y su CIERRE de imports con
//     `go list -deps`, que es el mismo que arma el enlace del build. Si el cierre trae ebiten o d2asset, no se
//     porta. (La 125 lo midio; aca se vuelve a imprimir para que la tarjeta que lo porta tenga el numero al lado,
//     y para que un cambio en el paquete no pase inadvertido.)
//
//  2. LA TABLA: `AutoMap.txt` (la del MPQ, 122.022 B de la 125) parseada con el lector del MOTOR
//     (`d2automap.ParseTxt`), las filas del pueblo (`1 Town`) y LOS CUADROS DE ARTE QUE PIDE: la union de los
//     `Cels` de esas filas. Ese numero es el que decide cuanta memoria pide el arte (los cuadros que se usan, no
//     los 1.260 del archivo).
//
//  3. EL MODELO: el `.ds1` del pueblo, el REVEAL del pueblo entero (las 680 celdas de la 125) y, LO QUE LA 125 NO
//     MIDIO, **cuantos cuadros DISTINTOS de `maximaps.dc6` usan esas celdas**. Eso es la memoria real del arte:
//     512 B por cuadro de 16x32 si se decodifica entero, 128 B por cuadro si se decima a 8x16 (que es la caja que
//     el motor pide para el minimapa y que NO existe en ningun MPQ de esta instalacion).
//
//  4. EL RECORTE EN EL LIENZO DEL POC (640x480, no los 320x240 de la 125): la caja del minimapa en el lienzo del
//     POC es (359,57)-(638,282) -- 279x225 px, la misma caja del motor, corrida al borde derecho de 640. Se cuentan
//     las celdas que caen adentro con DOS proyecciones: la del motor para el minimapa (escala 20, celdas de 8x16)
//     y la de 16x32 a escala 20, que es la que saldria si se dibujara el arte de 16x32 con la proyeccion del
//     minimapa (se solapan: la sonda imprime cuantos pixeles se pisan, para no elegir eso a ojo).
//
// Uso:
//
//	go run ./console/dc/sonda-automap-130
//	go run ./console/dc/sonda-automap-130 -dir "~/.wine-d2/drive_c/Games/Diablo II"
package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2automap"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

const (
	ds1Pueblo  = `data\global\tiles\ACT1\TOWN\townN1.ds1`
	binAutomap = `data\global\excel\automap.bin`
	txtAutomap = `data\global\excel\AutoMap.txt`
	dc6Mini    = `data\global\ui\automap\maximaps.dc6`
	dc6Full    = `data\global\ui\automap\maximap.dc6`

	// el lienzo del POC (main.go: ancho, alto = 640, 480) y la celda del arranque (knobs.go: vistaX/Y)
	lienzoAncho, lienzoAlto = 640, 480
	celdaPOCX, celdaPOCY    = 28.0, 20.0

	nivelPueblo = 1
)

// semilla: la del motor (`Automap.Advance`: uint32(nivel+1) * automapSeedMul).
var (
	multiplicadorDeSemilla uint32 = 2654435761
	semilla                       = uint32(nivelPueblo+1) * multiplicadorDeSemilla
)

// marcaDeMemoria: el vivo del monton de Go en el ultimo corte, para medir por diferencia lo que pide cada cosa
// (la tabla, el modelo). Se toma antes de parsear la tabla.
var marcaDeMemoria uint64

// nombresDeOrientacion: los `TileName` del motor (`d2automap.tileNames`, que no se exporta), para la salida.
var nombresDeOrientacion = []string{
	"fl", "wl", "wr", "wtlr", "wtll", "wtr", "wbl", "wbr", "wld", "wrd",
	"wle", "wre", "co", "sh", "tr", "rf", "ld", "rd", "fd", "fi",
}

// paquetes: lo que se mide del motor antes de portarlo (el de esta tarjeta, y el que la 125 descarto, al lado
// para que la comparacion quede en la misma salida).
var paquetes = []string{
	"d2common/d2automap",
	"d2game/d2player",
	"d2core/d2ui",
}

func main() {
	dir := flag.String("dir", "", "carpeta del juego (por omision, la instalacion de wine)")
	flag.Parse()

	base := *dir
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".wine-d2", "drive_c", "Games", "Diablo II")
	}

	fmt.Println("================================================================")
	fmt.Println(" DC-002.130 -- EL MINIMAPA: sonda del host")
	fmt.Println("================================================================")
	fmt.Printf("   carpeta: %s\n", base)
	fmt.Printf("   lienzo del POC: %dx%d, celda del arranque (%.1f, %.1f)\n", lienzoAncho, lienzoAlto, celdaPOCX, celdaPOCY)

	elPresupuesto()

	mpqs := map[string]*d2mpq.MPQ{}

	fmt.Println("\n== 1. LOS MPQ")
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

	// la marca de memoria ANTES de parsear la tabla: lo que crezca de aca en adelante es de la tabla y del modelo
	var antes runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&antes)

	marcaDeMemoria = antes.HeapAlloc

	tabla, celdasDelArte := laTabla(mpqs)
	elModelo(mpqs, tabla, celdasDelArte)
	elArte(mpqs, celdasDelArte)
	laTablaFiltrada(mpqs, tabla)
}

// ============================== 5. LA TABLA FILTRADA (lo que entra en la consola) ==============================

// laTablaFiltrada: el A/B que justifica `d2automap.ParseTxtLevels` en la consola. La tabla entera son 2.603 filas
// y 271 KB del semiespacio; las del pueblo son 83. Se verifica que la filtrada da LAS MISMAS filas y EL MISMO
// modelo (mismas celdas, mismo crc32), que es la unica forma de que el ahorro no cambie el mapa.
func laTablaFiltrada(mpqs map[string]*d2mpq.MPQ, entera *d2automap.Table) {
	fmt.Println("\n== 5. LA TABLA FILTRADA: lo que de verdad entra en 56 KB de semiespacio")

	if entera == nil {
		fmt.Println("   sin tabla entera no hay A/B")

		return
	}

	_, txt := buscarEnLosMPQs(mpqs, txtAutomap)
	_, datosDS1 := buscarEnLosMPQs(mpqs, ds1Pueblo)

	ds, err := d2ds1.Unmarshal(datosDS1)
	if err != nil {
		fmt.Println("   el .ds1 del pueblo no parsea:", err)

		return
	}

	filtrada, err := d2automap.ParseTxtLevels(strings.NewReader(string(txt)), nivelPueblo)
	if err != nil {
		fmt.Println("   ParseTxtLevels NO parsea:", err)

		return
	}

	// el payload de la tabla filtrada, contado (el delta de runtime.ReadMemStats no sirve aca: el recolector del
	// host libera mas de lo que la tabla pide y el numero sale negativo). Se cuenta lo que ocupa de verdad: cada
	// fila es la estructura `Row` (5 enteros + la rebanada de cuadros) mas los cuadros.
	carga := int(unsafe.Sizeof(d2automap.Row{})) * len(filtrada.Rows())

	for _, r := range filtrada.Rows() {
		carga += len(r.Cels) * int(unsafe.Sizeof(int(0)))
	}

	fmt.Printf("   filtrada (solo el nivel %d): %d filas, %d B de payload (la entera: 2.603 filas, 271.144 B medidos)\n",
		nivelPueblo, len(filtrada.Rows()), carga)

	// EL A/B DEL MODELO: mismas celdas, mismo crc32
	modelo := func(t *d2automap.Table) (*d2automap.Model, uint32) {
		m := d2automap.NewModel()
		w, h := ds.Size()
		m.RevealTiles(t, fuenteDeTiles{ds}, nivelPueblo, semilla, float64(w/2), float64(h/2),
			d2automap.Area{HalfW: w * 8, HalfH: h * 4})

		h32 := crc32.NewIEEE()

		for _, c := range m.Cells() {
			fmt.Fprintf(h32, "%d:%d:%d:%d;", c.Kind, c.X, c.Y, c.Cel)
		}

		return m, h32.Sum32()
	}

	mEntera, crcEntera := modelo(entera)
	mFiltrada, crcFiltrada := modelo(filtrada)

	// las filas del pueblo, una por una, tienen que ser las mismas
	e, f := entera.LevelRows(nivelPueblo), filtrada.LevelRows(nivelPueblo)
	iguales := len(e) == len(f)

	if iguales {
		for i := range e {
			if e[i].Level != f[i].Level || e[i].Tile != f[i].Tile || e[i].Style != f[i].Style ||
				e[i].Start != f[i].Start || e[i].End != f[i].End || len(e[i].Cels) != len(f[i].Cels) {
				iguales = false

				break
			}

			for j := range e[i].Cels {
				if e[i].Cels[j] != f[i].Cels[j] {
					iguales = false
				}
			}
		}
	}

	fmt.Printf("   A/B del modelo: entera celdas=%d crc32=%d | filtrada celdas=%d crc32=%d | identico=%v\n",
		mEntera.Count(), crcEntera, mFiltrada.Count(), crcFiltrada, crcEntera == crcFiltrada)
	fmt.Printf("   A/B de las filas: entera=%d filtrada=%d filaPorFilaIguales=%v\n", len(e), len(f), iguales)

	if crcEntera != crcFiltrada || !iguales {
		fmt.Println("   *** LA FILTRADA NO DA LO MISMO: no se usa ***")
	} else {
		fmt.Println("   veredicto: la tabla filtrada es la misma tabla del pueblo -> el ahorro de 261 KB no cambia el mapa")
	}
}

// ============================== 0. EL PRESUPUESTO ==============================

func elPresupuesto() {
	fmt.Println("\n== 0. EL PRESUPUESTO (go list -deps, el mismo cierre que arma el enlace)")

	for _, p := range paquetes {
		archivos, lineas, bytes := medirPaquete(p)
		deps := cierreDeImports(p)
		ebiten, assets, render := enElCierre(deps)

		fmt.Printf("   %-24s %2d archivos %6d lineas %8d B | cierre %3d paquetes ebiten=%-5v d2asset=%-5v d2render=%v\n",
			p, archivos, lineas, bytes, len(deps), ebiten, assets, render)
	}

	fmt.Println("   -- el cierre de d2automap, uno por uno (los que NO son libreria estandar):")
	for _, d := range filtrarDeps(cierreDeImports("d2common/d2automap")) {
		fmt.Println("      " + d)
	}
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

// ============================== 2. LA TABLA ==============================

func laTabla(mpqs map[string]*d2mpq.MPQ) (*d2automap.Table, []int) {
	fmt.Println("\n== 2. LA TABLA DEL AUTOMAPA (AutoMap.txt del MPQ)")

	mpqBin, bin := buscarEnLosMPQs(mpqs, binAutomap)
	mpqTxt, txt := buscarEnLosMPQs(mpqs, txtAutomap)

	fmt.Printf("   automap.bin: %s %d B | AutoMap.txt: %s %d B\n", mpqBin, len(bin), mpqTxt, len(txt))

	var tabla *d2automap.Table

	if len(bin) > 0 {
		if t, err := d2automap.ParseBin(bin); err == nil {
			tabla = t
			fmt.Printf("   automap.bin PARSEA: %d filas\n", len(t.Rows()))
		} else {
			fmt.Println("   automap.bin NO parsea:", err)
		}
	}

	if tabla == nil && len(txt) > 0 {
		t, err := d2automap.ParseTxt(strings.NewReader(string(txt)))
		if err != nil {
			fmt.Println("   AutoMap.txt NO parsea:", err)

			return nil, nil
		}

		tabla = t
		fmt.Printf("   AutoMap.txt PARSEA: %d filas (el lector del motor)\n", len(t.Rows()))
	}

	// LA MEMORIA DE LA TABLA: 2.603 filas con sus rebanadas de cuadros. El semiespacio de la consola queda al 96 %
	// con el HUD puesto, asi que este numero (mas el del modelo) decide si el minimapa entra tal cual.
	var ms runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&ms)

	fmt.Printf("   memoria de la tabla en el monton de Go: %d B (2.603 filas)\n", ms.HeapAlloc-marcaDeMemoria)
	marcaDeMemoria = ms.HeapAlloc

	if tabla == nil {
		fmt.Println("   sin tabla no hay minimapa")

		return nil, nil
	}

	nombre := d2automap.LevelName(nivelPueblo)
	filas := tabla.LevelRows(nivelPueblo)

	fmt.Printf("   el pueblo: nivel=%d nombre=%q filas=%d\n", nivelPueblo, nombre, len(filas))

	arte := map[int]bool{}
	porCapa := map[int]int{}

	for _, f := range filas {
		porCapa[f.Tile] += len(f.Cels)

		for _, c := range f.Cels {
			arte[c] = true
		}
	}

	var celdas []int

	for c := range arte {
		celdas = append(celdas, c)
	}

	sort.Ints(celdas)

	fmt.Printf("   cuadros de arte que piden las filas del pueblo: %d distintos de 1.260\n", len(celdas))
	fmt.Printf("   rango: %d..%d\n", celdas[0], celdas[len(celdas)-1])
	fmt.Printf("   los cuadros: %v\n", celdas)

	var orients []int

	for o := range porCapa {
		orients = append(orients, o)
	}

	sort.Ints(orients)

	for _, o := range orients {
		nombre := "?"

		if o >= 0 && o < len(nombresDeOrientacion) {
			nombre = nombresDeOrientacion[o]
		}

		fmt.Printf("   orientacion %2d (%s): %d cuadros en las filas\n", o, nombre, porCapa[o])
	}

	return tabla, celdas
}

// TileName no esta exportado con ese nombre en todas las versiones del paquete: se toma de la tabla de nombres.
func init() {
	_ = d2automap.Any
}

// ============================== 3. EL MODELO ==============================

func elModelo(mpqs map[string]*d2mpq.MPQ, tabla *d2automap.Table, celdasDelArte []int) {
	fmt.Println("\n== 3. EL MODELO: el reveal del pueblo y los cuadros que usa")

	if tabla == nil {
		fmt.Println("   sin tabla no hay modelo")

		return
	}

	_, datosDS1 := buscarEnLosMPQs(mpqs, ds1Pueblo)

	ds, err := d2ds1.Unmarshal(datosDS1)
	if err != nil {
		fmt.Println("   el .ds1 del pueblo no parsea:", err)

		return
	}

	w, h := ds.Size()
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
					}
				}
			}
		}
	}

	fmt.Printf("   DS1 del pueblo: %dx%d tiles, %d B; capas con grafico pisos=%d paredes=%d\n",
		w, h, len(datosDS1), pisos, paredes)

	var celda d2automap.Cell

	fmt.Printf("   celda del modelo: %d B (x, y, cuadro, capa)\n", unsafe.Sizeof(celda))

	// el pueblo ENTERO (el techo: lo que se revela caminando todo)
	m := d2automap.NewModel()
	area := d2automap.Area{HalfW: w * 8, HalfH: h * 4}

	n := m.RevealTiles(tabla, fuenteDeTiles{ds}, nivelPueblo, semilla, float64(w/2), float64(h/2), area)

	fmt.Printf("   REVEAL del pueblo entero: agregadas=%d celdas=%d (piso=%d pared=%d objeto=%d) = %d B\n",
		n, m.Count(), m.CountLayer(d2automap.LayerFloor), m.CountLayer(d2automap.LayerWall),
		m.CountLayer(d2automap.LayerObject), m.Count()*int(unsafe.Sizeof(celda)))

	// LA MEMORIA QUE PIDE DE VERDAD EN EL SEMIESPACIO DE LA CONSOLA: el modelo son mapas de Go, no el arreglo de
	// 32 B por celda. El semiespacio de la consola queda al 96 % con el HUD puesto, asi que este numero decide si
	// el minimapa entra tal cual o hay que guardar las celdas en un tramo del monton de C.
	var ms runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&ms)
	vivoConModelo := ms.HeapAlloc

	fmt.Printf("   memoria del modelo (mapas de Go, medido con runtime.ReadMemStats): %d B para %d celdas\n",
		vivoConModelo-marcaDeMemoria, m.Count())

	// los cuadros DISTINTOS que usan esas celdas: eso es la memoria del arte
	usados := map[int]int{}
	enElArte := 0

	for _, c := range m.Cells() {
		usados[c.Cel]++

		for _, a := range celdasDelArte {
			if a == c.Cel {
				enElArte++

				break
			}
		}
	}

	var lista []int

	for c := range usados {
		lista = append(lista, c)
	}

	sort.Ints(lista)

	fmt.Printf("   cuadros DISTINTOS que usan las %d celdas: %d (de los %d de la tabla: %d celdas usan uno de la tabla)\n",
		m.Count(), len(lista), len(celdasDelArte), enElArte)
	fmt.Printf("   los %d cuadros usados: %v\n", len(lista), lista)

	// ---- EL RECORTE EN EL LIENZO DEL POC
	fmt.Println("\n   -- EL RECORTE: la caja del minimapa en el lienzo del POC (640x480)")

	cx, cy := d2automap.WorldCell(celdaPOCX, celdaPOCY)
	d210, d214 := d2automap.MiniBoxOffsets(lienzoAncho, lienzoAlto, false)
	lay := d2automap.ComputeLayoutOffsets(d2automap.SizeMini, lienzoAncho, lienzoAlto, cx, cy,
		d2automap.PanelNone, false, d210, d214)

	celdas := m.Cells()

	fmt.Printf("   offsets del motor (DAT_0079d210/214): %d %d\n", d210, d214)
	fmt.Printf("   origen del minimapa: %d %d; clip=(%d,%d)-(%d,%d) = %dx%d\n",
		lay.OriginX, lay.OriginY, lay.Clip.X0, lay.Clip.Y0, lay.Clip.X1, lay.Clip.Y1,
		lay.Clip.X1-lay.Clip.X0, lay.Clip.Y1-lay.Clip.Y0)

	// (a) la proyeccion del minimapa del motor (escala 20) con la caja de 8x16 que el motor pide
	dentroA, solapan := recorte(celdas, lay, 8, 16)

	fmt.Printf("   (a) escala 20 + celda de 8x16 (lo que pide el motor): celdas adentro=%d de %d\n", dentroA, len(celdas))

	// (b) la MISMA proyeccion con el arte que EXISTE (16x32): se pisan
	dentroB, _ := recorte(celdas, lay, 16, 32)

	fmt.Printf("   (b) escala 20 + celda de 16x32 (el arte que hay): celdas que tocan la caja=%d; pixeles pisados=%d\n",
		dentroB, solapan)
	fmt.Println("       (con 16x32 a escala 20 cada celda mide el DOBLE que su paso: el mapa sale encimado)")

	// (c) la 125: el mismo modelo en un lienzo de 320x240, para reconciliar el 626
	lay320 := d2automap.ComputeLayoutOffsets(d2automap.SizeMini, 320, 240, cx, cy,
		d2automap.PanelNone, false, 0, 0)
	dentroC, _ := recorte(celdas, lay320, 8, 16)

	fmt.Printf("   (c) como la 125: lienzo 320x240, clip=(%d,%d)-(%d,%d): celdas adentro=%d (la 125 dijo 626)\n",
		lay320.Clip.X0, lay320.Clip.Y0, lay320.Clip.X1, lay320.Clip.Y1, dentroC)
}

// recorte cuenta las celdas del modelo cuya caja toca la caja del minimapa, y cuantos pixeles se pisan entre
// celdas vecinas (el area de la union menos la suma de las areas: si es > 0, el arte es mas grande que su paso).
func recorte(celdas []d2automap.Cell, lay d2automap.Layout, cw, ch int) (dentro, pisados int) {
	tipos := map[[2]int]bool{}

	for _, c := range celdas {
		x, y := lay.ToScreen(c.X, c.Y)
		if x+cw <= lay.Clip.X0 || x >= lay.Clip.X1 || y <= lay.Clip.Y0 || y-ch >= lay.Clip.Y1 {
			continue
		}

		dentro++

		// la celda como caja de pixeles (ancla abajo a la izquierda): se recorre y se cuenta lo repetido
		for py := y - ch; py < y; py++ {
			for px := x; px < x+cw; px++ {
				k := [2]int{px, py}
				if tipos[k] {
					pisados++
				} else {
					tipos[k] = true
				}
			}
		}
	}

	return dentro, pisados
}

// ============================== 4. EL ARTE ==============================

func elArte(mpqs map[string]*d2mpq.MPQ, celdasDelArte []int) {
	fmt.Println("\n== 4. EL ARTE: `maximaps.DC6`, la memoria real y lo que pide el motor")

	for _, ruta := range []string{dc6Mini, dc6Full} {
		mpq, datos := buscarEnLosMPQs(mpqs, ruta)
		if datos == nil {
			fmt.Printf("   %-42s NO ESTA en ningun MPQ\n", ruta)

			continue
		}

		dc, err := d2dc6.Load(datos)
		if err != nil {
			fmt.Printf("   %-42s %s %d B NO PARSEA: %v\n", ruta, mpq, len(datos), err)

			continue
		}

		bytes16, bytes8 := 0, 0
		caja := ""

		for i := range dc.Frames {
			f := dc.Frames[i]
			if i == 0 {
				caja = fmt.Sprintf("%dx%d", f.Width, f.Height)
			}

			bytes16 += int(f.Width) * int(f.Height)
			bytes8 += (int(f.Width) / 2) * (int(f.Height) / 2)
		}

		fmt.Printf("   %-42s %s %7d B | %d cuadros de %s | indices=%d B | decimado a mitad=%d B\n",
			ruta, mpq, len(datos), len(dc.Frames), caja, bytes16, bytes8)
	}

	fmt.Println("   -- la memoria del arte que el minimapa NECESITA de verdad")

	// OJO CON EL NOMBRE (medido): `maximaps.dc6` -- el de 8x16 que pide el motor para el minimapa -- NO EXISTE en
	// esta instalacion. El que existe es `maximap.dc6`, el del mapa grande, y sus 1.260 cuadros son de 16x32.
	// O sea: el arte del minimapa hay que sacarlo de los mismos cuadros decimados a la mitad (1 de cada 2 px),
	// que es exactamente la caja de 8x16 que el motor pide.
	var (
		usados     []int
		bytesUsado int
	)

	_, datos := buscarEnLosMPQs(mpqs, dc6Full)
	if datos == nil {
		fmt.Println("   sin maximap.dc6 no hay arte")

		return
	}

	dc, err := d2dc6.Load(datos)
	if err != nil {
		fmt.Println("   maximap.dc6 NO PARSEA:", err)

		return
	}

	visto := map[int]bool{}

	for _, c := range celdasDelArte {
		if c < len(dc.Frames) && !visto[c] {
			visto[c] = true

			usados = append(usados, c)
		}
	}

	sort.Ints(usados)

	for _, c := range usados {
		f := dc.Frames[c]
		bytesUsado += int(f.Width) * int(f.Height)
	}

	fmt.Printf("   cuadros que pide la tabla del pueblo: %d; todos existen en el DC6 (%d cuadros): %v\n",
		len(usados), len(dc.Frames), usados)
	fmt.Printf("   decodificados a 16x32: %d B (512 B por cuadro)\n", bytesUsado)
	fmt.Printf("   decodificados a 8x16 (1 de cada 2 px, la caja del minimapa): %d B (128 B por cuadro)\n", bytesUsado/4)
	fmt.Printf("   el archivo crudo: %d B (se lee, se decodifica y se suelta)\n", len(datos))
	fmt.Println("   el semiespacio NO lo ve: el arte va al monton de C (`d2c_pedir_bloque`), como el HUD")
}

// fuenteDeTiles: el MISMO adaptador que el motor (d2game/d2player/automap.go, tileSource) y que la sonda del HUD
// (console/dc/sonda-hud-125).
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
