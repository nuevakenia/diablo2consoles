package d2maprenderer

// DC-002.159j -- LA PARED QUE LEE BLOQUES QUE YA NO SON SUYOS: el aviso, con las dos versiones.
//
// POR QUE EXISTE. DC-002.159i dejo el techo por imagen (81.920 B) y con eso la corrida del Gate 0 no muere,
// pero el dato roto sigue: `estilo=0 sec=0 tipo=1` pide 8.394.240 B porque sus bloques tienen `minY=-29.648`,
// cuando el mismo tile en el host mide 288 de alto (`console/dc/medidas/paredes-159j`, 46.080 B).
//
// QUE HACE. En `generateWallCache`, justo despues de sacar `minY`/`maxY` de los bloques, se compara lo medido
// contra el SELLO que el motor guardo al cargar ese mismo tile (`d2mapengine.SelloDelTile159j`). El sello vive
// en un arreglo aparte del motor, asi que no comparte destino con lo que se esta midiendo:
//
//	coincide        -> el tile sigue siendo el que se cargo;
//	no coincide     -> la rebanada `Blocks` quedo apuntando a memoria reciclada (el `Primero` del sello dice
//	                   donde estaba) y el log escribe las dos versiones, sin suponer nada.
//
// El instrumento lo prende el arranque de la consola (`ActivarSellosDeLosTiles159j` del motor). Apagado no
// cuesta nada: una comparacion de una variable por pared compuesta.

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
)

// avisarSiElTileNoEsElQueSeCargo159j: la comparacion. `minY`/`maxY` son los que salieron de los bloques que el
// renderer acaba de leer, y `bloques` su tamano.
func (mr *MapRenderer) avisarSiElTileNoEsElQueSeCargo159j(tile *d2ds1.Tile, minY, maxY int32, bloques int) {
	sello, hay := mr.mapEngine.SelloDelTile159j(int(tile.Style), int(tile.Sequence), int(tile.Type),
		int(tile.RandomIndex))
	if !hay {
		return
	}

	if sello.Bloques == int32(bloques) && sello.MinY == minY && sello.MaxY == maxY {
		return
	}

	mr.Warningf("MAPA159J: EL TILE NO ES EL QUE SE CARGO estilo=%d sec=%d tipo=%d var=%d: "+
		"al cargar bloques=%d minY=%d maxY=%d (bloques en 0x%08x, Y-masa %08x); ahora bloques=%d minY=%d maxY=%d "+
		"-> la rebanada `Blocks` del tile apunta a memoria que ya no es suya (DC-002.159j)",
		tile.Style, tile.Sequence, int(tile.Type), tile.RandomIndex,
		sello.Bloques, sello.MinY, sello.MaxY, sello.Primero, sello.YMasa, bloques, minY, maxY)
}

// testigoDicho159j: la pared testigo ya se dijo (se dice UNA vez: es la que delato el defecto).
var testigoDicho159j bool

// avisarLaParedTestigo159j: la pared `estilo=0 sec=0 tipo=1` -- la que en DC-002.159i pedia 8.394.240 B porque
// sus bloques tenian `minY=-29.648` -- se dice UNA vez, ya compuesta, con su alto real y el tamano de su imagen.
// Es el criterio 1 de la tarjeta con nombre y apellido, en la corrida de la consola.
func (mr *MapRenderer) avisarLaParedTestigo159j(tile *d2ds1.Tile, minY, maxY int32, bloques int, real int32) {
	if testigoDicho159j || int(tile.Style) != 0 || int(tile.Sequence) != 0 ||
		int(tile.Type) != int(d2enum.TileLeftWall) || tile.RandomIndex != 0 {
		return
	}

	testigoDicho159j = true

	mr.Infof("MAPA159J: LA PARED TESTIGO estilo=0 sec=0 tipo=1 var=0 SE COMPONE: bloques=%d minY=%d maxY=%d "+
		"realHeight=%d imagen=%d B (el dato del host: 288 de alto y 46.080 B; en 159i pedia 8.394.240 B)",
		bloques, minY, maxY, real, tileSurfaceWidth*int(real))
}
