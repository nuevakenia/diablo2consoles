package d2difficulty

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"

// Progression: the d2s header keeps, in the status word bits 8..12 (byte
// 0x25), how far the character got. VERIFIED (Game.exe CHARINFO_RaiseProgression 0x5360d0, called by the boss quests
// with the act number, e.g. 0x58bb13 for Baal): progression = 5 * difficulty + act in the expansion (4 * difficulty
// + act in classic), kept only if it is not lower than the stored value; the quest kills raise it (Andariel 1,
// Duriel 2, Mephisto 3, Diablo 4, Baal 5). VERIFIED (SAVE_CheckCharFlagsAgainstGameMode 0x567a40): Nightmare needs
// a progression of at least 5 (classic 4), Hell at least 10 (classic 8); the errors are 0xd and 0xe. The
// character level plays no part in the gate.

// Progression reads the progression out of the header status word.
func Progression(status uint32) int { return int(status>>8) & 0x1f }

// WithProgression returns the status word with the progression replaced.
func WithProgression(status uint32, p int) uint32 {
	if p < 0 {
		p = 0
	}

	if p > 0x1f {
		p = 0x1f
	}

	return status&^(0x1f<<8) | uint32(p)<<8
}

// RaiseProgression is CHARINFO_RaiseProgression (VERIFIED 0x5360d0): the progression becomes 5 * difficulty + act
// (4 * difficulty + act in classic) unless the stored one is higher.
func RaiseProgression(stored, act, difficulty int, expansion bool) int {
	if v := ActsPerDifficulty(expansion)*difficulty + act; stored <= v {
		return v
	}

	return stored
}

// GateProgression is the minimum progression the menus want for a difficulty (VERIFIED 0x567a40): Normal none,
// Nightmare one difficulty of acts (5, classic 4), Hell two (10, classic 8).
func GateProgression(l Level, expansion bool) int {
	if l <= Normal {
		return 0
	}

	return ActsPerDifficulty(expansion) * int(Clamp(int(l)))
}

// ActsPerDifficulty is 5 in the expansion, 4 in classic.
func ActsPerDifficulty(expansion bool) int {
	if expansion {
		return 5
	}

	return 4
}

// bossQuest is the (act, quest) whose completion is the end of an act: Sisters
// to the Slaughter (Andariel), The Seven Tombs (Duriel), The Guardian
// (Mephisto), Terror's End (Diablo), Eve of Destruction (Baal). The "Terror's
// End" quest is Diablo, it ends classic Normal; in the expansion the last act
// boss is Baal (A5Q6).
var bossQuest = [d2s.NumActs][2]int{{1, 6}, {2, 6}, {3, 6}, {4, 2}, {5, 6}}

// ActBossDefeated reports whether the end-of-act quest of act (1..5) is
// complete or has its reward pending in a difficulty's quest record
// (UNVERIFIED that the reward-pending state already counts).
func ActBossDefeated(q *d2s.QuestRecord, act int) bool {
	if q == nil || act < 1 || act > d2s.NumActs {
		return false
	}

	a, n := bossQuest[act-1][0], bossQuest[act-1][1]

	return q.Completed(a, n) || q.RewardPending(a, n)
}

// ActsDefeated counts the acts whose boss is defeated, in order: counting stops
// at the first act that is not. In the expansion Diablo's death does NOT raise
// the progression (VERIFIED: QUEST_A4_TerrorsEnd_CompleteQuest 0x5b2950 calls
// CHARINFO_RaiseProgression only when the game is classic; D2MOO A4Q2 agrees),
// so after Mephisto the next raise is Baal's (act 5) and the count jumps from
// 3 to 5.
func ActsDefeated(q *d2s.QuestRecord, expansion bool) int {
	n := 0

	for act := 1; act <= ActsPerDifficulty(expansion); act++ {
		if expansion && act == 4 {
			if n == 3 && ActBossDefeated(q, 5) {
				return 5
			}

			return n
		}

		if !ActBossDefeated(q, act) {
			break
		}

		n++
	}

	return n
}

// ProgressionOf computes the progression byte of a hero from its three quest
// records: the finished difficulties count fully, the first unfinished one by
// the acts done.
func ProgressionOf(quests [3]d2s.QuestRecord, expansion bool) int {
	per := ActsPerDifficulty(expansion)
	total := 0

	for d := 0; d < int(Count); d++ {
		n := ActsDefeated(&quests[d], expansion)
		total += n

		if n < per {
			break
		}
	}

	return total
}

// Finished reports whether a difficulty has been completed: its last act boss
// is defeated in that difficulty's quest record, or the stored progression
// already reaches the end of it.
func Finished(l Level, quests [3]d2s.QuestRecord, progression int, expansion bool) bool {
	l = Clamp(int(l))
	per := ActsPerDifficulty(expansion)

	if progression >= per*(int(l)+1) {
		return true
	}

	return ActsDefeated(&quests[l], expansion) >= per
}

// Unlocked lists whether each difficulty may be started: Normal always,
// Nightmare after Normal is finished, Hell after Nightmare is finished.
func Unlocked(quests [3]d2s.QuestRecord, progression int, expansion bool) [3]bool {
	u := [3]bool{true, false, false}
	u[Nightmare] = Finished(Normal, quests, progression, expansion)
	u[Hell] = u[Nightmare] && Finished(Nightmare, quests, progression, expansion)

	return u
}

// Allowed reports whether one difficulty may be started.
func Allowed(l Level, quests [3]d2s.QuestRecord, progression int, expansion bool) bool {
	return Unlocked(quests, progression, expansion)[Clamp(int(l))]
}

// Highest returns the highest unlocked difficulty.
func Highest(u [3]bool) Level {
	for l := Hell; l > Normal; l-- {
		if u[l] {
			return l
		}
	}

	return Normal
}
