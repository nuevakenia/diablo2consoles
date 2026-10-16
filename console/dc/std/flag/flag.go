// Package flag: el parseo de banderas de linea de comandos, con la parte de Go que el motor usa.
//
// Por que existe: en sh-elf no hay biblioteca estandar de Go. Y este paquete NO es de adorno: el
// arranque del motor lo usa de verdad (d2app/app.go:224-248: flag.String, flag.Bool, flag.Int,
// flag.Usage, flag.PrintDefaults y flag.Parse), y las herramientas de cmd/ usan ademas StringVar,
// BoolVar, Duration, Arg, NArg, Args.
//
// Se escribe portando el flag de Go 1.27 linea por linea, y la prueba diferencial lo compara con el
// de verdad (console/dc/std/difftest/dc0025b_test.go): la misma lista de banderas y el mismo argv
// tienen que dar el mismo valor en cada bandera, los mismos argumentos que quedan, el MISMO TEXTO de
// error y el MISMO texto de PrintDefaults.
//
// LO UNICO QUE NO ES IGUAL A GO, y esta declarado aca y en el README:
//
//  1. Este paquete NO importa os NI time. En Go, flag saca los argumentos de os.Args y termina el
//     proceso con os.Exit, y flag.Duration usa time.ParseDuration. En la consola, d2std/os habla con
//     el VFS de KOS y d2std/time con el reloj de la RTC: los dos son paquetes que NO compilan en el
//     host, y si flag los importara, flag dejaria de poder compararse contra el flag de Go (que es el
//     unico juez honesto que hay). Por eso los dos enganches son VARIABLES:
//
//     - Argumentos: los argumentos de linea de comandos, o sea lo que en Go es os.Args[1:]. Los
//       engancha el arranque (en la consola, desde d2std/os; en la prueba, el propio test).
//     - Salir: quien termina el proceso (os.Exit en Go). Si el manejo es ExitOnError y no esta
//       enganchado, se avisa con panic en vez de seguir como si nada: un programa no puede quedarse
//       creyendo que termino.
//
//     flag.Duration y flag.DurationVar NO estan (pedirian time.ParseDuration): el motor las usa solo
//     en cmd/od2server y cmd/od2mpbot, que no son el binario del juego.
//
//  2. Los conjuntos de banderas (formales y ya puestas) son listas ordenadas por nombre por insertion
//     sort, no mapas: el orden es el mismo que el de Go (que los ordena por nombre en sortFlags) y el
//     paquete no necesita mapas.
//
//  3. isZeroValue (que en Go arma el valor cero del tipo del Value con reflect) es un type switch
//     sobre los tipos de valor de aca. Para un Value propio (flag.Var) se omite el (default ...) solo
//     cuando DefValue es vacio: sin reflect no hay forma de armar el valor cero de un tipo ajeno.
//
//  4. La salida por defecto (f.Output()) es io.Discard y no os.Stderr, por lo mismo del punto 1: el
//     que arranca el programa tiene que llamar a SetOutput (en la consola, a la consola serie).
package flag

import (
	"d2std/errors"
	"d2std/io"
	"d2std/strconv"
	"d2std/strings"
)

// ErrorHandling define como se comporta un FlagSet si el parseo falla.
type ErrorHandling int

const (
	ContinueOnError ErrorHandling = iota
	ExitOnError
	PanicOnError
)

// ---- los dos enganches (ver la cabecera: lo unico que no es igual a Go) ----

// Argumentos son los argumentos de linea de comandos SIN el nombre del programa: exactamente lo que
// en Go es os.Args[1:]. Los engancha el arranque del programa (o la prueba del host).
var Argumentos []string

// Salir termina el proceso con el codigo dado (os.Exit en Go). Lo engancha el arranque.
var Salir func(codigo int)

var (
	errParse = errors.New("parse error")
	errRange = errors.New("value out of range")
)

// ErrHelp es el error que devuelve Parse cuando se pidio -help o -h y esa bandera no esta definida.
var ErrHelp = errors.New("flag: help requested")

// A Value es el contrato de una bandera (lo que se registra con Var).
type Value interface {
	String() string
	Set(string) error
}

// boolFlag es el contrato de las banderas que no necesitan argumento ("-v" a secas), igual que en Go.
type boolFlag interface {
	Value
	IsBoolFlag() bool
}

// A Flag es el estado de una bandera.
type Flag struct {
	Name     string // nombre tal cual aparece en la linea de comandos
	Usage    string // texto de ayuda
	Value    Value  // valor de la bandera
	DefValue string // valor por defecto (como texto), para la ayuda
}

// ---- los tipos de valor (el mismo juego que usa el motor, mas los de una linea) ----

// -- bool

type boolValue bool

func newBoolValue(val bool, p *bool) *boolValue { *p = val; return (*boolValue)(p) }

func (b *boolValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		err = errParse
	}
	*b = boolValue(v)
	return err
}

func (b *boolValue) Get() interface{} { return bool(*b) }

func (b *boolValue) String() string { return strconv.FormatBool(bool(*b)) }

func (b *boolValue) IsBoolFlag() bool { return true }

// -- int

type intValue int

func newIntValue(val int, p *int) *intValue { *p = val; return (*intValue)(p) }

func (i *intValue) Set(s string) error {
	// bitSize 0 es "el ancho de int de la plataforma" (32 en la consola), igual que Atoi
	v, err := strconv.ParseInt(s, 0, 0)
	if err != nil {
		err = numError(err)
	}
	*i = intValue(v)
	return err
}

func (i *intValue) Get() interface{} { return int(*i) }

func (i *intValue) String() string { return strconv.Itoa(int(*i)) }

// -- int64

type int64Value int64

func newInt64Value(val int64, p *int64) *int64Value { *p = val; return (*int64Value)(p) }

func (i *int64Value) Set(s string) error {
	v, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		err = numError(err)
	}
	*i = int64Value(v)
	return err
}

func (i *int64Value) Get() interface{} { return int64(*i) }

func (i *int64Value) String() string { return strconv.FormatInt(int64(*i), 10) }

// -- uint

type uintValue uint

func newUintValue(val uint, p *uint) *uintValue { *p = val; return (*uintValue)(p) }

func (i *uintValue) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 0)
	if err != nil {
		err = numError(err)
	}
	*i = uintValue(v)
	return err
}

func (i *uintValue) Get() interface{} { return uint(*i) }

func (i *uintValue) String() string { return strconv.FormatUint(uint64(*i), 10) }

// -- float64

type float64Value float64

func newFloat64Value(val float64, p *float64) *float64Value { *p = val; return (*float64Value)(p) }

func (f *float64Value) Set(s string) error {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		err = numError(err)
	}
	*f = float64Value(v)
	return err
}

func (f *float64Value) Get() interface{} { return float64(*f) }

func (f *float64Value) String() string { return strconv.FormatFloat(float64(*f), 'g', -1, 64) }

// -- string

type stringValue string

func newStringValue(val string, p *string) *stringValue { *p = val; return (*stringValue)(p) }

func (s *stringValue) Set(val string) error { *s = stringValue(val); return nil }

func (s *stringValue) Get() interface{} { return string(*s) }

func (s *stringValue) String() string { return string(*s) }

// numError: el mismo recorte que hace Go con los errores de strconv ("parse error" / "value out of
// range" en vez del texto completo de strconv), porque ese texto sale en el mensaje de la bandera.
func numError(err error) error {
	ne, ok := err.(*strconv.NumError)
	if !ok {
		return err
	}
	if ne.Err == strconv.ErrSyntax {
		return errParse
	}
	if ne.Err == strconv.ErrRange {
		return errRange
	}
	return err
}

// ---- FlagSet ----

// A FlagSet representa un conjunto de banderas ya definidas.
type FlagSet struct {
	// Usage es la funcion que se llama cuando hay un error de parseo. Es una variable para que el
	// programa la reemplace (el motor lo hace: d2app/app.go:232).
	Usage func()

	name          string
	formales      []*Flag
	puestas       []*Flag
	args          []string
	errorHandling ErrorHandling
	output        io.Writer
	parsed        bool
}

// NewFlagSet devuelve un conjunto vacio con el nombre y el manejo de errores dados.
func NewFlagSet(name string, errorHandling ErrorHandling) *FlagSet {
	f := &FlagSet{errorHandling: errorHandling}
	f.name = name
	f.Usage = f.defaultUsage
	return f
}

func (f *FlagSet) Name() string { return f.name }

func (f *FlagSet) ErrorHandling() ErrorHandling { return f.errorHandling }

// Init fija el nombre y el manejo de errores (el conjunto cero queda sin nombre y ContinueOnError).
func (f *FlagSet) Init(name string, errorHandling ErrorHandling) {
	f.name = name
	f.errorHandling = errorHandling
}

// Output devuelve a donde van los mensajes. Sin SetOutput, a io.Discard (en Go es os.Stderr: ver la
// cabecera del paquete).
func (f *FlagSet) Output() io.Writer {
	if f.output == nil {
		return io.Discard
	}
	return f.output
}

// SetOutput fija a donde van los mensajes de ayuda y de error.
func (f *FlagSet) SetOutput(output io.Writer) { f.output = output }

// ordenarPorNombre: insertion sort, para que VisitAll y Visit salgan en orden alfabetico igual que en
// Go y el paquete no dependa de sort (ni de su lectura de descriptores de tipo).
func ordenarPorNombre(v []*Flag) []*Flag {
	copia := make([]*Flag, len(v))
	copy(copia, v)
	for i := 1; i < len(copia); i++ {
		for j := i; j > 0 && copia[j].Name < copia[j-1].Name; j-- {
			copia[j], copia[j-1] = copia[j-1], copia[j]
		}
	}
	return copia
}

// VisitAll recorre las banderas en orden alfabetico (todas, tambien las que no se pusieron).
func (f *FlagSet) VisitAll(fn func(*Flag)) {
	for _, fl := range ordenarPorNombre(f.formales) {
		fn(fl)
	}
}

// Visit recorre solo las banderas que se pusieron.
func (f *FlagSet) Visit(fn func(*Flag)) {
	for _, fl := range ordenarPorNombre(f.puestas) {
		fn(fl)
	}
}

// Lookup devuelve la bandera con ese nombre, o nil.
func (f *FlagSet) Lookup(name string) *Flag {
	for _, fl := range f.formales {
		if fl.Name == name {
			return fl
		}
	}
	return nil
}

// NFlag es cuantas banderas se pusieron.
func (f *FlagSet) NFlag() int { return len(f.puestas) }

// Arg es el i-esimo argumento que quedo despues del parseo.
func (f *FlagSet) Arg(i int) string {
	if i < 0 || i >= len(f.args) {
		return ""
	}
	return f.args[i]
}

// NArg es cuantos argumentos quedaron.
func (f *FlagSet) NArg() int { return len(f.args) }

// Args son los argumentos que quedaron despues del parseo.
func (f *FlagSet) Args() []string { return f.args }

// Parsed dice si ya se llamo a Parse.
func (f *FlagSet) Parsed() bool { return f.parsed }

// ---- definir banderas ----

// Var define una bandera con un Value propio. Si el nombre ya existe, entra en panic (como Go).
func (f *FlagSet) Var(value Value, name string, usage string) {
	if f.Lookup(name) != nil {
		panic("flag redefined: " + name)
	}
	fl := &Flag{Name: name, Usage: usage, Value: value, DefValue: value.String()}
	f.formales = append(f.formales, fl)
}

// -- bool

func (f *FlagSet) BoolVar(p *bool, name string, value bool, usage string) {
	f.Var(newBoolValue(value, p), name, usage)
}

func (f *FlagSet) Bool(name string, value bool, usage string) *bool {
	p := new(bool)
	f.BoolVar(p, name, value, usage)
	return p
}

// -- int

func (f *FlagSet) IntVar(p *int, name string, value int, usage string) {
	f.Var(newIntValue(value, p), name, usage)
}

func (f *FlagSet) Int(name string, value int, usage string) *int {
	p := new(int)
	f.IntVar(p, name, value, usage)
	return p
}

// -- int64

func (f *FlagSet) Int64Var(p *int64, name string, value int64, usage string) {
	f.Var(newInt64Value(value, p), name, usage)
}

func (f *FlagSet) Int64(name string, value int64, usage string) *int64 {
	p := new(int64)
	f.Int64Var(p, name, value, usage)
	return p
}

// -- uint

func (f *FlagSet) UintVar(p *uint, name string, value uint, usage string) {
	f.Var(newUintValue(value, p), name, usage)
}

func (f *FlagSet) Uint(name string, value uint, usage string) *uint {
	p := new(uint)
	f.UintVar(p, name, value, usage)
	return p
}

// -- float64

func (f *FlagSet) Float64Var(p *float64, name string, value float64, usage string) {
	f.Var(newFloat64Value(value, p), name, usage)
}

func (f *FlagSet) Float64(name string, value float64, usage string) *float64 {
	p := new(float64)
	f.Float64Var(p, name, value, usage)
	return p
}

// -- string

func (f *FlagSet) StringVar(p *string, name string, value string, usage string) {
	f.Var(newStringValue(value, p), name, usage)
}

func (f *FlagSet) String(name string, value string, usage string) *string {
	p := new(string)
	f.StringVar(p, name, value, usage)
	return p
}

// Set fija el valor de una bandera por nombre (lo usa el motor para las de configuracion).
func (f *FlagSet) Set(name, value string) error {
	fl := f.Lookup(name)
	if fl == nil {
		return errors.New("no such flag -" + name)
	}
	if err := fl.Value.Set(value); err != nil {
		return err
	}
	f.marcarPuesta(fl)
	return nil
}

func (f *FlagSet) marcarPuesta(fl *Flag) {
	for i, x := range f.puestas {
		if x.Name == fl.Name {
			f.puestas[i] = fl
			return
		}
	}
	f.puestas = append(f.puestas, fl)
}

// ---- la salida de ayuda ----

// UnquoteUsage saca el nombre entre acentos graves del texto de ayuda y devuelve el nombre y el texto
// sin los acentos. Sin acentos, el nombre sale del TIPO del valor (int, string, float, uint) y vacio
// para las booleanas.
func UnquoteUsage(fl *Flag) (name string, usage string) {
	usage = fl.Usage
	for i := 0; i < len(usage); i++ {
		if usage[i] == '`' {
			for j := i + 1; j < len(usage); j++ {
				if usage[j] == '`' {
					name = usage[i+1 : j]
					usage = usage[:i] + name + usage[j+1:]
					return name, usage
				}
			}
			break // un solo acento grave: se usa el nombre del tipo
		}
	}
	name = "value"
	switch fv := fl.Value.(type) {
	case boolFlag:
		if fv.IsBoolFlag() {
			name = ""
		}
	case *float64Value:
		name = "float"
	case *intValue, *int64Value:
		name = "int"
	case *stringValue:
		name = "string"
	case *uintValue:
		name = "uint"
	}
	return
}

// esValorCero: si el texto del valor por defecto es el del valor cero del tipo de la bandera (Go lo
// mira con reflect; aca es un type switch, ver el punto 3 de la cabecera).
func esValorCero(fl *Flag, def string) bool {
	switch fl.Value.(type) {
	case *boolValue:
		return def == "false"
	case *intValue, *int64Value, *uintValue, *float64Value:
		return def == "0"
	case *stringValue:
		return def == ""
	}
	return def == ""
}

// PrintDefaults imprime el texto de ayuda de todas las banderas, con el MISMO formato que Go.
func (f *FlagSet) PrintDefaults() {
	f.VisitAll(func(fl *Flag) {
		var b strings.Builder
		b.WriteString("  -")
		b.WriteString(fl.Name)
		name, usage := UnquoteUsage(fl)
		if len(name) > 0 {
			b.WriteString(" ")
			b.WriteString(name)
		}
		// Una booleana de una sola letra es tan comun que la ayuda va en la misma linea.
		if b.Len() <= 4 { // espacio, espacio, '-', 'x'
			b.WriteString("\t")
		} else {
			b.WriteString("\n    \t")
		}
		b.WriteString(strings.ReplaceAll(usage, "\n", "\n    \t"))
		if !esValorCero(fl, fl.DefValue) {
			if _, ok := fl.Value.(*stringValue); ok {
				b.WriteString(" (default ")
				b.WriteString(strconv.Quote(fl.DefValue))
				b.WriteString(")")
			} else {
				b.WriteString(" (default ")
				b.WriteString(fl.DefValue)
				b.WriteString(")")
			}
		}
		b.WriteString("\n")
		io.WriteString(f.Output(), b.String())
	})
}

func (f *FlagSet) defaultUsage() {
	if f.name == "" {
		io.WriteString(f.Output(), "Usage:\n")
	} else {
		io.WriteString(f.Output(), "Usage of "+f.name+":\n")
	}
	f.PrintDefaults()
}

// failf imprime el mensaje en la salida (como Go), llama a la ayuda y lo devuelve como error.
func (f *FlagSet) failf(msg string) error {
	io.WriteString(f.Output(), msg+"\n")
	f.usage()
	return errors.New(msg)
}

func (f *FlagSet) usage() {
	if f.Usage == nil {
		f.defaultUsage()
	} else {
		f.Usage()
	}
}

// ---- el parseo ----

func (f *FlagSet) parseOne() (bool, error) {
	if len(f.args) == 0 {
		return false, nil
	}
	s := f.args[0]
	if len(s) < 2 || s[0] != '-' {
		return false, nil
	}
	numMinuses := 1
	if s[1] == '-' {
		numMinuses++
		if len(s) == 2 { // "--" termina las banderas
			f.args = f.args[1:]
			return false, nil
		}
	}
	name := s[numMinuses:]
	if len(name) == 0 || name[0] == '-' || name[0] == '=' {
		return false, f.failf("bad flag syntax: " + s)
	}

	// es una bandera: tiene argumento?
	f.args = f.args[1:]
	hasValue := false
	value := ""
	for i := 1; i < len(name); i++ { // el '=' no puede ir primero
		if name[i] == '=' {
			value = name[i+1:]
			hasValue = true
			name = name[0:i]
			break
		}
	}

	fl := f.Lookup(name)
	if fl == nil {
		if name == "help" || name == "h" {
			f.usage()
			return false, ErrHelp
		}
		return false, f.failf("flag provided but not defined: -" + name)
	}

	if fv, ok := fl.Value.(boolFlag); ok && fv.IsBoolFlag() { // no necesita argumento
		if hasValue {
			if err := fv.Set(value); err != nil {
				return false, f.failf("invalid boolean value " + strconv.Quote(value) + " for -" + name + ": " + err.Error())
			}
		} else {
			if err := fv.Set("true"); err != nil {
				return false, f.failf("invalid boolean flag " + name + ": " + err.Error())
			}
		}
	} else {
		// tiene que llevar valor, que puede ser el argumento que sigue
		if !hasValue && len(f.args) > 0 {
			hasValue = true
			value, f.args = f.args[0], f.args[1:]
		}
		if !hasValue {
			return false, f.failf("flag needs an argument: -" + name)
		}
		if err := fl.Value.Set(value); err != nil {
			return false, f.failf("invalid value " + strconv.Quote(value) + " for flag -" + name + ": " + err.Error())
		}
	}
	f.marcarPuesta(fl)
	return true, nil
}

// Parse parsea las banderas de la lista dada (que NO incluye el nombre del programa).
func (f *FlagSet) Parse(arguments []string) error {
	f.parsed = true
	f.args = arguments
	for {
		seen, err := f.parseOne()
		if seen {
			continue
		}
		if err == nil {
			break
		}
		switch f.errorHandling {
		case ContinueOnError:
			return err
		case ExitOnError:
			if err == ErrHelp {
				f.salir(0)
			}
			f.salir(2)
		case PanicOnError:
			panic(err)
		}
	}
	return nil
}

// salir llama al enganche de Salida. Sin enganche avisa con panic: seguir como si el proceso hubiera
// terminado seria mentir (en Go aca no se vuelve nunca).
func (f *FlagSet) salir(codigo int) {
	if Salir == nil {
		panic("flag: ExitOnError sin Salir enganchado (ver la cabecera del paquete)")
	}
	Salir(codigo)
}

// ---- el conjunto global (lo que usa el motor con flag.String y flag.Parse) ----

// CommandLine es el conjunto global, el que usan las funciones de arriba (los argumentos salen de
// Argumentos y el manejo de errores es ExitOnError, como en Go).
var CommandLine = NewFlagSet("", ExitOnError)

// Usage es la funcion de ayuda del conjunto global (el motor la reemplaza).
var Usage = func() {
	f := CommandLine
	if f.name == "" {
		io.WriteString(f.Output(), "Usage:\n")
	} else {
		io.WriteString(f.Output(), "Usage of "+f.name+":\n")
	}
	f.PrintDefaults()
}

// Parse parsea Argumentos con el conjunto global.
func Parse() { CommandLine.Parse(Argumentos) }

// Parsed dice si el conjunto global ya se parseo.
func Parsed() bool { return CommandLine.Parsed() }

func VisitAll(fn func(*Flag)) { CommandLine.VisitAll(fn) }

func Visit(fn func(*Flag)) { CommandLine.Visit(fn) }

func Lookup(name string) *Flag { return CommandLine.Lookup(name) }

func Set(name, value string) error { return CommandLine.Set(name, value) }

func PrintDefaults() { CommandLine.PrintDefaults() }

func NFlag() int { return CommandLine.NFlag() }

func Arg(i int) string { return CommandLine.Arg(i) }

func NArg() int { return CommandLine.NArg() }

func Args() []string { return CommandLine.Args() }

func BoolVar(p *bool, name string, value bool, usage string) {
	CommandLine.BoolVar(p, name, value, usage)
}

func Bool(name string, value bool, usage string) *bool {
	return CommandLine.Bool(name, value, usage)
}

func IntVar(p *int, name string, value int, usage string) {
	CommandLine.IntVar(p, name, value, usage)
}

func Int(name string, value int, usage string) *int {
	return CommandLine.Int(name, value, usage)
}

func Int64Var(p *int64, name string, value int64, usage string) {
	CommandLine.Int64Var(p, name, value, usage)
}

func Int64(name string, value int64, usage string) *int64 {
	return CommandLine.Int64(name, value, usage)
}

func UintVar(p *uint, name string, value uint, usage string) {
	CommandLine.UintVar(p, name, value, usage)
}

func Uint(name string, value uint, usage string) *uint {
	return CommandLine.Uint(name, value, usage)
}

func Float64Var(p *float64, name string, value float64, usage string) {
	CommandLine.Float64Var(p, name, value, usage)
}

func Float64(name string, value float64, usage string) *float64 {
	return CommandLine.Float64(name, value, usage)
}

func StringVar(p *string, name string, value string, usage string) {
	CommandLine.StringVar(p, name, value, usage)
}

func String(name string, value string, usage string) *string {
	return CommandLine.String(name, value, usage)
}

func Var(value Value, name string, usage string) {
	CommandLine.Var(value, name, usage)
}
