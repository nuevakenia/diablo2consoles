package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
)

// DiabloCandidates implements d2monster.DiabloTargetSource: the player list
// MONAI_Diablo_FindBestTarget 0x5e7ec0 walks (list heads at game+0x10f8, one
// bucket after another). Wave 20 reading of the exe: the candidate callback
// MONAI_Diablo_IsTargetNear 0x5e7e50 only demands the same act byte and an
// octagonal distance to the anchor below 0x3fc subtiles, which is the whole
// act, so every living hero of the act qualifies and each one counts toward
// the weights' player count (the exe tests the callback on the first unit of
// a bucket only; hero buckets hold one hero each, so the quirk is moot here).
//
// The scored stats are the ones the engine tracks: fire, cold and lightning
// resistance, life percent and the left/right skill (attack rank and total
// level). The physical/magic resist group (0x24/0x25) and the damage group
// (0x16, 0x31-0x3a) and state 0xb are NOT provided and stay 0 (UNVERIFIED
// that this changes the choice noticeably: both only add to the score).
func (d *Director) DiabloCandidates(b *d2monster.Brain) []d2monster.DiaCandidate {
	var out []d2monster.DiaCandidate

	for _, id := range d.targetIDs() {
		p := d.targets[id]
		if p == nil || !d.targetable(p) {
			continue
		}

		x, y := playerSubtile(p)
		t := d2monster.Target{ID: id, X: x, Y: y, Size: 1, IsPlayer: true}
		st := d2monster.DiaStats{LifePct: 100}

		if p.Stats.MaxHealth > 0 {
			st.LifePct = p.Stats.Health * 100 / p.Stats.MaxHealth
		}

		st.S27, st.S2B, st.S29 = d.Resists(t)
		st.LeftRank, st.LeftLevel, st.LeftID = heroSkillScore(p.LeftSkill)
		st.RightRank, st.RightLevel, st.RightID = heroSkillScore(p.RightSkill)

		out = append(out, d2monster.DiaCandidate{Target: t, Stats: st})
	}

	return out
}

func heroSkillScore(hs *d2hero.HeroSkill) (rank, level, id int) {
	if hs == nil || hs.SkillRecord == nil {
		return 0, 0, 0
	}

	return hs.SkillRecord.Attackrank, hs.SkillPoints, hs.ID
}
