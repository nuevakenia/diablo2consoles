package d2skill

import "testing"

// TestChannelHold follows a held Inferno: the first run is a wind-up, the runs that follow make flames, and a
// pause longer than the window starts over (SRVST_InfernoSetupChannel 0x5c6ef0 and the Do gate 0x5c6d40).
func TestChannelHold(t *testing.T) {
	reg := loadRealRegistry(t)
	id := reg.ByName("Inferno").ID
	frame := 0
	p := &Pipeline{Skills: reg, Frame: func() int { return frame }}
	u := newUnit(reg, map[string]int{"Inferno": 5})
	tg := Target{X: 10, Y: 10}

	if !IsChannelStart(reg.ByName("Inferno")) || !IsChannelStart(reg.ByName("Arctic Blast")) {
		t.Fatal("Inferno and Arctic Blast use start function 11")
	}

	st := p.Start(u, id, tg)
	if !st.OK || !st.ChannelFirst || !p.ChannelOn(u) {
		t.Fatalf("first run: %+v", st)
	}

	if p.channelDo(u) { // wind-up: field 0
		t.Fatal("the first run must not make a flame")
	}

	if p.ChannelOn(u) || !p.ChannelActive(u) {
		t.Fatal("state 12 is off after Do, the list lives on")
	}

	frame = 9
	if st = p.Start(u, id, tg); !st.OK || st.ChannelFirst {
		t.Fatalf("second run inside the window: %+v", st)
	}

	if !p.channelDo(u) {
		t.Fatal("the second run makes a flame")
	}

	// no monstats2 row: the action ends one frame after the Do (0x5c6cb0); a row's InfernoLen byte replaces it
	if got := p.ChannelActionEnd(u); got != frame+1 {
		t.Errorf("action end %d, want %d", got, frame+1)
	}

	p.Opt.InfernoLenOf = func(Unit) (int, bool) { return 14, true }
	p.channelDo(u)

	if got := p.ChannelActionEnd(u); got != frame+14 {
		t.Errorf("action end with InfernoLen 14 = %d, want %d", got, frame+14)
	}

	p.Opt.InfernoLenOf = nil

	frame += ChannelHoldFrames + ChannelAnimSlack + 1
	if p.ChannelActive(u) {
		t.Fatal("the window must end")
	}

	if st = p.Start(u, id, tg); !st.ChannelFirst {
		t.Fatal("after a pause the hold starts over")
	}
}

func TestChannelStartManaGate(t *testing.T) {
	reg := loadRealRegistry(t)
	id := reg.ByName("Inferno").ID
	p := &Pipeline{Skills: reg, Frame: func() int { return 0 }}
	u := newUnit(reg, map[string]int{"Inferno": 1})
	u.mana = 5 << 8 // below startmana 6

	if st := p.Start(u, id, Target{X: 1, Y: 1}); st.OK || st.Reason != ReasonMana {
		t.Fatalf("%+v", st)
	}
}

// The hero's class number (0..6) indexes monstats rows 0..6 in SRVDO_InfernoScheduleAnimEnd (0x5c6cb0); the owner's
// monstats2 holds InfernoLen 1 for all of them (monsters/patch_d2/monstats2.txt under $D2_TABLES).
func TestInfernoLenHeroRows(t *testing.T) {
	hdr := tableRow(t, "monsters/patch_d2/monstats2.txt", -1)
	col := -1

	for i, h := range hdr {
		if h == "InfernoLen" {
			col = i
		}
	}

	if col < 0 {
		t.Fatal("no InfernoLen column")
	}

	for class := 0; class < 7; class++ {
		if got := tableRow(t, "monsters/patch_d2/monstats2.txt", class)[col]; got != "1" {
			t.Errorf("class %d InfernoLen %q, want 1", class, got)
		}
	}
}
