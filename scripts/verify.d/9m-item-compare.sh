scenario_name="item compare (the tooltip of an item shows how it compares with the worn one: inventory, stash, cube, vendor, held item, ground item; screenshots)"
scenario_realtime=1
scenario_timeout=300
# The revived sample hero (a level 94 sorceress with a full body) hovers items through the real input path (the console
# commands hoveritem / hoverground move the pointer) and a frame is saved after each hover, so the tooltip can be looked
# at: the files stay in $tmp, never in the repo. COMPARE lines log the plain text of the comparison.
ic=$tmp/itemcompare
scenario_env() {
  mkdir -p $ic/s94 $ic/wb94; rm -f $ic/s94/*.d2s(N) $ic/wb94/*.d2s(N)
  if [ -n "${D2_TABLES:-}" ] && go run scripts/d2s-revive.go "$D2S_SAMPLE_BODY" $ic/s94/Hero.d2s "$D2_TABLES" >/dev/null 2>&1; then
    echo "export OD2_AUTOGAME=\"$ic/s94/Hero.d2s\" OD2_D2S_WRITEBACK=\"$ic/wb94\" OD2_AUTOSPEED=3"
  else
    echo "# no revived sample hero, using the default save" >&2
  fi
  echo 'export OD2_AUTOTRADE_KEEP=1'
  local s='wait:2;say:clearinv;say:giveitem xtp;say:giveitem 7ws;say:giveitem lbb;say:giveitem tkf'
  # inventory: armour (defense), a melee weapon, a bow (slower) and a throwing knife
  s="$s;say:hoveritem inv xtp;wait:1;shot:$tmp/cmp-inv.png;say:comparelog inv xtp;say:comparelog inv 7ws;say:comparelog inv lbb;say:comparelog inv tkf"
  # stash and cube
  s="$s;say:stashgive xtp;say:hoveritem stash xtp;wait:1;shot:$tmp/cmp-stash.png;say:comparelog stash xtp"
  s="$s;say:cubeput lbb - - -;say:hoveritem cube lbb;wait:1;shot:$tmp/cmp-cube.png;say:comparelog cube lbb"
  # the held item: its tooltip stands over the picture
  s="$s;say:hoveritem inv 7ws;say:pickitem 7ws;wait:1;shot:$tmp/cmp-cursor.png;say:putitem"
  # the vendor window
  s="$s;click:left@npc:Charsi;until:NPC menu opened,60;menu:Trade;wait:1;say:hoveritem vendor 2hs;wait:1;shot:$tmp/cmp-vendor.png;say:comparelog vendor 2hs"
  # a ground item under the pointer
  s="$s;say:spawnground xtp 8 8;wait:2;say:hoverground Plate;wait:1;shot:$tmp/cmp-ground.png;exit"
  echo "export OD2_AUTOSCRIPT='$s'"
}
scenario_check() {
  grep -E "COMPARE|HOVERITEM|HOVERGROUND|AUTOSCRIPT RESULT" $log.txt | cut -c1-260 | head -30
  grep -q "AUTOSCRIPT RESULT PASS" $log.txt || { echo "FAIL: the item compare script did not pass"; fail=1; }
  grep -qE "COMPARE where=inv code=xtp lines=[1-9].*Defense: " $log.txt || { echo "FAIL: no defense comparison for the armour in the inventory"; fail=1; }
  grep -qE "COMPARE where=inv code=7ws lines=[1-9].*Damage: " $log.txt || { echo "FAIL: no damage comparison for the weapon in the inventory"; fail=1; }
  grep -qE "COMPARE where=inv code=lbb .*Damage: .*Attack Speed: " $log.txt || { echo "FAIL: no attack speed comparison for the bow"; fail=1; }
  grep -qE "COMPARE where=stash code=xtp lines=[1-9]" $log.txt || { echo "FAIL: no comparison on the stash"; fail=1; }
  grep -qE "COMPARE where=cube code=lbb lines=[1-9]" $log.txt || { echo "FAIL: no comparison in the cube"; fail=1; }
  grep -qE "COMPARE where=vendor " $log.txt || { echo "FAIL: the vendor window was not compared"; fail=1; }
  for n in inv stash cube cursor vendor ground; do
    [ -s $tmp/cmp-$n.png ] || { echo "FAIL: no screenshot cmp-$n.png"; fail=1; }
  done
}
