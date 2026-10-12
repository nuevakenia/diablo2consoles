# Autoscript text of the full-game chain (9w-fullgame-*), one function per segment (a game launch). Every function prints
# the script on stdout. Sourced after fullgame.sh. Wave 43, notes: d2-re-notes/w43-fullgame.md.
#
# The hero plays like a player would: NPC menus, exits, waypoints, kills, loot, Charsi's repair button. Where the engine
# lacks what a player has (no natural regeneration, no armour from the shop) the runs use two test aids that are named
# here and in the notes: OD2_AUTOSCRIPT_HEAL=2 (life/mana topped up, the last life point cannot be taken) and
# OD2_AUTOBUILD (the points of a level-up are spent by a fixed plan).

# ---- building blocks -------------------------------------------------------------------------------------------
fgs_talk()   { printf 'move:npc=%s;until:NPC menu opened: npc="%s",60;menu:Talk;wait:%s;' "$1" "$1" "${2:-6}"; }
# a player's trip to the weapon smith: the Repair All button of the trade window, then close it (positions: ui-layout.golden)
fgs_repair() { printf 'move:npc=%s;until:NPC menu opened: npc="%s",60;menu:Trade;wait:1;click:left@ui:211,461;wait:1;click:left@ui:368,461;wait:1;' "$1" "$1"; }
fgs_hop()    { local l; for l in "$@"; do printf 'walkto:exit=%s;expect:level=%s;wait:1;' "$l" "$l"; done; }
fgs_wpact()  { printf 'use:Waypoint;wait:1;panel:close;'; }
fgs_wpgo()   { printf 'use:Waypoint;waypoint:%s;expect:level=%s;wait:2;' "$1" "$1"; }
# clear the neighbourhood: a near sweep, loot, a wider sweep
fgs_clear()  { printf 'kill:near=%s,%s;loot:30,40;' "${1:-30}" "${2:-60}"; }
fgs_rest()   { printf 'say:restorevitals;'; }
fgs_shot()   { printf 'say:capframe %s/%s.png;' "$fg_dir" "$1"; }

# ---- Act 1 -----------------------------------------------------------------------------------------------------
# From the Rogue Encampment of a fresh hero (or of the next difficulty): Akara's Den of Evil, Kashya's Blood Raven, then the
# way to Andariel and her death. Ends in the Catacombs 4 level 37 next to the dead boss.
fg_sc_a1() {
  printf 'wait:1;'
  fgs_talk Akara; fgs_talk Akara; fgs_talk Kashya 4
  fgs_hop 2; fgs_clear 45 100; printf 'kill:all,200;loot:60,60;say:autobuild;'
  printf 'walkto:exit=8;expect:level=8;kill:all,300;until:Den of Evil cleared,10;loot:40,40;'
  fgs_hop 2 1
  fgs_talk Akara 8; printf 'say:autobuild;'
  fgs_hop 2 3; fgs_wpact; fgs_clear 40 100
  fgs_hop 17; fgs_clear 25 100; fgs_clear 40 100; printf 'kill:near=60,200;loot:40,40;'
  fgs_hop 3; fgs_wpgo 1; fgs_repair Charsi; fgs_talk Kashya 4
  # the Monastery road: Stony Field, the Underground Passage, Dark Wood, Black Marsh, Tamoe Highland, the Monastery
  fgs_wpgo 3; fgs_hop 4; fgs_wpact; fgs_clear 30 80; fgs_hop 10; fgs_clear 30 60; fgs_hop 5; fgs_wpact; fgs_clear 30 80
  fgs_hop 6; fgs_wpact; fgs_clear 30 80; fgs_hop 7; fgs_clear 30 80; fgs_hop 26 27; fgs_wpact; fgs_clear 30 60
  fgs_wpgo 1; fgs_repair Charsi; fgs_wpgo 27
  fgs_hop 28; fgs_clear; fgs_hop 29; fgs_wpact; fgs_clear; fgs_hop 30; fgs_clear; fgs_hop 31; fgs_clear; fgs_hop 32; fgs_wpact; fgs_clear
  fgs_hop 33; fgs_clear; fgs_hop 34; fgs_clear; fgs_hop 35; fgs_wpact; fgs_clear
  fgs_wpgo 1; fgs_repair Charsi; fgs_rest; fgs_wpgo 35
  fgs_hop 36; fgs_clear; fgs_hop 37; printf 'wait:3;say:autobuild;'
  printf 'kill:name=Andariel,900;loot:40,40;wait:2;'
  printf 'exit'
}

# the later acts live in their own files (run from the repository root, like verify.sh)
for fg_f in scripts/verify.d/lib/fullgame_a[2-5].sh(N); do source $fg_f; done
