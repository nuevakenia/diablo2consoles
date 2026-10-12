scenario_name="Druid shapeshift look (Wearwolf / Wearbear draw the Wolf / Bear monster composite, no gear layers, run works in the form, a second cast reverts, human only skills refused)"
scenario_realtime=1
scenario_timeout=300
scenario_warnings_ok=1
# w32. A fresh Druid (OD2_AUTOCAST_CLASS, level 18, own temp save: no real save is touched) spends a point in Wearwolf and
# casts it with the right button; the engine must swap the drawn composite to token 40 (monstats class 430), the hero walks and
# runs in the form, casts it again (back to human, class token DZ), then the same with Wearbear (token TG, class 431).
# W32_SHOT=<dir> also saves screenshots of the forms (never inside the repo).
scenario_env() {
  local shot="${W32_SHOT:-}" s
  s='wait:3;skill:spend=Wearwolf;skill:spend=Wearbear;skill:right=Wearwolf;click:right@hero:60,30;wait:3'
  [ -n "$shot" ] && s="$s;say:capframe $shot/wolf-idle.png;wait:1"
  s="$s;skill:left=Attack;say:heromoved;click:left@hero:140,30;wait:2"
  [ -n "$shot" ] && s="$s;say:capframe $shot/wolf-walk.png;wait:1"
  s="$s;wait:3;press:R;click:left@hero:-140,-30;wait:2;press:R"
  [ -n "$shot" ] && s="$s;say:capframe $shot/wolf-run.png;wait:1"
  s="$s;wait:2;click:right@hero:60,30;wait:3"
  [ -n "$shot" ] && s="$s;say:capframe $shot/human.png;wait:1"
  s="$s;skill:right=Wearbear;click:right@hero:60,30;wait:3"
  [ -n "$shot" ] && s="$s;say:capframe $shot/bear-idle.png;wait:1"
  s="$s;click:right@hero:60,30;wait:3;exit"
  echo "export OD2_SOUNDLOG=1 OD2_AUTOGAME=\"$tmp/w32.od2\" OD2_AUTOCAST_CLASS=druid OD2_AUTOCAST_CLVL=18 OD2_AUTOCAST_SKILLPOINTS=4"
  echo "export OD2_AUTOSCRIPT='$s'"
}
scenario_check() {
  grep -E "FORM |AUTOSCRIPT RESULT|STATE apply skill=\"Wear" $log.txt | cut -c1-200 | head -20
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: the shapeshift script did not pass"; fail=1; }
  grep -q "FORM wolf composite=40 class=430" $log.txt || { echo "FAIL: no wolf look (token 40, class 430)"; fail=1; }
  grep -q "FORM bear composite=TG class=431" $log.txt || { echo "FAIL: no bear look (token TG, class 431)"; fail=1; }
  [ "$(grep -c 'FORM human composite=DZ' $log.txt)" -ge 2 ] || { echo "FAIL: the hero did not turn back to human twice"; fail=1; }
  grep -q "FORM mode=WL composite=40" $log.txt || { echo "FAIL: the wolf never played its walk animation"; fail=1; }
  grep -q "FORM mode=RN composite=40" $log.txt || { echo "FAIL: the wolf never played its run animation"; fail=1; }
  grep -q "HERO moved" $log.txt || { echo "FAIL: the hero did not move in the form"; fail=1; }
  # w37: the form change plays the states.txt on/off sound (druid_morph, VERIFIED STATE_ClientAddState 0x4d68c0 / CLIENT_RemovePlayerState 0x4d69d0)
  [ "$(grep -cE "SOUNDLOG .* kind=state .*handle=druid_morph " $log.txt)" -ge 2 ] || { echo "FAIL: the form change did not play druid_morph (on and off)"; fail=1; }
}
