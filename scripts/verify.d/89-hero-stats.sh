scenario_name="hero stats (equipment bonuses: life/mana/stamina totals, defense, attack rating, damage, resists on the character panel; mana regenerates on its own)"
scenario_env() {
  # the second half empties the mana, lets the (x8) game clock run and reads it again: natural regeneration
  echo "export OD2_AUTOSCRIPT='wait:2;panel:character;wait:1;panel:close;say:setmana 0;panel:character;wait:1;panel:close;wait:4;panel:character;wait:1;panel:close;exit'"
}
scenario_check() {
  grep -E "stats: .* level=|PANEL character:|VITALS mana|AUTOSCRIPT RESULT" $log.txt | cut -c1-300
  # the real level 94 save stores life 869 / mana 221 / stamina 525 WITHOUT items and 1241 / 464 / 801 as current values:
  # the totals with equipment must be 1241 life (exact), 477 mana (464 is below it) and 801 stamina (exact)
  grep -qE "stats: .* level=94 .*life=1241/1241 mana=464/477 stamina=801/801" $log.txt || { echo "FAIL: character totals at load are not 1241/1241, 464/477, 801/801"; fail=1; }
  grep -qE "PANEL character: level=94 .*life=1241/1241 mana=[0-9]+/477 stamina=801/801" $log.txt || { echo "FAIL: character panel totals are not 1241/1241, x/477, 801/801"; fail=1; }
  grep -qE "PANEL character: .* def=[1-9][0-9]* ar=[1-9][0-9]* dmg=[0-9]+-[0-9]+ block=[0-9]+ res=fire:-?[0-9]+,cold:-?[0-9]+,light:-?[0-9]+,poison:-?[0-9]+" $log.txt || { echo "FAIL: no def/ar/dmg/res on the character panel line"; fail=1; }
  # natural mana regeneration (exe 0x57e6e0: max mana / (ManaRegen 120 * 25 frames) per 25 Hz frame = 3.9 mana per game
  # second for 477): the save's 464 is full (477) at the first panel; after setmana 0 the panel right away shows about 0 and
  # the one 5 script seconds later (wait: is game time; setmana to the last panel is 6 s, 150 frames * 40 raw / 256 = 23
  # mana, 25 seen) shows the mana that came back
  local m0 m1 m2
  m0=$(grep "PANEL character: level.* def=" $log.txt | sed -n 1p | sed -E 's/.* mana=([0-9]+)\/477.*/\1/')
  m1=$(grep "PANEL character: level.* def=" $log.txt | sed -n 2p | sed -E 's/.* mana=([0-9]+)\/477.*/\1/')
  m2=$(grep "PANEL character: level.* def=" $log.txt | sed -n 3p | sed -E 's/.* mana=([0-9]+)\/477.*/\1/')
  echo "mana regeneration: panels $m0 (from the save's 464), $m1 (after setmana 0), $m2 (6 s later)"
  case "$m0$m1$m2" in ''|*[!0-9]*) echo "FAIL: could not read the mana of three character panels"; fail=1 ;; *)
    [ "$m0" -gt 464 ] && [ "$m0" -le 477 ] || { echo "FAIL: mana did not rise from the saved 464 ($m0)"; fail=1; }
    [ "$m1" -le 40 ] || { echo "FAIL: mana was not emptied by setmana 0 ($m1)"; fail=1; }
    # the rate is measured against the simulated clock (sim= on the panel line), not the script waits: a wait ends at a
    # frame boundary and a slow frame of a loaded machine at x8 overshoots it by seconds
    local t1 t2 rate
    t1=$(grep "PANEL character: level.* def=" $log.txt | sed -n 2p | sed -nE 's/.* sim=([0-9.]+).*/\1/p')
    t2=$(grep "PANEL character: level.* def=" $log.txt | sed -n 3p | sed -nE 's/.* sim=([0-9.]+).*/\1/p')
    rate=$(awk -v a="$m1" -v b="$m2" -v t1="${t1:-0}" -v t2="${t2:-0}" 'BEGIN{ if (t2-t1 >= 2) printf "%.2f", (b-a)/(t2-t1); else print "none" }')
    echo "mana regeneration rate: $rate per game second (sim $t1 -> $t2)"
    if [ "$rate" = none ]; then echo "FAIL: no simulated-clock stamps on the character panels"; fail=1
    else
      awk -v r="$rate" 'BEGIN{ exit !(r >= 3.0 && r <= 5.0) }' || { echo "FAIL: mana did not regenerate by about 3.9 per game second ($rate: $m1 -> $m2 between sim $t1 and $t2)"; fail=1; }
    fi ;;
  esac
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: scripted scenario did not pass"; fail=1; }
}
