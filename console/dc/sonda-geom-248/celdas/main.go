// celdas.go: el censo de las CELDAS del nivel contra lo que cuenta el motor (DC-002.48).
//
// La pregunta: el motor, en el mismo .ds1 (Act1/Town/TownN1.ds1), informa `cells=2337 nofloor=0
// floors=2337`; el compositor pide 1982 pisos. ¿Las 355 celdas de diferencia son celdas SIN piso en el
// archivo, o son registros que el compositor descarta y el motor si dibuja?
//
// Cuenta, por capa de piso y por celda:
//   - celdas con al menos un registro de piso (Style!=0 || Seq!=0);
//   - celdas sin ningun registro de piso;
//   - registros con Prop1==0 o Hidden (los que el motor NO dibuja);
//   - celdas cuyo unico piso es un registro que el motor no dibuja.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2maprenderer"
)

func main() {
	mpqPath := flag.String("mpq", filepath.Join(os.Getenv("HOME"), ".wine-d2", "drive_c", "Games", "Diablo II", "d2data.mpq"), "d2data.mpq")
	ds1Name := flag.String("ds1", "data\\global\\tiles\\ACT1\\TOWN\\townN1.ds1", "el .ds1")
	flag.Parse()

	m, err := d2mpq.FromFile(*mpqPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "MPQ:", err)
		os.Exit(1)
	}

	defer m.Close()

	raw, err := m.ReadFile(*ds1Name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "DS1:", err)
		os.Exit(1)
	}

	d, err := d2ds1.Unmarshal(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "DS1 parse:", err)
		os.Exit(1)
	}

	w, h := d.Width(), d.Height()
	fmt.Printf("DS1 %s w=%d h=%d celdas=%d\n", *ds1Name, w, h, w*h)

	type censo struct {
		registros, dibujables, ocultos, prop0 int
	}

	total := make([]censo, 2)

	celdasConPiso := 0
	celdasSoloNoDibujable := 0

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			conPiso, dibujable := false, false

			for i := 0; i < 2; i++ {
				l := d.GetFloor(i)
				if l == nil {
					continue
				}

				t := l.Tile(x, y)
				if t == nil || (t.Style == 0 && t.Sequence == 0) {
					continue
				}

				conPiso = true
				total[i].registros++

				switch {
				case t.Hidden():
					total[i].ocultos++
				case t.Prop1 == 0:
					total[i].prop0++
				default:
					total[i].dibujables++
					dibujable = true
				}
			}

			if conPiso {
				celdasConPiso++
			}

			if conPiso && !dibujable {
				celdasSoloNoDibujable++
			}
		}
	}

	for i := 0; i < 2; i++ {
		if d.GetFloor(i) == nil {
			continue
		}

		fmt.Printf("CAPA piso %d: registros=%d dibujables=%d Hidden=%d Prop1==0 %d\n",
			i, total[i].registros, total[i].dibujables, total[i].ocultos, total[i].prop0)
	}

	fmt.Printf("CELDAS con algun registro de piso=%d de %d (sin ninguno=%d)\n", celdasConPiso, w*h, w*h-celdasConPiso)
	fmt.Printf("CELDAS cuyo piso NO se puede dibujar (Hidden o Prop1==0)=%d\n", celdasSoloNoDibujable)
	fmt.Println("(el motor informa en el mismo archivo: cells=2337 nofloor=0 floors=2337 walls=394)")

	// lo que el MOTOR considera visible, medido con su propia funcion (d2maprenderer.TileRange)
	for _, v := range [][2]int{{512, 512}, {800, 600}} {
		vp := d2maprenderer.NewViewport(0, 0, v[0], v[1])
		sx, sy, ex, ey := d2maprenderer.TileRange(vp, v[0], v[1], w, h)
		fmt.Printf("MOTOR TileRange ventana=%dx%d celdas=%d rango=(%d,%d)-(%d,%d)\n",
			v[0], v[1], (ex-sx)*(ey-sy), sx, sy, ex, ey)
	}
}
