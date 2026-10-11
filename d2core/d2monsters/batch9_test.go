package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

func golemOwner(life int) *d2mapentity.Player {
	return &d2mapentity.Player{Stats: &d2hero.HeroStatsState{Health: life, MaxHealth: 1000}}
}

func golemUnit(id uint32, owner *d2mapentity.Player, dmgPct, healPct, hp, maxHP int) *unit {
	u := petUnit(id, 0, 0, "NecroPet", "minion")
	u.ally.owner = owner
	u.ally.opt = MinionOptions{Owner: owner, OwnerDamagePct: dmgPct, HealSharePct: healPct}
	u.m.Vitals.HP, u.m.Vitals.MaxHP = hp, maxHP
	u.b.Allied = true

	return u
}

func TestShareGolemDamage(t *testing.T) {
	owner := golemOwner(500)
	g := golemUnit(1, owner, 10, 25, 300, 300)
	d := testDirector(g)

	if got := d.shareGolemDamage(g, 100); got != 90 || owner.Stats.Health != 490 {
		t.Fatalf("golem takes %d, owner life %d", got, owner.Stats.Health)
	}

	owner.Stats.Health = 3
	if got := d.shareGolemDamage(g, 100); got != 90 || owner.Stats.Health != 1 {
		t.Fatalf("owner is held at one point: golem takes %d, owner %d", got, owner.Stats.Health)
	}

	owner.Stats.Health = 0
	if got := d.shareGolemDamage(g, 100); got != 100 || owner.Stats.Health != 0 {
		t.Fatalf("a dead owner shares nothing: %d", got)
	}

	// a golem without the share (clay, iron, fire) is untouched
	plain := golemUnit(2, golemOwner(500), 0, 0, 300, 300)
	if got := d.shareGolemDamage(plain, 100); got != 100 || plain.ally.owner.Stats.Health != 500 {
		t.Fatalf("plain golem: %d", got)
	}
}

func TestShareHealWithGolem(t *testing.T) {
	owner := golemOwner(500)
	g := golemUnit(1, owner, 10, 25, 100, 1000)
	d := testDirector(g)

	if left := d.ShareHealWithGolem(owner, 200); left != 150 || g.m.Vitals.HP != 150 {
		t.Fatalf("owner keeps %d, golem hp %d", left, g.m.Vitals.HP)
	}

	g.m.Vitals.HP = 995 // only 5 points of room
	if left := d.ShareHealWithGolem(owner, 200); left != 195 || g.m.Vitals.HP != 1000 {
		t.Fatalf("clamped share: owner keeps %d, golem hp %d", left, g.m.Vitals.HP)
	}

	// another hero's golem takes nothing
	if left := d.ShareHealWithGolem(golemOwner(1), 200); left != 200 {
		t.Fatalf("foreign heal changed: %d", left)
	}
}

// siegeDirector knows the base class the siege scan matches.
func siegeDirector(us ...*unit) *Director {
	rec := &d2records.RecordManager{}
	rec.Monster.Stats = d2records.MonStats{"imp": {ID: siegeTargetClass, BaseKey: "Imp"}}

	d := testDirector(us...)
	d.asset = &d2asset.AssetManager{Records: rec}

	return d
}

func TestScanSiegeFilters(t *testing.T) {
	self := scanUnit(1, 0, 0, "Siege", 100)
	near := scanUnit(2, 10, 0, "Imp", 100)
	d := siegeDirector(self, near, scanUnit(3, 5, 0, "Imp", 100))
	q := d2monster.FBXScanQuery{Kind: d2monster.FBXScanSiegeTarget, Radius2: 400}

	// the FIRST unit of the walk wins, not the nearest
	if r := d.FBXScan(self.b, q); !r.Found || r.T.ID != 2 {
		t.Fatalf("first match: %+v", r)
	}

	near.attackTarget = 7 // already has a target (AI +0x14 is not -1)
	if r := d.FBXScan(self.b, q); !r.Found || r.T.ID != 3 {
		t.Fatalf("a unit with a target is skipped: %+v", r)
	}

	d.units[3].b.Mode = d2monster.ModeDead
	if r := d.FBXScan(self.b, q); r.Found {
		t.Fatalf("dead units are skipped: %+v", r)
	}

	d.units[3].b.Mode = d2monster.ModeNeutral
	d.units[3].b.Allied = true // alignment != 0
	if r := d.FBXScan(self.b, q); r.Found {
		t.Fatalf("allied units are skipped: %+v", r)
	}

	d.units[3].b.Allied = false
	d.units[3].ally = &allyState{kind: "minion"} // owned by a hero
	if r := d.FBXScan(self.b, q); r.Found {
		t.Fatalf("owned units are skipped: %+v", r)
	}

	d.units[3].ally = nil
	d.units[3].b.X = 30 // outside the squared radius
	if r := d.FBXScan(self.b, q); r.Found {
		t.Fatalf("radius: %+v", r)
	}

	other := scanUnit(4, 3, 0, "Other", 100)
	d.units[4] = other
	d.nextID = 4

	if r := d.FBXScan(self.b, q); r.Found {
		t.Fatalf("wrong class: %+v", r)
	}
}

func TestSiegeUpdateKeepsTheNearerTarget(t *testing.T) {
	self := scanUnit(1, 0, 0, "Siege", 100)
	a := scanUnit(2, 10, 0, "Imp", 100)
	b := scanUnit(3, 20, 0, "Imp", 100)
	c := scanUnit(4, 4, 0, "Imp", 100)
	d := siegeDirector(self, a, b, c)

	d.FBXSiegeUpdate(self.b, targetOf(a))
	if self.siegeTarget != 2 {
		t.Fatalf("first target: %d", self.siegeTarget)
	}

	d.FBXSiegeUpdate(self.b, targetOf(b)) // farther than the stored one: kept
	if self.siegeTarget != 2 {
		t.Fatalf("farther candidate replaced the target: %d", self.siegeTarget)
	}

	d.FBXSiegeUpdate(self.b, targetOf(c)) // nearer: replaces it
	if self.siegeTarget != 4 {
		t.Fatalf("nearer candidate: %d", self.siegeTarget)
	}

	d.FBXSiegeUpdate(self.b, targetOf(c)) // equal distance: replaces (the stored one is not strictly nearer)
	if self.siegeTarget != 4 {
		t.Fatalf("equal distance: %d", self.siegeTarget)
	}
}

func corpseUnit(id uint32, x, y int, soft bool) *unit {
	u := scanUnit(id, x, y, "Fallen", 0)
	u.m.StatEx = &d2records.MonStat2Record{IsSoft: soft}
	u.m.Brain = u.b
	u.m.Die()

	return u
}

func TestScanRegurgitateKeepsTheNearestSoftCorpse(t *testing.T) {
	self := scanUnit(1, 0, 0, "Regurgitator", 100)
	far := corpseUnit(2, 8, 0, true)
	near := corpseUnit(3, 4, 0, true)
	hard := corpseUnit(4, 1, 0, false)
	d := testDirector(self, far, near, hard)
	q := d2monster.FBXScanQuery{Kind: d2monster.FBXScanRegurgitate, Radius2: 100}

	if r := d.FBXScan(self.b, q); !r.Found || r.T.ID != 3 {
		t.Fatalf("nearest soft corpse: %+v", r)
	}

	q.Radius2 = 9 // 4 squared is 16: nothing inside
	if r := d.FBXScan(self.b, q); r.Found {
		t.Fatalf("radius: %+v", r)
	}

	// equal distances: the later unit replaces the earlier one (the test is <=)
	twin := corpseUnit(5, 0, 4, true)
	d.units[5], d.nextID = twin, 5
	q.Radius2 = 100

	if r := d.FBXScan(self.b, q); !r.Found || r.T.ID != 5 {
		t.Fatalf("tie goes to the last: %+v", r)
	}

	// living and allied units are not corpses
	alive := scanUnit(6, 1, 1, "Fallen", 100)
	alive.m.StatEx = &d2records.MonStat2Record{IsSoft: true}
	d.units[6], d.nextID = alive, 6

	if r := d.FBXScan(self.b, q); r.T.ID == 6 {
		t.Fatalf("a living monster is not eaten: %+v", r)
	}
}

func TestZealotFearGate(t *testing.T) {
	hero := golemOwner(500)
	other := golemOwner(500)
	minion := golemUnit(2, hero, 0, 0, 10, 10)
	merc := petUnit(3, 0, 0, "Melee", "")
	merc.merc = &mercUnit{owner: hero}

	d := testDirector(minion, merc)
	d.targets = map[uint32]*d2mapentity.Player{1: hero, 2: other}

	zealot := &d2monster.Brain{ID: 9}

	if d.FBXAfraidOf(zealot, d2monster.Target{ID: 1, IsPlayer: true}) {
		t.Fatal("without the hook nothing is feared")
	}

	d.opt.ZealotAfraid = func(p *d2mapentity.Player) bool { return p == hero }

	for name, tg := range map[string]d2monster.Target{
		"the hero":         {ID: 1, IsPlayer: true},
		"a hero minion":    {ID: unitTargetBase + 2},
		"a hero mercenary": {ID: mercTargetBase + 3},
	} {
		if !d.FBXAfraidOf(zealot, tg) {
			t.Errorf("%s should be feared", name)
		}
	}

	if d.FBXAfraidOf(zealot, d2monster.Target{ID: 2, IsPlayer: true}) {
		t.Error("a hero without the quest bit is not feared")
	}

	if d.FBXAfraidOf(zealot, d2monster.Target{ID: unitTargetBase + 99}) {
		t.Error("an unknown unit is not feared")
	}
}

func TestPlayerScalingIsTheEffectivePlayerCount(t *testing.T) {
	d := testDirector()

	if got := d.PlayerScaling(nil); got != 1 {
		t.Fatalf("single player: %d", got)
	}

	d.opt.PlayersFunc = func() int { return 3 }
	if got := d.PlayerScaling(nil); got != 3 {
		t.Fatalf("three heroes: %d", got)
	}

	d.opt.ForcedPlayers = 5
	if got := d.PlayerScaling(nil); got != 5 {
		t.Fatalf("forced: %d", got)
	}
}
