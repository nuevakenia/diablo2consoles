// Sscanf: leer valores desde un texto siguiendo un formato. Es la parte del fmt de go que el motor usa.
//
// MEDIDO ANTES DE ESCRIBIRLO, para no hacer de mas: el motor usa Sscanf en DOS lugares y con DOS formatos:
//
//	d2core/d2map/d2mapgen/real_maze_gen.go:437  fmt.Sscanf(lower, "place_group%d", &pct)
//	d2core/d2skills/engine.go:265               fmt.Sscanf(ref, "%d", &id)
//
// O sea: un literal y un %d. Con eso alcanza, pero se implementa un poco mas (varios verbos y varios
// literales seguidos) porque es barato y porque los formatos del motor pueden cambiar.
//
// LO QUE SOPORTA: texto literal, y los verbos %d %s %f %x %c %v, con el ancho opcional (%2d). Los
// espacios en el formato comen cualquier cantidad de espacios.
//
// OJO CON UNA COSA QUE LA PRUEBA CAZO: NO hay comodin. "%*d" no es de go, es de C. La prueba contra el
// Sscanf de go lo mostro claro: go contesta "bad verb '%*' for integer" y no lee nada. Asi que aca "%*d"
// tampoco lee: es un verbo que no se soporta, y se devuelve el error. Inventar un comportamiento que go no
// tiene seria peor que no tenerlo.
//
// LO QUE NO SOPORTA (y esta anotado para que no sorprenda): %e %g %b %o %q %p, y las interfaces
// Scanner/Formatter de go. Si el motor los llega a pedir, se ven al medir.
package fmt

import (
	"errors"
	"strconv"
	"strings"
)

// ErrSscan: no se pudo leer lo que pedia el formato.
var ErrSscan = errors.New("fmt: no se pudo leer segun el formato")

// Sscanf: lee de texto segun formato. Devuelve cuantos valores se leyeron.
func Sscanf(texto, formato string, args ...interface{}) (int, error) {
	// el texto se recorre por runas: los formatos del motor son ascii, pero el texto puede no serlo
	corriente := []rune(texto)
	i := 0
	leidos := 0
	argIdx := 0

	esEspacio := func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f'
	}

	// salta los espacios del texto
	saltar := func() {
		for i < len(corriente) && esEspacio(corriente[i]) {
			i++
		}
	}

	// lee hasta el proximo espacio (o hasta n caracteres si n > 0)
	palabra := func(n int) string {
		saltar()
		inicio := i
		for i < len(corriente) && !esEspacio(corriente[i]) {
			if n > 0 && i-inicio >= n {
				break
			}
			i++
		}
		return string(corriente[inicio:i])
	}

	for p := 0; p < len(formato); p++ {
		c := formato[p]

		if esEspacio(rune(c)) {
			saltar()
			continue
		}

		if c != '%' {
			// un caracter literal tiene que estar
			if i >= len(corriente) || byte(corriente[i]) != c {
				return leidos, ErrSscan
			}
			i++
			continue
		}

		// viene un verbo
		p++
		if p >= len(formato) {
			return leidos, ErrSscan
		}

		// el ancho (por ejemplo %2d). No hay comodin: en go no existe (ver arriba).
		ancho := 0
		for p < len(formato) && formato[p] >= '0' && formato[p] <= '9' {
			ancho = ancho*10 + int(formato[p]-'0')
			p++
		}
		if p >= len(formato) {
			return leidos, ErrSscanfVerbo()
		}
		verbo := formato[p]

		if verbo == '%' {
			// un %% pide un % literal
			if i >= len(corriente) || corriente[i] != '%' {
				return leidos, ErrSscanfVerbo()
			}
			i++
			continue
		}

		// tiene que haber un destino
		if argIdx >= len(args) {
			return leidos, ErrSscanfVerbo()
		}
		destino := args[argIdx]
		argIdx++

		switch verbo {
		case 'd', 'v':
			s := palabra(ancho)
			if s == "" {
				return leidos, ErrSscanfVerbo()
			}
			n, err := strconv.Atoi(s)
			if err != nil {
				// puede ser un entero con signo y mas digitos de los que entran: se avisa
				return leidos, ErrSscanfVerbo()
			}
			switch d := destino.(type) {
			case *int:
				*d = n
			case *int8:
				*d = int8(n)
			case *int16:
				*d = int16(n)
			case *int32:
				*d = int32(n)
			case *int64:
				*d = int64(n)
			case *uint:
				*d = uint(n)
			case *uint8:
				*d = uint8(n)
			case *uint16:
				*d = uint16(n)
			case *uint32:
				*d = uint32(n)
			case *uint64:
				*d = uint64(n)
			default:
				return leidos, ErrSscan
			}

		case 'x', 'X':
			s := palabra(ancho)
			if s == "" {
				return leidos, ErrSscanfVerbo()
			}
			n, err := strconv.ParseInt(s, 16, 64)
			if err != nil {
				return leidos, ErrSscanfVerbo()
			}
			switch d := destino.(type) {
			case *int:
				*d = int(n)
			case *int64:
				*d = n
			case *uint64:
				*d = uint64(n)
			case *uint32:
				*d = uint32(n)
			default:
				return leidos, ErrSscan
			}

		case 's':
			s := palabra(ancho)
			if s == "" {
				return leidos, ErrSscanfVerbo()
			}
			if d, ok := destino.(*string); ok {
				*d = s
			} else {
				return leidos, ErrSscan
			}

		case 'f', 'F':
			s := palabra(ancho)
			if s == "" {
				return leidos, ErrSscanfVerbo()
			}
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return leidos, ErrSscanfVerbo()
			}
			switch d := destino.(type) {
			case *float64:
				*d = f
			case *float32:
				*d = float32(f)
			default:
				return leidos, ErrSscan
			}

		case 'c':
			// %c lee un caracter, sin saltar espacios (como el de go)
			if i >= len(corriente) {
				return leidos, ErrSscanfVerbo()
			}
			r := corriente[i]
			i++
			switch d := destino.(type) {
			case *rune:
				*d = r
			case *byte:
				*d = byte(r)
			case *string:
				*d = string(r)
			default:
				return leidos, ErrSscan
			}

		default:
			// un verbo que no se soporta: se avisa fuerte en vez de leer cualquier cosa
			return leidos, ErrSscan
		}

		leidos++
	}

	return leidos, nil
}

// ErrSscanfVerbo: el error de "aca no hay lo que pedia el formato". Se arma aparte para que el mensaje
// diga donde, que es lo primero que uno quiere saber.
func ErrSscanfVerbo() error {
	return errors.New("fmt: el texto no tiene lo que pide el formato")
}

// Sscan: igual que Sscanf pero sin formato (lee los valores separados por espacios).
func Sscan(texto string, args ...interface{}) (int, error) {
	return Sscanln(texto, args...)
}

// Sscanln: los valores separados por espacios o saltos de linea.
func Sscanln(texto string, args ...interface{}) (int, error) {
	campos := strings.Fields(texto)
	leidos := 0
	for n, arg := range args {
		if n >= len(campos) {
			return leidos, ErrSscan
		}
		campo := campos[n]
		switch d := arg.(type) {
		case *string:
			*d = campo
		case *int:
			v, err := strconv.Atoi(campo)
			if err != nil {
				return leidos, ErrSscan
			}
			*d = v
		case *float64:
			v, err := strconv.ParseFloat(campo, 64)
			if err != nil {
				return leidos, ErrSscan
			}
			*d = v
		default:
			return leidos, ErrSscan
		}
		leidos++
	}
	return leidos, nil
}
