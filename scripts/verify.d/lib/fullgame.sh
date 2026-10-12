# Shared by the full-game chain scenarios (9w-fullgame-*): a fresh level 1 hero plays the game in segments of one game
# launch each; the exported .d2s of one segment is the only thing the next one starts from (a fresh config folder every
# time, so the .d2s alone has to carry quests, waypoints, difficulty and act). Wave 43, notes: d2-re-notes/w43-fullgame.md.
#
#   fg_init <dir>                       scratch folder of the chain (needs $tmp/od2, the built game)
#   fg_newchar <class> <name>           creates the hero (.d2s) through the hero creation path -> $fg_d2s
#   fg_seg <name> <script> [export...]  one game launch from $fg_d2s with that autoscript; afterwards $fg_d2s is the
#                                       exported .d2s of the segment and $fg_log.txt its log (ANSI stripped);
#                                       the logs of all segments are appended to $fg_all
#   fg_has <regex>                      grep in the logs of all segments
# Every segment runs OD2_TURBO=1 (virtual 25 Hz clock) unless OD2_TURBO=0.

fg_init() {
  fg_dir=$1; mkdir -p $fg_dir; fg_all=$fg_dir/all.txt; : > $fg_all; fg_n=0; fg_d2s=""
}

fg_launch() { # <command file>
  if [ -n "${OD2_VERIFY_LAUNCH:-}" ]; then ${=OD2_VERIFY_LAUNCH} $1 >/dev/null 2>&1 &
  elif launchctl asuser $(id -u) /usr/bin/true >/dev/null 2>&1; then launchctl asuser $(id -u) /bin/zsh $1 >/dev/null 2>&1 &
  else open -g $1
  fi
}

fg_wait() { # wait for this run's game to start and to end (killed after $1 seconds)
  local i lim=${1:-900}
  for i in {1..120}; do
    pgrep -f "$tmp/od2" >/dev/null && break
    [ $i -gt 5 ] && [ -s "${fg_log:-/nonexistent}" ] && break
    sleep 1
  done
  for i in {1..$lim}; do pgrep -f "$tmp/od2" >/dev/null || return 0; sleep 1; done
  echo "REAPED: segment $fg_name did not finish in ${lim}s, killing it"
  pkill -f "$tmp/od2"; sleep 1; pkill -9 -f "$tmp/od2" 2>/dev/null
  return 1
}

fg_cfg() { # a private config folder (a copy of a valid config.json) for one segment
  local c=$fg_dir/$fg_name.cfg
  rm -rf $c; mkdir -p $c
  cp "${OD2_VERIFY_CONFIG_DIR:-$HOME/Library/Application Support/OpenDiablo2}/config.json" $c/config.json
  echo $c
}

fg_newchar() { # <class> <name>
  fg_n=$((fg_n+1)); fg_name=$(printf '%02d-newchar' $fg_n)
  local wb=$fg_dir/$fg_name.wb cfg; cfg=$(fg_cfg)
  rm -rf $wb; mkdir -p $wb
  fg_log=$fg_dir/$fg_name.log; rm -f $fg_log
  {
    echo '#!/bin/zsh'
    echo "export OD2_PORT=$OD2_PORT OD2_CONFIG_DIR=\"$cfg\" OD2_AUTOTEST_MUTE=1 OD2_AUTONEWCHAR=$1 OD2_AUTONEWCHAR_NAME=$2 OD2_D2S_WRITEBACK=$wb OD2_AUTOEXIT=1 OD2_DISPLAY=${OD2_DISPLAY:-1512x982}"
    echo "$tmp/od2 > $fg_log 2>&1"
  } > $fg_dir/$fg_name.command
  chmod +x $fg_dir/$fg_name.command
  local slot; slot=$(./scripts/gameslot.sh acquire $$)
  fg_launch $fg_dir/$fg_name.command; fg_wait 120
  ./scripts/gameslot.sh release $slot
  sed 's/\x1b\[[0-9;]*m//g' $fg_log > $fg_log.txt; cat $fg_log.txt >> $fg_all
  fg_d2s=$(ls $wb/*.d2s(N) | head -1)
  [ -s "$fg_d2s" ] || { echo "FAIL: no new hero .d2s"; return 1; }
}

fg_seg() { # <name> <script> [export words...]
  fg_n=$((fg_n+1)); fg_name=$(printf '%02d-%s' $fg_n $1); local script=$2; shift 2
  local wb=$fg_dir/$fg_name.wb cfg in=$fg_dir/$fg_name.in.d2s; cfg=$(fg_cfg)
  rm -rf $wb; mkdir -p $wb; cp -f $fg_d2s $in
  fg_log=$fg_dir/$fg_name.log; rm -f $fg_log
  {
    echo '#!/bin/zsh'
    echo "export OD2_PORT=$OD2_PORT OD2_CONFIG_DIR=\"$cfg\" OD2_AUTOTEST_MUTE=1 OD2_AUTOEXIT=1 OD2_DISPLAY=${OD2_DISPLAY:-1512x982}"
    echo "export OD2_AUTOGAME=\"$in\" OD2_D2S_WRITEBACK=\"$wb\" OD2_AUTOSPEED=${OD2_FG_SPEED:-8}"
    [ "${OD2_TURBO:-1}" != 0 ] && echo "export OD2_TURBO=1"
    # the test aids of the chain (named in fullgame_scripts.sh): the hero cannot die, spends his points by a fixed plan,
    # repairs often enough that the axe does not break, and the runner waits for the long boss fights
    echo "export OD2_AUTOSCRIPT_HEAL=${FG_HEAL:-2} OD2_DURABILITY_CHANCE=${FG_DURABILITY:-1} OD2_AUTOSCRIPT_BUSY=${FG_BUSY:-4000}"
    echo "export OD2_AUTOBUILD='${FG_BUILD:-vit=3,str=2,dex=3|Bash:1,Axe Mastery:20,Iron Skin:20,Bash:20}'"
    local kv; for kv in "$@"; do echo "export ${kv%%=*}='${kv#*=}'"; done
    echo "export OD2_AUTOSCRIPT='$script'"
    echo "$tmp/od2 2>&1 | tee $fg_log"
  } > $fg_dir/$fg_name.command
  chmod +x $fg_dir/$fg_name.command
  local slot t0=$SECONDS; slot=$(./scripts/gameslot.sh acquire $$)
  fg_launch $fg_dir/$fg_name.command; fg_wait ${fg_timeout:-900}
  ./scripts/gameslot.sh release $slot
  echo "FG SEGMENT $fg_name wall=$((SECONDS - t0))s"
  sed 's/\x1b\[[0-9;]*m//g' $fg_log > $fg_log.txt; echo "=== SEGMENT $fg_name" >> $fg_all; cat $fg_log.txt >> $fg_all
  local out; out=$(ls $wb/*.d2s(N) | head -1)
  [ -s "$out" ] && fg_d2s=$out
  grep -q "AUTOSCRIPT RESULT PASS" $fg_log.txt
}

fg_has() { grep -qE "$1" $fg_all; }

# the map seed of the hero (from the last export of the last segment) and the level id of Tal Rasha's real tomb for it
fg_seed() { od -An -tu4 -j171 -N4 $fg_d2s | tr -d ' '; } # the map seed is the u32 at 0xAB of the .d2s
fg_tomb() { go run scripts/mapinfo.go tomb $(fg_seed) 2>/dev/null; }
# the road from Spider Forest to Lower Kurast of this seed: "76 77 78 79", or "76 78 79" when Great Marsh is a dead end
fg_jungle() { go run scripts/mapinfo.go jungle $(fg_seed) 2>/dev/null; }

# the exported .d2s line of the last save in the last segment
fg_last_export() { grep "D2S EXPORT reparse" $fg_log.txt | tail -1; }
