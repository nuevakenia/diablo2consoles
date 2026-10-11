package d2monsters

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
)

// Scans of the faithful N-Z ports (d2monster.FBXScanner), batch 8. Each filter
// was read from its exe callback (notes: ai-faithful-batch8.md). The callbacks
// stop the exe's unit walk at the first accepted unit only for the Defiler;
// the others keep the best candidate. Iteration is the Director's stable unit
// order (the exe walks its unit hash list, whose order is not reproduced).

// Base classes the scans match (MONSTER_GetBaseClassId of the candidate).
const (
	knightScanClass   = 0x136 // 0x5f9fb0
	overseerScanClass = 0x1c5 // 0x5e1570
	priestAllyA       = 0xeb  // MONAI_ZakarumPriest_AllyFilter
	priestAllyB       = 0xee
)

// scanSide reports whether u is a candidate unit of the caster's side for the
// scans: another monster that is not a hero summon or a mercenary and has the
// caster's alignment (UNIT_AreUnitsHostile false, alignment 0).
func (d *Director) scanSide(b *d2monster.Brain, u *unit) bool {
	return u.b != b && u.merc == nil && u.ally == nil && u.b.Allied == b.Allied && !u.mirror
}

// inClassFamily reports whether the unit belongs to the base class family.
func (d *Director) inClassFamily(u *unit, class int) bool {
	k := d.baseKeyOfClass(class)

	return k != "" && strings.EqualFold(u.m.Stat.BaseKey, k)
}

// livingMode is the exe's "not dead or dying" (modes 0 and 0xc excluded).
func livingMode(u *unit) bool {
	return u.m.Alive() && u.b.Mode != d2monster.ModeDying && u.b.Mode != d2monster.ModeDead
}

func targetOf(u *unit) d2monster.Target {
	return d2monster.Target{ID: u.b.ID, X: u.b.X, Y: u.b.Y, Size: u.b.Size}
}

// scanDefiler is SRVDO_Impregnate_IsValidTarget + edge distance <= 25
// (0x5eeb80): the first living monster, not class 0x222 / 0x227, not in state
// 0x6e, not converted, within 25 of the Putrid Defiler.
func (d *Director) scanDefiler(b *d2monster.Brain) d2monster.FBXScanResult {
	for _, u := range d.sortedUnits() {
		if u.b == b || u.merc != nil || u.ally != nil || u.mirror || !livingMode(u) {
			continue
		}

		if d.inClassFamily(u, 0x222) || d.inClassFamily(u, 0x227) || d.unitHasState(u.b, 0x6e) {
			continue
		}

		if d2monster.EdgeDistance(b.X-u.b.X, b.Y-u.b.Y, b.Size) <= 25 {
			return d2monster.FBXScanResult{Found: true, T: targetOf(u)}
		}
	}

	return d2monster.FBXScanResult{}
}

// scanKnightAlly is the Oblivion Knight's callback 0x5f9fb0: the nearest other
// living monster of base class 0x136 of the same side, within squared distance
// 2500. The callback also tracks the nearest such unit under 40 percent life,
// which the think function does not read.
func (d *Director) scanKnightAlly(b *d2monster.Brain) d2monster.FBXScanResult {
	best := 2500 + 1

	var res d2monster.FBXScanResult

	for _, u := range d.sortedUnits() {
		if !d.scanSide(b, u) || !livingMode(u) || !d.inClassFamily(u, knightScanClass) {
			continue
		}

		dx, dy := b.X-u.b.X, b.Y-u.b.Y
		if dd := dx*dx + dy*dy; dd < best {
			best, res = dd, d2monster.FBXScanResult{Found: true, T: targetOf(u)}
		}
	}

	return res
}

// scanOverseer is callback 0x5e1570: friendly living monsters of base class
// 0x1c5 not in state 0x8d. Count is how many pass; the first one below
// LifeBelow percent life within squared distance AllyRadius2 is T ("ally"), the
// first within squared distance EnemyRadius2 is T2 ("enemy", the exe also skips
// units whose monster data flag 8 is set; that flag is not decoded).
func (d *Director) scanOverseer(b *d2monster.Brain, q d2monster.FBXScanQuery) d2monster.FBXScanResult {
	var res d2monster.FBXScanResult

	for _, u := range d.sortedUnits() {
		if !d.scanSide(b, u) || !livingMode(u) || !d.inClassFamily(u, overseerScanClass) || d.unitHasState(u.b, 0x8d) {
			continue
		}

		res.Count++

		dx, dy := b.X-u.b.X, b.Y-u.b.Y
		dd := dx*dx + dy*dy

		if !res.Found && dd <= q.AllyRadius2 && u.b.HPPercent < q.LifeBelow {
			res.Found, res.T = true, targetOf(u)
		}

		if !res.Found2 && dd <= q.EnemyRadius2 {
			res.Found2, res.T2 = true, targetOf(u)
		}
	}

	return res
}

// scanPriestAlly is MONAI_ZakarumPriest_AllyFilter (0x5f62c0): living same-side
// monsters of base class 0xeb or 0xee within the squared radius and below 61
// percent life, counted. The exe never stores the best percent it compares
// against (the word stays 0x7fffffff), so every qualifying unit overwrites the
// result: the LAST one visited wins, not the weakest.
func (d *Director) scanPriestAlly(b *d2monster.Brain, radius2 int) d2monster.FBXScanResult {
	var res d2monster.FBXScanResult

	for _, u := range d.sortedUnits() {
		if !d.scanSide(b, u) || !livingMode(u) || !(d.inClassFamily(u, priestAllyA) || d.inClassFamily(u, priestAllyB)) {
			continue
		}

		dx, dy := b.X-u.b.X, b.Y-u.b.Y
		if dx*dx+dy*dy > radius2 || u.b.HPPercent >= 61 {
			continue
		}

		res.Count++
		res.Found, res.T = true, targetOf(u)
	}

	return res
}

// siegeTargetClass is the base class the siege weapons' scan accepts (0x5e05a0 compares MONSTER_GetBaseClassId
// with 0x1ec; the same code falls back to -1 when the table is shorter, which no monster matches).
const siegeTargetClass = 0x1ec

// scanSiege is the callback 0x5e05a0 of the Siege Tower and the Siege Beast (VERIFIED, batch 9): the FIRST unit of
// the walk that is another monster, not dead or dying, of base class 0x1ec, on the caster's side (the state/AC
// relation test: both alignment 0), of alignment 0, without an owner (not a hero summon), whose AI has no target
// yet (AiGeneral +0x14 is -1) and that lies within the squared radius. The callback returns the unit, which ends
// the walk, so this is the first match in walk order and not the nearest.
func (d *Director) scanSiege(b *d2monster.Brain, q d2monster.FBXScanQuery) d2monster.FBXScanResult {
	if b.Allied {
		return d2monster.FBXScanResult{}
	}

	for _, u := range d.sortedUnits() {
		if u.b == b || u.merc != nil || u.ally != nil || u.mirror || u.b.Allied || !livingMode(u) || u.attackTarget != 0 {
			continue
		}

		if !d.inClassFamily(u, siegeTargetClass) {
			continue
		}

		dx, dy := b.X-u.b.X, b.Y-u.b.Y
		if dx*dx+dy*dy <= q.Radius2 {
			return d2monster.FBXScanResult{Found: true, T: targetOf(u)}
		}
	}

	return d2monster.FBXScanResult{}
}

// scanRegurgitate is MONAI_Regurgitator_TargetFilter (0x5f9820, VERIFIED, batch 9): every other monster that is
// dead or dying, not in the state mask 0x150, of alignment 0 and whose monstats2 row has bit 0xc (the "soft"
// column, read from the table loader 0x658ee0) is a candidate; the unit flag tests (bit 1 set, bit 9 clear) are
// not decoded and not modelled. The callback never stops the walk: it keeps the candidate with the smallest
// squared distance, and a candidate at the same distance as the best so far replaces it (the test is <=), so the
// LAST of equally distant corpses wins. The block starts with the squared radius as its best distance.
func (d *Director) scanRegurgitate(b *d2monster.Brain, q d2monster.FBXScanQuery) d2monster.FBXScanResult {
	best := q.Radius2

	var res d2monster.FBXScanResult

	for _, u := range d.sortedUnits() {
		if u.b == b || u.merc != nil || u.ally != nil || u.mirror || u.b.Allied || u.m.Alive() || u.raising {
			continue
		}

		if u.m.StatEx == nil || !u.m.StatEx.IsSoft {
			continue
		}

		dx, dy := b.X-u.b.X, b.Y-u.b.Y
		if dd := dx*dx + dy*dy; dd <= best {
			best, res = dd, d2monster.FBXScanResult{Found: true, T: targetOf(u)}
		}
	}

	return res
}

// FBXSiegeUpdate implements d2monster.FBXSiegeTargetStore, MONAI_KeepNearerTargetIdInAiData (0x5e0650, VERIFIED
// batch 9): the weapon keeps the id of the unit stored in its AI data unless that unit still exists and is
// strictly nearer than the new candidate; otherwise the candidate's id replaces it.
func (d *Director) FBXSiegeUpdate(b *d2monster.Brain, t d2monster.Target) {
	u := d.unitOf(b)
	if u == nil {
		return
	}

	if cur := d.units[u.siegeTarget]; cur != nil && u.siegeTarget != 0 {
		cx, cy := b.X-cur.b.X, b.Y-cur.b.Y
		nx, ny := b.X-t.X, b.Y-t.Y

		if cx*cx+cy*cy < nx*nx+ny*ny {
			return
		}
	}

	u.siegeTarget = t.ID
}
