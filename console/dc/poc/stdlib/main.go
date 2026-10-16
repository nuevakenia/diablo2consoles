//go:build dreamcast

// DC-002.2: la biblioteca estandar minima corriendo en la Dreamcast emulada.
//
// Corre la misma familia de casos que la prueba diferencial del host (console/dc/std/difftest),
// pero aca no esta la stdlib de Go para comparar: las respuestas esperadas vienen calculadas por
// la biblioteca estandar REAL en el host y horneadas en casos_gen.go durante el build (lo genera
// scripts/console/gen-std-casos.sh con console/dc/std/cmd/gen-casos).
//
// La evidencia es la consola serie (numeros) y el color de pantalla: verde si coincidieron todos,
// rojo si alguno no.
package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// extern: el nombre del simbolo de C es tal cual (con go:extern gccgo lo mangla)
//
//extern d2c_serial_init
func d2c_serial_init()

//extern d2c_pantalla_init
func d2c_pantalla_init()

//extern d2c_pantalla_color
func d2c_pantalla_color(r, g, b float32)

//extern d2c_frame
func d2c_frame()

var bien, mal int
var fallos []string

func check(familia, caso, esperado, tengo string) {
	if esperado == tengo {
		bien++
		return
	}
	mal++
	if len(fallos) < 12 {
		fallos = append(fallos, familia+" "+caso+" esperaba "+strconv.Quote(esperado)+" y dio "+strconv.Quote(tengo))
	}
}

func main() {
	d2c_serial_init()
	println("STD-TEST: arranca el autotest de la stdlib minima (DC-002.2)")

	correrFmt()
	correrStrconv()
	correrStrings()
	correrErrores()

	for _, f := range fallos {
		println("STD-TEST FALLO:", f)
	}
	println("STD-TEST: casos", strconv.Itoa(bien+mal), "bien", strconv.Itoa(bien), "mal", strconv.Itoa(mal))

	d2c_pantalla_init()
	if mal == 0 {
		println("STD-TEST: RESULTADO OK - coincidio con la stdlib real en la consola")
		d2c_pantalla_color(0.0, 0.7, 0.0)
	} else {
		println("STD-TEST: RESULTADO MAL - hubo casos que no coincidieron")
		d2c_pantalla_color(0.7, 0.0, 0.0)
	}

	// el PVR necesita que el programa siga vivo
	for {
		d2c_frame()
	}
}

func correrFmt() {
	for _, c := range casosFmt {
		tengo := fmt.Sprintf(c.formato, c.args...)
		check("fmt", strconv.Quote(c.formato), c.esperado, tengo)
	}
}

func correrStrconv() {
	for _, c := range casosItoa {
		check("strconv", "Itoa", c.esperado, strconv.FormatInt(c.n, 10))
	}
	for _, c := range casosAtoi {
		v, err := strconv.Atoi(c.s)
		tengo := strconv.FormatInt(int64(v), 10) + " error=" + strconv.FormatBool(err != nil)
		esperado := strconv.FormatInt(c.esperado, 10) + " error=" + strconv.FormatBool(c.hayError)
		check("strconv", "Atoi("+strconv.Quote(c.s)+")", esperado, tengo)
	}
	for _, c := range casosQuote {
		check("strconv", "Quote", c.esperado, strconv.Quote(c.s))
	}
	for _, c := range casosFormatFloat {
		check("strconv", "FormatFloat", c.esperado, strconv.FormatFloat(c.f, 'f', c.prec, 64))
	}
}

func correrStrings() {
	for _, c := range casosStr {
		var tengo string
		switch c.fn {
		case "TrimSpace":
			tengo = strings.TrimSpace(c.a)
		case "ToLower":
			tengo = strings.ToLower(c.a)
		case "ToUpper":
			tengo = strings.ToUpper(c.a)
		case "Contains":
			tengo = strconv.FormatBool(strings.Contains(c.a, c.b))
		case "HasPrefix":
			tengo = strconv.FormatBool(strings.HasPrefix(c.a, c.b))
		case "HasSuffix":
			tengo = strconv.FormatBool(strings.HasSuffix(c.a, c.b))
		case "Index":
			tengo = strconv.Itoa(strings.Index(c.a, c.b))
		case "EqualFold":
			tengo = strconv.FormatBool(strings.EqualFold(c.a, c.b))
		case "Split":
			tengo = strings.Join(strings.Split(c.a, c.b), "|")
		case "ReplaceAll":
			tengo = strings.ReplaceAll(c.a, c.b, "-")
		case "Fields":
			tengo = strconv.Itoa(len(strings.Fields(c.a)))
		case "Join":
			tengo = strings.Join(strings.Split(c.a, "|"), "|")
		default:
			tengo = "funcion desconocida: " + c.fn
		}
		check("strings", c.fn, c.esperado, tengo)
	}
}

func correrErrores() {
	base := errors.New("base")
	e := fmt.Errorf("contexto: %w (codigo %d)", base, 7)
	check("errors", "Errorf con %w", "contexto: base (codigo 7)", e.Error())
	check("errors", "Is", "true", strconv.FormatBool(errors.Is(e, base)))
	check("errors", "Unwrap", "base", errors.Unwrap(e).Error())
	check("errors", "Join", "a\nb", errors.Join(errors.New("a"), nil, errors.New("b")).Error())

	// La FIRMA del metodo decide si un tipo implementa una interfaz (DC-002.2f): un envoltorio de
	// VARIOS %w tiene Unwrap() []error, asi que implementa interface{ Unwrap() []error } y NO
	// interface{ Unwrap() error }. Los esperados son los de Go real.
	//
	// Este caso es el que caza al mutante del runtime: si implements_interface vuelve a mirar solo el
	// nombre del metodo, la asercion de abajo dice true y el corpus se pone rojo.
	doble := fmt.Errorf("contexto: %w y %w", base, errors.New("otro"))
	_, varias := doble.(interface{ Unwrap() []error })
	_, solo := doble.(interface{ Unwrap() error })
	check("errors", "aserción Unwrap() []error sobre un envoltorio de varios", "true", strconv.FormatBool(varias))
	check("errors", "aserción Unwrap() error sobre un envoltorio de varios", "false", strconv.FormatBool(solo))
	check("errors", "Is con dos envueltos", "true", strconv.FormatBool(errors.Is(doble, base)))
}
