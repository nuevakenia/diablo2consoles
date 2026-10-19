//go:build dreamcast

// SONDA DE LAS CADENAS INTERIORES (DC-002.36).
//
// QUE MIDE: una CADENA de Go puede apuntar al MEDIO de un objeto mas grande (es lo que devuelve
// strings.Fields / strings.Split / el decodificador de json: pedazos del buffer que se leyo). En el Go de
// verdad eso es legal: el recolector mantiene vivo el objeto que CONTIENE el puntero. Esta sonda mide si el
// runtime de la consola lo hace, con NUMEROS y sin depender del POC del menu:
//
//   A  las cadenas son PEDAZOS de una sola reserva grande (una cadena de 20.000 B) que despues nadie
//      referencia: el caso del `strings.Fields(string(datos))` del POC del menu.
//   B  las cadenas se guardan en campos de cajas que SI viven en el monton (el campo `nombre` de cada
//      `textura` del plan del menu).
//   C  CONTROL SANO: las mismas cadenas pero cada una con su PROPIA reserva (una copia): tiene que
//      sobrevivir SIEMPRE, con el defecto y sin el. Si C se cae, el que mide esta mal, no el runtime.
//
// COMO: se toma la huella (CRC-32 de todas las cadenas pegadas) ANTES, se fuerzan recolecciones del GC con
// basura (bloques de 60.000 B que se van reemplazando) y se vuelve a tomar la huella DESPUES. Huella igual =
// las cadenas sobrevivieron; huella distinta = el objeto que las contiene se reciclo (esta ROJO).
//
// Lo que se imprime (la consola serie es la unica salida) lleva el prefijo SONDA-CADENAS para que el corpus
// lo pueda leer: los tres numeros antes, los tres despues y el veredicto de cada caso.
package main

import (
	"hash/crc32"
	"runtime"
	"strconv"
	"strings"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint

const (
	// el tamano del buffer del caso A: 1.200 B con ~215 palabras. Chico a proposito, y esto es MEDIDO:
	// el runtime de la consola ya tiene un conjunto vivo de ~354 KB de los 512 KB del semiespacio
	// (el primer GC de la sonda dispara con 394.040 B usados y la sonda habia reservado ~40 KB), asi que
	// lo que agregue el POC tiene que ser chico o la recoleccion no tiene a donde copiar. Con 20.000 B de
	// buffer y 40 bloques de basura de 60 KB el POC moria en `GC to-space overflow` a los 3,4 s
	// (medido: `Memory: arena=1210420 used=1208572`), sin imprimir ni el "antes".
	tamBuffer = 1200
	// cuantas recolecciones se fuerzan con runtime.GC() (el runtime de la consola tiene su GC y lo expone;
	// antes esto se forzaba con basura y eran 567 recolecciones: lento y con el conjunto vivo creciendo
	// por la ambiguedad del GC conservador).
	recolecciones = 5
	tamSobra      = 2048
)

// sobra: la basura chica que se reserva entre recoleccion y recoleccion (para que el GC tenga algo que
// tirar). Se guarda en una variable de paquete y se toca, o el compilador borra la reserva.
var sobra []byte

// caja: una cadena guardada en un CAMPO de un objeto del monton (el caso B: el campo `nombre` del plan).
type caja struct {
	nombre string
	largo  int
}

// huella: CRC-32 de las cadenas pegadas con un separador. Es el numero que tiene que ser igual antes y
// despues de las recolecciones.
func huella(cs []string) uint32 {
	var todo string

	for i := 0; i < len(cs); i++ {
		todo += cs[i]
		todo += "|"
	}

	return crc32.ChecksumIEEE([]byte(todo))
}

// casoA: un buffer GRANDE (una sola reserva: la cadena que devuelve `string(b)`) del que solo sobreviven
// PEDAZOS. Al volver de aca, nadie referencia la reserva entera: si el runtime no sigue los punteros
// interiores, la proxima recoleccion se la lleva.
func casoA() []string {
	b := make([]byte, tamBuffer)
	p := 0

	for i := 0; p < len(b)-8; i++ {
		palabra := "p" + strconv.Itoa(i) + " "
		if p+len(palabra) > len(b) {
			break
		}

		copy(b[p:], palabra)
		p += len(palabra)
	}

	return strings.Fields(string(b))
}

// casoB: las mismas cadenas, cada una en un CAMPO de una caja (los objetos viven en el monton y los alcanza
// el barrido de la pila). Es el camino PRECISO del GC (el campo de un struct).
func casoB(cs []string) []*caja {
	out := make([]*caja, 0, len(cs))

	for i := 0; i < len(cs); i++ {
		out = append(out, &caja{nombre: cs[i], largo: len(cs[i])})
	}

	return out
}

func nombres(cs []*caja) []string {
	out := make([]string, 0, len(cs))

	for i := 0; i < len(cs); i++ {
		out = append(out, cs[i].nombre)
	}

	return out
}

// suelta: una copia con reserva PROPIA (el puntero apunta al comienzo de un objeto, no al medio de otro).
func suelta(s string) string {
	b := make([]byte, len(s))

	for i := 0; i < len(s); i++ {
		b[i] = s[i]
	}

	return string(b)
}

// casoC (CONTROL SANO): las mismas cadenas, cada una con su propia reserva. No depende de los punteros
// interiores: tiene que sobrevivir con el runtime sano y con el defectuoso.
func casoC(cs []string) []string {
	out := make([]string, 0, len(cs))

	for i := 0; i < len(cs); i++ {
		out = append(out, suelta(cs[i]))
	}

	return out
}

// forzar: hace las recolecciones a mano. `runtime.GC()` del runtime de la consola llama a su recolector
// (el recolector es de libgodc y el paquete `runtime` lo expone con //extern d2c_gc), asi que esto es
// DETERMINISTA: no hay que llenar el semiespacio de basura para que el GC se digne a correr.
func forzar(n int) int {
	dieron := 0

	for i := 0; i < n; i++ {
		sobra = make([]byte, tamSobra)
		sobra[0] = byte(i)
		sobra[tamSobra-1] = byte(i)
		runtime.GC()
		dieron++
	}

	return dieron
}

// veredicto: compara las dos huellas y escribe la linea que lee el corpus.
func veredicto(caso string, antes, despues uint32, cuantas int) {
	if antes == despues {
		println("SONDA-CADENAS:", caso, "OK cadenas=", cuantas, "huella=", int64(antes))
		return
	}

	println("SONDA-CADENAS:", caso, "ROJO cadenas=", cuantas, "antes=", int64(antes), "despues=", int64(despues))
}

func main() {
	d2cSerialInit()

	println("SONDA-CADENAS: arranca (DC-002.36, las cadenas que apuntan al medio de otro objeto)")
	println("SONDA-CADENAS: runtime", runtime.GOOS, runtime.GOARCH)

	t := int64(d2cMs())

	// --- los tres conjuntos de cadenas ---
	a := casoA()          // PEDAZOS de una reserva grande
	b := casoB(a)         // las mismas, en campos de objetos del monton
	c := casoC(a)         // las mismas, cada una con reserva propia (control sano)

	println("SONDA-CADENAS: conjuntos A=", len(a), "B=", len(b), "C=", len(c), "armado_ms=", int64(d2cMs())-t)

	huellaA := huella(a)
	huellaB := huella(nombres(b))
	huellaC := huella(c)

	println("SONDA-CADENAS: ANTES A=", int64(huellaA), "B=", int64(huellaB), "C=", int64(huellaC))

	// --- las recolecciones: si la reserva grande no tiene quien la referencie, aca se recicla ---
	antes := int64(d2cMs())
	dieron := forzar(recolecciones)
	println("SONDA-CADENAS: recolecciones=", dieron, "de", recolecciones, "en", int64(d2cMs())-antes, "ms")

	// la huella de las MISMAS cadenas, ahora
	despuesA := huella(a)
	despuesB := huella(nombres(b))
	despuesC := huella(c)

	println("SONDA-CADENAS: DESPUES A=", int64(despuesA), "B=", int64(despuesB), "C=", int64(despuesC))

	veredicto("A", huellaA, despuesA, len(a))
	veredicto("B", huellaB, despuesB, len(b))
	veredicto("C", huellaC, despuesC, len(c))

	println("SONDA-CADENAS: fin ms=", int64(d2cMs())-t)
}
