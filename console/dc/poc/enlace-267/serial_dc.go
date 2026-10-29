//go:build dreamcast

// DC-002.67: lo que existe SOLO en el binario de la consola: la consola serie y el "no te mueras al final".
//
// El mismo main.go tiene que compilar en los dos lados (en el host da la salida de referencia), asi que el
// puente a C y el bucle final viven en este archivo aparte: `go run` en el host lo SALTA por el //go:build, y
// gccgo lo compila igual (gccgo no respeta //go:build: compila todos los .go que se le pasan, medido en
// DC-002.42).
package main

import "kos"

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ram_arena
func d2cRamArena() uint

//extern d2c_ram_libre
func d2cRamLibre() uint

//extern d2c_ram_pedir
func d2cRamPedir(bytes uint) uint

// init corre antes de main: deja la SCIF elegida (para que el println de libgodc salga en el log de Flycast)
// y cambia alTerminar por un bucle que mantiene el programa vivo despues del informe.
func init() {
	d2cSerialInit()

	// Sin esto, al volver main libgodc llama a arch_exit de KOS y Flycast (sin BIOS) reporta
	// "Fatal: SH4 exception when blocked": el binario se muere solo medio segundo despues de imprimir.
	// Con esto la imagen queda corriendo y la corrida se termina cuando el script la corta.
	alTerminar = func() {
		// El dato que decide si el motor entero cabe: lo que queda del monton de KOS despues del arranque
		// (el binario, los dos semiespacios de GC de libgodc y lo que ya se reservo).
		println("ENLACE267: RAM arena=", d2cRamArena(), "libre-en-arena=", d2cRamLibre())
		// el motor va a pedir la pantalla (640x480x4 = 1.228.800 B) y despues las texturas: se mide si el
		// monton puede dar esos bloques DE UNA, que es lo que decide si este binario sirve para el render
		println("ENLACE267: RAM pedir-4MB=", d2cRamPedir(4*1024*1024), "pedir-1MB=", d2cRamPedir(1024*1024),
			"pedir-1.2MB=", d2cRamPedir(1228800), "pedir-256KB=", d2cRamPedir(256*1024))

		for {
			kos.ThdSleep(1000)
		}
	}
}
