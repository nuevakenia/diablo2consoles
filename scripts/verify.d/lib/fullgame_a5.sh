# Act 5 of the full-game chain (sourced by fullgame_scripts.sh).

# Harrogath, the Siege (Shenk), Rescue on Mount Arreat, Larzuk's and Malah's talks.
fg_sc_a5a() {
  printf 'wait:3;'
  fgs_wpact; fgs_repair Larzuk; fgs_talk Larzuk 4; fgs_talk Malah 4; fgs_talk Qual-Kehk 4; fgs_talk Nihlathak 4
  fgs_hop 110; fgs_clear 30 80; printf 'kill:name=Shenk,900;loot:30,40;'
  fgs_hop 109; fgs_talk Larzuk 8; printf 'say:dumpjunk;say:giveitemq lsd 2;say:pickitem lsd;move:npc=Larzuk;until:REWARD socket done,60;say:putitem;wait:1;'
  fgs_hop 110 111; fgs_wpact; fgs_clear 30 80; printf 'kill:name=prison door,900;'
  fgs_wpgo 109; fgs_talk Qual-Kehk 8; fgs_talk Malah 8
  printf 'exit'
}

# The Frozen River: the block without the potion (Anya's hint), Malah's potion from the ground of the town, the block again,
# Anya's reward.
fgs_block() { printf 'walkto:object=558;wait:1;walkto:object=558;wait:1;walkto:object=558;wait:1;walkto:object=558;wait:3;'; }
fg_sc_a5b() {
  printf 'wait:3;'
  fgs_repair Larzuk
  fgs_hop 110 111 112 113; fgs_wpact; fgs_clear 30 80
  fgs_hop 114; fgs_block
  fgs_hop 113; fgs_wpgo 109; fgs_talk Malah 8
  printf 'say:dumpjunk;say:pickground ice;wait:5;say:putitem;wait:1;'
  fgs_wpgo 113; fgs_hop 114; fgs_block
  fgs_hop 113; fgs_wpgo 109; fgs_talk Malah 8; fgs_talk Anya 8; fgs_talk Anya 8
  # Anya opens the portal to Nihlathak's Temple: the Betrayal of Harrogath
  printf 'walkto:object=Portal;wait:3;expect:level=121;wait:3;'
  fgs_clear 30 80; fgs_hop 122; fgs_clear 30 80; fgs_hop 123; fgs_wpact; fgs_clear 30 80; fgs_hop 124
  printf 'wait:3;kill:near=60,2500;kill:near=60,2500;loot:30,40;'
  fgs_hop 123; fgs_wpgo 109; fgs_talk Anya 8; printf 'say:dumpjunk;say:giveitemq skp 4;say:pickitem skp;move:npc=Anya;until:REWARD personalize done,60;say:putitem;wait:1;'; fgs_talk Qual-Kehk 8; fgs_talk Qual-Kehk 8
  printf 'exit'
}

# The Ancients on the Arreat Summit (the altar wakes them), the gate of the Worldstone Keep, the Throne of Destruction.
fg_sc_a5c() {
  printf 'wait:3;'
  fgs_repair Larzuk
  fgs_wpgo 113; fgs_hop 115; fgs_wpact; fgs_clear 30 80; fgs_hop 117; fgs_wpact; fgs_clear 30 80; fgs_hop 118; fgs_wpact; fgs_clear 30 80
  fgs_hop 120; fgs_clear 30 80
  printf 'walkto:object=546;wait:5;kill:near=60,1500;kill:near=60,1500;kill:near=60,1500;wait:5;'
  printf 'walkto:object=547;wait:5;'
  fgs_hop 128; fgs_clear 30 80; fgs_hop 129; fgs_wpact; fgs_clear 30 80; fgs_hop 130; fgs_clear 30 80; fgs_hop 131
  printf 'exit'
}

# The Throne of Destruction: the waves of Baal's minions, Baal himself, the portal to the Worldstone Chamber, and the end.
fg_sc_a5d() {
  printf 'wait:3;'
  fgs_repair Larzuk
  fgs_wpgo 129; fgs_hop 130 131
  # the throne sends five waves; a wave comes 250 frames after the one before is announced, so the hero waits and fights
  local i
  for i in 1 2 3 4 5 6; do printf 'wait:14;kill:near=90,1500;'; done
  printf 'wait:20;walkto:object=Portal;wait:3;expect:level=132;wait:5;kill:near=90,3000;kill:near=90,3000;wait:5;'
  printf 'exit'
}
