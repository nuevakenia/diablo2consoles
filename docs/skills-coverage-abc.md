# Amazon, Barbarian and Assassin skill coverage

Skills of the three classes (30 each) against what the skill engine (`d2common/d2skill` pipeline, `d2core/d2skills` engine, `d2common/d2missile` simulation) does. Skill rows come from the 1.14 patch_d2 skills table (`D2_TABLES/skills/patch_d2/skills.txt`, the decompiled form of the compiled `.bin` that the exe reads; the old `d2exp/skills.bin` has a different layout and is not used). srvst/srvdo/clt* are the function ids of the exe tables (0x72f8d8 / 0x72fa48 / 0x726898 / 0x7269b8, see `skills-2.md` of the reverse engineering notes).

Status words: **fully simulated** (a ported function plus the missiles/states it needs, the rules read from the exe or documented), **partial** (runs, but the listed part is not simulated or is inferred "U"), **stub** (accepted but only a placeholder effect), **missing** (no do function, `Implemented` false). The status is from reading the handlers and the missile closure of each skill, not from an exhaustive per-skill exe audit; passives count as simulated when their stats reach the combat code.

| | before | after |
|---|---|---|
| fully simulated | 64 | 73 |
| partial | 17 | 17 |
| stub | 1 | 0 |
| missing | 8 | 0 |
| total | 90 | 90 |

## Amazon

| id | skill | st/do | clt st/do | before | after | notes |
|---|---|---|---|---|---|---|
| 6 | Magic Arrow | -/- | -/- | fully simulated | fully simulated | wave 23: SrvDmgFunc 1 (0x5aaba0) moves dl12 = 1 + (l-1) percent of the physical blow to magic |
| 7 | Fire Arrow | 4/- | 11/- | fully simulated | fully simulated | wave 23: SrvDmgFunc 1 moves dl12 = 3 + 2(l-1) percent of the blow to fire (was missing) |
| 8 | Inner Sight | -/6 | -/- | fully simulated | fully simulated | wave 23 read: SRVDO_InnerSight + SKILL_CreateTimedStateStatList; UNVERIFIED gap: the monster gates of 0x56c740 (monstats flag 0x100, a monstats2 byte) |
| 9 | Critical Strike | -/- | -/- | fully simulated | fully simulated | passive stats (PassiveTotals / mastery / defense hooks) |
| 10 | Jab | 5/7 | 12/16 | fully simulated | fully simulated |  |
| 11 | Cold Arrow | 4/- | 11/- | fully simulated | fully simulated | wave 23: SrvDmgFunc 1 moves dl12 = 3 + 2(l-1) percent of the blow to cold |
| 12 | Multiple Shot | 4/8 | 11/17 | fully simulated | fully simulated | wave 23: create flag 0x800 passes calc2 (par3 = 1) as the Activate delay of every arrow; the flank flag 0x10000 (data flag 2 -> damage flag 0x80) is UNVERIFIED |
| 13 | Dodge | -/- | -/- | fully simulated | fully simulated | passive stats (PassiveTotals / mastery / defense hooks) |
| 14 | Power Strike | 6/2 | -/- | fully simulated | fully simulated | wave 23 read SRVST_006 (0x5d9470): calc1 is the damage percent, the lightning is rolled into the blow |
| 15 | Poison Javelin | 4/- | 11/- | partial | fully simulated | the trail of clouds (poisonjav do function 2, poisonjavcloud do function 3) is in d2missile; wave 23: the cloud builds its own descriptor without the weapon blow (missile SrcDamage -1, 0x64ca60), so it does poison only |
| 16 | Exploding Arrow | 4/- | 11/- | fully simulated | fully simulated |  |
| 17 | Slow Missiles | -/6 | -/- | fully simulated | fully simulated | wave 23: the state's stat 0xa1 (skill_handofathena, ln12 = 33) now scales the speed of the missiles a monster fires (0x59d5d0: velocity * stat / 100, CanSlow missiles only); before, nothing read it |
| 18 | Avoid | -/- | -/- | fully simulated | fully simulated | passive stats (PassiveTotals / mastery / defense hooks) |
| 19 | Impale | 7/2 | -/- | fully simulated | fully simulated |  |
| 20 | Lightning Bolt | 4/- | 11/- | fully simulated | fully simulated | wave 23: SrvDmgFunc 12 (0x5aac30) turns the whole javelin blow (dl12 = 100) into lightning and clears the other fields |
| 21 | Ice Arrow | 4/- | 11/- | fully simulated | fully simulated | wave 23: SrvDmgFunc 2 (0x5aacf0) turns the cold length into the freeze length (dParam1 = 100 percent) and clears the chill: the arrow freezes (it only chilled before) |
| 22 | Guided Arrow | 4/10 | 11/18 | fully simulated | fully simulated | wave 23 (0x5da2f0): calc1 (ln34) is the arrow's damage percent through the create callback 0x5da2c0 (stat 0x19 += calc1) |
| 23 | Penetrate | -/- | -/- | fully simulated | fully simulated | passive item_tohit_percent (stat 0x77, ln12); wave 23: COMBAT_RollToHit adds stat 0x77 on missile hits too, the missile to-hit of a player now includes it (was skill to-hit only) |
| 24 | Charged Strike | 6/11 | -/19 | fully simulated | fully simulated | wave 23: the blow is SRVST_006: its lightning is part of the melee hit and calc1 (the bolt count) is its damage percent |
| 25 | Plague Javelin | 4/- | 11/- | partial | fully simulated | wave 23 (hit function 2, 0x5a7810 -> 0x5a6d70): the javelin bursts into a fan of 8 slow and 15 fast clouds drifting to the ring of offsets at distance 2 around the struck unit (d2missile cloudFan), each living Range + 3 sub loops; poison only, no weapon blow |
| 26 | Strafe | 8/12 | 13/20 | fully simulated | fully simulated | wave 23 (0x5da6c0): calc2 (ln12) is the damage percent of every arrow (callback 0x5da2c0), srvmissileb for a crossbow |
| 27 | Immolation Arrow | 4/- | 11/- | partial | fully simulated | hit function 9 (fire disc of immolationfire cells plus the radial damage) and do function 5 are in d2missile (disassembly read, wave 20); wave 23 adds SrvDmgFunc 3 on the fire (soft hit chance dParam1 of 128) |
| 28 | Dopplezon | -/15 | -/- | missing | fully simulated | summon (SRVDO_015 0x5dad10): life calc3 percent of the owner, lifetime calc2 frames; the decoy AI is monstats "Idle" (MONAI_Think_Idle 0x5ae8a0 only waits), so it stands still and draws aggro (VERIFIED, wave 20); the client look copy is not checked |
| 29 | Evade | -/- | -/- | fully simulated | fully simulated | passive stats (PassiveTotals / mastery / defense hooks) |
| 30 | Fend | 9/13 | 14/21 | fully simulated | fully simulated | wave 23 (SRVST_009 0x5d99b0): the hit count is min(calc1, enemies in melee range + 4), not always calc1 (Zeal keeps calc1) |
| 31 | Freezing Arrow | 4/- | 11/- | fully simulated | fully simulated |  |
| 32 | Valkyrie | -/16 | -/- | missing | fully simulated | summon (SRVDO_016 0x5daf10 + SKILL_SpawnSummonedMinion); sumskill1..4 Dodge/Avoid/Evade/Critical Strike are given at sumskNcalc levels and roll in the summon blows (wave 20; the monster side of the passive stats is UNVERIFIED) |
| 33 | Pierce | -/- | -/- | fully simulated | fully simulated | passive stats (PassiveTotals / mastery / defense hooks) |
| 34 | Lightning Strike | 10/14 | -/22 | fully simulated | fully simulated | wave 23: the blow carries the skill's lightning and calc1 (20) as damage percent (SRVST_010 0x5d9bb0); the chain victim is the smallest unit id above the struck unit (scan callback 0x569a40), not the nearest |
| 35 | Lightning Fury | 4/- | 11/- | partial | fully simulated | hit function 20 (0x5a8e60 + callback 0x5a8d90) is in d2missile and was checked again in wave 23; the furylightning bolts have SrcDamage -1, so they do lightning only (own descriptor) |

## Barbarian

| id | skill | st/do | clt st/do | before | after | notes |
|---|---|---|---|---|---|---|
| 126 | Bash | 32/2 | -/- | fully simulated | fully simulated |  |
| 127 | Sword Mastery | -/- | -/- | fully simulated | fully simulated | passive stats (PassiveTotals / mastery / defense hooks) |
| 128 | Axe Mastery | -/- | -/- | fully simulated | fully simulated | passive stats (PassiveTotals / mastery / defense hooks) |
| 129 | Mace Mastery | -/- | -/- | fully simulated | fully simulated | passive stats (PassiveTotals / mastery / defense hooks) |
| 130 | Howl | -/22 | -/25 | fully simulated | fully simulated |  |
| 131 | Find Potion | 33/69 | 26/38 | missing | fully simulated | SRVDO_069 0x5d6c40 + the act/difficulty potion table 0x73e988 (golden test) |
| 132 | Leap | 40/77 | 29/43 | partial | fully simulated | Wave 23 VERIFIED SRVST_040 0x5d8830 / 0x5d84d0: range = aurarangecalc (dm12) by octagonal distance, 1 subtile per frame, knockback at the landing |
| 133 | Double Swing | -/70 | 27/39 | fully simulated | fully simulated | Wave 23 VERIFIED 0x5d6ee0: two events, second blow on the next unit id in reach (attackrate calc3 not modelled) |
| 134 | Pole Arm Mastery | -/- | -/- | fully simulated | fully simulated | passive stats (PassiveTotals / mastery / defense hooks) |
| 135 | Throwing Mastery | -/- | -/- | fully simulated | fully simulated | passive stats (PassiveTotals / mastery / defense hooks) |
| 136 | Spear Mastery | -/- | -/- | fully simulated | fully simulated | passive stats (PassiveTotals / mastery / defense hooks) |
| 137 | Taunt | -/71 | -/- | partial | partial | curse-style state with a synthetic stat; AI reaction is U |
| 138 | Shout | -/68 | -/25 | fully simulated | fully simulated |  |
| 139 | Stun | 32/2 | -/- | fully simulated | fully simulated |  |
| 140 | Double Throw | -/74 | 11/42 | fully simulated | fully simulated | Wave 23 VERIFIED 0x5d7340: two missiles on one line (no fan) |
| 141 | Increased Stamina | -/- | -/- | fully simulated | fully simulated | passive stats (PassiveTotals / mastery / defense hooks) |
| 142 | Find Item | 34/72 | 28/40 | missing | fully simulated | SRVDO_FindItem 0x5d7210: bucket roll over Param1..4 picks monstats TreasureClass1..4 and drops it at the corpse |
| 143 | Leap Attack | 41/78 | 30/44 | partial | fully simulated | Wave 23 VERIFIED SRVST_041 0x5d9040 / 0x647040: lands 2 subtiles beyond the target, no range cap, no jump when in melee reach, victim = smallest unit id in reach, blow after the jump (w23-skills-barb.md) |
| 144 | Concentrate | 32/2 | -/- | fully simulated | fully simulated | Wave 23 VERIFIED 0x5d6910: aurastate after the blow (lifetime UNVERIFIED), calc4 percent to magic |
| 145 | Iron Skin | -/- | -/- | fully simulated | fully simulated | passive stats (PassiveTotals / mastery / defense hooks) |
| 146 | Battle Cry | -/68 | -/25 | fully simulated | fully simulated |  |
| 147 | Frenzy | -/9 | -/39 | fully simulated | fully simulated | Wave 23 VERIFIED 0x5d78b0: two blows per swing (second by round robin), state only after a hit, stack up to the skill level with the count as level, calc4 percent to magic |
| 148 | Increased Speed | -/- | -/- | fully simulated | fully simulated | passive stats (PassiveTotals / mastery / defense hooks) |
| 149 | Battle Orders | -/68 | -/25 | fully simulated | fully simulated |  |
| 150 | Grim Ward | 33/75 | 26/41 | missing | partial | corpse consumed, terror state on enemies within aurarangecalc every 6 frames for 200 frames (SRVDO_075 0x5d7430, missile table values); the small/medium/large variant by corpse size is not modelled |
| 151 | Whirlwind | 38/76 | 31/45 | missing | partial | hero walks to the point one subtile per frame hitting the nearest other enemy every weapon-delay frames (SRVST_038/SRVDO_076, 0x5d7a00/0x5d8010); walk speed and the scan radius unit are U |
| 152 | Berserk | 39/2 | -/- | fully simulated | fully simulated | Wave 23 VERIFIED 0x5d82a0: state (zero defense) for calc2 frames, calc4 percent converted to magic |
| 153 | Natural Resistance | -/- | -/- | fully simulated | fully simulated | passive stats (PassiveTotals / mastery / defense hooks) |
| 154 | War Cry | -/68 | -/25 | fully simulated | fully simulated |  |
| 155 | Battle Command | -/68 | -/25 | fully simulated | fully simulated |  |

## Assassin

| id | skill | st/do | clt st/do | before | after | notes |
|---|---|---|---|---|---|---|
| 251 | Fire Trauma | -/- | -/- | fully simulated | fully simulated |  |
| 252 | Claw Mastery | -/- | -/- | fully simulated | fully simulated | passive stats (PassiveTotals / mastery / defense hooks) |
| 253 | Psychic Hammer | 22/33 | 5/3 | partial | fully simulated | VERIFIED SRVDO_033 0x5d17f0 (batch next): one target, class knockback roll; no-target stand-in is U |
| 254 | Tiger Strike | 23/34 | -/- | fully simulated | fully simulated |  |
| 255 | Dragon Talon | 24/42 | 6/4 | partial | fully simulated | VERIFIED SRVDO_042 0x5d4370, SRVST_024 0x5d42a0, 0x5d4190: calc1 kicks, only the last kick that hits may carry knockback (w20); the kicks run in one cast here |
| 256 | Shock Field | -/43 | -/5 | fully simulated | fully simulated |  |
| 257 | Blade Sentinel | -/44 | -/- | partial | fully simulated | VERIFIED SRVDO_044 0x5d49c0: level = caster level (w20); creeper do function 20 and hit function 37 are in d2missile |
| 258 | Quickness | -/18 | -/- | fully simulated | fully simulated |  |
| 259 | Fists of Fire | 23/35 | -/- | fully simulated | fully simulated |  |
| 260 | Dragon Claw | 25/46 | -/- | partial | fully simulated | VERIFIED SRVDO_046 0x5d4cf0 / 0x5d4ba0: one blow per action frame (two per cast here), to-hit bonus plus stat 0x145, calc1 damage bonus, charges released |
| 261 | Charged Bolt Sentry | -/45 | -/- | fully simulated | fully simulated |  |
| 262 | Wake of Fire Sentry | -/45 | -/- | partial | fully simulated | VERIFIED SRVDO_125 0x5cf700 + missile do function 31 0x5addd0 (w20): the maker drops two flames per new cell along +-(quarter turn of the line of fire); sentry fires on the generic trap cadence (U) |
| 263 | Weapon Block | -/- | -/- | partial | partial | passive stat is keyed to the weapon type, so it is left out of PassiveTotals; the block rule lives in d2combat/avoid.go |
| 264 | Cloak of Shadows | -/47 | 7/- | partial | fully simulated | VERIFIED SRVDO_047 0x5d4fe0 / 0x5d4ec0 (batch next): aura on caster, target state + AI state 10 for monsters |
| 265 | Cobra Strike | 23/34 | -/- | fully simulated | fully simulated |  |
| 266 | Blade Fury | 26/48 | 8/6 | missing | partial | one bladefragment1 per do, repeat gate prgcalc1-1 frames (SRVDO_048 0x5d5260); the fragment flight/bounce rules of the missile are the generic ones |
| 267 | Fade | -/18 | -/- | fully simulated | fully simulated |  |
| 268 | Shadow Warrior | -/49 | -/- | fully simulated | fully simulated |  |
| 269 | Claws of Thunder | 23/35 | -/- | fully simulated | fully simulated |  |
| 270 | Dragon Tail | 27/50 | 9/7 | partial | fully simulated | VERIFIED SRVDO_050 0x5d5b90: the kick is the plain kick; the explosion is kick damage x (calc1 + fire mastery) percent in radius par3 around the target with the knockback bit (the damage field, +0x10 = fire, is U) |
| 271 | Lightning Sentry | -/45 | -/- | fully simulated | fully simulated |  |
| 272 | Inferno Sentry | -/45 | -/- | fully simulated | fully simulated |  |
| 273 | Mind Blast | -/51 | -/8 | partial | fully simulated | VERIFIED SRVDO_051 0x5d6120 (batch next): convert (dm56 chance) else damage; radius ignores the prgcalc1 override (U) |
| 274 | Blades of Ice | 23/35 | -/- | fully simulated | fully simulated |  |
| 275 | Dragon Flight | 12/52 | 5/- | missing | fully simulated | VERIFIED SRVDO_052 0x5d6290: needs a target unit; the branch is the animation frame (first: teleport when levels.txt Teleport != 0, 2 refuses a blocked line; second: the kick with the skill bonus + stat 0x145 and the charge release); the port lands next to the target (the exe goes to its position) |
| 276 | Death Sentry | -/45 | -/- | fully simulated | fully simulated |  |
| 277 | Blade Shield | 28/54 | -/- | stub | partial | SRVDO_054 0x5d6880 + SRVDO_142 0x5d6750 area tick every perdelay, skipped in town (w20); hit-roll rule U |
| 278 | Venom | -/18 | -/- | fully simulated | fully simulated |  |
| 279 | Shadow Master | -/49 | -/- | fully simulated | fully simulated |  |
| 280 | Royal Strike | 23/34 | -/- | partial | partial | charge strike works; the chaos ice missile (do function 35) is not simulated |

## What was added

`d2common/d2skill/class_abc.go` registers do functions 15, 16, 48, 52, 69, 72, 75, 76 and replaces 54; `d2core/d2skills/effects_abc.go` applies the new effect kinds (`loot`, `ward`, `whirl`, `shield`); `d2core/d2monsters/findloot.go` drops a corpse's treasure class column or a potion code. Unit tests: `d2common/d2skill/class_abc_test.go`, `d2core/d2skills/effects_abc_test.go`. In-game check: `scripts/verify.d/86-class-skills.sh` (log lines `SUMMON`, `MISSILE create name=bladefragment1`, `LOOT`, `WARD`, `WHIRL`, `SHIELD`).

Numbers taken from the exe for this work: the potion table at 0x73e988 (15 entries of 3 item codes, index = act - 1 + 5 * difficulty), the Find Item bucket rule, the Whirlwind step delay by weapon frame count (0x5d7de0), the eight Blade Fury hit-sub offsets of hit function 52 (not used: Blade Fury throws bladefragment1 directly). No emulator golden was made: the drlg-oracle harness only runs the level generator, so the table golden was read straight from the memory of the exe (read-only Ghidra).
