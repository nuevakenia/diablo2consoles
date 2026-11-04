package d2automap

// DC-002.146b -- LA SIEMBRA CON LA FILA MAS CERCANA, y el numero que la sostiene.
//
// El automapa del Blood Moor generado por el DRLG se llenaba solo en parte porque sus pisos son
// `act1/town/floor.dt1` estilo 0 y 5 con secuencia 0, y las filas del "1 Wilderness" cubren el estilo 0 para
// las secuencias 1..47 y no tienen NINGUNA fila para el estilo 5. `LookupCercana`/`CelCercana` y
// `RevealTilesConRespaldo` son el respaldo: la fila del mismo tipo con el estilo y la secuencia mas cercanos.
// Estos tests fijan la regla (y que el camino del motor, `RevealTiles`, NO cambia).

import "testing"

// tablaDePrueba: dos filas de piso (estilo 0 para 1..47, estilo 4 para 23..26) y una de pared (tipo 1).
func tablaDePrueba() *Table {
	return newTable([]Row{
		{Level: 2, Tile: 0, Style: 0, Start: 1, End: 47, Cels: []int{0, 1, 2, 3}},
		{Level: 2, Tile: 0, Style: 4, Start: 23, End: 26, Cels: []int{79}},
		{Level: 2, Tile: 1, Style: 0, Start: 0, End: 0, Cels: []int{21}},
	})
}

func TestLookupCercana(t *testing.T) {
	tabla := tablaDePrueba()

	// 1. la fila exacta se devuelve tal cual (la misma que Lookup)
	if r := tabla.LookupCercana(2, 0, 0, 5); r == nil || r.Style != 0 {
		t.Fatalf("(fl, estilo 0, seq 5) tiene fila exacta y LookupCercana devolvio %+v", r)
	}

	// 2. mismo estilo, secuencia fuera del rango: gana la fila de ese estilo
	if r := tabla.LookupCercana(2, 0, 0, 0); r == nil || r.Style != 0 || r.Start != 1 {
		t.Fatalf("(fl, estilo 0, seq 0): queria la fila del estilo 0 (1..47), devolvio %+v", r)
	}

	// 3. estilo sin fila: gana la fila del estilo MAS CERCANO (4 esta a 1 del 5)
	if r := tabla.LookupCercana(2, 0, 5, 0); r == nil || r.Style != 4 {
		t.Fatalf("(fl, estilo 5, seq 0): queria la fila del estilo 4, devolvio %+v", r)
	}

	// 4. un tipo sin filas no inventa grafico (una pared de tipo 14 -- arbol -- no tiene en el afuera)
	if r := tabla.LookupCercana(2, 14, 0, 0); r != nil {
		t.Fatalf("(tr, estilo 0, seq 0): no hay filas de ese tipo, devolvio %+v", r)
	}

	// 5. el cel sale de la fila elegida
	if cel, ok := tabla.CelCercana(2, 0, 5, 0, 0); !ok || cel != 79 {
		t.Fatalf("CelCercana (fl, 5, 0): got (%d, %v), queria (79, true)", cel, ok)
	}

	// 6. la fila exacta manda: con el mismo (tipo, estilo) y la secuencia dentro del rango no se cae al cel
	//    del estilo vecino
	if cel, ok := tabla.CelCercana(2, 0, 0, 2, 0); !ok || cel != 0 {
		t.Fatalf("CelCercana (fl, 0, 2): got (%d, %v), queria (0, true)", cel, ok)
	}
}

// fuenteDePrueba: la casilla (0,0) con un piso que la tabla NO tiene, la (1,0) con uno que si, y la (2,0) con
// una pared que si.
type fuenteDePrueba struct{}

func (fuenteDePrueba) Size() (w, h int) { return 3, 1 }

func (fuenteDePrueba) Tiles(tx, ty int) []TileRef {
	switch tx {
	case 0:
		return []TileRef{{Layer: LayerFloor, Type: 0, Style: 5, Sequence: 0}}
	case 1:
		return []TileRef{{Layer: LayerFloor, Type: 0, Style: 0, Sequence: 2}}
	default:
		return []TileRef{{Layer: LayerWall, Type: 1, Style: 0, Sequence: 0}}
	}
}

func TestRevealTilesConRespaldo(t *testing.T) {
	tabla := tablaDePrueba()
	area := Area{HalfW: 400, HalfH: 400}

	// el camino del motor: la casilla sin fila queda vacia (2 celdas de 3)
	sinRespaldo := NewModel()

	if added := sinRespaldo.RevealTiles(tabla, fuenteDePrueba{}, 2, 1, 1, 0, area); added != 2 {
		t.Fatalf("RevealTiles: agregadas= %d, queria 2 (la del estilo 5 no tiene fila)", added)
	}

	if sinRespaldo.HasAt(-8, 4) {
		t.Fatal("RevealTiles sembro la casilla (0,0), que no tiene fila: el camino del motor NO cambia")
	}

	// el respaldo: las 3 casillas tienen celda, y una vino del respaldo
	conRespaldo := NewModel()

	added, sembradas := conRespaldo.RevealTilesConRespaldo(tabla, fuenteDePrueba{}, 2, 1, 1, 0, area)

	if added != 3 || sembradas != 1 {
		t.Fatalf("RevealTilesConRespaldo: agregadas= %d sembradas= %d, queria 3 y 1", added, sembradas)
	}

	for tx := 0; tx < 3; tx++ {
		if !conRespaldo.HasAt(TileCell(tx, 0)) {
			t.Fatalf("la casilla %d quedo sin celda en el modelo", tx)
		}
	}

	if n := conRespaldo.Count(); n != 3 {
		t.Fatalf("celdas= %d, queria 3 (una por casilla)", n)
	}
}
