// scratch: lista los presets de nivel (LvlPrest) del acto 1 con sus archivos, para elegir el ds1 de la
// TRANSICION del POC mapa-103 (DC-002.109). No se commitea.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
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
		clave := strings.TrimPrefix(nombre, "/")
		clave = strings.ReplaceAll(clave, "/", "\\")

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

	rm, err := d2records.NewRecordManager(d2util.LogLevelNone)
	if err != nil {
		fmt.Println("records:", err)
		os.Exit(1)
	}

	for _, t := range []string{d2resource.LevelPreset, d2resource.LevelType} {
		raw, err := leer(t)
		if err != nil {
			fmt.Println("tabla", t, err)
			os.Exit(1)
		}

		dict := d2txt.LoadDataDictionary(raw)

		if err := rm.Load(t, dict); err != nil {
			fmt.Println("load", t, err)
			os.Exit(1)
		}
	}

	ids := make([]int, 0, len(rm.Level.Presets))
	for id := range rm.Level.Presets {
		ids = append(ids, id)
	}

	sort.Ints(ids)

	for _, id := range ids {
		p := rm.Level.Presets[id]
		if p.LevelID > 8 {
			continue
		}

		fmt.Printf("preset %3d levelID %2d (%s) size %dx%d files=%d: %v\n", id, p.LevelID,
			rm.Level.Types[p.LevelID].Name, p.SizeX, p.SizeY, p.FileCount, p.Files)
	}

	fmt.Println("--- level types 0..8")
	for i := 0; i < 9 && i < len(rm.Level.Types); i++ {
		fmt.Printf("type %d = %s\n", i, rm.Level.Types[i].Name)
	}
}
