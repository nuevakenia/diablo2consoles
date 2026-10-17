// bytes: las funciones de busqueda y comparacion que el motor usa (24 archivos), sobre el Buffer y
// el Reader que ya estaban.
//
// Van aparte del Buffer para que se vea de un vistazo que es lo que hay: comparar, buscar, reemplazar
// y partir. Todas trabajan con []byte y no convierten a string (eso seria una copia por llamada,
// justo lo que un motor de juego no quiere hacer por frame).
package bytes

import "d2std/unicode/utf8"

// isSpaceByte son los blancos que usa bytes.TrimSpace, con los codigos escritos a mano (sin escapes
// de texto para que no dependan de como se copie el archivo).
func isSpaceByte(c byte) bool {
	if c == ' ' {
		return true
	}
	// 9 es tabulador, 10 salto de linea, 11 tab vertical, 12 avance de pagina, 13 retorno
	return c == 9 || c == 10 || c == 11 || c == 12 || c == 13
}

// Equal dice si a y b tienen los mismos bytes.
func Equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Compare es -1, 0 o 1 segun el orden de a respecto de b (como bytes.Compare de Go).
func Compare(a, b []byte) int {
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

// IndexByte devuelve la posicion de c en b, o -1.
func IndexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}

// Index devuelve la posicion del primer sep en b, o -1.
func Index(b, sep []byte) int {
	n := len(sep)
	switch {
	case n == 0:
		return 0
	case n == 1:
		return IndexByte(b, sep[0])
	case n > len(b):
		return -1
	}
	for i := 0; i+n <= len(b); i++ {
		if Equal(b[i:i+n], sep) {
			return i
		}
	}
	return -1
}

// LastIndex devuelve la posicion del ultimo sep en b, o -1.
func LastIndex(b, sep []byte) int {
	n := len(sep)
	if n == 0 {
		return len(b)
	}
	if n > len(b) {
		return -1
	}
	for i := len(b) - n; i >= 0; i-- {
		if Equal(b[i:i+n], sep) {
			return i
		}
	}
	return -1
}

// Contains dice si b tiene a sep adentro.
func Contains(b, sep []byte) bool { return Index(b, sep) >= 0 }

// HasPrefix dice si b empieza con prefix.
func HasPrefix(b, prefix []byte) bool {
	return len(b) >= len(prefix) && Equal(b[:len(prefix)], prefix)
}

// HasSuffix dice si b termina con suffix.
func HasSuffix(b, suffix []byte) bool {
	return len(b) >= len(suffix) && Equal(b[len(b)-len(suffix):], suffix)
}

// IndexAny devuelve la posicion de la primera runa de chars que aparece en b, o -1.
func IndexAny(b []byte, chars string) int {
	if len(chars) == 0 {
		return -1
	}
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		for _, c := range []rune(chars) {
			if r == c {
				return i
			}
		}
		i += size
	}
	return -1
}

// ReplaceAll reemplaza todas las apariciones de old por new (si old esta vacio, mete new entre cada
// byte, como Go).
func ReplaceAll(s, old, new []byte) []byte {
	if len(old) == 0 {
		// caso raro de Go: entre cada byte
		res := make([]byte, 0, len(s)+len(new)*(len(s)+1))
		for i := 0; i < len(s); i++ {
			res = append(res, new...)
			res = append(res, s[i])
		}
		res = append(res, new...)
		return res
	}
	// cuento primero, para pedir la memoria una sola vez
	n := 0
	for i := 0; i+len(old) <= len(s); {
		if Equal(s[i:i+len(old)], old) {
			n++
			i += len(old)
			continue
		}
		i++
	}
	if n == 0 {
		// devuelvo una copia (Go devuelve el mismo slice si no hubo cambios, pero una copia es
		// mas segura: el llamador podria modificar el original esperando independencia)
		res := make([]byte, len(s))
		copy(res, s)
		return res
	}
	res := make([]byte, 0, len(s)+n*(len(new)-len(old)))
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && Equal(s[i:i+len(old)], old) {
			res = append(res, new...)
			i += len(old)
			continue
		}
		res = append(res, s[i])
		i++
	}
	return res
}

// TrimSpace saca los blancos de los dos lados.
func TrimSpace(s []byte) []byte {
	i, j := 0, len(s)
	for i < j && isSpaceByte(s[i]) {
		i++
	}
	for j > i && isSpaceByte(s[j-1]) {
		j--
	}
	return s[i:j]
}

// TrimCutset saca de los dos lados los bytes que estan en cutset.
func TrimCutset(s []byte, cutset string) []byte {
	i, j := 0, len(s)
	for i < j && indexCutset(s[i], cutset) {
		i++
	}
	for j > i && indexCutset(s[j-1], cutset) {
		j--
	}
	return s[i:j]
}

func indexCutset(c byte, cutset string) bool {
	for i := 0; i < len(cutset); i++ {
		if cutset[i] == c {
			return true
		}
	}
	return false
}

// Split parte s por sep (con sep vacio, parte en cada runa, como Go).
func Split(s, sep []byte) [][]byte {
	if len(sep) == 0 {
		return explode(s)
	}
	var res [][]byte
	for {
		i := Index(s, sep)
		if i < 0 {
			break
		}
		res = append(res, s[:i])
		s = s[i+len(sep):]
	}
	res = append(res, s)
	return res
}

func explode(s []byte) [][]byte {
	res := make([][]byte, 0, len(s))
	for i := 0; i < len(s); {
		_, size := utf8.DecodeRune(s[i:])
		res = append(res, s[i:i+size])
		i += size
	}
	return res
}

// Join une los trozos con sep.
func Join(s [][]byte, sep []byte) []byte {
	if len(s) == 0 {
		return []byte{}
	}
	n := len(sep) * (len(s) - 1)
	for _, t := range s {
		n += len(t)
	}
	res := make([]byte, 0, n)
	res = append(res, s[0]...)
	for _, t := range s[1:] {
		res = append(res, sep...)
		res = append(res, t...)
	}
	return res
}

// ToUpper y ToLower, solo ASCII (el motor las usa para nombres de archivo del MPQ, que son ASCII; el
// limite queda dicho aca).
func ToUpper(s []byte) []byte {
	res := make([]byte, len(s))
	for i, c := range s {
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		res[i] = c
	}
	return res
}

// ToLower, solo ASCII (ver ToUpper).
func ToLower(s []byte) []byte {
	res := make([]byte, len(s))
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		res[i] = c
	}
	return res
}
