package d2quest

import "testing"

// heard feeds a message the way the client acknowledges it, in the act of the
// current level.
func heard(g *Game, npc, msg int) { g.Hear(npc, msg) }

func kill(g *Game, monster, level int) []Effect {
	return g.Dispatch(Event{Kind: EvMonsterKilled, Monster: monster, Level: level})
}

func obj(g *Game, id, level int) []Effect {
	return g.Dispatch(Event{Kind: EvObjectOperated, Object: id, Level: level})
}

func pickup(g *Game, code string) []Effect {
	return g.Dispatch(Event{Kind: EvItemPickedUp, Item: code})
}

func effectCode(effects []Effect, kind EffectKind, code string) bool {
	for _, e := range effects {
		if e.Kind == kind && e.Code == code {
			return true
		}
	}

	return false
}

func bit(g *Game, id, b int) bool { return g.Rec.Get(g.Quest(id).Slot, b) }

func TestAct2To5MessagesHaveNPCsAndSounds(t *testing.T) {
	prefixes := []string{"A2Q2", "A2Q3", "A2Q4", "A2Q5", "A2Q6", "A2Q7", "A3Q0", "A3Q1", "A3Q2", "A3Q3", "A3Q4",
		"A3Q5", "A3Q6", "A4Q0", "A4Q1", "A4Q2", "A4Q3", "A4Q4", "A5Q1", "A5Q2", "A5Q3", "A5Q4", "A5Q5", "A5Q6"}

	for _, q := range prefixes {
		if len(speechTables(q)) == 0 {
			t.Errorf("%s has no speech tables", q)
		}

		for i, tbl := range speechTables(q) {
			for _, s := range tbl {
				if s.NPC < 0 {
					t.Errorf("%s table %d: unknown NPC in %+v", q, i, s)
				}

				if snd, ok := SoundForMessage(s.Msg); !ok || snd.Index == 0 {
					t.Errorf("%s table %d msg %d has no Sounds.txt row", q, i, s.Msg)
				}
			}
		}
	}
}

func TestEveryQuestHasItsSlot(t *testing.T) {
	want := map[int]int{
		QuestHoradricStaff: 10, QuestTaintedSun: 11, QuestArcane: 12, QuestSummoner: 13, QuestSevenTombs: 14,
		QuestA3Prologue: 16, QuestLamEsen: 17, QuestKhalim: 18, QuestBlade: 19, QuestGoldenBird: 20,
		QuestBlackenedTemple: 21, QuestGuardian: 22, QuestA4Prologue: 24, QuestFallenAngel: 25, QuestTerrorsEnd: 26,
		QuestHellforge: 27, QuestMalachai: 33, QuestSiege: 35, QuestRescue: 36, QuestPrison: 37, QuestBetrayal: 38,
		QuestRite: 39, QuestEve: 40,
	}

	g, _ := newGame(t)

	for id, slot := range want {
		q := g.Quest(id)
		if q == nil || q.Slot != slot {
			t.Errorf("quest %d: %+v want slot %d", id, q, slot)
		}
	}
}

func TestHoradricStaffQuest(t *testing.T) {
	g, _ := newGame(t)
	q := g.Quest(QuestHoradricStaff)

	moveTo(g, 1, LevelLutGholein)

	// nothing to report: Cain has nothing to say
	if lines := g.Activate(NPCCain2).Lines; len(lines) != 0 {
		t.Fatalf("lines %+v", lines)
	}

	// the scroll chest gives the Horadric Scroll, Cain deciphers it
	eff := obj(g, ObjectScrollChest, 60)
	if !effectCode(eff, EffectSpawn, ItemHoradricScroll) {
		t.Fatalf("scroll chest: %+v", eff)
	}

	pickup(g, ItemHoradricScroll)

	if !g.Marker(NPCCain2) {
		t.Error("Cain shows no marker for the scroll")
	}

	msgs, eff := talk(g, NPCCain2)
	if len(msgs) != 1 || msgs[0] != 335 || !effectCode(eff, EffectDeleteItem, ItemHoradricScroll) {
		t.Fatalf("scroll: %v %+v", msgs, eff)
	}

	if !g.get(q, FlagLeaveTown) || g.hasItem(ItemHoradricScroll) {
		t.Errorf("%s", g.Describe(q))
	}

	// amulet, staff shaft and cube are reported one by one
	for _, c := range []struct {
		item string
		msg  int
		bit  int
	}{
		{ItemViperAmulet, 336, FlagEnterArea}, {ItemStaffOfKingsShaft, 337, FlagCustom1},
		{ItemHoradricCube, 338, FlagCustom2},
	} {
		pickup(g, c.item)

		msgs, _ = talk(g, NPCCain2)
		if len(msgs) != 1 || msgs[0] != c.msg || !g.get(q, c.bit) {
			t.Fatalf("%s: %v %s", c.item, msgs, g.Describe(q))
		}
	}

	// the cube made the staff: Cain's final line, and the palace quest opens
	pickup(g, ItemHoradricStaff)

	if !g.get(q, flagAssembled) {
		t.Fatalf("not assembled %s", g.Describe(q))
	}

	msgs, _ = talk(g, NPCCain2)
	if len(msgs) != 1 || msgs[0] != 339 || !g.get(q, FlagCustom6) {
		t.Fatalf("final: %v %s", msgs, g.Describe(q))
	}

	// the staff in Tal Rasha's orifice completes the quest and deletes the items
	eff = obj(g, ObjectOrifice, 68)
	if !g.get(q, FlagRewardGranted) || !g.get(q, FlagPrimaryGoal) || !effectCode(eff, EffectDeleteItem, ItemHoradricStaff) ||
		!effectCode(eff, EffectPortal, "") {
		t.Fatalf("orifice: %s %+v", g.Describe(q), eff)
	}
}

func TestTaintedSunAndArcaneSanctuary(t *testing.T) {
	g, _ := newGame(t)
	sun, arc := g.Quest(QuestTaintedSun), g.Quest(QuestArcane)

	moveTo(g, 1, LevelLutGholein)

	if sun.State != 0 || arc.State != 0 {
		t.Fatalf("dormant states %d %d", sun.State, arc.State)
	}

	// entering the Lost City darkens the sun
	moveTo(g, LevelLutGholein, LevelLostCity)
	moveTo(g, LevelLostCity, LevelLutGholein)

	if sun.State != 1 {
		t.Fatalf("sun %s", g.Describe(sun))
	}

	msgs, _ := talk(g, NPCDrognan)
	if len(msgs) == 0 || msgs[0] != 348 || sun.State != 2 || !g.get(sun, FlagStarted) {
		t.Fatalf("Drognan: %v %s", msgs, g.Describe(sun))
	}

	moveTo(g, LevelLutGholein, LevelLostCity)

	if sun.State != 3 || !g.get(sun, FlagLeaveTown) {
		t.Fatalf("left town: %s", g.Describe(sun))
	}

	eff := obj(g, ObjectTaintedSunAltar, 61)
	if !g.get(sun, FlagRewardPending) || !g.get(sun, FlagPrimaryGoal) || !effectCode(eff, EffectSpawn, ItemViperAmulet) {
		t.Fatalf("altar: %s %+v", g.Describe(sun), eff)
	}

	moveTo(g, LevelLostCity, LevelLutGholein)

	if !g.Marker(NPCDrognan) || g.Marker(NPCFara) {
		t.Error("only the NPCs with an m1 line show the marker (Fara has none)")
	}

	msgs, _ = talk(g, NPCDrognan)
	// Drognan pays out (371) and, since the sun quest opens the Arcane
	// Sanctuary quest, goes on with its first line (373) in the same talk
	if len(msgs) != 2 || msgs[0] != 371 || msgs[1] != 373 || !g.get(sun, FlagRewardGranted) ||
		g.get(sun, FlagRewardPending) || arc.State != 2 {
		t.Fatalf("reward: %v %s %s", msgs, g.Describe(sun), g.Describe(arc))
	}

	msgs, _ = talk(g, NPCJerhyn)
	if !contains2(msgs, 377) || arc.State != 3 {
		t.Fatalf("Jerhyn 377: %v %s", msgs, g.Describe(arc))
	}

	// the palace guard speaks one of the open lines
	if l := g.Activate(NPCGuard2).Lines; len(l) != 1 || l[0].Msg < 187 || l[0].Msg > 189 {
		t.Errorf("guard %+v", l)
	}

	moveTo(g, LevelLutGholein, LevelArcane)

	if arc.State != 4 || !g.get(arc, FlagCustom1) {
		t.Fatalf("sanctuary: %s", g.Describe(arc))
	}

	obj(g, ObjectHorazonJournal, LevelArcane)

	if !g.get(arc, FlagRewardPending) || !g.get(arc, FlagPrimaryGoal) {
		t.Fatalf("journal: %s", g.Describe(arc))
	}

	moveTo(g, LevelArcane, LevelLutGholein)
	talk(g, NPCJerhyn)

	if !g.get(arc, FlagRewardGranted) {
		t.Fatalf("claim: %s", g.Describe(arc))
	}
}

func TestSummonerAndSevenTombs(t *testing.T) {
	g, _ := newGame(t)
	sum, arc, tombs, rad := g.Quest(QuestSummoner), g.Quest(QuestArcane), g.Quest(QuestSevenTombs), g.Quest(QuestRadament)

	moveTo(g, 1, LevelArcane)

	if sum.State != 1 || !g.get(sum, FlagStarted) {
		t.Fatalf("summoner %s", g.Describe(sum))
	}

	kill(g, NPCSummoner, LevelArcane)

	if !g.get(sum, FlagRewardPending) || !g.get(sum, FlagPrimaryGoal) {
		t.Fatalf("summoner dead %s", g.Describe(sum))
	}

	// the Summoner's death also completes the Arcane Sanctuary quest
	if !g.get(arc, FlagPrimaryGoal) {
		t.Errorf("arcane %s", g.Describe(arc))
	}

	moveTo(g, LevelArcane, LevelLutGholein)

	msgs, _ := talk(g, NPCAtma)
	if !contains2(msgs, 427) || !g.get(sum, FlagRewardGranted) {
		t.Fatalf("Atma: %v %s", msgs, g.Describe(sum))
	}

	// the Arcane Sanctuary quest (claimed by Atma's 406) opened the Duriel quest
	if tombs.State != 1 {
		t.Fatalf("tombs %s", g.Describe(tombs))
	}

	moveTo(g, LevelLutGholein, 41)
	g.Dispatch(Event{Kind: EvMonsterKilled, Monster: NPCRadament, Level: LevelSewers3})
	moveTo(g, 41, LevelLutGholein)
	talk(g, NPCAtma)

	if !g.get(rad, FlagRewardGranted) || tombs.State != 1 {
		t.Fatalf("radament done, tombs %s", g.Describe(tombs))
	}

	msgs, _ = talk(g, NPCJerhyn)
	if !contains2(msgs, 430) || tombs.State != 2 || !g.get(tombs, FlagStarted) {
		t.Fatalf("Jerhyn 430: %v %s", msgs, g.Describe(tombs))
	}

	moveTo(g, LevelLutGholein, LevelDurielLair)
	kill(g, NPCDuriel, LevelDurielLair)

	if tombs.State != 3 || !g.get(tombs, FlagCustom1) {
		t.Fatalf("Duriel dead %s", g.Describe(tombs))
	}

	// Tyrael in the lair
	g.Level = LevelDurielLair
	g.Hear(NPCTyrael1, 302)

	if tombs.State != 4 || !g.get(tombs, FlagPrimaryGoal) || !g.get(tombs, FlagLeaveTown) {
		t.Fatalf("Tyrael %s", g.Describe(tombs))
	}

	moveTo(g, LevelDurielLair, LevelLutGholein)

	// the townspeople comment once
	msgs, _ = talk(g, NPCAtma)
	if !contains2(msgs, 445) || !g.get(tombs, FlagCustom2) {
		t.Fatalf("Atma 445: %v %s", msgs, g.Describe(tombs))
	}

	msgs, _ = talk(g, NPCJerhyn)
	if !contains2(msgs, 442) || tombs.State != 5 || !g.get(tombs, FlagEnterArea) || g.get(tombs, FlagLeaveTown) {
		t.Fatalf("Jerhyn 442: %v %s", msgs, g.Describe(tombs))
	}

	msgs, eff := talk(g, NPCMeshif1)
	if !contains2(msgs, 450) || !g.get(tombs, FlagRewardGranted) || g.get(tombs, FlagEnterArea) {
		t.Fatalf("Meshif: %v %s", msgs, g.Describe(tombs))
	}

	if !hasEffect(eff, EffectUnlockAct) {
		t.Errorf("no act unlock: %+v", eff)
	}
}

func contains2(l []int, n int) bool {
	for _, x := range l {
		if x == n {
			return true
		}
	}

	return false
}

func TestTravelToAct3SetsActWord(t *testing.T) {
	g, _ := newGame(t)

	eff := g.TravelToAct3()
	if !g.Rec.Get(slotAct2Finished, FlagRewardGranted) || !hasEffect(eff, EffectUnlockAct) {
		t.Fatalf("%+v", eff)
	}

	if !g.ReturnGreetingPending(NPCAtma) || !g.get(g.Quest(QuestHoradricStaff), FlagRewardGranted) {
		t.Error("welcome-back bit or the staff completion missing")
	}

	g.TravelToAct4()
	g.TravelToAct5()

	if !g.Rec.Get(slotAct3Finished, FlagRewardGranted) || !g.Rec.Get(slotAct4Finished, FlagRewardGranted) {
		t.Error("act words")
	}
}

func TestAct3Prologue(t *testing.T) {
	g, _ := newGame(t)

	moveTo(g, 1, LevelKurastDocktown)

	msgs, _ := talk(g, NPCHratli)
	if !contains2(msgs, 466) || contains2(msgs, 465) { // the test hero is a Sorceress
		t.Fatalf("Hratli: %v", msgs)
	}

	if !g.Rec.Get(16, FlagRewardGranted) {
		t.Error("prologue slot not set")
	}

	if msgs, _ = talk(g, NPCHratli); len(msgs) != 0 {
		t.Errorf("Hratli repeats: %v", msgs)
	}
}

func TestAct3QuestLine(t *testing.T) {
	g, _ := newGame(t)
	lam, khalim, blade := g.Quest(QuestLamEsen), g.Quest(QuestKhalim), g.Quest(QuestBlade)
	bird, temple, guardian := g.Quest(QuestGoldenBird), g.Quest(QuestBlackenedTemple), g.Quest(QuestGuardian)

	moveTo(g, 1, LevelKurastDocktown)

	// Lam Esen's Tome: available after Lower Kurast was entered (VERIFIED 0x5b52c0), 5 stat points at Alkor's line
	if msgs, _ := talk(g, NPCAlkor); len(msgs) != 0 {
		t.Fatalf("Alkor before Lower Kurast: %v", msgs)
	}

	moveTo(g, LevelKurastDocktown, LevelLowerKurast)
	moveTo(g, LevelLowerKurast, LevelKurastDocktown)

	msgs, _ := talk(g, NPCAlkor)
	if len(msgs) == 0 || msgs[0] != 549 || lam.State != 2 {
		t.Fatalf("Alkor: %v %s", msgs, g.Describe(lam))
	}

	moveTo(g, LevelKurastDocktown, LevelRuinedTemple)

	if lam.State != 3 {
		t.Fatalf("temple %s", g.Describe(lam))
	}

	pickup(g, ItemLamEsenTome)

	if lam.State != 4 || g.get(lam, FlagRewardPending) || g.get(lam, FlagRewardGranted) {
		t.Fatalf("tome %s", g.Describe(lam))
	}

	moveTo(g, LevelRuinedTemple, LevelKurastDocktown)

	msgs, eff := talk(g, NPCAlkor)
	if len(msgs) != 1 || msgs[0] != 564 || !g.get(lam, FlagRewardGranted) || g.get(lam, FlagRewardPending) {
		t.Fatalf("Alkor reward: %v %s", msgs, g.Describe(lam))
	}

	if !effectCode(eff, EffectReward, "stat-points") || !effectCode(eff, EffectDeleteItem, ItemLamEsenTome) {
		t.Errorf("effects %+v", eff)
	}

	// Khalim's Will: Cain offers it after the Great Marsh was entered
	if msgs, _ = talk(g, NPCCain3); len(msgs) != 0 {
		t.Fatalf("Cain before the Great Marsh: %v", msgs)
	}

	moveTo(g, LevelKurastDocktown, LevelGreatMarsh)
	moveTo(g, LevelGreatMarsh, LevelKurastDocktown)

	msgs, _ = talk(g, NPCCain3)
	if len(msgs) != 1 || msgs[0] != 543 || !g.get(khalim, FlagStarted) {
		t.Fatalf("Cain: %v %s", msgs, g.Describe(khalim))
	}

	for _, p := range []struct {
		item string
		msg  int
		bit  int
	}{{ItemKhalimEye, 545, FlagLeaveTown}, {ItemKhalimHeart, 544, FlagCustom2}, {ItemKhalimBrain, 546, FlagEnterArea},
		{ItemKhalimFlail, 547, FlagCustom1}} {
		pickup(g, p.item)

		if msgs, _ = talk(g, NPCCain3); len(msgs) != 1 || msgs[0] != p.msg || !bit(g, QuestKhalim, p.bit) {
			t.Fatalf("%s: %v %s", p.item, msgs, g.Describe(khalim))
		}
	}

	// the cube makes the Will (the engine's cube): Cain tells the hero to smash the orb (548, CUSTOM3)
	for _, c := range []string{ItemKhalimEye, ItemKhalimHeart, ItemKhalimBrain, ItemKhalimFlail} {
		g.dropItem(c)
	}

	pickup(g, ItemKhalimWill)

	if msgs, _ = talk(g, NPCCain3); len(msgs) != 1 || msgs[0] != 548 || !bit(g, QuestKhalim, FlagCustom3) {
		t.Fatalf("Cain 548: %v %s", msgs, g.Describe(khalim))
	}

	// VERIFIED 0x5b9550: the Will must be wielded, the first hit does nothing, the second completes the quest at once
	g.Dispatch(Event{Kind: EvObjectOperated, Object: ObjectCompellingOrb, Level: LevelTravincal})
	g.Dispatch(Event{Kind: EvObjectOperated, Object: ObjectCompellingOrb, Level: LevelTravincal, Wielded: "ort"})
	g.Dispatch(Event{Kind: EvObjectOperated, Object: ObjectCompellingOrb, Level: LevelTravincal, Wielded: ItemKhalimWill})

	if g.OrbSmashed() || g.get(khalim, FlagRewardGranted) {
		t.Fatalf("the first hit must do nothing: %s", g.Describe(khalim))
	}

	eff = g.Dispatch(Event{Kind: EvObjectOperated, Object: ObjectCompellingOrb, Level: LevelTravincal, Wielded: ItemKhalimWill})
	if !g.OrbSmashed() || !g.get(khalim, FlagRewardGranted) || !g.get(khalim, FlagPrimaryGoal) || g.get(khalim, FlagRewardPending) ||
		!effectCode(eff, EffectDeleteItem, ItemKhalimWill) {
		t.Fatalf("orb %s %+v", g.Describe(khalim), eff)
	}

	// The Blade of the Old Religion: Hratli offers it once the Flayer Jungle was entered
	talk(g, NPCHratli)

	if blade.State != 0 {
		t.Fatalf("blade before the Flayer Jungle %s", g.Describe(blade))
	}

	moveTo(g, LevelKurastDocktown, LevelFlayerJungle)
	moveTo(g, LevelFlayerJungle, LevelKurastDocktown)
	talk(g, NPCHratli)

	if blade.State != 2 {
		t.Fatalf("blade %s", g.Describe(blade))
	}

	moveTo(g, LevelKurastDocktown, LevelFlayerJungle)

	if blade.State != 3 {
		t.Fatalf("leaving the Docks %s", g.Describe(blade))
	}

	pickup(g, ItemGidbinn)

	if blade.State != 4 || !g.get(blade, bladeCarry) {
		t.Fatalf("gidbinn %s", g.Describe(blade))
	}

	moveTo(g, LevelFlayerJungle, LevelKurastDocktown)

	// Ormus takes the blade (587) and offers the ring at once (593); Asheara's wolves (589) may come before or after
	if msgs, eff = talk(g, NPCOrmus); !contains2(msgs, 587) || !contains2(msgs, 593) || g.Items[ItemGidbinn] != 0 {
		t.Fatalf("Ormus %v %s", msgs, g.Describe(blade))
	}

	var ring *Effect

	for i := range eff {
		if eff[i].Kind == EffectGiveItem && eff[i].Code == ItemReward {
			ring = &eff[i]
		}
	}

	if ring == nil || ring.Quality != 2 || ring.Value != 21 {
		t.Fatalf("Ormus's ring (rare, item level 21 in Normal): %+v", eff)
	}

	if g.get(blade, FlagRewardGranted) {
		t.Fatalf("the Blade needs the wolves too: %s", g.Describe(blade))
	}

	_, eff = talk(g, NPCAsheara)
	if !g.get(blade, FlagRewardGranted) || !effectCode(eff, EffectReward, "hire-ironwolves") {
		t.Fatalf("Asheara %s %+v", g.Describe(blade), eff)
	}

	// The Golden Bird starts with the figurine
	if bird.State != 0 {
		t.Fatalf("bird early %s", g.Describe(bird))
	}

	pickup(g, ItemJadeFigurine)
	talk(g, NPCCain3)
	_, eff = talk(g, NPCMeshif2)

	if bird.State != 3 || !effectCode(eff, EffectSpawn, ItemGoldenBird) {
		t.Fatalf("Meshif %s %+v", g.Describe(bird), eff)
	}

	// Alkor takes the bird (534) and pays out (538) in one talk
	msgs, eff = talk(g, NPCAlkor)
	if !contains2(msgs, 534) || !contains2(msgs, 538) || !g.get(bird, FlagRewardGranted) || !effectCode(eff, EffectSpawn, ItemPotionOfLife) || effectCode(eff, EffectReward, "life-boost") {
		t.Fatalf("bird reward %s %+v", g.Describe(bird), eff)
	}

	if eff = g.DrinkPotionOfLife(); !effectCode(eff, EffectReward, "life-boost") || g.DrinkPotionOfLife() != nil {
		t.Fatalf("potion %+v", eff)
	}

	// The Blackened Temple: Ormus offers it once the Kurast Causeway was entered; it has no reward step
	if temple.State != 0 || guardian.State != 0 {
		t.Fatalf("chain %d %d", temple.State, guardian.State)
	}

	moveTo(g, LevelKurastDocktown, LevelKurastCauseway)
	moveTo(g, LevelKurastCauseway, LevelKurastDocktown)

	if temple.State != 1 {
		t.Fatalf("temple not offered: %s", g.Describe(temple))
	}

	talk(g, NPCOrmus)
	moveTo(g, LevelKurastDocktown, LevelTravincal)

	for _, c := range []int{NPCCouncilA, NPCCouncilB} {
		kill(g, c, LevelTravincal)
	}

	if g.get(temple, FlagPrimaryGoal) {
		t.Fatal("goal before the third council member")
	}

	kill(g, NPCCouncilC, LevelTravincal)

	// the orb is already smashed: the quest is done at once, no reward pending (VERIFIED 0x5b97d0 + D2MOO)
	if !g.get(temple, FlagRewardGranted) || g.get(temple, FlagRewardPending) || guardian.State != 1 {
		t.Fatalf("council %s; guardian %d", g.Describe(temple), guardian.State)
	}

	talk(g, NPCOrmus)
	moveTo(g, LevelKurastDocktown, LevelDurance1)
	moveTo(g, LevelDurance1, LevelDurance3)
	kill(g, NPCMephisto, LevelDurance3)

	// exe kill bits: done at once (done + primary goal), never reward pending
	if g.get(guardian, FlagRewardPending) || !g.get(guardian, FlagRewardGranted) || !g.get(guardian, FlagPrimaryGoal) {
		t.Fatalf("mephisto %s", g.Describe(guardian))
	}

	moveTo(g, LevelDurance3, LevelKurastDocktown)
	talk(g, NPCAlkor)

	if !g.get(guardian, FlagRewardGranted) {
		t.Fatalf("guardian reward %s", g.Describe(guardian))
	}
}

func TestAct4QuestLine(t *testing.T) {
	g, _ := newGame(t)
	fa, te, hf := g.Quest(QuestFallenAngel), g.Quest(QuestTerrorsEnd), g.Quest(QuestHellforge)

	moveTo(g, 1, LevelPandemonium)

	// Tyrael welcomes the hero (664) and offers both quests in one talk
	if msgs, _ := talk(g, NPCTyrael2); !contains2(msgs, 664) || !contains2(msgs, 670) || !contains2(msgs, 681) {
		t.Fatalf("prologue %v", msgs)
	}

	if fa.State != 2 || !g.get(fa, FlagStarted) {
		t.Fatalf("Tyrael %s", g.Describe(fa))
	}

	moveTo(g, LevelPandemonium, LevelPlainsDespair)
	kill(g, NPCIzual, LevelPlainsDespair)

	if !g.get(fa, FlagRewardPending) {
		t.Fatalf("Izual %s", g.Describe(fa))
	}

	moveTo(g, LevelPlainsDespair, LevelPandemonium)

	_, eff := talk(g, NPCTyrael2)
	if !g.get(fa, FlagRewardGranted) {
		t.Fatalf("reward %s", g.Describe(fa))
	}

	skill := 0

	for _, e := range eff {
		if e.Kind == EffectSkillPoint {
			skill += e.Value
		}
	}

	if skill != 2 {
		t.Errorf("skill points %d", skill)
	}

	// Terror's End: the expansion lines
	if te.State < 2 {
		t.Fatalf("terror %s", g.Describe(te))
	}

	moveTo(g, LevelPandemonium, LevelChaosSanctum)
	kill(g, NPCDiablo, LevelChaosSanctum)
	moveTo(g, LevelChaosSanctum, LevelPandemonium)

	// exe kill bits: the kill completes the quest (no reward talk)
	if g.get(te, FlagRewardPending) || !g.get(te, FlagRewardGranted) || !g.get(te, FlagPrimaryGoal) {
		t.Fatalf("diablo: %s", g.Describe(te))
	}

	talk(g, NPCTyrael2)

	// the Hellforge: Cain's first line depends on the soulstone
	if l := g.Activate(NPCCain4).Lines; len(l) == 0 || l[0].Msg != 679 {
		t.Fatalf("Cain without stone: %+v", l)
	}

	pickup(g, ItemMephistoSoulstone)

	if l := g.Activate(NPCCain4).Lines; len(l) == 0 || l[0].Msg != 678 {
		t.Fatalf("Cain with stone: %+v", l)
	}

	if l := g.Activate(NPCMalachai).Lines; len(l) != 1 || l[0].Msg != 668 {
		t.Errorf("Malachai %+v", l)
	}

	talk(g, NPCCain4)

	// without the hammer the forge does nothing
	obj(g, ObjectHellforge, LevelRiverOfFlame)

	if g.get(hf, FlagRewardPending) {
		t.Fatal("forge worked without the hammer")
	}

	pickup(g, ItemHellforgeHammer)

	eff = obj(g, ObjectHellforge, LevelRiverOfFlame)
	if !g.get(hf, FlagRewardPending) || !effectCode(eff, EffectDeleteItem, ItemMephistoSoulstone) {
		t.Fatalf("forge %s %+v", g.Describe(hf), eff)
	}

	talk(g, NPCCain4)

	if !g.get(hf, FlagRewardGranted) {
		t.Fatalf("Cain reward %s", g.Describe(hf))
	}

	if l := g.Activate(NPCMalachai).Lines; len(l) != 1 || l[0].Msg != 669 {
		t.Errorf("Malachai after %+v", l)
	}
}

func giveCodes(effects []Effect) []string {
	var out []string

	for _, e := range effects {
		if e.Kind == EffectGiveItem {
			out = append(out, e.Code)
		}
	}

	return out
}

func TestAct5QuestLine(t *testing.T) {
	g, _ := newGame(t)

	moveTo(g, 1, LevelHarrogath)

	// Siege on Harrogath: the thanks (20090) leave the reward pending; Larzuk's sockets complete it
	siege := g.Quest(QuestSiege)

	if msgs, _ := talk(g, NPCLarzuk); !contains2(msgs, 20077) || siege.State != 2 {
		t.Fatalf("Larzuk: %v %s", msgs, g.Describe(siege))
	}

	moveTo(g, LevelHarrogath, LevelBloodyFoothills)

	if siege.State != 3 || !g.get(siege, FlagEnterArea) {
		t.Fatalf("foothills %s", g.Describe(siege))
	}

	g.Dispatch(Event{Kind: EvMonsterKilled, Monster: 999, Super: "Shenk the Overseer", Level: LevelBloodyFoothills})

	if !g.get(siege, FlagRewardPending) || !g.SocketOwed() {
		t.Fatalf("Shenk %s", g.Describe(siege))
	}

	moveTo(g, LevelBloodyFoothills, LevelHarrogath)

	if !g.Marker(NPCLarzuk) {
		t.Error("no marker on Larzuk")
	}

	_, eff := talk(g, NPCLarzuk)
	if g.get(siege, FlagRewardGranted) || !g.get(siege, FlagRewardPending) || !g.get(siege, FlagCustom1) || g.get(siege, FlagStarted) ||
		!g.SocketOwed() {
		t.Fatalf("Larzuk's thanks must keep the reward pending (VERIFIED 0x584d60): %s", g.Describe(siege))
	}

	if !effectCode(eff, EffectReward, "socket-quest") {
		t.Errorf("no socket notice: %+v", eff)
	}

	// VERIFIED 0x5856a0: the item action sets bit 0 and clears bit 1
	g.ClaimSocket()

	if !g.get(siege, FlagRewardGranted) || g.get(siege, FlagRewardPending) || g.SocketOwed() {
		t.Fatalf("sockets added %s", g.Describe(siege))
	}

	if g.ClaimSocket() != nil {
		t.Error("a second socketing completed the quest again")
	}

	// Rescue on Mount Arreat: three doors, runes by the number of freed barbarians
	rescue := g.Quest(QuestRescue)
	talk(g, NPCQualKehk)
	moveTo(g, LevelHarrogath, LevelFrigidHighlands)

	if rescue.State != 3 || !g.get(rescue, FlagEnterArea) {
		t.Fatalf("highlands %s", g.Describe(rescue))
	}

	kill(g, NPCPrisonDoor, LevelFrigidHighlands)
	kill(g, NPCPrisonDoor, LevelFrigidHighlands)

	if g.get(rescue, FlagRewardPending) {
		t.Fatal("goal before the third door")
	}

	kill(g, NPCPrisonDoor, LevelFrigidHighlands)

	if !g.get(rescue, FlagRewardPending) || !g.get(rescue, FlagCustom1) {
		t.Fatalf("doors %s", g.Describe(rescue))
	}

	moveTo(g, LevelFrigidHighlands, LevelHarrogath)

	_, eff = talk(g, NPCQualKehk)
	if !g.get(rescue, FlagRewardGranted) || g.get(rescue, FlagRewardPending) {
		t.Fatalf("Qual-Kehk %s", g.Describe(rescue))
	}

	if got := giveCodes(eff); len(got) != 3 || got[0] != "r07" || got[1] != "r08" || got[2] != "r09" {
		t.Errorf("15 freed barbarians pay Tal, Ral and Ort: %v", got)
	}

	if effectCode(eff, EffectReward, "hire-barbarians") {
		t.Error("Qual-Kehk's reward is not a mercenary unlock")
	}

	// Prison of Ice
	prison := g.Quest(QuestPrison)

	if msgs, _ := talk(g, NPCMalah); !contains2(msgs, 20116) || prison.State != 2 {
		t.Fatalf("Malah: %v %s", msgs, g.Describe(prison))
	}

	moveTo(g, LevelHarrogath, LevelFrozenRiver)

	if prison.State != 3 {
		t.Fatalf("river %s", g.Describe(prison))
	}

	if l := g.Activate(NPCAnyaFrozen).Lines; len(l) != 1 || l[0].Msg != 20131 {
		t.Fatalf("frozen Anya %+v", l)
	}

	talk(g, NPCAnyaFrozen)

	if prison.State != 4 {
		t.Fatalf("20131 %s", g.Describe(prison))
	}

	// Malah gives the potion; without it the block does nothing
	g.Dispatch(Event{Kind: EvObjectOperated, Object: ObjectFrozenAnya, Level: LevelFrozenRiver})

	if prison.State != 4 || g.get(prison, FlagRewardPending) {
		t.Fatalf("the block melted without the potion: %s", g.Describe(prison))
	}

	moveTo(g, LevelFrozenRiver, LevelHarrogath)

	_, eff = talk(g, NPCMalah)
	if got := giveCodes(eff); len(got) != 1 || got[0] != ItemDefrostPotion {
		t.Fatalf("Malah's potion: %v", got)
	}

	pickup(g, ItemDefrostPotion)
	moveTo(g, LevelHarrogath, LevelFrozenRiver)

	eff = g.Dispatch(Event{Kind: EvObjectOperated, Object: ObjectFrozenAnya, Level: LevelFrozenRiver})
	if prison.State != 5 || !g.get(prison, FlagRewardPending) || !g.get(prison, FlagPrimaryGoal) ||
		!effectCode(eff, EffectDeleteItem, ItemDefrostPotion) {
		t.Fatalf("thawing %s %+v", g.Describe(prison), eff)
	}

	moveTo(g, LevelFrozenRiver, LevelHarrogath)

	// Malah's scroll (20132) and Anya's item (20136) in either order; the quest is rewarded after both
	_, eff = talk(g, NPCMalah)
	if got := giveCodes(eff); len(got) != 1 || got[0] != ItemMalahScroll || !g.get(prison, FlagCustom4) || g.get(prison, FlagRewardGranted) {
		t.Fatalf("Malah's scroll: %v %s", got, g.Describe(prison))
	}

	_, eff = talk(g, NPCDrehya)
	if got := giveCodes(eff); len(got) != 1 || got[0][:2] != "ob" || !g.get(prison, FlagRewardGranted) || g.get(prison, FlagRewardPending) {
		t.Fatalf("Anya's item (a sorceress orb): %v %s", got, g.Describe(prison))
	}

	// Betrayal of Harrogath: Anya's first line came in the same talk and opened the portal to the temple
	betrayal := g.Quest(QuestBetrayal)
	if betrayal.State != 2 || !hasEffectPortal(eff, LevelNihlathakTemple) {
		t.Fatalf("Anya %s %+v", g.Describe(betrayal), eff)
	}

	if eff = g.ReadScrollOfResistance(); !effectCode(eff, EffectReward, "resist-bonus") || g.ResistBonus() != 10 {
		t.Fatalf("scroll %+v", eff)
	}

	moveTo(g, LevelHarrogath, LevelNihlathakTemple)
	kill(g, NPCNihlathakBoss, LevelNihlathakTemple)

	if !g.get(betrayal, FlagRewardPending) || !g.PersonalizeOwed() {
		t.Fatalf("Nihlathak %s", g.Describe(betrayal))
	}

	moveTo(g, LevelNihlathakTemple, LevelHarrogath)
	talk(g, NPCDrehya)

	if g.get(betrayal, FlagRewardGranted) || !g.PersonalizeOwed() || !g.get(betrayal, FlagEnterArea) {
		t.Fatalf("Anya's thanks pay nothing yet: %s", g.Describe(betrayal))
	}

	g.ClaimPersonalize()

	if !g.get(betrayal, FlagRewardGranted) || g.get(betrayal, FlagRewardPending) {
		t.Fatalf("personalised %s", g.Describe(betrayal))
	}

	// Rite of Passage: the hero must be level 20 in Normal; the last Ancient pays experience at once
	rite := g.Quest(QuestRite)
	talk(g, NPCQualKehk)
	moveTo(g, LevelHarrogath, LevelArreatSummit)

	for _, a := range []int{NPCAncient1, NPCAncient2, NPCAncient3} {
		kill(g, a, LevelArreatSummit)
	}

	if g.get(rite, FlagRewardGranted) {
		t.Fatalf("a level 10 hero finished the Rite: %s", g.Describe(rite))
	}

	g2, _ := newGame(t)
	g2.Hero.Level = 20
	moveTo(g2, 1, LevelHarrogath)
	talk(g2, NPCQualKehk)
	moveTo(g2, LevelHarrogath, LevelArreatSummit)
	kill(g2, NPCAncient1, LevelArreatSummit)
	kill(g2, NPCAncient2, LevelArreatSummit)

	eff = kill(g2, NPCAncient3, LevelArreatSummit)
	r2 := g2.Quest(QuestRite)

	if !g2.get(r2, FlagRewardGranted) || g2.get(r2, FlagRewardPending) || !g2.get(r2, FlagPrimaryGoal) {
		t.Fatalf("rite %s", g2.Describe(r2))
	}

	var exp int

	for _, e := range eff {
		if e.Kind == EffectReward && e.Code == "experience" {
			exp = e.Value
		}
	}

	if exp != 1400000 || RiteExperience(Nightmare) != 20000000 || RiteExperience(Hell) != 40000000 {
		t.Errorf("experience %d", exp)
	}

	if RiteLevelGate(Normal) != 20 || RiteLevelGate(Nightmare) != 40 || RiteLevelGate(Hell) != 60 {
		t.Error("level gates")
	}

	// Eve of Destruction opened with the Rite of Passage
	eve := g2.Quest(QuestEve)
	if eve.State != 1 {
		t.Fatalf("eve %s", g2.Describe(eve))
	}

	moveTo(g2, LevelArreatSummit, LevelWorldstone1)
	moveTo(g2, LevelWorldstone1, LevelThrone)
	// Baal's death counts only in the Worldstone Chamber (exe 0x58bae0); in the throne room it changes nothing
	if eff := kill(g2, NPCBaalCrab, LevelThrone); g2.get(eve, FlagRewardGranted) || effectCode(eff, EffectReward, "unlock-difficulty") {
		t.Fatalf("baal killed in the throne room: %s", g2.Describe(eve))
	}

	moveTo(g2, LevelThrone, LevelWorldstoneChamber)

	eff = kill(g2, NPCBaalCrab, LevelWorldstoneChamber)
	if g2.get(eve, FlagRewardPending) || !g2.get(eve, FlagRewardGranted) || !g2.get(eve, FlagPrimaryGoal) ||
		!effectCode(eff, EffectReward, "unlock-difficulty") {
		t.Fatalf("baal %s %+v", g2.Describe(eve), eff)
	}

	moveTo(g2, LevelWorldstoneChamber, LevelHarrogath)
	talk(g2, NPCTyrael3)
}

func hasEffectPortal(effects []Effect, level int) bool {
	for _, e := range effects {
		if e.Kind == EffectPortal && e.Value == level {
			return true
		}
	}

	return false
}

// Rescue on Mount Arreat: the reward follows the number of freed barbarians (D2MOO ACT5Q2_Callback11_ScrollMessage);
// fewer than twelve leave the goal open.
func TestRescueRunesFollowTheFreedBarbarians(t *testing.T) {
	for _, c := range []struct {
		freed   [3]int
		runes   int
		reached bool
	}{{[3]int{5, 5, 5}, 3, true}, {[3]int{5, 5, 4}, 2, true}, {[3]int{5, 4, 3}, 1, true}, {[3]int{4, 4, 4}, 1, true}, {[3]int{5, 4, 2}, 0, false}} {
		g, _ := newGame(t)
		q := g.Quest(QuestRescue)

		moveTo(g, 1, LevelHarrogath)
		talk(g, NPCQualKehk)
		moveTo(g, LevelHarrogath, LevelFrigidHighlands)

		for _, n := range c.freed {
			g.Dispatch(Event{Kind: EvMonsterKilled, Monster: NPCPrisonDoor, Level: LevelFrigidHighlands, Freed: n})
		}

		if g.get(q, FlagRewardPending) != c.reached {
			t.Fatalf("%v: %s", c.freed, g.Describe(q))
		}

		if !c.reached {
			continue
		}

		moveTo(g, LevelFrigidHighlands, LevelHarrogath)

		_, eff := talk(g, NPCQualKehk)
		if got := giveCodes(eff); len(got) != c.runes {
			t.Errorf("%v: %d runes, want %d", c.freed, len(got), c.runes)
		}
	}
}

// Anya's reward is a rare item of the hero's class; the tier follows the difficulty and the hero level.
func TestAnyaRewardTiers(t *testing.T) {
	if got := AnyaRewardCodes(ClassAmazon, Normal, 99); got[0] != "am1" {
		t.Errorf("normal amazon %v", got)
	}

	if got := AnyaRewardCodes(ClassBarbarian, Nightmare, 46); got[0] != "ba6" || AnyaRewardCodes(ClassBarbarian, Nightmare, 45)[0] != "ba1" {
		t.Errorf("nightmare barbarian tier: %v", got)
	}

	if got := AnyaRewardCodes(ClassAssassin, Hell, 66); len(got) != 7 || got[0] != "7ar" || AnyaRewardCodes(ClassAssassin, Hell, 65)[0] != "ktr" {
		t.Errorf("hell assassin tier: %v", got)
	}

	if AnyaRewardCodes(9, Normal, 10) != nil {
		t.Error("unknown class")
	}
}

// Betrayal of Harrogath: Nihlathak's death pays only with Prison of Ice done or reward pending.
func TestBetrayalNeedsThePrison(t *testing.T) {
	g, _ := newGame(t)
	q := g.Quest(QuestBetrayal)

	moveTo(g, 1, LevelHarrogath)
	talk(g, NPCDrehya)
	moveTo(g, LevelHarrogath, LevelNihlathakTemple)
	kill(g, NPCNihlathakBoss, LevelNihlathakTemple)

	if g.get(q, FlagRewardPending) {
		t.Fatalf("Nihlathak counted without the Prison: %s", g.Describe(q))
	}
}

func TestLaterActQuestsRestoreFromBits(t *testing.T) {
	g, body := newGame(t)

	moveTo(g, 1, LevelHarrogath)
	talk(g, NPCLarzuk)
	moveTo(g, LevelHarrogath, LevelBloodyFoothills)

	// a new game from the same record restores the state from the bits
	g2 := New(body.QuestRecord(Normal), body.NPCFlags(), Normal)
	g2.Hero = Hero{Class: ClassSorceress, Level: 10}
	g2.Start()

	q := g2.Quest(QuestSiege)
	if q.State != 3 || q.LastState != 2 {
		t.Fatalf("restored %s", g2.Describe(q))
	}

	// a completed quest is inert and opens its successor
	g2.Rec.Set(g2.Quest(QuestRite).Slot, FlagRewardGranted)

	g3 := New(body.QuestRecord(Normal), body.NPCFlags(), Normal)
	g3.Start()

	if g3.Quest(QuestRite).NotIntro || g3.Quest(QuestEve).State != 1 {
		t.Errorf("rite %v eve %d", g3.Quest(QuestRite).NotIntro, g3.Quest(QuestEve).State)
	}
}

func TestLogCoversAllActs(t *testing.T) {
	g, _ := newGame(t)

	perAct := map[int]int{}

	for _, l := range g.Log() {
		perAct[l.Act]++
	}

	for act, want := range map[int]int{1: 6, 2: 6, 3: 6, 4: 3, 5: 6} {
		if perAct[act] != want {
			t.Errorf("act %d has %d log entries, want %d", act, perAct[act], want)
		}
	}
}
