package d2gamescreen

import "github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"

// pvpHitRollsBlock says whether the defender rolls its shield block for a hit of a hostile player: every melee
// swing (no skill name), and of the skill hits only those the attacker marked Blockable (the direct hit of a
// missile with physical damage; MISSILE_BuildHitFlagsAndRollBlock 0x5ab220). Fire Ball, Meteor and Blizzard hits
// carry no physical damage or are area hits, so they are never blocked. Kill reports never get here.
func pvpHitRollsBlock(p d2netpacket.PvPHitPacket) bool {
	return p.Skill == "" || p.Blockable
}
