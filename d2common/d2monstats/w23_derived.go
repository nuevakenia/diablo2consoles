package d2monstats

// Derived monster stats set by MONAI_InitMonsterStatsFromMonstats (0x00571af0), wave 23.

// MaxBlock is the cap of COMBAT_GetBlockChance (0x006228d0, VERIFIED 0x4b).
const MaxBlock = 75

// InnateBlockClasses are the monster classes that block although their monstats NoShldBlock flag is clear
// (MONSTER_HasInnateBlock 0x006227a0, VERIFIED: 0xf3, 0x136, 0x14d). The same function also lets a monster block
// that carries a shield in its off-hand component; that part is UNVERIFIED and not modelled.
var InnateBlockClasses = map[int]bool{0xf3: true, 0x136: true, 0x14d: true}

// MonsterBlock is the block chance a monster has against a landed hit (COMBAT_GetBlockChance, monster branch):
// stat 0x14 (the ToBlock column of the difficulty, copied at 0x00571d26) when the class has the NoShldBlock flag
// (monstats flag bit 26) or an innate block, else 0; capped at 75.
func MonsterBlock(class int, noShldBlock bool, toBlock int) int {
	if !noShldBlock && !InnateBlockClasses[class] {
		return 0
	}

	if toBlock > MaxBlock {
		return MaxBlock
	}

	if toBlock < 0 {
		return 0
	}

	return toBlock
}

// RegenStat is stat 0x4a (hpregen) of a fresh monster (0x00571f0c..): DamageRegen (monstats +0x1a0, must be
// 0..0xfff) times the maximum life in 1/256 points, shifted right by 12. For large lives (more than
// 0x7fffffff / DamageRegen) the exe shifts first, which loses the low bits. The monster gains this many 1/256
// life points per 25 Hz frame (MONSTER_LifeRegenEventTick 0x005a4340).
func RegenStat(maxLife256, damageRegen int) int {
	if damageRegen <= 0 || damageRegen >= 0x1000 {
		return 0
	}

	if 0x7fffffff/damageRegen < maxLife256 {
		return (maxLife256 >> 12) * damageRegen
	}

	return (damageRegen * maxLife256) >> 12
}

// SkillLevel is the level a monster skill is added to the unit with (0x00571fa0..): the monstats SkillLevel plus
// the DifficultyLevels MonsterSkillBonus of the game difficulty, only for a skill whose level column is above 0.
func SkillLevel(columnLevel, difficultyBonus int) int {
	if columnLevel <= 0 {
		return columnLevel
	}

	return columnLevel + difficultyBonus
}
