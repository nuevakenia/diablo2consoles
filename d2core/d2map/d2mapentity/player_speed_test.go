package d2mapentity

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

// Every hero class must have a walk and run speed: the Druid (hero enum 7) used to get none and froze when run was
// turned on.
func TestEveryHeroHasMoveSpeed(t *testing.T) {
	heroes := []d2enum.Hero{
		d2enum.HeroBarbarian, d2enum.HeroNecromancer, d2enum.HeroPaladin, d2enum.HeroAssassin,
		d2enum.HeroSorceress, d2enum.HeroAmazon, d2enum.HeroDruid,
	}

	for _, h := range heroes {
		p := &Player{Class: h}

		walk, run := p.moveSpeed(false), p.moveSpeed(true)
		if walk <= 0 || run <= walk {
			t.Errorf("%v: walk %v run %v", h, walk, run)
		}
	}
}
