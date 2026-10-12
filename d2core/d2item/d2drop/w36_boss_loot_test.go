package d2drop

import (
	"fmt"
	"regexp"
	"sort"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
)

// w36 Monte Carlo of the boss treasure classes on all three difficulties with
// the real tables (D2_TABLES): plausibility of the drop counts, the quality
// split and the special item families (runes, gems, jewels, charms, keys,
// essences, tokens). Absolute numbers are logged for the notes.

var (
	w36Rune  = regexp.MustCompile(`^r[0-3][0-9]$`)
	w36Gem   = regexp.MustCompile(`^g[cfszvlp][a-z]$`)
	w36Charm = regexp.MustCompile(`^cm[123]$`)
	w36Key   = regexp.MustCompile(`^(pk[123]|tes|ceh|bet|fed)$`)
	w36Ess   = regexp.MustCompile(`^(tes|ceh|bet|fed)$`)
)

type w36Stat struct {
	kills, drops int
	q            map[Quality]int
	fam          map[string]int
	maxRune      string
}

func w36Run(t *testing.T, d *Dropper, tc string, ilvl, players, n int, seed uint32, mf int) *w36Stat {
	t.Helper()

	st := &w36Stat{kills: n, q: map[Quality]int{}, fam: map[string]int{}}
	rng := d2rand.New(seed)

	for i := 0; i < n; i++ {
		out, err := d.Roll(&Context{RNG: rng, ILvl: ilvl, Players: players, MagicFind: mf, QualityLevel: ilvl}, tc)
		if err != nil {
			t.Fatalf("%s: %v", tc, err)
		}

		for _, dr := range out {
			st.drops++
			st.q[dr.Quality]++

			c := dr.Code

			switch {
			case w36Rune.MatchString(c):
				st.fam["rune"]++

				if c > st.maxRune {
					st.maxRune = c
				}
			case w36Ess.MatchString(c):
				st.fam["essence"]++
			case w36Key.MatchString(c):
				st.fam["key"]++
			case c == "toa":
				st.fam["token"]++
			case c == "jew":
				st.fam["jewel"]++
			case w36Charm.MatchString(c):
				st.fam["charm"]++
			case c == "gld":
				st.fam["gold"]++
			case w36Gem.MatchString(c):
				st.fam["gem"]++
			}
		}
	}

	return st
}

func (s *w36Stat) per(k int) float64 { return float64(k) / float64(s.kills) }

func (s *w36Stat) String() string {
	keys := make([]string, 0, len(s.fam))
	for k := range s.fam {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	out := fmt.Sprintf("drops/kill %.2f q[low %.3f nor %.3f sup %.3f mag %.3f set %.4f rare %.3f uni %.4f] maxrune %s",
		s.per(s.drops), s.per(s.q[1]), s.per(s.q[2]), s.per(s.q[3]), s.per(s.q[4]), s.per(s.q[5]), s.per(s.q[6]), s.per(s.q[7]), s.maxRune)

	for _, k := range keys {
		out += fmt.Sprintf(" %s %.3f", k, s.per(s.fam[k]))
	}

	return out
}

func TestW36BossLootDistributions(t *testing.T) {
	rt := loadReal(t)
	d := &Dropper{TCs: rt.tcs, Items: rt.items, Ratios: rt}

	type boss struct {
		name string
		tc   [3]string
		// item level on normal / nightmare / hell (boss monster levels, UNVERIFIED approximations)
		ilvl [3]int
	}

	bosses := []boss{
		{"Countess", [3]string{"Countess", "Countess (N)", "Countess (H)"}, [3]int{20, 47, 82}},
		{"Andariel", [3]string{"Andariel", "Andariel (N)", "Andariel (H)"}, [3]int{12, 49, 83}},
		{"Mephisto", [3]string{"Mephisto", "Mephisto (N)", "Mephisto (H)"}, [3]int{27, 56, 87}},
		{"Diablo", [3]string{"Diablo", "Diablo (N)", "Diablo (H)"}, [3]int{30, 60, 94}},
		{"Baal", [3]string{"Baal", "Baal (N)", "Baal (H)"}, [3]int{35, 64, 97}},
	}

	const kills = 3000

	for _, b := range bosses {
		var res [3]*w36Stat

		for diff := 0; diff < 3; diff++ {
			if _, ok := rt.tcs.TreasureClass(b.tc[diff]); !ok {
				t.Fatalf("TC %q missing", b.tc[diff])
			}

			res[diff] = w36Run(t, d, b.tc[diff], b.ilvl[diff], 1, kills, uint32(100+diff), 0)
			t.Logf("%s diff %d ilvl %d: %s", b.name, diff, b.ilvl[diff], res[diff])

			// Boss classes always give something, never more than the 6 item cap.
			if per := res[diff].per(res[diff].drops); per < 2 || per > 6.01 {
				t.Errorf("%s diff %d: %.2f drops per kill outside 2..6", b.name, diff, per)
			}
		}

		// The boss quality columns (983/1024 of the unique, set, rare rolls) push drops to
		// magical or better; the share of unique+set grows with the difficulty (higher ilvl).
		good := func(s *w36Stat) float64 { return s.per(s.q[4] + s.q[5] + s.q[6] + s.q[7]) }
		top := func(s *w36Stat) float64 { return s.per(s.q[5] + s.q[7]) }

		if good(res[2]) < 0.9 {
			t.Errorf("%s hell: %.2f magic-or-better items per kill, expected most drops to be magical or better", b.name, good(res[2]))
		}

		if top(res[2]) <= top(res[0]) {
			t.Errorf("%s: unique+set per kill hell %.4f should exceed normal %.4f", b.name, top(res[2]), top(res[0]))
		}
	}

	// Countess: runes by difficulty, the highest rune is the verified cap per difficulty
	// (r08 Ral, r16 Io, r24 Ist) and the rune count rises.
	caps := [3]string{"r08", "r16", "r24"}

	// (the "Good" classes of the Countess item pool carry their own Runes N entries, so the
	// cap is checked on the rune sub-class itself.)
	for diff, tc := range []string{"Countess Rune", "Countess Rune (N)", "Countess Rune (H)"} {
		s := w36Run(t, d, tc, []int{20, 47, 82}[diff], 1, kills, 9, 0)
		if s.maxRune == "" || s.maxRune > caps[diff] {
			t.Errorf("%s: highest rune %q above the cap %s", tc, s.maxRune, caps[diff])
		}

		if s.fam["rune"] == 0 {
			t.Errorf("%s: no runes dropped in %d kills", tc, kills)
		}
	}

	// Hell bosses drop their essences / keys (Mephisto hate essence, Diablo terror, Baal destruction).
	for tc, code := range map[string]string{"Mephisto (H)": "ceh", "Diablo (H)": "bet", "Baal (H)": "fed", "Andariel (H)": "tes"} {
		found := false
		rng := d2rand.New(5)

		for i := 0; i < 4000 && !found; i++ {
			out, _ := d.Roll(&Context{RNG: rng, ILvl: 90, Players: 1, QualityLevel: 90}, tc)
			for _, dr := range out {
				if dr.Code == code {
					found = true
				}
			}
		}

		// The essence entries are guaranteed or not depending on the class; only log when absent.
		t.Logf("%s drops %s within 4000 kills: %v", tc, code, found)
	}
}

// More magic find raises the unique share of a hell boss and more players raise the
// drop count (NoDrop shrinks).
func TestW36BossLootScaling(t *testing.T) {
	rt := loadReal(t)
	d := &Dropper{TCs: rt.tcs, Items: rt.items, Ratios: rt}

	a := w36Run(t, d, "Baal (H)", 97, 1, 3000, 1, 0)
	b := w36Run(t, d, "Baal (H)", 97, 1, 3000, 1, 300)

	if b.per(b.q[7]) <= a.per(a.q[7]) {
		t.Errorf("300 MF unique/kill %.4f not above 0 MF %.4f", b.per(b.q[7]), a.per(a.q[7]))
	}

	p8 := w36Run(t, d, "Countess Rune (H)", 82, 8, 3000, 2, 0)
	p1 := w36Run(t, d, "Countess Rune (H)", 82, 1, 3000, 2, 0)

	if p8.per(p8.drops) <= p1.per(p1.drops) {
		t.Errorf("8 players %.3f drops/kill not above 1 player %.3f", p8.per(p8.drops), p1.per(p1.drops))
	}
}
