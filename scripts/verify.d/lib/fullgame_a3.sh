# Act 3 of the full-game chain (sourced by fullgame_scripts.sh).
fgs_obj() { printf 'say:dumpjunk;walkto:object=%s;wait:3;kill:near=20,60;walkto:object=%s;wait:3;say:lootquest 15 20;' "$1" "$1"; }

# Travincal: Lam Esen's Tome to Alkor, the Council and Khalim's Flail, the cube (Khalim's Will from the organs the hero
# found), the Compelling Orb, the three Durance levels, Mephisto and the red portal to the Pandemonium Fortress.
fg_sc_a3b() {
  printf 'wait:3;'
  fgs_repair Hratli; fgs_talk Alkor 8; fgs_talk Ormus 4
  printf 'use:Waypoint;waypoint:83;expect:level=83;wait:3;say:dumpjunk;'
  printf 'walkto:object=404;kill:near=18,600;kill:near=1,1;say:pickground qf1;wait:4;say:putitem;wait:1;'
  printf 'say:cubeput qf1 qey qhr qbr;say:transmute;wait:2;say:cubetake;say:wield qf2;'
  printf 'walkto:object=404;wait:2;walkto:object=404;wait:2;'
  printf 'walkto:exit=100;expect:level=100;wait:2;walkto:exit=101;expect:level=101;wait:2;'
  printf 'use:Waypoint;wait:1;panel:close;walkto:exit=102;expect:level=102;wait:2;'
  printf 'kill:all,600;kill:all,600;kill:all,600;kill:near=1,1;wait:14;'
  printf 'walkto:object=342;expect:level=103;wait:3;'
  printf 'exit'
}

# Kurast Docktown, the jungle and its dungeons (the three organs of Khalim, Lam Esen's Tome, the lever of the sewers), up to
# Travincal. $1 is the jungle road of the seed (fg_jungle: Great Marsh is a dead end for some seeds), e.g. "76 78 79".
fg_sc_a3a() {
  local road="${*:-76 77 78 79}"
  printf 'wait:3;'
  fgs_repair Hratli; fgs_talk Alkor 4; fgs_talk Ormus 4; fgs_talk Hratli 4; fgs_talk Asheara 4; fgs_talk Meshif 4
  fgs_hop 76; fgs_wpact; fgs_clear 30 80; fgs_hop 85; fgs_clear 30 60; fgs_obj 407; fgs_hop 76
  if [[ " $road " == *" 77 "* ]]; then
    fgs_hop 77; fgs_clear 30 80; fgs_hop 78
  else
    # Great Marsh hangs off Spider Forest: a look into it, then the road on
    fgs_hop 77; fgs_clear 30 80; fgs_hop 76 78
  fi
  fgs_wpact; fgs_clear 30 80
  fgs_hop 88 89 91; printf 'kill:all,300;kill:all,300;loot:30,40;'; fgs_obj 406; fgs_hop 89 88 78
  fgs_hop 79; fgs_wpact; fgs_clear 30 80
  fgs_hop 80; fgs_wpact; fgs_clear 30 80
  fgs_hop 94; fgs_clear 30 60; fgs_obj 193; fgs_hop 80
  fgs_hop 92; fgs_clear 30 60; printf 'walkto:object=367;wait:3;'; fgs_hop 93; fgs_clear 30 60; fgs_obj 405; fgs_hop 92 80
  fgs_hop 81; fgs_wpact; fgs_clear 30 80; fgs_hop 82; fgs_clear 30 80; fgs_hop 83; fgs_wpact
  printf 'exit'
}
