package d2video

import (
	"errors"
	"fmt"
	"math"
	"math/cmplx"
)

// Bink audio (the 'BIKi' files carry one 44.1 kHz stereo track). Two codecs share one container: an RDFT variant
// (flag 0x1000 clear in the track flags; both channels are interleaved into one signal and transformed together)
// and a DCT variant (flag set; every channel has its own transform). Written from the public Bink format
// description and the structure of the FFmpeg binkaudio decoder: per block a float header, one 8 bit quantiser per
// Bark band, then run-length grouped coefficients of a given bit width; the inverse transform is followed by a
// linear cross fade over the first 1/16 of the block with the tail of the previous one.

const (
	binkAudioMaxChannels = 2
	binkAudioNumBands    = 25
	binkAudioQuantSteps  = 96
	binkAudioPacketHead  = 4 // every audio packet starts with its decoded size in bytes (not needed here)
)

// binkCriticalFreqs are the upper edges (Hz) of the Bark bands.
var binkCriticalFreqs = [binkAudioNumBands]int{
	100, 200, 300, 400, 510, 630, 770, 920,
	1080, 1270, 1480, 1720, 2000, 2320, 2700, 3150,
	3700, 4400, 5300, 6400, 7700, 9500, 12000, 15500,
	24500,
}

// binkRunLengths maps the 4 bit run code to a number of 8-coefficient groups.
var binkRunLengths = [16]int{2, 3, 4, 5, 6, 8, 9, 10, 11, 12, 13, 14, 15, 16, 32, 64}

// BinkAudioDecoder decodes the packets of one Bink audio track into interleaved 16 bit PCM.
type BinkAudioDecoder struct {
	sampleRate int // of the output, per channel
	channels   int // of the output
	dct        bool
	versionB   bool

	tchannels  int // transform channels: 1 for RDFT (interleaved signal), channels for DCT
	frameLen   int
	overlapLen int
	blockSize  int // output samples per block, all channels
	root       float64
	numBands   int
	bands      [binkAudioNumBands + 1]int
	quant      [binkAudioQuantSteps]float64

	previous [binkAudioMaxChannels][]float64
	coeffs   [binkAudioMaxChannels][]float64
	first    bool

	fft  *binkFFT
	work []complex128
}

// NewBinkAudioDecoder creates a decoder for a track with the given sample rate, channel count (1 or 2) and codec
// (dct false means RDFT). versionB selects the layout of the old 'b' revision files; Diablo II uses 'i'.
func NewBinkAudioDecoder(sampleRate, channels int, dct, versionB bool) (*BinkAudioDecoder, error) {
	if channels < 1 || channels > binkAudioMaxChannels {
		return nil, fmt.Errorf("bink audio: %d channels not supported", channels)
	}

	if sampleRate < 1000 || sampleRate > 192000 {
		return nil, fmt.Errorf("bink audio: sample rate %d not supported", sampleRate)
	}

	d := &BinkAudioDecoder{sampleRate: sampleRate, channels: channels, dct: dct, versionB: versionB, first: true}

	rate := sampleRate

	var bits int

	switch {
	case sampleRate < 22050:
		bits = 9
	case sampleRate < 44100:
		bits = 10
	default:
		bits = 11
	}

	if dct {
		d.tchannels = channels
	} else {
		// the RDFT variant codes the interleaved channels as one signal of twice the rate
		rate *= channels
		d.tchannels = 1

		if !versionB {
			for c := channels; c > 1; c >>= 1 {
				bits++
			}
		}
	}

	d.frameLen = 1 << uint(bits)
	d.overlapLen = d.frameLen / 16
	d.blockSize = (d.frameLen - d.overlapLen) * d.tchannels

	if dct {
		d.root = float64(d.frameLen) / (math.Sqrt(float64(d.frameLen)) * 32768)
	} else {
		d.root = 2 / (math.Sqrt(float64(d.frameLen)) * 32768)
	}

	for i := range d.quant {
		// 0.066399999 / log10(e): the step grows by about 0.5 dB per index
		d.quant[i] = math.Exp(float64(i)*0.15289164787221953823) * d.root
	}

	half := (rate + 1) / 2

	for d.numBands = 1; d.numBands < binkAudioNumBands; d.numBands++ {
		if half <= binkCriticalFreqs[d.numBands-1] {
			break
		}
	}

	d.bands[0] = 2

	for i := 1; i < d.numBands; i++ {
		d.bands[i] = (binkCriticalFreqs[i-1] * d.frameLen / half) &^ 1
	}

	d.bands[d.numBands] = d.frameLen

	for c := 0; c < d.tchannels; c++ {
		d.previous[c] = make([]float64, d.overlapLen)
		d.coeffs[c] = make([]float64, d.frameLen)
	}

	if dct {
		d.fft = newBinkFFT(2 * d.frameLen)
	} else {
		d.fft = newBinkFFT(d.frameLen)
	}

	d.work = make([]complex128, d.fft.n)

	return d, nil
}

// SampleRate is the output rate in Hz.
func (d *BinkAudioDecoder) SampleRate() int { return d.sampleRate }

// Channels is the number of interleaved output channels.
func (d *BinkAudioDecoder) Channels() int { return d.channels }

// SamplesPerBlock is the number of sample frames (one sample of every channel) one coded block yields.
func (d *BinkAudioDecoder) SamplesPerBlock() int { return d.blockSize / d.channels }

// Reset forgets the overlap of the previous block, as at the start of a stream or after a seek.
func (d *BinkAudioDecoder) Reset() { d.first = true }

func binkGetFloat(b *bitReader) float64 {
	power := int(b.bits(5))
	mant := float64(b.bits(23))
	f := math.Ldexp(mant, power-23)

	if b.bit() != 0 {
		f = -f
	}

	return f
}

// Decode decodes one audio packet as stored in a frame (LastAudio of the decoder, including the leading 32 bit
// size field) and appends its interleaved samples to dst. A packet may hold several blocks.
func (d *BinkAudioDecoder) Decode(dst []int16, packet []byte) ([]int16, error) {
	if len(packet) <= binkAudioPacketHead {
		return dst, nil
	}

	b := newBitReader(packet[binkAudioPacketHead:])
	out := make([]float64, d.blockSize)

	for b.left() >= 32 {
		if err := d.decodeBlock(b, out); err != nil {
			return dst, err
		}

		for _, v := range out {
			dst = append(dst, toS16(v))
		}

		b.align32()
	}

	return dst, nil
}

func toS16(v float64) int16 {
	r := math.RoundToEven(v * 32768)

	switch {
	case r > 32767:
		return 32767
	case r < -32768:
		return -32768
	}

	return int16(r)
}

var errBinkAudioShort = errors.New("bink audio: truncated block")

//nolint:funlen,gocognit,gocyclo // one pass over the coded block, mirrors the format
func (d *BinkAudioDecoder) decodeBlock(b *bitReader, out []float64) error {
	if d.dct {
		b.bits(2)
	}

	var quant [binkAudioNumBands]float64

	for ch := 0; ch < d.tchannels; ch++ {
		coeffs := d.coeffs[ch]

		coeffs[0] = binkGetFloat(b) * d.root
		coeffs[1] = binkGetFloat(b) * d.root

		if b.left() < d.numBands*8 {
			return errBinkAudioShort
		}

		for i := 0; i < d.numBands; i++ {
			v := int(b.bits(8))
			if v > binkAudioQuantSteps-1 {
				v = binkAudioQuantSteps - 1
			}

			quant[i] = d.quant[v]
		}

		k := 0
		q := quant[0]

		for i := 2; i < d.frameLen; {
			if b.left() < 0 {
				return errBinkAudioShort
			}

			j := i + 16

			if !d.versionB {
				if b.bit() != 0 {
					j = i + binkRunLengths[b.bits(4)]*8
				} else {
					j = i + 8
				}
			}

			if j > d.frameLen {
				j = d.frameLen
			}

			width := int(b.bits(4))

			if width == 0 {
				for x := i; x < j; x++ {
					coeffs[x] = 0
				}

				i = j

				for d.bands[k] < i {
					q = quant[k]
					k++
				}

				continue
			}

			for ; i < j; i++ {
				if d.bands[k] == i {
					q = quant[k]
					k++
				}

				c := b.bits(width)
				if c == 0 {
					coeffs[i] = 0
				} else if b.bit() != 0 {
					coeffs[i] = -q * float64(c)
				} else {
					coeffs[i] = q * float64(c)
				}
			}

		}

		if b.left() < 0 {
			return errBinkAudioShort
		}

		if d.dct {
			d.dct3(coeffs)
		} else {
			d.irdft(coeffs)
		}
	}

	// cross fade the start of this block with the saved tail of the previous one, keep the new tail
	for ch := 0; ch < d.tchannels; ch++ {
		cur := d.coeffs[ch]
		count := d.overlapLen * d.tchannels

		if !d.first {
			j := ch
			for i := 0; i < d.overlapLen; i, j = i+1, j+d.tchannels {
				cur[i] = (d.previous[ch][i]*float64(count-j) + cur[i]*float64(j)) / float64(count)
			}
		}

		copy(d.previous[ch], cur[d.frameLen-d.overlapLen:])
	}

	d.first = false

	n := d.frameLen - d.overlapLen

	if d.tchannels == 1 {
		copy(out, d.coeffs[0][:n])
	} else {
		for i := 0; i < n; i++ {
			for ch := 0; ch < d.tchannels; ch++ {
				out[i*d.tchannels+ch] = d.coeffs[ch][i]
			}
		}
	}

	return nil
}

// irdft is the inverse of a real FFT whose spectrum is packed as [Re X0, Re X(N/2), Re X1, Im X1, ...]; the
// result is unnormalised (a plain sum over the Hermitian spectrum).
func (d *BinkAudioDecoder) irdft(c []float64) {
	n := d.frameLen
	w := d.work

	w[0] = complex(c[0], 0)
	w[n/2] = complex(c[1], 0)

	for k := 1; k < n/2; k++ {
		v := complex(c[2*k], c[2*k+1])
		w[n-k] = v
		w[k] = cmplx.Conj(v)
	}

	d.fft.transform(w)

	for i := 0; i < n; i++ {
		c[i] = 0.5 * real(w[i])
	}
}

// dct3 is the DCT-III X[m] = (2/N) sum_k x[k] cos(pi k (2m+1) / (2N)), the scaling of the reference decoder.
func (d *BinkAudioDecoder) dct3(c []float64) {
	n := d.frameLen
	w := d.work
	// 2/N and the weight 1 of x[0] were checked against the output of the reference decoder
	scale := 2 / float64(n)

	for k := 0; k < n; k++ {
		a := math.Pi * float64(k) / float64(2*n)
		w[k] = complex(c[k]*math.Cos(a), c[k]*math.Sin(a))
	}

	for k := n; k < 2*n; k++ {
		w[k] = 0
	}

	d.fft.transform(w)

	for m := 0; m < n; m++ {
		c[m] = real(w[m]) * scale
	}
}

// binkFFT is a small in-place radix-2 complex FFT with the positive exponent (e^{+2 pi i nk/N}).
type binkFFT struct {
	n   int
	rev []int
	tw  []complex128
}

func newBinkFFT(n int) *binkFFT {
	f := &binkFFT{n: n, rev: make([]int, n), tw: make([]complex128, n/2)}

	bits := 0
	for 1<<uint(bits) < n {
		bits++
	}

	for i := 0; i < n; i++ {
		r := 0
		for b := 0; b < bits; b++ {
			if i>>uint(b)&1 != 0 {
				r |= 1 << uint(bits-1-b)
			}
		}

		f.rev[i] = r
	}

	for i := range f.tw {
		a := 2 * math.Pi * float64(i) / float64(n)
		f.tw[i] = complex(math.Cos(a), math.Sin(a))
	}

	return f
}

func (f *binkFFT) transform(x []complex128) {
	n := f.n

	for i, r := range f.rev {
		if i < r {
			x[i], x[r] = x[r], x[i]
		}
	}

	for size := 2; size <= n; size <<= 1 {
		half, step := size/2, n/size

		for start := 0; start < n; start += size {
			for k := 0; k < half; k++ {
				t := f.tw[k*step] * x[start+k+half]
				x[start+k+half] = x[start+k] - t
				x[start+k] += t
			}
		}
	}
}
