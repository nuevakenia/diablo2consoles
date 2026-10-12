package d2playertrade

import (
	"testing"
	"time"
)

func TestCheckRequest(t *testing.T) {
	ok := RequestCtx{AskerInTown: true, TargetInTown: true, SinceAsker: time.Hour, SinceTarget: time.Hour}
	if err := CheckRequest(ok); err != nil {
		t.Fatal(err)
	}

	c := ok
	c.TargetInTown = false

	if CheckRequest(c) != ErrNotInTown {
		t.Fatal("a trade outside a town must be refused")
	}

	c = ok
	c.AskerBusy = true

	if CheckRequest(c) != ErrTradeBusy {
		t.Fatal("a busy hero must be refused")
	}

	c = ok
	c.SinceTarget = 4999 * time.Millisecond

	if CheckRequest(c) != ErrTradeSoon {
		t.Fatal("a trade under 5 s after the last one must be refused")
	}

	c.SinceTarget = 5 * time.Second

	if CheckRequest(c) != nil {
		t.Fatal("5 s after the last trade is allowed")
	}
}

func TestCanChangeEquipment(t *testing.T) {
	if !CanChangeEquipment(false, 3) || !CanChangeEquipment(true, 0) || CanChangeEquipment(true, 1) {
		t.Fatal("equipment lock wrong")
	}
}
