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

// ranurasPorRango: cada cuantas ranuras se guarda el acumulado de ocupadas. Es el bloque que hay que
// recorrer para el rango parcial (64 bytes del mapa a 8 bits por byte = 512 ranuras). Mas chico = rango mas
// barato y tabla de rangos mas grande; 512 deja la tabla de d2data.mpq en 128 x 2 = 256 bytes.
const ranurasPorRango = 512

// DC-002.41: LAS RANURAS VACIAS DE LA TABLA DE HASH NO SE GUARDAN.
//
// POR QUE (medido, ver console/dc/evidencia/dc-002.41-monton-de-c.txt): la tabla de hash de d2data.mpq son
// 65.536 ranuras de 16 bytes y el arranque la pedia COMO UNA SOLA RESERVA CONTIGUA de 1.048.576 bytes al
// monton de C. Con los tres MPQ del disco eso no entra: al monton le quedaban 671.744 bytes de credito de
// `sbrk` (el tope es `_arch_mem_top - THD_KERNEL_STACK_SIZE`) cuando la tabla de d2data.mpq pidio sus
// 1.048.576, y el arranque murio en `gc_external_alloc: OOM` con `arena=1973312 used=1930672 free=42640`.
//
// La tabla del formato es casi toda VACIA: en d2data.mpq hay 11.083 entradas de bloque, o sea a lo sumo
// 11.083 ranuras ocupadas (el 17% de las 65.536). Guardar las 54.453 ranuras vacias (871.248 bytes) es lo
// que no entra. El arreglo guarda las ranuras OCUPADAS en un arreglo compacto y UN BIT por ranura para
// saber donde esta cada una:
//
//	hashRanuras  cuantas ranuras tiene la tabla (lo que antes media `len(hashTable) / 16`)
//	hashOcupadas 1 bit por ranura: 1 = la ranura tiene entrada; 0 = vacia (0xFFFFFFFF en las cuatro palabras)
//	hashEntradas las entradas OCUPADAS, 16 bytes cada una, en el MISMO orden de ranura
//	hashRangos   por cada bloque de `ranurasPorRango` ranuras: cuantas ocupadas hay ANTES del bloque
//
// El RANGO de una ranura (cuantas ocupadas hay antes de ella) es lo que convierte el numero de ranura en el
// indice del arreglo compacto: rango = hashRangos[bloque] + los bits del bloque anteriores a la ranura. Con
// la tabla de d2data.mpq la reserva pasa de 1.048.576 bytes a 11.083*16 + 65.536/8 + 128*2 = 185.776.
//
// LO QUE NO CAMBIA: la consulta (`buscarHash`) es el mismo sondeo del formato MPQ -- arranca en
// `hashString(nombre, 0) & (ranuras-1)` y avanza hasta la ranura vacia -- y el checksum de la tabla
// (`TableChecksum`) sigue siendo el de los BYTES de la tabla LOGICA, ranura por ranura, asi que los dos
// numeros que compara el corpus del host contra la consola (scripts/console/checar-tablas-233.sh) son los
// mismos que con la tabla entera guardada.

// hashEntries returns how many slots the hash table has (the stored bytes / 16).
func (mpq *MPQ) hashEntries() uint32 {
	return mpq.hashRanuras
}

// ranuraOcupada dice si la ranura `idx` tiene entrada guardada.
func (mpq *MPQ) ranuraOcupada(idx uint32) bool {
	return mpq.hashOcupadas[idx>>3]&(1<<(idx&7)) != 0
}

// bitsDelByte: cuantos bits encendidos tiene un byte del mapa de ocupadas (Kernighan: un bucle por bit
// encendido, sin tabla de 256 bytes que reservar).
func bitsDelByte(b byte) uint32 {
	n := uint32(0)

	for b != 0 {
		b &= b - 1
		n++
	}

	return n
}

// rangoDeRanura: cuantas ranuras OCUPADAS hay antes de `idx`. Es el indice de la entrada compacta.
func (mpq *MPQ) rangoDeRanura(idx uint32) uint32 {
	bloque := idx / ranurasPorRango
	r := uint32(mpq.hashRangos[bloque])

	// los bytes COMPLETOS del bloque que quedan antes del byte de `idx`...
	desde := bloque * (ranurasPorRango / 8)
	hasta := idx >> 3

	for b := desde; b < hasta; b++ {
		r += bitsDelByte(mpq.hashOcupadas[b])
	}

	// ...y los bits del mismo byte que estan antes de la ranura
	if m := idx & 7; m != 0 {
		r += bitsDelByte(mpq.hashOcupadas[hasta] & (1<<m - 1))
	}

	return r
}

// entradaHash reads slot `idx` of the hash table.
//
// DC-002.33: la tabla son BYTES contiguos y cada consulta arma una `Hash` (un objeto chico por CONSULTA).
// Antes habia un `*Hash` reservado por cada ranura de la tabla y ese arreglo de objetos era, junto con los
// `*Block`, el conjunto VIVO que no entraba en el semiespacio del GC de la consola.
//
// DC-002.41: los bytes guardados son los de las ranuras OCUPADAS (ver el comentario del arreglo, arriba) y el
// numero de ranura se traduce a su indice compacto con `rangoDeRanura`. Una ranura vacia (o una fuera de la
// tabla) devuelve la marca de vacia, que es lo que corta el sondeo de `buscarHash` igual que antes.
func (mpq *MPQ) entradaHash(idx uint32) Hash {
	if idx >= mpq.hashRanuras || !mpq.ranuraOcupada(idx) {
		// la ranura vacia del formato tiene la marca en las CUATRO palabras (`ranuraVacia`): devolverla
		// entera es lo mismo que leia el camino de la tabla completa, y su BlockIndex sigue cortando el
		// sondeo de `buscarHash`.
		return Hash{A: entradaVacia, B: entradaVacia, Locale: 0xFFFF, Platform: 0xFFFF, BlockIndex: entradaVacia}
	}

	base := int(mpq.rangoDeRanura(idx)) * bytesPorEntrada

	return Hash{
		A:          binary.LittleEndian.Uint32(mpq.hashEntradas[base:]),
		B:          binary.LittleEndian.Uint32(mpq.hashEntradas[base+4:]),
		Locale:     binary.LittleEndian.Uint16(mpq.hashEntradas[base+8:]),
		Platform:   binary.LittleEndian.Uint16(mpq.hashEntradas[base+10:]),
		BlockIndex: binary.LittleEndian.Uint32(mpq.hashEntradas[base+12:]),
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

// sumaDeTablaHash: la suma `suma = suma*31 + byte` de los bytes de la tabla de hash LOGICA (las ranuras tal
// como salieron del archivo, las vacias incluidas), armada ranura por ranura desde la representacion
// compacta. Es lo que mide `TableChecksum`: el numero NO cambia con el arreglo (esa es la mitad de host y
// consola del corpus de DC-002.33) y la cuenta se hace sin volver a reservar la tabla.
func (mpq *MPQ) sumaDeTablaHash() uint32 {
	suma := uint32(0)
	siguiente := 0 // la proxima entrada OCUPADA del arreglo compacto

	for i := uint32(0); i < mpq.hashRanuras; i++ {
		if mpq.ranuraOcupada(i) {
			base := siguiente * bytesPorEntrada
			for j := 0; j < bytesPorEntrada; j++ {
				suma = suma*31 + uint32(mpq.hashEntradas[base+j])
			}

			siguiente++

			continue
		}

		for j := 0; j < bytesPorEntrada; j++ {
			suma = suma*31 + uint32(0xFF)
		}
	}

	return suma
}

// ranuraVacia: una ranura vacia es la marca en las CUATRO palabras (asi la escribe el formato). Se mira
// entera -- y no solo el BlockIndex -- porque el checksum de la tabla es el de sus bytes: una ranura con el
// BlockIndex vacio pero el resto con datos (que el formato no escribe) se guarda tal cual.
func ranuraVacia(palabras []uint32) bool {
	for _, p := range palabras {
		if p != entradaVacia {
			return false
		}
	}

	return true
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

	// DC-002.41: la tabla NO se reserva entera. Lo que se reserva es el mapa de ocupadas (1 bit por ranura:
	// 8 KiB en la tabla mas grande del juego) y las entradas compactas (en d2data.mpq, 11.083 * 16 =
	// 177.328 bytes). La reserva de la tabla entera (1.048.576 bytes en d2data.mpq) es la que no entraba en
	// el monton de C (medido).
	mpq.hashRanuras = entradas
	mpq.hashOcupadas = make([]byte, (int(entradas)+7)/8)

	bloques := (int(entradas) + ranurasPorRango - 1) / ranurasPorRango
	mpq.hashRangos = make([]uint16, bloques)

	// El arreglo compacto se reserva DE UNA VEZ con la cota de entradas de BLOQUE, que el header trae sin
	// leer nada: cada entrada de bloque tiene su ranura en la tabla de hash (en los tres MPQ del juego son
	// 219 ranuras y 216 entradas, 11.083 y 11.083, y 10.010 y 10.010). Reservar exacto evita la cadena de
	// `append`, que en la consola dejo 905.400 bytes de monton de C para un arreglo final de 185.441 (los
	// intermedios no se reclaman a tiempo; medido en DC-002.41). Si el archivo trae mas ranuras ocupadas que
	// entradas de bloque -- que el formato no hace -- `append` sigue creciendo solo.
	tope := int(mpq.header.BlockTableEntries)
	if tope > int(entradas) {
		tope = int(entradas)
	}

	if tope > maxTableEntries {
		tope = maxTableEntries
	}

	mpq.hashEntradas = make([]byte, 0, tope*bytesPorEntrada)

	// el acumulador de UNA entrada (4 palabras) y la ranura que se esta armando
	var palabras [4]uint32

	enEntrada := 0
	idx := uint32(0)
	ocupadas := 0

	// La lectura sigue siendo POR TROZOS (DC-002.28): cada trozo ya viene descifrado y se procesa entrada por
	// entrada, sin guardar la tabla cruda.
	//
	// OJO CON LA COLA: si el archivo viene cortado a mitad de una ranura, esa ranura queda como VACIA (no se
	// guarda a medias). No cambia nada del camino sano -- una tabla cortada devuelve error y el que llama
	// descarta el MPQ (ver `FromFile`) -- y queda declarado aca.
	return decryptTablePorTrozos(mpq.file, entradas, "(hash table)",
		func(trozo []uint32) error {
			for _, palabra := range trozo {
				if idx >= entradas {
					break
				}

				if enEntrada == 0 && idx%ranurasPorRango == 0 {
					mpq.hashRangos[idx/ranurasPorRango] = uint16(ocupadas)
				}

				palabras[enEntrada] = palabra
				enEntrada++

				if enEntrada < 4 {
					continue
				}

				if !ranuraVacia(palabras[:]) {
					mpq.hashEntradas = append(mpq.hashEntradas,
						byte(palabras[0]), byte(palabras[0]>>8), byte(palabras[0]>>16), byte(palabras[0]>>24),
						byte(palabras[1]), byte(palabras[1]>>8), byte(palabras[1]>>16), byte(palabras[1]>>24),
						byte(palabras[2]), byte(palabras[2]>>8), byte(palabras[2]>>16), byte(palabras[2]>>24),
						byte(palabras[3]), byte(palabras[3]>>8), byte(palabras[3]>>16), byte(palabras[3]>>24))

					mpq.hashOcupadas[idx>>3] |= 1 << (idx & 7)
					ocupadas++
				}

				enEntrada = 0
				idx++
			}

			return nil
		})
}
