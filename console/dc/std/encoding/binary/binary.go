// Paquete binary: lectura y escritura de enteros en orden de bytes, como el de Go.
//
// Por que existe: el motor lee los archivos del juego (los MPQ, los DC6, las tablas DAT) con
// binary.LittleEndian y binary.Read (28 archivos). En sh-elf no hay biblioteca estandar de Go.
//
// LIMITE DECLARADO: Read soporta los tipos escalares (punteros a uint8/16/32/64, int8/16/32/64,
// float32/64 y los []byte). Go ademas acepta ESTRUCTURAS y arreglos, y para eso usa reflect: aca
// devuelve un error explicito en vez de mentir. El camino sin reflect (leer el descriptor del tipo
// y recorrer los campos por offset) es la tarjeta DC-002.5 y esta anotado en el README.
package binary

import (
	"d2std/io"
	"unsafe"
)

// float64frombits y float32frombits reinterpretan los bits: es el unico uso de unsafe del paquete
// (sin reinterpretar no hay forma de armar un float desde sus bytes).
func float64frombits(b uint64) float64 { return *(*float64)(unsafe.Pointer(&b)) }
func float32frombits(b uint32) float32 { return *(*float32)(unsafe.Pointer(&b)) }

// ByteOrder es el orden de bytes, con la misma forma que la de Go.
type ByteOrder interface {
	Uint16([]byte) uint16
	Uint32([]byte) uint32
	Uint64([]byte) uint64
	PutUint16([]byte, uint16)
	PutUint32([]byte, uint32)
	PutUint64([]byte, uint64)
	// los Append* son los que el motor usa en d2networking/d2mp/events.go (Go los tiene desde 1.19).
	// AppendByte NO va: Go no lo tiene en ByteOrder (lo caza la prueba diferencial contra Go).
	AppendUint16([]byte, uint16) []byte
	AppendUint32([]byte, uint32) []byte
	AppendUint64([]byte, uint64) []byte
	String() string
}

type littleEndian struct{}

func (littleEndian) Uint16(b []byte) uint16 { return uint16(b[0]) | uint16(b[1])<<8 }
func (littleEndian) Uint32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}
func (littleEndian) Uint64(b []byte) uint64 {
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
}
func (littleEndian) PutUint16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}
func (littleEndian) PutUint32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
func (littleEndian) PutUint64(b []byte, v uint64) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
	b[4] = byte(v >> 32)
	b[5] = byte(v >> 40)
	b[6] = byte(v >> 48)
	b[7] = byte(v >> 56)
}
func (littleEndian) AppendUint16(b []byte, v uint16) []byte {
	return append(b, byte(v), byte(v>>8))
}
func (littleEndian) AppendUint32(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
func (littleEndian) AppendUint64(b []byte, v uint64) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24),
		byte(v>>32), byte(v>>40), byte(v>>48), byte(v>>56))
}
func (littleEndian) String() string { return "LittleEndian" }

type bigEndian struct{}

func (bigEndian) Uint16(b []byte) uint16 { return uint16(b[1]) | uint16(b[0])<<8 }
func (bigEndian) Uint32(b []byte) uint32 {
	return uint32(b[3]) | uint32(b[2])<<8 | uint32(b[1])<<16 | uint32(b[0])<<24
}
func (bigEndian) Uint64(b []byte) uint64 {
	return uint64(b[7]) | uint64(b[6])<<8 | uint64(b[5])<<16 | uint64(b[4])<<24 |
		uint64(b[3])<<32 | uint64(b[2])<<40 | uint64(b[1])<<48 | uint64(b[0])<<56
}
func (bigEndian) PutUint16(b []byte, v uint16) {
	b[1] = byte(v)
	b[0] = byte(v >> 8)
}
func (bigEndian) PutUint32(b []byte, v uint32) {
	b[3] = byte(v)
	b[2] = byte(v >> 8)
	b[1] = byte(v >> 16)
	b[0] = byte(v >> 24)
}
func (bigEndian) PutUint64(b []byte, v uint64) {
	b[7] = byte(v)
	b[6] = byte(v >> 8)
	b[5] = byte(v >> 16)
	b[4] = byte(v >> 24)
	b[3] = byte(v >> 32)
	b[2] = byte(v >> 40)
	b[1] = byte(v >> 48)
	b[0] = byte(v >> 56)
}
func (bigEndian) AppendUint16(b []byte, v uint16) []byte {
	return append(b, byte(v>>8), byte(v))
}
func (bigEndian) AppendUint32(b []byte, v uint32) []byte {
	return append(b, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}
func (bigEndian) AppendUint64(b []byte, v uint64) []byte {
	return append(b, byte(v>>56), byte(v>>48), byte(v>>40), byte(v>>32),
		byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}
func (bigEndian) String() string { return "BigEndian" }

// LittleEndian y BigEndian son los dos ordenes.
var (
	LittleEndian ByteOrder = littleEndian{}
	BigEndian    ByteOrder = bigEndian{}
)

// errTipo es el error de un tipo que Read todavia no sabe leer (ver el limite declarado arriba).
type errTipo struct{ que string }

func (e errTipo) Error() string {
	return "encoding/binary: Read no soporta " + e.que + " (ver DC-002.5)"
}

// ErrTipoDeDato es el error exportado, por si el llamador lo quiere distinguir.
var ErrTipoDeDato error = errTipo{"ese tipo de dato"}

// Read lee la representacion binaria del dato desde r, en el orden pedido, y la deja en data.
// data tiene que ser un puntero a un escalar (o un []byte).
func Read(r io.Reader, order ByteOrder, data interface{}) error {
	switch v := data.(type) {
	case *uint8:
		var b [1]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return err
		}
		*v = b[0]
		return nil
	case *int8:
		var b [1]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return err
		}
		*v = int8(b[0])
		return nil
	case *uint16:
		var b [2]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return err
		}
		*v = order.Uint16(b[:])
		return nil
	case *int16:
		var b [2]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return err
		}
		*v = int16(order.Uint16(b[:]))
		return nil
	case *uint32:
		var b [4]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return err
		}
		*v = order.Uint32(b[:])
		return nil
	case *int32:
		var b [4]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return err
		}
		*v = int32(order.Uint32(b[:]))
		return nil
	case *uint64:
		var b [8]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return err
		}
		*v = order.Uint64(b[:])
		return nil
	case *int64:
		var b [8]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return err
		}
		*v = int64(order.Uint64(b[:]))
		return nil
	case *float32:
		var b [4]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return err
		}
		*v = float32frombits(order.Uint32(b[:]))
		return nil
	case *float64:
		var b [8]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return err
		}
		*v = float64frombits(order.Uint64(b[:]))
		return nil
	case *[]byte:
		if _, err := io.ReadFull(r, *v); err != nil {
			return err
		}
		return nil
	}
	return errTipo{"ese tipo de dato"}
}


// Write escribe la representacion binaria de data en w (lo inverso de Read). Lo pide el pl2 al
// serializar (Marshal). Mismas reglas que Read: structs de tamaNo fijo, sin relleno.
func Write(w interface{ Write([]byte) (int, error) }, order ByteOrder, data interface{}) error {
	buf := &buffer{}
	if err := writeData(buf, order, data); err != nil {
		return err
	}
	_, err := w.Write(buf.datos)
	return err
}

// buffer es un acumulador minimo (no se usa bytes.Buffer para no depender de el aca adentro).
type buffer struct{ datos []byte }

func (b *buffer) Write(p []byte) (int, error) {
	b.datos = append(b.datos, p...)
	return len(p), nil
}
