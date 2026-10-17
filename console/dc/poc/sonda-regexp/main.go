//go:build dreamcast
// +build dreamcast

// SONDA DE DC-002.16: la bandera (?i) del regexp de la consola.
//
// Antes (DC-002.15, segunda corrida): esta sonda aislo que el patron que cortaba el arranque del motor
// era el UNICO con bandera, d2core/d2map/d2mapgen/real_maze_gen.go:33
//
//	(?i)(pre|prev|up|ent|exit)\d*\.ds1$
//
// y que `(?i)a` sola ya paniqueaba. Ahora mide, en la consola y con el MISMO regexp que usa el motor
// (console/dc/std/regexp, el que se compila al .gox `regexp`):
//
//  1. Los patrones de NIVEL DE PAQUETE del motor, cada uno con su archivo:linea, compilados con
//     regexp.Compile (que DEVUELVE el error en vez de paniquear): asi el veredicto es el texto del error
//     y no un "no anda".
//  2. El PLEGADO de mayusculas, con el patron real del motor y los nombres de archivo que tiene que
//     aceptar. Una bandera que compila pero no pliega es una bandera MUERTA: ese fue el segundo defecto
//     de esta tarjeta y se ve en estos numeros (esperado 6 de 6 y 0 de 2), no en el compilador.
//  3. El sintoma del motor, al final: un regexp.MustCompile de nivel de paquete con la bandera. Con el
//     defecto puesto la corrida termina en panico ANTES de esta ultima linea (0 lineas MOTOR-ARRANQUE).
//
// Uso:
//
//	source scripts/console/env.sh
//	GODC_GOX=<gox> bash scripts/console/build-dc.sh console/dc/poc/sonda-regexp
//	CAPTURE=none bash scripts/console/run-flycast.sh /tmp/dcbuild/sonda-regexp/prog.elf 15 /tmp/dc-run-regexp
//	grep -a 'SONDA-REGEXP' /tmp/dc-run-regexp/flycast.log | tr -d '\r'
package main

import "regexp"

// los patrones que el motor pide de NIVEL DE PAQUETE (medidos con grep de regexp.MustCompile sobre el
// arbol del motor; los de los _test.go no cuentan: no entran al binario de la consola).
var delMotor = []struct{ donde, pat string }{
	{"d2core/d2item/diablo2item/item_factory.go:443", `\d+`},
	{"d2core/d2item/diablo2item/item_factory.go:450", `\D`},
	{"d2core/d2item/diablo2item/item_log.go:87", `\[[a-z]+\]`},
	{"d2core/d2monsters/combat.go:26", `\[[a-z]+\]`},
	{"d2game/d2player/inventory_imported.go:10", `\[[a-z]+\]`},
	{"d2core/d2ui/color_tokens.go:10", `\[[^\]]+\]`},
	{"d2core/d2ui/color_tokens.go:11", `\[[^\]]+\][^\[]+`},
	{"d2core/d2item/d2drop/loader.go:351", `^-?\d+$`},
	{"d2core/d2map/d2mapgen/real_maze_gen.go:33", `(?i)(pre|prev|up|ent|exit)\d*\.ds1$`},
}

// el patron del motor que lleva la bandera: los textos que tiene que aceptar (6) y los que NO (2). Los
// nombres son los del generador de mapas: las salas se llaman pre/prev/up/ent/exit con un numero y la
// extension .ds1, y en el MPQ aparecen en MAYUSCULAS.
var plegado = struct {
	si []string
	no []string
}{
	si: []string{"PRE1.ds1", "pre1.ds1", "Up.ds1", "EXIT34.ds1", "prev2.ds1", "data/global/tiles/PRE1.ds1"},
	no: []string{"x.ds1", "exit.ds2"},
}

// num: el numero a texto sin depender de strconv (para cuatro cuentas no hace falta). Que los numeros de
// la evidencia salgan en la linea es el punto: se comparan, no se describen.
func num(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

func main() {
	println("SONDA-REGEXP216: la bandera (?i) del regexp de la consola")

	println("SONDA-REGEXP216 -- 1. los " + num(len(delMotor)) + " patrones del motor con Compile (el error, no el panico) --")
	compilan := 0
	for _, c := range delMotor {
		_, err := regexp.Compile(c.pat)
		if err != nil {
			println("SONDA-REGEXP216   NO COMPILA " + c.donde + "  err=" + err.Error())
			continue
		}
		compilan++
	}
	println("SONDA-REGEXP216 patrones del motor que compilan: " + num(compilan) + " de " + num(len(delMotor)))

	println("SONDA-REGEXP216 -- 2. el plegado de mayusculas con el patron con bandera --")
	re, err := regexp.Compile(`(?i)(pre|prev|up|ent|exit)\d*\.ds1$`)
	if err != nil {
		println("SONDA-REGEXP216   el patron con bandera NO compila: " + err.Error())
	} else {
		si := 0
		for _, t := range plegado.si {
			if re.MatchString(t) {
				si++
			}
		}
		negros := 0
		for _, t := range plegado.no {
			if re.MatchString(t) {
				negros++
			}
		}
		println("SONDA-REGEXP216 plegados (esperado " + num(len(plegado.si)) + " de " + num(len(plegado.si)) + "): " + num(si))
		println("SONDA-REGEXP216 los que NO tienen que matchear (esperado 0 de " + num(len(plegado.no)) + "): " + num(negros))
	}

	println("SONDA-REGEXP216 -- 3. el sintoma del motor: MustCompile de nivel de paquete con la bandera --")
	regexp.MustCompile(`(?i)a`)
	println("SONDA-REGEXP216 la bandera no corta el arranque: el MustCompile paso de largo")
}
