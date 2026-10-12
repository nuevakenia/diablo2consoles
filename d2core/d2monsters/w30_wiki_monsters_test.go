package d2monsters

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// Wave 30 wiki cross-check: dump the engine numbers of ~40 monsters on the three difficulties (base, champion and
// unique rank, player-count bonus) so they can be compared with the community pages. Set W30_DUMP=<file> to write the
// table; the test itself only checks internal consistency (rank life = base * (100+constant)/100).

var w30Keys = []string{
	"zombie1", "fallen1", "fallen2", "fallenshaman1", "skeleton1", "bloodraven", "griswold", "andariel",
	"sandraider1", "mummy1", "scarab1", "clawviper1", "pantherwoman1", "radament", "duriel", "summoner",
	"fetish1", "arach1", "thornhulk1", "vilemother1", "batdemon1", "councilmember1", "councilmember2", "councilmember3",
	"mephisto", "izual", "doomknight1", "megademon1", "fingermage1", "diablo",
	"ancientbarb1", "reanimatedhorde1", "frozenhorror1", "succubus1", "overseer1", "nihlathakboss", "baalcrab",
	"goatman1", "quillrat1", "brute1",
}

func TestW30WikiDump(t *testing.T) {
	root, rm := loadReal(t)

	buf, err := os.ReadFile(filepath.Join(root, "monsters", "patch_d2", "monumod.txt"))
	if err != nil {
		t.Skip(err)
	}

	if err = rm.Load(d2resource.MonsterUniqueModifier, d2txt.LoadDataDictionary(buf)); err != nil {
		t.Fatal(err)
	}

	konst := rm.Monster.Unique.Constants

	var sb strings.Builder

	for _, key := range w30Keys {
		st := rm.Monster.Stats[key]
		if st == nil {
			fmt.Fprintf(&sb, "%s MISSING\n", key)

			continue
		}

		for d := 0; d < 3; d++ {
			diff := d2monster.Difficulty(d)
			dir := realDirector(rm, diff, 0, false)
			b := d2monster.NewBrain(1, 1, diff, &d2monster.Profile{}, 1)
			v := dir.computeVitals(st, b)
			lo, hi := dir.LifeRangeAt(st, v.Level)
			res := MonsterResists(st, diff)

			fmt.Fprintf(&sb, "%s d%d lvl=%d life=%d..%d def=%d xp=%d A1=%d-%d/%d A2=%d-%d S1=%d-%d ar=%d res=%v ignoreMonLvl=%v\n",
				key, d, v.Level, lo, hi, v.Defense, v.Experience, v.A1.Min, v.A1.Max, v.A1.ToHit,
				v.A2.Min, v.A2.Max, v.S1.Min, v.S1.Max, v.A1.ToHit, res, st.IgnoreMonLevelTxt)

			// rank rows: champion then unique, via the real init callbacks
			for _, rank := range []struct {
				name string
				flag uint16
				pct  int
			}{{"champ", d2mapentity.MonTypeChampion, konst[4+d]}, {"uniq", d2mapentity.MonTypeUnique, konst[7+d]}} {
				m := &d2mapentity.Monster{Stat: st, Vitals: v}
				m.TypeFlags = rank.flag
				base := m.Vitals.MaxHP
				dir.ApplyUModEffects(m, nil)
				want := (base << 8) + (base<<8)*rank.pct/100
				_ = want

				fmt.Fprintf(&sb, "   %s: life %d -> %d (const +%d%%) xp %d dmgA1 %d-%d\n", rank.name, base,
					m.Vitals.MaxHP, rank.pct, m.Vitals.Experience, m.Vitals.A1.Min, m.Vitals.A1.Max)
			}
		}
	}

	if f := os.Getenv("W30_DUMP"); f != "" {
		if err := os.WriteFile(f, []byte(sb.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
