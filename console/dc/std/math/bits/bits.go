// Paquete bits: manejo de bits, lo que el motor usa (OnesCount8, RotateLeft32).
//
// Por que existe: en sh-elf no hay biblioteca estandar de Go, y el motor cuenta bits y rota
// enteros en varios lugares (2 archivos lo importan directo; adentro del motor se usa mas).
//
// Las implementaciones son las clasicas de Hacker's Delight, que dan el mismo resultado que las de
// Go para todos los valores (el corpus lo prueba valor por valor con los 8 bits y los 32 de la
// rotacion). Go en el host usa instrucciones del procesador; aca se hace con cuentas.
package bits

// OnesCount8 cuenta los bits en uno de x.
func OnesCount8(x uint8) int {
	x = x - ((x >> 1) & 0x55)
	x = (x & 0x33) + ((x >> 2) & 0x33)
	return int((x + (x >> 4)) & 0x0f)
}

// OnesCount16 cuenta los bits en uno de x.
func OnesCount16(x uint16) int {
	return OnesCount8(uint8(x)) + OnesCount8(uint8(x>>8))
}

// OnesCount32 cuenta los bits en uno de x.
func OnesCount32(x uint32) int {
	x = x - ((x >> 1) & 0x55555555)
	x = (x & 0x33333333) + ((x >> 2) & 0x33333333)
	x = (x + (x >> 4)) & 0x0f0f0f0f
	return int((x * 0x01010101) >> 24)
}

// OnesCount64 cuenta los bits en uno de x.
func OnesCount64(x uint64) int {
	return OnesCount32(uint32(x)) + OnesCount32(uint32(x>>32))
}

// RotateLeft32 rota x a la izquierda k bits (k puede ser negativo o mayor que 32).
func RotateLeft32(x uint32, k int) uint32 {
	if k < 0 {
		return RotateRight32(x, -k)
	}
	k &= 31
	return x<<uint(k) | x>>uint(32-k)
}

// RotateRight32 rota x a la derecha k bits.
func RotateRight32(x uint32, k int) uint32 {
	if k < 0 {
		return RotateLeft32(x, -k)
	}
	k &= 31
	return x>>uint(k) | x<<uint(32-k)
}

// LeadingZeros32 cuenta los ceros a la izquierda de x.
func LeadingZeros32(x uint32) int {
	if x == 0 {
		return 32
	}
	n := 0
	for x&0x80000000 == 0 {
		x <<= 1
		n++
	}
	return n
}

// TrailingZeros32 cuenta los ceros a la derecha de x.
func TrailingZeros32(x uint32) int {
	if x == 0 {
		return 32
	}
	n := 0
	for x&1 == 0 {
		x >>= 1
		n++
	}
	return n
}

// Len32 devuelve el numero de bits que necesita x (Len32(0) = 0).
func Len32(x uint32) int { return 32 - LeadingZeros32(x) }
