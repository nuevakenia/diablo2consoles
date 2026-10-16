// Prueba diferencial de DC-002.5b: d2std/flag contra el flag de Go y d2std/encoding/csv contra el
// encoding/csv de Go.
//
// Los dos paquetes son Go PURO (no hablan con KOS), asi que este es el juez mas fuerte que tienen:
// la misma entrada por los dos lados y la salida tiene que ser identica, TEXTO A TEXTO. Para flag se
// compara el valor de cada bandera, los argumentos que quedan, los que se pusieron, el MENSAJE DE
// ERROR y la SALIDA escrita (el mensaje mas el texto de ayuda, que es lo que ve el usuario). Para csv
// se comparan las filas, el TEXTO de los errores (que trae linea y columna), el avance de la entrada
// y los bytes que escribe el Writer.
//
// Los dos anchos de int: la corrida en 386 (scripts/console/test-std-host.sh) es la que ve el ancho
// de int de la consola (32 bits). Por eso los casos que dependen del ancho viven aca y NO en el
// corpus de la consola (que se calcula con el int de 64 bits del host): estan marcados con
// soloHost.
package difftest

import (
	realcsv "encoding/csv"
	goerrors "errors"
	goflag "flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	mybytes "d2std/bytes"
	mycsv "d2std/encoding/csv"
	myflag "d2std/flag"
)

// ---- el Value propio de las dos pruebas (el mismo tipo sirve para los dos paquetes: los dos miran
// solo String() y Set(), que es el contrato de flag.Value) ----

type acumula struct{ vals []string }

func (a *acumula) String() string { return strings.Join(a.vals, ",") }

func (a *acumula) Set(s string) error {
	if s == "error" {
		return goerrors.New("valor que no acepta")
	}
	a.vals = append(a.vals, s)
	return nil
}

// ---- flag: el mismo conjunto de banderas por los dos lados ----

// miFlag corre el argv con d2std/flag y devuelve el texto medible.
func miFlag(args []string, exitOnError bool) string {
	manejo := myflag.ContinueOnError
	if exitOnError {
		manejo = myflag.ExitOnError
	}
	fs := myflag.NewFlagSet("prueba", manejo)
	var buf strings.Builder
	fs.SetOutput(&buf)
	var a acumula
	fb := fs.Bool("b", false, "bandera b")
	fo := fs.Bool("otra", true, "bandera otra")
	fstr := fs.String("s", "def", "texto de la bandera s")
	fn := fs.Int("n", 7, "numero de la bandera n")
	fs.Var(&a, "v", "valor acumulado")
	err := fs.Parse(args)
	return textoFlag(*fb, *fo, *fstr, *fn, a.String(), err, fs.Args(), fs.NFlag(), buf.String(), fs.Parsed())
}

// goFlag hace exactamente lo mismo con el flag de Go.
func goFlag(args []string, exitOnError bool) string {
	manejo := goflag.ContinueOnError
	if exitOnError {
		manejo = goflag.ExitOnError
	}
	fs := goflag.NewFlagSet("prueba", manejo)
	var buf strings.Builder
	fs.SetOutput(&buf)
	var a acumula
	fb := fs.Bool("b", false, "bandera b")
	fo := fs.Bool("otra", true, "bandera otra")
	fstr := fs.String("s", "def", "texto de la bandera s")
	fn := fs.Int("n", 7, "numero de la bandera n")
	fs.Var(&a, "v", "valor acumulado")
	err := fs.Parse(args)
	return textoFlag(*fb, *fo, *fstr, *fn, a.String(), err, fs.Args(), fs.NFlag(), buf.String(), fs.Parsed())
}

// textoFlag: el texto que se compara. Es UNA sola funcion para los dos lados a proposito: si la
// armada fuera distinta en cada lado, se estaria comparando la armada y no las banderas.
func textoFlag(b, otra bool, s string, n int, acumulado string, err error, restantes []string, puestas int, salida string, parsed bool) string {
	errTexto := ""
	if err != nil {
		errTexto = err.Error()
	}
	return fmt.Sprintf("b=%t otra=%t s=%q n=%d v=%q err=%q args=%q puestas=%d salida=%q parsed=%t",
		b, otra, s, n, acumulado, errTexto, strings.Join(restantes, "|"), puestas, salida, parsed)
}

// argvs es la lista de lineas de comandos que se prueban. La bandera soloHost marca los casos cuyo
// resultado depende del ANCHO DE INT (en la consola el int es de 32 bits).
var argvs = []struct {
	nombre   string
	args     []string
	soloHost bool
}{
	{"sin argumentos", nil, false},
	{"lista vacia", []string{}, false},
	{"bool corto", []string{"-b"}, false},
	{"bool con igual", []string{"-b=true"}, false},
	{"bool false", []string{"-b=false"}, false},
	{"bool con 0", []string{"-b=0"}, false},
	{"bool con 1", []string{"-b=1"}, false},
	{"bool con T", []string{"-b=T"}, false},
	{"bool con dos guiones", []string{"--b"}, false},
	{"bool con valor malo", []string{"-b=quizas"}, false},
	{"string con igual", []string{"-s=hola"}, false},
	{"string en el argumento de al lado", []string{"-s", "hola mundo"}, false},
	{"string vacio", []string{"-s="}, false},
	{"string que parece bandera", []string{"-s", "-b"}, false},
	{"string con barra invertida", []string{"--s=a\\b"}, false},
	{"string con salto de linea", []string{"-s=con\nsalto"}, false},
	{"int", []string{"-n=42"}, false},
	{"int negativo", []string{"-n", "-42"}, false},
	{"int hexadecimal", []string{"-n=0x1f"}, false},
	{"int octal 0o", []string{"-n=0o17"}, false},
	{"int octal 0", []string{"-n=017"}, false},
	{"int con signo", []string{"-n=+5"}, false},
	{"int binario", []string{"-n=0b101"}, false},
	{"int malo", []string{"-n=abc"}, false},
	{"int de doce digitos (NO entra en el int de 32: en la consola es out of range)", []string{"-n=123456789012"}, true},
	{"int del tope de 32 bits", []string{"-n=2147483647"}, false},
	{"int mas grande que el tope de 32 (esto lo ve el int de 32)", []string{"-n=4294967296"}, true},
	{"int que desborda 64", []string{"-n=99999999999999999999"}, true},
	{"repetida gana la ultima", []string{"-n=1", "-n=2"}, false},
	{"var acumula", []string{"-v=a", "-v=b"}, false},
	{"var con error de Set", []string{"-v=a", "-v=error"}, false},
	{"var sin valor", []string{"-v"}, false},
	{"desconocida", []string{"-z"}, false},
	{"desconocida larga con valor", []string{"--zz=1"}, false},
	{"ayuda con h", []string{"-h"}, false},
	{"ayuda con help", []string{"--help"}, false},
	{"guion solo", []string{"-"}, false},
	{"guion guion corta", []string{"-b", "--", "-n", "3"}, false},
	{"sintaxis con tres guiones", []string{"---b"}, false},
	{"sintaxis con igual primero", []string{"-=1"}, false},
	{"posicionales", []string{"uno", "dos"}, false},
	{"posicional primero corta", []string{"uno", "-b"}, false},
	{"un negativo suelto", []string{"-1"}, false},
	{"necesita argumento", []string{"-s"}, false},
	{"mezcla con posicional en el medio", []string{"-b", "-n", "3", "arg1", "-s=x"}, false},
}

func TestFlagParseContraGo(t *testing.T) {
	probados := 0
	for _, c := range argvs {
		if c.soloHost && strconv.IntSize == 32 {
			// en 386 el int del host es el de la consola: tambien se compara (es la misma cuenta)
		}
		quiero := goFlag(c.args, false)
		tengo := miFlag(c.args, false)
		if quiero != tengo {
			t.Errorf("argv %q [%s]:\n  Go     %s\n  nuestro %s", c.args, c.nombre, quiero, tengo)
			continue
		}
		probados++
	}
	t.Logf("flag: %d/%d lineas de comandos identicas al flag de Go", probados, len(argvs))
}

// TestFlagPrintDefaults: el TEXTO de ayuda tiene que salir igual. Se prueban tres conjuntos: el del
// motor (d2app/app.go:224-234, con los textos reales) y dos de juguete con los casos raros (acentos
// graves, ayuda de varias lineas, booleanas de una letra y de mas letras, defaults no cero).
func TestFlagPrintDefaults(t *testing.T) {
	mio := func() string {
		fs := myflag.NewFlagSet("prueba", myflag.ContinueOnError)
		var buf strings.Builder
		fs.SetOutput(&buf)
		ponerBanderasDeJuguete(fs)
		fs.PrintDefaults()
		return buf.String()
	}
	suyo := func() string {
		fs := goflag.NewFlagSet("prueba", goflag.ContinueOnError)
		var buf strings.Builder
		fs.SetOutput(&buf)
		ponerBanderasDeJuguete(fs)
		fs.PrintDefaults()
		return buf.String()
	}
	quiero, tengo := suyo(), mio()
	if quiero != tengo {
		t.Errorf("PrintDefaults de juguete:\n--- Go ---\n%s\n--- nuestro ---\n%s", quiero, tengo)
	}

	// el conjunto del motor, con sus textos de verdad
	mioMotor := func() string {
		fs := myflag.NewFlagSet("prueba", myflag.ContinueOnError)
		var buf strings.Builder
		fs.SetOutput(&buf)
		ponerBanderasDelMotor(fs)
		fs.PrintDefaults()
		return buf.String()
	}
	suyoMotor := func() string {
		fs := goflag.NewFlagSet("prueba", goflag.ContinueOnError)
		var buf strings.Builder
		fs.SetOutput(&buf)
		ponerBanderasDelMotor(fs)
		fs.PrintDefaults()
		return buf.String()
	}
	quiero, tengo = suyoMotor(), mioMotor()
	if quiero != tengo {
		t.Errorf("PrintDefaults del motor:\n--- Go ---\n%s\n--- nuestro ---\n%s", quiero, tengo)
	}
	// El texto del motor tiene que traer las siete banderas. Los textos esperados NO van escritos a
	// mano: estan MEDIDOS contra la salida del flag de Go (2026-10-16, Go 1.27.1):
	//   - las booleanas de UNA letra llevan la ayuda en la MISMA linea, separadas por un tabulador
	//     ("  -v	Show version"), no en la linea de abajo;
	//   - un default que es el VALOR CERO no se imprime: no hay "(default 0)" para dedicated/players,
	//     y si hay "(default 4)" para l.
	// Se le exige a los DOS textos (el de Go y el nuestro): si Go cambia el formato algun dia, esta
	// prueba lo dice en vez de quedar comparando contra un recuerdo.
	for _, caso := range []struct {
		nombre string
		texto  string
	}{{"Go", quiero}, {"nuestro", tengo}} {
		for _, esperado := range []string{
			"  -profile string\n", "  -dedicated\n", "  -players int\n",
			"  -l int\n", "  -lang string\n", "  -v	Show version\n", "  -h	Show help\n",
			"(default 4)",
		} {
			if !strings.Contains(caso.texto, esperado) {
				t.Errorf("el texto del motor (%s) no trae %q", caso.nombre, esperado)
			}
		}
	}
	t.Logf("flag: PrintDefaults identico en 2 conjuntos (%d bytes el del motor)", len(quiero))
}

// ponerBanderasDeJuguete: los casos raros del formato de ayuda.
func ponerBanderasDeJuguete[T interface {
	String(string, string, string) *string
	Bool(string, bool, string) *bool
	Int(string, int, string) *int
	Float64(string, float64, string) *float64
	Uint(string, uint, string) *uint
}](fs T) {
	fs.Bool("b", false, "bandera corta")
	fs.Bool("verbose", false, "bandera larga")
	fs.String("archivo", "", "archivo de entrada (sin default, es el cero)")
	fs.String("ruta", "data/global", "ruta con `camino` entre acentos graves")
	fs.Int("n", 7, "un numero (default no cero)")
	fs.Int("m", 0, "un numero en cero")
	fs.Float64("f", 1.5, "un flotante")
	fs.Uint("u", 3, "un entero sin signo")
	fs.String("ayuda", "linea uno\nlinea dos\n  linea tres", "ayuda de varias lineas")
}

// ponerBanderasDelMotor: el mismo conjunto que d2app/app.go, con sus textos reales.
func ponerBanderasDelMotor[T interface {
	String(string, string, string) *string
	Bool(string, bool, string) *bool
	Int(string, int, string) *int
}](fs T) {
	const (
		descProfile = "Profiles the program,\none of (cpu, mem, block, goroutine, trace, thread, mutex)"
		descPlayers = "Sets the number of max players for the dedicated server"
		descLogging = "Enables verbose logging. Log levels will include those below it.\n" +
			" 0 disables log messages\n" +
			" 1 shows fatal\n" +
			" 2 shows error\n" +
			" 3 shows warning\n" +
			" 4 shows info\n" +
			" 5 shows debug\n"
	)
	fs.String("profile", "", descProfile)
	fs.Bool("dedicated", false, "Starts a dedicated server")
	fs.Int("players", 0, descPlayers)
	fs.Int("l", 4, descLogging)
	fs.String("lang", "", "Game language: enUS deDE esES frFR itIT jaJP koKR plPL ptBR ruRU zhCN zhTW (default: the install's language)")
	fs.Bool("v", false, "Show version")
	fs.Bool("h", false, "Show help")
}

// TestFlagExitOnError: el manejo ExitOnError de verdad. En Go el proceso TERMINA (os.Exit(2) con un
// error, os.Exit(0) con -h) y eso no se puede atajar desde adentro: hay que correr a Go en un
// subproceso y comparar su codigo de salida y lo que escribio contra lo que hace nuestro enganche.
// Es la unica forma de medir el codigo de salida y no suponerlo.
func TestFlagExitOnError(t *testing.T) {
	if modo := os.Getenv("DIFT_FLAG_HIJO"); modo != "" {
		correrHijoFlag(modo, strings.Split(os.Getenv("DIFT_FLAG_ARGS"), "\x1f"))
		return
	}
	casos := []struct {
		nombre string
		modo   string
		args   []string
	}{
		{"bandera desconocida", "flags", []string{"-z"}},
		{"valor de int malo", "flags", []string{"-n=abc"}},
		{"bandera que necesita argumento", "flags", []string{"-s"}},
		{"ayuda con h", "flags", []string{"-h"}},
		{"argv sano", "flags", []string{"-b", "-n", "3", "resto"}},
	}
	for _, c := range casos {
		cmd := exec.Command(os.Args[0], "-test.run=TestFlagExitOnError")
		cmd.Env = append(os.Environ(),
			"DIFT_FLAG_HIJO="+c.modo,
			"DIFT_FLAG_ARGS="+strings.Join(c.args, "\x1f"))
		var stderr strings.Builder
		cmd.Stderr = &stderr
		cmd.Stdout = io.Discard
		err := cmd.Run()
		codigoGo := 0
		if ee, ok := err.(*exec.ExitError); ok {
			codigoGo = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("no pude correr el subproceso: %v", err)
		}

		// nuestro lado: el enganche Salir registra el codigo y corta con un panic propio (en la
		// consola lo engancha os.Exit; aca no se puede matar la prueba). Si el parseo no falla, no
		// se llama al enganche y el codigo queda en 0, que es lo que sale del proceso de Go.
		codigoMio := 0
		func() {
			myflag.Salir = func(c int) { codigoMio = c; panic(salidaDePrueba{}) }
			defer func() { myflag.Salir = nil; recover() }()
			fs := myflag.NewFlagSet("prueba", myflag.ExitOnError)
			var buf strings.Builder
			fs.SetOutput(&buf)
			ponerBanderasDeJuguete(fs)
			fs.Parse(c.args)
		}()
		if codigoMio != codigoGo {
			t.Errorf("%s: codigo de salida Go %d, nuestro %d", c.nombre, codigoGo, codigoMio)
		}
		if codigoGo != 0 {
			if !strings.Contains(stderr.String(), "flag provided but not defined") &&
				!strings.Contains(stderr.String(), "invalid value") &&
				!strings.Contains(stderr.String(), "flag needs an argument") {
				t.Errorf("%s: el subproceso no dijo por que salio: %q", c.nombre, stderr.String())
			}
		}
	}
	t.Logf("flag: ExitOnError comparado contra el proceso de Go en %d lineas (codigo de salida de verdad)", len(casos))
}

type salidaDePrueba struct{}

// correrHijoFlag: la mitad que corre en el subproceso, con el flag de GO. No vuelve (os.Exit).
func correrHijoFlag(modo string, args []string) {
	fs := goflag.NewFlagSet("prueba", goflag.ExitOnError)
	fs.SetOutput(os.Stderr)
	ponerBanderasDeJuguete(fs)
	fs.Parse(args)
}

// TestFlagGlobal: el conjunto global (CommandLine), que es el que usa el motor con flag.String y
// flag.Parse. La diferencia con Go es de donde salen los argumentos: aca de la variable Argumentos
// (la engancha el arranque; en Go es os.Args[1:]).
func TestFlagGlobal(t *testing.T) {
	casos := []struct {
		nombre string
		args   []string
	}{
		{"global vacio", nil},
		{"global con banderas", []string{"-b", "-n", "9", "-s=hola", "--b=false", "resto"}},
		{"global con dos guiones y despues", []string{"--s=x", "uno", "dos"}},
	}
	// OJO: ningun nombre que NO este definido en estos casos. Los dos conjuntos (el de Go y el
	// nuestro) son ExitOnError, y con una bandera desconocida el proceso TERMINA: el de Go de
	// verdad con os.Exit(2) adentro de la prueba, y la suite entera se cae sin decir por que (asi
	// quedo este caso escrito a mano: con una bandera de nombre inexistente). El manejo ExitOnError
	// se prueba en TestFlagExitOnError, que corre a Go en un subproceso justamente por eso.
	for _, c := range casos {
		// Go: un conjunto nuevo con los MISMOS nombres, manejando como el global (ExitOnError) y
		// con un argv que no falla (si fallara, Go mataria la prueba)
		suyo := func() string {
			fs := goflag.NewFlagSet("", goflag.ExitOnError)
			var buf strings.Builder
			fs.SetOutput(&buf)
			banderasGlobalesGo(fs)
			fs.Parse(c.args)
			return textoGlobal(fs.Args())
		}()
		// nuestro: el global de verdad, con los argumentos puestos por la variable Argumentos
		myflag.CommandLine = myflag.NewFlagSet("", myflag.ExitOnError)
		var buf strings.Builder
		myflag.CommandLine.SetOutput(&buf)
		banderasGlobalesMio(myflag.CommandLine)
		myflag.Argumentos = c.args
		myflag.Parse()
		mio := textoGlobal(myflag.Args())
		if suyo != mio {
			t.Errorf("global %q: Go %q, nuestro %q", c.args, suyo, mio)
		}
	}
	myflag.Argumentos = nil
	t.Logf("flag: el conjunto global coincide con Go en %d lineas", len(casos))
}

// banderasGlobalesMio / banderasGlobalesGo: el MISMO conjunto de banderas por los dos lados (no se
// puede compartir el codigo porque cada paquete tiene su propio tipo Value; los nombres, los defaults
// y los textos si son los mismos).
func banderasGlobalesMio(fs *myflag.FlagSet) {
	fs.Bool("b", false, "bandera b")
	fs.String("s", "def", "texto de la bandera s")
	fs.Int("n", 7, "numero de la bandera n")
}

func banderasGlobalesGo(fs *goflag.FlagSet) {
	fs.Bool("b", false, "bandera b")
	fs.String("s", "def", "texto de la bandera s")
	fs.Int("n", 7, "numero de la bandera n")
}

func textoGlobal(restantes []string) string { return "args=" + strings.Join(restantes, "|") }

// ---- encoding/csv ----

// serialFila / serialFilas: el texto que se compara (los separadores de control no aparecen en las
// entradas del corpus). Es una sola funcion para los dos lados.
func serialFila(f []string) string { return strings.Join(f, "\x01") }

func serialFilas(filas [][]string) string {
	trozos := make([]string, len(filas))
	for i, f := range filas {
		trozos[i] = serialFila(f)
	}
	return strings.Join(trozos, "\x02")
}

func errTextoDe(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

type opcionesCSV struct {
	nombre     string
	coma       rune
	comentario rune
	campos     int
	perezoso   bool
	espacios   bool
	reusa      bool
}

// opciones: los juegos de opciones que se prueban (el del motor primero: tabulador, campos libres y
// comillas perezosas, que es como lee las tablas de texto).
var opciones = []opcionesCSV{
	{"por defecto (coma)", ',', 0, 0, false, false, false},
	{"del motor (tabulador, campos libres, perezoso)", '\t', 0, -1, true, false, false},
	{"campos fijos en 3", ',', 0, 3, false, false, false},
	{"con comentario #", ',', '#', 0, false, false, false},
	{"salteando el espacio del principio", ',', 0, 0, false, true, false},
	{"reusando la rebanada", ',', 0, 0, false, false, true},
	{"punto y coma", ';', 0, 0, false, false, false},
	{"separador invalido (la coma es el comentario)", ',', ',', 0, false, false, false},
	{"separador invalido (el comentario es una comilla)", ',', '"', 0, false, false, false},
	{"separador cero", 0, 0, 0, false, false, false},
	{"separador de mas de un byte", '\u2022', 0, 0, false, false, false},
}

// entradasCSV: las entradas que se prueban con cada juego de opciones. Los casos raros de RFC 4180
// (comillas adentro, comas adentro de comillas, campos de varias lineas) y los que el motor tiene en
// sus tablas (tabuladores, filas de largo distinto, finales de linea de Windows).
var entradasCSV = []struct {
	nombre string
	datos  string
}{
	{"vacia", ""},
	{"solo salto", "\n"},
	{"solo dos saltos", "\n\n"},
	{"una fila", "a,b,c\n"},
	{"una fila sin salto final", "a,b,c"},
	{"dos filas", "a,b,c\nd,e,f\n"},
	{"dos filas sin salto final", "a,b,c\nd,e,f"},
	{"finales de Windows", "a,b\r\nc,d\r\n"},
	{"finales de Windows sin salto final", "a,b\r\nc,d"},
	{"campo entre comillas", "a,\"b\",c\n"},
	{"coma adentro de comillas", "\"a,b\",c\n"},
	{"comilla doble adentro", "\"a\"\"b\",c\n"},
	{"campo de varias lineas", "\"multi\nlinea\",x\n"},
	{"campo de varias lineas con Windows", "\"multi\r\nlinea\",x\r\n"},
	{"varias lineas y salto final", "\"a\nb\nc\",d\n"},
	{"comilla que cierra y sigue texto", "\"a\"b,c\n"},
	{"comilla suelta en el medio", "a\"b,c\n"},
	{"comilla suelta al final", "ab\",c\n"},
	{"campo vacio entre comillas", "\"\"\n"},
	{"cuatro comillas", "\"\"\"\"\n"},
	{"comilla sola", "\"\n"},
	{"comilla sin cerrar con linea nueva", "\"a\n"},
	{"comilla sin cerrar al final", "\"a"},
	{"comilla y nada", "\""},
	{"fila de distinto largo", "a,b\nc\n"},
	{"fila de distinto largo al reves", "a\nc,d,e\n"},
	{"lineas vacias en el medio", "a,b\n\n\nd,e\n"},
	{"saltos de sobra al final", "a,b\n\n\n"},
	{"espacios alrededor", "  a , b \n"},
	{"espacios adentro del campo", "a b,c\n"},
	{"comentario", "#comentario\na,b\n"},
	{"comentario con espacio adelante", " #no comentario\na,b\n"},
	{"comentario al final", "a,b\n#chau\n"},
	{"todo comentarios", "#uno\n#dos\n"},
	{"con tabuladores", "a\tb\tc\nd\te\tf\n"},
	{"tabulador adentro de comillas", "\"a\tb\",c\n"},
	{"campo vacio al final", "a,b,\n"},
	{"fila de un separador", ",\n"},
	{"no ascii", "café,niño\n"},
	{"BOM al principio", "\ufeffa,b\n"},
	{"byte nulo adentro", "a,b\x00c\n"},
	{"campo con salto de Windows suelto", "a\rb,c\n"},
}

// El lector de NUESTRO lado: bytes.NewReader de la consola. NO se le da el strings.Reader de Go ni
// siquiera en el host, y no es un detalle: la entrada de abajo termina devolviendo io.EOF, y el
// io.EOF de la biblioteca estandar de Go NO es el io.EOF de d2std (en la consola no existe la stdlib
// de Go). Medido: con strings.NewReader el csv nuestro veia un EOF "ajeno", no lo reconocia y ReadAll
// devolvia 255 de 429 combinaciones con err=EOF donde Go da err= (vacio). En la consola el lector real
// es de la consola, asi que este es tambien el montaje fiel.
func miLector(datos string) *mybytes.Reader { return mybytes.NewReader([]byte(datos)) }

// leerMio / leerGo: la misma corrida por los dos lados. Devuelve el texto medible: las filas
// serializadas, el error y el avance de la entrada (InputOffset), que dice cuantos bytes se
// consumieron de verdad.
func leerMio(datos string, op opcionesCSV) string {
	r := mycsv.NewReader(miLector(datos))
	r.Comma, r.Comment, r.FieldsPerRecord = op.coma, op.comentario, op.campos
	r.LazyQuotes, r.TrimLeadingSpace, r.ReuseRecord = op.perezoso, op.espacios, op.reusa
	filas, err := r.ReadAll()
	return "filas=" + serialFilas(filas) + " err=" + errTextoDe(err) + " offset=" + strconv.FormatInt(r.InputOffset(), 10)
}

func leerGo(datos string, op opcionesCSV) string {
	r := realcsv.NewReader(strings.NewReader(datos))
	r.Comma, r.Comment, r.FieldsPerRecord = op.coma, op.comentario, op.campos
	r.LazyQuotes, r.TrimLeadingSpace, r.ReuseRecord = op.perezoso, op.espacios, op.reusa
	filas, err := r.ReadAll()
	return "filas=" + serialFilas(filas) + " err=" + errTextoDe(err) + " offset=" + strconv.FormatInt(r.InputOffset(), 10)
}

func TestCSVReaderContraGo(t *testing.T) {
	probados := 0
	for _, e := range entradasCSV {
		for _, op := range opciones {
			quiero, tengo := leerGo(e.datos, op), leerMio(e.datos, op)
			if quiero != tengo {
				t.Errorf("entrada %q [%s] con %s:\n  Go     %s\n  nuestro %s", e.datos, e.nombre, op.nombre, quiero, tengo)
				continue
			}
			probados++
		}
	}
	t.Logf("csv Reader: %d/%d combinaciones identicas a Go", probados, len(entradasCSV)*len(opciones))
}

// TestCSVReaderDeAUno: Read() registro por registro, con FieldPos y el avance. ReadAll puede tapar un
// error de "cuando corto el registro"; leer de a uno y mirar la posicion lo muestra.
func TestCSVReaderDeAUno(t *testing.T) {
	entradas := []string{
		"a,b,c\nd,\"e,f\",g\nh,i,j\n",
		"\"1\n2\",3\r\n4,5\r\n",
		"a,b\nc\n",
		"\"sin cierre\n",
		"a,b\"c,d\n",
	}
	probados := 0
	for _, datos := range entradas {
		for _, perezoso := range []bool{false, true} {
			posiciones := func(r interface {
				Read() ([]string, error)
				FieldPos(int) (int, int)
			}) string {
				var trozos []string
				for {
					fila, err := r.Read()
					if err != nil {
						trozos = append(trozos, "err="+errTextoDe(err))
						break
					}
					pos := ""
					for i := range fila {
						if i > 0 {
							pos += ";"
						}
						l, c := r.FieldPos(i)
						pos += strconv.Itoa(l) + ":" + strconv.Itoa(c)
					}
					trozos = append(trozos, serialFila(fila)+"@"+pos)
				}
				return strings.Join(trozos, " ")
			}
			rm := mycsv.NewReader(miLector(datos))
			rm.LazyQuotes = perezoso
			rg := realcsv.NewReader(strings.NewReader(datos))
			rg.LazyQuotes = perezoso
			quiero, tengo := posiciones(rg), posiciones(rm)
			if quiero != tengo {
				t.Errorf("de a uno %q (perezoso=%t):\n  Go     %s\n  nuestro %s", datos, perezoso, quiero, tengo)
				continue
			}
			probados++
		}
	}
	t.Logf("csv Reader de a uno: %d/%d identicas (filas, posiciones y error)", probados, len(entradas)*2)
}

// TestCSVReuseRecord: con ReuseRecord la rebanada que devuelve Read se comparte entre llamadas (los
// textos de adentro son nuevos, la rebanada no). Se mide igual que se puede medir: si el primer
// elemento de dos lecturas seguidas cae en la MISMA direccion.
func TestCSVReuseRecord(t *testing.T) {
	datos := "aa,bb\ncc,dd\nee,ff\n"
	comparte := func(filas [][]string) bool { return len(filas) >= 2 && &filas[0][0] == &filas[1][0] }
	leer := func(f func() ([]string, error)) [][]string {
		var out [][]string
		for i := 0; i < 3; i++ {
			fila, err := f()
			if err != nil {
				break
			}
			out = append(out, fila)
		}
		return out
	}
	rg := realcsv.NewReader(strings.NewReader(datos))
	rg.ReuseRecord = true
	rm := mycsv.NewReader(miLector(datos))
	rm.ReuseRecord = true
	goComparte := comparte(leer(rg.Read))
	mioComparte := comparte(leer(rm.Read))
	if goComparte != mioComparte {
		t.Errorf("ReuseRecord comparte la rebanada: Go %t, nuestro %t", goComparte, mioComparte)
	}
	t.Logf("csv: ReuseRecord comparte la rebanada en los dos (Go=%t, nuestro=%t)", goComparte, mioComparte)
}

// TestCSVWriterContraGo: los bytes escritos tienen que ser iguales, y tambien el error cuando el
// separador no sirve. Se escribe en un bytes.Buffer de Go en los dos lados (lo que se compara es lo
// que produce el Writer, no el buffer).
func TestCSVWriterContraGo(t *testing.T) {
	casos := []struct {
		nombre string
		filas  [][]string
		coma   rune
		crlf   bool
	}{
		{"una fila simple", [][]string{{"a", "b", "c"}}, ',', false},
		{"campos con coma", [][]string{{"a,1", "b"}}, ',', false},
		{"campos con comilla", [][]string{{"di\"jo", "b"}}, ',', false},
		{"campos con salto", [][]string{{"una\nlinea", "b"}}, ',', false},
		{"campos con retorno", [][]string{{"una\rlinea", "b"}}, ',', false},
		{"campo vacio", [][]string{{"", ""}}, ',', false},
		{"campo con espacio adelante", [][]string{{" a", "b"}}, ',', false},
		{"campo con barra punto", [][]string{{"\\.", "b"}}, ',', false},
		{"varias filas", [][]string{{"a"}, {"b", "c"}, {}}, ',', false},
		{"con CRLF", [][]string{{"a", "b"}, {"c", "d"}}, ',', true},
		{"con CRLF y salto adentro", [][]string{{"a\nb", "c"}}, ',', true},
		{"con retorno y CRLF", [][]string{{"a\rb", "c"}}, ',', true},
		{"tabulador de separador", [][]string{{"a\t1", "b"}, {"c"}}, '\t', false},
		{"separador de mas de un byte", [][]string{{"a•b", "c"}}, '\u2022', false},
		{"campos con acentos", [][]string{{"café", "niño"}}, ',', false},
		{"separador invalido (salto)", [][]string{{"a"}}, '\n', false},
		{"separador invalido (comilla)", [][]string{{"a"}}, '"', false},
		{"separador cero", [][]string{{"a"}}, 0, false},
	}
	probados := 0
	for _, c := range casos {
		escribir := func(w interface {
			Write([]string) error
			Flush()
			Error() error
		}, setComa func(rune), setCRLF func(bool)) string {
			setComa(c.coma)
			setCRLF(c.crlf)
			err := error(nil)
			for _, f := range c.filas {
				if err = w.Write(f); err != nil {
					break
				}
			}
			w.Flush()
			return "salida=" + strconv.Quote("") + errTextoDe(err) + errTextoDe(w.Error())
		}
		var bufGo, bufMio strings.Builder
		wg := realcsv.NewWriter(&bufGo)
		wm := mycsv.NewWriter(&bufMio)
		_ = escribir
		// Go
		wg.Comma, wg.UseCRLF = c.coma, c.crlf
		errGo := error(nil)
		for _, f := range c.filas {
			if errGo = wg.Write(f); errGo != nil {
				break
			}
		}
		wg.Flush()
		textoGo := fmt.Sprintf("salida=%q err=%q ferr=%q", bufGo.String(), errTextoDe(errGo), errTextoDe(wg.Error()))
		// nuestro
		wm.Comma, wm.UseCRLF = c.coma, c.crlf
		errMio := error(nil)
		for _, f := range c.filas {
			if errMio = wm.Write(f); errMio != nil {
				break
			}
		}
		wm.Flush()
		textoMio := fmt.Sprintf("salida=%q err=%q ferr=%q", bufMio.String(), errTextoDe(errMio), errTextoDe(wm.Error()))
		if textoGo != textoMio {
			t.Errorf("Writer %s:\n  Go     %s\n  nuestro %s", c.nombre, textoGo, textoMio)
			continue
		}
		probados++
	}
	t.Logf("csv Writer: %d/%d identicos a Go (bytes, error y error pegajoso)", probados, len(casos))
}

// TestCSVWriterErrorPegajoso: si la salida de abajo falla, el error tiene que quedar guardado y
// salir por Error() en los dos lados (el motor no usa el Writer, pero esto es la mitad del contrato).
func TestCSVWriterErrorPegajoso(t *testing.T) {
	falso := &salidaQueFalla{deja: 3}
	wg := realcsv.NewWriter(falso)
	_ = wg.Write([]string{"una", "fila", "larga"})
	_ = wg.WriteAll([][]string{{"otra", "fila"}})
	wg.Flush()
	falloGo := wg.Error() != nil

	falso = &salidaQueFalla{deja: 3}
	wm := mycsv.NewWriter(falso)
	_ = wm.Write([]string{"una", "fila", "larga"})
	_ = wm.WriteAll([][]string{{"otra", "fila"}})
	wm.Flush()
	falloMio := wm.Error() != nil
	if falloGo != falloMio {
		t.Errorf("error pegajoso del Writer: Go %t, nuestro %t", falloGo, falloMio)
	}
	if !falloGo {
		t.Errorf("el propio Go no fallo: el montaje de la prueba esta mal")
	}
	t.Logf("csv: el error de la salida de abajo queda pegajoso en los dos (Go=%t, nuestro=%t)", falloGo, falloMio)
}

type salidaQueFalla struct {
	deja int
}

func (s *salidaQueFalla) Write(p []byte) (int, error) {
	if s.deja <= 0 {
		return 0, goerrors.New("la salida de abajo fallo")
	}
	if len(p) > s.deja {
		p = p[:s.deja]
	}
	s.deja -= len(p)
	return len(p), nil
}

// TestCSVTablasDelMotor: las tablas de texto del motor tal como las lee (Comma='\t',
// FieldsPerRecord=-1, LazyQuotes=true) y con las rarezas que traen: filas de largo distinto,
// tabuladores al final, comillas adentro de un campo, y campos con salto de linea.
func TestCSVTablasDelMotor(t *testing.T) {
	tandas := []struct {
		nombre string
		datos  string
	}{
		{"tabla chica", "Stat\tId\tDesc\nstrength\t0\tFuerza\n\ndexterity\t1\tDestreza\n"},
		{"filas de largo distinto", "a\tb\tc\n1\t2\n3\t4\t5\t6\n"},
		{"comilla adentro", "nombre\tdesc\nuno\t\"con\tcomilla\"\ndos\tcita \"suelta\"\n"},
		{"campo de varias lineas", "a\tb\n1\t\"una\nlinea\"\n"},
		{"tabulador final", "a\tb\n1\t\n"},
		{"columnas de la tabla de propiedades", "code\tstat\tsave add\tsave% add\nStrength\t0\t0\t0\nDexterity\t1\t0\t0\n"},
	}
	for _, tanda := range tandas {
		// el motor escribe el mismo texto dos veces (uno por lado) para que la comparacion sea del
		// paquete y no del archivo
		rg := realcsv.NewReader(strings.NewReader(tanda.datos))
		rg.Comma, rg.FieldsPerRecord, rg.LazyQuotes = '\t', -1, true
		filasGo, errGo := rg.ReadAll()

		rm := mycsv.NewReader(miLector(tanda.datos))
		rm.Comma, rm.FieldsPerRecord, rm.LazyQuotes = '\t', -1, true
		filasMio, errMio := rm.ReadAll()

		quizo := serialFilas(filasGo) + " err=" + errTextoDe(errGo)
		tengo := serialFilas(filasMio) + " err=" + errTextoDe(errMio)
		if quizo != tengo {
			t.Errorf("tabla del motor %s:\n  Go     %s\n  nuestro %s", tanda.nombre, quizo, tengo)
		}
	}
	t.Logf("csv: %d tablas del motor (tabulador, campos libres, comillas perezosas) identicas", len(tandas))
}

// TestCSVLimitesDeclarados: los limites escritos en la cabecera del paquete TIENEN que comportarse
// como estan documentados. Si algun dia dejan de ser ciertos, esta prueba lo dice.
func TestCSVLimitesDeclarados(t *testing.T) {
	// 1. TrimLeadingSpace solo saltea el espacio ASCII: Go tambien saltea U+00A0 (espacio duro) y
	//    nosotros no (no hay tablas de Unicode en la consola).
	datos := "\u00a0a,b\n"
	rg := realcsv.NewReader(strings.NewReader(datos))
	rg.TrimLeadingSpace = true
	filasGo, _ := rg.ReadAll()
	rm := mycsv.NewReader(miLector(datos))
	rm.TrimLeadingSpace = true
	filasMio, _ := rm.ReadAll()
	if len(filasGo) != 1 || len(filasMio) != 1 {
		t.Fatalf("no lei una fila de los dos lados: %v %v", filasGo, filasMio)
	}
	if filasGo[0][0] != "a" {
		t.Errorf("Go no salteo el espacio duro: %q (el limite documentado cambio de lado)", filasGo[0][0])
	}
	if filasMio[0][0] == "a" {
		t.Errorf("nosotros salteamos el espacio duro: %q (el limite documentado dice que no)", filasMio[0][0])
	}

	// 2. fieldNeedsQuotes: un campo que arranca con espacio duro se entrecomilla en Go y no en
	//    nuestro Writer (mismo motivo).
	var sbGo, sbMio strings.Builder
	wg := realcsv.NewWriter(&sbGo)
	_ = wg.Write([]string{"\u00a0a", "b"})
	wg.Flush()
	wm := mycsv.NewWriter(&sbMio)
	_ = wm.Write([]string{"\u00a0a", "b"})
	wm.Flush()
	if sbGo.String()[0] != '"' {
		t.Errorf("Go no entrecomillo el campo con espacio duro: %q (el limite documentado cambio de lado)", sbGo.String())
	}
	if sbMio.String()[0] == '"' {
		t.Errorf("nosotros entrecomillamos el espacio duro: %q (el limite documentado dice que no)", sbMio.String())
	}
	t.Logf("csv: los 2 limites declarados se comportan como estan escritos (Go %q / nuestro %q)", sbGo.String(), sbMio.String())
}
