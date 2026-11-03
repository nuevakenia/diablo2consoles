// SONDA DC-002.142 -- LA TINTA DE LA FUENTE: que indices trae un glifo de font16 despues de decodificar el DC6.
//
// POR QUE EXISTE. En la demo DC-002.140 el panel de los NPCs sale NEGRO con su borde: el texto esta en el log y
// la caja se pinta, pero no hay una letra en pantalla. El pincel (`RenderIndicesTintado`) SALTA el indice 0
// (transparente). Esta sonda contesta, con el archivo del juego delante, si los pixeles de "tinta" del glifo son
// justamente el indice 0 (y entonces el pincel los tira), cuantos pixeles no vacios tiene cada glifo, y que color
// les toca en la paleta de las unidades.
//
// Uso:
//
//	go run ./console/dc/sonda-tinta-142
//	go run ./console/dc/sonda-tinta-142 -letras "Akara"
package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sort"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2font"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

const (
	rutaFuente  = `data\local\font\latin\font16`
	rutaPalUni  = `data\global\palette\units\pal.dat`
	rutaPalAct1 = `data\global\palette\act1\pal.dat`
)

func leer(m *d2mpq.MPQ, ruta string) []byte {
	if m == nil {
		return nil
	}

	f, err := m.ReadFile(ruta)
	if err != nil {
		return nil
	}

	b := make([]byte, len(f))
	copy(b, f)

	return b
}

func paleta(b []byte) [256][3]uint8 {
	var p [256][3]uint8

	for i := 0; i < 256 && i*3+2 < len(b); i++ {
		p[i] = [3]uint8{b[i*3], b[i*3+1], b[i*3+2]}
	}

	return p
}

func main() {
	dir := flag.String("dir", "", "carpeta del juego (por omision, la instalacion de wine)")
	letras := flag.String("letras", "AkaraTalk", "letras a medir")
	flag.Parse()

	base := *dir
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".wine-d2", "drive_c", "Games", "Diablo II")
	}

	datos, err := d2mpq.FromFile(filepath.Join(base, "d2data.mpq"))
	if err != nil {
		fmt.Println("d2data.mpq NO ABRE:", err)

		return
	}

	tbl := leer(datos, rutaFuente+`.tbl`)
	dc6 := leer(datos, rutaFuente+`.DC6`)

	fmt.Printf("== font16: tabla=%d B  sprite=%d B\n", len(tbl), len(dc6))
	fmt.Printf("   crc32 tabla=%d sprite=%d\n", int(crc32.ChecksumIEEE(tbl)), int(crc32.ChecksumIEEE(dc6)))

	f, err := d2font.Load(tbl)
	if err != nil {
		fmt.Println("la tabla de la fuente NO parsea:", err)

		return
	}

	dc, err := d2dc6.Load(dc6)
	if err != nil {
		fmt.Println("el sprite de la fuente NO parsea:", err)

		return
	}

	pal := paleta(leer(datos, rutaPalUni))
	fmt.Printf("   cuadros del DC6=%d  glifos de la tabla=%d\n", len(dc.Frames), len(f.Glyphs))

	// EL COSTO DE TENER LA FUENTE ENTERA EN RAM: la suma de las cajas de los 256 cuadros (lo que iria al monton de C
	// si se decodificaran todos, que es la alternativa a decodificar solo las letras del texto).
	total, minW, maxW, maxH := 0, 999, 0, 0

	for i := range dc.Frames {
		w, h := int(dc.Frames[i].Width), int(dc.Frames[i].Height)
		total += w * h

		if w < minW {
			minW = w
		}

		if w > maxW {
			maxW = w
		}

		if h > maxH {
			maxH = h
		}
	}

	fmt.Printf("   fuente entera: 256 cuadros suman %d B de indices (ancho %d..%d alto max %d)\n", total, minW, maxW, maxH)

	for _, letra := range *letras {
		g, ok := f.Glyphs[rune(letra)]
		if !ok {
			fmt.Printf("   %q SIN GLIFO EN LA TABLA\n", string(letra))

			continue
		}

		idx := g.FrameIndex()
		if idx < 0 || idx >= len(dc.Frames) {
			fmt.Printf("   %q cuadro=%d FUERA DEL DC6 (cuadros=%d)\n", string(letra), idx, len(dc.Frames))

			continue
		}

		fr := dc.Frames[idx]
		px := dc.DecodeFrame(idx)

		hist := map[byte]int{}
		vacios, llenos := 0, 0

		for _, v := range px {
			hist[v]++

			if v == 0 {
				vacios++
			} else {
				llenos++
			}
		}

		claves := make([]int, 0, len(hist))
		for k := range hist {
			claves = append(claves, int(k))
		}

		sort.Ints(claves)

		fmt.Printf("   %q cuadro=%d caja=%dx%d paso=%d pixeles=%d vacios=%d llenos=%d\n",
			string(letra), idx, fr.Width, fr.Height, g.Width(), len(px), vacios, llenos)

		n := 0

		for _, k := range claves {
			if n >= 6 {
				fmt.Printf("        ... %d indices distintos\n", len(claves))

				break
			}

			fmt.Printf("        indice %3d x%-4d color=(%d,%d,%d)\n", k, hist[byte(k)], pal[k][0], pal[k][1], pal[k][2])
			n++
		}
	}
}
