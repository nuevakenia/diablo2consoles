scenario_name="w41 hunt: new Druid/Necromancer (OD2_HUNT_CLASS) walks the Rogue Encampment, talks to every NPC (all rows, Esc), trades, gambles, repairs, spends points"
scenario_turbo=1
scenario_warnings_ok=1
# Play-style hunt, wave 41: what a human does in the first town. Frames are read by the hunter (Read tool) from $tmp.
h1=$tmp/w41a1
scenario_env() {
  mkdir -p $h1/new $h1/wb; rm -f $h1/new/*.d2s(N) $h1/wb/*.d2s(N)
  local cls=${OD2_HUNT_CLASS:-Druid}
  {
    echo '#!/bin/zsh'
    echo "export OD2_PORT=$OD2_PORT OD2_AUTONEWCHAR=$cls OD2_AUTONEWCHAR_NAME=Hunter OD2_D2S_WRITEBACK=$h1/new OD2_AUTOEXIT=1"
    echo "$tmp/od2 > $h1/newchar.log 2>&1"
  } > $h1/newchar.command
  chmod +x $h1/newchar.command; launch_game $h1/newchar.command; wait_run
  local s="wait:1;say:capframe $tmp/town0.png;panel:inventory;wait:1;say:capframe $tmp/inv0.png;panel:close;panel:character;wait:1;say:capframe $tmp/char0.png;panel:close"
  s+=";panel:skills;wait:1;say:capframe $tmp/skills0.png;panel:close"
  local n r
  # every NPC: open, look, Esc; then every row by name
  for n in Akara Charsi Gheed Kashya Warriv; do
    s+=";move:npc=$n;until:NPC menu opened: npc=\"$n\",60;say:capframe $tmp/npc-$n.png;press:Escape;wait:1;say:capframe $tmp/npc-$n-esc.png"
    s+=";move:npc=$n;until:NPC menu opened: npc=\"$n\",60;menu:Talk;wait:3;say:capframe $tmp/talk-$n.png;wait:4"
  done
  s+=";move:npc=Akara;until:NPC menu opened: npc=\"Akara\",60;menu:Trade;wait:2;say:capframe $tmp/trade-Akara.png;press:Escape;wait:1;say:capframe $tmp/trade-Akara-esc.png"
  s+=";move:npc=Charsi;until:NPC menu opened: npc=\"Charsi\",60;menu:Trade;wait:2;say:capframe $tmp/trade-Charsi.png;press:Escape;wait:1"
  s+=";move:npc=Gheed;until:NPC menu opened: npc=\"Gheed\",60;menu:Gamble;wait:2;say:capframe $tmp/gamble-Gheed.png;press:Escape;wait:1"
  s+=";move:npc=Gheed;until:NPC menu opened: npc=\"Gheed\",60;menu:Trade;wait:2;say:capframe $tmp/trade-Gheed.png;press:Escape;wait:1"
  s+=";say:setgold 5000;say:autobuy Akara;wait:2;say:capframe $tmp/buy-Akara.png;panel:inventory;wait:1;say:capframe $tmp/inv-bought.png;panel:close"
  s+=";say:levelup 3;wait:1;panel:character;wait:1;say:capframe $tmp/char-levelup.png;click:left@ui:221,151;wait:1;click:left@ui:221,151;wait:1;say:capframe $tmp/char-spent.png;panel:close;panel:skills;wait:1;say:capframe $tmp/skills-levelup.png;skill:spend=${OD2_HUNT_SKILL:-Firestorm};wait:1;say:capframe $tmp/skills-spent.png;panel:close"
  echo "export OD2_AUTOGAME=\"$h1/new/Hunter.d2s\" OD2_D2S_WRITEBACK=\"$h1/wb\" OD2_AUTOSPEED=3"
  echo "export OD2_AUTOSCRIPT='$s;exit'"
}
scenario_check() {
  grep -E "AUTOSCRIPT step [0-9]+ FAIL|AUTOSCRIPT RESULT" $log.txt | cut -c1-220 | tail -20
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: script did not pass"; fail=1; }
}
