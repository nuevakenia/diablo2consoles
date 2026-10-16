// bake-list: lista los archivos de un MPQ de Diablo II (para elegir que hornear).
//
// Es una herramienta del HOST (como bake-dc6): usa el lector de MPQ del motor, tal cual.
//
// Uso:  go run ./console/dc/bake-list -mpq d2data.mpq -filtro 'ui\bigmenu'
//
// Por que existe: los assets del juego no entran al repo, se hornean al build, y para pedir un DC6 hay
// que saber como se llama adentro del MPQ. Esto lo dice, en vez de adivinarlo.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

func main() {
	mpqPath := flag.String("mpq", "", "archivo MPQ de entrada")
	filtro := flag.String("filtro", "", "solo las rutas que contengan este texto (en minusculas)")
	ext := flag.String("ext", "", "solo las rutas que terminen en esta extension (p.ej. .dc6)")
	flag.Parse()

	if *mpqPath == "" {
		fmt.Fprintln(os.Stderr, "uso: bake-list -mpq d2data.mpq [-filtro texto] [-ext .dc6]")
		os.Exit(2)
	}

	// el MPQ puede estar en un camino relativo a la carpeta de Diablo II, como en bake-dc6
	if _, err := os.Stat(*mpqPath); err != nil {
		carpeta := os.Getenv("D2_DIR")
		if carpeta == "" {
			carpeta = autodetectarD2()
		}
		if carpeta != "" {
			if _, err2 := os.Stat(filepath.Join(carpeta, *mpqPath)); err2 == nil {
				*mpqPath = filepath.Join(carpeta, *mpqPath)
			}
		}
	}

	m, err := d2mpq.FromFile(*mpqPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude abrir el MPQ:", err)
		os.Exit(1)
	}
	defer m.Close()

	datos, err := m.ReadFile("(listfile)")
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude leer el (listfile):", err)
		os.Exit(1)
	}

	f := strings.ToLower(*filtro)
	e := strings.ToLower(*ext)
	cant := 0
	for _, linea := range strings.Split(string(datos), "\n") {
		ruta := strings.TrimSpace(linea)
		if ruta == "" {
			continue
		}
		if f != "" && !strings.Contains(strings.ToLower(ruta), f) {
			continue
		}
		if e != "" && !strings.HasSuffix(strings.ToLower(ruta), e) {
			continue
		}
		fmt.Println(ruta)
		cant++
	}
	fmt.Fprintf(os.Stderr, "-- %d rutas\n", cant)
}

// autodetectarD2: los caminos tipicos de Diablo II en Wine, como en bake-dc6.
func autodetectarD2() string {
	candidatos := []string{
		filepath.Join(os.Getenv("HOME"), ".wine/drive_c/Program Files (x86)/Diablo II"),
		filepath.Join(os.Getenv("HOME"), ".wine/drive_c/Program Files/Diablo II"),
	}
	for _, c := range candidatos {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}
