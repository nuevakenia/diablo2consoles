//go:build dreamcast

// Sonda DC-002.2e: separa los cuatro caminos que quedaron en rojo en el autotest de la consola.
//
// Los cuatro casos que fallan (ver evidencia/dc-002.2d-punto-flotantes.txt) son:
//   (1) fmt "%c" con un caracter no ASCII -> byte nulo
//   (2) fmt "%t" sobre un []uint8 (Go formatea elemento por elemento)
//   (3)->(6) strconv.Atoi en los bordes de int64
//   (7) errors.Is con el envoltorio del Errorf %w
//
// Esta sonda no arregla nada: mide POR SEPARADO los caminos de los que depende cada uno, para
// saber si el defecto esta en nuestra implementacion (se caza en el host) o en la plataforma
// (se caza aca, como paso con insertPoint en DC-002.2d).
package main

import (
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"
)

//extern d2c_serial_init
func d2c_serial_init()

type envuelveUno interface{ Unwrap() error }
type envuelveMuchos interface{ Unwrap() []error }

func hex(s string) string { return fmt.Sprintf("%x", s) }

func main() {
	d2c_serial_init()
	println("E0: arranca la sonda de DC-002.2e")

	// ---- camino 1: la conversion rune -> texto (lo que usa %c) ----
	println("E1  string(rune 65))   hex=", hex(string(rune(65))), " esperado 41")
	println("E2  string(rune 127))  hex=", hex(string(rune(127))), " esperado 7f")
	println("E3  string(rune 128))  hex=", hex(string(rune(128))), " esperado c280")
	println("E4  string(rune 233))  hex=", hex(string(rune(233))), " esperado c3a9")
	println("E5  string(rune 255))  hex=", hex(string(rune(255))), " esperado c3bf")
	println("E6  string(rune 0x4E2D)) hex=", hex(string(rune(0x4E2D))), " esperado e4b8ad")
	println("E7  string(rune -1))   hex=", hex(string(rune(-1))), " esperado efbfbd")
	// el mismo camino con nuestra propia codificacion (utf8.EncodeRune del paquete minimo)
	var buf [4]byte
	n := utf8.EncodeRune(buf[:], 255)
	println("E8  utf8.EncodeRune(255) n=", strconv.Itoa(n), "hex=", hex(string(buf[:n])), " esperado 2 c3bf")
	n = utf8.EncodeRune(buf[:], 0x4E2D)
	println("E9  utf8.EncodeRune(0x4E2D) n=", strconv.Itoa(n), "hex=", hex(string(buf[:n])), " esperado 3 e4b8ad")
	var buf2 []byte
	buf2 = utf8.AppendRune(buf2, 233)
	println("E10 utf8.AppendRune(233) len=", strconv.Itoa(len(buf2)), "hex=", hex(string(buf2)), " esperado 2 c3a9")

	// y el verbo en si, que es el caso del corpus
	println("E11 fmt %c de int(255)  hex=", hex(fmt.Sprintf("%c", 255)), " esperado c3bf")
	println("E12 fmt %c de int(233)  hex=", hex(fmt.Sprintf("%c", 233)), " esperado c3a9")
	println("E13 fmt %q de int(255)  =", fmt.Sprintf("%q", 255), " esperado \"\\u00ff\"")
	println("E14 fmt %U de int(255)  =", fmt.Sprintf("%U", 255))

	// ---- camino 2: verbos que no aplican al elemento de un arreglo ----
	println("E15 fmt %t de []byte(abc)   =", fmt.Sprintf("%t", []byte("abc")))
	println("E16 fmt %d de []byte(abc)   =", fmt.Sprintf("%d", []byte("abc")))
	println("E17 fmt %c de []byte(abc)   =", fmt.Sprintf("%c", []byte("abc")))
	println("E18 fmt %v de []byte{0,255,127} =", fmt.Sprintf("%v", []byte{0x00, 0xff, 0x7f}))

	// ---- camino 3: Atoi en los bordes ----
	v, err := strconv.Atoi("9223372036854775807")
	println("E19 Atoi(maxint64) =", strconv.FormatInt(int64(v), 10), "err=", strconv.FormatBool(err != nil))
	v, err = strconv.Atoi("-9223372036854775808")
	println("E20 Atoi(minint64) =", strconv.FormatInt(int64(v), 10), "err=", strconv.FormatBool(err != nil))
	v, err = strconv.Atoi("9223372036854775808")
	println("E21 Atoi(desborde) =", strconv.FormatInt(int64(v), 10), "err=", strconv.FormatBool(err != nil))
	// el tope con el ancho de int de la consola (32 bits): asi se ve si el tope se calcula a 64
	println("E22 ParseInt(desborde,10,32) =", strconv.FormatInt(mustParse("9223372036854775808", 32), 10))
	println("E23 ParseInt(-desborde,10,32) =", strconv.FormatInt(mustParse("-9223372036854775809", 32), 10))
	println("E24 el ancho de int de la consola es 32:", strconv.FormatBool(int64(int(1<<31-1)) == int64(2147483647)))

	// ---- camino 4: errors.Is y las interfaces ----
	base := errors.New("base")
	e := fmt.Errorf("contexto: %w (codigo %d)", base, 7)
	println("E25 Error() de Errorf con %w:", e.Error())
	println("E26 errors.Is(e, base) =", strconv.FormatBool(errors.Is(e, base)))
	println("E27 Unwrap(e) =", fmt.Sprintf("%v", errors.Unwrap(e)))

	var i1, i2 error = base, base
	println("E28 dos variables error con el mismo valor:", strconv.FormatBool(i1 == i2))
	var i3 error = base
	println("E29 error == el error guardado en interface{}:", strconv.FormatBool(i3 == interface{}(base).(error)))

	if m, ok := interface{}(e).(envuelveMuchos); ok {
		println("E30 asercion a Unwrap() []error:", "ok", "cuantos=", strconv.Itoa(len(m.Unwrap())))
	} else {
		println("E30 asercion a Unwrap() []error: FALLO")
	}
	if u, ok := interface{}(e).(envuelveUno); ok {
		println("E31 asercion a Unwrap() error:", "ok", "->", fmt.Sprintf("%v", u.Unwrap()))
	} else {
		println("E31 asercion a Unwrap() error: FALLO")
	}
	switch x := interface{}(e).(type) {
	case envuelveUno:
		println("E32 type switch eligio Unwrap() error ->", fmt.Sprintf("%v", x.Unwrap()))
	case envuelveMuchos:
		println("E32 type switch eligio Unwrap() []error, cuantos=", strconv.Itoa(len(x.Unwrap())))
	default:
		println("E32 type switch: ninguno de los dos")
	}
	// la comparacion de punteros a errorString, que es lo que Is tiene que hacer al final
	e2 := errors.New("base")
	println("E33 dos errorString distintos:", strconv.FormatBool(e2 == interface{}(base).(error)))
	println("E34 e2.Error():", e2.Error(), " base.Error():", base.Error())
	println("E35 fin de la sonda")

	for {
	}
}

func mustParse(s string, bits int) int64 {
	v, _ := strconv.ParseInt(s, 10, bits)
	return v
}
