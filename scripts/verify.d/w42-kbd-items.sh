scenario_name="keyboard-only item moving (arrow keys move a cell cursor over the inventory and stash, Enter picks up and drops, PageUp/PageDown hop between the grids; no mouse)"
wb=$tmp/writeback-w42-kbd
# The stash panel opens next to the inventory (panel:stash). The first arrow key shows the cursor; kbdfind (test helper)
# puts it on the first inventory item; Enter picks it up; PageUp hops to the stash, Enter drops it there; kbdfind + Enter
# pick it up again, PageDown + Enter drops it back into the inventory. Every Enter is a left click through the mouse handlers.
scenario_env() {
  mkdir -p $wb
  echo "export OD2_D2S_WRITEBACK=\"$wb\" OD2_AUTOMONSTER=\"zombie1,1\" OD2_AUTOMONSTER_PASSIVE=1 OD2_AUTOMONSTER_HOLD=1"
  local s='wait:2;panel:stash;wait:1;press:Down;press:PageDown;press:Right;press:Down;press:Left;press:Up;say:kbdfind'
  s="$s;press:Enter;press:Enter;press:Enter;press:PageUp;press:Enter;say:kbdfind;press:Enter;press:PageDown;press:Enter"
  s="$s;panel:close;wait:1;exit"
  echo "export OD2_AUTOSCRIPT='$s'"
}
scenario_check() {
  grep -E "KBDGRID|AUTOSCRIPT RESULT" $log.txt | cut -c1-220 | head -40
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: script did not pass"; fail=1; }
  # Enter with no cursor showing stays the chat key: no click, no KBDGRID enter before the first show
  first=$(grep -m1 -E "KBDGRID (show|enter)" $log.txt)
  echo "$first" | grep -q "KBDGRID show" || { echo "FAIL: the first key must only show the cursor: $first"; fail=1; }
  # the cursor moved over the grid: Down went to row 1, Right/Down/Left/Up came back
  grep -qE "KBDGRID move panel=inventory cell=\(1,1\)" $log.txt || { echo "FAIL: arrow keys did not move the cursor to (1,1)"; fail=1; }
  grep -qE "KBDGRID hop panel=" $log.txt || { echo "FAIL: PageUp/PageDown did not hop"; fail=1; }
  # Enter on the first item: picked up; Enter again on the (now empty) cell: put back exactly there; Enter: picked up again
  enters=("${(@f)$(grep 'KBDGRID enter' $log.txt | tr -d '\r')}")
  [ ${#enters} -eq 6 ] || { echo "FAIL: want 6 Enter clicks, got ${#enters}"; fail=1; return; }
  echo "${enters[1]}" | grep -qE "panel=inventory .* cursor_before=- cursor_after=[a-z0-9]+ act=pickup" || { echo "FAIL: Enter did not pick an item up"; fail=1; }
  echo "${enters[2]}" | grep -qE "panel=inventory .* cursor_before=[a-z0-9]+ cursor_after=- act=place" || { echo "FAIL: Enter did not drop the item back on its cell"; fail=1; }
  echo "${enters[3]}" | grep -qE "panel=inventory .* cursor_before=- cursor_after=[a-z0-9]+ act=pickup" || { echo "FAIL: the second pick up failed"; fail=1; }
  # PageUp hops to the stash: Enter there drops (free cell) or swaps (occupied cell), never picks up with a held item
  echo "${enters[4]}" | grep -qE "panel=stash .* cursor_before=[a-z0-9]+ .* act=(place|swap|merge)" || { echo "FAIL: Enter in the stash did not drop or swap: ${enters[4]}"; fail=1; }
  # the last Enter, back on the inventory, drops what is on the cursor (or swaps)
  echo "${enters[6]}" | grep -qE "panel=inventory .* act=(place|swap|merge)" || { echo "FAIL: the last Enter did not drop into the inventory: ${enters[6]}"; fail=1; }
  # no item is lost or made: inventory + stash + cursor stays the same across all Enters
  n=$(print -l ${enters} | sed 's/.* total=//' | sort -u | wc -l | tr -d ' ')
  [ "$n" -eq 1 ] || { echo "FAIL: the item total changed during the keyboard moves"; fail=1; }
}
