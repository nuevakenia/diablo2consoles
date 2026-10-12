package d2monster

// FaithfulW40 lists the handler wave 40 newly counts: Hireable (MONAI_Think_Hireable 0x5e4240), rewritten from
// the exe with the no-leader alternate states, the footprint / town / region tests and the follow kinds of
// MONAI_TeleportToLeader 0x5e2810. The host side (MercLeaderOps) is optional; without it the follow kinds fall
// back to Run/WalkTo, so the engine behaviour for the follow ring is UNVERIFIED until the Director implements it.
func FaithfulW40() []string { return []string{"Hireable"} }

// ReverifiedW40 lists handlers re-read against the exe in wave 40 (already counted before): SiegeTower,
// SiegeBeast, HighPriest (the Council), Tentacle, TentacleHead, Hydra, Imp, Totem, Vulture, Vampire, Succubus,
// SuccubusWitch, DarkWanderer, BaalThrone, BaalMinion. Fixes found: Imp returns after the offset cast, Totem uses
// UNIT_GetDistanceToUnit, DarkWanderer sets unit flag 0x20000 before dying, SuccubusWitch resets the distance when
// the attack-target scan finds nobody, the HighPriest heal scan may pick the priest itself, Vulture's prey scan is
// now hosted (NearestPrey).
func ReverifiedW40() []string {
	return []string{"SiegeTower", "SiegeBeast", "HighPriest", "Tentacle", "TentacleHead", "Hydra", "Imp", "Totem",
		"Vulture", "Vampire", "Succubus", "SuccubusWitch", "DarkWanderer", "BaalThrone", "BaalMinion"}
}
