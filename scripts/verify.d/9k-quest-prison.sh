scenario_name="Quest walkthrough, Act 5: Prison of Ice in the real world (Siege and Rescue done, Malah, the frozen Anya block of the Frozen River, Malah's potion, quest log)"
# The sample hero (quests reset) finds the Prison of Ice closed until Rescue on Mount Arreat is done (the Act 5 quests
# open in order, exe sequence filters), takes Malah's quest, goes to the Frozen River by the Crystalline Passage, operates the frozen
# Anya block without the potion (her hint, message 20131), fetches Malah's potion and thaws her with it (the quest
# reaches its goal, reward pending). Needs D2_TABLES + D2S_SAMPLE_BODY.
# (Malah's potion falls to the ground in town, which the loot walk skips: the script puts one in the pack, as a pick-up would)
# UNVERIFIED: where the block stands (the Drehya marker 460 of the level, else a block of the level's own).
source scripts/quest_walk_lib.zsh
scenario_warnings_ok=1
# (the block stands far from the entry of the level; a walk to an object gives up after 12 seconds, so it is repeated)
qw_block() { printf 'walkto:object=558;wait:1;walkto:object=558;wait:1;walkto:object=558;wait:1;walkto:object=558;wait:3;'; }
scenario_env() {
  # (Malah's offer before the chain reaches her quest: nothing; then Siege and Rescue are marked done)
  local s="$(qw_begin 5)$(qw_talk Malah)$(qw_panel 5 3)say:completequest 5 1;say:completequest 5 2;$(qw_talk Malah)$(qw_panel 5 3)"
  # (the Frozen River is no waypoint level: the hero goes by the Crystalline Passage and its exit)
  s+="say:setwaypoint 113 1;wait:11;$(qw_wp 113)$(qw_go 114)$(qw_panel 5 3)$(qw_block)$(qw_panel 5 3)"
  s+="$(qw_go 113)$(qw_wp 109)$(qw_talk Malah)say:giveitemq ice 0;$(qw_panel 5 3)$(qw_wp 113)$(qw_go 114)$(qw_block)$(qw_panel 5 3)exit"
  qw_env prison "$s"
}
scenario_check() {
  grep -E "AUTOSCRIPT RESULT|ANYA|QUEST A5Q3|QUEST SPEECH npc=\"(Malah|Anya)\"|QUESTPANEL act=5 quest=3|quest object|QUEST EFFECT" $log.txt | grep -v "log-update" | cut -c1-200
  qw_sample prison || { echo "SKIP: no revived sample hero in this run"; return; }
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: the Prison of Ice walkthrough did not pass"; fail=1; }
  # the order: no Malah quest line before Rescue is done
  grep -q "QUEST SPEECH npc=\"Malah\" class=513 msg=20116 " $log.txt || { echo "FAIL: Malah did not offer the quest after Rescue was done"; fail=1; }
  # the world: the frozen block stands in the Frozen River (own block or placed at the marker)
  grep -qE "ANYA (frozen Anya placed at the marker|the level carries [0-9]+ frozen Anya block)" $log.txt || { echo "FAIL: no frozen Anya in the Frozen River"; fail=1; }
  grep -q "ANYA the frozen block is operated without Malah's potion" $log.txt || { echo "FAIL: the block did not answer without the potion"; fail=1; }
  grep -qE "QUEST SPEECH npc=\"[A-Za-z0-9 ]*\" class=527 msg=20131 " $log.txt || { echo "FAIL: no hint (20131) from the frozen Anya"; fail=1; }
  grep -qE "QUEST SPEECH npc=\"Malah\" class=513 msg=20127 " $log.txt || { echo "FAIL: Malah did not give the potion"; fail=1; }
  grep -q "QUEST object operated id=558" $log.txt || { echo "FAIL: the block was not operated with the potion"; fail=1; }
  grep -q "QUEST EFFECT delete-item code=ice " $log.txt || { echo "FAIL: the potion was not used up"; fail=1; }
  grep -q "QUEST A5Q3 state 4->5" $log.txt || { echo "FAIL: thawing Anya did not move the quest to its goal"; fail=1; }
  qw_laterquests
  grep -E "AUTOSCRIPT step [0-9]+ FAIL" $log.txt | cut -c1-200
}
