//go:build dreamcast

// SONDA-GC-277 -- EL RECOLECTOR DE ESTA CONSOLA DEVUELVE MEMORIA?
//
// POR QUE EXISTE. La medicion de DC-002.77 (POC `zonas-waypoint`) midio que entrar al pueblo cuesta
// ~7,4 MB de monton y que la segunda zona ya no entra: el runtime muere con `gc_external_alloc: OOM`.
// Antes de echarle la culpa al CACHE hay que saber si el runtime de la consola RECUPERA lo que ya no se
// usa. Y hay un dato que asusta: en esa corrida `runtime.GC()` dejo `uordblks` EXACTAMENTE igual, byte
// por byte, y `runtime.ReadMemStats` ya se midio en cero (DC-002.9, `poc/goroutines`).
//
// QUE MIDE. Tres formas de tirar basura y ver si el monton la recupera, con el mismo instrumento
// (mallinfo().uordblks, el "usado" del monton de C, que es donde el runtime de la consola pide memoria:
// el OOM de la consola es literalmente un `sbrk` fallido del recolector):
//
//   A. 12 trozos de 256 KB, uno detras de otro, sin pedir el GC  -> si el monton crece 3 MB, no recupera.
//   B. lo mismo pidiendo `runtime.GC()` en cada vuelta            -> si igual crece, el GC no ayuda.
//   C. un solo trozo de 1 MB en un bucle, reasignado              -> si el monton se queda quieto, la
//                                                                   memoria libre SI se reusa cuando el
//                                                                   tamano ya esta pedido.
//
// Lo que salga decide COMO se lee la medicion de las zonas: si el runtime no recupera, el "pico de monton"
// de una carga es el total leido desde el arranque, no lo que el cache conserva.
package main

import "runtime"

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_usado
func d2cUsado() int

//extern d2c_arena
func d2cArena() int

//extern d2c_sbrk
func d2cSbrk() uint

//extern d2c_techo
func d2cTecho() uint

func init() { d2cSerialInit() }

const trozo = 256 * 1024

func main() {
	println("SONDA-GC: arranca -- el recolector de la consola devuelve memoria?")
	println("SONDA-GC: al arrancar usado=", d2cUsado(), "arena=", d2cArena(), "techo=", int(d2cTecho()-d2cSbrk()),
		"bytes libres bajo el techo")

	// -------- A: tirar basura sin pedir el GC --------
	base := d2cUsado()
	println("SONDA-GC: A) 12 trozos de 256 KB, sin GC -- base usado=", base)

	for i := 0; i < 12; i++ {
		b := make([]byte, trozo)
		b[0] = byte(i)
		b[trozo-1] = byte(i)

		println("SONDA-GC: A vuelta=", i, "usado=", d2cUsado(), "delta=", d2cUsado()-base)
		b = nil
	}

	trasA := d2cUsado()
	println("SONDA-GC: A total: 12 x", trozo, "=", 12*trozo, "bytes pedidos; el monton subio", trasA-base, "bytes")

	// -------- B: lo mismo, pidiendo el GC --------
	base = d2cUsado()
	println("SONDA-GC: B) 12 trozos de 256 KB con runtime.GC() -- base usado=", base)

	for i := 0; i < 12; i++ {
		b := make([]byte, trozo)
		b[0] = byte(i)
		b[trozo-1] = byte(i)
		b = nil

		runtime.GC()

		println("SONDA-GC: B vuelta=", i, "usado=", d2cUsado(), "delta=", d2cUsado()-base)
	}

	trasB := d2cUsado()
	println("SONDA-GC: B total: el monton subio", trasB-base, "bytes de los", 12*trozo, "pedidos")

	// -------- C: el mismo tamano una y otra vez --------
	base = d2cUsado()
	println("SONDA-GC: C) 12 trozos de 256 KB REASIGNADOS -- base usado=", base)

	var vivo []byte

	for i := 0; i < 12; i++ {
		vivo = make([]byte, trozo)
		vivo[0] = byte(i)

		println("SONDA-GC: C vuelta=", i, "usado=", d2cUsado(), "delta=", d2cUsado()-base)
	}

	println("SONDA-GC: C total: el monton subio", d2cUsado()-base, "bytes (lo que crezca aca es el pedido de UNO)")

	println("SONDA-GC: al final usado=", d2cUsado(), "arena=", d2cArena())
	println("SONDA-GC: fin")
}
