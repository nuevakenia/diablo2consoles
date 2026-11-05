package d2mapengine

// DC-002.159b -- DEVOLVER LA MEMORIA DEL MAPA CUANDO EL MAPA YA NO SE REPINTA.
//
// POR QUE EXISTE. El pueblo compuesto deja el semiespacio del recolector (1.835.008 B) al 69 % de datos vivos
// (1.268.000 B) y el compuesto del heroe agrega ~436.000 B, asi que el recolector muere con
// `GC to-space overflow` (medido en DC-002.159, ver console/dc/evidencia/). Esa memoria esta del lado del MAPA:
// las 2.312 casillas del nivel y, cada una, sus rebanadas de `Components` (1 a 3 elementos cada una, o sea por
// debajo del corte de 64 KB: viven en el SEMIESPACIO, no en el monton externo).
//
// CUANDO SE PUEDEN SOLTAR. Desde DC-002.157 el mapa se pinta UNA vez (mapa estatico del renderer) y los cuadros
// siguientes se saltan el renderer entero. Con el mapa pintado y sin nivel nuevo por delante, `d2mapengine` no
// se vuelve a leer: ni las casillas, ni los `.dt1` (los bloques crudos los devuelve `d2asset.SoltarArenaDeBloques`,
// DC-002.101). Ese es el lugar que el heroe necesita.
//
// EL CONTRATO, dicho como es:
//   - solo se llama con el mapa YA PINTADO en el lienzo y sin repintado por delante (`SoltarElMapaPintado159`
//     del renderer). Si el mapa se vuelve a pintar despues de esto, se pinta un mundo vacio;
//   - `GetStartPosition` recorre las casillas para buscar el objeto de entrada: el que llama tiene que ANOTAR
//     el punto de arranque antes de soltar (`SetStartPosition`, que es la via del motor para fijarlo);
//   - la colision del ente NO usa las casillas (`mapEntity.Step` mueve sobre el camino que ya tiene), asi que
//     el heroe sigue caminando igual;
//   - esto NO devuelve el bloque de casillas en si (`m.tiles`, 616.968 B del monton externo): se deja puesto y
//     en cero, para que `TileAt` siga devolviendo una casilla vacia y no un puntero a memoria ajena.

import (
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
)

// casillasSoltadas159: el nivel se quedo sin casillas (lo prendio `SoltarLasCasillasDelMapa`).
var casillasSoltadas159 bool

// SoltarLasCasillasDelMapa devuelve al recolector las rebanadas de las casillas compuestas: cada `Components`
// queda en cero (las cinco rebanadas sueltas) y el nivel queda vacio. Devuelve cuantas casillas se vaciaron y
// cuantos bytes de registros del `.ds1` se devolvieron (la cuenta, no una medida del recolector).
//
// En el pueblo son ~12.000 registros: es la memoria que el compuesto del heroe necesita.
func (m *MapEngine) SoltarLasCasillasDelMapa() (casillas, bytes int) {
	if m == nil {
		return 0, 0
	}

	tamanoDelRegistro := int(unsafe.Sizeof(d2ds1.Tile{}))
	registros := 0

	for i := range m.tiles {
		c := &m.tiles[i].Components

		n := cap(c.Walls) + cap(c.Orientations) + cap(c.Floors) + cap(c.Shadows) + cap(c.Substitutions)
		if n == 0 {
			continue
		}

		registros += n
		casillas++

		// `MapTile{}` suelta las cinco rebanadas (el recolector se lleva sus registros); la casilla queda
		// vacia pero EN SU LUGAR, asi el indice sigue siendo valido.
		m.tiles[i] = MapTile{}
	}

	casillasSoltadas159 = true

	return casillas, registros * tamanoDelRegistro
}

// SoltarLasTablasDeTiles devuelve los metadatos de los `.dt1` del nivel: la lista de tiles (con sus
// `EncodedData`, que apuntaban al arena de bloques) y los indices de busqueda por estilo/secuencia.
//
// Son ~507 tiles de 272 B (~138 KB) mas el indice. Nadie los vuelve a mirar cuando el mapa no se repinta:
// `GetTiles` es lo unico que los lee y solo lo llama el renderer cuando compone la cache.
func (m *MapEngine) SoltarLasTablasDeTiles() (tiles, bytes int) {
	if m == nil {
		return 0, 0
	}

	tiles = len(m.dt1TileData)
	bytes = cap(m.dt1TileData)*int(unsafe.Sizeof(d2dt1.Tile{})) + cap(m.dt1Files)*int(unsafe.Sizeof("")) +
		cap(m.dt1Starts)*int(unsafe.Sizeof(int(0))) + cap(m.dt1Rank)*int(unsafe.Sizeof(int32(0)))

	m.dt1TileData = nil
	m.dt1Files = nil
	m.dt1Starts = nil
	m.dt1Rank = nil
	m.dt1Index = nil
	m.dt1KeyCount = nil
	m.sellos159j = nil
	m.raizDeBloques = nil

	return tiles, bytes
}

// CasillasSoltadas159 dice si el nivel se quedo sin casillas (lo consulta el instrumento).
func CasillasSoltadas159() bool { return casillasSoltadas159 }
