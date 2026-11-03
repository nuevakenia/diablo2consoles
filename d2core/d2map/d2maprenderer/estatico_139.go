package d2maprenderer

// DC-002.139: lo que el POC necesita saber del mapa para decidir si puede dejar de repintarlo.
//
//	TraslacionDeLaPantalla139 -- la traslacion de la vista en pixeles de pantalla. Es lo que se mueve cuando la
//	                             camara se mueve: dos cuadros con el mismo numero tienen el mapa en el mismo
//	                             lugar, y entonces volver a pintar los 248 tiles da los mismos pixeles.
//	TilesAnimados139         -- cuantos tiles animados (agua, antorchas) se pintaron en el ultimo cuadro. Si hay
//	                             alguno el mapa NO se puede saltar: su animacion avanza con el reloj, no con la
//	                             camara.

// TraslacionDeLaPantalla139: la traslacion de la vista, en pixeles de pantalla.
func (mr *MapRenderer) TraslacionDeLaPantalla139() (int, int) {
	return mr.viewport.GetTranslationScreen()
}

// TilesAnimados139: cuantos tiles animados se pintaron desde el ultimo `contarCuadro135` (o sea en el ultimo
// cuadro dibujado).
func (mr *MapRenderer) TilesAnimados139() int { return tilesAnimados139 }

// tilesAnimados139: el contador del cuadro. Lo sube `renderFloor`/`renderWall`/`renderRoof` cuando el tile que
// pintan tiene animacion.
var tilesAnimados139 int

// contarTileAnimado139: lo llama el renderer por cada tile animado que pinta.
func contarTileAnimado139() { tilesAnimados139++ }

// reiniciarTilesAnimados139: el arranque de cada cuadro, para que el numero sea de ESTE cuadro.
func reiniciarTilesAnimados139() { tilesAnimados139 = 0 }
