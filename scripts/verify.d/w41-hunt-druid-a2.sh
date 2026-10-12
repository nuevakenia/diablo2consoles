scenario_name="w41 hunt: Druid in Act 2: Lut Gholein NPCs (menus, trade, gamble, repair), desert, waypoint, tomb"
scenario_turbo=1
scenario_warnings_ok=1
# Play-style hunt, wave 41. A fresh Druid (OD2_AUTONEWCHAR) with 20 console levels and the Act 1 quests forced.
h4=$tmp/w41a2
scenario_env() {
  mkdir -p $h4/new $h4/wb; rm -f $h4/new/*.d2s(N) $h4/wb/*.d2s(N)
  local cls=${OD2_HUNT_CLASS:-Druid} skill=${OD2_HUNT_SKILL:-Firestorm}
  {
    echo '#!/bin/zsh'
    echo "export OD2_PORT=$OD2_PORT OD2_AUTONEWCHAR=$cls OD2_AUTONEWCHAR_NAME=Hunter OD2_D2S_WRITEBACK=$h4/new OD2_AUTOEXIT=1"
    echo "$tmp/od2 > $h4/newchar.log 2>&1"
  } > $h4/newchar.command
  chmod +x $h4/newchar.command; launch_game $h4/newchar.command; wait_run
  local s="wait:1;say:levelup 20"
  repeat 10 s+=";skill:spend=$skill"
  s+=";skill:left=$skill;say:completequest 1 6;travel:2;expect:level=40;wait:4;say:capframe $tmp/a2-town.png"
  local n
  # every NPC: open the menu, look at it, Esc, open again and take the Talk row
  for n in Warriv Fara Greiz Elzix Drognan Atma Lysander Jerhyn Meshif; do
    s+=";move:npc=$n;until:NPC menu opened: npc=\"$n\",60;say:capframe $tmp/a2-$n.png;press:Escape;wait:1"
  done
  # vendors
  s+=";say:setgold 20000"
  s+=";move:npc=Fara;until:NPC menu opened: npc=\"Fara\",60;menu:Trade;wait:2;say:capframe $tmp/a2-trade-Fara.png;press:Escape;wait:1"
  s+=";move:npc=Drognan;until:NPC menu opened: npc=\"Drognan\",60;menu:Trade;wait:2;say:capframe $tmp/a2-trade-Drognan.png;press:Escape;wait:1"
  s+=";move:npc=Elzix;until:NPC menu opened: npc=\"Elzix\",60;menu:Gamble;wait:2;say:capframe $tmp/a2-gamble-Elzix.png;press:Escape;wait:1"
  s+=";move:npc=Greiz;until:NPC menu opened: npc=\"Greiz\",60;menu:Hire;wait:2;say:capframe $tmp/a2-hire-Greiz.png;press:Escape;wait:1"
  s+=";say:autobuy Fara;wait:2;panel:inventory;wait:1;say:capframe $tmp/a2-inv.png;panel:close"
  s+=";use:Waypoint;wait:2;say:capframe $tmp/a2-wp-town.png;press:Escape;wait:1"
  hop() { s+=";walkto:exit=$1;expect:level=$1;wait:2;say:capframe $tmp/a2-$1.png"; }
  play() { s+=";kill:near=30,60;loot:30,40;say:capframe $tmp/a2-$1-after.png"; }
  hop 41; play 41
  s+=";say:spawnchest 5 2 118;wait:1;click:left@object:5;wait:6;click:left@object:2;wait:6;click:left@object:118;wait:6;say:capframe $tmp/a2-objs.png;loot:10,20"
  hop 42; play 42
  hop 56; play 56; hop 42
  hop 41
  echo "export OD2_AUTOGAME=\"$h4/new/Hunter.d2s\" OD2_D2S_WRITEBACK=\"$h4/wb\" OD2_AUTOSPEED=3 OD2_POPULATE_DENSITY=25 OD2_AUTOSCRIPT_HEAL=1"
  echo "export OD2_AUTOSCRIPT='$s;exit'"
}
scenario_check() {
  grep -E "AUTOSCRIPT step [0-9]+ FAIL|AUTOSCRIPT RESULT|LEVEL objects|LEVEL CHANGE" $log.txt | cut -c1-200 | tail -30
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: script did not pass"; fail=1; }
}
scenario_timeout=1200
