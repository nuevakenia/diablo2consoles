//go:build dreamcast

// DC-002.39b: EL VALOR QUE LA RECOLECCION NO PUEDE VER -- un REGISTRO del que llama, no una ranura de la
// pila.
//
// QUE MIDE ESTE POC (y por que es chiquito): DC-002.39 dejo medido y declarado un residuo en el camino de
// la CONSULTA del corpus (`scripts/console/checar-tablas-233.sh`, control 2b): la consulta de los 21.253
// nombres pierde entre 0 y 6 nombres por corrida, siempre con el mismo sintoma -- el nombre que falla esta
// BIEN FORMADO y el MISMO nombre consultado de nuevo SI se encuentra (`segundo intento contiene= true`).
// O sea: el DATO esta bien y el que falla es el VALOR que el bucle tiene en la mano en la vuelta en la que
// cae una recoleccion.
//
// Lo que se midio en DC-002.39 y lo que NO alcanzo:
//   - el par {puntero leido, 49} se busco palabra por palabra en los 16 MB de RAM y NO EXISTE: el valor no
//     salio de ninguna copia, se leyo de dos REGISTROS (no de una ranura de la pila);
//   - el arreglo vivo, releido por indice en ese mismo instante, estaba impecable (mismo puntero, largo
//     10): el recolector mantiene bien la configuracion;
//   - derramar y recargar los bancos r8..r13 ALREDEDOR de `gc_collect` no alcanzo (es tarde: para entonces
//     los registros ya no son los del que llama);
//   - el tope de 64 KiB del barrido del stack no recorta nada (usado 5.728) y los punteros interiores
//     dentro del arreglo no se rechazan (`RECHAZA= 0`);
//   - el POC `config-viva` (406 vueltas forzando recolecciones, SIN trabajo adentro) NUNCA se corrompe: el
//     agujero necesita que el cuerpo del bucle haga TRABAJO (reservas) con el valor del bucle vivo.
//
// LA HIPOTESIS QUE ESTE POC PONE A PRUEBA (y que hay que medir, no suponer): `gc_scan_stack()` barre la
// PILA desde el SP del recolector hasta el tope del stack y `gc_scan_registers` se borro del runtime ("no
// podia actualizar los registros despues de copiar, era inutil; el barrido del stack caza los registros
// derramados"). Pero un valor vivo en un registro CALLE-SAVED (r8..r14) que ningun marco intermedio
// derrama NO ESTA EN NINGUNA PARTE que el recolector mire: la recoleccion lo mueve y el valor del bucle se
// queda apuntando al semiespacio VIEJO (que se reusa), o sea el largo sale de memoria que ya es de otro
// objeto (de ahi el 49 y el -1795502946).
//
// COMO LO MIDE: arma un buffer grande (el "archivo de configuracion") y `cuantos` strings que son PEDAZOS
// de ese buffer (punteros INTERIORES, como los nombres que deja el decodificador de la capa: es el caso
// que pide el arranque de DC-002.36). El buffer NO se devuelve: queda vivo SOLO por los pedazos. Despues
// recorre los pedazos con `range` (el valor del bucle en un registro) y en cada vuelta hace TRABAJO
// (reservas que disparan recolecciones) ANTES de usar el valor del bucle. La prueba es una sola:
//
//	len(s) == len(vivos[i])  y  ptrDe(s) == ptrDe(vivos[i])
//
// Si el recolector movio el objeto, las dos copias tienen que quedar actualizadas: si el valor del bucle
// no se actualizo (estaba en un registro que nadie barrio), el puntero queda en el semiespacio viejo.
//
// LECTURA DEL RESULTADO: si `fallos= 0` en todas las rondas, el POC NO reproduce el agujero (hay que
// subirle la presion al compilador: mas vivos, mas trabajo, otra forma del bucle). Si hay fallos, la linea
// dice el par medido (leido vs vivo) y el runtime, con su volcado de bancos, dice CUAL registro lo tenia.
//
// La consola serie se abre en csrc/cshim.c (en Go no se puede: `dbgio_dev_select`). El `ebiten` se importa
// por el ENLACE, no por el dibujo: es el que arrastra el paquete `kos` de libgodc, y sin el el binario
// muere con `Unhandled exception` antes de imprimir una linea (medido en DC-002.33).
package main

import (
	"runtime"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_monton_libre
func d2cMontonLibre() int

//extern d2c_ms
func d2cMs() int

const (
	cuantos      = 64          // los "nombres" vivos (punteros interiores al buffer)
	largoNombre  = 13          // como "CRS2LITA11HS"
	trabajoBytes = 6 * 1024    // lo que reserva el cuerpo del bucle (el trabajo del motor)
	ranuras      = 8           // trozos de basura que se ROTAN (el viejo queda basura de verdad)
	trozo        = 40 * 1024   //
	rondas       = 40          //
	maxReportes  = 12          // no mas de 12 lineas de fallo (el log tiene que seguir siendo legible)
)

// suma: acumula algo que sale del trabajo, para que el compilador no se lleve las reservas puestas.
var suma int

// cabecera: la cabecera de un string, para poder fabricar un PEDAZO (un string que apunta al MEDIO de un
// buffer, sin copiar). Portadores uintptr, nunca unsafe.Pointer en un campo: el runtime de la consola no
// tiene los descriptores de unsafe.Pointer y el enlace falla (leccion de la rama DC-002).
type cabecera struct {
	datos uintptr
	largo int
}

// pedazo: el string que apunta al MEDIO del buffer, sin copiar.
func pedazo(crudo []byte, desde, hasta int) string {
	var c cabecera

	c.datos = uintptr(unsafe.Pointer(&crudo[desde]))
	c.largo = hasta - desde

	return *(*string)(unsafe.Pointer(&c))
}

// ptrDe: el puntero de datos del string (el primer campo de su cabecera). UN dato del runtime: sirve para
// ver si quedo apuntando al semiespacio que el recolector acaba de abandonar.
func ptrDe(s string) uintptr {
	return *(*uintptr)(unsafe.Pointer(&s))
}

// carga: el buffer grande (el "archivo de configuracion") y los nombres como PEDAZOS suyos. El buffer NO se
// devuelve: queda vivo solo por los pedazos, como en el camino del json de la capa.
func carga() []string {
	crudo := make([]byte, cuantos*(largoNombre+1))

	for i := 0; i < cuantos; i++ {
		base := i * (largoNombre + 1)
		for j := 0; j < largoNombre; j++ {
			crudo[base+j] = byte('a' + (i+j)%26)
		}
		crudo[base+largoNombre] = byte('\n')
	}

	vivos := make([]string, cuantos)
	for i := 0; i < cuantos; i++ {
		base := i * (largoNombre + 1)
		vivos[i] = pedazo(crudo, base, base+largoNombre)
	}

	return vivos
}

// trabajo: el cuerpo del bucle, con reservas adentro (el trabajo del motor: abrir y leer un MPQ). Devuelve
// un dato del trabajo para que la reserva no se optimice.
func trabajo(n int) int {
	b := make([]byte, n)

	for i := 0; i < n; i += 512 {
		b[i] = byte(i)
	}

	return len(b) + int(b[0])
}

func main() {
	d2cSerialInit()

	println("BANCO-VIVO arranca (DC-002.39b: el valor del bucle que la recoleccion no ve)")
	println("BANCO-VIVO montonC=", d2cMontonLibre(), "ebitenLibre=", ebiten.MemLibreDelMonton())

	vivos := carga()
	println("BANCO-VIVO nombres=", len(vivos), "largo0=", len(vivos[0]), "ptr0=", ptrDe(vivos[0]))

	arranque := d2cMs()
	basura := make([][]byte, ranuras)
	fallos := 0
	comparaciones := 0
	reportes := 0

	for ronda := 1; ronda <= rondas; ronda++ {
		basura[ronda%ranuras] = make([]byte, trozo)
		suma += trabajo(trabajoBytes)

		for i, s := range vivos {
			suma += trabajo(trabajoBytes)

			comparaciones++

			if len(s) != len(vivos[i]) || ptrDe(s) != ptrDe(vivos[i]) {
				fallos++

				if reportes < maxReportes {
					reportes++
					println("BANCO-VIVO-FALLO ronda=", ronda, "i=", i,
						"leidoPtr=", ptrDe(s), "leidoLen=", len(s),
						"vivoPtr=", ptrDe(vivos[i]), "vivoLen=", len(vivos[i]))
				}
			}
		}

		// La recoleccion EXPLICITA cada 4 vueltas: la del umbral la pide el monton solo (y esa es la que
		// importa), pero forzarla garantiza que haya recolecciones tambien con el semiespacio a medio usar.
		if ronda%4 == 0 {
			runtime.GC()
		}

		if ronda%5 == 0 {
			println("BANCO-VIVO ronda=", ronda, "comparaciones=", comparaciones, "fallos=", fallos,
				"ms=", d2cMs()-arranque, "libreC=", d2cMontonLibre())
		}
	}

	println("BANCO-VIVO-TOTAL rondas=", rondas, "comparaciones=", comparaciones, "fallos=", fallos,
		"suma=", suma, "ms=", d2cMs()-arranque, "libreC=", d2cMontonLibre(),
		"ebitenLibre=", ebiten.MemLibreDelMonton())
}
