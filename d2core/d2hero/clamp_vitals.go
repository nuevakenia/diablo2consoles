package d2hero

// ClampVitalsToMax lowers the current stamina, mana and life to their maximum, in that order, like
// STATS_ClampVitalsToMax (0x6278e0, VERIFIED order: stat 10, then 8, then 6). The game runs it whenever a
// stat list is removed or a base value falls (taking off a Life item, a buff ending), so the current values
// never exceed the new maxima. Life is never below zero here (death is decided elsewhere).
func (st *HeroStatsState) ClampVitalsToMax() {
	if st.Stamina > float64(st.MaxStamina) {
		st.Stamina = float64(st.MaxStamina)
	}

	if st.Mana > st.MaxMana {
		st.Mana = st.MaxMana
	}

	if st.Health > st.MaxHealth {
		st.Health = st.MaxHealth
	}
}

// FinalizeLoadedVitals is the vitals part of the save loader's finishing step
// (SAVE_FinalizeLoadedPlayerStats 0x568c90, VERIFIED by disassembly): after the
// items are attached the current stamina is set to the maximum stamina total (the
// saved current stamina is ignored); life and mana keep the saved values (the
// caller's two arguments), then the gold limits (see d2inventory.SanitizeLoadedGold)
// and the speed percents 0x43..0x45 = 100 apply. Call it after RecalcStats.
func (st *HeroStatsState) FinalizeLoadedVitals() {
	if st == nil {
		return
	}

	st.Stamina = float64(st.MaxStamina)
	st.ClampVitalsToMax()
}
