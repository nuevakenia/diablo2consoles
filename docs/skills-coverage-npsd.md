# Player skill coverage: Necromancer, Paladin, Sorceress, Druid

Generated from `d2common/d2skill/coverage_npsd.go` (`D2_WRITE_COVERAGE=<file> go test ./d2common/d2skill -run CoverageNPSD`).
Source of truth for the skill rows: the patch_d2 `skills.txt` (the export of the compiled `skills.bin`; `D2_TABLES=$HOME/git/d2-tables`).
`do` is the srvdofunc (server do table 0x72fa48 in `skills-2.md`); `-` is the generic missile path.

Status: **F** fully simulated, **P** partial (runs, a documented part is approximated or missing), **S** stub (only a state nothing reads), **M** missing (cast refused), **T** passive (no cast, feeds other skills).
The statuses come from reading the handlers and the scenario logs (`scripts/verify.d/86-class-skills.sh`), not from diffing every skill against the exe. `VERIFIED` notes were read in the decompilation.

| | F | P | S | M | T |
|---|---|---|---|---|---|
| before (start of feat/skills-npsd) | 59 | 42 | 3 | 8 | 8 |
| after | 98 | 14 | 0 | 0 | 8 |


## Necromancer

| id | skill | do | before | after | note |
|---|---|---|---|---|---|
| 66 | Amplify Damage | 30 | F | F | state with damageresist -100 on enemies in range |
| 67 | Teeth | 8 | F | F | missile fan (do 8) |
| 68 | Bone Armor | 18 | F | F | absorb state, Director HeroDefense hook |
| 69 | Skeleton Mastery | - | T | T | feeds the Raise Skeleton order (hp, damage) |
| 70 | Raise Skeleton | 31 | F | F | d2summon stats, roster limits |
| 71 | Dim Vision | 30 | S | F | before: state only; now forces monster AI state 10 (blind) |
| 72 | Weaken | 30 | F | F | state with damageresist / damage stats |
| 73 | Poison Dagger | 32 | F | F | melee + poison stream |
| 74 | Corpse Explosion | 55 | P | F | VERIFIED 0x5c2c60 / 0x5c2b90: radius (r+1)/2, life roll lo..hi-1 over the corpse life, caster/corpse level scale, calc3 element split, physical part zeroed beyond (r/2)^2 squared subtiles (skills-batch-next2.md) |
| 75 | Clay Golem | 56 | F | F | golem order, one at a time |
| 76 | Iron Maiden | 30 | F | F | reflect via afterHit |
| 77 | Terror | 30 | F | F | flee state |
| 78 | Bone Wall | 60 | P | P | wall pieces as blocking monsters, exe piece layout approximated. READ (0x5c37a0, not wired): the first wall stands at the target and two bonewallmaker missiles (do 13, ported in d2missile/gaps.go) fly perpendicular to the cast line summoning one wall per new subtile, calc2/2 walls each, linked to the first wall; the engine does not consume EventSummon yet |
| 79 | Golem Mastery | - | T | T | feeds golem orders |
| 80 | Raise Skeletal Mage | 31 | P | F | VERIFIED corpse test 0x6468b0 and summon level 0x5c2850 (batch 5); the mage is armed with the sumskill missile (necromage1) through the Director SkillFirer instead of meleeing; the caster flag source of the pet AI is U |
| 81 | Confuse | 61 | S | F | before: state only; now forces confuse (list 9, mode 3) |
| 82 | Life Tap | 30 | F | F | heal via afterHit |
| 83 | Poison Explosion | 63 | P | F | VERIFIED 0x5c3dc0 / 0x5a6d70: eight stationary poisonexplosioncloud missiles on the radius-2 square ring around the corpse (d2skill.PoisonExplosionCells), the corpse is consumed |
| 84 | Bone Spear | - | F | F | generic missile path |
| 85 | BloodGolem | 56 | P | F | VERIFIED 0x5c4600 / 0x5c4800 (batch 9): Param5 percent of every blow the golem takes is paid from the owner life instead (owner never below one point, golem reduced by the full share), Param6 percent of the healing the owner receives goes to the golem (clamped to its missing life) and is taken from the heal; wired through d2monsters.shareGolemDamage / ShareHealWithGolem and the potion hook. Event 23 (damage the golem deals) has no handler body and is not modelled |
| 86 | Attract | 59 | S | F | before: state only; now forces attract (list 9, alignment 1) |
| 87 | Decrepify | 30 | F | F | state, slow and damage stats |
| 88 | Bone Prison | 62 | P | F | VERIFIED 0x5c3c50: 12 pieces on the offset tables 0x6e45e0 / 0x6e4610 (d2skill.BonePrisonOffsets); the leader link of the pieces is not modelled |
| 89 | Summon Resist | - | T | T | VERIFIED 0x5c1f20 (wave 20): the owner stat 0x15d becomes fire / lightning / cold (only when the minion has no absorb of that element) and always poison resistance of every new skeleton, mage and golem (d2skill.SummonResistMods, d2summon.Mods.SummonResist) |
| 90 | IronGolem | 57 | M | F | VERIFIED 0x5c1070 / 0x5c3100 (batch 5): needs a ground item target (Target.GroundItem), the golem stands at the item, the item is consumed, summon level 0x5c2850; the item-derived golem stats are not simulated (U) |
| 91 | Lower Resist | 30 | F | F | state with resist stats |
| 92 | Poison Nova | 22 | F | F | nova missile ring (do 22) |
| 93 | Bone Spirit | 10 | F | F | homing missile (do 10) |
| 94 | FireGolem | 56 | P | P | golem; the fire absorb / damage aura stats come from the order only. READ (batch 9): sumskill1 Holy Fire at level min(ln56,30) is added to the golem as a skill; how a monster activates an aura skill was not found, so the burning aura is not simulated. Wave 20 re-read 0x5c2f60 / 0x5c1f20: nothing new about the aura |
| 95 | Revive | 58 | P | F | VERIFIED 0x5c35a0: life rolled between the level-scaled min and max of the corpse class at the corpse level (Director.LifeRangeAt), then the caster-level cap |

## Paladin

| id | skill | do | before | after | note |
|---|---|---|---|---|---|
| 96 | Sacrifice | 64 | F | F | self damage + strike |
| 97 | Smite | 150 | P | F | VERIFIED 0x5ccdd0: shield damage plus the Holy Shield skill's damage range at its level (StateSkiller), calc1 percent, always hits, calc2 stun; knockback bit 8 of ResultFlags rides on the hit |
| 98 | Might | 65 | F | F | friendly aura (do 65): stats re-applied every pulse |
| 99 | Prayer | 65 | F | F | friendly aura (do 65): stats re-applied every pulse |
| 100 | Resist Fire | 65 | F | F | friendly aura (do 65): stats re-applied every pulse |
| 101 | Holy Bolt | - | P | F | VERIFIED 0x5a7a40: allies (pets, mercenary, converted units) are targets of the bolt and healed calc1..calc2 (Healer); undead/demon class filter on enemies |
| 102 | Holy Fire | 66 | F | F | damage aura (do 66) |
| 103 | Thorns | 65 | F | F | friendly aura (do 65): stats re-applied every pulse |
| 104 | Defiance | 65 | F | F | friendly aura (do 65): stats re-applied every pulse |
| 105 | Resist Cold | 65 | F | F | friendly aura (do 65): stats re-applied every pulse |
| 106 | Zeal | 13 | F | F | multi hit |
| 107 | Charge | 67 | P | P | rush + strike, movement approximated; wave 22 read SRVST_031 0x5cdbb0: the rush velocity is base * (Param1 + max(stat 0x43, 50)) / 100 (d2skill.ChargeVelocity, not yet wired: the engine still moves in one step and strikes at once), the target must lack state 0x36 |
| 108 | Blessed Aim | 65 | F | F | friendly aura (do 65): stats re-applied every pulse |
| 109 | Cleansing | 65 | F | F | friendly aura (do 65): stats re-applied every pulse |
| 110 | Resist Lightning | 65 | F | F | friendly aura (do 65): stats re-applied every pulse |
| 111 | Vengeance | 2 | F | F | elemental melee |
| 112 | Blessed Hammer | 73 | P | F | VERIFIED 0x5ce5a0: path type 0xe spiral (PATH_ApplyAngleStepVelocity 0x67d050, d2missile.SpiralNodes) and the Concentration damage scale (0x647550) |
| 113 | Concentration | 65 | F | F | friendly aura (do 65): stats re-applied every pulse |
| 114 | Holy Freeze | 81 | F | F | enemy aura, cold damage + slow; wave 22: the aura applier 0x5cd240 clamps velocitypercent / attackrate to the monster cold effect (d2state.ClampAuraSlow, wired to enemy auras); the hook 0x5cece0 / callback 0x5ced30 are still unread |
| 115 | Vigor | 65 | F | F | friendly aura (do 65): stats re-applied every pulse |
| 116 | Conversion | 79 | M | P | do 79 added (roll, state, level/life scale, charm AI); VERIFIED 0x56c0a0/0x5dc1e0: unique and super unique monsters are no valid target (skills-batch3.md); act-boss exclusion U. Batch 9 re-read SRVDO_079 0x5ce8c0 + SRVDO_ConversionRollSuccess 0x5ce820: the success test is not a hireling, a monster, alignment 0, the 0x56c0a0 candidate test (the Revive corpse test, flag bits still unnamed) and a roll below calc1 on the CASTER seed; the target must also come from a landed melee hit; the base class 0x1ec with state 0x8f is refused; the revert callback 0x5ce710 restores level and life ratio; wave 22: MONSTATS_IsRevivableType 0x5dc1e0 is ported (convertibleType: walk mode, not boss, switchai); the flag bits are named |
| 117 | Holy Shield | 18 | F | F | self state (do 18) |
| 118 | Holy Shock | 66 | F | F | damage aura |
| 119 | Sanctuary | 66 | P | F | VERIFIED 0x5cd880 + filter 0x569100: aurafilter 59270 reaches hostile undead non-boss monsters only, the hit carries result flag 8 (knockback) |
| 120 | Meditation | 65 | F | F | friendly aura (do 65): stats re-applied every pulse |
| 121 | Fist of the Heavens | 80 | P | P | VERIFIED do 80 + hit 22 (0x5cebd0 / 0x5ab7f0): the delay missile marks the target, strikes it with the lightning at its end and sends calc4 holy bolts to the enemies around; the srvoverlay is left to the engine; wave 22 re-read 0x5cebd0: needs a target unit, the missile sits on it with its type and id in the data fields, the srvoverlay goes on the target (cosmetic); nothing else is open here |
| 122 | Fanaticism | 65 | F | F | friendly aura (do 65): stats re-applied every pulse |
| 123 | Conviction | 66 | F | F | enemy resist aura |
| 124 | Redemption | 82 | P | F | VERIFIED 0x5cf410 / 0x5cf2e0: Euclidean radius, per corpse roll, heal and consume; a pulse that redeemed pays (lvlmana*(lvl-1)+mana)<<manashift (0 in the shipped row) |
| 125 | Salvation | 65 | F | F | friendly aura (do 65): stats re-applied every pulse |

## Sorceress

| id | skill | do | before | after | note |
|---|---|---|---|---|---|
| 36 | Fire Bolt | - | F | F | generic missile path |
| 37 | Warmth | - | T | T |  |
| 38 | Charged Bolt | 17 | F | F | do 17 |
| 39 | Ice Bolt | - | F | F | generic missile path |
| 40 | Frozen Armor | 18 | F | F | do 18 self state with chill on melee attackers |
| 41 | Inferno | 19 | F | F | stream (do 19) |
| 42 | Static Field | 20 | F | F | do 20 |
| 43 | Telekinesis | 21 | M | P | do 21 added: damage on a targeted monster; item pickup and object use are not simulated |
| 44 | Frost Nova | 22 | F | F | do 22 |
| 45 | Ice Blast | - | F | F | generic missile path |
| 46 | Blaze | 23 | F | F |  |
| 47 | Fire Ball | - | F | F | generic missile path |
| 48 | Nova | 22 | F | F | do 22 |
| 49 | Lightning | - | F | F | generic missile path |
| 50 | Shiver Armor | 18 | F | F | do 18 self state with chill on melee attackers |
| 51 | Fire Wall | 24 | F | F |  |
| 52 | Enchant | 25 | F | F |  |
| 53 | Chain Lightning | 26 | F | F |  |
| 54 | Teleport | 27 | P | F | levels.txt Teleport flag (0 refuse, 2 no walls) added (verified 0x5c84d0) |
| 55 | Glacial Spike | - | F | F | generic missile path |
| 56 | Meteor | 28 | P | P | SRVDO_028 creates meteorcenter (hit 14: area damage + meteorfire burning ground with its own fire damage columns); bodies read |
| 57 | Thunder Storm | 29 | P | F | VERIFIED 0x5c8650 + 0x569a80: radius Param7 (circle), perdelay period, strikes rotate through the units in range by id (d2skill.NextAfter) |
| 58 | Energy Shield | 23 | P | F | VERIFIED aurafunc 24 (0x5c8840): absorb pct of damage, limited by mana*16/ratio, cost absorb*ratio/16, shield ends at 0 mana; per-type walk and the player-attacker exclusion simplified (U) |
| 59 | Blizzard | 28 | P | P | SRVDO_028 creates blizzardcenter; SrvDoFunc 10 drops blizzard1 shards (calc1 radius, calc2 period); bodies read; blizzard1 hits only its own cell (missile Size not modelled) |
| 60 | Chilling Armor | 18 | F | F | do 18 self state with chill on melee attackers |
| 61 | Fire Mastery | - | T | T |  |
| 62 | Hydra | 144 | M | P | do 144 added: 3 stationary fire shooters in the exe triangle; hydra AI timing U |
| 63 | Lightning Mastery | - | T | T |  |
| 64 | Frozen Orb | - | P | P | VERIFIED do 15 (64 entry heading table, Param2 steps), hit 29 (ring only at life 0, sHitPar1 stride) and do 16 (the nova turns 45 degrees every Param2 frames for Param1 frames), d2missile/orb.go and gaps.go; create struct fields 28/32 taken as an offset from the orb (UNVERIFIED) |
| 65 | Cold Mastery | - | T | T |  |

## Druid

| id | skill | do | before | after | note |
|---|---|---|---|---|---|
| 221 | Raven | 114 | P | F | summon level (calc2) now set through the order; stats from d2summon + MonLvl AC/AR (verified 0x5c2850) |
| 222 | Plague Poppy | 115 | M | F | VERIFIED wave 20 (w20-skills-druid.md): the Poppy casts Vine Attack (SRVDO_130 0x5d02d0 / 0x5d0220) at the sumskill level: calc1 vines (min(24, ln12)) start under the target and head the four cardinal steps on path type 10, the vine missile (SrvDoFunc 26 0x5ad4e0) drops its trail every 9 frames and the trail (hit function 50 0x5aa340) hurts for all but its last 15 frames; do 115 itself: vine monster with calc1 life percent, calc2 level. The trail takes the damage of the summoning skill (its Skill column names Plague Poppy, UNVERIFIED for Cycle of Life / Vines) |
| 223 | Wearwolf | 116 | P | F | VERIFIED 0x5c4e80 + 0x56a480: casting a form again (or the other one) only takes the shape back (group clear includes itself), free of mana while shapeshifted; the form attack skills are not rebound (restrict/State1) |
| 224 | Shape Shifting | - | T | T |  |
| 225 | Firestorm | 117 | P | F | VERIFIED 0x5c5130: calc1 firestormmaker emitters, the first straight at the target, the others from the caster through the shared hook 0x5c7340 (seed destX+index, path type 10, life clamped to 77). Wave 20: the path type 10 stepper is PATH_ApplyDirectionStepVelocity 0x67d160 (the earlier 0x5d2df0 is an unused skill callback): life/2 steps of two subtiles along the 8-way direction to the destination, each turned by -1, 0 or +1 eighth through a 32-entry table (d2missile.PathType10Nodes) |
| 226 | Oak Sage | 119 | P | F | VERIFIED (batch 5) aurafilter 0x10103 through filter 0x569100: the totem aura reaches the hero, the mercenary and the summoned minions inside aurarangecalc (minions through d2monsters.AuraBoost: damage, attack rating and life percent; thorns on minions not applied) |
| 227 | Summon Spirit Wolf | 119 | P | F | summon level (calc2) + MonLvl AC/AR now applied; resist / armor / damage stats from the order |
| 228 | Wearbear | 116 | P | F | VERIFIED like Wearwolf (0x5c4e80): toggle and free recast in the form |
| 229 | Molten Boulder | - | P | P | emerge (hit 48) -> boulder (do 6 trail, hit 47) -> moltenboulderfirepath fire; bodies read, hit 47 monster-class bit UNVERIFIED; wave 22: hit 47 class bit 0xb is the monstats2 large column, SrvDmgFunc 14 0x5ab120 is a knockback roll on the missile seed (dParam1 mode, dParam2 chance: small x2, large immune; d2missile.BoulderKnockChance) and is wired to the hit push; the footprint of Size 2 (d2missile.FootprintOverlap) is the rest |
| 230 | Arctic Blast | 19 | F | F | stream (do 19) |
| 231 | Cycle of Life | 115 | M | F | VERIFIED wave 20: the cycleoflife monster only has CorpseCycler (no Vine Attack, monstats). SRVST_063 0x5d1090 + recycler delay SrvDoFunc 29 0x5ad910: each corpse (state 0x76) feeds one cycle, 45 frames later the owner gets calc1 (dm12 over 3..12) percent of its maximum life; engine host after the AI gate of thinkCycleOfLife (gap aip1 50, idle aip3 25). UNVERIFIED: the vine does not walk to its corpse, every corpse within aurarangecalc 10 counts |
| 232 | Feral Rage | 120 | P | F | VERIFIED 0x5c57c0: count = min(calc2, count+1), aura stats evaluated with the count as level, applies on a miss too, keeps the wolf form (no group clear) |
| 233 | Maul | 120 | P | F | VERIFIED 0x5c57c0: same stacking rule as Feral Rage (damagepercent = count * par3, stunlength at the count) |
| 234 | Eruption | 28 | P | P | SRVDO_028 creates erruption center; SrvDoFunc 25 scatter (calc1/calc2) read; the crack and fire sub missiles are not read; wave 22: the crack has no server sub missiles (SrvDmgFunc 8 only adds a hit overlay; smoke is client side); what is left is the Size 3 hit footprint (d2missile.FootprintOverlap, not wired) |
| 235 | Cyclone Armor | 18 | F | F | do 18 |
| 236 | Heart of Wolverine | 119 | P | F | VERIFIED (batch 5) aurafilter 0x10103 through filter 0x569100: the totem aura reaches the hero, the mercenary and the summoned minions inside aurarangecalc (minions through d2monsters.AuraBoost: damage, attack rating and life percent; thorns on minions not applied) |
| 237 | Summon Fenris | 119 | P | F | summon level (calc2) + MonLvl AC/AR now applied |
| 238 | Rabies | 121 | P | P | VERIFIED bite + infection + plague (0x5c6b70 / 0x5c5dc0), do 30 plague and hit 53 contagion (d2missile/gaps.go); the plague only spreads between units that implement d2missile.Ownable and Stateful (the engine monsters do not yet) |
| 239 | Fire Claws | 2 | F | F | do 2 melee with fire |
| 240 | Twister | 118 | P | F | VERIFIED 0x5c52c0 / 0x5c5000: calc1 twisters, ALL through the shared per-missile hook (no straight first one, no fan); wave 20: path type 10 wired (PathType10Nodes, 0x67d160), the twister pulses are SrvDoFunc 27 |
| 241 | Vines | 115 | M | F | VERIFIED wave 20: like Cycle of Life but the VineCycler restores mana (SrvDoFunc 33 0x5ada80, dm12 over 1..8 percent of maximum mana); the vinecreature monster has VineCycler only |
| 242 | Hunger | 122 | M | F | do 122 added: melee roll, calc1 damage percent, life and mana steal (verified 0x5c5f40) |
| 243 | Shock Wave | 8 | P | F | missile fan (do 8, calc1 = 5 shockwave missiles; the fan geometry is still the 8 degree stand-in of Multiple Shot). Wave 20: the stun is the exe rule, SrvDmgFunc 7 0x5aaef0: Param1 + (level-1) * Param2 frames (40 + 15 per level) and hit class 0x60 on every hit of the shockwave missile (d2missile.StunLength); wave 22: the fan is the exe geometry (FanSpacing / FanDestinations, 0x56b0e0 and 0x56b120 read): five destinations in a row through the aim point perpendicular to the line of fire; the shockwave missile pair is one missile; nothing else is open |
| 244 | Volcano | 123 | P | P | SRVDO_123 creates volcano with a random data byte; SrvDoFunc 28 lobs debris, hit 51 spawns the fire; bodies read; wave 22 re-read SRVDO_123 0x5c60c0, SrvDoFunc 28 0x5ad6f0 and hit 51 0x5aa3b0 against the port: they match; what is left is the Size 2 / 3 hit footprints of the fire |
| 245 | Tornado | 118 | P | F | same do function as Twister (calc1 = 1); the tornado missile (SrvDoFunc 27) is modelled and wave 20 wired its path type 10 (0x67d160) |
| 246 | Spirit of Barbs | 119 | P | F | VERIFIED (batch 5) aurafilter 0x10103 through filter 0x569100: the totem aura reaches the hero, the mercenary and the summoned minions inside aurarangecalc (minions through d2monsters.AuraBoost: damage, attack rating and life percent; thorns on minions not applied) |
| 247 | Summon Grizzly | 119 | P | F | summon level (calc2) + MonLvl AC/AR now applied |
| 248 | Fury | 13 | F | F | multi hit (do 13) |
| 249 | Armageddon | 124 | P | F | VERIFIED 0x5c6200 + state active function 0x5c65b0 (batch 9): every Param4 frames the caster generator is re-seeded from the skill slot, up to five square-random points within Param3 are tried (clear line and empty cell), armageddoncontrol is created there (hit function 56 area damage + armageddonfire) |
| 250 | Hurricane | 124 | P | F | VERIFIED state active function 0x5c63f0 (batch 9): every Param4 frames an area damage around the caster of radius aurarangecalc with the skill physical and elemental roll; the PvP part of aurafilter 42883 is not modelled |

## Not in this list

Fire Blast / Fire Trauma (and the other trap bombs) are Assassin skills, handled by the Amazon/Barbarian/Assassin branch.
