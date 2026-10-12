package d2monster

import "testing"

// targetFailWorld refuses walks toward a unit but accepts walks to a point,
// like a blocked chase next to a free floor.
type targetFailWorld struct{ *fakeWorld }

func (w targetFailWorld) MoveTo(b *Brain, p Point, t *Target, reach int, run bool) bool {
	if t != nil {
		return false
	}

	return w.fakeWorld.MoveTo(b, p, t, reach, run)
}

// TestWalkToTargetFailureTail covers the flag 2 tail of 0x5dd980 reached by
// WalkToTarget(target, 7): one roll, below 70 wander 4, else sleep 10, and
// the call reports success so the handler's own stall (aip2) never runs.
func TestWalkToTargetFailureTail(t *testing.T) {
	for id := uint32(1); id <= 40; id++ {
		w := targetFailWorld{newFake(12, false)}
		b := NewBrain(id, 1, Normal, profile("Skeleton", 100, 15, 0, 0), testSeed)
		b.X, b.Y = 100, 100

		sh := shadow(b)
		sh.Roll(100) // the approach chance (aip1 = 100)
		wander := sh.Roll(100) < 70

		Tick(w, b)

		if wander {
			if len(w.log) != 1 || b.Wake == 15 {
				t.Fatalf("id %d: want wander walk, log=%v wake=%d", id, w.log, b.Wake)
			}
		} else if b.Wake != 10 || len(w.log) != 0 {
			t.Fatalf("id %d: want sleep 10, log=%v wake=%d", id, w.log, b.Wake)
		}
	}
}

// TestWalkToOtherReachKeepsPlainFailure: only the melee 7 is the flags byte.
func TestWalkToOtherReachKeepsPlainFailure(t *testing.T) {
	w := targetFailWorld{newFake(12, false)}
	b := brainAt(profile("Skeleton", 100, 15, 0, 0))
	c := &Ctx{B: b, W: w}

	if c.WalkTo(w.target, 3) || c.Acted() {
		t.Fatal("reach 3 failure must stay a plain failure")
	}
}

// TestWanderOffsetShapes pins the 0x5dcff0 / 0x5de330 scheme: |offset| of one
// axis is n, the other is in [0,n), 4 LCG steps are consumed, and
// WalkNearTarget uses the same draws around the target.
func TestWanderOffsetShapes(t *testing.T) {
	for _, n := range []int{2, 4, 5, 8} {
		for id := uint32(1); id <= 30; id++ {
			b := NewBrain(id, 1, Normal, profile("Skeleton"), testSeed)
			c := &Ctx{B: b, W: newFake(5, false)}
			ref := shadow(b)

			dx, dy := c.wanderOffset(n)
			ax, ay := abs(dx), abs(dy)

			if !(ax == n && ay < n || ay == n && ax < n) {
				t.Fatalf("n=%d id=%d offset (%d,%d)", n, id, dx, dy)
			}

			// four draws: step, roll, step, step
			s1 := ref.Step()
			r := int(ref.Roll(int32(n)))
			sx, sy := ref.Step(), ref.Step()
			wx, wy := r, n
			if s1&1 != 0 {
				wx, wy = n, r
			}
			if sx&1 != 0 {
				wx = -wx
			}
			if sy&1 != 0 {
				wy = -wy
			}
			if wx != dx || wy != dy {
				t.Fatalf("n=%d id=%d got (%d,%d) want (%d,%d)", n, id, dx, dy, wx, wy)
			}

			if *b.Seed != *ref {
				t.Fatalf("n=%d id=%d seed not advanced by exactly four draws", n, id)
			}
		}
	}
}

func TestWalkNearTargetUsesTargetOrigin(t *testing.T) {
	w := newFake(5, false)
	b := brainAt(profile("Skeleton"))
	ref := shadow(b)
	c := &Ctx{B: b, W: w}
	tg := &Target{ID: 1, X: 150, Y: 160}

	if !c.WalkNearTarget(tg, 4) {
		t.Fatal("walk refused")
	}

	cc := &Ctx{B: &Brain{Seed: ref}}
	dx, dy := cc.wanderOffset(4)
	want := "walk-to(" + itoa(150+dx) + "," + itoa(160+dy) + ")"

	if w.last() != want {
		t.Fatalf("got %q want %q", w.last(), want)
	}
}
