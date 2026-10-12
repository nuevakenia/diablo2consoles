package d2skills

// CureUnit is the cure part of a healing NPC's interaction (d2state.Set.Cure): poison, freeze and the states
// curable says so lose their effect. It returns how many states it removed.
func (e *Engine) CureUnit(unitID string, curable func(name string) bool) int {
	st := e.sets[unitID]
	if st == nil {
		return 0
	}

	return len(st.Cure(e.frame, curable))
}
