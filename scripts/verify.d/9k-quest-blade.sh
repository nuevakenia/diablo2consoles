scenario_name="Quest walkthrough, Act 3: The Blade of the Old Religion in the real world (Hratli, Flayer Jungle, Gidbinn, Flayer Dungeon, Ormus, Asheara, reward, quest log, .d2s)"
# The sample hero (quests reset) hears nothing about the quest from Hratli before the Flayer Jungle (exe 0x5b6ea0: his
# line 571 is only heard once the jungle was entered), enters the jungle by waypoint (quest available), returns to
# Kurast and takes Hratli's quest line, goes back (state 3), picks up Gidbinn, walks into the Flayer Dungeon and back,
# hands the blade to Ormus (587, takes it; ring 593), talks to Asheara (589, Iron Wolves) and gets the reward
# (rare ring from Ormus, both done = complete, in either order). Needs D2_TABLES + D2S_SAMPLE_BODY.
# Gidbinn is not on the altar: operating the object wakes the fetish boss, whose death drops the blade (D2MOO ACT3Q3).
source scripts/quest_walk_lib.zsh
scenario_warnings_ok=1
scenario_env() {
  local s="$(qw_begin 3)say:setwaypoint 78 1;say:setwaypoint 75 1;$(qw_panel 3 3)$(qw_talk Hratli)$(qw_panel 3 3)"
  # Gidbinn first (the hero arrives by the waypoint; a walk from the altar back to the waypoint finds no way in this
  # generated jungle, see docs/PLAYTEST.md), then out of the Flayer Dungeon hole and back to the waypoint from there
  # first visit of the jungle (the quest becomes available), back by portal, Hratli's line, and in again
  s+="wait:11;$(qw_wp 78)wait:2;say:spawnportal 75;use:Portal;expect:level=75;wait:3;$(qw_talk Hratli)$(qw_panel 3 3)$(qw_wp 78)"
  # (operating the Gidbinn object wakes the fetish boss; its death drops the blade where it fell, D2MOO ACT3Q3)
  s+="kill:all,100;$(qw_chest 252 4)say:killgidbinn;wait:2;say:lootquest 15 30;wait:2;say:lootquest 15 30;$(qw_panel 3 3)$(qw_go 88)$(qw_go 78)wait:2;say:spawnportal 75;use:Portal;expect:level=75;wait:3;"
  # (the way back to Kurast is the act travel: the Flayer Dungeon hole leaves the hero at a spot of the generated jungle
  # from which no walk reaches the waypoint on every build; verify runs failed at 'use:Waypoint' with 30+ tiles to go)
  s+="$(qw_talk Ormus 10)$(qw_panel 3 3)$(qw_talk Asheara 10)$(qw_panel 3 3)$(qw_talk Ormus 10)$(qw_panel 3 3)exit"
  qw_env blade "$s"
}
scenario_check() {
  grep -E "AUTOSCRIPT RESULT|QUEST A3Q3|QUEST SPEECH npc=\"(Hratli|Ormus|Asheara)\"|QUESTPANEL act=3 quest=3|quest object|QUEST EFFECT|ACT3 Gidbinn" $log.txt | grep -v "log-update" | cut -c1-200
  qw_sample blade || { echo "SKIP: no revived sample hero in this run"; return; }
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: the Blade of the Old Religion walkthrough did not pass"; fail=1; }
  # the NPC lines with message ids and Sounds.txt rows: Hratli's quest line, Ormus (takes the blade) and Asheara
  for m in 571 587 589 593; do
    grep -qE "QUEST SPEECH npc=\"[A-Za-z]+\" class=[0-9]+ msg=$m mode=[0-9] .* sound=[0-9]+ handle=ESOUND_" $log.txt || { echo "FAIL: message $m was not spoken"; fail=1; }
  done
  grep -q "LEVEL quest object id=252 name=\"Gidbinn\"" $log.txt || { echo "FAIL: no Gidbinn in the Flayer Jungle"; fail=1; }
  grep -qE "QUEST object operated id=252|ACT3 .* operated \(id 252\)" $log.txt || { echo "FAIL: Gidbinn was not operated"; fail=1; }
  grep -q "QUEST EFFECT spawn-boss code=boss:fetish11" $log.txt || { echo "FAIL: the Gidbinn object did not wake the fetish boss"; fail=1; }
  grep -q "ACT3 Gidbinn's fetish boss wakes" $log.txt || { echo "FAIL: the fetish boss was not spawned"; fail=1; }
  grep -q "QUEST EFFECT spawn-drop code=g33" $log.txt || { echo "FAIL: the boss's death did not drop the blade"; fail=1; }
  grep -q 'LOOT stored "g33"' $log.txt || { echo "FAIL: Gidbinn was not picked up"; fail=1; }
  grep -q "QUEST EFFECT delete-item code=g33" $log.txt || { echo "FAIL: Ormus kept no blade"; fail=1; }
  grep -qE "QUEST EFFECT give-item code=rin quality=2 ilvl/count=(21|35|75) " $log.txt || { echo "FAIL: Ormus gave no rare ring (ilvl 21, 35 or 75 by difficulty)"; fail=1; }
  grep -q "QUEST EFFECT reward hire-ironwolves" $log.txt || { echo "FAIL: Asheara's reward (Iron Wolves)"; fail=1; }
  for t in "0->1" "1->2" "2->3" "3->4" "4->5"; do
    grep -q "QUEST A3Q3 state $t" $log.txt || { echo "FAIL: no state change $t of A3Q3"; fail=1; }
  done
  # the quest log: no page before Hratli, page text on the way, the completed text after the reward
  grep -q 'QUESTPANEL act=3 quest=3 status=0 .*text=""' $log.txt || { echo "FAIL: panel before the quest"; fail=1; }
  # (pages of the exe log-state function 0x5b6940: 2 after Hratli, 4 while carrying Gidbinn, 5 after Ormus took it with the
  # wolves open, 6 with the wolves granted and the ring open; pages 1 and 3 need flags this walk does not set)
  for pg in "2 .*Pick up the Gidbinn" "4 .*Talk to Asheara" "5 .*Talk to Ormus" "6 .*text=\"\""; do
    grep -qE "QUESTPANEL act=3 quest=3 status=$pg" $log.txt || { echo "FAIL: quest log page ${pg%% *} text of the Blade quest"; fail=1; }
  done
  grep -q "QUESTPANEL act=3 quest=3 status=-1 .*completed" $log.txt || { echo "FAIL: panel completed text"; fail=1; }
  qw_laterquests
  qw_slot_has 19 0x1001 || { echo "FAIL: quest slot 19 is not saved as done in the .d2s"; fail=1; }
  grep -E "AUTOSCRIPT step [0-9]+ FAIL" $log.txt | cut -c1-200
}
