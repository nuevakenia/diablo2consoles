package d2monster

// FaithfulW38 lists the handlers wave 38 compared with the exe, pinned with unit tests (w38_*_test.go) and
// that no earlier Faithful*/Reverified* list named: FallenShaman (the Skill2 roll is drawn even when the slot is
// undefined, 0x5f04d0), Mephisto (selector re-read, speed override value 0 instead of 4, 0x5f6950), Griswold
// (0x5e4a70), PantherJavelin (the regroup test is aip3 < squared distance, 0x5dfed0), Idle (0x5ae8a0) and None
// (0x5ae890, an empty think). Hireable stays out: the Go port is an approximation of the 1108-byte think.
func FaithfulW38() []string {
	return []string{"FallenShaman", "Mephisto", "Griswold", "PantherJavelin", "Idle", "None"}
}
