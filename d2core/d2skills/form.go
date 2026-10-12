package d2skills

import (
	"sort"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// Shapeshift look (w32). A state whose States.txt row has gfxtype 1 shows its owner as the monstats class gfxclass
// (hcIdx): wolf 139 -> class 430 (token 40), bear 140 -> class 431 (token TG). The engine looks at the hero's states
// every frame and swaps the drawn composite of the Player (d2mapentity.Player.SetForm); the saved hero is not touched.
// The speed is not changed by the form itself (UNIT_GetBaseMoveSpeedFixed 0x56c370 reads the class walk velocity,
// see d2-re-notes w23-skills-druid.md); the form state's own aurastat columns do the rest. The sounds are the state's
// onsound / offsound (druid_morph).

// formLook is what a state shows its owner as.
type formLook struct {
	state, token, weaponClass, onSound, offSound string
	class                                        int
}

// FormLookOf resolves the monster look of a state name (nil when the state does not disguise its owner as a monster).
func FormLookOf(recs *d2records.RecordManager, state string) *formLook {
	st := recs.States[state]
	if st == nil || !st.Disguise || st.GfxType != 1 {
		return nil
	}

	for _, ms := range recs.Monster.Stats {
		if ms.ID != st.GfxClass {
			continue
		}

		wc := "hth"
		if ex := recs.Monster.Stats2[ms.ExtraDataKey]; ex != nil && ex.BaseWeaponClass != "" {
			wc = ex.BaseWeaponClass
		}

		return &formLook{state: state, token: ms.AnimationDirectoryToken, weaponClass: wc, onSound: st.OnSound,
			offSound: st.OffSound, class: ms.ID}
	}

	return nil
}

// activeForm is the look of the form state the hero is in, nil in human shape.
func (e *Engine) activeForm(id string) *formLook {
	set := e.setOf(id)

	for _, s := range formStates {
		if set.Active(e.frame, s) {
			return FormLookOf(e.asset.Records, s)
		}
	}

	return nil
}

// syncForms makes every hero's drawn shape agree with its form states. A form does not outlast the hero's death
// (HeroDied ends the states). The skills allow the forms in town (skills.txt InTown 1) and nothing found ends them on
// a level change, so the states (and the look) stay across areas.
func (e *Engine) syncForms() {
	ids := make([]string, 0, len(e.heroes))
	for id := range e.heroes {
		ids = append(ids, id)
	}

	sort.Strings(ids)

	if e.formMode == nil {
		e.formMode = map[string]string{}
	}

	for _, id := range ids {
		h := e.heroes[id]
		if h == nil || h.p == nil || h.merc != nil {
			continue
		}

		p := h.p
		look := e.activeForm(id)

		want := ""
		if look != nil {
			want = look.token
		}

		if p.FormToken() == want {
			if want != "" { // the animation the form plays (walk, run, attack ...), one line per change
				if m := p.DrawnMode(); m != e.formMode[id] {
					e.formMode[id] = m
					e.emit("state", "FORM mode=%s composite=%s", m, want)
				}
			}

			continue
		}

		if look == nil {
			from := p.FormToken()
			p.ClearForm()
			e.emit("state", "FORM human composite=%s (was %s)", p.Class.GetToken(), from)
			e.formSound(p.Position.X(), p.Position.Y(), e.lastFormOff)

			continue
		}

		if err := p.SetForm(look.token, look.weaponClass); err != nil {
			e.Debugf("form %s: %v", look.state, err)
			e.formFailed(id, look)

			continue
		}

		e.lastFormOff = look.offSound
		e.emit("state", "FORM %s composite=%s class=%d", look.state, look.token, look.class)
		e.formSound(p.Position.X(), p.Position.Y(), look.onSound)
	}
}

// formFailed remembers a form whose composite did not load so it is not retried every frame.
func (e *Engine) formFailed(id string, look *formLook) {
	e.setOf(id).Remove(look.state)
	e.emit("state", "FORM %s has no composite %s: the state is dropped", look.state, look.token)
}

func (e *Engine) formSound(x, y float64, handle string) {
	if handle == "" {
		return
	}

	e.sound(SoundEvent{Kind: "state", Handle: handle, X: x, Y: y, Who: "form", Hero: true})
}
