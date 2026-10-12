package d2skill

import "sort"

// Whirlwind rules read from the exe (read-only Ghidra, Wave 20):
//
//   - SKILL_WhirlwindComputeNextStepDelay 0x5d7de0 is a hit scheduler, not a
//     plain delay: the first call stores "frame + 4" and allows one hit; later
//     calls allow none until the game frame reaches the stored frame, then
//     advance it by the weapon period (WhirlDelay) and allow 1 hit, or 2 when
//     both hands hold melee weapons (SKILL_IsDualWieldingTypeWeapons 0x5d7d90,
//     item type 0x2e = ItemTypes row 46 = "mele", VERIFIED by table, see TestExeTableRows).
//   - SRVDO_076 0x5d8010 picks every victim with SKILL_ScanRadiusForTargets
//     0x569a80 (radius 5, euclidean: squared distance <= 25, VERIFIED in
//     0x569510). Its callback (0x569a40) walks the units in range and chooses
//     the smallest unit id strictly greater than the previous victim's id,
//     else the smallest id of all (wrapping). So victims rotate in id order and
//     a lone monster is hit again and again (the old port skipped it).
//   - With no victim in range the remembered id is reset and the rest of that
//     tick's hits are dropped.
//   - SRVST_038 0x5d7a00 moves at the class walk velocity (charstats
//     WalkVelocity << 8, UNIT_GetBaseMoveSpeedFixed 0x56c370), not the run one;
//     the exe refuses to start while unit state 0xc is set (state 12 = states.txt row
//     "inferno", VERIFIED by table, see TestExeTableRows; enforced by the engine as whirlRefusedState).

// WhirlFirstDelay is the frames added on the first scheduler call.
const WhirlFirstDelay = 4

// WhirlRadius2 is the squared scan radius (5 subtiles).
const WhirlRadius2 = 25

// WhirlSched is the per-run scheduler of SKILL_WhirlwindComputeNextStepDelay.
// Next is the stored frame (the skill's aux24); zero means "not started".
type WhirlSched struct{ Next int }

// Step returns how many hits happen at game frame frame: period is
// WhirlDelay's value, dual says both hands hold melee weapons.
func (s *WhirlSched) Step(frame, period int, dual bool) int {
	if s.Next == 0 {
		s.Next = frame + WhirlFirstDelay

		return 1
	}

	if frame < s.Next {
		return 0
	}

	next := s.Next + period
	if next < frame {
		next = frame + period
	}

	s.Next = next

	if dual {
		return 2
	}

	return 1
}

// WhirlNext picks the index in ids of the next victim after last (a unit id,
// "" for none): the smallest id greater than last, else the smallest id.
// Returns -1 for an empty list. ids is not modified. The engine's string ids
// stand for the exe's increasing unit ids (UNVERIFIED: they sort in spawn
// order only when the host's ids do).
func WhirlNext(ids []string, last string) int {
	if len(ids) == 0 {
		return -1
	}

	order := make([]int, len(ids))
	for i := range order {
		order[i] = i
	}

	sort.Slice(order, func(a, b int) bool { return ids[order[a]] < ids[order[b]] })

	if last != "" {
		for _, i := range order {
			if ids[i] > last {
				return i
			}
		}
	}

	return order[0]
}

// DualWielder is implemented by units that can report two melee weapons.
type DualWielder interface{ DualWielding() bool }
