scenario_realtime=1
scenario_timeout=240
scenario_warnings_ok=1
scenario_name="w41 frame-time/alloc profile at 2560x1080 (OD2_PERF_W41_MODE=town|pack|automap; CPU/heap profiles to OD2_PPROF_CPU_OUT / OD2_PPROF_HEAP_OUT)"
# town: hero walks between NPCs; pack: Frigid Highlands with 120 monsters and the hero moving; automap: same with the full map on.
scenario_env() {
  local mode="${OD2_PERF_W41_MODE:-town}"
  echo "export OD2_DISPLAY=2560x1080 OD2_AUTOPERF=1 OD2_AUTOPERF_SECONDS=${OD2_PERF_W41_SECONDS:-25} OD2_AUTOPERF_WARMUP=6"
  [ -n "${OD2_PPROF_CPU_OUT:-}" ] && echo "export OD2_PPROF_CPU=$OD2_PPROF_CPU_OUT"
  [ -n "${OD2_PPROF_HEAP_OUT:-}" ] && echo "export OD2_PPROF_HEAP=$OD2_PPROF_HEAP_OUT"
  case $mode in
  town) echo "export OD2_AUTOSCRIPT='wait:1;move:npc=Akara;wait:7;panel:close;move:npc=Kashya;wait:7;panel:close;move:npc=Charsi;wait:7;panel:close;move:npc=Gheed;wait:7;panel:close;move:npc=Warriv;wait:7;exit'" ;;
  pack) echo "export OD2_AUTOLEVEL=111 OD2_AUTOMONSTER='fallen1,120' OD2_AUTOMONSTER_SECONDS=120 OD2_AUTOSCRIPT='wait:8;move:40,40;wait:8;move:70,40;wait:8;move:40,40;wait:8;exit'" ;;
  automap) echo "export OD2_AUTOLEVEL=111 OD2_AUTOMONSTER='fallen1,120' OD2_AUTOMONSTER_SECONDS=120 OD2_AUTOSCRIPT='wait:2;automap:full;wait:6;move:40,40;wait:8;move:70,40;wait:8;move:40,40;wait:8;exit'" ;;
  esac
}
scenario_check() {
  grep -E "PERF (update|render|frame|runtime|cpu |slow)|AUTOSCRIPT" $log.txt | cut -c1-220
}
