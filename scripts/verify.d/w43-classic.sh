scenario_name="w43 classic game: a classic (non Lord of Destruction) hero keeps to Acts I-IV (Act V refused even with the quests done and travelfree), logs GAME MODE classic and its .d2s has no expansion status bit"
scenario_turbo=1
scenario_warnings_ok=1
c1=$tmp/w43
scenario_env() {
  mkdir -p $c1/new $c1/wb; rm -f $c1/new/*.d2s(N) $c1/wb/*.d2s(N)
  {
    echo '#!/bin/zsh'
    echo "export OD2_PORT=$OD2_PORT OD2_AUTONEWCHAR=Sorceress,classic OD2_AUTONEWCHAR_NAME=Classy OD2_D2S_WRITEBACK=$c1/new OD2_AUTOEXIT=1"
    echo "$tmp/od2 > $c1/newchar.log 2>&1"
  } > $c1/newchar.command
  chmod +x $c1/newchar.command; launch_game $c1/newchar.command; wait_run
  local s="wait:1;say:resetquests;say:completequest 1 6;say:completequest 2 6;say:completequest 3 6;say:completequest 4 2"
  s+=";travel:2;expect:level=40;wait:2;travel:3;expect:level=75;wait:2;travel:4;expect:level=103;wait:2"
  s+=";refuse:5;say:travelfree 1;refuse:5;say:travelfree 0;exit"
  echo "export OD2_AUTOGAME=\"$c1/new/Classy.d2s\" OD2_D2S_WRITEBACK=\"$c1/wb\" OD2_AUTOSPEED=3 OD2_AUTODIFFICULTY=1"
  echo "export OD2_AUTOSCRIPT='$s'"
}
scenario_check() {
  grep -E "AUTOSCRIPT step [0-9]+ FAIL|AUTOSCRIPT RESULT|GAME MODE|TRAVEL refused|LEVEL change .* refused" $log.txt | cut -c1-220 | tail -12
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: script did not pass"; fail=1; }
  grep -q "GAME MODE classic" $log.txt || { echo "FAIL: the hero did not play a classic game"; fail=1; }
  grep -q "GAME MODE expansion" $log.txt && { echo "FAIL: an expansion game was announced"; fail=1; }
  # the new .d2s: status byte 0x24 has no expansion bit 0x20; class byte 0x28 is Sorceress (1)
  python3 -I - "$c1/new/Classy.d2s" <<'PY' || { echo "FAIL: Classy.d2s is not a classic character"; fail=1; }
import sys
d = open(sys.argv[1], "rb").read()
assert d[0x24] & 0x20 == 0, "expansion bit set: %#x" % d[0x24]
assert d[0x28] == 1, "class %d" % d[0x28]
PY
  # the save the game wrote back is still classic
  for f in $c1/wb/Classy.d2s(N); do
    python3 -I -c 'import sys; d=open(sys.argv[1],"rb").read(); assert d[0x24]&0x20==0' "$f" || { echo "FAIL: written-back save became expansion"; fail=1; }
  done
}
