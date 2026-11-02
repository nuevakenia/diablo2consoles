// DC-002.122 -- SONDA DE DIRECCIONES: que cuadro del COF del heroe le toca a cada direccion del control.
//
// POR QUE EXISTE. El golpe tiene que dibujarse mirando al bicho: la direccion del COF sale de la cadena del motor
// (`d2vector.DirectionTo` -> `d2cof.Dir64ToCof`), y con 16 direcciones (el COF del heroe) el numero no es
// evidente. Esta sonda imprime esa cadena para las 8 direcciones de la cruceta y para el monstruo (8 direcciones),
// asi que los numeros del POC se pueden auditar desde el host sin abrir el emulador.
//
// Uso:  go run ./console/dc/sonda-dir-122            (heroe, 16 direcciones)
//
//	go run ./console/dc/sonda-dir-122 -dirs 8      (monstruo)
package main

import (
	"flag"
	"fmt"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"
)

func main() {
	dirs := flag.Int("dirs", 16, "direcciones del COF (16 heroe, 8 monstruo)")
	flag.Parse()

	origen := d2vector.NewVector(0, 0)

	// las 8 direcciones de la cruceta, en el orden en que las pide el control
	nombres := []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}
	dxs := []float64{0, 1, 1, 1, 0, -1, -1, -1}
	dys := []float64{-1, -1, 0, 1, 1, 1, 0, -1}

	fmt.Printf("dirs=%d\n", *dirs)

	for i := range nombres {
		destino := d2vector.NewVector(dxs[i], dys[i])
		ang := origen.DirectionTo(*destino)
		fmt.Printf("  offset=(%+.0f,%+.0f) %-2s dir64=%2d cof=%2d\n", dxs[i], dys[i], nombres[i], ang,
			d2cof.Dir64ToCof(ang, *dirs))
	}
}
