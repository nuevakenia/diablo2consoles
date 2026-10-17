//go:build dreamcast

// DC-002.18: EL INSTRUMENTO. Imprime, en cada serializacion, POR DONDE paso (Marshal, MarshalIndent,
// Unmarshal, Encode, Decode) y SOBRE QUE TIPO, y si ese tipo sabe hacerlo solo.
//
// POR QUE ESTA ACA Y NO EN EL json DE LA CONSOLA: el nombre del tipo no se puede sacar sin reflexion (que
// es justo lo que no hay), asi que hay que leer el descriptor de tipo con unsafe. Ese layout es el de gccgo
// en la consola; el mismo codigo compilado en el host (gc) leeria el nombre como un puntero a cualquier
// lado. El json de la consola solo deja el punto de enganche (`json.Instrumento`); el que imprime es el POC
// de la consola, que es el unico que existe alla.
//
// La lectura del descriptor es LA MISMA de las sondas de DC-002.2f (console/dc/poc/sonda-g), que es donde
// se midio que este layout es el correcto para el runtime de la consola: `size, ptrdata, hash+tflag+align+
// fieldAlign+code, equalfn, gcdata, reflection (el nombre), ptrToThis`, con palabras de 4 bytes.
//
// Es una copia de la de console/dc/poc/json-config/marca.go a proposito: cada POC se arma solo y no
// depende de otro POC para medir.
package main

import (
	"encoding/json"
	"unsafe"
)

type cadenaGo struct {
	data *byte
	len  uintptr
}

type descTipo struct {
	size       uintptr
	ptrdata    uintptr
	hash       uint32
	tflag      uint8
	align      uint8
	fieldAlign uint8
	code       uint8
	equalfn    uintptr
	gcdata     uintptr
	reflection *cadenaGo
	ptrToThis  uintptr
}

// tipoDe: los dos campos de una variable de interfaz vacia son {descriptor del tipo, dato}. Para NOMBRAR
// el tipo hace falta el primero.
func tipoDe(v interface{}) uintptr { return *(*uintptr)(unsafe.Pointer(&v)) }

// nombreDeTipo: el texto del campo `reflection` del descriptor (el nombre del tipo como lo conoce el
// runtime). Si no se puede leer, lo dice: no inventa un nombre.
func nombreDeTipo(v interface{}) string {
	t := tipoDe(v)
	if t == 0 {
		return "nil"
	}

	s := (*descTipo)(unsafe.Pointer(t)).reflection
	if s == nil || s.data == nil || s.len == 0 || s.len > 200 {
		return "sin-nombre"
	}

	b := make([]byte, int(s.len))
	origen := unsafe.Slice(s.data, s.len)

	for i := uintptr(0); i < s.len; i++ {
		b[i] = origen[i]
	}

	return string(b)
}

// nSerializaciones: cuantas serializaciones conto el instrumento en esta corrida.
var nSerializaciones int

// instrumentoJSON: le dice al json de la consola quien imprime las marcas.
func instrumentoJSON() {
	json.Instrumento = func(punto string, v interface{}, sabe bool) {
		nSerializaciones++
		println("JSON-CONSOLA", punto, "tipo=", nombreDeTipo(v), "desc=", tipoDe(v), "sabe=", sabe,
			"n=", nSerializaciones)
	}
}
