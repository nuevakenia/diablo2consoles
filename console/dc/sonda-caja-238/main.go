// sonda-caja-238: mide la CAJA de pixeles NO VACIOS de cada tile de un DT1 (el decodificador del motor).
//
// Sirve para contestar una pregunta de ESCALA que decide como se pega el personaje sobre el mapa: el
// compositor del nivel (nivel.go) avanza 32 px en x y 16 en y por celda del DS1; el renderer del motor
// (viewport.go) avanza 80 x 40 por unidad de mundo. Si el contenido del tile de piso mide 32x16, la celda
// del DS1 es de 32x16 (nivel.go tiene razon); si mide 80x40, tiene razon el renderer.
//
// Uso: go run ./console/dc/sonda-caja-238 -mpq <d2data.mpq> -dt1 'data\global\tiles\ACT1\TOWN\floor.dt1'
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

func main() {
	mpqPath := flag.String("mpq", "", "")
	dt1Name := flag.String("dt1", `data\global\tiles\ACT1\TOWN\floor.dt1`, "")
	limite := flag.Int("limite", 12, "cuantos tiles medir")
	flag.Parse()

	mpq, err := d2mpq.FromFile(*mpqPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer mpq.Close()

	raw, err := mpq.ReadFile(*dt1Name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	dt1, err := d2dt1.LoadDT1(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("SONDA238c DT1 %s tiles=%d\n", *dt1Name, len(dt1.Tiles))

	medidos := 0

	for i := range dt1.Tiles {
		t := &dt1.Tiles[i]
		alto := int(t.Height)
		if alto < 0 {
			alto = -alto
		}

		if alto <= 0 || int(t.Width) <= 0 || alto > 512 {
			continue
		}

		minY := 0
		for _, b := range t.Blocks {
			if int(b.Y) < minY {
				minY = int(b.Y)
			}
		}

		ancho := int(t.Width)
		lienzo := make([]byte, ancho*alto)
		d2dt1.DecodeTileGfxData(t.Blocks, &lienzo, int32(-minY), int32(ancho))

		x0, y0, x1, y1 := ancho, alto, -1, -1

		for y := 0; y < alto; y++ {
			for x := 0; x < ancho; x++ {
				if lienzo[y*ancho+x] == 0 {
					continue
				}

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

		tipo := "?"

		fmt.Printf("SONDA238c tile %d estilo=%d sec=%d tipo=%d lienzo=%dx%d cajaNOvacia=(%d,%d)-(%d,%d) = %dx%d\n",
			i, t.Style, t.Sequence, t.Type, ancho, alto, x0, y0, x1, y1, x1-x0+1, y1-y0+1)

		_ = tipo
		medidos++
		if medidos >= *limite {
			break
		}
	}

	_ = strings.ToLower
}
