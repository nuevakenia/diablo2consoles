package d2gamescreen

import "fmt"

// commandBeltOver is "beltover": the pointer rests on an 800x600 column point (scripts, for the belt tooltip).
func (v *Game) commandBeltOver(args []string) error {
	var x, y int
	if _, err := fmt.Sscanf(args[0]+" "+args[1], "%d %d", &x, &y); err != nil {
		return err
	}

	v.Infof("BELTOVER x=%d y=%d cell=%d", x, y, v.gameControls.BeltHoverForTest(x, y))

	return nil
}

// commandSetBelt is "setbelt": wears the belt with this base code (none = no belt) and empties the potion belt (scripts).
func (v *Game) commandSetBelt(args []string) error {
	n, err := v.gameControls.SetBeltForTest(args[0])
	if err != nil {
		return err
	}

	v.Infof("SETBELT code=%s boxes=%d", args[0], n)

	return nil
}

// commandBeltInfo is "beltinfo": the belt size and state in the log (scripts).
func (v *Game) commandBeltInfo(_ []string) error {
	v.Infof("BELTINFO %s", v.gameControls.BeltBoxesNow())

	return nil
}
