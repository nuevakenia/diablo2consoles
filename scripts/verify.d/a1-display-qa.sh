scenario_name="display QA (town, panels, stash, vendor, waypoint, automap, notice; shots in OD2_QA_DIR at OD2_DISPLAY)"
scenario_realtime=1
scenario_timeout=300
# Display QA (docs/DISPLAY.md): run with OD2_DISPLAY=WxH OD2_QUIET=1 OD2_QA_DIR=<folder outside the repo>; the shots are looked at by eye.
scenario_env() {
  local d=$tmp/qa q=${OD2_QA_DIR:-$tmp/qa-shots}; mkdir -p $d/s94 $d/wb94 $q; rm -f $d/s94/*.d2s(N) $d/wb94/*.d2s(N)
  if [ -n "${D2_TABLES:-}" ] && go run scripts/d2s-revive.go "$D2S_SAMPLE_BODY" $d/s94/Hero.d2s "$D2_TABLES" >/dev/null 2>&1; then
    echo "export OD2_AUTOGAME=\"$d/s94/Hero.d2s\" OD2_D2S_WRITEBACK=\"$d/wb94\" OD2_AUTOSPEED=3"
  fi
  local s="wait:3;say:capframe $q/town.png;panel:character;panel:inventory;wait:1;say:capframe $q/char-inv.png;panel:close"
  s="$s;panel:skills;panel:quest;wait:1;say:capframe $q/quest-skills.png;panel:close"
  s="$s;panel:stash;panel:inventory;wait:1;say:capframe $q/stash-inv.png;panel:close"
  s="$s;panel:trade;panel:inventory;wait:1;say:capframe $q/trade-inv.png;panel:close"
  s="$s;panel:npcmenu;wait:1;say:capframe $q/npcmenu.png;panel:close"
  s="$s;automap:full;wait:1;say:capframe $q/automap.png;automap:mini;wait:1;say:capframe $q/automap-mini.png;automap:off"
  s="$s;say:completequest 1 1;wait:2;say:capframe $q/notice.png;wait:1"
  s="$s;use:Waypoint;wait:2;say:capframe $q/waypoint.png;panel:close;exit"
  echo "export OD2_AUTOSCRIPT='$s'"
}
scenario_check() {
  grep -E "AUTOSCRIPT RESULT|DISPLAY size" $log.txt | cut -c1-200 | head
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: display QA script did not pass"; fail=1; }
}
