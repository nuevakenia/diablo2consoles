scenario_timeout=900
scenario_turbo=1  # scripted autoplay, no wall-clock assertions (numbers: ~/git/d2-re-notes/turbo-mode.md)
scenario_name="Rite of Passage (Arreat Summit altar refuses, the statues wake the three Ancients, a town portal resets them, the third death pays the quest and the altar opens the Worldstone Keep gate)"
ri=$tmp/rite
# Played by OD2_AUTOSCRIPT. The hero (level 94 sample) travels to Harrogath with the quest chain forced, steps into the
# Arreat Summit through a spawned portal, and drives the fight with the scenario commands ritestart / ritekill /
# ritealtar (d2game/d2gamescreen/rite_live.go; rules in d2common/d2quest/w29_ancients.go, notes w29-ancients.md).
# The sample character is a dead hardcore Sorceress; scripts/d2s-revive.go makes a living copy (needs D2_TABLES).
scenario_warnings_ok=1
scenario_env() {
  mkdir -p $ri/s94 $ri/wb94; rm -f $ri/s94/Hero.d2s $ri/wb94/*.d2s(N)
  if [ -n "${D2_TABLES:-}" ] && make_hero $ri/s94/Hero.d2s; then
    local s="wait:1;say:resetquests;say:completequest 1 6;travel:2;expect:level=40;wait:2;say:completequest 2 6;travel:3;expect:level=75;wait:2;say:completequest 3 6;travel:4;expect:level=103;wait:2;say:completequest 4 2;travel:5;expect:level=109;wait:3"
    s+=";say:completequest 5 1;say:completequest 5 2;say:completequest 5 3;say:completequest 5 4;wait:6"
    s+=";say:spawnportal 120;use:Portal;expect:level=120;wait:3;say:restorevitals"
    # the altar refuses while the Rite is neither done nor pending
    s+=";say:ritealtar;expect:log=RITE the altar does nothing"
    # the statues' introduction wakes the three Ancients
    s+=";say:ritestart;wait:1;expect:log=RITE Ancient 3 wakes;expect:log=RITE the fight starts"
    # a town portal in the summit resets them; closing it arms the fight again
    s+=";say:townportal free;wait:2;expect:log=RITE a town portal opened in the summit"
    s+=";say:closeportal;wait:2;expect:log=RITE the last portal closed"
    # killing all three: the third death ends the fight and pays the quest
    s+=";say:restorevitals;say:ritekill;wait:2;expect:log=RITE the third Ancient died"
    s+=";say:ritealtar;expect:log=RITE the altar opens the way to the Worldstone Keep;say:ritestatus;wait:1;exit"
    echo "export OD2_AUTOGAME=\"$ri/s94/Hero.d2s\" OD2_D2S_WRITEBACK=\"$ri/wb94\" OD2_AUTOSPEED=4 OD2_AUTOMONSTER_DIFF=0"
    echo "export OD2_AUTOSCRIPT='$s'"
  else
    echo "# no revived sample (D2_TABLES unset or the tool failed)" >&2
    echo "export OD2_AUTOSCRIPT='wait:1;exit'"
  fi
}
scenario_check() {
  grep -E "RITE |AUTOSCRIPT RESULT" $log.txt | cut -c1-200 | tail -30
  [ -s $ri/s94/Hero.d2s ] || { echo "SKIP: no revived sample hero in this run"; return; }
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: the Rite of Passage script did not pass"; fail=1; }
  grep -E "AUTOSCRIPT step [0-9]+ FAIL" $log.txt | cut -c1-200
  grep -qE "LEVEL CHANGE .*to=120" $log.txt || { echo "FAIL: the hero did not reach the Arreat Summit"; fail=1; }
  for l in "RITE the altar does nothing" "RITE Ancient 1 wakes" "RITE Ancient 2 wakes" "RITE Ancient 3 wakes" "RITE the fight starts" \
           "RITE a town portal opened in the summit" "RITE the last portal closed" "RITE the third Ancient died" \
           "RITE the altar opens the way"; do
    grep -q "$l" $log.txt || { echo "FAIL: no log line '$l'"; fail=1; }
  done
  # the Ancients are the classes 540..542 and the quest system saw all three deaths
  grep -qE "RITE Ancient 1 wakes .*class 540" $log.txt || { echo "FAIL: the first Ancient is not class 540"; fail=1; }
  grep -qE "RITE Ancient 3 wakes .*class 542" $log.txt || { echo "FAIL: the third Ancient is not class 542"; fail=1; }
  grep -qE "QUEST A5Q5 slot=39 bits 0x[0-9a-f]+->0x[0-9a-f]+" $log.txt || { echo "FAIL: no quest bit change for Rite of Passage"; fail=1; }
  # the fight armed again after the reset: six Ancients woke in all (three, reset, three)
  [ "$(grep -c 'RITE Ancient [123] wakes' $log.txt)" -ge 6 ] || { echo "FAIL: the Ancients did not wake again after the portal closed"; fail=1; }
  if grep -E "\[ERROR\]|panic" $log.txt | grep -v "skipping missing"; then echo "FAIL: errors in the Rite log"; fail=1; fi
}
