package d2mpq

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
