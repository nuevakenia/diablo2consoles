scenario_name="w41 hunt: new Necromancer (OD2_HUNT_CLASS) plays Act 1 outdoors: chests/shrines/wells, loot and equip, waypoints, caves, Burial Grounds"
scenario_turbo=1
scenario_warnings_ok=1
# Play-style hunt, wave 41: what a human does after the town. Frames are read by the hunter (Read tool) from $tmp.
h2=$tmp/w41a1f
scenario_env() {
  mkdir -p $h2/new $h2/wb; rm -f $h2/new/*.d2s(N) $h2/wb/*.d2s(N)
  local cls=${OD2_HUNT_CLASS:-Necromancer} skill=${OD2_HUNT_SKILL:-Teeth}
  {
    echo '#!/bin/zsh'
    echo "export OD2_PORT=$OD2_PORT OD2_AUTONEWCHAR=$cls OD2_AUTONEWCHAR_NAME=Hunter OD2_D2S_WRITEBACK=$h2/new OD2_AUTOEXIT=1"
    echo "$tmp/od2 > $h2/newchar.log 2>&1"
  } > $h2/newchar.command
  chmod +x $h2/newchar.command; launch_game $h2/newchar.command; wait_run
  local s="wait:1;say:levelup 8"
  local i
  repeat 6 s+=";skill:spend=$skill"
  s+=";skill:left=$skill"
  # hop <level>: walk through the border or the entrance tile that leads there, look at it
  hop() { s+=";walkto:exit=$1;expect:level=$1;wait:2;say:capframe $tmp/f-$1.png"; }
  # play <level>: fight and pick up whatever lies about
  play() { s+=";kill:near=30,60;loot:30,40;say:capframe $tmp/f-$1-after.png"; }
  hop 2; play 2
  # containers, shrine, well next to the hero: open each one with a real click (scripts/verify.d/8g-click-bloodmoor.sh)
  s+=";say:spawnchest 5 1 7;until:AUTOGROUND chest-spawn id=5,30;say:capframe $tmp/chests.png"
  s+=";click:left@object:5;until:AUTOGROUND chest id=5,120;wait:2;say:capframe $tmp/chest-open.png"
  s+=";click:left@object:1;wait:6;click:left@object:7;wait:6;say:capframe $tmp/chest-all.png;loot:10,20;panel:inventory;wait:1;say:capframe $tmp/inv-loot.png;panel:close"
  s+=";say:spawnchest 2 118 9;wait:1;click:left@object:2;wait:6;say:capframe $tmp/shrine.png;click:left@object:118;wait:6;say:capframe $tmp/well.png;click:left@object:9;wait:6"
  # equip: a better weapon from the ground
  s+=";say:spawnground clb 3 3;wait:1;loot:8,12;say:wield clb;wait:1;panel:character;wait:1;say:capframe $tmp/char-equip.png;panel:close;panel:inventory;wait:1;say:capframe $tmp/inv-equip.png;panel:close"
  hop 8; play 8; hop 2
  hop 3; play 3
  s+=";use:Waypoint;wait:2;say:capframe $tmp/wp-open.png;press:Escape;wait:1"
  hop 9; play 9; hop 3
  hop 17; play 17; hop 3
  hop 4; play 4
  s+=";use:Waypoint;wait:2;say:capframe $tmp/wp-open2.png;waypoint:3;expect:level=3;wait:3;say:capframe $tmp/wp-arrive.png"
  echo "export OD2_AUTOGAME=\"$h2/new/Hunter.d2s\" OD2_D2S_WRITEBACK=\"$h2/wb\" OD2_AUTOSPEED=3 OD2_POPULATE_DENSITY=25"
  echo "export OD2_AUTOSCRIPT='$s;exit'"
}
scenario_check() {
  grep -E "AUTOSCRIPT step [0-9]+ FAIL|AUTOSCRIPT RESULT" $log.txt | cut -c1-220 | tail -20
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: script did not pass"; fail=1; }
}
scenario_timeout=900
