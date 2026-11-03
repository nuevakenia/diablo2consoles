// automap-moor: la tabla del automapa y el nombre de la zona del nivel de AFUERA (DC-002.144c).
//
// QUE MIDE Y POR QUE, antes de tocar la consola:
//
//  1. `AutoMap.txt` filtrado a `1 Wilderness` (LvlTypes 2, el tipo del nivel de afuera que compone el
//     cruce): cuantas filas y cuantos cuadros de `maximap.dc6` pide. Es el presupuesto de la RECARGA del
//     minimapa: si el afuera no tuviera filas, la recarga daria un mapa vacio en la consola y el numero
//     no se podria leer.
//  2. El nombre del nivel 2 de `levels.txt` por las dos columnas que el motor conoce (`Name`, el
//     identificador, y `LevelName`, la clave del `string.tbl`), que es lo que el HUD va a poner cuando
//     termine el pueblo: el fallback `d2level.DefaultLevelNames` NO tiene la fila 2.
//
// Uso:  go run ./console/dc/medidas/automap-moor [carpeta de los MPQ]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2automap"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2level"
)

func autodetectarD2() string {
	if d := os.Getenv("D2_DIR"); d != "" {
		return d
	}

	return filepath.Join(os.Getenv("HOME"), ".wine-d2", "drive_c", "Games", "Diablo II")
}

func main() {
	dir := autodetectarD2()
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	entradas, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println("no puedo leer la carpeta del juego:", err)
		os.Exit(1)
	}

	enDisco := map[string]string{}
	for _, e := range entradas {
		enDisco[strings.ToLower(e.Name())] = e.Name()
	}

	var mpqs []*d2mpq.MPQ

	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq"} {
		if real, hay := enDisco[nombre]; hay {
			m, err := d2mpq.FromFile(filepath.Join(dir, real))
			if err != nil {
				fmt.Println("no abri", real, err)
				os.Exit(1)
			}

			mpqs = append(mpqs, m)
			fmt.Printf("MPQ %-14s %s\n", real, filepath.Join(dir, real))
		}
	}

	leer := func(nombre string) ([]byte, error) {
		clave := strings.TrimPrefix(nombre, "/")
		clave = strings.ReplaceAll(clave, "/", string(rune(92)))

		var ultimo error

		for _, m := range mpqs {
			if datos, err := m.ReadFile(clave); err == nil {
				return datos, nil
			} else { //nolint:golint
				ultimo = err
			}
		}

		return nil, ultimo
	}

	fmt.Println("\n== 1. AutoMap.txt: las filas del pueblo y las del afuera")

	crudo, err := leer("data/global/excel/AutoMap.txt")
	if err != nil {
		fmt.Println("   AutoMap.txt:", err)
		os.Exit(1)
	}

	fmt.Printf("   archivo: %d B\n", len(crudo))

	for _, nivel := range []int{1, 2} {
		tabla, err := d2automap.ParseTxtLevels(strings.NewReader(string(crudo)), nivel)
		if err != nil {
			fmt.Println("   nivel", nivel, "NO parsea:", err)
			continue
		}

		filas := tabla.LevelRows(nivel)

		visto := map[int]bool{}
		cels := 0

		for _, f := range filas {
			for _, c := range f.Cels {
				if !visto[c] {
					visto[c] = true
					cels++
				}
			}
		}

		bytes := 0
		for _, f := range filas {
			bytes += 32
			for range f.Cels {
				bytes += 8
			}
		}

		fmt.Printf("   nivel= %d (%-14s) filas= %-3d cuadrosDeArte= %-3d payloadAprox= %d B\n",
			nivel, d2automap.LevelName(nivel), len(filas), cels, bytes)

		if len(filas) > 0 {
			fmt.Printf("      primera fila: tile=%d style=%d seq=[%d,%d] cels=%v\n",
				filas[0].Tile, filas[0].Style, filas[0].Start, filas[0].End, filas[0].Cels)
			fmt.Printf("      ultima  fila: tile=%d style=%d seq=[%d,%d] cels=%v\n",
				filas[len(filas)-1].Tile, filas[len(filas)-1].Style, filas[len(filas)-1].Start,
				filas[len(filas)-1].End, filas[len(filas)-1].Cels)
		}
	}

	fmt.Println("\n== 2. levels.txt: el nombre del nivel 2 (el que el HUD va a poner)")

	raw, err := leer("data/global/excel/levels.txt")
	if err != nil {
		fmt.Println("   levels.txt:", err)
		os.Exit(1)
	}

	fmt.Printf("   archivo: %d B\n", len(raw))

	// la tabla es TSV: se lee a mano para no arrastrar el RecordManager entero (que pide todas las tablas)
	lineas := strings.Split(string(raw), "\n")
	if len(lineas) < 2 {
		fmt.Println("   levels.txt: sin filas")
		os.Exit(1)
	}

	cab := strings.Split(strings.TrimRight(lineas[0], "\r"), "	")
	col := map[string]int{}

	for i, c := range cab {
		col[strings.TrimSpace(c)] = i
	}

	fmt.Printf("   columnas: %d\n", len(cab))

	indice := func(nombres ...string) int {
		for _, n := range nombres {
			if i, hay := col[n]; hay {
				return i
			}
		}

		return -1
	}

	iID := indice("Id", "id", "ID")
	iNombre := indice("Name", "Name ")
	iClave := indice("LevelName")
	fmt.Printf("   columnas que importan: id=%d Name=%d LevelName=%d\n", iID, iNombre, iClave)

	for _, id := range []int{1, 2} {
		nombre, clave := "", ""

		for _, l := range lineas[1:] {
			f := strings.Split(strings.TrimRight(l, "\r"), "	")
			if iID < 0 || iID >= len(f) || strings.TrimSpace(f[iID]) != fmt.Sprint(id) {
				continue
			}

			if iNombre >= 0 && iNombre < len(f) {
				nombre = strings.TrimSpace(f[iNombre])
			}

			if iClave >= 0 && iClave < len(f) {
				clave = strings.TrimSpace(f[iClave])
			}

			break
		}

		fmt.Printf("   nivel= %d levels.txt Name= %q LevelName= %q\n", id, nombre, clave)
	}

	fmt.Println("\n== 3. el fallback del motor (d2level.DefaultLevelNames)")

	for _, id := range []int{1, 2} {
		fmt.Printf("   DefaultLevelNames[%d]= %q\n", id, d2level.DefaultLevelNames[id])
	}
}
