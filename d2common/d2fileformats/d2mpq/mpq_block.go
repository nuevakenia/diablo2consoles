package d2mpq

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
