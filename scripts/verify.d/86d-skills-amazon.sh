scenario_name="Amazon skills (every Amazon skill without an earlier scenario: cast by a fresh Amazon, each with an effect)"
scenario_turbo=1
# w27 skill coverage (docs/skills-coverage.md, scripts/verify.d/lib/skillcov.sh): a fresh Amazon (OD2_AUTOCAST_CLASS)
# casts the Amazon skills that no other scenario casts, at zombies that are spawned again when all are dead, and every
# skill must show its outcome in the log (damage, state, summon, missile spawned or hit) besides the finished cast.
source scripts/verify.d/lib/skillcov.sh
scenario_env() {
  local list="Fire Arrow,1!!;Magic Arrow,1!!;Cold Arrow,1!!;Exploding Arrow,1!!;Ice Arrow,1!!;Guided Arrow,1!!;Lightning Bolt,1!!;Freezing Arrow,1!!;Lightning Fury,1!!;Multiple Shot,1!!;Strafe,1!!;Jab,2!!;Power Strike,2!!;Impale,2!!;Fend,2!!;Charged Strike,2!!;Lightning Strike,2!!;Inner Sight,1;Slow Missiles,1;Plague Javelin,1!!;Immolation Arrow,1!"
  echo "export OD2_AUTOCAST_CLASS=amazon OD2_AUTOCAST_MANA=900 OD2_AUTOCAST_LEVEL=10 OD2_AUTOMONSTER=\"zombie1,5\" OD2_AUTOMONSTER_SECONDS=300"
  echo "export OD2_AUTOCAST=\"$list\""
}
scenario_check() {
  grep -E "AUTOCAST summary" $log.txt | cut -c1-210 | head -3
  local skills=("Fire Arrow" "Magic Arrow" "Cold Arrow" "Exploding Arrow" "Ice Arrow" "Guided Arrow" "Lightning Bolt" "Freezing Arrow" "Lightning Fury" "Multiple Shot" "Strafe" "Jab" "Power Strike" "Impale" "Fend" "Charged Strike" "Lightning Strike" "Inner Sight" "Slow Missiles" "Plague Javelin" "Immolation Arrow")
  skillcov_report "${skills[@]}"
  skillcov_cast "${skills[@]}"
  skillcov_effect dmg "Fire Arrow"
  skillcov_effect hit "Fire Arrow" 'firearrow'
  skillcov_effect dmg "Magic Arrow"
  skillcov_effect hit "Magic Arrow" 'magicarrow'
  skillcov_effect dmg "Cold Arrow"
  skillcov_effect hit "Cold Arrow" 'coldarrow'
  skillcov_effect dmg "Exploding Arrow"
  skillcov_effect hit "Exploding Arrow" 'explodingarrow'
  skillcov_effect dmg "Ice Arrow"
  skillcov_effect hit "Ice Arrow" 'icearrow'
  skillcov_effect dmg "Guided Arrow"
  skillcov_effect hit "Guided Arrow" 'guidedarrow'
  skillcov_effect dmg "Lightning Bolt"
  skillcov_effect hit "Lightning Bolt" 'lightningjavelin'
  skillcov_effect dmg "Freezing Arrow"
  skillcov_effect hit "Freezing Arrow" 'freezingarrow'
  skillcov_effect dmg "Lightning Fury"
  skillcov_effect hit "Lightning Fury" 'lightningfury'
  skillcov_effect dmg "Multiple Shot"
  skillcov_effect missile "Multiple Shot" 'multipleshot(arrow|bolt)'
  skillcov_effect dmg "Strafe"
  skillcov_effect hit "Strafe" 'strafe(arrow|bolt)'
  skillcov_effect melee "Jab"
  skillcov_effect dmg "Jab"
  skillcov_effect melee "Power Strike"
  skillcov_effect dmg "Power Strike"
  skillcov_effect melee "Impale"
  skillcov_effect dmg "Impale"
  skillcov_effect melee "Fend"
  skillcov_effect melee "Charged Strike"
  skillcov_effect missile "Charged Strike" 'chargedstrikebolt'
  skillcov_effect melee "Lightning Strike"
  skillcov_effect missile "Lightning Strike" 'lightningstrike'
  skillcov_effect state "Inner Sight"
  skillcov_effect state "Slow Missiles"
  skillcov_effect state "Plague Javelin"
  skillcov_effect missile "Plague Javelin" 'plaguejavelin'
  skillcov_effect area "Immolation Arrow"
  skillcov_effect missile "Immolation Arrow" 'immolationarrow'
  ! grep -qE "panic|fatal error|SIGSEGV" $log.txt || { echo "FAIL: crash in the log"; fail=1; }
}
