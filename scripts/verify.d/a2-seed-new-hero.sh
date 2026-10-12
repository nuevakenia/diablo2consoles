scenario_name="map seed of new heroes made in the menu (own non-zero seed, stored in the .d2s, same town on reload, two heroes differ)"
# Two new characters are created through the menu; the first is played again from the character select.
scenario_warnings_ok=1
scenario_timeout=300
sd=$tmp/seednew
scenario_env() {
  rm -rf $sd; mkdir -p $sd/saves $sd/cfg
  cp "${OD2_VERIFY_CONFIG_DIR:-$HOME/Library/Application Support/OpenDiablo2}/config.json" $sd/cfg/ 2>/dev/null
  echo "unset OD2_AUTOGAME OD2_D2S_WRITEBACK OD2_AUTOMAPSEED"
  echo "export OD2_CONFIG_DIR=\"$sd/cfg\" OD2_D2S_DIR=\"$sd/saves\""
  local f="single,new,create:Sorceress:Seedy,difficulty:0,wait:5,saveexit"
  f="$f,new,create:Necromancer:Other,difficulty:0,wait:5,saveexit"
  f="$f,select:Seedy,play,wait:5,saveexit,exit"
  echo "export OD2_AUTOFLOW=\"$f\""
}
scenario_check() {
  grep -E "map seed of|new map seed|real town:|NEWCHAR created" $log.txt | cut -c1-220
  seedy=("${(@f)$(grep -o "map seed of Seedy: [0-9]*" $log.txt | tr -d '\r' | sed 's/.*: //')}")
  other=("${(@f)$(grep -o "map seed of Other: [0-9]*" $log.txt | tr -d '\r' | sed 's/.*: //')}")
  [ ${#seedy} -eq 2 ] || { echo "FAIL: want 2 'map seed of Seedy' lines, got ${#seedy}"; fail=1; return; }
  [ ${#other} -eq 1 ] || { echo "FAIL: want 1 'map seed of Other' line, got ${#other}"; fail=1; return; }
  [ "$seedy[1]" != 0 ] && [ -n "$seedy[1]" ] || { echo "FAIL: Seedy has no seed"; fail=1; }
  [ "$seedy[1]" = "$seedy[2]" ] || { echo "FAIL: Seedy's seed changed between the loads: $seedy[1] -> $seedy[2]"; fail=1; }
  [ "$seedy[1]" != "$other[1]" ] || { echo "FAIL: two new heroes got the same seed $seedy[1]"; fail=1; }
  grep -q "new map seed for" $log.txt && { echo "FAIL: a hero made by the menu had no seed at the start"; fail=1; }
  # the seed is in the real .d2s of the character (header 0xAB)
  inheader=$(python3 -I -c 'import struct,sys; print(struct.unpack("<I", open(sys.argv[1],"rb").read()[0xAB:0xAF])[0])' $sd/saves/Seedy.d2s 2>/dev/null)
  [ "$inheader" = "$seedy[1]" ] || { echo "FAIL: Seedy.d2s has seed '$inheader', the game used $seedy[1]"; fail=1; }
  # the town is built by the client and the server at every load: lines 1-2 Seedy, 3-4 Other, 5-6 Seedy again
  towns=("${(@f)$(grep -o "real town: .*" $log.txt | tr -d '\r')}")
  [ ${#towns} -eq 6 ] || { echo "FAIL: want 6 town lines (client and server, three loads), got ${#towns}"; fail=1; return; }
  [ "$towns[1]" = "$towns[5]" ] && [ "$towns[2]" = "$towns[6]" ] || { echo "FAIL: Seedys town differs after the reload:"; echo " 1: $towns[1]"; echo " 5: $towns[5]"; fail=1; }
  grep -q "AUTOFLOW step [0-9]* exit" $log.txt || { echo "FAIL: the flow did not reach its end"; fail=1; }
  # w37 save integrity: the new .d2s files are valid, nothing half-written is left, saving again kept the previous version
  ls -A $sd/saves | grep -q '\.tmp' && { echo "FAIL: temp file left in $sd/saves"; fail=1; }
  grep -qE "refusing to write an invalid save|SAVE DAMAGED|not loaded completely|changed on disk" $log.txt && { echo "FAIL: the save layer refused or found damage"; fail=1; }
  # Seedy was saved twice after its creation (two exits): the first original and the previous version are kept
  [ -f $sd/saves/Seedy.d2s.bak ] && [ -f $sd/saves/Seedy.d2s.bak1 ] || { echo "FAIL: no .bak/.bak1 next to Seedy.d2s: $(ls -A $sd/saves)"; fail=1; }
  for f in $sd/saves/Seedy.d2s $sd/saves/Other.d2s; do
    python3 -I - "$f" <<'PY' || { echo "FAIL: $f is not a valid .d2s"; fail=1; }
import struct, sys
d = open(sys.argv[1], "rb").read()
assert struct.unpack("<I", d[0:4])[0] == 0xAA55AA55 and struct.unpack("<I", d[8:12])[0] == len(d)
s = 0
for i, b in enumerate(d):
    if 12 <= i < 16: b = 0
    s = (((s << 1) | (s >> 31)) & 0xFFFFFFFF) + b & 0xFFFFFFFF
assert s == struct.unpack("<I", d[12:16])[0], "checksum"
PY
  done
}
