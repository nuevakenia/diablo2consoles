# Where help is needed

Status handoff (2026-10-10, integration 47989eb2). The fork is a native Apple-Silicon macOS Diablo II built on OpenDiablo2 (Go + Ebiten),
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

## Engine fidelity (needs a Ghidra reader)

- Skills with approximate behaviour: 168 of 210 have engine behaviour. Open: Blood Golem, Fire Golem, Armageddon
  meteors, Conversion flag bits, Holy Bolt healing remote party members (needs a packet), Charged Strike aim and
  Fend/Zeal hit cadence (8-frame guess), Strafe/Lightning Strike extras on the client.
- Monster AI: 128 of 148 are ported faithfully. Open: Shadow Master feed, Zealot fear gate, Regurgitator and Siege
  Tower scan filters, FallenShaman scan, Diablo's second output, BatDemon flight boost (no monster regeneration).
- UI panels: 11 of 14 match the original; move-gold dialog and the NPC menu golden are open; the party panel is
  missing one toggle.
- The summon caps (MinionSpawner 10, Hydra 9, Overseer/Impregnate, DiabPrison 4) are safety bounds, not exe values.
- Lava levels 107/108 make about 8x fewer monster groups than the density rolls predict; probably the estimate, not
  verified against the original.
- Ubers (UberDiablo, UberMephisto) stand still as in the exe in this build; restore the two `register` lines in
  `ai_boss2.go` if fighting Ubers are wanted.
- The ranked gap list from the source-file study lives in the author's notes (`gaps-from-study.md`); items N1-N25.

## Features not started or parked

- Windows build: a zig cross-compile worked once (needs the Unix-only `autoperf.go` split); not committed.
- Cinematics: Bink video and audio decode exist (`d2common/d2data/d2video`, off by default `OD2_PLAY_VIDEOS=1`);
  audio sync was only checked by clock, not by ear; the Cinematics menu path is untested.
- Widescreen leftovers: bottom HUD bar side fill (`OD2_BAR_FILL`), NPC subtitle placement.
- Scenario hardening: `scripts/verify.d/wip/8g-click-dungeon.sh` (Jail Level 1 click matrix) does not pass yet;
  three real-input scenarios need a retry on first attempt (a click that lands mid-cast becomes a walk).
- `test/real-input-3` (on the fork, not landed): scripted input through the input manager plus new UI/keyboard/click
  scenarios; it made the party/PvP scenario fail (`panel "party" did not open`), so it was left out. Start there.
- Hit-queue purge (N16, `OD2_HITQUEUE=1`) is default off because `9f-pvp-skills-ear` fails with it on.
- The in-repo Bink tables credit FFmpeg (LGPL-2.1-or-later); pin a release tag if provenance matters.
