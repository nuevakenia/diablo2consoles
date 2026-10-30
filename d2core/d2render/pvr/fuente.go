//go:build dreamcast

package pvr

import (
	"fmt"
	"image/color"
)

// ============================================================ la fuente (PROVISIONAL)

// LA FUENTE ES PROVISIONAL Y ESTA DECLARADO: el motor dibuja texto con d2font + los DC6 de la interfaz
// (un sprite paletizado por caracter, con su propia paleta), que llegan con el Renderer (DC-002.70). Aca
// hay una fuente de 3x5 dibujada por software, lo minimo para que DrawTextf escriba algo legible en la
// consola y en la prueba de humo.
//
// POR QUE 3x5: un caracter ocupa 15 bits, asi que la tabla entera entra en un switch sin datos ni init en
// el arranque (en la consola cada byte de .rodata cuenta). Cada glifo son 5 filas de 3 pixeles, de arriba
// abajo, con el bit de mas peso a la IZQUIERDA.
const (
	fuenteAncho = 3
	fuenteAlto  = 5
	fuentePaso  = 4 // 3 de ancho + 1 de separacion
)

// glifo: los 5 renglones de 3 bits de un caracter. El segundo valor dice si existe.
func glifo(c rune) (uint16, bool) {
	switch c {
	case '0':
		return filas(7, 5, 5, 5, 7), true
	case '1':
		return filas(2, 6, 2, 2, 7), true
	case '2':
		return filas(7, 1, 7, 4, 7), true
	case '3':
		return filas(7, 1, 7, 1, 7), true
	case '4':
		return filas(5, 5, 7, 1, 1), true
	case '5':
		return filas(7, 4, 7, 1, 7), true
	case '6':
		return filas(7, 4, 7, 5, 7), true
	case '7':
		return filas(7, 1, 1, 1, 1), true
	case '8':
		return filas(7, 5, 7, 5, 7), true
	case '9':
		return filas(7, 5, 7, 1, 7), true
	case 'A':
		return filas(7, 5, 7, 5, 5), true
	case 'B':
		return filas(6, 5, 6, 5, 6), true
	case 'C':
		return filas(7, 4, 4, 4, 7), true
	case 'D':
		return filas(6, 5, 5, 5, 6), true
	case 'E':
		return filas(7, 4, 7, 4, 7), true
	case 'F':
		return filas(7, 4, 7, 4, 4), true
	case 'G':
		return filas(7, 4, 5, 5, 7), true
	case 'H':
		return filas(5, 5, 7, 5, 5), true
	case 'I':
		return filas(7, 2, 2, 2, 7), true
	case 'J':
		return filas(1, 1, 1, 5, 7), true
	case 'K':
		return filas(5, 5, 6, 5, 5), true
	case 'L':
		return filas(4, 4, 4, 4, 7), true
	case 'M':
		return filas(5, 7, 7, 5, 5), true
	case 'N':
		return filas(6, 5, 5, 5, 5), true
	case 'O':
		return filas(7, 5, 5, 5, 7), true
	case 'P':
		return filas(7, 5, 7, 4, 4), true
	case 'Q':
		return filas(7, 5, 5, 7, 1), true
	case 'R':
		return filas(7, 5, 7, 6, 5), true
	case 'S':
		return filas(7, 4, 7, 1, 7), true
	case 'T':
		return filas(7, 2, 2, 2, 2), true
	case 'U':
		return filas(5, 5, 5, 5, 7), true
	case 'V':
		return filas(5, 5, 5, 5, 2), true
	case 'W':
		return filas(5, 5, 7, 7, 5), true
	case 'X':
		return filas(5, 5, 2, 5, 5), true
	case 'Y':
		return filas(5, 5, 2, 2, 2), true
	case 'Z':
		return filas(7, 1, 2, 4, 7), true
	case '.':
		return filas(0, 0, 0, 0, 2), true
	case ',':
		return filas(0, 0, 0, 2, 4), true
	case ':':
		return filas(0, 2, 0, 2, 0), true
	case '-':
		return filas(0, 0, 7, 0, 0), true
	case '+':
		return filas(0, 2, 7, 2, 0), true
	case '/':
		return filas(1, 1, 2, 4, 4), true
	case '(':
		return filas(1, 2, 2, 2, 1), true
	case ')':
		return filas(4, 2, 2, 2, 4), true
	case '=':
		return filas(0, 7, 0, 7, 0), true
	case '%':
		return filas(5, 1, 2, 4, 5), true
	case '_':
		return filas(0, 0, 0, 0, 7), true
	case ' ':
		return 0, true
	}

	return 0, false
}

// filas: arma el glifo a partir de sus 5 renglones.
func filas(f0, f1, f2, f3, f4 uint16) uint16 {
	return f0<<12 | f1<<9 | f2<<6 | f3<<3 | f4
}

// dibujarTexto escribe el texto con la fuente 3x5. Los caracteres que no estan en la tabla se SALTEAN (se
// ve un hueco): no se dibuja un cuadradito de "no encontrado" porque el texto del motor no es de este
// backend, y un caracter raro tiene que ser visible como raro, no como un error de dibujado.
func dibujarTexto(dst *Imagen, texto string, x, y int, c color.RGBA) {
	if c.A == 0 {
		c = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	}

	cx := x

	for _, letra := range texto {
		if letra == '\n' {
			cx = x
			y += fuenteAlto + 1

			continue
		}

		g, ok := glifo(letra)
		if ok {
			for fy := 0; fy < fuenteAlto; fy++ {
				fila := (g >> uint(12-3*fy)) & 0x7
				for fx := 0; fx < fuenteAncho; fx++ {
					if fila&(1<<uint(2-fx)) != 0 {
						dst.Poner(cx+fx, y+fy, c)
					}
				}
			}
		}

		cx += fuentePaso
	}
}

// sprintf: el formateo de DrawTextf. Es una funcion aparte para que el que lea DrawTextf no tenga que
// saber que el formateo es de fmt (y para poder cambiarlo si la consola no lo banca).
func sprintf(format string, params ...interface{}) string {
	return fmt.Sprintf(format, params...)
}
