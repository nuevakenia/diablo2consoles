scenario_name="w42 arrival void: the first frames in a new outdoor level are drawn (no black half screen), Act 2 desert and Act 1 fields"
scenario_turbo=1
scenario_warnings_ok=1
# Frames $tmp/w42av-*.png are read by eye: the first captured frame after each level change (wait:0 and wait:1).
h1=$tmp/w42av
scenario_env() {
  mkdir -p $h1/new $h1/wb; rm -f $h1/new/*.d2s(N) $h1/wb/*.d2s(N)
  {
    echo '#!/bin/zsh'
    echo "export OD2_PORT=$OD2_PORT OD2_AUTONEWCHAR=Necromancer OD2_AUTONEWCHAR_NAME=Hunter OD2_D2S_WRITEBACK=$h1/new OD2_AUTOEXIT=1"
    echo "$tmp/od2 > $h1/newchar.log 2>&1"
  } > $h1/newchar.command
  chmod +x $h1/newchar.command; launch_game $h1/newchar.command; wait_run
  local s="wait:1;say:levelup 12"
  hop() { s+=";walkto:exit=$1;expect:level=$1;say:capframe $tmp/w42av-$1-a.png;wait:1;say:capframe $tmp/w42av-$1-b.png;wait:2;say:capframe $tmp/w42av-$1-c.png"; }
  hop 2; hop 3; hop 4
  echo "export OD2_AUTOGAME=\"$h1/new/Hunter.d2s\" OD2_D2S_WRITEBACK=\"$h1/wb\" OD2_AUTOSPEED=3 OD2_NOPOPULATE=1"
  echo "export OD2_AUTOSCRIPT='$s;exit'"
}
scenario_check() {
  grep -E "AUTOSCRIPT step [0-9]+ FAIL|AUTOSCRIPT RESULT|LEVEL CHANGE" $log.txt | cut -c1-200 | tail -12
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: script did not pass"; fail=1; }
}
scenario_timeout=600
