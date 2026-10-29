//go:build dreamcast

package main

// LA LETRA DEL POC (DC-002.61): un tipo de letra de 5x7 pixeles, dibujado a mano.
//
// POR QUE UNO PROPIO Y NO LA DEL JUEGO: la consola no tiene teclado ni motor de texto, y la letra del juego
// (data\local\font\font16.dc6 + la tabla .tbl, `d2common/d2fileformats/d2font`) son ~200 KB mas de heap
// justo en el arranque del PVR, que es la parte fragil de este POC (DC-002.50). El panel del inventario
// necesita escribir CUATRO nombres y un contador: 5x7 alcanza y no toca el MPQ.
//
// Cada glifo son 7 filas de 5 bits: el bit 16 (0b10000) es la columna de la izquierda.

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// glifoDe: la fila de bits de un caracter (en mayusculas). Lo que no esta en la tabla se dibuja como un
// cuadrado vacio, que es lo que hace cualquier fuente de consola.
func glifoDe(c byte) [7]uint8 {
	switch c {
	case 'A':
		return [7]uint8{0b01110, 0b10001, 0b10001, 0b11111, 0b10001, 0b10001, 0b10001}
	case 'B':
		return [7]uint8{0b11110, 0b10001, 0b10001, 0b11110, 0b10001, 0b10001, 0b11110}
	case 'C':
		return [7]uint8{0b01110, 0b10001, 0b10000, 0b10000, 0b10000, 0b10001, 0b01110}
	case 'D':
		return [7]uint8{0b11110, 0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b11110}
	case 'E':
		return [7]uint8{0b11111, 0b10000, 0b10000, 0b11110, 0b10000, 0b10000, 0b11111}
	case 'F':
		return [7]uint8{0b11111, 0b10000, 0b10000, 0b11110, 0b10000, 0b10000, 0b10000}
	case 'G':
		return [7]uint8{0b01110, 0b10001, 0b10000, 0b10111, 0b10001, 0b10001, 0b01111}
	case 'H':
		return [7]uint8{0b10001, 0b10001, 0b10001, 0b11111, 0b10001, 0b10001, 0b10001}
	case 'I':
		return [7]uint8{0b11111, 0b00100, 0b00100, 0b00100, 0b00100, 0b00100, 0b11111}
	case 'J':
		return [7]uint8{0b00111, 0b00010, 0b00010, 0b00010, 0b00010, 0b10010, 0b01100}
	case 'K':
		return [7]uint8{0b10001, 0b10010, 0b10100, 0b11000, 0b10100, 0b10010, 0b10001}
	case 'L':
		return [7]uint8{0b10000, 0b10000, 0b10000, 0b10000, 0b10000, 0b10000, 0b11111}
	case 'M':
		return [7]uint8{0b10001, 0b11011, 0b10101, 0b10101, 0b10001, 0b10001, 0b10001}
	case 'N':
		return [7]uint8{0b10001, 0b11001, 0b10101, 0b10011, 0b10001, 0b10001, 0b10001}
	case 'O':
		return [7]uint8{0b01110, 0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b01110}
	case 'P':
		return [7]uint8{0b11110, 0b10001, 0b10001, 0b11110, 0b10000, 0b10000, 0b10000}
	case 'Q':
		return [7]uint8{0b01110, 0b10001, 0b10001, 0b10001, 0b10101, 0b10010, 0b01101}
	case 'R':
		return [7]uint8{0b11110, 0b10001, 0b10001, 0b11110, 0b10100, 0b10010, 0b10001}
	case 'S':
		return [7]uint8{0b01111, 0b10000, 0b10000, 0b01110, 0b00001, 0b00001, 0b11110}
	case 'T':
		return [7]uint8{0b11111, 0b00100, 0b00100, 0b00100, 0b00100, 0b00100, 0b00100}
	case 'U':
		return [7]uint8{0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b01110}
	case 'V':
		return [7]uint8{0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b01010, 0b00100}
	case 'W':
		return [7]uint8{0b10001, 0b10001, 0b10001, 0b10101, 0b10101, 0b11011, 0b10001}
	case 'X':
		return [7]uint8{0b10001, 0b10001, 0b01010, 0b00100, 0b01010, 0b10001, 0b10001}
	case 'Y':
		return [7]uint8{0b10001, 0b10001, 0b01010, 0b00100, 0b00100, 0b00100, 0b00100}
	case 'Z':
		return [7]uint8{0b11111, 0b00001, 0b00010, 0b00100, 0b01000, 0b10000, 0b11111}
	case '0':
		return [7]uint8{0b01110, 0b10001, 0b10011, 0b10101, 0b11001, 0b10001, 0b01110}
	case '1':
		return [7]uint8{0b00100, 0b01100, 0b00100, 0b00100, 0b00100, 0b00100, 0b01110}
	case '2':
		return [7]uint8{0b01110, 0b10001, 0b00001, 0b00010, 0b00100, 0b01000, 0b11111}
	case '3':
		return [7]uint8{0b11111, 0b00010, 0b00100, 0b00010, 0b00001, 0b10001, 0b01110}
	case '4':
		return [7]uint8{0b00010, 0b00110, 0b01010, 0b10010, 0b11111, 0b00010, 0b00010}
	case '5':
		return [7]uint8{0b11111, 0b10000, 0b11110, 0b00001, 0b00001, 0b10001, 0b01110}
	case '6':
		return [7]uint8{0b00110, 0b01000, 0b10000, 0b11110, 0b10001, 0b10001, 0b01110}
	case '7':
		return [7]uint8{0b11111, 0b00001, 0b00010, 0b00100, 0b01000, 0b01000, 0b01000}
	case '8':
		return [7]uint8{0b01110, 0b10001, 0b10001, 0b01110, 0b10001, 0b10001, 0b01110}
	case '9':
		return [7]uint8{0b01110, 0b10001, 0b10001, 0b01111, 0b00001, 0b00010, 0b01100}
	case '/':
		return [7]uint8{0b00001, 0b00010, 0b00010, 0b00100, 0b01000, 0b01000, 0b10000}
	case ',':
		return [7]uint8{0b00000, 0b00000, 0b00000, 0b00000, 0b01100, 0b01100, 0b01000}
	case '-':
		return [7]uint8{0b00000, 0b00000, 0b00000, 0b11111, 0b00000, 0b00000, 0b00000}
	case '=':
		return [7]uint8{0b00000, 0b00000, 0b11111, 0b00000, 0b11111, 0b00000, 0b00000}
	case '.':
		return [7]uint8{0b00000, 0b00000, 0b00000, 0b00000, 0b00000, 0b01100, 0b01100}
	case ':':
		return [7]uint8{0b00000, 0b01100, 0b01100, 0b00000, 0b01100, 0b01100, 0b00000}
	}

	return [7]uint8{}
}

// dibujarTexto: escribe un texto en la pantalla, con la letra de arriba. `escala` multiplica el glifo (1 =
// 5x7 pixeles, 2 = 10x14). Las minusculas se dibujan como mayusculas.
func dibujarTexto(pantalla *ebiten.Image, x, y, escala int, texto string, r, g, b uint8) {
	cx := x

	for i := 0; i < len(texto); i++ {
		c := texto[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}

		glifo := glifoDe(c)

		for fila := 0; fila < 7; fila++ {
			for col := 0; col < 5; col++ {
				if glifo[fila]&(16>>uint(col)) == 0 {
					continue
				}

				for dy := 0; dy < escala; dy++ {
					for dx := 0; dx < escala; dx++ {
						pantalla.SetRGBA(cx+col*escala+dx, y+fila*escala+dy, color.RGBA{R: r, G: g, B: b, A: 255})
					}
				}
			}
		}

		cx += 6 * escala
	}
}
