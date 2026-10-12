scenario_name="Paladin skills (every Paladin skill without an earlier scenario: cast by a fresh Paladin, each with an effect)"
scenario_turbo=1
# w27 skill coverage (docs/skills-coverage.md, scripts/verify.d/lib/skillcov.sh): a fresh Paladin (OD2_AUTOCAST_CLASS)
# casts the Paladin skills that no other scenario casts, at zombies that are spawned again when all are dead, and every
# skill must show its outcome in the log (damage, state, summon, missile spawned or hit) besides the finished cast.
source scripts/verify.d/lib/skillcov.sh
scenario_env() {
  local list="Holy Bolt,1!!;Blessed Hammer,1!!;Fist of the Heavens,1!!;Smite,2!!;Zeal,2!!;Vengeance,2!!;Charge,1;Resist Fire,1;Resist Cold,1;Resist Lightning,1;Thorns,1;Defiance,1;Blessed Aim,1;Cleansing,1;Meditation,1;Vigor,1;Prayer,1;Redemption,1;Salvation,1;Fanaticism,2;Holy Shield,1;Conviction,1;Concentration,1;Holy Shock,1!;Holy Freeze,1;Sanctuary,1"
  echo "export OD2_AUTOCAST_CLASS=paladin OD2_AUTOCAST_MANA=900 OD2_AUTOCAST_LEVEL=10 OD2_AUTOMONSTER=\"zombie1,5\" OD2_AUTOMONSTER_SECONDS=300"
  echo "export OD2_AUTOCAST=\"$list\""
}
scenario_check() {
  grep -E "AUTOCAST summary" $log.txt | cut -c1-210 | head -3
  local skills=("Holy Bolt" "Blessed Hammer" "Fist of the Heavens" "Smite" "Zeal" "Vengeance" "Charge" "Resist Fire" "Resist Cold" "Resist Lightning" "Thorns" "Defiance" "Blessed Aim" "Cleansing" "Meditation" "Vigor" "Prayer" "Redemption" "Salvation" "Fanaticism" "Holy Shield" "Conviction" "Concentration" "Holy Shock" "Holy Freeze" "Sanctuary")
  skillcov_report "${skills[@]}"
  skillcov_cast "${skills[@]}"
  skillcov_effect dmg "Holy Bolt"
  skillcov_effect hit "Holy Bolt" 'holybolt'
  skillcov_effect dmg "Blessed Hammer"
  skillcov_effect hit "Blessed Hammer" 'blessedhammer'
  skillcov_effect dmg "Fist of the Heavens"
  skillcov_effect missile "Fist of the Heavens" 'fistoftheheavensdelay'
  skillcov_effect melee "Smite"
  skillcov_effect dmg "Smite"
  skillcov_effect melee "Zeal"
  skillcov_effect dmg "Zeal"
  skillcov_effect melee "Vengeance"
  skillcov_effect dmg "Vengeance"
  skillcov_effect raw "Charge" 'MOVE skill="Charge"'
  skillcov_effect state "Resist Fire"
  skillcov_effect state "Resist Cold"
  skillcov_effect state "Resist Lightning"
  skillcov_effect state "Thorns"
  skillcov_effect state "Defiance"
  skillcov_effect state "Blessed Aim"
  skillcov_effect state "Cleansing"
  skillcov_effect state "Meditation"
  skillcov_effect state "Vigor"
  skillcov_effect state "Prayer"
  skillcov_effect state "Redemption"
  skillcov_effect state "Salvation"
  skillcov_effect state "Fanaticism"
  skillcov_effect state "Holy Shield"
  skillcov_effect state "Conviction"
  skillcov_effect state "Concentration"
  skillcov_effect state "Holy Shock"
  skillcov_effect dmg "Holy Shock"
  skillcov_effect state "Holy Freeze"
  skillcov_effect state "Sanctuary"
  ! grep -qE "panic|fatal error|SIGSEGV" $log.txt || { echo "FAIL: crash in the log"; fail=1; }
}
