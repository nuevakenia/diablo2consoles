package d2monsters

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// Summon caps as far as the exe has them (wave 24, read in Game.exe 1.14b):
//
//   - Hydra (SRVDO_144 0x5c8ab0): the skill's petmax calc is evaluated and passed, with the pettype, to
//     SKILL_ServerSpawnSummonedMonster for each of the three heads; that is the pet limit of the type
//     (PETS_SetPetLimitForType). The 1.14b row says 99, so the exe bound is 99, not the 9 the port used.
//     Heads live Param1 + (level-1) * Param2 frames and the casters cast on a delay, so the live count stays
//     near nine anyway.
//   - DiabPrison (SRVDO_104 0x5cb900 -> 0x5b1100): pettype "none", no petmax, no count: each cast makes four
//     units from the template at 0x73a668 (class boneprison1 + index) around the target. The exe has no live
//     cap; four is the size of one cast and stays a HOST bound (engine choice, the exe has none).
//   - Overseer Whip (131, 0x5d0580) and Impregnate (133, 0x5d0850) create no unit (a different effect), so
//     they have no cap.
//   - Nest users with their own think function bound themselves by aip values (selfLimitedSummoners). EvilHole
//     (0x5fa590) lays aip1 units in total; that total is the upper bound of its live count.
//
// exeSummonCap returns the cap the table gives and whether it is an exe value; (0, false) means the host bound
// of HostSummonCap applies.
func exeSummonCap(k SummonKind, rec *d2records.SkillRecord, caster *d2records.MonStatRecord, petmax int) (int, bool) {
	switch k {
	case SummonHydra:
		if petmax > 0 && rec != nil && rec.Pettype != "" && !strings.EqualFold(rec.Pettype, "none") {
			return petmax, true
		}
	case SummonNest:
		if caster != nil && strings.EqualFold(caster.AiKey, "EvilHole") {
			if n := maxInt3(caster.AiParameterNormal1, caster.AiParameterNightmare1, caster.AiParameterHell1); n > 0 {
				return n, true
			}
		}
	}

	return 0, false
}

func maxInt3(a, b, c int) int {
	if b > a {
		a = b
	}

	if c > a {
		a = c
	}

	return a
}
