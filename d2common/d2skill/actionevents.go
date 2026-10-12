package d2skill

// Action events of a melee animation (wave 25, exe Game.exe 1.14b, notes w25-melee.md).
//
// In the exe a melee skill does not hit "at the end of the swing" and a multi-hit skill does not hit "every N
// frames". Every attack animation record (AnimData.d2, 144 flag bytes per record) marks some frames with a
// flag 1..4. When a player enters an attack mode the server walks the animation once, in advance, and creates one
// timed event per flagged frame (EVENT_SchedulePathStepEvents 0x551a20), plus one "mode end" event after the
// last frame. The event of a flag 1 or 2 runs the skill's do function (0x57e450, the player's skill-mode
// handler, tests the event data against 1 and 2 and calls SKILL_ServerDoCurrentSkill 0x56db20); flags 3 and 4
// create the event but do not run the do function (3 is the sound marker). The do functions of Fend, Zeal,
// Fury (SRVDO_013 0x5da910) and Strafe (SRVDO_012 0x5da6c0) hit once, count down the skill's remaining-hit
// counter and, while it is positive, call EVENT_RescheduleActionEventsByPercent (0x551b90) which throws the
// pending events away and schedules the next ones from an earlier point of the animation. That reschedule is
// the whole burst cadence. This file is a pure port of both schedulers.

// Flags of an action frame (AnimData flag byte, monseq event column for sequences).
const (
	// ActionFlagNone marks a frame without event.
	ActionFlagNone = 0
	// ActionFlagHit (1) and ActionFlagMissile (2) run the do function of the used skill.
	ActionFlagHit     = 1
	ActionFlagMissile = 2
	// ActionFlagSound (3) is the "play the swing sound" marker (CLTDO_Jab plays dosound at 3); the server creates
	// the event but its handler does not call the do function.
	ActionFlagSound = 3
	// ActionFlagSkill (4) is created like the others and ignored by the do handler (UNVERIFIED: what else uses it).
	ActionFlagSkill = 4
)

// maxActionFrame is the last frame index the initial scheduler reads (0x8f, the record holds 144 flags).
const maxActionFrame = 0x8f

// RunsDo reports whether an event with this flag runs the skill's do function (VERIFIED, 0x57e450 compares the
// event data with 1 and 2).
func RunsDo(flag int) bool { return flag == ActionFlagHit || flag == ActionFlagMissile }

// ActionEvent is one scheduled animation event.
type ActionEvent struct {
	// Tick is the game frame of the event, counted from the moment of scheduling (>= 1).
	Tick int
	// Flag is the AnimData flag byte (1..4).
	Flag int
}

// flagAt reads the flag byte of a frame; frames outside the table read 0 (the exe reads the memory around the
// record for frame -1 after a full restart, UNVERIFIED value; 0 is what the record header gives for the
// animation records, which have a zero byte before the flags).
func flagAt(flags []byte, f int) int {
	if f < 0 || f >= len(flags) {
		return 0
	}

	return int(flags[f])
}

// ScheduleActionEvents ports EVENT_SchedulePathStepEvents (0x551a20, VERIFIED): the animation starts at
// startFrame (UNIT_GetAttackFrameCount, see AttackStartFrame) and advances by rate (8.8 frames per game frame)
// against total (8.8, frames per direction << 8). Each game frame the frames crossed since the last scan are
// read; a flagged frame creates an event at that game frame. The result lists the events and the game frame of
// the mode end event. A rate of 0 ends the mode after one frame.
func ScheduleActionEvents(flags []byte, startFrame, rate, total int) (events []ActionEvent, endTick int) {
	if rate <= 0 {
		return nil, 1
	}

	pos := startFrame << 8
	next := startFrame // first frame not read yet
	tick := 0

	for {
		pos += rate
		if pos >= total {
			if tick == 0 {
				tick = 1
			}

			return events, tick + 1
		}

		tick++

		f := pos >> 8
		if f < next {
			continue
		}

		for next <= f && next <= maxActionFrame {
			switch fl := flagAt(flags, next); fl {
			case ActionFlagHit, ActionFlagMissile, ActionFlagSkill, ActionFlagSound:
				events = append(events, ActionEvent{Tick: tick, Flag: fl})
			}

			next++
		}
	}
}

// RescheduleActionEvents ports EVENT_RescheduleActionEventsByPercent (0x551b90, VERIFIED). elapsed is the number
// of game frames since the unit's animation marker (the exe keeps it in the unit: game frame minus the frame
// stored at +0x44 by the last scheduler; it is set to the game frame of the mode start), pct the percent of the
// elapsed time that is forgiven (Fend, Zeal, Fury: Param2 of the skill; Strafe: Param6). The animation then
// restarts from frame elapsed*(100-pct)/100 (C division, the exe mixes game frames and animation frames here:
// it is exact at rate 256) and the events from that frame on are scheduled again, counted from now. The new
// marker is now minus the restart frame, returned as newElapsed (the elapsed time at the moment of the call
// after rescheduling).
func RescheduleActionEvents(flags []byte, elapsed, pct, rate, total int) (newElapsed int, events []ActionEvent, endTick int) {
	start := elapsed * (100 - pct) / 100
	newElapsed = start

	if rate <= 0 {
		return newElapsed, nil, 1
	}

	pos := start << 8
	next := start - 1
	tick := 0

	for pos+rate < total {
		cur := pos + rate
		f := cur >> 8
		tick++

		for ; next <= f; next++ {
			if fl := flagAt(flags, next); fl > 0 && fl < 5 {
				events = append(events, ActionEvent{Tick: tick, Flag: fl})
			}
		}

		pos = cur
	}

	if tick == 0 {
		tick = 1
	}

	return newElapsed, events, tick + 1
}

// ActionClock follows the action events of one running attack animation across reschedules. Time is counted in
// game frames from the start of the mode (the first call to NewActionClock is frame 0).
type ActionClock struct {
	flags   []byte
	rate    int
	total   int
	now     int           // game frame of the last event run
	marker  int           // game frame the animation counts as started (exe: unit +0x44 >> 8)
	pending []ActionEvent // absolute ticks
	end     int           // absolute tick of the mode end event
}

// NewActionClock schedules the events of a fresh attack animation. startFrame is AttackStartFrame, rate the
// animation rate (d2animspeed.AttackRate), frames the frames per direction of the AnimData record.
func NewActionClock(flags []byte, startFrame, rate, frames int) *ActionClock {
	c := &ActionClock{flags: flags, rate: rate, total: frames << 8}

	ev, end := ScheduleActionEvents(flags, startFrame, rate, c.total)
	c.pending, c.end = ev, end

	return c
}

// NextDo is the absolute game frame of the first pending do event (flag 1 or 2); ok is false when the mode ends
// without another one.
func (c *ActionClock) NextDo() (tick int, ok bool) {
	for _, e := range c.pending {
		if RunsDo(e.Flag) {
			return e.Tick, true
		}
	}

	return 0, false
}

// DoEvents lists the absolute game frames of every pending do event.
func (c *ActionClock) DoEvents() []int {
	var out []int

	for _, e := range c.pending {
		if RunsDo(e.Flag) {
			out = append(out, e.Tick)
		}
	}

	return out
}

// End is the absolute game frame of the mode end event.
func (c *ActionClock) End() int { return c.end }

// Now is the game frame of the last event run.
func (c *ActionClock) Now() int { return c.now }

// RunNextDo runs the next do event: time moves to it and the events up to it are consumed. It returns its game
// frame, false when none is left.
func (c *ActionClock) RunNextDo() (tick int, ok bool) {
	for i, e := range c.pending {
		if RunsDo(e.Flag) {
			c.now = e.Tick
			c.pending = c.pending[i+1:]

			return e.Tick, true
		}
	}

	return 0, false
}

// Reschedule is what a burst do function calls after its hit: the pending events are dropped and
// RescheduleActionEvents lays out the new ones from the current frame.
func (c *ActionClock) Reschedule(pct int) {
	elapsed := c.now - c.marker
	if elapsed < 0 {
		elapsed = 0
	}

	ne, ev, end := RescheduleActionEvents(c.flags, elapsed, pct, c.rate, c.total)
	c.marker = c.now - ne

	c.pending = c.pending[:0]
	for _, e := range ev {
		c.pending = append(c.pending, ActionEvent{Tick: c.now + e.Tick, Flag: e.Flag})
	}

	c.end = c.now + end
}

// BurstGaps runs a whole burst on the clock: the first do event is the cast's own hit, then each of the n-1
// further hits waits for the next do event after a reschedule with pct. It returns the game frame of every hit
// (the first included) and stops early when the mode ends before a do event is pending.
func BurstGaps(c *ActionClock, hits, pct int) []int {
	var out []int

	for i := 0; i < hits; i++ {
		t, ok := c.RunNextDo()
		if !ok {
			break
		}

		out = append(out, t)

		if i+1 < hits {
			c.Reschedule(pct)
		}
	}

	return out
}

// AttackStartFrame is UNIT_GetAttackFrameCount (0x623d00, VERIFIED): the frame a player's attack animation
// (modes A1, A2, and S3 / S4 of a dual wielder) starts at, from a table of dwords at 0x6ea410 indexed by
// hero class + 7 * weapon class id. Only the Amazon and the Sorceress have non-zero entries: hand to hand 1,
// the five melee classes 1hs 1ht stf 2hs 2ht 2, bow and crossbow 0. classIdx is the hero class (0 Amazon ..
// 6 Assassin), wclass the COF weapon class code ("hth", "1hs", ...).
func AttackStartFrame(classIdx int, wclass string) int {
	if classIdx != 0 && classIdx != 1 {
		return 0
	}

	switch wclass {
	case "hth", "":
		return 1
	case "1hs", "1ht", "stf", "2hs", "2ht":
		return 2
	}

	return 0
}

// SeqRows describes one player sequence (monseq style table in the exe at 0x742a38.., 6 byte rows; the row
// index is the animation frame of the sequence): its length and the rows whose event is a do event.
type SeqRows struct {
	Rows   int
	DoRows []int
	Sound  []int // rows with the sound flag
}

// Sequence numbers of the skills.txt seqnum column that are decoded here.
const (
	SeqJab    = 1
	SeqImpale = 8
)

// seqTables are the player sequences of Jab and Impale per weapon class (VERIFIED by reading the exe tables:
// DAT 0x744580 points to one table per sequence, each with one 12 byte entry {rows ptr, rows, rows} per weapon
// class entry; the entry order is hth 0, 1ht 1, 2ht 2, 1hs 3, 2hs 4, bow 5, xbw 6, stf 7, the others empty).
// Jab uses hand to hand, one hand thrust and two hand thrust only; a weapon class without an entry cannot play
// the sequence (the exe raises a fatal error when the lookup finds none).
var seqTables = map[int]map[string]SeqRows{
	SeqJab: {
		"hth": {Rows: 13, DoRows: []int{8}},
		"1ht": {Rows: 18, DoRows: []int{3, 9, 15}, Sound: []int{1, 7, 13}},
		"2ht": {Rows: 21, DoRows: []int{3, 10, 17}, Sound: []int{1, 8, 15}},
	},
	SeqImpale: {
		"hth": {Rows: 13, DoRows: []int{8}},
		"1ht": {Rows: 21, DoRows: []int{13}},
		"2ht": {Rows: 24, DoRows: []int{15}},
	},
}

// Sequence returns the rows of a player sequence for a weapon class.
func Sequence(seq int, wclass string) (SeqRows, bool) {
	r, ok := seqTables[seq][wclass]

	return r, ok
}

// Flags expands the rows into a flag array for the schedulers.
func (s SeqRows) Flags() []byte {
	fl := make([]byte, s.Rows)

	for _, r := range s.DoRows {
		fl[r] = ActionFlagHit
	}

	for _, r := range s.Sound {
		fl[r] = ActionFlagSound
	}

	return fl
}
