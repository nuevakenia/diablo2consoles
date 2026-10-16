// Package bytes: el minimo que el motor usa de verdad (bytes.Buffer), medido al compilar
// d2common/d2datautils y d2fileformats/d2dc6 para SH-4 en DC-002.3.
//
// Limites declarados (lo que NO esta, y se agrega cuando un paquete del motor lo pida): las funciones
// de busqueda y comparacion (Index, Equal, Contains, Split, ...) y las de conversion de texto. En
// cuanto existan, la prueba diferencial del host las cubre como a las demas.
package bytes

// Buffer es un buffer de bytes, con el subconjunto que el motor usa.
type Buffer struct {
	buf []byte
}

// NewBuffer arma un Buffer con el contenido inicial dado (como en Go, el buffer usa el slice).
func NewBuffer(buf []byte) *Buffer { return &Buffer{buf: buf} }

// NewBufferString arma un Buffer con el texto dado.
func NewBufferString(s string) *Buffer { return &Buffer{buf: []byte(s)} }

// Write agrega los bytes al final.
func (b *Buffer) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	return len(p), nil
}

// WriteString agrega el texto al final.
func (b *Buffer) WriteString(s string) (int, error) {
	b.buf = append(b.buf, s...)
	return len(s), nil
}

// WriteByte agrega un byte al final.
func (b *Buffer) WriteByte(c byte) error {
	b.buf = append(b.buf, c)
	return nil
}

// Bytes devuelve el contenido (sin copiar, como en Go).
func (b *Buffer) Bytes() []byte { return b.buf }

// Len devuelve cuantos bytes hay.
func (b *Buffer) Len() int { return len(b.buf) }

// Reset vacia el buffer sin soltar la memoria.
func (b *Buffer) Reset() { b.buf = b.buf[:0] }

// String devuelve el contenido como texto.
func (b *Buffer) String() string { return string(b.buf) }
