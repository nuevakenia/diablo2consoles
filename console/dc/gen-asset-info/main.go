// DC-002.20: EL ASSET EMBEBIDO DEL MOTOR, medido en el HOST (la referencia de verdad).
//
// El motor, en `d2common/d2util/assets/assets.go`, hace:
//
//	gzip.NewReader(bytes.NewReader(CompressedDebugText)) -> bmp.Decode(s)
//
// y en la consola ese camino muere con `panic: assets: bmp.Decode failed: gzip: invalid header`. Para
// saber que tiene que salir BIEN antes de tocar el `gzip` de la consola, esto mide en el host, con el
// `compress/gzip` y el `bmp` DE VERDAD:
//
//   - los bytes EMBEBIDOS: tamano, sha256 y la cabecera (los dos primeros tienen que ser 1f 8b);
//   - lo que sale al descomprimir: tamano, sha256 y los primeros bytes (el "BM" del BMP);
//   - el TAMANO del BMP (ancho, alto, bits por pixel) de la cabecera, sin decodificar los pixeles;
//   - LA CUENTA QUE IMPORTA PARA LA CONSOLA: el `gzip` de la consola pide el destino con
//     `d2c_zlib_inflate_bound(len(comprimido))` = 4*len + 65536, asi que aca se compara ese numero contra
//     el tamano REAL descomprimido. Si el real no entra en el calculado, el `-1` del puente sale por
//     TAMANO y no por cabecera, y el mensaje `gzip: invalid header` estaria mintiendo sobre la causa.
//
// Uso:  go run ./console/dc/gen-asset-info
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"os"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util/assets"
)

// minInt: el go.mod del motor declara una version de Go sin el `min` de la biblioteca.
func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func main() {
	crudo := assets.CompressedDebugText

	sumaCrudo := sha256.Sum256(crudo)

	fmt.Printf("ASSET-INFO embebido bytes=%d sha256=%s\n", len(crudo), hex.EncodeToString(sumaCrudo[:]))
	fmt.Printf("ASSET-INFO embebido cabecera=% x (los dos primeros tienen que ser 1f 8b)\n", crudo[:minInt(16, len(crudo))])

	lector, err := gzip.NewReader(bytes.NewReader(crudo))
	if err != nil {
		fmt.Fprintln(os.Stderr, "ASSET-INFO el gzip del host NO abre el asset:", err)
		os.Exit(1)
	}

	descomprimido, err := io.ReadAll(lector)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ASSET-INFO el gzip del host falla al leer:", err)
		os.Exit(1)
	}

	sumaDes := sha256.Sum256(descomprimido)

	fmt.Printf("ASSET-INFO descomprimido bytes=%d sha256=%s\n", len(descomprimido),
		hex.EncodeToString(sumaDes[:]))
	fmt.Printf("ASSET-INFO descomprimido cabecera=% x (BMP: los dos primeros son 42 4d, o sea BM)\n",
		descomprimido[:minInt(16, len(descomprimido))])

	// La huella que compara la consola (el corpus del POC gzip-asset): CRC32 IEEE del resultado, que es lo
	// que el `d2std/hash/crc32` de la consola puede calcular con el mismo polinomio.
	fmt.Printf("ASSET-INFO descomprimido crc32=%08x\n", crc32.ChecksumIEEE(descomprimido))
	fmt.Printf("ASSET-INFO embebido crc32=%08x\n", crc32.ChecksumIEEE(crudo))

	// La cabecera del BMP: ancho y alto en los offsets 18 y 22, y los bits por pixel en el 28.
	if len(descomprimido) >= 30 {
		ancho := int32(binary.LittleEndian.Uint32(descomprimido[18:22]))
		alto := int32(binary.LittleEndian.Uint32(descomprimido[22:26]))
		bpp := binary.LittleEndian.Uint16(descomprimido[28:30])
		fmt.Printf("ASSET-INFO bmp ancho=%d alto=%d bits_por_pixel=%d\n", ancho, alto, bpp)
	}

	// EL NUMERO QUE DECIDE SI EL `-1` DEL PUENTE ES DE TAMANO O DE CABECERA.
	cota := len(crudo)*4 + 65536

	fmt.Printf("ASSET-INFO cota_de_la_consola bytes=%d (4*%d + 65536) real=%d entra=%v sobra=%d\n",
		cota, len(crudo), len(descomprimido), len(descomprimido) <= cota, cota-len(descomprimido))

	// y el otro numero: cuanto ocupa el destino en un solo bloque contiguo, que es lo que el `gzip` de la
	// consola le pide al monton de una vez (`make`) sobre una arena de 2,5 MB.
	fmt.Printf("ASSET-INFO el_destino_de_una_sola_vez bytes=%d\n", cota)
}
