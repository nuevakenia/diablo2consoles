// Comparador del host: corre el MISMO codigo del motor (d2dc6.Load + el resumen) que corre en la
// consola, sobre los MISMOS bytes horneados del MPQ, y escribe el resumen por stdout.
//
// Uso:  go run ./console/dc/motorcmp -dc6 <dir_de_build>/<name>.dc6 [-pal <name>.pal] [-png ref.png]
//
// La comparacion contra la consola la hace scripts/console/compare-motor-dc6.sh: las lineas
// "MOTOR-DC6 frame ..." tienen que salir identicas de los dos lados.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/motorresumen"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dat"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
)

func main() {
	dc6Path := flag.String("dc6", "", "DC6 crudo horneado del MPQ")
	palPath := flag.String("pal", "", "pal.dat crudo horneado del MPQ (si esta, tambien sale el resumen RGBA)")
	pngPath := flag.String("png", "", "si esta, escribe los RGBA del frame -frame como PNG (la referencia con la que se compara la captura de la consola)")
	frameN := flag.Int("frame", 0, "que frame se escribe con -png")
	flag.Parse()

	if *dc6Path == "" {
		fatal("falta -dc6")
	}

	raw, err := os.ReadFile(*dc6Path)
	if err != nil {
		fatal("leyendo %s: %v", *dc6Path, err)
	}

	d, err := d2dc6.Load(raw)
	if err != nil {
		fatal("parseando el DC6: %v", err)
	}

	fmt.Printf("MOTOR-DC6 origen=host bytes=%d\n", len(raw))

	for _, l := range motorresumen.Lineas(d) {
		fmt.Println(l)
	}

	// DC-002.9: el color. La paleta la decodifica el mismo d2dat.Load del motor y los RGBA salen del
	// mismo d2util.ImgIndexToRGBA: lo unico que cambia del lado de la consola es el compilador.
	if *palPath != "" {
		palRaw, err := os.ReadFile(*palPath)
		if err != nil {
			fatal("leyendo %s: %v", *palPath, err)
		}

		pal, err := d2dat.Load(palRaw)
		if err != nil {
			fatal("parseando la paleta: %v", err)
		}

		fmt.Printf("MOTOR-PVR origen=host palbytes=%d\n", len(palRaw))

		for _, l := range motorresumen.LineasRGBA(d, pal) {
			fmt.Println(l)
		}

		// la referencia en PNG: el mismo camino que la consola sube al PVR (DecodeFrame +
		// ImgIndexToRGBA), para que "se ve el logo" sea un numero y no una impresion. La captura del
		// emulador se compara contra este PNG pixel a pixel (scripts/console/compare-capture.py).
		if *pngPath != "" {
			escribirPNG(d, pal, *frameN, *pngPath)
		}
	}

	fmt.Println("MOTOR-DC6 fin")
}

// escribirPNG deja el frame pedido como PNG RGBA. Usa image/png de la biblioteca estandar REAL: esto
// corre en el host (x86-64), no en la consola.
func escribirPNG(d *d2dc6.DC6, pal d2interface.Palette, n int, ruta string) {
	if n < 0 || n >= len(d.Frames) || d.Frames[n] == nil {
		fatal("el frame %d no existe en el DC6", n)
	}

	idx := d.DecodeFrame(n)
	if idx == nil {
		fatal("el frame %d no decodifico", n)
	}

	f := d.Frames[n]
	rgba := d2util.ImgIndexToRGBA(idx, pal)

	img := image.NewRGBA(image.Rect(0, 0, int(f.Width), int(f.Height)))
	copy(img.Pix, rgba)

	out, err := os.Create(ruta)
	if err != nil {
		fatal("creando %s: %v", ruta, err)
	}
	defer out.Close()

	if err := png.Encode(out, img); err != nil {
		fatal("escribiendo el PNG: %v", err)
	}

	fmt.Printf("MOTOR-PVR png=%s frame=%d %dx%d\n", ruta, n, f.Width, f.Height)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
