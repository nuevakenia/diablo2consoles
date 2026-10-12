# Status log (moved out of the README)

The long status board, plan, backlog, branch list and dated success log that used to sit in the README. The README keeps
the short honest summary; update this file when something is verified. No game data or Blizzard code belongs here.

## Status board

_Last updated: 2026-10-11 (wave 21 landed: integration 694b5da0; full verify 5 of 6 jobs passed 39/39 and the 6th's only red (9m Act 3 orb needs Khalim's Will wielded) was fixed and re-run alone; all 7 classes x Acts 1-5 playthroughs pass (35/35); starter equipment, quest rules from the exe, assassin sentry AI, aura pulse period; wave 20 landed: integration c93982b3, full verify ALL JOBS PASSED 39 of 39 scenarios x6 after two fixes (audio music clock, Act 1 playthrough hero now levels Fire Bolt because exe monster modifiers make the Den's Corpsefire pack lethal to a level-1 hero); skills 177/210, faithful AIs 134/148, UI panels 12/14; batch 15 landed: integration d9a079d2, run 32062, six jobs ALL PASSED with no retry at 37/37/37/36/36/36 scenarios, every scenario now running at 1512x982 with 1920x1080 and ultrawide support, turbo mode on eight playthroughs, per-level multiplayer worlds, skills/AI/gap work from the exe; v1 weighted headline about 87.6%; v1 scope change: the human-tester and human-listen (sound) bars moved to v2, so the weighted headline reads about 85.4% on the same bars; after batches 8 and 9 (integration 1e283cd2; 7 clean full verifies in a row through batch 7; batch 8 had no clean full run on its final tree, so the streak after that interruption is 1 (batch 9)); earlier, after the pass6 landing; Game v1 complete 69%, 70% with held branches; fourth full verify run 77399: all jobs passed, no retry; earlier run 47389: 71 of 74 sections green, 3 red) · Claude updates this on every merged success._

_2026-10-12 (wave 36-43 landed as land/w38 f6fe5248, full verify run 813: ALL JOBS PASSED, 291 scenarios in 6 jobs, first attempts retried only for flakes): monster missile skills hit the hero (bosses), exe-ported Hireable/pet/summon AI, Rite of Passage altar and gate on the right objects, quest sharing and robustness fixes in multiplayer, mouse wheel and keyboard item moving, vendor tabs, character sheet damage boxes, neighbour-level backdrop, classic (non-LoD) game mode, tooltip item compare, mercenary hire list and portrait, cinematics, fullscreen default, save hardening. feat/w43-fullgame (Normal to Hell chain) is NOT in this tree: it broke Nightmare scenarios and stays on its branch._

### ✅ Working now (each one verified, not assumed)

| Done | Evidence |
|---|---|
| **Skills for all seven classes** (~130 skills): the status-effect engine (poison, burn, chill, stun, fear, curses), auras, summons (skeletons, golems, wolves, ravens), traps, Paladin/Druid/Assassin/Barbarian/Amazon/Necromancer/Sorceress castings | Scenario casts 30 skills across the 7 classes; several formulas spot-checked in the binary, others marked unverified |
| **Skill hotkeys and the skill selection screen**: 16 hotkey slots, the popup laid out like the real game, spending points with prerequisites, all saved in the `.d2s` | Scenario selects, assigns F-keys, spends points and checks the exported header; screenshots read |
| **Remaining boss and monster behaviours**: Summoner, Duriel, Mephisto, Diablo, Izual, Vulture, Baal minions and the forced states (fear, confuse, attract, charm) | Real Summoner skill slots confirmed in the binary; monsters fight monsters when charmed or confused |
| **Nightmare and Hell**: a difficulty screen (only unlocked difficulties), the real scaling tables (a Nightmare Fallen: level 36, defence 369, damage 13-27, 1,669 xp), per-difficulty quests and waypoints | Checked against the real tables; saved in the `.d2s` active-difficulty byte |
| **Multiplayer**: two instances join over TCP using the real game's packet framing (Huffman table read from the binary); both build the same level from the host's map seed, see each other, walk, cast and chat | Two-process scenario passes: both saw the other's name and position, walk/cast/chat arrived, clean leave |
| **The quest system**: the intro quests, all of Act 1 and Radament's Lair — states, NPC speech with the real voice lines, rewards, quest log, saved in the `.d2s` | A scripted Act 1 line passes 70 of 70 checks; Den of Evil with real killed cave monsters; the quest bits appear in the exported `.d2s` |
| Runs natively on Apple Silicon with the **real 1.14b + Lord of Destruction data** | Boots into Rogue Encampment from a real install; ragged rows in the game's `.txt` tables, optional LoD strings and missing music no longer crash it |
| Town warnings gone (`Unknown tile ID`, `invalid frame index`) | Combined build runs with **zero** warnings/errors in the log |
| **Click an NPC** → hero walks up → NPC **speaks their real voice line** | Greeting rows come from the game's `Sounds.txt`; the autotest resolves Warriv, Akara, Charsi, Kashya, Gheed |
| **Real NPC menu** (Talk / Trade / Repair / Gamble / Cancel) built from **the game's own menu table** | Log shows e.g. Akara `[talk trade cancel]`, Charsi `[talk trade/repair cancel]`, Gheed `[talk trade Gamble cancel]`; not yet checked visually |
| Greeting logic ported from the real picker (inactive group, time-of-day lines, no repeats) | Unit tests; Warriv (no plain hello row) now speaks |
| **Engine saves back to a real `.d2s`** (autosave every ~5.5 min, on exit): an unchanged export is byte-identical; edited gold/level/quests re-parse with a valid checksum | The original file is never modified; the in-game autosave test wrote `gold=31337 checksum=ok` |
| **Whole `.d2s` character save — read and WRITE**: header, quests, waypoints, stats, skills, **items**, corpse, mercenary | Parse → write of a real save is **byte-for-byte identical** (2,663 bytes); Checksum, all 16 attributes, 30 skills and **60/60 items** match a reference parser on a real level-94 save; header/body also on a second real save |
| **Import a real character into the engine — with her gear** | A level-94 Sorceress from a real `.d2s` loads, starts in town and wears her real Spired Helm, Archon Plate, Battle Boots, Light Gauntlets, Flail, Short Staff and Monarch |
| **Diablo II's own random number generator** (`d2rand`) and the level-seed hierarchy | Reverse engineered from the binary; tests use independent Python vectors; checked instruction-by-instruction against the real code: no differences |
| **Test without clicking** (`OD2_AUTOGAME`, `OD2_AUTOTALK`, `OD2_AUTOMENU`, …) | Lets the AI verify changes by itself; see the quickstart |
| Reverse-engineering map of the game | all 11,257 functions named in Ghidra (the last 22 Warden/module names are partly unverified); 277 source files indexed; notes on units, saves, packets, rendering, sound, NPC menu, **skills and combat formulas** |

### 🔧 In progress right now (agents run in waves; the machine is the limit)

| Work item | Where |
|---|---|
| **Acts 2 to 5 towns and travel between acts** (merging with the latest code) | branch `feat/act-towns` |
| **A scripted first hour of Act 1**, finding and fixing the seams between systems | branch `feat/act1-playthrough` |
| **Exact outdoor tile ids**, proven against the real game | branch `feat/drlg-tiles` |
| **Act 4 and 5 generators in Go** (spec verified against the real game) | next |
| Research: Act 2 desert and Act 3 jungle generators | RE notes: `drlg-act23-outdoor` |
| Imported-character UI, last merge pending | `feat/imported-hero-ui` |

### 🎯 Plan and priorities (set by Claude)

1. **Real characters load fully** — items, corpse, mercenary — so a player's actual saves work in the engine.
2. **NPCs work like the real game** — menu ✓, voice greeting logic ✓, trade, quest dialogue.
3. **Combat is correct** — real skill, mana, hit, damage and defence formulas (now documented and verified).
4. **Real maps** — reproduce the original level generator from a seed.
5. **Loot and monsters** — item generation, monster AI, quests.
6. **Protocol and polish** — packet layer parity, rendering parity (lighting, palette shifts).

### 📋 Backlog (not started)

Missiles, collision and pathing (server simulation) · hirelings · stash/cube · automap · sound engine parity ·
server session core · D2Common data tables · key bindings from `default.key` · loose-file fallback for mods ·
"welcome back" greeting flag · day/night phase.

## Branches

| Branch | What |
|---|---|
| `master` / `integration` | Everything merged and building — **start here** |
| `fix/real-game-data-1.14b` | Load unmodified 1.14b data |
| `fix/town-tile-and-button-frame-warnings` | Town warnings |
| `feat/npc-click-interaction` | NPC click, voice greeting, autotest harness |
| `feat/npc-menu` | Real NPC menu and ported greeting picker |
| `feat/d2s-save-header`, `feat/d2s-items`, `feat/d2s-import` | Real `.d2s` saves |
| `docs/macos-quickstart` | macOS setup guide |

## Success log (newest first)

| Date | Success |
|---|---|
| 2026-10-10 | **Batches 8 and 9 landed** (`integration` = 1e283cd2, full verify record: batch 6b run 31638 passed (6th clean in a row), batch 7 run 7644 passed (7th), batch 8 run 36803 had two red scenarios (`92-objects2` and `93-objects-gaps`; fixed and re-run alone, so no clean full run on the final tree), batch 9 run 98647 passed fully; current streak is 1 clean full verify after the batch 8 interruption). Batch 8: the Portal shrine now sends the hero to the act start; the Sand Maggot Queen lays her spawn units (7 in the `9j-quest-staff` scenario, bounded by the aip1 data and `HostSummonCap` 40); population minimums use `auto` on 91 scenarios; the 92/93 object-scenario regression caused by the portal level change was fixed by an autotest skip (`OD2_AUTOOBJECT_SHRINE`, removed again once the shrine opened a portal instead of changing level). Batch 9, Mac click controls: a ground click always walks, a monster click uses the left skill, Shift+click stands still, and right-click, two-finger click, Ctrl+click and Cmd+click use the right skill; hold-to-walk works in camp; the exit-walk stall clock is reset (a hypothesis fix, UNVERIFIED: scenario 9h stays on the watch list); player death removes the hero's minions (the D2MOO rule, since verified against the exe). The `HostSummonCap` values stay UNVERIFIED host bounds (the exe has no live cap in the summon handlers; see exe-verify-unverified.md). |
| 2026-10-10 | **Batches 6b and 7 landed** (`integration` = 5732ca55, seven clean full verifies in a row): batch 6b monster summons, Bone Wall and Rabies hooks, and the Stygian Hag brood fix; batch 7 summoner `HostSummonCap` safety caps (MinionSpawner 10, Hydra 9, Overseer Whip/Impregnate 6, DiabPrison 4; the cap values are UNVERIFIED against the exe) and the party-trade refusal sequence fix. |
| 2026-10-10 | **Batch 6a landed** (`integration` and `master` = 4e1824e3): the exact PvP damage bound in the PvP scenario (no drift found across four batches), level quest gates enforced in the engine (Canyon, tombs, Palace, Arcane Sanctuary, Durance and the Worldstone levels, from the exe's rules), and exe-derived object behaviour: barrels explode with damage, chests are locked or trapped (keys consumed, trap monsters), wells have two pulses and refill, racks create a random base item, shrines act (Storm, Exploding, Poison, Warping, Gem Upgrade). Full verify: five of six jobs green; the sixth crossed the new gates with a scenario that had not completed the gating quests (fixed in the scenario). Next: batch 5 (quests, all eight scenarios green alone), batch 6b (monster summons, Bone Wall, Rabies) and the seven-class playthrough matrix |
| 2026-10-10 | **Batch 4 landed, second fully green full verify in a row** (`integration` and `master` = 5510c7b4, no flake and no retry): all 26 skill missile gaps closed with real function bodies from the exe (Meteor, Blizzard, Volcano, Frozen Orb, Bone Wall, the Barbarian shouts, Rabies and more), burning ground now hurts players through a fractional damage carry, and the trade window race is closed in the client, protocol and server with an offer sequence number. Barbarian passes all five acts with real combat (28 min, no deaths). Next: quests (Siege and Blade), the seven-class playthrough matrix, and your play-test |
| 2026-10-10 | **Batch 3 landed, first fully green full verify** (`integration` and `master` = 697ecb6c): the whole Act 3 chain with real combat (Docks to Travincal, the Council, Khalim's Flail, the Orb, Durance, Mephisto and the red portal), the multiplayer game screen (host and join through the menus, two windows synced through the realm) and the real Act 2 wall fade. Run 63179: all six jobs passed, 193 scenarios, one retried flake. Next: batch 4 (skills missiles incl. Meteor/Blizzard PvP, the trade offer race fix, quest walkthroughs), the all-classes playthrough matrix and your play-test |
| 2026-10-10 | **Batch 2 landed** (`integration` and `master` = 59012a7a): natural mana regeneration from the exe (25 Hz; replenish-life item stat; the Act 5 caves mana failures dropped from about 4,000 to 400), the Act 5 endgame table fix (Baal's waves), preset population and exact maze tiles, class-skills retry, a chase fix (no warp click while fighting) and the town left-click fix (a spell on the left button no longer stops walking in town). Full verify: all five acts' playthroughs pass; nine scenario reds (stale expectations, a PvP margin the regen closed, a trade-window race) were fixed and re-run alone. Next: batch 3 (Act 3 depth, multiplayer game screen, Act 2 wall fade), then skills missiles and quests |
| 2026-10-10 | **Batch 1 landed** (`integration` and `master` = 1dcef1be): Mac release hardening (release.sh, dmg and bundle checks, CI), the exe-derived stamina model and bare-hand damage, and the world-objects audit (wells, shrines, chests). Full verify: 4 of 6 jobs green; the two reds were load and timing flakes (a waypoint cooldown margin, a monster standing on arrival stairs) that passed alone after fixes. Next: batch 2 (natural mana regeneration, Act 5 endgame table fix, preset population, class-skills retry), then skills missiles, multiplayer game screen, quests and Act 3 depth |
| 2026-10-10 | **LANDED: first real full verify green on the combined tree** (`integration` and `master` = 2e50f0e4): pass5 + population + playthroughs + skills-mp + travel + the Act 3 unseal, curse retry and Act 5 caves fixes. Run 50637: 6 jobs x 30 scenarios, 177 green; the 3 reds (class skills, Act 3 and Act 5 caves playthroughs) were fixed and passed alone. All five acts now pass a scripted real-combat playthrough (Act 3 in 874 s). Caveats: no second full run on the final tree; the Act 5 caves pass relies on restoring vitals (the engine has no natural mana regen yet, agent working on it). Headline Game v1 complete 64% (66% with held branches) |
| 2026-10-09 | **Progress board refreshed from real numbers**: Ghidra all 11,257 `Game.exe` functions named (last 22 Warden/module functions named later the same day, a few names unverified); new bar *Real full verify green* from a real `verify_parallel.sh` log (74 sections ran, 71 green; red: Act 3 gate, skill bar, act travel; 4 of 6 jobs ALL CHECKS PASSED); *Game v1 complete* 58% (64% with branch work) |
| 2026-10-09 | **Honest verification reset**: earlier parallel verifies ran only unit tests (filter bug) or skipped scenarios (unset sample save), so "all green" claims were void; guard added on a branch. First real runs: ~120 scenario sections across 6 jobs, red only on Act 3 gate (root cause found: level 100 stair tile style), skill bar (two root causes fixed) and load-dependent flakes. Pending on branches, not yet landed: exact maze/preset tiles, unique/champion packs, audio volume rules, difficulty and combat-math audits, 22 last exe names (11,257 of 11,257 named) |
| 2026-10-09 | **Exact tile placement for Acts 2–5 outdoors** (72 levels, 7,558 rooms equal to the emulator), the **Act 2 start playable** (Lut Gholein → Rocky Waste → Halls of the Dead), level state kept when you leave and return, **caves 13–16 and 37**, **Acts 2–5 quest logic**, **monster AI for every archetype** (17 faithful ports, the rest stand-ins), **real NPC speech and music fixed** (the paths were wrong; test runs were muted so nobody noticed), progress chart over time |
| 2026-10-09 | **Acts 2, 3, 4 and 5 level generators match the real game (1,626 levels)**, exact tile records for Act 1 outdoors (240 levels), act towns and travel, party/trade/PvP, boss encounters and Baal waves, options menu and hardcore death, the scripted first hour of Act 1, imported-hero UI, `OpenDiablo2.app` packaging merged (full verify: ALL CHECKS PASSED) |
| 2026-10-09 | Skills for all seven classes, skill hotkeys and the selection screen, Nightmare/Hell difficulty, boss behaviours and forced states, multiplayer and documentation merged; Act 4/5 level generators specified and checked against the real game |
| 2026-10-09 | **Act 1 outdoors match the real game on 240 levels**; mercenaries, equipment rules, world objects, automap and a 5× faster frame merged |
| 2026-10-09 | Death/respawn/new characters merged (new Druid byte-identical to the real file); the Act 1 outdoor generation algorithm reverse engineered and a reference port matched the real game on 208 levels |
| 2026-10-09 | **All maze levels of all five acts proven identical to the real game**; equipment affects the hero (explains 1241/869); positional/ambient audio; a double-clickable Mac app |
| 2026-10-09 | Level generator proven for **all Act 1-3 maze levels**; waypoints, portals, doors and level changes; Gheed's gamble and Cain's identify; test scenarios now launch games without Terminal windows |
| 2026-10-09 | **Level generator proven identical to the real game** (2,550 maze records, 50 world layouts); real dungeons render and play; skills, monsters part 2, stash/cube/belt, lighting merged; test runner restructured into one file per scenario |
| 2026-10-09 | Engine saves characters back to real `.d2s`; hireling and renderer reverse engineering done (real lighting model, hire cost and stat formulas) |
| 2026-10-09 | **Monsters** with the original AI, pathfinding, combat, death and loot run in the engine; test runs no longer collide on the server port |
| 2026-10-09 | Vendors, ground items and chests, the sound engine, CI + scripted autotests and the Go level generator merged; skills part 2 researched (365 skill functions named, the calc language decoded) |
| 2026-10-09 | Loot, trade, quests, packets, key bindings and day/night merged; monster AI think functions named (~90 created); maze level generation and the Act 1 world layout reverse engineered |
| 2026-10-09 | `.d2s` **writer**: a real save round-trips byte-for-byte; combat formulas implemented and verified against 12 binary functions |
| 2026-10-09 | Quests (state layout, framework, Den of Evil) and inventory/trade (price formulas, packets) reverse engineered |
| 2026-10-09 | Item generation reverse engineered: treasure classes, quality rolls with magic find, affixes, property stats (12 gaps vs OpenDiablo2 listed) |
| 2026-10-09 | Monster AI mapped: 148 AI types, aggro rules, spawning and level scaling |
| 2026-10-09 | Random number generator checked instruction-by-instruction against the real binary: no differences |
| 2026-10-09 | Equipment imported from a real `.d2s`; Diablo II's random number generator and seed hierarchy implemented (`d2rand`) |
| 2026-10-09 | Level generation (DRLG) core reverse engineered: seed hierarchy, dispatch, preset/maze/outdoor structure |
| 2026-10-09 | Skills and combat formulas reverse engineered and verified (mana cost, to-hit, defence, block, critical strike, RNG) |
| 2026-10-09 | Real NPC menu opens from the game's menu table; greeting picker ported |
| 2026-10-09 | `Parse` reads a whole save (corpse, mercenary, golem) |
| 2026-10-09 | Real `.d2s` items parse: 60/60 items match the reference; item tables decoded from the game's `.bin` |
| 2026-10-09 | Real level-94 character imported and started in the engine |
| 2026-10-09 | Town warnings confirmed gone in the running game |
| 2026-10-09 | Real `.d2s` body (quests, waypoints, stats, skills) verified on two saves |
| 2026-10-09 | NPC greetings fixed for every town NPC; autotest harness lets the AI verify without clicking |
| 2026-10-09 | Ghidra decompiler built for macOS; first real functions read; NPC menu table found |
| 2026-10-09 | Engine boots with real Diablo II 1.14b + LoD data on Apple Silicon |

