// Package regexp: expresiones regulares para la consola.
//
// Por que una implementacion propia: el regexp de Go es RE2 (un motor grande, con automatas). El motor
// de Diablo II lo usa en 16 lugares y MIDIENDO los patrones que pide son todos chicos:
//
//     \d+          \D           ^-?\d+$        [^a-z0-9\-]+     \[[a-z]+\]
//     ^cm[123]$    ^g[cfszvlp][a-z]$           ^r[0-3][0-9]$    ^(pk[123]|tes|ceh|bet|fed)
//     skill\('([^']+)          (?i)...
//
// Nada de lo que hay ahi necesita un automata: se resuelven con un motor por vuelta atras (backtracking)
// de los de toda la vida. Eso es lo que hay aca.
//
// Que soporta: literales, clases [abc], rangos [a-z], clases negadas [^abc], los escapes \d \D \w \W \s
// \S y los literales escapados, los anclas ^ y $, los cuantificadores + * ? y {n,m}, la alternancia |,
// los grupos con y sin captura, y el flag (?i) de "no distinguir mayusculas".
//
// Que NO: no hay clases POSIX ([:alpha:]), ni \b, ni los grupos con nombre, ni los modos de varias
// lineas. Si el motor los pide, se ven al medir (el error dice el patron) y se agregan.

package regexp

import (
	"d2std/errors"
	"d2std/strings"
)

// Regexp: el patron ya compilado.
type Regexp struct {
	patron     string
	sinMayus   bool
	raiz       *nodo
	subexpresiones int
}

// Error: lo que devuelve Compile cuando el patron no se entiende.
type Error struct {
	Patron string
	Msg    string
}

func (e *Error) Error() string { return "regexp: " + e.Msg + " en " + e.Patron }

// MustCompile compila y hace panic si el patron esta mal (igual que en Go).
func MustCompile(patron string) *Regexp {
	r, err := Compile(patron)
	if err != nil {
		panic(err)
	}
	return r
}

// Compile compila el patron.
func Compile(patron string) (*Regexp, error) {
	r := &Regexp{patron: patron}

	p := patron
	// DC-002.16: el prefijo de la bandera mide CUATRO bytes ("(?i)" = '(' '?' 'i' ')'), no tres. Con
	// p[3:] quedaba "i)a" y el parser se plantaba en el ')' -> Compile devolvia
	// `regexp: sobra )a en (?i)a`, un MustCompile de nivel de paquete paniqueaba y el arranque del motor
	// no llegaba ni a main (medido en la consola: panic con un *regexp.Error, 0 lineas MOTOR-ARRANQUE).
	if strings.HasPrefix(p, "(?i)") {
		r.sinMayus = true
		p = p[4:]
	}

	raiz, resto, err := parseAlternancia(p, r)
	if err != nil {
		return nil, err
	}
	if resto != "" {
		return nil, &Error{patron, "sobra " + resto}
	}
	if raiz == nil {
		return nil, errors.New("regexp: patron vacio")
	}
	r.raiz = raiz
	return r, nil
}

// String: el patron (igual que en Go).
func (r *Regexp) String() string { return r.patron }

// NumSubexp: cuantos grupos de captura tiene.
func (r *Regexp) NumSubexp() int { return r.subexpresiones }

// ================================================================ el arbol del patron
//
// El patron se arma como un arbol chico de nodos, y despues se prueba por vuelta atras. Es la forma mas
// directa: para patrones de este tamano no hace falta nada mas sofisticado.

type nodo struct {
	tipo     byte // 0 = secuencia, 1 = alternancia, 2 = literal, 3 = clase, 4 = ancla, 5 = repetir, 6 = grupo
	valor    string
	clase    *claseChar
	ancla    byte   // '^' o '$'
	min      int
	max      int // -1 = sin tope
	sub      []*nodo
	subcampo int // el numero de grupo, 0 = ninguno
}

// claseChar: un conjunto de caracteres (con la posibilidad de estar negado).
type claseChar struct {
	negada  bool
	rangos  [][2]rune
	digitos bool // \d
	noDig   bool // \D
	espacio bool // \s
	noEsp   bool // \S
	palabra bool // \w
	noPal   bool // \W
}

func (c *claseChar) contiene(r rune, sinMayus bool) bool {
	esta := false

	if c.digitos && r >= '0' && r <= '9' {
		esta = true
	}
	if c.espacio && (r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v') {
		esta = true
	}
	if c.palabra && (esPalabra(r) || (sinMayus && esPalabra(alternarMayus(r)))) {
		esta = true
	}
	for _, rg := range c.rangos {
		// primero tal cual
		if r >= rg[0] && r <= rg[1] {
			esta = true
			break
		}
		// despues sin distinguir mayusculas, si el patron lo pide
		if sinMayus {
			rm := alternativaMayus(r)
			if rm != r && rm >= rg[0] && rm <= rg[1] {
				esta = true
				break
			}
		}
	}

	if c.negada {
		return !esta
	}
	return esta
}

func esPalabra(r rune) bool {
	return r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func alternarMayus(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	if r >= 'A' && r <= 'Z' {
		return r + 32
	}
	return r
}

// plegarMayus: lleva la letra a minuscula. Es lo que hace falta para COMPARAR dos caracteres con (?i)
// ("A" y "a" tienen que dar igual). Ojo: no confundir con alternarMayus, que es un INTERCAMBIO de caso
// (a->A, A->a) -- es biyectivo, asi que alternarMayus(x) == alternarMayus(y) solo cuando x == y, y usado
// para comparar deja la bandera (?i) MUERTA. Ese fue el segundo defecto de DC-002.16: `(?i)a` no
// encontraba la "A" de "BANANA" ni con el prefijo ya cortado bien.
// Solo ASCII, a proposito: la consola no tiene las tablas de plegado de Unicode (limite declarado en
// TestLimitesConocidos).
func plegarMayus(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 32
	}
	return r
}

// alternativaMayus: el intercambio de caso, para preguntar "¿el OTRO caso cae en esta clase?" (el
// test simetrico que usa contiene para [a-z] y [A-Z]; ahi el intercambio es lo correcto y el plegado no).
func alternativaMayus(r rune) rune { return alternarMayus(r) }

// ================================================================ parseo

func parseAlternancia(p string, r *Regexp) (*nodo, string, error) {
	primero, resto, err := parseSecuencia(p, r)
	if err != nil {
		return nil, resto, err
	}
	if strings.HasPrefix(resto, "|") {
		opciones := []*nodo{primero}
		for strings.HasPrefix(resto, "|") {
			var sig *nodo
			sig, resto, err = parseSecuencia(resto[1:], r)
			if err != nil {
				return nil, resto, err
			}
			opciones = append(opciones, sig)
		}
		return &nodo{tipo: 1, sub: opciones}, resto, nil
	}
	return primero, resto, nil
}

func parseSecuencia(p string, r *Regexp) (*nodo, string, error) {
	sec := &nodo{tipo: 0}
	for len(p) > 0 {
		c := p[0]
		if c == '|' || c == ')' {
			break
		}
		var n *nodo
		var err error
		n, p, err = parseAtomo(p, r)
		if err != nil {
			return nil, p, err
		}
		// los cuantificadores
		n, p = parseCuantificador(n, p)
		sec.sub = append(sec.sub, n)
	}
	if len(sec.sub) == 1 && sec.sub[0].tipo != 5 {
		return sec.sub[0], p, nil
	}
	return sec, p, nil
}

func parseAtomo(p string, r *Regexp) (*nodo, string, error) {
	c := p[0]
	switch c {
	case '^', '$':
		return &nodo{tipo: 4, ancla: c}, p[1:], nil
	case '.':
		// "." = cualquier caracter menos el salto de linea
		return &nodo{tipo: 3, clase: &claseChar{negada: true, rangos: [][2]rune{{'\n', '\n'}}}}, p[1:], nil
	case '(':
		// grupo, con o sin captura
		captura := true
		resto := p[1:]
		if strings.HasPrefix(resto, "?:") {
			captura = false
			resto = resto[2:]
		}
		nro := 0
		if captura {
			r.subexpresiones++
			nro = r.subexpresiones
		}
		sub, resto2, err := parseAlternancia(resto, r)
		if err != nil {
			return nil, resto2, err
		}
		if !strings.HasPrefix(resto2, ")") {
			return nil, resto2, &Error{r.patron, "falta cerrar un grupo"}
		}
		return &nodo{tipo: 6, sub: []*nodo{sub}, subcampo: nro}, resto2[1:], nil
	case '[':
		cl, resto, err := parseClase(p[1:], r)
		return &nodo{tipo: 3, clase: cl}, resto, err
	case '\\':
		if len(p) < 2 {
			return nil, "", &Error{r.patron, "barra al final"}
		}
		e := p[1]
		switch e {
		case 'd':
			return &nodo{tipo: 3, clase: &claseChar{digitos: true}}, p[2:], nil
		case 'D':
			return &nodo{tipo: 3, clase: &claseChar{negada: true, rangos: [][2]rune{{'0', '9'}}}}, p[2:], nil
		case 'w':
			return &nodo{tipo: 3, clase: &claseChar{palabra: true}}, p[2:], nil
		case 'W':
			return &nodo{tipo: 3, clase: &claseChar{negada: true, palabra: true}}, p[2:], nil
		case 's':
			return &nodo{tipo: 3, clase: &claseChar{espacio: true}}, p[2:], nil
		case 'S':
			return &nodo{tipo: 3, clase: &claseChar{negada: true, espacio: true}}, p[2:], nil
		case 'n':
			return &nodo{tipo: 2, valor: "\n"}, p[2:], nil
		case 't':
			return &nodo{tipo: 2, valor: "\t"}, p[2:], nil
		default:
			return &nodo{tipo: 2, valor: string(e)}, p[2:], nil
		}
	default:
		return &nodo{tipo: 2, valor: string(c)}, p[1:], nil
	}
}

func parseClase(p string, r *Regexp) (*claseChar, string, error) {
	cl := &claseChar{}
	if strings.HasPrefix(p, "^") {
		cl.negada = true
		p = p[1:]
	}
	primero := true
	for len(p) > 0 {
		c := p[0]
		if c == ']' && !primero {
			return cl, p[1:], nil
		}
		primero = false

		if c == '\\' && len(p) > 1 {
			switch p[1] {
			case 'd':
				cl.digitos = true
			case 'D':
				cl.noDig = true
			case 'w':
				cl.palabra = true
			case 'W':
				cl.noPal = true
			case 's':
				cl.espacio = true
			case 'S':
				cl.noEsp = true
			case 'n':
				cl.rangos = append(cl.rangos, [2]rune{'\n', '\n'})
			case 't':
				cl.rangos = append(cl.rangos, [2]rune{'\t', '\t'})
			default:
				cl.rangos = append(cl.rangos, [2]rune{rune(p[1]), rune(p[1])})
			}
			p = p[2:]
			continue
		}

		// rango a-z
		if len(p) >= 3 && p[1] == '-' {
			cl.rangos = append(cl.rangos, [2]rune{rune(c), rune(p[2])})
			p = p[3:]
			continue
		}

		cl.rangos = append(cl.rangos, [2]rune{rune(c), rune(c)})
		p = p[1:]
	}
	return nil, p, &Error{r.patron, "falta cerrar una clase"}
}

func parseCuantificador(n *nodo, p string) (*nodo, string) {
	if len(p) == 0 {
		return n, p
	}
	switch p[0] {
	case '+':
		return &nodo{tipo: 5, min: 1, max: -1, sub: []*nodo{n}}, p[1:]
	case '*':
		return &nodo{tipo: 5, min: 0, max: -1, sub: []*nodo{n}}, p[1:]
	case '?':
		return &nodo{tipo: 5, min: 0, max: 1, sub: []*nodo{n}}, p[1:]
	case '{':
		// {n} o {n,m}
		cierre := strings.IndexByte(p, '}')
		if cierre < 0 {
			return n, p
		}
		dentro := p[1:cierre]
		min, max := 0, 0
		if i := strings.IndexByte(dentro, ','); i >= 0 {
			min = parseNumero(dentro[:i])
			if dentro[i+1:] == "" {
				max = -1
			} else {
				max = parseNumero(dentro[i+1:])
			}
		} else {
			min = parseNumero(dentro)
			max = min
		}
		return &nodo{tipo: 5, min: min, max: max, sub: []*nodo{n}}, p[cierre+1:]
	}
	return n, p
}

func parseNumero(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return n
		}
		n = n*10 + int(s[i]-'0')
	}
	return n
}

// ================================================================ ejecucion (vuelta atras)

// capturas: donde empezo y termino cada grupo (de a pares), mientras se prueba el patron.
type capturas struct {
	rango []int
}

// coincide prueba el nodo contra el texto desde la posicion pos, y devuelve TODAS las posiciones
// alcanzables. La lista es lo que permite la vuelta atras: si lo que sigue del patron falla, se prueba
// la siguiente.
func (r *Regexp) coincide(n *nodo, texto []rune, pos int, caps *capturas) []int {
	switch n.tipo {
	case 2: // literal
		l := []rune(n.valor)
		if len(l) == 1 {
			if pos < len(texto) && r.igual(texto[pos], l[0]) {
				return []int{pos + 1}
			}
			return nil
		}
		if len(texto)-pos < len(l) {
			return nil
		}
		for i, c := range l {
			if !r.igual(texto[pos+i], c) {
				return nil
			}
		}
		return []int{pos + len(l)}

	case 3: // clase
		if pos < len(texto) && n.clase.contiene(texto[pos], r.sinMayus) {
			return []int{pos + 1}
		}
		return nil

	case 4: // ancla
		switch n.ancla {
		case '^':
			if pos == 0 {
				return []int{pos}
			}
		case '$':
			if pos == len(texto) {
				return []int{pos}
			}
		}
		return nil

	case 0: // secuencia: cada sub, en orden
		posiciones := []int{pos}
		for _, sub := range n.sub {
			var siguientes []int
			for _, p := range posiciones {
				siguientes = append(siguientes, r.coincide(sub, texto, p, caps)...)
			}
			if len(siguientes) == 0 {
				return nil
			}
			posiciones = siguientes
		}
		return posiciones

	case 1: // alternancia: cualquiera de las opciones
		var salida []int
		for _, op := range n.sub {
			salida = append(salida, r.coincide(op, texto, pos, caps)...)
		}
		return salida

	case 6: // grupo
		if n.subcampo == 0 {
			return r.coincide(n.sub[0], texto, pos, caps)
		}
		antes := len(caps.rango)
		caps.rango = append(caps.rango, -1, -1)
		// probar el sub y quedarse con la primera que llegue al final del grupo
		posiciones := r.coincide(n.sub[0], texto, pos, caps)
		if len(posiciones) == 0 {
			caps.rango = caps.rango[:antes]
			return nil
		}
		salidas := make([]int, 0, len(posiciones))
		for _, p := range posiciones {
			// la captura se queda con la mas LARGA que llego al final del grupo. Si se guardara la
			// ultima sin mas, "skill\('([^']+)" capturaba "F" en vez de "Fireball" (lo cazo la prueba
			// contra el regexp de Go): la cadena de + prueba todas las salidas y la ultima es la corta.
			if p-pos >= caps.rango[len(caps.rango)-1]-pos || caps.rango[len(caps.rango)-2] < 0 {
				caps.rango[len(caps.rango)-2] = pos
				caps.rango[len(caps.rango)-1] = p
			}
			salidas = append(salidas, p)
		}
		return salidas

	case 5: // repetir (con cuantificador)
		return r.repetir(n, texto, pos, caps, 0)
	}
	return nil
}

func (r *Regexp) repetir(n *nodo, texto []rune, pos int, caps *capturas, hechas int) []int {
	// la cantidad minima tiene que estar: si no llegamos, probar otra vez
	if hechas < n.min {
		salidas := r.coincide(n.sub[0], texto, pos, caps)
		var resultado []int
		for _, p := range salidas {
			if p == pos && hechas+1 >= n.min {
				// se repitio sin avanzar (patron vacio): cortar para no dar vueltas
				resultado = append(resultado, p)
				continue
			}
			resultado = append(resultado, r.repetir(n, texto, p, caps, hechas+1)...)
		}
		return deduplicar(resultado)
	}

	// ya se cumplio el minimo: el orden importa, y es la diferencia entre un + de verdad y un + que no
	// lo es. En Go los cuantificadores son GREEDY: primero se intenta repetir todo lo posible, y solo
	// despues se prueba el camino corto. Por eso aca va primero la repeticion y al final el "no repetir
	// mas" -- al reves, "aaa+" devolvia "a" en vez de "aaa" (lo cazo la prueba contra el regexp de Go).
	var salida []int
	if n.max < 0 || hechas < n.max {
		salidas := r.coincide(n.sub[0], texto, pos, caps)
		for _, p := range salidas {
			if p == pos {
				continue
			}
			salida = append(salida, r.repetir(n, texto, p, caps, hechas+1)...)
		}
	}
	salida = append(salida, pos)
	return deduplicar(salida)
}

func deduplicar(pos []int) []int {
	if len(pos) < 2 {
		return pos
	}
	visto := make(map[int]bool, len(pos))
	salida := pos[:0]
	for _, p := range pos {
		if !visto[p] {
			visto[p] = true
			salida = append(salida, p)
		}
	}
	return salida
}

// igual compara dos caracteres, respetando el flag (?i).
//
// DC-002.16: pliega los DOS al mismo caso (minuscula). Antes usaba alternarMayus en los dos lados, que
// es un intercambio: alternarMayus(a) == alternarMayus(b) solo si a == b, asi que la bandera (?i) no
// plegaba NADA en los literales y el patron del motor seguia siendo sensible a mayusculas.
func (r *Regexp) igual(a, b rune) bool {
	if a == b {
		return true
	}
	if r.sinMayus {
		return plegarMayus(a) == plegarMayus(b)
	}
	return false
}

// ================================================================ la API publica

// MatchString: si el patron esta en alguna parte del texto (sin anclas, como en Go).
func (r *Regexp) MatchString(s string) bool {
	return r.FindStringIndex(s) != nil
}

// Match: la version con bytes.
func (r *Regexp) Match(b []byte) bool { return r.MatchString(string(b)) }

// FindStringIndex: donde empieza y termina la primera coincidencia.
func (r *Regexp) FindStringIndex(s string) []int {
	texto := []rune(s)
	caps := &capturas{}
	for i := 0; i <= len(texto); i++ {
		salidas := r.coincide(r.raiz, texto, i, caps)
		if len(salidas) > 0 {
			return []int{i, salidas[0]}
		}
	}
	return nil
}

// FindAllStringIndex: todas las coincidencias.
func (r *Regexp) FindAllStringIndex(s string, n int) [][]int {
	texto := []rune(s)
	var salida [][]int
	desde := 0
	for desde <= len(texto) {
		caps := &capturas{}
		encontrado := false
		for i := desde; i <= len(texto); i++ {
			salidas := r.coincide(r.raiz, texto, i, caps)
			if len(salidas) > 0 {
				salida = append(salida, []int{i, salidas[0]})
				encontrado = true
				if salidas[0] > i {
					desde = salidas[0]
				} else {
					desde = i + 1
				}
				break
			}
		}
		if !encontrado {
			break
		}
		if n > 0 && len(salida) >= n {
			break
		}
	}
	return salida
}

// FindAllString: todas las coincidencias, como texto.
func (r *Regexp) FindAllString(s string, n int) []string {
	indices := r.FindAllStringIndex(s, n)
	texto := []rune(s)
	salida := make([]string, 0, len(indices))
	for _, ix := range indices {
		salida = append(salida, string(texto[ix[0]:ix[1]]))
	}
	return salida
}

// FindString: la primera coincidencia, como texto.
func (r *Regexp) FindString(s string) string {
	ix := r.FindStringIndex(s)
	if ix == nil {
		return ""
	}
	return string([]rune(s)[ix[0]:ix[1]])
}

// FindStringSubmatch: la primera coincidencia con sus grupos (el indice 0 es la coincidencia entera).
func (r *Regexp) FindStringSubmatch(s string) []string {
	texto := []rune(s)
	caps := &capturas{}
	for i := 0; i <= len(texto); i++ {
		salidas := r.coincide(r.raiz, texto, i, caps)
		if len(salidas) > 0 {
			salida := make([]string, 0, 1+r.subexpresiones)
			salida = append(salida, string(texto[i:salidas[0]]))
			for g := 1; g <= r.subexpresiones; g++ {
				ini := caps.rango[(g-1)*2]
				fin := caps.rango[(g-1)*2+1]
				if ini < 0 || fin < ini {
					salida = append(salida, "")
					continue
				}
				salida = append(salida, string(texto[ini:fin]))
			}
			return salida
		}
	}
	return nil
}

// ReplaceAll: reemplaza todas las coincidencias. La sustitucion puede tener $1, $2... para los grupos.
func (r *Regexp) ReplaceAll(src, repl []byte) []byte {
	return []byte(r.ReplaceAllString(string(src), string(repl)))
}

// ReplaceAllString: la version con texto.
func (r *Regexp) ReplaceAllString(src, repl string) string {
	var sb strings.Builder
	texto := []rune(src)
	desde := 0
	caps := &capturas{}
	for desde <= len(texto) {
		encontrado := false
		for i := desde; i <= len(texto); i++ {
			caps = &capturas{}
			salidas := r.coincide(r.raiz, texto, i, caps)
			if len(salidas) > 0 {
				sb.WriteString(string(texto[desde:i]))
				sb.WriteString(r.expandir(repl, texto, i, salidas[0], caps))
				if salidas[0] > i {
					desde = salidas[0]
				} else {
					if i < len(texto) {
						sb.WriteRune(texto[i])
					}
					desde = i + 1
				}
				encontrado = true
				break
			}
		}
		if !encontrado {
			break
		}
	}
	if desde < len(texto) {
		sb.WriteString(string(texto[desde:]))
	}
	return sb.String()
}

// ReplaceAllStringFunc: reemplaza con una funcion.
func (r *Regexp) ReplaceAllStringFunc(src string, repl func(string) string) string {
	var sb strings.Builder
	texto := []rune(src)
	desde := 0
	caps := &capturas{}
	for desde <= len(texto) {
		encontrado := false
		for i := desde; i <= len(texto); i++ {
			caps = &capturas{}
			salidas := r.coincide(r.raiz, texto, i, caps)
			if len(salidas) > 0 {
				sb.WriteString(string(texto[desde:i]))
				sb.WriteString(repl(string(texto[i:salidas[0]])))
				if salidas[0] > i {
					desde = salidas[0]
				} else {
					if i < len(texto) {
						sb.WriteRune(texto[i])
					}
					desde = i + 1
				}
				encontrado = true
				break
			}
		}
		if !encontrado {
			break
		}
	}
	if desde < len(texto) {
		sb.WriteString(string(texto[desde:]))
	}
	return sb.String()
}

// Split: parte el texto por el patron.
func (r *Regexp) Split(s string, n int) []string {
	if n == 1 {
		return []string{s}
	}
	indices := r.FindAllStringIndex(s, n)
	texto := []rune(s)
	var salida []string
	desde := 0
	for _, ix := range indices {
		if n > 0 && len(salida) >= n-1 {
			break
		}
		if ix[0] == ix[1] && ix[0] == desde {
			continue
		}
		salida = append(salida, string(texto[desde:ix[0]]))
		desde = ix[1]
	}
	salida = append(salida, string(texto[desde:]))
	return salida
}

// expandir: arma el texto del reemplazo, con $1, $2, ${1} y $0.
func (r *Regexp) expandir(repl string, texto []rune, ini, fin int, caps *capturas) string {
	var sb strings.Builder
	for i := 0; i < len(repl); i++ {
		if repl[i] != '$' || i+1 >= len(repl) {
			sb.WriteByte(repl[i])
			continue
		}
		sig := repl[i+1]
		if sig >= '0' && sig <= '9' {
			g := int(sig - '0')
			if g == 0 {
				sb.WriteString(string(texto[ini:fin]))
				i++
				continue
			}
			sb.WriteString(r.grupo(caps, texto, g))
			i++
			continue
		}
		if sig == '{' {
			cierre := strings.IndexByte(repl[i:], '}')
			if cierre > 2 {
				g := parseNumero(repl[i+2 : i+cierre])
				sb.WriteString(r.grupo(caps, texto, g))
				i += cierre
				continue
			}
		}
		sb.WriteByte(repl[i])
	}
	return sb.String()
}

func (r *Regexp) grupo(caps *capturas, texto []rune, g int) string {
	if g <= 0 || g > r.subexpresiones {
		return ""
	}
	ini := caps.rango[(g-1)*2]
	fin := caps.rango[(g-1)*2+1]
	if ini < 0 || fin < ini {
		return ""
	}
	return string(texto[ini:fin])
}

// FindStringIndex ya esta arriba. Faltaban las versiones con bytes, que d2ui usa.

// Find: la primera coincidencia, en bytes.
func (r *Regexp) Find(b []byte) []byte {
	ix := r.FindIndex(b)
	if ix == nil {
		return nil
	}
	return b[ix[0]:ix[1]]
}

// FindIndex: donde empieza y termina la primera coincidencia, en bytes.
func (r *Regexp) FindIndex(b []byte) []int {
	// el indice que devuelve FindStringIndex es sobre el texto de la cadena; si hay caracteres de mas
	// de un byte, la posicion no coincide con la de los bytes, asi que se cuenta aparte.
	texto := []rune(string(b))
	caps := &capturas{}
	for i := 0; i <= len(texto); i++ {
		salidas := r.coincide(r.raiz, texto, i, caps)
		if len(salidas) > 0 {
			return []int{len(string(texto[:i])), len(string(texto[:salidas[0]]))}
		}
	}
	return nil
}

// FindAll: todas las coincidencias, en bytes.
func (r *Regexp) FindAll(b []byte, n int) [][]byte {
	texto := []rune(string(b))
	var salida [][]byte
	desde := 0
	for desde <= len(texto) {
		caps := &capturas{}
		encontrado := false
		for i := desde; i <= len(texto); i++ {
			salidas := r.coincide(r.raiz, texto, i, caps)
			if len(salidas) > 0 {
				ini := len(string(texto[:i]))
				fin := len(string(texto[:salidas[0]]))
				salida = append(salida, b[ini:fin])
				if salidas[0] > i {
					desde = salidas[0]
				} else {
					desde = i + 1
				}
				encontrado = true
				break
			}
		}
		if !encontrado {
			break
		}
		if n > 0 && len(salida) >= n {
			break
		}
	}
	return salida
}

// FindAllIndex: todas las coincidencias, con posiciones.
func (r *Regexp) FindAllIndex(b []byte, n int) [][]int {
	texto := []rune(string(b))
	var salida [][]int
	desde := 0
	for desde <= len(texto) {
		caps := &capturas{}
		encontrado := false
		for i := desde; i <= len(texto); i++ {
			salidas := r.coincide(r.raiz, texto, i, caps)
			if len(salidas) > 0 {
				salida = append(salida, []int{len(string(texto[:i])), len(string(texto[:salidas[0]]))})
				if salidas[0] > i {
					desde = salidas[0]
				} else {
					desde = i + 1
				}
				encontrado = true
				break
			}
		}
		if !encontrado {
			break
		}
		if n > 0 && len(salida) >= n {
			break
		}
	}
	return salida
}
