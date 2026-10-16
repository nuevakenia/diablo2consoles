// strings: lo que faltaba del muro despues de la primera tanda (Compare, IndexAny/LastIndexAny,
// Title y Map). Va aparte para no tocar lo que ya estaba medido en DC-002.2/Dc-002.4.
package strings

import "unicode/utf8"

// Compare es -1, 0 o 1 segun el orden de a respecto de b (la misma regla que Go: comparacion de
// bytes, no de runas).
func Compare(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}

// IndexAny devuelve la posicion de la primera runa de chars que aparece en s, o -1.
func IndexAny(s, chars string) int {
	if len(chars) == 0 {
		return -1
	}
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		for _, c := range chars {
			if r == c {
				return i
			}
		}
		i += size
	}
	return -1
}

// LastIndexAny devuelve la posicion de la ULTIMA runa de chars que aparece en s, o -1.
func LastIndexAny(s, chars string) int {
	if len(chars) == 0 {
		return -1
	}
	for i := len(s); i > 0; {
		r, size := utf8.DecodeLastRuneInString(s[:i])
		for _, c := range chars {
			if r == c {
				return i - size
			}
		}
		i -= size
	}
	return -1
}

// isSeparatorDeTitulo replica la regla de Go: los alfanumericos ASCII y el guion bajo NO separan
// palabra; el resto de los ASCII, si. Para los no-ASCII (multibyte) se toma como letra y no separa,
// que es el caso comun; el limite queda dicho aca.
func isSeparatorDeTitulo(r rune) bool {
	if r <= 0x7F {
		if r >= '0' && r <= '9' {
			return false
		}
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= 'A' && r <= 'Z' {
			return false
		}
		return r != '_'
	}
	return false
}

// Title pone en mayuscula la primera letra de cada palabra (como strings.Title de Go, que no baja el
// resto: "hOLA" es "HOLA").
func Title(s string) string {
	// Se arma por trozos: mayuscula, separador, resto.
	var b []byte
	previoEsSeparador := true
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if previoEsSeparador && r < utf8.RuneSelf {
			// ASCII: la paso a mayuscula
			c := byte(r)
			if c >= 'a' && c <= 'z' {
				c -= 'a' - 'A'
			}
			b = append(b, c)
		} else {
			b = append(b, s[i:i+size]...)
		}
		previoEsSeparador = isSeparatorDeTitulo(r)
		i += size
	}
	return string(b)
}

// Map aplica f a cada runa y arma el string resultado (si f devuelve un valor negativo, la runa se
// saca, como en Go).
func Map(f func(rune) rune, s string) string {
	var b []byte
	for _, r := range s {
		nr := f(r)
		if nr < 0 {
			continue
		}
		b = utf8.AppendRune(b, nr)
	}
	return string(b)
}
