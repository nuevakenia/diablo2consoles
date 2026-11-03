// scratch: mide los tamanos de los archivos de paleta (para saber que bloque del monton de C es cada uno).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

func main() {
	dir := os.Args[1]
	entradas, _ := os.ReadDir(dir)
	enDisco := map[string]string{}
	for _, e := range entradas {
		enDisco[strings.ToLower(e.Name())] = e.Name()
	}

	var mpqs []*d2mpq.MPQ

	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq"} {
		if real, hay := enDisco[nombre]; hay {
			m, err := d2mpq.FromFile(dir + "/" + real)
			if err != nil {
				fmt.Println("no abri", real, err)
				os.Exit(1)
			}

			mpqs = append(mpqs, m)
		}
	}

	leer := func(nombre string) ([]byte, error) {
		for _, m := range mpqs {
			if !m.Contains(nombre) {
				continue
			}

			return m.ReadFile(nombre)
		}

		return nil, fmt.Errorf("no esta: %s", nombre)
	}

	for _, ruta := range []string{
		`data\global\palette\act1\pal.dat`,
		`data\global\palette\act1\pal.pl2`,
		`data\global\palette\sky\pal.dat`,
		`data\global\palette\units\pal.dat`,
		`data\global\excel\monstats.txt`,
		`data\global\tiles\act1\outdoors\wild1.ds1`,
	} {
		datos, err := leer(ruta)
		if err != nil {
			fmt.Println(ruta, "->", err)
			continue
		}

		fmt.Println(ruta, "bytes=", len(datos))
	}
}
