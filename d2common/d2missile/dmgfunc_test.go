package d2missile

import "testing"

func TestStunLength(t *testing.T) {
	for _, c := range []struct{ p1, p2, lvl, want int }{{40, 15, 1, 40}, {40, 15, 3, 70}, {40, 15, 20, 325}, {40, 15, 0, 0}} {
		if got := StunLength(c.p1, c.p2, c.lvl); got != c.want {
			t.Errorf("level %d: %d, want %d", c.lvl, got, c.want)
		}
	}
}

// SrvDmgFunc 7 (0x5aaef0, Shock Wave): the hit carries a stun of Param1 + (level-1) * Param2 frames of the missile's
// skill and the hit class 0x60.
func TestDmgFunc7SetsStunAndHitClass(t *testing.T) {
	w := newWorld()
	sp := &Spec{ID: 511, Name: "shockwave", SrvDoFunc: 1, SrvDmgFunc: 7, Vel: 20, MaxVel: 20, Range: 14, CollideType: 3,
		CollideKill: true, LastCollide: true, Size: 1}
	s := NewSim(w, specs{"shockwave": sp})
	evs := collect(s)

	w.targets = append(w.targets, newTarget("mob", 45, 0))

	_, err := s.Create(CreateParams{Spec: sp, Owner: ownerWith(0), SkillID: 243, Level: 3,
		Skill: SkillInfo{Param: [7]int{0, 40, 15}}, Damage: DamageDesc{PhysMin: 256, PhysMax: 256},
		X: 40.5, Y: 0.5, DestX: 60, DestY: 0.5})
	if err != nil {
		t.Fatal(err)
	}

	stepN(s, w, 14)

	hits := 0

	for _, e := range *evs {
		if e.Kind != EventHit {
			continue
		}

		hits++

		if e.Damage.StunLen != 70 || e.Damage.HitClass != 0x60 {
			t.Errorf("stun %d class %#x, want 70 and 0x60", e.Damage.StunLen, e.Damage.HitClass)
		}
	}

	if hits != 1 {
		t.Fatalf("%d hits", hits)
	}
}
