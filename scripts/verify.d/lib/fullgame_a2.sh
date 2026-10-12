# Act 2 of the full-game chain (sourced by fullgame_scripts.sh).
fgs_chest() { printf 'say:dumpjunk;walkto:object=%s;wait:3;say:lootquest 15 %s;' "$1" "${2:-30}"; }
fgs_travel() { printf 'move:npc=%s;until:NPC menu opened: npc="%s",60;menu:%s;wait:3;expect:level=%s;wait:3;' "$1" "$1" "$2" "$3"; }

# Lut Gholein: the trip east, the talk with the town, Radament's Lair (and the scroll chest next to it).
fg_sc_a2a() {
  printf 'wait:2;'
  fgs_travel Warriv "go east" 40
  fgs_repair Fara; fgs_talk Jerhyn 4; fgs_talk Atma; fgs_talk Fara 4; fgs_talk Drognan 4
  # Radament: the manhole of the town, three sewer levels, the Book of Skill, Atma
  fgs_hop 47 48; fgs_wpact; fgs_clear 40 80; fgs_hop 49; printf 'kill:name=Radament,400;kill:near=40,100;say:lootquest 200 90;say:useitem ass;'
  fgs_chest 355
  fgs_hop 48 47 40; fgs_talk Atma 10; printf 'say:autobuild;'
  printf 'exit'
}

# The pieces of the Horadric Staff: the Cube in the Halls of the Dead, the staff in the Maggot Lair, the amulet in the Claw
# Viper Temple.
fg_sc_a2b() {
  printf 'wait:2;'
  fgs_repair Fara
  fgs_hop 41; fgs_clear 30 80; fgs_hop 42; fgs_wpact; fgs_clear 30 80
  fgs_hop 56; fgs_clear 30 60; fgs_hop 57; fgs_wpact; fgs_clear 30 60; fgs_hop 60; fgs_clear 30 60; fgs_chest 354
  fgs_hop 57 56 42 43; fgs_clear 30 80; fgs_hop 62 63 64; fgs_clear 30 60; fgs_chest 356
  fgs_hop 63 62 43 44; fgs_clear 30 80; fgs_hop 45; fgs_clear 30 80; fgs_hop 58 61; fgs_clear 30 60; fgs_chest 'Tainted Sun Altar'
  printf 'exit'
}

# The Summoner, the Journal, the Canyon of the Magi, Tal Rasha's real tomb (found by the map seed), the staff in the
# orifice, Duriel, Tyrael's portal and Meshif's ship to Act 3. $1 is the level id of the real tomb.
fg_sc_a2d() {
  printf 'wait:2;'
  fgs_repair Fara; printf 'say:cubetake;'
  fgs_hop 50 51 52 53 54; printf 'walkto:object=Portal;wait:3;expect:level=74;wait:3;'
  printf 'kill:name=Summoner,1500;kill:near=30,100;loot:30,40;say:dumpjunk;walkto:object=357;wait:3;'
  printf 'walkto:object=Portal;wait:3;expect:level=46;wait:3;'
  fgs_hop "$1"; printf 'kill:near=40,100;walkto:object=152;wait:5;'
  printf 'walkto:object=Portal;wait:3;expect:level=73;wait:3;kill:name=Duriel,3000;'
  printf 'wait:6;'
  fgs_talk Tyrael 6
  printf 'walkto:object=Portal;wait:3;expect:level=40;wait:3;'
  fgs_talk Jerhyn 6
  fgs_travel Meshif "sail east" 75
  printf 'exit'
}

# Cain's readings, the staff in the cube, the Palace and the Arcane Sanctuary.
fg_sc_a2c() {
  printf 'wait:2;'
  fgs_repair Fara
  fgs_talk "Deckard Cain" 10; fgs_talk "Deckard Cain" 10; fgs_talk "Deckard Cain" 10; fgs_talk "Deckard Cain" 10
  printf 'say:cubeput msf vip - -;say:transmute;wait:2;'
  fgs_talk "Deckard Cain" 10
  fgs_talk Jerhyn 6
  fgs_hop 50 51 52 53 54; fgs_clear 30 60
  printf 'exit'
}
