package d2records

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
)

// The Skeletal Mage (wave 23): SRVDO_149 0x5cc400 adds the mage type 0..3 to srvmissilea, and the shot takes its
// damage from the missile record at the level of the mage's monster skill (sumsk1calc), because the necromage
// missiles carry no Skill column (MISSILE_BuildDamageDescriptor 0x64ca60). Uses D2_TABLES.
func TestRealMageMissileKinds(t *testing.T) {
	rm := loadRealRecords(t)
	mt := rm.MissileTable()

	for kind, want := range []string{"NecroMage1", "NecroMage2", "NecroMage3", "NecroMage4"} {
		if got := d2skill.MageMissile(mt, "necromage1", kind); mt.ByName(got) == nil || mt.ByName(got).Name != mt.ByName(want).Name {
			t.Errorf("kind %d: %q, want %q", kind, got, want)
		}
	}
}

func TestRealMageOwnDamage(t *testing.T) {
	r := realCast(t, "Raise Skeletal Mage", 5)
	sk := r.p.Skills.ByName("Raise Skeletal Mage")

	cast := func(missile string, lvl int) *auditMissile {
		m := r.p.CastTrapOwn(r.u, sk.ID, missile, lvl, 50, 50, d2skill.Target{X: 55, Y: 50, Unit: r.w.foe, UX: 55, UY: 50})
		if m == nil {
			t.Fatalf("no %s", missile)
		}

		return &auditMissile{m.Damage.Poison.Min, m.Damage.Poison.Max, m.Damage.Poison.Len, m.Damage.Cold.Min,
			m.Damage.Fire.Min, m.Damage.Lightning.Min, m.Level}
	}

	// poison mage at monster-skill level 5: EMin 6 + 4 * 1 at HitShift 0, length 500 + 4 * 250 frames
	p := cast("necromage1", 5)
	if p.pMin != 10 || p.pMax != 10 || p.pLen != 1500 || p.lvl != 5 {
		t.Errorf("poison mage %+v", p)
	}

	// the other three types are cold, fire and lightning
	if c := cast("necromage2", 5); c.cold == 0 || c.pMin != 0 {
		t.Errorf("cold mage %+v", c)
	}

	if f := cast("necromage3", 5); f.fire == 0 {
		t.Errorf("fire mage %+v", f)
	}

	if l := cast("necromage4", 5); l.ltng == 0 {
		t.Errorf("lightning mage %+v", l)
	}

	// a higher monster-skill level hits harder
	if hi := cast("necromage1", 9); hi.pMin <= p.pMin {
		t.Errorf("level 9 poison %+v not above level 5 %+v", hi, p)
	}
}

// SRVDO_010 (0x5da2f0): calc1 of the homing skills is a damage percent added to the missile, not a switch; Guided
// Arrow has ln34, Bone Spirit has 0.
func TestRealHomingCalc1IsDamagePercent(t *testing.T) {
	bs := realCast(t, "Bone Spirit", 20)
	if len(bs.res.Missiles) != 1 || bs.res.Missiles[0].Damage.DamagePct != 0 {
		t.Errorf("Bone Spirit %+v", bs.res.Missiles)
	}

	ga := realCast(t, "Guided Arrow", 20)
	if len(ga.res.Missiles) != 1 || ga.res.Missiles[0].Damage.DamagePct <= 0 {
		t.Errorf("Guided Arrow damage percent %+v", ga.res.Missiles)
	}
}

type auditMissile struct {
	pMin, pMax, pLen int32
	cold, fire, ltng int32
	lvl              int
}
