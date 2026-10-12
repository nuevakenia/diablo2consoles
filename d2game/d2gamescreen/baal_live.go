package d2gamescreen

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2boss"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2monsters"
)

// The live Baal encounter (wave 43). Until then the Throne of Destruction only ran under OD2_AUTOBOSS=baal: in a played game
// nothing happened on the throne, Baal never came, the quest "Eve of Destruction" and the next difficulty were out of
// reach. The decisions are d2boss.Throne's (waves, the throne morph, Baal walking into the Worldstone Chamber portal); the
// scenario harness of autoboss.go carries them out with real monsters near the hero and this file lets the running game
// use the same code: it hooks the Baal AIs of the level's director to the manager, feeds it the kills and the clock, and
// makes the portal a real object. Where the original stands the throne, the waves and the portal is UNVERIFIED (near the
// hero, like the harness).

type baalLive struct {
	t      *bossAutoTest
	dir    *d2monsters.Director // the director the hooks are on (a level change makes a new one)
	frames float64
	portal bool
}

// baalLiveEnter runs when the hero arrives in a level.
func (v *Game) baalLiveEnter(level int) {
	if (level != d2boss.LevelThrone && level != d2boss.LevelWorldstone) || len(bossAutoNames()) > 0 || v.localPlayer == nil {
		return
	}

	d := v.monsterDirector()
	if d == nil || v.quests() == nil {
		return
	}

	if v.baal == nil {
		t := &bossAutoTest{
			super: map[*d2mapentity.Monster]int{}, objects: map[int]d2monster.Point{},
			baseline: map[string]uint16{}, results: map[string]uint16{},
		}
		t.mgr = d2boss.New(func(s string) { v.Infof("%s", s) })
		v.baal = &baalLive{t: t}
	}

	b := v.baal
	b.portal = false
	b.t.spawned = nil

	if b.dir != d {
		b.dir = d
		v.baalHook(b)
	}

	v.Infof("BAAL live encounter: level %d entered", level)
	v.bossApply(b.t, b.t.mgr.Enter(level))
}

// baalHook connects the Baal AIs of a director to the manager and puts the kill feed in front of the quest system's hook.
func (v *Game) baalHook(b *baalLive) {
	t := b.t

	v.monsters.SetBossHooks(d2monsters.BossHooks{
		Throne: func(_ *d2monster.Brain, step d2monster.ThroneStep, wave int) bool {
			as, ok := t.mgr.ThroneStep(int(step), wave)
			v.bossApply(t, as)

			return ok
		},
		WaveCleared: func() bool { return t.mgr.WaveCleared() },
		NearestObject: func(br *d2monster.Brain, class, radius int) (d2monster.Point, int, bool) {
			p, ok := t.objects[class]
			if !ok {
				return d2monster.Point{}, 0, false
			}

			d := d2monster.EdgeDistance(br.X-p.X, br.Y-p.Y, 1)

			return p, d, d <= radius
		},
		Left: func(_ *d2monster.Brain) {
			v.Infof("BAAL left the level through the portal")
			v.bossApply(t, t.mgr.BaalLeft())
		},
	})

	prev := v.monsters.OnKill
	if prev == nil {
		prev = v.onMonsterKilled
	}

	v.monsters.OnKill = func(ev d2monsters.KillEvent) {
		if prev != nil {
			prev(ev)
		}

		super := -1
		if s, ok := t.super[ev.Monster]; ok {
			super = s
		}

		v.bossApply(t, t.mgr.Killed(d2boss.Kill{Class: ev.Class, Super: super, Name: ev.Label}))
	}
}

// advanceBaalLive gives the manager its clock (25 frames a second) while the hero is in the throne room or the chamber.
func (v *Game) advanceBaalLive(elapsed float64) {
	b := v.baal
	if b == nil || v.localPlayer == nil || v.monsters == nil || b.dir != v.monsters {
		return
	}

	if l := v.currentLevel(); l != d2boss.LevelThrone && l != d2boss.LevelWorldstone {
		return
	}

	b.frames += elapsed * 25

	if n := int(b.frames); n > 0 {
		b.frames -= float64(n)
		v.bossApply(b.t, b.t.mgr.Tick(n))
	}

	// the portal the encounter placed (a record of where it stands) becomes an object the hero can walk into
	if p, ok := b.t.objects[d2boss.ObjWorldstone]; ok && !b.portal {
		b.portal = true
		v.baalPortal(p)
	}
}

// baalPortal makes the portal to the Worldstone Chamber a real object at the recorded subtile.
func (v *Game) baalPortal(p d2monster.Point) {
	rec := v.asset.Records.Object.Details[portalObjectID]
	if rec == nil {
		return
	}

	ob, err := v.gameClient.MapEngine.NewObject(p.X, p.Y, rec, d2resource.PaletteUnits)
	if err != nil {
		v.Warningf("BAAL portal: %v", err)

		return
	}

	ob.PortalDest, ob.PortalOwner = d2boss.LevelWorldstone, v.gameClient.PlayerID
	v.gameClient.MapEngine.AddEntity(ob)
	v.Infof("BAAL the portal to the Worldstone Chamber opens at subtile (%d,%d)", p.X, p.Y)
}
