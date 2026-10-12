package drlgoutdoor

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2drlg"
)

// Wave 43 (full-game chain): the Act 3 outdoor levels are NOT always one road 76 - 77 - 78: for a quarter of the seeds the
// exe's own placement (the drlg goldens, outdoor_act3.json seed 2) puts Great Marsh (77) on a dead end of Spider Forest (76),
// and the road to Lower Kurast leads 76 - 78 - 79. A scripted hero must plan by the seed (scripts/mapinfo.go). What has to
// hold for every seed: every outdoor level 75..83 can be reached from the town over the adjacency links.
func TestAct3EveryLevelReachableForAnySeed(t *testing.T) {
	tb := tables23(t)
	dead := 0

	for seed := uint32(1); seed < 3000; seed += 7 {
		w, err := PlaceAct3World(tb, seed, d2drlg.Normal)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}

		seen := map[int]bool{75: true}
		queue := []int{75}

		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]

			for id := 75; id <= 83; id++ {
				if !seen[id] && rectsAdjacent(w.Rects[cur], w.Rects[id]) {
					seen[id] = true

					queue = append(queue, id)
				}
			}
		}

		for id := 75; id <= 83; id++ {
			if !seen[id] {
				t.Errorf("seed %d: level %d cannot be reached from the town", seed, id)
			}
		}

		if !rectsAdjacent(w.Rects[77], w.Rects[78]) {
			dead++
		}
	}

	if dead == 0 {
		t.Errorf("no seed with Great Marsh on a dead end: the goldens have such seeds (seed 2)")
	}
}
