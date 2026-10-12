scenario_name="mouse wheel skill cycling (press:WheelDown / WheelUp go through the input manager as wheel notches; the bound previous / next skill changes the right skill; Shift = left skill)"
wb=$tmp/writeback-w42-wheel
# InjectWheel -> key events of the wheel pseudo keys (d2input/wheel.go) -> GameControls.OnKeyDown -> key map ->
# SelectNextSkill / SelectPreviousSkill -> d2hero.CycleSkill. Three notches down then three up must come back to the start.
scenario_env() {
  mkdir -p $wb
  echo "export OD2_D2S_WRITEBACK=\"$wb\" OD2_AUTOMONSTER=\"zombie1,1\" OD2_AUTOMONSTER_PASSIVE=1 OD2_AUTOMONSTER_HOLD=1"
  local s='wait:2;skill:right=Fire Ball;skill:left=Attack'
  s="$s;press:WheelDown;press:WheelDown;press:WheelDown;press:WheelUp;press:WheelUp;press:WheelUp"
  s="$s;press:shift+WheelDown;press:shift+WheelUp;skill:use=right;wait:1;exit"
  echo "export OD2_AUTOSCRIPT='$s'"
}
scenario_check() {
  grep -E "SKILLBAR wheel|AUTOSCRIPT RESULT" $log.txt | cut -c1-200 | head -20
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: script did not pass"; fail=1; }
  # the right skill walked three skills on and the same three back
  fwd=("${(@f)$(grep 'SKILLBAR wheel hand=right dir=1 ' $log.txt | tr -d '\r' | sed 's/.*"\(.*\)" -> "\(.*\)"/\1>\2/')}")
  back=("${(@f)$(grep 'SKILLBAR wheel hand=right dir=-1 ' $log.txt | tr -d '\r' | sed 's/.*"\(.*\)" -> "\(.*\)"/\1>\2/')}")
  [ ${#fwd} -eq 3 ] && [ ${#back} -eq 3 ] || { echo "FAIL: want 3 notches each way, got ${#fwd} / ${#back}"; fail=1; return; }
  [ "${fwd[1]%%>*}" = "Fire Ball" ] || { echo "FAIL: first notch did not start from Fire Ball: ${fwd[1]}"; fail=1; }
  [ "${fwd[1]%%>*}" != "${fwd[1]##*>}" ] || { echo "FAIL: first notch did not change the skill"; fail=1; }
  [ "${fwd[1]##*>}" = "${fwd[2]%%>*}" ] && [ "${fwd[2]##*>}" = "${fwd[3]%%>*}" ] || { echo "FAIL: forward notches do not chain: $fwd"; fail=1; }
  [ "${back[1]%%>*}" = "${fwd[3]##*>}" ] || { echo "FAIL: first notch back did not start where the forward run ended"; fail=1; }
  [ "${back[3]##*>}" = "Fire Ball" ] || { echo "FAIL: three notches back did not return to Fire Ball: ${back[3]}"; fail=1; }
  # Shift + wheel is the left skill: Attack leaves and comes back
  grep -qE 'SKILLBAR wheel hand=left dir=1 "Attack" -> ' $log.txt || { echo "FAIL: shift+WheelDown did not step the left skill"; fail=1; }
  grep -qE 'SKILLBAR wheel hand=left dir=-1 .* -> "Attack"' $log.txt || { echo "FAIL: shift+WheelUp did not return the left skill to Attack"; fail=1; }
}
