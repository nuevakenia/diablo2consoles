// ebitenutil: las ayudas de dibujo de Ebitengine, para la consola.
//
// PARA QUE SE USA EN EL JUEGO: d2core/d2render/ebiten las usa para lo de siempre: escribir texto de
// depuracion (DebugPrint), dibujar lineas y rectangulos (DrawLine, DrawRect) y cargar una imagen de un
// archivo (NewImageFromFile). Son herramientas del que esta programando, no del juego.
//
// LO QUE HACE ESTA VERSION, dicho claro:
//
//   - DrawLine y DrawRect: DIBUJAN de verdad, sobre la imagen de la consola, que es la que el backend
//     manda al PVR. O sea que se ven en la pantalla.
//
//   - DebugPrint: en la consola la depuracion NO se escribe sobre el juego, se escribe en la CONSOLA SERIE
//     (que es el equivalente exacto de la Dreamcast a la ventana de debug de la PC: es donde uno mira los
//     mensajes). Asi que DebugPrint imprime por ahi, que es mas util que tapar el juego con letras.
//
//   - NewImageFromFile: devuelve error. En la consola las imagenes NO se cargan por nombre de archivo del
//     sistema: salen del asset manager, que las lee del romdisk por el manifiesto (asi ya funciona el POC
//     del menu, con el DC6 del fondo). Cargar "por ruta" es de la version de escritorio.
//
// MEDIDO: el motor pide estos cuatro nombres (grep de ebitenutil.* en d2core/d2render/ebiten).
package ebitenutil

import (
	"errors"
	"image"
	"image/color"

	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
)

// ErrSinArchivos: en la consola las imagenes no se cargan por ruta (ver arriba).
var ErrSinArchivos = errors.New("ebitenutil: en la consola las imagenes salen del asset manager (romdisk), no de una ruta del sistema")

// DebugPrint: escribe un texto en la esquina de arriba a la izquierda. En la consola va a la consola serie.
func DebugPrint(imagen *ebiten.Image, texto string) {
	DebugPrintAt(imagen, texto, 0, 0)
}

// DebugPrintAt: lo mismo, en la posicion que se le diga. La posicion se imprime junto al texto para no
// perderla (en la consola serie no hay coordenadas).
func DebugPrintAt(imagen *ebiten.Image, texto string, x, y int) {
	fmt.Printf("[%d,%d] %s\n", x, y, texto)
}

// DrawLine: una linea entre dos puntos (el algoritmo de Bresenham: el de siempre para lineas enteras).
func DrawLine(dst *ebiten.Image, x1, y1, x2, y2 float64, clr color.Color) {
	ix1, iy1 := int(x1), int(y1)
	ix2, iy2 := int(x2), int(y2)

	dx := ix2 - ix1
	dy := iy2 - iy1
	sx, sy := 1, 1
	if dx < 0 {
		dx = -dx
		sx = -1
	}
	if dy < 0 {
		dy = -dy
		sy = -1
	}

	err := dx - dy
	for {
		pintar(dst, ix1, iy1, clr)
		if ix1 == ix2 && iy1 == iy2 {
			return
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			ix1 += sx
		}
		if e2 < dx {
			err += dx
			iy1 += sy
		}
	}
}

// DrawRect: el CONTORNO de un rectangulo (en ebitenutil es el contorno, no el relleno).
func DrawRect(dst *ebiten.Image, x, y, ancho, alto float64, clr color.Color) {
	x0, y0 := int(x), int(y)
	x1, y1 := int(x+ancho-1), int(y+alto-1)

	for px := x0; px <= x1; px++ {
		pintar(dst, px, y0, clr)
		pintar(dst, px, y1, clr)
	}
	for py := y0; py <= y1; py++ {
		pintar(dst, x0, py, clr)
		pintar(dst, x1, py, clr)
	}
}

// DrawFilledRect: el rectangulo RELLENO (lo usa el panel de mensajes del juego).
func DrawFilledRect(dst *ebiten.Image, x, y, ancho, alto float64, clr color.Color) {
	x0, y0 := int(x), int(y)
	x1, y1 := int(x+ancho-1), int(y+alto-1)
	for py := y0; py <= y1; py++ {
		for px := x0; px <= x1; px++ {
			pintar(dst, px, py, clr)
		}
	}
}

// NewImageFromFile: en la consola no se cargan imagenes por ruta (ver arriba).
func NewImageFromFile(ruta string) (*ebiten.Image, image.Image, error) {
	return nil, nil, ErrSinArchivos
}

// pintar: pone un pixel si cae dentro de la imagen.
func pintar(dst *ebiten.Image, x, y int, clr color.Color) {
	limites := dst.Bounds()
	if x < limites.Min.X || x >= limites.Max.X || y < limites.Min.Y || y >= limites.Max.Y {
		return
	}
	r, g, b, a := clr.RGBA()
	dst.SetRGBA(x, y, color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)})
}
