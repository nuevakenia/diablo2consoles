package d2skills

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"

// ConcentrationDamagePct implements d2skill.ConcentrationHolder: the damage
// percent of the hero's Concentration state alone (0x647550).
func (h *heroUnit) ConcentrationDamagePct() int {
	in := h.e.setOf(h.p.ID()).Get(h.e.frame, "concentration")
	if in == nil {
		return 0
	}

	v := 0

	for _, m := range in.Mods {
		if m.Stat == "damagepercent" {
			v += m.Value
		}
	}

	return v
}

var _ d2skill.ConcentrationHolder = (*heroUnit)(nil)

// RunVelocity implements d2skill.RunVelocityHolder: the class run velocity of charstats.txt, the base of the
// Charge rush (SRVST_031 0x5cdbb0; the byte is charstats +0x41, the RunVelocity column, shifted 8 bits: VERIFIED from
// the loader 0x612590 and the disassembly).
func (h *heroUnit) RunVelocity() int {
	if h.merc != nil || h.p == nil || h.e.asset == nil {
		return 0
	}

	if st := h.e.asset.Records.Character.Stats[h.p.Class]; st != nil {
		return st.VelocityRun
	}

	return 0
}

var _ d2skill.RunVelocityHolder = (*heroUnit)(nil)
