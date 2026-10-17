// Package sha256: el SHA-256 de la biblioteca estandar, para la consola.
//
// Por que esta aca y no en C: el motor lo usa en UN lugar (d2hero/d2s_integrity.go, la integridad de los
// archivos de guardado) y con UN simbolo (sha256.Sum256). SHA-256 son 64 rondas de cuentas simples sobre
// 32 bits: no hay nada especifico de la plataforma, y como es un algoritmo CONOCIDO con una respuesta
// conocida, se puede probar contra el de Go bit a bit (esta la prueba en el difftest). Eso lo hace un
// candidato perfecto para escribir en Go y no para puentear.
//
// Lo que NO esta: la version incremental (New/Write/Sum) ni las variantes con otra semilla. Si el motor
// las pide, se ven al medir.
package sha256

import "math/bits"

// Las constantes de la ronda: los primeros 32 bits de las partes fraccionarias de las raices cubicas de
// los primeros 64 numeros primos. Son las del estandar, no se inventan.
var k = [64]uint32{
	0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
	0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
	0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
	0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
	0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
	0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
	0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
	0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
}

// las constantes iniciales: las partes fraccionarias de las raices cuadradas de los primeros 8 primos.
const (
	h0 = 0x6a09e667
	h1 = 0xbb67ae85
	h2 = 0x3c6ef372
	h3 = 0xa54ff53a
	h4 = 0x510e527f
	h5 = 0x9b05688c
	h6 = 0x1f83d9ab
	h7 = 0x5be0cd19
)

// Size: el tamano del resultado, en bytes.
const Size = 32

// BlockSize: el tamano del bloque.
const BlockSize = 64

// Sum256 devuelve el SHA-256 de data (es lo unico que usa el motor: d2hero lo llama para verificar los
// archivos de guardado).
func Sum256(data []byte) [32]byte {
	// 1. el mensaje se rellena: un 1, ceros, y el largo original en bits (8 bytes, big-endian)
	largo := len(data)
	relleno := make([]byte, llena(largo))
	copy(relleno, data)
	relleno[largo] = 0x80
	bitsTotales := uint64(largo) * 8
	for i := 0; i < 8; i++ {
		relleno[len(relleno)-1-i] = byte(bitsTotales >> (8 * uint(i)))
	}

	// 2. el estado
	h := [8]uint32{h0, h1, h2, h3, h4, h5, h6, h7}

	// 3. bloque por bloque
	var w [64]uint32
	for inicio := 0; inicio < len(relleno); inicio += BlockSize {
		bloque := relleno[inicio : inicio+BlockSize]

		// las primeras 16 palabras, big-endian
		for i := 0; i < 16; i++ {
			w[i] = uint32(bloque[i*4])<<24 | uint32(bloque[i*4+1])<<16 |
				uint32(bloque[i*4+2])<<8 | uint32(bloque[i*4+3])
		}
		// y las 48 que salen de las anteriores
		for i := 16; i < 64; i++ {
			s0 := bits.RotateLeft32(w[i-15], -7) ^ bits.RotateLeft32(w[i-15], -18) ^ (w[i-15] >> 3)
			s1 := bits.RotateLeft32(w[i-2], -17) ^ bits.RotateLeft32(w[i-2], -19) ^ (w[i-2] >> 10)
			w[i] = w[i-16] + s0 + w[i-7] + s1
		}

		a, b, c, d := h[0], h[1], h[2], h[3]
		e, f, g, hh := h[4], h[5], h[6], h[7]

		for i := 0; i < 64; i++ {
			S1 := bits.RotateLeft32(e, -6) ^ bits.RotateLeft32(e, -11) ^ bits.RotateLeft32(e, -25)
			ch := (e & f) ^ (^e & g)
			temp1 := hh + S1 + ch + k[i] + w[i]
			S0 := bits.RotateLeft32(a, -2) ^ bits.RotateLeft32(a, -13) ^ bits.RotateLeft32(a, -22)
			maj := (a & b) ^ (a & c) ^ (b & c)
			temp2 := S0 + maj

			hh = g
			g = f
			f = e
			e = d + temp1
			d = c
			c = b
			b = a
			a = temp1 + temp2
		}

		h[0] += a
		h[1] += b
		h[2] += c
		h[3] += d
		h[4] += e
		h[5] += f
		h[6] += g
		h[7] += hh
	}

	// 4. el resultado, big-endian
	var salida [32]byte
	for i := 0; i < 8; i++ {
		salida[i*4] = byte(h[i] >> 24)
		salida[i*4+1] = byte(h[i] >> 16)
		salida[i*4+2] = byte(h[i] >> 8)
		salida[i*4+3] = byte(h[i])
	}
	return salida
}

// llena: cuanto ocupa el mensaje con el relleno (multiplo de 64, con lugar para el 1 y los 8 bytes del
// largo).
func llena(largo int) int {
	n := largo + 1 + 8
	if resto := n % BlockSize; resto != 0 {
		n += BlockSize - resto
	}
	return n
}
