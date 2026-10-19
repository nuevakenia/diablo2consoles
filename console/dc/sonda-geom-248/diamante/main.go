// sonda-geom-248 (segunda parte): el TAMANO REAL del diamante de un tile de PISO.
//
// La primera parte midio Width/Height del DT1 (160x128 en este nivel), pero eso incluye el margen
// transparente de arriba. Lo que define la geometria es el DIAMANTE: la caja de los pixeles con indice
// distinto de 0 dentro del tile decodificado. Con el diamante se sabe el paso de la celda:
// en una proyeccion isometrica 2:1, la celda vecina esta a medio diamante de ancho y medio de alto.
//
// Imprime, para los primeros tiles de piso y los primeros de pared:
//   - Width/Height del DT1, min/max de los bloques, y la caja de los pixeles no vacios (x0,y0,x1,y1);
//   - el ancho/alto del diamante medido y el paso que implica.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

const barra = string(rune(92))

func rutaDT1(f string) string {
	f = strings.ToLower(f)
	f = strings.ReplaceAll(f, "c:", "")
	f = strings.ReplaceAll(f, ".tg1", ".dt1")

	if i := strings.LastIndex(f, "tiles"); i >= 0 {
		f = f[i+len("tiles"):]
	}

	f = strings.TrimPrefix(f, barra)
	f = strings.TrimPrefix(f, "/")

	return "data/global/tiles/" + f
}

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

	fmt.Printf("DS1 w=%d h=%d\n", d.Width(), d.Height())

	vistos := map[string]bool{}
	pisos, paredes := 0, 0

	for _, f := range d.Files {
		nombre := rutaDT1(f)

		crudo, err := m.ReadFile(strings.ReplaceAll(nombre, "/", barra))
		if err != nil {
			continue
		}

		dt1, err := d2dt1.LoadDT1(crudo)
		if err != nil {
			continue
		}

		for i := range dt1.Tiles {
			t := &dt1.Tiles[i]
			esPiso := t.Type == 0

			if esPiso && pisos >= 4 {
				continue
			}

			if !esPiso && paredes >= 6 {
				continue
			}

			ancho := int(t.Width)
			alto := int(t.Height)
			neg := alto < 0
			if neg {
				alto = -alto
			}

			minY, maxY := 0, 0
			for _, b := range t.Blocks {
				if int(b.Y) < minY {
					minY = int(b.Y)
				}

				if int(b.Y) > maxY {
					maxY = int(b.Y)
				}
			}

			idx := make([]byte, ancho*alto)
			offY := -minY
			if neg {
				offY = 0
			}

			d2dt1.DecodeTileGfxData(t.Blocks, &idx, int32(offY), int32(ancho))

			x0, y0, x1, y1 := ancho, alto, -1, -1
			noCero := 0

			for y := 0; y < alto; y++ {
				for x := 0; x < ancho; x++ {
					if idx[y*ancho+x] == 0 {
						continue
					}

					noCero++

					if x < x0 {
						x0 = x
					}

					if y < y0 {
						y0 = y
					}

					if x > x1 {
						x1 = x
					}

					if y > y1 {
						y1 = y
					}
				}
			}

			clase := "PISO"
			if esPiso {
				pisos++
			} else {
				clase = "PARED"
				paredes++
			}

			k := fmt.Sprintf("%s %dx%d", clase, ancho, alto)
			if vistos[k] {
				continue
			}

			vistos[k] = true

			fmt.Printf("%-6s style=%d seq=%d type=%d DT1=%dx%d bloquesY=%d..%d offY=%d\n",
				clase, t.Style, t.Sequence, t.Type, ancho, alto, minY, maxY, offY)
			fmt.Printf("       pixelesNoCero=%d caja=(%d,%d)-(%d,%d)  diamante=%dx%d\n",
				noCero, x0, y0, x1, y1, x1-x0+1, y1-y0+1)
			fmt.Printf("       fila del medio (y=%d): ", (y0+y1)/2)

			fila := idx[((y0+y1)/2)*ancho:]
			ini, fin := -1, -1

			for x := 0; x < ancho; x++ {
				if fila[x] != 0 {
					if ini < 0 {
						ini = x
					}

					fin = x
				}
			}

			fmt.Printf("x=%d..%d (ancho %d)\n", ini, fin, fin-ini+1)
		}
	}
}
