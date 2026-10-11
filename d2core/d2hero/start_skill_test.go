package d2hero

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

func TestStartSkillName(t *testing.T) {
	for _, c := range []struct {
		name  string
		stats *d2records.CharStatRecord
		hero  d2enum.Hero
		want  string
	}{
		{"sorceress default", &d2records.CharStatRecord{}, d2enum.HeroSorceress, "Fire Bolt"},
		{"sorceress nil record", nil, d2enum.HeroSorceress, "Fire Bolt"},
		{"column wins", &d2records.CharStatRecord{StartSkillBonus: " Charged Bolt "}, d2enum.HeroSorceress, "Charged Bolt"},
		{"attack is no start skill", &d2records.CharStatRecord{StartSkillBonus: "Attack"}, d2enum.HeroBarbarian, ""},
		{"other classes none", &d2records.CharStatRecord{}, d2enum.HeroNecromancer, ""},
	} {
		if got := startSkillName(c.stats, c.hero); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
