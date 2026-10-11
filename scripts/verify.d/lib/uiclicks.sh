# Shared by the UI click scenarios (8g-ui-*): clicks on the interface through the input manager at a given window size.
# Positions are @ui:x,y in the 800x600 interface column (the rectangles of scripts/verify.d/ui-layout.golden, which the interface
# maps to the real window size), so the same script is valid at 1512x982 and at 1920x1080. Waits are on the "INPUT panels after click"
# lines the clicks log (they show which panels are open after the click), never on a clock.
#
#   ui_clicks_env <WxH>     prints the shell lines for scenario_env (the display size and the script)
#   ui_clicks_check         the scenario_check
ui_clicks_env() {
  echo "export OD2_DISPLAY=$1"
  local P='INPUT panels after click'
  local s='say:heropos'
  # 1 the skill buttons open their menu and a second click closes it
  s="$s;click:left@ui:141,576;until:skillmenu=true,60;click:left@ui:141,576;until:skillmenu=false,60"
  s="$s;click:left@ui:659,576;until:skillmenu=true,60;click:left@ui:659,576;until:skillmenu=false,60"
  # 2 the mini panel: its toggle shows the strip, the buttons open the character panel and the inventory; each is closed again
  # (the strip moves aside while a panel is open, so the next button is only clicked once the strip is back)
  s="$s;click:left@ui:399,572;wait:0.5"
  s="$s;click:left@ui:336,540;until:character=true,60;click:left@ui:224,464;until:character=false,60"
  # a right panel (inventory, skill tree) moves the strip 131 pixels to the left while it is open
  s="$s;click:left@ui:357,540;until:inventory=true,60;click:left@ui:226,540;until:inventory=false,60"
  s="$s;click:left@ui:378,540;until:skills=true,60;click:left@ui:247,540;until:skills=false,60"
  # 3 the stash object opens the stash (a left panel); its close button closes it
  s="$s;say:spawnchest 267 267 267;until:AUTOGROUND chest-spawn id=267,30;click:left@object:267;until:stash opened,120;wait:0.5;click:left@ui:352,464;until:stash=false,60"
  # 4 the belt (the stash opened the inventory with it): a click on a potion picks it up, a second click puts it back
  s="$s;click:left@ui:437,576;until:cursor_item=true,60;click:left@ui:437,576;until:cursor_item=false,60"
  s="$s;click:left@ui:357,540;until:inventory=false,60"
  echo "export OD2_AUTOSCRIPT='$s;say:heropos;exit'"
}
ui_clicks_check() {
  grep -E "INPUT panels after click|INPUT world|AUTOSCRIPT RESULT|AUTOSCRIPT step [0-9]* FAIL" $log.txt | cut -c1-260 | head -60
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: ui click script did not pass"; fail=1; }
  # every click lands on the interface: none of them may become a click on the world (a walk or a cast)
  [ "$(grep -c 'INPUT world-click' $log.txt)" = 0 ] || { echo "FAIL: a click on the interface reached the world"; fail=1; }
  # each panel was seen open by a click and the last state has everything closed again
  for k in skillmenu character inventory skills cursor_item; do
    grep "INPUT panels after click" $log.txt | grep -q " $k=true" || { echo "FAIL: no click opened $k"; fail=1; }
  done
  grep "INPUT panels after click" $log.txt | tail -1 | grep -q "inventory=false character=false skills=false .* stash=false .* cursor_item=false skillmenu=false" \
    || { echo "FAIL: panels still open after the last click"; fail=1; }
  grep -q "stash opened" $log.txt || { echo "FAIL: the stash object did not open the stash"; fail=1; }
}
