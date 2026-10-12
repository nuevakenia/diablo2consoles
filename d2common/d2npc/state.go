package d2npc

// GreetMode is the mode SOUND_PickNpcGreeting (0x4dd4c0) is called with.
type GreetMode int

// The greeting modes: a normal pick (greeting / inactive / time of day groups) or the GREETING_RETURN line.
const (
	GreetNormal GreetMode = 0
	GreetReturn GreetMode = 2
)

// npcState is the run time part of one entry of the state array.
type npcState struct {
	chosen        int  // +0xd
	spoke         bool // +0x11: a speech was shown, roll the gossip again after a room change
	returnPending bool // +0x12: set by packet 0x91, cleared when the NPC is opened
	allowed       bool // +0x15: the greeting may play (starts true, cleared when it does)
}

// Gate is the client's memory of the NPC state array: the greeting rules of UI_StartNpcInteraction (0x4b2f50)
// and the gossip index of every NPC (UI_InitNpcMenuEntry), re-rolled the way UI_ResetNpcMenuEntriesOnPlayerChange
// (0x4b0630, called when the hero's room changes) does.
type Gate struct {
	st      []npcState
	started bool // DAT_007b8cf2: the first speech rolled every entry
}

// NewGate makes the state of a freshly entered game: every greeting allowed, nothing pending.
func NewGate() *Gate {
	g := &Gate{st: make([]npcState, len(gossipEntries))}
	g.Reset()

	return g
}

func (g *Gate) index(class int) int {
	for i := range gossipEntries {
		if gossipEntries[i].Class == class {
			return i
		}
	}

	return -1
}

// Reset is what FUN_004afb00 does when a game is (re)entered: it clears the pending flags.
func (g *Gate) Reset() {
	for i := range g.st {
		g.st[i] = npcState{allowed: true}
	}

	g.started = false
}

// MarkReturn is packet 0x91 (UI_MarkNpcReturnGreetingFlags 0x4afd20): the listed classes get a return
// greeting the next time they are opened. A class without an entry is ignored.
func (g *Gate) MarkReturn(classes ...int) {
	for _, c := range classes {
		if i := g.index(c); i >= 0 {
			g.st[i].returnPending = true
		}
	}
}

// ReturnPending tells whether the class has a return greeting waiting.
func (g *Gate) ReturnPending(class int) bool {
	i := g.index(class)

	return i >= 0 && g.st[i].returnPending
}

// OpenGreeting is the greeting part of UI_StartNpcInteraction after the menu is built. A class that is not in
// the state array always plays a normal greeting. Otherwise the return flag selects the return line and is
// cleared, and the greeting plays only while the allowed flag is set (it is cleared by playing).
func (g *Gate) OpenGreeting(class int) (play bool, mode GreetMode) {
	i := g.index(class)
	if i < 0 {
		return true, GreetNormal
	}

	s := &g.st[i]

	mode = GreetNormal

	if s.returnPending {
		mode = GreetReturn
		s.returnPending = false
	}

	if !s.allowed {
		return false, mode
	}

	s.allowed = false

	return true, mode
}

// RoomChanged is UI_ResetNpcMenuEntriesOnPlayerChange for the local hero: every greeting is allowed again and
// the NPCs that spoke since get a new gossip index (reroll is the picker bound to the hero).
func (g *Gate) RoomChanged(reroll func(e *Entry) int) {
	for i := range g.st {
		s := &g.st[i]
		if s.spoke {
			s.spoke = false
			s.chosen = reroll(&gossipEntries[i])
		}

		s.allowed = true
	}
}

// Spoke records that a speech of the NPC was shown (UI_ShowNpcMenuAfterGreeting sets +0x11). The first speech
// of the game rolls the gossip index of every NPC (UI_InitAllNpcMenuEntries).
func (g *Gate) Spoke(class int, reroll func(e *Entry) int) {
	if !g.started {
		g.started = true

		for i := range g.st {
			g.st[i].chosen = reroll(&gossipEntries[i])
		}
	}

	if i := g.index(class); i >= 0 {
		g.st[i].spoke = true
	}
}

// Chosen is the gossip record index of the NPC (+0xd); 0 until the first speech rolled it.
func (g *Gate) Chosen(class int) int {
	if i := g.index(class); i >= 0 {
		return g.st[i].chosen
	}

	return 0
}
