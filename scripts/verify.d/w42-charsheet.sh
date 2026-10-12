scenario_name="w42 character sheet: Damage and Attack Rating boxes are filled (frame of the panel for a new Barbarian)"
scenario_turbo=1
scenario_warnings_ok=1
# Frame: $tmp/w42cs-char.png (read by eye); the PANEL log line carries the values.
h1=$tmp/w42cs
scenario_env() {
  mkdir -p $h1/new $h1/wb; rm -f $h1/new/*.d2s(N) $h1/wb/*.d2s(N)
  {
    echo '#!/bin/zsh'
    echo "export OD2_PORT=$OD2_PORT OD2_AUTONEWCHAR=Barbarian OD2_AUTONEWCHAR_NAME=Hunter OD2_D2S_WRITEBACK=$h1/new OD2_AUTOEXIT=1"
    echo "$tmp/od2 > $h1/newchar.log 2>&1"
  } > $h1/newchar.command
  chmod +x $h1/newchar.command; launch_game $h1/newchar.command; wait_run
  local s="wait:1;panel:character;wait:1;say:capframe $tmp/w42cs-char.png;panel:close"
  echo "export OD2_AUTOGAME=\"$h1/new/Hunter.d2s\" OD2_D2S_WRITEBACK=\"$h1/wb\" OD2_AUTOSPEED=3"
  echo "export OD2_AUTOSCRIPT='$s;exit'"
}
scenario_check() {
  grep -E "AUTOSCRIPT step [0-9]+ FAIL|AUTOSCRIPT RESULT|PANEL" $log.txt | cut -c1-260 | tail -10
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: script did not pass"; fail=1; }
}
