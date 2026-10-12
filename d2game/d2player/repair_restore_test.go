package d2player

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2equip"
)

func TestRestoredLocs(t *testing.T) {
	prev := map[d2equip.Loc]bool{d2equip.LocTorso: true, d2equip.LocHead: true}
	now := map[d2equip.Loc]bool{d2equip.LocHead: true, d2equip.LocRightHand: true}

	got := restoredLocs(prev, now)
	if len(got) != 1 || got[0] != d2equip.LocTorso {
		t.Fatalf("restoredLocs = %v, want [torso]", got)
	}

	if len(restoredLocs(nil, now)) != 0 {
		t.Fatal("newly broken is not restored")
	}
}

func TestRepairStep(t *testing.T) {
	if n, ok := repairStep(0, 10, 5); !ok || n != 1 {
		t.Fatalf("0/10 -> %d %v", n, ok)
	}

	if n, ok := repairStep(10, 10, 5); ok || n != 10 {
		t.Fatalf("full -> %d %v", n, ok)
	}

	if _, ok := repairStep(3, 10, 0); ok {
		t.Fatal("value 0 must not step")
	}
}
