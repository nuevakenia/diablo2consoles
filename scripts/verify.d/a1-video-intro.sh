scenario_name="video intro (OD2_PLAY_VIDEOS: Bink video + audio decoded and streamed for 5 s, picture follows the audio clock, no underruns)"
scenario_timeout=120
scenario_realtime=1
# The audio device pulls the soundtrack at real time, so this runs at the real clock. OD2_AUTOTEST_MUTE (the default
# of verify.sh) plays the stream at volume 0: the clock, sample counts and underruns are still real. Needs the
# user's d2video.mpq through the config (the intro is read from the game data, never stored in the repository).
scenario_env() {
  echo "unset OD2_AUTOGAME OD2_AUTOSPEED"
  echo "export OD2_PLAY_VIDEOS=1 OD2_VIDEO_SECONDS=5"
}
scenario_check() {
  grep -E "playing .*\.bik|VIDEO end" $log.txt | cut -c1-260
  grep -q "playing .*audio track(s), clock: audio" $log.txt || { echo "FAIL: the intro did not start with an audio clock"; fail=1; }
  line=$(grep "VIDEO end audio_decoded=" $log.txt | tail -1)
  [ -n "$line" ] || { echo "FAIL: no VIDEO end line"; fail=1; return; }
  decoded=$(echo "$line" | sed -E 's/.*audio_decoded=([0-9]+).*/\1/')
  played=$(echo "$line" | sed -E 's/.*played=([0-9]+).*/\1/')
  under=$(echo "$line" | sed -E 's/.*underruns=([0-9]+).*/\1/')
  bad=$(echo "$line" | sed -E 's/.*bad_packets=([0-9]+).*/\1/')
  # 5 s at 44.1 kHz = 220500 sample frames played (plus the device latency); the decoder runs 0.6 s ahead of that
  [ "$played" -ge 200000 ] && [ "$played" -le 260000 ] || { echo "FAIL: played $played sample frames, want about 5 s (220500)"; fail=1; }
  [ "$decoded" -ge "$played" ] && [ "$decoded" -le 320000 ] || { echo "FAIL: decoded $decoded sample frames vs played $played"; fail=1; }
  [ "$under" = 0 ] || { echo "FAIL: $under audio underruns"; fail=1; }
  [ "$bad" = 0 ] || { echo "FAIL: $bad audio packets failed to decode"; fail=1; }
}
