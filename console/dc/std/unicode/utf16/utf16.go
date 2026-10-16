// Package utf16: la conversion de texto a pares de 16 bits, para la consola. El motor la usa al leer
// los textos del juego, que vienen en ese formato.
package utf16

// Encode arma los pares de 16 bits de un texto (es lo mismo que hace go: los caracteres de mas de
// 0xFFFF salen como dos unidades).
func Encode(s []rune) []uint16 {
	sa := make([]uint16, 0, len(s))
	for _, r := range s {
		if r < 0x10000 {
			sa = append(sa, uint16(r))
			continue
		}
		r -= 0x10000
		sa = append(sa, uint16(0xd800+(r>>10)), uint16(0xdc00+(r&0x3ff)))
	}
	return sa
}

// Decode deshace Encode.
func Decode(s []uint16) []rune {
	sa := make([]rune, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch r := s[i]; {
		case r < 0xd800 || 0xdfff < r:
			sa = append(sa, rune(r))
		case 0xd800 <= r && r < 0xdc00 && i+1 < len(s) && 0xdc00 <= s[i+1] && s[i+1] < 0xe000:
			sa = append(sa, rune((r-0xd800)<<10|(s[i+1]-0xdc00))+0x10000)
			i++
		default:
			sa = append(sa, 0xfffd)
		}
	}
	return sa
}

// DecodeRune devuelve el caracter y cuantas unidades ocupo.
func DecodeRune(s []uint16) (rune, int) {
	if len(s) == 0 {
		return 0xfffd, 0
	}
	switch r := s[0]; {
	case r < 0xd800 || 0xdfff < r:
		return rune(r), 1
	case 0xd800 <= r && r < 0xdc00 && len(s) >= 2 && 0xdc00 <= s[1] && s[1] < 0xe000:
		return rune((r-0xd800)<<10|(s[1]-0xdc00)) + 0x10000, 2
	default:
		return 0xfffd, 1
	}
}
