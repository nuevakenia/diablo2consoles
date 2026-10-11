package d2gamescreen

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2object"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// Object state across a level change: wells (regrowing objects) and used shrines.
//
// VERIFIED (0x540850 SUNITINACTIVE_RestoreUnitsForRoom, d2object.Regrow): a well comes back with its saved
// pulse count plus at most one pulse when a refill period (objects.txt Parm0) passed, and one refill event
// is scheduled for each pulse still missing. A shrine keeps its rolled type and the frame of its pending
// re-arm event; a re-arm already due is run one frame after the restore (0x540850, event 5).

type savedWell struct {
	charges int
	frame   int // game frame of the save
}

type savedShrine struct {
	shrine    d2object.Shrine
	remaining float64 // seconds left until the re-arm; <= 0 when none was pending
	pending   bool
}

// saveObjectStates fills the well and shrine maps of a level the hero leaves.
func (v *Game) saveObjectStates(sl *savedLevel) {
	frame := v.gameFrame()

	for _, e := range v.gameClient.MapEngine.Entities() {
		ob, ok := e.(*d2mapentity.Object)
		if !ok {
			continue
		}

		in := v.objects.instances[ob.ID()]
		if in == nil {
			continue
		}

		switch {
		case in.wellInit:
			if sl.wells == nil {
				sl.wells = map[savedObject]savedWell{}
			}

			sl.wells[objectKey(ob)] = savedWell{charges: in.charges, frame: frame}
		case in.shrine != nil:
			if sl.shrines == nil {
				sl.shrines = map[savedObject]savedShrine{}
			}

			sl.shrines[objectKey(ob)] = savedShrine{shrine: *in.shrine, remaining: in.rearmAt - v.objects.clock, pending: in.rearmAt > 0}
		}
	}
}

// restoreObjectStates applies the saved well and shrine state to the rebuilt map.
func (v *Game) restoreObjectStates(sl *savedLevel) (wells, shrines int) {
	frame := v.gameFrame()

	for _, e := range v.gameClient.MapEngine.Entities() {
		ob, ok := e.(*d2mapentity.Object)
		if !ok {
			continue
		}

		key := objectKey(ob)

		if w, ok := sl.wells[key]; ok && ob.Record().Parm[0] > 0 {
			rec := ob.Record()
			r := d2object.Regrow(w.charges, d2object.WellCharges(rec.Parm[2]), w.frame, frame, rec.Parm[0], rec.Parm[2])

			if r.OK {
				in := v.objectInstance(ob)
				in.wellInit, in.charges = true, r.Charges

				for _, d := range r.RefillAfter {
					in.wellRefills = append(in.wellRefills, v.objects.clock+float64(d)/d2object.FramesPerSecond)
				}

				if r.Charges > 0 {
					_, _ = ob.Close() // a well that was empty and regrew a pulse is usable again
				}

				wells++
			}
		}

		if s, ok := sl.shrines[key]; ok {
			in := v.objectInstance(ob)
			sh := s.shrine
			in.shrine = &sh

			if s.pending {
				in.rearmAt = v.objects.clock + s.remaining
				if s.remaining < 1/d2object.FramesPerSecond {
					in.rearmAt = v.objects.clock + 1/d2object.FramesPerSecond
				}
			}

			shrines++
		}
	}

	return wells, shrines
}
