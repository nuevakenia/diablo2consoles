scenario_name="boss fights (wave 36: each boss fought by a scripted hero: skills cast, missiles that hit, curses, auras, kill, drops)"
scenario_turbo=1
scenario_timeout=900
# OD2_AUTOBOSSFIGHT (d2game/d2gamescreen/autobossfight.go): the boss is spawned beside a spectator hero that cannot die and
# takes its life down over OD2_BOSSFIGHT_SECONDS game seconds; "BOSSFIGHT summary" lists the monstats skills it cast (casts=)
# and the effects the wave 36 host produced (fx=: missiles:<skill> volleys, missilehit hero hits by those missiles,
# curse:<skill>, aura:<skill>). OD2_W36_BOSSES (monstats keys), OD2_W36_DIFF (0..2), OD2_W36_SECONDS, OD2_W36_RING pick them.
w36_bosses=${OD2_W36_BOSSES:-andariel,duriel,mephisto,diablo,baalcrab,summoner}
scenario_env() {
  echo "export OD2_NOMERC=1 OD2_AUTOBOSSFIGHT=$w36_bosses OD2_AUTOMONSTER_DIFF=${OD2_W36_DIFF:-2} OD2_BOSSFIGHT_SECONDS=${OD2_W36_SECONDS:-70} OD2_BOSSFIGHT_RING=${OD2_W36_RING:-26}"
}
scenario_check() {
  grep -E "BOSSFIGHT|MONSTER (curse|aura) " $log.txt | cut -c1-330
  grep -E "MONSTER attack name=[A-Za-z ]* id=[0-9]+ mode=[A-Za-z0-9]+ missile hit=true" $log.txt | head -4 | cut -c1-200
  grep -q "BOSSFIGHT RESULT PASS" $log.txt || { echo "FAIL: the boss fights did not pass"; fail=1; }
  # per boss: the monstats skills its AI must reach in a fight of this length, the effects that must show, a kill, a drop
  need() { # boss pattern...
    local b=$1; shift
    local line; line=$(grep "BOSSFIGHT summary boss=$b " $log.txt | head -1)
    [ -n "$line" ] || { echo "FAIL: no fight summary for $b"; fail=1; return; }
    local s
    for s in "$@"; do
      echo "$line" | grep -qE "$s" || { echo "FAIL: $b: no $s in the summary"; fail=1; }
    done
    echo "$line" | grep -q "killed=true" || { echo "FAIL: $b was not killed"; fail=1; }
    echo "$line" | grep -qE "drops=[1-9]" || { echo "FAIL: $b dropped nothing"; fail=1; }
  }
  local b
  for b in ${(s:,:)w36_bosses}; do
    case $b in
      andariel) need andariel "casts=[^ ]*AndrialSpray:" "missilehit:[1-9]" ;;
      duriel) need duriel "casts=[^ ]*Jab:" "casts=[^ ]*Smite:" "aura:Holy_Freeze" ;;
      mephisto) need mephisto "casts=[^ ]*PrimeLightning:" "missiles:PrimeBolt" "missilehit:[1-9]" ;;
      diablo) need diablo "casts=[^ ]*DiabLight:" "missilehit:[1-9]" ;;
      baalcrab) need baalcrab "casts=[^ ]*Baal_Inferno:" "curse:Defense_Curse" ;;
      summoner) need summoner "casts=[^ ]*Glacial_Spike:" "curse:Weaken" ;;
      *) need $b ;;
    esac
  done
  go test -count=1 ./d2core/d2monsters/ -run 'TestW36' 2>&1 | grep -v "ld: warning\|^# " | grep -v "^ok" && fail=1
}
