// Paquete hex: la conversion a hexadecimal, lo que el motor usa (EncodeToString).
//
// Por que existe: en sh-elf no hay biblioteca estandar de Go; el motor lo usa al mostrar claves y
// al volcar datos (1 archivo directo, mas adentro).
package hex

const digitos = "0123456789abcdef"

// EncodeToString devuelve el hexadecimal de src, en minusculas (como Go).
func EncodeToString(src []byte) string {
	dst := make([]byte, len(src)*2)
	for i, b := range src {
		dst[i*2] = digitos[b>>4]
		dst[i*2+1] = digitos[b&0x0f]
	}
	return string(dst)
}

// Encode copia el hexadecimal de src en dst (necesita len(dst) >= 2*len(src)) y devuelve cuanto
// escribio.
func Encode(dst, src []byte) int {
	for i, b := range src {
		if i*2+1 >= len(dst) {
			return i * 2
		}
		dst[i*2] = digitos[b>>4]
		dst[i*2+1] = digitos[b&0x0f]
	}
	return len(src) * 2
}

// DecodedLen devuelve cuantos bytes ocupan n digitos hexadecimales.
func DecodedLen(n int) int { return n / 2 }

// valor devuelve el valor del digito, y -1 si no es hexadecimal.
func valor(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// Decode decodifica src (hexadecimal) en dst y devuelve cuantos bytes escribio. Con un digito
// invalido corta ahi, como Go.
func Decode(dst, src []byte) (int, error) {
	i, j := 0, 0
	for ; j+1 < len(src); j += 2 {
		a, b := valor(src[j]), valor(src[j+1])
		if a < 0 || b < 0 {
			return i, ErrorHex
		}
		if i >= len(dst) {
			return i, ErrorHex
		}
		dst[i] = byte(a<<4 | b)
		i++
	}
	if j < len(src) {
		return i, ErrorHex // quedo un digito suelto
	}
	return i, nil
}

// ErrorHex es el error de un digito invalido (en Go es hex.InvalidByteError; aca alcanza con uno).
type errorHex string

func (e errorHex) Error() string { return string(e) }

// ErrorHex se exporta para que el llamador lo pueda comparar por identidad.
var ErrorHex error = errorHex("encoding/hex: digito invalido")
