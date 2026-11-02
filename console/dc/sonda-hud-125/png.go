// DC-002.125 -- EL VOLCADO DE LOS GRAFICOS DEL HUD A PNG, CON LA PALETA DEL JUEGO.
//
// POR QUE EXISTE. La unica forma honesta de decir "este cuadro es el globo ROJO de la vida y este el AZUL del
// mana" es mirarlos pintados con la paleta que usa el juego y decir el COLOR MEDIO de sus pixeles. En la consola
// los dos globos se dibujan con la misma paleta (`sky/pal.dat`), asi que la diferencia de color es de los INDICES
// del cuadro, no de un tinte: si el cuadro 0 da azul y el 1 rojo, el cuadro 0 no es el de la vida.
//
// Y de paso deja el artefacto: los PNG de las piezas que el POC pega, para que cualquiera los mire sin abrir el
// MPQ.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2font"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

// paletaDe: los 768 B de un `pal.dat` a RGBA, con el 0 transparente como en el pincel. Se lee B,G,R como el
// motor (`d2dat.Load`: b=data[i*3+0], g=data[i*3+1], r=data[i*3+2]).
func paletaDe(mpqs map[string]*d2mpq.MPQ, ruta string) [256][4]uint8 {
	var pal [256][4]uint8

	_, datos := buscarEnLosMPQs(mpqs, ruta)
	if len(datos) < 768 {
		return pal
	}

	for i := 0; i < 256; i++ {
		pal[i] = [4]uint8{datos[i*3+2], datos[i*3+1], datos[i*3+0], 255}
	}

	return pal
}

// pngDeIndices: un cuadro de indices a PNG (el indice 0 es transparente, como en el motor).
func pngDeIndices(destino string, indices []byte, ancho, alto int, pal [256][4]uint8) error {
	img := image.NewRGBA(image.Rect(0, 0, ancho, alto))

	for y := 0; y < alto; y++ {
		for x := 0; x < ancho; x++ {
			i := y*ancho + x
			if i >= len(indices) || indices[i] == 0 {
				continue
			}

			p := pal[indices[i]]
			img.SetRGBA(x, y, color.RGBA{R: p[0], G: p[1], B: p[2], A: 255})
		}
	}

	f, err := os.Create(destino)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	return png.Encode(f, img)
}

// colorMedio: el promedio de R, G y B de los pixeles NO vacios de un cuadro. Es el numero con el que se decide
// cual globo es la vida y cual el mana.
func colorMedio(indices []byte, pal [256][4]uint8) (int, int, int, int) {
	var r, g, b, n int

	for _, v := range indices {
		if v == 0 {
			continue
		}

		p := pal[v]
		r += int(p[0])
		g += int(p[1])
		b += int(p[2])
		n++
	}

	if n == 0 {
		return 0, 0, 0, 0
	}

	return r / n, g / n, b / n, n
}

// volcarPNG: las piezas del HUD que el POC pega, a PNG, con su color medio.
func volcarPNG(mpqs map[string]*d2mpq.MPQ, dir string, clase string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Printf("   no puedo crear %s: %v\n", dir, err)

		return
	}

	palCielo := paletaDe(mpqs, `data\global\palette\sky\pal.dat`)
	palUni := paletaDe(mpqs, `data\global\palette\units\pal.dat`)

	fmt.Printf("\n== 6. LOS GRAFICOS DEL HUD A PNG (paleta del juego) en %s\n", dir)

	piezas := []struct {
		nombre  string
		ruta    string
		cuadros []int
		pal     [256][4]uint8
		etiqueta string
	}{
		{"panel-ctrlpnl7", `data\global\ui\PANEL\ctrlpnl7.DC6`, []int{0, 1, 2, 3, 4, 5}, palCielo, "cielo"},
		{"globo-hlthmana", `data\global\ui\PANEL\hlthmana.DC6`, []int{0, 1, 2, 3}, palCielo, "cielo"},
		{"tapa-overlap", `data\global\ui\PANEL\overlap.DC6`, []int{0, 1}, palCielo, "cielo"},
		{"cinturon-popbelt", `data\global\ui\PANEL\ctrlpnl_popbelt.DC6`, []int{0}, palUni, "unidades"},
		{"pocion-invhp1", `data\global\items\invhp1.dc6`, []int{0}, palUni, "unidades"},
		{"pocion-invmp1", `data\global\items\invmp1.dc6`, []int{0}, palUni, "unidades"},
	}

	for _, p := range piezas {
		_, datos := buscarEnLosMPQs(mpqs, p.ruta)
		if len(datos) == 0 {
			fmt.Printf("   %-20s NO ESTA (%s)\n", p.nombre, p.ruta)

			continue
		}

		dc, err := d2dc6.Load(datos)
		if err != nil {
			fmt.Printf("   %-20s NO PARSEA: %v\n", p.nombre, err)

			continue
		}

		for _, c := range p.cuadros {
			if c >= len(dc.Frames) {
				continue
			}

			fr := dc.Frames[c]
			indices := dc.DecodeFrame(c)
			r, g, b, n := colorMedio(indices, p.pal)

			destino := filepath.Join(dir, fmt.Sprintf("%s-c%d.png", p.nombre, c))
			if err := pngDeIndices(destino, indices, int(fr.Width), int(fr.Height), p.pal); err != nil {
				fmt.Printf("   %-20s cuadro %d no se escribe: %v\n", p.nombre, c, err)

				continue
			}

			fmt.Printf("   %-20s cuadro %d  caja=%dx%d  paleta=%-8s pixeles=%4d  colorMedio RGB=%3d,%3d,%3d\n",
				p.nombre, c, fr.Width, fr.Height, p.etiqueta, n, r, g, b)
		}
	}

	volcarLaZonaAUnaFuente(mpqs, dir, palUni, clase)
}

// volcarLaZonaAUnaFuente: el nombre de la zona compuesto con `font16`, igual que lo pega el POC (con el AVANCE de
// cada letra, no con el ancho de la caja: por eso mide 141 px y no 224), a PNG.
func volcarLaZonaAUnaFuente(mpqs map[string]*d2mpq.MPQ, dir string, pal [256][4]uint8, clase string) {
	frase := "Rogue Encampment"

	_, tabla := buscarEnLosMPQs(mpqs, `data\local\font\latin\font16.tbl`)
	_, sprite := buscarEnLosMPQs(mpqs, `data\local\font\latin\font16.DC6`)

	if len(tabla) == 0 || len(sprite) == 0 {
		fmt.Println("   la fuente 16 no esta")

		return
	}

	f, err := d2font.Load(tabla)
	if err != nil {
		fmt.Printf("   la tabla de la fuente no parsea: %v\n", err)

		return
	}

	dc, err := d2dc6.Load(sprite)
	if err != nil {
		fmt.Printf("   el sprite de la fuente no parsea: %v\n", err)

		return
	}

	ancho, alto := 0, 0

	for i := 0; i < len(frase); i++ {
		if g, ok := f.Glyphs[rune(frase[i])]; ok {
			ancho += g.Width()
			if g.Height() > alto {
				alto = g.Height()
			}
		}
	}

	lienzo := make([]byte, ancho*alto)
	x := 0

	for i := 0; i < len(frase); i++ {
		g, ok := f.Glyphs[rune(frase[i])]
		if !ok {
			continue
		}

		cuadro := g.FrameIndex()
		if cuadro < 0 || cuadro >= len(dc.Frames) {
			continue
		}

		fr := dc.Frames[cuadro]
		indices := dc.DecodeFrame(cuadro)

		for y := 0; y < int(fr.Height) && y < alto; y++ {
			for gx := 0; gx < int(fr.Width); gx++ {
				if x+gx >= ancho {
					break
				}

				lienzo[y*ancho+x+gx] = indices[y*int(fr.Width)+gx]
			}
		}

		x += g.Width()
	}

	destino := filepath.Join(dir, "zona-font16.png")
	if err := pngDeIndices(destino, lienzo, ancho, alto, pal); err != nil {
		fmt.Printf("   el nombre de la zona no se escribe: %v\n", err)

		return
	}

	r, g, b, n := colorMedio(lienzo, pal)
	fmt.Printf("   %-20s %q  %dx%d px  pixeles=%d  colorMedio RGB=%d,%d,%d  (avance de la tabla; 16x14 serian %d)\n",
		"zona-font16", frase, ancho, alto, n, r, g, b, len(frase)*14)
}
