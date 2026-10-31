// REGLA EJECUTABLE (estilo ArchUnit): en el camino de consola, un asset se lee SÓLO por la Source
// (`asset.Source`, `console/dc/fuenteconsola`) y con dueño (`ReadFileInto` / `Pedir` / `Liberar`).
//
// POR QUE UNA REGLA Y NO UN COMENTARIO EN EL ADR. El ADR (docs/adr/0001-...) dice la decisión de Daniel del
// 31/10: los assets grandes van fuera del montón de Go, con liberación explícita, y el motor no se toca. Una
// decisión que sólo vive en un documento se pierde en la tercera tarjeta: el que llega nuevo escribe
// `mpq.ReadFile` porque es lo que encuentra en el POC de al lado, y el defecto vuelve medido en RAM (DC-002.96:
// el mismo archivo de 1 MB por el camino viejo deja 3.633.432 B tomados; por la Source vuelve al byte).
//
// POR QUE EN GO CON go/ast Y NO CON grep. El árbol está lleno de PROSA: `lectura_consola.go` y `mpq.go`
// nombran `io.ReadAll` en un comentario que explica justamente por qué NO se usa. Un grep se pone rojo con el
// comentario que enseña a no hacerlo, y el que lo ve lo apaga. El AST no ve comentarios ni strings: ve
// llamadas.
//
// QUE SE CONSIDERA LECTURA DE ASSET (una falta):
//
//	os.ReadFile          - el archivo entero al montón de Go
//	ioutil.ReadFile      - idem
//	X.ReadFile(...)      - el `ReadFile` del MPQ: reserva `make([]byte, tamano)` y el recolector no la devuelve
//	                       (en libgodc todo pedido de más de 64 KB se va a `gc_external_alloc`)
//	X.ReadFileStream(...) - el stream del MPQ, que termina en `io.ReadAll`
//	io.ReadAll/ioutil.ReadAll - duplica el buffer (512 B, 1 KB, 2 KB...) y con un .dt1 de 505 KB mata la consola
//	os.Open              - abrir a mano es la forma de leer por trozos sin que se note
//
// y lo que NO se castiga: `ReadFileInto` (el camino con dueño: escribe en el tramo que ya se posee), `Pedir`/
// `Liberar` de la fuente, `Contains`, y `d2asset.LoadFile` (que desde DC-002.99 pide un tramo a la Source).
//
// LA DEUDA SE DECLARA, NO SE TAPA. Hay lecturas que YA existían cuando se escribió la regla (el POC del
// personaje, el adaptador de audio). Esas van una por una en
// scripts/console/assets-fuera-de-la-source.deuda.txt, con el motivo y la tarjeta que las paga. Toda lectura
// que no esté en esa lista es una violación NUEVA y el chequeo sale rojo (exit 1). Así el número sólo puede
// bajar: el que arregla una borra su línea (el chequeo lo avisa como "pagada") y el que agrega una no tiene
// dónde esconderla.
//
// Uso:  go run ./scripts/console/regla-assets-por-la-source [-raiz .] [-deuda archivo] [-v]
//
//	exit 0 = verde · exit 1 = hay violaciones nuevas · exit 2 = no se pudo correr
//
// El envoltorio de la casa es scripts/console/checar-assets-por-la-source.sh (y su mutador,
// mutar-assets-por-la-source.sh, que exige que la regla cace un caso puesto a propósito).
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ambito es un directorio del ÁRBOL QUE SE REVISA. La lista es explícita a propósito: el camino de consola es
// lo que la casa dice que es, no "todo lo que tenga una palabra linda en el nombre". Los POC viejos
// (`console/dc/poc/*`) son banco de pruebas con crc32 medido y no se revisan: si uno pasa a ser el producto,
// se suma acá y su deuda se declara.
type ambito struct {
	dir    string
	porque string
}

var ambitos = []ambito{
	{"console/dc/fuenteconsola", "la Source de consola (DC-002.96): el único lugar del camino que pide bytes al MPQ"},
	{"console/dc/poc/fuente-consola", "el POC de la Source: los seis formatos con dueño (DC-002.96)"},
	{"console/dc/poc/mapa-101", "el pueblo compuesto y dibujado por el camino del motor + PVR (DC-002.101)"},
	{"console/dc/poc/carga-motor-299", "la carga del motor por tramos de la Source (DC-002.99)"},
	{"console/dc/poc/dcc-arena", "el .dcc decodificado sobre la arena (DC-002.98)"},
	{"console/dc/poc/coop", "el cooperativo de cuatro jugadores (DC-002.87/.88)"},
	{"console/dc/poc/personaje-mapa", "el POC del personaje: deuda declarada, se migra en su tarjeta (DC-002.96 §6.3)"},
	{"d2core/d2render/pvr", "el adaptador de render de la consola (Fase B)"},
	{"d2core/d2input/maple", "el adaptador de input de la consola (DC-002.71)"},
	{"d2core/d2audio/aica", "el adaptador de audio de la consola (DC-002.72)"},
}

// falta es una lectura de asset encontrada en el alcance.
type falta struct {
	archivo string // relativo a la raíz, con "/" (como en la deuda)
	linea   int
	funcion string // función envolvente, con el tipo si es método: "Proveedor.cargar"
	llamada string // lo escrito: "mpq.ReadFile", "os.ReadFile", "io.ReadAll", ...
	ordinal int    // cuántas veces aparece (archivo, función, llamada) hasta ésta inclusive
}

// huella es la identidad de la falta para la deuda: no lleva la línea (el código se mueve y la deuda no
// tiene que envejecer por un enter), pero SÍ el ordinal dentro de la función: una segunda lectura igual dentro
// de la misma función es una falta nueva, y así no se esconde detrás de la primera.
func (f falta) huella() string {
	return fmt.Sprintf("%s|%s|%s|%d", f.archivo, f.funcion, f.llamada, f.ordinal)
}

// entrada es una línea de la deuda.
type entrada struct {
	huella string
	tipo   string // DEUDA (a pagar) o PERMANENTE (una lectura que siempre va a estar, con su porqué)
	motivo string
}

func main() {
	raiz := flag.String("raiz", ".", "raíz del árbol (el repo)")
	deuda := flag.String("deuda", "", "archivo de deuda declarada (por defecto <raiz>/scripts/console/assets-fuera-de-la-source.deuda.txt)")
	verboso := flag.Bool("v", false, "muestra el alcance con su porqué y cada llamada permitida del camino con dueño")
	listarAlcance := flag.Bool("alcance", false, "imprime un directorio del alcance por línea y sale (lo lee el mutador, para no tener la lista dos veces)")
	flag.Parse()

	if *listarAlcance {
		for _, a := range ambitos {
			fmt.Println(a.dir)
		}
		os.Exit(0)
	}

	abs, err := filepath.Abs(*raiz)
	if err != nil {
		fmt.Fprintf(os.Stderr, "raiz invalida %q: %v\n", *raiz, err)
		os.Exit(2)
	}
	if _, err := os.Stat(abs); err != nil {
		fmt.Fprintf(os.Stderr, "no encuentro la raiz %s\n", abs)
		os.Exit(2)
	}

	archivoDeuda := *deuda
	if archivoDeuda == "" {
		archivoDeuda = filepath.Join(abs, "scripts", "console", "assets-fuera-de-la-source.deuda.txt")
	}
	declaradas, err := leerDeuda(archivoDeuda)
	if err != nil {
		fmt.Fprintf(os.Stderr, "deuda: %v\n", err)
		os.Exit(2)
	}

	fmt.Println("== REGLA: en el camino de consola un asset se lee SÓLO por la Source, y con dueño ==")
	fmt.Printf("alcance: %d directorios declarados (ADR docs/adr/0001-assets-fuera-del-heap-de-go.md)\n", len(ambitos))
	if *verboso {
		for _, a := range ambitos {
			fmt.Printf("  %-32s %s\n", a.dir, a.porque)
		}
	}

	todas, archivos, pruebas, conDueno, err := revisar(abs, *verboso)
	if err != nil {
		fmt.Fprintf(os.Stderr, "revision: %v\n", err)
		os.Exit(2)
	}

	fmt.Printf("archivos .go revisados: %d   (los _test.go no entran: %d archivos; un test pide la referencia al\n", archivos, pruebas)
	fmt.Printf("  MPQ para comparar, no es camino de producción)\n")
	fmt.Printf("lecturas por el camino con dueño (ReadFileInto/Pedir): %d  -- no son violación, es el camino\n", conDueno)
	fmt.Printf("lecturas fuera de la Source detectadas: %d\n", len(todas))
	cuentaDeuda, cuentaPerm := 0, 0
	for _, e := range declaradas {
		if e.tipo == "PERMANENTE" {
			cuentaPerm++
		} else {
			cuentaDeuda++
		}
	}
	fmt.Printf("deuda declarada: %d entradas (%d a pagar + %d permanentes)\n", len(declaradas), cuentaDeuda, cuentaPerm)

	// clasificar: declarada (deuda o permanente) vs nueva.
	var nuevas []falta
	usadas := map[string]bool{}
	var pagadas []entrada
	for _, f := range todas {
		h := f.huella()
		if _, ok := declaradas[h]; ok {
			usadas[h] = true
			continue
		}
		nuevas = append(nuevas, f)
	}
	for h, e := range declaradas {
		if !usadas[h] {
			pagadas = append(pagadas, e)
		}
	}
	sort.Slice(pagadas, func(i, j int) bool { return pagadas[i].huella < pagadas[j].huella })

	if len(pagadas) > 0 {
		fmt.Println()
		fmt.Printf("AVISO: %d entradas de la deuda ya no aparecen en el árbol (¿se pagaron? borralas):\n", len(pagadas))
		for _, e := range pagadas {
			fmt.Printf("  %s   (%s: %s)\n", e.huella, e.tipo, e.motivo)
		}
	}

	fmt.Println()
	if len(nuevas) > 0 {
		fmt.Printf("VIOLACIONES NUEVAS: %d\n", len(nuevas))
		for _, f := range nuevas {
			fmt.Printf("  %s:%d  en %s -> %s\n", f.archivo, f.linea, f.funcion, f.llamada)
		}
		fmt.Println()
		fmt.Println("ROJO: esas lecturas no están en la deuda declarada. El camino de consola pide el asset a la")
		fmt.Println("      Source (`Pedir`/`Liberar`, `ReadFileInto`) o, si de verdad es una excepción, se declara")
		fmt.Println("      en scripts/console/assets-fuera-de-la-source.deuda.txt con su motivo y su tarjeta.")
		os.Exit(1)
	}

	fmt.Println("VERDE: ninguna lectura de asset fuera de la Source en el camino de consola (la deuda declarada no creció)")
	os.Exit(0)
}

// revisar recorre el alcance y devuelve todas las lecturas fuera de la Source, cuántos archivos de producción
// se revisaron, cuántos _test.go quedaron afuera y cuántas lecturas van por el camino con dueño (para que se
// vea que la regla distingue).
func revisar(raiz string, verboso bool) (faltas []falta, archivos, pruebas, conDueno int, err error) {
	for _, a := range ambitos {
		dir := filepath.Join(raiz, filepath.FromSlash(a.dir))
		if _, e := os.Stat(dir); e != nil {
			// Un ámbito que no existe es un error de la regla, no un verde: si el directorio se movió, la
			// lista quedó mintiendo y hay que arreglarla.
			return nil, 0, 0, 0, fmt.Errorf("el ambito %s no existe (¿se movió? actualizá la lista de la regla)", a.dir)
		}
		e := filepath.Walk(dir, func(ruta string, info os.FileInfo, e error) error {
			if e != nil {
				return e
			}
			if info.IsDir() {
				if n := info.Name(); n == "testdata" || n == "csrc" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(ruta, ".go") {
				return nil
			}
			if strings.HasSuffix(ruta, "_test.go") {
				pruebas++
				return nil
			}
			rel, e := filepath.Rel(raiz, ruta)
			if e != nil {
				return e
			}
			rel = filepath.ToSlash(rel)
			fs := token.NewFileSet()
			f, e := parser.ParseFile(fs, ruta, nil, 0)
			if e != nil {
				return fmt.Errorf("%s: no parsea: %v", rel, e)
			}
			archivos++
			ord := map[string]int{}
			for _, d := range f.Decls {
				fn, ok := d.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				faltas = append(faltas, revisarCuerpo(fs, rel, nombreDe(fn), fn.Body, ord, &conDueno, verboso)...)
			}
			return nil
		})
		if e != nil {
			return nil, 0, 0, 0, e
		}
	}
	return faltas, archivos, pruebas, conDueno, nil
}

// nombreDe arma el nombre de la función envolvente, con el tipo cuando es método ("Proveedor.cargar").
func nombreDe(fn *ast.FuncDecl) string {
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		t := fn.Recv.List[0].Type
		if star, ok := t.(*ast.StarExpr); ok {
			t = star.X
		}
		if id, ok := t.(*ast.Ident); ok {
			return id.Name + "." + fn.Name.Name
		}
	}
	return fn.Name.Name
}

// revisarCuerpo camina un cuerpo de función y devuelve las lecturas. Las clausuras se revisan aparte
// (recursión) para que el nombre de la función diga dónde está la lectura.
func revisarCuerpo(fset *token.FileSet, archivo, funcion string, cuerpo *ast.BlockStmt, ord map[string]int, conDueno *int, verboso bool) []falta {
	var out []falta
	ast.Inspect(cuerpo, func(n ast.Node) bool {
		if lit, ok := n.(*ast.FuncLit); ok {
			out = append(out, revisarCuerpo(fset, archivo, funcion+".clausura", lit.Body, ord, conDueno, verboso)...)
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		metodo := sel.Sel.Name
		receptor := ""
		if id, ok := sel.X.(*ast.Ident); ok {
			receptor = id.Name
		}
		escrito := metodo
		if receptor != "" {
			escrito = receptor + "." + metodo
		}
		pos := fset.Position(call.Pos())
		switch {
		case metodo == "ReadFileInto" || metodo == "Pedir" || metodo == "Liberar":
			*conDueno++
			if verboso {
				fmt.Printf("  [con dueño] %s:%d  %s\n", archivo, pos.Line, escrito)
			}
			return true
		case metodo == "ReadAll" && (receptor == "io" || receptor == "ioutil"),
			metodo == "ReadFile",
			metodo == "ReadFileStream",
			metodo == "Open" && receptor == "os":
		default:
			return true
		}
		clave := funcion + "|" + escrito
		ord[clave]++
		out = append(out, falta{
			archivo: archivo,
			linea:   pos.Line,
			funcion: funcion,
			llamada: escrito,
			ordinal: ord[clave],
		})
		return true
	})
	return out
}

// leerDeuda lee el archivo de deuda declarada.
//
// Formato, una entrada por línea (las líneas vacías y las que empiezan con # se ignoran):
//
//	archivo|funcion|llamada|n   TIPO   motivo
//
// Los cuatro primeros campos van separados por "|"; después de ellos, TIPO y motivo van separados por espacios
// (el motivo es todo lo que queda, así que puede tener "|" adentro). TIPO es DEUDA (hay que pagarla: el camino
// se migra a la Source) o PERMANENTE (esa lectura no es un asset o la interfaz obliga a tenerla, y nunca va a
// pasar por la Source; el motivo tiene que decirlo).
func leerDeuda(ruta string) (map[string]entrada, error) {
	b, err := os.ReadFile(ruta)
	if err != nil {
		return nil, err
	}
	out := map[string]entrada{}
	for n, linea := range strings.Split(string(b), "\n") {
		l := strings.TrimSpace(linea)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		partes := strings.SplitN(l, "|", 4)
		if len(partes) < 4 {
			return nil, fmt.Errorf("%s:%d: se esperan 3 campos con \"|\" (archivo|funcion|llamada) y hay %d", ruta, n+1, len(partes)-1)
		}
		campos := strings.Fields(partes[3])
		if len(campos) < 3 {
			return nil, fmt.Errorf("%s:%d: se espera \"n TIPO motivo\" despues de la llamada (n y TIPO: %v)", ruta, n+1, campos)
		}
		ordinal := campos[0]
		tipo := campos[1]
		if tipo != "DEUDA" && tipo != "PERMANENTE" {
			return nil, fmt.Errorf("%s:%d: TIPO %q no es DEUDA ni PERMANENTE", ruta, n+1, tipo)
		}
		motivo := strings.Join(campos[2:], " ")
		h := strings.Join([]string{
			strings.TrimSpace(partes[0]),
			strings.TrimSpace(partes[1]),
			strings.TrimSpace(partes[2]),
			ordinal,
		}, "|")
		out[h] = entrada{huella: h, tipo: tipo, motivo: motivo}
	}
	return out, nil
}
