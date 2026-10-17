// Prueba diferencial del CRC-32 de la consola contra el de la biblioteca de Go.
//
// POR QUE IMPORTA: el CRC-32 es la suma de control con la que el formato gzip cierra el archivo (los
// ultimos 8 bytes son el CRC de lo que se comprimio y su tamano). Si la suma sale distinta, el archivo
// gzip lo RECHAZA cualquier descompresor, incluido el de Go: no es un detalle, es que no abre. Y una suma
// de control mal calculada es de las cosas que no se notan hasta que alguien mas intenta abrir el archivo.
//
// El caso que mas importa es justamente el raro: la suma se lleva INVERTIDA mientras se calcula y se
// invierte al final, asi que un error ahi da una suma que "parece" bien (es un numero de 32 bits) pero no
// coincide con la de nadie.
package difftest

import (
	gocrc32 "hash/crc32"
	"testing"

	"d2std/hash/crc32"
)

// TestCRC32ContraGo: comparar contra Go con los casos que ejercitan los bordes de la tabla.
func TestCRC32ContraGo(t *testing.T) {
	casos := [][]byte{
		nil,
		{},
		{0x00},
		{0xff},
		[]byte("a"),
		[]byte("abc"),
		[]byte("hola Papu"),
		[]byte("The quick brown fox jumps over the lazy dog"),
		// los bordes de tamano de un bloque deflate (65535) y un poco mas
		bytesRepetidos(65535, 0x41),
		bytesRepetidos(65536, 0x41),
		bytesRepetidos(70000, 0x7f),
		// todos los valores de byte, para que la tabla se recorra entera
		todosLosBytes(),
	}

	for _, caso := range casos {
		esperado := gocrc32.ChecksumIEEE(caso)
		obtenido := crc32.ChecksumIEEE(caso)
		if esperado != obtenido {
			t.Errorf("ChecksumIEEE de %d bytes = %08x; el de Go da %08x", len(caso), obtenido, esperado)
		}
	}
}

// TestCRC32AcumuladoContraGo: la suma de a pedazos tiene que dar lo mismo que de una sola vez.
//
// El gzip no escribe todo junto: va sumando a medida que le llegan los pedazos. Si la suma acumulada y la
// de una vez no coincidieran, el archivo saldria bien solo cuando lo escriben de un saque.
func TestCRC32AcumuladoContraGo(t *testing.T) {
	datos := []byte("hola Papu, esto lo sumo de a pedacitos para ver si da lo mismo")

	suma := crc32.NewIEEE()
	// de a 7 bytes, que no divide el largo (asi queda un pedazo final distinto)
	for i := 0; i < len(datos); i += 7 {
		fin := i + 7
		if fin > len(datos) {
			fin = len(datos)
		}
		if _, err := suma.Write(datos[i:fin]); err != nil {
			t.Fatalf("Write fallo: %v", err)
		}
	}

	if esperado := gocrc32.ChecksumIEEE(datos); suma.Sum32() != esperado {
		t.Errorf("la suma de a pedazos da %08x; el de Go da %08x", suma.Sum32(), esperado)
	}

	// Reset: volver a empezar tiene que dar la suma de lo nuevo y nada mas
	suma.Reset()
	if _, err := suma.Write([]byte("abc")); err != nil {
		t.Fatalf("Write fallo: %v", err)
	}
	if esperado := gocrc32.ChecksumIEEE([]byte("abc")); suma.Sum32() != esperado {
		t.Errorf("despues de Reset, la suma da %08x; el de Go da %08x", suma.Sum32(), esperado)
	}

	// el tamano y el bloque, como en Go
	if suma.Size() != gocrc32.Size {
		t.Errorf("Size da %d; el de Go da %d", suma.Size(), gocrc32.Size)
	}
}

// TestCRC32SumContraGo: `Sum` pega los 4 bytes de la suma al final (asi la usa el gzip para cerrar).
func TestCRC32SumContraGo(t *testing.T) {
	datos := []byte("el final del archivo")

	nuestra := crc32.NewIEEE()
	deGo := gocrc32.NewIEEE()
	_, _ = nuestra.Write(datos)
	_, _ = deGo.Write(datos)

	unosCuantos := []byte{0xaa, 0xbb}
	a := nuestra.Sum(unosCuantos)
	b := deGo.Sum(unosCuantos)

	if len(a) != len(b) {
		t.Fatalf("Sum devolvio %d bytes y el de Go %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("Sum devolvio % x y el de Go % x", a, b)
		}
	}
}

// bytesRepetidos: un pedazo de n bytes iguales.
func bytesRepetidos(n int, valor byte) []byte {
	d := make([]byte, n)
	for i := range d {
		d[i] = valor
	}
	return d
}

// todosLosBytes: los 256 valores posibles, para que la tabla se recorra entera.
func todosLosBytes() []byte {
	d := make([]byte, 256)
	for i := range d {
		d[i] = byte(i)
	}
	return d
}
