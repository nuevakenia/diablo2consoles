package d2monster

import "testing"

type opsMerc struct {
	*fakeMerc
	blocked, town, same bool
	follows             []string
	cleared             []int
}

func (o *opsMerc) ClearState(_ *Brain, s int)        { o.cleared = append(o.cleared, s) }
func (o *opsMerc) FootprintBlocked(*Brain) bool      { return o.blocked }
func (o *opsMerc) InTownRoom(*Brain) bool            { return o.town }
func (o *opsMerc) SameRegion(*Brain, OwnerInfo) bool { return o.same }
func (o *opsMerc) LeaderFollow(_ *Brain, _ OwnerInfo, k int, run bool, sp int) bool {
	r := "w"
	if run {
		r = "r"
	}

	o.follows = append(o.follows, itoa(k)+r+itoa(sp))

	return true
}

func newOpsMerc(dist int) (*opsMerc, *Brain) {
	f, b := newMerc(dist)

	return &opsMerc{fakeMerc: f, same: true}, b
}

// 0x5e4240: follow kinds and speeds per ring and leader mode.
func TestW40HireableFollowKinds(t *testing.T) {
	for _, tc := range []struct {
		dist int
		mode Mode
		want string
	}{
		{30, ModeNeutral, "1r60"},
		{20, ModeWalk, "0w0"},
		{20, ModeRun, "0r60"},
		{20, ModeNeutral, ""},
	} {
		o, b := newOpsMerc(tc.dist)
		o.owner.Mode = tc.mode
		Tick(o, b)

		got := ""
		if len(o.follows) > 0 {
			got = o.follows[0]
		}

		if got != tc.want {
			t.Errorf("dist %d mode %d: %q want %q", tc.dist, tc.mode, got, tc.want)
		}
	}
}

// A walking or running merc returns without scheduling; no owner picks alternate state 6.
func TestW40HireableEarlyOutsAndNoOwner(t *testing.T) {
	o, b := newOpsMerc(30)
	b.Mode = ModeWalk
	Tick(o, b)

	if len(o.follows) != 0 || b.Wake != waitForever {
		t.Errorf("walking merc: follows=%v wake=%d", o.follows, b.Wake)
	}

	o, b = newOpsMerc(8)
	o.noOwner = true
	Tick(o, b)

	if b.Def.Name != "State6" || b.Wake != 10 {
		t.Errorf("no owner: def=%s wake=%d", b.Def.Name, b.Wake)
	}

	o, b = newOpsMerc(8)
	o.noOwner = true
	b.Class = 271
	Tick(o, b)

	if b.Wake != 10 {
		t.Errorf("rogue no owner wake=%d", b.Wake)
	}

	o, b = newOpsMerc(8)
	o.owner.IsPlayer = false
	Tick(o, b)

	if b.Def.Name != "State6" {
		t.Errorf("non-player leader: def=%s", b.Def.Name)
	}
}

// A taunt state is cleared; town rooms skip the attack; another region follows with kind 0 running.
func TestW40HireableIdleBranches(t *testing.T) {
	o, b := newOpsMerc(8)
	o.states[0xc] = true
	Tick(o, b)

	if len(o.cleared) != 1 || o.cleared[0] != 0xc {
		t.Errorf("cleared %v", o.cleared)
	}

	o, b = newOpsMerc(8)
	o.town, o.hasTarget, o.attackOK, o.dist = true, true, true, 2
	Tick(o, b)

	if o.casts != 0 {
		t.Error("attacked in town")
	}

	o, b = newOpsMerc(8)
	o.same = false
	Tick(o, b)

	if len(o.follows) != 1 || o.follows[0] != "0r60" {
		t.Errorf("other region: %v", o.follows)
	}

	// scratch band 0x12: far ring 18, so a walking leader at 17 is not followed (it is with the default 16)
	o, b = newOpsMerc(17)
	b.Scratch[0] = 0x12
	o.owner.Mode = ModeWalk
	Tick(o, b)

	if len(o.follows) != 0 {
		t.Errorf("band: %v", o.follows)
	}
}
