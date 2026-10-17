// Prueba del sha256 de la consola contra el de la biblioteca estandar de verdad.
//
// SHA-256 es un algoritmo con respuesta conocida: si esto coincide bit a bit con el de Go, esta bien. No
// hace falta comparar "a ojo" ni confiar en que compila.
package difftest

import (
	gosha "crypto/sha256"
	"math/rand"
	"testing"

	"d2std/crypto/sha256"
)

func TestSha256ContraGo(t *testing.T) {
	casos := [][]byte{
		{},
		[]byte("a"),
		[]byte("abc"),
		[]byte("El veloz murcielago hindu comia feliz cardillo y kiwi."),
		// justo en los bordes del bloque (64) y del largo (7 bytes = un bloque, 8 = dos)
		make([]byte, 55),
		make([]byte, 56),
		make([]byte, 63),
		make([]byte, 64),
		make([]byte, 65),
		make([]byte, 1000),
		{0x00, 0xff, 0x80, 0x7f, 0x01},
	}

	// y datos al azar, para cubrir largos que no se me ocurren
	azar := rand.New(rand.NewSource(4321)) //nolint:gosec // no es criptografia, es una prueba
	for i := 0; i < 20; i++ {
		n := azar.Intn(300)
		datos := make([]byte, n)
		for j := range datos {
			datos[j] = byte(azar.Intn(256))
		}
		casos = append(casos, datos)
	}

	for i, datos := range casos {
		nuestro := sha256.Sum256(datos)
		verdadero := gosha.Sum256(datos)

		if nuestro != verdadero {
			t.Errorf("caso %d (%d bytes): nuestro=%x go=%x", i, len(datos), nuestro, verdadero)
		}
	}
}

// TestSha256Largos: el largo del relleno tiene que dar multiplo de 64 siempre.
func TestSha256Largos(t *testing.T) {
	for n := 0; n < 200; n++ {
		datos := make([]byte, n)
		nuestro := sha256.Sum256(datos)
		verdadero := gosha.Sum256(datos)
		if nuestro != verdadero {
			t.Fatalf("largo %d: nuestro=%x go=%x", n, nuestro, verdadero)
		}
	}
}
