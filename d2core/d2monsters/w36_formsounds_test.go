package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

func TestFormAttackSounds(t *testing.T) {
	// the shape of the werewolf row: vocal 85 percent, kick weapon sound always
	r := &d2records.MonsterSoundRecord{
		Attack1: "druidwerewolf_attack_1", Attack1Delay: 5, Attack1Probability: 85,
		Weapon1: "weapon_kick_1", Weapon1Delay: 5, Weapon1Volume: 150,
		HitSound: "druidwerewolf_hit_1",
	}

	got := FormAttackSounds(r, d2monster.ModeAttack1, func(int) int { return 0 })
	if len(got) != 2 || got[0].Handle != "druidwerewolf_attack_1" || got[1].Handle != "weapon_kick_1" || got[1].Volume != 150 {
		t.Fatalf("attack sounds %+v", got)
	}

	got = FormAttackSounds(r, d2monster.ModeAttack1, func(int) int { return 99 }) // vocal roll fails
	if len(got) != 1 || got[0].Handle != "weapon_kick_1" {
		t.Fatalf("vocal roll failed, want only the weapon sound: %+v", got)
	}

	if h := FormHitSound(r); len(h) != 1 || h[0].Handle != "druidwerewolf_hit_1" {
		t.Fatalf("hit sound %+v", h)
	}

	if len(FormHitSound(nil)) != 0 {
		t.Fatal("no record, no hit sound")
	}
}
