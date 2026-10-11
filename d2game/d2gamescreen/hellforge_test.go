package d2gamescreen

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2quest"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2reward"
)

func TestForgeShowerClock(t *testing.T) {
	f := newForgeShower(1, true, 2, 1234)

	var dropped []string

	drop := func(c string) bool { dropped = append(dropped, c); return true }

	// first pulse is immediate, then one pulse every 0x14 frames
	var pulses, frames int

	for ; frames < 1000; frames++ {
		res, ok, over := f.advance(1, drop)
		if !ok {
			continue
		}

		pulses++

		if over {
			if res.Rune == "" {
				t.Fatal("expansion shower ended without a rune")
			}

			break
		}

		if res.Next != d2reward.HellforgeDelayFrames {
			t.Fatalf("next %d", res.Next)
		}
	}

	// stages 4,3,2,1 drop one item each, then the rune comes with the last pulse
	if pulses != 4 || len(dropped) != 5 {
		t.Fatalf("pulses=%d dropped=%v", pulses, dropped)
	}

	if frames != 3*d2reward.HellforgeDelayFrames {
		t.Fatalf("the shower lasted %d frames, want %d", frames, 3*d2reward.HellforgeDelayFrames)
	}

	hell := d2reward.HellforgeRunes(2)
	found := false

	for _, r := range hell {
		found = found || r == dropped[4]
	}

	if !found {
		t.Fatalf("rune %s not of hell difficulty", dropped[4])
	}
}

func TestForgeShowerEndsSilentlyWithoutItems(t *testing.T) {
	f := newForgeShower(1, true, 0, 1)

	res, ok, over := f.advance(1, func(string) bool { return false })
	if !ok || !over || len(res.Dropped) != 0 || res.Rune != "" {
		t.Fatalf("%+v %v %v", res, ok, over)
	}
}

func TestIsHellforgeSmash(t *testing.T) {
	yes := d2quest.Effect{Kind: d2quest.EffectDeleteItem, Quest: d2quest.QuestHellforge, Code: d2quest.ItemMephistoSoulstone}
	if !isHellforgeSmash(yes) {
		t.Fatal("the soulstone deletion is the smash")
	}

	no := yes
	no.Code = "hfh"

	if isHellforgeSmash(no) {
		t.Fatal("other deletions are not")
	}
}
