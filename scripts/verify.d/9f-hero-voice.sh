scenario_name="hero voice (sound events fire: needmana on a refused cast, hit grunt when zombies strike; each at most once per 74 ticks)"
scenario_env() { echo 'export OD2_SOUNDLOG=1 OD2_AUTOMONSTER="zombie1,2" OD2_AUTOMONSTER_SECONDS=30 OD2_AUTOMONSTER_DIFF=0 OD2_AUTOCAST="Fire Bolt,4" OD2_AUTOCAST_MANA=1'; }
scenario_check() {
  echo "hero voice lines:"
  grep -E "HEROVOICE" $log.txt | cut -c1-160 | sort | uniq -c | sort -rn | head -10
  grep -qE "HEROVOICE id=0x15 handle=[a-z]+_needmana_1" $log.txt || { echo "FAIL: no needmana voice on a refused cast"; fail=1; }
  grep -qE "SOUNDLOG .* kind=hero-voice .*handle=[a-z]+_needmana_1" $log.txt || { echo "FAIL: needmana never reached the sound bank"; fail=1; }
  grep -qE "HEROVOICE hit handle=[a-z]+_hit_1" $log.txt || { echo "FAIL: no hit grunt when the hero was struck"; fail=1; }
  # the hit grunt reaches the sound bank as the class row of the hero (record slot 0), once per hit
  grep -qE "SOUNDLOG .* kind=hero-voice .*handle=[a-z]+_hit_[1-5] .*decision=(played|stolen|queued|inaudible)" $log.txt || { echo "FAIL: hit grunt never reached the sound bank"; fail=1; }
  # repeats of a refusal voice are cut by the gap / the "unit sound already playing" test (w36)
  grep -qE "HEROVOICE id=0x15 suppressed" $log.txt || { echo "FAIL: repeated needmana was never suppressed"; fail=1; }
  # a voice line never starts while the previous hero voice is still sounding (refusals are the gated ones)
  # the 74-tick repeat gap: needmana (id 0x15) lines at most 30s*25/74 + 1
  local n=$(grep -cE "HEROVOICE id=0x15 handle=" $log.txt)
  [ "$n" -le 11 ] || { echo "FAIL: needmana repeated $n times (gap not applied)"; fail=1; }
}
scenario_unmuted=0
scenario_timeout=240
