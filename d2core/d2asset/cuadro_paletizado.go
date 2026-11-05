package d2asset

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
)

// superficieConLienzo es la parte de una Surface que permite expandir los indices de un cuadro DENTRO de su
// propio lienzo, sin buffer intermedio.
//
// La cumple el backend de la consola (d2core/d2render/pvr: `Surface.ExpandirIndices`, que escribe en los
// pixeles de la superficie). La de la PC (ebiten, que dibuja sobre un *ebiten.Image) no, y sigue por el
// camino de siempre: `ImgIndexToRGBA` + `ReplacePixels`. Por eso es una interfaz OPCIONAL y no un metodo
// nuevo en d2interface.Surface: el contrato compartido no se toca y ningun backend queda obligado a
// implementar algo que no puede cumplir.
type superficieConLienzo interface {
	ExpandirIndices(indices []byte, paleta d2interface.Palette)
}

// superficieConLienzoDeRecorte159f: la parte de una Surface que sabe expandir un cuadro MAS CHICO que su
// lienzo (DC-002.159f).
//
// POR QUE HACE FALTA OTRA. La animacion de la consola reusa UNA superficie del tamano maximo de la direccion
// (`armarLaSuperficieDelCuadro`): el cuadro que se carga ocupa la esquina de arriba a la izquierda y el
// sobrante queda como estaba. `ExpandirIndices` escribe el cuadro como un bloque contiguo, asi que en un
// lienzo mas ancho las filas quedarian corridas; esta version escribe fila por fila.
//
// La cumple el backend de la consola. La PC no la necesita: ahi cada cuadro tiene su superficie exacta.
type superficieConLienzoDeRecorte159f interface {
	ExpandirIndicesRecorte159f(indices []byte, paleta d2interface.Palette, ancho, alto int)
}

// cargarCuadroEnSuperficie deja los indices de un cuadro de animacion (DC6 o DCC) de `ancho` x `alto` pixeles
// en la superficie que ya se creo para ese cuadro.
//
// DC-002.85: si el backend sabe expandir en su lienzo, la expansion se hace ahi y el buffer intermedio de
// 4 x pixeles NO se pide. Si no, se arma el buffer y se copia, que es como estaba desde siempre.
//
// DC-002.159f: si el lienzo es MAS GRANDE que el cuadro (la superficie reusada del tamano maximo), se expande
// con la version por filas; el que dibuja recorta despues (`Animation.Render`) para no pintar el sobrante.
func cargarCuadroEnSuperficie(sfc d2interface.Surface, indexData []byte, palette d2interface.Palette,
	ancho, alto int,
) {
	anchoLienzo, altoLienzo := sfc.GetSize()

	if anchoLienzo > ancho || altoLienzo > alto {
		if conRecorte, ok := sfc.(superficieConLienzoDeRecorte159f); ok {
			conRecorte.ExpandirIndicesRecorte159f(indexData, palette, ancho, alto)

			return
		}
	}

	if conLienzo, ok := sfc.(superficieConLienzo); ok {
		conLienzo.ExpandirIndices(indexData, palette)

		return
	}

	sfc.ReplacePixels(d2util.ImgIndexToRGBA(indexData, palette))
}
