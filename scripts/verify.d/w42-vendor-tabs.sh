scenario_name="w42 vendor tabs: Charsi/Akara windows open on the exe's tab, tabs filter the stock by item type page"
scenario_turbo=1
scenario_warnings_ok=1
# Frames (capframe) are written to $tmp/w42vt-*.png; read them by eye. The log gets "vendor tab:" lines on each switch.
h1=$tmp/w42vt
scenario_env() {
  mkdir -p $h1/new $h1/wb; rm -f $h1/new/*.d2s(N) $h1/wb/*.d2s(N)
  {
    echo '#!/bin/zsh'
    echo "export OD2_PORT=$OD2_PORT OD2_AUTONEWCHAR=Druid OD2_AUTONEWCHAR_NAME=Hunter OD2_D2S_WRITEBACK=$h1/new OD2_AUTOEXIT=1"
    echo "$tmp/od2 > $h1/newchar.log 2>&1"
  } > $h1/newchar.command
  chmod +x $h1/newchar.command; launch_game $h1/newchar.command; wait_run
  local s="wait:1"
  local n
  for n in Charsi Akara; do
    s+=";move:npc=$n;until:NPC menu opened: npc=\"$n\",60;menu:Trade;wait:2;say:capframe $tmp/w42vt-$n-0.png"
    s+=";click:left@ui:200,85;wait:1;say:capframe $tmp/w42vt-$n-1.png"
    s+=";click:left@ui:120,85;wait:1;say:capframe $tmp/w42vt-$n-armor.png"
    s+=";click:left@ui:360,85;wait:1;say:capframe $tmp/w42vt-$n-misc.png;press:Escape;wait:1"
  done
  echo "export OD2_AUTOGAME=\"$h1/new/Hunter.d2s\" OD2_D2S_WRITEBACK=\"$h1/wb\" OD2_AUTOSPEED=3"
  echo "export OD2_AUTOSCRIPT='$s;exit'"
}
scenario_check() {
  grep -E "AUTOSCRIPT step [0-9]+ FAIL|AUTOSCRIPT RESULT|trade window opened|vendor tab:" $log.txt | cut -c1-220 | tail -30
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: script did not pass"; fail=1; }
  grep -q "vendor tab: vendor=Charsi tab=0" $log.txt || { echo "FAIL: no switch to Charsi's Armor tab"; fail=1; }
}
