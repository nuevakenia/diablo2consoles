// Package utf8: lo minimo de codificacion UTF-8 que necesitan strings y fmt.
//
// El motor trabaja con rutas y nombres ASCII, pero los strings pueden traer texto del juego, asi
// que el conteo y el corte por caracter se hacen bien (no "un byte = un caracter"). Es la misma
// semantica de error de Go: byte invalido -> utf8.RuneError con tamaño 1.
package utf8

// Constantes de la codificacion.
const (
	RuneError = '\uFFFD'
	RuneSelf  = 0x80
	MaxRune   = '\U0010FFFF'
	UTFMax    = 4
)

// RuneLen devuelve cuantos bytes ocupa r (1 a 4), o -1 si no es valido.
func RuneLen(r rune) int {
	switch {
	case r < 0:
		return -1
	case r < 0x80:
		return 1
	case r < 0x800:
		return 2
	case r >= 0xD800 && r <= 0xDFFF:
		return -1
	case r < 0x10000:
		return 3
	case r <= MaxRune:
		return 4
	}
	return -1
}

// RuneCountInString cuenta caracteres, no bytes.
func RuneCountInString(s string) int {
	n := 0
	for i := 0; i < len(s); {
		if s[i] < RuneSelf {
			i++
		} else {
			_, size := DecodeRuneInString(s[i:])
			i += size
		}
		n++
	}
	return n
}

// DecodeRuneInString decodifica el primer caracter de s.
func DecodeRuneInString(s string) (rune, int) {
	if len(s) == 0 {
		return RuneError, 0
	}
	p0 := s[0]
	if p0 < RuneSelf {
		return rune(p0), 1
	}
	return decodeRune(s)
}

func decodeRune(s string) (rune, int) {
	p0 := s[0]
	n := len(s)

	if n < 2 {
		return RuneError, 1
	}
	p1 := s[1]
	if p1 < 0x80 || p1 > 0xBF {
		return RuneError, 1
	}

	// 2 bytes
	if p0 < 0xE0 {
		if p0 < 0xC2 {
			return RuneError, 1
		}
		r := rune(p0&0x1F)<<6 | rune(p1&0x3F)
		return r, 2
	}

	if n < 3 {
		return RuneError, 1
	}
	p2 := s[2]
	if p2 < 0x80 || p2 > 0xBF {
		return RuneError, 1
	}
	if p0 < 0xF0 {
		// 3 bytes
		if p0 == 0xE0 && p1 < 0xA0 {
			return RuneError, 1
		}
		if p0 == 0xED && p1 > 0x9F { // subrogados
			return RuneError, 1
		}
		r := rune(p0&0x0F)<<12 | rune(p1&0x3F)<<6 | rune(p2&0x3F)
		return r, 3
	}

	n--
	if n < 4 {
		return RuneError, 1
	}
	p3 := s[3]
	if p3 < 0x80 || p3 > 0xBF {
		return RuneError, 1
	}
	// 4 bytes
	if p0 > 0xF4 || (p0 == 0xF0 && p1 < 0x90) || (p0 == 0xF4 && p1 > 0x8F) {
		return RuneError, 1
	}
	r := rune(p0&0x07)<<18 | rune(p1&0x3F)<<12 | rune(p2&0x3F)<<6 | rune(p3&0x3F)
	return r, 4
}

// EncodeRune escribe r en p (que tiene que tener lugar) y devuelve cuantos bytes uso.
func EncodeRune(p []byte, r rune) int {
	switch i := RuneLen(r); {
	case i < 0:
		return EncodeRune(p, RuneError)
	case i == 1:
		p[0] = byte(r)
		return 1
	case i == 2:
		p[0] = 0xC0 | byte(r>>6)
		p[1] = 0x80 | byte(r)&0x3F
		return 2
	case i == 3:
		p[0] = 0xE0 | byte(r>>12)
		p[1] = 0x80 | byte(r>>6)&0x3F
		p[2] = 0x80 | byte(r)&0x3F
		return 3
	default:
		p[0] = 0xF0 | byte(r>>18)
		p[1] = 0x80 | byte(r>>12)&0x3F
		p[2] = 0x80 | byte(r>>6)&0x3F
		p[3] = 0x80 | byte(r)&0x3F
		return 4
	}
}

// AppendRune agrega r a p.
func AppendRune(p []byte, r rune) []byte {
	var buf [UTFMax]byte
	n := EncodeRune(buf[:], r)
	return append(p, buf[:n]...)
}

const (
	// surogateMin/Max: rangos que no son caracteres validos.
	surrogateMin = 0xD800
	surrogateMax = 0xDFFF
)

// Valid informa si s es UTF-8 valido.
func Valid(s string) bool {
	for i := 0; i < len(s); {
		r, size := DecodeRuneInString(s[i:])
		if r == RuneError && size == 1 {
			return false
		}
		i += size
	}
	return true
}

// RuneCount cuenta caracteres en p.
func RuneCount(p []byte) int { return RuneCountInString(string(p)) }
