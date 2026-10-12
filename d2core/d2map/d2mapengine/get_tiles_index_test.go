package d2mapengine

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
)

// The indexed GetTiles must return exactly what the linear scan returns, in the same order, for
// every key (present or not).
func TestGetTilesIndexMatchesScan(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	indexed := &MapEngine{}
	scanned := &MapEngine{}

	for f := 0; f < 4; f++ {
		batch := make([]d2dt1.Tile, 60)
		for i := range batch {
			batch[i] = d2dt1.Tile{
				Style: int32(rng.Intn(4)), Sequence: int32(rng.Intn(3)), Type: int32(rng.Intn(3)),
				RarityFrameIndex: int32(i),
			}
		}

		indexed.dt1TileData = append(indexed.dt1TileData, batch...)
		indexed.extendRank() // what addDT1 does after each file
		scanned.dt1TileData = append(scanned.dt1TileData, batch...)
	}

	if len(indexed.dt1Rank) != len(indexed.dt1TileData) || indexed.dt1Index == nil {
		t.Fatal("index not built")
	}

	for style := 0; style < 5; style++ {
		for seq := 0; seq < 4; seq++ {
			for typ := 0; typ < 4; typ++ {
				a := indexed.GetTiles(style, seq, d2enum.TileType(typ))
				b := scanned.GetTiles(style, seq, d2enum.TileType(typ))

				if !reflect.DeepEqual(a, b) {
					t.Fatalf("key %d/%d/%d: indexed %d tiles, scan %d tiles (or different order)", style, seq, typ, len(a), len(b))
				}
			}
		}
	}
}
