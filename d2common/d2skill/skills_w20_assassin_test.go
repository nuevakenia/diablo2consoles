package d2skill

import "testing"

func init() {
	rows = append(rows,
		row{"skill": "Sentry W20", "Id": "730", "charclass": "ass", "srvdofunc": "45", "summon": "chargeboltsentry",
			"pettype": "assassintrap", "petmax": "5", "summode": "S1", "manashift": "8"},
		row{"skill": "Sentinel W20", "Id": "731", "charclass": "ass", "srvdofunc": "44", "summon": "bladecreeper",
			"pettype": "assassintrap", "petmax": "5", "summode": "S1", "manashift": "8"},
	)
}

// SKILL_SummonSentryUnit (0x5d4790) computes the summon level like any pet; Blade Sentinel (0x5d49c0)
// then overwrites it with the caster's own level.
func TestSentryLevels(t *testing.T) {
	cf := newClassFixture(map[string]int{"Sentry W20": 6, "Sentinel W20": 6})
	cf.u.level = 40

	_, r := cf.cast("Sentry W20", 5, 5)
	if got, want := effectOf(t, r, "summon").Summon.Level, SummonLevel(40, 6); got != want {
		t.Errorf("sentry level %d, want %d", got, want)
	}

	_, r = cf.cast("Sentinel W20", 5, 5)
	if got := effectOf(t, r, "summon").Summon.Level; got != 40 {
		t.Errorf("blade sentinel level %d, want the caster's 40", got)
	}
}
