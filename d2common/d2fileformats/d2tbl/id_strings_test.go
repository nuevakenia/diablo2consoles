package d2tbl

import (
	"encoding/binary"
	"testing"
)

type tblNode struct {
	id        uint16
	key, text string
}

// buildTbl hand-assembles a tiny table; slots without a node stay inactive.
func buildTbl(slots int, nodes map[int]tblNode) []byte {
	hdr := make([]byte, 21)
	binary.LittleEndian.PutUint16(hdr[2:], uint16(len(nodes)))
	binary.LittleEndian.PutUint32(hdr[4:], uint32(slots))

	strStart := 21 + 2*len(nodes) + 17*slots
	body := []byte{}
	nodeBytes := make([]byte, 17*slots)

	for slot := 0; slot < slots; slot++ {
		n, ok := nodes[slot]
		if !ok {
			continue
		}

		keyPos := strStart + len(body)
		body = append(body, []byte(n.key)...)
		body = append(body, 0)
		valPos := strStart + len(body)
		body = append(body, []byte(n.text)...)
		body = append(body, 0)

		b := nodeBytes[17*slot:]
		b[0] = 1
		binary.LittleEndian.PutUint16(b[1:], n.id)
		binary.LittleEndian.PutUint32(b[7:], uint32(keyPos))
		binary.LittleEndian.PutUint32(b[11:], uint32(valPos))
		binary.LittleEndian.PutUint16(b[15:], uint16(len(n.text)+1))
	}

	out := append(hdr, make([]byte, 2*len(nodes))...)
	out = append(out, nodeBytes...)

	return append(out, body...)
}

func TestLoadIDStrings(t *testing.T) {
	data := buildTbl(5, map[int]tblNode{
		1: {0x1055, "automapgame", "Game: "},
		3: {0x1422, "x", "Hell"},
	})

	got, err := LoadIDStrings(data, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 || got[0x1055] != "Game: " || got[0x1422] != "Hell" {
		t.Fatalf("got %v", got)
	}
}

func TestLoadIDStringsTruncated(t *testing.T) {
	if _, err := LoadIDStrings([]byte{1, 2, 3}, nil); err == nil {
		t.Fatal("expected an error")
	}
}
