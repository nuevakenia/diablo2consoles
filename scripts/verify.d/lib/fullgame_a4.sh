# Act 4 of the full-game chain (sourced by fullgame_scripts.sh).

# The Pandemonium Fortress, Izual on the Plains of Despair, the River of Flame.
fg_sc_a4a() {
  printf 'wait:3;'
  fgs_wpact; fgs_repair Halbu; fgs_talk Tyrael 6; fgs_talk "Deckard Cain" 4
  fgs_hop 104; fgs_clear 30 80; fgs_hop 105; fgs_clear 30 80
  printf 'kill:name=Izual,900;loot:30,40;'
  fgs_hop 106; fgs_wpact; fgs_clear 30 80; fgs_hop 107; fgs_wpact; fgs_clear 30 80
  fgs_wpgo 103
  fgs_talk Tyrael 8
  printf 'exit'
}

# The Chaos Sanctuary: the five seals, their bosses, Diablo, and Tyrael's portal to Harrogath.
fg_sc_a4b() {
  printf 'wait:3;'
  fgs_repair Halbu
  fgs_wpgo 107
  fgs_hop 108; printf 'wait:3;'
  printf 'walkto:object=393;until:CHAOS seal operated id=393,120;'
  printf 'walkto:object=392;until:CHAOS spawn seal boss,120;kill:near=30,900;'
  printf 'walkto:object=394;until:CHAOS spawn seal boss,120;kill:near=30,900;'
  printf 'walkto:object=395;until:CHAOS seal operated id=395,120;'
  printf 'walkto:object=396;until:CHAOS spawn seal boss,120;kill:near=30,900;'
  printf 'until:class=243 at subtile,120;kill:near=40,2400;kill:near=40,2400;'
  printf 'wait:5;'
  fgs_hop 107; fgs_wpgo 103
  fgs_talk Tyrael 8
  printf 'expect:level=109;wait:3;'
  printf 'exit'
}
