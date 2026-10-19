#!/usr/bin/env python3
"""DC-002.33: LAS TABLAS DEL MPQ COMPACTAS (bytes + consulta por indice) -- el arreglo y su mutante.

POR QUE ES UNA SOLA VOLUNTAD (y no un parche a mano): el arreglo de esta tarjeta y su mutante son el MISMO
cambio al reves. El mutante tiene que dejar el motor como estaba (un objeto de Go por entrada: el
`map[uint64]*Hash` y el `[]*Block`) porque ESE es el estado que muere en el semiespacio del GC de la consola
al abrir d2data.mpq, y el arreglo tiene que volver a los bytes. Si los dos textos no salen de aca, la vuelta
se hace a mano y a mano se olvida: el mutante queda pegado y la corrida siguiente mide cualquier cosa.

Uso:  fix-mpq-tablas-233.py --estado       (que dice cada archivo: arreglo, pristino u otro)
      fix-mpq-tablas-233.py --pristino     (el mutante: un objeto de Go por entrada)
      fix-mpq-tablas-233.py --sano         (el arreglo: las tablas como bytes)

Es IDEMPOTENTE: aplicarlo dos veces deja el mismo contenido, y cada modo verifica que los 7 archivos quedaron
en el estado pedido (si alguno queda en otro, sale con error).

DC-002.41: el texto del estado SANO NO es el de DC-002.33 al pie de la letra. La representacion de las tablas
evoluciono -- de la tabla de hash solo se guardan las ranuras OCUPADAS (un bit por ranura + las entradas
compactas + los rangos por bloque), porque la reserva de la tabla ENTERA de d2data.mpq (1.048.576 bytes
contiguos) es la que no entraba en el monton de C del arranque -- y este script es el que declara cual es el
"estado sano" del arbol: si su SANO no fuera el nuevo, `--estado` diria 2/7 y el corpus de DC-002.34 (control
0) se pondria rojo antes de empezar. El PRISTINO (el mutante de un objeto de Go por entrada) es el mismo de
siempre. Los numeros que ese corpus compara (entradas y checksums de las dos tablas) NO cambian: el checksum
de la tabla de hash se arma ranura por ranura sobre los bytes de la tabla logica.
"""

import hashlib
import os
import sys

RAIZ = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

ARCHIVOS = [
    "d2common/d2fileformats/d2mpq/mpq.go",
    "d2common/d2fileformats/d2mpq/mpq_hash.go",
    "d2common/d2fileformats/d2mpq/mpq_block.go",
    "d2common/d2fileformats/d2mpq/explode_test.go",
    "d2common/d2fileformats/d2mpq/fuzz_test.go",
    "d2common/d2fileformats/d2mpq/mpq_trozos_test.go",
    "d2common/d2fileformats/d2mpq/mpq_equivalencia_test.go",
]

os.chdir(RAIZ)

# EL ARREGLO (DC-002.33): las dos tablas del MPQ como bytes contiguos, consultadas por indice.
SANO = {
    'd2common/d2fileformats/d2mpq/mpq.go': r'''package d2mpq

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

var _ d2interface.Archive = &MPQ{} // Static check to confirm struct conforms to interface

// MPQ represents an MPQ archive
type MPQ struct {
	filePath string
	file     *os.File
	// DC-002.33: las dos tablas del MPQ van como BYTES contiguos (16 por entrada) y el que consulta las lee
	// por indice, sin un objeto de Go por entrada. Antes eran `map[uint64]*Hash` + `[]*Block`: ese conjunto
	// VIVO no entraba en el semiespacio del GC de la consola (512 KiB) con las tablas de un archivo del
	// juego (d2data.mpq: 65.536 ranuras de hash + 11.083 de bloque) y el arranque del motor moria ahi.
	//
	// DC-002.41: de la tabla de HASH solo se guardan las ranuras OCUPADAS (un bit por ranura + las entradas
	// compactas + los rangos por bloque; ver `mpq_hash.go`). La reserva de la tabla entera de d2data.mpq
	// (1.048.576 bytes contiguos) es la que NO entraba en el monton de C del arranque con los tres MPQ del
	// disco: el arranque moria en `gc_external_alloc: OOM` pidiendola con 671.744 bytes de credito de `sbrk`.
	// La tabla de BLOQUE sigue como bytes contiguos: sus entradas son densas (no hay ranuras que sacar).
	hashRanuras  uint32   // cuantas ranuras tiene la tabla de hash
	hashOcupadas []byte   // 1 bit por ranura: 1 = tiene entrada
	hashEntradas []byte   // las entradas OCUPADAS, 16 bytes cada una, en orden de ranura
	hashRangos   []uint16 // por bloque de `ranurasPorRango` ranuras: ocupadas ANTES del bloque
	blockTable   []byte
	header       Header
	fileSize     int64 // size of the archive on disk, to reject offsets and sizes beyond it
}

// TableChecksum returns the byte checksum of the two tables of the archive (hash table, block table).
//
// DC-002.33: como las tablas son bytes contiguos, el corpus diferencial del port compara la tabla ENTERA
// entre el host y la consola con dos numeros (antes habia que recorrer el mapa y el arreglo de objetos, que
// en la consola no entraban). La suma es la de los POC del port: `suma = suma*31 + byte`, sobre los bytes.
//
// DC-002.41: la tabla de hash ya no guarda las ranuras vacias, asi que su suma se arma ranura por ranura
// desde la representacion compacta (`sumaDeTablaHash`). El NUMERO es el mismo de siempre -- los bytes que
// suma son los de la tabla logica, con las vacias incluidas -- que es lo que hace que este checksum siga
// sirviendo de barrera entre el host y la consola.
func (mpq *MPQ) TableChecksum() (hashTable, blockTable uint32) {
	return mpq.sumaDeTablaHash(), sumaDeBytes(mpq.blockTable)
}

// TableEntries returns how many slots each table has (what the port prints as "entradas").
func (mpq *MPQ) TableEntries() (hashEntries, blockEntries uint32) {
	return mpq.hashEntries(), mpq.blockEntries()
}

func sumaDeBytes(datos []byte) uint32 {
	suma := uint32(0)
	for _, b := range datos {
		suma = suma*31 + uint32(b)
	}

	return suma
}

// PatchInfo represents patch info for the MPQ.
type PatchInfo struct {
	Length   uint32   // Length of patch info header, in bytes
	Flags    uint32   // Flags. 0x80000000 = MD5 (?)
	DataSize uint32   // Uncompressed size of the patch file
	MD5      [16]byte // MD5 of the entire patch file after decompression
}

// New loads an MPQ file and only reads the header
func New(fileName string) (*MPQ, error) {
	mpq := &MPQ{filePath: fileName}

	var err error
	if runtime.GOOS == "linux" {
		mpq.file, err = openIgnoreCase(fileName)
	} else {
		mpq.file, err = os.Open(fileName) //nolint:gosec // Will fix later
	}

	if err != nil {
		return nil, err
	}

	if st, serr := mpq.file.Stat(); serr == nil {
		mpq.fileSize = st.Size()
	}

	if err := mpq.readHeader(); err != nil {
		mpq.file.Close()

		return nil, fmt.Errorf("failed to read reader: %v", err)
	}

	return mpq, nil
}

// FromFile loads an MPQ file and returns a MPQ structure
func FromFile(fileName string) (*MPQ, error) {
	mpq, err := New(fileName)
	if err != nil {
		return nil, err
	}

	if err := mpq.readHashTable(); err != nil {
		mpq.file.Close()
		return nil, fmt.Errorf("failed to read hash table: %v", err)
	}

	if err := mpq.readBlockTable(); err != nil {
		mpq.file.Close()
		return nil, fmt.Errorf("failed to read block table: %v", err)
	}

	return mpq, nil
}

// getFileBlockData gets a block table entry
func (mpq *MPQ) getFileBlockData(fileName string) (*Block, error) {
	blockIndex, ok := mpq.buscarHash(fileName)
	if !ok {
		return nil, errors.New("file not found")
	}

	if blockIndex >= mpq.blockEntries() {
		return nil, errors.New("invalid block index")
	}

	return mpq.bloqueEn(blockIndex), nil
}

// Close closes the MPQ file
func (mpq *MPQ) Close() error {
	return mpq.file.Close()
}

// ReadFile reads a file from the MPQ and returns a memory stream
func (mpq *MPQ) ReadFile(fileName string) ([]byte, error) {
	fileBlockData, err := mpq.getFileBlockData(fileName)
	if err != nil {
		return []byte{}, err
	}

	// each stream gets its own copy of the block: streams are used from several goroutines
	block := *fileBlockData
	block.FileName = strings.ToLower(fileName)
	fileBlockData = &block

	stream, err := CreateStream(mpq, fileBlockData, fileName)
	if err != nil {
		return []byte{}, err
	}

	buffer := make([]byte, fileBlockData.UncompressedFileSize)
	if _, err := stream.Read(buffer, 0, fileBlockData.UncompressedFileSize); err != nil {
		return []byte{}, err
	}

	return buffer, nil
}

// ReadFileStream reads the mpq file data and returns a stream
func (mpq *MPQ) ReadFileStream(fileName string) (d2interface.DataStream, error) {
	fileBlockData, err := mpq.getFileBlockData(fileName)
	if err != nil {
		return nil, err
	}

	// each stream gets its own copy of the block: streams are used from several goroutines
	block := *fileBlockData
	block.FileName = strings.ToLower(fileName)
	fileBlockData = &block

	stream, err := CreateStream(mpq, fileBlockData, fileName)
	if err != nil {
		return nil, err
	}

	return &MpqDataStream{stream: stream}, nil
}

// ReadTextFile reads a file and returns it as a string
func (mpq *MPQ) ReadTextFile(fileName string) (string, error) {
	data, err := mpq.ReadFile(fileName)

	if err != nil {
		return "", err
	}

	return string(data), nil
}

// Listfile returns the list of files in this MPQ
func (mpq *MPQ) Listfile() ([]string, error) {
	data, err := mpq.ReadFile("(listfile)")

	if err != nil {
		return nil, err
	}

	raw := strings.TrimRight(string(data), "\x00")
	s := bufio.NewScanner(strings.NewReader(raw))

	var filePaths []string

	for s.Scan() {
		filePath := s.Text()
		filePaths = append(filePaths, filePath)
	}

	return filePaths, nil
}

// Path returns the MPQ file path
func (mpq *MPQ) Path() string {
	return mpq.filePath
}

// Contains returns bool for whether the given filename exists in the mpq
func (mpq *MPQ) Contains(filename string) bool {
	_, ok := mpq.buscarHash(filename)
	return ok
}

// Size returns the size of the mpq in bytes
func (mpq *MPQ) Size() uint32 {
	return mpq.header.ArchiveSize
}

func openIgnoreCase(mpqPath string) (*os.File, error) {
	// First see if file exists with specified case
	mpqFile, err := os.Open(mpqPath) //nolint:gosec // Will fix later
	if err != nil {
		mpqName := filepath.Base(mpqPath)
		mpqDir := filepath.Dir(mpqPath)

		var files []fs.FileInfo
		files, err = ioutil.ReadDir(mpqDir)

		if err != nil {
			return nil, err
		}

		for _, file := range files {
			if strings.EqualFold(file.Name(), mpqName) {
				mpqName = file.Name()
				break
			}
		}

		return os.Open(filepath.Join(mpqDir, mpqName)) //nolint:gosec // Will fix later
	}

	return mpqFile, err
}

// maxFileSize bounds the size of one file read into memory (the largest game file is far smaller).
const maxFileSize = 1 << 28

// checkBlock rejects block table entries whose sizes cannot be real, before anything is allocated
// from them.
func (mpq *MPQ) checkBlock(b *Block) error {
	if b.UncompressedFileSize > maxFileSize {
		return fmt.Errorf("file size %d is too large", b.UncompressedFileSize)
	}

	if mpq.fileSize > 0 && int64(b.FilePosition) > mpq.fileSize {
		return errors.New("file starts beyond the end of the archive")
	}

	if !b.HasFlag(FileCompress) && !b.HasFlag(FileImplode) && mpq.fileSize > 0 &&
		int64(b.FilePosition)+int64(b.UncompressedFileSize) > mpq.fileSize {
		return errors.New("file extends beyond the end of the archive")
	}

	return nil
}
''',
    'd2common/d2fileformats/d2mpq/mpq_hash.go': r'''package d2mpq

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
''',
    'd2common/d2fileformats/d2mpq/mpq_block.go': r'''package d2mpq

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

// FileFlag represents flags for a file record in the MPQ archive
type FileFlag uint32

const (
	// FileImplode - File is compressed using PKWARE Data compression library
	FileImplode FileFlag = 0x00000100
	// FileCompress - File is compressed using combination of compression methods
	FileCompress FileFlag = 0x00000200
	// FileEncrypted - The file is encrypted
	FileEncrypted FileFlag = 0x00010000
	// FileFixKey - The decryption key for the file is altered according to the position of the file in the archive
	FileFixKey FileFlag = 0x00020000
	// FilePatchFile - The file contains incremental patch for an existing file in base MPQ
	FilePatchFile FileFlag = 0x00100000
	// FileSingleUnit - Instead of being divided to 0x1000-bytes blocks, the file is stored as single unit
	FileSingleUnit FileFlag = 0x01000000
	// FileDeleteMarker - File is a deletion marker, indicating that the file no longer exists. This is used to allow patch
	// archives to delete files present in lower-priority archives in the search chain. The file usually
	// has length of 0 or 1 byte and its name is a hash
	FileDeleteMarker FileFlag = 0x02000000
	// FileSectorCrc - File has checksums for each sector. Ignored if file is not compressed or imploded.
	FileSectorCrc FileFlag = 0x04000000
	// FileExists - Set if file exists, reset when the file was deleted
	FileExists FileFlag = 0x80000000
)

// Block represents an entry in the block table
type Block struct { // 16 bytes
	FilePosition         uint32
	CompressedFileSize   uint32
	UncompressedFileSize uint32
	Flags                FileFlag
	// Local Stuff...
	FileName       string
	EncryptionSeed uint32
}

// HasFlag returns true if the specified flag is present
func (b *Block) HasFlag(flag FileFlag) bool {
	return (b.Flags & flag) != 0
}

func (b *Block) calculateEncryptionSeed(fileName string) {
	fileName = fileName[strings.LastIndex(fileName, `\`)+1:]
	seed := hashString(fileName, 3)
	b.EncryptionSeed = (seed + b.FilePosition) ^ b.UncompressedFileSize
}

// blockEntries returns how many entries the block table has (the stored bytes / 16).
func (mpq *MPQ) blockEntries() uint32 {
	return uint32(len(mpq.blockTable) / bytesPorEntrada)
}

// bloqueEn reads entry `idx` of the block table.
//
// DC-002.33: la tabla son BYTES y cada consulta arma una `Block`: una por ARCHIVO leido, no una por entrada
// de la tabla. El arreglo de `*Block` (11.083 entradas = 11.083 objetos de Go en d2data.mpq) era lo que
// llenaba el semiespacio del GC de la consola y mataba el arranque del motor.
func (mpq *MPQ) bloqueEn(idx uint32) *Block {
	base := int(idx) * bytesPorEntrada

	return &Block{
		FilePosition:         binary.LittleEndian.Uint32(mpq.blockTable[base:]),
		CompressedFileSize:   binary.LittleEndian.Uint32(mpq.blockTable[base+4:]),
		UncompressedFileSize: binary.LittleEndian.Uint32(mpq.blockTable[base+8:]),
		Flags:                FileFlag(binary.LittleEndian.Uint32(mpq.blockTable[base+12:])),
	}
}

//nolint:gomnd // number
func (mpq *MPQ) readBlockTable() error {
	if _, err := mpq.file.Seek(int64(mpq.header.BlockTableOffset), io.SeekStart); err != nil {
		return err
	}

	entradas := mpq.header.BlockTableEntries
	if entradas > maxTableEntries {
		return fmt.Errorf("block table entries %d is not sane", entradas)
	}

	// DC-002.28: por trozos, igual que la tabla de hash (ver `decryptTablePorTrozos`). La de d2data.mpq son
	// 11.083 entradas = 177.328 bytes: con el camino de un solo tiron eran dos reservas contiguas de ese
	// tamano mas los objetos `Block` de cada entrada.
	//
	// DC-002.33: los trozos se copian a los BYTES de la tabla (177.328 bytes: mas que los 64 KiB del umbral
	// de libgodc, o sea una reserva del monton de C) en vez de armar un `*Block` por entrada.
	mpq.blockTable = make([]byte, int(entradas)*bytesPorEntrada)

	escritas := uint32(0)

	return decryptTablePorTrozos(mpq.file, entradas, "(block table)",
		func(trozo []uint32) error {
			for _, palabra := range trozo {
				if escritas >= entradas*4 {
					break
				}

				binary.LittleEndian.PutUint32(mpq.blockTable[escritas*4:], palabra)
				escritas++
			}

			return nil
		})
}
''',
    'd2common/d2fileformats/d2mpq/explode_test.go': r'''package d2mpq

import (
	"bytes"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/JoshVarga/blast"
)

// blastDecompress is the reference decoder explode replaced.
func blastDecompress(data []byte) (out []byte, err error) {
	defer func() { // the reference panics on some streams the blast writer produces
		if r := recover(); r != nil {
			out, err = nil, errExplodeCode
		}
	}()

	r, err := blast.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	return io.ReadAll(r)
}

// blastCompress compresses with the library's writer, which panics on some inputs.
func blastCompress(data []byte, typ, dict uint) (cmp *bytes.Buffer, ok bool) {
	defer func() {
		if recover() != nil {
			cmp, ok = nil, false
		}
	}()

	cmp = new(bytes.Buffer)
	w := blast.NewWriter(cmp, typ, dict)
	_, _ = w.Write(data)
	_ = w.Close()

	return cmp, true
}

func TestExplodeKnownVector(t *testing.T) {
	got, err := explode([]byte{0x00, 0x04, 0x82, 0x24, 0x25, 0x8f, 0x80, 0x7f}, 0)
	if err != nil || string(got) != "AIAIAIAIAIAIA" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestExplodeMatchesBlast(t *testing.T) {
	rnd := rand.New(rand.NewSource(7))

	gen := map[string]func(n int) []byte{
		"random": func(n int) []byte { b := make([]byte, n); rnd.Read(b); return b },
		"few symbols": func(n int) []byte {
			b := make([]byte, n)
			for i := range b {
				b[i] = byte(rnd.Intn(5))
			}

			return b
		},
		"runs and repeats": func(n int) []byte {
			var b []byte
			for len(b) < n {
				if len(b) > 10 && rnd.Intn(3) == 0 {
					s := rnd.Intn(len(b) - 5)
					b = append(b, b[s:s+2+rnd.Intn(300)%(len(b)-s-1)]...)
				} else {
					b = append(b, bytes.Repeat([]byte{byte(rnd.Intn(256))}, 1+rnd.Intn(40))...)
				}
			}

			return b[:n]
		},
	}

	// a fixed order: the generators share one random stream, and with a random map order some streams make
	// the third party blast reader loop forever (a flaky hang of the whole test binary)
	names := make([]string, 0, len(gen))
	for name := range gen {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		g := gen[name]

		for _, typ := range []uint{blast.Binary, blast.ASCII} {
			for _, dict := range []uint{blast.DictionarySize1024, blast.DictionarySize2048, blast.DictionarySize4096} {
				for _, n := range []int{1, 2, 3, 100, 4095, 4096, 4097, 20000} {
					data := g(n)

					cmp, ok := blastCompress(data, typ, dict)
					if !ok {
						continue
					}

					// the blast writer emits broken streams for some larger incompressible inputs (the old
					// reader fails on them too): only streams the old reader decodes to the input are used
					if ref, err := blastDecompress(cmp.Bytes()); err != nil || !bytes.Equal(ref, data) {
						continue
					}

					for _, hint := range []int{0, n, 10} {
						got, err := explode(cmp.Bytes(), hint)
						if err != nil {
							t.Fatalf("%s/%d/%d/%d: %v", name, typ, dict, n, err)
						}

						if !bytes.Equal(got, data) {
							t.Fatalf("%s/%d/%d/%d: output differs", name, typ, dict, n)
						}
					}
				}
			}
		}
	}
}

func TestExplodeRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
	}{
		{"empty", nil},
		{"bad literal flag", []byte{2, 4, 0, 0}},
		{"bad dictionary", []byte{0, 7, 0, 0}},
		{"truncated", []byte{0x00, 0x04, 0x82, 0x24}},
		{"no end code", bytes.Repeat([]byte{0}, 40)[:40]},
	}

	for _, tt := range tests {
		if tt.name == "no end code" {
			tt.in = append([]byte{0, 4}, tt.in...)
		}

		if _, err := explode(tt.in, 0); err == nil {
			t.Errorf("%s: no error", tt.name)
		}
	}

	// a distance that points before the start of the output
	if _, err := explode([]byte{0x00, 0x04, 0x03, 0x00, 0x00, 0x00}, 0); err == nil {
		t.Error("distance before the start: no error")
	}
}

// TestExplodeRealArchive decodes every file of a game archive with both decoders. It needs
// D2_GAME_DIR (the install folder) and reads archives only.
func TestExplodeRealArchive(t *testing.T) {
	dir := os.Getenv("D2_GAME_DIR")
	if dir == "" {
		t.Skip("D2_GAME_DIR not set")
	}

	m, err := FromFile(filepath.Join(dir, "d2data.mpq"))
	if err != nil {
		t.Skip(err)
	}

	defer m.Close()

	checked := 0

	// DC-002.33: la tabla de hash son BYTES contiguos y se recorre por indice (antes era un mapa).
	if m.hashEntries() == 0 {
		t.Fatal("el archivo no tiene tabla de hash")
	}

	for i, entradas := uint32(0), m.blockEntries(); i < entradas; i++ {
		b := m.bloqueEn(i)
		if b.UncompressedFileSize == 0 || b.UncompressedFileSize > 1<<20 || b.HasFlag(FileEncrypted) || b.HasFlag(FileSingleUnit) {
			continue
		}

		if !b.HasFlag(FileCompress) && !b.HasFlag(FileImplode) {
			continue
		}

		st := &Stream{MPQ: m, Block: b, Index: 0xFFFFFFFF}
		st.Size = 0x200 << m.header.BlockSize

		if st.loadBlockOffsets() != nil {
			continue
		}

		for i := 0; i+1 < len(st.Positions); i++ {
			raw := make([]byte, st.Positions[i+1]-st.Positions[i])
			if _, err := m.file.ReadAt(raw, int64(b.FilePosition+st.Positions[i])); err != nil {
				break
			}

			want := uint32(0)
			if want = b.UncompressedFileSize - uint32(i)*st.Size; want > st.Size {
				want = st.Size
			}

			if uint32(len(raw)) == want || len(raw) < 2 {
				continue
			}

			body, isMulti := raw, false
			if b.HasFlag(FileCompress) {
				if raw[0] != 8 { // only the PKWare method
					continue
				}

				body, isMulti = raw[1:], true
			}

			_ = isMulti

			ref, err1 := blastDecompress(body)
			got, err2 := explode(body, int(want))

			if (err1 == nil) != (err2 == nil) || !bytes.Equal(ref, got) {
				t.Fatalf("sector %d of block %+v differs (ref err %v, got err %v)", i, b.FilePosition, err1, err2)
			}

			checked++
		}

		if checked > 3000 {
			break
		}
	}

	t.Logf("%d imploded sectors identical", checked)

	if checked == 0 {
		t.Log("no imploded sectors found in the archive")
	}
}

// BenchmarkExplode decodes imploded sectors of the real archive (needs D2_GAME_DIR) with the new and
// the old decoder.
func BenchmarkExplode(b *testing.B) {
	dir := os.Getenv("D2_GAME_DIR")
	if dir == "" {
		b.Skip("D2_GAME_DIR not set")
	}

	m, err := FromFile(filepath.Join(dir, "d2data.mpq"))
	if err != nil {
		b.Skip(err)
	}

	defer m.Close()

	var sectors [][]byte

	total := 0

	for i, entradas := uint32(0), m.blockEntries(); i < entradas; i++ {
		blk := m.bloqueEn(i)
		if blk.UncompressedFileSize < 8192 || blk.UncompressedFileSize > 1<<20 || blk.HasFlag(FileEncrypted) ||
			blk.HasFlag(FileSingleUnit) || !blk.HasFlag(FileCompress) {
			continue
		}

		st := &Stream{MPQ: m, Block: blk}
		st.Size = 0x200 << m.header.BlockSize

		if st.loadBlockOffsets() != nil || len(st.Positions) < 3 {
			continue
		}

		raw := make([]byte, st.Positions[1]-st.Positions[0])
		if _, err := m.file.ReadAt(raw, int64(blk.FilePosition+st.Positions[0])); err != nil || raw[0] != 8 {
			continue
		}

		sectors = append(sectors, raw[1:])
		total += int(st.Size)

		if len(sectors) == 400 {
			break
		}
	}

	b.Run("explode", func(b *testing.B) {
		b.SetBytes(int64(total))

		for i := 0; i < b.N; i++ {
			for _, sec := range sectors {
				if _, err := explode(sec, int(0x1000)); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("blast", func(b *testing.B) {
		b.SetBytes(int64(total))

		for i := 0; i < b.N; i++ {
			for _, sec := range sectors {
				if _, err := blastDecompress(sec); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}
''',
    'd2common/d2fileformats/d2mpq/fuzz_test.go': r'''package d2mpq

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// synthMPQ builds a small, valid MPQ archive in memory (nothing from the game): a plain file, a
// zlib compressed multi-sector file and a single unit file. mutate may alter the table words before
// they are encrypted, to build hostile archives.
func synthMPQ(mutate func(hash, block []uint32)) []byte {
	return synthMPQSized(8, 2, mutate)
}

// synthMPQSized is synthMPQ with the size of the two tables as parameters (DC-002.28): the tables of the
// game are tens of thousands of entries (d2data.mpq: 65.536 de hash) and las tablas se leen POR TROZOS, asi
// que hace falta poder armar un MPQ cuyas tablas crucen el marco del trozo (1.024 entradas) para comparar
// el camino por trozos contra el de la tabla entera.
func synthMPQSized(hashEntries, blockEntries uint32, mutate func(hash, block []uint32)) []byte {
	if hashEntries < 2 || blockEntries < 2 {
		panic("synthMPQSized: hacen falta al menos las 2 entradas de los dos archivos")
	}

	cryptoLookup(0)

	const blockShift = 3 // 4 KiB sectors

	sector := 0x200 << blockShift

	plain := bytes.Repeat([]byte("plain file "), 40)

	big := bytes.Repeat([]byte("compressed sector data "), 400) // 9200 bytes: three sectors

	var body bytes.Buffer

	body.Write(make([]byte, 32)) // header goes here

	// file 0: stored as is
	pos0 := uint32(body.Len())
	body.Write(plain)

	// file 1: offset table + zlib sectors
	pos1 := uint32(body.Len())
	nsec := (len(big) + sector - 1) / sector
	offsets := make([]uint32, nsec+1)

	var sectors [][]byte

	for i := 0; i < nsec; i++ {
		end := (i + 1) * sector
		if end > len(big) {
			end = len(big)
		}

		var z bytes.Buffer

		z.WriteByte(2) // zlib

		w := zlib.NewWriter(&z)
		_, _ = w.Write(big[i*sector : end])
		_ = w.Close()

		sectors = append(sectors, z.Bytes())
	}

	offsets[0] = uint32(len(offsets) * 4)
	for i, s := range sectors {
		offsets[i+1] = offsets[i] + uint32(len(s))
	}

	_ = binary.Write(&body, binary.LittleEndian, offsets)

	for _, s := range sectors {
		body.Write(s)
	}

	compSize1 := uint32(body.Len()) - pos1

	hashOff := uint32(body.Len())
	blockOff := hashOff + hashEntries*16

	hash := make([]uint32, hashEntries*4)
	for i := range hash {
		hash[i] = 0xFFFFFFFF
	}

	// DC-002.33: cada entrada va en la ranura donde el SONdeo de su nombre la deja: eso es lo que hacen los
	// MPQ de verdad y lo que ahora usa `buscarHash`. Antes cada entrada iba en una ranura fija (0, 1, ...)
	// porque la consulta era contra un mapa que indexaba por las dos mitades del nombre y la posicion no
	// importaba: con una tabla de bytes la posicion ES la consulta.
	setHash := func(name string, block uint32) {
		slot := int(hashString(name, 0) & (hashEntries - 1))

		for i := 0; i < int(hashEntries); i++ {
			s := (slot + i) & int(hashEntries-1)
			if hash[s*4+3] != 0xFFFFFFFF { // ranura ocupada: sigue el sondeo
				continue
			}

			hash[s*4] = hashString(name, 1)
			hash[s*4+1] = hashString(name, 2)
			hash[s*4+2] = 0
			hash[s*4+3] = block

			return
		}

		panic("synthMPQSized: no queda ranura libre en la tabla de hash")
	}

	setHash("plain.txt", 0)
	setHash("big.txt", 1)

	blocks := make([]uint32, blockEntries*4)
	copy(blocks, []uint32{
		pos0, uint32(len(plain)), uint32(len(plain)), uint32(FileExists),
		pos1, compSize1, uint32(len(big)), uint32(FileExists | FileCompress),
	})

	if mutate != nil {
		mutate(hash, blocks)
	}

	encrypt(hash, hashString("(hash table)", 3))
	encrypt(blocks, hashString("(block table)", 3))

	_ = binary.Write(&body, binary.LittleEndian, hash)
	_ = binary.Write(&body, binary.LittleEndian, blocks)

	out := body.Bytes()
	copy(out, "MPQ\x1a")
	binary.LittleEndian.PutUint32(out[4:], 32)
	binary.LittleEndian.PutUint32(out[8:], uint32(len(out)))
	binary.LittleEndian.PutUint16(out[12:], 0)
	binary.LittleEndian.PutUint16(out[14:], blockShift)
	binary.LittleEndian.PutUint32(out[16:], hashOff)
	binary.LittleEndian.PutUint32(out[20:], blockOff)
	binary.LittleEndian.PutUint32(out[24:], hashEntries)
	binary.LittleEndian.PutUint32(out[28:], blockEntries)

	return out
}

// readAll reads a bounded amount of every block of the archive.
func readAll(t testing.TB, path string) {
	m, err := FromFile(path)
	if err != nil {
		return
	}
	defer m.Close()

	// DC-002.33: la tabla de bloque son BYTES y se recorre por indice; cada consulta arma la `Block` de ese
	// numero (antes se recorria el mapa de hash y se seguia el BlockIndex de cada ranura).
	for bi, entradas := uint32(0), m.blockEntries(); bi < entradas; bi++ {
		b := m.bloqueEn(bi)

		s, err := CreateStream(m, b, "file")
		if err != nil {
			continue
		}

		buf := make([]byte, 1<<16)
		for i := 0; i < 64; i++ {
			n, err := s.Read(buf, 0, uint32(len(buf)))
			if err != nil || n == 0 {
				break
			}
		}
	}

	_, _ = m.ReadFile("plain.txt")
	_, _ = m.ReadFile("big.txt")
	_, _ = m.ReadFile("(listfile)")
}

func TestSynthMPQ(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.mpq")
	if err := os.WriteFile(p, synthMPQ(nil), 0o600); err != nil {
		t.Fatal(err)
	}

	m, err := FromFile(p)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	d, err := m.ReadFile("plain.txt")
	if err != nil || !bytes.HasPrefix(d, []byte("plain file ")) || len(d) != 440 {
		t.Fatalf("plain: %v %d", err, len(d))
	}

	d, err = m.ReadFile("big.txt")
	if err != nil || len(d) != 9200 || !bytes.HasPrefix(d, []byte("compressed sector data ")) {
		t.Fatalf("big: %v %d", err, len(d))
	}
}

// TestHostileMPQ: hand-made hostile headers and tables must fail with errors, quickly, without
// allocating by their claimed sizes.
func TestHostileMPQ(t *testing.T) {
	huge := uint32(0xFFFFFFF0)

	cases := map[string]func([]byte) []byte{
		"huge hash table": func(b []byte) []byte { binary.LittleEndian.PutUint32(b[24:], huge); return b },
		"huge block table": func(b []byte) []byte {
			binary.LittleEndian.PutUint32(b[28:], 0x40000000)
			return b
		},
		"block size shift 40": func(b []byte) []byte { binary.LittleEndian.PutUint16(b[14:], 40); return b },
		"table offsets beyond file": func(b []byte) []byte {
			binary.LittleEndian.PutUint32(b[16:], huge)
			return b
		},
		"truncated":   func(b []byte) []byte { return b[:len(b)/2] },
		"header only": func(b []byte) []byte { return b[:32] },
	}

	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "h.mpq")
			if err := os.WriteFile(p, f(synthMPQ(nil)), 0o600); err != nil {
				t.Fatal(err)
			}

			readAll(t, p)
		})
	}

	// block table entries claiming huge or inconsistent sizes
	for name, mut := range map[string]func(hash, block []uint32){
		"huge size":       func(h, b []uint32) { b[2] = 0xFFFFFFFF },
		"huge compressed": func(h, b []uint32) { b[5] = 0xFFFFFFFF; b[6] = 0xFFFFFFF0 },
		"position beyond": func(h, b []uint32) { b[0] = 0xFFFFFFFF },
		"hash to block 99": func(h, b []uint32) {
			h[3] = 99
		},
		"colliding names": func(h, b []uint32) {
			h[4], h[5], h[6], h[7] = h[0], h[1], h[2], 1
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "h.mpq")
			if err := os.WriteFile(p, synthMPQ(mut), 0o600); err != nil {
				t.Fatal(err)
			}

			readAll(t, p)
		})
	}
}

// FuzzArchive opens arbitrary bytes as an MPQ and reads every file; it must never panic, hang or
// allocate by a size read from the file.
func FuzzArchive(f *testing.F) {
	f.Add(synthMPQ(nil))
	f.Add(synthMPQ(func(h, b []uint32) { b[2] = 0xFFFFFFFF }))
	f.Add(synthMPQ(func(h, b []uint32) { b[3] |= uint32(FileEncrypted | FileFixKey) }))
	f.Add(synthMPQ(func(h, b []uint32) { b[7] |= uint32(FileSingleUnit) }))
	f.Add(synthMPQ(func(h, b []uint32) { b[3] = uint32(FileExists | FileImplode) }))
	f.Add([]byte("MPQ\x1a"))
	f.Add([]byte{})

	dir := f.TempDir()

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			return
		}

		p := filepath.Join(dir, "fuzz.mpq")
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Skip()
		}

		readAll(t, p)
	})
}

// FuzzSector feeds arbitrary bytes to the sector decompressors.
func FuzzSector(f *testing.F) {
	f.Add([]byte{2, 0x78, 0x9c, 3, 0}, uint32(16))
	f.Add([]byte{8, 0, 4}, uint32(16))
	f.Add([]byte{0x40, 0, 0, 0, 0}, uint32(16))
	f.Add([]byte{0x41, 0, 0, 0, 0}, uint32(16))
	f.Add([]byte{0x81, 0, 0, 0, 0, 0, 0}, uint32(16))
	f.Add([]byte{}, uint32(0))

	f.Fuzz(func(t *testing.T, data []byte, expected uint32) {
		if expected > 1<<20 {
			expected = 1 << 20
		}

		_, _ = decompressMulti(data, expected)
		_, _ = pkDecompressHint(data, int(expected))
	})
}
''',
    'd2common/d2fileformats/d2mpq/mpq_trozos_test.go': r'''package d2mpq

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// lectorAnotado: entrega los datos de a lo sumo `tope` bytes por Read y ANOTA el pedido mas grande que le
// hicieron. Es el instrumento de DC-002.28: lo que no entra en la Dreamcast es el pedido de la tabla
// entera de un tiron (la de hash de d2data.mpq son 1.048.576 bytes), y eso se mide con el TAMANO DEL
// PEDIDO, no con la memoria del proceso (que en el host sobra y taparia el defecto).
type lectorAnotado struct {
	datos []byte
	pos   int
	tope  int
	mayor int
}

func (l *lectorAnotado) Read(p []byte) (int, error) {
	if len(p) > l.mayor {
		l.mayor = len(p)
	}

	if l.pos >= len(l.datos) {
		return 0, io.EOF
	}

	n := len(p)
	if n > l.tope {
		n = l.tope
	}

	if l.pos+n > len(l.datos) {
		n = len(l.datos) - l.pos
	}

	copy(p, l.datos[l.pos:l.pos+n])

	l.pos += n

	return n, nil
}

// TestDecryptTablePorTrozosLeeDeAPoco: la tabla se descifra por trozos, palabra por palabra igual que de un
// solo tiron, y NUNCA se pide mas de un trozo de una vez. Los tamanos elegidos cruzan el marco del trozo
// (1.024 entradas) y el ultimo caso trae la tabla cortada, que tiene que entregar lo que llego y despues el
// error (igual que el camino viejo).
func TestDecryptTablePorTrozosLeeDeAPoco(t *testing.T) {
	tests := []struct {
		name    string
		entries uint32
		extra   int // bytes que faltan del final (tabla cortada)
	}{
		{"una entrada", 1, 0},
		{"justo antes del marco", tablaTrozoEntradas - 1, 0},
		{"justo el marco", tablaTrozoEntradas, 0},
		{"una entrada despues del marco", tablaTrozoEntradas + 1, 0},
		{"cuatro trozos y monedas", 4*tablaTrozoEntradas + 7, 0},
		{"cortada en el segundo trozo", 3 * tablaTrozoEntradas, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			datos := make([]byte, int(tt.entries)*16-tt.extra)
			for i := range datos {
				datos[i] = byte(i*29 + 11)
			}

			quiero, quieroErr := naiveDecryptTable(bytes.NewReader(datos), tt.entries, "(hash table)")

			lector := &lectorAnotado{datos: datos, tope: 1 << 20}
			var got []uint32

			trozos := 0

			gotErr := decryptTablePorTrozos(lector, tt.entries, "(hash table)", func(trozo []uint32) error {
				got = append(got, trozo...)
				trozos++

				return nil
			})

			if (quieroErr == nil) != (gotErr == nil) {
				t.Fatalf("error %v, quiero %v", gotErr, quieroErr)
			}

			// Con la tabla cortada las dos versiones dejan de hablar del mismo largo: el camino por trozos
			// entrega las palabras que SI llegaron (12.283 contra 12.288 en el caso cortado) y el de un solo
			// tiron devuelve el arreglo completo con la cola en cero. Lo que importa es que lo que llego sea
			// igual palabra por palabra, y que los dos contesten el error.
			n := len(got)
			if len(quiero) < n {
				n = len(quiero)
			}

			for i := 0; i < n; i++ {
				if got[i] != quiero[i] {
					t.Fatalf("palabra %d = %#x, quiero %#x", i, got[i], quiero[i])
				}
			}

			// EL TOPE DEL PEDIDO, en un numero FIJO y no en el de la constante: mas de 64 KiB de una vez se
			// va al monton de C de la consola (`gc_external_alloc`, el umbral de "objeto grande" de
			// libgodc) y es justo el pedido que no entra (medido: `gc_external_alloc: OOM` con la tabla de
			// d2data en 1.048.576 bytes). Con un tope que se moviera con la constante, un trozo de 128 KiB
			// pasaria el control: el control tiene que morder.
			if pedidoMaximo := 64 << 10; lector.mayor > pedidoMaximo {
				t.Fatalf("pidio %d bytes de una vez: el tope es %d (mas que eso se va al monton de C en la consola)", lector.mayor, pedidoMaximo)
			}

			// y que de verdad vaya por trozos: una tabla mas larga que un trozo tiene que llegar en varios
			esperadosTrozos := int((tt.entries + tablaTrozoEntradas - 1) / tablaTrozoEntradas)

			if trozos != esperadosTrozos {
				t.Fatalf("llego en %d trozos, esperaba %d", trozos, esperadosTrozos)
			}
		})
	}
}

// TestTablasPorTrozosUnMPQDeVerdad: un MPQ armado por el generador de las pruebas, con las dos tablas mas
// grandes que tres trozos, abierto con `FromFile` (el camino del motor) tiene que dar EXACTAMENTE las
// mismas dos tablas que la version que descifra la tabla entera palabra por palabra.
//
// Es la misma clase de control que el corpus del host: el numero (las entradas) sale del MPQ, no de la
// prueba, y la comparacion es contra el camino viejo, no contra una constante escrita a mano.
func TestTablasPorTrozosUnMPQDeVerdad(t *testing.T) {
	const (
		hashEntradas   = uint32(4096) // 4 trozos: cruza el marco 4 veces
		bloqueEntradas = uint32(3000) // casi 3 trozos
	)

	p := filepath.Join(t.TempDir(), "grande.mpq")
	if err := os.WriteFile(p, synthMPQSized(hashEntradas, bloqueEntradas, nil), 0o600); err != nil {
		t.Fatal(err)
	}

	m, err := FromFile(p)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	// la referencia: la tabla cruda del archivo, descifrada entera palabra por palabra
	crudoHash := make([]byte, int(hashEntradas)*16)
	if _, err := m.file.ReadAt(crudoHash, int64(m.header.HashTableOffset)); err != nil {
		t.Fatal(err)
	}

	quiero, err := naiveDecryptTable(bytes.NewReader(crudoHash), hashEntradas, "(hash table)")
	if err != nil {
		t.Fatal(err)
	}

	// DC-002.33: la tabla de hash ya no es un mapa de objetos, son BYTES: se compara ranura por ranura contra
	// la referencia (el camino de la tabla entera), que es la misma que se compara para la de bloque.
	if got := m.hashEntries(); got != hashEntradas {
		t.Fatalf("la tabla de hash tiene %d ranuras, quiero %d", got, hashEntradas)
	}

	for n, i := uint32(0), uint32(0); i < hashEntradas; n, i = n+4, i+1 {
		got := m.entradaHash(i)

		if got.A != quiero[n] || got.B != quiero[n+1] ||
			got.Locale != uint16(quiero[n+2]>>16) || got.Platform != uint16(quiero[n+2]&0xFFFF) ||
			got.BlockIndex != quiero[n+3] {
			t.Fatalf("ranura %d = %+v, quiero %d/%d/%d/%d", i, got,
				quiero[n], quiero[n+1], quiero[n+2], quiero[n+3])
		}
	}

	// y la tabla de bloques, entrada por entrada
	crudoBloque := make([]byte, int(bloqueEntradas)*16)
	if _, err := m.file.ReadAt(crudoBloque, int64(m.header.BlockTableOffset)); err != nil {
		t.Fatal(err)
	}

	quieroBloques, err := naiveDecryptTable(bytes.NewReader(crudoBloque), bloqueEntradas, "(block table)")
	if err != nil {
		t.Fatal(err)
	}

	if got := m.blockEntries(); got != bloqueEntradas {
		t.Fatalf("la tabla de bloque tiene %d entradas, quiero %d", got, bloqueEntradas)
	}

	for i := uint32(0); i < bloqueEntradas; i++ {
		n := i * 4
		got := m.bloqueEn(i)

		if got.FilePosition != quieroBloques[n] || got.CompressedFileSize != quieroBloques[n+1] ||
			got.UncompressedFileSize != quieroBloques[n+2] || uint32(got.Flags) != quieroBloques[n+3] {
			t.Fatalf("bloque %d = %+v, quiero %d/%d/%d/%d", i, *got,
				quieroBloques[n], quieroBloques[n+1], quieroBloques[n+2], quieroBloques[n+3])
		}
	}
}

// TestDecryptTableSigueSiendoLaReferencia: el camino de la tabla entera (el que usan las pruebas y el
// banco de pruebas) tiene que seguir dando lo mismo que el de por trozos: si los dos se van de las manos a
// la vez, la comparacion de arriba no lo caza.
func TestDecryptTableSigueSiendoLaReferencia(t *testing.T) {
	const entradas = uint32(3*tablaTrozoEntradas + 5)

	datos := make([]byte, entradas*16)
	for i := range datos {
		datos[i] = byte(i*7 + 3)
	}

	quiero, err := naiveDecryptTable(bytes.NewReader(datos), entradas, "(block table)")
	if err != nil {
		t.Fatal(err)
	}

	got, err := decryptTable(bytes.NewReader(datos), entradas, "(block table)")
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != len(quiero) {
		t.Fatalf("%d palabras, quiero %d", len(got), len(quiero))
	}

	for i := range quiero {
		if got[i] != quiero[i] {
			t.Fatalf("palabra %d = %#x, quiero %#x", i, got[i], quiero[i])
		}
	}
}
''',
    'd2common/d2fileformats/d2mpq/mpq_equivalencia_test.go': r'''package d2mpq

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mapaViejo es el indice que el motor tenia hasta DC-002.33: una clave por ranura de la tabla de hash, con
// las DOS mitades del nombre pegadas (`hashString(nombre, 1) << 32 | hashString(nombre, 2)`), y el ultimo
// que escribe gana. Es el camino que la tarjeta reemplaza por los BYTES + el sondeo, asi que sirve de
// referencia: `buscarHash` tiene que encontrar lo mismo.
//
// DC-002.41: recibe las palabras de la tabla CRUDA (las que salen del archivo, ranuras vacias incluidas),
// NO la representacion que guarda el paquete: con las ranuras vacias fuera del arreglo, una referencia
// armada con `entradaHash` no seria independiente (un error del arreglo saldria igual en las dos puntas).
func mapaViejo(palabras []uint32) map[uint64]uint32 {
	viejo := make(map[uint64]uint32)

	for i := 0; i+4 <= len(palabras); i += 4 {
		viejo[uint64(palabras[i])<<32|uint64(palabras[i+1])] = palabras[i+3]
	}

	return viejo
}

// TestBuscarHashIgualQueElMapaViejo: el camino del HOST tiene que seguir dando lo mismo que daba el mapa de
// objetos. Se compara, nombre por nombre del `(listfile)` de cada MPQ del JUEGO, la consulta nueva (el sondeo
// sobre los bytes) contra la vieja (el mapa), y se cuentan de paso los nombres que NINGUNO de los dos
// encuentra (nombres que quedaron fuera del `(listfile)`, que no es el indice completo del archivo).
//
// Necesita D2_GAME_DIR (la carpeta de la instalacion) y solo LEE los MPQ.
func TestBuscarHashIgualQueElMapaViejo(t *testing.T) {
	dir := os.Getenv("D2_GAME_DIR")
	if dir == "" {
		t.Skip("D2_GAME_DIR not set")
	}

	archivos := []string{"patch_D2.mpq", "patch_d2.mpq", "d2char.mpq", "d2data.mpq"}

	var medidos int

	for _, archivo := range archivos {
		ruta := filepath.Join(dir, archivo)
		if _, err := os.Stat(ruta); err != nil {
			continue
		}

		m, err := FromFile(ruta)
		if err != nil {
			t.Fatalf("%s: %v", archivo, err)
		}

		datos, err := m.ReadFile("(listfile)")
		if err != nil {
			m.Close()

			t.Fatalf("%s: (listfile): %v", archivo, err)
		}

		// DC-002.41: la referencia sale de la tabla CRUDA del archivo, descifrada palabra por palabra por el
		// camino de siempre (`naiveDecryptTable`): el sondeo se compara contra los bytes que estan en el
		// disco, no contra otra vuelta de la representacion nueva.
		cruda := make([]byte, int(m.hashEntries())*bytesPorEntrada)
		if _, errR := m.file.ReadAt(cruda, int64(m.header.HashTableOffset)); errR != nil {
			t.Fatalf("%s: no pude leer la tabla de hash cruda: %v", archivo, errR)
		}

		palabras, errR := naiveDecryptTable(bytes.NewReader(cruda), m.hashEntries(), "(hash table)")
		if errR != nil {
			t.Fatalf("%s: %v", archivo, errR)
		}

		viejo := mapaViejo(palabras)

		nombres, iguales, distintos, fuera := 0, 0, 0, 0

		for _, linea := range strings.Split(string(datos), "\n") {
			nombre := strings.TrimSpace(strings.TrimRight(linea, "\x00"))

			if nombre == "" {
				continue
			}

			nombres++

			idxViejo, okViejo := viejo[hashFilename(nombre)]
			idxNuevo, okNuevo := m.buscarHash(nombre)

			switch {
			case !okViejo && !okNuevo:
				fuera++
			case okViejo != okNuevo || idxViejo != idxNuevo:
				distintos++

				if distintos <= 5 {
					t.Errorf("%s: %q: el mapa viejo dice (%d, %v) y el sondeo (%d, %v)",
						archivo, nombre, idxViejo, okViejo, idxNuevo, okNuevo)
				}
			default:
				iguales++
			}
		}

		hashSum, blockSum := m.TableChecksum()

		// DC-002.41: lo que la tabla de hash ocupa AHORA en RAM (las ranuras ocupadas + 1 bit por ranura +
		// los rangos) contra lo que ocupaba con la tabla entera. Es el numero de la tarjeta.
		reserva := cap(m.hashEntradas) + len(m.hashOcupadas) + 2*len(m.hashRangos)

		t.Logf("%s: entradas_hash=%d entradas_bloque=%d nombres=%d iguales=%d distintos=%d fuera_del_listfile=%d checksum_hash=%d checksum_bloque=%d ranuras_ocupadas=%d reserva_hash=%d reserva_tabla_entera=%d",
			archivo, m.hashEntries(), m.blockEntries(), nombres, iguales, distintos, fuera, hashSum, blockSum,
			len(m.hashEntradas)/bytesPorEntrada, reserva, int(m.hashEntries())*bytesPorEntrada)

		m.Close()

		if distintos != 0 {
			t.Fatalf("%s: %d nombres donde el sondeo y el mapa viejo NO dan lo mismo", archivo, distintos)
		}

		if nombres == 0 {
			t.Fatalf("%s: el (listfile) vino vacio", archivo)
		}

		medidos++
	}

	if medidos == 0 {
		t.Skip("no hay MPQ del juego en D2_GAME_DIR")
	}
}
''',
}

# EL PRISTINO (lo que habia hasta DC-002.33): un objeto de Go por entrada.
PRISTINO = {
    'd2common/d2fileformats/d2mpq/mpq.go': r'''package d2mpq

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

var _ d2interface.Archive = &MPQ{} // Static check to confirm struct conforms to interface

// MPQ represents an MPQ archive
type MPQ struct {
	filePath string
	file     *os.File
	hashes   map[uint64]*Hash
	blocks   []*Block
	header   Header
	fileSize int64 // size of the archive on disk, to reject offsets and sizes beyond it
}

// PatchInfo represents patch info for the MPQ.
type PatchInfo struct {
	Length   uint32   // Length of patch info header, in bytes
	Flags    uint32   // Flags. 0x80000000 = MD5 (?)
	DataSize uint32   // Uncompressed size of the patch file
	MD5      [16]byte // MD5 of the entire patch file after decompression
}

// New loads an MPQ file and only reads the header
func New(fileName string) (*MPQ, error) {
	mpq := &MPQ{filePath: fileName}

	var err error
	if runtime.GOOS == "linux" {
		mpq.file, err = openIgnoreCase(fileName)
	} else {
		mpq.file, err = os.Open(fileName) //nolint:gosec // Will fix later
	}

	if err != nil {
		return nil, err
	}

	if st, serr := mpq.file.Stat(); serr == nil {
		mpq.fileSize = st.Size()
	}

	if err := mpq.readHeader(); err != nil {
		mpq.file.Close()

		return nil, fmt.Errorf("failed to read reader: %v", err)
	}

	return mpq, nil
}

// FromFile loads an MPQ file and returns a MPQ structure
func FromFile(fileName string) (*MPQ, error) {
	mpq, err := New(fileName)
	if err != nil {
		return nil, err
	}

	if err := mpq.readHashTable(); err != nil {
		mpq.file.Close()
		return nil, fmt.Errorf("failed to read hash table: %v", err)
	}

	if err := mpq.readBlockTable(); err != nil {
		mpq.file.Close()
		return nil, fmt.Errorf("failed to read block table: %v", err)
	}

	return mpq, nil
}

// getFileBlockData gets a block table entry
func (mpq *MPQ) getFileBlockData(fileName string) (*Block, error) {
	fileEntry, ok := mpq.hashes[hashFilename(fileName)]
	if !ok {
		return nil, errors.New("file not found")
	}

	if fileEntry.BlockIndex >= uint32(len(mpq.blocks)) {
		return nil, errors.New("invalid block index")
	}

	return mpq.blocks[fileEntry.BlockIndex], nil
}

// Close closes the MPQ file
func (mpq *MPQ) Close() error {
	return mpq.file.Close()
}

// ReadFile reads a file from the MPQ and returns a memory stream
func (mpq *MPQ) ReadFile(fileName string) ([]byte, error) {
	fileBlockData, err := mpq.getFileBlockData(fileName)
	if err != nil {
		return []byte{}, err
	}

	// each stream gets its own copy of the block: streams are used from several goroutines
	block := *fileBlockData
	block.FileName = strings.ToLower(fileName)
	fileBlockData = &block

	stream, err := CreateStream(mpq, fileBlockData, fileName)
	if err != nil {
		return []byte{}, err
	}

	buffer := make([]byte, fileBlockData.UncompressedFileSize)
	if _, err := stream.Read(buffer, 0, fileBlockData.UncompressedFileSize); err != nil {
		return []byte{}, err
	}

	return buffer, nil
}

// ReadFileStream reads the mpq file data and returns a stream
func (mpq *MPQ) ReadFileStream(fileName string) (d2interface.DataStream, error) {
	fileBlockData, err := mpq.getFileBlockData(fileName)
	if err != nil {
		return nil, err
	}

	// each stream gets its own copy of the block: streams are used from several goroutines
	block := *fileBlockData
	block.FileName = strings.ToLower(fileName)
	fileBlockData = &block

	stream, err := CreateStream(mpq, fileBlockData, fileName)
	if err != nil {
		return nil, err
	}

	return &MpqDataStream{stream: stream}, nil
}

// ReadTextFile reads a file and returns it as a string
func (mpq *MPQ) ReadTextFile(fileName string) (string, error) {
	data, err := mpq.ReadFile(fileName)

	if err != nil {
		return "", err
	}

	return string(data), nil
}

// Listfile returns the list of files in this MPQ
func (mpq *MPQ) Listfile() ([]string, error) {
	data, err := mpq.ReadFile("(listfile)")

	if err != nil {
		return nil, err
	}

	raw := strings.TrimRight(string(data), "\x00")
	s := bufio.NewScanner(strings.NewReader(raw))

	var filePaths []string

	for s.Scan() {
		filePath := s.Text()
		filePaths = append(filePaths, filePath)
	}

	return filePaths, nil
}

// Path returns the MPQ file path
func (mpq *MPQ) Path() string {
	return mpq.filePath
}

// Contains returns bool for whether the given filename exists in the mpq
func (mpq *MPQ) Contains(filename string) bool {
	_, ok := mpq.hashes[hashFilename(filename)]
	return ok
}

// Size returns the size of the mpq in bytes
func (mpq *MPQ) Size() uint32 {
	return mpq.header.ArchiveSize
}

func openIgnoreCase(mpqPath string) (*os.File, error) {
	// First see if file exists with specified case
	mpqFile, err := os.Open(mpqPath) //nolint:gosec // Will fix later
	if err != nil {
		mpqName := filepath.Base(mpqPath)
		mpqDir := filepath.Dir(mpqPath)

		var files []fs.FileInfo
		files, err = ioutil.ReadDir(mpqDir)

		if err != nil {
			return nil, err
		}

		for _, file := range files {
			if strings.EqualFold(file.Name(), mpqName) {
				mpqName = file.Name()
				break
			}
		}

		return os.Open(filepath.Join(mpqDir, mpqName)) //nolint:gosec // Will fix later
	}

	return mpqFile, err
}

// maxFileSize bounds the size of one file read into memory (the largest game file is far smaller).
const maxFileSize = 1 << 28

// checkBlock rejects block table entries whose sizes cannot be real, before anything is allocated
// from them.
func (mpq *MPQ) checkBlock(b *Block) error {
	if b.UncompressedFileSize > maxFileSize {
		return fmt.Errorf("file size %d is too large", b.UncompressedFileSize)
	}

	if mpq.fileSize > 0 && int64(b.FilePosition) > mpq.fileSize {
		return errors.New("file starts beyond the end of the archive")
	}

	if !b.HasFlag(FileCompress) && !b.HasFlag(FileImplode) && mpq.fileSize > 0 &&
		int64(b.FilePosition)+int64(b.UncompressedFileSize) > mpq.fileSize {
		return errors.New("file extends beyond the end of the archive")
	}

	return nil
}
''',
    'd2common/d2fileformats/d2mpq/mpq_hash.go': r'''package d2mpq

import "io"

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

//nolint:gomnd // number
func (mpq *MPQ) readHashTable() error {
	if _, err := mpq.file.Seek(int64(mpq.header.HashTableOffset), io.SeekStart); err != nil {
		return err
	}

	mpq.hashes = make(map[uint64]*Hash)

	// DC-002.28: la tabla se descifra POR TROZOS y cada trozo va derecho al mapa, sin materializar la
	// tabla entera. Antes se pedia la tabla completa (dos reservas contiguas de `entradas*16` bytes) y en
	// la Dreamcast eso no entra: la de hash de d2data.mpq son 1.048.576 bytes y el monton de C de la
	// consola queda en pocos cientos de bytes libres (medido: el motor muere en `gc_external_alloc: OOM`
	// al abrir ese archivo). El unico que crece con el tamano de la tabla es el mapa, que es el
	// instrumento del motor (una entrada por nombre).
	return decryptTablePorTrozos(mpq.file, mpq.header.HashTableEntries, "(hash table)",
		func(trozo []uint32) error {
			for n := 0; n+4 <= len(trozo); n += 4 {
				e := &Hash{
					A: trozo[n],
					B: trozo[n+1],
					// https://github.com/OpenDiablo2/OpenDiablo2/issues/812
					Locale:     uint16(trozo[n+2] >> 16),    //nolint:gomnd // binary data
					Platform:   uint16(trozo[n+2] & 0xFFFF), //nolint:gomnd // binary data
					BlockIndex: trozo[n+3],
				}
				mpq.hashes[e.Name64()] = e
			}

			return nil
		})
}
''',
    'd2common/d2fileformats/d2mpq/mpq_block.go': r'''package d2mpq

import (
	"io"
	"strings"
)

// FileFlag represents flags for a file record in the MPQ archive
type FileFlag uint32

const (
	// FileImplode - File is compressed using PKWARE Data compression library
	FileImplode FileFlag = 0x00000100
	// FileCompress - File is compressed using combination of compression methods
	FileCompress FileFlag = 0x00000200
	// FileEncrypted - The file is encrypted
	FileEncrypted FileFlag = 0x00010000
	// FileFixKey - The decryption key for the file is altered according to the position of the file in the archive
	FileFixKey FileFlag = 0x00020000
	// FilePatchFile - The file contains incremental patch for an existing file in base MPQ
	FilePatchFile FileFlag = 0x00100000
	// FileSingleUnit - Instead of being divided to 0x1000-bytes blocks, the file is stored as single unit
	FileSingleUnit FileFlag = 0x01000000
	// FileDeleteMarker - File is a deletion marker, indicating that the file no longer exists. This is used to allow patch
	// archives to delete files present in lower-priority archives in the search chain. The file usually
	// has length of 0 or 1 byte and its name is a hash
	FileDeleteMarker FileFlag = 0x02000000
	// FileSectorCrc - File has checksums for each sector. Ignored if file is not compressed or imploded.
	FileSectorCrc FileFlag = 0x04000000
	// FileExists - Set if file exists, reset when the file was deleted
	FileExists FileFlag = 0x80000000
)

// Block represents an entry in the block table
type Block struct { // 16 bytes
	FilePosition         uint32
	CompressedFileSize   uint32
	UncompressedFileSize uint32
	Flags                FileFlag
	// Local Stuff...
	FileName       string
	EncryptionSeed uint32
}

// HasFlag returns true if the specified flag is present
func (b *Block) HasFlag(flag FileFlag) bool {
	return (b.Flags & flag) != 0
}

func (b *Block) calculateEncryptionSeed(fileName string) {
	fileName = fileName[strings.LastIndex(fileName, `\`)+1:]
	seed := hashString(fileName, 3)
	b.EncryptionSeed = (seed + b.FilePosition) ^ b.UncompressedFileSize
}

//nolint:gomnd // number
func (mpq *MPQ) readBlockTable() error {
	if _, err := mpq.file.Seek(int64(mpq.header.BlockTableOffset), io.SeekStart); err != nil {
		return err
	}

	// DC-002.28: por trozos, igual que la tabla de hash (ver `decryptTablePorTrozos`). La de d2data.mpq
	// son 11.083 entradas = 177.328 bytes: con el camino de un solo tiron eran dos reservas contiguas de
	// ese tamano mas los objetos `Block` de cada entrada.
	return decryptTablePorTrozos(mpq.file, mpq.header.BlockTableEntries, "(block table)",
		func(trozo []uint32) error {
			for n := 0; n+4 <= len(trozo); n += 4 {
				mpq.blocks = append(mpq.blocks, &Block{
					FilePosition:         trozo[n],
					CompressedFileSize:   trozo[n+1],
					UncompressedFileSize: trozo[n+2],
					Flags:                FileFlag(trozo[n+3]),
				})
			}

			return nil
		})
}
''',
    'd2common/d2fileformats/d2mpq/explode_test.go': r'''package d2mpq

import (
	"bytes"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/JoshVarga/blast"
)

// blastDecompress is the reference decoder explode replaced.
func blastDecompress(data []byte) (out []byte, err error) {
	defer func() { // the reference panics on some streams the blast writer produces
		if r := recover(); r != nil {
			out, err = nil, errExplodeCode
		}
	}()

	r, err := blast.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	return io.ReadAll(r)
}

// blastCompress compresses with the library's writer, which panics on some inputs.
func blastCompress(data []byte, typ, dict uint) (cmp *bytes.Buffer, ok bool) {
	defer func() {
		if recover() != nil {
			cmp, ok = nil, false
		}
	}()

	cmp = new(bytes.Buffer)
	w := blast.NewWriter(cmp, typ, dict)
	_, _ = w.Write(data)
	_ = w.Close()

	return cmp, true
}

func TestExplodeKnownVector(t *testing.T) {
	got, err := explode([]byte{0x00, 0x04, 0x82, 0x24, 0x25, 0x8f, 0x80, 0x7f}, 0)
	if err != nil || string(got) != "AIAIAIAIAIAIA" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestExplodeMatchesBlast(t *testing.T) {
	rnd := rand.New(rand.NewSource(7))

	gen := map[string]func(n int) []byte{
		"random": func(n int) []byte { b := make([]byte, n); rnd.Read(b); return b },
		"few symbols": func(n int) []byte {
			b := make([]byte, n)
			for i := range b {
				b[i] = byte(rnd.Intn(5))
			}

			return b
		},
		"runs and repeats": func(n int) []byte {
			var b []byte
			for len(b) < n {
				if len(b) > 10 && rnd.Intn(3) == 0 {
					s := rnd.Intn(len(b) - 5)
					b = append(b, b[s:s+2+rnd.Intn(300)%(len(b)-s-1)]...)
				} else {
					b = append(b, bytes.Repeat([]byte{byte(rnd.Intn(256))}, 1+rnd.Intn(40))...)
				}
			}

			return b[:n]
		},
	}

	// a fixed order: the generators share one random stream, and with a random map order some streams make
	// the third party blast reader loop forever (a flaky hang of the whole test binary)
	names := make([]string, 0, len(gen))
	for name := range gen {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		g := gen[name]

		for _, typ := range []uint{blast.Binary, blast.ASCII} {
			for _, dict := range []uint{blast.DictionarySize1024, blast.DictionarySize2048, blast.DictionarySize4096} {
				for _, n := range []int{1, 2, 3, 100, 4095, 4096, 4097, 20000} {
					data := g(n)

					cmp, ok := blastCompress(data, typ, dict)
					if !ok {
						continue
					}

					// the blast writer emits broken streams for some larger incompressible inputs (the old
					// reader fails on them too): only streams the old reader decodes to the input are used
					if ref, err := blastDecompress(cmp.Bytes()); err != nil || !bytes.Equal(ref, data) {
						continue
					}

					for _, hint := range []int{0, n, 10} {
						got, err := explode(cmp.Bytes(), hint)
						if err != nil {
							t.Fatalf("%s/%d/%d/%d: %v", name, typ, dict, n, err)
						}

						if !bytes.Equal(got, data) {
							t.Fatalf("%s/%d/%d/%d: output differs", name, typ, dict, n)
						}
					}
				}
			}
		}
	}
}

func TestExplodeRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
	}{
		{"empty", nil},
		{"bad literal flag", []byte{2, 4, 0, 0}},
		{"bad dictionary", []byte{0, 7, 0, 0}},
		{"truncated", []byte{0x00, 0x04, 0x82, 0x24}},
		{"no end code", bytes.Repeat([]byte{0}, 40)[:40]},
	}

	for _, tt := range tests {
		if tt.name == "no end code" {
			tt.in = append([]byte{0, 4}, tt.in...)
		}

		if _, err := explode(tt.in, 0); err == nil {
			t.Errorf("%s: no error", tt.name)
		}
	}

	// a distance that points before the start of the output
	if _, err := explode([]byte{0x00, 0x04, 0x03, 0x00, 0x00, 0x00}, 0); err == nil {
		t.Error("distance before the start: no error")
	}
}

// TestExplodeRealArchive decodes every file of a game archive with both decoders. It needs
// D2_GAME_DIR (the install folder) and reads archives only.
func TestExplodeRealArchive(t *testing.T) {
	dir := os.Getenv("D2_GAME_DIR")
	if dir == "" {
		t.Skip("D2_GAME_DIR not set")
	}

	m, err := FromFile(filepath.Join(dir, "d2data.mpq"))
	if err != nil {
		t.Skip(err)
	}

	defer m.Close()

	checked := 0

	for h := range m.hashes {
		_ = h
		break
	}

	for _, b := range m.blocks {
		if b.UncompressedFileSize == 0 || b.UncompressedFileSize > 1<<20 || b.HasFlag(FileEncrypted) || b.HasFlag(FileSingleUnit) {
			continue
		}

		if !b.HasFlag(FileCompress) && !b.HasFlag(FileImplode) {
			continue
		}

		st := &Stream{MPQ: m, Block: b, Index: 0xFFFFFFFF}
		st.Size = 0x200 << m.header.BlockSize

		if st.loadBlockOffsets() != nil {
			continue
		}

		for i := 0; i+1 < len(st.Positions); i++ {
			raw := make([]byte, st.Positions[i+1]-st.Positions[i])
			if _, err := m.file.ReadAt(raw, int64(b.FilePosition+st.Positions[i])); err != nil {
				break
			}

			want := uint32(0)
			if want = b.UncompressedFileSize - uint32(i)*st.Size; want > st.Size {
				want = st.Size
			}

			if uint32(len(raw)) == want || len(raw) < 2 {
				continue
			}

			body, isMulti := raw, false
			if b.HasFlag(FileCompress) {
				if raw[0] != 8 { // only the PKWare method
					continue
				}

				body, isMulti = raw[1:], true
			}

			_ = isMulti

			ref, err1 := blastDecompress(body)
			got, err2 := explode(body, int(want))

			if (err1 == nil) != (err2 == nil) || !bytes.Equal(ref, got) {
				t.Fatalf("sector %d of block %+v differs (ref err %v, got err %v)", i, b.FilePosition, err1, err2)
			}

			checked++
		}

		if checked > 3000 {
			break
		}
	}

	t.Logf("%d imploded sectors identical", checked)

	if checked == 0 {
		t.Log("no imploded sectors found in the archive")
	}
}

// BenchmarkExplode decodes imploded sectors of the real archive (needs D2_GAME_DIR) with the new and
// the old decoder.
func BenchmarkExplode(b *testing.B) {
	dir := os.Getenv("D2_GAME_DIR")
	if dir == "" {
		b.Skip("D2_GAME_DIR not set")
	}

	m, err := FromFile(filepath.Join(dir, "d2data.mpq"))
	if err != nil {
		b.Skip(err)
	}

	defer m.Close()

	var sectors [][]byte

	total := 0

	for _, blk := range m.blocks {
		if blk.UncompressedFileSize < 8192 || blk.UncompressedFileSize > 1<<20 || blk.HasFlag(FileEncrypted) ||
			blk.HasFlag(FileSingleUnit) || !blk.HasFlag(FileCompress) {
			continue
		}

		st := &Stream{MPQ: m, Block: blk}
		st.Size = 0x200 << m.header.BlockSize

		if st.loadBlockOffsets() != nil || len(st.Positions) < 3 {
			continue
		}

		raw := make([]byte, st.Positions[1]-st.Positions[0])
		if _, err := m.file.ReadAt(raw, int64(blk.FilePosition+st.Positions[0])); err != nil || raw[0] != 8 {
			continue
		}

		sectors = append(sectors, raw[1:])
		total += int(st.Size)

		if len(sectors) == 400 {
			break
		}
	}

	b.Run("explode", func(b *testing.B) {
		b.SetBytes(int64(total))

		for i := 0; i < b.N; i++ {
			for _, sec := range sectors {
				if _, err := explode(sec, int(0x1000)); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("blast", func(b *testing.B) {
		b.SetBytes(int64(total))

		for i := 0; i < b.N; i++ {
			for _, sec := range sectors {
				if _, err := blastDecompress(sec); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}
''',
    'd2common/d2fileformats/d2mpq/fuzz_test.go': r'''package d2mpq

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// synthMPQ builds a small, valid MPQ archive in memory (nothing from the game): a plain file, a
// zlib compressed multi-sector file and a single unit file. mutate may alter the table words before
// they are encrypted, to build hostile archives.
func synthMPQ(mutate func(hash, block []uint32)) []byte {
	return synthMPQSized(8, 2, mutate)
}

// synthMPQSized is synthMPQ with the size of the two tables as parameters (DC-002.28): the tables of the
// game are tens of thousands of entries (d2data.mpq: 65.536 de hash) and las tablas se leen POR TROZOS, asi
// que hace falta poder armar un MPQ cuyas tablas crucen el marco del trozo (1.024 entradas) para comparar
// el camino por trozos contra el de la tabla entera.
func synthMPQSized(hashEntries, blockEntries uint32, mutate func(hash, block []uint32)) []byte {
	if hashEntries < 2 || blockEntries < 2 {
		panic("synthMPQSized: hacen falta al menos las 2 entradas de los dos archivos")
	}

	cryptoLookup(0)

	const blockShift = 3 // 4 KiB sectors

	sector := 0x200 << blockShift

	plain := bytes.Repeat([]byte("plain file "), 40)

	big := bytes.Repeat([]byte("compressed sector data "), 400) // 9200 bytes: three sectors

	var body bytes.Buffer

	body.Write(make([]byte, 32)) // header goes here

	// file 0: stored as is
	pos0 := uint32(body.Len())
	body.Write(plain)

	// file 1: offset table + zlib sectors
	pos1 := uint32(body.Len())
	nsec := (len(big) + sector - 1) / sector
	offsets := make([]uint32, nsec+1)

	var sectors [][]byte

	for i := 0; i < nsec; i++ {
		end := (i + 1) * sector
		if end > len(big) {
			end = len(big)
		}

		var z bytes.Buffer

		z.WriteByte(2) // zlib

		w := zlib.NewWriter(&z)
		_, _ = w.Write(big[i*sector : end])
		_ = w.Close()

		sectors = append(sectors, z.Bytes())
	}

	offsets[0] = uint32(len(offsets) * 4)
	for i, s := range sectors {
		offsets[i+1] = offsets[i] + uint32(len(s))
	}

	_ = binary.Write(&body, binary.LittleEndian, offsets)

	for _, s := range sectors {
		body.Write(s)
	}

	compSize1 := uint32(body.Len()) - pos1

	hashOff := uint32(body.Len())
	blockOff := hashOff + hashEntries*16

	hash := make([]uint32, hashEntries*4)
	for i := range hash {
		hash[i] = 0xFFFFFFFF
	}

	setHash := func(slot int, name string, block uint32) {
		hash[slot*4] = hashString(name, 1)
		hash[slot*4+1] = hashString(name, 2)
		hash[slot*4+2] = 0
		hash[slot*4+3] = block
	}

	setHash(0, "plain.txt", 0)
	setHash(1, "big.txt", 1)

	blocks := make([]uint32, blockEntries*4)
	copy(blocks, []uint32{
		pos0, uint32(len(plain)), uint32(len(plain)), uint32(FileExists),
		pos1, compSize1, uint32(len(big)), uint32(FileExists | FileCompress),
	})

	if mutate != nil {
		mutate(hash, blocks)
	}

	encrypt(hash, hashString("(hash table)", 3))
	encrypt(blocks, hashString("(block table)", 3))

	_ = binary.Write(&body, binary.LittleEndian, hash)
	_ = binary.Write(&body, binary.LittleEndian, blocks)

	out := body.Bytes()
	copy(out, "MPQ\x1a")
	binary.LittleEndian.PutUint32(out[4:], 32)
	binary.LittleEndian.PutUint32(out[8:], uint32(len(out)))
	binary.LittleEndian.PutUint16(out[12:], 0)
	binary.LittleEndian.PutUint16(out[14:], blockShift)
	binary.LittleEndian.PutUint32(out[16:], hashOff)
	binary.LittleEndian.PutUint32(out[20:], blockOff)
	binary.LittleEndian.PutUint32(out[24:], hashEntries)
	binary.LittleEndian.PutUint32(out[28:], blockEntries)

	return out
}

// readAll reads a bounded amount of every file of the archive.
func readAll(t testing.TB, path string) {
	m, err := FromFile(path)
	if err != nil {
		return
	}
	defer m.Close()

	for _, h := range m.hashes {
		if h.BlockIndex >= uint32(len(m.blocks)) {
			continue
		}

		b := *m.blocks[h.BlockIndex]

		s, err := CreateStream(m, &b, "file")
		if err != nil {
			continue
		}

		buf := make([]byte, 1<<16)
		for i := 0; i < 64; i++ {
			n, err := s.Read(buf, 0, uint32(len(buf)))
			if err != nil || n == 0 {
				break
			}
		}
	}

	_, _ = m.ReadFile("plain.txt")
	_, _ = m.ReadFile("big.txt")
	_, _ = m.ReadFile("(listfile)")
}

func TestSynthMPQ(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.mpq")
	if err := os.WriteFile(p, synthMPQ(nil), 0o600); err != nil {
		t.Fatal(err)
	}

	m, err := FromFile(p)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	d, err := m.ReadFile("plain.txt")
	if err != nil || !bytes.HasPrefix(d, []byte("plain file ")) || len(d) != 440 {
		t.Fatalf("plain: %v %d", err, len(d))
	}

	d, err = m.ReadFile("big.txt")
	if err != nil || len(d) != 9200 || !bytes.HasPrefix(d, []byte("compressed sector data ")) {
		t.Fatalf("big: %v %d", err, len(d))
	}
}

// TestHostileMPQ: hand-made hostile headers and tables must fail with errors, quickly, without
// allocating by their claimed sizes.
func TestHostileMPQ(t *testing.T) {
	huge := uint32(0xFFFFFFF0)

	cases := map[string]func([]byte) []byte{
		"huge hash table": func(b []byte) []byte { binary.LittleEndian.PutUint32(b[24:], huge); return b },
		"huge block table": func(b []byte) []byte {
			binary.LittleEndian.PutUint32(b[28:], 0x40000000)
			return b
		},
		"block size shift 40": func(b []byte) []byte { binary.LittleEndian.PutUint16(b[14:], 40); return b },
		"table offsets beyond file": func(b []byte) []byte {
			binary.LittleEndian.PutUint32(b[16:], huge)
			return b
		},
		"truncated":   func(b []byte) []byte { return b[:len(b)/2] },
		"header only": func(b []byte) []byte { return b[:32] },
	}

	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "h.mpq")
			if err := os.WriteFile(p, f(synthMPQ(nil)), 0o600); err != nil {
				t.Fatal(err)
			}

			readAll(t, p)
		})
	}

	// block table entries claiming huge or inconsistent sizes
	for name, mut := range map[string]func(hash, block []uint32){
		"huge size":       func(h, b []uint32) { b[2] = 0xFFFFFFFF },
		"huge compressed": func(h, b []uint32) { b[5] = 0xFFFFFFFF; b[6] = 0xFFFFFFF0 },
		"position beyond": func(h, b []uint32) { b[0] = 0xFFFFFFFF },
		"hash to block 99": func(h, b []uint32) {
			h[3] = 99
		},
		"colliding names": func(h, b []uint32) {
			h[4], h[5], h[6], h[7] = h[0], h[1], h[2], 1
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "h.mpq")
			if err := os.WriteFile(p, synthMPQ(mut), 0o600); err != nil {
				t.Fatal(err)
			}

			readAll(t, p)
		})
	}
}

// FuzzArchive opens arbitrary bytes as an MPQ and reads every file; it must never panic, hang or
// allocate by a size read from the file.
func FuzzArchive(f *testing.F) {
	f.Add(synthMPQ(nil))
	f.Add(synthMPQ(func(h, b []uint32) { b[2] = 0xFFFFFFFF }))
	f.Add(synthMPQ(func(h, b []uint32) { b[3] |= uint32(FileEncrypted | FileFixKey) }))
	f.Add(synthMPQ(func(h, b []uint32) { b[7] |= uint32(FileSingleUnit) }))
	f.Add(synthMPQ(func(h, b []uint32) { b[3] = uint32(FileExists | FileImplode) }))
	f.Add([]byte("MPQ\x1a"))
	f.Add([]byte{})

	dir := f.TempDir()

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			return
		}

		p := filepath.Join(dir, "fuzz.mpq")
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Skip()
		}

		readAll(t, p)
	})
}

// FuzzSector feeds arbitrary bytes to the sector decompressors.
func FuzzSector(f *testing.F) {
	f.Add([]byte{2, 0x78, 0x9c, 3, 0}, uint32(16))
	f.Add([]byte{8, 0, 4}, uint32(16))
	f.Add([]byte{0x40, 0, 0, 0, 0}, uint32(16))
	f.Add([]byte{0x41, 0, 0, 0, 0}, uint32(16))
	f.Add([]byte{0x81, 0, 0, 0, 0, 0, 0}, uint32(16))
	f.Add([]byte{}, uint32(0))

	f.Fuzz(func(t *testing.T, data []byte, expected uint32) {
		if expected > 1<<20 {
			expected = 1 << 20
		}

		_, _ = decompressMulti(data, expected)
		_, _ = pkDecompressHint(data, int(expected))
	})
}
''',
    'd2common/d2fileformats/d2mpq/mpq_trozos_test.go': r'''package d2mpq

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// lectorAnotado: entrega los datos de a lo sumo `tope` bytes por Read y ANOTA el pedido mas grande que le
// hicieron. Es el instrumento de DC-002.28: lo que no entra en la Dreamcast es el pedido de la tabla
// entera de un tiron (la de hash de d2data.mpq son 1.048.576 bytes), y eso se mide con el TAMANO DEL
// PEDIDO, no con la memoria del proceso (que en el host sobra y taparia el defecto).
type lectorAnotado struct {
	datos []byte
	pos   int
	tope  int
	mayor int
}

func (l *lectorAnotado) Read(p []byte) (int, error) {
	if len(p) > l.mayor {
		l.mayor = len(p)
	}

	if l.pos >= len(l.datos) {
		return 0, io.EOF
	}

	n := len(p)
	if n > l.tope {
		n = l.tope
	}

	if l.pos+n > len(l.datos) {
		n = len(l.datos) - l.pos
	}

	copy(p, l.datos[l.pos:l.pos+n])

	l.pos += n

	return n, nil
}

// TestDecryptTablePorTrozosLeeDeAPoco: la tabla se descifra por trozos, palabra por palabra igual que de un
// solo tiron, y NUNCA se pide mas de un trozo de una vez. Los tamanos elegidos cruzan el marco del trozo
// (1.024 entradas) y el ultimo caso trae la tabla cortada, que tiene que entregar lo que llego y despues el
// error (igual que el camino viejo).
func TestDecryptTablePorTrozosLeeDeAPoco(t *testing.T) {
	tests := []struct {
		name    string
		entries uint32
		extra   int // bytes que faltan del final (tabla cortada)
	}{
		{"una entrada", 1, 0},
		{"justo antes del marco", tablaTrozoEntradas - 1, 0},
		{"justo el marco", tablaTrozoEntradas, 0},
		{"una entrada despues del marco", tablaTrozoEntradas + 1, 0},
		{"cuatro trozos y monedas", 4*tablaTrozoEntradas + 7, 0},
		{"cortada en el segundo trozo", 3 * tablaTrozoEntradas, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			datos := make([]byte, int(tt.entries)*16-tt.extra)
			for i := range datos {
				datos[i] = byte(i*29 + 11)
			}

			quiero, quieroErr := naiveDecryptTable(bytes.NewReader(datos), tt.entries, "(hash table)")

			lector := &lectorAnotado{datos: datos, tope: 1 << 20}
			var got []uint32

			trozos := 0

			gotErr := decryptTablePorTrozos(lector, tt.entries, "(hash table)", func(trozo []uint32) error {
				got = append(got, trozo...)
				trozos++

				return nil
			})

			if (quieroErr == nil) != (gotErr == nil) {
				t.Fatalf("error %v, quiero %v", gotErr, quieroErr)
			}

			// Con la tabla cortada las dos versiones dejan de hablar del mismo largo: el camino por trozos
			// entrega las palabras que SI llegaron (12.283 contra 12.288 en el caso cortado) y el de un solo
			// tiron devuelve el arreglo completo con la cola en cero. Lo que importa es que lo que llego sea
			// igual palabra por palabra, y que los dos contesten el error.
			n := len(got)
			if len(quiero) < n {
				n = len(quiero)
			}

			for i := 0; i < n; i++ {
				if got[i] != quiero[i] {
					t.Fatalf("palabra %d = %#x, quiero %#x", i, got[i], quiero[i])
				}
			}

			// EL TOPE DEL PEDIDO, en un numero FIJO y no en el de la constante: mas de 64 KiB de una vez se
			// va al monton de C de la consola (`gc_external_alloc`, el umbral de "objeto grande" de
			// libgodc) y es justo el pedido que no entra (medido: `gc_external_alloc: OOM` con la tabla de
			// d2data en 1.048.576 bytes). Con un tope que se moviera con la constante, un trozo de 128 KiB
			// pasaria el control: el control tiene que morder.
			if pedidoMaximo := 64 << 10; lector.mayor > pedidoMaximo {
				t.Fatalf("pidio %d bytes de una vez: el tope es %d (mas que eso se va al monton de C en la consola)", lector.mayor, pedidoMaximo)
			}

			// y que de verdad vaya por trozos: una tabla mas larga que un trozo tiene que llegar en varios
			esperadosTrozos := int((tt.entries + tablaTrozoEntradas - 1) / tablaTrozoEntradas)

			if trozos != esperadosTrozos {
				t.Fatalf("llego en %d trozos, esperaba %d", trozos, esperadosTrozos)
			}
		})
	}
}

// TestTablasPorTrozosUnMPQDeVerdad: un MPQ armado por el generador de las pruebas, con las dos tablas mas
// grandes que tres trozos, abierto con `FromFile` (el camino del motor) tiene que dar EXACTAMENTE las
// mismas dos tablas que la version que descifra la tabla entera palabra por palabra.
//
// Es la misma clase de control que el corpus del host: el numero (las entradas) sale del MPQ, no de la
// prueba, y la comparacion es contra el camino viejo, no contra una constante escrita a mano.
func TestTablasPorTrozosUnMPQDeVerdad(t *testing.T) {
	const (
		hashEntradas   = uint32(4096) // 4 trozos: cruza el marco 4 veces
		bloqueEntradas = uint32(3000) // casi 3 trozos
	)

	p := filepath.Join(t.TempDir(), "grande.mpq")
	if err := os.WriteFile(p, synthMPQSized(hashEntradas, bloqueEntradas, nil), 0o600); err != nil {
		t.Fatal(err)
	}

	m, err := FromFile(p)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	// la referencia: la tabla cruda del archivo, descifrada entera palabra por palabra
	crudoHash := make([]byte, int(hashEntradas)*16)
	if _, err := m.file.ReadAt(crudoHash, int64(m.header.HashTableOffset)); err != nil {
		t.Fatal(err)
	}

	quiero, err := naiveDecryptTable(bytes.NewReader(crudoHash), hashEntradas, "(hash table)")
	if err != nil {
		t.Fatal(err)
	}

	// el mismo armado que readHashTable: una entrada por ranura (las vacias colapsan en una sola clave)
	esperado := make(map[uint64]*Hash)

	for n, i := uint32(0), uint32(0); i < hashEntradas; n, i = n+4, i+1 {
		esperado[(uint64(quiero[n])<<32)|uint64(quiero[n+1])] = &Hash{
			A:          quiero[n],
			B:          quiero[n+1],
			Locale:     uint16(quiero[n+2] >> 16),
			Platform:   uint16(quiero[n+2] & 0xFFFF),
			BlockIndex: quiero[n+3],
		}
	}

	if len(m.hashes) != len(esperado) {
		t.Fatalf("el mapa tiene %d entradas, quiero %d", len(m.hashes), len(esperado))
	}

	for clave, e := range esperado {
		got, ok := m.hashes[clave]
		if !ok {
			t.Fatalf("falta la clave %#x", clave)
		}

		if *got != *e {
			t.Fatalf("la clave %#x = %+v, quiero %+v", clave, *got, *e)
		}
	}

	// y la tabla de bloques, entrada por entrada
	crudoBloque := make([]byte, int(bloqueEntradas)*16)
	if _, err := m.file.ReadAt(crudoBloque, int64(m.header.BlockTableOffset)); err != nil {
		t.Fatal(err)
	}

	quieroBloques, err := naiveDecryptTable(bytes.NewReader(crudoBloque), bloqueEntradas, "(block table)")
	if err != nil {
		t.Fatal(err)
	}

	if len(m.blocks) != int(bloqueEntradas) {
		t.Fatalf("%d bloques, quiero %d", len(m.blocks), bloqueEntradas)
	}

	for i := uint32(0); i < bloqueEntradas; i++ {
		n := i * 4
		got := m.blocks[i]

		if got.FilePosition != quieroBloques[n] || got.CompressedFileSize != quieroBloques[n+1] ||
			got.UncompressedFileSize != quieroBloques[n+2] || uint32(got.Flags) != quieroBloques[n+3] {
			t.Fatalf("bloque %d = %+v, quiero %d/%d/%d/%d", i, *got,
				quieroBloques[n], quieroBloques[n+1], quieroBloques[n+2], quieroBloques[n+3])
		}
	}
}

// TestDecryptTableSigueSiendoLaReferencia: el camino de la tabla entera (el que usan las pruebas y el
// banco de pruebas) tiene que seguir dando lo mismo que el de por trozos: si los dos se van de las manos a
// la vez, la comparacion de arriba no lo caza.
func TestDecryptTableSigueSiendoLaReferencia(t *testing.T) {
	const entradas = uint32(3*tablaTrozoEntradas + 5)

	datos := make([]byte, entradas*16)
	for i := range datos {
		datos[i] = byte(i*7 + 3)
	}

	quiero, err := naiveDecryptTable(bytes.NewReader(datos), entradas, "(block table)")
	if err != nil {
		t.Fatal(err)
	}

	got, err := decryptTable(bytes.NewReader(datos), entradas, "(block table)")
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != len(quiero) {
		t.Fatalf("%d palabras, quiero %d", len(got), len(quiero))
	}

	for i := range quiero {
		if got[i] != quiero[i] {
			t.Fatalf("palabra %d = %#x, quiero %#x", i, got[i], quiero[i])
		}
	}
}
''',
    'd2common/d2fileformats/d2mpq/mpq_equivalencia_test.go': None,
}


# ================================================================================================
# EL ESTADO DEL ARBOL: sano (las tablas del MPQ como BYTES, DC-002.33) o pristino (un objeto de Go por
# entrada, que es lo que moria en el semiespacio del GC de la consola).
# ================================================================================================
def sha(ruta):
    with open(ruta, "rb") as f:
        return hashlib.sha256(f.read()).hexdigest()[:16]


def lee(ruta):
    try:
        with open(ruta) as f:
            return f.read()
    except FileNotFoundError:
        return None


def escribe(ruta, texto):
    if texto is None:
        if os.path.exists(ruta):
            os.remove(ruta)
        return

    with open(ruta, "w") as f:
        f.write(texto)


def estado():
    iguales_sano, iguales_pristino, otros = [], [], []

    for p in ARCHIVOS:
        actual = lee(p)

        if actual == SANO[p]:
            iguales_sano.append(p)
        elif actual == PRISTINO[p]:
            iguales_pristino.append(p)
        else:
            otros.append(p)

    return iguales_sano, iguales_pristino, otros


def informe(etiqueta):
    s, p, o = estado()
    print("%s: %d/%d como el arreglo, %d/%d como el pristino, %d en otro estado"
          % (etiqueta, len(s), len(ARCHIVOS), len(p), len(ARCHIVOS), len(o)))

    for ruta in o:
        print("   OTRO ESTADO: %s (sha %s)" % (ruta, sha(ruta) if os.path.exists(ruta) else "ausente"))

    return o


def pon(estado_deseado, etiqueta):
    for p in ARCHIVOS:
        escribe(p, estado_deseado[p])

    s, pr, o = estado()
    print("%s: aplicado" % etiqueta)

    assert not o, "quedaron %d archivos en otro estado: %s" % (len(o), o)

    if estado_deseado is SANO:
        assert len(s) == len(ARCHIVOS), "el arreglo no quedo: %d/%d" % (len(s), len(ARCHIVOS))
        print("   las 7 fuentes de d2mpq quedaron con el arreglo (tablas como BYTES + el diferencial del host)")
    else:
        assert len(pr) == len(ARCHIVOS), "el pristino no quedo: %d/%d" % (len(pr), len(ARCHIVOS))
        print("   las 7 fuentes de d2mpq quedaron como estaban (un objeto de Go por entrada)")


def main():
    args = sys.argv[1:] or ["--estado"]

    if "--estado" in args:
        informe("estado")
    elif "--pristino" in args:
        pon(PRISTINO, "mutante DC-002.33 (un objeto de Go por entrada)")
    elif "--sano" in args:
        pon(SANO, "DC-002.33 (las tablas como bytes)")
    else:
        print(__doc__)
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
