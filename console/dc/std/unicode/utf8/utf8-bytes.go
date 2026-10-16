// utf8: las dos funciones que faltaban para que el paquete bytes y el paquete strings de la consola
// puedan recorrer []byte y strings por runas (DC-002.5): DecodeRune sobre bytes (sin armar un string,
// que en el motor significaria copiar) y DecodeLastRuneInString.
package utf8

// DecodeRune decodifica el primer caracter de p. Es DecodeRuneInString sobre bytes.
func DecodeRune(p []byte) (rune, int) {
	if len(p) == 0 {
		return RuneError, 0
	}
	p0 := p[0]
	if p0 < RuneSelf {
		return rune(p0), 1
	}
	return decodeRuneBytes(p)
}

func decodeRuneBytes(p []byte) (rune, int) {
	p0 := p[0]
	n := len(p)

	if n < 2 {
		return RuneError, 1
	}
	p1 := p[1]
	if p1 < 0x80 || p1 > 0xBF {
		return RuneError, 1
	}
	// 2 bytes
	if p0 < 0xE0 {
		if p0 < 0xC2 {
			return RuneError, 1
		}
		return rune(p0&0x1F)<<6 | rune(p1&0x3F), 2
	}
	if n < 3 {
		return RuneError, 1
	}
	p2 := p[2]
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
		return rune(p0&0x0F)<<12 | rune(p1&0x3F)<<6 | rune(p2&0x3F), 3
	}
	if n < 4 {
		return RuneError, 1
	}
	p3 := p[3]
	if p3 < 0x80 || p3 > 0xBF {
		return RuneError, 1
	}
	// 4 bytes
	if p0 > 0xF4 || (p0 == 0xF0 && p1 < 0x90) || (p0 == 0xF4 && p1 > 0x8F) {
		return RuneError, 1
	}
	return rune(p0&0x07)<<18 | rune(p1&0x3F)<<12 | rune(p2&0x3F)<<6 | rune(p3&0x3F), 4
}

// DecodeLastRuneInString decodifica el ultimo caracter de s (la misma regla que Go: si detrás del
// ultimo byte no esta el arranque de una secuencia valida, devuelve RuneError y 1).
func DecodeLastRuneInString(s string) (rune, int) {
	end := len(s)
	if end == 0 {
		return RuneError, 0
	}
	start := end - 1
	for start > 0 && s[start]&0xC0 == 0x80 {
		start--
	}
	r, size := DecodeRuneInString(s[start:])
	if size != end-start {
		return RuneError, 1
	}
	return r, size
}
