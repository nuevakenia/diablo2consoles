package d2dcc

import (
	"errors"
	"fmt"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2datautils"
)

const dccFileSignature = 0x74
const directionOffsetMultiplier = 8

const maxFramesPerDirection = 1024 // real files have at most a few dozen

// DCC represents a DCC file.
type DCC struct {
	Signature          int
	Version            int
	NumberOfDirections int
	FramesPerDirection int
	Directions         []*DCCDirection
	directionOffsets   []int
	fileData           []byte
}

// Load loads a DCC file.
func Load(fileData []byte) (dcc *DCC, err error) {
	// the bit reader and the cell decoder panic on truncated or corrupt data
	defer d2datautils.RecoverError("dcc", &err)

	result, err := readHeader(fileData)
	if err != nil {
		return nil, err
	}

	result.Directions = make([]*DCCDirection, result.NumberOfDirections)

	for i := 0; i < result.NumberOfDirections; i++ {
		result.Directions[i] = result.decodeDirection(i)
	}

	return result, nil
}

// LoadDirection decodes ONE direction of a DCC file and leaves the rest decoded as nil (DC-002.49).
//
// PARA QUE: los graficos de los personajes traen 16 direcciones (8 los monstruos) y quien anima un
// personaje en UNA direccion necesita 1 de 16. Load() decodifica las 16 SIEMPRE: en la Dreamcast eso es
// 16 veces el trabajo y 16 veces la basura (medido en DC-002.49: la capa LG del cuerpo de la amazona,
// 63.718 B de archivo, son 202.352 B de pixeles decodificados, de los que se usan 12.784), y el
// recolector de libgodc se atraganta con esa basura: la capa LG del cuerpo de la amazona tardaba 42.608 ms
// con Load() (medido en la consola, DC-002.49). La direccion pedida es la UNICA que queda decodificada.
func LoadDirection(fileData []byte, direction int) (dcc *DCC, err error) {
	defer d2datautils.RecoverError("dcc", &err)

	result, err := readHeader(fileData)
	if err != nil {
		return nil, err
	}

	if direction < 0 || direction >= result.NumberOfDirections {
		return nil, fmt.Errorf("dcc: direction %d out of range (the file has %d)",
			direction, result.NumberOfDirections)
	}

	result.Directions = make([]*DCCDirection, result.NumberOfDirections)
	result.Directions[direction] = result.decodeDirection(direction)

	return result, nil
}

// readHeader: la cabecera y la tabla de offsets de las direcciones, sin decodificar ninguna.
func readHeader(fileData []byte) (*DCC, error) {
	result := &DCC{
		fileData: fileData,
	}

	var bm = d2datautils.CreateBitMuncher(fileData, 0)

	result.Signature = int(bm.GetByte())

	if result.Signature != dccFileSignature {
		return nil, errors.New("signature expected to be 0x74 but it is not")
	}

	result.Version = int(bm.GetByte())
	result.NumberOfDirections = int(bm.GetByte())
	result.FramesPerDirection = int(bm.GetInt32())

	if result.NumberOfDirections == 0 || result.FramesPerDirection <= 0 ||
		result.FramesPerDirection > maxFramesPerDirection || result.FramesPerDirection > len(fileData) {
		return nil, fmt.Errorf("dcc: %d directions of %d frames is out of range",
			result.NumberOfDirections, result.FramesPerDirection)
	}

	if bm.GetInt32() != 1 {
		return nil, errors.New("this value isn't 1. It has to be 1")
	}

	bm.GetInt32() // TotalSizeCoded

	result.directionOffsets = make([]int, result.NumberOfDirections)

	for i := 0; i < result.NumberOfDirections; i++ {
		result.directionOffsets[i] = int(bm.GetInt32())
	}

	return result, nil
}

// decodeDirection decodes and returns the given direction
func (d *DCC) decodeDirection(direction int) *DCCDirection {
	return CreateDCCDirection(d2datautils.CreateBitMuncher(d.fileData,
		d.directionOffsets[direction]*directionOffsetMultiplier), d)
}

// Clone creates a copy of the DCC
func (d *DCC) Clone() *DCC {
	clone := *d
	copy(clone.directionOffsets, d.directionOffsets)
	copy(clone.fileData, d.fileData)
	clone.Directions = make([]*DCCDirection, len(d.Directions))

	for i := range d.Directions {
		cloneDirection := *d.Directions[i]
		clone.Directions = append(clone.Directions, &cloneDirection)
	}

	return &clone
}
