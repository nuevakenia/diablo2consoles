scenario_name="friendly fire (area skills, curses and auras cast over the hero's own golem: no damage or state on it; enemies are still hit)"
# w36: a fresh Necromancer summons a Clay Golem, then casts area skills (Meteor, Blizzard, Frozen Orb, Nova, Poison Nova, Fire Wall,
# Hydra, Chain Lightning, Fire Ball, Holy Fire) and curses (Amplify Damage, Lower Resist, Terror) at zombies. The golem fights beside
# the hero, so the areas land on it. The exe never hits a unit that UNIT_AreUnitsHostile (0x552270) calls non-hostile.
scenario_env() {
  local list="Clay Golem,1;Meteor,10;Blizzard,10;Frozen Orb,10;Nova,10;Poison Nova,10;Fire Wall,10;Hydra,10;Chain Lightning,10;Fire Ball,10;Holy Fire,10;Amplify Damage,5;Lower Resist,5;Terror,5"
  echo "export OD2_AUTOCAST_CLASS=necromancer OD2_AUTOCAST_MANA=2000 OD2_AUTOCAST_LEVEL=10 OD2_AUTOMONSTER=\"zombie1,5\" OD2_AUTOMONSTER_SECONDS=300"
  echo "export OD2_AUTOCAST=\"$list\""
}
scenario_check() {
  local names n label bad
  grep -E "AUTOCAST (skill_done|summary)" $log.txt | cut -c1-200 | head -20
  grep -qE "MINION spawn name=" $log.txt || { echo "FAIL: no minion was summoned"; fail=1; }
  grep -qE "DAMAGE skill=.* target=Zombie" $log.txt || { echo "FAIL: the area skills hurt no enemy"; fail=1; }
  # every summoned unit label (the golem): no damage line and no state on it from the hero
  names=("${(@f)$(grep -oE 'MINION spawn name=[^=]*( key=)' $log.txt | sed 's/MINION spawn name=//; s/ key=$//' | sort -u)}")
  for label in $names; do
    [ -n "$label" ] || continue
    n=$(grep -cE "DAMAGE skill=.* target=$label( |\$)" $log.txt)
    echo "friendly unit '$label': hero damage lines $n"
    [ "$n" = 0 ] || { echo "FAIL: the hero's skills damaged its own '$label'"; fail=1; }
    n=$(grep -cE "STATE apply .*unit=$label( |\$)" $log.txt)
    [ "$n" = 0 ] || { echo "FAIL: a hostile state was put on the hero's own '$label'"; fail=1; }
    grep -qE "SKILL (static_field|.*) target=$label " $log.txt && { echo "FAIL: a skill effect named the hero's own '$label'"; fail=1; }
  done
  # the guard's own lines: a refused or spared effect means an upstream list leaked a friend (harmless, but reported)
  echo "friendly guard lines: $(grep -cE 'FRIENDLY (refused|spared)' $log.txt)"
  ! grep -qE "panic|fatal error|SIGSEGV" $log.txt || { echo "FAIL: crash in the log"; fail=1; }
}
