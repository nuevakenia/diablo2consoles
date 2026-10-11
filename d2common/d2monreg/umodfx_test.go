package d2monreg

import (
	"os"
	"path/filepath"
	"testing"
)

// fxStub is a unit with a plain stat map.
type fxStub struct {
	stats    map[int]int
	base     map[int]int
	champion bool
	vel      int
	half     bool
	flags    uint16
	skills   [][3]int
	ai20     bool
	seeded   int
	name     uint16
}

func newStub() *fxStub {
	return &fxStub{stats: map[int]int{}, base: map[int]int{}, vel: 6}
}

func (u *fxStub) Stat(id int) int { return u.stats[id] }
func (u *fxStub) BaseStat(id int) int {
	if v, ok := u.base[id]; ok {
		return v
	}

	return u.stats[id]
}
func (u *fxStub) SetStat(id, v int) { u.stats[id] = v; u.base[id] = v }
func (u *fxStub) AddStat(id, v int) {
	if v != 0 {
		u.stats[id] += v
	}
}
func (u *fxStub) IsChampion() bool            { return u.champion }
func (u *fxStub) OrTypeFlags(m uint16)        { u.flags |= m }
func (u *fxStub) AdvanceNameSeed()            { u.seeded++ }
func (u *fxStub) NameSeed() uint16            { return u.name }
func (u *fxStub) Velocity() int               { return u.vel }
func (u *fxStub) HalfDamage() bool            { return u.half }
func (u *fxStub) SetAIFlag20()                { u.ai20 = true }
func (u *fxStub) ScheduleTimerEvent()         {}
func (u *fxStub) GrantSkill(s, l int, a bool) { u.skills = append(u.skills, [3]int{s, l, b2i(a)}) }

func b2i(b bool) int {
	if b {
		return 1
	}

	return 0
}

// stockConsts are the constants column of the stock monumod.txt (rows 0..33).
var stockConsts = []int{20, 100, 75, 50, 200, 150, 100, 300, 200, 100, 75, 100, 50, 100, 75, 150, 0, 33, 33, 0, 50, 50, 33, 33, 33, 50, 50, 50, 66, 66, 66, 100, 100, 100}

func stockCtx(diff int) *FxCtx {
	return &FxCtx{
		Difficulty: diff, Expansion: true,
		Const: func(r int) int {
			if r < 0 || r >= len(stockConsts) {
				return 0
			}

			return stockConsts[r]
		},
		LvlCount:            100,
		LvlDamage:           func(row int, _ bool, d int) int { return 10*row + d },
		ChampionDamageBonus: [3]int{90, 75, 66}[diff],
	}
}

func TestFxHitPoints(t *testing.T) {
	cases := []struct {
		diff     int
		flag     bool
		champion bool
		pct      int
	}{
		{0, false, false, 100}, {1, false, false, 75}, {2, false, false, 50}, // minion rows 1..3
		{0, true, true, 200}, {1, true, true, 150}, {2, true, true, 100}, // champion rows 4..6
		{0, true, false, 300}, {1, true, false, 200}, {2, true, false, 100}, // unique rows 7..9
	}

	for _, c := range cases {
		u := newStub()
		u.stats[7], u.stats[6], u.champion = 1000<<8, 1000<<8, c.champion
		u.stats[0x4a] = 77
		stockCtx(c.diff).Callback(u, 2, c.flag)

		want := 1000 << 8
		want += want * c.pct / 100

		if u.stats[7] != want || u.stats[6] != want {
			t.Errorf("%+v: life %d/%d want %d", c, u.stats[7], u.stats[6], want)
		}

		if c.flag && u.stats[0x4a] != 0 {
			t.Errorf("%+v: regeneration kept", c)
		}

		if !c.flag && u.stats[0x4a] != 77 {
			t.Errorf("%+v: a member lost its regeneration", c)
		}
	}
}

func TestFxLevelAndBase(t *testing.T) {
	u := newStub()
	u.stats[12], u.stats[13] = 30, 100
	c := stockCtx(0)
	c.ApplyInit(u, nil, nil)

	if u.stats[12] != 33 || u.stats[13] != 500 {
		t.Fatalf("level %d xp %d", u.stats[12], u.stats[13])
	}

	if u.seeded != 1 {
		t.Fatalf("name seed steps %d", u.seeded)
	}
}

func TestFxExtraStrong(t *testing.T) {
	u, m := newStub(), newStub()
	c := stockCtx(1)
	c.Callback(u, 5, true)
	c.Callback(m, 5, false)

	// leader: damage 75 * 150 / 100 = 112, to-hit 75 * 100 / 100 = 75; member: 75 * 75 / 100 = 56, 75 * 50 / 100 = 37
	if u.stats[0x19] != 112 || u.stats[0x77] != 75 {
		t.Errorf("leader %d %d", u.stats[0x19], u.stats[0x77])
	}

	if m.stats[0x19] != 56 || m.stats[0x77] != 37 {
		t.Errorf("member %d %d", m.stats[0x19], m.stats[0x77])
	}

	h := newStub()
	h.half = true
	c.Callback(h, 5, true)

	if h.stats[0x19] != 56 {
		t.Errorf("half class %d", h.stats[0x19])
	}
}

func TestFxFastAndFanatic(t *testing.T) {
	for _, c := range []struct{ vel, want int }{{6, 100}, {10, 76}, {16, 10}, {20, 10}, {0, 0}} {
		u := newStub()
		u.vel = c.vel
		stockCtx(0).Callback(u, 6, true)

		if u.stats[0x43] != c.want {
			t.Errorf("vel %d: speed %d want %d", c.vel, u.stats[0x43], c.want)
		}
	}

	f := newStub()
	f.stats[12], f.stats[13], f.stats[0x1f] = 20, 100, 200
	f.vel = 10
	stockCtx(0).Callback(f, 37, true)

	// armor percent -70, level -1, xp 100 - 2*20 = 60, damage 90 * 75 / 100 = 67, to-hit 90 * 100 / 100 = 90 ... row 11 is
	// 100 and row 10 is 75
	if f.stats[0x10] != -70 || f.stats[12] != 19 || f.stats[13] != 60 {
		t.Errorf("fanatic %v", f.stats)
	}

	if f.stats[0x19] != 90 || f.stats[0x77] != 67 || f.stats[0x43] != 76 {
		t.Errorf("fanatic damage %v", f.stats)
	}
}

func TestFxChampionKits(t *testing.T) {
	// possessed: life doubled, type bit 0x20, +20 speed
	p := newStub()
	p.stats[7], p.stats[6], p.stats[12], p.stats[13] = 100<<8, 100<<8, 10, 50
	stockCtx(0).Callback(p, 38, true)

	if p.stats[7] != 200<<8 || p.flags != 0x20 || p.stats[0x43] != 20 || p.stats[12] != 9 || p.stats[13] != 30 {
		t.Errorf("possessed %v flags %#x", p.stats, p.flags)
	}

	// berserk: life x 0.25, damage and to-hit +300% of the bonus; the bonus is halved for the half class
	b := newStub()
	b.stats[7], b.stats[6] = 100<<8, 100<<8
	b.half = true
	stockCtx(2).Callback(b, 39, true)

	if b.stats[7] != 25<<8 || b.stats[0x77] != 198 || b.stats[0x19] != 99 {
		t.Errorf("berserk %v", b.stats)
	}

	// ghostly: physical resist 80, no speed change, cold damage of the champion rows
	g := newStub()
	g.stats[12], g.stats[13] = 40, 100
	stockCtx(0).Callback(g, 36, true)

	if g.flags != 0x40 || g.stats[0x24] != 80 || g.stats[0x43] != 0 {
		t.Errorf("ghostly %v flags %#x", g.stats, g.flags)
	}

	// level 39 after the penalty? the element uses the level at call time: penalty first (40 -> 39), row 39 value 390
	if g.stats[0x36] != 390*33/100 || g.stats[0x37] != 390*50/100 || g.stats[0x38] != 150 {
		t.Errorf("ghostly cold %v", g.stats)
	}
}

func TestFxElementals(t *testing.T) {
	// Fire Enchanted at level 30 in Nightmare: damage value 10*30+1 = 301, leader rows 28+1 / 31+1 = 66% / 100%
	u := newStub()
	u.stats[12] = 30
	stockCtx(1).Callback(u, 9, true)

	if u.stats[0x30] != 301*66/100 || u.stats[0x31] != 301 || u.stats[0x27] != 75 {
		t.Errorf("fire %v", u.stats)
	}

	// minion rows 16+1 and 19+1: 33% and 50%... row 17 = 33, row 20 = 50; no resist for a member
	m := newStub()
	m.stats[12] = 30
	stockCtx(1).Callback(m, 9, false)

	if m.stats[0x30] != 301*33/100 || m.stats[0x31] != 301*50/100 || m.stats[0x27] != 0 {
		t.Errorf("fire member %v", m.stats)
	}

	// Cold: length 100 + 5 * level row
	c := newStub()
	c.stats[12] = 30
	stockCtx(0).Callback(c, 18, true)

	if c.stats[0x38] != 250 || c.stats[0x2b] != 75 {
		t.Errorf("cold %v", c.stats)
	}

	// Poison: (5 * row + 150) * 2
	p := newStub()
	p.stats[12] = 30
	stockCtx(0).Callback(p, 23, true)

	if p.stats[0x3b] != 600 || p.stats[0x2d] != 75 {
		t.Errorf("poison %v", p.stats)
	}

	// Mana burn: x 256 and 20 magic resist
	mb := newStub()
	mb.stats[12] = 30
	stockCtx(0).Callback(mb, 25, true)

	if mb.stats[0x3e] != (300*66/100)*256 || mb.stats[0x3f] != 300*256 || mb.stats[0x25] != 20 {
		// rows 28 / 31 of the stock table: 66 and 100
		t.Errorf("mana burn %v", mb.stats)
	}

	// the level is clamped to the last row, and below 1 to 1
	hi := newStub()
	hi.stats[12] = 500
	stockCtx(0).Callback(hi, 17, true)

	if hi.stats[0x33] != 10*99 {
		t.Errorf("clamped level %v", hi.stats)
	}
}

func TestFxResistCaps(t *testing.T) {
	// Magic Resistant: cold, fire and lightning +40 (each while under 100 and fewer than two at the cap)
	u := newStub()
	stockCtx(0).Callback(u, 8, true)

	if u.stats[0x2b] != 40 || u.stats[0x27] != 40 || u.stats[0x29] != 40 {
		t.Errorf("resist %v", u.stats)
	}

	// two resists already at 100: nothing changes
	v := newStub()
	v.stats[0x2b], v.stats[0x27] = 100, 100
	stockCtx(0).Callback(v, 8, true)

	if v.stats[0x29] != 0 {
		t.Errorf("capped %v", v.stats)
	}

	// cold at 80 becomes 120 and counts as one; fire at 90 becomes 130 (second): lightning stays
	w := newStub()
	w.stats[0x2b], w.stats[0x27] = 80, 90
	stockCtx(0).Callback(w, 8, true)

	if w.stats[0x2b] != 120 || w.stats[0x27] != 130 || w.stats[0x29] != 0 {
		t.Errorf("cap order %v", w.stats)
	}

	// Spectral Hit: +20 to the three elements that are under 75
	s := newStub()
	s.stats[0x27] = 80
	stockCtx(0).Callback(s, 27, true)

	if s.stats[0x2b] != 20 || s.stats[0x27] != 80 || s.stats[0x29] != 20 {
		t.Errorf("spectral %v", s.stats)
	}

	// Stone Skin: armor doubled, physical resist +50
	k := newStub()
	k.stats[0x1f] = 120
	stockCtx(0).Callback(k, 28, true)

	if k.stats[0x1f] != 240 || k.stats[0x24] != 50 {
		t.Errorf("stone skin %v", k.stats)
	}
}

func TestFxTeleportAndAura(t *testing.T) {
	u := newStub()
	stockCtx(0).Callback(u, 26, true)

	if len(u.skills) != 1 || u.skills[0] != [3]int{0xb8, 1, 1} || !u.ai20 {
		t.Errorf("teleport %v %v", u.skills, u.ai20)
	}

	m := newStub()
	stockCtx(0).Callback(m, 26, false)

	if len(m.skills) != 0 {
		t.Errorf("member got teleport")
	}

	// the aura roll: 6 entries below level 20, 7 from level 20; the skill level is level * mul / div
	for lvl, want := range map[int]int{1: 6, 19: 6, 20: 7, 90: 7} {
		_, _, n := AuraPick(lvl, func(n int) int { return n - 1 })
		if n != want {
			t.Errorf("level %d: %d entries want %d", lvl, n, want)
		}
	}

	idx, sl, _ := AuraPick(60, func(int) int { return 0 })
	if AuraTable[idx].Skill != 98 || sl != 10 {
		t.Errorf("aura level %d skill %d", sl, AuraTable[idx].Skill)
	}

	a := newStub()
	a.stats[12] = 24
	a.name = 0x1234
	stockCtx(0).Callback(a, 30, true)

	if len(a.skills) != 1 || a.skills[0][2] != 1 {
		t.Errorf("aura %v", a.skills)
	}
}

func TestFxIgnoredIDs(t *testing.T) {
	for _, id := range []int{3, 7, 10, 29, 31, 40, 42, 99} {
		u := newStub()
		u.stats[12] = 20
		stockCtx(0).Callback(u, id, true)

		if len(u.stats) != 1 || u.stats[12] != 20 || len(u.skills) != 0 {
			t.Errorf("id %d changed the unit: %v", id, u.stats)
		}
	}
}

func TestNextNameSeed(t *testing.T) {
	lo, hi, name := NextNameSeed(12345, 0x29a)

	_ = hi

	if lo != 0x015B2E77 || name != 0x2E77 {
		// the same step as d2rand.Seed.Step on the seed 12345
		t.Errorf("%#x %#x %#x", lo, hi, name)
	}
}

// The stock table's own constants row, when the real tables are around.
func TestFxStockConstants(t *testing.T) {
	dir := os.Getenv("D2_TABLES")
	if dir == "" {
		t.Skip("D2_TABLES unset")
	}

	b, err := os.ReadFile(filepath.Join(dir, "monsters", "patch_d2", "monumod.txt"))
	if err != nil {
		t.Skip("no monumod.txt")
	}

	tb := &Tables{}
	mt := []byte("type\tequiv1\tequiv2\tequiv3\n")

	if err := tb.LoadUMods(b, mt); err != nil {
		t.Fatal(err)
	}

	for i, want := range stockConsts {
		if got := tb.UMods.Rows[i].Constant; got != want {
			t.Errorf("row %d constant %d want %d", i, got, want)
		}
	}
}

// TestFxDenPackLife pins the cause of the 9b Act 1 playthrough failures of wave 20: a super unique pack (the Den of
// Evil's) now gets the full init loop, so on Normal the leader has 4x and each follower 2x its base life
// (monumod.txt constants rows 1 and 7). A level 1 hero with a plain attack no longer clears the Den; the scenario
// brings a levelled Sorceress with Fire Bolt (scripts/verify.d/9b-act1-playthrough.sh).
func TestFxDenPackLife(t *testing.T) {
	leader := newStub()
	leader.stats[7], leader.stats[6] = 40<<8, 40<<8

	members := make([]FxUnit, 0, 5)
	stubs := make([]*fxStub, 0, 5)

	for i := 0; i < 5; i++ {
		m := newStub()
		m.stats[7], m.stats[6] = 20<<8, 20<<8
		members = append(members, m)
		stubs = append(stubs, m)
	}

	stockCtx(0).ApplyInit(leader, nil, members)

	if got := leader.stats[7] >> 8; got != 160 {
		t.Errorf("leader life %d, want 160 (+300%%)", got)
	}

	for i, m := range stubs {
		if got := m.stats[7] >> 8; got != 40 {
			t.Errorf("follower %d life %d, want 40 (+100%%)", i, got)
		}
	}
}
