// Package png: el PNG de la biblioteca estandar, en la parte que la consola puede tener.
//
// PARA QUE SE USA: d2app/app.go:588 guarda una captura de pantalla en PNG al apretar una tecla. Es una
// herramienta del que programa, no parte del juego.
//
// POR QUE NO ESTA: en la consola no se puede escribir a disco. El romdisk donde vive el juego es de SOLO
// LECTURA (ya esta declarado en el os de la consola: abrir para escribir falla). Un encoder de PNG que
// comprima y devuelva bytes para que nadie los pueda guardar no sirve para nada: seria codigo que corre
// para que el error venga un paso despues y mas confuso.
//
// Asi que Encode devuelve el error, con el motivo escrito. No se finge: si alguien llega a apretar la tecla
// de la captura, va a leer por que no se puede, en vez de que no pase nada.
//
// (El DECODIFICADOR de PNG tampoco esta: en la consola las imagenes vienen del asset manager, que lee los
// formatos del juego -- DC6, BMP -- del romdisk por el manifiesto, no de un PNG suelto.)
package png

import (
	"d2std/errors"
	"d2std/image"
	"d2std/io"
)

// ErrSoloLectura: en la consola no se puede escribir (el romdisk es de solo lectura).
var ErrSoloLectura = errors.New("png: en la consola no se puede escribir a disco (el romdisk es de solo lectura)")

// Encode: no se puede (ver el comentario del paquete).
func Encode(w io.Writer, m image.Image) error { return ErrSoloLectura }

// EncodeResample: no se puede, por lo mismo.
func EncodeResample(w io.Writer, m image.Image, o *struct{}) error { return ErrSoloLectura }
