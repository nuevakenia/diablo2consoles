scenario_name="quick equip (OD2_PICKUP=quick: a hero that took everything off clicks a helm on the ground and wears it at once; a second helm goes to the inventory)"
scenario_realtime=1
scenario_warnings_ok=1 # the stripped sample hero is exported with the worn items it no longer wears
scenario_timeout=240
qe=$tmp/quickequip
scenario_env() {
  mkdir -p $qe/s94 $qe/wb94; rm -f $qe/s94/*.d2s(N) $qe/wb94/*.d2s(N)
  if [ -n "${D2_TABLES:-}" ] && go run scripts/d2s-revive.go "$D2S_SAMPLE_BODY" $qe/s94/Hero.d2s "$D2_TABLES" >/dev/null 2>&1; then
    echo "export OD2_AUTOGAME=\"$qe/s94/Hero.d2s\" OD2_D2S_WRITEBACK=\"$qe/wb94\" OD2_AUTOSPEED=3"
  fi
  echo 'export OD2_REALMAPS=1 OD2_AUTOLEVEL=2 OD2_PICKUP=quick'
  local s='expect:level=2;say:clearinv;say:stripall;say:clearinv'
  s="$s;say:spawnground cap 8 8;until:AUTOGROUND spawn source=command name=,30;click:left@item:Cap;until:QUICKEQUIP name=,120"
  s="$s;say:spawnground cap 9 9;until:AUTOGROUND spawn source=command name=,30;click:left@item:Cap;until:QUICKPICKUP name=,120;exit"
  echo "export OD2_AUTOSCRIPT='$s'"
}
scenario_check() {
  grep -E "STRIPALL|QUICKEQUIP|QUICKPICKUP|AUTOSCRIPT RESULT" $log.txt | cut -c1-200 | head
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: the quick equip script did not pass"; fail=1; }
  [ "$(grep -c 'QUICKEQUIP name=' $log.txt)" -ge 1 ] || { echo "FAIL: no QUICKEQUIP of the first helm"; fail=1; }
  grep -q 'QUICKPICKUP name=' $log.txt || { echo "FAIL: the second helm did not go into the inventory"; fail=1; }
}
