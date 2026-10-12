package d2player

// layoutRects is what the gold dialog really placed (frame, OK and cancel buttons, edit box anchor), for the
// OD2_AUTOUILAYOUT log. Buttons report their top left corner; the exe stands them on their bottom edge, so the
// rectangle's top is y - height (GoldDialogSized).
func (s *MoveGoldPanel) layoutRects() []UIRect {
	if s.panel == nil || s.ok == nil || s.cancel == nil {
		return nil
	}

	return []UIRect{
		spriteBox("golddlg", "frame", s.panel),
		widgetRect("golddlg", "ok", s.ok),
		widgetRect("golddlg", "cancel", s.cancel),
	}
}
