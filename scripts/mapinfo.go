//go:build ignore

// mapinfo prints facts of the world a map seed draws that the full-game chain scripts need to plan their route
// (they walk exits, and the levels' neighbours depend on the seed):
//
//	go run scripts/mapinfo.go tomb  <seed>   the level id (66..72) of Tal Rasha's real tomb
//	go run scripts/mapinfo.go jungle <seed>  the Act 3 road from Kurast Docktown to Lower Kurast, e.g. "76 77 78 79" or "76 78 79"
//	                                         (Great Marsh is a dead end for some seeds: the exe's own placement, verified by the
//	                                         drlg goldens)
//
// D2_TABLES must name the extracted tables folder (drlg/patch_d2/Levels.txt, LvlPrest.txt, LvlSub.txt, drlg/bin/...).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2drlg"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2drlg/drlgoutdoor"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: go run scripts/mapinfo.go tomb|jungle <map seed>")
		os.Exit(2)
	}

	seed64, err := strconv.ParseUint(os.Args[2], 10, 32)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	seed := uint32(seed64)

	switch os.Args[1] {
	case "tomb":
		fmt.Println(d2drlg.RealTomb(seed))
	case "jungle":
		root := os.Getenv("D2_TABLES")
		rd := func(p string) []byte {
			b, err := os.ReadFile(filepath.Join(root, "drlg", p))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}

			return b
		}

		tb, err := d2drlg.Load(d2drlg.Raw{Levels: rd("patch_d2/Levels.txt"), LvlPrest: rd("patch_d2/LvlPrest.txt"),
			LvlPrestBin: rd("bin/patch_d2/lvlprest.bin"), LvlSub: rd("patch_d2/LvlSub.txt")})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		w, err := drlgoutdoor.PlaceAct3World(tb, seed, d2drlg.Normal)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		road := []string{"76"}
		if drlgoutdoor.RectsAdjacent(w.Rects[76], w.Rects[77]) && drlgoutdoor.RectsAdjacent(w.Rects[77], w.Rects[78]) {
			road = append(road, "77")
		}

		road = append(road, "78", "79")
		fmt.Println(strings.Join(road, " "))
	}
}
