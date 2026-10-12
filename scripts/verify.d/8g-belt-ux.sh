scenario_name="belt UX (a 4-box belt takes potions in every cell by real clicks; worn belts of 8/12/16 boxes, expanded rows, column rule, swap, shift-click, Q)"
scenario_realtime=1
scenario_timeout=300
scenario_warnings_ok=1 # setbelt takes the belt off the hero: the save export warns that the worn belt is kept in the file
bx=$tmp/beltux
shot_a=$bx/a.png
# belt cell centre in the 800x600 column: x = 437 + 31*col, y = 576 - 32*row (the front row is the bottom one)
_cx() { echo $((437 + 31 * $1)); }
_cy() { echo $((576 - 32 * $1)); }
_fill() { local k out="say:giveitem $1" ; for k in $(seq 2 $2); do out="$out;say:giveitem $1"; done; echo "$out;wait:0.3"; }
# inventory potions sit in column 9 (x 694), row r at y 329 + 29*r
_move() { echo "click:left@ui:694,$((329 + 29 * $1));wait:0.4;click:left@ui:$(_cx $2),$(_cy $3);wait:0.4"; }
scenario_env() {
  mkdir -p $bx/s94 $bx/wb94; rm -f $bx/s94/*.d2s(N) $bx/wb94/*.d2s(N)
  if [ -n "${D2_TABLES:-}" ] && go run scripts/d2s-revive.go "$D2S_SAMPLE_BODY" $bx/s94/Hero.d2s "$D2_TABLES" >/dev/null 2>&1; then
    echo "export OD2_AUTOGAME=\"$bx/s94/Hero.d2s\" OD2_D2S_WRITEBACK=\"$bx/wb94\" OD2_AUTOSPEED=3"
  fi
  echo 'export OD2_REALMAPS=1 OD2_AUTOLEVEL=2'
  # BELTUX_DISPLAY=1920x1080 reruns the same column clicks at another window size (a pure translation of the 800x600 column)
  [ -n "${BELTUX_DISPLAY:-}" ] && echo "export OD2_DISPLAY=$BELTUX_DISPLAY"
  local c s='expect:level=2;press:I;until:inventory=true,60'
  # _fill <code> <n>: n potions into the inventory (they fill the right column from the top row down)
  # _move <inv row> <belt col> <belt row>: a real click on the potion, a real click on the belt cell
  # (a) no belt worn: four boxes, the four front cells
  s="$s;say:setbelt none;say:beltinfo;say:clearinv"
  s="$s;$(_fill hp1 4)"
  for c in 0 1 2 3; do s="$s;$(_move $c $c 0)"; done
  s="$s;say:beltlist;say:capframe $shot_a"
  # (b) sash (8 boxes), expanded: rows 1 then 0, back row first
  for belt in lbl:8 mbl:12 ulc:16; do
    code=${belt%%:*}; n=${belt##*:}; rows=$((n / 4))
    s="$s;say:setbelt $code;say:beltinfo;press:Tilde;wait:0.5;say:beltinfo;say:clearinv"
    local r
    for r in $(seq $((rows - 1)) -1 0); do
      s="$s;$(_fill hp1 4)"
      for c in 0 1 2 3; do s="$s;$(_move $c $c $r)"; done
    done
    s="$s;say:beltlist;say:capframe $bx/b$code.png;press:Tilde"
  done
  # (c) belt of 12, expanded: hp1 in column 0 (rows 0-2) and cell (1,0); then mana potions: a swap with the
  # hp1 of (0,1), a drop into columns that hold another kind, a shift-click from the inventory
  s="$s;say:setbelt mbl;press:Tilde;wait:0.5;say:clearinv;$(_fill hp1 4)"
  s="$s;$(_move 0 0 0);$(_move 1 0 1);$(_move 2 0 2);$(_move 3 1 0);say:beltlist"
  s="$s;$(_fill mp1 4);$(_move 0 0 1);click:left@ui:$(_cx 3),$(_cy 0);wait:0.4;say:beltlist"
  s="$s;$(_move 1 1 2);$(_move 2 1 1);say:beltlist"
  s="$s;click:left+shift@ui:694,$((329 + 29 * 3));wait:0.5;say:beltlist"
  s="$s;say:beltover $(_cx 1) $(_cy 2);wait:0.5;say:capframe $bx/c-tooltip.png;press:I;wait:0.5;say:capframe $bx/c-closed.png"
  for c in 1 2 3; do s="$s;press:Q;wait:0.6;say:beltlist"; done
  s="$s;press:Tilde;wait:1;exit"
  echo "export OD2_AUTOSCRIPT='$s'"
}
scenario_check() {
  grep -E "belt cell|BELT contents|AUTOSCRIPT RESULT|AUTOSCRIPT step [0-9]* FAIL" $log.txt | cut -c1-200 | head -60
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: the belt script did not pass"; fail=1; }
  local want
  for want in "n=4 \[hp1@a hp1@b hp1@c hp1@d\]" "n=8 " "n=12 " "n=16 " \
    "n=5 \[hp1@a hp1@b hp1@d mp1@e hp1@i\]" "n=7 \[hp1@a hp1@b hp1@d mp1@e mp1@f hp1@i mp1@j\]"; do
    grep -q "BELT contents $want" $log.txt || { echo "FAIL: no belt state $want (a drop was refused or went to another cell)"; fail=1; }
  done
  grep -q "belt cell 3: hp1" $log.txt && grep -q "BELTOVER .* cell=9" $log.txt || { echo "FAIL: swap or hover cell"; fail=1; }
  [ "$(grep -c 'BELT use slot=' $log.txt)" -ge 3 ] || { echo "FAIL: Q did not drink three potions"; fail=1; }
  [ -s $bx/c-tooltip.png ] && [ -s $bx/bulc.png ] || { echo "FAIL: no screenshots"; fail=1; }
}
