package d2portal

import "testing"

func TestItemCost(t *testing.T) {
	tests := []struct {
		name   string
		kind   ItemKind
		charge int
		ok     bool
		want   UseCost
	}{
		{"scroll cast", ItemScroll, 0, true, UseCost{Allowed: true, Consumed: true}},
		{"scroll refused in town", ItemScroll, 0, false, UseCost{Allowed: true}},
		{"tome cast", ItemTome, 5, true, UseCost{Allowed: true, Consumed: true, TomeAfter: 4}},
		{"tome last scroll", ItemTome, 1, true, UseCost{Allowed: true, Consumed: true, TomeAfter: 0}},
		{"tome refused in town keeps charges", ItemTome, 5, false, UseCost{Allowed: true, TomeAfter: 5}},
		{"empty tome never starts", ItemTome, 0, true, UseCost{Allowed: false}},
	}

	for _, tc := range tests {
		if got := ItemCost(tc.kind, tc.charge, tc.ok); got != tc.want {
			t.Errorf("%s: got %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func TestOwnerRule(t *testing.T) {
	const me, other uint32 = 10, 20

	tests := []struct {
		name         string
		owner, user  uint32
		userParty    int
		ownerParty   int
		ownerPresent bool
		want         bool
	}{
		{"ownerless", NoOwner, me, NoParty, NoParty, false, true},
		{"owner", me, me, NoParty, NoParty, true, true},
		{"stranger without party", other, me, NoParty, 1, true, false},
		{"stranger other party", other, me, 2, 1, true, false},
		{"same party", other, me, 1, 1, true, true},
		{"owner left the game, user in a party", other, me, 3, NoParty, false, true},
		{"owner left the game, user in no party", other, me, NoParty, NoParty, false, false},
	}

	for _, tc := range tests {
		if got := OwnerRule(tc.owner, tc.user, tc.userParty, tc.ownerPresent, tc.ownerParty); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

// The pair of the caster: field end F (remembered), town end T. Entering F goes to town and keeps the pair; the trip back
// through T closes both.
func TestDecideUseRoundTrip(t *testing.T) {
	const (
		me  uint32 = 7
		fID uint32 = 100
		tID uint32 = 200
	)

	base := UseInput{Class: ClassTownPortal, HasLinked: true, Owner: me, User: me, UserParty: NoParty, SinceLastUseMs: -1, LastPortalID: fID}

	down := base
	down.PortalID, down.LinkedID = fID, tID

	if r := DecideUse(down); r.Outcome != UseOK || !r.UseLinked || r.DestroyPair {
		t.Errorf("field end: %+v", r)
	}

	up := base
	up.PortalID, up.LinkedID = tID, fID

	if r := DecideUse(up); r.Outcome != UseOK || !r.UseLinked || !r.DestroyPair {
		t.Errorf("town end of the owner must close the pair: %+v", r)
	}

	// a party member (his own remembered portal differs) never closes it
	guest := up
	guest.User, guest.UserParty, guest.OwnerParty, guest.OwnerPresent, guest.LastPortalID = 8, 1, 1, true, 999

	if r := DecideUse(guest); r.Outcome != UseOK || r.DestroyPair {
		t.Errorf("guest: %+v", r)
	}

	// a permanent portal is never destroyed
	perm := up
	perm.Class = ClassPermanentPortal

	if r := DecideUse(perm); r.DestroyPair {
		t.Errorf("permanent portal destroyed: %+v", r)
	}

	// a portal without a partner uses the level in its object data
	lone := base
	lone.HasLinked, lone.PortalID = false, fID

	if r := DecideUse(lone); r.UseLinked || r.DestroyPair {
		t.Errorf("lone: %+v", r)
	}
}

func TestDecideUseRefusals(t *testing.T) {
	base := UseInput{Class: ClassTownPortal, HasLinked: true, PortalID: 1, LinkedID: 2, Owner: 5, User: 5, SinceLastUseMs: -1, UserParty: NoParty, LastPortalID: 1}

	tests := []struct {
		name string
		mod  func(*UseInput)
		want UseOutcome
	}{
		{"ok", func(*UseInput) {}, UseOK},
		{"busy", func(i *UseInput) { i.Busy = true }, UseRefusedBusy},
		{"hostile, not mine", func(i *UseInput) { i.Hostile, i.User, i.UserParty = true, 6, 1 }, UseRefusedHostile},
		{"hostile, mine", func(i *UseInput) { i.Hostile = true }, UseOK},
		{"cooldown", func(i *UseInput) { i.SinceLastUseMs = 4999 }, UseRefusedCooldown},
		{"cooldown over", func(i *UseInput) { i.SinceLastUseMs = 5000 }, UseOK},
		{"not yours", func(i *UseInput) { i.User, i.OwnerPresent = 6, true }, UseRefusedNotYours},
		// the quest gate only bites a user whose remembered portal is neither end (somebody else's pair)
		{"quest locked guest", func(i *UseInput) {
			i.User, i.UserParty, i.OwnerParty, i.OwnerPresent, i.LastPortalID, i.QuestLocked = 6, 1, 1, true, 77, true
		}, UseRefusedQuest},
		{"quest ignored for the caster", func(i *UseInput) { i.QuestLocked = true }, UseOK},
		{"quest ignored on a permanent portal", func(i *UseInput) { i.Class, i.LastPortalID, i.QuestLocked = ClassPermanentPortal, 77, true }, UseOK},
	}

	for _, tc := range tests {
		in := base
		tc.mod(&in)

		if got := DecideUse(in).Outcome; got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}
