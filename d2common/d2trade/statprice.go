package d2trade

// Port of the four price add-on routines of TRADE_CalcItemPrice (Game.exe
// 1.14b): TRADE_AddStatPropertyPrices (0x629010), TRADE_AddSingleSkillPrice
// (0x62eef0), TRADE_AddSocketedItemPrices (0x6294f0) and
// TRADE_CalcRechargeCost (0x628ed0). All arithmetic is 32 bit; the sums are
// divided with an UNSIGNED division by the stack divisor, as in the original.
// Notes: d2-re-notes/w23-economy.md.

const (
	// Skill parameter packing of chance-to-cast and charged-skill stats:
	// skill id = param >> ParamShift, level = param & ParamMask. The exe reads
	// the shift from a table-derived global (0x9648dc, set by the ItemStatCost
	// loader from a Save Param Bits column, 6 when that is outside 1..8) and
	// the mask from the word after the table header. UNVERIFIED that the mask
	// is (1<<shift)-1; the shipped values are 6 and 0x3f.
	ParamShift = 6
	ParamMask  = 0x3f

	// Stat ids the add-ons read.
	StatSingleSkill  = 0x6b // item_singleskill
	StatChargedSkill = 0xcc // item_charged_skill

	hugeProduct = 0x10000
)

// StatCost is the price part of an ItemStatCost row.
type StatCost struct {
	Encode   int // exe field bEncode (the Encode column)
	ValShift int
	Mult     int // "Multiply" column
	Add      int // "Add" column
}

// SkillCost is the price part of a skills.txt row.
type SkillCost struct {
	Mult     int // "cost mult"
	Add      int // "cost add"
	ReqLevel int
}

// StatEntry is one stat of the item's stat list: the stored value (already
// shifted left by the stat's ValShift, as the game keeps it) of the stat id
// and parameter. Value is the bonus the item's property lists add (full minus
// base list value).
type StatEntry struct {
	ID    int
	Param int
	Value int
}

// Tables are the data tables the add-ons read.
type Tables struct {
	Stats  map[int]StatCost
	Skills map[int]SkillCost
}

func (t *Tables) skill(id int) (SkillCost, bool) {
	if t == nil || id < 0 {
		return SkillCost{}, false
	}

	s, ok := t.Skills[id]

	return s, ok
}

// div4096 is the sell-side division of the skill terms: (x + (x<0 ? 4095 : 0)) >> 12.
func div4096(x int) int {
	x32 := int32(x)

	return int((x32 + (x32>>31)&0xfff) >> 12)
}

// unsignedShare is sum / d with the original's unsigned division.
func unsignedShare(sum, d int) int {
	if d == 0 {
		return 0
	}

	return int(int32(uint32(int32(sum)) / uint32(int32(d))))
}

// scaledTerm is mult*x*k/1024 (shift 10) or /4096 (shift 12) plus add, or with the
// division first when big.
func scaledTerm(x, k, mult, add int, big bool, shift12 bool) int {
	div := div1024
	if shift12 {
		div = div4096
	}

	if big {
		return w(w(div(w(x*k))*mult) + add)
	}

	return w(div(w(w(mult*x)*k)) + add)
}

// StatPropertyDelta is TRADE_AddStatPropertyPrices: the amounts added to the
// buy, sell and repair accumulators (a, b, c) by the item's stat list, with
// the stack divisor d. Entries with the same id and parameter must already be
// merged.
func StatPropertyDelta(t *Tables, entries []StatEntry, a, b, c, d int) (da, db, dc int) {
	var sa, sb, sc int

	for _, e := range entries {
		v := e.Value
		if v == 0 || t == nil {
			continue
		}

		sc0, ok := t.Stats[e.ID]
		if !ok {
			continue
		}

		if sc0.ValShift != 0 {
			v >>= uint(sc0.ValShift & 0x1f)
		}

		var ea, eb, ec int

		switch sc0.Encode {
		case 1:
			s, ok := t.skill(e.Param)
			if !ok {
				continue
			}

			big := w(a*v) >= hugeProduct && s.Mult != 0
			ea, eb, ec = scaledTerm(a, v, s.Mult, s.Add, big, false), scaledTerm(b, v, s.Mult, s.Add, big, true), scaledTerm(c, v, s.Mult, s.Add, big, false)
		case 2, 3:
			s, ok := t.skill(e.Param >> ParamShift)
			if !ok {
				continue // the original dereferences a null row here
			}

			lvl := e.Param & ParamMask
			big := w(a*lvl) > 0xffff && s.Mult != 0
			ea, eb, ec = scaledTerm(a, lvl, s.Mult, s.Add, big, false), scaledTerm(b, lvl, s.Mult, s.Add, big, true), scaledTerm(c, lvl, s.Mult, s.Add, big, false)
		case 4:
			// packed min/max (the "by time" stats): the mean of the two halves
			hi := (v >> 12 & 0x3ff) - 0x100
			lo := (v >> 2 & 0x3ff) - 0x100
			avg := (hi + lo) / 2
			big := w(a*avg) >= hugeProduct && sc0.Mult != 0
			ea, eb, ec = scaledTerm(a, avg, sc0.Mult, sc0.Add, big, false), scaledTerm(b, avg, sc0.Mult, sc0.Add, big, false), scaledTerm(c, avg, sc0.Mult, sc0.Add, big, false)
		default:
			big := w(a*v) >= hugeProduct && sc0.Mult != 0
			ea, eb, ec = scaledTerm(a, v, sc0.Mult, sc0.Add, big, false), scaledTerm(b, v, sc0.Mult, sc0.Add, big, false), scaledTerm(c, v, sc0.Mult, sc0.Add, big, false)
		}

		sa, sb, sc = w(sa+ea), w(sb+eb), w(sc+ec)
	}

	return unsignedShare(sa, d), unsignedShare(sb, d), unsignedShare(sc, d)
}

// SingleSkillDelta is TRADE_AddSingleSkillPrice. skills are the entries of
// stat 0x6b (Param = skill id, Value = level); the caller skips it for item
// types without staff mods (ItemTypes StaffMods "none"). Quirk kept: the sell
// side divides by 4096 where the other two divide by 1024.
func SingleSkillDelta(t *Tables, skills []StatEntry, a, b, c, d int) (da, db, dc int) {
	var sa, sb, sc int

	for _, e := range skills {
		s, ok := t.skill(e.Param)
		if !ok {
			continue
		}

		weight := w(e.Value*2 - 1)

		if w(a) < hugeProduct || s.Mult == 0 {
			sa = w(sa + w(w(div1024(w(s.Mult*a))+s.Add)*weight))
			sb = w(sb + w(w(div4096(w(b*s.Mult))+s.Add)*weight))
			sc = w(sc + w(w(div1024(w(c*s.Mult))+s.Add)*weight))
		} else {
			m10, m12 := div1024(s.Mult), div4096(s.Mult)
			sa = w(sa + w(w(m10*a+s.Add)*weight))
			sb = w(sb + w(w(m12*b+s.Add)*weight))
			sc = w(sc + w(w(c*m10+s.Add)*weight))
		}
	}

	return unsignedShare(sa, d), unsignedShare(sb, d), unsignedShare(sc, d)
}

// SocketedDelta is TRADE_AddSocketedItemPrices: half of each socketed item's
// base cost, added to all three accumulators.
func SocketedDelta(costs []int) int {
	sum := 0

	for _, c := range costs {
		sum += c / 2
	}

	return sum
}

// rechargeScale is the constant the caller passes to TRADE_CalcRechargeCost
// (0x2710 at 0x62fe89).
const rechargeScale = 10000

// RechargeCost is TRADE_CalcRechargeCost: for every charged skill entry
// (stat 0xcc, Param = skill<<6|level, Value = current | max<<8) the missing
// share of the charges times a per-charge price.
func RechargeCost(t *Tables, charged []StatEntry) int {
	total := 0

	for _, e := range charged {
		cur, max := e.Value&0xff, e.Value>>8
		skillID := e.Param >> ParamShift

		s, ok := t.skill(skillID)
		if !ok || cur >= max || max == 0 {
			continue
		}

		lvl := (e.Param & ParamMask) + 2 + (s.ReqLevel/6)*2

		var per int
		if w(lvl*rechargeScale) < hugeProduct || s.Mult == 0 {
			per = div1024(w(w(s.Mult*lvl) * rechargeScale))
		} else {
			per = w(div1024(w(lvl*rechargeScale)) * s.Mult)
		}

		total = w(total + w(w((max-cur)*(per+s.Add))/max))
	}

	return total
}
