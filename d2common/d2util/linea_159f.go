package d2util

import "os"

// DC-002.159f -- LAS LINEAS DEL INSTRUMENTO, ARMADAS SIN `fmt` Y SIN `append`.
//
// POR QUE EXISTE. La sonda de la basura por cuadro (DC-002.159d) imprime dos lineas cada 15 cuadros: la del
// heroe (`HEROE159 ...`) y la del tramo (`GC159D ...`). Las dos se armaban con `fmt.Sprintf` y concatenacion de
// strings, y la del tramo con ~16 `Sprintf` mas 16 concatenaciones (ver `gc_159d.go`). Medido en la consola con
// las marcas finas de esta tarjeta: esas lineas costaban ~4.500 B/cuadro de basura, MAS QUE TODO EL DIBUJO DEL
// HEROE JUNTO. O sea: el instrumento que mide la basura era la mitad de la basura.
//
// SEGUNDA MEDICION, y la que manda sobre el diseno: cambiar `Sprintf` por un constructor con `append` sobre un
// `[]byte` reusado bajo el costo solo de 4.500 a 3.100 B/cuadro. En gccgo cada `append` de un trozo paga la
// copia del buffer entero (una linea de ~500 caracteres se arma con ~50 trozos: ~n²/2 bytes de basura por
// linea, medido). Por eso esto NO usa `append` ni `make`: es un ARREGLO FIJO con un cursor, se escribe byte por
// byte y se manda el recorte en UNA escritura. Asi el instrumento no pide memoria, punto.
//
// LA LINEA SALE CON EL MISMO FORMATO que daba `Infof`: `[<prefijo>][INFO]<reset> <texto>\r\n`. Los lectores de
// la evidencia (`tabla159d.py`, `vivos159d.py`) siguen buscando lo mismo en el log.

// linea159fLargo: lo que entra en una linea. Las dos mas largas que se imprimen (el tramo con el neto y las
// marcas finas, y el estado del heroe) miden ~600 y ~300 caracteres, asi que 2 KB sobra; lo que no entra se
// corta (y se nota: la linea queda sin el final).
const linea159fLargo = 2048

// Linea159f: la linea del instrumento, en un arreglo fijo con un cursor.
type Linea159f struct {
	buf [linea159fLargo]byte
	n   int
}

// linea159f: la linea compartida. Se arma y se envia de a una: no hay dos a la vez.
var linea159f Linea159f

// NuevaLinea159f: arranca una linea del instrumento con el prefijo y el nivel del logger.
func (l *Logger) NuevaLinea159f() *Linea159f {
	ln := &linea159f
	ln.n = 0

	ln.buf[ln.n] = '['
	ln.n++

	if l != nil {
		ln.Tx(l.prefix)
	}

	ln.Tx("][INFO]")
	ln.Tx(colorEscapeReset)

	ln.buf[ln.n] = ' '
	ln.n++

	return ln
}

// Tx: texto tal cual (byte por byte: nada de `append`).
func (ln *Linea159f) Tx(s string) *Linea159f {
	for i := 0; i < len(s); i++ {
		if ln.n >= linea159fLargo {
			return ln
		}

		ln.buf[ln.n] = s[i]
		ln.n++
	}

	return ln
}

// I: un entero en decimal (sin `strconv` ni `fmt`).
func (ln *Linea159f) I(v int) *Linea159f {
	if v == 0 {
		return ln.Tx("0")
	}

	if v < 0 {
		ln.buf[ln.n] = '-'
		ln.n++
		v = -v
	}

	var d [20]byte

	i := len(d)

	for v > 0 {
		i--
		d[i] = byte('0' + v%10)
		v /= 10
	}

	for ; i < len(d); i++ {
		if ln.n >= linea159fLargo {
			return ln
		}

		ln.buf[ln.n] = d[i]
		ln.n++
	}

	return ln
}

// B: un entero con la unidad de bytes (lo que usa toda la evidencia: `123B`).
func (ln *Linea159f) B(v int) *Linea159f {
	ln.I(v)

	return ln.Tx("B")
}

// F2: un numero con DOS decimales, redondeado (la celda del heroe sale asi: 26.40).
func (ln *Linea159f) F2(v float64) *Linea159f {
	if v < 0 {
		ln.buf[ln.n] = '-'
		ln.n++
		v = -v
	}

	centesimas := int(v*100 + 0.5)

	ln.I(centesimas / 100)
	ln.Tx(".")

	d := centesimas % 100

	if d < 10 {
		ln.Tx("0")
	}

	return ln.I(d)
}

// Si: un booleano como `true`/`false` (lo que imprimia `%v`).
func (ln *Linea159f) Si(v bool) *Linea159f {
	if v {
		return ln.Tx("true")
	}

	return ln.Tx("false")
}

// Enviar: cierra la linea y la escribe DE UNA VEZ, derecho por la consola serie.
//
// POR QUE NO VA POR EL LOGGER (medido, no supuesto). `l.Write` termina en el `fmt.Output` de la stdlib de la
// consola, que por defecto es `builtinOutput`: `print(string(p))`, o sea una conversion a string NUEVA por cada
// escritura y encima por el `print` del runtime. Con las marcas finas de esta tarjeta, la linea del heroe quedo
// en `cArmaEst=3 B` (armarla no cuesta nada) y `cEnviaEst=1.186 B`; la del tramo, `cEnviaTra=2.050 B`. O sea:
// el 100% del costo de las lineas era la escritura, y era el 44% de la basura del cuadro entero.
//
// `os.Stdout` en la consola es un `*os.File` con la marca de consola: su `Write` es UNA llamada al extern
// (`d2c_escribir_consola` -> `dbgio_write_buffer_xlat`), sin string ni buffer intermedio.
//
// El nivel y el prefijo se respetan igual: el prefijo lo puso `NuevaLinea159f` y el nivel se mira aca.
func (ln *Linea159f) Enviar(l *Logger) {
	if l == nil || l.level < LogLevelInfo {
		return
	}

	ln.Tx("\r\n")

	if _, err := os.Stdout.Write(ln.buf[:ln.n]); err != nil {
		// en la consola no hay a quien avisar de un error de escritura: la linea se pierde y el que mide lo
		// nota porque falta, no porque el instrumento se caiga
		return
	}
}

// Texto: la linea como string (para lo que la necesita asi; NO se usa en el camino por cuadro).
func (ln *Linea159f) Texto() string { return string(ln.buf[:ln.n]) }
