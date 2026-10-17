// Package gif: el GIF de la biblioteca estandar, en la parte que la consola puede tener.
//
// PARA QUE SE USA: d2app/app.go graba las capturas de pantalla en un GIF animado (Encode por cuadro,
// Decode para volver a leer el cuadro, EncodeAll para armar el archivo). Es la grabacion de partidas del
// que programa, no parte del juego.
//
// POR QUE NO ESTA: por lo mismo que image/png. En la consola no hay disco donde escribir (el romdisk es de
// solo lectura), asi que el GIF se comprimiria para que despues nadie lo pueda guardar. Los tres
// (Encode, Decode, EncodeAll) devuelven el error con el motivo escrito.
//
// OJO CON UNA COSA QUE NO ES OBVIA: aca Decode TAMBIEN falla, aunque leer se podria en principio. La razon
// es concreta: el unico uso del motor es leer lo que acaba de escribir el propio grabador
// (app.go:640: gif.Decode(&buffer) sobre el buffer que lleno gif.Encode un renglon antes). Si Encode no
// puede escribir, ese buffer esta vacio y el Decode no tendria nada que decodificar. Poner un decodificador
// de LZW para leer un buffer que siempre va a estar vacio es trabajo que no cambia nada.
//
// Si algun dia el motor necesita LEER un GIF de verdad (por ejemplo para animaciones de interfaz), esto se
// ve al medir y se escribe el decodificador.
package gif

import (
	"d2std/errors"
	"d2std/image"
	"d2std/io"
)

// ErrSoloLectura: en la consola no se puede escribir (el romdisk es de solo lectura).
var ErrSoloLectura = errors.New("gif: en la consola no se puede escribir a disco (el romdisk es de solo lectura)")

// GIF: el archivo (los cuadros, los retardos y la paleta), igual que en la biblioteca estandar.
type GIF struct {
	Image     []*image.Paletted
	Delay     []int
	LoopCount int
	Disposal  []byte
	Config    image.Config
	BackgroundIndex byte
}

// Options: las opciones del encoder (la parte que el motor toca).
type Options struct {
	NumColors    int
	Quantizer    interface{}
	Drawer       interface{}
	NumDecodeColors int
}

// Encode: no se puede (ver el comentario del paquete).
func Encode(w io.Writer, m image.Image, o *Options) error { return ErrSoloLectura }

// EncodeAll: no se puede, por lo mismo.
func EncodeAll(w io.Writer, g *GIF) error { return ErrSoloLectura }

// Decode: no se puede, y esta dicho por que arriba (el buffer que se leeria siempre esta vacio).
func Decode(r io.Reader) (image.Image, error) { return nil, ErrSoloLectura }

// DecodeAll: lo mismo.
func DecodeAll(r io.Reader) (*GIF, error) { return nil, ErrSoloLectura }
