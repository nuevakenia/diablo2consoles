scenario_name="area entry hero voice (more than 61 ticks after a level change the hero says the find line of the level's quest: Blood Moor -> act1_find_wilderness, once per record)"
# VERIFIED in the exe (w37): SOUND_PlayAreaEntryVoiceOnLevelChange 0x4c8d10, table 0x728d40 (d2audio.AreaVoices).
scenario_env() {
  echo "export OD2_SOUNDLOG=1 OD2_AUTOSCRIPT='wait:2;say:resetquests;wait:1;say:spawnportal 2;use:Portal;waitlog:LEVEL built: level 2;waitlog:AREAVOICE level=2;wait:3;say:spawnportal 3;use:Portal;waitlog:LEVEL built: level 3;wait:6;exit'"
}
scenario_check() {
  echo "area voice lines:"
  grep -E "AREAVOICE|HEROVOICE" $log.txt | cut -c1-170 | head -8
  grep -qE "AREAVOICE level=2 .*voice=0x2f quest=1" $log.txt || { echo "FAIL: no area voice for Blood Moor"; fail=1; }
  grep -qE "SOUNDLOG .* kind=hero-voice .*handle=[a-z]+_act1_find_wilderness" $log.txt || { echo "FAIL: the find_wilderness line never reached the sound bank"; fail=1; }
  # level 3 is part of the same record: it was used up by the visit to level 2
  grep -qE "AREAVOICE level=3 " $log.txt && { echo "FAIL: Cold Plains said its line again"; fail=1; }
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: script did not finish"; fail=1; }
}
scenario_unmuted=0
scenario_timeout=300
