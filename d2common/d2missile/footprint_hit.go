package d2missile

import "math"

// NearWorld is optionally implemented by a World: every living unit whose own cell lies within r cells
// (Chebyshev) of (x, y), without the unit-size reach that World.Targets applies. It lets the Sim apply the exe's
// footprint test (FootprintOverlap) to missiles of Size 2 and 3.
type NearWorld interface {
	TargetsWithin(x, y, r int) []Target
}

// hitCandidates are the units a missile standing on node (cx, cy) tests this step. Size 1 missiles (and worlds
// without NearWorld) keep the engine's own per-cell lookup. Size 2 and 3 missiles (w37: blizzard shards, the
// erruption and volcano fire, the molten boulder, lightning) use the exe's footprint test of the standard move
// (ROOM_ForEachNearRoomContainingPoint 0x6430a0 with the MISSILE's size, VERIFIED w22): a unit is hit when
// FootprintOverlap says its cell and its size overlap the missile's footprint. A unit that cannot say where it is
// (no Positioned) stays on the per-cell lookup.
func (s *Sim) hitCandidates(m *Missile, cx, cy int) []Target {
	base := s.World.Targets(cx, cy)

	size := m.Spec.Size
	if size < 2 || size > 3 {
		return base
	}

	nw, ok := s.World.(NearWorld)
	if !ok {
		return base
	}

	var out []Target

	seen := map[string]bool{}

	for _, t := range nw.TargetsWithin(cx, cy, 2) {
		pos, ok := t.(Positioned)
		if !ok || seen[t.ID()] {
			continue
		}

		ux, uy := pos.SubPos()

		us := targetSize(t)
		if us < 1 {
			us = 1
		}

		if FootprintOverlap(size, us, cx, cy, int(math.Floor(ux)), int(math.Floor(uy))) {
			seen[t.ID()] = true

			out = append(out, t)
		}
	}

	for _, t := range base {
		if _, ok := t.(Positioned); !ok && !seen[t.ID()] {
			out = append(out, t)
		}
	}

	return out
}
