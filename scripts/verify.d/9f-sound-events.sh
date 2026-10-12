scenario_name="sound events (SOUNDLOG: ground drops play item_flippy then the drop sound later, a pickup plays item_pickup, a skill click goes through the engine, zombies get the HitClass impact layer on hit and death)"
scenario_env() { echo 'export OD2_SOUNDLOG=1 OD2_AUTOMONSTER="zombie1,2" OD2_AUTOMONSTER_SECONDS=40 OD2_AUTOMONSTER_DIFF=0 OD2_AUTOCAST="Fire Bolt,4" OD2_AUTOGROUND=1,4 OD2_AUTOPICKUP=1 OD2_AUTOGROUND_HOLD=40'; }
scenario_check() {
  echo "sounds started (kind handle: count):"
  grep -oE "SOUNDLOG t=[0-9.]+ kind=[a-z-]+ music=[a-z]+ handle=[a-z0-9_]+" $log.txt | sed -E 's/t=[0-9.]+ //' | sort | uniq -c | sort -rn | head -30
  grep -qE "SOUNDLOG .* kind=drop .*handle=item_flippy .*decision=" $log.txt || { echo "FAIL: no item_flippy on a ground drop"; fail=1; }
  grep -qE "SOUNDLOG .* kind=drop .*handle=(item|cursor)_[a-z0-9_]+ " $log.txt || { echo "FAIL: no item drop sound"; fail=1; }
  grep -qE "SOUNDLOG .* kind=pickup .*handle=item_pickup " $log.txt || { echo "FAIL: pickup did not play item_pickup"; fail=1; }
  grep -qE "SOUNDLOG .* kind=pickup .*handle=cursor_point_drop" $log.txt && { echo "FAIL: old pickup sound still played"; fail=1; }
  # zombie1: monstats2 HitClass 12 -> Sounds.txt row 0x145 (impact_claw_1.. family, body layer)
  grep -qE "SOUNDLOG .* kind=impact .*handle=impact_claw_[0-9]+ id=32[5-8] " $log.txt || { echo "FAIL: no HitClass impact layer (impact_claw group)"; fail=1; }
  grep -qE "SOUNDLOG .* kind=death " $log.txt || { echo "FAIL: no monster death sound"; fail=1; }
}
scenario_unmuted=0
scenario_timeout=300
