scenario_name="Druid form-only skills in play (Maul refused in human shape; Feral Rage and Rabies cast in the wolf, Maul in the bear, from the skill bar with real clicks; mana paid; form sounds)"
scenario_realtime=1
scenario_timeout=420
scenario_warnings_ok=1
# w39. A fresh Druid (OD2_AUTOCAST_CLASS, level 20, own temp save) with full mana spends points in Wearwolf, Feral Rage,
# Rabies, Wearbear and Maul. In human shape Maul is refused (restrict 2, 0x645450); in the wolf Feral Rage and Rabies
# bite a held zombie; the form is left and the bear enters; Maul hits from the bear. The form skills are free from
# inside a form (nocostinstate, 0x569ee0) and the form change plays druid_morph (states.txt on/off sound).
scenario_env() {
  local s
  s='wait:2;expect:level=2;wait:3;skill:spend=Wearwolf;skill:spend=Feral Rage;skill:spend=Rabies;skill:spend=Wearbear;skill:spend=Maul'
  s="$s;skill:left=Maul;click:left@monster;wait:2"
  s="$s;skill:right=Wearwolf;click:right@hero:60,30;wait:3"
  s="$s;skill:left=Feral Rage;click:left@monster;wait:4;click:left@monster;wait:4"
  s="$s;skill:left=Rabies;click:left@monster;wait:4;click:left@monster;wait:4"
  s="$s;skill:right=Wearbear;click:right@hero:60,30;wait:3;click:right@hero:60,30;wait:3"
  s="$s;skill:left=Maul;click:left@monster;wait:4;click:left@monster;wait:4;click:left@monster;wait:4;exit"
  echo "export OD2_SOUNDLOG=1 OD2_AUTOCAST_MANA=400 OD2_AUTOGAME=\"$tmp/w39.od2\" OD2_AUTOCAST_CLASS=druid OD2_AUTOCAST_CLVL=20 OD2_AUTOCAST_SKILLPOINTS=8"
  echo 'export OD2_REALMAPS=1 OD2_AUTOLEVEL=2 OD2_AUTOMONSTER="zombie1,10" OD2_AUTOMONSTER_PASSIVE=1 OD2_AUTOMONSTER_HOLD=1 OD2_AUTOMONSTER_DIFF=0 OD2_AUTOMONSTER_SECONDS=300'
  echo "export OD2_AUTOSCRIPT='$s'"
}
scenario_check() {
  grep -E "FORM (wolf|bear|human)|AUTOSCRIPT RESULT|CAST start skill=\"(Maul|Feral Rage|Rabies)\"|CAST do skill=\"(Maul|Feral Rage|Rabies)\"" $log.txt | cut -c1-210 | head -30
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: the script did not pass"; fail=1; }
  # the first Maul (human shape) is the first Maul start line and must be refused
  first=$(grep -m1 'CAST start skill="Maul"' $log.txt)
  case "$first" in *ok=false*) ;; *) echo "FAIL: Maul was not refused in human shape ($first)"; fail=1;; esac
  grep -q "FORM wolf composite=40" $log.txt || { echo "FAIL: no wolf"; fail=1; }
  grep -q "FORM bear composite=TG" $log.txt || { echo "FAIL: no bear"; fail=1; }
  grep -qE 'CAST do skill="Feral Rage" .*ok=true' $log.txt || { echo "FAIL: Feral Rage never landed in the wolf"; fail=1; }
  grep -qE 'CAST do skill="Rabies" .*ok=true' $log.txt || { echo "FAIL: Rabies never landed in the wolf"; fail=1; }
  grep -qE 'CAST do skill="Maul" .*ok=true' $log.txt || { echo "FAIL: Maul never landed in the bear"; fail=1; }
  [ "$(grep -cE "SOUNDLOG .* kind=state .*handle=druid_morph " $log.txt)" -ge 3 ] || { echo "FAIL: too few druid_morph sounds"; fail=1; }
}
