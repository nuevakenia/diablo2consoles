package d2mpq

import (
	"encoding/binary"
	"errors"
	"io"
)

// Header Represents a MPQ file
type Header struct {
	Magic             [4]byte
	HeaderSize        uint32
	ArchiveSize       uint32
	FormatVersion     uint16
	BlockSize         uint16
	HashTableOffset   uint32
	BlockTableOffset  uint32
	HashTableEntries  uint32
	BlockTableEntries uint32
}

func (mpq *MPQ) readHeader() error {
	if _, err := mpq.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// DC-002.27: la cabecera se lee CAMPO POR CAMPO, no con binary.Read sobre el struct.
	//
	// Medido en la Dreamcast (POC motor-disco, disco.gdi): `binary.Read(.., &mpq.header)` contesta
	// "encoding/binary: Read no soporta ese tipo de dato (ver DC-002.5)" y con eso el arranque del motor
	// se queda en su cartel "file not found: /cd/juego/patch_d2.mpq" -- porque d2app/initialization.go:85
	// TIRA el error de abajo y solo muestra su propio mensaje. El `binary` de la consola es la stdlib
	// minima: sabe leer escalares y []byte, y un struct necesitaria reflector, que esta fuera del port
	// (console/dc/std no tiene reflect). La cabecera son 32 bytes con los campos pegados (sin relleno):
	// 4 de la firma, 6 enteros de 4 bytes y 2 de 2 bytes.
	var b [headerSize]byte

	if _, err := io.ReadFull(mpq.file, b[:]); err != nil {
		return err
	}

	copy(mpq.header.Magic[:], b[0:4])
	mpq.header.HeaderSize = binary.LittleEndian.Uint32(b[4:8])
	mpq.header.ArchiveSize = binary.LittleEndian.Uint32(b[8:12])
	mpq.header.FormatVersion = binary.LittleEndian.Uint16(b[12:14])
	mpq.header.BlockSize = binary.LittleEndian.Uint16(b[14:16])
	mpq.header.HashTableOffset = binary.LittleEndian.Uint32(b[16:20])
	mpq.header.BlockTableOffset = binary.LittleEndian.Uint32(b[20:24])
	mpq.header.HashTableEntries = binary.LittleEndian.Uint32(b[24:28])
	mpq.header.BlockTableEntries = binary.LittleEndian.Uint32(b[28:32])

	if string(mpq.header.Magic[:]) != "MPQ\x1A" {
		return errors.New("invalid mpq header")
	}

	// the sector size is 0x200<<BlockSize: a shift past 31 would give a zero sector size (division by zero)
	if mpq.header.BlockSize > maxBlockSizeShift {
		return errors.New("invalid mpq block size")
	}

	return nil
}

// headerSize: the MPQ header is 32 bytes (0x20): [4]byte + 5*uint32 + 2*uint16 with no padding, so the raw
// read is byte for byte what binary.Read wrote on the host.
const headerSize = 32

// maxBlockSizeShift limits the sector size to 0x200<<15 = 16 MiB (Diablo II uses 3, 4 KiB).
const maxBlockSizeShift = 15
