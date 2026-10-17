package d2config

// DC-002.18: LA CONFIGURACION SE ESCRIBE Y SE LEE A MANO (sin reflexion).
//
// POR QUE EXISTE ESTE ARCHIVO: la Dreamcast no tiene reflexion (reflect son 43 paquetes de la biblioteca
// estandar y no estan portados), asi que el encoding/json de la consola solo puede serializar los tipos
// que saben hacerlo ellos mismos (MarshalJSON/UnmarshalJSON). Medido en DC-002.18: el arranque del motor
// llegaba a `d2app.Create().Run()` y se caia en `d2config.Save()` con
//
//	json: en la consola no hay reflexion: solo se pueden serializar los tipos que saben hacerlo ellos
//	mismos (MarshalJSON/UnmarshalJSON)
//
// porque `Configuration` no tenia las dos formas. Este archivo se las da, y ninguna de las dos necesita
// reflexion.
//
// LA REFERENCIA DE LOS BYTES ES LA BIBLIOTECA DE VERDAD: este mismo archivo se compila en el host (con el
// json de Go) y el difftest exige que la salida sea byte a byte igual a la de json.MarshalIndent de Go
// sobre la misma configuracion. En la consola se vuelve a medir contra los bytes que horneo el host
// (console/dc/poc/json-config). O sea: el config.json de la consola y el del host son EL MISMO ARCHIVO.
//
// LO QUE SE DECLARA (limites medidos, no escondidos):
//   - los MENSAJES de error no son los de Go (son texto de diagnostico; nadie los compara).
//   - el emparejamiento de claves es EXACTO, en mayusculas y minusculas como las escribe este archivo. Go,
//     cuando no encuentra la clave, la busca sin distinguir mayusculas: eso no esta (los nombres los
//     escribe este mismo archivo).
//   - un numero con fraccion en un campo entero es un error (como en Go), pero el texto del error no es el
//     de Go.
//   - NaN e Infinito no se pueden escribir (como en Go, que devuelve "json: unsupported value: NaN").

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Los dos errores de este archivo. El texto es NUESTRO (limite declarado): lo que importa es que la
// operacion falle y diga donde, no que el mensaje coincida con el de la biblioteca de Go.
var (
	errValorNoSoportado = errors.New("json: no se puede escribir un NaN ni un Infinito")
	errJSONMalo         = errors.New("json: el texto no tiene la forma de un config.json")
)

// MarshalJSON escribe la configuracion como la escribe la biblioteca de verdad: los mismos campos, en el
// orden en que estan declarados, con los mismos tipos, con `omitempty` donde la struct lo pide y con las
// claves de los mapas en orden alfabetico (lo que hace Go).
//
// Sale COMPACTO a proposito: la sangria la agrega MarshalIndent, que reacomoda el TEXTO (ver el Indent del
// json de la consola). Asi el archivo que escribe la consola queda igual al que escribe el host.
func (c *Configuration) MarshalJSON() ([]byte, error) {
	e := &escritorJSON{}

	previa := e.abrir('{')

	e.campoLista("MpqLoadOrder", c.MpqLoadOrder)
	e.campoCadena("MpqPath", c.MpqPath)
	e.campoEntero("TicksPerSecond", c.TicksPerSecond)
	e.campoEntero("FpsCap", c.FpsCap)

	if err := e.campoFlotante("SfxVolume", c.SfxVolume); err != nil {
		return nil, err
	}

	if err := e.campoFlotante("BgmVolume", c.BgmVolume); err != nil {
		return nil, err
	}

	e.campoBool("FullScreen", c.FullScreen)
	e.campoBool("RunInBackground", c.RunInBackground)
	e.campoBool("VsyncEnabled", c.VsyncEnabled)
	e.campoCadena("Backend", c.Backend)

	previaDisplay := e.claveObjeto("Display")
	e.campoEntero("Width", c.Display.Width)
	e.campoEntero("Height", c.Display.Height)
	e.cerrar('}', previaDisplay)

	e.campoEntero("WindowScale", c.WindowScale)
	e.campoCadena("D2SDir", c.D2SDir)

	if c.Language != "" { // `json:",omitempty"`
		e.campoCadena("Language", c.Language)
	}

	if len(c.Options) > 0 { // `json:",omitempty"`
		previaOpciones := e.claveObjeto("Options")

		for _, k := range clavesDeInt(c.Options) {
			e.claveCadena(k)
			e.entero(c.Options[k])
		}

		e.cerrar('}', previaOpciones)
	}

	if len(c.KeyBindings) > 0 { // `json:",omitempty"`
		previaTeclas := e.claveObjeto("KeyBindings")

		for _, k := range clavesDeLista(c.KeyBindings) {
			e.claveCadena(k)
			e.lista(c.KeyBindings[k])
		}

		e.cerrar('}', previaTeclas)
	}

	e.cerrar('}', previa)

	return e.b, nil
}

// UnmarshalJSON lee la configuracion del texto json. Los campos que no estan quedan como estaban (lo mismo
// que hace Go) y las claves que no se conocen se saltean.
func (c *Configuration) UnmarshalJSON(datos []byte) error {
	l := &lectorJSON{datos: datos}

	return l.configuracion(c)
}

// ---------------------------------------------------------------------------------------------------
// escritura
// ---------------------------------------------------------------------------------------------------

// escritorJSON arma el texto. `primero` es el estado de separacion del contenedor ABIERTO MAS ADENTRO (el
// primero no lleva coma); abrir/cerrar lo guardan y lo reponen, asi que los objetos anidados no se pisan
// el estado con el de afuera.
type escritorJSON struct {
	b       []byte
	primero bool
}

func (e *escritorJSON) abrir(c byte) bool {
	e.b = append(e.b, c)
	previa := e.primero
	e.primero = true

	return previa
}

func (e *escritorJSON) cerrar(c byte, previa bool) {
	e.b = append(e.b, c)
	e.primero = previa
}

func (e *escritorJSON) sep() {
	if !e.primero {
		e.b = append(e.b, ',')
	}

	e.primero = false
}

func (e *escritorJSON) campo(nombre string) {
	e.sep()
	e.cadena(nombre)
	e.b = append(e.b, ':')
}

func (e *escritorJSON) claveCadena(k string) {
	e.campo(k)
}

func (e *escritorJSON) claveObjeto(nombre string) bool {
	e.campo(nombre)

	return e.abrir('{')
}

func (e *escritorJSON) campoCadena(nombre, v string) {
	e.campo(nombre)
	e.cadena(v)
}

func (e *escritorJSON) campoEntero(nombre string, v int) {
	e.campo(nombre)
	e.entero(v)
}

func (e *escritorJSON) campoBool(nombre string, v bool) {
	e.campo(nombre)

	if v {
		e.b = append(e.b, "true"...)
	} else {
		e.b = append(e.b, "false"...)
	}
}

func (e *escritorJSON) campoFlotante(nombre string, v float64) error {
	e.campo(nombre)

	var err error

	e.b, err = escribirFlotante(e.b, v)

	return err
}

// campoLista escribe `"nombre":[...]` (o `null` si la lista es nil, como Go).
func (e *escritorJSON) campoLista(nombre string, v []string) {
	e.campo(nombre)
	e.lista(v)
}

func (e *escritorJSON) lista(v []string) {
	if v == nil {
		e.b = append(e.b, "null"...)

		return
	}

	previa := e.abrir('[')

	for _, s := range v {
		e.sep()
		e.cadena(s)
	}

	e.cerrar(']', previa)
}

func (e *escritorJSON) entero(v int) {
	e.b = strconv.AppendInt(e.b, int64(v), 10)
}

// cadena escribe una cadena con los escapes del json: los obligatorios (comilla, barra invertida y los
// caracteres de control) MAS los que agrega el `json.Marshal` de Go por defecto para que el texto se pueda
// meter en HTML sin sustos (`<`, `>` y `&` como \u00XX) y los separadores de linea U+2028/U+2029. Los
// bytes que no son UTF-8 valido se escriben como \ufffd (Go hace lo mismo).
func (e *escritorJSON) cadena(s string) {
	e.b = append(e.b, '"')

	for i := 0; i < len(s); {
		c := s[i]

		if c < utf8.RuneSelf {
			if c >= 0x20 && c != '"' && c != '\\' && c != '<' && c != '>' && c != '&' {
				e.b = append(e.b, c)
				i++

				continue
			}

			e.b = append(e.b, '\\')

			switch c {
			case '\\', '"':
				e.b = append(e.b, c)
			case '\b':
				e.b = append(e.b, 'b')
			case '\f':
				e.b = append(e.b, 'f')
			case '\n':
				e.b = append(e.b, 'n')
			case '\r':
				e.b = append(e.b, 'r')
			case '\t':
				e.b = append(e.b, 't')
			default:
				// \u00XX (los de control y los de HTML)
				const hex = "0123456789abcdef"
				e.b = append(e.b, 'u', '0', '0', hex[c>>4], hex[c&0xf])
			}

			i++

			continue
		}

		r, tamano := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && tamano == 1 {
			// Bytes que no son UTF-8 valido: el json de Go escribe el CARACTER de reemplazo (U+FFFD, o sea
			// los tres bytes ef bf bd) y avanza UN byte por cada uno. Medido en este host con
			// `json.Marshal("mitad \xe4\xb8 fin")` -> 22 6d 69 74 61 64 20 EF BF BD EF BF BD 20 66 69 6e 22
			// (no es el escape \ufffd de 6 caracteres: son los bytes del caracter).
			e.b = append(e.b, 0xef, 0xbf, 0xbd)
			i++

			continue
		}

		if r == '\u2028' || r == '\u2029' {
			if r == '\u2028' {
				e.b = append(e.b, "\\u2028"...)
			} else {
				e.b = append(e.b, "\\u2029"...)
			}

			i += tamano

			continue
		}

		e.b = append(e.b, s[i:i+tamano]...)
		i += tamano
	}

	e.b = append(e.b, '"')
}

// escribirFlotante es la regla del json de Go: notacion fija, salvo que el valor absoluto sea menor que
// 1e-6 o mayor o igual que 1e21, que pasa a cientifica; y en la cientifica se limpia el cero del exponente
// (e-09 -> e-9). La mantisa la da el "mas corto que se lea de vuelta igual" (prec -1) de strconv, que es
// el mismo que usa Go.
func escribirFlotante(b []byte, v float64) ([]byte, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil, errValorNoSoportado
	}

	formato := byte('f')

	if abs := math.Abs(v); abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		formato = 'e'
	}

	s := strconv.FormatFloat(v, formato, -1, 64)

	if formato == 'e' {
		n := len(s)
		if n >= 4 && s[n-4] == 'e' && s[n-3] == '-' && s[n-2] == '0' {
			s = s[:n-2] + s[n-1:]
		}
	}

	return append(b, s...), nil
}

func clavesDeInt(m map[string]int) []string {
	claves := make([]string, 0, len(m))
	for k := range m {
		claves = append(claves, k)
	}

	sort.Strings(claves)

	return claves
}

func clavesDeLista(m map[string][]string) []string {
	claves := make([]string, 0, len(m))
	for k := range m {
		claves = append(claves, k)
	}

	sort.Strings(claves)

	return claves
}

// ---------------------------------------------------------------------------------------------------
// lectura
// ---------------------------------------------------------------------------------------------------

// lectorJSON es el lector: recorre el texto una vez y va llenando la configuracion. No hay reflexion ni
// mapa de campos: las claves se comparan contra los nombres de la struct, uno por uno.
type lectorJSON struct {
	datos []byte
	i     int
}

func (l *lectorJSON) blancos() {
	for l.i < len(l.datos) {
		switch l.datos[l.i] {
		case ' ', '\t', '\n', '\r':
			l.i++
		default:
			return
		}
	}
}

func (l *lectorJSON) mirar(c byte) bool {
	return l.i < len(l.datos) && l.datos[l.i] == c
}

func (l *lectorJSON) tomar(c byte) error {
	if !l.mirar(c) {
		return errJSONMalo
	}

	l.i++

	return nil
}

func (l *lectorJSON) fin() error {
	l.blancos()

	if l.i != len(l.datos) {
		return errJSONMalo
	}

	return nil
}

// configuracion lee el objeto de arriba y va despachando por clave.
func (l *lectorJSON) configuracion(c *Configuration) error {
	l.blancos()

	if err := l.tomar('{'); err != nil {
		return err
	}

	l.blancos()

	if l.mirar('}') {
		l.i++

		return l.fin()
	}

	for {
		clave, err := l.cadena()
		if err != nil {
			return err
		}

		l.blancos()

		if err := l.tomar(':'); err != nil {
			return err
		}

		if err := l.campo(c, clave); err != nil {
			return err
		}

		l.blancos()

		switch {
		case l.mirar(','):
			l.i++
			l.blancos()
		case l.mirar('}'):
			l.i++

			return l.fin()
		default:
			return errJSONMalo
		}
	}
}

func (l *lectorJSON) campo(c *Configuration, clave string) error {
	if l.esNulo() {
		// `null`: en un valor simple (o en la struct Display) el json de Go NO cambia nada; en una lista o
		// un mapa lo deja en nil. Medido en el host: json.Unmarshal(`{"MpqPath":null}`) deja MpqPath como
		// estaba.
		switch clave {
		case "MpqLoadOrder":
			c.MpqLoadOrder = nil
		case "Options":
			c.Options = nil
		case "KeyBindings":
			c.KeyBindings = nil
		}

		return nil
	}

	var err error

	switch nombreDeCampo(clave) {
	case "MpqLoadOrder":
		c.MpqLoadOrder, err = l.listaCadenas()
	case "MpqPath":
		c.MpqPath, err = l.cadena()
	case "TicksPerSecond":
		c.TicksPerSecond, err = l.entero()
	case "FpsCap":
		c.FpsCap, err = l.entero()
	case "SfxVolume":
		c.SfxVolume, err = l.flotante()
	case "BgmVolume":
		c.BgmVolume, err = l.flotante()
	case "FullScreen":
		c.FullScreen, err = l.booleano()
	case "RunInBackground":
		c.RunInBackground, err = l.booleano()
	case "VsyncEnabled":
		c.VsyncEnabled, err = l.booleano()
	case "Backend":
		c.Backend, err = l.cadena()
	case "Display":
		err = l.display(&c.Display)
	case "WindowScale":
		c.WindowScale, err = l.entero()
	case "D2SDir":
		c.D2SDir, err = l.cadena()
	case "Language":
		c.Language, err = l.cadena()
	case "Options":
		c.Options, err = l.mapaEnteros()
	case "KeyBindings":
		c.KeyBindings, err = l.mapaListas()
	default:
		// una clave que no conocemos: se saltea el valor y se sigue (Go hace lo mismo)
		err = l.saltarValor()
	}

	return err
}

// nombresDeCampos son las claves de la struct, en el orden en que se escriben. Es la lista que usa
// nombreDeCampo para el emparejamiento sin distinguir mayusculas.
var nombresDeCampos = [...]string{
	"MpqLoadOrder", "MpqPath", "TicksPerSecond", "FpsCap", "SfxVolume", "BgmVolume",
	"FullScreen", "RunInBackground", "VsyncEnabled", "Backend", "Display", "WindowScale",
	"D2SDir", "Language", "Options", "KeyBindings",
}

// nombreDeCampo devuelve el nombre canonico de una clave: primero EXACTO y, si no, sin distinguir
// mayusculas, que es el orden de preferencia del json de Go. Medido en el host:
// json.Unmarshal(`{"mpqpath":"x"}`) llena MpqPath. "" si no es ninguna (esa clave se saltea).
func nombreDeCampo(clave string) string {
	for _, n := range nombresDeCampos {
		if clave == n {
			return n
		}
	}

	for _, n := range nombresDeCampos {
		if strings.EqualFold(clave, n) {
			return n
		}
	}

	return ""
}

func (l *lectorJSON) display(d *DisplayConfig) error {
	l.blancos()

	if err := l.tomar('{'); err != nil {
		return err
	}

	l.blancos()

	if l.mirar('}') {
		l.i++

		return nil
	}

	for {
		clave, err := l.cadena()
		if err != nil {
			return err
		}

		l.blancos()

		if err := l.tomar(':'); err != nil {
			return err
		}

		switch nombreDeCampoDisplay(clave) {
		case "Width":
			if !l.esNulo() {
				d.Width, err = l.entero()
			}
		case "Height":
			if !l.esNulo() {
				d.Height, err = l.entero()
			}
		default:
			err = l.saltarValor()
		}

		if err != nil {
			return err
		}

		l.blancos()

		switch {
		case l.mirar(','):
			l.i++
			l.blancos()
		case l.mirar('}'):
			l.i++

			return nil
		default:
			return errJSONMalo
		}
	}
}

func nombreDeCampoDisplay(clave string) string {
	for _, n := range [...]string{"Width", "Height"} {
		if clave == n {
			return n
		}
	}

	for _, n := range [...]string{"Width", "Height"} {
		if strings.EqualFold(clave, n) {
			return n
		}
	}

	return ""
}

// esNulo consume un `null` si es lo que viene (para los campos, donde `null` es "no cambies nada").
func (l *lectorJSON) esNulo() bool {
	l.blancos()

	if len(l.datos)-l.i >= 4 && string(l.datos[l.i:l.i+4]) == "null" {
		l.i += 4

		return true
	}

	return false
}

// listaCadenas lee `["a","b"]` (y tambien `null`, que Go deja como la lista vacia... no: como el valor que
// ya tenia; aca se deja nil, que es lo que hace json.Unmarshal de Go con null en un campo de slice).
func (l *lectorJSON) listaCadenas() ([]string, error) {
	l.blancos()

	if l.mirar('n') {
		if err := l.nulo(); err != nil {
			return nil, err
		}

		return nil, nil
	}

	if err := l.tomar('['); err != nil {
		return nil, err
	}

	l.blancos()

	if l.mirar(']') {
		l.i++

		return []string{}, nil
	}

	var lista []string

	for {
		s, err := l.cadena()
		if err != nil {
			return nil, err
		}

		lista = append(lista, s)

		l.blancos()

		switch {
		case l.mirar(','):
			l.i++
			l.blancos()
		case l.mirar(']'):
			l.i++

			return lista, nil
		default:
			return nil, errJSONMalo
		}
	}
}

func (l *lectorJSON) mapaEnteros() (map[string]int, error) {
	l.blancos()

	if l.mirar('n') {
		if err := l.nulo(); err != nil {
			return nil, err
		}

		return nil, nil
	}

	if err := l.tomar('{'); err != nil {
		return nil, err
	}

	l.blancos()

	m := map[string]int{}

	if l.mirar('}') {
		l.i++

		return m, nil
	}

	for {
		clave, err := l.cadena()
		if err != nil {
			return nil, err
		}

		l.blancos()

		if err := l.tomar(':'); err != nil {
			return nil, err
		}

		v, err := l.entero()
		if err != nil {
			return nil, err
		}

		m[clave] = v

		l.blancos()

		switch {
		case l.mirar(','):
			l.i++
			l.blancos()
		case l.mirar('}'):
			l.i++

			return m, nil
		default:
			return nil, errJSONMalo
		}
	}
}

func (l *lectorJSON) mapaListas() (map[string][]string, error) {
	l.blancos()

	if l.mirar('n') {
		if err := l.nulo(); err != nil {
			return nil, err
		}

		return nil, nil
	}

	if err := l.tomar('{'); err != nil {
		return nil, err
	}

	l.blancos()

	m := map[string][]string{}

	if l.mirar('}') {
		l.i++

		return m, nil
	}

	for {
		clave, err := l.cadena()
		if err != nil {
			return nil, err
		}

		l.blancos()

		if err := l.tomar(':'); err != nil {
			return nil, err
		}

		v, err := l.listaCadenas()
		if err != nil {
			return nil, err
		}

		m[clave] = v

		l.blancos()

		switch {
		case l.mirar(','):
			l.i++
			l.blancos()
		case l.mirar('}'):
			l.i++

			return m, nil
		default:
			return nil, errJSONMalo
		}
	}
}

// nulo consume un `null`.
func (l *lectorJSON) nulo() error {
	if len(l.datos)-l.i < 4 || string(l.datos[l.i:l.i+4]) != "null" {
		return errJSONMalo
	}

	l.i += 4

	return nil
}

func (l *lectorJSON) booleano() (bool, error) {
	l.blancos()

	switch {
	case len(l.datos)-l.i >= 4 && string(l.datos[l.i:l.i+4]) == "true":
		l.i += 4

		return true, nil
	case len(l.datos)-l.i >= 5 && string(l.datos[l.i:l.i+5]) == "false":
		l.i += 5

		return false, nil
	}

	return false, errJSONMalo
}

// entero lee un numero sin fraccion ni exponente (si el texto no es eso, es un error, como en Go).
func (l *lectorJSON) entero() (int, error) {
	texto, err := l.numero()
	if err != nil {
		return 0, err
	}

	v, err := strconv.Atoi(texto)
	if err != nil {
		return 0, errJSONMalo
	}

	return v, nil
}

func (l *lectorJSON) flotante() (float64, error) {
	texto, err := l.numero()
	if err != nil {
		return 0, err
	}

	v, err := strconv.ParseFloat(texto, 64)
	if err != nil {
		return 0, errJSONMalo
	}

	return v, nil
}

// numero devuelve el TEXTO del numero (como lo escribio el que lo escribio: el json no pierde precision).
func (l *lectorJSON) numero() (string, error) {
	l.blancos()

	j := l.i

	if j < len(l.datos) && l.datos[j] == '-' {
		j++
	}

	digitos := false

	for j < len(l.datos) && l.datos[j] >= '0' && l.datos[j] <= '9' {
		j++
		digitos = true
	}

	if !digitos {
		return "", errJSONMalo
	}

	if j < len(l.datos) && l.datos[j] == '.' {
		j++

		trasPunto := false

		for j < len(l.datos) && l.datos[j] >= '0' && l.datos[j] <= '9' {
			j++
			trasPunto = true
		}

		if !trasPunto {
			return "", errJSONMalo
		}
	}

	if j < len(l.datos) && (l.datos[j] == 'e' || l.datos[j] == 'E') {
		j++

		if j < len(l.datos) && (l.datos[j] == '+' || l.datos[j] == '-') {
			j++
		}

		exp := false

		for j < len(l.datos) && l.datos[j] >= '0' && l.datos[j] <= '9' {
			j++
			exp = true
		}

		if !exp {
			return "", errJSONMalo
		}
	}

	texto := string(l.datos[l.i:j])
	l.i = j

	return texto, nil
}

// saltarValor saltea cualquier valor (el que no conocemos). Los objetos y los arrays se siguen anidando.
func (l *lectorJSON) saltarValor() error {
	l.blancos()

	if l.i >= len(l.datos) {
		return errJSONMalo
	}

	switch c := l.datos[l.i]; {
	case c == '"':
		_, err := l.cadena()

		return err
	case c == '{':
		l.i++

		return l.saltarHasta('}')
	case c == '[':
		l.i++

		return l.saltarHasta(']')
	case c == 't' || c == 'f':
		_, err := l.booleano()

		return err
	case c == 'n':
		return l.nulo()
	default:
		_, err := l.numero()

		return err
	}
}

// saltarHasta saltea hasta el cierre del contenedor que se acaba de abrir (respeta las cadenas).
func (l *lectorJSON) saltarHasta(cierre byte) error {
	profundidad := 1

	for l.i < len(l.datos) {
		switch c := l.datos[l.i]; c {
		case '"':
			if _, err := l.cadena(); err != nil {
				return err
			}
		case '{', '[':
			profundidad++
			l.i++
		case '}', ']':
			profundidad--
			l.i++

			if profundidad == 0 {
				if c != cierre {
					return errJSONMalo
				}

				return nil
			}
		default:
			l.i++
		}
	}

	return errJSONMalo
}

// cadena lee una cadena json (con sus escapes) y devuelve el texto sin escapes.
func (l *lectorJSON) cadena() (string, error) {
	l.blancos()

	if l.i >= len(l.datos) || l.datos[l.i] != '"' {
		return "", errJSONMalo
	}

	l.i++

	var b []byte

	for l.i < len(l.datos) {
		c := l.datos[l.i]

		switch {
		case c == '"':
			l.i++

			return string(b), nil
		case c == '\\':
			r, err := l.escape()
			if err != nil {
				return "", err
			}

			b = utf8.AppendRune(b, r)
		case c < 0x20:
			return "", errJSONMalo
		default:
			b = append(b, c)
			l.i++
		}
	}

	return "", errJSONMalo
}

// escape resuelve un escape de cadena json (los siete cortos y \uXXXX, con los pares de sustitutos).
func (l *lectorJSON) escape() (rune, error) {
	l.i++ // la barra invertida

	if l.i >= len(l.datos) {
		return 0, errJSONMalo
	}

	c := l.datos[l.i]
	l.i++

	switch c {
	case '"', '\\', '/':
		return rune(c), nil
	case 'b':
		return '\b', nil
	case 'f':
		return '\f', nil
	case 'n':
		return '\n', nil
	case 'r':
		return '\r', nil
	case 't':
		return '\t', nil
	case 'u':
		r, err := l.hexCuatro()
		if err != nil {
			return 0, err
		}

		// par de sustitutos (lo que hace Go): alto + bajo = un caracter; un sustituto suelto es U+FFFD
		if r >= 0xd800 && r <= 0xdbff && l.i+1 < len(l.datos) && l.datos[l.i] == '\\' && l.datos[l.i+1] == 'u' {
			guarda := l.i
			l.i += 2

			bajo, err := l.hexCuatro()
			if err != nil {
				return 0, err
			}

			if bajo >= 0xdc00 && bajo <= 0xdfff {
				return 0x10000 + (r-0xd800)<<10 + (bajo - 0xdc00), nil
			}

			l.i = guarda
		}

		if r >= 0xd800 && r <= 0xdfff {
			return utf8.RuneError, nil
		}

		return r, nil
	}

	return 0, errJSONMalo
}

func (l *lectorJSON) hexCuatro() (rune, error) {
	if l.i+4 > len(l.datos) {
		return 0, errJSONMalo
	}

	var v rune

	for n := 0; n < 4; n++ {
		c := l.datos[l.i+n]

		var d rune

		switch {
		case c >= '0' && c <= '9':
			d = rune(c - '0')
		case c >= 'a' && c <= 'f':
			d = rune(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = rune(c-'A') + 10
		default:
			return 0, errJSONMalo
		}

		v = v*16 + d
	}

	l.i += 4

	return v, nil
}
