// DC-002.145 -- EL BLOOD MOOR DE VERDAD, MEDIDO EN EL HOST ANTES DE TOCAR LA CONSOLA.
//
// QUE HACE. Corre el DRLG del acto 1 (el mismo port que usa el motor de la PC:
// `drlgworld.Generate` + `drlgoutdoor.Generate` + `Level.BuildTiles`) para el nivel 2 (Blood Moor) con una
// semilla, y saca los numeros que la consola necesita para decidir si el afuera entra POR REGIONES:
//
//  1. el rectangulo del nivel en el mundo y cuantas casillas tiene (56 x 96 = 5.376);
//  2. los cuartos (llanos de 8x8 y presets), con el `.ds1` de cada preset;
//  3. los registros de tile que salen de `BuildTiles`, agrupados por `.dt1`;
//  4. el arena de bloques que la consola tendria que pedir (los bloques RLE de los tiles cuya
//     `(estilo, secuencia, tipo)` esta en las claves del nivel, que es exactamente lo que copia
//     `d2asset.PrepararArenaDeBloquesUsados`) y el buffer prestado de lectura (el `.dt1` mas grande);
//  5. lo mismo por REGION (32x32 casillas por omision) y una tabla de todas las regiones, para elegir la que se
//     compone primero.
//
// Uso:
//
//	go run ./console/dc/medidas/moor-drlg "<carpeta de Diablo II>"
//	go run ./console/dc/medidas/moor-drlg "<dir>" -seed 1 -lado 32 -ox 0 -oy 0
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2drlg"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2drlg/drlgoutdoor"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2drlg/drlgworld"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

const (
	nivel             = 2 // el Blood Moor en Levels.txt
	casillasPorCuarto = 8
)

// origenX/origenY: el origen del nivel en el mundo (Rect.X/Y). Los registros del DRLG vienen en coordenadas del
// MUNDO; las regiones son relativas al nivel (0..55 x 0..95).
var origenX, origenY int

func main() {
	seed := flag.Uint("seed", 1, "semilla del juego")
	lado := flag.Int("lado", 32, "lado de la region a medir, en casillas")
	ox := flag.Int("ox", -1, "origen x de la region (-1 = todas las regiones)")
	oy := flag.Int("oy", -1, "origen y de la region (-1 = todas las regiones)")
	diff := flag.Int("diff", 0, "dificultad (0 normal, 1 nightmare, 2 hell)")
	rutaPlan := flag.String("plan", "", "escribe el plan de la region como fuente Go en esta ruta")
	flag.Parse()

	dir := flag.Arg(0)
	if dir == "" {
		dir = os.Getenv("D2_DIR")
	}

	if dir == "" {
		fmt.Println("falta la carpeta de Diablo II (argumento o D2_DIR)")
		os.Exit(2)
	}

	leer := abrirMPQs(dir)

	tb, err := tablas(leer)
	if err != nil {
		fmt.Println("tablas:", err)
		os.Exit(1)
	}

	rec, ok := tb.Level(nivel)
	if !ok {
		fmt.Println("no hay fila del nivel", nivel)
		os.Exit(1)
	}

	fmt.Printf("NIVEL id=%d nombre=%q drlgType=%d levelType=%d levels.txt=%dx%d\n",
		nivel, rec.Name, rec.DrlgType, rec.LevelType, rec.SizeX[0], rec.SizeY[0])

	lay, err := drlgworld.Generate(tb, uint32(*seed), d2drlg.Difficulty(*diff))
	if err != nil {
		fmt.Println("mundo:", err)
		os.Exit(1)
	}

	p, err := drlgoutdoor.ParamsFromLayout(tb, lay, nivel, uint32(*seed))
	if err != nil {
		fmt.Println("params:", err)
		os.Exit(1)
	}

	fmt.Printf("PARAMS mundo rect=%d,%d %dx%d casillas=%d odFlags=%#x baseSeed=%#x vecinos=%d\n",
		p.Rect.X, p.Rect.Y, p.Rect.W, p.Rect.H, p.Rect.W*p.Rect.H, p.OdFlags, p.BaseSeed, len(p.Neighbors))
	fmt.Printf("PARAMS pueblo=%d,%d %dx%d\n", p.Town.X, p.Town.Y, p.Town.W, p.Town.H)

	for _, n := range p.Neighbors {
		fmt.Printf("VECINO nivel=%d lado=%d rect=%d,%d %dx%d\n", n.Level, n.Dir, n.Rect.X, n.Rect.Y, n.Rect.W, n.Rect.H)
	}

	origenX, origenY = p.Rect.X, p.Rect.Y

	env := drlgoutdoor.NewEnv(tb, func(file string) ([]byte, error) {
		return leer("data/global/tiles/" + file)
	})

	lv, err := drlgoutdoor.Generate(env, p)
	if err != nil {
		fmt.Println("drlg:", err)
		os.Exit(1)
	}

	ancho, alto := lv.W*casillasPorCuarto, lv.H*casillasPorCuarto

	fmt.Printf("GENERADO rejilla=%dx%d cuartos=%d casillas=%dx%d\n", lv.W, lv.H, len(lv.Rooms), ancho, alto)

	llanos := 0
	presets := map[string]int{}

	for _, r := range lv.Rooms {
		if r.Type == 1 {
			llanos++

			continue
		}

		ruta := "?"

		if pr, hay := tb.PrestByDef(r.PrestDef); hay && r.File < len(pr.File) {
			ruta = drlgoutdoor.NormalizePrestFile(pr.File[r.File])
		}

		presets[ruta]++
	}

	rutas := make([]string, 0, len(presets))
	for r := range presets {
		rutas = append(rutas, r)
	}

	sort.Strings(rutas)

	fmt.Printf("CUARTOS llanos=%d presets=%d piezasDistintas=%d\n", llanos, len(lv.Rooms)-llanos, len(rutas))

	for _, r := range rutas {
		fmt.Printf("  pieza %-40s x%d\n", r, presets[r])
	}

	tiles, err := lv.BuildTiles()
	if err != nil {
		fmt.Println("BuildTiles:", err)
		os.Exit(1)
	}

	fmt.Println("=== EL NIVEL ENTERO (56 x 96) ===")
	medir(tiles, 0, 0, 0, leer).imprimir("NIVEL")

	if *rutaPlan != "" {
		if *ox < 0 || *oy < 0 {
			fmt.Println("el plan pide -ox y -oy (la region a componer)")
			os.Exit(2)
		}

		pl := planificar(tiles, *lado, *ox, *oy, uint32(*seed), leer)
		pl.anchoNivel, pl.altoNivel = ancho, alto

		if err := emitir(pl, *rutaPlan); err != nil {
			fmt.Println("no pude escribir el plan:", err)
			os.Exit(1)
		}

		fmt.Printf("PLAN archivo=%s region=%dx%d desde %d,%d celdas=%d registros=%d piezas=%d claves=%d buffer=%d crudo=%d spawn=%d,%d\n",
			*rutaPlan, pl.lado, pl.lado, pl.regionX, pl.regionY, len(pl.celdas), pl.registros, len(pl.piezas),
			len(pl.claves), pl.buffer, pl.crudo, pl.spawnX, pl.spawnY)

		return
	}

	if *ox >= 0 && *oy >= 0 {
		fmt.Printf("=== LA REGION %dx%d desde %d,%d ===\n", *lado, *lado, *ox, *oy)
		medir(tiles, *lado, *ox, *oy, leer).imprimir("REGION")

		return
	}

	fmt.Printf("=== LAS REGIONES DE %dx%d ===\n", *lado, *lado)

	for fy := 0; fy+*lado <= alto; fy += *lado {
		for fx := 0; fx+*lado <= ancho; fx += *lado {
			medir(tiles, *lado, fx, fy, leer).imprimir(fmt.Sprintf("REGION %d,%d", fx, fy))
		}
	}
}

// abrirMPQs: el lector de los dos MPQ del juego (el parche tiene las tablas, d2data los .ds1 y los .dt1).
func abrirMPQs(dir string) func(string) ([]byte, error) {
	entradas, _ := os.ReadDir(dir)
	enDisco := map[string]string{}

	for _, e := range entradas {
		enDisco[strings.ToLower(e.Name())] = e.Name()
	}

	var mpqs []*d2mpq.MPQ

	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq"} {
		real, hay := enDisco[nombre]
		if !hay {
			continue
		}

		m, err := d2mpq.FromFile(dir + "/" + real)
		if err != nil {
			fmt.Println("no abri", real, err)
			os.Exit(1)
		}

		mpqs = append(mpqs, m)
	}

	if len(mpqs) == 0 {
		fmt.Println("no encontre patch_d2.mpq / d2data.mpq en", dir)
		os.Exit(1)
	}

	return func(nombre string) ([]byte, error) {
		clave := strings.TrimPrefix(nombre, "/")
		clave = strings.ReplaceAll(clave, "/", "\\")

		var ultimo error

		for _, m := range mpqs {
			datos, err := m.ReadFile(clave)
			if err == nil {
				return datos, nil
			}

			ultimo = err
		}

		return nil, ultimo
	}
}

func tablas(leer func(string) ([]byte, error)) (*d2drlg.Tables, error) {
	get := func(p string) []byte {
		b, err := leer(p)
		if err != nil {
			return nil
		}

		return b
	}

	raw := d2drlg.Raw{
		Levels:      get("/data/global/excel/Levels.txt"),
		LvlMaze:     get("/data/global/excel/LvlMaze.txt"),
		LvlPrest:    get("/data/global/excel/LvlPrest.txt"),
		LvlPrestBin: get("/data/global/excel/lvlprest.bin"),
		LvlTypes:    get("/data/global/excel/LvlTypes.txt"),
		LvlSub:      get("/data/global/excel/LvlSub.txt"),
		LvlWarp:     get("/data/global/excel/LvlWarp.txt"),
	}

	if raw.Levels == nil || raw.LvlMaze == nil {
		return nil, fmt.Errorf("faltan tablas")
	}

	t, err := d2drlg.Load(raw)
	if err != nil && raw.LvlPrestBin != nil {
		raw.LvlPrestBin = nil

		return d2drlg.Load(raw)
	}

	return t, err
}

// medido: lo que una parte del nivel (o el nivel entero) le pide a la consola.
type medido struct {
	casillas    int
	sinPiso     int
	registros   int
	pisos       int
	paredes     int
	sombras     int
	crudo       int
	mayorCrudo  int
	mayorNombre string
	bloquesRef  int
	arena       int
	claves      int
	archivos    []string
	porArchivo  map[string]int
	minX, maxX  int
	minY, maxY  int
}

// medir: cuenta los registros que caen en el rectangulo [ox,ox+lado) x [oy,oy+lado) (lado == 0 = todo el
// nivel), y despues calcula los bloques y el arena como los calcula la consola (por clave).
func medir(tiles []*drlgoutdoor.RoomTiles, lado, ox, oy int, leer func(string) ([]byte, error)) *medido {
	m := &medido{porArchivo: map[string]int{}}

	dentro := func(x, y int) bool {
		if lado == 0 {
			return true
		}

		return x-origenX >= ox && y-origenY >= oy && x-origenX < ox+lado && y-origenY < oy+lado
	}

	type ref struct {
		archivo string
		indice  int
	}

	refs := map[ref]bool{}
	claves := map[int64]bool{}
	casillas := map[[2]int]bool{}

	for _, rt := range tiles {
		if rt == nil {
			continue
		}

		for _, r := range rt.Floors {
			x, y := rt.Room.X+r.X, rt.Room.Y+r.Y
			if !dentro(x, y) {
				continue
			}

			m.pisos++
			refs[ref{r.Tile.File(), r.Tile.Idx}] = true
			casillas[[2]int{x - origenX, y - origenY}] = true
		}

		for _, r := range rt.Walls {
			x, y := rt.Room.X+r.X, rt.Room.Y+r.Y
			if !dentro(x, y) {
				continue
			}

			m.paredes++
			refs[ref{r.Tile.File(), r.Tile.Idx}] = true
		}

		for _, r := range rt.Shadows {
			x, y := rt.Room.X+r.X, rt.Room.Y+r.Y
			if !dentro(x, y) {
				continue
			}

			m.sombras++
			refs[ref{r.Tile.File(), r.Tile.Idx}] = true
		}
	}

	m.registros = m.pisos + m.paredes + m.sombras
	m.casillas = len(casillas)

	if len(casillas) > 0 {
		primero := true

		for k := range casillas {
			if primero {
				m.minX, m.maxX, m.minY, m.maxY = k[0], k[0], k[1], k[1]
				primero = false

				continue
			}

			if k[0] < m.minX {
				m.minX = k[0]
			}

			if k[0] > m.maxX {
				m.maxX = k[0]
			}

			if k[1] < m.minY {
				m.minY = k[1]
			}

			if k[1] > m.maxY {
				m.maxY = k[1]
			}
		}
	}

	if lado > 0 {
		m.sinPiso = lado*lado - m.casillas
	}

	porArchivoRefs := map[string]map[int]bool{}
	for r := range refs {
		if porArchivoRefs[r.archivo] == nil {
			porArchivoRefs[r.archivo] = map[int]bool{}
		}

		porArchivoRefs[r.archivo][r.indice] = true
	}

	archivos := make([]string, 0, len(porArchivoRefs))
	for f := range porArchivoRefs {
		archivos = append(archivos, f)
	}

	sort.Strings(archivos)

	parsed := map[string]*d2dt1.DT1{}

	for _, f := range archivos {
		m.porArchivo[f] = len(porArchivoRefs[f])

		b, err := leer("data/global/tiles/" + f)
		if err != nil {
			fmt.Printf("   (no pude leer %s: %v)\n", f, err)

			continue
		}

		m.crudo += len(b)

		if len(b) > m.mayorCrudo {
			m.mayorCrudo, m.mayorNombre = len(b), f
		}

		dt, err := d2dt1.LoadDT1(b)
		if err != nil {
			fmt.Printf("   (no pude parsear %s: %v)\n", f, err)

			continue
		}

		parsed[f] = dt

		// las claves del nivel son las de los tiles REFERENCIADOS, con el tipo del propio .dt1 (es lo que
		// mira el motor: `claveDeTile(style, sequence, tile.Type)`)
		for i := range dt.Tiles {
			if !porArchivoRefs[f][i] {
				continue
			}

			t := &dt.Tiles[i]
			claves[clave(int64(t.Style), int64(t.Sequence), int64(t.Type))] = true

			for _, bl := range t.Blocks {
				m.bloquesRef += len(bl.EncodedData)
			}
		}
	}

	m.claves = len(claves)
	m.archivos = archivos

	// el arena: bloques de TODOS los tiles cuya clave esta en el nivel (`PrepararArenaDeBloquesUsados`)
	for _, f := range archivos {
		dt := parsed[f]
		if dt == nil {
			continue
		}

		for i := range dt.Tiles {
			t := &dt.Tiles[i]
			if !claves[clave(int64(t.Style), int64(t.Sequence), int64(t.Type))] {
				continue
			}

			for _, bl := range t.Blocks {
				m.arena += len(bl.EncodedData)
			}
		}
	}

	return m
}

func (m *medido) imprimir(titulo string) {
	fmt.Printf("%s casillas=%d conPiso=%d sinPiso=%d registros=%d piso/pared/sombra=%d/%d/%d rango=x %d..%d y %d..%d\n",
		titulo, m.casillas+m.sinPiso, m.casillas, m.sinPiso, m.registros, m.pisos, m.paredes, m.sombras,
		m.minX, m.maxX, m.minY, m.maxY)
	fmt.Printf("%s dt1s=%d crudo=%d mayorCrudo=%d (%s) bloquesRef=%d arenaDeBloques=%d claves=%d\n",
		titulo, len(m.archivos), m.crudo, m.mayorCrudo, m.mayorNombre, m.bloquesRef, m.arena, m.claves)
	fmt.Printf("%s piezas: %s\n", titulo, strings.Join(m.archivos, " "))

	for _, f := range m.archivos {
		fmt.Printf("   %-38s tiles=%d\n", f, m.porArchivo[f])
	}
}

// clave: el mismo empaquetado de 21 bits que `d2asset.ClaveDeTile` y el motor.
func clave(estilo, secuencia, tipo int64) int64 {
	const bits, mascara = 21, (1 << 21) - 1

	return estilo&mascara<<(2*bits) | secuencia&mascara<<bits | tipo&mascara
}
