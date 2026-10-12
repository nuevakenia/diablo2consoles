scenario_name="Full game chain (a fresh level 1 Sorceress plays Act 1 to Baal in Normal by kill experience and real quest triggers; FG_DIFFS='0 1 2' goes on through Nightmare and Hell; the .d2s of each segment is the only state handed on)"
# Wave 43. A new hero (OD2_AUTONEWCHAR, starter kit, level 1) plays the main line of every act and the quests on its way
# (Den of Evil, Blood Raven, Radament, the Horadric Staff, the Summoner, Duriel, Khalim's Will, Mephisto, Izual, Diablo, the
# Siege, Rescue, Prison of Ice, Betrayal, the Ancients, Baal) as a sequence of game launches (segments, scripts/verify.d/lib/
# fullgame*.sh): each starts from the .d2s the one before exported, in a fresh config folder, so quests, waypoints, difficulty
# and act all travel through the save file. The levels come from the DRLG of the hero's map seed (routes that depend on the seed
# are planned by scripts/mapinfo.go). Test aids, named in lib/fullgame_scripts.sh: OD2_AUTOSCRIPT_HEAL=2 (life and mana topped up,
# the last life point cannot be taken: the engine has no regeneration and the hero wears the starter kit), OD2_AUTOBUILD (the
# points of a level-up are spent by a fixed plan), OD2_DURABILITY_CHANCE=1, and the console commands dumpjunk, cubetake, padsto,
# giveitemq (the item a reward NPC works on). FG_DIFFS (default "0") selects the difficulties; the next difficulty is entered
# through the difficulty screen (OD2_AUTODIFFICULTY), which refuses a difficulty the save has not unlocked.
# Notes: ~/git/d2-re-notes/w43-fullgame.md. Run alone: OD2_MAX_GAMES=3 OD2_VERIFY_ONLY=9w-fullgame scripts/verify.sh
scenario_timeout=300
scenario_turbo=1
scenario_warnings_ok=1
source scripts/verify.d/lib/fullgame.sh
source scripts/verify.d/lib/fullgame_scripts.sh
source scripts/verify.d/lib/fullgame_chain.sh
scenario_env() {
  fg_init $tmp/fullgame
  FG_BUILD='vit=2,ene=1,str=1,dex=1|Fire Bolt!:1,Fire Ball!:20,Fire Bolt:20'
  fg_ok=1
  if ! fg_newchar Sorceress Playtest; then fg_ok=0; fi
  local d
  for d in ${=FG_DIFFS:-0}; do
    [ $fg_ok = 1 ] || break
    FG_EXTRA=""; [ $d != 0 ] && FG_EXTRA="OD2_AUTODIFFICULTY=$d"
    fg_diff=$d
    fg_chain a1 a5d || fg_ok=0
    # the save after Baal must unlock the next difficulty
    cp $fg_d2s $tmp/fullgame/after-diff$d.d2s
    fg_last_d2s[$d]=$fg_d2s
  done
  echo "# the chain ran inside scenario_env (OD2_AUTOSCRIPT below only ends the verify.sh game)"
  echo "export OD2_AUTOSCRIPT='wait:1;exit'"
}
typeset -A fg_last_d2s
scenario_check() {
  [ -s $fg_all ] || { echo "FAIL: the chain left no log"; fail=1; return; }
  grep -E "^FG SEGMENT|FG SEGMENT .* failed" $fg_all | tail -3
  [ "$fg_ok" = 1 ] || { echo "FAIL: a segment died without a .d2s: ${fg_chain_fail}"; fail=1; }
  [ -z "${fg_chain_fail// /}" ] || { echo "FAIL: segments with failed script steps:$fg_chain_fail"; fail=1; }
  grep -E "^panic" $fg_all | head -3 && { echo "FAIL: a game panicked"; fail=1; }
  # the quests that decide the main line (Normal)
  local q
  for q in "A1Q1 slot=1 .*quest complete" "A1Q2 slot=2 .*Blood Raven killed" "A1Q6 slot=6 .*Andariel killed" "A2Q1 slot=9 .*reward taken" \
           "A2Q2 slot=10 .*Horadric Staff assembled" "A2Q5 slot=13 .*goal reached" "A2Q6 slot=14 .*Duriel killed" "A3Q2 slot=18 .*Compelling Orb is smashed" \
           "A3Q6 slot=22 .*" "A4Q1 slot=25 .*goal reached" "A4Q2 slot=26 .*goal reached" "A5Q1 slot=35 .*goal reached" "A5Q4 slot=38 .*goal reached" \
           "A5Q5 slot=39 .*goal reached" "A5Q6 slot=40 .*goal reached"; do
    grep -qE "QUEST $q" $fg_all || { echo "FAIL: no quest line '$q'"; fail=1; }
  done
  # act transitions: Warriv, Meshif, Mephisto's portal, Tyrael
  for q in "from=1 to=40 .*via=act:npc" "from=40 to=75 .*via=act:npc" "to=103 .*actChange=true" "to=109 .*actChange=true"; do
    grep -qE "LEVEL CHANGE $q" $fg_all || { echo "FAIL: no act transition '$q'"; fail=1; }
  done
  # Baal dies in the Worldstone Chamber and the next difficulty opens
  grep -q "QUEST EFFECT reward unlock-difficulty" $fg_all || { echo "FAIL: Baal's death did not unlock a difficulty"; fail=1; }
  if [[ " ${FG_DIFFS:-0} " == *" 1 "* ]]; then
    grep -q "DIFFICULTY chosen .*difficulty=Nightmare" $fg_all || { echo "FAIL: Nightmare was not chosen (the save after Baal did not unlock it?)"; fail=1; }
  fi
  local d
  for d in ${=FG_DIFFS:-0}; do
    echo "difficulty $d: $(grep 'D2S EXPORT reparse' $fg_all | tail -1 | grep -oE 'difficulty=[0-9] act=[0-9]')"
  done
}
