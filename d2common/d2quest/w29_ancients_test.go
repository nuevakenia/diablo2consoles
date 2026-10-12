package d2quest

import "testing"

func TestAncientClasses(t *testing.T) {
	for _, c := range []int{540, 541, 542} {
		if !IsAncientClass(c) {
			t.Errorf("class %d is an Ancient", c)
		}
	}

	if IsAncientClass(539) || IsAncientClass(543) {
		t.Error("the statues and the throne Baal are not Ancients")
	}
}

func TestAncientsFightFullRun(t *testing.T) {
	var f AncientsFight

	for i := 0; i < 3; i++ {
		if !f.MayStart() {
			t.Fatalf("statue %d may wake", i)
		}

		f.Wake()
	}

	if f.MayStart() {
		t.Error("a fourth Ancient must not wake")
	}

	if !f.Running() {
		t.Error("the fight runs")
	}

	if f.AncientDied() || f.AncientDied() {
		t.Error("only the third death ends the fight")
	}

	if !f.AncientDied() || !f.Done || f.Running() {
		t.Error("the third death ends the fight")
	}

	if f.AncientDied() {
		t.Error("a death after the end counts nothing")
	}
}

func TestAncientsPortalReset(t *testing.T) {
	var f AncientsFight
	f.Wake()
	f.Wake()
	f.Wake()

	reset, rearm := f.PortalsChanged(1)
	if !reset || rearm || f.Armed || f.Spawned != 0 || f.Deaths != 0 {
		t.Fatalf("a portal resets the fight: reset=%v rearm=%v %+v", reset, rearm, f)
	}

	if f.MayStart() {
		t.Error("no Ancient wakes while a portal is open")
	}

	if reset, rearm = f.PortalsChanged(2); reset || rearm {
		t.Error("a second portal changes nothing")
	}

	if _, rearm = f.PortalsChanged(1); rearm {
		t.Error("one portal still open")
	}

	if reset, rearm = f.PortalsChanged(0); reset || rearm || RiteReArmsOnClose {
		t.Errorf("w40: closing the last portal re-arms nothing (0x58ad30): reset=%v rearm=%v", reset, rearm)
	}

	if f.Armed {
		t.Error("the fight stays unarmed until the altar is used again")
	}

	if !f.MayStart() {
		t.Error("the statues may wake again")
	}
}

func TestAncientsPortalNotArmed(t *testing.T) {
	var f AncientsFight

	f.PortalsChanged(1)

	if _, rearm := f.PortalsChanged(0); rearm {
		t.Error("a fight that was never armed is not re-armed")
	}
}

func TestAncientsPortalAfterDone(t *testing.T) {
	var f AncientsFight
	f.Done = true

	if reset, rearm := f.PortalsChanged(1); reset || rearm {
		t.Error("a finished fight is not reset by portals")
	}

	if f.MayStart() {
		t.Error("a finished fight never wakes the statues")
	}
}

func TestAltarWorks(t *testing.T) {
	var f AncientsFight

	if f.AltarWorks(false, false) {
		t.Error("the altar needs the Rite bits")
	}

	if !f.AltarWorks(true, false) || !f.AltarWorks(false, true) {
		t.Error("done or pending opens the altar")
	}

	f.Wake()

	if f.AltarWorks(true, false) {
		t.Error("not while the fight is on")
	}
}

func TestRiteRewardRecipient(t *testing.T) {
	cases := []struct {
		level, diff int
		there, done bool
		want        bool
	}{
		{20, Normal, true, false, true},
		{19, Normal, true, false, false},
		{39, Nightmare, true, false, false},
		{40, Nightmare, true, false, true},
		{60, Hell, true, false, true},
		{59, Hell, true, false, false},
		{90, Hell, false, false, false},
		{90, Hell, true, true, false},
	}

	for _, c := range cases {
		if got := RiteRewardRecipient(c.level, c.diff, c.there, c.done); got != c.want {
			t.Errorf("level %d diff %d there=%v done=%v: %v, want %v", c.level, c.diff, c.there, c.done, got, c.want)
		}
	}
}
