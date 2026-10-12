scenario_name="object sounds (SOUNDLOG: the waypoint plays object_waypoint_open when it is first activated, a town portal plays the cast sound, the open sound of the portal object and the enter sound when it is used; doors play their wood / gate / stone sound on open and close)"
# UNVERIFIED in the exe (w36): which row each object plays; the rows are the ones named after the objects.
scenario_env() {
  echo "export OD2_SOUNDLOG=1 OD2_AUTOSCRIPT='wait:2;say:setwaypoint 1 0;use:Waypoint;waitlog:WAYPOINT activated level=1;say:spawnchest 13 47 91;wait:1;use:13;wait:1;use:13;wait:1;use:47;wait:1;use:91;wait:1;say:spawnportal 3;use:Portal;waitlog:LEVEL built: level 3;wait:2;say:townportal free;waitlog:PORTAL object level=3;wait:6;use:Portal;waitlog:PORTAL used;wait:2;exit'"
}
scenario_check() {
  echo "object sounds (kind handle: count):"
  grep -oE "SOUNDLOG t=[0-9.]+ kind=(waypoint|door|portal-[a-z]+) music=[a-z]+ handle=[a-z0-9_]+" $log.txt | sed -E 's/t=[0-9.]+ //' | sort | uniq -c
  local ok='decision=(played|stolen|queued|inaudible)'
  grep -qE "SOUNDLOG .* kind=waypoint .*handle=object_waypoint_open .*$ok" $log.txt || { echo "FAIL: no waypoint activate sound"; fail=1; }
  grep -qE "SOUNDLOG .* kind=portal-cast .*handle=player_townportal_cast .*$ok" $log.txt || { echo "FAIL: no town portal cast sound"; fail=1; }
  grep -qE "SOUNDLOG .* kind=portal-open .*handle=object_townportal .*$ok" $log.txt || { echo "FAIL: no town portal open sound"; fail=1; }
  grep -qE "SOUNDLOG .* kind=portal-enter .*handle=player_townportal_enter .*$ok" $log.txt || { echo "FAIL: no town portal enter sound"; fail=1; }
  grep -qE "SOUNDLOG .* kind=door .*handle=object_door_wood_open .*$ok" $log.txt || { echo "FAIL: no wooden door open sound"; fail=1; }
  grep -qE "SOUNDLOG .* kind=door .*handle=object_door_wood_close .*$ok" $log.txt || { echo "FAIL: no wooden door close sound"; fail=1; }
  grep -qE "SOUNDLOG .* kind=door .*handle=object_door_gate_open .*$ok" $log.txt || { echo "FAIL: no gate open sound"; fail=1; }
  grep -qE "SOUNDLOG .* kind=door .*handle=object_door_stone .*$ok" $log.txt || { echo "FAIL: no stone door sound"; fail=1; }
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: script did not finish"; fail=1; }
}
scenario_unmuted=0
scenario_timeout=300
