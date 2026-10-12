scenario_name="held Inferno (the right button is held: one wind-up run, then flames while held; each Do schedules the action end InfernoLen frames later; releasing stops the casts)"
scenario_realtime=1
wb=$tmp/writeback-w42-inferno
# The channel (d2common/d2skill/channel.go): the first run of a hold makes no flame (CHANNEL first=true), the runs after it
# do (first=false, CAST do ... missiles>=1) as long as the button repeats. SRVDO_InfernoScheduleAnimEnd (0x5c6cb0,
# VERIFIED w39): the action ends InfernoLen frames after each Do (CHANNEL do infernolen=N action_end=F, then CHANNEL
# action-end frame=F). The owner's tables hold 1 for the seven heroes. Outdoors (Blood Moor): Inferno cannot be cast in town.
scenario_env() {
  mkdir -p $wb
  echo "export OD2_D2S_WRITEBACK=\"$wb\" OD2_AUTOLEVEL=2 OD2_AUTOMONSTER=\"zombie1,1\" OD2_AUTOMONSTER_PASSIVE=1 OD2_AUTOMONSTER_HOLD=1"
  local s='wait:2;say:learnskillid 41;skill:right=Inferno;hold:4,right@hero:150,30;wait:1.5;say:heropos;exit'
  echo "export OD2_AUTOSCRIPT='$s'"
}
scenario_check() {
  grep -E "CHANNEL|HOLD (start|end)|CAST (start|do) skill=\"Inferno\"|AUTOSCRIPT RESULT" $log.txt | cut -c1-200 | head -40
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: script did not pass"; fail=1; }
  first=$(grep -c 'CHANNEL skill="Inferno" first=true' $log.txt)
  cont=$(grep -c 'CHANNEL skill="Inferno" first=false' $log.txt)
  [ "$first" -eq 1 ] || { echo "FAIL: want exactly one wind-up run in one hold, got $first (the flame must persist, not restart)"; fail=1; }
  [ "$cont" -ge 6 ] || { echo "FAIL: the flame did not persist while held (continuation runs=$cont)"; fail=1; }
  # the first Do of the hold makes no flame, later ones do
  flames=$(grep 'CAST do skill="Inferno" ' $log.txt | grep -c 'missiles=[1-9]')
  [ "$flames" -ge 6 ] && [ "$flames" -eq "$cont" ] || { echo "FAIL: every run after the wind-up must make a flame (flames=$flames continuation runs=$cont)"; fail=1; }
  grep 'CAST do skill="Inferno" ' $log.txt | head -1 | grep -q 'missiles=0' || { echo "FAIL: the wind-up Do made a flame"; fail=1; }
  # the exe's rule: the action end is InfernoLen frames after the Do, taken from the tables (1 for the heroes)
  dos=$(grep -c 'CHANNEL do skill="Inferno"' $log.txt)
  [ "$dos" -ge 3 ] || { echo "FAIL: want 3 or more channel Dos, got $dos"; fail=1; }
  grep 'CHANNEL do skill="Inferno"' $log.txt | grep -qvE 'infernolen=1 ' && { echo "FAIL: infernolen is not the 1 of the hero tables"; fail=1; }
  grep -E 'CHANNEL (do|action-end) skill="Inferno"' $log.txt | tr -d '\r' | awk '
    /CHANNEL do/ { match($0, /frame=[0-9]+/); f=substr($0, RSTART+6, RLENGTH-6); match($0, /infernolen=[0-9]+/); l=substr($0, RSTART+11, RLENGTH-11); expect=f+l; pending=1; next }
    /CHANNEL action-end/ { match($0, /frame=[0-9]+/); g=substr($0, RSTART+6, RLENGTH-6); if (pending && g != expect) { bad=1; print "action-end at " g " want " expect } ; pending=0; ends++ }
    END { if (bad || ends < 3) { print "action-ends=" ends; exit 1 } }' || { echo "FAIL: action ends do not follow Do + InfernoLen"; fail=1; }
  # releasing the button stops the casts: no cast start after HOLD end
  after=$(awk '/HOLD end/ {e=1; next} e && /CAST start skill="Inferno"/ {n++} END {print n+0}' $log.txt)
  [ "$after" -eq 0 ] || { echo "FAIL: $after Inferno casts started after the button was released"; fail=1; }
}
