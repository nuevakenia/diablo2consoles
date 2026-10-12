package d2inventory

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// Potion effects, from the misc.txt columns pSpell, len, stat1/calc1 and
// stat2/calc2 (the item tables, not the game's code). What the columns mean is
// read from the shipped rows, not verified in the binary:
//
//   - pSpell 3 (healing and mana potions): stat1 is hpregen / manarecovery and
//     calc1 the total amount (Greater Healing 320 life, Greater Mana 250 mana),
//     restored evenly over len frames (25 frames per second: 256 frames about
//     10 s). The original ticks the amount once per frame batch; an even
//     spread over the same time is the approximation used here.
//   - pSpell 5 (rejuvenation): stat1 hitpoints and stat2 mana with calc1 and
//     calc2 as a PERCENT of the maximum, applied at once (35 and 100).
//
// Other pSpell values (antidote, thawing, stamina, scrolls) have no effect here.

const potionFramesPerSecond = 25.0

const (
	spellOverTime = 3
	spellRejuv    = 5
)

// PotionEffect is what drinking a potion does.
type PotionEffect struct {
	// Instant amounts, in percent of the maximum.
	InstantHPPercent, InstantManaPercent int
	// Over time, in points, spread across Seconds.
	HP, Mana float64
	Seconds  float64
}

// IsEmpty reports whether the potion does nothing here.
func (e PotionEffect) IsEmpty() bool {
	return e.InstantHPPercent == 0 && e.InstantManaPercent == 0 && e.HP == 0 && e.Mana == 0
}

// PotionEffectOf reads the effect of a misc.txt potion.
func PotionEffectOf(rec *d2records.ItemCommonRecord) PotionEffect {
	var e PotionEffect

	if rec == nil || !rec.Useable {
		return e
	}

	switch rec.SpellType {
	case spellOverTime:
		e.Seconds = float64(rec.EffectLength) / potionFramesPerSecond

		for _, u := range rec.UsageStats {
			amount := calcNumber(string(u.Calc))

			switch strings.ToLower(u.Stat) {
			case "hpregen":
				e.HP += float64(amount)
			case "manarecovery":
				e.Mana += float64(amount)
			}
		}
	case spellRejuv:
		for _, u := range rec.UsageStats {
			amount := calcNumber(string(u.Calc))

			switch strings.ToLower(u.Stat) {
			case "hitpoints":
				e.InstantHPPercent += amount
			case "mana":
				e.InstantManaPercent += amount
			}
		}
	}

	return e
}

// calcNumber reads a calc column that holds a plain number (all potion rows).
func calcNumber(s string) int {
	n := 0

	for _, r := range strings.TrimSpace(s) {
		if r < '0' || r > '9' {
			return 0
		}

		n = n*10 + int(r-'0')
	}

	return n
}

// Regen is the over-time part of the potions the hero drank. Each pool (life,
// mana) keeps the points still to come, the seconds still to run and the rate.
// Drinking again while one works follows ITEM_ServerUsePotionVariantA 0x5bbf70
// (VERIFIED): the state's end moves out by the new length, and the per-frame
// value becomes (oldRate*remaining + newAmount) / (remaining + newLength), so the
// points left and the new ones are spread over the summed time and none is lost.
type Regen struct {
	hp, mana regenPool
}

type regenPool struct {
	left, secs, rate float64
}

func (p *regenPool) add(amount, seconds float64) {
	if amount <= 0 {
		return
	}

	rest := p.secs
	if rest < 0 || p.left <= 0 {
		rest = 0
	}

	p.left += amount
	p.secs = rest + seconds
	p.rate = p.left / p.secs
}

func (p *regenPool) tick(dt float64) float64 {
	if p.left <= 0 {
		return 0
	}

	p.secs -= dt

	taken := p.rate * dt
	if taken > p.left || p.secs <= 0 {
		taken = p.left
	}

	p.left -= taken
	if p.left <= 0 {
		*p = regenPool{}
	}

	return taken
}

// Add starts the over-time part of an effect.
func (r *Regen) Add(e PotionEffect) {
	if e.Seconds <= 0 || (e.HP == 0 && e.Mana == 0) {
		return
	}

	r.hp.add(e.HP, e.Seconds)
	r.mana.add(e.Mana, e.Seconds)
}

// Active reports whether anything is left to restore.
func (r *Regen) Active() bool { return r.hp.left > 0 || r.mana.left > 0 }

// Tick advances dt seconds and returns the points restored in that time.
func (r *Regen) Tick(dt float64) (hp, mana float64) {
	return r.hp.tick(dt), r.mana.tick(dt)
}

// ApplyInstant applies the instant part of an effect to a pool and returns the
// new value (clamped to max).
func ApplyInstant(current, max, percent int) int {
	v := current + max*percent/100
	if v > max {
		return max
	}

	return v
}
