package d2quest

import (
	"fmt"
	"strings"
	"testing"
)

// w36: play-like walks through Acts 2 and 3. After every step the log page of
// the quests involved is compared with the page the walk expects (the page n
// of quest A<a>Q<q> is the string qstsa<a>q<q><n> of the string table; 13 is
// the completed marker).

func logLine(g *Game, ids ...int) string {
	var sb strings.Builder

	for _, id := range ids {
		q := g.Quest(id)
		fmt.Fprintf(&sb, " %s=p%d", q.Label, g.LogPage(q))
	}

	return sb.String()
}

// pageIs asserts the log page of one quest.
func pageIs(t *testing.T, g *Game, id, want int, what string) {
	t.Helper()

	if got := g.LogPage(g.Quest(id)); got != want {
		t.Errorf("%s: %s log page %d, want %d (%s)", what, g.Quest(id).Label, got, want, g.Describe(g.Quest(id)))
	}
}

func TestW36TaintedSunLogPages(t *testing.T) {
	g, _ := newGame(t)

	moveTo(g, 1, LevelLutGholein)
	pageIs(t, g, QuestTaintedSun, 0, "dormant")
	// UNVERIFIED: the middle pages (2 of the sun, 2 of Radament) are not pushed by the port; the exe pushes them
	// from callbacks (Radament wake-up tick, ...) that are not decoded. Only the first and the last pages are pinned.

	moveTo(g, LevelLutGholein, LevelLostCity)
	moveTo(g, LevelLostCity, LevelLutGholein)
	pageIs(t, g, QuestTaintedSun, 0, "sun darkened but Drognan not met: no page yet")

	talk(g, NPCDrognan)
	moveTo(g, LevelLutGholein, LevelLostCity)

	obj(g, ObjectTaintedSunAltar, 61)
	g.Tick(20)
	pageIs(t, g, QuestTaintedSun, 3, "altar destroyed: speak with the townsfolk")

	moveTo(g, LevelLostCity, LevelLutGholein)
	talk(g, NPCDrognan)
	pageIs(t, g, QuestTaintedSun, 13, "paid by Drognan")
}

func TestW36RadamentLogPages(t *testing.T) {
	g, _ := newGame(t)

	moveTo(g, 1, LevelLutGholein)
	talk(g, NPCAtma)
	pageIs(t, g, QuestRadament, 1, "Atma asked: find the lair")

	moveTo(g, LevelLutGholein, 41)

	kill(g, NPCRadament, LevelSewers3)
	g.Tick(20)
	pageIs(t, g, QuestRadament, 3, "Radament dead: return to Atma")

	// the Book of Skill: exactly one skill point, once
	if eff := g.ReadBookOfSkill(); len(eff) != 1 || eff[0].Kind != EffectSkillPoint || eff[0].Value != 1 {
		t.Errorf("book of skill: %+v", eff)
	}

	if eff := g.ReadBookOfSkill(); len(eff) != 0 {
		t.Errorf("the book gave a second skill point: %+v", eff)
	}

	talk(g, NPCAtma)
	pageIs(t, g, QuestRadament, 13, "paid by Atma")
}
