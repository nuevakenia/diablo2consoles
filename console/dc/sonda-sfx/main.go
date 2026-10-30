// Sonda del host (DC-002.72): lista los WAV de un MPQ y muestra los mas chicos con su ruta.
//
// POR QUE EXISTE: la tarjeta pide que el efecto de la prueba sea un WAV DEL MOTOR (de un MPQ), no el
// fixture que genera el POC. Para elegirlo hay que ver que hay adentro: este programa abre el MPQ con el
// MISMO paquete que usa el motor (d2common/d2fileformats/d2mpq) y lista (listfile) filtrando por extension.
//
// Uso:  go run ./console/dc/sonda-sfx <mpq> [ext] [n]
//   mpq  ruta del MPQ (p.ej. ~/.wine-d2/.../d2Sfx.mpq)
//   ext  extension a buscar sin punto (por defecto wav)
//   n    cuantos mostrar, los mas CHICOS primero (por defecto 20)
package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("uso: sonda-sfx <mpq> [ext] [n]")
		os.Exit(2)
	}

	ext := "wav"
	if len(os.Args) > 2 {
		ext = strings.ToLower(os.Args[2])
	}

	n := 20
	if len(os.Args) > 3 {
		if v, err := strconv.Atoi(os.Args[3]); err == nil {
			n = v
		}
	}

	m, err := d2mpq.FromFile(os.Args[1])
	if err != nil {
		fmt.Println("ERROR abriendo el MPQ:", err)
		os.Exit(1)
	}

	defer m.Close()

	lista, err := m.Listfile()
	if err != nil {
		fmt.Println("ERROR leyendo (listfile):", err)
		os.Exit(1)
	}

	type entrada struct {
		ruta  string
		bytes int
	}

	var elegidos []entrada

	total := 0

	for _, f := range lista {
		if !strings.HasSuffix(strings.ToLower(f), "."+ext) {
			continue
		}

		total++

		datos, err := m.ReadFile(f)
		if err != nil {
			fmt.Println("  ilegible:", f, err)
			continue
		}

		elegidos = append(elegidos, entrada{ruta: f, bytes: len(datos)})
	}

	sort.Slice(elegidos, func(i, j int) bool { return elegidos[i].bytes < elegidos[j].bytes })

	fmt.Printf("MPQ %s: %d archivos en (listfile), %d con extension .%s\n", os.Args[1], len(lista), total, ext)

	for i, e := range elegidos {
		if i >= n {
			break
		}

		fmt.Printf("  %8d B  %s\n", e.bytes, e.ruta)
	}
}
