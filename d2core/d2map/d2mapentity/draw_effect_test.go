package d2mapentity

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

func TestMissileDrawEffect(t *testing.T) {
	for trans, want := range map[int]d2enum.DrawEffect{
		0: d2enum.DrawEffectNormal, 1: d2enum.DrawEffectModulate, 2: d2enum.DrawEffectBurn,
		3: d2enum.DrawEffectNormal, -1: d2enum.DrawEffectNormal,
	} {
		if got := MissileDrawEffect(trans); got != want {
			t.Errorf("trans %d: got %d want %d", trans, got, want)
		}
	}

	// the numbering must equal the exe draw modes 3, 4, 5
	if d2enum.DrawEffectModulate != 3 || d2enum.DrawEffectBurn != 4 || d2enum.DrawEffectNormal != 5 {
		t.Error("DrawEffect enum order differs from the exe draw modes")
	}
}
