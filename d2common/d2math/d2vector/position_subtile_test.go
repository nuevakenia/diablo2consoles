package d2vector

import (
	"math/rand"
	"testing"
)

// SubTileXY must give bit-identical results to SubTileOffset (it only avoids the allocations).
func TestSubTileXYMatchesSubTileOffset(t *testing.T) {
	rng := rand.New(rand.NewSource(7))

	check := func(x, y float64) {
		p := NewPosition(x, y)
		want := p.SubTileOffset()
		gx, gy := p.SubTileXY()

		if gx != want.X() || gy != want.Y() {
			t.Fatalf("(%v,%v): got (%v,%v) want (%v,%v)", x, y, gx, gy, want.X(), want.Y())
		}
	}

	for _, v := range []float64{0, 4.9999999999, 5, -0.0000001, -5, 1e-17, 123456.789} {
		check(v, v+0.3)
	}

	for i := 0; i < 200000; i++ {
		check((rng.Float64()-0.2)*4000, (rng.Float64()-0.2)*4000)
	}
}

func BenchmarkSubTileOffset(b *testing.B) {
	p := NewPosition(123.4, 567.8)
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = p.SubTileOffset().X()
	}
}

func BenchmarkSubTileXY(b *testing.B) {
	p := NewPosition(123.4, 567.8)
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = p.SubTileXY()
	}
}
