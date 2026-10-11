package d2monster

// Wave 20: Act 5 and endgame AIs compared with Game.exe 1.14b (notes:
// w20-ai-act5.md in the reverse-engineering notes). The handlers stay in their
// original files (ai_baal.go, ai_faithful_a.go, ai_boss2.go); this file holds
// the shared gates and the lists the progress counts read.

// TargetStater is STATE_UnitHasState on the attack target (a hero or a
// monster). Absent: no state is ever up.
type TargetStater interface {
	TargetHasState(b *Brain, t Target, state int) bool
}

// AuraTargetStateSource answers skills.txt auratargetstate of a monstats slot
// (-1 for none), the sibling of AuraStateSource.
type AuraTargetStateSource interface {
	SlotAuraTargetState(b *Brain, slot int) int
}

// auraTargetState is the slot's auratargetstate (-1 without a source).
func (c *Ctx) auraTargetState(slot int) int {
	if a, ok := c.W.(AuraTargetStateSource); ok {
		return a.SlotAuraTargetState(c.B, slot)
	}

	return -1
}

// skillStatesFree is the pair of tests the Baal throne and the second Ancient
// make before a skill (VERIFIED, 0x5ee400 and 0x5edeb0): the skill's aurastate
// must not already be up on the caster and its auratargetstate must not be up
// on the target. A state id below minState counts as "no state" (the throne
// accepts ids from 0, the Ancient only from 1); missing hosts mean no state is
// ever up.
func (c *Ctx) skillStatesFree(slot int, t Target, minState int) bool {
	if as := c.auraState(slot); as >= minState && c.W.HasState(c.B, as) {
		return false
	}

	if ts := c.auraTargetState(slot); ts >= minState {
		if s, ok := c.W.(TargetStater); ok && s.TargetHasState(c.B, t, ts) {
			return false
		}
	}

	return true
}

// FaithfulW20 lists the handlers newly counted as faithful in wave 20: the
// Baal encounter AIs, whose flows were read from the think functions and the
// differences fixed (notes: w20-ai-act5.md).
//
//	BaalThrone (skill gates; the exe has no wave gate), BaalTaunt (the taunter
//	hops onto the hero; the cast ends the think), BaalToStairs (quest hook and
//	state 0x92), BaalCrab and BaalCrabClone (engaged is line of sight, scan
//	scoring with leash 0x4b, anchor distance of the target, nearby is the
//	difficulty, clone death and wait rules).
func FaithfulW20() []string {
	return []string{"BaalThrone", "BaalTaunt", "BaalToStairs", "BaalCrab", "BaalCrabClone"}
}

// RefixedW20 lists handlers that were already counted (FaithfulA) and got a
// further fix from the exe in wave 20: Ancient (scan filter and scoring, state
// B skill gates) and MinionSpawner (class-family count).
func RefixedW20() []string { return []string{"Ancient", "MinionSpawner"} }

// ReverifiedW20 lists handlers read against the exe in wave 20 that already
// matched (not counted again): BaalTentacle, BaalMinion (one roll-order fix),
// DeathMauler, SuicideMinion, AncientStatue, Minion, NpcBarb, Wussie and the
// stand-still UberBaal (0x5fc430 is a bare return).
func ReverifiedW20() []string {
	return []string{"BaalTentacle", "BaalMinion", "DeathMauler", "SuicideMinion", "AncientStatue", "Minion",
		"NpcBarb", "Wussie", "UberBaal"}
}
