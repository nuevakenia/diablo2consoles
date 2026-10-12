package d2mapentity

import (
	"fmt"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
)

// Shapeshift look of the hero (Druid Werewolf / Werebear). In the exe a state with gfxtype 1 (States.txt) shows the
// unit as the monster class named by gfxclass (a monstats hcIdx), UNIT_GetShapeshiftAdjustedMode 0x646620 maps the
// requested animation mode onto the monster's modes. The mode table itself (0x6ec8b8) was not read, so FormMode
// is UNVERIFIED: it keeps the codes the monster has (NU WL RN GH A1 A2 BL SC S1 S3 S4 DT DD) and folds the player only
// ones (town modes, throw, kick, S2) onto the nearest monster mode. The saved hero is never touched: the form is a
// second composite that replaces the drawn one while the state lasts.

// FormMode maps a hero animation mode onto the mode a monster composite plays in a form (UNVERIFIED mapping).
func FormMode(m d2enum.PlayerAnimationMode) d2enum.PlayerAnimationMode {
	switch m {
	case d2enum.PlayerAnimationModeTownNeutral:
		return d2enum.PlayerAnimationModeNeutral
	case d2enum.PlayerAnimationModeTownWalk:
		return d2enum.PlayerAnimationModeWalk
	case d2enum.PlayerAnimationModeThrow, d2enum.PlayerAnimationModeKick:
		return d2enum.PlayerAnimationModeAttack1
	case d2enum.PlayerAnimationModeSkill2:
		return d2enum.PlayerAnimationModeAttack2
	case d2enum.PlayerAnimationModeSequence, d2enum.PlayerAnimationModeKnockBack:
		return d2enum.PlayerAnimationModeGetHit
	}

	return m
}

// FormToken is the monster animation token of the form the hero is in ("" in human shape).
func (p *Player) FormToken() string { return p.formToken }

// DrawnMode is the animation mode code the drawn composite plays now (NU, WL, RN, A1 ...).
func (p *Player) DrawnMode() string { return p.composite.GetAnimationMode() }

// InForm is true while the hero is drawn as a monster.
func (p *Player) InForm() bool { return p.formToken != "" }

// modeFor is the animation mode that is asked from the drawn composite.
func (p *Player) modeFor(m d2enum.PlayerAnimationMode) d2enum.PlayerAnimationMode {
	if p.formToken != "" {
		return FormMode(m)
	}

	return m
}

// SetForm draws the hero as the monster composite of token (weaponClass is the monster's base weapon class,
// "hth" for wolf and bear). The hero's equipment layers are not drawn in a form. An error leaves the hero as it was.
func (p *Player) SetForm(token, weaponClass string) error {
	if token == "" {
		p.ClearForm()
		return nil
	}

	if p.formToken == token {
		return nil
	}

	human := p.composite
	if p.formHuman != nil {
		human = p.formHuman
	}

	c, err := human.AssetManager.LoadComposite(d2enum.ObjectTypeCharacter, token, d2resource.PaletteUnits)
	if err != nil {
		return fmt.Errorf("form composite %q: %w", token, err)
	}

	dir := p.composite.GetDirection()

	if err := c.SetMode(FormMode(p.GetAnimationMode()), weaponClass); err != nil {
		return fmt.Errorf("form composite %q: %w", token, err)
	}

	c.SetDirection(dir)

	c.PrefetchModes(weaponClass, d2enum.PlayerAnimationModeWalk, d2enum.PlayerAnimationModeRun, d2enum.PlayerAnimationModeNeutral,
		d2enum.PlayerAnimationModeAttack1, d2enum.PlayerAnimationModeGetHit, d2enum.PlayerAnimationModeBlock,
		d2enum.PlayerAnimationModeDeath)

	p.formHuman, p.composite, p.formToken = human, c, token
	p.refreshMode()

	return nil
}

// ClearForm takes the human shape back (the form state ended or the hero died).
func (p *Player) ClearForm() {
	if p.formToken == "" || p.formHuman == nil {
		return
	}

	dir := p.composite.GetDirection()
	p.composite, p.formHuman, p.formToken = p.formHuman, nil, ""
	p.composite.SetDirection(dir)
	p.refreshMode()
	p.applyHumanEquipment()
}

// refreshMode puts the (new) drawn composite into the mode the hero is in, keeping a death pose.
func (p *Player) refreshMode() {
	switch p.deathPhase {
	case deathPhaseDying:
		p.setDeathMode(d2enum.PlayerAnimationModeDeath)
	case deathPhaseDead:
		p.setDeathMode(d2enum.PlayerAnimationModeDead)
	default:
		if err := p.SetAnimationMode(p.GetAnimationMode()); err != nil {
			fmt.Printf("player %s: form mode: %v\n", p.name, err)
		}
	}
}

// applyHumanEquipment redraws the equipment layers after the form ended (the gear may have changed meanwhile).
func (p *Player) applyHumanEquipment() {
	if p.Equipment == nil {
		return
	}

	if err := p.composite.SetMode(p.GetAnimationMode(), p.Equipment.RightHand.GetWeaponClass()); err != nil {
		fmt.Printf("player %s: weapon class change: %v\n", p.name, err)
	}

	if err := p.composite.Equip(equipmentLayers(p.Equipment)); err != nil {
		fmt.Printf("player %s: equip: %v\n", p.name, err)
	}

	p.composite.SetDirection(p.composite.GetDirection())
}
