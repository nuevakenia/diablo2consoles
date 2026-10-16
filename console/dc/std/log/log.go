// Package log: el minimo que el motor usa (Print/Printf/Println y los Panic), medido al compilar
// d2common/d2datautils para SH-4 en DC-002.3.
//
// Limites declarados: el ENCABEZADO de Go ("2009/01/23 01:23:23 ") no esta. Eso necesita las conversiones
// de calendario (Year/Month/Day/Hour/Minute/Second), que el paquete time de la consola todavia no tiene;
// la libc de KOS las da (gmtime_r esta en kos/time.h), asi que se pueden hacer, pero no estan hechas. Los
// flags y el prefijo se guardan y se devuelven, pero el encabezado no cambia todavia. Tampoco hay Logger
// propio ni niveles. La salida va al mismo destino que fmt.Output, que el arranque de la consola enchufa
// (consola serie o pantalla).
package log

import (
	"d2std/fmt"
	"d2std/io"
	"d2std/os"
)

// Print escribe los operandos; si no terminan en salto de linea, lo agrega (como Go).
func Print(v ...interface{}) { escribir(fmt.Sprint(v...)) }

// Printf escribe el formato.
func Printf(format string, v ...interface{}) { escribir(fmt.Sprintf(format, v...)) }

// Println escribe los operandos separados por espacio y con salto de linea.
func Println(v ...interface{}) { escribir(fmt.Sprintln(v...)) }

// Panic escribe y despues entra en panico (como Go: el texto del panico es el mensaje).
func Panic(v ...interface{}) {
	s := fmt.Sprint(v...)
	escribir(s)
	panic(s)
}

// Panicf escribe con formato y despues entra en panico.
func Panicf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	escribir(s)
	panic(s)
}

// Panicln escribe los operandos y despues entra en panico.
func Panicln(v ...interface{}) {
	s := fmt.Sprintln(v...)
	escribir(s)
	panic(s)
}

// Fatal escribe y despues termina el programa. En la consola no hay procesos que matar: os.Exit se queda
// en un lazo y no vuelve (ver el paquete os), asi que lo que hay despues de un Fatal no corre.
func Fatal(v ...interface{}) {
	escribir(fmt.Sprint(v...))
	os.Exit(1)
}

// Fatalf escribe con formato y despues termina.
func Fatalf(format string, v ...interface{}) {
	escribir(fmt.Sprintf(format, v...))
	os.Exit(1)
}

// Fatalln escribe con salto y despues termina.
func Fatalln(v ...interface{}) {
	escribir(fmt.Sprintln(v...))
	os.Exit(1)
}

// SetOutput cambia donde van los mensajes (es fmt.Output, el mismo destino que usa el resto).
func SetOutput(w io.Writer) { fmt.Output = w }

// Writer devuelve el destino.
func Writer() io.Writer { return fmt.Output }

// SetFlags guarda los flags (el encabezado con fecha todavia no se arma: ver el encabezado del paquete).
func SetFlags(f int) { banderas = f }

// Flags devuelve los flags.
func Flags() int { return banderas }

// SetPrefix guarda el prefijo (si, se usa: va delante del mensaje).
func SetPrefix(p string) { prefijo = p }

// Prefix devuelve el prefijo.
func Prefix() string { return prefijo }

// banderas y prefijo: lo que el motor configura y hoy se refleja solo el prefijo.
var banderas = LstdFlags
var prefijo = ""

// Los flags de Go, para que el motor que los nombre compile. Ver el encabezado: el encabezado con fecha
// no esta armado todavia.
const (
	Ldate = 1 << iota
	Ltime
	Lmicroseconds
	Llongfile
	Lshortfile
	LUTC
	Lmsgprefix
	LstdFlags = Ldate | Ltime
)

func escribir(s string) {
	if len(s) == 0 || s[len(s)-1] != '\n' {
		s += "\n"
	}
	fmt.Fprint(fmt.Output, prefijo+s)
}
