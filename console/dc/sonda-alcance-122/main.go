// DC-002.122 -- SONDA DEL ALCANCE: la tabla de distancias del exe, impresa desde el host.
//
// POR QUE EXISTE. El criterio de la tarjeta es que un golpe de la celda de al lado llegue y uno de lejos no. Ese
// numero sale de `d2skill.TableDistanceSized` (= `UNIT_GetTableDistance` 0x642980) y del alcance verificado de 7
// subtiles. Esta sonda imprime la tabla para que los numeros del POC se puedan auditar desde el host, sin abrir el
// emulador, con la MISMA funcion que usa `console/dc/poc/juego-122/alcance_122.go`.
//
// Uso:  go run ./console/dc/sonda-alcance-122
package main

import (
	"fmt"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
)

func main() {
	// el alcance VERIFICADO del exe (d2core/d2monsters/director.go, meleeInRange) y el tamano del bicho por
	// omision de monstats2 (`d2skill.UnitSizeOf` cuando el objetivo no declara SizeX)
	const (
		alcance     = 7
		tamanoBicho = 1
	)

	fmt.Printf("alcance=%d subtiles (meleeInRange, verificado del exe)\n", alcance)
	fmt.Printf("tamanoHeroe=%d (HeroUnitSize, UNIT_GetSizeX 0x6207a0) tamanoBicho=%d (SizeX por omision)\n",
		d2skill.HeroUnitSize, tamanoBicho)

	separaciones := []struct {
		celdas float64
		dx, dy int
		nota   string
	}{
		{0, 0, 0, "la misma celda (la colision de DC-002.121 lo prohibe)"},
		{1, 5, 0, "la celda de al lado, en la misma fila (CELDA_P1=\"0 1\")"},
		{1, 0, 5, "la celda de al lado, en la misma columna"},
		{1.41, 5, 5, "la celda de al lado, en diagonal"},
		{2, 10, 0, "a dos celdas (CELDA_P1=\"-2 1\")"},
		{2.83, 10, 10, "a dos celdas en diagonal"},
	}

	for _, s := range separaciones {
		d := d2skill.TableDistanceSized(s.dx, s.dy, d2skill.HeroUnitSize, tamanoBicho)
		fmt.Printf("  dx=%2d dy=%2d (%4.2f celdas) distanciaTabla=%2d alcanza=%-5v  %s\n",
			s.dx, s.dy, s.celdas, d, d <= alcance, s.nota)
	}
}
