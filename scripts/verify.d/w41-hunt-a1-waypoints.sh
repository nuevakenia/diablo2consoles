scenario_name="w41 hunt: Act 1 outdoor waypoints exist (Cold Plains, Stony Field, Dark Wood, Black Marsh)"
scenario_turbo=1
scenario_warnings_ok=1
# A human always finds the waypoint of each Act 1 outdoor level. OD2_AUTOMAPSEED=<n> plays another world layout.
h3=$tmp/w41wp
scenario_env() {
  mkdir -p $h3/new $h3/wb; rm -f $h3/new/*.d2s(N) $h3/wb/*.d2s(N)
  {
    echo '#!/bin/zsh'
    echo "export OD2_PORT=$OD2_PORT OD2_AUTONEWCHAR=Necromancer OD2_AUTONEWCHAR_NAME=Hunter OD2_D2S_WRITEBACK=$h3/new OD2_AUTOEXIT=1"
    echo "$tmp/od2 > $h3/newchar.log 2>&1"
  } > $h3/newchar.command
  chmod +x $h3/newchar.command; launch_game $h3/newchar.command; wait_run
  local s="wait:1;say:levelup 12"
  hop() { s+=";walkto:exit=$1;expect:level=$1;wait:2;say:capframe $tmp/wp-$1.png"; }
  hop 2; hop 3; hop 4; hop 10; hop 5; hop 6
  echo "export OD2_AUTOGAME=\"$h3/new/Hunter.d2s\" OD2_D2S_WRITEBACK=\"$h3/wb\" OD2_AUTOSPEED=3 OD2_NOPOPULATE=1"
  echo "export OD2_AUTOSCRIPT='$s;exit'"
}
scenario_check() {
  grep -E "AUTOSCRIPT step [0-9]+ FAIL|AUTOSCRIPT RESULT|LEVEL objects|LEVEL CHANGE" $log.txt | cut -c1-200 | tail -20
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: script did not pass"; fail=1; }
  # the line "LEVEL objects:" precedes the level change into the level it describes (Blood Moor, then Cold Plains ...)
  local lv
  for lv in 3 4 5 6; do
    awk -v lv=$lv '/LEVEL objects:/{o=$0} $0 ~ ("LEVEL CHANGE from=[0-9]+ to=" lv " "){print o}' $log.txt | grep -q "Waypoint" || { echo "FAIL: level $lv has no Waypoint object"; fail=1; }
  done
}
scenario_timeout=900
