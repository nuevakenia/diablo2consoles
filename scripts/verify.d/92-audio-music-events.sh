scenario_realtime=1
scenario_name="audio rules by log (Blood Moor: music slot, day/night ambience, ambient event offsets and schedule; muted but logged)"
# OD2_AUTOTEST_MUTE keeps the device silent; the AMBIENT / AUTOAMBIENT / SOUNDAT lines still show every decision.
scenario_env() { echo 'export OD2_AUTOAMBIENT="Blood Moor" OD2_AUTOAMBIENT_SECONDS=8 OD2_AUTOAMBIENT_SPEED=10 OD2_SOUNDLOG=1'; }
scenario_check() {
  grep -aE "AMBIENT (env change|day phase|music|event scheduled)" $log.txt | cut -c1-300 | head -12
  grep -aq "AMBIENT env change env=ESOUNDENVIRON_WILDERNESS.* music=music_wilderness" $log.txt || { echo "FAIL: Blood Moor did not pick the wilderness song"; fail=1; }
  grep -aq "AMBIENT music playing=music_wilderness" $log.txt || { echo "FAIL: music slot never adopted the wilderness song"; fail=1; }
  grep -aq "AMBIENT day phase .*night=true.*ambience=scene_wilderness_night" $log.txt || { echo "FAIL: no night ambience after dusk"; fail=1; }
  grep -aq "AMBIENT event scheduled handle=event_wild_day_1 first_in=.* interval=" $log.txt || { echo "FAIL: no day event schedule"; fail=1; }
  grep -aq "AMBIENT event scheduled handle=event_wild_night_1" $log.txt || { echo "FAIL: no night event schedule"; fail=1; }
  # every ambient event sits 450..1000 sound units from the hero (x = +-[450,750], y = +-100 doubled)
  local bad=$(grep -a "SOUNDAT kind=ambient-event" $log.txt | sed -n 's/.* dist=\([0-9]*\) .*/\1/p' | awk '$1<450 || $1>1000' | wc -l)
  [ $((bad)) -eq 0 ] || { echo "FAIL: $bad ambient events outside the 450..1000 ring"; fail=1; }
  grep -aq "SOUNDAT kind=ambient-event" $log.txt || { echo "FAIL: no ambient event played"; fail=1; }
  grep -aq "AUTOAMBIENT summary" $log.txt || { echo "FAIL: no summary"; fail=1; }
}
