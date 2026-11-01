package d2mapstamp

// DC-002.110 -- EL `.ds1` DE UN NIVEL FUERA DEL SEMIESPACIO DEL RECOLECTOR: la puerta del stamp.
//
// POR QUE EXISTE. Medido en DC-002.109 (Flycast, `gc_stats`): un nivel compuesto deja ~413.000 B vivos en el
// semiespacio del recolector de libgodc (1.835.008 B) despues de una recoleccion COMPLETA, y son los del `.ds1`
// del nivel (el mismo salto que da el parseo). Con eso la vuelta del cambio de nivel no entra: 1.735.048 B
// despues de parsear su `.ds1` + ~382.632 B de las casillas > 1.835.008 B, aunque el monton de C este sano
// (949.688 B libres). La ida si entraba; lo que no entra es VOLVER.
//
// CUANTO ES (medido en el host con `console/dc/medidas/ds1-110`): el `.ds1` del pueblo son 57x41 = 2337 celdas
// en 4 capas (2 de pared, 1 de piso, 1 de sombra) de `sizeof(d2ds1.Tile)` cada una, o sea 224.352 B en el host
// (261.744 B en el SH-4). El resto del `.ds1` (lista de archivos, objetos, caminos de NPC) son unos pocos KB.
//
// LA CURA esta en `d2common/d2fileformats/d2ds1/arena_110.go`: las celdas de las capas se piden en UN bloque del
// monton de C (mas de 64 KB: en libgodc eso es `gc_external_alloc`, fuera de los dos semiespacios) con
// liberador explicito inyectado desde el arranque de la consola. Este archivo es la PUERTA del motor: el que
// compone un nivel suelta el bloque por aca, sin saber como esta hecho.
//
// EL CONTRATO, que es lo que hay que respetar al llamarlo:
//
//   - `SoltarLosTilesDelDS1` se llama DESPUES de `PlaceStampClippedWhere`, con el nivel ya compuesto. Ahi el
//     `.ds1` no lo mira nadie: las casillas del mapa son COPIAS de las celdas (no punteros) y ni la cache de
//     tiles ni el dibujado leen un byte del `.ds1`.
//   - Despues de soltar, el stamp queda con las capas VACIAS: `Tile` devuelve nil, no memoria devuelta. Un uso
//     tardio se ve como un stamp vacio en vez de leer memoria de otro (el mismo contrato del tramo prestado de
//     la fuente, DC-002.96, y del arena de bloques, DC-002.101).
//   - En la PC no hace nada y devuelve 0: ahi no se instala liberador y las celdas las recoge el recolector.
//
// POR QUE LLAMA A FUNCIONES DE PAQUETE DE `d2ds1` Y NO A SUS METODOS. MEDIDO en esta tarjeta: gccgo 15.1 no
// resolvia `mr.ds1.SoltarArenaDeTiles()` desde `d2mapstamp` -- contestaba "reference to undefined field or method"
// sobre metodos que SI estan en el `.gox` de `d2ds1` (comprobado con `strings`) -- y las FUNCIONES de paquete de
// `d2ds1` se resuelven sin problema. El mismo codigo compila sin ruido en un paquete aparte, asi que la causa
// exacta del defecto no quedo aislada; lo que queda es el camino que compila. Anotado como trampa del toolchain
// para el proximo que llame a `d2ds1` desde el motor.

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"

// SoltarLosTilesDelDS1 devuelve al monton el bloque de celdas del `.ds1` de este stamp, y devuelve los bytes que
// el liberador dijo haber devuelto (0 si el `.ds1` vive en el monton de Go o si no hay liberador instalado, que
// es siempre el caso en la PC). Ver el contrato arriba.
func (mr *Stamp) SoltarLosTilesDelDS1() int {
	if mr == nil || mr.ds1 == nil {
		return 0
	}

	return d2ds1.SoltarArenaDeTilesDe(mr.ds1)
}

// TilesDelDS1EnArena dice si las celdas de las capas de este stamp viven fuera del monton de Go (DC-002.110).
func (mr *Stamp) TilesDelDS1EnArena() bool {
	return mr != nil && mr.ds1 != nil && d2ds1.EnArenaDe(mr.ds1)
}

// BytesDeLaArenaDelDS1 es el tamaño del bloque de celdas del stamp (0 si vive en el monton de Go).
func (mr *Stamp) BytesDeLaArenaDelDS1() int {
	if mr == nil || mr.ds1 == nil {
		return 0
	}

	return d2ds1.BytesDeLaArenaDe(mr.ds1)
}
