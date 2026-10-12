package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2summon"
)

// w31: MONSTER_SetupLevelScaledStats 0x5a2960 runs before every monster attack (MONAI_ExecuteAiCommand 0x5a5630) and
// rewrites the attack numbers with the SkillDamage skill at the owner's CURRENT level (VERIFIED). The Summoner hands the
// minion that recompute as Stats.Live; stats that are not part of it stay as they were at the spawn.
func TestW31LiveSkillDamage(t *testing.T) {
	tpl, err := d2summon.LoadTemplates([]byte("Id\thcIdx\tAI\tVelocity\tRun\tminHP\tmaxHP\tAC\tA1MinD\tA1MaxD\tA1TH\tSkillDamage\n" +
		"spiritwolf\t1\tNecroPet\t6\t6\t60\t80\t10\t0\t0\t0\tSummon Spirit Wolf\n" +
		"necroskeleton\t2\tNecroPet\t6\t6\t20\t20\t10\t2\t4\t30\t\n"))
	if err != nil {
		t.Fatal(err)
	}

	w := newFakePets()
	s := NewSummoner(w, tpl, d2summon.Normal, 1)
	place := func(i, n int) (int, int) { return 10, 20 }

	lvl, known := 1, true
	phys := map[int][3]int{1: {2, 6, 0}, 10: {13, 17, 60}, 20: {41, 45, 190}}
	s.SkillNow = func(name string) (int, int, int, bool) {
		if name != "Summon Spirit Wolf" || !known {
			return 0, 0, 0, false
		}

		p := phys[lvl]

		return p[0], p[1], p[2], true
	}

	o := &d2skill.SummonOrder{Key: "spiritwolf", PetType: "spiritwolf", Max: 5, Count: 1, Kind: "minion",
		SkillName: "Summon Spirit Wolf", SkillDmgMin: 2, SkillDmgMax: 6}

	ids, _ := s.Cast("hero", "spiritwolf", o, nil, place)
	st := w.stats[ids[0]]

	if st.DmgMin != 2 || st.DmgMax != 6 || st.Live == nil {
		t.Fatalf("spawn stats %+v", st)
	}

	maxHP, def := st.MaxHP, st.Defense

	for _, l := range []int{10, 20} {
		lvl = l
		mn, mx, ar := st.Live()

		if want := phys[l]; mn != want[0] || mx != want[1] || ar != want[2] {
			t.Errorf("owner skill level %d: live %d-%d ar %d, want %v", l, mn, mx, ar, want)
		}
	}

	if st.MaxHP != maxHP || st.Defense != def {
		t.Error("life and defense are fixed at the spawn")
	}

	// the owner lost the skill (or it is unknown): the minion keeps the numbers of its template alone
	known = false

	if mn, mx, _ := st.Live(); mn != 0 || mx != 0 {
		t.Errorf("no skill: %d-%d", mn, mx)
	}

	// a template without SkillDamage never gets a live recompute
	sk := &d2skill.SummonOrder{Key: "necroskeleton", PetType: "skeleton", Max: 2, Count: 1, Kind: "minion"}
	ids, _ = s.Cast("hero", "necroskeleton", sk, nil, place)

	if got := w.stats[ids[0]]; got.Live != nil || got.DmgMin != 2 || got.DmgMax != 4 {
		t.Errorf("skeleton %+v", got)
	}
}
