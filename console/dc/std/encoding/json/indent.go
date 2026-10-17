// Indent: el re-acomodador de texto json de la consola (el equivalente de json.Indent).
//
// POR QUE SE PUEDE HACER SIN REFLEXION: esto no mira la ESTRUCTURA de Go (eso es justo lo que no se
// puede), mira TEXTO json: recorre los bytes y solo AGREGA espacios y saltos FUERA de las cadenas, que
// es el mismo camino por el que el host acomoda lo que devuelve un MarshalJSON. Con esto la consola
// escribe el config.json con los MISMOS bytes que el host (medido caso por caso en el difftest del
// host, y en la consola contra los bytes horneados del host).
//
// LIMITE DECLARADO: los MENSAJES de error no son los de Go (no se copian; son texto de diagnostico que
// nadie compara). Lo que si se exige, y se mide en el difftest del host, es coincidir en (a) si el
// texto es json valido y (b) los bytes de la salida cuando lo es.
//
// La forma del json valido que se exige es la de la gramatica (RFC 8259): un valor suelto en la raiz;
// dentro de un objeto, clave (cadena) y despues ':'; una coma de mas o de menos es error; los numeros
// son los de la gramatica (-?int[.frac][e[+-]exp], sin ceros a la izquierda, sin punto suelto); las
// cadenas no llevan caracteres de control crudos y los escapes son los de json.
package json

import "d2std/errors"

// ErrIndent: el texto no es json valido (ver el limite de arriba: el mensaje no es el de Go).
var ErrIndent = errors.New("json: el texto no es json valido")

// Indent copia `datos` y le agrega la sangria: `prefijo` al principio de cada linea y `sangria` una vez
// por nivel de anidamiento (igual que json.Indent de Go).
func Indent(datos []byte, prefijo, sangria string) ([]byte, error) {
	t := &indentador{
		datos:        datos,
		prefijo:      prefijo,
		indentacion:  sangria,
		salida:       make([]byte, 0, len(datos)+len(datos)/2),
		estados:      make([]byte, 0, 8),
		tipos:        make([]byte, 0, 8),
	}

	if err := t.correr(); err != nil {
		return nil, err
	}

	return t.salida, nil
}

// indentador es el estado de la pasada. Estados de un contenedor:
//
//	'e' objeto recien abierto (una clave, o el cierre si esta vacio)
//	'k' objeto despues de una coma (una clave: una coma de mas es error)
//	'd' objeto con su clave (los dos puntos)
//	'v' array abierto o valor esperado (un valor; en un array admite el cierre si esta vacio)
//	'x' array despues de una coma: viene un valor (el cierre NO vale: eso es una coma de mas)
//	'c' ya hay un valor (una coma o el cierre)
type indentador struct {
	datos       []byte
	salida      []byte
	prefijo     string
	indentacion string
	estados     []byte
	tipos       []byte // 'o' objeto, 'a' array (de arriba hacia abajo)
	profundidad int
	sangria     bool // se abrio un contenedor y todavia no llego su primer token
	raizLista   bool // el valor de la raiz ya termino: no puede venir otro
	i           int
}

func (t *indentador) correr() error {
	for t.i < len(t.datos) {
		c := t.datos[t.i]

		if t.raizLista {
			// Despues del valor de la raiz solo se admiten blancos... y los blancos se COPIAN TAL CUAL.
			// No es un descuido nuestro: el scanner de Go, en el estado de fin de la raiz, devuelve scanEnd
			// (no scanSkipSpace) para un blanco, asi que json.Indent lo escribe. Medido en el difftest
			// ("blancos alrededor": Go devuelve "}\r\n" con su blanco atras).
			if esBlanco(c) {
				t.salida = append(t.salida, c)
				t.i++

				continue
			}

			return ErrIndent
		}

		if esBlanco(c) {
			t.i++
			continue
		}

		switch t.estado() {
		case 'v', 'x':
			if err := t.valor(); err != nil {
				return err
			}
		case 'e':
			if c == '}' {
				t.cerrar('}')
				continue
			}
			if err := t.clave(); err != nil {
				return err
			}
		case 'k':
			if err := t.clave(); err != nil {
				return err
			}
		case 'd':
			if c != ':' {
				return ErrIndent
			}
			t.escribir(1)
			t.salida = append(t.salida, ' ')
			t.ponerEstado('v')
		case 'c':
			switch c {
			case ',':
				t.escribir(1)
				if t.tipos[len(t.tipos)-1] == 'o' {
					t.ponerEstado('k')
				} else {
					t.ponerEstado('x') // en un array, despues de la coma VIENE un valor (la coma de mas es error)
				}
				t.salto()
			case '}', ']':
				t.cerrar(c)
			default:
				return ErrIndent
			}
		default:
			return ErrIndent
		}
	}

	if len(t.estados) != 0 || !t.raizLista {
		return ErrIndent // se cerro el texto con un contenedor abierto (o sin ningun valor)
	}

	return nil
}

// estado: el de arriba de la pila; sin pila, la raiz espera un valor.
func (t *indentador) estado() byte {
	if len(t.estados) == 0 {
		return 'v'
	}

	return t.estados[len(t.estados)-1]
}

func (t *indentador) ponerEstado(e byte) {
	if len(t.estados) > 0 {
		t.estados[len(t.estados)-1] = e
	}
}

// token: se consume la sangria pendiente del contenedor (la del primer token que aparece adentro). El
// cierre de un contenedor NO pasa por aca: un contenedor vacio se escribe `{}` / `[]` sin saltos.
func (t *indentador) token() {
	if t.sangria {
		t.sangria = false
		t.profundidad++
		t.salto()
	}
}

func (t *indentador) salto() {
	t.salida = append(t.salida, '\n')
	t.salida = append(t.salida, t.prefijo...)

	for n := 0; n < t.profundidad; n++ {
		t.salida = append(t.salida, t.indentacion...)
	}
}

func (t *indentador) escribir(n int) {
	t.salida = append(t.salida, t.datos[t.i:t.i+n]...)
	t.i += n
}

func (t *indentador) abrir(c byte) {
	t.token()
	t.salida = append(t.salida, c)
	t.i++

	if c == '{' {
		t.estados = append(t.estados, 'e')
		t.tipos = append(t.tipos, 'o')
	} else {
		t.estados = append(t.estados, 'v')
		t.tipos = append(t.tipos, 'a')
	}

	t.sangria = true
}

func (t *indentador) cerrar(c byte) {
	if t.sangria {
		// contenedor vacio: no lleva salto ni cambia la profundidad
		t.sangria = false
	} else {
		t.profundidad--
		t.salto()
	}

	t.salida = append(t.salida, c)
	t.i++

	t.estados = t.estados[:len(t.estados)-1]
	t.tipos = t.tipos[:len(t.tipos)-1]

	if len(t.estados) > 0 {
		t.ponerEstado('c')
	} else {
		t.raizLista = true
	}
}

// valor: consume un valor completo (cadena, numero, literal, objeto o array).
func (t *indentador) valor() error {
	c := t.datos[t.i]

	if c == ']' && len(t.estados) > 0 && t.tipos[len(t.tipos)-1] == 'a' && t.estado() == 'v' {
		t.cerrar(']')

		return nil
	}

	switch {
	case c == '"':
		if err := t.cadena(); err != nil {
			return err
		}
	case c == '{', c == '[':
		t.abrir(c)

		return nil
	case c == 't', c == 'f', c == 'n':
		if err := t.literal(); err != nil {
			return err
		}
	case c == '-' || (c >= '0' && c <= '9'):
		if err := t.numero(); err != nil {
			return err
		}
	default:
		return ErrIndent
	}

	// el valor termino: el contenedor de arriba espera una coma o su cierre
	if len(t.estados) > 0 {
		t.ponerEstado('c')
	} else {
		t.raizLista = true
	}

	return nil
}

// clave: dentro de un objeto, una clave (que tiene que ser una cadena y no puede repetirse el cierre).
func (t *indentador) clave() error {
	if t.datos[t.i] != '"' {
		return ErrIndent
	}

	if err := t.cadena(); err != nil {
		return err
	}

	t.ponerEstado('d')

	return nil
}

// cadena: copia una cadena json TAL CUAL (con sus escapes) y valida que cierre y que los escapes sean
// los de json (los 7 cortos y \u con 4 digitos hexadecimales). Los caracteres de control van escapados.
func (t *indentador) cadena() error {
	t.token()
	t.salida = append(t.salida, '"')

	j := t.i + 1

	for j < len(t.datos) {
		switch c := t.datos[j]; c {
		case '"':
			j++
			t.salida = append(t.salida, t.datos[t.i+1:j]...)
			t.i = j

			return nil
		case '\\':
			if j+1 >= len(t.datos) {
				return ErrIndent
			}

			switch e := t.datos[j+1]; e {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				j += 2
			case 'u':
				if j+5 >= len(t.datos) {
					return ErrIndent
				}

				for n := 2; n < 6; n++ {
					if !esHex(t.datos[j+n]) {
						return ErrIndent
					}
				}

				j += 6
			default:
				return ErrIndent
			}
		default:
			if c < 0x20 {
				return ErrIndent
			}

			j++
		}
	}

	return ErrIndent // no cierra
}

// literal: true, false o null, exactos y seguidos de un separador (cosa que `truex` no lo sea).
func (t *indentador) literal() error {
	t.token()

	for _, palabra := range []string{"true", "false", "null"} {
		if t.empieza(palabra) && t.separador(t.i+len(palabra)) {
			t.escribir(len(palabra))

			return nil
		}
	}

	return ErrIndent
}

// numero: la gramatica de json: -?int[.frac][eE[+-]exp], sin ceros a la izquierda y sin punto suelto.
func (t *indentador) numero() error {
	t.token()

	j := t.i

	if t.datos[j] == '-' {
		j++
	}

	if j >= len(t.datos) {
		return ErrIndent
	}

	switch {
	case t.datos[j] == '0':
		j++
	case t.datos[j] >= '1' && t.datos[j] <= '9':
		for j < len(t.datos) && t.datos[j] >= '0' && t.datos[j] <= '9' {
			j++
		}
	default:
		return ErrIndent
	}

	if j < len(t.datos) && t.datos[j] == '.' {
		j++

		if j >= len(t.datos) || t.datos[j] < '0' || t.datos[j] > '9' {
			return ErrIndent
		}

		for j < len(t.datos) && t.datos[j] >= '0' && t.datos[j] <= '9' {
			j++
		}
	}

	if j < len(t.datos) && (t.datos[j] == 'e' || t.datos[j] == 'E') {
		j++

		if j < len(t.datos) && (t.datos[j] == '+' || t.datos[j] == '-') {
			j++
		}

		if j >= len(t.datos) || t.datos[j] < '0' || t.datos[j] > '9' {
			return ErrIndent
		}

		for j < len(t.datos) && t.datos[j] >= '0' && t.datos[j] <= '9' {
			j++
		}
	}

	if !t.separador(j) {
		return ErrIndent
	}

	t.salida = append(t.salida, t.datos[t.i:j]...)
	t.i = j

	return nil
}

// separador: lo que puede venir despues de un literal o un numero (fin, blanco o puntuacion).
func (t *indentador) separador(j int) bool {
	if j >= len(t.datos) {
		return true
	}

	c := t.datos[j]

	return esBlanco(c) || c == ',' || c == '}' || c == ']'
}

func (t *indentador) empieza(palabra string) bool {
	if t.i+len(palabra) > len(t.datos) {
		return false
	}

	for n := 0; n < len(palabra); n++ {
		if t.datos[t.i+n] != palabra[n] {
			return false
		}
	}

	return true
}

func esBlanco(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func esHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
