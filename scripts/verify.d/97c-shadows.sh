scenario_name="Assassin shadows (Shadow Warrior copies the hero's skills with the fatigue gate, Shadow Master learns the class skills, scores them, keeps buffs up; casts run through the skill engine)"
# An Assassin (OD2_AUTOCAST_CLASS) casts Shadow Warrior, a few buffs and then Shadow Master (the shadows share a pet type, so the
# Master replaces the Warrior) while zombies are spawned again whenever they are all dead. The shadows are driven by the ported
# thinkers 0x5e9ff0 / 0x5ea9f0 (w25: Pre-hook skill copy, ShadowScore candidate stack, buff upkeep 0x5eab13, the Warrior's
# CopySkillCast fatigue); their skills run through MercSkillHooks.Cast, so a "SHADOW cast ... ok=true" line has a matching
# "CAST do skill=..." line of the engine with the merc(...) caster.
# GAP (documented, not asserted): the owner's own target is not adopted by the Master (ShadowTargeting.OwnerTarget needs the
# hero's target), and the Warrior copies "Attack" when the hero has no right or left skill selected (a fresh scenario hero).
scenario_env() {
  local list="Shadow Warrior,1;Quickness,1;Venom,1;Fade,1;Quickness,1;Venom,1;Shadow Master,1"
  list+=";Quickness,1;Venom,1;Fade,1;Quickness,1;Venom,1;Fade,1;Quickness,1;Venom,1"
  echo "export OD2_AUTOCAST_CLASS=assassin OD2_AUTOCAST_MANA=900 OD2_AUTOCAST_LEVEL=20 OD2_AUTOMONSTER=\"zombie1,6\" OD2_AUTOMONSTER_SECONDS=900"
  echo "export OD2_AUTOCAST=\"$list\""
}
scenario_check() {
  grep -E "SHADOW (init|cast)|MINION spawn name=Shadow|AUTOCAST summary" $log.txt | cut -c1-220 | head -40
  grep -qE "MINION spawn name=Shadow Warrior" $log.txt || { echo "FAIL: no Shadow Warrior"; fail=1; }
  grep -qE "MINION spawn name=Shadow Master" $log.txt || { echo "FAIL: no Shadow Master"; fail=1; }
  # the Pre-hooks ran: the Master got its copy of the class skills (plain attack + the allowed skills the engine can run)
  grep -qE "SHADOW init name=Shadow Master kind=2 skills=([6-9]|[1-9][0-9])" $log.txt || { echo "FAIL: the Shadow Master learned no class skills"; fail=1; }
  grep -qE "SHADOW init name=Shadow Warrior kind=1 " $log.txt || { echo "FAIL: the Shadow Warrior Pre-hook did not run"; fail=1; }
  # the Warrior moves (hero speed, its monstats velocity is 0) and fights: the hero has no skill selected, so it copies "Attack"
  grep -qE "MINION attack name=Shadow Warrior " $log.txt || { echo "FAIL: the Shadow Warrior never attacked"; fail=1; }
  # a skill cast by a shadow reached the engine and was accepted
  grep -qE "SHADOW cast name=Shadow [A-Za-z]+ skill=\"[^\"]+\" id=[0-9]+ level=[0-9]+ target=[^ ]+ ok=true" $log.txt || { echo "FAIL: no shadow skill was cast"; fail=1; }
  grep -qE "CAST start merc=merc\(" $log.txt || { echo "FAIL: the engine saw no shadow caster"; fail=1; }
  # no skill of a shadow is refused all the time
  local ok bad
  ok=$(grep -cE "SHADOW cast .* ok=true" $log.txt)
  bad=$(grep -cE "SHADOW cast .* ok=false" $log.txt)
  echo "shadow casts ok=$ok refused=$bad"
  [ "$bad" -le $((ok * 3 + 3)) ] || { echo "FAIL: too many refused shadow casts"; fail=1; }
  grep -E "\[(ERROR|WARNING)\]|panic" $log.txt | grep -v "skipping missing" | head -3 | grep -q . && { echo "FAIL: errors in the log"; fail=1; }
  grep -qE "AUTOCAST summary " $log.txt || { echo "FAIL: no cast summary"; fail=1; }
}
