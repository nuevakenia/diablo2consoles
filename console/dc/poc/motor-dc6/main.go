//go:build dreamcast

// DC-002.3: el primer paquete REAL del motor corriendo en la Dreamcast emulada.
//
// No hay codigo del motor copiado aca: el POC importa d2dc6 y el paquete del resumen tal como estan
// en el repo, los compila sh-elf-gccgo (scripts/console/build-motor-gox.sh) y los enlaza. Lo unico
// que aporta la consola son los bytes del DC6, horneados del MPQ al build (bake-dc6) porque aca no
// hay sistema de archivos.
//
// La evidencia es la consola serie: las lineas "MOTOR-DC6 ..." tienen que ser identicas a las que
// imprime el comparador del host sobre el mismo DC6 (scripts/console/compare-motor-dc6.sh).
package main

import (
	"strconv"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/motorresumen"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
)

//extern d2c_serial_init
func d2c_serial_init()

func main() {
	d2c_serial_init()

	println("MOTOR-DC6 arranca: el decodificador DC6 del motor (DC-002.3)")
	println("MOTOR-DC6 origen=consola bytes=", strconv.Itoa(len(assetDC6)))

	d, err := d2dc6.Load(assetDC6)
	if err != nil {
		println("MOTOR-DC6 FALLO parseando:", err.Error())
	} else {
		for _, l := range motorresumen.Lineas(d) {
			println(l)
		}
	}

	println("MOTOR-DC6 fin")

	// el programa se queda vivo para que la consola serie termine de vaciarse
	for {
	}
}
