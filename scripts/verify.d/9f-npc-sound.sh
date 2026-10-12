scenario_name="NPC greeting sound (SOUNDLOG: the greeting of a town NPC goes through the voice bank as kind=npc-greeting, not around it)"
scenario_env() { echo 'export OD2_SOUNDLOG=1 OD2_AUTOTALK="Akara,Charsi"'; }
scenario_check() {
  grep -E "NPC greeting:" $log.txt | cut -c1-200
  grep -qE "SOUNDLOG .* kind=npc-greeting .*handle=(akara|charsi)_greeting[a-z0-9_]* .*decision=(played|stolen|queued|inaudible)" $log.txt \
    || { echo "FAIL: no NPC greeting in the sound log"; fail=1; }
}
scenario_unmuted=0
scenario_timeout=240
