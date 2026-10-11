package d2video

import "errors"

var errBinkOverrun = errors.New("bink: bitstream overrun")

// bitReader reads a little-endian (LSB first) bitstream, the bit order used by Bink video.
type bitReader struct {
	data []byte
	pos  int // in bits
	end  int // in bits
}

func newBitReader(data []byte) *bitReader {
	return &bitReader{data: data, end: len(data) * 8}
}

// peek returns up to 25 bits without consuming them; bits past the end read as zero.
func (b *bitReader) peek(n int) uint32 {
	idx := b.pos >> 3
	shift := uint(b.pos & 7)

	var w uint64

	for i := 0; i < 5; i++ {
		if idx+i < len(b.data) {
			w |= uint64(b.data[idx+i]) << (8 * uint(i))
		}
	}

	return uint32((w >> shift) & ((1 << uint(n)) - 1))
}

func (b *bitReader) bits(n int) uint32 {
	if n == 0 {
		return 0
	}

	v := b.peek(n)
	b.pos += n

	return v
}

func (b *bitReader) bit() uint32 {
	return b.bits(1)
}

func (b *bitReader) left() int {
	return b.end - b.pos
}

// align32 skips to the next 32-bit boundary.
func (b *bitReader) align32() {
	if b.pos&31 != 0 {
		b.pos += 32 - (b.pos & 31)
	}
}

// vlc is a flat lookup table for one Bink Huffman tree (codes are LSB first).
type vlc struct {
	bits  int
	sym   [128]uint8
	count [128]uint8
}

var binkVLCs [16]vlc

func init() {
	for i := 0; i < 16; i++ {
		maxBits := int(binkTreeLens[i][15])
		t := &binkVLCs[i]
		t.bits = maxBits

		for s := 0; s < 16; s++ {
			l := int(binkTreeLens[i][s])
			code := int(binkTreeBits[i][s])

			for fill := 0; fill < 1<<uint(maxBits-l); fill++ {
				idx := code | fill<<uint(l)
				t.sym[idx] = uint8(s)
				t.count[idx] = uint8(l)
			}
		}
	}
}

func (t *vlc) read(b *bitReader) int {
	idx := b.peek(t.bits)
	b.pos += int(t.count[idx])

	return int(t.sym[idx])
}
