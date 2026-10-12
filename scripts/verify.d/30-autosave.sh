scenario_name="autosave (OD2_AUTOSAVE=1: set gold in game, exit, exported .d2s re-parses)"
wb=$tmp/writeback
scenario_env() {
  mkdir -p $wb   # keeps the exported file out of the Saves folder
  echo "export OD2_AUTOSAVE=1 OD2_D2S_WRITEBACK=\"$wb\""
}
scenario_check() {
  grep -E "D2S EXPORT|AUTOSCRIPT RESULT" $log.txt | cut -c1-300
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: autosave script did not pass"; fail=1; }
  grep -q "D2S EXPORT reparse: .*gold=31337 .*checksum=ok" $log.txt || { echo "FAIL: exported .d2s does not show the new gold"; fail=1; }
  ls $wb/*.d2s >/dev/null 2>&1 || { echo "FAIL: no exported .d2s in $wb"; fail=1; }
  # w37 save integrity: no temp files, no refused or damaged saves; from the second save on the previous version is kept
  ls -A $wb | grep -q '\.tmp' && { echo "FAIL: temp file left in $wb"; fail=1; }
  grep -qE "refusing to write an invalid save|SAVE DAMAGED|not loaded completely|changed on disk" $log.txt && { echo "FAIL: the save layer refused or found damage"; fail=1; }
  saves=$(grep -c "D2S EXPORT path=" $log.txt)
  echo "saves in this run: $saves"
  if [ "$saves" -ge 2 ]; then
    ls $wb/*.d2s.bak1(N) >/dev/null 2>&1 || { echo "FAIL: $saves saves but no .bak1 of the previous version"; fail=1; }
  fi
  for f in $wb/*.d2s(N) $wb/*.d2s.bak*(N); do
    [ -f "$f" ] || continue
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
