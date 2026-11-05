package d2mapengine

// DC-002.159j -- LAS REBANADAS DE BLOQUES DE LOS `.dt1` FUERA DEL ALCANCE DEL RECOLECTOR (y su raiz).
//
// EL DEFECTO, MEDIDO. `m.dt1TileData` es la tabla de tiles del nivel, y en el pueblo son 311 tiles de 272 B =
// 84.592 B en un pedido que crece por duplicacion: **91.392 B** (cap 336). Todo pedido de mas de
// `GC_LARGE_OBJECT_THRESHOLD` (64 KB) va a `gc_external_alloc` (`gc_heap.c:107`), que es un `malloc` que NO se
// registra en ninguna tabla de raices, y el Cheney de `gc_copy.c` **no escanea el interior de un bloque
// externo** (lo mismo que midio DC-002.159k con las casillas del mapa y arreglo DC-002.159l con el arena).
//
// Adentro de cada `d2dt1.Tile` de esa tabla hay una rebanada `Blocks` (los bloques del grafico: 8..45 por tile,
// 6.737 en el pueblo = 188.636 B). Esas rebanadas viven en el SEMIESPACIO del recolector, pero el unico puntero
// que las referencia esta DENTRO del bloque externo, o sea que el recolector no las ve: no son raices. La
// primera recoleccion que cae durante la composicion se las lleva, y el tile queda con el tamano y la direccion
// viejos (`Blocks` es una rebanada: `ptr`, `len` y `cap` quedan como estaban) apuntando a memoria reciclada.
//
// COMO SE VE EN LA CONSOLA. El `Height` del tile (que esta en el bloque externo) sigue bien y el `minY` de sus
// bloques no: la pared `estilo=0 sec=0 tipo=1` -288 declarado y `minY=-29.648` leido, o sea `realHeight=52.464`
// y un pedido de imagen de 160 x 52.464 = **8.394.240 B** (DC-002.159i, que le puso un techo para no morir).
// En el host el mismo tile mide 288 de alto y su imagen son 46.080 B (`console/dc/medidas/paredes-159j`, que
// compara los 86 tiles de las cuatro familias de pared del pueblo contra un `.dt1` recien descomprimido: 86
// coincidencias, 0 diferencias).
//
// LA CURA. `raizDeBloques`: una rebanada propia del motor con la MISMA rebanada `Blocks` de cada tile, en el
// orden de `dt1TileData`. Es chica (12 B por tile en el SH-4, 3.732 B en el pueblo), asi que vive en el
// semiespacio y el recolector la SI recorre: cada elemento es una raiz, el recolector conserva las rebanadas y
// **corrige el puntero adentro de `raizDeBloques`** cuando las mueve. El puntero que quedo adentro de la tabla
// externa no se corrige (no se escanea), asi que el que MANDA es el de la raiz: cada vez que el motor entrega un
// tile (`GetTiles`, `exactComponent`) le repone la rebanada `Blocks` desde `raizDeBloques`.
//
// POR QUE NO SE COPIAN LOS BLOQUES AL ARENA DE C (que es lo que hizo DC-002.159l con las casillas). Porque no
// entran: 6.737 bloques x 28 B = 188.636 B y el monton de C del cuadro 1 esta en el techo (`montonLibre= 38.808`
// y `breakLibre= 188.416` medidos en la corrida de 159l). La raiz cuesta 3.732 B y no pide un solo byte exterior.
//
// POR QUE NO SE COPIA TAMPOCO AL SEMIESPACIO "A MANO". Ya esta ahi: lo que falta no es memoria, es el PUNTERO
// que el recolector pueda ver. Esto es exactamente eso y nada mas.
//
// EL PRECIO. 12 B por tile mientras el nivel vive, y una copia de rebanada (3 palabras) por cada tile entregado.
// En la PC el camino es el mismo y da lo mismo: alli el recolector es exacto y la tabla no es externa, pero la
// rebanada que se repone es la misma de siempre.

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"

// raizDeBloques: por indice de `dt1TileData`, la rebanada `Blocks` de ese tile. Ver la cabecera.
func (m *MapEngine) raizDeLosBloques(desde int) {
	for i := len(m.raizDeBloques); i < desde; i++ {
		m.raizDeBloques = append(m.raizDeBloques, m.blocksDelTile(i))
	}

	for i := desde; i < len(m.dt1TileData); i++ {
		m.raizDeBloques = append(m.raizDeBloques, m.dt1TileData[i].Blocks)
	}
}

// blocksDelTile: la rebanada que la tabla tiene AHORA (solo para rellenar huecos de la raiz).
func (m *MapEngine) blocksDelTile(i int) []d2dt1.Block {
	if i < 0 || i >= len(m.dt1TileData) {
		return nil
	}

	return m.dt1TileData[i].Blocks
}

// tileConLosBloquesVivos: el tile `i` de la tabla con su rebanada `Blocks` repuesta desde la raiz (que es la que
// el recolector corrige cuando mueve los bloques). Sin raiz (una tabla armada a mano, como en las pruebas) se
// devuelve el tile tal cual.
func (m *MapEngine) tileConLosBloquesVivos(i int) d2dt1.Tile {
	t := m.dt1TileData[i]

	if i < len(m.raizDeBloques) {
		t.Blocks = m.raizDeBloques[i]
	}

	return t
}

// EstadoDeLaRaizDeBloques159j: cuantas rebanadas guarda la raiz (lo consulta el instrumento del arranque).
func (m *MapEngine) EstadoDeLaRaizDeBloques159j() (rebanadas, bloques int) {
	for i := range m.raizDeBloques {
		bloques += len(m.raizDeBloques[i])
	}

	return len(m.raizDeBloques), bloques
}
