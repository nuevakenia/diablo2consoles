package d2skill

import "testing"

// Death Sentry (w23): the corpse range is the linear value of the sentry's first skill, and that skill runs
// SRVDO_055 at the sentry's level with the hero as the owner. Real skills.txt rows from $D2_TABLES (skipped when
// unset).
func TestSentryCorpseExplosionCast(t *testing.T) {
	reg := loadRealRegistry(t)
	p := &Pipeline{Skills: reg, Frame: func() int { return 0 }}

	sk := reg.ByName("mon death sentry")
	if sk == nil || sk.SrvDoFunc != 55 {
		t.Fatalf("skill %+v", sk)
	}

	for lvl, want := range map[int]int{0: 0, 1: 10, 5: 14, 20: 29} {
		if got := p.LinearValue(sk.ID, lvl); got != want {
			t.Errorf("LinearValue(%d) = %d, want %d", lvl, got, want)
		}
	}

	u := newUnit(reg, map[string]int{}) // the hero does not hold the monster skill

	tg := Target{Corpse: true, CX: 5, CY: 5, CorpseID: "c1", CorpseHP: 200, CorpseLevel: 1}
	res := p.CastSentrySkill(u, sk.ID, 4, tg)

	if !res.OK || res.Level != 4 || len(res.Effects) != 1 {
		t.Fatalf("res %+v", res)
	}

	ef := res.Effects[0]
	if ef.Kind != "area_hit" || ef.CorpseID != "c1" || ef.Desc == nil || ef.Radius < 1 {
		t.Errorf("effect %+v", ef)
	}

	// no corpse target: refused
	if res := p.CastSentrySkill(u, sk.ID, 4, Target{}); res.OK {
		t.Error("cast without a corpse")
	}

	if res := p.CastSentrySkill(u, sk.ID, 0, tg); res.OK {
		t.Error("level 0")
	}
}
