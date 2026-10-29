package main

// DC-002.49b: LA DIRECCION DEL MOVIMIENTO.
//
// El motor no inventa la direccion de una entidad: la saca de la cadena
//   (dx,dy) en celdas -> d2vector.Vector.DirectionTo (64 direcciones) -> d2cof.Dir64ToCof (las del COF)
// que es la MISMA que usa para pintar (`d2core/d2asset/composite.go:70`). Aca se hace exactamente eso, y se
// midio que los 8 estados de la cruceta dan 8 direcciones distintas del COF: [0 1 4 7 8 9 12 15]
// (verif-personaje-238 -censo-control, evidencia dc-002.49b, seccion 3).
//
// Costo medido de tener las 8 compuestas (host, verif-personaje-238 -censo-dirs):
//   TN 246.768 B de indices, TW 148.720 B; el giro despues es elegir otro ciclo: 0 ms de cuadro.

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"
)

// giroSordo: EL MUTANTE del criterio 4. Con esto en true la direccion se queda en la 0 (la que dibujaba
// DC-002.38) mientras el control se sigue leyendo y el personaje se sigue moviendo: las capturas con distinta
// tecla tienen que ser IDENTICAS. Se enciende con GIRO_SORDO=1 en el build (sed), igual que cicloSordo.
var giroSordo = false

// dirActual: la direccion del COF que el personaje esta mirando. Arranca en la del POC de DC-002.38.
var dirActual = dirPersonaje

// dirsCOFDelPersonaje: cuantas direcciones tiene el COF del heroe. Se MIDE al cargarlo (la amazona: 16); el
// valor de aca es el que usa el control antes de que el COF este cargado.
var dirsCOFDelPersonaje = 16

// direccionDelControl: el estado de la cruceta (cada eje en -1,0,1) -> la direccion del COF, con las funciones
// del motor. No hay tabla propia: si el motor cambia de criterio, esto lo sigue.
func direccionDelControl(ejeX, ejeY float64) int {
	dx, dy := DireccionDelPiso(ejeX, ejeY)

	origen := d2vector.NewVector(0, 0)
	destino := d2vector.NewVector(dx, dy)

	return d2cof.Dir64ToCof(origen.DirectionTo(*destino), dirsCOFDelPersonaje)
}

// direccionesDelControl: las direcciones del COF DISTINTAS que la cruceta puede pedir (8 de los 9 estados: el
// centro no pide ninguna). Es el conjunto que hay que tener compuesto para que el personaje gire sin pagar nada
// en el cuadro.
func direccionesDelControl() []int {
	vistas := make(map[int]bool, 8)
	dirs := make([]int, 0, 8)

	for ejeX := -1; ejeX <= 1; ejeX++ {
		for ejeY := -1; ejeY <= 1; ejeY++ {
			if ejeX == 0 && ejeY == 0 {
				continue
			}

			d := direccionDelControl(float64(ejeX), float64(ejeY))
			if vistas[d] {
				continue
			}

			vistas[d] = true
			dirs = append(dirs, d)
		}
	}

	return dirs
}
