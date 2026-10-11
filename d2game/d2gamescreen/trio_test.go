package d2gamescreen

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2uber"
)

func TestTrioRewardWiring(t *testing.T) {
	var (
		tr     d2uber.Trio
		charms []string
		extras []int
	)

	charm := func(c string) { charms = append(charms, c) }
	extra := func(n int) { extras = append(extras, n) }

	for _, class := range []int{d2uber.ClassUberMephisto, 12, d2uber.ClassUberDiablo} {
		if trioReward(&tr, class, 2, charm, extra) {
			t.Fatalf("class %d completed the trio", class)
		}
	}

	if !trioReward(&tr, d2uber.ClassUberBaal, 2, charm, extra) {
		t.Fatal("the third boss must complete the trio")
	}

	if len(charms) != 1 || charms[0] != "cm2" || len(extras) != 1 || extras[0] != 2 {
		t.Fatalf("charms=%v extras=%v", charms, extras)
	}
}
