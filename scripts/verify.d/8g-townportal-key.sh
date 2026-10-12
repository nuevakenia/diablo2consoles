scenario_name="town portal hot key (X in the field opens a portal with a scroll of the inventory, else a tome; the portal object is drawn; a second X with the portal open replaces it; no scroll: nothing opens)"
scenario_realtime=1
scenario_timeout=240
tk=$tmp/tpkey
scenario_env() {
  mkdir -p $tk/s94 $tk/wb94; rm -f $tk/s94/*.d2s(N) $tk/wb94/*.d2s(N)
  if [ -n "${D2_TABLES:-}" ] && go run scripts/d2s-revive.go "$D2S_SAMPLE_BODY" $tk/s94/Hero.d2s "$D2_TABLES" >/dev/null 2>&1; then
    echo "export OD2_AUTOGAME=\"$tk/s94/Hero.d2s\" OD2_D2S_WRITEBACK=\"$tk/wb94\""
  fi
  echo 'export OD2_REALMAPS=1 OD2_AUTOLEVEL=2'
  local s='expect:level=2;wait:2;say:clearinv;press:X;wait:1;say:giveitem tsc;wait:1;press:X;wait:3;say:portals;say:capframe /private/tmp/tpkey-a.png;exit'
  echo "export OD2_AUTOSCRIPT='$s'"
}
scenario_check() {
  grep -E "TOWNPORTAL|PORTAL cast|PORTAL paid|PORTAL list|AUTOSCRIPT RESULT" $log.txt | cut -c1-200 | head
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: the town portal key script did not pass"; fail=1; }
  grep -q "TOWNPORTAL key: no Scroll" $log.txt || { echo "FAIL: X without a scroll should do nothing and say so"; fail=1; }
  [ "$(grep -c 'PORTAL cast level=' $log.txt)" = 1 ] || { echo "FAIL: want exactly one portal cast (the second X, with a scroll)"; fail=1; }
  grep -q "PORTAL list level=2 pairs=1 objects=1" $log.txt || { echo "FAIL: the portal object is not in the level"; fail=1; }
}
