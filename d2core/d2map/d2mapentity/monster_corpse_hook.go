package d2mapentity

// MarkCorpse sets the dead flag and the dead-mode bookkeeping without playing the death animation. It exists for
// the tests of corpse searches (a Monster without a composite cannot run Die); the game uses Die.
func (m *Monster) MarkCorpse(ageSeconds float64) {
	m.dead = true
	m.deadTime = ageSeconds
}
