// Package gzip: la descompresion de gzip para la consola, sobre el mismo puente a la zlib de KOS que
// usa compress/zlib.
//
// gzip es zlib con otra cabecera: la zlib de KOS ya descomprime las dos (se le pide inflateInit2 con
// ventana 15+32, que es "decidi vos si es zlib o gzip"). Asi que aca no hay algoritmo nuevo: se envuelve
// el mismo puente y se expone la interfaz de Go.
//
// Lo que NO esta: el lado de escritura (NewWriter) ni las variantes con diccionario. El juego lee
// assets comprimidos y no escribe gzip en la consola.
package gzip

import (
	"d2std/hash"
	"d2std/hash/crc32"
	"d2std/io"
	"unsafe"
)

// DC-002.20: el inflate EN STREAMING del puente de la zlib de KOS. Antes se usaba el de una sola vez
// (`d2c_zlib_inflate`), que pide el resultado ENTERO en un destino calculado con una REGLA (4 x comprimido
// + 64 KB). Medido con el asset EMBEBIDO del motor: comprimido 2.209 B, la regla da 74.372 B y el asset
// descomprimido son 98.358 B, asi que la zlib devolvia error, este paquete lo traducia a `gzip: invalid
// header` (un mensaje que mentia sobre la causa) y el motor paniqueaba al arrancar.
//
// Con el streaming el destino es el buffer del que lee: 512 B en el corpus, lo que pida el motor.
//
//extern d2c_zlib_stream_init
func d2cZlibStreamInit() int32

//extern d2c_zlib_stream_step
func d2cZlibStreamStep(slot int32, src uintptr, srcLen int32, dst uintptr, dstCap int32) int32

//extern d2c_zlib_stream_consumido
func d2cZlibStreamConsumido(slot int32) int32

//extern d2c_zlib_stream_fin
func d2cZlibStreamFin(slot int32) int32

//extern d2c_zlib_stream_end
func d2cZlibStreamEnd(slot int32) int32

// Header: la cabecera de un miembro gzip. Se completa al leer (el motor la mira para saber si es texto
// o binario, que es lo que hace gzip.Header.Text).
type Header struct {
	Name    string
	Comment string
	ModTime interface{} // time.Time en Go; aca se deja como esta porque el reloj de la consola no esta
	OS      byte
	Text    bool
}

// ErrChecksum y ErrHeader: los mismos errores de Go.
type errorString string

func (e errorString) Error() string { return string(e) }

const (
	ErrChecksum = errorString("gzip: invalid checksum")
	ErrHeader   = errorString("gzip: invalid header")
)

// Reader: el lector de gzip. Descomprime EN STREAMING: pide los bytes en el buffer que le pasan (ver el
// comentario de los externs) en vez de materializar el resultado entero.
type Reader struct {
	comprimido []byte
	pos        int // cuantos bytes del comprimido ya consumio la zlib
	slot       int32
	abierto    bool
	fin        bool
	cerrado    bool
	err        error
	Header     Header
}

// NewReader devuelve un lector que descomprime r.
func NewReader(r io.Reader) (*Reader, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	g := &Reader{comprimido: data, slot: -1}
	// la cabecera de gzip son 10 bytes fijos: 1f 8b, el metodo, las banderas, la fecha, y los dos
	// ultimos. Se miran solo para llenar Header (si no es gzip, la zlib va a fallar igual y con mejor
	// mensaje que inventado aca).
	if len(data) >= 10 && data[0] == 0x1f && data[1] == 0x8b {
		g.Header.OS = data[9]
		g.Header.Text = data[3]&1 != 0
	}
	return g, nil
}

// Read: los bytes descomprimidos, de a lo que entre en p.
//
// El bucle es por el caso normal del inflate: un paso puede CONSUMIR todo el comprimido y escribir CERO
// bytes (la zlib esta llenando su ventana), y otro puede devolver menos de lo que entra en p. Se corta
// cuando hubo salida, cuando la zlib dijo que el flujo termino, o cuando un paso no avanzo por NINGUN lado
// (ese ultimo caso es un defecto del puente y se declara como error en vez de colgar el motor en un bucle).
func (g *Reader) Read(p []byte) (int, error) {
	if g.cerrado {
		return 0, io.EOF
	}
	if g.fin {
		return 0, io.EOF
	}
	if len(p) == 0 {
		return 0, nil
	}
	if !g.abierto {
		g.slot = d2cZlibStreamInit()
		if g.slot < 0 {
			g.err = ErrHeader

			return 0, g.err
		}
		g.abierto = true
	}

	for {
		resto := g.comprimido[g.pos:]
		if len(resto) == 0 {
			// se acabo el comprimido y la zlib no dijo que termino: el flujo esta cortado
			g.err = ErrHeader

			return 0, g.err
		}

		n := d2cZlibStreamStep(g.slot, uintptr(unsafe.Pointer(&resto[0])), int32(len(resto)),
			uintptr(unsafe.Pointer(&p[0])), int32(len(p)))
		consumido := int(d2cZlibStreamConsumido(g.slot))
		termino := d2cZlibStreamFin(g.slot) != 0

		g.pos += consumido

		if n < 0 {
			g.err = ErrHeader

			return 0, g.err
		}
		if termino {
			g.fin = true
		}
		if n > 0 {
			return int(n), nil
		}
		if g.fin {
			return 0, io.EOF
		}
		if consumido == 0 {
			// ni salida ni consumo: el puente no avanza. Mejor el error que un bucle infinito.
			g.err = ErrHeader

			return 0, g.err
		}
	}
}

// Close: suelta el slot del inflate (el estado de la zlib vive en C, no en el monton de Go).
func (g *Reader) Close() error {
	if g.abierto {
		d2cZlibStreamEnd(g.slot)
		g.abierto = false
	}
	g.cerrado = true
	g.comprimido = nil
	return nil
}

// ---------------------------------------------------------------- el lado de escritura
//
// PARA QUE SE USA: d2networking/d2server/d2udpclientconnection comprime los paquetes que manda por la red
// (gzip.NewWriterLevel con la compresion maxima). Sin esto no compila d2server, y sin d2server no compila
// d2networking, ni d2gamescreen, ni d2app: el juego entero.
//
// QUE HACE Y QUE NO (esto importa y esta declarado): el archivo que sale es un gzip DE VERDAD -- lo abre
// cualquier descompresor, incluido el de Go -- pero NO COMPRIME: los datos van adentro tal cual, en
// bloques del tipo "guardado" del formato deflate. O sea que el tamano no baja, pero el formato es
// correcto y la informacion no se pierde.
//
// POR QUE ASI Y NO CON COMPRESION: comprimir de verdad pide DEFLATE (el algoritmo de compresion con la
// ventana de 32 KB), y eso la consola ya lo tiene RESUELTO DEL OTRO LADO: el zlib de KOS, que se usa para
// DESCOMPRIMIR (ver el Reader de este paquete). El lado que comprime no esta expuesto en el binding. Se
// podria escribir, pero es un tramo aparte; y para el juego de un jugador no hace falta, porque la red no
// se usa. Queda como limite declarado, no como olvido.

const (
	// Los niveles de compresion, con los mismos nombres y valores que Go.
	NoCompression      = 0
	BestSpeed          = 1
	BestCompression    = 9
	DefaultCompression = -1
	HuffmanOnly        = -2
)

// ErrCerrado: escribir despues de cerrar.
var ErrCerrado = errorString("gzip: el archivo ya se cerro")

// ErrNivelInvalido: un nivel de compresion que no existe.
var ErrNivelInvalido = errorString("gzip: nivel de compresion invalido")

// Writer: el que arma el archivo gzip.
type Writer struct {
	destino  io.Writer
	crc      hash.Hash32
	escritos uint32 // cuantos bytes entraron (va al final del archivo)
	nivel    int
	cerrado  bool
	cabecera bool // si ya se escribio la cabecera
}

// NewWriter: un escritor con la compresion de siempre.
func NewWriter(w io.Writer) *Writer {
	z, _ := NewWriterLevel(w, DefaultCompression)
	return z
}

// NewWriterLevel: un escritor con el nivel que se pida.
//
// El nivel SE ACEPTA Y SE GUARDA, pero aca no cambia nada: los bloques van guardados (ver arriba), asi que
// los nueve niveles dan el mismo archivo. Se guarda igual para que el que lo consulte reciba lo que pidio.
func NewWriterLevel(w io.Writer, nivel int) (*Writer, error) {
	switch nivel {
	case NoCompression, BestSpeed, BestCompression, DefaultCompression, HuffmanOnly:
	default:
		if nivel < 1 || nivel > 9 {
			return nil, ErrNivelInvalido
		}
	}

	z := &Writer{destino: w, crc: crc32.NewIEEE(), nivel: nivel}
	if err := z.escribirCabecera(); err != nil {
		return nil, err
	}
	return z, nil
}

// escribirCabecera: los 10 bytes del formato gzip.
//
// El detalle que hay que hacer bien: el ultimo byte dice QUE SISTEMA hizo el archivo, y 0xff quiere decir
// "no se sabe" (asi lo escribe Go). Poner un numero que diga "Linux" seria mentir sobre de donde salio.
func (z *Writer) escribirCabecera() error {
	if z.cabecera {
		return nil
	}
	z.cabecera = true
	// 1f 8b = la marca del gzip, 08 = deflate, 00 = sin banderas,
	// 00 00 00 00 = sin fecha, 00 = sin pista de compresion, ff = sistema desconocido
	_, err := z.destino.Write([]byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff})
	return err
}

// Write: mete datos al archivo.
func (z *Writer) Write(p []byte) (int, error) {
	if z.cerrado {
		return 0, ErrCerrado
	}
	if len(p) == 0 {
		return 0, nil
	}

	_, _ = z.crc.Write(p)
	z.escritos += uint32(len(p))

	if err := z.escribirBloques(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// escribirBloques: parte los datos en bloques del tipo "guardado".
//
// Un bloque guardado del deflate es: un byte que dice si es el ultimo y que el tipo es 0, el largo (2
// bytes, de atras para adelante), el complemento de ese largo (otros 2), y los datos. El complemento es la
// forma que tiene el formato de avisar si el bloque se corto.
func (z *Writer) escribirBloques(datos []byte) error {
	for len(datos) > 0 {
		n := len(datos)
		if n > 65535 {
			n = 65535
		}

		largo := uint16(n)
		cabecera := []byte{
			0x00, // no es el ultimo bloque, y el tipo es guardado
			byte(largo), byte(largo >> 8),
			byte(^largo), byte(^largo >> 8),
		}
		if _, err := z.destino.Write(cabecera); err != nil {
			return err
		}
		if _, err := z.destino.Write(datos[:n]); err != nil {
			return err
		}
		datos = datos[n:]
	}
	return nil
}

// Flush: el gzip de verdad cierra el bloque que tenga a medio armar. Aca los bloques se cierran solos en
// cada Write, asi que no hay nada que hacer.
func (z *Writer) Flush() error {
	if z.cerrado {
		return ErrCerrado
	}
	return nil
}

// Close: cierra el archivo: el bloque final vacio, la suma de control y el tamano.
func (z *Writer) Close() error {
	if z.cerrado {
		return nil
	}
	z.cerrado = true

	// el bloque final: ultimo (0x01), guardado, y sin datos
	if _, err := z.destino.Write([]byte{0x01, 0x00, 0x00, 0xff, 0xff}); err != nil {
		return err
	}

	// los ultimos 8 bytes: la suma de control y el tamano SIN comprimir (asi, de atras para adelante)
	suma := z.crc.Sum32()
	final := []byte{
		byte(suma), byte(suma >> 8), byte(suma >> 16), byte(suma >> 24),
		byte(z.escritos), byte(z.escritos >> 8), byte(z.escritos >> 16), byte(z.escritos >> 24),
	}
	if _, err := z.destino.Write(final); err != nil {
		return err
	}

	// si el que escribe sabe cerrar (un archivo), se le avisa
	if cerrador, ok := z.destino.(io.Closer); ok {
		return cerrador.Close()
	}
	return nil
}
