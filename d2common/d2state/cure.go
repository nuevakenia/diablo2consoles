package d2state

// Cure is what a healing NPC does to the unit it heals (NPCSRV_RestoreVitalsOnTalk 0x576c80): it removes the
// poison and the freeze state (state ids 2 and 1), then every active state for which curable says so (the
// exe tests STATE_IsStateInMaskFC, VERIFIED to be the states.txt "curable" column: DATATBL_LoadStatesTable 0x617c50
// builds one mask per flag bit and the one at +0xfc is bit 12, which is "curable").
// Poison damage streams end with the poison state. It returns the names of what it removed.
func (s *Set) Cure(frame int, curable func(name string) bool) []string {
	var removed []string

	for _, n := range s.Names(frame) {
		if n == "poison" || n == "freeze" || (curable != nil && curable(n)) {
			s.Remove(n)

			removed = append(removed, n)
		}
	}

	kept := s.streams[:0]
	cleared := false

	for _, st := range s.streams {
		if st.Kind == "poison" && st.Until > frame {
			cleared = true

			continue
		}

		kept = append(kept, st)
	}

	s.streams = kept

	if cleared {
		delete(s.frac, "poison")

		if len(removed) == 0 {
			removed = append(removed, "poison")
		}
	}

	return removed
}
