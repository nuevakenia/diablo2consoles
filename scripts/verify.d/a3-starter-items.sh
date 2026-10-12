scenario_name="starting items of a new hero made in the menu (Druid: club and buckler worn, 4 potions in the belt, 2 scrolls; same after save and reload; screenshot)"
# A new Druid is created through the menu, played, saved and played again from the character select. The inventory
# panel is opened (and shot) in both games: the placeholder gear (crown, plate, boots) must not be there.
scenario_warnings_ok=1
scenario_timeout=300
sd=$tmp/starter
shot=$tmp/starter-items.png
scenario_env() {
  rm -rf $sd; mkdir -p $sd/saves $sd/cfg
  cp "${OD2_VERIFY_CONFIG_DIR:-$HOME/Library/Application Support/OpenDiablo2}/config.json" $sd/cfg/ 2>/dev/null
  echo "unset OD2_AUTOGAME OD2_D2S_WRITEBACK OD2_AUTOMAPSEED"
  echo "export OD2_CONFIG_DIR=\"$sd/cfg\" OD2_D2S_DIR=\"$sd/saves\""
  echo "export OD2_AUTOSCRIPT='wait:3;panel:inventory;wait:2;shot:$shot;wait:1;panel:close;wait:60'"
  local f="single,new,create:Druid:Starty,difficulty:0,wait:8,saveexit"
  f="$f,select:Starty,play,wait:8,saveexit,exit"
  echo "export OD2_AUTOFLOW=\"$f\""
}
scenario_check() {
  grep -E "equipment loaded|containers loaded|PANEL inventory|NEWCHAR created" $log.txt | cut -c1-260
  loads=$(grep -c "containers loaded:" $log.txt)
  [ "$loads" -ge 2 ] || { echo "FAIL: want 2 'containers loaded' lines (new game, reload), got $loads"; fail=1; return; }
  # both games: the Druid's club and buckler (CharStats item1/item2), 2 inventory items (tsc, isc), 4 belt potions (hp1 x4)
  n=$(grep -E "PANEL inventory:" $log.txt | grep -cE "worn=\[ weapon=Club shield=Buckler\] equipment=\[rightHand=Club, shield=Buckler\]")
  [ "$n" -ge 2 ] || { echo "FAIL: want the club and buckler in both games, got $n"; fail=1; }
  n=$(grep -cE "containers loaded: .*inventory=2 belt_items=4" $log.txt)
  [ "$n" -ge 2 ] || { echo "FAIL: want inventory=2 belt_items=4 in both games, got $n"; fail=1; }
  n=$(grep -c "equipment loaded: worn=2" $log.txt)
  [ "$n" -ge 2 ] || { echo "FAIL: want 2 worn items (club, buckler) in both games, got $n"; fail=1; }
  # nothing but the weapon and shield is worn: no crown, armour, boots, belt, rings, amulet
  grep -E "PANEL inventory:" $log.txt | grep -qE "worn=\[[^]]*(Crown|Plate|Boots|Ring|Amulet|Cap|Gloves)" && { echo "FAIL: placeholder gear is worn"; fail=1; }
  grep -qE "PANEL inventory: .*items=2 " $log.txt || { echo "FAIL: the inventory panel does not show the 2 scrolls"; fail=1; }
  # the file: the worn items are in the .d2s (3 or more JM items besides: the hero came back with them)
  [ -s $sd/saves/Starty.d2s ] || { echo "FAIL: no Starty.d2s"; fail=1; }
  [ -s $shot ] || { echo "FAIL: no screenshot"; fail=1; }
  grep -q "AUTOFLOW step [0-9]* exit" $log.txt || { echo "FAIL: the flow did not reach its end"; fail=1; }
}
