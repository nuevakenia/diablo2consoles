package d2skill

// AuraStatsCovered is the mana test of an aura pulse for the stats of the aura (SRVDO_065 0x5cd4b0 and 066
// 0x5cd880, VERIFIED wave 23): a player's aurastat values are gathered only while the mana (24.8 fixed point) is
// at least the upkeep, (lvlmana*(lvl-1)+mana)<<manashift. Equal is enough.
func AuraStatsCovered(mana8, upkeep int) bool { return upkeep <= mana8 }

// AuraPassiveCovered is the stricter test for the passive stats that ride on the caster's aura state (SRVDO_065,
// VERIFIED): the upkeep must be STRICTLY below the mana, so an upkeep of zero still needs some mana (Fanaticism's
// damage is gone at zero mana).
func AuraPassiveCovered(mana8, upkeep int) bool { return upkeep < mana8 }
