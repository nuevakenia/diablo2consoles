scenario_name="Assassin skills (every Assassin skill without an earlier scenario: cast by a fresh Assassin, each with an effect)"
scenario_turbo=1
# w27 skill coverage (docs/skills-coverage.md, scripts/verify.d/lib/skillcov.sh): a fresh Assassin (OD2_AUTOCAST_CLASS)
# casts the Assassin skills that no other scenario casts, at zombies that are spawned again when all are dead, and every
# skill must show its outcome in the log (damage, state, summon, missile spawned or hit) besides the finished cast.
source scripts/verify.d/lib/skillcov.sh
scenario_env() {
  local list="Tiger Strike,2!!;Fire Trauma,2;Claws of Thunder,2!!;Blades of Ice,2!!;Psychic Hammer,1!!;Mind Blast,2!!;Cloak of Shadows,1;Shock Field,1!!;Blade Sentinel,1;Charged Bolt Sentry,1;Wake of Fire Sentry,1;Inferno Sentry,1;Death Sentry,1!!;Dragon Claw,2!!;Dragon Tail,1!!;Cobra Strike,2!!;Royal Strike,2!!;Death Sentry,1!!~explode"
  echo "export OD2_AUTOCAST_CLASS=assassin OD2_AUTOCAST_MANA=900 OD2_AUTOCAST_LEVEL=10 OD2_AUTOMONSTER=\"zombie1,5\" OD2_AUTOMONSTER_SECONDS=300"
  echo "export OD2_AUTOCAST=\"$list\""
}
scenario_check() {
  grep -E "AUTOCAST summary" $log.txt | cut -c1-210 | head -3
  local skills=("Tiger Strike" "Fire Trauma" "Claws of Thunder" "Blades of Ice" "Psychic Hammer" "Mind Blast" "Cloak of Shadows" "Shock Field" "Blade Sentinel" "Charged Bolt Sentry" "Wake of Fire Sentry" "Inferno Sentry" "Death Sentry" "Dragon Claw" "Dragon Tail" "Cobra Strike" "Royal Strike")
  skillcov_report "${skills[@]}"
  skillcov_cast "${skills[@]}"
  skillcov_effect melee "Tiger Strike"
  skillcov_effect dmg "Tiger Strike"
  skillcov_effect area "Fire Trauma"
  skillcov_effect melee "Claws of Thunder"
  skillcov_effect dmg "Claws of Thunder"
  skillcov_effect state "Claws of Thunder"
  skillcov_effect melee "Blades of Ice"
  skillcov_effect dmg "Blades of Ice"
  skillcov_effect state "Blades of Ice"
  skillcov_effect dmg "Psychic Hammer"
  skillcov_effect raw "Mind Blast" '(DAMAGE skill="Mind Blast" .*dmg=[1-9]|STATE convert skill="Mind Blast")'
  skillcov_effect state "Cloak of Shadows"
  skillcov_effect missile "Shock Field" 'shock'
  skillcov_effect hit "Shock Field" 'shock field on ground'
  skillcov_effect dmg "Shock Field"
  skillcov_effect summon "Blade Sentinel"
  skillcov_effect raw "Blade Sentinel" 'TRAP fire skill="Blade Sentinel"'
  skillcov_effect summon "Charged Bolt Sentry"
  skillcov_effect raw "Charged Bolt Sentry" 'TRAP fire skill="Charged Bolt Sentry"'
  skillcov_effect summon "Wake of Fire Sentry"
  skillcov_effect raw "Wake of Fire Sentry" 'TRAP fire skill="Wake of Fire Sentry"'
  skillcov_effect summon "Inferno Sentry"
  skillcov_effect raw "Inferno Sentry" 'TRAP fire skill="Inferno Sentry"'
  skillcov_effect summon "Death Sentry"
  skillcov_effect raw "Death Sentry" 'TRAP fire skill="Death Sentry"'
  skillcov_effect dmg "Death Sentry"
  # w29: the sentry's first skill explodes a corpse near its target (TRAP explode), with area damage
  skillcov_effect raw "Death Sentry" 'TRAP explode skill="Death Sentry"'
  skillcov_effect melee "Dragon Claw"
  skillcov_effect dmg "Dragon Claw"
  skillcov_effect melee "Dragon Tail"
  skillcov_effect melee "Cobra Strike"
  skillcov_effect state "Cobra Strike"
  skillcov_effect melee "Royal Strike"
  skillcov_effect state "Royal Strike"
  ! grep -qE "panic|fatal error|SIGSEGV" $log.txt || { echo "FAIL: crash in the log"; fail=1; }
}
