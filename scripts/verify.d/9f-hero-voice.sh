scenario_name="hero voice (sound events fire: needmana on a refused cast, hit grunt when monsters strike; each at most once per 74 ticks)"
scenario_env() { echo 'export OD2_SOUNDLOG=1 OD2_AUTOMONSTER="zombie1,2" OD2_AUTOMONSTER_SECONDS=30 OD2_AUTOMONSTER_DIFF=0 OD2_AUTOCAST="Fire Bolt,4" OD2_AUTOCAST_MANA=1'; }
scenario_check() {
  echo "hero voice lines:"
  grep -E "HEROVOICE" $log.txt | cut -c1-160 | sort | uniq -c | sort -rn | head -10
  grep -qE "HEROVOICE id=0x15 handle=[a-z]+_needmana_1" $log.txt || { echo "FAIL: no needmana voice on a refused cast"; fail=1; }
  grep -qE "SOUNDLOG .* kind=hero-voice .*handle=[a-z]+_needmana_1" $log.txt || { echo "FAIL: needmana never reached the sound bank"; fail=1; }
  # The hit grunt has its own game (below): the zombies of this one shamble about and attacked 0 to 100 times in 30 s,
  # and skeletons that rush the hero delay her first cast until her mana has come back, so no refused cast.
  # the 74-tick repeat gap, measured in sound-engine ticks (a loaded machine runs fewer ticks per wall second, so a
  # count against the scenario seconds flaked: 16 lines in 58 s of a slow clock)
  local gaps=$(grep -oE "HEROVOICE id=0x15 handle=[a-z_0-9]+ tick=[0-9]+" $log.txt | sed 's/.*tick=//' | awk 'NR>1{print $1-p} {p=$1}' | sort -n | head -1)
  echo "needmana smallest tick gap: ${gaps:-none}"
  [ -z "$gaps" ] || [ "$gaps" -ge 74 ] || { echo "FAIL: needmana repeated after $gaps ticks (gap 74 not applied)"; fail=1; }
  hit_grunt_game
}
# hit_grunt_game: a second game where two skeletons attack a passive hero and every attack that reaches her connects
# (OD2_AUTOMONSTER_ALWAYSHIT; the level 94 hero against level 2 skeletons sits on the 5% minimum to-hit)
hit_grunt_game() {
  local hg=$tmp/9f-hit sl
  cp -f "${OD2_VERIFY_SAVE:-$D2S_SAMPLE_BODY}" $hg.d2s; rm -f $hg.d2s.bak
  {
    echo '#!/bin/zsh'
    echo "export OD2_PORT=$OD2_PORT"
    echo "export OD2_AUTOGAME=\"$hg.d2s\" OD2_AUTOEXIT=1 OD2_AUTOTEST_MUTE=1 OD2_AUTOSPEED=8"
    [ -n "${OD2_VERIFY_CONFIG_DIR:-}" ] && echo "export OD2_CONFIG_DIR=\"$OD2_VERIFY_CONFIG_DIR\""
    echo "export OD2_AUTOMONSTER=skeleton1,2 OD2_AUTOMONSTER_SECONDS=12 OD2_AUTOMONSTER_DIFF=0 OD2_AUTOMONSTER_ALWAYSHIT=1 OD2_AUTOMONSTER_PASSIVE=1"
    echo "$tmp/od2 2>&1 | tee $hg.log"
  } > $hg.command
  chmod +x $hg.command; rm -f $hg.log
  sl=$(./scripts/gameslot.sh acquire $$)
  launch_game $hg.command
  local log=$hg.log
  wait_run
  ./scripts/gameslot.sh release $sl
  sed 's/\x1b\[[0-9;]*m//g' $hg.log > $hg.log.txt
  grep -qE "HEROVOICE hit handle=[a-z]+_hit_1" $hg.log.txt || { echo "FAIL: no hit grunt when the hero was struck"; fail=1; mkdir -p ${OD2_VERIFY_FLAKE_DIR:-/tmp/od2-flake-logs}; cp -f $hg.log.txt ${OD2_VERIFY_FLAKE_DIR:-/tmp/od2-flake-logs}/9f-hit.$$.txt; }
  grep -E "HEROVOICE hit" $hg.log.txt | head -2 | cut -c1-160
}
scenario_unmuted=0
scenario_timeout=240
