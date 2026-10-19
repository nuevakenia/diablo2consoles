package difftest

// DC-002.47: el caso que la difftest de DC-002.45 NO probaba.
//
// El stream del MPQ no se comporta como un archivo plano: cuando el resto del archivo entra en un solo
// pedido, devuelve LOS BYTES Y `io.EOF` EN LA MISMA LLAMADA. Le pasa a toda tabla que mide menos que el
// buffer (LvlWarp.txt: 3.935 B contra 4.096), y a cualquier tabla en su ultimo trozo.
//
// La difftest de DC-002.45 usaba un lector de trozos que devuelve (n, nil) y el fin recien en la llamada
// siguiente, asi que este caso nunca se midio. En la consola el arranque se quedaba en la 3a tabla y el
// motor terminaba en su pantalla de error con el texto "EOF": esta prueba aisla si el que se come los bytes
// es nuestro `d2std` cuando el error viene pegado a los datos.
//
// DC-002.47 (el otro defecto del instrumento, medido con la traza): EL EOF DE LA CONSOLA NO ES EL DE GO.
// `d2std/io.EOF` es un valor propio (`errors.New("EOF")`) y el csv de la consola compara contra ESE: un
// lector de prueba que devuelva el `io.EOF` del host no dispara ninguna de las ramas de fin (el `readLine`
// no convierte "datos + fin en la misma llamada" y el `readRecord` no reconoce el fin), asi que la prueba
// pasa midiendo otra cosa. Aca todos los lectores devuelven `mio.EOF` y las comparaciones de fin van por
// `esEOF` (el TEXTO del error), porque Go y d2std usan valores distintos para el mismo EOF.

import (
	realcsv "encoding/csv"
	"io"
	"strings"
	"testing"

	mycsv "d2std/encoding/csv"
	mio "d2std/io"
)

// esEOF: si el error es un fin de entrada, mirando el TEXTO. No se puede comparar contra un unico `io.EOF`:
// el de Go y el de `d2std/io` son VALORES distintos (mismo texto, otra identidad).
func esEOF(err error) bool {
	return err != nil && err.Error() == "EOF"
}

// lectorDeFilas: lo que comparten el csv de Go y el de la consola (una sola llamada que interesa).
type lectorDeFilas interface {
	Read() ([]string, error)
}

// lectorDelMpq imita al stream del MPQ: el primer Read devuelve todos los bytes JUNTO con el fin.
// `fin` es el EOF DE ESE LADO (io.EOF para el csv de Go, mio.EOF para el de la consola): ver la cabecera.
type lectorDelMpq struct {
	datos []byte
	fin   error
	yaDio bool
}

func (l *lectorDelMpq) Read(p []byte) (int, error) {
	if l.yaDio {
		return 0, l.fin
	}

	l.yaDio = true

	n := copy(p, l.datos)
	if n < len(l.datos) {
		// no entro todo en este pedido: queda para el siguiente
		l.datos = l.datos[n:]
		l.yaDio = false
		return n, nil
	}

	return n, l.fin
}

// lectorPorTrozos es el de DC-002.45: nunca devuelve datos y error juntos.
type lectorPorTrozos struct {
	datos []byte
	trozo int
	fin   error
}

func (l *lectorPorTrozos) Read(p []byte) (int, error) {
	if len(l.datos) == 0 {
		return 0, l.fin
	}

	t := l.trozo
	if t > len(l.datos) {
		t = len(l.datos)
	}
	if t > len(p) {
		t = len(p)
	}

	n := copy(p, l.datos[:t])
	l.datos = l.datos[t:]

	return n, nil
}

// leerTodo lee registros con el lector dado hasta el fin, y devuelve los registros y el error final.
func leerTodo(lector func(io.Reader) lectorDeFilas, r io.Reader) ([][]string, error) {
	rd := lector(r)
	var (
		salida [][]string
		ultimo error
	)

	for {
		rec, err := rd.Read()
		if esEOF(err) {
			// el registro que vino CON el fin: el contrato de Go permite `Read` devolviendo los dos, y la
			// consola tambien (el ultimo trozo trae la fila y el fin juntos). Descartarlo aca mediria otra
			// cosa que la consola.
			if len(rec) > 0 {
				salida = append(salida, rec)
			}
			return salida, ultimo
		}
		if err != nil {
			return salida, err
		}
		salida = append(salida, rec)
		ultimo = nil
	}
}

// tabla chica: entra entera en el primer pedido (el caso de LvlWarp.txt).
const tablaChica = "a\tb\tc\n1\t2\t3\n4\t5\t6\n"

func TestDC00247ElMpqDevuelveDatosYFinJuntos(t *testing.T) {
	t.Log("DC-002.47: mismo lector con forma de stream del MPQ (datos + fin pegados), con el EOF de la consola")

	casos := []struct {
		nombre string
		bytes  []byte
	}{
		{"3.935 B (LvlWarp.txt), entra entero", []byte(strings.Repeat(tablaChica, 90))},
		{"11 KB (LvlTypes.txt), no entra entero", []byte(strings.Repeat(tablaChica, 260))},
		// la ultima linea SIN salto de linea del final: el ultimo trozo trae la fila y el fin JUNTOS, y la
		// fila tiene que salir igual (es el caso del `\r` final, que se cae solo al final de la entrada)
		{"3.935 B sin salto final", []byte(strings.TrimSuffix(strings.Repeat(tablaChica, 90), "\n"))},
		{"una sola fila sin salto final", []byte("a\tb\tc")},
	}

	for _, c := range casos {
		// la de Go, que es la verdad (con el EOF de Go)
		deGo, errGo := leerTodo(func(r io.Reader) lectorDeFilas {
			return realcsv.NewReader(r)
		}, &lectorDelMpq{datos: append([]byte(nil), c.bytes...), fin: io.EOF})

		// la nuestra (con el EOF de la consola)
		nuestra, errNuestro := leerTodo(func(r io.Reader) lectorDeFilas {
			return mycsv.NewReader(r)
		}, &lectorDelMpq{datos: append([]byte(nil), c.bytes...), fin: mio.EOF})

		if len(deGo) != len(nuestra) {
			t.Errorf("DC-002.47 %s: Go leyo %d registros y d2std %d (err Go=%v, err nuestro=%v)",
				c.nombre, len(deGo), len(nuestra), errGo, errNuestro)
		}
		if esEOF(errGo) != esEOF(errNuestro) {
			t.Errorf("DC-002.47 %s: fin distinto: Go=%v, d2std=%v", c.nombre, errGo, errNuestro)
		}

		for i := range deGo {
			if i >= len(nuestra) {
				break
			}
			if strings.Join(deGo[i], "|") != strings.Join(nuestra[i], "|") {
				t.Errorf("DC-002.47 %s: registro %d distinto:\n  Go:    %q\n  d2std: %q",
					c.nombre, i, deGo[i], nuestra[i])
			}
		}

		t.Logf("DC-002.47 %-42s Go=%d registros d2std=%d registros  ¿fin? Go=%v d2std=%v",
			c.nombre, len(deGo), len(nuestra), esEOF(errGo), esEOF(errNuestro))
	}
}

// TestDC00247LosTrozosSiguenBien deja amarrada la prueba de DC-002.45 (datos y fin separados) para que
// arreglar lo de arriba no la rompa.
func TestDC00247LosTrozosSiguenBien(t *testing.T) {
	datos := []byte(strings.Repeat(tablaChica, 90))

	deGo, errGo := leerTodo(func(r io.Reader) lectorDeFilas {
		return realcsv.NewReader(r)
	}, &lectorPorTrozos{datos: append([]byte(nil), datos...), trozo: 100, fin: io.EOF})

	nuestra, errNuestro := leerTodo(func(r io.Reader) lectorDeFilas {
		return mycsv.NewReader(r)
	}, &lectorPorTrozos{datos: append([]byte(nil), datos...), trozo: 100, fin: mio.EOF})

	if len(deGo) != len(nuestra) || esEOF(errGo) != esEOF(errNuestro) {
		t.Errorf("DC-002.47 trozos: Go=%d (¿fin? %v) d2std=%d (¿fin? %v)",
			len(deGo), esEOF(errGo), len(nuestra), esEOF(errNuestro))
	}

	t.Logf("DC-002.47 trozos de 100 B: Go=%d registros d2std=%d registros", len(deGo), len(nuestra))
}
