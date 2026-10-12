package d2animspeed

import "strings"

// Class tokens of the seven heroes and the two Druid forms (COF/AnimData record prefixes).
const (
	TokenWerewolf = "40" // monstats Wolf (id 430), the gfxclass of state 139 "wolf"
	TokenWerebear = "TG" // monstats Bear (id 431), the gfxclass of state 140 "bear"
)

// ClassTokens maps the class name to its token.
var ClassTokens = map[string]string{
	"Amazon": "AM", "Sorceress": "SO", "Necromancer": "NE", "Paladin": "PA",
	"Barbarian": "BA", "Druid": "DZ", "Assassin": "AI",
}

// WeaponClasses lists the weapon class codes of the COF names in the order of the exe (hand to hand first).
var WeaponClasses = []string{"HTH", "1HS", "1HT", "2HS", "2HT", "STF", "BOW", "XBW", "1JS", "1JT", "1SS", "1ST", "HT1", "HT2"}

// PlayerRecord returns the AnimData record of token + mode + weapon class (e.g. "PA", "BL", "1HS"), the key the
// exe's COF lookup uses. mode is one of A1 A2 GH BL SC TH KK (RN WL NU for the Druid forms). ok is false when
// the record does not exist (a class cannot play the mode with that weapon class).
func PlayerRecord(token, mode, wclass string) (Mode, bool) {
	m, ok := playerRecords[strings.ToUpper(token+mode+wclass)]

	return m, ok
}

// Shapeshift-adjusted modes: while a form state is active the exe plays the monster record of the form
// (UNIT_GetShapeshiftAdjustedMode 0x624830 region, VERIFIED): the unit is looked up as a monster of the state's
// gfxclass and the hit / block / attack rules of the monster modes (3, 6, 4/5) run with the same formulas as
// the player ones, on the form's record.

// AttackStartFrame is UNIT_GetAttackFrameCount (0x623d00, VERIFIED): the frame the attack animation (modes A1
// and A2) starts at. Only the Amazon and the Sorceress have non-zero entries: hand to hand 1, the five melee
// classes (1HS 1HT STF 2HS 2HT) 2, bows and crossbows 0.
func AttackStartFrame(class, wclass string) int {
	if class != "Amazon" && class != "Sorceress" {
		return 0
	}

	switch strings.ToUpper(wclass) {
	case "HTH", "":
		return 1
	case "1HS", "1HT", "STF", "2HS", "2HT":
		return 2
	}

	return 0
}

// AttackTicks is the length in game frames of an attack animation as the exe schedules its mode end
// (EVENT_SchedulePathStepEvents 0x551a20, VERIFIED): the animation starts at frame start, adds the 8.8 rate
// every game frame and ends the first frame the position reaches frames<<8; the end event comes at that frame
// (at least 2). stat44 is the attack percent stat before IAS (100 - weapon speed modifier + skill speed), ias
// the item stat 93 (diminished here).
func AttackTicks(m Mode, start, stat44, ias int) int {
	rate := AttackRate(m.Speed, stat44, ias, 0, false)
	if rate <= 0 {
		return 1
	}

	n := (m.Frames<<8 - start<<8 + rate - 1) / rate
	if n < 2 {
		n = 2
	}

	return n
}

// AttackBreakpoints lists the smallest IAS values at which the attack gets one game frame shorter, for a given
// stat44 (100 - WSM + skill bonus). It starts with 0.
func AttackBreakpoints(m Mode, start, stat44 int) []int {
	return Breakpoints(func(ias int) int { return AttackTicks(m, start, stat44, ias) }, MaxTableStat+1, MaxTableStat)
}

// AttackStat44 is the stat 0x44 a hero has before IAS: 100 (PLAYER_InitBaseStatsFromClass 0x56e5d0, VERIFIED)
// minus the weapon speed modifier (weapons.txt speed, a weapon's own stat 0x44 is -speed, ITEMGEN_InitItemBaseStats
// 0x555b10, VERIFIED) plus the skill's speed term (a list of stat 0x44 from the skill, SKILL_ApplyAttackRateStatList
// 0x56c2e0 / CLTST_AttachSpeedStatList 0x4c46b0, VERIFIED for Bash and Dragon Tail; other skills use the same
// stat through their own passes, UNVERIFIED).
func AttackStat44(wsm, skillBonus int) int { return AttackBasePct - wsm + skillBonus }
