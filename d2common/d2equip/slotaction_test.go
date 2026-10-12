package d2equip

import "testing"

func TestSlotActionFor(t *testing.T) {
	r := testRules(t)
	hero := Hero{Class: ClassPaladin, Str: 200, Dex: 200, Level: 99}
	sword, staff := weapon("ssd", "swor", false), weapon("sst", "staf", true)
	shield, helm := armor("buc", "shie"), armor("cap", "helm")
	weak := Hero{Class: ClassPaladin, Str: 10, Dex: 10, Level: 1}
	heavy := armor("plt", "tors")
	heavy.ReqStr = 100

	cases := []struct {
		name          string
		body          map[Loc]*Item
		cursor        *Item
		loc           Loc
		fits, stackOK bool
		hero          Hero
		want          SlotAction
	}{
		{"pick up a helm", map[Loc]*Item{LocHead: helm}, nil, LocHead, true, false, hero, ActionPickUp},
		{"empty slot, empty cursor", nil, nil, LocHead, true, false, hero, ActionNone},
		{"helm into an empty head", nil, helm, LocHead, true, false, hero, ActionPlace},
		{"helm swaps a helm", map[Loc]*Item{LocHead: helm}, helm, LocHead, true, false, hero, ActionSwap},
		{"helm on the feet", nil, helm, LocFeet, true, false, hero, ActionNone},
		{"too weak", nil, heavy, LocTorso, true, false, weak, ActionNone},
		{"shield next to a sword", map[Loc]*Item{LocRightHand: sword}, shield, LocLeftHand, true, false, hero, ActionPlace},
		{"two-hander over a shield, hand empty", map[Loc]*Item{LocLeftHand: shield}, staff, LocRightHand, true, false, hero, ActionPlaceMoveOther},
		{"shield next to a two-hander", map[Loc]*Item{LocRightHand: staff}, shield, LocLeftHand, true, false, hero, ActionPlaceMoveOther},
		{"two-hander replaces a sword, shield displaced, room", map[Loc]*Item{LocRightHand: sword, LocLeftHand: shield}, staff, LocRightHand, true, false, hero, ActionSwapMoveOther},
		{"same without room", map[Loc]*Item{LocRightHand: sword, LocLeftHand: shield}, staff, LocRightHand, false, false, hero, ActionNone},
		{"sword replaces a sword", map[Loc]*Item{LocRightHand: sword}, weapon("axe", "axe", false), LocRightHand, true, false, hero, ActionSwap},
		{"click the empty off hand under a two-hander", map[Loc]*Item{LocRightHand: staff}, nil, LocLeftHand, true, false, hero, ActionPickUpTwoHander},
		{"arrows onto arrows", map[Loc]*Item{LocRightHand: weapon("bow", "bow", true), LocLeftHand: armor("aqv", "bowq")}, armor("aqv", "bowq"), LocLeftHand, true, true, hero, ActionCombine},
	}

	for _, c := range cases {
		if got := r.SlotActionFor(c.hero, c.body, c.cursor, c.loc, false, c.fits, c.stackOK); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
}

func TestRefusalVoice(t *testing.T) {
	r := testRules(t)
	hero := Hero{Class: ClassPaladin, Str: 200, Dex: 200, Level: 99}
	weak := Hero{Class: ClassPaladin, Str: 10, Dex: 10, Level: 1}
	heavy := armor("plt", "tors")
	heavy.ReqStr = 100
	unid := armor("cap", "helm")
	unid.Identified = false
	staff, sword, shield := weapon("sst", "staf", true), weapon("ssd", "swor", false), armor("buc", "shie")

	cases := []struct {
		name   string
		hero   Hero
		body   map[Loc]*Item
		cursor *Item
		loc    Loc
		fits   bool
		want   int
	}{
		{"strength missing: cantuseyet", weak, nil, heavy, LocTorso, true, VoiceCantUse},
		{"unidentified: cantuseyet", hero, nil, unid, LocHead, true, VoiceCantUse},
		{"wrong class: impossible", hero, nil, armor("ba3", "phlm"), LocHead, true, VoiceImpossible},
		{"wrong slot: impossible", hero, nil, armor("cap", "helm"), LocFeet, true, VoiceImpossible},
		{"no room for the displaced shield: impossible", hero, map[Loc]*Item{LocRightHand: sword, LocLeftHand: shield}, staff, LocRightHand, false, VoiceImpossible},
		{"allowed: no line", hero, nil, armor("cap", "helm"), LocHead, true, 0},
		{"empty cursor: no line", hero, nil, nil, LocHead, true, 0},
	}

	for _, c := range cases {
		if got := r.RefusalVoice(c.hero, c.body, c.cursor, c.loc, c.fits, false); got != c.want {
			t.Errorf("%s: voice %#x want %#x", c.name, got, c.want)
		}
	}
}
