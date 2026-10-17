// Package crc32: la suma de control CRC-32, la que usa el formato gzip y el zip.
//
// PARA QUE SE USA: el gzip de la consola lo necesita para cerrar el archivo (los ultimos 8 bytes de un
// gzip son el CRC-32 de lo que se comprimio y su tamano). Lo pide d2networking/d2server/d2udpclientconnection,
// que comprime los paquetes de la red.
//
// ES LA CUENTA DE VERDAD: la misma tabla y el mismo polinomio que usa Go (IEEE, 0xEDB88320), asi que un
// archivo armado aca lo DESCOMPRIME el gzip de Go sin protestar. Eso es justamente lo que se prueba.
package crc32

import "hash"

// IEEE: el polinomio que usa gzip (el nombre viene de la norma que lo define).
const IEEE = 0xedb88320

// Size: cuantos bytes ocupa la suma (4).
const Size = 4

// Table: la tabla de 256 valores que acelera la cuenta (se calcula una vez).
type Table [256]uint32

// MakeTable: arma la tabla para un polinomio.
func MakeTable(poly uint32) *Table {
	t := new(Table)
	for i := 0; i < 256; i++ {
		c := uint32(i)
		for j := 0; j < 8; j++ {
			if c&1 != 0 {
				c = poly ^ (c >> 1)
			} else {
				c >>= 1
			}
		}
		t[i] = c
	}
	return t
}

// la tabla de siempre, calculada al arrancar (es lo que hace Go)
var ieeeTable = MakeTable(IEEE)

// digest: la suma que se va acumulando (es un hash.Hash32, la misma forma que en Go).
type digest struct {
	tabla *Table
	suma  uint32
}

// New: una suma nueva con esa tabla.
func New(tabla *Table) hash.Hash32 { return &digest{tabla: tabla, suma: 0} }

// NewIEEE: una suma nueva con la tabla de gzip (es la que se usa).
func NewIEEE() hash.Hash32 { return New(ieeeTable) }

// Write: agrega datos a la suma.
func (d *digest) Write(p []byte) (int, error) {
	d.suma = actualizar(d.suma, d.tabla, p)
	return len(p), nil
}

// Sum32: la suma hasta ahora.
func (d *digest) Sum32() uint32 { return d.suma }

// Reset: vuelve a empezar.
func (d *digest) Reset() { d.suma = 0 }

// Size: cuantos bytes ocupa la suma.
func (d *digest) Size() int { return Size }

// BlockSize: el CRC-32 no agrupa por bloques.
func (d *digest) BlockSize() int { return 1 }

// Sum: agrega los 4 bytes de la suma al final de lo que le pasen (la forma de la interfaz hash.Hash).
func (d *digest) Sum(datos []byte) []byte {
	s := d.suma
	return append(datos, byte(s>>24), byte(s>>16), byte(s>>8), byte(s))
}

// ChecksumIEEE: la suma de un pedazo de datos de una sola vez.
func ChecksumIEEE(datos []byte) uint32 {
	return actualizar(0, ieeeTable, datos)
}

// actualizar: la cuenta del CRC-32 (la de siempre: la tabla y un XOR al final).
//
// OJO CON EL DETALLE QUE ES FACIL EQUIVOCARSE: la suma se lleva INVERTIDA mientras se calcula y se
// invierte al terminar (por eso aparece el ^0xffffffff en los dos extremos). Si se saltea, la suma sale
// al reves y el archivo no lo abre nadie.
func actualizar(suma uint32, tabla *Table, datos []byte) uint32 {
	c := ^suma
	for _, b := range datos {
		c = tabla[byte(c)^b] ^ (c >> 8)
	}
	return ^c
}
