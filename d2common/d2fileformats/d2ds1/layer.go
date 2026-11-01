package d2ds1

// layerStreamType represents a layer stream type
type layerStreamType int

// Layer stream types
const (
	layerStreamWall1 layerStreamType = iota
	layerStreamWall2
	layerStreamWall3
	layerStreamWall4
	layerStreamOrientation1
	layerStreamOrientation2
	layerStreamOrientation3
	layerStreamOrientation4
	layerStreamFloor1
	layerStreamFloor2
	layerStreamShadow1
	layerStreamSubstitute1
)

type (
	tileRow  []Tile    // index is x coordinate
	tileGrid []tileRow // index is y coordinate
)

// Layer is an abstraction of a tile grid with some helper methods
type Layer struct {
	tiles tileGrid

	// DC-002.110 -- LA CAPA QUE VIVE FUERA DEL MONTON DE GO. Cuando el `.ds1` reserva su bloque en el monton
	// de C (ver arena_110.go), cada capa recibe una rebanada de ESE bloque (`celda`) y sus filas son vistas
	// adentro: no se pide memoria por fila, y el recolector no la copia. Con `celda` en nil la capa es la de
	// siempre (un `make` por fila, el camino de la PC).
	celda []Tile
}

// Tile returns the tile at the given x,y coordinate in the grid, or nil if empty.
func (l *Layer) Tile(x, y int) *Tile {
	if len(l.tiles) == 0 {
		// capa sin celdas: soltada (DC-002.110) o vacia. Se dice nil en vez de indexar una rejilla que no esta.
		return nil
	}

	if l.Width() < x || l.Height() < y {
		return nil
	}

	return &l.tiles[y][x]
}

// SetTile sets the tile at the given x,y coordinate in the tile grid
func (l *Layer) SetTile(x, y int, t *Tile) {
	if l.Width() > x || l.Height() > y {
		return
	}

	l.tiles[y][x] = *t
}

// Width returns the width of the tile grid
func (l *Layer) Width() int {
	if len(l.tiles[0]) < 1 {
		l.SetWidth(1)
	}

	return len(l.tiles[0])
}

// SetWidth sets the width of the tile grid, minimum of 1
func (l *Layer) SetWidth(w int) *Layer {
	if w < 1 {
		w = 1
	}

	// ensure at least one row
	if len(l.tiles) < 1 {
		l.tiles = make(tileGrid, 1)
	}

	// create/copy tiles as required to satisfy width
	for y := range l.tiles {
		if (w - len(l.tiles[y])) == 0 { // if requested width same as row width
			continue
		}

		tmpRow := make(tileRow, w)

		for x := range tmpRow {
			if x < len(l.tiles[y]) { // if tile exists
				tmpRow[x] = l.tiles[y][x] // copy it
			}
		}

		l.tiles[y] = tmpRow
	}

	return l
}

// Height returns the height of the tile grid
func (l *Layer) Height() int {
	if len(l.tiles) < 1 {
		l.SetHeight(1)
	}

	return len(l.tiles)
}

// SetHeight sets the height of the tile grid, minimum of 1
func (l *Layer) SetHeight(h int) *Layer {
	if h < 1 {
		h = 1
	}

	// make tmpGrid to move existing tiles into
	tmpGrid := make(tileGrid, h)

	for y := range tmpGrid {
		tmpGrid[y] = make(tileRow, l.Width())
	}

	// move existing tiles over
	for y := range l.tiles {
		if y >= len(tmpGrid) {
			continue
		}

		for x := range l.tiles[y] {
			if x >= len(tmpGrid[y]) {
				continue
			}

			tmpGrid[y][x] = l.tiles[y][x]
		}
	}

	l.tiles = tmpGrid

	return l
}

// Size returns the width and height of the tile grid
func (l *Layer) Size() (w, h int) {
	return l.Width(), l.Height()
}

// SetSize sets the width and height of the tile grid
func (l *Layer) SetSize(w, h int) *Layer {
	// DC-002.110: si la capa vive en el bloque del `.ds1` (monton de C), el tamaño solo rearma las VISTAS de
	// las filas adentro del bloque: no se pide memoria, y pedirla seria volver al semiespacio del recolector.
	if l.celda != nil {
		if w < 1 {
			w = 1
		}

		if h < 1 {
			h = 1
		}

		if w*h <= len(l.celda) && l.armarFilasDeLaCelda(w, h) {
			return l
		}

		// no entra en el bloque: la capa vuelve al camino de siempre (y el bloque se queda con el `.ds1`)
		l.celda = nil
	}

	return l.SetWidth(w).SetHeight(h)
}

// usarCelda le da a la capa su rebanada del bloque del `.ds1` (DC-002.110): a partir de aca las filas son
// vistas adentro del bloque y ninguna asignacion de la capa va al monton de Go.
func (l *Layer) usarCelda(celda []Tile, w, h int) {
	if l == nil || len(celda) < w*h || w < 1 || h < 1 {
		return
	}

	l.celda = celda[:w*h]
	l.armarFilasDeLaCelda(w, h)
}

// soltarCelda deja la capa sin celdas (se llama antes de devolver el bloque al monton). La capa sigue
// existiendo -- con su rejilla vacia -- para que un uso tardio se vea vacio y no lea memoria devuelta.
func (l *Layer) soltarCelda() {
	if l == nil {
		return
	}

	l.celda = nil
	l.tiles = nil
}

// EnArena dice si las celdas de la capa viven fuera del monton de Go (DC-002.110).
func (l *Layer) EnArena() bool { return l != nil && l.celda != nil }

// armarFilasDeLaCelda arma las filas como rebanadas adentro del bloque. Solo pide las cabeceras de las filas
// (alto x 12 B), que es lo unico que queda en el monton de Go.
func (l *Layer) armarFilasDeLaCelda(w, h int) bool {
	if l.celda == nil || w < 1 || h < 1 || w*h > len(l.celda) {
		return false
	}

	rejilla := make(tileGrid, h)
	for y := range rejilla {
		rejilla[y] = l.celda[y*w : (y+1)*w]
	}

	l.tiles = rejilla

	return true
}
