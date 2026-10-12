scenario_name="town portal round trip (X opens a portal in the Blood Moor, the hero walks into it and arrives in the Rogue Encampment, walks into the town side portal and is back in the Blood Moor)"
scenario_realtime=1
scenario_timeout=300
pr=$tmp/portalrt
scenario_env() {
  mkdir -p $pr/s94 $pr/wb94; rm -f $pr/s94/*.d2s(N) $pr/wb94/*.d2s(N)
  if [ -n "${D2_TABLES:-}" ] && go run scripts/d2s-revive.go "$D2S_SAMPLE_BODY" $pr/s94/Hero.d2s "$D2_TABLES" >/dev/null 2>&1; then
    echo "export OD2_AUTOGAME=\"$pr/s94/Hero.d2s\" OD2_D2S_WRITEBACK=\"$pr/wb94\""
  fi
  echo 'export OD2_REALMAPS=1 OD2_AUTOLEVEL=2'
  local s='expect:level=2;wait:2;say:clearinv;say:giveitem tsc;wait:1;press:X;wait:3'
  s="$s;click:left@object:59;until:LEVEL built: level 1,120;wait:3;expect:level=1;wait:5"
  s="$s;click:left@object:59;until:LEVEL built: level 2,120;wait:3;expect:level=2;exit"
  echo "export OD2_AUTOSCRIPT='$s'"
}
scenario_check() {
  grep -E "PORTAL|LEVEL built|AUTOSCRIPT RESULT|AUTOSCRIPT step [0-9]+ FAIL" $log.txt | cut -c1-200 | head -14
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: the portal round trip did not pass"; fail=1; }
}
