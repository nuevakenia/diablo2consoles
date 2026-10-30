//go:build dreamcast

package pvr

import (
	"image"
	"strconv"
)

// recortar devuelve una imagen NUEVA con la seccion pedida (lo que RenderSection necesita: el origen de un
// dibujado puede ser un pedazo de otra superficie).
//
// La seccion se recorta al tamano del origen: un rectangulo que se sale no se adivina ni se rellena --
// devuelve solo lo que existe, que es lo que se puede comparar contra el host.
func (i *Imagen) recortar(r image.Rectangle) *Imagen {
	x0, y0 := r.Min.X, r.Min.Y
	x1, y1 := r.Max.X, r.Max.Y

	if x0 < 0 {
		x0 = 0
	}

	if y0 < 0 {
		y0 = 0
	}

	if x1 > i.ancho {
		x1 = i.ancho
	}

	if y1 > i.alto {
		y1 = i.alto
	}

	out := NuevaImagen(x1-x0, y1-y0)
	if x1 <= x0 || y1 <= y0 {
		return out
	}

	filaBytes := (x1 - x0) * 4

	for y := y0; y < y1; y++ {
		origen := (y*i.ancho + x0) * 4
		destino := (y - y0) * filaBytes
		copy(out.pixeles[destino:destino+filaBytes], i.pixeles[origen:origen+filaBytes])
	}

	return out
}

// itoa: un entero a texto, para los mensajes de error del backend.
//
// SE USA strconv Y NO UN ARRAY LOCAL A MANO: la version que armaba el numero en un `var buf [20]byte` hacia
// que GCCGO SE CAYERA COMPILANDO ("internal compiler error: in fill_in_array, at go/go-gcc.cc:1184"), sin
// decir nada del codigo. Es un defecto del compilador de la consola, y esta anotado para que el proximo que
// lo vea en otro paquete no persiga su propio codigo (medido en DC-002.10, pvr.go del shim).
func itoa(n int) string { return strconv.Itoa(n) }
