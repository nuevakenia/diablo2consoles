scenario_name="belt from the ground (a potion clicked on the ground rides the cursor, the inventory is closed, a click on each empty belt box of a 4-box belt and of an expanded row puts it there)"
scenario_realtime=1
scenario_timeout=300
scenario_warnings_ok=1 # setbelt takes the belt off the hero: the save export warns that the worn belt is kept in the file
bg=$tmp/beltground
scenario_env() {
  mkdir -p $bg/s94 $bg/wb94; rm -f $bg/s94/*.d2s(N) $bg/wb94/*.d2s(N)
  if [ -n "${D2_TABLES:-}" ] && go run scripts/d2s-revive.go "$D2S_SAMPLE_BODY" $bg/s94/Hero.d2s "$D2_TABLES" >/dev/null 2>&1; then
    echo "export OD2_AUTOGAME=\"$bg/s94/Hero.d2s\" OD2_D2S_WRITEBACK=\"$bg/wb94\" OD2_AUTOSPEED=3"
  fi
  echo 'export OD2_REALMAPS=1 OD2_AUTOLEVEL=2'
  local still='say:herostill;until:HERO still,120'
  local s='expect:level=2;say:heromoved;click:left@hero:200,0;until:HERO moved,120;'"$still"
  local c cx
  s="$s;say:setbelt none;say:clearinv"
  for c in 0 1 2 3; do
    cx=$((437 + 31 * c))
    s="$s;say:spawnground hp1 8 8;until:AUTOGROUND spawn source=command name=,30;click:left@item:Healing;wait:1;click:left@ui:$cx,576;wait:0.5;$still"
  done
  s="$s;say:beltlist;say:setbelt lbl;press:Tilde;wait:0.5"
  for c in 0 1 2 3; do
    cx=$((437 + 31 * c))
    s="$s;say:spawnground hp1 8 8;until:AUTOGROUND spawn source=command name=,30;click:left@item:Healing;wait:1;click:left@ui:$cx,544;wait:0.5;$still"
  done
  s="$s;say:beltlist;exit"
  echo "export OD2_AUTOSCRIPT='$s'"
}
scenario_check() {
  grep -E "belt cell|BELT contents|AUTOSCRIPT RESULT|AUTOSCRIPT step [0-9]* FAIL" $log.txt | cut -c1-200 | head -30
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: the belt ground script did not pass"; fail=1; }
  grep -q "BELT contents n=4 " $log.txt || { echo "FAIL: the four potions did not reach the 4-box belt"; fail=1; }
  grep -q "BELT contents n=4 \[hp1@e hp1@f hp1@g hp1@h\]" $log.txt || { echo "FAIL: the four potions did not reach row 2 of the sash (cells e to h)"; fail=1; }
}
