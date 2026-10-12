scenario_name="Necromancer skills (every Necromancer skill without an earlier scenario: cast by a fresh Necromancer, each with an effect)"
scenario_turbo=1
# w27 skill coverage (docs/skills-coverage.md, scripts/verify.d/lib/skillcov.sh): a fresh Necromancer (OD2_AUTOCAST_CLASS)
# casts the Necromancer skills that no other scenario casts, at zombies that are spawned again when all are dead, and every
# skill must show its outcome in the log (damage, state, summon, missile spawned or hit) besides the finished cast.
source scripts/verify.d/lib/skillcov.sh
scenario_env() {
  local list="Teeth,1!!;Bone Spear,1!!;Bone Spirit,1!!;Poison Dagger,1!!;Weaken,1;Life Tap,1;Decrepify,1;Lower Resist,1;Terror,1;Iron Maiden,1;Bone Prison,1;Bone Wall,1;Poison Explosion,1;Clay Golem,1;BloodGolem,1;IronGolem,1;FireGolem,1;Revive,1"
  echo "export OD2_AUTOCAST_CLASS=necromancer OD2_AUTOCAST_MANA=900 OD2_AUTOCAST_LEVEL=10 OD2_AUTOMONSTER=\"zombie1,5\" OD2_AUTOMONSTER_SECONDS=300"
  echo "export OD2_AUTOCAST=\"$list\""
}
scenario_check() {
  grep -E "AUTOCAST summary" $log.txt | cut -c1-210 | head -3
  local skills=("Teeth" "Bone Spear" "Bone Spirit" "Poison Dagger" "Weaken" "Life Tap" "Decrepify" "Lower Resist" "Terror" "Iron Maiden" "Bone Prison" "Bone Wall" "Poison Explosion" "Clay Golem" "BloodGolem" "IronGolem" "FireGolem" "Revive")
  skillcov_report "${skills[@]}"
  skillcov_cast "${skills[@]}"
  skillcov_effect dmg "Teeth"
  skillcov_effect hit "Teeth" 'teeth'
  skillcov_effect dmg "Bone Spear"
  skillcov_effect hit "Bone Spear" 'bonespear'
  skillcov_effect dmg "Bone Spirit"
  skillcov_effect hit "Bone Spirit" 'bonespirit'
  skillcov_effect melee "Poison Dagger"
  skillcov_effect dmg "Poison Dagger"
  skillcov_effect state "Weaken"
  skillcov_effect state "Life Tap"
  skillcov_effect state "Decrepify"
  skillcov_effect state "Lower Resist"
  skillcov_effect state "Terror"
  skillcov_effect state "Iron Maiden"
  skillcov_effect summon "Bone Prison"
  skillcov_effect summon "Bone Wall"
  skillcov_effect area "Poison Explosion"
  skillcov_effect missile "Poison Explosion" 'poisonexplosioncloud'
  skillcov_effect summon "Clay Golem"
  skillcov_effect summon "BloodGolem"
  skillcov_effect summon "IronGolem"
  skillcov_effect summon "FireGolem"
  skillcov_effect summon "Revive"
  ! grep -qE "panic|fatal error|SIGSEGV" $log.txt || { echo "FAIL: crash in the log"; fail=1; }
}
