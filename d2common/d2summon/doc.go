// Package d2summon models summoned minions of the player: the stats a minion
// gets from monstats.txt plus the summoning skill's level-scaled modifiers
// (Stats), and the per-owner limits that decide how many of a kind may live
// (Roster). The follow / attack behaviour lives in d2monster (NecroPet,
// Raven, Vines, AssassinSentry AIs).
//
// The package is pure: it consumes the SummonOrder that d2skill produces and
// the tab-separated monstats.txt, and knows nothing about the engine.
//
// Evidence levels follow the repository convention. VERIFIED: the skills.txt
// columns and their meaning (summon, pettype, petmax, aurastat*, passivestat*,
// calc1; skills-combat.md) and the pet AI constants (monster-ai-2.md 6.2).
// UNVERIFIED (marked in the code): the rounding of the armor class percent
// step and the Raven top-up rule. VERIFIED since wave 37: the life and
// damage order (SKILL_SpawnSummonedMinion 0x5c22c0) and the eviction of the
// oldest minion of a pet type at the limit (0x573a30).
package d2summon
