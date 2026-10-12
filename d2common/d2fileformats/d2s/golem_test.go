package d2s

import (
	"bytes"
	"errors"
	"testing"
)

// The 'kf' golem section follows the exe (writer 0x567710, reader 0x568b60): present for every class of an
// expansion save, flag byte 0 or 1, then one item. All saves here are synthetic.

func TestGolemSectionEveryExpansionClass(t *testing.T) {
	for _, ts := range matrixTableSets(t) {
		tb := ts.tb

		for class := Amazon; class <= Assassin; class++ {
			c := matrixBase(t, tb)
			c.Header.Class = class
			c.NoGolemSection = false

			out := roundTripExact(t, ts.name, c, tb)
			if !bytes.HasSuffix(out, []byte{'k', 'f', 0}) {
				t.Fatalf("%s %v: expansion save must end with the empty golem section", ts.name, class)
			}

			// hardcore, dead, ladder flags change nothing about the tail
			c.Header.Status |= StatusHardcore | StatusDied | StatusLadder
			out = roundTripExact(t, ts.name+"/flags", c, tb)

			if !bytes.HasSuffix(out, []byte{'k', 'f', 0}) {
				t.Fatalf("%s %v: flags changed the tail", ts.name, class)
			}
		}
	}
}

func TestGolemSectionItemRoundTrip(t *testing.T) {
	tb := synthTables(t)
	_, _, _, golem := matrixItems(t, tb)

	c := matrixBase(t, tb)
	c.Header.Class = Necromancer
	c.NoGolemSection = false
	c.Golem = &golem

	out := roundTripExact(t, "golem", c, tb)

	back, err := Parse(out, tb)
	if err != nil || back.Golem == nil || !itemsEqual(golem, *back.Golem) || len(back.Trailing) != 0 {
		t.Fatalf("golem item lost: %v %+v", err, back.Golem)
	}
}

func TestGolemSectionEdgeCases(t *testing.T) {
	tb := synthTables(t)
	_, _, _, golem := matrixItems(t, tb)

	c := matrixBase(t, tb)
	c.Header.Class = Necromancer
	c.NoGolemSection = false
	c.Golem = &golem

	good, err := Write(c, tb)
	if err != nil {
		t.Fatal(err)
	}

	// classic saves cannot hold a golem; Write refuses rather than drop it
	classic := matrixBase(t, tb)
	classic.Header.Status = 0
	classic.Golem = &golem

	if _, err = Write(classic, tb); !errors.Is(err, ErrWouldDrop) {
		t.Fatalf("classic golem: %v", err)
	}

	// a classic save has no golem section at all
	classic.Golem = nil

	out, err := Write(classic, tb)
	if err != nil || bytes.HasSuffix(out, []byte{'k', 'f', 0}) {
		t.Fatalf("classic tail: %v", err)
	}

	// a wrong tag where the golem section belongs is rejected
	i := bytes.LastIndex(good, golemTag)
	badTag := append([]byte(nil), good...)
	badTag[i] = 'x'

	if _, err = Parse(fixup(badTag), tb); err == nil {
		t.Fatal("bad golem tag accepted")
	}

	// flag set but the item is cut off, or the flag byte itself is missing
	for _, n := range []int{i + 2, i + 3, i + 4, len(good) - 1} {
		cut := fixup(append([]byte(nil), good[:n]...))
		if _, err = Parse(cut, tb); err == nil {
			t.Fatalf("golem truncated at %d accepted", n)
		}
	}

	// a save from an older build without the section parses and is reproduced as is
	legacy := fixup(append([]byte(nil), good[:i]...))

	lc, err := Parse(legacy, tb)
	if err != nil || !lc.NoGolemSection || lc.Golem != nil {
		t.Fatalf("legacy tail: %v", err)
	}

	again, err := Write(lc, tb)
	if err != nil || !bytes.Equal(again, legacy) {
		t.Fatalf("legacy save not reproduced: %v", err)
	}
}
