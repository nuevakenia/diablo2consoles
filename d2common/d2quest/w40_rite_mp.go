package d2quest

// Wave 40: the open items of Rite of Passage, read from the exe (read-only Ghidra). Addresses: the altar 0x58b0f0, the
// per-hero log callback 0x589c70, the introduction 0x58a0c0 / 0x589d90, the spawn 0x589f10, the reset 0x589e50, the
// portal counters 0x58ace0 / 0x58ad30, the death 0x58a7b0 with its two per-hero callbacks 0x58a600 and 0x58a5a0 and the
// party callback 0x58a4f0.

// Timing of the statue wake-up (VERIFIED 0x589d90 / 0x589f10): the introduction puts the three statues in mode 3 at
// once and schedules the spawn event for the current game frame plus 0x14 (20 frames, 0.8 s at 25 frames per second);
// a spawn that finds no room tries again 10 frames later.
const (
	RiteStatueWakeFrames = 20
	RiteSpawnRetryFrames = 10
	RiteFramesPerSecond  = 25

	riteAltarMaxLiveState = 4 // from node state 4 on the altar only goes to mode 1 (0x58b0f0)
)

// RiteNode is the part of the game's quest node the altar reads: Active is the byte at +9 (the node is switched on in
// this game), Sub the byte at +0xb (3 once the quest log was refreshed for the introduction), State the byte at +0xc.
type RiteNode struct {
	Active bool
	Sub    int
	State  int
}

// RiteAltarResult is what operating the altar (0x58b0f0, altar in mode 0) does.
type RiteAltarResult struct {
	ResetAncients bool // a town portal is open in the arena: the Ancients are reset first (0x589e50)
	Speak         bool // message 20002 (0x4e22), the introduction, goes from the altar to the hero
	NewState      int  // the node state afterwards (the altar lifts a state below 2 to 2, message 0x59f)
	RefreshLog    bool // the log callback 0x589c70 is queued for every hero and the sub-state becomes 3
}

// RiteAltarStep is the decision table of the altar, VERIFIED from the disassembly of 0x58b0f0:
//   - a node that is not active in this game: reset when a portal is open, then speak;
//   - an active node in state 4 or higher: nothing (the altar only goes to mode 1);
//   - an active node below state 4: reset when a portal is open, speak, lift state <2 to 2 and, when the sub-state
//     is not 3 yet, set it to 3 and queue the log callback for every hero.
//
// In every case the altar then goes to mode 1 and the quest data field +0x4c becomes 2 (when the altar was in mode 0).
func RiteAltarStep(n RiteNode, portalsOpen int) RiteAltarResult {
	res := RiteAltarResult{NewState: n.State}
	if n.Active && n.State >= riteAltarMaxLiveState {
		return res
	}

	res.ResetAncients = portalsOpen > 0
	res.Speak = true

	if !n.Active {
		return res
	}

	if n.State < 2 {
		res.NewState = 2
	}

	res.RefreshLog = n.Sub != 3

	return res
}

// RiteLogRefreshes says whether the per-hero callback 0x589c70 sends the quest-log update of the Rite node to a hero:
// when the record has bit 0 or bit 15 (done / complete on entry) and bit 13 or bit 14 (goal reached). It changes no
// bit; it only refreshes the client's log.
func RiteLogRefreshes(rec func(bit int) bool) bool {
	return (rec(FlagRewardGranted) || rec(FlagCompletedEarly)) && (rec(FlagPrimaryGoal) || rec(FlagCompletedNow))
}

// RiteReArmsOnClose documents the exe: a town portal opened in level 120 (0x58ace0) counts up and clears both armed
// bytes (+0x10, +0x11) and resets the Ancients; a closed one (0x58ad30) counts down and sets +0x11 only when +0x10
// is still set. +0x10 is set only by the introduction (0x58a0c0), which needs the counter at 0, so a close after a
// reset NEVER re-arms the fight (VERIFIED by reading the three functions together). The reset (0x589e50) puts the
// statues and the altar back in mode 0: the hero operates the altar once more and the Ancients wake again.
const RiteReArmsOnClose = false

// RiteHero describes one hero of the game for the death of the third Ancient.
type RiteHero struct {
	ID    string
	Level int
	Area  int    // level id the hero stands in
	Party string // party id, "" without a party
	Done  bool   // slot-39 bit 0 already set
}

// RiteOutcome is what the death does to one hero's Rite record.
type RiteOutcome int

// The outcomes: nothing, the reward (bits 0 and 13 and the experience) or only the goal mark (bit 14).
const (
	RiteNothing RiteOutcome = iota
	RiteReward
	RiteGoalOnly
)

// RiteDeathOutcomes is the shared quest state when the third Ancient dies (VERIFIED 0x58a7b0, 0x58a600, 0x58a5a0,
// 0x58a4f0), for every hero of the game. In the exe's order:
//  1. the killer, when its bit 0 is clear and its level reaches 20*(difficulty+1), gets the reward (no area test);
//  2. every hero in the game, with bit 0 clear, level at the gate and standing in level 120 gets the reward, and then
//     the members of its party that stand in Act 5, with bit 0 clear and level at the gate, get it too;
//  3. every hero that still has bit 0 clear anywhere in the game gets bit 14 (goal reached) and a log update, but no
//     reward and no experience (the exe also tests bit 13; a hero with bit 13 set and bit 0 clear cannot occur here).
//
// A hero already done is left alone. The party spread of step 2 starts only from a hero whose own bit 0 is still
// clear at that moment (the killer, rewarded in step 1, does not spread: a quirk kept as it is).
func RiteDeathOutcomes(killer string, difficulty int, heroes []RiteHero) map[string]RiteOutcome {
	gate := RiteLevelGate(difficulty)
	out := make(map[string]RiteOutcome, len(heroes))
	done := make(map[string]bool, len(heroes))

	for _, h := range heroes {
		done[h.ID] = h.Done
	}

	pay := func(h RiteHero) {
		done[h.ID] = true
		out[h.ID] = RiteReward
	}

	for _, h := range heroes {
		if h.ID == killer && !done[h.ID] && h.Level >= gate {
			pay(h)
		}
	}

	for _, h := range heroes {
		if done[h.ID] || h.Level < gate || h.Area != LevelSummit {
			continue
		}

		pay(h)

		if h.Party == "" {
			continue
		}

		for _, m := range heroes {
			if m.Party == h.Party && !done[m.ID] && m.Level >= gate && actOfLevel(m.Area) == 4 {
				pay(m)
			}
		}
	}

	for _, h := range heroes {
		if !done[h.ID] {
			out[h.ID] = RiteGoalOnly
		}
	}

	return out
}

// RiteShared applies an outcome decided elsewhere to this game's hero (another player killed the third Ancient;
// quest records are client-owned in OpenDiablo2, so the server sends each client its outcome). It reports whether
// the record changed. A reward needs the level gate and a clear bit 0; the goal mark needs bits 0 and 13 clear.
// Experience comes back as an effect.
func (g *Game) RiteShared(o RiteOutcome) bool {
	q := g.byID[QuestRite]
	if q == nil {
		return false
	}

	switch o {
	case RiteReward:
		if g.get(q, FlagRewardGranted) || g.Hero.Level < RiteLevelGate(g.Difficulty) {
			return false
		}

		g.setState(q, 4)
		g.set(q, FlagPrimaryGoal, "Rite shared with the party: goal")
		g.set(q, FlagRewardGranted, "Rite shared with the party: done (VERIFIED 0x58a600 / 0x58a4f0)")
		g.clear(q, FlagRewardPending, "Rite shared with the party")
		g.globalDone(q)
		g.emit(Effect{Kind: EffectReward, Quest: q.ID, Code: "experience", Value: RiteExperience(g.Difficulty),
			Note: "Rite of Passage shared with the party (VERIFIED 0x58a600 / 0x58a4f0)"})
		g.chainFrom(q)

		return true
	case RiteGoalOnly:
		if g.get(q, FlagRewardGranted) || g.get(q, FlagPrimaryGoal) {
			return false
		}

		g.set(q, FlagCompletedNow, "Rite finished by the party without this hero: bit 14 (VERIFIED 0x58a5a0)")

		return true
	}

	return false
}

// RiteAltarLift moves the node state of this game's Rite quest from below 2 to 2 when the altar is used (0x58b0f0,
// message 0x59f). It reports whether it changed anything. The state is the engine's own number for the node state.
func (g *Game) RiteAltarLift() bool {
	q := g.byID[QuestRite]
	if q == nil || q.State >= 2 {
		return false
	}

	g.setState(q, 2)

	return true
}
