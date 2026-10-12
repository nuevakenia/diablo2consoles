package d2monreg

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"

// What a monster modifier (monumod.txt row) does to the unit that carries it: the init callback table of the exe.
//
// VERIFIED against the decompile and disassembly of Game.exe 1.14b (w20 monsters):
//
//	0x59fc40 MONSTER_RunUModInitCallbacks  the loop: the four base ids 1..4 (0x6e3618), then the unit's own nine
//	                                       modifier bytes (unit data +0x1c, stops at the first 0). For each id the
//	                                       callback at table 0x739360[id] runs on the leader with flag 1 and (through
//	                                       0x59fc20) on every member of the leader's group with flag 0.
//	callback table (id: function): 1 name seed 0x59e880, 2 hit points 0x59e960, 4 level 0x59e9e0, 5 strong 0x59f340,
//	6 fast 0x59f470, 8/27/28 resist caps 0x59eee0, 9 fire 0x59f4f0, 16 champion 0x59ea20, 17 lightning 0x59f650,
//	18 cold 0x59f7b0, 23 poison 0x59f930, 25 mana burn 0x59fab0, 26 teleport 0x59f160, 30 aura 0x59f1b0,
//	36 ghostly 0x59ec20, 37 fanatic 0x59ed70, 38 possessed 0x59edb0, 39 berserk 0x59ee00, 41 timer event 0x59eea0.
//	Ids without an entry (3 light, 7 curse, 29 multishot, ...) do nothing here; they act through the hook table
//	(umodhook.go, read in wave 22: Cursed, Multishot and the modifier 41 timer are ported there).
//
// The Ghidra names of 0x59eee0 / 0x59f1b0 / 0x59f160 were mislabelled; the table slots above are what the bytes say:
// the Magic Resistant / Spectral Hit / Stone Skin rows all point at the resist-cap routine, Aura Enchanted at the
// random-aura routine and Teleportation at the skill grant.
//
// Stat ids used (itemstatcost): 6/7 life, 12 level, 13 experience, 0x10 armor percent, 0x19 damage percent,
// 0x1f armor class, 0x24 physical resist, 0x25 magic resist, 0x27/0x29/0x2b/0x2d fire/lightning/cold/poison resist,
// 0x30..0x3f the elemental damage stats, 0x43 velocity percent, 0x4a life regeneration, 0x77 to-hit percent.

// FxUnit is the unit a modifier changes (the exe's stat list of a monster).
type FxUnit interface {
	// Stat is STATS_GetUnitStat, BaseStat the base value only.
	Stat(id int) int
	BaseStat(id int) int
	// SetStat is STATS_SetUnitStat, AddStat STATS_AddStatValueIfNonZero (a zero value changes nothing).
	SetStat(id, v int)
	AddStat(id, v int)
	// IsChampion tells the type flag 0x4 (MONSTER_IsChampion).
	IsChampion() bool
	// OrTypeFlags ORs bits into the monster type word (+0x16): ghostly 0x40, possessed 0x20.
	OrTypeFlags(mask uint16)
	// AdvanceNameSeed is the rndname step: seed = lo * 0x6ac690c5 + hi, the low word becomes the name seed.
	AdvanceNameSeed()
	// NameSeed is the unit's name seed (monster data +0x14).
	NameSeed() uint16
	// Velocity is the monstats word at +0x32 (the walk velocity), 0 or negative: none.
	Velocity() int
	// HalfDamage is true for the class the damage bonus is halved for (base class id 0x76, UNVERIFIED which
	// monster that is).
	HalfDamage() bool
	// GrantSkill adds a skill; active sets it as the monster's active skill.
	GrantSkill(skill, level int, active bool)
	// SetAIFlag20 is MONAI_SetMonsterDataFlag20 (set by Teleportation).
	SetAIFlag20()
	// ScheduleTimerEvent is the event 7 at frame + 0x4b of modifier 41.
	ScheduleTimerEvent()
}

// FxCtx is the game state the callbacks read.
type FxCtx struct {
	Difficulty int
	// Expansion picks the L- columns of monlvl.txt (the exe: game+0x6a or game+0x74 set).
	Expansion bool
	// Const is the constants column of a monumod row (0 out of range).
	Const func(row int) int
	// LvlCount is the number of monlvl rows, LvlDamage the DM / L-DM value of the difficulty at a row.
	LvlCount  int
	LvlDamage func(row int, expansion bool, diff int) int
	// ChampionDamageBonus is difficultylevels.txt ChampionDamageBonus of the game difficulty (record +0x34).
	ChampionDamageBonus int
}

func (c *FxCtx) diff() int {
	if c.Difficulty < 0 {
		return 0
	}

	if c.Difficulty > 2 {
		return 2
	}

	return c.Difficulty
}

func (c *FxCtx) konst(row int) int {
	if c.Const == nil {
		return 0
	}

	return c.Const(row)
}

// pct100 is x * pct / 100 truncated toward zero (the exe's multiply by 0x51eb851f and shift).
func pct100(x, pct int) int { return int(int64(x) * int64(pct) / 100) }

// BaseModIDs are the ids the init loop runs for every unit before its own list (0x6e3618).
var BaseModIDs = [4]int{1, 2, 3, 4}

// ApplyInit is MONSTER_RunUModInitCallbacks without the minion spawn: the leader gets every callback with flag 1,
// each group member the same with flag 0. ids are the leader's modifier bytes (a 0 ends the list).
func (c *FxCtx) ApplyInit(leader FxUnit, ids []int, members []FxUnit) {
	run := func(id int) {
		c.Callback(leader, id, true)

		for _, m := range members {
			c.Callback(m, id, false)
		}
	}

	for _, id := range BaseModIDs {
		run(id)
	}

	for i, id := range ids {
		if id == 0 || i >= MaxUMods {
			break
		}

		run(id)
	}
}

// Callback runs the init callback of one modifier id on a unit (flag: leader 1, group member 0).
func (c *FxCtx) Callback(u FxUnit, id int, flag bool) {
	switch id {
	case 1:
		if flag {
			u.AdvanceNameSeed()
		}
	case 2:
		c.hpMultiply(u, flag)
	case 4:
		u.SetStat(12, u.Stat(12)+3)
		u.SetStat(13, u.BaseStat(13)*5)
	case 5:
		c.strong(u, flag)
	case 6:
		c.speedUp(u)
	case 8, 27, 28:
		c.resistCaps(u, id, flag)
	case 9:
		c.elemental(u, id, flag, 0x30, 0x31, 0)
	case 17:
		c.elemental(u, id, flag, 0x32, 0x33, 0)
	case 18:
		c.elemental(u, id, flag, 0x36, 0x37, 0x38)
	case 23:
		c.elemental(u, id, flag, 0x39, 0x3a, 0x3b)
	case 25:
		c.elemental(u, id, flag, 0x3e, 0x3f, 0)
	case 16:
		c.minionPenalty(u, id, flag)
	case 26:
		if flag {
			u.GrantSkill(0xb8, 1, true)
			u.SetAIFlag20()
		}
	case 30:
		if flag {
			c.randomAura(u)
		}
	case 36:
		c.ghostly(u, flag)
	case 37:
		if flag {
			u.SetStat(0x10, -0x46)
			c.minionPenalty(u, id, flag)
		}
	case 38:
		if flag {
			u.OrTypeFlags(0x20)
			c.scaleHP(u, 100)
			c.minionPenalty(u, id, flag)
		}
	case 39:
		c.berserk(u, flag)
	case 41:
		u.ScheduleTimerEvent()
	}
}

// scaleHP is 0x59e8c0: life += life * pct / 100, written to the maximum (stat 7) and the current life (stat 6). Life
// is the stat value (<< 8). The exe computes huge values as (life / 100) * pct; the same split is kept.
func (c *FxCtx) scaleHP(u FxUnit, pct int) {
	hp := u.Stat(7)

	var delta int

	switch {
	case hp > 0x100000:
		delta = (hp / 100) * pct
	case pct > 0x10000:
		delta = (pct / 100) * hp
	default:
		delta = pct100(hp, pct)
	}

	u.SetStat(7, hp+delta)
	u.SetStat(6, hp+delta)
}

// hpMultiply is 0x59e960 (hit point bonus), for difficulty 0..2 only: a group member uses the minion row
// (constants 1 + d), the leader the champion row (4 + d) when the champion bit is set, else the unique row (7 + d).
// It also zeroes the life regeneration (stat 0x4a) of the leader.
func (c *FxCtx) hpMultiply(u FxUnit, flag bool) {
	if c.Difficulty > 2 {
		return
	}

	d := c.diff()

	if !flag {
		c.scaleHP(u, c.konst(1+d))

		return
	}

	row := 7 + d
	if u.IsChampion() {
		row = 4 + d
	}

	c.scaleHP(u, c.konst(row))
	u.SetStat(0x4a, 0)
}

// damageBonus halves the damage part for the half-damage class.
func damageBonus(u FxUnit, dmg int) int {
	if u.HalfDamage() {
		return dmg / 2
	}

	return dmg
}

// strong is 0x59f340 (Extra Strong): damage and to-hit percent from constants rows 15 / 13 (leader) or 14 / 12
// (member), times the ChampionDamageBonus of the difficulty.
func (c *FxCtx) strong(u FxUnit, flag bool) {
	dmgRow, thRow := 14, 12
	if flag {
		dmgRow, thRow = 15, 13
	}

	dmg := pct100(c.ChampionDamageBonus, c.konst(dmgRow))
	th := pct100(c.ChampionDamageBonus, c.konst(thRow))

	u.AddStat(0x19, damageBonus(u, dmg))
	u.AddStat(0x77, th)
}

// speedBonus is the velocity percent of Fast / Fanatic: 0x800 / velocity - 0x80, at least 10, at most 100.
func speedBonus(vel int) int {
	v := 0x800/vel - 0x80
	if v <= 10 {
		return 10
	}

	if v > 100 {
		return 100
	}

	return v
}

// speedUp is 0x59f470 (Fast): the velocity stat 0x43 grows by speedBonus; nothing for a class without velocity.
func (c *FxCtx) speedUp(u FxUnit) {
	if vel := u.Velocity(); vel > 0 {
		u.AddStat(0x43, speedBonus(vel))
	}
}

// minionPenalty is 0x59ea20 (the champion kit): level -1, experience reduced by two fifths, damage and to-hit
// percent from constants rows 11 / 10 times the ChampionDamageBonus, and a speed change by the caller id (36
// ghostly none, 37 fanatic speedBonus, others +20). Leader only.
func (c *FxCtx) minionPenalty(u FxUnit, id int, flag bool) {
	if !flag {
		return
	}

	u.SetStat(12, u.Stat(12)-1)

	xp := u.BaseStat(13)
	u.SetStat(13, xp-(xp/5)*2)

	dmg := pct100(c.ChampionDamageBonus, c.konst(11))
	th := pct100(c.ChampionDamageBonus, c.konst(10))

	u.AddStat(0x19, damageBonus(u, dmg))
	u.AddStat(0x77, th)

	vel := u.Velocity()
	if vel <= 0 || id == 36 {
		return
	}

	if id == 37 {
		u.AddStat(0x43, speedBonus(vel))

		return
	}

	u.AddStat(0x43, 0x14)
}

// berserk is 0x59ee00: life -75 percent, to-hit +300 and damage +300 percent (times ChampionDamageBonus).
func (c *FxCtx) berserk(u FxUnit, flag bool) {
	if !flag {
		return
	}

	c.scaleHP(u, -75)

	b := pct100(c.ChampionDamageBonus, 300)

	u.AddStat(0x19, damageBonus(u, b))
	u.AddStat(0x77, b)
}

// lvlIndex clamps the unit level into the monlvl rows: at least 1, at most the last row; -1 when there is none.
func (c *FxCtx) lvlIndex(level int) int {
	lv := level
	if lv < 1 {
		lv = 1
	}

	last := c.LvlCount - 1
	if lv >= last {
		lv = last
	}

	if lv < 0 || lv >= c.LvlCount {
		return -1
	}

	return lv
}

// elemental is the shared body of Fire / Lightning / Cold / Poison Enchanted and Mana Burn: the monlvl damage of
// the unit's level (clamped) times a constants-row percent, as the minimum and maximum of the element's damage
// stats (leader rows 28 + d / 31 + d, member rows 16 + d / 19 + d). lenStat, when set: cold length 100 + 5 * row,
// poison length (150 + 5 * row) * 2; mana burn values are multiplied by 256. The resist caps routine follows.
func (c *FxCtx) elemental(u FxUnit, id int, flag bool, minStat, maxStat, lenStat int) {
	d := c.diff()

	idx := c.lvlIndex(u.Stat(12))
	if idx < 0 || c.LvlDamage == nil {
		return
	}

	dm := c.LvlDamage(idx, c.Expansion, d)

	minRow, maxRow := 16+d, 19+d
	if flag {
		minRow, maxRow = 28+d, 31+d
	}

	lo := pct100(dm, c.konst(minRow))
	hi := pct100(dm, c.konst(maxRow))

	if id == 25 {
		lo *= 256
		hi *= 256
	}

	u.AddStat(minStat, lo)
	u.AddStat(maxStat, hi)

	switch id {
	case 18:
		u.AddStat(lenStat, 100+idx*5)
	case 23:
		u.AddStat(lenStat, (idx*5+150)*2)
	}

	c.resistCaps(u, id, flag)
}

// ghostly is 0x59ec20: type bit 0x40, physical resist 80, the champion kit without a speed change and cold damage
// from the champion rows (22 + d minimum, 25 + d maximum) with a length of 150.
func (c *FxCtx) ghostly(u FxUnit, flag bool) {
	if !flag {
		return
	}

	u.OrTypeFlags(0x40)
	u.SetStat(0x24, 0x50)
	c.minionPenalty(u, 36, flag)

	d := c.diff()

	idx := c.lvlIndex(u.Stat(12))
	if idx < 0 || c.LvlDamage == nil {
		return
	}

	dm := c.LvlDamage(idx, c.Expansion, d)

	u.AddStat(0x37, pct100(dm, c.konst(25+d)))
	u.AddStat(0x36, pct100(dm, c.konst(22+d)))
	u.AddStat(0x38, 150)
}

// resistCaps is 0x59eee0: after an elemental, resist or stone skin modifier it raises the matching resists, but
// only while fewer than two of the six resists (0x27, 0x29, 0x2b, 0x2d, 0x24, 0x25) are at 100 or more. Leader only.
func (c *FxCtx) resistCaps(u FxUnit, id int, flag bool) {
	if !flag {
		return
	}

	if id == 28 {
		u.SetStat(0x1f, u.Stat(0x1f)*2)
	}

	fire, light, cold, pois := u.Stat(0x27), u.Stat(0x29), u.Stat(0x2b), u.Stat(0x2d)
	phys, mag := u.Stat(0x24), u.Stat(0x25)

	n := 0

	for _, v := range []int{fire, light, cold, pois, phys, mag} {
		if v > 99 {
			n++
		}
	}

	if n >= 2 {
		return
	}

	// bump raises a resist by add while it is under limit and counts it when it reaches 100
	bump := func(stat, cur, limit, add int) bool {
		if cur >= limit {
			return false
		}

		u.SetStat(stat, cur+add)

		if cur+add > 99 {
			n++
		}

		return true
	}

	switch id {
	case 8:
		bump(0x2b, cold, 100, 40)

		if n < 2 {
			bump(0x27, fire, 100, 40)
		}

		if n < 2 {
			bump(0x29, light, 100, 40)
		}
	case 9:
		u.SetStat(0x27, fire+0x4b)
	case 18:
		u.SetStat(0x2b, cold+0x4b)
	case 17:
		u.SetStat(0x29, light+0x4b)
	case 23:
		u.SetStat(0x2d, pois+0x4b)
	case 25:
		u.SetStat(0x25, mag+0x14)
	case 27:
		bump(0x2b, cold, 0x4b, 0x14)

		if n < 2 {
			bump(0x27, fire, 0x4b, 0x14)
		}

		if n < 2 {
			bump(0x29, light, 0x4b, 0x14)
		}
	case 28:
		u.SetStat(0x24, phys+0x32)
	}
}

// AuraEntry is one row of the random aura table at 0x7392b0 (five dwords: level threshold, add, multiplier,
// divisor, skill id).
type AuraEntry struct{ Level, Add, Mul, Div, Skill int }

// AuraTable is the table at 0x7392b0. The roll is over the entries whose threshold the monster level has reached
// (at least one).
var AuraTable = [8]AuraEntry{
	{0, 0, 1, 6, 98}, {0, 0, 1, 6, 102}, {0, 0, 1, 5, 108}, {0, 0, 1, 7, 114},
	{0, 0, 1, 8, 123}, {0, 0, 1, 8, 122}, {20, 0, 1, 8, 118}, {999, 0, 0, 1, 103},
}

// AuraPick is the choice of 0x59f1b0 for a level and the roll result r in [0, n): the entry index and the skill
// level ((add + level) * mul / div, 1..99). n is the number of reachable entries.
func AuraPick(level int, roll func(n int) int) (idx, skillLevel, n int) {
	if level < 1 {
		level = 1
	}

	for _, e := range AuraTable {
		if e.Level <= level {
			n++
		}
	}

	if n == 0 {
		n = 1
	}

	idx = roll(n)
	e := AuraTable[idx]

	skillLevel = (e.Add + level) * e.Mul / e.Div
	if skillLevel < 2 {
		skillLevel = 1
	} else if skillLevel > 98 {
		skillLevel = 99
	}

	return idx, skillLevel, n
}

// randomAura is 0x59f1b0 (Aura Enchanted): a random aura skill at a level that grows with the monster level, rolled
// with a seed made from the unit's name seed. The branches for a monster data word of 0x25 (forced entry 5) and for
// class 0x2c0 (skill 0x7b) are not ported (UNVERIFIED meaning).
func (c *FxCtx) randomAura(u FxUnit) {
	level := u.Stat(12)

	idx, lvl, _ := AuraPick(level, func(n int) int {
		return rollNameSeed(u.NameSeed(), n)
	})

	u.GrantSkill(AuraTable[idx].Skill, lvl, true)
}

// rollNameSeed is the bounded roll 0x59f1b0 takes on a fresh seed made from the unit's name seed (DRLG_InitSeed
// then RAND_RollSeedBounded; the seeding is read as d2rand.New, UNVERIFIED).
func rollNameSeed(nameSeed uint16, n int) int {
	return int(d2rand.New(uint32(nameSeed)).Roll(int32(n)))
}

// NextNameSeed is the rndname step 0x59e880 on the unit seed (lo, hi): the new 64-bit value is lo * 0x6ac690c5 + hi;
// its low word becomes the seed's lo, the high word the seed's hi, and the low 16 bits the name seed.
func NextNameSeed(lo, hi uint32) (newLo, newHi uint32, name uint16) {
	t := uint64(lo)*0x6ac690c5 + uint64(hi)

	return uint32(t), uint32(t >> 32), uint16(t)
}
