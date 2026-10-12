scenario_name="cinematics menu screen (OD2_AUTOSCREEN=cinematics: the movie list of the exe, centred on the 800x600 column)"
scenario_timeout=120
scenario_env() {
  echo "unset OD2_AUTOGAME"
  echo "export OD2_AUTOSCREEN=cinematics OD2_AUTOSHOT=\"$tmp/cinemenu.png\" OD2_AUTOSHOT_SECONDS=5 OD2_AUTOEXIT=1"
}
scenario_check() {
  grep -E "AUTOSHOT|ERROR" $log.txt | cut -c1-200
  [ -s $tmp/cinemenu.png ] || { echo "FAIL: no screenshot"; fail=1; return; }
  [ -n "${OD2_KEEP_CAPTURES:-}" ] && cp $tmp/cinemenu.png "$OD2_KEEP_CAPTURES/cinemenu.png"
  grep -E "\[ERROR\]|panic" $log.txt | grep -v "skipping missing" && { echo "FAIL: errors in the log"; fail=1; }
}
