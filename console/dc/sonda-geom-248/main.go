// sonda-geom-248: mide la GEOMETRIA REAL de los tiles del nivel (DC-002.48).
//
// La pregunta que contesta: el compositor `nivel.go` avanza la celda (x,y) 32 px en x y 16 px en y
// ("viewport.go: WorldToOrtho usa tileWidth=32 y tileHeight=16"), pero el viewport del MOTOR
// (d2core/d2map/d2maprenderer/viewport.go) tiene tileWidth=80 y tileHeight=40. Uno de los dos miente.
// Lo que no se puede deducir es el TAMANO de los graficos: cada tile del DT1 trae su Width/Height.
//
// Imprime:
//   - el DS1 (version, w, h, capas, tiles no vacios por capa);
//   - el histograma de Width x Height de los tiles PISO y de los tiles PARED tal como vienen del DT1;
//   - el paso del MOTOR (80x40) y el del compositor (32x16), y cuantos pixeles del lienzo cambian;
//   - para las paredes, min/max de los bloques (de donde sale YAdjust = minY+80).
//
// Uso: go run ./console/dc/sonda-geom-248 -mpq "$HOME/.wine-d2/drive_c/Games/Diablo II/d2data.mpq"
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

const ds1Defecto = "data\\global\\tiles\\ACT1\\TOWN\\townN1.ds1"

const barraInvertida = string(rune(92))

// rutaDelDT1: la misma normalizacion del motor (nivel.go: RutaDelDT1).
func rutaDelDT1(f string) string {
	f = strings.ToLower(f)
	f = strings.ReplaceAll(f, "c:", "")
	f = strings.ReplaceAll(f, ".tg1", ".dt1")

	if i := strings.LastIndex(f, "tiles"); i >= 0 {
		f = f[i+len("tiles"):]
	}

	f = strings.TrimPrefix(f, barraInvertida)
	f = strings.TrimPrefix(f, "/")

	return "data/global/tiles/" + f
}

func leerDelMPQ(m *d2mpq.MPQ, nombre string) ([]byte, error) {
	return m.ReadFile(strings.ReplaceAll(nombre, "/", barraInvertida))
}

func autodetectar() string {
	if d := os.Getenv("D2_DIR"); d != "" {
		return filepath.Join(d, "d2data.mpq")
	}

	p := filepath.Join(os.Getenv("HOME"), ".wine-d2", "drive_c", "Games", "Diablo II", "d2data.mpq")
	if _, err := os.Stat(p); err == nil {
		return p
	}

	return ""
}

func main() {
	mpqPath := flag.String("mpq", "", "d2data.mpq (por defecto: la instalacion del juego)")
	ds1Name := flag.String("ds1", ds1Defecto, "el .ds1 del nivel")
	flag.Parse()

	ruta := *mpqPath
	if ruta == "" {
		ruta = autodetectar()
	}

	if ruta == "" {
		fmt.Fprintln(os.Stderr, "no encuentro d2data.mpq (usa -mpq o D2_DIR)")
		os.Exit(1)
	}

	m, err := d2mpq.FromFile(ruta)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude abrir el MPQ:", err)
		os.Exit(1)
	}

	defer m.Close()

	raw, err := m.ReadFile(*ds1Name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude leer el DS1:", err)
		os.Exit(1)
	}

	d, err := d2ds1.Unmarshal(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude parsear el DS1:", err)
		os.Exit(1)
	}

	fmt.Printf("DS1 %s bytes=%d version=%d act=%d w=%d h=%d files=%d\n",
		*ds1Name, len(raw), d.Version(), int(d.Act), d.Width(), d.Height(), len(d.Files))

	for i := 0; i < 2; i++ {
		l := d.GetFloor(i)
		if l == nil {
			continue
		}

		fmt.Printf("CAPA piso %d: %dx%d tiles no vacios=%d\n", i, l.Width(), l.Height(), cuenta(l))
	}

	for i := 0; i < 4; i++ {
		l := d.GetWall(i)
		if l == nil {
			continue
		}

		fmt.Printf("CAPA pared %d: %dx%d tiles no vacios=%d\n", i, l.Width(), l.Height(), cuenta(l))
	}

	// los DT1, en el orden del DS1 (el orden en que el motor arma dt1TileData)
	hist := map[string]int{}
	dh := map[string]int{}

	var minPisoW, minPisoH, maxPisoW, maxPisoH = 1 << 30, 1 << 30, 0, 0
	var minParW, minParH, maxParW, maxParH = 1 << 30, 1 << 30, 0, 0

	totalPiso, totalPared := 0, 0

	for _, f := range d.Files {
		nombre := rutaDelDT1(f)

		rawDT1, err := leerDelMPQ(m, nombre)
		if err != nil {
			fmt.Printf("DT1 %s FALLO: %v\n", nombre, err)
			continue
		}

		dt1, err := d2dt1.LoadDT1(rawDT1)
		if err != nil {
			fmt.Printf("DT1 %s PARSE-FALLO: %v\n", nombre, err)
			continue
		}

		fmt.Printf("DT1 %s bytes=%d tiles=%d\n", nombre, len(rawDT1), len(dt1.Tiles))

		for i := range dt1.Tiles {
			t := &dt1.Tiles[i]
			k := fmt.Sprintf("%dx%d", t.Width, t.Height)
			dh[k]++

			alto := int(t.Height)
			if alto < 0 {
				alto = -alto
			}

			if t.Type == 0 || t.Type == 1 || t.Type == 2 { // 0=piso, 1-2 sombra/pared baja; el tipo real lo dice el DS1
				_ = alto
			}

			// por tipo del DT1: los pisos son Type 0
			if t.Type == 0 {
				totalPiso++
				if int(t.Width) < minPisoW {
					minPisoW = int(t.Width)
				}
				if int(t.Width) > maxPisoW {
					maxPisoW = int(t.Width)
				}
				if alto < minPisoH {
					minPisoH = alto
				}
				if alto > maxPisoH {
					maxPisoH = alto
				}
			} else {
				totalPared++
				if int(t.Width) < minParW {
					minParW = int(t.Width)
				}
				if int(t.Width) > maxParW {
					maxParW = int(t.Width)
				}
				if alto < minParH {
					minParH = alto
				}
				if alto > maxParH {
					maxParH = alto
				}
			}

			hist[k]++
		}
	}

	fmt.Println()
	fmt.Printf("TAMANOS de TODOS los tiles de los DT1 del nivel (Width x Height, histograma):\n")

	for _, k := range claves(hist) {
		fmt.Printf("  %-10s %d\n", k, hist[k])
	}

	fmt.Println()
	fmt.Printf("PISO (Type 0): n=%d Width %d..%d  Height(abs) %d..%d\n", totalPiso, minPisoW, maxPisoW, minPisoH, maxPisoH)
	fmt.Printf("PARED (Type>0): n=%d Width %d..%d  Height(abs) %d..%d\n", totalPared, minParW, maxParW, minParH, maxParH)
	fmt.Println()
	fmt.Println("PASO de la celda: MOTOR (viewport.go) tileWidth=80 tileHeight=40  |  compositor nivel.go 32/16")

	// DC-002.48: las reglas con las que el RENDERER decide si un tile se dibuja o no son MAS ESTRICTAS que
	// "Style!=0 || Seq!=0": `generateFloorCache`/`renderPass1` piden `!Hidden() && Prop1 != 0` y
	// `generateWallCache` agrega `!Type.Special()`. Un compositor que solo mire Style/Seq dibuja tiles que el
	// motor NO dibuja. Aca se cuenta cuantos son en este nivel, que es el numero que dice si esa diferencia
	// afecta a esta ventana.
	saltadosProp, saltadosHidden, saltadosSpecial := 0, 0, 0

	for i := 0; i < 2; i++ {
		l := d.GetFloor(i)
		if l == nil {
			continue
		}

		for y := 0; y < l.Height(); y++ {
			for x := 0; x < l.Width(); x++ {
				t := l.Tile(x, y)
				if t == nil || (t.Style == 0 && t.Sequence == 0) {
					continue
				}

				if t.Hidden() {
					saltadosHidden++
				}

				if t.Prop1 == 0 {
					saltadosProp++
				}
			}
		}
	}

	for i := 0; i < 4; i++ {
		l := d.GetWall(i)
		if l == nil {
			continue
		}

		for y := 0; y < l.Height(); y++ {
			for x := 0; x < l.Width(); x++ {
				t := l.Tile(x, y)
				if t == nil || (t.Style == 0 && t.Sequence == 0) {
					continue
				}

				if t.Hidden() {
					saltadosHidden++
				}

				if t.Prop1 == 0 {
					saltadosProp++
				}

				if d2enum.TileType(t.Type).Special() {
					saltadosSpecial++
				}
			}
		}
	}

	fmt.Printf("NO-DIBUJADOS por el motor: Prop1==0 %d, Hidden %d, Type.Special() %d (de %d pedidos)\n",
		saltadosProp, saltadosHidden, saltadosSpecial, d.Width()*d.Height())
}

func cuenta(l interface {
	Width() int
	Height() int
	Tile(x, y int) *d2ds1.Tile
}) int {
	c := 0

	for y := 0; y < l.Height(); y++ {
		for x := 0; x < l.Width(); x++ {
			t := l.Tile(x, y)
			if t != nil && (t.Style != 0 || t.Sequence != 0) {
				c++
			}
		}
	}

	return c
}

func claves(m map[string]int) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}

	sort.Strings(ks)

	return ks
}
