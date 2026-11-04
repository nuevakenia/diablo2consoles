package d2automap

import "sort"

// Layer is one of the engine's cell lists (UI\automap.cpp keeps one balanced
// tree per list in the AutoMap record; floors, walls and objects are drawn in
// that order, each sorted by y then x). (VERIFIED)
type Layer int

// Layers, in drawing order.
const (
	LayerFloor Layer = iota
	LayerWall
	LayerObject
	numLayers
)

// groupOf is the engine's DAT_00710da8 table: cells that belong to the same
// group never share a position (FUN_00453190), e.g. two different pieces of
// the same wall at one place. Cells not in the table (group -1) are never
// added twice at one position. (VERIFIED)
var groupOf = func() map[int]int {
	pairs := [][2]int{
		{0, 0}, {1, 0}, {2, 0}, {3, 0}, {6, 1}, {7, 1}, {8, 1}, {11, 2}, {12, 2}, {13, 3}, {14, 3},
		{20, 4}, {38, 4}, {21, 5}, {39, 5}, {46, 6}, {47, 6}, {48, 6}, {49, 6}, {51, 7}, {52, 7}, {53, 7}, {54, 7},
		{60, 8}, {70, 8}, {61, 9}, {71, 9}, {120, 10}, {169, 10}, {171, 10}, {121, 11}, {170, 11}, {172, 11},
		{257, 12}, {258, 12}, {259, 12}, {266, 13}, {267, 13}, {337, 14}, {338, 14},
		{472, 15}, {473, 15}, {474, 15}, {475, 15}, {520, 16}, {521, 16}, {522, 16}, {533, 17}, {534, 17},
	}

	m := make(map[int]int, len(pairs))
	for _, p := range pairs {
		m[p[0]] = p[1]
	}

	return m
}()

// Group returns the dedup group of a cel, -1 when it has none.
func Group(cel int) int {
	if g, ok := groupOf[cel]; ok {
		return g
	}

	return -1
}

// Cell is a revealed automap cell.
type Cell struct {
	X, Y int // cell space, scale 10
	Cel  int // MaxiMap.dc6 frame
	Kind Layer
}

// DC-002.130: LAS CLAVES DE LOS MAPAS DEL MODELO VAN EMPAQUETADAS EN UN ENTERO, no en una estructura.
//
// POR QUE. Medido en la consola (Flycast, SH4): con clave de estructura (`posKey`, `tileKey`) la PRIMERA escritura
// de mapa de la corrida -`m.seen[tk] = true`, el primer renglon del primer revelado- moria con `Illegal
// instruction` en `runtime.mapassign` (gccgo 15 cruzado a sh-elf; el PC resuelve ahi y no hay linea). Con clave
// entera los mismos mapas escriben y el revelado sigue. El rango de sobra para el modelo: la capa es 0..2, x e y
// son celdas en escala 10 (el pueblo entero mide 570x410) y el indice de casilla es el de `Tiles` (menos de 64).
const sesgoDeCelda = 1 << 19 // x e y se guardan en 20 bits con este sesgo, para que entren negativos

// celKey empaqueta (capa, x, y) en un entero: capa en los 8 bits altos, x en 20, y en 20.
func celKey(l Layer, x, y int) uint64 {
	return uint64(l)<<56 | (uint64(x+sesgoDeCelda)&0xFFFFF)<<20 | (uint64(y+sesgoDeCelda) & 0xFFFFF)
}

// celKeyPartes deshace celKey.
func celKeyPartes(k uint64) (l Layer, x, y int) {
	return Layer(k >> 56), int((k>>20)&0xFFFFF) - sesgoDeCelda, int(k&0xFFFFF) - sesgoDeCelda
}

// vistoKey empaqueta (capa, tx, ty, indice de casilla) en un entero: capa 4 bits, tx 16, ty 16, indice 12.
func vistoKey(l Layer, x, y, i int) uint64 {
	return uint64(l)<<60 | uint64(x&0xFFFF)<<44 | uint64(y&0xFFFF)<<28 | uint64(i&0xFFF)
}

// Model is the revealed part of one level's automap.
type Model struct {
	cells map[uint64][]int
	count [numLayers]int
	seen  map[uint64]bool // tiles already processed (the engine's flag 0x40000)

	sorted  []Cell
	isDirty bool
}

// NewModel returns an empty model.
func NewModel() *Model {
	return &Model{cells: map[uint64][]int{}, seen: map[uint64]bool{}}
}

// Add inserts a cell. It returns false for a duplicate (same position and
// layer where the new cel is ungrouped or shares the group of an existing one).
func (m *Model) Add(l Layer, x, y, cel int) bool {
	k := celKey(l, x, y)
	for _, e := range m.cells[k] {
		if e == cel {
			return false
		}

		g := Group(cel)
		if g == -1 || g == Group(e) {
			return false
		}
	}

	m.cells[k] = append(m.cells[k], cel)
	m.count[l]++
	m.isDirty = true

	return true
}

// Count returns the number of revealed cells (all layers).
func (m *Model) Count() int { return m.count[LayerFloor] + m.count[LayerWall] + m.count[LayerObject] }

// CountLayer returns the number of revealed cells of one layer.
func (m *Model) CountLayer(l Layer) int { return m.count[l] }

// Clear forgets everything (a new game of the same level).
func (m *Model) Clear() {
	*m = *NewModel()
}

// Cells returns all cells in drawing order: by layer, then y, then x, then cel.
// The slice is cached until the model changes and must not be modified.
func (m *Model) Cells() []Cell {
	if !m.isDirty && m.sorted != nil {
		return m.sorted
	}

	out := make([]Cell, 0, m.Count())

	for k, cels := range m.cells {
		l, x, y := celKeyPartes(k)

		for _, c := range cels {
			out = append(out, Cell{X: x, Y: y, Cel: c, Kind: l})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}

		if a.Y != b.Y {
			return a.Y < b.Y
		}

		if a.X != b.X {
			return a.X < b.X
		}

		return a.Cel < b.Cel
	})

	m.sorted, m.isDirty = out, false

	return out
}

// TileRef describes one tile layer entry of a map tile.
type TileRef struct {
	Layer Layer
	// Type is the tile orientation (0 for floors), Style the main index and
	// Sequence the sub index of the tile's DT1 data.
	Type, Style, Sequence int
}

// TileSource gives the tiles of a map to the reveal.
type TileSource interface {
	// Size is the map size in tiles.
	Size() (w, h int)
	// Tiles returns the drawable layers of a tile (floors and walls).
	Tiles(tx, ty int) []TileRef
}

// Area is the part of the map a reveal looks at, around a centre cell.
type Area struct {
	// HalfW and HalfH are the half extents in cell space (scale 10). The game
	// reveals the tiles it has drawn on screen, so the default (see ScreenArea)
	// is the game screen divided by 10 plus a margin of a tile.
	HalfW, HalfH int
}

// ScreenArea returns the area of a game screen of w x h pixels: the original
// reveals the tiles that were drawn (flag 0x20000), i.e. the visible ones.
// (the exact margin is UNVERIFIED)
func ScreenArea(w, h int) Area {
	return Area{HalfW: w/20 + 16, HalfH: h/20 + 8}
}

// RevealTiles adds the tiles of src inside area around the hero (tile
// coordinates, fractional) to the model and returns how many cells were added.
// A tile is processed once. level is the LvlTypes id of the level; seed makes
// the choice among the cels of a row stable (the engine uses the level seed).
func (m *Model) RevealTiles(t *Table, src TileSource, level int, seed uint32, heroX, heroY float64, a Area) int {
	added, _ := m.reveal(t, src, level, seed, heroX, heroY, a, false)

	return added
}

// RevealTilesConRespaldo is RevealTiles with the console port's fill policy
// (DC-002.146b): a tile whose (orientation, style, sequence) has no row in the
// table still gets the cel of the NEAREST row of its orientation
// (Table.CelCercana), instead of no cel at all. It returns the cells added and
// how many of them came from that fallback.
//
// The engine's RevealTiles does not change: a level whose tiles are all in
// AutoMap.txt (the town) gets exactly the same cells either way, which is what
// the console measures (`MINIMAPA_SIEMBRA=no` is the A/B).
func (m *Model) RevealTilesConRespaldo(t *Table, src TileSource, level int, seed uint32, heroX, heroY float64,
	a Area) (added, sembradas int) {
	return m.reveal(t, src, level, seed, heroX, heroY, a, true)
}

// reveal is the body of both reveal flavours; respaldo picks Table.CelCercana
// over Table.Cel.
func (m *Model) reveal(t *Table, src TileSource, level int, seed uint32, heroX, heroY float64, a Area,
	respaldo bool) (added, sembradas int) {
	w, h := src.Size()
	hcx, hcy := WorldCell(heroX, heroY)

	// the tiles inside the diamond-shaped screen rectangle: |dx-dy|*8 <= HalfW and |dx+dy|*4 <= HalfH
	// around the hero tile; bound the scan by the box of the rectangle.
	rx := a.HalfW/8 + 2
	ry := a.HalfH/4 + 2
	x0, x1 := int(heroX)-(rx+ry)/2-2, int(heroX)+(rx+ry)/2+2
	y0, y1 := int(heroY)-(rx+ry)/2-2, int(heroY)+(rx+ry)/2+2

	if x0 < 0 {
		x0 = 0
	}

	if y0 < 0 {
		y0 = 0
	}

	if x1 > w-1 {
		x1 = w - 1
	}

	if y1 > h-1 {
		y1 = h - 1
	}

	for ty := y0; ty <= y1; ty++ {
		for tx := x0; tx <= x1; tx++ {
			cx, cy := TileCell(tx, ty)

			if abs(float64(cx)-hcx) > float64(a.HalfW) || abs(float64(cy)-hcy) > float64(a.HalfH) {
				continue
			}

			for i, ref := range src.Tiles(tx, ty) {
				tk := vistoKey(ref.Layer, tx, ty, i)
				if m.seen[tk] {
					continue
				}

				m.seen[tk] = true

				cel, ok := t.Cel(level, ref.Type, ref.Style, ref.Sequence, tileHash(seed, tx, ty, i))
				deRespaldo := false

				if !ok && respaldo {
					cel, ok = t.CelCercana(level, ref.Type, ref.Style, ref.Sequence, tileHash(seed, tx, ty, i))
					deRespaldo = ok
				}

				if !ok {
					continue
				}

				y := cy
				if ref.Type >= 16 {
					y += UpperWallShift
				}

				if m.Add(ref.Layer, cx, y, cel) {
					added++

					if deRespaldo {
						sembradas++
					}
				}
			}
		}
	}

	return added, sembradas
}

// HasAt says whether the model has any cell at that cell position (any layer).
// It is the console's "this cell of the map is on the automap" question
// (DC-002.146b), answered with three map lookups and no allocation.
func (m *Model) HasAt(x, y int) bool {
	for l := Layer(0); l < numLayers; l++ {
		if len(m.cells[celKey(l, x, y)]) > 0 {
			return true
		}
	}

	return false
}

// AddObject adds the cell of an object (objects.txt AutoMap column) at its
// world position in tiles: the engine puts it at (isoX/10+1, isoY/10-3).
// (VERIFIED for the offsets; objects whose cell is 0 or less have none)
func (m *Model) AddObject(cel int, tx, ty float64) bool {
	if cel <= 0 {
		return false
	}

	x, y := WorldCell(tx, ty)

	return m.Add(LayerObject, int(x)+1, int(y)-3, cel)
}

// tileHash is a small integer hash for the random cel choice.
func tileHash(seed uint32, x, y, i int) uint32 {
	h := seed ^ uint32(x)*0x9e3779b1 ^ uint32(y)*0x85ebca6b ^ uint32(i)*0xc2b2ae35
	h ^= h >> 15
	h *= 0x2c1b3c6d
	h ^= h >> 12

	return h
}
