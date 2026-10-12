package d2quest

import "testing"

// w37: play-like walks through Acts 4 and 5 with the quest log page checked after each step. The page n of quest
// A<a>Q<q> is the string qstsa<a>q<q><n>; the pages pushed by the exe come from the handlers read in w37 (see
// ~/git/d2-re-notes/w37-acts45.md). Pages the port does not push (Izual page 2 from his think hook, Rescue page 2 from
// the freed groups) are listed there as open.

func w37Page(t *testing.T, g *Game, id, want int, what string) {
	t.Helper()

	for _, l := range g.Log() {
		if l.Quest == id {
			if l.Page != want {
				t.Errorf("%s: %s log page %d, want %d (%s)", what, g.Quest(id).Label, l.Page, want, g.Describe(g.Quest(id)))
			}

			return
		}
	}

	t.Fatalf("quest %d not in the log", id)
}

func w37Status(t *testing.T, g *Game, id int, want LogStatus, what string) {
	t.Helper()

	for _, l := range g.Log() {
		if l.Quest == id && l.Status != want {
			t.Errorf("%s: %s status %d, want %d", what, g.Quest(id).Label, l.Status, want)
		}
	}
}

func TestW37Act4LogPages(t *testing.T) {
	g, _ := newGame(t)

	moveTo(g, 1, LevelPandemonium)
	talk(g, NPCTyrael2)
	g.Tick(20)
	w37Page(t, g, QuestFallenAngel, 1, "Tyrael sent the hero to Izual: look for him")
	w37Page(t, g, QuestTerrorsEnd, 1, "Terror's End opens with Tyrael's second talk")

	// VERIFIED 0x5b1d60: the Plains of Despair do not move the page past 1 (page 2 is pushed by Izual's think hook)
	moveTo(g, LevelPandemonium, LevelPlainsDespair)
	g.Tick(20)
	w37Page(t, g, QuestFallenAngel, 1, "Plains of Despair")

	kill(g, NPCIzual, LevelPlainsDespair)
	g.Tick(20)
	w37Page(t, g, QuestFallenAngel, 3, "Izual dead: see Tyrael")
	w37Status(t, g, QuestFallenAngel, LogCompleting, "Izual dead")

	moveTo(g, LevelPlainsDespair, LevelPandemonium)
	_, eff := talk(g, NPCTyrael2)

	skill := 0

	for _, e := range eff {
		if e.Kind == EffectSkillPoint {
			skill += e.Value
		}
	}

	if skill != 2 {
		t.Errorf("Tyrael pays %d skill points, want 2", skill)
	}

	w37Status(t, g, QuestFallenAngel, LogCompleted, "Tyrael paid")

	// Terror's End: the Chaos Sanctuary keeps page 1, the kill completes at once (no reward talk)
	moveTo(g, LevelPandemonium, LevelChaosSanctum)
	g.Tick(20)
	w37Page(t, g, QuestTerrorsEnd, 1, "Chaos Sanctuary")

	kill(g, NPCDiablo, LevelChaosSanctum)
	g.Tick(20)
	w37Status(t, g, QuestTerrorsEnd, LogCompleted, "Diablo dead")
}

func TestW37HellforgeLogPages(t *testing.T) {
	g, _ := newGame(t)

	moveTo(g, 1, LevelPandemonium)
	talk(g, NPCTyrael2)
	kill(g, NPCDiablo, LevelChaosSanctum)

	// Cain without a stone makes one (679); with one he explains the forge (678)
	if l := g.Activate(NPCCain4).Lines; len(l) == 0 || l[0].Msg != 679 {
		t.Fatalf("Cain without a soulstone: %+v", l)
	}

	talk(g, NPCCain4)
	g.Tick(20)
	w37Page(t, g, QuestHellforge, 4, "Cain closed his dialog: consult with Cain was shown")

	pickup(g, ItemMephistoSoulstone)
	obj(g, ObjectHellforge, LevelRiverOfFlame)
	g.Tick(20)
	w37Page(t, g, QuestHellforge, 3, "forge opened: use the hammer")

	pickup(g, ItemHellforgeHammer)

	for i := 0; i < 3; i++ {
		g.Dispatch(Event{Kind: EvObjectOperated, Object: ObjectHellforge, Level: LevelRiverOfFlame, Wielded: ItemHellforgeHammer})
	}

	w37Status(t, g, QuestHellforge, LogCompleting, "forge smashed")

	talk(g, NPCCain4)
	w37Status(t, g, QuestHellforge, LogCompleted, "Cain thanked")
}

func TestW37Act5LogPages(t *testing.T) {
	g, _ := newGame(t)
	g.Hero.Level = 30

	// Siege: Larzuk starts it, the foothills keep page 1, Shenk's death leaves it pending until the sockets
	moveTo(g, 1, LevelHarrogath)
	talk(g, NPCLarzuk)
	w37Page(t, g, QuestSiege, 1, "Larzuk")
	moveTo(g, LevelHarrogath, LevelBloodyFoothills)
	g.Tick(20)
	w37Page(t, g, QuestSiege, 1, "Bloody Foothills")
	g.Dispatch(Event{Kind: EvMonsterKilled, Monster: 999, Super: "Shenk the Overseer", Level: LevelBloodyFoothills})
	g.Tick(20)
	w37Status(t, g, QuestSiege, LogCompleting, "Shenk dead")
	moveTo(g, LevelBloodyFoothills, LevelHarrogath)
	talk(g, NPCLarzuk)
	w37Status(t, g, QuestSiege, LogCompleting, "Larzuk's thanks keep the reward pending until the sockets")
	g.ClaimSocket()
	w37Status(t, g, QuestSiege, LogCompleted, "sockets added")

	// Rescue: Qual-Kehk page 1, both Highlands levels keep page 1 (exe levels 111 and 112), three doors: page 3
	talk(g, NPCQualKehk)
	w37Page(t, g, QuestRescue, 1, "Qual-Kehk")
	moveTo(g, LevelHarrogath, LevelArreatPlateau)
	g.Tick(20)

	if g.Quest(QuestRescue).State != 3 {
		t.Errorf("Arreat Plateau must advance Rescue to state 3: %s", g.Describe(g.Quest(QuestRescue)))
	}

	w37Page(t, g, QuestRescue, 1, "Arreat Plateau")

	for i := 0; i < 3; i++ {
		kill(g, NPCPrisonDoor, LevelFrigidHighlands)
	}

	g.Tick(20)
	w37Page(t, g, QuestRescue, 3, "doors broken: return to Qual-Kehk")
	moveTo(g, LevelFrigidHighlands, LevelHarrogath)

	_, eff := talk(g, NPCQualKehk)
	if got := giveCodes(eff); len(got) != 3 {
		t.Errorf("15 barbarians pay three runes, got %v", got)
	}

	w37Status(t, g, QuestRescue, LogCompleted, "Qual-Kehk paid")
}

func TestW37PrisonLogPages(t *testing.T) {
	g, _ := newGame(t)
	g.Hero.Level = 30
	g.ForceDone(QuestRescue)

	moveTo(g, 1, LevelHarrogath)

	// VERIFIED 0x587460: Malah's first talk moves the state but pushes no page
	talk(g, NPCMalah)
	w37Page(t, g, QuestPrison, 0, "Malah asked")

	// VERIFIED 0x587c70: the Frozen River pushes page 1; Anya's frozen line (20131) page 3
	moveTo(g, LevelHarrogath, LevelFrozenRiver)
	w37Page(t, g, QuestPrison, 1, "Frozen River")

	talk(g, NPCAnyaFrozen)
	w37Page(t, g, QuestPrison, 3, "frozen Anya spoke")

	// Malah's potion is given once per game
	moveTo(g, LevelFrozenRiver, LevelHarrogath)

	_, eff := talk(g, NPCMalah)
	if got := giveCodes(eff); len(got) != 1 || got[0] != ItemDefrostPotion {
		t.Fatalf("Malah's potion: %v", got)
	}

	pickup(g, ItemDefrostPotion)
	w37Page(t, g, QuestPrison, 4, "potion in the pack")

	// a hero who dropped the potion does not get a second one
	g.Dispatch(Event{Kind: EvItemRemoved, Item: ItemDefrostPotion})

	if _, eff = talk(g, NPCMalah); len(giveCodes(eff)) != 0 {
		t.Errorf("Malah gave a second potion: %v", giveCodes(eff))
	}

	pickup(g, ItemDefrostPotion)
	moveTo(g, LevelHarrogath, LevelFrozenRiver)
	g.Dispatch(Event{Kind: EvObjectOperated, Object: ObjectFrozenAnya, Level: LevelFrozenRiver})
	w37Page(t, g, QuestPrison, 5, "Anya thawed: both owe a gift")
	moveTo(g, LevelFrozenRiver, LevelHarrogath)

	// Malah first: the scroll; the page goes to 6 (only Anya is left)
	_, eff = talk(g, NPCMalah)
	if got := giveCodes(eff); len(got) != 1 || got[0] != ItemMalahScroll {
		t.Fatalf("Malah's scroll: %v", got)
	}

	w37Page(t, g, QuestPrison, 6, "scroll given, Anya left")
	w37Status(t, g, QuestPrison, LogCompleting, "scroll given")

	// Anya's item: a rare piece of the hero's class; the quest is done and Betrayal starts
	_, eff = talk(g, NPCDrehya)
	if got := giveCodes(eff); len(got) != 1 {
		t.Fatalf("Anya's item: %v", got)
	}

	w37Status(t, g, QuestPrison, LogCompleted, "both gifts given")

	// Anya gives her item once only
	if _, eff = talk(g, NPCDrehya); len(giveCodes(eff)) != 0 {
		t.Errorf("Anya gave a second item: %v", giveCodes(eff))
	}

	// the scroll gives +10 to the four resistances, once
	if eff = g.ReadScrollOfResistance(); !effectCode(eff, EffectReward, "resist-bonus") {
		t.Errorf("scroll: %+v", eff)
	}

	if eff = g.ReadScrollOfResistance(); len(eff) != 0 {
		t.Errorf("the scroll was read twice: %+v", eff)
	}
}

func TestW37BetrayalRiteEvePages(t *testing.T) {
	g, _ := newGame(t)
	g.Hero.Level = 30
	g.ForceDone(QuestPrison)

	moveTo(g, 1, LevelHarrogath)
	talk(g, NPCDrehya)
	w37Page(t, g, QuestBetrayal, 1, "Anya's portal")
	moveTo(g, LevelHarrogath, LevelNihlathakTemple)
	kill(g, NPCNihlathakBoss, LevelNihlathakTemple)
	g.Tick(20)
	w37Status(t, g, QuestBetrayal, LogCompleting, "Nihlathak dead")
	moveTo(g, LevelNihlathakTemple, LevelHarrogath)
	talk(g, NPCDrehya)
	w37Status(t, g, QuestBetrayal, LogCompleting, "Anya's thanks pay nothing yet")
	g.ClaimPersonalize()
	w37Status(t, g, QuestBetrayal, LogCompleted, "personalised")

	// Rite of Passage: page 1 from Qual-Kehk and on the summit (VERIFIED 0x58a9a0), done by the third Ancient
	talk(g, NPCQualKehk)
	w37Page(t, g, QuestRite, 1, "Qual-Kehk")
	moveTo(g, LevelHarrogath, LevelArreatSummit)
	w37Page(t, g, QuestRite, 1, "summit")

	for _, a := range []int{NPCAncient1, NPCAncient2, NPCAncient3} {
		kill(g, a, LevelArreatSummit)
	}

	w37Status(t, g, QuestRite, LogCompleted, "Ancients dead")

	// Eve of Destruction: Baal in the Worldstone Chamber completes it, in the throne room it does not
	moveTo(g, LevelArreatSummit, LevelWorldstone1)
	moveTo(g, LevelWorldstone1, LevelThrone)
	kill(g, NPCBaalCrab, LevelThrone)
	w37Status(t, g, QuestEve, LogNotStarted, "Baal in the throne room")
	moveTo(g, LevelThrone, LevelWorldstoneChamber)
	kill(g, NPCBaalCrab, LevelWorldstoneChamber)
	w37Status(t, g, QuestEve, LogCompleted, "Baal dead")
}
