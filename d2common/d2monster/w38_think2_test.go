package d2monster

import "testing"

func TestW38CurseGateBossKeepsAI(t *testing.T) {
	p := profile("Zombie")
	p.Boss = true
	b := brainAt(p)
	b.WakeShouted = true
	before := b.Def
	b.Scratch[0] = 5
	b.ApplyForced(ForcedFear, 0, 100, 1)

	if b.Def.Name != before.Name || b.Scratch[0] != 0 {
		t.Fatalf("boss: def %s scratch %v (class AI kept, scratch reset)", b.Def.Name, b.Scratch)
	}

	q := brainAt(profile("Zombie"))
	q.ApplyForced(ForcedFear, 0, 100, 1)

	if q.Def.Name == "Zombie" {
		t.Fatal("a normal class takes the fear state")
	}

	np := profile("Zombie")
	np.NoSwitchAI = true
	n := brainAt(np)
	n.ApplyForced(ForcedBlind, 0, 100, 1)

	if n.Def.Name != "Zombie" {
		t.Fatal("switchai clear keeps the class AI")
	}
}

func TestW38BatDemonThink2(t *testing.T) {
	p := profile("BatDemon")
	p.NoSwitchAI = true
	b := brainAt(p)
	b.WakeShouted = true
	b.Scratch[0] = 2
	b.ApplyForced(ForcedFear, 0, 100, 1)

	if b.Scratch[0] != 2 {
		t.Fatal("swap to think2 keeps the scratch fields")
	}

	w := newFake(5, true)
	b.Mode = ModeSkill4
	b.Wake = 0

	Tick(w, b)

	if b.Scratch[0] != 1 || b.Wake != 1 {
		t.Fatalf("S4: phase %d wake %d", b.Scratch[0], b.Wake)
	}

	// mode back to neutral, phase 1: full reset
	b.Mode, b.Wake = ModeNeutral, 0
	Tick(w, b)

	if b.Scratch[0] != 0 || b.Forced != ForcedNone || b.Wake != 1 {
		t.Fatalf("reset: %v forced %v wake %d", b.Scratch, b.Forced, b.Wake)
	}

	// phase 4 queues S3 and stays on think2
	b.ApplyForced(ForcedFear, 1, 100, 1)
	b.Scratch[0], b.Mode, b.Wake = 4, ModeNeutral, 0
	Tick(w, b)

	if b.Scratch[0] != 1 || w.last() != "attack10" {
		t.Fatalf("phase4: %v %v", b.Scratch, w.log)
	}
}

func TestW38AnchorKeepingThink2(t *testing.T) {
	p := profile("Diablo")
	p.Boss = true
	b := brainAt(p)
	b.WakeShouted = true
	b.PushCommand(Command{Type: 0, X: 40, Y: 50})
	b.ApplyForced(ForcedTaunt, 0, 100, 1)

	w := newFake(5, true)
	Tick(w, b)

	c := b.PeekCommand()
	if c == nil || c.Type != CmdAnchor || c.X != 40 || c.Y != 50 {
		t.Fatalf("anchor not kept: %+v", c)
	}
}

func TestW38SandMaggotThink2(t *testing.T) {
	p := profile("SandMaggot")
	p.Boss = true
	p.Skills[0] = SkillSlot{Name: "x"}
	b := brainAt(p)
	b.WakeShouted = true
	b.ApplyForced(ForcedFear, 0, 100, 1)
	b.PushCommand(Command{Type: cmdSandMaggotSpit, Delay: 1})

	w := newFake(5, true)
	Tick(w, b)

	if w.last() != "cast0" || b.FindCommand(cmdSandMaggotSpit).Delay != 0 || b.Wake != 30 {
		t.Fatalf("spit: %v wake %d", w.log, b.Wake)
	}

	b.Scratch[2] = 9
	b.Wake = 0
	Tick(w, b)

	if b.Scratch[2] != 9 || b.Forced != ForcedNone {
		t.Fatalf("reset keeps +0x1c: %v", b.Scratch)
	}
}

func TestW38FrogDemonThink2(t *testing.T) {
	p := profile("FrogDemon")
	p.Boss = true
	b := brainAt(p)
	b.WakeShouted = true
	b.ApplyForced(ForcedFear, 0, 100, 1)

	Tick(newFake(5, true), b)

	if b.Forced != ForcedNone || b.Wake != 1 {
		t.Fatalf("frog: forced %v wake %d", b.Forced, b.Wake)
	}
}
