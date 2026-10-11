scenario_name="keyboard through the input manager (belt hotkeys 1-4, skill hotkeys F1-F8, I C T A Q Tab panels, run toggle R, Esc menu chosen with the mouse)"
scenario_realtime=1
scenario_timeout=240
# Every key is injected like a window key event (InputInjector), every wait is on the line the key's effect logs: "INPUT panels after <key>"
# (the panel state after each injected key, in the Game Screen log), "BELT use slot=", "SKILLBAR select" and the escape menu's
# "MENU show layout=" / "MENU closed" (click:left@menu:<row> clicks the row of the page that is showing).
scenario_env() {
  local k='INPUT panels after'
  local s='say:heropos'
  # belt columns 1 to 4: each key drinks the front potion of its column (the sample hero has potions in all four)
  for n in 1 2 3 4; do s="$s;press:$n;until:BELT use slot=$n,30"; done
  # skill hotkeys: the sample hero has skills on F1 to F7, F8 is empty and selects nothing
  for n in 1 2 3 4 5 6 7; do s="$s;press:F$n;until:SKILLBAR select,30"; done
  s="$s;press:F8;until:$k F8,30"
  # panels: each key opens, the same key closes
  for p in "I:inventory" "C:character" "T:skills" "Q:quest" "A:character"; do
    key=${p%%:*}; st=${p##*:}
    s="$s;press:$key;until:$st=true,30;press:$key;until:$st=false,30"
  done
  s="$s;press:Tab;until:automap=true,30;press:Tab;until:automap=false,30"
  # run toggle
  s="$s;press:R;until:running=true,30;press:R;until:running=false,30"
  # shift+F1 (the second skill set key) and a modified key reach the handlers too
  s="$s;press:shift+F1;until:$k shift+F1,30"
  # the escape menu: Esc opens it, the mouse picks OPTIONS (row 0), PREVIOUS MENU (last row of the options page, 6) and RETURN TO GAME (2)
  s="$s;press:Escape;until:menu=true,30;click:left@menu:0;until:MENU show layout,30;click:left@menu:6;until:MENU show layout,30;click:left@menu:2;until:MENU closed,30"
  echo "export OD2_AUTOSCRIPT='$s;say:heropos;exit'"
}
scenario_check() {
  grep -E "INPUT panels after|BELT use|SKILLBAR select|MENU |AUTOSCRIPT RESULT|AUTOSCRIPT step [0-9]* FAIL" $log.txt | cut -c1-200 | head -90
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: keyboard script did not pass"; fail=1; }
  body=$(grep -v AUTOSCRIPT $log.txt)
  for n in 1 2 3 4; do echo "$body" | grep -q "BELT use slot=$n " || { echo "FAIL: belt hotkey $n drank nothing"; fail=1; }; done
  [ "$(echo "$body" | grep -c 'SKILLBAR select hand=')" = 8 ] || { echo "FAIL: want 8 skill selections (F1 to F7, shift+F1)"; fail=1; }
  for k in inventory character skills quest automap running; do
    echo "$body" | grep "INPUT panels after" | grep -q " $k=true" || { echo "FAIL: no key turned $k on"; fail=1; }
  done
  echo "$body" | grep -q "INPUT panels after Escape: .* menu=true" || { echo "FAIL: Esc did not open the menu"; fail=1; }
  [ "$(echo "$body" | grep -c 'MENU show layout')" = 3 ] || { echo "FAIL: the mouse should have changed the menu page 3 times (options, back, return)"; fail=1; }
  [ "$(echo "$body" | grep -c 'MENU closed')" = 1 ] || { echo "FAIL: the menu should have been closed once by RETURN TO GAME"; fail=1; }
  echo "$body" | grep "INPUT panels after" | tail -1 | grep -q "menu=false" || { echo "FAIL: the menu is still open at the end"; fail=1; }
}
