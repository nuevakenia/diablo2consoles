package d2mapengine

// DC-002.159j -- EL SELLO DE LOS BLOQUES DE CADA TILE: la prueba de que el tile que el renderer lee es el que
// se cargo.
//
// POR QUE EXISTE. La corrida de DC-002.159i pidio una imagen de pared de 8.394.240 B para `estilo=0 sec=0
// tipo=1` (`minY=-29.648`) mientras el mismo tile, mirado en el host, declara `Height=-288` y 288 de alto
// (`console/dc/medidas/paredes-159j`: 46.080 B, `minY` de -256). El `Height` viaja DENTRO del `d2dt1.Tile`
// (correcto en la consola) y el `minY`/`maxY` salen de la rebanada `Blocks` (basura). O sea: el tile de
// `dt1TileData` esta bien y **su rebanada de bloques ya no es suya**.
//
// QUE HACE. Cuando el instrumento esta prendido, al cargar cada `.dt1` se guarda un SELLO por tile en un
// arreglo chico y propio (`m.sellos159j`, 24 B por tile: el tamano de la rebanada, su `minY`/`maxY` y la
// direccion del primer bloque). El renderer, al componer la pared, compara lo que ve contra el sello: si
// coincide, el tile sigue siendo el que se cargo; si no, la memoria de sus bloques fue reciclada (o movida) y
// el numero queda en el log con las dos versiones, sin suponer nada.
//
// POR QUE EN EL MOTOR Y NO EN EL RENDERER. `GetTiles` devuelve COPIAS de `d2dt1.Tile`, asi que el renderer no
// puede saber cual es el original; el motor si (es el dueno de `dt1TileData`). El sello se guarda aparte
// justamente para que no comparta destino con lo que se esta midiendo.
//
// PRECIO. Cero cuando el instrumento esta apagado (una comparacion de una variable por pared compuesta), y
// 24 B por tile cuando esta prendido. El camino de la PC no lo prende nadie.

import (
	"fmt"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
)

// Sello159j: lo que tenia la rebanada `Blocks` de un tile en el momento de cargarlo.
type Sello159j struct {
	Bloques int32
	MinY    int32
	MaxY    int32
	YMasa   uint32 // FNV-1a de los `Y` de los bloques, en orden
	Primero uintptr
}

var instrumentoSellos159j bool

// ActivarSellosDeLosTiles159j prende/apaga el instrumento (lo prende el arranque de la consola cuando la
// corrida es de esta tarjeta).
func ActivarSellosDeLosTiles159j(v bool) { instrumentoSellos159j = v }

// SellosDeLosTiles159j dice si el instrumento esta prendido (lo consulta el instrumento del arranque).
func SellosDeLosTiles159j() bool { return instrumentoSellos159j }

// sellar159j: llena el sello de los tiles que se acaban de agregar a `dt1TileData` (desde `desde`).
func (m *MapEngine) sellar159j(desde int) {
	if !instrumentoSellos159j {
		return
	}

	for i := len(m.sellos159j); i < desde; i++ {
		m.sellos159j = append(m.sellos159j, Sello159j{})
	}

	for i := desde; i < len(m.dt1TileData); i++ {
		m.sellos159j = append(m.sellos159j, sellarUnTile159j(&m.dt1TileData[i]))
	}
}

func sellarUnTile159j(t *d2dt1.Tile) Sello159j {
	s := Sello159j{Bloques: int32(len(t.Blocks)), YMasa: 2166136261}

	if len(t.Blocks) > 0 {
		s.Primero = uintptr(unsafe.Pointer(&t.Blocks[0]))
	}

	for _, b := range t.Blocks {
		if int32(b.Y) < s.MinY {
			s.MinY = int32(b.Y)
		}

		if int32(b.Y)+32 > s.MaxY {
			s.MaxY = int32(b.Y) + 32
		}

		s.YMasa ^= uint32(int32(b.Y))
		s.YMasa *= 16777619
	}

	return s
}

// SelloDelTile159j: el sello del tile que `GetTiles(estilo, secuencia, tipo)` devuelve en la posicion
// `variante` (el mismo orden que usa el renderer: el de `dt1Index`, que es el de `dt1TileData`). `hay` es falso
// si el instrumento esta apagado o si esa clave/variante no esta.
func (m *MapEngine) SelloDelTile159j(estilo, secuencia, tipo, variante int) (Sello159j, bool) {
	if !instrumentoSellos159j {
		return Sello159j{}, false
	}

	if len(m.dt1Rank) == len(m.dt1TileData) && m.dt1Index != nil {
		indices := m.dt1Index[claveDeTile(int32(estilo), int32(secuencia), int32(tipo))]
		if variante < 0 || variante >= len(indices) {
			return Sello159j{}, false
		}

		idx := int(indices[variante])
		if idx < 0 || idx >= len(m.sellos159j) {
			return Sello159j{}, false
		}

		return m.sellos159j[idx], true
	}

	visto := 0

	for i := range m.dt1TileData {
		t := &m.dt1TileData[i]
		if t.Style != int32(estilo) || t.Sequence != int32(secuencia) || t.Type != int32(tipo) {
			continue
		}

		if visto == variante && i < len(m.sellos159j) {
			return m.sellos159j[i], true
		}

		visto++
	}

	return Sello159j{}, false
}

// EstadoDeLasTablasDeTiles159j: el tamano de `dt1TileData` y de sus rebanadas de bloques, para saber si el
// recolector de libgodc lo tiene en el semiespacio (lo recorre) o en el monton externo (NO lo recorre: ver
// `SoltarLasCasillasDelNivelAnterior`, DC-002.159l).
func (m *MapEngine) EstadoDeLasTablasDeTiles159j() string {
	sizeofTile := int(unsafe.Sizeof(d2dt1.Tile{}))
	sizeofBloque := int(unsafe.Sizeof(d2dt1.Block{}))

	bloques, bytesDeBloques := 0, 0

	for i := range m.dt1TileData {
		bloques += len(m.dt1TileData[i].Blocks)
		bytesDeBloques += len(m.dt1TileData[i].Blocks) * sizeofBloque
	}

	tabla := cap(m.dt1TileData) * sizeofTile

	donde := "SEMIESPACIO (el recolector lo recorre: las rebanadas de adentro son raices)"
	if tabla > 64*1024 {
		donde = "MONTON EXTERNO (el recolector NO recorre su interior: las rebanadas de adentro NO son raices)"
	}

	rebanadas, _ := m.EstadoDeLaRaizDeBloques159j()

	return fmt.Sprintf("tiles=%d cap=%d sizeof= %d B -> tabla=%d B %s; bloques=%d x %d B = %d B; raiz=%d "+
		"rebanadas (%d B)", len(m.dt1TileData), cap(m.dt1TileData), sizeofTile, tabla, donde, bloques,
		sizeofBloque, bytesDeBloques, rebanadas, rebanadas*12)
}

// EstadoDelosSellos159j: cuantos sellos hay (para el log del arranque).
func (m *MapEngine) EstadoDelosSellos159j() int { return len(m.sellos159j) }
