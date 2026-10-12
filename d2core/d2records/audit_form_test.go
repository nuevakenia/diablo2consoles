package d2records

// formOf: the audit unit casts a form only skill (restrict 2) from inside the
// first form its State1 names, like the exe check 0x645450 demands.
func (u *auditUnit) formOf() string {
	if sk := u.reg.ByID(u.self); sk != nil && sk.Restrict == 2 {
		return sk.FormStates[0]
	}

	return ""
}

func (u *auditUnit) Shapeshifted() bool         { return u.formOf() != "" }
func (u *auditUnit) HasFormState(n string) bool { return u.formOf() == n }
