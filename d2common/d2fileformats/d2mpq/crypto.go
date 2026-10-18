package d2mpq

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"sync"
)

var cryptoBuffer [0x500]uint32 //nolint:gochecknoglobals // will fix later..
var cryptoOnce sync.Once       //nolint:gochecknoglobals // guards the lazy build: archives are opened from several goroutines

func cryptoLookup(index uint32) uint32 {
	cryptoOnce.Do(cryptoInitialize)

	return cryptoBuffer[index]
}

//nolint:gomnd // Decryption magic
func cryptoInitialize() {
	seed := uint32(0x00100001)

	for index1 := 0; index1 < 0x100; index1++ {
		index2 := index1

		for i := 0; i < 5; i++ {
			seed = (seed*125 + 3) % 0x2AAAAB
			temp1 := (seed & 0xFFFF) << 0x10
			seed = (seed*125 + 3) % 0x2AAAAB
			temp2 := seed & 0xFFFF
			cryptoBuffer[index2] = temp1 | temp2
			index2 += 0x100
		}
	}
}

//nolint:gomnd // Decryption magic
func decrypt(data []uint32, seed uint32) {
	seed2 := uint32(0xeeeeeeee)

	for i := 0; i < len(data); i++ {
		seed2 += cryptoLookup(0x400 + (seed & 0xff))
		result := data[i]
		result ^= seed + seed2

		seed = ((^seed << 21) + 0x11111111) | (seed >> 11)
		seed2 = result + seed2 + (seed2 << 5) + 3
		data[i] = result
	}
}

//nolint:gomnd // Decryption magic
func decryptBytes(data []byte, seed uint32) {
	seed2 := uint32(0xEEEEEEEE)
	for i := 0; i < len(data)-3; i += 4 {
		seed2 += cryptoLookup(0x400 + (seed & 0xFF))
		result := binary.LittleEndian.Uint32(data[i : i+4])
		result ^= seed + seed2
		seed = ((^seed << 21) + 0x11111111) | (seed >> 11)
		seed2 = result + seed2 + (seed2 << 5) + 3

		data[i+0] = uint8(result & 0xff)
		data[i+1] = uint8((result >> 8) & 0xff)
		data[i+2] = uint8((result >> 16) & 0xff)
		data[i+3] = uint8((result >> 24) & 0xff)
	}
}

//nolint:gomnd // Decryption magic
func decryptTable(r io.Reader, size uint32, name string) ([]uint32, error) {
	if size > maxTableEntries {
		return nil, fmt.Errorf("%s has %d entries", name, size)
	}

	tabla := make([]uint32, size*4)
	var escritas uint32

	err := decryptTablePorTrozos(r, size, name, func(trozo []uint32) error {
		copy(tabla[escritas:], trozo)
		escritas += uint32(len(trozo))

		return nil
	})

	return tabla, err
}

// tablaTrozoEntradas: cuantas entradas (16 bytes cada una) se leen y descifran de una vez.
//
// 1.024 entradas son 16 KiB de crudo + 4 KiB de palabras ya descifradas: entra de sobra en el semiespacio
// de 512 KiB del GC de la consola y queda por debajo del umbral de 64 KiB del "objeto grande", que es el
// que se va al monton de C (`gc_external_alloc`) y en la Dreamcast NO entra (medido en DC-002.28).
const tablaTrozoEntradas = 1024

// decryptTablePorTrozos: lee y descifra una tabla del MPQ POR TROZOS, llamando a `porTrozo` con cada trozo
// ya descifrado (palabras de 32 bits) y sin reservar la tabla entera.
//
// POR QUE (DC-002.28): el descifrado de una tabla es una CADENA -- cada palabra depende del `seed` y el
// `seed2` que dejo la anterior -- y eso no impide leerla por partes: el estado se hereda de un trozo al
// siguiente. Lo que no se puede es reservar la tabla entera: la de hash de d2data.mpq son 65.536 entradas
// = 1.048.576 bytes, y el camino de un solo tiron pedia DOS reservas contiguas de ese tamano (el crudo y
// la tabla de `[]uint32`), con el monton de C de la consola en pocos bytes libres (medido: el motor muere
// en `gc_external_alloc: OOM` al abrir ese archivo, con las tres tablas sumando ~4,6 MB de pedidos).
//
// Lo que NO cambia: el resultado es palabra por palabra el mismo que el de un solo tiron (lo compara el
// corpus del host: `crypto_test.go` contra la version palabra por palabra, y `TestTablasPorTrozos` sobre
// un MPQ de verdad armado por `synthMPQSized`).
func decryptTablePorTrozos(r io.Reader, entries uint32, name string, porTrozo func(trozo []uint32) error) error {
	cryptoLookup(0) // make sure the crypto table is built

	// a table entry is four words; a hostile header must not make this allocate gigabytes
	if entries > maxTableEntries {
		return fmt.Errorf("%s has %d entries", name, entries)
	}

	seed := hashString(name, 3)
	seed2 := uint32(0xEEEEEEEE)

	crudo := make([]byte, tablaTrozoEntradas*4*4)
	palabras := make([]uint32, tablaTrozoEntradas*4)

	for restantes := entries; restantes > 0; {
		cuantas := uint32(tablaTrozoEntradas)
		if restantes < cuantas {
			cuantas = restantes
		}

		leidos, err := io.ReadFull(r, crudo[:cuantas*4*4])
		n := uint32(leidos) / 4

		for i := uint32(0); i < n; i++ {
			seed2 += cryptoBuffer[0x400+(seed&0xff)]

			result := binary.LittleEndian.Uint32(crudo[i*4:])
			result ^= seed + seed2

			seed = ((^seed << 21) + 0x11111111) | (seed >> 11)
			seed2 = result + seed2 + (seed2 << 5) + 3
			palabras[i] = result
		}

		if n > 0 {
			if errTrozo := porTrozo(palabras[:n]); errTrozo != nil {
				return errTrozo
			}
		}

		// La tabla cortada (o el archivo cortado) devuelve el error DESPUES de entregar lo que si llego,
		// igual que el camino de un solo tiron: el llamador decide si con eso alcanza.
		if err != nil {
			return err
		}

		restantes -= cuantas
	}

	return nil
}

func hashFilename(key string) uint64 {
	a, b := hashString(key, 1), hashString(key, 2)
	return uint64(a)<<32 | uint64(b)
}

//nolint:gomnd // Decryption magic
func hashString(key string, hashType uint32) uint32 {
	seed1 := uint32(0x7FED7FED)
	seed2 := uint32(0xEEEEEEEE)

	/* prepare seeds. */
	for _, char := range strings.ToUpper(key) {
		seed1 = cryptoLookup((hashType*0x100)+uint32(char)) ^ (seed1 + seed2)
		seed2 = uint32(char) + seed1 + seed2 + (seed2 << 5) + 3
	}

	return seed1
}

//nolint:unused,deadcode,gomnd // will use this for creating mpq's
func encrypt(data []uint32, seed uint32) {
	seed2 := uint32(0xeeeeeeee)

	for i := 0; i < len(data); i++ {
		seed2 += cryptoLookup(0x400 + (seed & 0xff))
		result := data[i]
		result ^= seed + seed2

		seed = ((^seed << 21) + 0x11111111) | (seed >> 11)
		seed2 = data[i] + seed2 + (seed2 << 5) + 3
		data[i] = result
	}
}

// maxTableEntries bounds the hash and block tables (16 bytes an entry); real archives have under 100k.
const maxTableEntries = 1 << 22
