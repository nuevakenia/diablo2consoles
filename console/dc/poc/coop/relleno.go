//go:build dreamcast

package main

import "runtime"

// DC-002.87 -- LOS TRES MANDOS DE LA CACERIA: el relleno de `.bss`, la reserva del monton de C y la falla de
// prueba.
//
// POR QUE EXISTEN. DC-002.84 midio que el desenlace del salto a basura no lo decide una condicion del juego:
// dos builds con el mismo codigo de juego, uno pinta y el otro muere, con 32 B de diferencia en el
// `1ST_READ.BIN`, y el que muere llega al primer cuadro con el arena de `malloc` casi sin sobrante (392 KB
// contra 4.016 KB). Para cazar el cuadro que muere con el volcado de registros puesto hacen falta los mandos
// que mueven esas dos variables solas:
//
//	rellenoDeLayout  bytes de `.bss` que no hacen nada. Mueven el comienzo del monton de Go (los dos
//	                 semiespacios se piden con memalign detras del .bss) y el arena de malloc.
//	reservaDeMonton  bytes que se le quitan al monton de C al arrancar y no se devuelven. Reproduce el
//	                 "arena casi sin sobrante" del build que muere SIN cambiar el codigo de juego.
//
// Y uno mas, que no es de la caceria sino del INSTRUMENTO:
//
//	fallaDePrueba    provoca una excepcion de verdad al arrancar (escritura a una direccion que no existe).
//	                 Es la prueba en contra del volcado de registros: si el dump del runtime esta bien
//	                 enganchado, tiene que salir `SONDA-REG FALLO` con los 16 registros. Un instrumento que
//	                 nunca se probo encendido no es evidencia.
//
// Los tres se cambian solo desde `scripts/console/repro-layout-287.sh` (nunca a mano) y el `init` imprime con
// que build se corrio: sin esa linea dos corridas no se pueden comparar.
//
// El `init` toca las cosas a proposito: `len()` se pliega en la compilacion, y una variable global que nadie
// toca se la lleva el enlazador con `--gc-sections` (o sea que el relleno no existiria y la tarjeta estaria
// midiendo otra cosa).
var rellenoDeLayout = [1]byte{}

// reservaDeMonton: bytes que se le quitan al monton de C (0 = ninguno).
const reservaDeMonton = 0

// fallaDePrueba: si esta prendido, el POC se cae a proposito al arrancar para probar el volcado de registros.
const fallaDePrueba = false

// recoleccionesDePrueba (DC-002.87b): cuantas recolecciones FORZADAS se hacen al arrancar, antes de que el
// POC cargue nada. Es el otro eje del acomodo, el que DC-002.84 no movio: una recoleccion de mas o de menos
// cambia QUE SEMIESPACIO esta activo cuando el motor arma el primer cuadro, sin tocar una linea de la logica
// del POC ni del motor. Se cambia solo desde `scripts/console/repro-layout-287.sh` (4o parametro) y el init
// imprime con cuantas se corrio.
var recoleccionesDePrueba = 0

//extern d2c_reservar
func d2cReservar(bytes uint) uint

//extern d2c_falla_de_prueba
func d2cFallaDePrueba()

func init() {
	rellenoDeLayout[0] = 0

	reservados := d2cReservar(reservaDeMonton)

	println("COOP287: RELLENO bytes=", len(rellenoDeLayout), "primero=", int(rellenoDeLayout[0]),
		"reservaPedida=", reservaDeMonton, "reservaQuedo=", int(reservados), "fallaDePrueba=", fallaDePrueba)

	if fallaDePrueba {
		println("COOP287: FALLA DE PRUEBA -- escritura a 0xBAAAAAAD, el volcado tiene que salir AHORA")

		d2cFallaDePrueba()
	}

	// DC-002.87b: el eje de las recolecciones. Va DESPUES del aviso, para que el log diga con cuantas se
	// corrio y para que las recolecciones ocurran con el runtime ya en marcha (que es cuando mueven el
	// semiespacio activo).
	for i := 0; i < recoleccionesDePrueba; i++ {
		runtime.GC()
	}
	if recoleccionesDePrueba > 0 {
		println("COOP287: RECOLECCIONES forzadas=", recoleccionesDePrueba, "antes de cargar nada")
	}
}
