package d2monsters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// Wave 21: life of the super uniques and bosses on all three difficulties against the exe formulas, derived here
// from the raw tables. Exe facts (VERIFIED, Game.exe 1.14b): the base life is the monlvl HP of the monster level
// times the monstats Min/MaxHP percent (0x571af0); a super unique then runs the init callbacks, whose hit point
// step (0x59e960 + 0x59e8c0) adds constants row 7+d percent to the leader (row 4+d for a champion) and row 1+d
// to every linked follower; the bosses of the story (Andariel, Duriel, Mephisto, Diablo, Baal) are plain classes
// and carry no rank bonus. The rows are 300/200/100 (unique leader), 100/75/50 (follower) in the stock table.
func TestRealRankLife(t *testing.T) {
	root, rm := loadReal(t)

	buf, err := os.ReadFile(filepath.Join(root, "monsters", "patch_d2", "monumod.txt"))
	if err != nil {
		t.Skip(err)
	}

	if err = rm.Load(d2resource.MonsterUniqueModifier, d2txt.LoadDataDictionary(buf)); err != nil {
		t.Fatal(err)
	}

	ms := readRaw(t, filepath.Join(root, "monsters", "patch_d2", "monstats.txt"))
	ml := readRaw(t, filepath.Join(root, "monsters", "patch_d2", "monlvl.txt"))
	mu := readRaw(t, filepath.Join(root, "monsters", "patch_d2", "monumod.txt"))
	su := readRaw(t, filepath.Join(root, "monsters", "patch_d2", "superuniques.txt"))

	konst := make([]int, 0, len(mu.rows))
	for _, r := range mu.rows {
		konst = append(konst, mu.n(r, "constants"))
	}

	sfx := [3]string{"", "(N)", "(H)"}
	classRow := func(key string) []string {
		for _, r := range ms.rows {
			if strings.EqualFold(ms.s(r, "Id"), key) {
				return r
			}
		}

		t.Fatalf("no monstats row %s", key)

		return nil
	}

	// base life range of a class from the raw tables
	baseRange := func(row []string, d int) (lo, hi, lvl int) {
		lvl = ms.n(row, "Level"+sfx[d])
		hp := 100

		if ms.n(row, "noRatio") == 0 {
			for _, r := range ml.rows {
				if ml.n(r, "Level") == lvl {
					hp = ml.n(r, "L-HP"+sfx[d])
				}
			}
		}

		lo, hi = hp*ms.n(row, "MinHP"+sfx[d])/100, hp*ms.n(row, "MaxHP"+sfx[d])/100
		if hi < lo {
			hi = lo
		}

		return lo, hi, lvl
	}

	bonus := func(hp, pct int) int { // 0x59e8c0 on the stat value (<< 8)
		s := hp << 8
		if s > 0x100000 { // the exe divides first for stat values above 0x100000 (about 4096 life)
			return (s + (s/100)*pct) >> 8
		}

		return (s + s*pct/100) >> 8
	}

	supers := []string{"Corpsefire", "Bishibosh", "Bonebreak", "Coldcrow", "Griswold", "Radament", "Summoner", "Izual"}
	bosses := []string{"andariel", "duriel", "mephisto", "diablo", "baalcrab"}

	for d := 0; d < 3; d++ {
		diff := d2monster.Difficulty(d)
		dir := realDirector(rm, diff, 0, false)

		for _, name := range supers {
			var sr []string

			for _, r := range su.rows {
				if strings.EqualFold(su.s(r, "Superunique"), name) {
					sr = r
				}
			}

			if sr == nil {
				continue
			}

			row := classRow(su.s(sr, "Class"))
			st := statOfRow(rm, ms.n(row, "hcIdx"), ms.s(row, "Id"))
			lo, hi, _ := baseRange(row, d)

			b := d2monster.NewBrain(1, 1, diff, &d2monster.Profile{}, 1)
			leader := &d2mapentity.Monster{Stat: st, Vitals: dir.computeVitals(st, b)}
			leader.Modifiers = []int{su.n(sr, "Mod1"), su.n(sr, "Mod2"), su.n(sr, "Mod3")}
			base := leader.Vitals.MaxHP

			if base < lo || base > hi {
				t.Errorf("%s %v: base life %d outside %d..%d", name, diff, base, lo, hi)
			}

			fol := &d2mapentity.Monster{Stat: st, Vitals: dir.computeVitals(st, b)}
			fbase := fol.Vitals.MaxHP

			dir.ApplyUModEffects(leader, []*d2mapentity.Monster{fol})

			if got, want := leader.Vitals.MaxHP, bonus(base, konst[7+d]); got != want {
				t.Errorf("%s %v: leader life %d, want %d from base %d", name, diff, got, want, base)
			}

			if got, want := fol.Vitals.MaxHP, bonus(fbase, konst[1+d]); got != want {
				t.Errorf("%s %v: follower life %d, want %d from base %d", name, diff, got, want, fbase)
			}

			if leader.Vitals.HP != leader.Vitals.MaxHP {
				t.Errorf("%s %v: current life %d, max %d", name, diff, leader.Vitals.HP, leader.Vitals.MaxHP)
			}

			t.Logf("%s %v: base %d..%d leader %d", name, diff, lo, hi, leader.Vitals.MaxHP)
		}

		for _, key := range bosses {
			row := classRow(key)
			st := statOfRow(rm, ms.n(row, "hcIdx"), ms.s(row, "Id"))
			lo, hi, lvl := baseRange(row, d)

			b := d2monster.NewBrain(1, 1, diff, &d2monster.Profile{}, 1)
			v := dir.computeVitals(st, b)

			if v.Level != lvl || v.MaxHP < lo || v.MaxHP > hi {
				t.Errorf("%s %v: level %d life %d, want level %d life %d..%d", key, diff, v.Level, v.MaxHP, lvl, lo, hi)
			}

			t.Logf("%s %v: level %d life %d (range %d..%d)", key, diff, v.Level, v.MaxHP, lo, hi)
		}
	}
}
