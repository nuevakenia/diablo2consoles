# Shared by the a2-menu-flow-* scenarios: the real menu flow (main menu, character select, hero creation, then in the
# game the escape menu and its options) at a given window size, with a screenshot at every stop. Checks, with
# scripts/shotcheck.go (rules in d2common/d2display/shotcheck.go, unit tested):
#   - every screenshot has the size of the window,
#   - the menu screens (main menu, character select, hero creation) show their 800x600 picture scaled to fit and
#     centred, with black bars (the non-black content is the centred 4:3 region),
#   - the escape menu and the options page (drawn over the game) are in a screenshot of the window size.
# menu_env W H prints the scenario_env lines;  menu_check W H  is the scenario_check.
menu_env() {
  local W=$1 H=$2 d=$tmp/menu$1x$2
  rm -rf $d; mkdir -p $d/saves $d/cfg
  cp "$D2S_SAMPLE_BODY" $d/saves/Sample.d2s
  cp "${OD2_VERIFY_CONFIG_DIR:-$HOME/Library/Application Support/OpenDiablo2}/config.json" $d/cfg/ 2>/dev/null
  echo "unset OD2_AUTOGAME OD2_D2S_WRITEBACK"
  echo "export OD2_DISPLAY=${W}x${H} OD2_CONFIG_DIR=\"$d/cfg\" OD2_D2S_DIR=\"$d/saves\""
  local f="mainmenu,shot:$d/1-main.png,single,shot:$d/2-charselect.png,new,shot:$d/3-create.png,create:Sorceress:Shotty,difficulty:0"
  # in the game (the new hero): the escape menu, its options page and the video options page (the menu is shown
  # by the flow: synthetic keys and clicks of OD2_AUTOSCRIPT do not reach the escape menu, which listens on the input manager)
  f="$f,wait:3,escmenu:main,shot:$d/4-escape.png,escmenu:options,shot:$d/5-options.png,escmenu:video,shot:$d/6-video.png,wait:1,exit"
  echo "export OD2_AUTOFLOW=\"$f\""
}

menu_check() {
  local W=$1 H=$2 d=$tmp/menu$1x$2
  grep -E "AUTOFLOW|NEWCHAR created" $log.txt | cut -c1-200 | head -20
  grep -qE "AUTOFLOW step [0-9]+ exit" $log.txt || { echo "FAIL: the flow did not reach its last step"; fail=1; }
  grep -q "AUTOFLOW escmenu" $log.txt && { echo "FAIL: an escape menu page of the flow does not exist"; fail=1; }
  grep -q "NEWCHAR created Shotty" $log.txt || { echo "FAIL: the hero was not created through the menu"; fail=1; }
  local f
  for f in 1-main 2-charselect; do
    go run scripts/shotcheck.go menu $d/$f.png $W $H || fail=1
  done
  go run scripts/shotcheck.go menu-top $d/3-create.png $W $H || fail=1   # the campfire ground at the bottom is black
  for f in 4-escape 5-options 6-video; do
    go run scripts/shotcheck.go size $d/$f.png $W $H || fail=1
  done
}
