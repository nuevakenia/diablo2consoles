package d2monsters

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2difficulty"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monstats"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// Wave 23: monster stat computation per difficulty against the exe formulas (Game.exe 1.14b, 0x00571af0 with
// MONTBL_GetLevelScaledStats 0x006551e0), derived here from the raw tables for representative monsters:
// level rule, hit points with the player count bonus, defense, experience, A1/A2/S1 damage and to-hit, block
// chance, resistances, skill levels and the life regeneration stat.

var w23Monsters = []string{
	"skeleton1", "zombie1", "fallen1", "goatman1", "blunderbore1", "andariel", "duriel", "mephisto", "diablo",
	"baalcrab", "summoner", "radament", "izual", "bloodraven", "nihlathakboss", "griswold", "councilmember1",
	"ancientbarb1", "ancientbarb2", "ancientbarb3", "baalthrone", "overseer1", "doomknight1", "unraveler1",
}

func TestW23RepresentativeMonsters(t *testing.T) {
	root, rm := loadReal(t)

	ms := readRaw(t, filepath.Join(root, "monsters", "patch_d2", "monstats.txt"))
	ml := readRaw(t, filepath.Join(root, "monsters", "patch_d2", "monlvl.txt"))

	sfx := [3]string{"", "(N)", "(H)"}
	scale := func(a, pct int) int { return a * pct / 100 }
	checked := 0

	for _, key := range w23Monsters {
		var row []string

		for _, r := range ms.rows {
			if strings.EqualFold(ms.s(r, "Id"), key) {
				row = r

				break
			}
		}

		if row == nil {
			t.Logf("%s: not in monstats", key)

			continue
		}

		st := statOfRow(rm, ms.n(row, "hcIdx"), key)
		if st == nil {
			t.Fatalf("%s: record not loaded", key)
		}

		noRatio := ms.n(row, "noRatio") > 0
		align := ms.n(row, "Align")

		for d := 0; d < 3; d++ {
			diff := d2monster.Difficulty(d)
			s := sfx[d]

			// the boss rule: a boss/noRatio class always keeps its monstats Level, in every area
			dir := realDirector(rm, diff, 0, false)
			dir.areaLevel = 99
			b := d2monster.NewBrain(1, 1, diff, &d2monster.Profile{}, 1)
			v := dir.computeVitals(st, b)

			wantLvl := ms.n(row, "Level"+s)
			if d > 0 && !noRatio && ms.n(row, "boss") == 0 {
				wantLvl = 99 // ordinary classes follow the area (MonLvlEx)
			}

			if v.Level != wantLvl {
				t.Errorf("%s %v: level %d, want %d", key, diff, v.Level, wantLvl)
			}

			// monlvl row of the level, L-* columns
			n := [5]int{100, 100, 100, 100, 100} // hp, ac, th, dm, xp
			for _, r := range ml.rows {
				if ml.n(r, "Level") == wantLvl && !noRatio {
					for i, c := range []string{"HP", "AC", "TH", "DM", "XP"} {
						n[i] = ml.n(r, "L-"+c+s)
					}
				}
			}

			// S1 damage and to-hit
			wmin, wmax := scale(n[3], ms.n(row, "S1MinD"+s)), scale(n[3], ms.n(row, "S1MaxD"+s))
			if wmax < wmin {
				wmax = wmin
			}

			if v.S1.Min != wmin || v.S1.Max != wmax {
				t.Errorf("%s %v: S1 damage %d-%d, want %d-%d", key, diff, v.S1.Min, v.S1.Max, wmin, wmax)
			}

			// block chance (stat 0x14): gated by NoShldBlock or the innate classes, capped at 75
			wantBlock := 0
			if ms.n(row, "NoShldBlock") > 0 || st.ID == 0xf3 || st.ID == 0x136 || st.ID == 0x14d {
				wantBlock = ms.n(row, "ToBlock"+s)
				if wantBlock > 75 {
					wantBlock = 75
				}
			}

			if v.Block != wantBlock {
				t.Errorf("%s %v: block %d, want %d", key, diff, v.Block, wantBlock)
			}

			// skill levels: the column plus MonsterSkillBonus (0, 3, 7) when the column is above 0
			bonus := []int{0, 3, 7}[d]
			prof := profileFromRecord(st, diff)

			for i := 0; i < d2monster.NumSkills; i++ {
				col := ms.n(row, "Sk"+string(rune('1'+i))+"lvl")
				if col > 0 && prof.Skills[i].Level != col+bonus {
					t.Errorf("%s %v: skill %d level %d, want %d", key, diff, i+1, prof.Skills[i].Level, col+bonus)
				}

				if col <= 0 && prof.Skills[i].Level != col {
					t.Errorf("%s %v: skill %d level %d, want %d", key, diff, i+1, prof.Skills[i].Level, col)
				}
			}

			// player count bonus on life and experience: the table 0,0,50,100,..,350, then (n-2)*50
			hpMin, hpMax := scale(n[0], ms.n(row, "MinHP"+s)), scale(n[0], ms.n(row, "MaxHP"+s))
			if hpMax < hpMin {
				hpMax = hpMin
			}

			for _, players := range []int{1, 2, 3, 5, 8, 10} {
				dp := realDirector(rm, diff, 0, false)
				dp.areaLevel = 99
				dp.opt.Players = players
				vp := dp.computeVitals(st, d2monster.NewBrain(1, 1, diff, &d2monster.Profile{}, 1))

				pct, xpPct := 0, 0

				if align == 0 {
					table := []int{0, 0, 50, 100, 150, 200, 250, 300, 350}
					if players < 9 {
						pct, xpPct = table[players], table[players]
					} else {
						pct, xpPct = (players-2)*50, (players*5+0x82)*2
					}
				}

				lo, hi := hpMin, hpMax
				if lo < 1 {
					lo = 1
				}

				if hi < 1 {
					hi = 1
				}

				lo, hi = lo+lo*pct/100, hi+hi*pct/100
				if hi > 0x7fffff {
					hi = 0x7fffff
				}

				if lo > 0x7fffff {
					lo = 0x7fffff
				}

				if vp.MaxHP < lo || vp.MaxHP > hi {
					t.Errorf("%s %v players %d: hp %d outside %d..%d", key, diff, players, vp.MaxHP, lo, hi)
				}

				xp := scale(n[4], ms.n(row, "Exp"+s))
				if want := xp + xp*xpPct/100; vp.Experience != want {
					t.Errorf("%s %v players %d: xp %d, want %d", key, diff, players, vp.Experience, want)
				}
			}

			checked++
		}
	}

	if checked < 40 {
		t.Errorf("only %d (monster, difficulty) pairs checked", checked)
	}
}

// the regeneration stat of 0x00571af0 and its per-frame use (0x005a4340).
func TestW23Regen(t *testing.T) {
	// a monster of 100 life with DamageRegen 2: 100*256*2 >> 12 = 12 (1/256 points per frame)
	if got := d2monstats.RegenStat(100<<8, 2); got != 12 {
		t.Errorf("RegenStat(100 life, 2) = %d, want 12", got)
	}

	// large lives shift first: (0x7fffff<<8 >> 12) * 4
	if got, want := d2monstats.RegenStat(0x7fffff<<8, 4), (0x7fffff<<8>>12)*4; got != want {
		t.Errorf("RegenStat big = %d, want %d", got, want)
	}

	if d2monstats.RegenStat(100<<8, 0) != 0 || d2monstats.RegenStat(100<<8, 0x1000) != 0 {
		t.Error("DamageRegen 0 or out of range must give 0")
	}

	d := &Director{}
	m := &d2mapentity.Monster{}
	m.Vitals.HP, m.Vitals.MaxHP = 90, 100
	u := &unit{m: m, stats: map[int]int{statHPRegen: 128}} // half a point per frame

	for i := 0; i < 3; i++ {
		d.regenStep(u)
	}

	// 3 frames * 128 = 384/256 = 1 point and 128/256 left over
	if m.Vitals.HP != 91 || u.regenFrac != 128 {
		t.Errorf("after 3 frames: hp %d frac %d, want 91 and 128", m.Vitals.HP, u.regenFrac)
	}

	m.Vitals.HP = 100
	d.regenStep(u)

	if m.Vitals.HP != 100 || u.regenFrac != 0 {
		t.Errorf("full life: hp %d frac %d", m.Vitals.HP, u.regenFrac)
	}

	m.Vitals.HP = 99
	u.stats[statHPRegen] = 1000
	d.regenStep(u)

	if m.Vitals.HP != 100 {
		t.Errorf("regen must stop at the maximum, hp %d", m.Vitals.HP)
	}
}

func TestW23DifficultyBonusTable(t *testing.T) {
	for d, want := range []int{0, 3, 7} {
		if got := d2difficulty.Default.Row(d2difficulty.Level(d)).MonsterSkillBonus; got != want {
			t.Errorf("difficulty %d bonus %d, want %d", d, got, want)
		}
	}

	if d2monstats.SkillLevel(0, 7) != 0 || d2monstats.SkillLevel(-1, 7) != -1 || d2monstats.SkillLevel(5, 7) != 12 {
		t.Error("SkillLevel must add the bonus only to columns above 0")
	}
}

func TestW23MonsterBlock(t *testing.T) {
	for _, c := range []struct {
		class   int
		noShld  bool
		toBlock int
		want    int
	}{
		{1, false, 30, 0}, {1, true, 30, 30}, {1, true, 90, 75}, {0xf3, false, 20, 20}, {0x136, false, 10, 10},
		{0x14d, false, 99, 75}, {2, true, -5, 0},
	} {
		if got := d2monstats.MonsterBlock(c.class, c.noShld, c.toBlock); got != c.want {
			t.Errorf("%+v: %d", c, got)
		}
	}
}
