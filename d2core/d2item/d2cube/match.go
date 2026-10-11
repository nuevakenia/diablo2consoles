package d2cube

import (
	"strings"
	"time"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2item/d2drop"
)

// Item is an item in the cube (or the result of a recipe). It mirrors what the
// hero file keeps of an item, so the game converts to and from it directly.
type Item struct {
	Code     string
	Quality  d2drop.Quality // 0 is treated as normal
	ILvl     int
	Seed     int64
	Unique   string
	SetItem  string
	Set      string
	Prefixes []string
	Suffixes []string
	Ethereal bool
	Quantity int // stack size, 0 = default

	Sockets  int      // number of sockets
	Socketed []string // codes of the gems, runes and jewels in them
	Runeword string   // name of the runeword the item became

	// Mods are the properties a recipe attached (crafted mods, extra sockets
	// are not here but in Sockets).
	Mods []RolledMod

	// Damaged is set by recipes that repair: the game sets full durability.
	Repaired   bool
	Recharged  bool
	Identified bool
	Crafted    bool

	// FileIndex is the unique/set row index (-1 or 0 when none); used by the
	// recipe input condition op 27.
	FileIndex int
	// QuestDifficulty is the difficulty a quest item was found in (stat
	// 0x164), checked by the quest recipes (op 28).
	QuestDifficulty int
	// Stat returns a stat of the item for the per-item recipe conditions
	// (ops 15 to 26); nil means no stats are known.
	Stat StatFunc
}

// StatKind selects which value of a stat a recipe condition reads.
type StatKind int

// Stat kinds: the total (STATS_GetUnitStat), the base value
// (STATS_GetUnitBaseStat) and the bonus (STATS_GetUnitStatBonus).
const (
	StatTotal StatKind = iota
	StatBase
	StatBonus
)

// StatFunc reads a stat. shift is ItemStatCost valshift: the exe compares the
// stat with value >> shift. known is false for a stat without a row, which
// fails the condition.
type StatFunc func(kind StatKind, id int) (value int, shift uint, known bool)

// RolledMod is a recipe property with its value rolled.
type RolledMod struct {
	Code  string
	Param string
	Min   int
	Max   int
	Value int // rolled between Min and Max
}

func (i *Item) quality() d2drop.Quality {
	if i.Quality == 0 {
		return d2drop.QualityNormal
	}

	return i.Quality
}

// Context is the state of the game a recipe is checked against.
type Context struct {
	PlayerLevel int
	Difficulty  int    // 0 normal, 1 nightmare, 2 hell
	Ladder      bool   // ladder / realm game
	Expansion   bool   // Lord of Destruction
	Class       string // class code: ama sor nec pal bar dru ass

	// Now is the date the day conditions (ops 1 and 2) read; zero means
	// time.Now().
	Now time.Time
	// Stat reads a hero stat for the player conditions (ops 3 to 14).
	Stat StatFunc
}

// Match is a recipe together with the cube items that satisfy its inputs.
type Match struct {
	Recipe *Recipe
	// Assigned[k] lists the cube item indexes that fill Recipe.Inputs[k].
	Assigned [][]int
}

func (c *Context) allows(r *Recipe) bool {
	switch {
	case !r.Enabled:
		return false
	case r.Ladder && !c.Ladder:
		return false
	case c.Difficulty < r.MinDiff:
		return false
	case r.Version >= 100 && !c.Expansion:
		return false
	}

	if len(r.Class) > 0 {
		ok := false

		for _, k := range r.Class {
			if k == c.Class {
				ok = true
			}
		}

		if !ok {
			return false
		}
	}

	return c.condition(r)
}

// condition evaluates the recipe's op/param/value the way CUBE_ServerTransmute
// (0x5642d0) does: op 1 passes on a day of month in [param, value], op 2 on
// the weekday (1 = Sunday) value, ops 3 to 14 compare a hero stat (four
// comparisons per kind: at least, at most, differs, equals; kinds total, base,
// bonus). Other ops (15 to 28 belong to the inputs, 0 and unknown ones to
// nobody) pass here. A stat without a row fails. The comparison names for ops
// 5/6 follow the decompile; none of ops 1 to 14 is used by the shipped table.
func (c *Context) condition(r *Recipe) bool {
	switch {
	case r.Op == 1:
		d := c.Now
		if d.IsZero() {
			d = time.Now()
		}

		return d.Day() >= r.Param && d.Day() <= r.Value
	case r.Op == 2:
		d := c.Now
		if d.IsZero() {
			d = time.Now()
		}

		return int(d.Weekday())+1 == r.Value
	case r.Op >= 3 && r.Op <= 14:
		return statCondition(c.Stat, r.Op-3, r.Param, r.Value)
	}

	return true
}

// statCondition checks one stat condition; idx is 0..11: kind = idx/4, compare
// = idx%4 (0 at least, 1 at most, 2 differs, 3 equals). A nil reader passes
// (the caller has no stats), an unknown stat fails.
func statCondition(fn StatFunc, idx, id, value int) bool {
	if fn == nil {
		return true
	}

	v, shift, known := fn(StatKind(idx/4), id)
	if !known {
		return false
	}

	want := value >> shift

	switch idx % 4 {
	case 0:
		return v >= want
	case 1:
		return v <= want
	case 2:
		return v != want
	}

	return v == want
}

// IsIngredient reports whether an item is a recipe ingredient rather than the
// item a recipe works on: gems, runes, jewels, scrolls, potions, books.
func (cat *Catalog) IsIngredient(it *Item) bool {
	b, ok := cat.bases[it.Code]
	if !ok {
		return false
	}

	for _, t := range []string{"sock", "scro", "poti", "book", "rpot", "key"} {
		if b.HasType(t) {
			return true
		}
	}

	return false
}

// Matches reports whether one cube item satisfies an input (ignoring qty).
func (cat *Catalog) Matches(in *Input, it *Item) bool {
	b, ok := cat.bases[it.Code]
	if !ok {
		return false
	}

	switch {
	case in.Any:
	case in.Code != "":
		if it.Code != in.Code && !(in.Upgrade && sameOrHigherTier(in.Code, b)) {
			return false
		}
	case in.Type != "":
		if !b.HasType(in.Type) {
			return false
		}
	case in.Unique != "":
		if it.quality() != d2drop.QualityUnique || !uniqueNameIs(it.Unique, in.Unique) {
			return false
		}
	}

	if in.Quality != 0 && it.quality() != in.Quality {
		return false
	}

	if (in.Ethereal == yes3 && !it.Ethereal) || (in.Ethereal == no3 && it.Ethereal) {
		return false
	}

	if (in.Socketed == yes3 && it.Sockets == 0) || (in.Socketed == no3 && it.Sockets > 0) {
		return false
	}

	if in.Tier != TierNone && b.Tier != in.Tier {
		return false
	}

	// upg (0x5636e0 region): the item may be the input's base item or a higher
	// tier of the same family (normal, exceptional, elite).
	if in.Upgrade && in.Code != "" && !sameOrHigherTier(in.Code, b) {
		return false
	}

	if in.NoRuneword && it.Runeword != "" {
		return false
	}

	return true
}

// sameOrHigherTier reports whether the item base b is the base item code or a
// higher tier of its family: normal item code only equals; exceptional also
// accepts the normal one; elite accepts all three.
func sameOrHigherTier(code string, b *Base) bool {
	switch {
	case b.Code == code:
		return true
	case b.Code == b.Norm:
		return false
	case b.Code == b.Uber:
		return code == b.Norm
	case b.Code == b.Ultra:
		return code == b.Norm || code == b.Uber
	}

	return false
}

// matchInput adds the recipe-level input rules to Matches: the quest
// difficulty check of op 28 and, for the first input, the item conditions of
// ops 15 to 27.
func (cat *Catalog) matchInput(ctx *Context, r *Recipe, slot int, in *Input, it *Item) bool {
	if !cat.Matches(in, it) {
		return false
	}

	if r.Op == 28 {
		if b := cat.bases[it.Code]; b != nil && b.Quest && b.QuestDiffCheck && it.QuestDifficulty < ctx.Difficulty {
			return false
		}
	}

	if slot == 0 {
		switch {
		case r.Op >= 15 && r.Op <= 26:
			if !statCondition(it.Stat, r.Op-15, r.Param, r.Value) {
				return false
			}
		case r.Op == 27:
			// The exe rejects an item whose unique/set row equals value.
			if it.FileIndex == r.Value {
				return false
			}
		}
	}

	return true
}

func uniqueNameIs(have, want string) bool {
	norm := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		return strings.TrimPrefix(s, "the ")
	}

	return have != "" && norm(have) == norm(want)
}

// Find returns the first recipe of the table (in table order, like the game)
// whose inputs are exactly the items in the cube, or nil.
func (cat *Catalog) Find(tab *Table, ctx *Context, items []Item) *Match {
	if len(items) == 0 {
		return nil
	}

	for _, r := range tab.Recipes {
		if !ctx.allows(r) {
			continue
		}

		if m := cat.assign(ctx, r, items); m != nil {
			return m
		}
	}

	return nil
}

// assign searches an assignment of cube items to the inputs such that every
// cube item is used exactly once.
func (cat *Catalog) assign(ctx *Context, r *Recipe, items []Item) *Match {
	// The exe requires the number of items in the cube to equal the recipe's
	// numinputs column (CUBE_ServerTransmute 0x5642d0); fall back to the sum of
	// the input counts when the column is blank.
	need := r.NumInputs
	if need == 0 {
		for k := range r.Inputs {
			need += cat.itemCount(&r.Inputs[k])
		}
	}

	if need != len(items) {
		return nil
	}

	used := make([]bool, len(items))
	asg := make([][]int, len(r.Inputs))

	var solve func(k int) bool

	solve = func(k int) bool {
		if k == len(r.Inputs) {
			return true
		}

		in := &r.Inputs[k]
		want := cat.itemCount(in)

		var pick func(start, got int) bool

		pick = func(start, got int) bool {
			if got == want {
				return solve(k + 1)
			}

			for i := start; i < len(items); i++ {
				if used[i] || !cat.matchInput(ctx, r, k, in, &items[i]) || !cat.quantityOK(in, &items[i]) {
					continue
				}

				used[i] = true
				asg[k] = append(asg[k], i)

				if pick(i+1, got+1) {
					return true
				}

				asg[k] = asg[k][:len(asg[k])-1]
				used[i] = false
			}

			return false
		}

		return pick(0, 0)
	}

	if !solve(0) {
		return nil
	}

	return &Match{Recipe: r, Assigned: asg}
}

// itemCount is how many cube items an input takes. The exe counts item units
// only: a stackable input asks for qty separate stacks (2 arrows -> bolts
// needs two quivers in the cube), the stack quantity is never read
// (CUBE_CheckRecipeInputSlot 0x562e40).
func (cat *Catalog) itemCount(in *Input) int { return in.Qty }

func (cat *Catalog) quantityOK(in *Input, it *Item) bool { return true }
