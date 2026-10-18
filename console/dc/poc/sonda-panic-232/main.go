//go:build dreamcast

// DC-002.32: LA TRAZA DE UN PANIC DE VERDAD (el instrumento de la palanca del .eh_frame).
//
// Por que existe: la palanca mas barata del peso del binario es el `.eh_frame` (1.073.108 bytes = 6,4% de la
// RAM de la consola: son las tablas de desenrollado de excepciones, y el motor no usa excepciones de C++).
// Pero NO es gratis sacarlas: gccgo las puede necesitar para los `panic`/`recover`, y el motor SI usa
// panics. Por eso la tarjeta exige su propia medicion: un panic de verdad tiene que seguir dando su traza
// DESPUES del recorte; si la pierde, la palanca se revierte.
//
// Este POC es ese instrumento y nada mas: arma una pila de llamadas de cuatro niveles (para que la traza
// tenga de donde leer algo) y paniquea en el fondo. Lo que la consola serie imprima es el dato: el mensaje
// del panic, la goroutine, la traza y la linea de memoria. Se corre DOS veces, con el mismo binario enlazado
// con y sin `.eh_frame`, y las dos salidas se comparan.
//
// OJO con el diagnostico: la traza del runtime de la consola puede no salir por falta de informacion de
// desenrollado (el runtime avisa "frame pointers not enabled!" cuando no puede leer la pila). Eso es una
// propiedad del BINARIO, no del `.eh_frame`: lo que la tarjeta mide es si el recorte la EMPEORA. Si la traza
// ya no sale (por la razon que sea) y despues del recorte tampoco, el recorte no es lo que la tira.
package main

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_arranque_marca
func d2cArranqueMarca(que uint) uint

// relleno: una variable de paquete para que haya datos entre las funciones y la pila no salga toda igual.
var relleno = [64]byte{1: 7, 33: 9}

// nivel4: el fondo de la pila, donde esta el panic.
func nivel4(n int) int {
	if n <= 0 {
		panic("DC-002.32: un panic de verdad")
	}

	return nivel4(n-1) + int(relleno[n%64])
}

func nivel3(n int) int { return nivel4(n) }
func nivel2(n int) int { return nivel3(n) }
func nivel1(n int) int { return nivel2(n) }

func main() {
	d2cSerialInit()

	println("PANIC-232 arranca (DC-002.32: la traza de un panic de verdad)")
	d2cArranqueMarca(1)

	println("PANIC-232 llamando al panic desde nivel1")

	x := nivel1(3)

	println("PANIC-232 esto no deberia salir (¿hay recover?):", x)
}
