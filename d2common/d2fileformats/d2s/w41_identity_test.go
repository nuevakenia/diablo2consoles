package d2s

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"testing"
)

// TestW41ParseWriteIdentity: for every single-bit flip of a real save (checksum repaired) that still parses,
// Write(Parse(x)) should equal x: a mismatch means a field the game stores is dropped or normalised by the
// parser. Mismatches are grouped and logged.
func TestW41ParseWriteIdentity(t *testing.T) {
	tables := robustTables()
	saves := realSaves()

	if tables == nil || len(saves) == 0 {
		t.Skip("real saves or tables not available")
	}

	for si, orig := range saves {
		base, berr := Parse(orig, tables)
		if berr != nil {
			continue
		}

		padOnly := 0
		bad := map[string]int{}
		parsed := 0

		for i := HeaderSize; i < len(orig); i++ {
			for bit := 0; bit < 8; bit++ {
				d := append([]byte(nil), orig...)
				d[i] ^= 1 << bit
				d = fixup(d)

				c, err := Parse(d, tables)
				if err != nil {
					continue
				}

				parsed++

				out, err := Write(c, tables)
				if err != nil {
					bad[fmt.Sprintf("flip@0x%X.%d: write error %v", i, bit, err)]++

					continue
				}

				if !bytes.Equal(out, d) {
					// a bit that changes nothing in the parsed model is alignment padding the game writes as zero
					same := *c
					same.Header = base.Header

					if reflect.DeepEqual(&same, base) {
						padOnly++

						continue
					}

					if len(out) == len(d) {
						same := true
						for k := range out {
							if (k < 12 || k >= 16) && out[k] != d[k] {
								same = false
							}
						}
						if same {
							bad[fmt.Sprintf("flip@0x%X.%d: dropped (bit not preserved)", i, bit)]++
							continue
						}
					}
					var offs []string
					for k := range out {
						if k < len(d) && (k < 12 || k >= 16) && out[k] != d[k] && len(offs) < 4 {
							offs = append(offs, fmt.Sprintf("0x%X:%02X->%02X", k, d[k], out[k]))
						}
					}
					bad[fmt.Sprintf("flip@0x%X.%d: len %d->%d diff %v", i, bit, len(d), len(out), offs)]++
				}
			}
		}

		keys := make([]string, 0, len(bad))
		for k := range bad {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for n, k := range keys {
			if n < 60 {
				t.Logf("save %d: %s", si, k)
			}
		}

		t.Logf("save %d: %d parsed flips, %d lossy (model changed, bytes differ), %d padding-only", si, parsed, len(bad), padOnly)
	}
}
