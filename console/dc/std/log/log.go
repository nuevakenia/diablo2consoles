// Package log: el minimo que el motor usa (Print/Printf/Println y los Panic), medido al compilar
// d2common/d2datautils para SH-4 en DC-002.3.
//
// Limites declarados: no hay fecha ni prefijos (en la consola no hay reloj), no hay Logger propio ni
// niveles. La salida va al mismo destino que fmt.Output, que el arranque de la consola enchufa
// (consola serie o pantalla).
package log

import "d2std/fmt"

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

func escribir(s string) {
	if len(s) == 0 || s[len(s)-1] != '\n' {
		s += "\n"
	}
	fmt.Fprint(fmt.Output, s)
}
