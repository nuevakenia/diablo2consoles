// Package io: solo lo que necesita nuestro fmt para escribir (Writer y sus variantes).
//
// En la consola el destino final lo enchufa el arranque (consola serie o pantalla); aca solo
// vive el contrato.
package io

// Writer es el contrato de escritura, igual que en Go.
type Writer interface {
	Write(p []byte) (n int, err error)
}

// StringWriter lo implementan los que saben escribir un string sin convertirlo antes.
type StringWriter interface {
	WriteString(s string) (n int, err error)
}

// Discard se traga todo (util para medir sin ensuciar la salida).
var Discard Writer = discard{}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

// WriteString escribe s en w usando WriteString si esta disponible.
func WriteString(w Writer, s string) (n int, err error) {
	if sw, ok := w.(StringWriter); ok {
		return sw.WriteString(s)
	}
	return w.Write([]byte(s))
}

// ByteWriter: requerido por encoding/binary.
type ByteWriter interface {
	WriteByte(c byte) error
}
