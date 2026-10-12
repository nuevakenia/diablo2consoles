package d2monsters

import (
	"sort"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// Skill book of the Assassin's shadows (w25-shadows, notes d2-re-notes/w25-shadows.md).
//
// The exe gives a shadow a real skill list (units have a skill node list): the Pre-hook of the Shadow Master copies
// every class skill of the owner's class (MONAI_Pre_ShadowMaster 0x5ea4f0), the Shadow Warrior learns the owner's
// right and left skill at every think (0x5e9ff0). The Go side keeps that list in shadowBook, scores it with the
// ported d2monster.ShadowScore / ShadowPick, and casts through the skill engine exactly as a hireling does
// (MercSkillHooks.Cast): the shadow plays the monster animation, and at its hit frame the engine runs the skill with
// the shadow as caster.
//
// Deviations (UNVERIFIED, all documented in the notes):
//   - only skills the engine can run (MercSkillHooks.Supported) are in the scored list;
//   - the shadow has no mana pool in the exe either (the skill's mana cost feeds the fatigue counter), so none is
//     modelled; what the engine charges its hireling caster (always enough) is not an exe rule;
//   - the owner's own target (ShadowTargeting.OwnerTarget) is not known to the Director, so the shadow never adopts it;
//   - STATE_UnitHasStateInMaskDC (the "cast mask") is not decoded and reads as false.

// shadowEntry is one skill of a shadow: the skills.txt row, the scoring view and the facts the copy rules read.
type shadowEntry struct {
	rec                      *d2records.SkillRecord
	view                     d2monster.ShadowSkill
	mode                     d2monster.Mode
	melee                    bool // SKILL_GetAttackAnimTypeForSkill == 1: needs reach (UNVERIFIED: anim A1 or KK)
	class                    int  // SKILL_GetSkillClass: the charclass index, 7 for none
	petType                  int
	summons, summonsSelf     bool
	mana, lvlMana, manaShift int
}

type shadowPending struct {
	e        *shadowEntry
	targetID uint32
	self     bool
}

// shadowBook is the skill list of one shadow (insertion order, like the exe's node list).
type shadowBook struct {
	entries []*shadowEntry
	byID    map[int]*shadowEntry
	nm      *shadowNames
	leftID  int
	leftSet bool
	pending *shadowPending
	petType int
}

// shadowNames gives the state, stat and pet type names of skills.txt small ids (1 and up) for the scoring, which
// works on integers; one table per Director, so an id means the same for every shadow.
type shadowNames struct {
	names []string
	idx   map[string]int
}

func newShadowBook(nm *shadowNames) *shadowBook {
	return &shadowBook{byID: map[int]*shadowEntry{}, nm: nm, leftID: -1, petType: -1}
}

// shadowNameTable is the Director's table, created on first use.
func (d *Director) shadowNameTable() *shadowNames {
	if d.shadowNm == nil {
		d.shadowNm = &shadowNames{idx: map[string]int{}}
	}

	return d.shadowNm
}

// intern gives a name an id; an empty name is -1 (the table's "none").
func (sb *shadowBook) intern(name string) int {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return -1
	}

	if i, ok := sb.nm.idx[name]; ok {
		return i
	}

	sb.nm.names = append(sb.nm.names, name)
	sb.nm.idx[name] = len(sb.nm.names)

	return len(sb.nm.names)
}

func (nm *shadowNames) name(i int) string {
	if i < 1 || i > len(nm.names) {
		return ""
	}

	return nm.names[i-1]
}

func (sb *shadowBook) name(i int) string { return sb.nm.name(i) }

// shadowClassIndex is the exe's class number of a skills.txt charclass code (7: none).
func shadowClassIndex(code string) int {
	switch strings.ToLower(code) {
	case "ama":
		return 0
	case "sor":
		return 1
	case "nec":
		return 2
	case "pal":
		return 3
	case "bar":
		return 4
	case "dru":
		return 5
	case "ass":
		return 6
	}

	return 7
}

// shadowEType is the elemtypes.txt row of a skills.txt etype code (the exe's bEType). VERIFIED rows from the game's
// table: none 0, fire 1, ltng 2, mag 3, cold 4, pois 5, life 6, mana 7, stam 8, stun 9, rand 10, burn 11, frze 12.
func shadowEType(code string) int {
	for i, c := range []string{"", "fire", "ltng", "mag", "cold", "pois", "life", "mana", "stam", "stun", "rand", "burn",
		"frze"} {
		if strings.EqualFold(code, c) {
			return i
		}
	}

	return 0
}

func shadowModeOf(rec *d2records.SkillRecord) d2monster.Mode {
	if m, ok := d2monster.ParseMode(rec.Monanim); ok && m != d2monster.ModeNeutral && m != d2monster.ModeDying {
		return m
	}

	switch rec.Anim {
	case d2enum.PlayerAnimationModeCast:
		return d2monster.ModeSpecialCast
	case d2enum.PlayerAnimationModeSequence:
		return d2monster.ModeCast
	case d2enum.PlayerAnimationModeSkill1:
		return d2monster.ModeSkill1
	case d2enum.PlayerAnimationModeSkill2:
		return d2monster.ModeSkill2
	case d2enum.PlayerAnimationModeSkill3:
		return d2monster.ModeSkill3
	case d2enum.PlayerAnimationModeSkill4:
		return d2monster.ModeSkill4
	case d2enum.PlayerAnimationModeAttack2:
		return d2monster.ModeAttack2
	}

	return d2monster.ModeAttack1
}

// shadowOf returns the unit of a shadow brain and its book, creating the book with the plain attack in it.
func (d *Director) shadowOf(b *d2monster.Brain) (*unit, *shadowBook) {
	u := d.unitOf(b)
	if u == nil || u.ally == nil || !isShadowAI(u) {
		return nil, nil
	}

	if u.ally.shadow == nil {
		sb := newShadowBook(d.shadowNameTable())
		sb.petType = sb.intern(strings.ToLower(u.ally.opt.Tag))
		u.ally.shadow = sb

		d.shadowLearn(u, sb, 0, 1)
	}

	return u, u.ally.shadow
}

// shadowLearn is SKILL_AddOrSetSkillLevel on the shadow: the skill joins the list (or changes level).
func (d *Director) shadowLearn(u *unit, sb *shadowBook, id, level int) *shadowEntry {
	if e := sb.byID[id]; e != nil {
		e.view.Level = level

		return e
	}

	var rec *d2records.SkillRecord

	if d.asset != nil && d.asset.Records != nil {
		rec = d.asset.Records.Skill.Details[id]
	} else if id == 0 { // no tables (unit tests): the plain attack only
		rec = &d2records.SkillRecord{ID: 0, Skill: "Attack", Anim: d2enum.PlayerAnimationModeAttack1}
	}

	if rec == nil {
		return nil
	}

	e := &shadowEntry{
		rec: rec, mode: shadowModeOf(rec), class: shadowClassIndex(rec.Charclass), petType: sb.intern(rec.Pettype),
		summons: rec.Summon != "", mana: rec.Mana, lvlMana: rec.Lvlmana, manaShift: rec.Manashift,
		melee: rec.Anim == d2enum.PlayerAnimationModeAttack1 || rec.Anim == d2enum.PlayerAnimationModeKick,
	}

	if u != nil && u.m != nil && u.m.Stat != nil {
		e.summonsSelf = strings.EqualFold(rec.Summon, u.m.Stat.Key)
	}

	bits4 := 0
	if rec.Progressive {
		bits4 = 4
	}

	e.view = d2monster.ShadowSkill{
		ID: id, AIType: rec.Aitype, AIBonus: rec.Aibonus, ReqLevel: rec.Reqlevel, EType: shadowEType(rec.EType),
		AuraState: sb.intern(rec.Aurastate), AuraTargetState: sb.intern(rec.Auratargetstate),
		AuraStat1: sb.intern(rec.Aurastat1), SrvMissile: -1, SrvMissileA: -1, MissileRange: -1,
		Bits4: bits4, DoFunc: rec.Srvdofunc, Level: level,
	}

	if d.asset != nil && d.asset.Records != nil {
		mt := d.asset.Records.MissileTable()

		if sp := mt.ByName(rec.Srvmissile); rec.Srvmissile != "" && sp != nil {
			e.view.SrvMissile = sp.ID
		}

		if sp := mt.ByName(rec.Srvmissilea); rec.Srvmissilea != "" && sp != nil {
			e.view.SrvMissileA, e.view.MissileRange = sp.ID, sp.Range
		}
	}

	sb.entries = append(sb.entries, e)
	sb.byID[id] = e

	return e
}

// shadowOwner is the hero a shadow follows.
func (u *unit) shadowOwner() *d2mapentity.Player {
	if u == nil || u.ally == nil {
		return nil
	}

	return u.ally.owner
}

// ShadowInit implements d2monster.ShadowPre: MONAI_Pre_ShadowWarrior 0x5e9fa0, MONAI_Pre_ShadowMaster 0x5ea4f0 and
// MONAI_Pre_ShadowMasterNoInit 0x5ea620.
func (d *Director) ShadowInit(b *d2monster.Brain, kind d2monster.ShadowKind) {
	u, sb := d.shadowOf(b)
	if u == nil {
		return
	}

	defer func() {
		d.emit("shadow", "SHADOW init name=%s kind=%d skills=%d level=%d", u.m.Label(), kind, len(sb.entries), b.Scratch[2])
	}()

	switch kind {
	case d2monster.ShadowKindMaster:
		b.Scratch = [3]int{0, 0, 1}
	default:
		b.Scratch[2] = 1
	}

	p := u.shadowOwner()
	if p == nil {
		return
	}

	// the owner's level of the skill monstats aip8 (normal column) names (268 Shadow Warrior, 279 Shadow Master),
	// items included (SKILL_GetTotalLevel flag 1)
	skillID := 0
	if raw := b.Profile.AIPRaw[8]; raw != [3]int{} {
		skillID = raw[0]
	} else {
		skillID = b.AIP(8)
	}

	if lvl := ownerSkillTotal(p, skillID); lvl > 0 {
		b.Scratch[2] = lvl
	}

	if kind != d2monster.ShadowKindMaster {
		return
	}

	// copy every allowed class skill of the owner's class, in class order, at ShadowMasterLevel
	code := strings.ToLower(p.Class.GetToken3())
	ids := make([]int, 0, 32)

	for id, rec := range d.asset.Records.Skill.Details {
		if strings.EqualFold(rec.Charclass, code) {
			ids = append(ids, id)
		}
	}

	sort.Ints(ids)

	for _, id := range ids {
		rec := d.asset.Records.Skill.Details[id]
		if d.mercHooks.Supported != nil && !d.mercHooks.Supported(rec.Skill) {
			continue
		}

		e := d.shadowLearn(u, sb, id, 1)
		if e == nil || !d2monster.ShadowCopyAllowed(e.summons, e.summonsSelf, e.petType, sb.petType, true) {
			d.shadowForget(sb, id)

			continue
		}

		e.view.Level = d2monster.ShadowMasterLevel(ownerSkillBase(p, id), b.Scratch[2])
	}

}

func (d *Director) shadowForget(sb *shadowBook, id int) {
	if sb.byID[id] == nil {
		return
	}

	delete(sb.byID, id)

	for i, e := range sb.entries {
		if e.view.ID == id {
			sb.entries = append(sb.entries[:i], sb.entries[i+1:]...)

			return
		}
	}
}

// ownerSkillBase is the owner's own points in a skill (1 when it has none: the exe's default, 0x5ea490).
func ownerSkillBase(p *d2mapentity.Player, id int) int {
	if s := p.Skills[id]; s != nil && s.SkillPoints > 0 {
		return s.SkillPoints
	}

	return 1
}

// ownerSkillTotal is SKILL_GetTotalLevel(owner, skill, 1): points plus the +skills of the equipment, 0 for a skill
// the owner has not.
func ownerSkillTotal(p *d2mapentity.Player, id int) int {
	s := p.Skills[id]
	if s == nil {
		return 0
	}

	return d2hero.EffectiveSkillLevel(p.Stats, p.Class, s)
}

// ---- d2monster.FB1ShadowBook ----

// HasSkills implements d2monster.FB1ShadowBook: the unit owns a skill list (unit+0xa8 != 0).
func (d *Director) HasSkills(b *d2monster.Brain) bool {
	_, sb := d.shadowOf(b)

	return sb != nil && len(sb.entries) > 0
}

// shadowViews are the skills as the scoring reads them, in list order.
func (d *Director) shadowViews(sb *shadowBook) []d2monster.ShadowSkill {
	out := make([]d2monster.ShadowSkill, 0, len(sb.entries))

	for _, e := range sb.entries {
		out = append(out, e.view)
	}

	return out
}

// CastQueued implements d2monster.FB1ShadowBook: MONAI_ShadowMaster_CastChosenSkill 0x5ea920 for a skill of the list.
func (d *Director) CastQueued(b *d2monster.Brain, skillID int, tgt d2monster.Target, inRange bool) bool {
	u, sb := d.shadowOf(b)
	if u == nil {
		return false
	}

	t := tgt

	return d.shadowCast(u, sb, skillID, &t, inRange)
}

// shadowCast queues one skill of the list. A target that is the shadow itself casts at no unit (buffs); a melee kind
// out of reach runs to the target instead (MONAI_RunNoTargetWithFlags). The engine runs the skill at the hit frame
// (shadowFire).
func (d *Director) shadowCast(u *unit, sb *shadowBook, id int, tgt *d2monster.Target, inRange bool) bool {
	e := sb.byID[id]
	if e == nil {
		return false
	}

	b := u.b
	self := tgt == nil || tgt.ID == b.ID
	aim := d2monster.Target{ID: b.ID, X: b.X, Y: b.Y, Size: b.Size}

	if !self {
		tu := d.units[tgt.ID]
		// the exe wants the target flagged (bit 2) and outside town; here: alive and a unit of the world
		if tu == nil || !tu.m.Alive() {
			return false
		}

		aim = *tgt
	}

	if id == 0 {
		return !self && d.shadowPlay(b, d2monster.ModeAttack1, aim)
	}

	if e.melee && !inRange && !self {
		d.moveTo(u, d2monster.Point{X: aim.X, Y: aim.Y}, &d2monster.Target{ID: aim.ID, X: aim.X, Y: aim.Y, Size: aim.Size},
			meleeInRange, true)

		return true
	}

	sb.pending = &shadowPending{e: e, targetID: aim.ID, self: self}

	if d.shadowPlay(b, e.mode, aim) {
		return true
	}

	if e.mode != d2monster.ModeAttack1 && d.shadowPlay(b, d2monster.ModeAttack1, aim) {
		return true
	}

	sb.pending = nil

	return false
}

// shadowPlay starts the animation of a shadow's action (Attack; a test replaces it, the animation needs a composite).
func (d *Director) shadowPlay(b *d2monster.Brain, mode d2monster.Mode, t d2monster.Target) bool {
	if d.shadowAttack != nil {
		return d.shadowAttack(b, mode, t)
	}

	return d.Attack(b, mode, t)
}

// shadowFire runs the pending skill at the animation's hit frame and reports whether it consumed the strike.
func (d *Director) shadowFire(u *unit) bool {
	if u.ally == nil || u.ally.shadow == nil || u.ally.shadow.pending == nil {
		return false
	}

	sb := u.ally.shadow
	p := sb.pending
	sb.pending = nil

	owner := u.shadowOwner()
	if owner == nil || d.mercHooks.Cast == nil {
		return false
	}

	var tm *d2mapentity.Monster

	if !p.self {
		tu := d.units[p.targetID]
		if tu == nil || !tu.m.Alive() {
			return true // the victim is gone: the cast is lost
		}

		tm = tu.m
	}

	x, y := u.m.SubtilePos()
	a1 := u.m.Vitals.A1
	c := MercCast{
		Owner: owner, Token: u.b.ID, Skill: p.e.rec.Skill, SkillLevel: p.e.view.Level, Level: u.m.Vitals.Level, X: x, Y: y,
		Target: tm, AR: a1.ToHit, DmgMin: a1.Min, DmgMax: a1.Max,
	}

	ok := d.mercHooks.Cast(c)

	tname := "-"
	if tm != nil {
		tname = tm.Label()
	}

	d.emit("shadow", "SHADOW cast name=%s skill=%q id=%d level=%d target=%s ok=%v", u.m.Label(), p.e.rec.Skill, p.e.view.ID,
		p.e.view.Level, tname, ok)

	if ok {
		d.Counters.MinionHits++
	}

	return ok
}

// Upkeep implements d2monster.FB1ShadowBook: the buff and left-skill loop of 0x5eab13.
func (d *Director) Upkeep(b *d2monster.Brain, _ *d2monster.Target, inRange bool) bool {
	u, sb := d.shadowOf(b)
	if u == nil {
		return false
	}

	return d2monster.ShadowUpkeep(b, d, d.shadowViews(sb), func(id int) bool {
		return d.shadowCast(u, sb, id, nil, inRange)
	})
}

// Choose implements d2monster.FB1ShadowBook for a host without ShadowEnv; the Director has one, so the think never
// reaches it. Kept as the plain-attack answer.
func (d *Director) Choose(_ *d2monster.Brain, _ d2monster.Target, _ *d2monster.Target, _ bool) (int, bool) {
	return 0, false
}

// Copy implements d2monster.FB1ShadowBook for the Shadow Warrior: the tail of MONAI_Think_ShadowWarrior 0x5e9ff0 after
// the dice. The owner's right and left skills join the shadow's list at ShadowWarriorLevel; the chosen one (the plain
// attack when basic) goes through MONAI_ShadowWarriorCopySkillCast, the other one is the second try, the plain attack
// the last. calc is the cooldown calc of the cast skill (the think adds /3 + 18 and the frame).
func (d *Director) Copy(b *d2monster.Brain, _ d2monster.Target, tgt d2monster.Target, useLeft, basic, inRange bool) (int, bool) {
	u, sb := d.shadowOf(b)
	if u == nil {
		return 0, false
	}

	owner := u.shadowOwner()
	if owner == nil {
		return 0, false
	}

	// the exe needs both selected skills; a hero here may have none selected (no skill bar yet), which counts as the
	// plain attack on that side (UNVERIFIED stand-in, a real hero always has both)
	rID, lID := 0, 0
	if owner.RightSkill != nil {
		rID = owner.RightSkill.ID
	}

	if owner.LeftSkill != nil {
		lID = owner.LeftSkill.ID
	}
	right := d.shadowLearn(u, sb, rID, 1)
	left := d.shadowLearn(u, sb, lID, 1)

	if right == nil || left == nil {
		return 0, false
	}

	right.view.Level = d2monster.ShadowWarriorLevel(b.Scratch[2], ownerSkillTotal(owner, rID))
	left.view.Level = d2monster.ShadowWarriorLevel(b.Scratch[2], ownerSkillTotal(owner, lID))

	chosen := right
	if useLeft {
		chosen = left
	}

	if basic {
		chosen = sb.byID[0]
	}

	ownerClass := shadowClassIndex(strings.ToLower(owner.Class.GetToken3()))

	try := func(e *shadowEntry) bool {
		if e == nil {
			return false
		}

		in := d2monster.ShadowCopyIn{
			Frame: d.frame, OwnerClass: ownerClass, SkillClass: e.class, SkillID: e.view.ID,
			Cost: d2monster.ShadowManaWhole(e.mana, e.lvlMana, e.manaShift, e.view.Level),
		}
		in.Valid = d2monster.ShadowValidateCopy(d.shadowValidateIn(u, sb, e, tgt, inRange))

		return d2monster.ShadowCopyCast(b, in)
	}

	pick := chosen
	if !try(pick) {
		if pick != left {
			pick = left
		} else {
			pick = right
		}

		if !try(pick) {
			pick = sb.byID[0]
		}
	}

	if pick == nil {
		return 0, false
	}

	// the cast: a melee kind out of reach runs to the target (the think does that when Copy declines)
	if pick.melee && !inRange {
		return 0, false
	}

	var queued bool

	if pick.view.ID == 0 {
		queued = d.shadowPlay(b, d2monster.ModeAttack1, tgt)
	} else {
		t := tgt
		queued = d.shadowCast(u, sb, pick.view.ID, &t, inRange)
	}

	if !queued {
		return 0, false
	}

	delay := 0
	if d.mercHooks.Delay != nil && pick.view.ID != 0 {
		x, y := u.m.SubtilePos()
		delay = d.mercHooks.Delay(MercCast{Owner: owner, Token: b.ID, Skill: pick.rec.Skill, SkillLevel: pick.view.Level,
			Level: u.m.Vitals.Level, X: x, Y: y})
	}

	return delay, true
}

// shadowValidateIn collects what MONAI_ShadowWarrior_ValidateCopiedSkill reads.
func (d *Director) shadowValidateIn(u *unit, sb *shadowBook, e *shadowEntry, tgt d2monster.Target, inRange bool) d2monster.ShadowValidateIn {
	in := d2monster.ShadowValidateIn{
		Skill: e.view, InRange: inRange, HasTarget: true,
		Allowed: d2monster.ShadowCopyAllowed(e.summons, e.summonsSelf, e.petType, sb.petType, u.shadowOwner() != nil),
	}

	if e.view.AuraState > 0 {
		v := d.shadowStateOf(u, nil, sb.name(e.view.AuraState))
		in.SelfState = v.Active

		if val, ok := v.Stats[sb.name(e.view.AuraStat1)]; ok {
			in.StateStat, in.StateStatOK = val, true
		}
	}

	if e.view.AuraTargetState > 0 {
		in.TargetHas = d.targetHasState(u, tgt, sb.name(e.view.AuraTargetState))
	}

	return in
}

func (d *Director) shadowStateOf(u *unit, m *d2mapentity.Monster, state string) MercStateView {
	if d.mercHooks.State == nil || state == "" {
		return MercStateView{}
	}

	return d.mercHooks.State(u.shadowOwner(), u.b.ID, m, state)
}

func (d *Director) targetHasState(u *unit, t d2monster.Target, state string) bool {
	tu := d.units[t.ID]
	if tu == nil || tu.m == nil {
		return false
	}

	return d.shadowStateOf(u, tu.m, state).Active
}

// shadowStateByID is shadowStateOf for a state id of the shared table.
func (d *Director) shadowStateByID(u *unit, state int) MercStateView {
	return d.shadowStateOf(u, nil, d.shadowNameTable().name(state))
}

var _ d2monster.FB1ShadowBook = (*Director)(nil)
var _ d2monster.ShadowPre = (*Director)(nil)
