package d2monster

import "testing"

// ownerWorld is a world with an owner (the PetWorld Owner / Teleport pair a
// host has for summoned minions) but no FB1Pet of its own.
type ownerWorld struct {
	*fakeWorld
	owner     *OwnerInfo
	teleports int
}

func (w *ownerWorld) Owner(*Brain) (OwnerInfo, bool) {
	if w.owner == nil {
		return OwnerInfo{}, false
	}

	return *w.owner, true
}

func (w *ownerWorld) Teleport(*Brain) bool {
	w.teleports++

	return true
}

// plainBook is a FB1ShadowBook that has skills and strikes in reach.
type plainBook struct {
	*ownerWorld
	copies int
}

func (w *plainBook) HasSkills(*Brain) bool                                 { return true }
func (w *plainBook) CastQueued(*Brain, int, Target, bool) bool             { return false }
func (w *plainBook) Upkeep(*Brain, *Target, bool) bool                     { return false }
func (w *plainBook) Choose(*Brain, Target, *Target, bool) (int, bool)      { return 0, false }
func (w *plainBook) Copy(_ *Brain, _, _ Target, _, _, in bool) (int, bool) { w.copies++; return 0, in }

func TestShadowPetAdapter(t *testing.T) {
	t.Run("a world with an owner needs no FB1Pet", func(t *testing.T) {
		w := &ownerWorld{fakeWorld: newFake(5, true), owner: &OwnerInfo{Target: Target{ID: 9, X: 100, Y: 100, Size: 1, IsPlayer: true}}}
		b := fb1Brain("ShadowWarrior", 50, 50, 50, 1)
		c := &Ctx{B: b, W: w}

		p, ok := shadowPetOf(c)
		if !ok {
			t.Fatal("no pet relation")
		}

		l, ok := p.LeaderUnit(b)
		if !ok || l.ID != 9 {
			t.Errorf("leader %v %v", l, ok)
		}
	})

	t.Run("no owner and no leader link: unavailable, the warrior sleeps 100", func(t *testing.T) {
		w := &ownerWorld{fakeWorld: newFake(5, true)}
		b := fb1Brain("ShadowWarrior", 50, 50, 50, 1)
		fb1Think(w, b, 5, true, true)

		if b.Wake != 100 {
			t.Errorf("wake %d", b.Wake)
		}
	})

	t.Run("owner far away with a target: mode 3, teleport next to the owner", func(t *testing.T) {
		// edge distance of the owner >= 0x51 with a target present (SelectPetActionByRange)
		w := &ownerWorld{fakeWorld: newFake(5, true), owner: &OwnerInfo{Target: Target{ID: 9, X: 100 + 100, Y: 100, Size: 1, IsPlayer: true}}}
		b := fb1Brain("ShadowWarrior", 200, 200, 50, 1)
		fb1Think(w, b, 5, true, true)

		if w.teleports != 1 {
			t.Errorf("teleports %d, log %v", w.teleports, w.log)
		}
	})

	t.Run("owner in reach, target in reach: the book's copy strikes", func(t *testing.T) {
		w := &plainBook{ownerWorld: &ownerWorld{fakeWorld: newFake(5, true),
			owner: &OwnerInfo{Target: Target{ID: 9, X: 100, Y: 100, Size: 1, IsPlayer: true}, Mode: ModeWalk}}}
		b := fb1Brain("ShadowWarrior", 50, 50, 100, 1, 0, 0, 0, 100)
		fb1Think(w, b, 5, true, true)

		if w.copies != 1 {
			t.Errorf("copies %d, log %v", w.copies, w.log)
		}
	})
}
