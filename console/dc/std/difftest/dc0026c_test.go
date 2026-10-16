// Prueba diferencial de DC-002.6c: bufio (Scanner, Reader y Writer).
//
// El Scanner tiene el mismo contrato que el de Go, asi que se compara token por token contra el de Go,
// pero con una diferencia importante en la entrada: al mio se le da un Reader propio que sirve los bytes
// DE A POCO (incluso de a uno), para forzar todos los recortes del buffer. El de Go se prueba con
// strings.Reader. Si los dos sacan los mismos tokens, el mio esta bien.
package difftest

import (
	realbufio "bufio"
	"strings"
	"testing"

	mybufio "d2std/bufio"
	"d2std/io"
)

// readerTrozo: un io.Reader de la consola que entrega los bytes en trozos de a lo sumo `trozo`.
type readerTrozo struct {
	datos []byte
	pos   int
	trozo int
}

func (r *readerTrozo) Read(p []byte) (int, error) {
	if r.pos >= len(r.datos) {
		return 0, io.EOF
	}
	n := len(p)
	if r.trozo > 0 && n > r.trozo {
		n = r.trozo
	}
	if n > len(r.datos)-r.pos {
		n = len(r.datos) - r.pos
	}
	copy(p, r.datos[r.pos:r.pos+n])
	r.pos += n
	return n, nil
}

// textos: los casos, elegidos para pegarle a los bordes del scanner.
var textos = []string{
	"",
	"una linea",
	"una linea\n",
	"una\ndos\ntres\n",
	"una\ndos\ntres",
	"\n",
	"\n\n\n",
	"una\n\ndos\n",
	"con retorno\r\nlinea dos\r\n",
	"a\r\n\r\nb",
	"espacios   al   final   \n  y al principio\n",
	"tabuladas\tcon\ttabs\n",
	strings.Repeat("x", 5000) + "\ndespues\n",
	strings.Repeat("larga ", 2000) + "\nfin sin salto",
}

func tokens(esc *realbufio.Scanner) []string {
	var t []string
	for esc.Scan() {
		t = append(t, esc.Text())
	}
	return t
}

func TestBufioScanLines(t *testing.T) {
	for i, texto := range textos {
		esc := realbufio.NewScanner(strings.NewReader(texto))
		esperado := tokens(esc)
		if err := esc.Err(); err != nil {
			t.Fatalf("caso %d: el scanner de Go fallo: %v", i, err)
		}
		// con el buffer entero y con trozos de 1, 3, 7 y 13 bytes
		for _, trozo := range []int{0, 1, 3, 7, 13} {
			mio := mybufio.NewScanner(&readerTrozo{datos: []byte(texto), trozo: trozo})
			var tengo []string
			for mio.Scan() {
				tengo = append(tengo, mio.Text())
			}
			if err := mio.Err(); err != nil {
				t.Errorf("caso %d (trozo %d): mi scanner fallo: %v", i, trozo, err)
				continue
			}
			if len(esperado) != len(tengo) {
				t.Errorf("caso %d (trozo %d): Go da %d tokens y el mio %d\n  Go:    %q\n  mio:   %q",
					i, trozo, len(esperado), len(tengo), esperado, tengo)
				continue
			}
			for k := range esperado {
				if esperado[k] != tengo[k] {
					t.Errorf("caso %d (trozo %d) token %d: Go da %q y el mio %q", i, trozo, k, esperado[k], tengo[k])
				}
			}
		}
	}
}

func TestBufioScanWords(t *testing.T) {
	for i, texto := range textos {
		esc := realbufio.NewScanner(strings.NewReader(texto))
		esc.Split(realbufio.ScanWords)
		esperado := tokens(esc)
		for _, trozo := range []int{0, 1, 5} {
			mio := mybufio.NewScanner(&readerTrozo{datos: []byte(texto), trozo: trozo})
			mio.Split(mybufio.ScanWords)
			var tengo []string
			for mio.Scan() {
				tengo = append(tengo, mio.Text())
			}
			if len(esperado) != len(tengo) {
				t.Errorf("palabras caso %d (trozo %d): Go da %d y el mio %d\n  Go: %q\n  mio: %q",
					i, trozo, len(esperado), len(tengo), esperado, tengo)
				continue
			}
			for k := range esperado {
				if esperado[k] != tengo[k] {
					t.Errorf("palabras caso %d (trozo %d) token %d: Go %q, mio %q", i, trozo, k, esperado[k], tengo[k])
				}
			}
		}
	}
}

func TestBufioReadBytes(t *testing.T) {
	// ReadBytes/ReadString: tienen que devolver lo mismo que Go, con el separador incluido
	for i, texto := range textos {
		for _, trozo := range []int{0, 1, 4} {
			real := realbufio.NewReader(strings.NewReader(texto))
			esperados := []string{}
			for {
				l, err := real.ReadString('\n')
				if l != "" {
					esperados = append(esperados, l)
				}
				if err != nil {
					break
				}
			}
			mio := mybufio.NewReader(&readerTrozo{datos: []byte(texto), trozo: trozo})
			tengo := []string{}
			for {
				l, err := mio.ReadString('\n')
				if l != "" {
					tengo = append(tengo, l)
				}
				if err != nil {
					break
				}
			}
			if len(esperados) != len(tengo) {
				t.Errorf("ReadString caso %d (trozo %d): Go da %d y el mio %d", i, trozo, len(esperados), len(tengo))
				continue
			}
			for k := range esperados {
				if esperados[k] != tengo[k] {
					t.Errorf("ReadString caso %d (trozo %d) pieza %d: Go %q, mio %q", i, trozo, k, esperados[k], tengo[k])
				}
			}
		}
	}
}

// TestBufioReaderPorTrozos: Read con buffers de distinto tamano tiene que devolver el MISMO texto (los
// cortes pueden ser distintos: lo que no puede cambiar es el contenido).
func TestBufioReaderPorTrozos(t *testing.T) {
	for i, texto := range textos {
		for _, tam := range []int{1, 2, 8, 64, 4096, 8192} {
			mio := mybufio.NewReader(&readerTrozo{datos: []byte(texto), trozo: 3})
			buf := make([]byte, tam)
			var salida []byte
			for {
				n, err := mio.Read(buf)
				if n > 0 {
					salida = append(salida, buf[:n]...)
				}
				if err != nil {
					break
				}
			}
			if string(salida) != texto {
				t.Errorf("Reader caso %d (buffer %d): devolvio %q y tenia que devolver %q",
					i, tam, string(salida), texto)
			}
		}
	}
}

// TestBufioWriter: el Writer tiene que juntar y vaciar, y lo que sale tiene que ser el texto entero.
type sumidero struct{ datos []byte }

func (s *sumidero) Write(p []byte) (int, error) {
	s.datos = append(s.datos, p...)
	return len(p), nil
}

func TestBufioWriter(t *testing.T) {
	texto := "una linea\notra mas\n" + strings.Repeat("relleno ", 500) + "\n"
	s := &sumidero{}
	escr := mybufio.NewWriter(s)
	// de a pedacitos para forzar los vaciados con el buffer lleno
	for i := 0; i < len(texto); i += 7 {
		f := i + 7
		if f > len(texto) {
			f = len(texto)
		}
		if _, err := escr.WriteString(texto[i:f]); err != nil {
			t.Fatalf("WriteString fallo: %v", err)
		}
	}
	if err := escr.Flush(); err != nil {
		t.Fatalf("Flush fallo: %v", err)
	}
	if string(s.datos) != texto {
		t.Errorf("el Writer escribio %q y tenia que escribir %q", string(s.datos), texto)
	}
	if escr.Buffered() != 0 {
		t.Errorf("despues de Flush quedaron %d bytes en el buffer", escr.Buffered())
	}
}
