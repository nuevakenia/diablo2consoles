// Package unicode: las funciones de clasificacion de runas, en version ASCII para la consola.
//
// Límite declarado, y es grande: acá las letras y los numeros son SOLO ASCII. El juego trae textos en
// latin1 (los .tbl) y en su mayoria son ASCII; las vocales acentuadas NO se clasifican ni se pasan a
// mayuscula. Cuando haga falta de verdad, esto se reemplaza por las tablas completas de Go (son
// ~500 KB de datos y en la consola eso pesa), pero entonces hay que medirlo antes.
//
// Lo que si es exacto: la firma es la de Go, asi que se puede probar contra el paquete de verdad en el
// host, y lo que se prueba es la parte ASCII, que es la que se usa.
package unicode

const (
	MaxRune   = '\U0010FFFF'
	ReplacementChar = '\uFFFD'
	MaxASCII  = '\u007F'
	MaxLatin1 = '\u00FF'
)

// IsLetter: letras ASCII (a-z, A-Z).
func IsLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// IsDigit: digitos ASCII (0-9).
func IsDigit(r rune) bool { return r >= '0' && r <= '9' }

// IsNumber: los digitos y nada mas (en ASCII no hay mas categorias de numero).
func IsNumber(r rune) bool { return IsDigit(r) }

// IsSpace: los espacios de ASCII (incluye \t \n \v \f \r y el espacio).
func IsSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\v' || r == '\f' || r == '\r'
}

// IsUpper / IsLower / ToUpper / ToLower: solo ASCII.
func IsUpper(r rune) bool { return r >= 'A' && r <= 'Z' }
func IsLower(r rune) bool { return r >= 'a' && r <= 'z' }

func ToUpper(r rune) rune {
	if IsLower(r) {
		return r - ('a' - 'A')
	}
	return r
}

func ToLower(r rune) rune {
	if IsUpper(r) {
		return r + ('a' - 'A')
	}
	return r
}

// IsPrint: los imprimibles de ASCII.
func IsPrint(r rune) bool { return r >= 0x20 && r <= 0x7E }

// IsControl: los de control de ASCII (y cualquier cosa por debajo del espacio).
func IsControl(r rune) bool { return (r >= 0 && r <= 0x1F) || (r >= 0x7F && r <= 0x9F) }

// IsPunct: la puntuacion de ASCII, con la misma idea que Go (imprimible que no es letra, digito ni
// espacio).
func IsPunct(r rune) bool {
	return IsPrint(r) && !IsLetter(r) && !IsDigit(r) && !IsSpace(r)
}

// IsGraphic: imprimible que no es espacio.
func IsGraphic(r rune) bool { return IsPrint(r) && !IsSpace(r) }
