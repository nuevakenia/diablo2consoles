package d2video

import (
	"encoding/binary"
	"math"
	"math/rand"
	"os"
	"strconv"
	"testing"
)

// The transforms against their definitions, on random input.
func TestBinkAudioTransformsMatchDefinition(t *testing.T) {
	rng := rand.New(rand.NewSource(3))

	d, _ := NewBinkAudioDecoder(11025, 2, true, false) // frameLen 512
	n := d.frameLen
	x := make([]float64, n)

	for i := range x {
		x[i] = rng.NormFloat64()
	}

	got := append([]float64(nil), x...)
	d.dct3(got)

	for m := 0; m < n; m += 37 {
		want := 0.0
		for k := 0; k < n; k++ {
			want += x[k] * math.Cos(math.Pi*float64(k)*float64(2*m+1)/float64(2*n))
		}

		want *= 2 / float64(n)

		if math.Abs(got[m]-want) > 1e-9 {
			t.Fatalf("dct3[%d] = %g, want %g", m, got[m], want)
		}
	}

	r, _ := NewBinkAudioDecoder(11025, 2, false, false) // frameLen 1024
	n = r.frameLen
	x = make([]float64, n)

	for i := range x {
		x[i] = rng.NormFloat64()
	}

	got = append([]float64(nil), x...)
	r.irdft(got)

	for m := 0; m < n; m += 53 {
		// x[m] = 1/2 * (X0 + (-1)^m X(N/2) + 2 sum_k Re(conj(X_k) e^{2 pi i m k / N}))
		want := x[0] + x[1]*float64(1-2*(m&1))

		for k := 1; k < n/2; k++ {
			a := 2 * math.Pi * float64(m*k) / float64(n)
			want += 2 * (x[2*k]*math.Cos(a) + x[2*k+1]*math.Sin(a))
		}

		want *= 0.5

		if math.Abs(got[m]-want) > 1e-9 {
			t.Fatalf("irdft[%d] = %g, want %g", m, got[m], want)
		}
	}
}

// OD2_VIDEO_TEST_FILE=<a user's .bik> decodes the first audio packets to real PCM: the rate and channel count of
// the track, the expected number of samples, and audible (non-silent, not clipped) content. OD2_VIDEO_TEST_PACKETS
// bounds the number of frames (default 120). OD2_VIDEO_REF_PCM=<s16le> (made by another decoder) is compared too.
func TestBinkAudioRealFile(t *testing.T) {
	path := os.Getenv("OD2_VIDEO_TEST_FILE")
	if path == "" {
		t.Skip("OD2_VIDEO_TEST_FILE not set")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	dec, err := CreateBinkDecoder(data)
	if err != nil {
		t.Fatal(err)
	}

	if len(dec.AudioTracks) == 0 {
		t.Skip("file has no audio track")
	}

	ad, err := dec.NewAudioDecoder(0)
	if err != nil {
		t.Fatal(err)
	}

	if ad.SampleRate() != int(dec.AudioTracks[0].AudioSampleRateHz) || ad.Channels() < 1 {
		t.Fatalf("decoder %d Hz %d ch for track %+v", ad.SampleRate(), ad.Channels(), dec.AudioTracks[0])
	}

	frames := 120
	if s := os.Getenv("OD2_VIDEO_TEST_PACKETS"); s != "" {
		frames, _ = strconv.Atoi(s)
	}

	var pcm []int16

	for i := 0; i < frames && i < dec.NumFrames(); i++ {
		if _, err = dec.NextFrame(); err != nil {
			t.Fatal(err)
		}

		if pcm, err = ad.Decode(pcm, dec.LastAudio[0]); err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
	}

	if len(pcm) == 0 || len(pcm)%(ad.SamplesPerBlock()*ad.Channels()) != 0 {
		t.Fatalf("%d samples is not a whole number of blocks of %d", len(pcm), ad.SamplesPerBlock()*ad.Channels())
	}

	var peak, energy float64

	clipped := 0

	for _, v := range pcm {
		a := math.Abs(float64(v))
		peak = math.Max(peak, a)
		energy += a * a

		if v == 32767 || v == -32768 {
			clipped++
		}
	}

	t.Logf("%d Hz, %d ch, %d sample frames (%.2fs), peak %g, rms %.1f, %d clipped", ad.SampleRate(), ad.Channels(),
		len(pcm)/ad.Channels(), float64(len(pcm)/ad.Channels())/float64(ad.SampleRate()), peak,
		math.Sqrt(energy/float64(len(pcm))), clipped)

	if peak < 100 {
		t.Fatalf("decoded audio is silent (peak %g)", peak)
	}

	if clipped > len(pcm)/50 {
		t.Fatalf("%d of %d samples clipped: wrong scaling?", clipped, len(pcm))
	}

	if refPath := os.Getenv("OD2_VIDEO_REF_PCM"); refPath != "" {
		ref, err := os.ReadFile(refPath)
		if err != nil {
			t.Fatal(err)
		}

		n := len(ref) / 2
		if len(pcm) < n {
			n = len(pcm)
		}

		maxDiff := 0

		for i := 0; i < n; i++ {
			d := int(int16(binary.LittleEndian.Uint16(ref[2*i:]))) - int(pcm[i])
			if d < 0 {
				d = -d
			}

			if d > maxDiff {
				maxDiff = d
			}
		}

		t.Logf("compared %d samples with the reference: max difference %d", n, maxDiff)

		if maxDiff > 2 {
			t.Fatalf("differs from the reference by up to %d", maxDiff)
		}
	}
}
