package d2skill

// BoneArmorAbsorb is the absorb event of Bone Armor (auraeventfunc 22, the table slot at 0x72fda0 points to
// 0x5c4300, no function is defined there in Ghidra; read from the disassembly, VERIFIED). It runs for each hit the
// armor's owner takes, with the damage struct of the hit:
//   - it needs the owner's aurastate list and a positive value of the armor's maximum stat (aurastat2);
//   - only the PHYSICAL part of the hit (damage struct +8, 8.8 fixed point) is touched: the pool (aurastat1, same
//     8.8 unit) first soaks it; a pool larger than the physical damage keeps the remainder and the physical part
//     becomes 0, a pool not larger is used up and the physical part loses exactly the pool;
//   - the new pool is stored back; at 0 the armor state is removed (toggled off, stat list freed); otherwise a
//     display percent MulDiv(pool, 100, max) is compared with the stat 0x54 on the list and, when it moved by 5 or
//     more, the state overlay (skill column at record +0x80) is re-added and the stat is rewritten (cosmetic).
//
// Nothing here depends on the attacker, on the other damage types (fire, cold, lightning, poison, magic pass
// untouched) or on a per-orbiter count: the "6 per orbiter" of the earlier note is the client display only
// (UNVERIFIED, the orbiter count is not read by this function). Returns the physical damage left, the pool left,
// whether the armor ends and the display percent.
func BoneArmorAbsorb(physical, pool, poolMax int) (physLeft, poolLeft int, ended bool, displayPct int) {
	if poolMax <= 0 || pool <= 0 {
		// a missing/empty pool: the exe returns without touching the hit when the max stat is not positive; with a
		// zero pool it only removes the armor
		return physical, 0, poolMax > 0, 0
	}

	if pool > physical {
		poolLeft, physLeft = pool-physical, 0
	} else {
		poolLeft, physLeft = 0, physical-pool
	}

	if poolLeft == 0 {
		return physLeft, 0, true, 0
	}

	return physLeft, poolLeft, false, int(int64(poolLeft) * 100 / int64(poolMax))
}
