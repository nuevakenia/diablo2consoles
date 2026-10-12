# Where help is needed

Status handoff (2026-10-10, integration 694b5da0). The fork is a native Apple-Silicon macOS Diablo II built on OpenDiablo2 (Go + Ebiten),
using the owner's own 1.14b + Lord of Destruction files. Nothing in this repository contains game data or Blizzard
code. Every number in the engine comes from reading the original executable (read-only, in Ghidra) or from public
format documentation; items marked UNVERIFIED in code or in `docs/KNOWN_GAPS.md` are guesses.

How the project is verified: `scripts/verify.sh` (one scenario or all), `scripts/verify_parallel.sh 6` (the full
run, six jobs), scenarios in `scripts/verify.d/`. They need the owner's game files plus the environment variables
described in `docs/macos-quickstart.md`. Use a private `OD2_VERIFY_CONFIG_DIR` (a copy of `config.json`) so test runs
do not write into the real Saves folder. Scenarios run at 1512x982 by default (`docs/DISPLAY.md`) and the long
playthroughs run in turbo mode (`OD2_TURBO`, `~/git/d2-re-notes/turbo-mode.md` in the author's notes).

## Highest value: people with a Mac and the game files

- **Play it.** Nobody except the author has played the build by hand. Report anything that feels wrong: clicking,
  walking, holding the button to walk, the Cmd+click right skill, widescreen layout at 1920x1080 and ultrawide,
  Acts 1-5, multiplayer host/join, trading, sound. The human-played and human-listened bars are in `docs/progress.json`
  under v2.
- **Sound and music** have never been confirmed by a human listen.

Recent fixes you can verify by hand: menus scale to the window at any size; a character created in the menu keeps its own map seed (town layout stays the same on reload).

## Engine fidelity (needs a Ghidra reader)

- Skills with approximate behaviour: Charged Strike aim and Fend/Zeal hit cadence (8-frame guess) are open. Wave 24 closed
  Conversion (leader detach and aura-state strip), the Inferno / Arctic Blast held-button channel, the Strafe lone-enemy
  burst and the client Strafe / Lightning Strike extras. Blood Golem, Fire Golem absorb, Armageddon and the Holy Bolt party
  packet were already in; the only leftovers there are the Holy Fire pulse timing of the Fire Golem and the host
  `ChannelAnimSlack` of the Inferno channel (both marked UNVERIFIED in the code).
- Monster AI: 134 of 148 are ported faithfully. w24 re-read the open pieces in the exe: the Zealot fear gate, the
  Regurgitator and Siege Tower scan filters, the Fallen Shaman scan and Diablo's player-count output are ported and
  hosted (the unit-flag bits 1 and 9 of the Regurgitator filter are only approximated); the Bat Demon flight boost
  works once monsters carry the regeneration stat (branch feat/w23-difficulty). The Assassin's Shadow Warrior and
  Shadow Master now run their ported thinkers (follow, leash, plain attack); their skill scoring and the copy of the
  hero's skills are still not hosted.
- UI panels: move-gold dialog and NPC menu now have golden numbers (9f-ui-layout, 800x600); the party panel loot toggle
  shows for hardcore heroes (game-info flag 4); its effect (a packet) and a party golden (needs two players) are open.
- The summon caps: the Hydra bound is the exe pet limit (skills.txt petmax, 99) and EvilHole is bounded by its aip1; DiabPrison
  (4 per cast) and the Nest / MinionSpawner stand-in bound of 10 stay host choices because the exe has no live count for them.
- Lava levels 107/108 make about 8x fewer monster groups than the density rolls predict; probably the estimate, not
  verified against the original.
- Ubers (UberDiablo, UberMephisto) stand still as in the exe in this build; restore the two `register` lines in
  `ai_boss2.go` if fighting Ubers are wanted.
- The ranked gap list from the source-file study lives in the author's notes (`gaps-from-study.md`); items N1-N25.

## Features not started or parked

- Windows build: a zig cross-compile worked once (needs the Unix-only `autoperf.go` split); not committed.
- Cinematics: Bink video and audio decode exist (`d2common/d2data/d2video`, off by default `OD2_PLAY_VIDEOS=1`);
  audio sync was only checked by clock, not by ear; the Cinematics menu path is untested.
- Widescreen: the bar side fill is now `tile` by default (`OD2_BAR_FILL=none` to turn off); the subtitle box is centred on the column (golden numbers); ultrawide not looked at by eye.
- Scenario hardening: `scripts/verify.d/wip/8g-click-dungeon.sh` (Jail Level 1 click matrix) does not pass yet;
  three real-input scenarios need a retry on first attempt (a click that lands mid-cast becomes a walk).
- `test/real-input-3` (on the fork, not landed): scripted input through the input manager plus new UI/keyboard/click
  scenarios; it made the party/PvP scenario fail (`panel "party" did not open`), so it was left out. Start there.
- Hit-queue purge (N16, `OD2_HITQUEUE=1`) is default off because `9f-pvp-skills-ear` fails with it on.
- The in-repo Bink tables credit FFmpeg (LGPL-2.1-or-later); pin a release tag if provenance matters.

## Wave 20 notes (2026-10-11)

- Quest rules from the exe are landed (Act 1-5); the Rescue prisoners, the frozen Anya block, the Gidbinn boss and the Act 5 quest order are in the world (wave 22); the town Anya and the iced Drehya monster are not.
- The class matrix (docs/class-matrix.md) passes 35/35; the walk give-up clock was fixed (it used to reset every cycle).
- Exe monster modifiers are now applied: super-unique/champion life bonuses made the Den of Evil lethal to a level-1 hero without Fire Bolt.
