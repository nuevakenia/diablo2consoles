scenario_name="map seed of the sample hero through the menu (log line, seed = .d2s header, same town TownE1 on a second load)"
# A copy of the sample .d2s is played from the character select twice (Save and Exit in between). Private config
# and save folders: nothing real is written.
scenario_warnings_ok=1
scenario_timeout=240
sd=$tmp/seedsample
sample_name=$(dd if="$D2S_SAMPLE_BODY" bs=1 skip=20 count=16 2>/dev/null | tr -d '\0')
scenario_env() {
  rm -rf $sd; mkdir -p $sd/saves $sd/cfg
  cp "$D2S_SAMPLE_BODY" $sd/saves/Sample.d2s
  cp "${OD2_VERIFY_CONFIG_DIR:-$HOME/Library/Application Support/OpenDiablo2}/config.json" $sd/cfg/ 2>/dev/null
  echo "unset OD2_AUTOGAME OD2_D2S_WRITEBACK OD2_AUTOMAPSEED"
  echo "export OD2_CONFIG_DIR=\"$sd/cfg\" OD2_D2S_DIR=\"$sd/saves\""
  local one="select:$sample_name,play,difficulty:0,wait:5,saveexit"
  echo "export OD2_AUTOFLOW=\"single,$one,$one,exit\""
}
scenario_check() {
  grep -E "map seed of|real town:|AUTOFLOW saveexit" $log.txt | cut -c1-220
  want=$(python3 -I -c 'import struct,sys; print(struct.unpack("<I", open(sys.argv[1],"rb").read()[0xAB:0xAF])[0])' "$D2S_SAMPLE_BODY")
  seeds=("${(@f)$(grep -o "map seed of $sample_name: [0-9]*" $log.txt | tr -d '\r' | sed 's/.*: //')}")
  [ ${#seeds} -eq 2 ] || { echo "FAIL: want 2 'map seed of $sample_name' lines, got ${#seeds}"; fail=1; return; }
  [ "$seeds[1]" = "$want" ] || { echo "FAIL: first load used seed $seeds[1], the save has $want"; fail=1; }
  [ "$seeds[2]" = "$want" ] || { echo "FAIL: second load used seed $seeds[2], the save has $want"; fail=1; }
  towns=("${(@f)$(grep -o "real town: .*" $log.txt | tr -d '\r')}")
  [ ${#towns} -eq 4 ] || { echo "FAIL: want 4 town lines (client and server, two loads), got ${#towns}"; fail=1; return; }
  [ "$towns[1]" = "$towns[3]" ] && [ "$towns[2]" = "$towns[4]" ] || { echo "FAIL: the town differs between the two loads:"; echo " 1: $towns[1]"; echo " 3: $towns[3]"; fail=1; }
  echo "$towns[1]" | grep -q "TownE1" || { echo "FAIL: the sample hero's town is not TownE1: $towns[1]"; fail=1; }
  grep -q "AUTOFLOW step [0-9]* exit" $log.txt || { echo "FAIL: the flow did not reach its end"; fail=1; }
}
