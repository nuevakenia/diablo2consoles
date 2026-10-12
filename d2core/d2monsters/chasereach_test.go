package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

func TestApplyChaseReach(t *testing.T) {
	stats2 := d2records.MonStats2{
		"lancer": {MeleeRng: 3},
		"plain":  {MeleeRng: 0},
	}

	for _, tc := range []struct {
		name string
		key  string
		want int
	}{
		{"row with melee range", "lancer", 3},
		{"row with zero", "plain", 0},
		{"missing row", "none", 0},
	} {
		p := &d2monster.Profile{}
		applyChaseReach(p, &d2records.MonStatRecord{ExtraDataKey: tc.key}, stats2)

		if p.ChaseReach != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, p.ChaseReach, tc.want)
		}
	}

	applyChaseReach(nil, nil, nil) // must not panic
}
