// Medida de DC-002.110: DONDE VIVE Y CUANTO PESA el `.ds1` de un nivel compuesto.
//
// QUE MIDE. El `.ds1` parseado de un nivel (`act1/town/TownN1.ds1` el pueblo, `TownE1.ds1` el de la
// TRANSICION del POC mapa-103) es lo que en DC-002.109 quedo vivo en el semiespacio del recolector despues
// de una recoleccion completa (~413.000 B medidos con `gc_stats` en Flycast). Esta medida dice, en el HOST,
// que piezas son esas: capas x celdas x bytes de una celda, y cuanto crece el monton de Go antes/despues de
// parsear y despues de soltar la referencia (en el host el recolector es EXACTO, asi que la referencia
// soltada se libera: es la prueba de que lo que en la consola lo deja vivo es el barrido conservador de la
// pila de libgodc, no una raiz del motor).
//
// Uso:  go run ./console/dc/medidas/ds1-110 <dir del juego>
//
// No se commitea el resultado: los numeros van a console/dc/evidencia/dc-002.110/.
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("uso: ds1-110 <dir del juego>")
		os.Exit(1)
	}

	dir := os.Args[1]

	var mpqs []*d2mpq.MPQ

	for _, nombre := range []string{"PATCH_D2.MPQ", "D2DATA.MPQ"} {
		m, err := d2mpq.FromFile(dir + "/" + nombre)
		if err != nil {
			fmt.Println("no abri", nombre, err)
			os.Exit(1)
		}

		mpqs = append(mpqs, m)
	}

	leer := func(nombre string) ([]byte, error) {
		clave := strings.ReplaceAll(strings.TrimPrefix(nombre, "/"), "/", "\\")

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

	fmt.Println("sizeof(d2ds1.Tile) =", unsafe.Sizeof(d2ds1.Tile{}), "B")
	fmt.Println()

	for _, ruta := range []string{
		"/data/global/tiles/act1/town/TownN1.ds1",
		"/data/global/tiles/act1/town/TownE1.ds1",
	} {
		fmt.Println("========================", ruta)

		crudo, err := leer(ruta)
		if err != nil {
			fmt.Println("  no pude leerlo:", err)

			continue
		}

		fmt.Println("  archivo crudo =", len(crudo), "B")

		runtime.GC()

		var antes runtime.MemStats

		runtime.ReadMemStats(&antes)

		ds1, err := d2ds1.Unmarshal(crudo)
		if err != nil {
			fmt.Println("  no parseo:", err)

			continue
		}

		runtime.GC()

		var conEl runtime.MemStats

		runtime.ReadMemStats(&conEl)

		ancho, alto := ds1.Size()
		celdas := ancho * alto
		capas := len(ds1.Walls) + len(ds1.Floors) + len(ds1.Shadows) + len(ds1.Substitutions)

		fmt.Printf("  %dx%d = %d celdas; capas: walls=%d floors=%d shadows=%d subs=%d -> %d\n",
			ancho, alto, celdas, len(ds1.Walls), len(ds1.Floors), len(ds1.Shadows), len(ds1.Substitutions), capas)
		fmt.Println("  celdas x 24 B x capas =", celdas*int(unsafe.Sizeof(d2ds1.Tile{}))*capas, "B")
		fmt.Println("  objetos =", len(ds1.Objects), " archivos =", len(ds1.Files),
			" grupos de sustitucion =", len(ds1.SubstitutionGroups))
		fmt.Println("  monton de Go despues de parsear y recolectar = +", int64(conEl.HeapAlloc)-int64(antes.HeapAlloc), "B")

		// la prueba de la RAIZ: se suelta el `.ds1` y se recolecta otra vez. En el host (recolector EXACTO) lo
		// que no tiene raiz se va; en libgodc el barrido de la pila es conservador y lo deja pegado (DC-002.109).
		ds1 = nil
		crudo = nil

		runtime.GC()

		var sinEl runtime.MemStats

		runtime.ReadMemStats(&sinEl)

		fmt.Println("  monton de Go despues de soltarlo y recolectar =",
			int64(sinEl.HeapAlloc)-int64(antes.HeapAlloc), "B (la diferencia es lo que vuelve)")
		fmt.Println()
	}
}
