package d2skills

import (
	"hash/fnv"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// unitRoll100 is the seed%100 roll the hit state appliers draw from the TARGET unit's own seed (VERIFIED at
// 0x578830 stun skip and 0x578ca0 shatter visual: both advance the seed of the unit that is hit, with the
// standard generator lo*0x6ac690c5+hi). Each monster gets its own generator derived from the engine seed and
// its id, so runs stay reproducible.
func (e *Engine) unitRoll100(id string) func() int {
	return func() int {
		if e.unitRolls == nil {
			e.unitRolls = map[string]*d2rand.Seed{}
		}

		s := e.unitRolls[id]
		if s == nil {
			h := fnv.New32a()
			_, _ = h.Write([]byte(id))
			s = d2rand.New(e.opt.Seed ^ h.Sum32())
			e.unitRolls[id] = s
		}

		return int(s.Roll(100))
	}
}

// monsterDataFlag8 is MONSTER_HasDataFlag16(unit, 8): the unit data type word has the unique bit (0x8,
// MonTypeUnique, confirmed by the predicate MONSTER_IsUnique and the 0xAC spawn packet bit order). Effect of the
// bit on states is VERIFIED (stun lands 10 percent of the time, freeze becomes chill, open wounds halved).
func monsterDataFlag8(m *d2mapentity.Monster) bool { return m.TypeFlags&d2mapentity.MonTypeUnique != 0 }
