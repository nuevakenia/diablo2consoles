package d2util

import (
	"log"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

// ImgIndexToRGBA converts the given indices byte slice and palette into
// a byte slice of RGBA values
func ImgIndexToRGBA(indexData []byte, palette d2interface.Palette) []byte {
	bytesPerPixel := 4
	colorData := make([]byte, len(indexData)*bytesPerPixel)

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
