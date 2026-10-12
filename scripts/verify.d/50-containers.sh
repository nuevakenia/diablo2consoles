scenario_name="containers (stash, cube, belt, inventory import; stash object; belt potions)"
scenario_env() { echo 'export OD2_AUTOSTASH=1 OD2_AUTOPANEL=stash,cube,belt,inventory OD2_AUTOBELT=1,2,3,4'; }
scenario_check() {
  grep -E "AUTOPANEL (stash object|panel=[a-z]+ size|spec round)|containers loaded" $log.txt | cut -c1-200
  grep -qE "AUTOPANEL stash object opened the stash" $log.txt || { echo "FAIL: the stash object did not open the stash"; fail=1; }
  for p in stash cube belt inventory; do
    grep -qE "AUTOPANEL panel=$p size=" $log.txt || { echo "FAIL: no $p panel"; fail=1; }
  done
  # the grid rectangles (800x600 column space, the same at every window size) and every item inside them: the stash
  # grid sits on the recess of the picture it is drawn with (TradeStash.dc6 6x8 at 154,142 with the expansion data)
  grep -qE "AUTOPANEL panel=stash size=6x8 grid=154,142,328,374 outside=0 " $log.txt || { echo "FAIL: stash grid rectangle or items outside the grid"; fail=1; }
  grep -qE "AUTOPANEL panel=cube size=3x4 grid=198,199,285,315 outside=0 " $log.txt || { echo "FAIL: cube grid rectangle or items outside the grid"; fail=1; }
  grep -qE "AUTOPANEL spec round trip: checked=[0-9]+ mismatched=0" $log.txt || { echo "FAIL: saved items do not rebuild identically"; fail=1; }
}
