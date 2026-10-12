package d2herostats

import (
	"bytes"
	"reflect"
	"strconv"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2animspeed"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
)

// Wave 31 mechanics leftovers: shapeshift FHR/FBR rows, per weapon class tables, attack speed with the weapon
// speed modifier, stamina numbers. Engine values come from the exe rules (see the notes w31-mech2.md).

func TestWerebearWerewolfRows(t *testing.T) {
	// werebear: TGGHHTH 5 frames at 184 (not the 7 frame Necromancer row).
	bear := DeriveBreakpoints(FHR, 5, 184)
	if want := []int{0, 5, 10, 16, 24, 37, 54, 86, 152, 360}; !reflect.DeepEqual(bear, want) {
		t.Errorf("werebear FHR %v", bear)
	}

	if nec, _ := Lookup("Necromancer", FHR); reflect.DeepEqual(bear, nec.Table) {
		t.Error("the werebear row must differ from the Necromancer row")
	}

	for _, tc := range []struct {
		note string
		kind Kind
		want []int
	}{
		{"werewolf", FHR, []int{0, 9, 20, 42, 86, 280}},
		{"werebear", FHR, []int{0, 5, 10, 16, 24, 37, 54, 86, 152, 360}},
		{"werewolf", FBR, []int{0, 7, 15, 27, 48, 86, 200}},
		{"werebear", FBR, []int{0, 5, 10, 16, 27, 40, 65, 109, 223}},
	} {
		found := false

		for _, e := range Tables {
			if e.Class == "Druid" && e.Kind == tc.kind && e.Note == tc.note {
				found = true

				if !reflect.DeepEqual(e.Table, tc.want) {
					t.Errorf("%s %s: %v want %v", tc.note, tc.kind, e.Table, tc.want)
				}

				if e.Unverified != "" {
					t.Errorf("%s %s should be settled, flag %q", tc.note, tc.kind, e.Unverified)
				}

				tok := d2animspeed.TokenWerewolf
				if tc.note == "werebear" {
					tok = d2animspeed.TokenWerebear
				}

				mode := "GH"
				if tc.kind == FBR {
					mode = "BL"
				}

				m, ok := d2animspeed.PlayerRecord(tok, mode, "HTH")
				if !ok || m.Frames != e.Anim.Frames || m.Speed != e.Anim.Speed {
					t.Errorf("%s %s anim %+v vs record %v", tc.note, tc.kind, e.Anim, m)
				}
			}
		}

		if !found {
			t.Errorf("missing %s %s", tc.note, tc.kind)
		}
	}
}

func TestPaladinShieldBlockSettled(t *testing.T) {
	// the shield is never part of the weapon class: with a one handed sword or mace (and a shield) the Paladin
	// plays PABL1HS / PABL1HT, both 3 frames at 256.
	want := []int{0, 13, 32, 86, 600}

	for _, wc := range []string{"HTH", "1HS", "1HT"} {
		e, ok := WeaponTable("Paladin", FBR, wc, "")
		if !ok || !reflect.DeepEqual(e.Table, want) {
			t.Errorf("paladin block %s: %v %v", wc, e.Table, ok)
		}
	}

	if e, _ := Lookup("Paladin", FBR); !reflect.DeepEqual(e.Table, want) || e.Unverified != "" {
		t.Errorf("default paladin block %v %q", e.Table, e.Unverified)
	}

	// the old community row (the Sorceress hit recovery row) matches no Paladin block record
	old := []int{0, 5, 9, 14, 20, 30, 42, 60, 86, 142, 280}
	for _, e := range WeaponTables("Paladin", FBR) {
		if reflect.DeepEqual(e.Table, old) {
			t.Errorf("paladin %s block has the old row", e.WClass)
		}
	}
}

// Hit and block records differ by weapon class for a few heroes only. Pinned so that a change of the data is
// noticed: (class, kind, weapon class) of every record whose speed differs from the hand to hand one.
func TestWeaponClassExceptions(t *testing.T) {
	type key struct {
		class string
		kind  Kind
		wc    string
	}

	got := map[key]int{}

	for class := range d2animspeed.ClassTokens {
		for _, k := range []Kind{FHR, FBR} {
			hth, ok := WeaponTable(class, k, "HTH", "")
			if !ok {
				t.Fatalf("%s %s has no hth record", class, k)
			}

			for _, wc := range d2animspeed.WeaponClasses {
				e, ok := WeaponTable(class, k, wc, "")
				if ok && (e.Anim.Speed != hth.Anim.Speed || e.Anim.Frames != hth.Anim.Frames) {
					got[key{class, k, wc}] = e.Anim.Speed
				}
			}
		}
	}

	want := map[key]int{
		{"Amazon", FBR, "1HS"}: 88, {"Paladin", FBR, "2HS"}: 168,
		{"Paladin", FHR, "2HT"}: 192, {"Paladin", FHR, "STF"}: 192, {"Druid", FHR, "1HS"}: 248,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("exceptions %v want %v", got, want)
	}
}

func TestWeaponTableMatchesDefault(t *testing.T) {
	// the hand to hand table equals the default Lookup table where the default is a hth record
	for _, c := range []string{"Amazon", "Sorceress", "Necromancer", "Paladin", "Barbarian", "Assassin"} {
		for _, k := range []Kind{FHR, FBR, FCR} {
			def, _ := Lookup(c, k)

			e, ok := WeaponTable(c, k, "HTH", "")
			if !ok || !reflect.DeepEqual(e.Table, def.Table) {
				t.Errorf("%s %s: %v vs default %v", c, k, e.Table, def.Table)
			}
		}
	}
}

// The attack length of the exe's event scheduler (d2skill.ScheduleActionEvents, ported from 0x551a20) agrees with
// AttackTicks for every class, weapon class and mode, across IAS and weapon speed.
func TestAttackGameFramesMatchScheduler(t *testing.T) {
	for class, tok := range d2animspeed.ClassTokens {
		for _, wc := range d2animspeed.WeaponClasses {
			for _, mode := range []string{"A1", "A2"} {
				m, ok := d2animspeed.PlayerRecord(tok, mode, wc)
				if !ok {
					continue
				}

				start := d2animspeed.AttackStartFrame(class, wc)
				if start != d2skill.AttackStartFrame(exeIndex(class), lower(wc)) {
					t.Fatalf("start frame %s %s", class, wc)
				}

				for _, wsm := range []int{-30, -10, 0, 10} {
					for _, ias := range []int{0, 20, 60, 120, 400} {
						rate := d2animspeed.AttackRate(m.Speed, d2animspeed.AttackStat44(wsm, 0), ias, 0, false)
						_, end := d2skill.ScheduleActionEvents(make([]byte, m.Frames), start, rate, m.Frames<<8)

						got, _ := AttackGameFrames(class, wc, mode, wsm, 0, ias)
						if got != end {
							t.Fatalf("%s %s %s wsm %d ias %d: %d scheduler %d", class, wc, mode, wsm, ias, got, end)
						}
					}
				}
			}
		}
	}
}

func exeIndex(class string) int {
	return map[string]int{"Amazon": 0, "Sorceress": 1, "Necromancer": 2, "Paladin": 3, "Barbarian": 4, "Druid": 5, "Assassin": 6}[class]
}

func lower(s string) string { return string(bytes.ToLower([]byte(s))) }

func TestIASTableAnchors(t *testing.T) {
	// Paladin, one handed sword (speed 0): 15 frames, first breakpoint 9 IAS
	tab, ok := IASTable("Paladin", "1HS", "A1", 0, 0)
	if !ok || len(tab) < 4 || tab[0] != 0 || tab[1] != 9 {
		t.Fatalf("paladin 1hs %v", tab)
	}

	// each entry is the smallest IAS that reaches its length
	for i := 1; i < len(tab); i++ {
		a, _ := AttackGameFrames("Paladin", "1HS", "A1", 0, 0, tab[i]-1)
		b, _ := AttackGameFrames("Paladin", "1HS", "A1", 0, 0, tab[i])

		if b >= a {
			t.Errorf("breakpoint %d (%d) does not shorten: %d -> %d", i, tab[i], a, b)
		}
	}

	// the weapon speed modifier changes the table: a fast weapon starts shorter, a slow one longer (the
	// breakpoints themselves are not ordered by speed, they follow where the rate crosses a whole frame count)
	fast, _ := IASTable("Paladin", "1HS", "A1", -20, 0)
	slow, _ := IASTable("Paladin", "1HS", "A1", 10, 0)

	f0, _ := AttackGameFrames("Paladin", "1HS", "A1", -20, 0, 0)
	s0, _ := AttackGameFrames("Paladin", "1HS", "A1", 10, 0, 0)

	if !(f0 < 15 && 15 < s0) || reflect.DeepEqual(fast, tab) || reflect.DeepEqual(slow, tab) {
		t.Errorf("wsm: fast %d %v normal %v slow %d %v", f0, fast[:2], tab[:2], s0, slow[:2])
	}

	// the Assassin entries of the default list equal the derived tables
	e, _ := Lookup("Assassin", IAS)
	if w, _ := WeaponTable("Assassin", IAS, "HTH", "A1"); !reflect.DeepEqual(w.Table, e.Table) {
		t.Errorf("assassin hth %v vs %v", w.Table, e.Table)
	}
}

func TestStaminaRunTime(t *testing.T) {
	data := wikiTables(t, "CharStats.txt")

	lines := bytes.Split(data, []byte("\n"))
	cols := map[string]int{}

	for i, c := range bytes.Split(bytes.TrimRight(lines[0], "\r"), []byte("\t")) {
		cols[string(c)] = i
	}

	// seconds a fresh hero can run: stamina*256 raw / (RunDrain*2 per frame) / 25 frames per second
	want := map[string]float64{
		"Amazon": 84 * 256 / 40.0 / 25, "Sorceress": 74 * 256 / 40.0 / 25, "Necromancer": 79 * 256 / 40.0 / 25,
		"Paladin": 89 * 256 / 40.0 / 25, "Barbarian": 92 * 256 / 40.0 / 25, "Druid": 84 * 256 / 40.0 / 25,
		"Assassin": 95 * 256 / 30.0 / 25,
	}

	seen := 0

	for _, ln := range lines[1:] {
		f := bytes.Split(bytes.TrimRight(ln, "\r"), []byte("\t"))
		if len(f) <= cols["RunDrain"] {
			continue
		}

		w, ok := want[string(f[0])]
		if !ok {
			continue
		}

		stam, _ := strconv.Atoi(string(f[cols["stamina"]]))
		drain, _ := strconv.Atoi(string(f[cols["RunDrain"]]))

		secs := float64(stam*256) / float64(StaminaDrainPerStep(drain, -1, 0, false)) / FramesPerSecond
		if d := secs - w; d > 0.01 || d < -0.01 {
			t.Errorf("%s runs %.2f s, want %.2f", f[0], secs, w)
		}

		seen++
	}

	if seen != 7 {
		t.Errorf("classes seen %d", seen)
	}

	// heavy body armor (speed column 10) doubles the drain, light armor (5) does not; stat 0x9a lowers it by percent
	if a, b, c := StaminaDrainPerStep(20, 0, 0, false), StaminaDrainPerStep(20, 5, 0, false), StaminaDrainPerStep(20, 10, 0, false); a != 40 || b != 40 || c != 80 {
		t.Errorf("armor speed drain %d %d %d", a, b, c)
	}

	if got := StaminaDrainPerStep(20, 10, 50, false); got != 40 {
		t.Errorf("50 percent drain reduction: %d", got)
	}

	// standing recovery refills the whole bar in 256 frames of maxraw>>8 each (about ten seconds)
	maxRaw := 92 * 256
	if got := StaminaRegenPerTick(maxRaw, 0, ModeNeutral, 0); got != 92 {
		t.Errorf("standing regen %d", got)
	}

	if got := StaminaRegenPerTick(maxRaw, 0, ModeTownWalk, 0); got != 46 {
		t.Errorf("town walk regen %d", got)
	}

	if got := StaminaRegenPerTick(maxRaw, 0, ModeRun, 0); got != 0 {
		t.Errorf("running regen without stat 0x1c: %d", got)
	}
}
