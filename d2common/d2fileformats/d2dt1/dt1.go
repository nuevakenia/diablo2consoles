package d2dt1

import (
	"fmt"
	"sort"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2datautils"
)

// BlockDataFormat represents the format of the block data
type BlockDataFormat int16

const (
	// BlockFormatRLE specifies the block format is RLE encoded
	BlockFormatRLE BlockDataFormat = 0

	// BlockFormatIsometric specifies the block format isometrically encoded
	BlockFormatIsometric BlockDataFormat = 1
)

const (
	numUnknownHeaderBytes = 260
	knownMajorVersion     = 7
	knownMinorVersion     = 6
	tileHeaderSize        = 96 // bytes of one tile header in the file
	blockHeaderSize       = 20 // bytes of one block header in the file
	numUnknownTileBytes1  = 4
	numUnknownTileBytes2  = 4
	numUnknownTileBytes3  = 7
	numUnknownTileBytes4  = 12
	numUnknownBlockBytes  = 2
)

// DT1 represents a DT1 file.
type DT1 struct {
	majorVersion  int32
	minorVersion  int32
	numberOfTiles int32
	bodyPosition  int32
	Tiles         []Tile
}

// New creates a new DT1
func New() *DT1 {
	result := &DT1{
		majorVersion: knownMajorVersion,
		minorVersion: knownMinorVersion,
	}

	return result
}

// LoadDT1 loads a DT1 record
//
// Los bloques quedan APUNTANDO al archivo crudo (`fileData`): `ReadBytes` devuelve una subslica
// (`stream_reader.go`), no una copia. Eso esta bien cuando el archivo se queda vivo (el camino de la PC:
// el cache guarda el DT1 y el crudo lo recolecta el GC del host). NO sirve cuando el crudo se va a
// reusar para el archivo siguiente: para eso esta `LoadDT1CopiaBloques` (DC-002.78).
//
//nolint:funlen,gocognit,gocyclo // Can't reduce
func LoadDT1(fileData []byte) (dt1 *DT1, err error) {
	return loadDT1(fileData, false)
}

// LoadDT1CopiaBloques carga un DT1 COPIANDO los datos codificados de cada bloque, para que el resultado no
// dependa del archivo crudo y este se pueda reusar (o soltar) enseguida.
//
// DC-002.78 -- POR QUE HACE FALTA. En la Dreamcast todo lo que pasa de 64 KB se va al monton externo del
// runtime y no lo devuelve nadie (medido en DC-002.77 con `console/dc/poc/sonda-gc-277`): con el parseo de
// siempre, cada .dt1 leido (300 KB - 1 MB) deja su crudo tomado para siempre, y la segunda zona muere por
// OOM. Copiando los bloques, el crudo se puede reusar para el archivo siguiente y lo que QUEDA son las
// copias, una por bloque, que miden 8..1.181 B (medido con `console/dc/medidas/bloques`): todas por debajo
// de `GC_LARGE_OBJECT_THRESHOLD`, o sea que viven en el semiespacio y el recolector las devuelve cuando el
// cache desaloja la entrada.
//
// El precio es el del `TamanoDT1`: la copia pesa lo mismo que el archivo crudo (el .dt1 son casi todos
// datos de bloque). Se paga UNA vez, en el semiespacio del recolector, en lugar de PARA SIEMPRE en el
// monton externo.
func LoadDT1CopiaBloques(fileData []byte) (*DT1, error) {
	return loadDT1(fileData, true)
}

//nolint:funlen,gocognit,gocyclo // Can't reduce
func loadDT1(fileData []byte, copiarBloques bool) (dt1 *DT1, err error) {
	defer d2datautils.RecoverError("dt1", &err)

	result := &DT1{}
	br := d2datautils.CreateStreamReader(fileData)

	result.majorVersion, err = br.ReadInt32()
	if err != nil {
		return nil, err
	}

	result.minorVersion, err = br.ReadInt32()
	if err != nil {
		return nil, err
	}

	if result.majorVersion != knownMajorVersion || result.minorVersion != knownMinorVersion {
		const fmtErr = "expected to have a version of 7.6, but got %d.%d instead"
		return nil, fmt.Errorf(fmtErr, result.majorVersion, result.minorVersion)
	}

	br.SkipBytes(numUnknownHeaderBytes)

	result.numberOfTiles, err = br.ReadInt32()
	if err != nil {
		return nil, err
	}

	result.bodyPosition, err = br.ReadInt32()
	if err != nil {
		return nil, err
	}

	br.SetPosition(uint64(result.bodyPosition))

	// a tile header is 96 bytes; a hostile count must not allocate more than the file can hold
	if err = d2datautils.CheckCount("dt1 tiles", int64(result.numberOfTiles), tileHeaderSize, br.Remaining()); err != nil {
		return nil, err
	}

	result.Tiles = make([]Tile, result.numberOfTiles)
	blocksLeft := br.Size() / blockHeaderSize // all blocks of all tiles together

	for tileIdx := range result.Tiles {
		tile := Tile{}

		tile.Direction, err = br.ReadInt32()
		if err != nil {
			return nil, err
		}

		tile.RoofHeight, err = br.ReadInt16()
		if err != nil {
			return nil, err
		}

		var matFlagBytes uint16

		matFlagBytes, err = br.ReadUInt16()
		if err != nil {
			return nil, err
		}

		tile.MaterialFlags = NewMaterialFlags(matFlagBytes)

		tile.Height, err = br.ReadInt32()
		if err != nil {
			return nil, err
		}

		tile.Width, err = br.ReadInt32()
		if err != nil {
			return nil, err
		}

		br.SkipBytes(numUnknownTileBytes1)

		tile.Type, err = br.ReadInt32()
		if err != nil {
			return nil, err
		}

		tile.Style, err = br.ReadInt32()
		if err != nil {
			return nil, err
		}

		tile.Sequence, err = br.ReadInt32()
		if err != nil {
			return nil, err
		}

		tile.RarityFrameIndex, err = br.ReadInt32()
		if err != nil {
			return nil, err
		}

		tile.unknown2, err = br.ReadBytes(numUnknownTileBytes2)
		if err != nil {
			return nil, err
		}

		if copiarBloques {
			// tambien los bytes sueltos del tile: son una subslica del crudo igual que los bloques
			tile.unknown2 = copiarBytes(tile.unknown2)
		}

		for i := range tile.SubTileFlags {
			var subtileFlagBytes byte

			subtileFlagBytes, err = br.ReadByte()
			if err != nil {
				return nil, err
			}

			tile.SubTileFlags[i] = NewSubTileFlags(subtileFlagBytes)
		}

		br.SkipBytes(numUnknownTileBytes3)

		tile.blockHeaderPointer, err = br.ReadInt32()
		if err != nil {
			return nil, err
		}

		tile.blockHeaderSize, err = br.ReadInt32()
		if err != nil {
			return nil, err
		}

		var numBlocks int32

		numBlocks, err = br.ReadInt32()
		if err != nil {
			return nil, err
		}

		if numBlocks < 0 || uint64(numBlocks) > blocksLeft {
			return nil, fmt.Errorf("dt1: %d blocks in tile %d exceed the file", numBlocks, tileIdx)
		}

		blocksLeft -= uint64(numBlocks)
		tile.Blocks = make([]Block, numBlocks)

		br.SkipBytes(numUnknownTileBytes4)

		result.Tiles[tileIdx] = tile
	}

	for tileIdx := range result.Tiles {
		tile := &result.Tiles[tileIdx]
		br.SetPosition(uint64(tile.blockHeaderPointer))

		for blockIdx := range tile.Blocks {
			result.Tiles[tileIdx].Blocks[blockIdx].X, err = br.ReadInt16()
			if err != nil {
				return nil, err
			}

			result.Tiles[tileIdx].Blocks[blockIdx].Y, err = br.ReadInt16()
			if err != nil {
				return nil, err
			}

			br.SkipBytes(numUnknownBlockBytes)

			result.Tiles[tileIdx].Blocks[blockIdx].GridX, err = br.ReadByte()
			if err != nil {
				return nil, err
			}

			result.Tiles[tileIdx].Blocks[blockIdx].GridY, err = br.ReadByte()
			if err != nil {
				return nil, err
			}

			result.Tiles[tileIdx].Blocks[blockIdx].format, err = br.ReadInt16()
			if err != nil {
				return nil, err
			}

			result.Tiles[tileIdx].Blocks[blockIdx].Length, err = br.ReadInt32()
			if err != nil {
				return nil, err
			}

			br.SkipBytes(numUnknownBlockBytes)

			result.Tiles[tileIdx].Blocks[blockIdx].FileOffset, err = br.ReadInt32()
			if err != nil {
				return nil, err
			}
		}

		for blockIndex, block := range tile.Blocks {
			br.SetPosition(uint64(tile.blockHeaderPointer + block.FileOffset))

			if block.Length < 0 {
				return nil, fmt.Errorf("dt1: negative block length %d", block.Length)
			}

			encodedData, err := br.ReadBytes(int(block.Length))
			if err != nil {
				return nil, err
			}

			if copiarBloques {
				encodedData = copiarBytes(encodedData)
			}

			tile.Blocks[blockIndex].EncodedData = encodedData
		}
	}

	return result, nil
}

// copiarBytes: la copia de un bloque (o de los bytes sueltos de un tile) a memoria propia, para que el
// DT1 no dependa del archivo crudo. Ver `LoadDT1CopiaBloques`.
func copiarBytes(datos []byte) []byte {
	if len(datos) == 0 {
		return datos
	}

	copia := make([]byte, len(datos))
	copy(copia, datos)

	return copia
}

// Marshal encodes dt1 data back to byte slice
func (d *DT1) Marshal() []byte {
	sw := d2datautils.CreateStreamWriter()

	// header
	sw.PushInt32(d.majorVersion)
	sw.PushInt32(d.minorVersion)
	sw.PushBytes(d.unknownHeaderBytes()...)
	sw.PushInt32(d.numberOfTiles)
	sw.PushInt32(d.bodyPosition)

	// Step 1 - encoding tiles headers
	for i := 0; i < len(d.Tiles); i++ {
		sw.PushInt32(d.Tiles[i].Direction)
		sw.PushInt16(d.Tiles[i].RoofHeight)
		sw.PushUint16(d.Tiles[i].MaterialFlags.Encode())
		sw.PushInt32(d.Tiles[i].Height)
		sw.PushInt32(d.Tiles[i].Width)
		sw.PushBytes(d.Tiles[i].unknown1()...)
		sw.PushInt32(d.Tiles[i].Type)
		sw.PushInt32(d.Tiles[i].Style)
		sw.PushInt32(d.Tiles[i].Sequence)
		sw.PushInt32(d.Tiles[i].RarityFrameIndex)
		sw.PushBytes(d.Tiles[i].unknown2...)

		for _, j := range d.Tiles[i].SubTileFlags {
			sw.PushBytes(j.Encode())
		}

		sw.PushBytes(d.Tiles[i].unknown3()...)
		sw.PushInt32(d.Tiles[i].blockHeaderPointer)
		sw.PushInt32(d.Tiles[i].blockHeaderSize)
		sw.PushInt32(int32(len(d.Tiles[i].Blocks)))
		sw.PushBytes(d.Tiles[i].unknown4()...)
	}

	// we must sort blocks first
	blocks := make(map[int][]Block)
	for i := range d.Tiles {
		blocks[int(d.Tiles[i].blockHeaderPointer)] = d.Tiles[i].Blocks
	}

	keys := make([]int, 0, len(blocks))
	for i := range blocks {
		keys = append(keys, i)
	}

	sort.Ints(keys)

	// Step 2 - encoding blocks
	for i := 0; i < len(keys); i++ {
		// Step 2.1 - encoding blocks' header
		for j := range blocks[keys[i]] {
			sw.PushInt16(blocks[keys[i]][j].X)
			sw.PushInt16(blocks[keys[i]][j].Y)
			sw.PushBytes(blocks[keys[i]][j].unknown1()...)
			sw.PushBytes(blocks[keys[i]][j].GridX)
			sw.PushBytes(blocks[keys[i]][j].GridY)
			sw.PushInt16(blocks[keys[i]][j].format)
			sw.PushInt32(blocks[keys[i]][j].Length)
			sw.PushBytes(blocks[keys[i]][j].unknown2()...)
			sw.PushInt32(blocks[keys[i]][j].FileOffset)
		}

		// Step 2.2 - encoding blocks' data
		for j := range blocks[keys[i]] {
			sw.PushBytes(blocks[keys[i]][j].EncodedData...)
		}
	}

	return sw.GetBytes()
}

func (d *DT1) unknownHeaderBytes() []byte {
	result := make([]byte, numUnknownHeaderBytes)
	return result
}
