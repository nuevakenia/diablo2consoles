//go:build dreamcast

// DC-002.13: EL ARRANQUE DEL MOTOR DE VERDAD en la Dreamcast.
//
// Este POC no dibuja un asset suelto ni un cuadro armado por nosotros: llama al MISMO arranque que usa el
// juego en el host, el de main.go de la raiz del repo:
//
//	instance := d2app.Create(gitBranch, gitCommit)
//	if err := instance.Run(); err != nil { ... }
//
// Lo unico que agrega la consola es lo que el host hace por el sistema operativo: la consola serie (para
// poder leer despues) y el video, por el backend de ebiten de la consola (IniciarPantalla), que es el mismo
// camino que usa el motor para su pantalla.
//
// LO QUE SE MIDE ES HASTA DONDE LLEGA, no que funcione: el arranque del motor pide su configuracion y sus
// MPQ (d2data.mpq son 267.330.019 bytes) y la consola no tiene disco. Donde se caiga (y lo que alcance a
// imprimir por la consola serie, porque el `log` de la stdlib minima sale por la serial) es el dato.
package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/OpenDiablo2/OpenDiablo2/d2app"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_arranque_marca
func d2cArranqueMarca(que uint) uint // marca de avance: escribe en la serial lo que ya paso

func main() {
	d2cSerialInit()

	println("MOTOR-ARRANQUE arranca (DC-002.13: el entry point del motor en la consola)")
	println("MOTOR-ARRANQUE runtime", "sh4")

	// el video: sin esto el motor no tiene pantalla. Se arranca ANTES de Run() para que un fallo del video
	// se vea como un numero (y no como un cuelgue dentro del motor).
	if err := ebiten.IniciarPantalla(640, 480); err != nil {
		println("MOTOR-ARRANQUE el video no arranco:", err.Error())
	} else {
		println("MOTOR-ARRANQUE video listo, VRAM de la pantalla", ebiten.VramDeLaPantalla(), "bytes")
	}

	println("MOTOR-ARRANQUE llamando a d2app.Create().Run()")
	d2cArranqueMarca(1)

	instance := d2app.Create("DC-002.13", "dreamcast")
	d2cArranqueMarca(2)

	if err := instance.Run(); err != nil {
		println("MOTOR-ARRANQUE Run() termino con error:", err.Error())
	} else {
		println("MOTOR-ARRANQUE Run() volvio sin error")
	}

	d2cArranqueMarca(3)
	println("MOTOR-ARRANQUE fin")
}
