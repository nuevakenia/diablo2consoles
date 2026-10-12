scenario_name="cinematics in the game (Act 1 -> 2 trip with OD2_PLAY_VIDEOS=1: the Act II movie plays over the game, letterboxed, the game stands still, skipmovie ends it and the trip goes on to Lut Gholein)"
scenario_realtime=1
scenario_timeout=300
# Needs the user's d2video.mpq (through the config); frames are captured with capframe and read by eye, the movie is
# never stored in the repository. The soundtrack runs muted (OD2_AUTOTEST_MUTE) but the audio clock is real.
cg=$tmp/cinegame
scenario_env() {
  mkdir -p $cg/s94 $cg/wb94; rm -f $cg/s94/*.d2s(N) $cg/wb94/*.d2s(N)
  if [ -n "${D2_TABLES:-}" ] && go run scripts/d2s-revive.go "$D2S_SAMPLE_BODY" $cg/s94/Hero.d2s "$D2_TABLES" >/dev/null 2>&1; then
    echo "export OD2_AUTOGAME=\"$cg/s94/Hero.d2s\" OD2_D2S_WRITEBACK=\"$cg/wb94\""
  fi
  echo 'export OD2_REALMAPS=1 OD2_AUTOLEVEL=2 OD2_PLAY_VIDEOS=1'
  local s="expect:level=2;wait:1;say:resetquests;say:completequest 1 6;travel:2;wait:2;say:capframe $tmp/cine-a.png;wait:3;say:capframe $tmp/cine-b.png;say:skipmovie;wait:2;expect:level=40;wait:3;say:capframe $tmp/cine-after.png;exit"
  echo "export OD2_AUTOSCRIPT='$s'"
}
scenario_check() {
  grep -E "CINEMATIC|MOVIE|playing .*\.bik|VIDEO end|AUTOSCRIPT RESULT" $log.txt | cut -c1-240 | head
  grep -q "CINEMATIC act 1 -> 2 movie=2 .*playback=true" $log.txt || { echo "FAIL: no Act II movie for the trip"; fail=1; }
  grep -q "MOVIE play row=2" $log.txt || { echo "FAIL: the movie did not start"; fail=1; }
  grep -q "playing .*act02start.*audio track" $log.txt || { echo "FAIL: the Act II movie was not opened"; fail=1; }
  grep -q "MOVIE end .*skipped=true" $log.txt || { echo "FAIL: the movie did not end by skipmovie"; fail=1; }
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: the script did not pass (level 40 after the movie)"; fail=1; }
  for f in cine-a cine-b cine-after; do
    [ -s $tmp/$f.png ] || { echo "FAIL: $f.png missing"; fail=1; }
    [ -n "${OD2_KEEP_CAPTURES:-}" ] && cp $tmp/$f.png "$OD2_KEEP_CAPTURES/$f.png"
  done
}
