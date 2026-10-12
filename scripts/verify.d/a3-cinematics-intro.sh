scenario_name="cinematics intro and menu (OD2_PLAY_VIDEOS=1: the opening movie letterboxed on a 2560x1080 window and on the default window, the Cinematics menu screen)"
scenario_timeout=240
scenario_realtime=1
# Needs the user's d2video.mpq through the config. Three runs of the game would be needed for three pictures, so this
# scenario takes the intro frame at 2560x1080 (pillarbox: the picture is height limited) and the Cinematics menu; the
# default window is covered by a3-cinematics-ingame.
scenario_env() {
  echo "unset OD2_AUTOGAME OD2_AUTOSPEED"
  echo "export OD2_PLAY_VIDEOS=1 OD2_DISPLAY=2560x1080 OD2_AUTOSHOT=\"$tmp/intro.png\" OD2_AUTOSHOT_SECONDS=30 OD2_AUTOEXIT=1"
}
scenario_check() {
  grep -E "playing .*\.bik|AUTOSHOT|VIDEO end" $log.txt | cut -c1-240
  grep -q "playing .*d2intro.*audio track.*clock: audio" $log.txt || { echo "FAIL: the intro did not start with an audio clock"; fail=1; }
  [ -s $tmp/intro.png ] || { echo "FAIL: no screenshot"; fail=1; return; }
  [ -n "${OD2_KEEP_CAPTURES:-}" ] && cp $tmp/intro.png "$OD2_KEEP_CAPTURES/intro-2560.png"
  dims=$(python3 -I -c 'import struct,sys; b=open(sys.argv[1],"rb").read(24); print(*struct.unpack(">II", b[16:24]))' $tmp/intro.png)
  [ "$dims" = "2560 1080" ] || { echo "FAIL: screenshot is $dims, want 2560 1080"; fail=1; }
}
