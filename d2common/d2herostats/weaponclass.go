package d2herostats

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2animspeed"

// Per weapon class animation tables (hit recovery, block recovery, attack speed).
//
// The exe picks the AnimData record of an action from token + mode + weapon class of the equipped weapon
// (COMPOSIT_ResolveWeaponClassCode 0x650480, VERIFIED). A shield is never consulted for the class, so a Paladin
// with a shield blocks with the record of her weapon. The frames and speeds below come from the expansion
// animdata.d2 (VERIFIED, d2animspeed.PlayerRecord); most classes have the same hit and block numbers for every
// weapon class, the exceptions are listed by TestWeaponClassExceptions.

// WeaponEntry is one derived table for a class, kind and weapon class.
type WeaponEntry struct {
	Class  string
	Kind   Kind
	WClass string // weapon class code: HTH, 1HS, 1HT, 2HS, 2HT, STF, BOW, XBW, 1JS, 1JT, 1SS, 1ST, HT1, HT2
	Mode   string // AnimData mode: GH, BL, A1 or A2
	Anim   Anim
	Start  int // first frame the attack animation plays from (IAS only)
	Table  []int
}

func modeFor(k Kind, attack2 bool) string {
	switch k {
	case FHR:
		return "GH"
	case FBR:
		return "BL"
	case FCR:
		return "SC"
	}

	if attack2 {
		return "A2"
	}

	return "A1"
}

// WeaponTable derives the table of a class, kind and weapon class (WSM 0, no skill speed for IAS). mode selects
// the attack animation (A1 or A2) for IAS and is ignored for the other kinds. ok is false when the class has no
// such record (it cannot use that weapon class).
func WeaponTable(class string, k Kind, wclass, mode string) (WeaponEntry, bool) {
	tok, ok := d2animspeed.ClassTokens[class]
	if !ok {
		return WeaponEntry{}, false
	}

	if k != IAS {
		mode = modeFor(k, false)
	}

	m, ok := d2animspeed.PlayerRecord(tok, mode, wclass)
	if !ok {
		return WeaponEntry{}, false
	}

	e := WeaponEntry{Class: class, Kind: k, WClass: wclass, Mode: mode, Anim: Anim{tok + mode + wclass, m.Frames, m.Speed}}

	if k == IAS {
		e.Start = d2animspeed.AttackStartFrame(class, wclass)
		e.Table = d2animspeed.AttackBreakpoints(m, e.Start, d2animspeed.AttackStat44(0, 0))
	} else {
		e.Table = DeriveBreakpoints(k, m.Frames, m.Speed)
	}

	return e, true
}

// IASTable is the attack speed breakpoint list for a weapon: wsm is the weapon's speed column (negative for fast
// weapons) and skill the speed term of the skill (Zeal, Fury, Dragon Talon...; UNVERIFIED how each skill sets it).
func IASTable(class, wclass, mode string, wsm, skill int) ([]int, bool) {
	tok, ok := d2animspeed.ClassTokens[class]
	if !ok {
		return nil, false
	}

	m, ok := d2animspeed.PlayerRecord(tok, mode, wclass)
	if !ok {
		return nil, false
	}

	return d2animspeed.AttackBreakpoints(m, d2animspeed.AttackStartFrame(class, wclass), d2animspeed.AttackStat44(wsm, skill)), true
}

// AttackGameFrames is the length of an attack in game frames (the mode end event of the exe's scheduler) for a
// weapon with the given speed modifier, a skill speed term and the item IAS stat.
func AttackGameFrames(class, wclass, mode string, wsm, skill, ias int) (int, bool) {
	tok, ok := d2animspeed.ClassTokens[class]
	if !ok {
		return 0, false
	}

	m, ok := d2animspeed.PlayerRecord(tok, mode, wclass)
	if !ok {
		return 0, false
	}

	return d2animspeed.AttackTicks(m, d2animspeed.AttackStartFrame(class, wclass), d2animspeed.AttackStat44(wsm, skill), ias), true
}

// WeaponTables lists every weapon class table of a class and kind (A1 attacks for IAS), in the exe's weapon class
// order, skipping the classes the hero has no record for.
func WeaponTables(class string, k Kind) []WeaponEntry {
	var out []WeaponEntry

	for _, w := range d2animspeed.WeaponClasses {
		if e, ok := WeaponTable(class, k, w, "A1"); ok {
			out = append(out, e)
		}
	}

	return out
}
