scenario_name="Sorceress skills (every Sorceress skill without an earlier scenario: cast by a fresh Sorceress, each with an effect)"
scenario_turbo=1
# w27 skill coverage (docs/skills-coverage.md, scripts/verify.d/lib/skillcov.sh): a fresh Sorceress (OD2_AUTOCAST_CLASS)
# casts the Sorceress skills that no other scenario casts, at zombies that are spawned again when all are dead, and every
# skill must show its outcome in the log (damage, state, summon, missile spawned or hit) besides the finished cast.
source scripts/verify.d/lib/skillcov.sh
scenario_env() {
  local list="Charged Bolt,1!!;Ice Bolt,1!!;Ice Blast,1!!;Lightning,1!!;Telekinesis,2!!;Glacial Spike,1!;Fire Ball,1!;Frozen Armor,1;Shiver Armor,1;Chilling Armor,1;Enchant,1;Static Field,1;Blaze,1;Inferno,1;Nova,1;Frost Nova,1;Fire Wall,1;Meteor,1;Frozen Orb,1"
  echo "export OD2_AUTOCAST_CLASS=sorceress OD2_AUTOCAST_MANA=900 OD2_AUTOCAST_LEVEL=10 OD2_AUTOMONSTER=\"zombie1,5\" OD2_AUTOMONSTER_SECONDS=300"
  echo "export OD2_AUTOCAST=\"$list\""
}
scenario_check() {
  grep -E "AUTOCAST summary" $log.txt | cut -c1-210 | head -3
  local skills=("Charged Bolt" "Ice Bolt" "Ice Blast" "Lightning" "Telekinesis" "Glacial Spike" "Fire Ball" "Frozen Armor" "Shiver Armor" "Chilling Armor" "Enchant" "Static Field" "Blaze" "Inferno" "Nova" "Frost Nova" "Fire Wall" "Meteor" "Frozen Orb")
  skillcov_report "${skills[@]}"
  skillcov_cast "${skills[@]}"
  skillcov_effect dmg "Charged Bolt"
  skillcov_effect missile "Charged Bolt" 'chargedbolt'
  skillcov_effect dmg "Ice Bolt"
  skillcov_effect hit "Ice Bolt" 'icebolt'
  skillcov_effect dmg "Ice Blast"
  skillcov_effect hit "Ice Blast" 'iceblast'
  skillcov_effect dmg "Lightning"
  skillcov_effect hit "Lightning" 'lightningbolt'
  skillcov_effect dmg "Telekinesis"
  skillcov_effect area "Glacial Spike"
  skillcov_effect missile "Glacial Spike" 'glacialspike'
  skillcov_effect area "Fire Ball"
  skillcov_effect missile "Fire Ball" 'fireball'
  skillcov_effect state "Frozen Armor"
  skillcov_effect state "Shiver Armor"
  skillcov_effect state "Chilling Armor"
  skillcov_effect state "Enchant"
  skillcov_effect state "Static Field"
  skillcov_effect state "Blaze"
  skillcov_effect chan "Inferno"
  skillcov_effect missile "Nova" 'nova'
  skillcov_effect missile "Frost Nova" 'frostnova'
  skillcov_effect missile "Fire Wall" 'firewallmaker'
  skillcov_effect missile "Meteor" 'meteor.*'
  skillcov_effect missile "Frozen Orb" 'frozenorb'
  ! grep -qE "panic|fatal error|SIGSEGV" $log.txt || { echo "FAIL: crash in the log"; fail=1; }
}
