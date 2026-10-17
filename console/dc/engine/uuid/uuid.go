// Package uuid: los identificadores unicos de google/uuid, reescritos para la consola.
//
// Medido: el motor usa uuid en UN solo lugar y con UN solo simbolo (uuid.New, en d2mapentity/factory.go,
// que es lo que le da identidad a cada entidad del mapa). Nada de Parse, ni de versiones, ni de bytes:
// no tiene sentido portar la libreria entera por eso.
//
// Lo que si importa es lo mismo de siempre: la FORMA. En google/uuid, UUID es un [16]byte y String()
// devuelve "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" en minusculas. Aca igual, porque el juego puede
// estar comparando o guardando esos textos.
//
// De donde sale el azar: del math/rand de la consola (que en la Dreamcast usa el reloj de KOS al
// arrancar). No hace falta que sea impredecible: es para que dos entidades del mapa no choquen.
package uuid

import (
	"math/rand"
)

// UUID: 16 bytes, igual que en google/uuid.
type UUID [16]byte

// New devuelve un identificador nuevo (version 4, como uuid.New).
func New() UUID {
	var u UUID
	for i := 0; i < 16; i += 4 {
		v := rand.Uint32()
		u[i] = byte(v >> 24)
		u[i+1] = byte(v >> 16)
		u[i+2] = byte(v >> 8)
		u[i+3] = byte(v)
	}
	// la version (4) y la variante, como manda el formato
	u[6] = (u[6] & 0x0f) | 0x40
	u[8] = (u[8] & 0x3f) | 0x80
	return u
}

// NewString: el identificador nuevo, ya en texto.
func NewString() string { return New().String() }

// los digitos en hexadecimal, en minusculas (como google/uuid)
const hexDigitos = "0123456789abcdef"

// String: el formato de siempre, "8-4-4-4-12" en minusculas.
func (u UUID) String() string {
	salida := make([]byte, 36)
	j := 0
	for i := 0; i < 16; i++ {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			salida[j] = '-'
			j++
		}
		salida[j] = hexDigitos[u[i]>>4]
		salida[j+1] = hexDigitos[u[i]&0x0f]
		j += 2
	}
	return string(salida)
}

// Bytes: los 16 bytes.
func (u UUID) Bytes() []byte { return u[:] }
