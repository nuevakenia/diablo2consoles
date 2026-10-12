package d2skills

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2state"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// convertibleType is MONSTATS_IsRevivableType (V, wave 22), the type test inside MONAI_IsCorpseReviveCandidate
// 0x5dc1e0 that Conversion (SKILL_IsValidMonsterSkillTarget 0x56c0a0) and the corpse search of Revive share: the
// monster row must have the walk mode (MONSTATS2_IsModeAvailable(id, 2): the mWL column of monstats2), must NOT have
// bit 6 of the monstats bit word (the "boss" column: mask 0x40) and must have bit 16 of it (the "switchai"
// column: mask 0x10000). Bit numbers are the positions in the monstats loader table 0x652930..0x653000. The
// other parts of 0x5dc1e0 (not state 0x36, the unit's type word mask 0xa for unique / super unique, which
// canConvertMonster covers, mode eligibility) are tested elsewhere. A record that is missing is not refused (the
// engine has monsters without a record only in tests).
func convertibleType(stat *d2records.MonStatRecord, statEx *d2records.MonStat2Record) bool {
	if stat == nil {
		return true
	}

	if stat.IsSpecialBoss || !stat.IsAiSwitchable {
		return false
	}

	if statEx != nil && !statEx.HasAnimationMode[2] {
		return false
	}

	return true
}

// auraTargetMods are the stats an enemy aura puts on a monster: the aura's stat list with the slow clamp of the
// aura applier (0x5cd240, d2state.ClampAuraSlow) for the monster's cold effect.
func auraTargetMods(m *d2mapentity.Monster, stats []d2skill.StatMod) []d2state.StatMod {
	mods := statMods(stats)
	if m == nil || m.Stat == nil {
		return mods
	}

	return d2state.ClampAuraSlow(mods, coldEffect(m))
}

// whirlRefusedState is the engine name of state 12 of states.txt (the "inferno" row), the state that makes the
// Whirlwind start function return 0 (d2skill.WhirlwindRefusedState; the state ids are states.txt rows, pinned by d2skill.TestExeTableRows).
const whirlRefusedState = "inferno"
