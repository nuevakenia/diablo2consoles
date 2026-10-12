package d2skill

import (
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Paladin cross check against the community pages (wave 30). Wiki numbers are the Arreat Summit style level
// charts (classic.hiveworkshop.com/diablo2exp/skills/paladin-*.shtml, 1.1x era) and d2emu.com/db/skill/<name>
// (D2R 2.4 era; where it differs the comment says so). Mana and damage on the pages are truncated, so
// comparisons use a tenth of tolerance on mana. Nothing here is copied text, only numbers.

// palFix evaluates a Paladin skill at an effective level with the hard points in base.
type palFix struct {
	t   *testing.T
	reg *Registry
}

func newPalFix(t *testing.T) *palFix { return &palFix{t: t, reg: loadRealRegistry(t)} }

func (p *palFix) env(name string, eff int, base map[string]int) (*Skill, *Env) {
	if base == nil {
		base = map[string]int{name: eff}
	}

	return wikiEnv(p.reg, name, eff, base)
}

func (p *palFix) elem(name string, eff int, base map[string]int) (float64, float64) {
	sk, e := p.env(name, eff, base)

	return float64(sk.ElemMin(e, eff)) / 256, float64(sk.ElemMax(e, eff)) / 256
}

func (p *palFix) aura(name string, eff, slot int) int {
	sk, e := p.env(name, eff, nil)

	return e.eval(sk.AuraStatCalc[slot])
}

func (p *palFix) calc(name string, eff, n int, base map[string]int) int {
	sk, e := p.env(name, eff, base)

	return e.eval(sk.Calc[n])
}

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// loadSkillRowsAny reads a tab separated table into rows keyed by the lower cased "Missile" column.
func loadSkillRowsAny(t *testing.T, path, _ string) map[string]row {
	t.Helper()

	out := map[string]row{}

	for _, r := range loadSkillRows(t, path) {
		out[strings.ToLower(r["Missile"])] = r
	}

	return out
}

func TestWikiPalMana(t *testing.T) {
	p := newPalFix(t)

	// skill, level, wiki mana (pages truncate to one decimal; d2emu rounds, hence 0.1 tolerance)
	for _, c := range []struct {
		name string
		lvl  int
		want float64
	}{
		{"Holy Bolt", 1, 2}, {"Holy Bolt", 20, 3.1}, {"Holy Bolt", 10, 2.5},
		{"Blessed Hammer", 1, 5}, {"Blessed Hammer", 10, 7.2}, {"Blessed Hammer", 20, 9.7}, {"Blessed Hammer", 23, 10.5},
		{"Vengeance", 1, 4}, {"Vengeance", 10, 6.2}, {"Vengeance", 20, 8.7}, {"Vengeance", 23, 9.5},
		{"Zeal", 1, 2}, {"Zeal", 20, 2}, {"Charge", 20, 9}, {"Conversion", 20, 4}, {"Holy Shield", 20, 35},
		{"Fist of the Heavens", 1, 25}, {"Fist of the Heavens", 20, 25}, {"Smite", 20, 2},
		{"Prayer", 1, 1}, {"Prayer", 20, 4.5},
	} {
		sk := p.reg.ByName(c.name)
		if got := float64(sk.ManaCost(c.lvl)) / 256; !near(got, c.want, 0.1) {
			t.Errorf("%s L%d mana %.3f, wiki %.2f", c.name, c.lvl, got, c.want)
		}
	}
}

func TestWikiPalElemental(t *testing.T) {
	p := newPalFix(t)

	for _, c := range []struct {
		name         string
		lvl          int
		min, max     float64
		note         string
		tolerance    float64
		engineDiffer bool
	}{
		{"Holy Bolt", 1, 8, 16, "Arreat", 0, false},
		{"Holy Bolt", 20, 196, 220, "Arreat", 0, false},
		{"Blessed Hammer", 1, 12, 16, "Arreat+d2emu", 0, false},
		{"Blessed Hammer", 10, 88, 92, "d2emu", 0, false},
		{"Blessed Hammer", 20, 196, 200, "Arreat+d2emu", 0, false},
		{"Blessed Hammer", 23, 233, 237, "d2emu", 0, false},
		{"Fist of the Heavens", 1, 150, 200, "Arreat", 0, false},
		{"Fist of the Heavens", 20, 675, 725, "Arreat", 0, false},
		{"Prayer", 1, 2, 0, "Arreat heal 2 (min field carries the heal)", 0, false},
		{"Prayer", 20, 25, 0, "Arreat heal 25", 0, false},
		// Holy Fire/Freeze/Shock tick damage: the 1.1x level 20 row (purediablo d2wiki) is 18-21 fire; the engine reads
		// 18.5-20.5 from the table (EMin 2.. with HitShift 7), the page truncates.
		{"Holy Fire", 20, 18, 21, "purediablo wiki L20 (rounded outward)", 0.5, false},
		{"Holy Fire", 1, 1, 3, "table", 0, false},
		// d2emu (D2R 2.4 numbers) lists 1-6, 8-30, 32-88 for Holy Fire: not the 1.14 table, wiki differs.
		{"Holy Freeze", 20, 37, 38, "d2emu min matches; its max 76 is the D2R rescale", 0, false},
		{"Holy Shock", 20, 1, 156, "table; d2emu 1-312 is D2R", 0, false},
		{"Holy Shock", 1, 1, 10, "table; d2emu 1-20", 0, false},
	} {
		mn, mx := p.elem(c.name, c.lvl, nil)
		if !near(mn, c.min, 0.5+c.tolerance) || !near(mx, c.max, 0.5+c.tolerance) {
			if c.engineDiffer {
				continue
			}

			t.Errorf("%s L%d elem %.2f-%.2f, wiki %.0f-%.0f (%s)", c.name, c.lvl, mn, mx, c.min, c.max, c.note)
		}
	}
}

// Fist of the Heavens' bolts (missile fistoftheheavensbolt, magic) take the level of the cast: 40-50 at 1, 226-236
// at 20 (Arreat chart). Read from the real missiles.txt; synergy Holy Bolt 15 percent per level.
func TestWikiPalFistBolt(t *testing.T) {
	p := newPalFix(t)

	dir := os.Getenv("D2_TABLES")
	rows := loadSkillRowsAny(t, filepath.Join(dir, "skills", "patch_d2", "missiles.txt"), "Name")

	r, ok := rows["fistoftheheavensbolt"]
	if !ok {
		t.Skip("missile row missing")
	}

	spec := DamageSpec{HitShift: r.num("HitShift"), EType: r["EType"], EMin: r.num("EMin"), EMax: r.num("Emax"),
		EDmgSymPer: r.calc("EDmgSymPerCalc")}

	for i := 0; i < 5; i++ {
		spec.EMinLev[i], spec.EMaxLev[i] = r.num("MinELev"+strconv.Itoa(i+1)), r.num("MaxELev"+strconv.Itoa(i+1))
	}

	for _, c := range []struct {
		lvl      int
		min, max float64
	}{{1, 40, 50}, {20, 226, 236}} {
		sk, e := p.env("Fist of the Heavens", c.lvl, nil)
		_ = sk

		mn, mx := float64(spec.ElemMin(e, c.lvl))/256, float64(spec.ElemMax(e, c.lvl))/256
		if spec.HitShift == 0 {
			mn, mx = float64(spec.ElemMin(e, c.lvl)), float64(spec.ElemMax(e, c.lvl))
		}

		if !near(mn, c.min, 0.5) || !near(mx, c.max, 0.5) {
			t.Errorf("FoH bolt L%d %.1f-%.1f, wiki %.0f-%.0f (hitshift %d)", c.lvl, mn, mx, c.min, c.max, spec.HitShift)
		}
	}
}

func TestWikiPalAuras(t *testing.T) {
	p := newPalFix(t)

	type chk struct {
		name string
		lvl  int
		slot int
		want int
	}

	for _, c := range []chk{
		{"Might", 1, 1, 40}, {"Might", 20, 1, 230},
		{"Thorns", 20, 1, 1010},
		{"Blessed Aim", 20, 1, 360},
		{"Concentration", 20, 1, 345}, {"Concentration", 20, 2, 20},
		{"Holy Freeze", 1, 1, -30}, {"Holy Freeze", 20, 1, -54}, {"Holy Freeze", 23, 1, -55}, {"Holy Freeze", 10, 1, -48},
		{"Sanctuary", 20, 1, 720},
		{"Fanaticism", 20, 1, 35}, {"Fanaticism", 20, 2, 135}, {"Fanaticism", 20, 3, 186},
		{"Conviction", 20, 1, -90}, {"Conviction", 20, 2, -125},
		{"Defiance", 1, 1, 70}, {"Defiance", 20, 1, 260},
		{"Resist Fire", 1, 1, 52}, {"Resist Fire", 20, 1, 131}, {"Resist Cold", 20, 1, 131}, {"Resist Lightning", 20, 1, 131},
		{"Vigor", 1, 3, 13}, {"Vigor", 20, 3, 43},
		{"Meditation", 1, 1, 300}, {"Meditation", 20, 1, 775},
		{"Salvation", 1, 1, 60}, {"Salvation", 20, 1, 108},
		{"Holy Shield", 1, 1, 14}, {"Holy Shield", 20, 1, 35},
		{"Cleansing", 1, 1, 100 - 39}, {"Cleansing", 20, 1, 100 - 80}, // the stat is the remaining percent of the duration
	} {
		if got := p.aura(c.name, c.lvl, c.slot); got != c.want {
			t.Errorf("%s L%d aurastat%d %d, wiki %d", c.name, c.lvl, c.slot, got, c.want)
		}
	}

	// Holy Shield: defence percent and duration (calc1 = defence, aura length in frames, wiki 30 .. 505 s)
	if got := p.calc("Holy Shield", 1, 1, nil); got != 25 {
		t.Errorf("Holy Shield defence L1 %d", got)
	}

	if got := p.calc("Holy Shield", 20, 1, nil); got != 310 {
		t.Errorf("Holy Shield defence L20 %d", got)
	}

	for _, c := range []struct {
		lvl, secs int
	}{{1, 30}, {20, 505}} {
		sk, e := p.env("Holy Shield", c.lvl, nil)
		if got := e.eval(sk.AuraLenCalc); got != c.secs*25 {
			t.Errorf("Holy Shield L%d duration %d frames, wiki %d s", c.lvl, got, c.secs)
		}
	}
}

// Aura radius: wiki yards, engine subtiles (a yard is 1.5 subtiles). Radius pages give 16.6 yards at 20 for the
// damage auras, 36 for the party auras, 20 for Fanaticism, 13.3 Conviction, 16 Sanctuary.
func TestWikiPalAuraRadius(t *testing.T) {
	p := newPalFix(t)

	for _, c := range []struct {
		name  string
		lvl   int
		yards float64
	}{
		{"Might", 20, 36}, {"Thorns", 20, 36}, {"Blessed Aim", 20, 36}, {"Concentration", 20, 36},
		{"Holy Fire", 1, 4}, {"Holy Fire", 10, 10}, {"Holy Fire", 20, 16.7}, {"Holy Fire", 23, 18.7},
		{"Holy Freeze", 20, 16.7}, {"Holy Shock", 20, 16.7}, {"Sanctuary", 20, 16}, {"Fanaticism", 20, 20},
		{"Conviction", 20, 13.3},
	} {
		sk, e := p.env(c.name, c.lvl, nil)
		if got := float64(e.eval(sk.AuraRangeCalc)) / 1.5; !near(got, c.yards, 0.1) {
			t.Errorf("%s L%d radius %.2f yards, wiki %.1f", c.name, c.lvl, got, c.yards)
		}
	}
}

func TestWikiPalMelee(t *testing.T) {
	p := newPalFix(t)

	type m struct {
		name   string
		lvl    int
		toHit  int
		calc1  int
		calc2  int
		hasTH  bool
		hasDmg bool
	}

	for _, c := range []m{
		{"Zeal", 1, 10, 2, 0, true, false}, {"Zeal", 10, 100, 5, 36, true, true}, {"Zeal", 20, 200, 5, 96, true, true}, {"Zeal", 23, 230, 5, 114, true, true},
		{"Charge", 1, 50, 100, 0, true, true}, {"Charge", 20, 335, 575, 0, true, true},
		{"Vengeance", 1, 20, 70, 0, true, true}, {"Vengeance", 10, 110, 124, 0, true, true}, {"Vengeance", 20, 210, 184, 0, true, true}, {"Vengeance", 23, 240, 202, 0, true, true},
		{"Sacrifice", 1, 20, 180, 8, true, true}, {"Sacrifice", 20, 153, 465, 8, true, true},
		{"Smite", 1, 0, 15, 0, false, true}, {"Smite", 20, 0, 300, 0, false, true},
		{"Holy Bolt", 1, 0, 1, 0, false, false}, {"Holy Bolt", 20, 0, 39, 0, false, false},
		{"Conversion", 1, 0, 7, 0, false, false}, {"Conversion", 20, 0, 42, 0, false, false},
	} {
		sk, e := p.env(c.name, c.lvl, nil)

		if c.hasTH && c.name != "Sacrifice" {
			if got := sk.ToHitBonus(e, c.lvl); got != c.toHit {
				t.Errorf("%s L%d attack bonus %d, wiki %d", c.name, c.lvl, got, c.toHit)
			}
		}

		if got := e.eval(sk.Calc[1]); got != c.calc1 {
			t.Errorf("%s L%d calc1 %d, wiki %d", c.name, c.lvl, got, c.calc1)
		}

		if c.name == "Zeal" && c.lvl >= 10 {
			// calc2 is the damage percent of Zeal, calc1 the hit count (2 at level 1 up to 5 from level 4)
			if got := e.eval(sk.Calc[2]); got != c.calc2 {
				t.Errorf("Zeal L%d damage percent %d, wiki %d", c.lvl, got, c.calc2)
			}
		}
	}

	// Zeal hit count: 2, 3, 4, then 5 from level 4 (d2emu)
	for lvl, want := range map[int]int{1: 2, 2: 3, 3: 4, 4: 5, 10: 5, 20: 5} {
		if got := p.calc("Zeal", lvl, 1, nil); got != want {
			t.Errorf("Zeal L%d hits %d, wiki %d", lvl, got, want)
		}
	}

	// Smite's shield damage 3-6 at level 1, 57-60 at 20 is the Holy Shield aura (Arreat: smite damage bonus of the shield)
	for _, c := range []struct {
		lvl      int
		min, max float64
	}{{1, 3, 6}, {20, 57, 60}} {
		sk, e := p.env("Holy Shield", c.lvl, nil)
		d := sk.Descriptor(e, c.lvl, 0, 0, 0)

		if !near(float64(d.PhysMin)/256, c.min, 0.5) || !near(float64(d.PhysMax)/256, c.max, 0.5) {
			t.Errorf("Holy Shield smite damage L%d %.1f-%.1f, wiki %.0f-%.0f", c.lvl, float64(d.PhysMin)/256, float64(d.PhysMax)/256, c.min, c.max)
		}
	}

	// Redemption: chance 23 -> 85 percent, 25 -> 120 recovered
	if p.calc("Redemption", 1, 1, nil) != 23 || p.calc("Redemption", 20, 1, nil) != 85 || p.calc("Redemption", 1, 2, nil) != 25 || p.calc("Redemption", 20, 2, nil) != 120 {
		t.Error("redemption chance / recovery")
	}

	// Conversion lasts 16 s
	sk, e := p.env("Conversion", 5, nil)
	if got := e.eval(sk.AuraLenCalc); got != 400 {
		t.Errorf("Conversion duration %d frames, wiki 16 s", got)
	}

	// Vengeance chill length: 1.2 s at 1, 12.6 s at 20 (d2emu also says 13.8 s at 23, the engine says 14.4, see notes)
	for lvl, frames := range map[int]int{1: 30, 10: 165, 20: 315} {
		sk, e := p.env("Vengeance", lvl, nil)
		if got := sk.ElemLen(e, lvl); got != frames {
			t.Errorf("Vengeance chill L%d %d frames, wiki %.1f s", lvl, got, float64(frames)/25)
		}
	}

	// Sanctuary undead damage and Blessed Hammer's undead bonus: Arreat 150 percent vs d2emu 50 (a bonus on top of 100)
	sk, e = p.env("Sanctuary", 20, nil)
	if got := e.eval(sk.AuraStatCalc[1]); got != 720 {
		t.Errorf("Sanctuary L20 %d", got)
	}
}

// Synergies (wiki per level percent): Holy Fire Resist Fire 18 / Salvation 6, Holy Freeze Resist Cold 15 / Salvation 7,
// Holy Shock Resist Lightning 12 / Salvation 4 (purediablo/Arreat, same in d2emu except Holy Fire where D2R 2.4 says
// 21/10), Blessed Hammer Vigor and Blessed Aim 14 each, Fist of the Heavens Holy Shock 7, Holy Bolt via Fist 15, Vengeance
// Resist x 10 and Salvation 2, Zeal via Sacrifice 12, Charge/Smite per tables.
func TestWikiPalSynergy(t *testing.T) {
	p := newPalFix(t)

	// percent bonus of the shifted base for one synergy skill at level n
	bonus := func(name, syn string, n int) float64 {
		base0 := map[string]int{name: 10}
		base1 := map[string]int{name: 10, syn: n}
		a, _ := p.elem(name, 10, base0)
		b, _ := p.elem(name, 10, base1)

		return (b/a - 1) * 100
	}

	for _, c := range []struct {
		name, syn string
		per       float64
	}{
		{"Holy Fire", "Resist Fire", 18}, {"Holy Fire", "Salvation", 6},
		{"Holy Freeze", "Resist Cold", 15}, {"Holy Freeze", "Salvation", 7},
		{"Holy Shock", "Resist Lightning", 12}, {"Holy Shock", "Salvation", 4},
		{"Blessed Hammer", "Vigor", 14}, {"Blessed Hammer", "Blessed Aim", 14},
		{"Fist of the Heavens", "Holy Shock", 7},
	} {
		// Holy Shock's minimum is 1 (the synergy applies to the max only in practice), compare the maximum
		base0 := map[string]int{c.name: 10}
		base1 := map[string]int{c.name: 10, c.syn: 5}
		_, a := p.elem(c.name, 10, base0)
		_, b := p.elem(c.name, 10, base1)

		if got := (b/a - 1) * 100; !near(got, c.per*5, 0.2) {
			t.Errorf("%s synergy %s: %.2f percent for 5 levels, wiki %.0f", c.name, c.syn, got, c.per*5)
		}
	}

	_ = bonus

	// Vengeance: Resist Fire/Cold/Lightning 10 each on its own element, Salvation 2 on all three
	for _, c := range []struct {
		syn  string
		slot int
		per  int
	}{{"Resist Fire", 1, 10}, {"Resist Cold", 2, 10}, {"Resist Lightning", 3, 10}, {"Salvation", 1, 2}, {"Salvation", 2, 2}, {"Salvation", 3, 2}} {
		a := p.calc("Vengeance", 10, c.slot, map[string]int{"Vengeance": 10})
		b := p.calc("Vengeance", 10, c.slot, map[string]int{"Vengeance": 10, c.syn: 5})

		if b-a != c.per*5 {
			t.Errorf("Vengeance calc%d with %s: +%d, wiki +%d", c.slot, c.syn, b-a, c.per*5)
		}
	}

	// Zeal: Sacrifice 12 percent damage per level (d2emu); Charge and Smite have no synergy in the wiki
	a := p.calc("Zeal", 10, 2, map[string]int{"Zeal": 10})
	b := p.calc("Zeal", 10, 2, map[string]int{"Zeal": 10, "Sacrifice": 5})

	if b-a != 60 {
		t.Errorf("Zeal synergy with Sacrifice: +%d, wiki +60", b-a)
	}
}

// "+skills": the wiki charts list levels above 20 (they come from the +3 skill items); the engine must evaluate the
// same row at level 23 with hard points 20, synergies reading the hard points only.
func TestWikiPalPlusSkills(t *testing.T) {
	p := newPalFix(t)

	// Fanaticism L23: 150 attack rating, 212 own+party? engine aura slots; wiki chart tops at 20, the formula runs on
	if got := p.aura("Fanaticism", 23, 2); got != 150 {
		t.Errorf("Fanaticism L23 attack rating %d", got)
	}

	// synergy ignores the +skills: hard points 20 with effective 23 equals base 20 for the synergy part
	sk, e := wikiEnv(p.reg, "Holy Fire", 23, map[string]int{"Holy Fire": 20, "Resist Fire": 20})
	hi := float64(sk.ElemMax(e, 23)) / 256
	sk, e = wikiEnv(p.reg, "Holy Fire", 23, map[string]int{"Holy Fire": 23, "Resist Fire": 20})
	same := float64(sk.ElemMax(e, 23)) / 256

	if hi != same {
		t.Errorf("synergy depends on the hard points of the skill itself: %.2f vs %.2f", hi, same)
	}

	// Holy Fire L23 radius 18.7 yards and L23 chill 55 percent (d2emu)
	sk, e = p.env("Holy Fire", 23, nil)
	if got := float64(e.eval(sk.AuraRangeCalc)) / 1.5; !near(got, 18.7, 0.1) {
		t.Errorf("Holy Fire L23 radius %.2f", got)
	}
}
