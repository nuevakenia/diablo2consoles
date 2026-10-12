package d2audio

import "strings"

// Hero voice lines (Game.exe 1.14b, UNITSND_PlayUnitSoundById 0x4c8470 and the
// per class table at 0x728d24, 7 records of 11 Sounds.txt indices).
//
// Verified from the exe: the voice id selects a Sounds.txt row of the hero's
// class record:
//
//	0x13 impossible   0x14 cantuseyet   0x15 needmana
//	0x16 needkey      0x17 cantcarry    0x18 not_in_town
//	0x19..0x20 chat_a..chat_h (record base +2 per id; only when no unit sound plays)
//	0x21..0x53 record base 9 + (id-0x21): the quest and area lines starting with
//	           act1_complete_andariel (act1/act2/act3... complete, find, near...)
//
// Record slots 0 and 1 are the hit and death vocals (played by the mode sounds,
// not by this function). The six "refusal" ids (0x13..0x18) are suppressed while
// a unit sound plays and when the same id repeats within 0x4a (74) ticks; a dead
// hero says nothing. Quest lines 0x21.. start 12 ticks late except
// 0x26,0x28..0x2c,0x2e,0x2f,0x37,0x38 which start at once.

// Hero voice ids of UNITSND_PlayUnitSoundById.
const (
	VoiceImpossible = 0x13
	VoiceCantUse    = 0x14
	VoiceNeedMana   = 0x15
	VoiceNeedKey    = 0x16
	VoiceCantCarry  = 0x17
	VoiceNotInTown  = 0x18
	VoiceChatFirst  = 0x19
	VoiceChatLast   = 0x20
	VoiceQuestFirst = 0x21
	VoiceQuestLast  = 0x53

	// VoiceRepeatTicks is the minimum gap before the same refusal voice repeats.
	VoiceRepeatTicks = 0x4a
	// VoiceQuestDelayTicks is the start delay of most quest lines.
	VoiceQuestDelayTicks = 12
)

var refusalSuffix = map[int]string{
	VoiceImpossible: "impossible_1", VoiceCantUse: "cantuseyet", VoiceNeedMana: "needmana_1",
	VoiceNeedKey: "needkey_1", VoiceCantCarry: "cantcarry_1", VoiceNotInTown: "not_in_town_1",
}

// HeroClassPrefixes are the Sounds.txt handle prefixes by d2s class number.
var HeroClassPrefixes = []string{"amazon", "sorceress", "necromancer", "paladin", "barbarian", "druid", "assassin"}

// VoiceHandle returns the Sounds.txt handle of voice id for a class prefix
// (amazon, ...). Hit and death are not voice ids. Quest lines need indexOf (handle ->
// Sounds.txt index) and handleAt (index -> handle); ok is false when the id has no row.
func VoiceHandle(class string, id int, indexOf func(string) (int, bool), handleAt func(int) string) (string, bool) {
	class = strings.ToLower(class)

	switch {
	case id >= VoiceImpossible && id <= VoiceNotInTown:
		h := class + "_" + refusalSuffix[id]
		if class == "assassin" && id == VoiceCantCarry {
			h = "assassin_cant_carry" // the one row that breaks the naming
		}

		return h, true
	case id >= VoiceChatFirst && id <= VoiceChatLast:
		return class + "_chat_" + string(rune('a'+id-VoiceChatFirst)) + "_1", true
	case id >= VoiceQuestFirst && id <= VoiceQuestLast:
		base, ok := indexOf(class + "_act1_complete_andariel")
		if !ok {
			return "", false
		}

		h := handleAt(base + id - VoiceQuestFirst)

		return h, h != ""
	}

	return "", false
}

// VoiceDelay is the start delay in ticks the exe gives voice id.
func VoiceDelay(id int) int {
	if id < VoiceQuestFirst || id > VoiceQuestLast {
		return 0
	}

	switch id {
	case 0x26, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2e, 0x2f, 0x37, 0x38:
		return 0
	}

	return VoiceQuestDelayTicks
}

// VoiceGate applies the exe's suppression rules to hero voice requests.
type VoiceGate struct {
	lastID   int
	lastTick int64
}

// Allow reports whether the hero may say voice id now. unitSoundPlaying is
// whether a unit sound (a voice) is already playing, dead whether the hero is dead.
func (g *VoiceGate) Allow(id int, now int64, unitSoundPlaying, dead bool) bool {
	if dead {
		return false
	}

	if id >= VoiceChatFirst && id <= VoiceChatLast {
		return !unitSoundPlaying
	}

	if id < VoiceImpossible || id > VoiceNotInTown {
		return true
	}

	if unitSoundPlaying {
		return false
	}

	if id == g.lastID && g.lastTick != 0 && now-g.lastTick <= VoiceRepeatTicks {
		return false
	}

	g.lastID, g.lastTick = id, now

	return true
}
