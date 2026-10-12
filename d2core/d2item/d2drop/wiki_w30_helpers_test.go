package d2drop

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
)

func w30Names(t *testing.T) []string {
	raw, err := os.ReadFile(filepath.Join(os.Getenv("D2_TABLES"), "ItemStatCost.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for i, l := range strings.Split(string(raw), "\n") {
		if i == 0 || l == "" {
			continue
		}
		out = append(out, strings.Split(strings.TrimRight(l, "\r"), "\t")[0])
	}
	return out
}

type w30Range struct{ min, max float64 }

// w30Ranges creates the item n times and returns the min/max per stat name/param
// (item stat lists, values divided by 2^valshift... raw values).
func w30Ranges(t *testing.T, c *Creator, req Request, n int, names []string, runeword *Runeword) map[string]w30Range {
	out := map[string]w30Range{}
	for i := 0; i < n; i++ {
		r := req
		r.GameSeed = d2rand.Seed{Lo: uint32(1000003*i + 17), Hi: 0x29a}
		r.Game = &GameState{Ladder: true}
		got, err := c.Create(r)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if runeword != nil {
			got = c.ApplyRuneword(got, r, runeword)
		}
		for _, w := range got.Writes {
			if w.Kind == 'S' {
				continue
			}
			k := names[w.Stat]
			if w.Param != 0 {
				k = fmt.Sprintf("%s/%d", k, w.Param)
			}
			if w.List != 0 {
				k = fmt.Sprintf("%s@%x", k, w.List)
			}
			v := float64(w.Value) / float64(int(1)<<uint(c.Props.ValShift[w.Stat]))
			x, ok := out[k]
			if !ok {
				x = w30Range{v, v}
			}
			if v < x.min {
				x.min = v
			}
			if v > x.max {
				x.max = v
			}
			out[k] = x
		}
	}
	return out
}

func w30Print(m map[string]w30Range) string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var b strings.Builder
	for _, k := range ks {
		fmt.Fprintf(&b, " %s=%g..%g", k, m[k].min, m[k].max)
	}
	return b.String()
}

func w30Creator(t *testing.T) (*Creator, []string) {
	root := os.Getenv("D2_TABLES")
	if root == "" {
		t.Skip("D2_TABLES not set")
	}
	c, err := LoadCreator(DirTables{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	return c, w30Names(t)
}
