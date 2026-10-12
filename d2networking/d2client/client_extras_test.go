package d2client

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2calc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// Wave 24: the client extras of Strafe (CLTDO_020) and Lightning Strike (CLTDO_022) once the client can see enemies.
func TestClientExtras(t *testing.T) {
	caster := d2skill.SubPoint{X: 0, Y: 0}
	target := d2skill.SubPoint{X: 10, Y: 0}

	t.Run("strafe fires the burst at a lone enemy", func(t *testing.T) {
		rec := &d2records.SkillRecord{Cltdofunc: 20, Cltmissilea: "arrow", Cltmissileb: "bolt",
			Calc1: d2calc.Compile("3", d2calc.KindSkill), Calc3: d2calc.Compile("3", d2calc.KindSkill),
			Aurarangecalc: d2calc.Compile("20", d2calc.KindSkill)}
		ex := clientExtras{Enemies: []d2skill.ChainTarget{{ID: 7, Pos: target}}}

		shots := planClientShotsWith(rec, 1, caster, target, ex)
		if len(shots) != 3 {
			t.Fatalf("%d arrows, want 3", len(shots))
		}

		for _, s := range shots {
			if s.slot != 1 || s.from != caster || s.to != target {
				t.Errorf("%+v", s)
			}
		}

		if one := planClientShots(rec, 1, caster, target); len(one) != 1 {
			t.Errorf("no enemies seen: %d shots", len(one))
		}
	})

	t.Run("lightning strike chains to the next unit id", func(t *testing.T) {
		rec := &d2records.SkillRecord{Cltdofunc: 22, Cltmissilea: "strike",
			Calc1: d2calc.Compile("6", d2calc.KindSkill), Calc2: d2calc.Compile("4", d2calc.KindSkill)}
		other := d2skill.SubPoint{X: 12, Y: 0}
		ex := clientExtras{Enemies: []d2skill.ChainTarget{{ID: 3, Pos: target}, {ID: 9, Pos: other}, {ID: 5, Pos: d2skill.SubPoint{X: 11, Y: 1}}}}
		ex.TargetID = 3

		shots := planClientShotsWith(rec, 1, caster, target, ex)
		if len(shots) != 2 || shots[1].from != target || shots[1].to != (d2skill.SubPoint{X: 11, Y: 1}) {
			t.Errorf("%+v", shots)
		}
	})
}
