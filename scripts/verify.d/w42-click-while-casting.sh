scenario_name="w42 click while casting: a walk click during a cast is dropped (logged), a walk click after it works; no queueing"
scenario_turbo=1
scenario_warnings_ok=1
# The exe ignores walk orders while a cast runs (PLRMODE_CanInterruptCurrentMode); it never queues them.
h1=$tmp/w42cc
scenario_env() {
  mkdir -p $h1/new $h1/wb; rm -f $h1/new/*.d2s(N) $h1/wb/*.d2s(N)
  {
    echo '#!/bin/zsh'
    echo "export OD2_PORT=$OD2_PORT OD2_AUTONEWCHAR=Sorceress OD2_AUTONEWCHAR_NAME=Hunter OD2_D2S_WRITEBACK=$h1/new OD2_AUTOEXIT=1"
    echo "$tmp/od2 > $h1/newchar.log 2>&1"
  } > $h1/newchar.command
  chmod +x $h1/newchar.command; launch_game $h1/newchar.command; wait_run
  # Fire Bolt on the right button, then at once a left click on the ground (a walk) while the cast animation runs
  # spells are not castable in town: learn Fire Bolt, walk out to the Blood Moor first
  local s="wait:1;say:levelup 3;skill:spend=Fire Bolt;walkto:exit=2;expect:level=2;wait:2;skill:right=Fire Bolt"
  s+=";click:right@hero:60,-20;click:left@hero:-80,40;wait:3;say:capframe $tmp/w42cc-after.png"
  s+=";click:left@hero:-80,40;wait:2"
  echo "export OD2_AUTOGAME=\"$h1/new/Hunter.d2s\" OD2_D2S_WRITEBACK=\"$h1/wb\" OD2_AUTOSPEED=1 OD2_NOPOPULATE=1"
  echo "export OD2_AUTOSCRIPT='$s;exit'"
}
scenario_check() {
  grep -E "AUTOSCRIPT step [0-9]+ FAIL|AUTOSCRIPT RESULT|INPUT click ignored|INPUT world-click" $log.txt | cut -c1-200 | tail -12
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: script did not pass"; fail=1; }
  grep -q "INPUT click ignored while casting" $log.txt || { echo "FAIL: the walk click during the cast was not dropped"; fail=1; }
  [ "$(grep -c 'INPUT world-click button=0 .*action=move' $log.txt)" -ge 1 ] || { echo "FAIL: the walk click after the cast did not walk"; fail=1; }
}
