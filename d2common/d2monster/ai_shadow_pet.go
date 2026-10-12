package d2monster

// Owner relation of the Assassin's shadows (w24-ai-left).
//
// MONAI_Think_ShadowWarrior 0x5e9ff0 and MONAI_Think_ShadowMaster 0x5ea9f0 both
// call MONAI_GetLeaderUnit and MONAI_SelectPetActionByRange(target, leader,
// inRange, 0, 6) (VERIFIED in w20/batch 6; the flag argument is 0 and the leash
// 6). The pet bookkeeping that call needs is already ported for the other
// summoned minions (fbnLeader / fbnSelectPet, ai_pet_fb*.go), so a host that
// has the pet world (Owner, Teleport, optional FBNPetStepper) does not have to
// implement FB1Pet by hand: this adapter supplies it. A host that implements
// FB1Pet itself still wins.

// shadowPetAdapter is FB1Pet over the fbn* pet helpers of one think call.
type shadowPetAdapter struct{ c *Ctx }

// LeaderUnit is MONAI_GetLeaderUnit: the linked leader brain, else the owner
// the host reports.
func (a shadowPetAdapter) LeaderUnit(_ *Brain) (Target, bool) {
	l, ok := fbnLeader(a.c)
	if !ok {
		return Target{}, false
	}

	return l.Target, true
}

// PetAction is MONAI_SelectPetActionByRange(target, leader, inRange, 0, 6).
func (a shadowPetAdapter) PetAction(_ *Brain, tgt *Target, leader Target, _ bool) bool {
	l, ok := fbnLeader(a.c)
	if !ok {
		l = OwnerInfo{Target: leader}
	}

	return fbnSelectPet(a.c, tgt, l, false, false, 6)
}

// shadowPetOf returns the pet relation the shadow thinkers use: the host's own
// FB1Pet, else the adapter when the world can tell an owner (PetWorld-like
// Owner method) or the brain has a leader link.
func shadowPetOf(c *Ctx) (FB1Pet, bool) {
	if p, ok := c.W.(FB1Pet); ok {
		return p, true
	}

	if c.B.Leader != nil {
		return shadowPetAdapter{c}, true
	}

	if _, ok := c.W.(interface {
		Owner(b *Brain) (OwnerInfo, bool)
	}); ok {
		return shadowPetAdapter{c}, true
	}

	return nil, false
}
