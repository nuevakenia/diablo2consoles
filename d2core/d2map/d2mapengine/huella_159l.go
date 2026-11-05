package d2mapengine

// DC-002.159l -- LA HUELLA DEL MAPA COMPUESTO, COMO INSTRUMENTO DEL ARRANQUE.
//
// POR QUE EXISTE. DC-002.159k midio el defecto con una huella (FNV-1a de 32 bits sobre `Prop1`, `Style` y
// `Sequence` del primer piso y la primera pared de cada casilla) y la dejo en instrumentos que se revirtieron:
// la unica forma de comparar "lo que compuso la consola" con "lo que compone el host" era volver a aplicarlos a
// mano. Esto es lo mismo, pero en el arbol y sin tocar el camino: la llama el arranque de la consola en las
// etapas del mapa y la prueba del host (`huella_159l_test.go`) con el MISMO criterio.
//
// LOS NUMEROS SON LOS DE 159k, y estan comprobados: con el pueblo del MPQ esta funcion da 2.337 pisos,
// 338 paredes y FNV 2704422391 (= -1590544905 con signo), que son exactamente los tres numeros que la evidencia
// de DC-002.159k anota como "host, casillas-puestas". La cuenta se rehizo contra el `.ds1` real y se eligio la
// unica combinacion que reproduce ese FNV bit a bit (el orden y el filtro importan; ver el comentario del
// cuerpo).
//
// LO QUE MIDE, DICHO COMO ES: cuantas casillas tienen un piso de verdad y cuantas una pared visible, y un FNV
// de (Prop1, Style, Sequence) de esos primeros registros. No es un checksum del mapa entero (no mira
// `RandomIndex` ni los subtiles, que los pone `PrepareTile`); es la identidad del contenido que viene del
// `.ds1`, que es justo lo que se perdia.

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"

// HuellaDelMapa159l: la huella del contenido compuesto del nivel. `pisos` y `paredes` son casillas (no
// registros), `fnv` es el FNV-1a de 32 bits de (piso, pared) por casilla en orden de filas.
//
// EL CENSO ES EL MISMO DE `censoDelPueblo` (la pantalla del pueblo, DC-002.155): un piso cuenta si su primer
// registro no esta oculto y tiene `Prop1 != 0`, y una pared igual (las ocultas no tienen grafico y el motor las
// saltea). El FNV, en cambio, mezcla el primer registro de las dos rebanadas de CADA casilla sin filtrar: es la
// unica forma que reproduce el 2704422391 de DC-002.159k, y como se aplica igual en el host y en la consola, la
// comparacion sigue siendo valida (y el test la fija contra ese numero).
func (m *MapEngine) HuellaDelMapa159l() (pisos, paredes int, fnv uint32) {
	const (
		fnvBase  uint32 = 2166136261
		fnvPrime uint32 = 16777619
	)

	fnv = fnvBase

	mezclar := func(t *d2ds1.Tile) {
		fnv ^= uint32(t.Prop1)
		fnv *= fnvPrime
		fnv ^= uint32(t.Style)
		fnv *= fnvPrime
		fnv ^= uint32(t.Sequence)
		fnv *= fnvPrime
	}

	if m == nil {
		return 0, 0, fnv
	}

	for i := range m.tiles {
		c := &m.tiles[i].Components

		if len(c.Floors) > 0 {
			if c.Floors[0].Prop1 != 0 && !c.Floors[0].Hidden() {
				pisos++
			}

			mezclar(&c.Floors[0])
		}

		if len(c.Walls) > 0 {
			if c.Walls[0].Prop1 != 0 && !c.Walls[0].Hidden() {
				paredes++
			}

			mezclar(&c.Walls[0])
		}
	}

	return pisos, paredes, fnv
}
