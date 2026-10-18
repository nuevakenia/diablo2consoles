// verif-dc6-235: EL LADO DEL HOST del primer hito visible (DC-002.35).
//
// Hace, con los MISMOS bytes y el MISMO codigo del motor que la consola, los numeros que el POC
// imprime por la consola serie:
//   - el CRC-32 (IEEE, el de hash/crc32 y el de zlib) del DC6 crudo tal como sale del MPQ;
//   - el CRC-32 de los indices COMPUESTOS (todos los frames pegados en un lienzo, con la misma
//     composicion que hace el horneador del host y el POC);
//   - el CRC-32 de la paleta ARGB1555 (256 colores, dos bytes cada uno, el bajo primero).
// Y escribe un PNG de referencia con los indices + la paleta, que es contra lo que se compara la
// captura del emulador.
//
// POR QUE ESTE PROGRAMA Y NO `console/dc/bake`: bake lee el DC6 y la paleta DEL MISMO MPQ, y el DC6
// de este hito (waitingroombkgd.dc6) esta en patch_d2.mpq mientras su paleta (pal.pl2) esta en
// d2data.mpq. Aca los dos entran como ARCHIVOS (los que ya extrajo bake-dc6), que es ademas lo que
// hace la consola: leer bytes de un disco.
//
// Uso:  go run ./console/dc/verif-dc6-235 -dc6 <archivo> -pal <pl2> -out <dir> -name <base>
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2pl2"
)

func main() {
	dc6Path := flag.String("dc6", "", "archivo DC6 crudo (el que extrae bake-dc6 del MPQ)")
	palPath := flag.String("pal", "", "archivo PL2 crudo")
	outDir := flag.String("out", ".", "directorio donde dejar el PNG de referencia")
	name := flag.String("name", "asset", "nombre base")
	flag.Parse()

	if *dc6Path == "" || *palPath == "" {
		fatal("faltan -dc6 y/o -pal")
	}

	raw, err := os.ReadFile(*dc6Path)
	if err != nil {
		fatal("leyendo %s: %v", *dc6Path, err)
	}

	fmt.Printf("HOST DC6 bytes=%d crc32=%d\n", len(raw), crc32.ChecksumIEEE(raw))

	// DC-002.35, criterio 1: los mismos bytes y el CRC-32 de cada trozo de 64 KiB que imprime la consola.
	// El archivo esta COMPRIMIDO en el MPQ (implode de PKWARE), asi que esta es la referencia de lo que
	// tiene que salir de la DESCOMPRESION: con los trozos se ve en cual se separa la consola.
	fmt.Printf("HOST DC6 primeros 16 bytes: %s\n", hex16(raw))

	for i := 0; i < len(raw); i += trozoBytes {
		fin := i + trozoBytes
		if fin > len(raw) {
			fin = len(raw)
		}

		fmt.Printf("HOST DC6 trozo %d desde=%d bytes=%d crc32=%d\n",
			i/trozoBytes, i, fin-i, crc32.ChecksumIEEE(raw[i:fin]))
	}

	dc, err := d2dc6.Load(raw)
	if err != nil {
		fatal("parseando DC6: %v", err)
	}

	fmt.Printf("HOST DC6 frames=%d\n", len(dc.Frames))

	idx, w, h, pegados := componer(dc)
	fmt.Printf("HOST DC6 compuesto w=%d h=%d frames pegados=%d indices=%d crc32=%d\n",
		w, h, pegados, len(idx), crc32.ChecksumIEEE(idx))

	pb, err := os.ReadFile(*palPath)
	if err != nil {
		fatal("leyendo %s: %v", *palPath, err)
	}

	pal, err := d2pl2.Load(pb)
	if err != nil {
		fatal("parseando PL2: %v", err)
	}

	colors := pal.BasePalette.Colors

	pvrPal := make([]uint16, 256)

	buf := make([]byte, 512)
	for i := 0; i < 256; i++ {
		c := colors[i]

		alpha := uint16(1)
		if i == 0 {
			alpha = 0
		}

		pvrPal[i] = alpha<<15 | uint16(c.R>>3)<<10 | uint16(c.G>>3)<<5 | uint16(c.B>>3)
		binary.LittleEndian.PutUint16(buf[i*2:], pvrPal[i])
	}

	fmt.Printf("HOST PALETA bytes=%d colores=%d crc32=%d\n", len(pb), len(colors), crc32.ChecksumIEEE(buf))

	// el PNG de referencia, con la MISMA paleta y los MISMOS indices que se suben a la consola
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := idx[y*w+x]
			if i == 0 {
				continue // transparente
			}

			c := colors[i]
			img.SetNRGBA(x, y, color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255})
		}
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatal("creando %s: %v", *outDir, err)
	}

	pngPath := filepath.Join(*outDir, *name+"-referencia.png")
	f, err := os.Create(pngPath)
	if err != nil {
		fatal("creando el PNG: %v", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		fatal("escribiendo el PNG: %v", err)
	}

	// los pixeles que NO son transparentes: el numero del criterio 2 del lado del host
	noFondo := 0
	for _, v := range idx {
		if v != 0 {
			noFondo++
		}
	}

	fmt.Printf("HOST REFERENCIA png=%s pixelesNoTransparentes=%d de %d\n", pngPath, noFondo, len(idx))
}

// trozoBytes: el tamano de trozo con el que se comparan las dos puntas (64 KiB, el mismo de la consola).
const trozoBytes = 65536

// hex16: los primeros 16 bytes en hexa, el mismo formato que imprime el POC de la consola.
func hex16(datos []byte) string {
	const hexa = "0123456789abcdef"

	n := 16
	if len(datos) < n {
		n = len(datos)
	}

	salida := make([]byte, 0, n*3)

	for i := 0; i < n; i++ {
		if i > 0 {
			salida = append(salida, ' ')
		}

		salida = append(salida, hexa[datos[i]>>4], hexa[datos[i]&0x0f])
	}

	return string(salida)
}

// componer: la MISMA composicion que el horneador del host y el POC de la consola.
func componer(dc *d2dc6.DC6) ([]byte, int, int, int) {
	minX, minY := math.MaxInt32, math.MaxInt32
	maxX, maxY := math.MinInt32, math.MinInt32

	for _, f := range dc.Frames {
		if f == nil {
			continue
		}

		x0, y0 := int(f.OffsetX), int(f.OffsetY)
		x1, y1 := x0+int(f.Width), y0+int(f.Height)

		if x0 < minX {
			minX = x0
		}

		if y0 < minY {
			minY = y0
		}

		if x1 > maxX {
			maxX = x1
		}

		if y1 > maxY {
			maxY = y1
		}
	}

	if minX > maxX || minY > maxY {
		return nil, 0, 0, 0
	}

	w, h := maxX-minX, maxY-minY
	topDown := make([]byte, w*h)
	pegados := 0

	for i, f := range dc.Frames {
		if f == nil {
			continue
		}

		fi := dc.DecodeFrame(i)
		if fi == nil {
			continue
		}

		fw, fh := int(f.Width), int(f.Height)
		ox, oy := int(f.OffsetX)-minX, int(f.OffsetY)-minY

		for y := 0; y < fh; y++ {
			for x := 0; x < fw; x++ {
				v := fi[y*fw+x]
				if v == 0 {
					continue
				}

				cx, cy := ox+x, oy+y
				if cx < 0 || cy < 0 || cx >= w || cy >= h {
					continue
				}

				topDown[cy*w+cx] = v
			}
		}

		pegados++
	}

	return topDown, w, h, pegados
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
