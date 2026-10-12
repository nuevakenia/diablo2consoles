package d2gamescreen

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// OD2_AUTOBOSSFIGHT=<monstats key>[,key...] (wave 36) fights each named boss in turn: the real monster is spawned
// beside the hero with its own AI, the hero is a spectator that cannot die and a scripted attacker that takes the
// boss's life down in OD2_BOSSFIGHT_SECONDS game seconds (default 45, damage is credited to the hero so the kill,
// the experience, the quest hooks and the treasure class run the normal way). The scenario records which monstats
// skill slots the boss cast ("MONSTER cast" lines), which other monster classes appeared during the fight (summons,
// waves, prisons), whether the boss died and what it dropped, and logs one "BOSSFIGHT summary" line per boss:
//
//	BOSSFIGHT summary boss=<key> diff=<n> casts=<skill:count,...> new=<class:count,...> killed=<bool> drops=<n>
//
// OD2_BOSSFIGHT_HELPERS=1 keeps the monsters that appear (waves, minions) alive for the rest of the fight; the
// default kills nothing but the boss. The scenario scripts/verify.d/w36-boss-fights.sh asserts the lines.

const (
	bossFightRing    = 10   // subtiles from the hero the boss starts at
	bossFightDelay   = 3.0  // seconds after the hero exists
	bossFightDefault = 45.0 // seconds of game time to take the boss from full life to zero
	bossFightGrace   = 3.0  // seconds after the kill (drops, death events)
	bossFightStride  = 0.4  // seconds between the hero's blows
)

type bossFightTest struct {
	keys    []string
	idx     int
	elapsed float64
	round   *bossFightRound
	seconds float64
	ring    int
	failed  []string
	hooked  bool
}

type bossFightRound struct {
	key      string
	boss     *d2mapentity.Monster
	start    float64
	nextHit  float64
	diedAt   float64
	seen     map[*d2mapentity.Monster]bool
	newClass map[string]int
	maxHP    int
	lowest   int
	casts    map[string]int
	fx       map[string]int
	drops    int
}

func bossFightKeys() []string {
	var out []string

	for _, k := range strings.Split(os.Getenv("OD2_AUTOBOSSFIGHT"), ",") {
		if k = strings.ToLower(strings.TrimSpace(k)); k != "" {
			out = append(out, k)
		}
	}

	return out
}

func pairs(m map[string]int) string {
	if len(m) == 0 {
		return "-"
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s:%d", strings.ReplaceAll(k, " ", "_"), m[k])
	}

	return strings.Join(parts, ",")
}

// advanceBossFight runs the OD2_AUTOBOSSFIGHT scenario.
func (v *Game) advanceBossFight(elapsed float64) {
	keys := bossFightKeys()
	if len(keys) == 0 || v.localPlayer == nil || v.monsters == nil {
		return
	}

	t := v.bossFight
	if t == nil {
		t = &bossFightTest{keys: keys, seconds: bossFightDefault}

		if s, err := strconv.ParseFloat(os.Getenv("OD2_BOSSFIGHT_SECONDS"), 64); err == nil && s > 0 {
			t.seconds = s
		}

		t.ring = bossFightRing
		if n, err := strconv.Atoi(os.Getenv("OD2_BOSSFIGHT_RING")); err == nil && n > 0 {
			t.ring = n
		}

		v.bossFight = t
		v.Infof("BOSSFIGHT start bosses=%v seconds=%.0f", keys, t.seconds)
	}

	if v.localPlayer.Stats != nil { // a spectator that cannot die
		v.localPlayer.Stats.Health = v.localPlayer.Stats.MaxHealth
		v.localPlayer.Stats.Mana = v.localPlayer.Stats.MaxMana
	}

	if !t.hooked {
		t.hooked = true
		prev := v.monsters.OnEvent

		v.monsters.OnEvent = func(kind, line string) {
			if prev != nil {
				prev(kind, line)
			}

			r := t.round
			if r == nil {
				return
			}

			switch kind {
			case "cast":
				if !strings.Contains(line, fmt.Sprintf(" id=%d slot=", r.boss.Brain.ID)) {
					return // a summon or a bystander
				}

				if i := strings.Index(line, "skill=\""); i >= 0 {
					rest := line[i+len("skill=\""):]
					if j := strings.Index(rest, "\""); j >= 0 {
						r.casts[rest[:j]]++
					}
				}
			case "bossfx": // missiles / curse / aura lines of the wave 36 skill effects
				if !strings.Contains(line, fmt.Sprintf(" id=%d ", r.boss.Brain.ID)) {
					return
				}

				verb := strings.Fields(line)[1]
				if i := strings.Index(line, "skill=\""); i >= 0 {
					rest := line[i+len("skill=\""):]
					if j := strings.Index(rest, "\""); j >= 0 {
						r.fx[verb+":"+rest[:j]]++
					}
				} else {
					r.fx[verb]++
				}
			case "attack":
				if strings.Contains(line, " missile hit=true") && strings.Contains(line, fmt.Sprintf(" id=%d ", r.boss.Brain.ID)) {
					r.fx["missilehit"]++
				}
			case "drop":
				if strings.Contains(line, "tc=") && strings.Contains(line, "items=") && !strings.Contains(line, "items=0") {
					r.drops++
				}
			}
		}
	}

	t.elapsed += elapsed
	if t.elapsed < bossFightDelay || t.idx >= len(t.keys) {
		return
	}

	if t.round == nil {
		v.startBossRound(t)

		return
	}

	v.stepBossRound(t, elapsed)
}

func (v *Game) startBossRound(t *bossFightTest) {
	key := t.keys[t.idx]
	stat := v.monsters.FindStat(key)

	if stat == nil {
		v.Infof("BOSSFIGHT FAIL boss=%s: no such monster", key)
		t.failed = append(t.failed, key)
		t.idx++
		v.endBossFight(t)

		return
	}

	hx, hy := int(v.localPlayer.Position.X()), int(v.localPlayer.Position.Y())

	m, err := v.monsters.SpawnNear(stat, hx+t.ring, hy, 3)
	if err != nil {
		v.Infof("BOSSFIGHT FAIL boss=%s: spawn: %v", key, err)
		t.failed = append(t.failed, key)
		t.idx++
		v.endBossFight(t)

		return
	}

	r := &bossFightRound{key: key, boss: m, start: t.elapsed, nextHit: t.elapsed + 2, seen: map[*d2mapentity.Monster]bool{},
		newClass: map[string]int{}, casts: map[string]int{}, fx: map[string]int{}, maxHP: m.Vitals.MaxHP, lowest: m.Vitals.MaxHP}

	for _, o := range v.monsters.Monsters() {
		r.seen[o] = true
	}

	t.round = r

	b := m.Brain
	v.Infof("BOSSFIGHT begin boss=%s name=%q class=%d ai=%s implemented=%v hp=%d", key, m.Label(), stat.ID, b.Profile.AI,
		b.Def != nil && b.Def.Implemented, r.maxHP)
}

func (v *Game) stepBossRound(t *bossFightTest, elapsed float64) {
	r := t.round
	age := t.elapsed - r.start

	for _, o := range v.monsters.Monsters() {
		if !r.seen[o] {
			r.seen[o] = true
			r.newClass[o.Label()]++
		}
	}

	if r.boss.Alive() {
		if hp := r.boss.Vitals.HP; hp < r.lowest {
			r.lowest = hp
		}

		// the scripted hero: one blow every bossFightStride seconds, sized so the boss falls in t.seconds
		for t.elapsed >= r.nextHit && age > 2 {
			r.nextHit += bossFightStride

			dmg := int(float64(r.maxHP) * bossFightStride / t.seconds)
			if dmg < 1 {
				dmg = 1
			}

			v.monsters.Damage(r.boss, dmg, v.localPlayer)

			if !r.boss.Alive() {
				break
			}
		}
	}

	if !r.boss.Alive() && r.diedAt == 0 {
		r.diedAt = t.elapsed
	}

	timeout := age > t.seconds*3+20

	if (r.diedAt > 0 && t.elapsed-r.diedAt >= bossFightGrace) || timeout {
		v.finishBossRound(t, timeout)
	}
}

func (v *Game) finishBossRound(t *bossFightTest, timeout bool) {
	r := t.round
	killed := !r.boss.Alive()

	v.Infof("BOSSFIGHT summary boss=%s diff=%s casts=%s new=%s killed=%v drops=%d lowest=%d/%d fx=%s seconds=%.0f", r.key,
		os.Getenv("OD2_AUTOMONSTER_DIFF"), pairs(r.casts), pairs(r.newClass), killed, r.drops, r.lowest, r.maxHP,
		pairs(r.fx), t.elapsed-r.start)

	if timeout || !killed {
		t.failed = append(t.failed, r.key)
		v.Infof("BOSSFIGHT FAIL boss=%s: not killed in time", r.key)
	}

	// the leftovers of this fight (minions, waves) must not disturb the next one
	for _, o := range v.monsters.Monsters() {
		if o.Alive() {
			v.monsters.KillMonster(o)
		}
	}

	t.round = nil
	t.idx++

	if t.idx >= len(t.keys) {
		v.endBossFight(t)
	} else {
		t.elapsed = bossFightDelay
	}
}

func (v *Game) endBossFight(t *bossFightTest) {
	if t.idx < len(t.keys) {
		return
	}

	if len(t.failed) == 0 {
		v.Infof("BOSSFIGHT RESULT PASS bosses=%d", len(t.keys))
	} else {
		v.Infof("BOSSFIGHT RESULT FAIL %v", t.failed)
	}

	t.idx = len(t.keys) + 1
	v.autoTestExit()
}
