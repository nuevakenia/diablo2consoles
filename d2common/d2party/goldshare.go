package d2party

import "sort"

// Purse is one player's gold and carry limit (InventoryGoldLimit of the level).
type Purse struct{ Have, Cap int }

// GoldShare is the gold one player receives from a pickup.
type GoldShare struct {
	ID  string
	Add int
}

// GoldSplit is the result of a gold pickup: the gold each player receives
// (sorted by id, the picker included) and the part that fits nowhere, which
// the game spills on the ground as standard gold piles.
type GoldSplit struct {
	Shares   []GoldShare
	Overflow int
}

func room(p Purse, amount int) (given, over int) {
	if amount <= 0 {
		return 0, 0
	}

	if p.Have+amount > p.Cap {
		given = p.Cap - p.Have
		if given < 0 {
			given = 0
		}

		return given, amount - given
	}

	return amount, 0
}

// ShareGold is the gold distribution of a pickup, ITEMMODE_PickupGoldFromGround
// 0x55a5f0 -> PARTY_DistributeGoldPickup 0x53e640 (VERIFIED from the disassembly):
//
//   - gold linked to a PLAYER owner (a pile a hero dropped, unit flag-ex bit 10
//     with a linked type 0) goes to the picker alone, clamped to the carry limit
//     (playerDropped);
//   - a picker with no party, or with fewer than 2 counted members, takes the
//     gold alone (clamped, the rest is Overflow);
//   - otherwise n = the party members in the picker's level for which 0x552230 is
//     false (taken as "not a body", the same predicate the experience split uses;
//     UNVERIFIED meaning), picker included; every OTHER member in the picker's
//     level receives amount/n (clamped to the member's own carry limit, the
//     part that does not fit comes back to the picker), the picker keeps the
//     rest (remainder of the division included) clamped to its limit. The
//     distribution only runs when the picker itself is alive (unit mode neither
//     0 nor 0x11); a dead member is still given a share (only the count skips
//     it). There is NO distance test for gold, unlike experience.
//
// purses must hold every party member that may take part; a member without a
// purse is skipped. Ownership and loot timers do not exist for ground items in
// this pickup path (any player may take any mode-3 item).
func (r *Roster) ShareGold(picker string, amount int, purses map[string]Purse, playerDropped bool) GoldSplit {
	var out GoldSplit

	if amount <= 0 {
		return out
	}

	pk, ok := r.players[picker]
	solo := func() GoldSplit {
		given, over := room(purses[picker], amount)

		return GoldSplit{Shares: []GoldShare{{ID: picker, Add: given}}, Overflow: over}
	}

	if !ok || playerDropped || pk.party == 0 {
		return solo()
	}

	var others []*entry

	n := 0

	for _, o := range r.players {
		if o.party != pk.party || o.Area != pk.Area {
			continue
		}

		if !o.body {
			n++
		}

		if o != pk {
			if _, has := purses[o.ID]; has {
				others = append(others, o)
			}
		}
	}

	if n < 2 || pk.body {
		return solo()
	}

	sort.Slice(others, func(i, j int) bool { return others[i].ID < others[j].ID })

	share := amount / n
	keep := amount

	for _, o := range others {
		given, over := room(purses[o.ID], share)
		out.Shares = append(out.Shares, GoldShare{ID: o.ID, Add: given})
		keep += over - share
	}

	given, over := room(purses[picker], keep)
	out.Shares = append(out.Shares, GoldShare{ID: picker, Add: given})
	out.Overflow = over

	sort.Slice(out.Shares, func(i, j int) bool { return out.Shares[i].ID < out.Shares[j].ID })

	return out
}
