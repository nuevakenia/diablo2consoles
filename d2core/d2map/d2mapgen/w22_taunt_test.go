package d2mapgen

import (
	"strings"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2drlg/drlgoutdoor"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
)

// Where the Baal Taunt comes from (wave 22): Game.exe has no creator of the class, the DS1 rooms of the Worldstone Keep
// that hold the stairs up (Expansion/BaalLair/baal*up0?.ds1) carry a preset monster named baaltaunt. The real population
// path (real_pop.go createPreset) turns those nodes into NPC placements and the monster Director adopts them.
// Needs D2_GAME_DIR.
func TestBaalTauntPresetInStairsRooms(t *testing.T) {
	a := perfAssets(t)
	seen := map[string]bool{}
	found := 0

	for _, pr := range a.Records.Level.Presets {
		for _, f := range pr.Files {
			if f == "" || seen[f] || !strings.Contains(strings.ToLower(f), "baallair") {
				continue
			}

			seen[f] = true

			data, err := a.LoadFile("/data/global/tiles/" + drlgoutdoor.NormalizePrestFile(f))
			if err != nil {
				continue
			}

			ds, err := d2ds1.Unmarshal(data)
			if err != nil {
				continue
			}

			for _, o := range ds.Objects {
				if p := a.Records.Monster.Presets[ds.Act]; o.Type == 1 && o.ID >= 0 && o.ID < len(p) &&
					strings.EqualFold(p[o.ID], "baaltaunt") {
					found++

					if !strings.Contains(strings.ToLower(f), "up0") {
						t.Errorf("%s holds a Baal Taunt but is not a stairs room", f)
					}
				}
			}
		}
	}

	if found == 0 {
		t.Error("no Baal Taunt preset in the Worldstone Keep rooms")
	}
}
