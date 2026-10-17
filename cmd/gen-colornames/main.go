// Genera el paquete colornames de la consola a partir de la tabla REAL de golang.org/x/image.
//
// POR QUE GENERADO Y NO ESCRITO A MANO: son 148 colores con sus valores exactos. Copiarlos a mano es
// garantia de un error de tipeo en algun canal (y un color mal no se nota: se ve "casi" igual). Leyendo la
// tabla real, el resultado es el de Go o no compila.
//
// Uso: go run gen-colornames.go <archivo destino>
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"golang.org/x/image/colornames"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "falta el archivo destino")
		os.Exit(1)
	}

	nombres := make([]string, 0, len(colornames.Map))
	for n := range colornames.Map {
		nombres = append(nombres, n)
	}
	sort.Strings(nombres)

	var b strings.Builder
	b.WriteString(`// Package colornames: los nombres de los colores de golang.org/x/image, para la consola.
//
// ESTE ARCHIVO ESTA GENERADO: no se edita a mano. Lo genera cmd/gen-colornames.go leyendo la tabla REAL de
// golang.org/x/image/colornames, asi que los valores son los de Go o no son. (A mano seria copiar 148
// colores y equivocarse en algun canal, que es el peor error posible: un color que se ve "casi" bien.)
//
// PARA QUE SE USA: d2core/d2render/ebiten/ebiten_panic_screen.go lo usa UNA vez, para pintar la pantalla de
// error de rojo (colornames.Darkred). Con eso alcanzaria para ese uso, pero la tabla completa no cuesta
// nada (esta generada) y evita que el proximo uso tenga que volver aca.
//
// El tipo del valor es color.RGBA, igual que en Go, y los colores son OPACOS (alfa 0xff): son colores con
// nombre, no colores con transparencia.
package colornames

import "image/color"

// Map: los nombres y sus colores, como en golang.org/x/image.
var Map = map[string]color.RGBA{
`)
	for _, n := range nombres {
		c := colornames.Map[n]
		fmt.Fprintf(&b, "\t%q: {R: 0x%02x, G: 0x%02x, B: 0x%02x, A: 0x%02x},\n", n, c.R, c.G, c.B, c.A)
	}
	b.WriteString("}\n\n")

	// y los colores con el nombre capitalizado, que es la forma en que se usan (colornames.Darkred).
	// SON VARIABLES, NO CONSTANTES: en golang.org/x/image son `var` (un color.RGBA es un struct y go no
	// acepta un struct como constante: gccgo lo canta como "invalid constant type"). Mejor dicho: se
	// copia la forma de Go, que es var.
	b.WriteString("// Los colores con su nombre (es la forma en que se usan: colornames.Darkred).\n// Son var y no const, igual que en golang.org/x/image.\nvar (\n")
	for _, n := range nombres {
		c := colornames.Map[n]
		fmt.Fprintf(&b, "\t%s = color.RGBA{R: 0x%02x, G: 0x%02x, B: 0x%02x, A: 0x%02x}\n",
			capitalizar(n), c.R, c.G, c.B, c.A)
	}
	b.WriteString(")\n")

	if err := os.WriteFile(os.Args[1], []byte(b.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("colornames: %d colores generados en %s\n", len(nombres), os.Args[1])
}

// capitalizar: "darkred" -> "Darkred" (asi los nombra Go: solo la primera letra).
func capitalizar(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
