//go:build dreamcast

package d2loader

import "unsafe"

// DC-002.43 -- el lado CONSOLA de las marcas de carga (ver la cabecera de `marca_carga.go`).
//
// Este archivo SOLO entra en el binario de la consola: el `//go:build dreamcast` lo saca del build del host
// (el `go` de verdad respeta el tag) y gccgo -- que NO respeta tags -- lo compila siempre, que es justo lo
// que hace falta. El que se saca a mano del build de la consola es el otro: `marca_carga_host.go`.
//
// `d2c_marca_carga` la define `console/dc/std/csrc/marcas_carga.c`, que va en TODOS los binarios de la
// consola (el C de la stdlib lo enlazan build-dc.sh y build-motor-arranque.sh), asi que cualquier POC que
// enlace el motor tiene el simbolo.

//extern d2c_marca_carga
func d2cMarcaCarga(punto int, ruta *byte, largo uint32)

// cabeceraCadena es como gccgo guarda un string: puntero a los bytes y largo. Se lee por unsafe para pasar
// los bytes al C SIN COPIARLOS (una copia seria una reserva en el monton justo cuando se esta midiendo el
// monton). El GC no corre adentro de la llamada al C (el C no reserva en el monton de Go), asi que los
// bytes no se mueven mientras el C los lee.
type cabeceraCadena struct {
	datos *byte
	largo uintptr
}

// Marca avisa un punto de la carga: lo imprime el C con la memoria del momento (marcas_carga.c).
func Marca(ruta string, punto int) {
	c := (*cabeceraCadena)(unsafe.Pointer(&ruta))
	d2cMarcaCarga(punto, c.datos, uint32(c.largo))
}

// init: la marca de CONTROL -- pensada para que una corrida sin el instrumento puesto se note sola. OJO,
// MEDIDO en DC-002.43: este `init` NO SE EJECUTA en el binario de la consola. Con estos .gox, los inits de
// los paquetes del motor NO entran en la cadena de arranque: `d2loader..import` (que es donde gccgo mete el
// cuerpo del init) queda DEFINIDO y no lo referencia NADIE en prog.elf (`sh-elf-objdump -d prog.elf | grep
// '1d2loader\.\.import>'` devuelve solo su propia definicion), mientras `d2asset..import` si aparece
// referenciado. Por eso la corrida no imprime esta linea. El control que SI vale es el de la tabla del
// corpus (`tabla-estados-234.py`: "marcas de carga: 0 -- corrida SIN el instrumento"): si no hay NINGUNA
// marca, la corrida no trae el instrumento. Se deja el init igual porque no hace dano (si la cadena de
// arranque cambia, imprime el control) y porque sacarlo cambiaria el .gox que se midio.
func init() {
	Marca("instrumento-de-carga-puesto", MarcaInstrumento)
}
