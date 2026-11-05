package d2util

import (
	"log"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

// ImgIndexToRGBA converts the given indices byte slice and palette into
// a byte slice of RGBA values
//
// DC-002.97 (instrumento): cada llamada PIDE un buffer de 4 x pixeles y lo devuelve como copia. En el camino
// de pintado de la consola tiene que dar CERO -- ahi la expansion va directo al lienzo de la superficie
// (`ImgIndexToRGBAEn` por `Surface.ExpandirIndices`), y este contador es lo que lo prueba cuadro a cuadro.
var (
	PedidosDeExpansion int
	BytesDeExpansion   int
)

func ImgIndexToRGBA(indexData []byte, palette d2interface.Palette) []byte {
	bytesPerPixel := 4
	colorData := make([]byte, len(indexData)*bytesPerPixel)

	PedidosDeExpansion++
	BytesDeExpansion += len(colorData)

	ImgIndexToRGBAEn(colorData, indexData, palette)

	return colorData
}

// ImgIndexToRGBAEn expande los indices de paleta DENTRO de un buffer que presta el que llama, sin pedir
// memoria propia.
//
// POR QUE EXISTE (DC-002.85). `ImgIndexToRGBA` pide un buffer de 4 x pixeles por cuadro y devuelve una
// copia que el que llama vuelve a copiar en su lienzo (Surface.ReplacePixels) y despues tira: dos buffers
// vivos por cuadro. En la consola ese pedido es el recurso que se agota -- DC-002.84 midio que el primer
// cuadro de un `Composite` pide ~96 superficies, cada una con su intermedio. Con esta version el que
// expande escribe derecho en el buffer del destino (el lienzo de la superficie) y el intermedio no existe.
//
// LAS REGLAS SON LAS MISMAS QUE `ImgIndexToRGBA`, byte a byte: el indice 0 queda transparente (0,0,0,0) sin
// importar la paleta, y el resto toma R,G,B y A de su color. `destino` tiene que tener lugar para
// len(indexData)*4 bytes; lo que sobre no se toca y si falta, no se escribe nada (no se adivina: un buffer
// corto es un error del que llama, y escribir medio cuadro haria comparar cosas distintas creyendo que son
// las mismas).
func ImgIndexToRGBAEn(destino []byte, indexData []byte, palette d2interface.Palette) {
	const bytesPerPixel = 4

	if len(destino) < len(indexData)*bytesPerPixel {
		return
	}

	for i := 0; i < len(indexData); i++ {
		j := i * bytesPerPixel

		// Index zero is hardcoded transparent regardless of palette
		if indexData[i] == 0 {
			destino[j] = 0
			destino[j+1] = 0
			destino[j+2] = 0
			destino[j+3] = 0

			continue
		}

		c, err := palette.GetColor(int(indexData[i]))
		if err != nil {
			log.Print(err)
		}

		destino[j] = c.R()
		destino[j+1] = c.G()
		destino[j+2] = c.B()
		destino[j+3] = c.A()
	}
}

// ImgIndexToRGBAEnRecorte159f expande un cuadro de `ancho` x `alto` indices DENTRO de un destino MAS ANCHO
// (`anchoDestino`), sin pedir memoria propia.
//
// POR QUE EXISTE (DC-002.159f). La animacion del heroe reusa UNA superficie del tamano maximo de la direccion
// para todos sus cuadros (antes pedia una superficie nueva cada vez que la caja del cuadro no entraba en la que
// habia, y caminando eso es cada giro: 105 superficies por vuelta). Con la superficie mas grande que el cuadro,
// la expansion tiene que escribir POR FILAS: `ImgIndexToRGBAEn` escribe el cuadro como un bloque contiguo y en
// un lienzo mas ancho las filas quedarian corridas (el dibujo saldria en diagonal).
//
// LAS REGLAS DE LOS PIXELES SON LAS MISMAS (`ImgIndexToRGBAEn`, una sola definicion del mapeo): indice 0
// transparente y el resto toma R,G,B,A de su color. Si el destino no entra, no se escribe nada.
func ImgIndexToRGBAEnRecorte159f(destino, indices []byte, palette d2interface.Palette, ancho, alto, anchoDestino int) {
	const bytesPerPixel = 4

	if ancho <= 0 || alto <= 0 || anchoDestino < ancho || len(indices) < ancho*alto {
		return
	}

	if len(destino) < anchoDestino*alto*bytesPerPixel {
		return
	}

	for y := 0; y < alto; y++ {
		fila := indices[y*ancho : (y+1)*ancho]
		destinoFila := destino[y*anchoDestino*bytesPerPixel:]

		for x := 0; x < ancho; x++ {
			j := x * bytesPerPixel

			if fila[x] == 0 {
				destinoFila[j] = 0
				destinoFila[j+1] = 0
				destinoFila[j+2] = 0
				destinoFila[j+3] = 0

				continue
			}

			c, err := palette.GetColor(int(fila[x]))
			if err != nil {
				log.Print(err)
			}

			destinoFila[j] = c.R()
			destinoFila[j+1] = c.G()
			destinoFila[j+2] = c.B()
			destinoFila[j+3] = c.A()
		}
	}
}
