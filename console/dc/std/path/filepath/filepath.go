// Paquete path/filepath para la consola: los paths del motor (317 llamadas a Join, mas Base, Clean,
// Dir, Ext, SplitList y Abs).
//
// Es PURO TEXTO: no toca el sistema de archivos ni depende de la plataforma, y por eso se puede probar
// de la mejor manera posible: comparando contra el filepath de Go, caso por caso, en el host
// (difftest/dc0026b_test.go) y con los mismos casos horneados en el corpus de la consola.
//
// En la consola el separador es "/" y NO hay volumenes ("C:") ni "\\": eso simplifica el codigo con
// respecto al de Go, pero el RESULTADO tiene que ser identico (es lo que se mide).
package filepath

// Separator es el separador de caminos.
const Separator = '/'

// ListSeparator es el separador de listas de caminos (SplitList).
const ListSeparator = ':'

// IsAbs dice si el camino es absoluto (empieza con "/").
func IsAbs(path string) bool { return len(path) > 0 && path[0] == '/' }

// Join junta los elementos con "/" y limpia el resultado. Es la funcion mas usada del paquete (317
// llamadas en el motor), asi que tiene que dar EXACTAMENTE lo mismo que Go: los elementos vacios se
// ignoran y Join() de cero elementos es "".
func Join(elem ...string) string {
	partes := make([]string, 0, len(elem))
	for _, e := range elem {
		if e != "" {
			partes = append(partes, e)
		}
	}
	if len(partes) == 0 {
		return ""
	}
	return Clean(juntar(partes, "/"))
}

// juntar une con el separador (no se usa strings.Join para no meter una dependencia).
func juntar(partes []string, sep string) string {
	s := ""
	for i, p := range partes {
		if i > 0 {
			s += sep
		}
		s += p
	}
	return s
}

// Clean deja el camino en su forma lexica: sin ".", sin ".." que se puedan resolver, sin barras
// repetidas y sin barra final. Es el algoritmo de Go (puro texto, nada de FS).
func Clean(path string) string {
	if path == "" {
		return "."
	}
	raiz := ""
	if path[0] == '/' {
		raiz = "/"
	}
	var salida []string
	i := 0
	for i < len(path) {
		for i < len(path) && path[i] == '/' {
			i++
		}
		if i >= len(path) {
			break
		}
		j := i
		for j < len(path) && path[j] != '/' {
			j++
		}
		el := path[i:j]
		i = j
		if el == "." {
			continue
		}
		if el == ".." {
			if len(salida) > 0 && salida[len(salida)-1] != ".." {
				salida = salida[:len(salida)-1]
			} else if raiz == "" {
				salida = append(salida, "..")
			}
			continue
		}
		salida = append(salida, el)
	}
	res := raiz
	for k, e := range salida {
		if k > 0 {
			res += "/"
		}
		res += e
	}
	if res == "" {
		return "."
	}
	return res
}

// Base es el ultimo elemento del camino. Base("") y Base("/") son "." y "/" (como en Go).
func Base(path string) string {
	if path == "" {
		return "."
	}
	// se sacan las barras del final (pero una barra sola es la raiz)
	i := len(path)
	for i > 0 && path[i-1] == '/' {
		i--
	}
	if i == 0 {
		return "/"
	}
	path = path[:i]
	j := i
	for j > 0 && path[j-1] != '/' {
		j--
	}
	return path[j:]
}

// Dir es todo menos el ultimo elemento. OJO con el algoritmo: NO se sacan las barras del final antes de
// cortar (eso fue un defecto mio, cazado por la prueba diferencial): se corta en la ULTIMA barra y se
// limpia lo de antes con Clean, que es el que saca las barras del final. Por eso Dir("/a/b/") es "a/b" y
// Dir("/") es "/", no ".".
func Dir(path string) string {
	i := len(path) - 1
	for i >= 0 && path[i] != '/' {
		i--
	}
	return Clean(path[:i+1])
}

// Ext es el sufijo desde el ultimo punto del ultimo elemento (vacio si no hay).
func Ext(path string) string {
	for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
		if path[i] == '.' {
			return path[i:]
		}
	}
	return ""
}

// Split parte en directorio y archivo: corta en la ULTIMA barra y NO limpia nada (a diferencia de Dir).
// Por eso Split("a/b/") da ("a/b/", "") y Split("/") da ("/", ""). El segundo defecto mio que cazo la
// prueba diferencial fue este: yo sacaba las barras del final y Split("a/b/") daba ("a/", "b/").
func Split(path string) (dir, file string) {
	i := len(path) - 1
	for i >= 0 && path[i] != '/' {
		i--
	}
	return path[:i+1], path[i+1:]
}

// SplitList parte una lista de caminos (separados por ":" en la consola).
func SplitList(path string) []string {
	if path == "" {
		return []string{}
	}
	var partes []string
	actual := ""
	for i := 0; i < len(path); i++ {
		if path[i] == ListSeparator {
			partes = append(partes, actual)
			actual = ""
			continue
		}
		actual += string(path[i])
	}
	partes = append(partes, actual)
	return partes
}

// Abs: en la consola no hay "directorio actual" (no hay procesos), asi que un camino relativo se
// resuelve contra la raiz del romdisk. Es lo que el motor espera de un "absoluto" en un entorno
// chico: que sea absoluto. Queda declarado que no hay cwd.
func Abs(path string) (string, error) {
	if IsAbs(path) {
		return Clean(path), nil
	}
	return Clean("/" + path), nil
}

// ToSlash y FromSlash existen porque el motor los nombra: en la consola el separador ya es "/".
func ToSlash(path string) string   { return path }
func FromSlash(path string) string { return path }

// QUE NO ESTA, Y POR QUE
//
// Glob (13 llamadas en el motor) y Walk (2) necesitan LISTAR directorios, o sea el paquete os, que en
// la consola habla con KOS. Si este archivo importara os, el paquete entero dejaria de poder probarse
// en el host contra el filepath de Go, y la parte de texto (que es el 95% del uso: 317 Join, 26 Base,
// 21 Clean, 16 Dir, 12 Ext) perderia su prueba diferencial. Asi que Glob y Walk NO estan aca todavia:
// van con el bucket de datos (DC-002.9), que es donde el motor los usa (el descubrimiento de datos del
// setup). El dia que se portee ese codigo, la falta se ve al compilar, que es como tiene que verse.
//
// Tampoco estan EvalSymlinks, Rel, VolumeName ni Match: en la consola no hay enlaces, ni directorio
// actual (no hay procesos), ni volumenes ("C:").
