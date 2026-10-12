package d2cube

import (
	"fmt"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2item/d2drop"
)

// Product is one result of a transmute.
type Product struct {
	Item Item

	// Source is the cube item index a "useitem" result was made from, -1 for a
	// new item.
	Source int

	// Roll says the game must roll the item's identity for Item.Quality: the
	// magic/rare/set/unique affixes (or the crafted ones) of Item.Code at
	// Item.ILvl, seeded with Item.Seed. Forced affixes (Item.Prefixes and
	// Suffixes, set from pre=/suf=) replace the rolled ones of their kind.
	Roll bool

	// CraftAffixes is, for crafted items, how many random magic affixes the
	// rolled item keeps besides the recipe's mods.
	CraftAffixes int

	// Returned are the codes of the items taken out of the sockets of a
	// "rem" result; the game puts them in the inventory.
	Returned []string

	// Portal is set for the named portal results ("Cow Portal", ...): no item
	// is created, the game opens that portal.
	Portal string
}

// Result is the outcome of a transmute.
type Result struct {
	Recipe *Recipe
	// Consumed are the cube item indexes that are used up. A "useitem"
	// product replaces its Source, which is consumed too.
	Consumed []int
	Products []Product
}

// OutputLevel is the item level of a recipe result, as CUBE_ServerCreateRecipeOutputs
// (0x5638a0) computes it: a fixed lvl wins; otherwise plvl percent of the hero
// level plus ilvl percent of the first input's item level (each product
// truncated by 100). The result is at least 1 and at most the highest hero
// level (99). With all three blank the level is 1 (not the input's level).
func OutputLevel(o *Output, playerLevel, mainILvl int) int {
	lvl := o.Level

	if lvl == 0 {
		lvl = o.PLevel * playerLevel / 100 //nolint:gomnd // percent
		lvl += o.ILevel * mainILvl / 100   //nolint:gomnd
	}

	switch {
	case lvl < 1:
		return 1
	case lvl > 99: //nolint:gomnd // the highest hero level
		return 99
	}

	return lvl
}

// CraftedAffixCount is the number of random magic affixes of a crafted item
// (ITEMGEN_RollCraftedAffixes, notes itemgen.md 1.7): at least 1, 2 above item
// level 30, 3 above 50, 4 above 70; the roll rand(5) can raise it.
func CraftedAffixCount(rng d2drop.RNG, ilvl int) int {
	n := 1

	switch {
	case ilvl > 70: //nolint:gomnd // thresholds from the binary
		n = 4
	case ilvl > 50: //nolint:gomnd
		n = 3
	case ilvl > 30: //nolint:gomnd
		n = 2
	}

	if r := int(rng.Roll(5)); r > n { //nolint:gomnd
		n = r
	}

	return n
}

// MainItem returns the cube item a "useitem" or "usetype" result is based on:
// the item matched by the recipe's first input (the last one when it takes
// several), which is what the exe records in the output slots while checking
// input 0 (CUBE_CheckRecipeInputSlot 0x562e40).
func (cat *Catalog) MainItem(m *Match, items []Item) int {
	if len(m.Assigned) > 0 && len(m.Assigned[0]) > 0 {
		l := m.Assigned[0]
		return l[len(l)-1]
	}

	for _, list := range m.Assigned {
		if len(list) > 0 {
			return list[0]
		}
	}

	return -1
}

// Execute runs a matched recipe.
func (cat *Catalog) Execute(m *Match, ctx *Context, items []Item, rng d2drop.RNG) (*Result, error) {
	res := &Result{Recipe: m.Recipe}

	for _, list := range m.Assigned {
		res.Consumed = append(res.Consumed, list...)
	}

	main := cat.MainItem(m, items)
	if main < 0 {
		return nil, fmt.Errorf("recipe %d has no inputs", m.Recipe.Row)
	}

	for k := range m.Recipe.Outputs {
		p, err := cat.makeProduct(&m.Recipe.Outputs[k], ctx, items, main, rng)
		if err != nil {
			return nil, fmt.Errorf("recipe %d %q output %d: %w", m.Recipe.Row, m.Recipe.Description, k+1, err)
		}

		res.Products = append(res.Products, p)
	}

	return res, nil
}

func newSeed(rng d2drop.RNG) int64 { return int64(rng.Roll(1<<30)) + 1 }

func (cat *Catalog) makeProduct(o *Output, ctx *Context, items []Item, main int, rng d2drop.RNG) (Product, error) {
	src := &items[main]
	p := Product{Source: -1}

	if o.Special != "" {
		p.Portal = o.Special
		return p, nil
	}

	ilvl := OutputLevel(o, ctx.PlayerLevel, src.ILvl)

	switch {
	case o.UseItem:
		p.Item = *src
		p.Source = main

		// The exe clones the input and only swaps its base (0x5638a0): lvl,
		// plvl and ilvl are not read for a "useitem" result, the item keeps
		// its own item level. No shipped recipe sets them on one.
		if o.Remove {
			p.Returned = append(p.Returned, src.Socketed...)
		}

		if err := cat.modifyInPlace(o, &p.Item); err != nil {
			return p, err
		}
	case o.UseType:
		p.Item = Item{Code: src.Code, ILvl: ilvl, Seed: newSeed(rng), Quality: o.Quality}
		if o.Regrade {
			// "reg": the quality, the item level and the unique/set row come
			// from the input (the exe reads them off the item).
			p.Item.Quality, p.Item.ILvl = src.Quality, src.ILvl
			p.Item.Unique, p.Item.SetItem, p.Item.Set = src.Unique, src.SetItem, src.Set
			ilvl = src.ILvl
		}
	case o.Code != "":
		p.Item = Item{Code: o.Code, ILvl: ilvl, Seed: newSeed(rng), Quality: o.Quality}
	case o.Type != "":
		code, ok := cat.PickBase(rng, o.Type, ilvl)
		if !ok {
			return p, fmt.Errorf("no base item of type %q", o.Type)
		}

		p.Item = Item{Code: code, ILvl: ilvl, Seed: newSeed(rng), Quality: o.Quality}
	default:
		return p, fmt.Errorf("output %q names nothing", o.Raw)
	}

	if !o.UseItem {
		p.Item.Ethereal = o.Ethereal
		p.Item.Identified = o.Quality == 0 || o.Quality == d2drop.QualityNormal
		p.Roll = o.Quality >= d2drop.QualityMagic

		if o.Qty > 0 {
			p.Item.Quantity = o.Qty
		}

		cat.forceAffixes(o, &p)

		if o.Quality == d2drop.QualityCrafted {
			p.CraftAffixes = CraftedAffixCount(rng, ilvl)
			p.Item.Crafted = true
		}
	}

	cat.applyMods(o, &p.Item, rng)

	if o.Sockets > 0 && p.Item.Sockets == 0 {
		p.Item.Sockets = RecipeSockets(p.Item.quality(), minInt(o.Sockets, cat.MaxSockets(p.Item.Code, p.Item.ILvl)))
	}

	return p, nil
}

// modifyInPlace applies a "useitem" result's flags to a copy of the input.
func (cat *Catalog) modifyInPlace(o *Output, it *Item) error {
	if o.Tier != TierNone && o.Upgrade {
		code, ok := cat.Upgrade(it.Code, o.Tier)
		if !ok {
			return fmt.Errorf("%s has no tier %d version", it.Code, o.Tier)
		}

		it.Code = code
	}

	// A clone keeps its quality; only a crafted token changes it (the exe
	// then gives the clone a rare prefix and suffix name, 0x5638a0). A
	// "useitem" with any other quality token is not in the shipped table.
	if o.Quality == d2drop.QualityCrafted {
		it.Quality = o.Quality
	}

	if o.Repair {
		it.Repaired = true
	}

	if o.Recharge {
		it.Recharged = true
	}

	if o.Unsocket || o.Remove {
		it.Socketed = nil
		it.Runeword = ""
	}

	if o.Ethereal {
		it.Ethereal = true
	}

	// qty= on a stackable result sets the stack, limited by maxstack (the
	// exe does this for repair results: arrows and throwing weapons refill).
	if b := cat.bases[it.Code]; o.Qty > 0 && b != nil && b.Stackable {
		it.Quantity = o.Qty
		if b.MaxStack > 0 && it.Quantity > b.MaxStack {
			it.Quantity = b.MaxStack
		}
	}

	return nil
}

// forceAffixes resolves pre=N and suf=N.
func (cat *Catalog) forceAffixes(o *Output, p *Product) {
	for _, id := range o.Prefix {
		if name, ok := cat.AffixName(true, id); ok {
			p.Item.Prefixes = append(p.Item.Prefixes, name)
		}
	}

	for _, id := range o.Suffix {
		if name, ok := cat.AffixName(false, id); ok {
			p.Item.Suffixes = append(p.Item.Suffixes, name)
		}
	}
}

// applyMods rolls the property lines of a result. The pseudo property "sock"
// sets the number of sockets (capped by what the base item can hold); the
// others are kept for the game to turn into item properties.
func (cat *Catalog) applyMods(o *Output, it *Item, rng d2drop.RNG) {
	for _, m := range o.Mods {
		// Exe: a chance of 0 or 100 and above always applies; otherwise the
		// property is skipped when chance < roll%100, so it applies with
		// (chance+1)/100 (0x5638a0, seed % 100).
		if m.Chance > 0 && m.Chance < 100 && m.Chance < int(rng.Roll(100)) {
			continue
		}

		v := m.Min
		if m.Max > m.Min {
			v = m.Min + int(rng.Roll(int32(m.Max-m.Min+1)))
		}

		if m.Code == "sock" {
			it.Sockets = minInt(v, cat.MaxSockets(it.Code, it.ILvl))
			continue
		}

		it.Mods = append(it.Mods, RolledMod{Code: m.Code, Param: m.Param, Min: m.Min, Max: m.Max, Value: v})
	}
}

// RecipeSockets limits the sockets a recipe result gets by its quality, as
// CUBE_ServerCreateRecipeOutputs (0x5638a0) does after the base limit: magic
// and tempered items keep up to 3 (4 or more become 3), set, rare, unique and
// crafted items get at most 1.
func RecipeSockets(q d2drop.Quality, n int) int {
	switch q {
	case d2drop.QualityMagic, QualityTempered:
		if n >= 4 { //nolint:gomnd
			return 3
		}
	case d2drop.QualitySet, d2drop.QualityRare, d2drop.QualityUnique, d2drop.QualityCrafted:
		if n > 1 {
			return 1
		}
	}

	return n
}

func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}
