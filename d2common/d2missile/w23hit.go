package d2missile

// Wave 23: hit functions of monster missiles. Read in Game.exe 1.14b (V).

// Tables of SKILL_SpawnMissileRingPattern (0x56b270, V): the cardinal and
// diagonal ring points (x at 0x6e27a0, y at 0x6e2780) and the offset pairs
// (a = 0x6e27ec, b = 0x6e27c0) combined with the sign tables 0x6e2808 (x) and
// 0x6e27dc (y).
var (
	ringCardX = [7]int{20, -20, 0, 0, 14, -14, -14}
	ringCardY = [7]int{0, 0, 20, -20, 14, 14, -14}
	ringA     = [7]int{18, 20, 17, 20, 15, 19, 18}
	ringB     = [7]int{8, 2, 11, 4, 13, 6, 9}
	ringSX    = [4]int{-1, 1, 1, -1}
	ringSY    = [4]int{-1, -1, 1, 1}
)

// RingOffsets returns the destination offsets (from the source) at which
// SKILL_SpawnMissileRingPattern creates its missiles for a requested count n,
// in creation order. The first missile is always (14, -14). Then, for up to
// seven rings, the exe creates pairs (sx*a, sy*b) and (sx*b, sy*a) for the four
// sign pairs while the counter n-1 stays positive (each pair costs 2), and one
// cardinal point per ring (cost 1). The outer loop only stops when the counter
// is exactly 0 at the top of a ring or after the seventh ring, so a counter
// that went negative still gets the remaining cardinal points. A count whose
// (n-1)/5 exceeds 7 is a fatal error in the exe; here the list is empty.
func RingOffsets(n int) [][2]int {
	if (n-1)/5 > 7 {
		return nil
	}

	out := [][2]int{{14, -14}}
	left := n - 1

	for i := 0; i < 7; i++ {
		if left == 0 {
			return out
		}

		for j := 0; j < 4; j++ {
			if left < 1 {
				break
			}

			out = append(out, [2]int{ringSX[j] * ringA[i], ringSY[j] * ringB[i]})
			out = append(out, [2]int{ringSX[j] * ringB[i], ringSY[j] * ringA[i]})
			left -= 2
		}

		out = append(out, [2]int{ringCardX[i], ringCardY[i]})
		left--
	}

	return out
}

// ring creates a RingOffsets(n) pattern of missile sub (create kind 3: source
// and destination offset) at the missile; path10 gives them the type 10 path
// (the creation callback 0x5a9b60 sets path type 10 and recomputes the path).
func (s *Sim) ring(m *Missile, name string, n int, path10 bool) {
	sub := s.lookup(name)
	if sub == nil {
		return
	}

	for i, o := range RingOffsets(n) {
		p := CreateParams{Spec: sub, Parent: m, Owner: m.Owner, SkillID: m.SkillID, Level: m.Level,
			Damage: m.Damage, AreaRadius: m.AreaRadius, X: m.X, Y: m.Y, DestX: m.X + float64(o[0]), DestY: m.Y + float64(o[1])}

		if path10 {
			p.PathType10, p.PathSeed = true, uint32(i) // seed source UNVERIFIED
		}

		_, _ = s.Create(p)
	}
}

// w23Hit holds the monster hit functions:
//   - 15 (0x5a8820): HitSubMissile1 on the missile's cell; 1 (ends) without a
//     sub missile or owner, else 0.
//   - 16 (0x5a8900): the skill's aura state on the target for max(calc4, 5)
//     frames; 1 when the skill, owner or state is missing; 0 otherwise.
//   - 19 (0x5a8c00): the same with auralencalc (min 5); 3 when applied, 1 else.
//   - 24 (0x5a7680): area damage at the missile, radius sHitPar1 else calc1
//     (min 1) (AreaRadius); returns 1.
//   - 44 (0x5a7890): the area damage of hit 3 regardless of the target, radius
//     sHitPar1 else aurarangecalc; 1 when something was hit, 3 otherwise (the
//     sim returns 1).
//   - 31 (0x5a98c0): needs an owner and a target; heals the owner by the rolled
//     damage (clamped to its maximum); returns 2 + CollideKill (UNVERIFIED which
//     damage components the calc type fills; the sim sums all).
//   - 38 (0x5a9bc0): ring of HitSubMissile1 at the missile, count
//     (level-1)*sHitPar2 + sHitPar1 (calc4 when not positive), each with path
//     type 10; returns 1.
//   - 45 (0x5a9f90): the same ring with count sHitPar1 (> 0 required), path
//     type 10 only when sHitPar2 is set.
//   - 43 (0x5a9e60): the target regains life min + rand(max - min) of the
//     skill (HealMin/HealMax); returns CollideKill (1 without target).
func (s *Sim) w23Hit(m *Missile, t Target) int {
	sp := m.Spec

	switch sp.SrvHitFunc {
	case 15:
		if sp.HitSubMissile[0] == "" || m.Owner.ID == "" {
			return resKill
		}

		s.spawnHitSub(m, 1)

		return 0
	case 16, 19:
		if m.Owner.ID == "" || m.Skill.AuraTargetState == "" || (sp.SrvHitFunc == 19 && t == nil) {
			return resKill
		}

		frames := m.Skill.AuraLen
		if sp.SrvHitFunc == 16 {
			frames = m.Skill.Calc4
		}

		if t != nil {
			if frames < 5 {
				frames = 5
			}

			s.effect(m, Event{Kind: EventState, Missile: m, Target: t, Name: m.Skill.AuraTargetState, Frames: frames})
		}

		if sp.SrvHitFunc == 19 {
			return resKill | resDamage
		}

		return 0
	case 24, 44:
		if m.Owner.ID == "" && sp.SrvHitFunc == 24 {
			return resKill
		}

		s.areaDamage(m)

		return resKill
	case 31:
		if m.Owner.ID == "" || t == nil {
			return resKill
		}

		d := m.Damage.Roll(m.Owner.Roller)
		amount := int(d.Physical + d.Fire + d.Lightning + d.Magic + d.Cold + d.Poison)

		if amount > 0 && m.Owner.Heal != nil {
			m.Owner.Heal(amount)
		}

		if sp.CollideKill {
			return resKill | resDamage
		}

		return resDamage
	case 38, 45:
		if sp.HitSubMissile[0] == "" || m.Owner.ID == "" {
			return resKill
		}

		n := sp.SHitPar[0]
		path10 := true

		if sp.SrvHitFunc == 38 {
			n = (m.Level-1)*sp.SHitPar[1] + sp.SHitPar[0]
			if n < 1 {
				n = m.Skill.Calc4
			}
		} else {
			path10 = sp.SHitPar[1] != 0
			if n <= 0 {
				return resKill
			}
		}

		s.ring(m, sp.HitSubMissile[0], n, path10)

		return resKill
	case 43:
		if t == nil {
			return resKill
		}

		amount := m.HealMin
		if span := m.HealMax - m.HealMin; span > 0 && m.Owner.Roller != nil {
			amount += int(m.Owner.Roller.Roll(int32(span)))
		}

		if h, ok := t.(Healer); ok && amount > 0 {
			h.Heal(amount)
		}

		s.emit(Event{Kind: EventHeal, Missile: m, Target: t, Heal: amount})

		if sp.CollideKill {
			return resKill
		}

		return 0
	}

	return 0
}

// Blade Fury style burst (hit 52): the eight offsets (x at 0x6e3f08, y at
// 0x6e3ee8, V bytes) of radius 16 around the missile.
var burstX = [8]int{16, 16, 0, -16, -16, -16, 0, 16}
var burstY = [8]int{0, 16, 16, 16, 0, -16, -16, -16}

// ManaHolder is optionally implemented by a Target (a player) for hit
// function 55.
type ManaHolder interface {
	Mana() int
	SetMana(v int)
}

// w23HitB holds hit functions 52, 55 and 58 (monster missiles).
//   - 52 (0x5aa490): with HitSubMissile1 and an owner, the sub missile is
//     created at every sHitPar1-th (min 1) of the eight burst offsets (create
//     kind 2: destination = parent position + offset), with the offset kept
//     in its data fields 0x28 / 0x2c; returns 1.
//   - 55 (0x5aa6b0, Baal's inferno): only players; with sHitPar1 > 0 the
//     target loses mana: mana * sHitPar1 / 100 (min 1, at most its mana; the
//     MulDiv operands are UNVERIFIED), result 2 (the damage applies, the
//     missile stays); 1 when the target has no mana or sHitPar1 is empty; 2
//     for a non-player.
//   - 58 (0x5aa940): one HitSubMissile1 at a random cell within sHitPar1
//     (min 1) of the missile (seed = integer x; the two offset rolls are
//     modelled with Roll, UNVERIFIED against RAND_RollSeedBounded); returns 1.
func (s *Sim) w23HitB(m *Missile, t Target) int {
	sp := m.Spec

	switch sp.SrvHitFunc {
	case 52:
		sub := s.lookup(sp.HitSubMissile[0])
		if sp.HitSubMissile[0] == "" {
			return resKill
		}

		if m.Owner.ID == "" || sub == nil {
			return resKill
		}

		step := sp.SHitPar[0]
		if step < 1 {
			step = 1
		}

		for i := 0; i < 8; i += step {
			ch, _ := s.Create(CreateParams{Spec: sub, Parent: m, Owner: m.Owner, SkillID: m.SkillID, Level: m.Level,
				Damage: m.Damage, AreaRadius: m.AreaRadius, X: m.X, Y: m.Y,
				DestX: m.X + float64(burstX[i]), DestY: m.Y + float64(burstY[i])})
			if ch != nil {
				ch.Data28, ch.Data2C = uint32(int32(burstX[i])), uint32(int32(burstY[i]))
			}
		}

		return resKill
	case 55:
		if t == nil || !t.IsPlayer() {
			return resDamage
		}

		if sp.SHitPar[0] <= 0 {
			return resKill
		}

		mh, ok := t.(ManaHolder)
		if !ok || mh.Mana() < 1 {
			return resKill
		}

		mana := mh.Mana()

		drain := mana * sp.SHitPar[0] / 100
		if drain < 1 {
			drain = 1
		}

		if drain > mana {
			drain = mana
		}

		mh.SetMana(mana - drain)

		return resDamage
	case 58:
		sub := s.lookup(sp.HitSubMissile[0])
		if sp.HitSubMissile[0] == "" || m.Owner.ID == "" {
			return resKill
		}

		r := sp.SHitPar[0]
		if r < 1 {
			r = 1
		}

		cx, cy := int(m.X), int(m.Y)
		m.seed.Init(uint32(cx))

		dx := int(m.seed.Roll(int32(r*2+1))) + cx - r
		dy := int(m.seed.Roll(int32(r*2+1))) + cy - r

		if sub != nil {
			_, _ = s.Create(CreateParams{Spec: sub, Parent: m, Owner: m.Owner, SkillID: m.SkillID, Level: m.Level,
				Damage: m.Damage, AreaRadius: m.AreaRadius, X: m.X, Y: m.Y, DestX: float64(dx) + 0.5, DestY: float64(dy) + 0.5})
		}

		return resKill
	}

	return 0
}
