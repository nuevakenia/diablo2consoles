// Paquete hash: la interfaz Hash64 y sus dos hermanas, como en Go.
//
// Por que existe: el motor pasa hashes por interfaz (hash.Hash64 lo usa en un archivo, y adentro
// del motor se usa mas). Es la interfaz de la biblioteca estandar, sin mas.
package hash

// Hash es la interfaz de los hashes de 32 y 64 bits clasicos.
type Hash interface {
	// Write carga datos al hash (nunca devuelve error).
	Write(p []byte) (n int, err error)
	// Sum agrega el hash actual a b sin cambiarlo.
	Sum(b []byte) []byte
	// Reset vuelve al estado inicial.
	Reset()
	// Size devuelve el ancho del hash en bytes.
	Size() int
	// BlockSize devuelve el tamano del bloque.
	BlockSize() int
}

// Hash32 es la interfaz de los hashes de 32 bits.
type Hash32 interface {
	Hash
	Sum32() uint32
}

// Hash64 es la interfaz de los hashes de 64 bits.
type Hash64 interface {
	Hash
	Sum64() uint64
}
