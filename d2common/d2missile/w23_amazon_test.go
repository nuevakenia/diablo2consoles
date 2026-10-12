package d2missile

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2calc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2combat"
)

// plagueSpecs are the rows of the shipped missiles.txt that matter for Plague Javelin.
func plagueSpecs() (*Spec, *Spec, specs) {
	cloud := &Spec{ID: 221, Name: "plaguejavcloud", SrvDoFunc: 3, Param: [5]int{2, 4}, Range: 60, SubLoop: true,
		SubStart: 10, SubStop: 19, CollideType: 3, Size: 2}
	jav := &Spec{ID: 43, Name: "plaguejavelin", SrvDoFunc: 3, SrvHitFunc: 2, SHitPar: [3]int{1, 2, 3}, Vel: 24, Range: 15,
		CollideType: 3, AlwaysExplode: true, HitSubMissile: [4]string{"plaguejavcloud"}}

	return jav, cloud, specs{"plaguejavelin": jav, "plaguejavcloud": cloud}
}

// Hit function 2 (0x5a7810 -> 0x5a6d70, VERIFIED wave 23): a Plague Javelin leaves 8 slow clouds (every second of
// the 16 ring offsets, Param1 << 7) and 15 fast ones (offsets 1..15, Param2 << 7), living Range plus three
// sub loops, each drifting to its offset of the ring around the struck unit.
func TestPlagueJavelinMakesTheCloudFan(t *testing.T) {
	jav, _, tab := plagueSpecs()
	w := newWorld()
	s := NewSim(w, tab)
	m, _ := s.Create(CreateParams{Spec: jav, Level: 1, X: 10.5, Y: 3.5, DestX: 30, DestY: 3.5, Owner: Owner{ID: "hero"}})

	before := len(s.pending) + len(s.missiles)
	ret, ok := s.hitFunc(m, nil)

	if !ok || ret != resKill|resDamage {
		t.Fatalf("ret %d ok %v", ret, ok)
	}

	var clouds []*Missile

	for _, c := range append(append([]*Missile{}, s.missiles...), s.pending...) {
		if c.Spec.Name == "plaguejavcloud" {
			clouds = append(clouds, c)
		}
	}

	if len(clouds) != 23 || len(s.pending)+len(s.missiles)-before != 23 {
		t.Fatalf("clouds %d", len(clouds))
	}

	slow, fast := 0, 0
	dests := map[[2]float64]int{}

	for _, c := range clouds {
		switch c.Velocity {
		case 2 << 7:
			slow++
		case 4 << 7:
			fast++
		default:
			t.Errorf("velocity %d", c.Velocity)
		}

		if c.Total != 60+(19-10)*3 {
			t.Errorf("life %d", c.Total)
		}

		dx, dy := c.Dest()
		dests[[2]float64{dx - 10.5, dy - 3.5}]++
	}

	if slow != 8 || fast != 15 {
		t.Errorf("slow %d fast %d", slow, fast)
	}

	// the 16 ring offsets: even indices once (slow), odd indices twice would need ring 2 on even ones too, so
	// the slow ring covers 0,2,4..14 and the fast one 1..15: every offset of the ring once, plus offset 1..15 even
	// ones a second time
	if dests[[2]float64{0, 2}] != 1 || dests[[2]float64{1, 2}] != 1 || dests[[2]float64{2, 2}] != 2 {
		t.Errorf("dests %v", dests)
	}
}

// The centre of the fan is the struck unit.
func TestCloudFanCentresOnTheTarget(t *testing.T) {
	jav, _, tab := plagueSpecs()
	w := newWorld()
	s := NewSim(w, tab)
	m, _ := s.Create(CreateParams{Spec: jav, Level: 1, X: 10.5, Y: 3.5, DestX: 30, DestY: 3.5, Owner: Owner{ID: "hero"}})
	tg := &posTarget{fakeTarget: newTarget("m", 20, 5)}
	tg.px, tg.py = 20.5, 5.5

	s.cloudFan(m, tg)

	found := false

	for _, c := range append(append([]*Missile{}, s.missiles...), s.pending...) {
		if c.Spec.Name == "plaguejavcloud" && c.X == 20.5 && c.Y == 5.5 {
			found = true
		}
	}

	if !found {
		t.Fatal("no cloud starts at the target")
	}
}

// A sub missile without a drift speed stays where it is made (create velocity 0), and ring 2 is off when
// sHitPar1 is 0 (the gas potions of the table have one ring).
func TestCloudFanOneRingAndStationary(t *testing.T) {
	cloud := &Spec{ID: 1, Name: "gas", SrvDoFunc: 3, Range: 30}
	pot := &Spec{ID: 2, Name: "pot", SrvHitFunc: 2, SHitPar: [3]int{0, 2, 3}, Range: 10, HitSubMissile: [4]string{"gas"}}
	s := NewSim(newWorld(), specs{"gas": cloud, "pot": pot})
	m, _ := s.Create(CreateParams{Spec: pot, Level: 1, X: 5.5, Y: 5.5, DestX: 9, DestY: 5.5, Owner: Owner{ID: "h"}})

	s.cloudFan(m, nil)

	n := 0

	for _, c := range append(append([]*Missile{}, s.missiles...), s.pending...) {
		if c.Spec.Name == "gas" {
			n++

			if c.Velocity != 0 {
				t.Errorf("velocity %d", c.Velocity)
			}
		}
	}

	if n != 8 {
		t.Errorf("clouds %d", n)
	}
}

func dmgSpec(fn int, etype, calc string, p1, p2 int) *Spec {
	sp := &Spec{ID: 9, Name: "m", SrvDmgFunc: fn, EType: etype, DParam: [2]int{p1, p2}}
	if calc != "" {
		sp.DmgCalc1 = d2calc.Compile(calc, d2calc.KindMissile)
	}

	return sp
}

func newDmgMissile(t *testing.T, sp *Spec, lvl int) (*Sim, *Missile) {
	t.Helper()

	s := NewSim(newWorld(), nil)
	m, err := s.Create(CreateParams{Spec: sp, Level: lvl, X: 1.5, Y: 1.5, DestX: 9, DestY: 1.5, Owner: Owner{ID: "h"}})
	if err != nil {
		t.Fatal(err)
	}

	return s, m
}

// SrvDmgFunc 1 (0x5aaba0, VERIFIED): dl12 percent of the physical damage moves to the element of the missile.
// Fire Arrow level 5: 3 + 4 * 2 = 11 percent.
func TestDamageFunc1FireArrowConvertsPhysicalToFire(t *testing.T) {
	s, m := newDmgMissile(t, dmgSpec(1, "fire", "dl12", 3, 2), 5)
	d := d2combat.Damage{Physical: 1000 << 8, Fire: 7 << 8}

	s.dmgFunc(m, nil, &d)

	moved := int32(1000<<8) * 11 / 100
	if d.Physical != 1000<<8-moved || d.Fire != 7<<8+moved {
		t.Errorf("phys %d fire %d (moved %d)", d.Physical, d.Fire, moved)
	}
}

// Magic Arrow converts to magic, Cold Arrow to cold, and the percent is clamped to 100 and to the damage there is.
func TestDamageFunc1TypesAndClamps(t *testing.T) {
	cases := []struct {
		etype string
		get   func(*d2combat.Damage) int32
	}{
		{"mag", func(d *d2combat.Damage) int32 { return d.Magic }},
		{"cold", func(d *d2combat.Damage) int32 { return d.Cold }},
		{"pois", func(d *d2combat.Damage) int32 { return d.Poison }},
		{"ltng", func(d *d2combat.Damage) int32 { return d.Lightning }},
	}

	for _, c := range cases {
		s, m := newDmgMissile(t, dmgSpec(1, c.etype, "dl12", 90, 5), 4) // 105 percent, clamped to 100
		d := d2combat.Damage{Physical: 500 << 8}
		s.dmgFunc(m, nil, &d)

		if d.Physical != 0 || c.get(&d) != 500<<8 {
			t.Errorf("%s: phys %d elem %d", c.etype, d.Physical, c.get(&d))
		}
	}

	// no calc, or a calc of 0: nothing happens
	s, m := newDmgMissile(t, dmgSpec(1, "fire", "", 3, 2), 5)
	d := d2combat.Damage{Physical: 100 << 8}
	s.dmgFunc(m, nil, &d)

	if d.Physical != 100<<8 || d.Fire != 0 {
		t.Errorf("without calc: %+v", d)
	}
}

// SrvDmgFunc 12 (0x5aac30, VERIFIED): Lightning Bolt (dParam1 100) turns the whole physical blow into lightning,
// keeps the lightning the descriptor rolled, and clears every other field and the ignore flags.
func TestDamageFunc12LightningBoltMakesTheBlowLightning(t *testing.T) {
	s, m := newDmgMissile(t, dmgSpec(12, "ltng", "dl12", 100, 0), 3)
	d := d2combat.Damage{Flags: d2combat.DamageFlagIgnoreUndead | d2combat.DamageFlagNoLifeLeech, Physical: 40 << 8,
		Lightning: 10 << 8, Fire: 5 << 8, Poison: 9 << 8}

	s.dmgFunc(m, nil, &d)

	if d.Physical != 0 || d.Fire != 0 || d.Poison != 0 || d.Lightning != 50<<8 {
		t.Errorf("%+v", d)
	}

	if d.Flags != d2combat.DamageFlagNoLifeLeech {
		t.Errorf("flags %#x", d.Flags)
	}
}

// SrvDmgFunc 2 (0x5aacf0, VERIFIED): Ice Arrow turns the cold length into freeze length (dParam1 percent) and
// clears the chill length.
func TestDamageFunc2IceArrowFreezes(t *testing.T) {
	s, m := newDmgMissile(t, dmgSpec(2, "", "", 100, 0), 1)
	d := d2combat.Damage{ColdLen: 80}

	s.dmgFunc(m, nil, &d)

	if d.FreezeLen != 80 || d.ColdLen != 0 {
		t.Errorf("%+v", d)
	}

	s, m = newDmgMissile(t, dmgSpec(2, "", "", 50, 0), 1)
	d = d2combat.Damage{ColdLen: 80}
	s.dmgFunc(m, nil, &d)

	if d.FreezeLen != 40 {
		t.Errorf("half: %+v", d)
	}
}

// SrvDmgFunc 3 (0x5aad50, VERIFIED): one roll of the missile's own seed, value & 0x7f below dParam1 (19) sets the
// soft hit bit 0x4000.
func TestDamageFunc3SoftHit(t *testing.T) {
	s, m := newDmgMissile(t, dmgSpec(3, "fire", "", 19, 0), 1)
	probe := m.seed
	soft, n := 0, 2000

	for i := 0; i < n; i++ {
		want := probe.Step()&0x7f < 19
		d := d2combat.Damage{}
		s.dmgFunc(m, nil, &d)

		if got := d.Result&ResultSoftHit != 0; got != want {
			t.Fatalf("roll %d: got %v want %v", i, got, want)
		}

		if want {
			soft++
		}
	}

	if soft < n*19/128/2 || soft > n*19/128*2 {
		t.Errorf("soft hits %d of %d", soft, n)
	}
}

// Create flag 0x800: the Activate override (Multiple Shot passes calc2) replaces the Activate column.
func TestCreateActivateOverride(t *testing.T) {
	sp := fireBolt()
	sp.Activate = 3
	s := NewSim(newWorld(), nil)

	a, _ := s.Create(CreateParams{Spec: sp, Level: 1, DestX: 30})
	b, _ := s.Create(CreateParams{Spec: sp, Level: 1, DestX: 30, Activate: 1, HasActivate: true})

	if a.CollideFrom != a.Life-3 || b.CollideFrom != b.Life-1 {
		t.Errorf("collide from %d/%d life %d/%d", a.CollideFrom, b.CollideFrom, a.Life, b.Life)
	}
}
