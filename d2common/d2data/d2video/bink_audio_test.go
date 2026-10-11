package d2video

import (
	"encoding/binary"
	"math"
	"math/rand"
	"testing"
)

// bitWriter writes the LSB first bitstream the decoder reads.
type bitWriter struct {
	buf  []byte
	nbit int
}

func (w *bitWriter) put(v uint32, n int) {
	for i := 0; i < n; i++ {
		if w.nbit&7 == 0 {
			w.buf = append(w.buf, 0)
		}

		if v>>uint(i)&1 != 0 {
			w.buf[len(w.buf)-1] |= 1 << uint(w.nbit&7)
		}

		w.nbit++
	}
}

func (w *bitWriter) align32() {
	for w.nbit&31 != 0 {
		w.put(0, 1)
	}
}

// putFloat writes the Bink float: 5 bit exponent, 23 bit mantissa, sign (value = mant * 2^(power-23)).
func (w *bitWriter) putFloat(mant uint32, power int, neg bool) {
	w.put(uint32(power), 5)
	w.put(mant, 23)

	if neg {
		w.put(1, 1)
	} else {
		w.put(0, 1)
	}
}

// synthBlock describes the coefficients of one transform channel of a test block.
type synthBlock struct {
	dc0, dc1 int   // the two header floats (integers, exponent 23)
	quant    int   // quantiser index used for all bands
	coef     []int // signed coefficient values from index 2 on (missing = 0)
	width    int   // bit width of the groups that have any coefficient
}

// encodeBlock writes one block for decoder d (header, quantisers, coefficient groups of 8, alignment).
func encodeBlock(w *bitWriter, d *BinkAudioDecoder, chans []synthBlock) {
	if d.dct {
		w.put(0, 2)
	}

	for _, c := range chans {
		w.putFloat(uint32(absInt(c.dc0)), 23, c.dc0 < 0)
		w.putFloat(uint32(absInt(c.dc1)), 23, c.dc1 < 0)

		for i := 0; i < d.numBands; i++ {
			w.put(uint32(c.quant), 8)
		}

		for i := 2; i < d.frameLen; i += 8 {
			any := false

			for j := i; j < i+8; j++ {
				if j-2 < len(c.coef) && c.coef[j-2] != 0 {
					any = true
				}
			}

			w.put(0, 1) // a group of 8

			if !any {
				w.put(0, 4)
				continue
			}

			w.put(uint32(c.width), 4)

			for j := i; j < i+8; j++ {
				v := 0
				if j-2 < len(c.coef) {
					v = c.coef[j-2]
				}

				w.put(uint32(absInt(v)), c.width)

				if v != 0 {
					w.put(b2u(v < 0), 1)
				}
			}
		}
	}

	w.align32()
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}

	return v
}

func b2u(b bool) uint32 {
	if b {
		return 1
	}

	return 0
}

// packet wraps blocks the way a frame stores them: 32 bit decoded size, then the bit data.
func audioPacket(blocksBytes []byte, decodedBytes int) []byte {
	p := make([]byte, 4, 4+len(blocksBytes))
	binary.LittleEndian.PutUint32(p, uint32(decodedBytes))

	return append(p, blocksBytes...)
}

func goertzel(x []float64, freq float64) float64 {
	w := 2 * math.Pi * freq
	c := 2 * math.Cos(w)

	var s1, s2 float64

	for _, v := range x {
		s0 := v + c*s1 - s2
		s2 = s1
		s1 = s0
	}

	return math.Sqrt(s1*s1 + s2*s2 - c*s1*s2)
}

func TestBinkAudioGeometry(t *testing.T) {
	tests := []struct {
		rate, ch   int
		dct        bool
		wantFrames int // sample frames per block
		wantFrame  int // transform length
	}{
		{44100, 2, false, 1920, 4096},
		{44100, 2, true, 1920, 2048},
		{44100, 1, false, 1920, 2048},
		{22050, 1, false, 960, 1024},
		{11025, 2, true, 480, 512},
	}

	for _, tt := range tests {
		d, err := NewBinkAudioDecoder(tt.rate, tt.ch, tt.dct, false)
		if err != nil {
			t.Fatal(err)
		}

		if d.SamplesPerBlock() != tt.wantFrames || d.frameLen != tt.wantFrame {
			t.Errorf("%+v: block %d frameLen %d", tt, d.SamplesPerBlock(), d.frameLen)
		}

		if d.bands[0] != 2 || d.bands[d.numBands] != d.frameLen {
			t.Errorf("%+v: band edges %v", tt, d.bands[:d.numBands+1])
		}

		for i := 1; i <= d.numBands; i++ {
			if d.bands[i] < d.bands[i-1] {
				t.Errorf("%+v: bands not monotonic: %v", tt, d.bands[:d.numBands+1])
			}
		}
	}

	if _, err := NewBinkAudioDecoder(44100, 3, false, false); err == nil {
		t.Error("3 channels accepted")
	}

	if _, err := NewBinkAudioDecoder(5, 2, false, false); err == nil {
		t.Error("absurd sample rate accepted")
	}
}

func TestBinkAudioSilence(t *testing.T) {
	for _, dct := range []bool{false, true} {
		d, _ := NewBinkAudioDecoder(44100, 2, dct, false)
		w := &bitWriter{}
		chans := 1

		if dct {
			chans = 2
		}

		for b := 0; b < 3; b++ {
			encodeBlock(w, d, make([]synthBlock, chans))
		}

		pcm, err := d.Decode(nil, audioPacket(w.buf, 3*3840*2))
		if err != nil {
			t.Fatal(err)
		}

		if len(pcm) != 3*3840 {
			t.Fatalf("dct=%v: %d samples, want %d", dct, len(pcm), 3*3840)
		}

		for i, v := range pcm {
			if v != 0 {
				t.Fatalf("dct=%v: sample %d = %d, want silence", dct, i, v)
			}
		}
	}
}

// A single RDFT coefficient pair (Re, Im of bin k) must come out as a sinusoid of k cycles per frame on the
// interleaved signal, i.e. k/4096 cycles per interleaved sample, with the same signal on the left/right pattern.
func TestBinkAudioRDFTSinusoid(t *testing.T) {
	d, _ := NewBinkAudioDecoder(44100, 2, false, false)
	w := &bitWriter{}

	const bin = 100

	coef := make([]int, 4096)
	coef[2*bin-2] = 15 // Re X[bin]

	for b := 0; b < 2; b++ {
		encodeBlock(w, d, []synthBlock{{quant: 60, coef: coef, width: 4}})
	}

	pcm, err := d.Decode(nil, audioPacket(w.buf, 0))
	if err != nil {
		t.Fatal(err)
	}

	x := make([]float64, 3840)
	for i := range x {
		x[i] = float64(pcm[3840+i])
	}

	f := float64(bin) / 4096
	on, off := goertzel(x, f), goertzel(x, f*1.5)

	if on < 100*off || on < 1000 {
		t.Fatalf("expected a tone at bin %d: on=%g off=%g", bin, on, off)
	}
}

func TestBinkAudioDCTSinusoid(t *testing.T) {
	d, _ := NewBinkAudioDecoder(44100, 2, true, false)
	w := &bitWriter{}

	const k = 64

	coef := make([]int, 2048)
	coef[k-2] = 15

	for b := 0; b < 2; b++ {
		encodeBlock(w, d, []synthBlock{{quant: 60, coef: coef, width: 4}, {quant: 60}})
	}

	pcm, err := d.Decode(nil, audioPacket(w.buf, 0))
	if err != nil {
		t.Fatal(err)
	}

	left, right := make([]float64, 1920), make([]float64, 1920)
	for i := range left {
		left[i] = float64(pcm[3840+2*i])
		right[i] = float64(pcm[3840+2*i+1])
	}

	// DCT-III basis k: cos(pi k (2n+1) / (2N)), k/(2N) cycles per sample
	f := float64(k) / 4096
	if on, off := goertzel(left, f), goertzel(left, f*1.5); on < 100*off || on < 1000 {
		t.Fatalf("left: on=%g off=%g", on, off)
	}

	for _, v := range right {
		if v != 0 {
			t.Fatal("right channel should be silent")
		}
	}
}

func TestBinkAudioOverlapContinuity(t *testing.T) {
	// two identical blocks of a steady tone must splice without a jump at the block seam
	d, _ := NewBinkAudioDecoder(44100, 2, true, false)
	w := &bitWriter{}
	coef := make([]int, 2048)
	coef[30] = 12

	for b := 0; b < 4; b++ {
		encodeBlock(w, d, []synthBlock{{quant: 30, coef: coef, width: 4}, {quant: 30, coef: coef, width: 4}})
	}

	pcm, _ := d.Decode(nil, audioPacket(w.buf, 0))
	if len(pcm) != 4*3840 {
		t.Fatalf("%d samples", len(pcm))
	}

	maxStep := 0.0

	for i := 4; i < len(pcm); i += 2 {
		maxStep = math.Max(maxStep, math.Abs(float64(pcm[i])-float64(pcm[i-2])))
	}

	if maxStep > 6000 {
		t.Fatalf("discontinuity of %g in the output", maxStep)
	}
}

func TestBinkAudioTruncated(t *testing.T) {
	d, _ := NewBinkAudioDecoder(44100, 2, false, false)
	w := &bitWriter{}
	rng := rand.New(rand.NewSource(1))
	coef := make([]int, 4096)

	for i := range coef {
		coef[i] = rng.Intn(15) - 7
	}

	encodeBlock(w, d, []synthBlock{{quant: 50, coef: coef, width: 4}})

	if _, err := d.Decode(nil, audioPacket(w.buf[:len(w.buf)/2], 0)); err == nil {
		t.Fatal("a truncated block must be an error")
	}

	// garbage must not panic
	for i := 0; i < 50; i++ {
		junk := make([]byte, 200+rng.Intn(3000))
		rng.Read(junk)
		d.Reset()
		_, _ = d.Decode(nil, audioPacket(junk, 0))
	}
}

func TestBinkAudioShortPackets(t *testing.T) {
	d, _ := NewBinkAudioDecoder(44100, 2, false, false)

	for _, p := range [][]byte{nil, {1}, {1, 2, 3, 4}} {
		out, err := d.Decode(nil, p)
		if err != nil || len(out) != 0 {
			t.Errorf("packet %v: %v %d", p, err, len(out))
		}
	}
}
