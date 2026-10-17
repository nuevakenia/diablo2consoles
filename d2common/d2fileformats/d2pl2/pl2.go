package d2pl2

import (
	"fmt"
	"unsafe"
)

// PL2 represents a palette file.
type PL2 struct {
	BasePalette PL2Palette

	LightLevelVariations [32]PL2PaletteTransform
	InvColorVariations   [16]PL2PaletteTransform
	SelectedUintShift    PL2PaletteTransform
	AlphaBlend           [3][256]PL2PaletteTransform
	AdditiveBlend        [256]PL2PaletteTransform
	MultiplicativeBlend  [256]PL2PaletteTransform
	HueVariations        [111]PL2PaletteTransform
	RedTones             PL2PaletteTransform
	GreenTones           PL2PaletteTransform
	BlueTones            PL2PaletteTransform
	UnknownVariations    [14]PL2PaletteTransform
	MaxComponentBlend    [256]PL2PaletteTransform
	DarkendColorShift    PL2PaletteTransform

	TextColors      [13]PL2Color24Bits
	TextColorShifts [13]PL2PaletteTransform
}

// Load lee los datos binarios del pl2.
//
// El PL2 es un blob de bytes: TODOS sus campos son arrays de bytes (PL2Color y PL2Color24Bits son bytes,
// los transforms son [256]uint8), sin relleno ni alineacion especial. O sea que el struct en memoria es
// exactamente el archivo, byte a byte, en la Dreamcast (SH-4 es little-endian, igual que el formato).
// Por eso se copia el buffer tal cual, con unsafe: es lo que hacia restruct, pero sin reflexion.
//
// ANTES usaba restruct (que arma la lectura por reflexion a partir de los tags del struct). El formato
// PL2 es todo arrays contiguos de bytes sin relleno, asi que la lectura binaria de la biblioteca
// estandar hace exactamente lo mismo sin traer reflexion: se cambia restruct por binary.Read y el
// resultado es el mismo byte a byte. (Es el unico lugar del motor que usaba restruct: medido.)
func Load(data []byte) (*PL2, error) {
	largo := int(unsafe.Sizeof(PL2{}))
	if len(data) < largo {
		return nil, fmt.Errorf("pl2: se esperaban %d bytes, llegaron %d", largo, len(data))
	}

	result := &PL2{}
	// OJO, ACA NO SE PUEDE USAR unsafe.Slice: el go.mod del repo dice go 1.16, y unsafe.Slice existe recien
	// en go 1.17. Con unsafe.Slice, el compilador del HOST (el que hornea los assets, go build ./console/dc/bake)
	// rechaza el paquete entero con "unsafe.Slice requires go1.17 or later" y el horneado de assets no corre.
	// La forma de antes es la misma cosa: un puntero al arreglo, recortado al largo pedido.
	destino := (*[1 << 30]byte)(unsafe.Pointer(result))[:largo:largo]
	copy(destino, data[:largo])

	return result, nil
}

// Marshal arma los bytes del pl2 (lo inverso de Load).
func (p *PL2) Marshal() []byte {
	largo := int(unsafe.Sizeof(PL2{}))
	origen := (*[1 << 30]byte)(unsafe.Pointer(p))[:largo:largo]

	salida := make([]byte, largo)
	copy(salida, origen)

	return salida
}
