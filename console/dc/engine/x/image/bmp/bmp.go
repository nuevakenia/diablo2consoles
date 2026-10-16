// Package bmp: el decodificador de BMP de golang.org/x/image, para la consola.
//
// Por que hace falta: los assets de texto del motor (d2common/d2util/assets) son un BMP con los glifos,
// y ese BMP es el que arma la fuente con la que el juego dibuja TODO el texto de la interfaz. Sin esto
// no hay texto en pantalla.
//
// Que soporta: los dos formatos que trae el juego -- 8 bits con paleta y 24 bits -- en sus dos ordenes
// de filas (de arriba a abajo, que es el moderno, y de abajo a arriba, que es el viejo). Los comprimidos
// (RLE) y los de 16/32 bits no estan: si el juego los usa, se ve al medir y se agregan.
package bmp

import (
	"errors"
	"image"
	"image/color"
	"io"
)

// errFormato es el error de "esto no es un BMP que yo entienda".
var errFormato = errors.New("bmp: formato no soportado (se soportan 8 bits con paleta y 24 bits, sin comprimir)")

// Config: la cabecera. Igual que la de x/image/bmp.
type Config struct {
	Width, Height int
	Palette       color.Palette
}

// Decode lee un BMP de r y devuelve la imagen.
func Decode(r io.Reader) (image.Image, error) {
	cfg, imgData, err := decodeConfig(r)
	if err != nil {
		return nil, err
	}
	return imageFromData(cfg, imgData)
}

// DecodeConfig lee solo la cabecera (el motor la usa para saber el tamano antes de decodificar).
func DecodeConfig(r io.Reader) (Config, error) {
	cfg, _, err := decodeConfig(r)
	return cfg, err
}

// leeEntero lee un entero little-endian de n bytes.
func leeEntero(b []byte) uint32 {
	var v uint32
	for i := len(b) - 1; i >= 0; i-- {
		v = v<<8 | uint32(b[i])
	}
	return v
}

// decodeConfig lee la cabecera del archivo y devuelve los datos crudos de los pixeles.
func decodeConfig(r io.Reader) (Config, []byte, error) {
	// La cabecera del archivo: 14 bytes (tipo BM, tamano, reservado, offset de los pixeles)
	cabeceraArchivo := make([]byte, 14)
	if _, err := io.ReadFull(r, cabeceraArchivo); err != nil {
		return Config{}, nil, err
	}
	if cabeceraArchivo[0] != 'B' || cabeceraArchivo[1] != 'M' {
		return Config{}, nil, errors.New("bmp: no empieza con BM")
	}
	offsetPixeles := leeEntero(cabeceraArchivo[10:14])

	// La cabecera de la imagen (BITMAPINFOHEADER, 40 bytes): es la que trae el juego
	cab := make([]byte, 40)
	if _, err := io.ReadFull(r, cab); err != nil {
		return Config{}, nil, err
	}
	tamCab := leeEntero(cab[0:4])
	if tamCab < 40 {
		return Config{}, nil, errors.New("bmp: cabecera vieja (BITMAPCOREHEADER) no soportada")
	}
	ancho := int(int32(leeEntero(cab[4:8])))
	alto := int(int32(leeEntero(cab[8:12])))
	profundidad := leeEntero(cab[14:16])
	compresion := leeEntero(cab[16:20])

	if compresion != 0 {
		return Config{}, nil, errFormato
	}
	if profundidad != 8 && profundidad != 24 {
		return Config{}, nil, errFormato
	}

	// si el alto viene negativo, las filas estan de arriba a abajo (y el alto es el valor absoluto)
	abajoArriba := true
	if alto < 0 {
		alto = -alto
		abajoArriba = false
	}

	cfg := Config{Width: ancho, Height: alto}

	// la paleta (solo en 8 bits): 4 bytes por color (B, G, R, 0)
	if profundidad == 8 {
		// la paleta arranca despues de esta cabecera; si la cabecera es mas grande, se salta el resto
		extra := int(tamCab) - 40
		if extra > 0 {
			if _, err := io.ReadFull(r, make([]byte, extra)); err != nil {
				return Config{}, nil, err
			}
		}
		colores := make([]byte, 256*4)
		if _, err := io.ReadFull(r, colores); err != nil {
			return Config{}, nil, err
		}
		cfg.Palette = make(color.Palette, 256)
		for i := 0; i < 256; i++ {
			cfg.Palette[i] = color.RGBA{
				R: colores[i*4+2],
				G: colores[i*4+1],
				B: colores[i*4+0],
				A: 255,
			}
		}
	}

	// saltar hasta el offset de los pixeles (puede haber relleno)
	leidos := 14 + int(tamCab)
	if offsetPixeles > uint32(leidos) {
		salto := int(offsetPixeles) - leidos
		if _, err := io.ReadFull(r, make([]byte, salto)); err != nil {
			return Config{}, nil, err
		}
	}

	// las filas van alineadas a 4 bytes
	fila := (ancho*int(profundidad)/8 + 3) / 4 * 4
	datos := make([]byte, fila*alto)
	if _, err := io.ReadFull(r, datos); err != nil {
		return Config{}, nil, err
	}

	// si las filas vienen de abajo a arriba, se dan vuelta ahora (asi el resto no se entera)
	if abajoArriba {
		volteado := make([]byte, len(datos))
		for y := 0; y < alto; y++ {
			copy(volteado[y*fila:(y+1)*fila], datos[(alto-1-y)*fila:(alto-y)*fila])
		}
		datos = volteado
	}

	return cfg, datos, nil
}

// imageFromData arma la imagen a partir de los datos ya ordenados.
func imageFromData(cfg Config, datos []byte) (image.Image, error) {
	if len(cfg.Palette) > 0 {
		// 8 bits con paleta
		img := image.NewPaletted(image.Rectangle{
			Min: image.Point{X: 0, Y: 0},
			Max: image.Point{X: cfg.Width, Y: cfg.Height},
		}, cfg.Palette)
		fila := (cfg.Width + 3) / 4 * 4
		for y := 0; y < cfg.Height; y++ {
			for x := 0; x < cfg.Width; x++ {
				img.SetColorIndex(x, y, datos[y*fila+x])
			}
		}
		return img, nil
	}

	// 24 bits
	img := image.NewRGBA(image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: cfg.Width, Y: cfg.Height},
	})
	fila := ((cfg.Width*3 + 3) / 4) * 4
	for y := 0; y < cfg.Height; y++ {
		for x := 0; x < cfg.Width; x++ {
			i := y*fila + x*3
			img.SetRGBA(x, y, color.RGBA{
				R: datos[i+2],
				G: datos[i+1],
				B: datos[i+0],
				A: 255,
			})
		}
	}
	return img, nil
}
