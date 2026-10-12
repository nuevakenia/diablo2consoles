scenario_name="Barbarian skills (every Barbarian skill without an earlier scenario: cast by a fresh Barbarian, each with an effect)"
scenario_turbo=1
# w27 skill coverage (docs/skills-coverage.md, scripts/verify.d/lib/skillcov.sh): a fresh Barbarian (OD2_AUTOCAST_CLASS)
# casts the Barbarian skills that no other scenario casts, at zombies that are spawned again when all are dead, and every
# skill must show its outcome in the log (damage, state, summon, missile spawned or hit) besides the finished cast.
source scripts/verify.d/lib/skillcov.sh
scenario_env() {
  local list="Battle Cry,1!!@3;Howl,1;Shout,1;Taunt,1;Battle Command,1;Stun,2!!;Leap,1;Leap Attack,1!!;Double Swing,2!!;Frenzy,2!!;Concentrate,2!!;Berserk,2!!;Double Throw,1!!"
  echo "export OD2_AUTOCAST_CLASS=barbarian OD2_AUTOCAST_MANA=900 OD2_AUTOCAST_LEVEL=10 OD2_AUTOMONSTER=\"zombie1,5\" OD2_AUTOMONSTER_SECONDS=300"
  echo "export OD2_AUTOCAST=\"$list\""
}
scenario_check() {
  grep -E "AUTOCAST summary" $log.txt | cut -c1-210 | head -3
  local skills=("Battle Cry" "Howl" "Shout" "Taunt" "Battle Command" "Stun" "Leap" "Leap Attack" "Double Swing" "Frenzy" "Concentrate" "Berserk" "Double Throw")
  skillcov_report "${skills[@]}"
  skillcov_cast "${skills[@]}"
  skillcov_effect missile "Battle Cry" 'battlecry'
  skillcov_effect raw "Battle Cry" 'STATE apply by=.* unit=Zombie state=battlecry'
  skillcov_effect raw "Howl" 'STATE apply by=.* unit=Zombie state=terror'
  skillcov_effect missile "Howl" 'howl'
  skillcov_effect state "Shout"
  skillcov_effect state "Taunt"
  skillcov_effect state "Battle Command"
  skillcov_effect melee "Stun"
  skillcov_effect dmg "Stun"
  skillcov_effect raw "Leap" 'MOVE skill="Leap"'
  skillcov_effect melee "Leap Attack"
  skillcov_effect dmg "Leap Attack"
  skillcov_effect melee "Double Swing"
  skillcov_effect dmg "Double Swing"
  skillcov_effect melee "Frenzy"
  skillcov_effect dmg "Frenzy"
  skillcov_effect state "Frenzy"
  skillcov_effect melee "Concentrate"
  skillcov_effect dmg "Concentrate"
  skillcov_effect state "Concentrate"
  skillcov_effect melee "Berserk"
  skillcov_effect dmg "Berserk"
  skillcov_effect state "Berserk"
  skillcov_effect dmg "Double Throw"
  skillcov_effect missile "Double Throw" 'javelin'
  ! grep -qE "panic|fatal error|SIGSEGV" $log.txt || { echo "FAIL: crash in the log"; fail=1; }
}
