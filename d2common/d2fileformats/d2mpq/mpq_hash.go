package d2mpq

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Hash represents a hashed file entry in the MPQ file
type Hash struct { // 16 bytes
	A          uint32
	B          uint32
	Locale     uint16
	Platform   uint16
	BlockIndex uint32
}

// Name64 returns part A and B as uint64
func (h *Hash) Name64() uint64 {
	return uint64(h.A)<<32 | uint64(h.B)
}

// bytesPorEntrada es lo que ocupa una entrada en cualquiera de las dos tablas del MPQ (16 crudos).
const bytesPorEntrada = 16

// entradaVacia es la marca de "ranura vacia" de la tabla de hash: el sondeo termina ahi.
const entradaVacia uint32 = 0xFFFFFFFF

// hashEntries returns how many slots the hash table has (the stored bytes / 16).
func (mpq *MPQ) hashEntries() uint32 {
	return uint32(len(mpq.hashTable) / bytesPorEntrada)
}

// entradaHash reads slot `idx` of the hash table.
//
// DC-002.33: la tabla son BYTES contiguos y cada consulta arma una `Hash` (un objeto chico por CONSULTA).
// Antes habia un `*Hash` reservado por cada ranura de la tabla y ese arreglo de objetos era, junto con los
// `*Block`, el conjunto VIVO que no entraba en el semiespacio del GC de la consola.
func (mpq *MPQ) entradaHash(idx uint32) Hash {
	base := int(idx) * bytesPorEntrada

	return Hash{
		A:          binary.LittleEndian.Uint32(mpq.hashTable[base:]),
		B:          binary.LittleEndian.Uint32(mpq.hashTable[base+4:]),
		Locale:     binary.LittleEndian.Uint16(mpq.hashTable[base+8:]),
		Platform:   binary.LittleEndian.Uint16(mpq.hashTable[base+10:]),
		BlockIndex: binary.LittleEndian.Uint32(mpq.hashTable[base+12:]),
	}
}

// buscarHash busca `fileName` en la tabla de hash con el SONdeo del formato MPQ: arranca en
// `hashString(nombre, 0) & (ranuras-1)` y avanza de a una ranura hasta la ranura vacia. Devuelve el indice
// de bloque y si lo encontro.
//
// Antes esto era una consulta al mapa `map[uint64]*Hash`, que daba lo mismo porque cada entrada guarda las
// dos mitades del nombre (`hashString(nombre, 1)` y `hashString(nombre, 2)`) en la ranura donde el sondeo
// del nombre la deja.
func (mpq *MPQ) buscarHash(fileName string) (uint32, bool) {
	ranuras := mpq.hashEntries()
	if ranuras == 0 {
		return 0, false
	}

	mascara := ranuras - 1 // la tabla de hash del MPQ es una potencia de dos

	a, b := hashString(fileName, 1), hashString(fileName, 2)

	idx := hashString(fileName, 0) & mascara

	for i := uint32(0); i < ranuras; i++ {
		e := mpq.entradaHash(idx)
		if e.BlockIndex == entradaVacia {
			return 0, false
		}

		if e.A == a && e.B == b {
			return e.BlockIndex, true
		}

		idx = (idx + 1) & mascara
	}

	return 0, false
}

//nolint:gomnd // number
func (mpq *MPQ) readHashTable() error {
	if _, err := mpq.file.Seek(int64(mpq.header.HashTableOffset), io.SeekStart); err != nil {
		return err
	}

	entradas := mpq.header.HashTableEntries
	if entradas > maxTableEntries {
		return fmt.Errorf("hash table entries %d is not sane", entradas)
	}

	// DC-002.33: la tabla ENTERA va como BYTES (16 por ranura) y el que consulta la lee por indice, sin un
	// objeto de Go por entrada. La reserva es UNA sola de `entradas*16` bytes: la de d2data.mpq son
	// 1.048.576 bytes, mas grande que los 64 KiB del umbral de libgodc (`gc_alloc`:
	// `size > GC_LARGE_OBJECT_THRESHOLD` -> `gc_external_alloc`, el monton de C), asi que el semiespacio del
	// GC no la ve. La lectura sigue siendo POR TROZOS (DC-002.28): cada trozo se copia derecho a los bytes.
	mpq.hashTable = make([]byte, int(entradas)*bytesPorEntrada)

	escritas := uint32(0)

	return decryptTablePorTrozos(mpq.file, entradas, "(hash table)",
		func(trozo []uint32) error {
			for _, palabra := range trozo {
				if escritas >= entradas*4 {
					break
				}

				binary.LittleEndian.PutUint32(mpq.hashTable[escritas*4:], palabra)
				escritas++
			}

			return nil
		})
}
