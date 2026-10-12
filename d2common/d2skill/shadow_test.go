package d2skill

import "testing"

// Shadow Warrior / Master with the real skills.txt rows ($D2_TABLES, skipped when unset): no bonus at level 1,
// from level 2 every aura slot carries aurastatcalc2 (lvl*par3 = 12*lvl for the Warrior) and life grows
// (lvl-1)*Param1 percent.
func TestShadowBonuses(t *testing.T) {
	reg := loadRealRegistry(t)

	sw := reg.ByName("Shadow Warrior")
	if sw == nil || sw.SrvDoFunc != 49 {
		t.Fatalf("skill %+v", sw)
	}

	env := func(lvl int) *Env { return realEnv(reg, "Shadow Warrior", lvl, nil) }

	if a, p := shadowBonuses(env(1), sw, 1); a != nil || p != nil || shadowHPPct(sw, 1) != 0 {
		t.Error("level 1 gets no bonus")
	}

	a, p := shadowBonuses(env(10), sw, 10)
	if len(a) != 4 || len(p) != 4 {
		t.Fatalf("slots: aura %v passive %v", a, p)
	}

	for _, m := range a {
		if m.Value != 120 {
			t.Errorf("aura %s = %d, want 120 (every slot uses aurastatcalc2)", m.Stat, m.Value)
		}
	}

	for _, m := range p {
		if m.Value != 40 { // passivecalc2 = min(lvl*4,75)
			t.Errorf("passive %s = %d, want 40", m.Stat, m.Value)
		}
	}

	if got := shadowHPPct(sw, 10); got != 9*sw.Params[1] || got != 135 {
		t.Errorf("hp pct %d", got)
	}
}
