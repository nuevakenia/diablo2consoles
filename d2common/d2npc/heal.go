package d2npc

// Healers are the NPCs that restore the hero when an interaction starts (NPCSRV_RestoreVitalsOnTalk 0x576c80,
// called from NPCSRV_BeginInteractionForPlayer 0x570cc0 when the hero's target is that NPC): Akara 0x94, Malah
// 0x201, Atma 0xb0, Fara 0xb2, Jamella 0x195 and Ormus 0xff. (monster-ai.md doubted Atma and Fara; the class
// list in the code is as written here.)
//
//nolint:gochecknoglobals // static exe data
var healers = map[int]bool{
	ClassAkara: true, ClassMalah: true, ClassAtma: true, ClassFara: true, ClassJamella: true, ClassOrmus: true,
}

// IsHealer reports whether an NPC class restores the hero on interaction.
func IsHealer(class int) bool { return healers[class] }

// Vitals are the three pools a healer fills.
type Vitals struct {
	Life, MaxLife       int
	Mana, MaxMana       int
	Stamina, MaxStamina float64
}

// Restore fills life, mana and stamina up to their maximum (each only when below it; the exe never lowers a
// pool). changed tells whether any pool moved. The same call also removes the poison and freeze states, the
// states of the "curable" column and heals the hero's pets to full life (not part of Vitals).
func Restore(v Vitals) (out Vitals, changed bool) {
	out = v

	if out.Life < out.MaxLife {
		out.Life, changed = out.MaxLife, true
	}

	if out.Mana < out.MaxMana {
		out.Mana, changed = out.MaxMana, true
	}

	if out.Stamina < out.MaxStamina {
		out.Stamina, changed = out.MaxStamina, true
	}

	return out, changed
}

// HealingCost is UNIT_GetHealingCost (0x622fc0), the price of the "Heal" row that the exe's menu code can show
// (string 0xd09) but no static table row carries, so the row never appears in 1.14b. Stats are 8.8 fixed point:
// cost = level * ((maxLife - life) >> 8 + (maxMana - mana) >> 8) >> 2, 0 when both pools are full.
func HealingCost(level int, life, maxLife, mana, maxMana uint32) int {
	var lost uint32

	if life < maxLife {
		lost += (maxLife - life) >> 8
	}

	if mana < maxMana {
		lost += (maxMana - mana) >> 8
	}

	if lost == 0 {
		return 0
	}

	return level * int(lost) >> 2
}
