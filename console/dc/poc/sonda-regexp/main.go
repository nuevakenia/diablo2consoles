//go:build dreamcast
// +build dreamcast

// Sonda de DC-002.15 (segunda corrida): por que el arranque del motor paniquea con un *regexp.Error.
//
// Primera corrida (console/dc/poc/sonda-regexp, 9 patrones de nivel de paquete del motor): los cuatro
// primeros compilan (^-?\d+$ , \d+ , \D , \[[a-z]+\]) y el QUINTO se lleva la sonda:
//
//	SONDA-REGEXP probando d2core/d2map/d2mapgen/real_maze_gen.go:33 (?i)(pre|prev|up|ent|exit)\d*\.ds1$
//	panic: (value of kind 22 at 0x8c09c898)
//
// Esta corrida separa las tres cosas que ese patron tiene y los anteriores no -- la bandera (?i), la
// alternacion (a|b) y \d*\. -- para decir CUAL rompe el regexp de la consola, en vez de suponer.
package main

import "regexp"

var casos = []struct{ que, pat string }{
	{"solo la bandera (?i) con una letra", `(?i)a`},
	{"alternacion sola", `(pre|prev)`},
	{"\\d* y \\. solos", `\d*\.ds1$`},
	{"clase y cuantificador", `[a-z]*`},
	{"ancla y fin", `^abc$`},
	{"bandera y ancla", `(?i)^abc$`},
	{"alternacion con bandera", `(?i)(pre|prev)`},
	{"clase, bandera y \\d*", `(?i)[a-z]\d*\.ds1$`},
	{"el patron que falla, entero", `(?i)(pre|prev|up|ent|exit)\d*\.ds1$`},
}

func main() {
	println("SONDA-REGEXP2: separando la bandera, la alternacion y el resto")

	for _, c := range casos {
		println("SONDA-REGEXP2 probando", c.que, c.pat)

		re := regexp.MustCompile(c.pat) // el MISMO camino que usan los paquetes del motor

		if re.MatchString("hola") {
			println("SONDA-REGEXP2   ok (con match)")
		} else {
			println("SONDA-REGEXP2   ok (sin match)")
		}
	}

	println("SONDA-REGEXP2: probados", len(casos), "casos")
}
