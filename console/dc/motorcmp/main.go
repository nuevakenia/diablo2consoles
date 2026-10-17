// Comparador del host: corre el MISMO codigo del motor (d2dc6.Load + el resumen) que corre en la
// consola, sobre los MISMOS bytes horneados del MPQ, y escribe el resumen por stdout.
//
// Uso:  go run ./console/dc/motorcmp -dc6 <dir_de_build>/<name>.dc6
//
// La comparacion contra la consola la hace scripts/console/compare-motor-dc6.sh: las lineas
// "MOTOR-DC6 frame ..." tienen que salir identicas de los dos lados.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/motorresumen"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dat"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
)

func main() {
	dc6Path := flag.String("dc6", "", "DC6 crudo horneado del MPQ")
	palPath := flag.String("pal", "", "pal.dat crudo horneado del MPQ (si esta, tambien sale el resumen RGBA)")
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
	}

	fmt.Println("MOTOR-DC6 fin")
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
