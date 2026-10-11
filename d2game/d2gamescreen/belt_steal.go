package d2gamescreen

import (
	"math/rand"
	"os"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2game/d2player"
)

// The belt theft of Game.exe 0x5a0be0 (d2steal): a hit that connects on a
// player may take the first potion of its belt, which lands on the ground
// next to the player. The exe hangs it on a monster hit callback (which
// monster is unverified); here a hit of a hostile player triggers it, on the
// victim's own client, which owns the belt.

// beltThief is the part of the hero's controls the theft needs.
type beltThief interface {
	StealBeltItem(step func() uint32) (d2player.InventoryItem, bool)
}

// stealFromBelt runs the theft and hands the stolen item's code to drop.
func stealFromBelt(b beltThief, step func() uint32, drop func(code string)) bool {
	it, ok := b.StealBeltItem(step)
	if !ok {
		return false
	}

	drop(strings.TrimSpace(it.GetItemCode()))

	return true
}

// beltStealStep is the roll of the theft: the generator of the attacker. A
// test-only env (OD2_BELTSTEAL_ROLL=<n>) fixes it.
func beltStealStep() func() uint32 {
	if s := os.Getenv("OD2_BELTSTEAL_ROLL"); s != "" {
		n := uint32(0)
		for _, c := range s {
			n = n*10 + uint32(c-'0')
		}

		return func() uint32 { return n }
	}

	return rand.Uint32 // nolint:gosec // a game roll, not security
}

// pvpBeltSteal is called for every PvP hit that connected on the local hero.
func (v *Game) pvpBeltSteal(attacker string) {
	if v.gameControls == nil {
		return
	}

	stealFromBelt(v.gameControls, beltStealStep(), func(code string) {
		v.Infof("BELT STEAL by=%q code=%s dropped to the ground", attacker, code)
		v.debugSpawnItemAtPlayer(code)
	})
}

// commandGiveBelt is "givebelt <code>".
func (v *Game) commandGiveBelt(args []string) error {
	cell, err := v.gameControls.GiveBeltItem(args[0])
	if err != nil {
		return err
	}

	v.Infof("GIVEBELT code=%s cell=%d", args[0], cell)

	return nil
}

// commandBeltList is "beltlist": the belt codes in the log.
func (v *Game) commandBeltList(_ []string) error {
	var parts []string

	for i, c := range v.gameControls.BeltCodes() {
		if c != "" {
			parts = append(parts, strings.TrimSpace(c)+"@"+string(rune('a'+i)))
		}
	}

	v.Infof("BELT contents n=%d [%s]", len(parts), strings.Join(parts, " "))

	return nil
}
