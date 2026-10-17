//go:build dreamcast

// DC-002.20: EL GZIP DE LA CONSOLA CONTRA EL ASSET EMBEBIDO DEL MOTOR, medido en Flycast.
//
// El arranque del motor ya pasa la seleccion de los archivos del juego (DC-002.19) y muere en:
//
//	panic: assets: bmp.Decode failed: gzip: invalid header
//
// de `d2common/d2util/assets.CreateTextImage`, sobre el `CompressedDebugText` EMBEBIDO en el binario (o sea
// que el defecto NO depende de los MPQ, que en la corrida estaban vacios).
//
// QUE MIDE ESTE POC, y por que cada numero:
//
//  1. LOS BYTES EMBEBIDOS tal como los deja el compilador para SH-4: tamano y CRC32. El host da
//     2.209 bytes y crc32 aa973211 (console/dc/gen-asset-info). Si no coinciden, el problema estaria en el
//     binario y no en el gzip.
//
//  2. EL CAMINO VIEJO, el que hoy falla: `d2c_zlib_inflate` de una sola vez con el destino que calcula
//     `d2c_zlib_inflate_bound` (4 x comprimido + 64 KB). Se imprime el tamano del destino, lo que devuelve
//     la zlib y lo que el `gzip` de la consola traduce a error. El host dice que el asset descomprimido son
//     98.358 bytes, asi que si la cota es 74.372 el `-1` es de TAMANO y el mensaje `invalid header` miente.
//
//  3. EL CAMINO NUEVO, en streaming y en pedazos CHICOS (512 bytes, a proposito: con un destino grande el
//     streaming tambien "andaria" sin probar que entrega por partes): cantidad de bytes, cantidad de
//     lecturas y CRC32. Tiene que dar 98.358 bytes y crc32 5dce3e98, los mismos del host.
//
//  4. EL CAMINO DEL MOTOR, tal cual: `gzip.NewReader` + `bmp.Decode(s)` sobre el asset embebido, que es la
//     linea que paniquea. Se imprime el tamano del BMP decodificado (256 x 128 x 24 bits en el host).
package main

import (
	"compress/gzip"
	"hash/crc32"
	"io"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util/assets"
	"golang.org/x/image/bmp"
)

//extern d2c_serial_init
func d2cSerialInit()

// el puente de una sola vez (el que usa el camino viejo del gzip), declarado aca para medirlo directo
//
//extern d2c_zlib_inflate
func d2cZlibInflate(src []byte, srcLen int32, dst []byte, dstCap int32) int32

//extern d2c_zlib_inflate_bound
func d2cZlibInflateBound(srcLen int32) int32

func main() {
	d2cSerialInit()

	crudo := assets.CompressedDebugText

	println("GZIP-ASSET embebido bytes=", len(crudo), "crc32=", crc32.ChecksumIEEE(crudo))

	// ---------------------------------------------------------------- 2. el camino viejo (el defecto)
	cota := d2cZlibInflateBound(int32(len(crudo)))
	destino := make([]byte, cota)
	n := d2cZlibInflate(crudo, int32(len(crudo)), destino, int32(cota))

	println("GZIP-ASSET viejo cota=", cota, "devuelto=", n, "entra_el_real=", 98358 <= int(cota))

	if n >= 0 {
		println("GZIP-ASSET viejo bytes=", n, "crc32=", crc32.ChecksumIEEE(destino[:n]))
	} else {
		println("GZIP-ASSET viejo la zlib devolvio error: el gzip de la consola lo traduce a invalid header")
	}

	// ---------------------------------------------------------------- 3. el camino nuevo, por pedazos
	lector, err := gzip.NewReader(bytesDe(crudo))
	if err != nil {
		println("GZIP-ASSET nuevo NewReader error:", err.Error())

		return
	}

	var salida []byte

	chico := make([]byte, 512)
	lecturas := 0

	for {
		m, errL := lector.Read(chico)
		if m > 0 {
			salida = append(salida, chico[:m]...)
			lecturas++
		}

		if errL == io.EOF {
			break
		}

		if errL != nil {
			println("GZIP-ASSET nuevo Read error:", errL.Error())

			return
		}
	}

	_ = lector.Close()

	println("GZIP-ASSET nuevo bytes=", len(salida), "crc32=", crc32.ChecksumIEEE(salida), "lecturas_de_512=", lecturas)

	if len(salida) >= 16 {
		println("GZIP-ASSET nuevo cabecera=", salida[0], salida[1], salida[2], salida[3],
			" (host: 66 77 54 128, o sea BM)")
	}

	// ---------------------------------------------------------------- 4. el camino del motor, tal cual
	lector2, err2 := gzip.NewReader(bytesDe(crudo))
	if err2 != nil {
		println("GZIP-ASSET motor NewReader error:", err2.Error())

		return
	}

	imagen, err3 := bmp.Decode(lector2)
	if err3 != nil {
		println("GZIP-ASSET motor bmp.Decode error:", err3.Error())
	} else {
		caja := imagen.Bounds()
		println("GZIP-ASSET motor bmp.Decode OK ancho=", caja.Dx(), "alto=", caja.Dy())
	}

	_ = lector2.Close()
}

// bytesDe: el `bytes.NewReader` de Go devuelve un *bytes.Reader, que este paquete no tiene. Lo que el
// `gzip.NewReader` de la consola necesita es un `io.Reader` y su implementacion hace `io.ReadAll`, asi que
// con un lector propio de un pedazo alcanza (y ademas deja medir que no pida nada raro).
func bytesDe(datos []byte) io.Reader {
	return &lectorDeBytes{datos: datos}
}

type lectorDeBytes struct {
	datos []byte
	pos   int
}

func (l *lectorDeBytes) Read(p []byte) (int, error) {
	if l.pos >= len(l.datos) {
		return 0, io.EOF
	}

	n := copy(p, l.datos[l.pos:])
	l.pos += n

	return n, nil
}
