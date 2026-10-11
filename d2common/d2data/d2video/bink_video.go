package d2video

import (
	"errors"
	"fmt"
)

// Bink 1 video frame decoding (revisions 'd'..'k'; Diablo II ships 'i'). Written from the public format
// description (block types, bundles, Huffman trees, the Bink DCT); the revision 'b' codec is not supported.

// block types
const (
	blkSkip = iota
	blkScaled
	blkMotion
	blkRun
	blkResidue
	blkIntra
	blkFill
	blkInter
	blkPattern
	blkRaw
)

// bundle (data source) numbers
const (
	srcBlockTypes = iota
	srcSubBlockTypes
	srcColors
	srcPattern
	srcXOff
	srcYOff
	srcIntraDC
	srcInterDC
	srcRun
	numSources
)

const dcStartBits = 11

var errBinkData = errors.New("bink: invalid data")

type tree struct {
	num  int
	syms [16]uint8
}

func (t *tree) get(b *bitReader) int {
	return int(t.syms[binkVLCs[t.num].read(b)])
}

type bundle struct {
	length int
	tree   tree
	data   []int32
	curDec int // -1 when decoding of this bundle is finished for the plane
	curPtr int
}

type plane struct {
	pix    []byte
	stride int
	rows   int
}

// videoCodec holds the state that persists from frame to frame.
type videoCodec struct {
	version    byte
	width      int
	height     int
	hasAlpha   bool
	swapPlanes bool
	gray       bool

	cur, prev [4]plane // Y, U, V (and A) planes
	bundles   [numSources]bundle
	colHigh   [16]tree
	colLast   int
	frameNum  int
}

func alignUp(v, a int) int { return (v + a - 1) &^ (a - 1) }

func newVideoCodec(version byte, w, h int, alpha bool) (*videoCodec, error) {
	if version < 'd' || version > 'k' {
		return nil, fmt.Errorf("bink: unsupported video revision %q", version)
	}

	if w <= 0 || h <= 0 || w > 8192 || h > 8192 {
		return nil, fmt.Errorf("bink: bad dimensions %dx%d", w, h)
	}

	c := &videoCodec{version: version, width: w, height: h, hasAlpha: alpha, swapPlanes: version >= 'h'}

	aw, ah := alignUp(w, 16), alignUp(h, 16)
	for i := 0; i < 4; i++ {
		pw, ph := aw, ah
		if i == 1 || i == 2 {
			pw, ph = aw/2, ah/2
		}

		c.cur[i] = plane{pix: make([]byte, pw*ph), stride: pw, rows: ph}
		c.prev[i] = plane{pix: make([]byte, pw*ph), stride: pw, rows: ph}
	}

	blocks := (aw / 8) * (ah / 8)
	for i := range c.bundles {
		c.bundles[i].data = make([]int32, blocks*64)
	}

	return c, nil
}

func log2(v int) int {
	n := 0
	for v > 1 {
		v >>= 1
		n++
	}

	return n
}

func (c *videoCodec) initLengths(width, bw int) {
	width = alignUp(width, 8)
	b := &c.bundles

	b[srcBlockTypes].length = log2((width>>3)+511) + 1
	b[srcSubBlockTypes].length = log2((width>>4)+511) + 1
	b[srcColors].length = log2(bw*64+511) + 1
	b[srcIntraDC].length = log2((width>>3)+511) + 1
	b[srcInterDC].length = b[srcIntraDC].length
	b[srcXOff].length = b[srcIntraDC].length
	b[srcYOff].length = b[srcIntraDC].length
	b[srcPattern].length = log2((bw<<3)+511) + 1
	b[srcRun].length = log2(bw*48+511) + 1
}

func merge(b *bitReader, dst, src []uint8, size int) {
	s1, s2 := 0, size
	n1, n2 := size, size
	d := 0

	for {
		if b.bit() == 0 {
			dst[d] = src[s1]
			s1++
			n1--
		} else {
			dst[d] = src[s2]
			s2++
			n2--
		}

		d++

		if n1 == 0 || n2 == 0 {
			break
		}
	}

	for ; n1 > 0; n1-- {
		dst[d] = src[s1]
		d++
		s1++
	}

	for ; n2 > 0; n2-- {
		dst[d] = src[s2]
		d++
		s2++
	}
}

func readTree(b *bitReader, t *tree) error {
	if b.left() < 4 {
		return errBinkOverrun
	}

	t.num = int(b.bits(4))
	if t.num == 0 {
		for i := range t.syms {
			t.syms[i] = uint8(i)
		}

		return nil
	}

	if b.bit() != 0 {
		var used [16]bool

		n := int(b.bits(3))
		for i := 0; i <= n; i++ {
			t.syms[i] = uint8(b.bits(4))
			used[t.syms[i]] = true
		}

		for i := 0; i < 16 && n < 15; i++ {
			if !used[i] {
				n++
				t.syms[n] = uint8(i)
			}
		}

		return nil
	}

	var tmp1, tmp2 [16]uint8

	in, out := tmp1[:], tmp2[:]
	n := int(b.bits(2))

	for i := range in {
		in[i] = uint8(i)
	}

	for i := 0; i <= n; i++ {
		size := 1 << uint(i)
		for off := 0; off < 16; off += size << 1 {
			merge(b, out[off:], in[off:], size)
		}

		in, out = out, in
	}

	copy(t.syms[:], in)

	return nil
}

func (c *videoCodec) readBundle(b *bitReader, num int) error {
	if num == srcColors {
		for i := range c.colHigh {
			if err := readTree(b, &c.colHigh[i]); err != nil {
				return err
			}
		}

		c.colLast = 0
	}

	if num != srcIntraDC && num != srcInterDC {
		if err := readTree(b, &c.bundles[num].tree); err != nil {
			return err
		}
	}

	c.bundles[num].curDec = 0
	c.bundles[num].curPtr = 0

	return nil
}

// checkRead implements the common prologue: returns the element count or 0 if nothing is to be decoded.
func (bu *bundle) checkRead(b *bitReader) int {
	if bu.curDec < 0 || bu.curDec > bu.curPtr {
		return 0
	}

	t := int(b.bits(bu.length))
	if t == 0 {
		bu.curDec = -1
	}

	return t
}

func (bu *bundle) room(n int) bool { return bu.curDec+n <= len(bu.data) }

func (c *videoCodec) readRuns(b *bitReader, bu *bundle) error {
	t := bu.checkRead(b)
	if t == 0 {
		return nil
	}

	if !bu.room(t) || b.left() < 1 {
		return errBinkData
	}

	if b.bit() != 0 {
		v := int32(b.bits(4))
		for i := 0; i < t; i++ {
			bu.data[bu.curDec+i] = v
		}

		bu.curDec += t

		return nil
	}

	for end := bu.curDec + t; bu.curDec < end; bu.curDec++ {
		bu.data[bu.curDec] = int32(bu.tree.get(b))
	}

	return nil
}

func signed(b *bitReader, v int) int {
	if v != 0 && b.bit() != 0 {
		return -v
	}

	return v
}

func (c *videoCodec) readMotion(b *bitReader, bu *bundle) error {
	t := bu.checkRead(b)
	if t == 0 {
		return nil
	}

	if !bu.room(t) || b.left() < 1 {
		return errBinkData
	}

	if b.bit() != 0 {
		v := int32(signed(b, int(b.bits(4))))
		for i := 0; i < t; i++ {
			bu.data[bu.curDec+i] = v
		}

		bu.curDec += t

		return nil
	}

	for end := bu.curDec + t; bu.curDec < end; bu.curDec++ {
		bu.data[bu.curDec] = int32(signed(b, bu.tree.get(b)))
	}

	return nil
}

var rleLens = [4]int{4, 8, 12, 32}

func (c *videoCodec) readBlockTypes(b *bitReader, bu *bundle) error {
	t := bu.checkRead(b)
	if t == 0 {
		return nil
	}

	if c.version == 'k' {
		t ^= 0xBB
		if t == 0 {
			bu.curDec = -1
			return nil
		}
	}

	if !bu.room(t) || b.left() < 1 {
		return errBinkData
	}

	end := bu.curDec + t

	if b.bit() != 0 {
		v := int32(b.bits(4))
		for i := 0; i < t; i++ {
			bu.data[bu.curDec+i] = v
		}

		bu.curDec += t

		return nil
	}

	last := int32(0)

	for bu.curDec < end {
		v := bu.tree.get(b)
		if v < 12 {
			last = int32(v)
			bu.data[bu.curDec] = last
			bu.curDec++

			continue
		}

		run := rleLens[v-12]
		if end-bu.curDec < run {
			return errBinkData
		}

		for i := 0; i < run; i++ {
			bu.data[bu.curDec+i] = last
		}

		bu.curDec += run
	}

	return nil
}

func (c *videoCodec) readPatterns(b *bitReader, bu *bundle) error {
	t := bu.checkRead(b)
	if t == 0 {
		return nil
	}

	if !bu.room(t) {
		return errBinkData
	}

	for end := bu.curDec + t; bu.curDec < end; bu.curDec++ {
		if b.left() < 2 {
			return errBinkOverrun
		}

		v := bu.tree.get(b)
		v |= bu.tree.get(b) << 4
		bu.data[bu.curDec] = int32(v)
	}

	return nil
}

func (c *videoCodec) colorValue(b *bitReader, bu *bundle) int32 {
	c.colLast = c.colHigh[c.colLast].get(b)
	v := c.colLast<<4 | bu.tree.get(b)

	if c.version < 'i' {
		sign := int(int8(v) >> 7)
		v = ((v & 0x7F) ^ sign) - sign
		v += 0x80
	}

	return int32(v & 0xFF)
}

func (c *videoCodec) readColors(b *bitReader, bu *bundle) error {
	t := bu.checkRead(b)
	if t == 0 {
		return nil
	}

	if !bu.room(t) || b.left() < 1 {
		return errBinkData
	}

	if b.bit() != 0 {
		v := c.colorValue(b, bu)
		for i := 0; i < t; i++ {
			bu.data[bu.curDec+i] = v
		}

		bu.curDec += t

		return nil
	}

	for end := bu.curDec + t; bu.curDec < end; bu.curDec++ {
		if b.left() < 2 {
			return errBinkOverrun
		}

		bu.data[bu.curDec] = c.colorValue(b, bu)
	}

	return nil
}

func (c *videoCodec) readDCs(b *bitReader, bu *bundle, startBits int, hasSign bool) error {
	length := bu.checkRead(b)
	if length == 0 {
		return nil
	}

	sb := startBits
	if hasSign {
		sb--
	}

	if b.left() < sb {
		return errBinkOverrun
	}

	v := int(b.bits(sb))
	if v != 0 && hasSign && b.bit() != 0 {
		v = -v
	}

	if len(bu.data)-bu.curDec < 1 {
		return errBinkData
	}

	bu.data[bu.curDec] = int32(v)
	bu.curDec++
	length--

	for i := 0; i < length; i += 8 {
		len2 := length - i
		if len2 > 8 {
			len2 = 8
		}

		if len(bu.data)-bu.curDec < len2 {
			return errBinkData
		}

		bsize := int(b.bits(4))
		if bsize == 0 {
			for j := 0; j < len2; j++ {
				bu.data[bu.curDec] = int32(v)
				bu.curDec++
			}

			continue
		}

		for j := 0; j < len2; j++ {
			v2 := signed(b, int(b.bits(bsize)))
			v += v2

			if v < -32768 || v > 32767 {
				return errBinkData
			}

			bu.data[bu.curDec] = int32(v)
			bu.curDec++
		}
	}

	return nil
}

func (c *videoCodec) value(src int) int {
	bu := &c.bundles[src]
	v := bu.data[bu.curPtr]
	bu.curPtr++

	return int(v)
}

// readDCTCoeffs reads the coefficient significance maps and values of a DCT block and returns
// the quantiser index and the list of coefficients (in scan position) that were set.
func readDCTCoeffs(b *bitReader, block *[64]int32, coefIdx *[64]int) (quant, count int, err error) {
	var coefList, modeList [128]int

	listStart, listEnd := 64, 64

	push := func(c, m int) {
		coefList[listEnd] = c
		modeList[listEnd] = m
		listEnd++
	}

	if b.left() < 4 {
		return 0, 0, errBinkOverrun
	}

	push(4, 0)
	push(24, 0)
	push(44, 0)
	push(1, 3)
	push(2, 3)
	push(3, 3)

	for bits := int(b.bits(4)) - 1; bits >= 0; bits-- {
		pos := listStart

		for pos < listEnd {
			if (modeList[pos]|coefList[pos]) == 0 || b.bit() == 0 {
				pos++
				continue
			}

			ccoef := coefList[pos]
			mode := modeList[pos]

			switch mode {
			case 0, 2:
				if mode == 0 {
					coefList[pos] = ccoef + 4
					modeList[pos] = 1
				} else {
					coefList[pos] = 0
					modeList[pos] = 0
					pos++
				}

				for i := 0; i < 4; i, ccoef = i+1, ccoef+1 {
					if b.bit() != 0 {
						listStart--
						coefList[listStart] = ccoef
						modeList[listStart] = 3

						continue
					}

					t := coefValue(b, bits)
					block[binkScan[ccoef]] = int32(t)
					coefIdx[count] = ccoef
					count++
				}
			case 1:
				modeList[pos] = 2

				for i := 0; i < 3; i++ {
					ccoef += 4
					push(ccoef, 2)
				}
			case 3:
				t := coefValue(b, bits)
				block[binkScan[ccoef]] = int32(t)
				coefIdx[count] = ccoef
				count++
				coefList[pos] = 0
				modeList[pos] = 0
				pos++
			}
		}

		if b.left() < -64 {
			return 0, 0, errBinkOverrun
		}
	}

	return int(b.bits(4)), count, nil
}

func coefValue(b *bitReader, bits int) int {
	if bits == 0 {
		return 1 - int(b.bit()<<1)
	}

	t := int(b.bits(bits)) | 1<<uint(bits)
	if b.bit() != 0 {
		return -t
	}

	return t
}

func unquantize(block *[64]int32, quant *[64]uint32, count int, coefIdx *[64]int) {
	block[0] = int32(uint32(block[0])*quant[0]) >> 11

	for i := 0; i < count; i++ {
		idx := coefIdx[i]
		p := binkScan[idx]
		block[p] = int32(uint32(block[p])*quant[idx]) >> 11
	}
}

// readResidue reads a residue block (difference added to a motion compensated block).
func readResidue(b *bitReader, block *[64]int16, masks int) {
	var coefList, modeList [128]int

	var nz [64]int

	nzCount := 0
	listStart, listEnd := 64, 64

	push := func(c, m int) {
		coefList[listEnd] = c
		modeList[listEnd] = m
		listEnd++
	}

	push(4, 0)
	push(24, 0)
	push(44, 0)
	push(0, 2)

	for mask := 1 << b.bits(3); mask != 0; mask >>= 1 {
		for i := 0; i < nzCount; i++ {
			if b.bit() == 0 {
				continue
			}

			if block[nz[i]] < 0 {
				block[nz[i]] -= int16(mask)
			} else {
				block[nz[i]] += int16(mask)
			}

			masks--
			if masks < 0 {
				return
			}
		}

		pos := listStart

		for pos < listEnd {
			if (coefList[pos]|modeList[pos]) == 0 || b.bit() == 0 {
				pos++
				continue
			}

			ccoef := coefList[pos]
			mode := modeList[pos]

			switch mode {
			case 0, 2:
				if mode == 0 {
					coefList[pos] = ccoef + 4
					modeList[pos] = 1
				} else {
					coefList[pos] = 0
					modeList[pos] = 0
					pos++
				}

				for i := 0; i < 4; i, ccoef = i+1, ccoef+1 {
					if b.bit() != 0 {
						listStart--
						coefList[listStart] = ccoef
						modeList[listStart] = 3

						continue
					}

					nz[nzCount] = int(binkScan[ccoef])
					nzCount++
					block[binkScan[ccoef]] = int16(signed(b, mask)) // mask is never 0 here

					masks--
					if masks < 0 {
						return
					}
				}
			case 1:
				modeList[pos] = 2

				for i := 0; i < 3; i++ {
					ccoef += 4
					push(ccoef, 2)
				}
			case 3:
				nz[nzCount] = int(binkScan[ccoef])
				nzCount++
				block[binkScan[ccoef]] = int16(signed(b, mask))
				coefList[pos] = 0
				modeList[pos] = 0
				pos++

				masks--
				if masks < 0 {
					return
				}
			}
		}
	}
}

// The Bink inverse DCT (fixed point).
const (
	idctA1 = 2896
	idctA2 = 2217
	idctA3 = 3784
	idctA4 = -5352
)

func mul(x, y int32) int32 { return int32(uint32(x)*uint32(y)) >> 11 }

func idct1d(s [8]int32) [8]int32 {
	a0 := s[0] + s[4]
	a1 := s[0] - s[4]
	a2 := s[2] + s[6]
	a3 := mul(idctA1, s[2]-s[6])
	a4 := s[5] + s[3]
	a5 := s[5] - s[3]
	a6 := s[1] + s[7]
	a7 := s[1] - s[7]
	b0 := a4 + a6
	b1 := mul(idctA3, a5+a7)
	b2 := mul(idctA4, a5) - b0 + b1
	b3 := mul(idctA1, a6-a4) - b2
	b4 := mul(idctA2, a7) + b3 - b1

	return [8]int32{
		a0 + a2 + b0, a1 + a3 - a2 + b2, a1 - a3 + a2 + b3, a0 - a2 - b4,
		a0 - a2 + b4, a1 - a3 + a2 - b3, a1 + a3 - a2 - b2, a0 + a2 - b0,
	}
}

// idct transforms block in place; the result is the 8x8 residue/pixel values.
func idct(block *[64]int32) {
	var tmp [64]int32

	for i := 0; i < 8; i++ {
		var col [8]int32

		zero := true

		for k := 0; k < 8; k++ {
			col[k] = block[i+8*k]
			if k > 0 && col[k] != 0 {
				zero = false
			}
		}

		if zero {
			for k := 0; k < 8; k++ {
				tmp[i+8*k] = col[0]
			}

			continue
		}

		out := idct1d(col)
		for k := 0; k < 8; k++ {
			tmp[i+8*k] = out[k]
		}
	}

	for i := 0; i < 8; i++ {
		var row [8]int32

		copy(row[:], tmp[8*i:8*i+8])

		out := idct1d(row)
		for k := 0; k < 8; k++ {
			block[8*i+k] = (out[k] + 0x7F) >> 8
		}
	}
}

func idctPut(dst []byte, stride int, block *[64]int32) {
	idct(block)

	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			dst[y*stride+x] = byte(block[y*8+x])
		}
	}
}

func idctAdd(dst []byte, stride int, block *[64]int32) {
	idct(block)

	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			dst[y*stride+x] += byte(block[y*8+x])
		}
	}
}

func fillBlock(dst []byte, stride, size int, v byte) {
	for y := 0; y < size; y++ {
		row := dst[y*stride : y*stride+size]
		for x := range row {
			row[x] = v
		}
	}
}

func scaleBlock(src *[64]byte, dst []byte, stride int) {
	for j := 0; j < 8; j++ {
		for i := 0; i < 8; i++ {
			v := src[j*8+i]
			dst[2*j*stride+2*i] = v
			dst[2*j*stride+2*i+1] = v
			dst[(2*j+1)*stride+2*i] = v
			dst[(2*j+1)*stride+2*i+1] = v
		}
	}
}

func (c *videoCodec) runBlock(b *bitReader, scan *[64]uint8, out func(pos int, v byte)) error {
	i := 0

	for {
		run := c.value(srcRun) + 1

		i += run
		if i > 64 {
			return errBinkData
		}

		if b.bit() != 0 {
			v := byte(c.value(srcColors))
			for j := 0; j < run; j++ {
				out(int(scan[i-run+j]), v)
			}
		} else {
			for j := 0; j < run; j++ {
				out(int(scan[i-run+j]), byte(c.value(srcColors)))
			}
		}

		if i >= 63 {
			break
		}
	}

	if i == 63 {
		out(int(scan[63]), byte(c.value(srcColors)))
	}

	return nil
}

func (c *videoCodec) decodePlane(b *bitReader, idx int, isChroma bool) (err error) {
	defer func() {
		// Corrupt data may index out of the bundles; report rather than crash.
		if r := recover(); r != nil {
			err = fmt.Errorf("bink: corrupt plane data: %v", r)
		}
	}()

	pl := &c.cur[idx]
	prev := &c.prev[idx]
	stride := pl.stride

	bw, bh := (c.width+7)>>3, (c.height+7)>>3
	width, height := c.width, c.height

	if isChroma {
		bw, bh = (c.width+15)>>4, (c.height+15)>>4
		width, height = c.width>>1, c.height>>1
	}

	if c.version == 'k' && b.bit() != 0 {
		fill := byte(b.bits(8))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				pl.pix[y*stride+x] = fill
			}
		}

		b.align32()

		return nil
	}

	if width < 8 {
		width = 8
	}

	c.initLengths(width, bw)

	for i := 0; i < numSources; i++ {
		if err = c.readBundle(b, i); err != nil {
			return err
		}
	}

	refEnd := (bw - 1 + stride*(bh-1)) * 8

	var coefIdx [64]int

	for by := 0; by < bh; by++ {
		bs := &c.bundles

		steps := []func() error{
			func() error { return c.readBlockTypes(b, &bs[srcBlockTypes]) },
			func() error { return c.readBlockTypes(b, &bs[srcSubBlockTypes]) },
			func() error { return c.readColors(b, &bs[srcColors]) },
			func() error { return c.readPatterns(b, &bs[srcPattern]) },
			func() error { return c.readMotion(b, &bs[srcXOff]) },
			func() error { return c.readMotion(b, &bs[srcYOff]) },
			func() error { return c.readDCs(b, &bs[srcIntraDC], dcStartBits, false) },
			func() error { return c.readDCs(b, &bs[srcInterDC], dcStartBits, true) },
			func() error { return c.readRuns(b, &bs[srcRun]) },
		}
		for _, s := range steps {
			if err = s(); err != nil {
				return err
			}
		}

		if b.left() < 0 {
			return errBinkOverrun
		}

		base := 8 * by * stride

		for bx := 0; bx < bw; bx++ {
			off := base + bx*8
			dst := pl.pix[off:]
			blk := c.value(srcBlockTypes)

			if (by&1 != 0 || bx&1 != 0) && blk == blkScaled {
				bx++
				continue
			}

			switch blk {
			case blkSkip:
				copyBlock(dst, prev.pix[off:], stride)
			case blkScaled:
				if err = c.scaledBlock(b, dst, stride, &coefIdx); err != nil {
					return err
				}

				bx++
			case blkMotion, blkResidue, blkInter:
				if err = c.motion(dst, prev, off, stride, refEnd); err != nil {
					return err
				}

				switch blk {
				case blkResidue:
					var res [64]int16

					readResidue(b, &res, int(b.bits(7)))

					for y := 0; y < 8; y++ {
						for x := 0; x < 8; x++ {
							dst[y*stride+x] += byte(res[y*8+x])
						}
					}
				case blkInter:
					var blockD [64]int32

					blockD[0] = int32(c.value(srcInterDC))

					q, n, e := readDCTCoeffs(b, &blockD, &coefIdx)
					if e != nil {
						return e
					}

					unquantize(&blockD, &binkInterQuant[q], n, &coefIdx)
					idctAdd(dst, stride, &blockD)
				}
			case blkRun:
				scan := &binkPatterns[b.bits(4)]
				if err = c.runBlock(b, scan, func(pos int, v byte) {
					dst[(pos>>3)*stride+(pos&7)] = v
				}); err != nil {
					return err
				}
			case blkIntra:
				var blockD [64]int32

				blockD[0] = int32(c.value(srcIntraDC))

				q, n, e := readDCTCoeffs(b, &blockD, &coefIdx)
				if e != nil {
					return e
				}

				unquantize(&blockD, &binkIntraQuant[q], n, &coefIdx)
				idctPut(dst, stride, &blockD)
			case blkFill:
				fillBlock(dst, stride, 8, byte(c.value(srcColors)))
			case blkPattern:
				col := [2]byte{byte(c.value(srcColors)), byte(c.value(srcColors))}

				for y := 0; y < 8; y++ {
					v := c.value(srcPattern)
					for x := 0; x < 8; x, v = x+1, v>>1 {
						dst[y*stride+x] = col[v&1]
					}
				}
			case blkRaw:
				cs := &c.bundles[srcColors]
				for y := 0; y < 8; y++ {
					for x := 0; x < 8; x++ {
						dst[y*stride+x] = byte(cs.data[cs.curPtr+y*8+x])
					}
				}

				cs.curPtr += 64
			default:
				return fmt.Errorf("bink: unknown block type %d", blk)
			}
		}
	}

	b.align32()

	return nil
}

func copyBlock(dst, src []byte, stride int) {
	for y := 0; y < 8; y++ {
		copy(dst[y*stride:y*stride+8], src[y*stride:y*stride+8])
	}
}

func (c *videoCodec) motion(dst []byte, prev *plane, off, stride, refEnd int) error {
	xoff := c.value(srcXOff)
	yoff := c.value(srcYOff)
	ref := off + xoff + yoff*stride

	if ref < 0 || ref > refEnd {
		return fmt.Errorf("bink: motion vector out of bounds (%d,%d)", xoff, yoff)
	}

	copyBlock(dst, prev.pix[ref:], stride)

	return nil
}

// scaledBlock decodes a 16x16 block built from an 8x8 block that is doubled in both directions.
func (c *videoCodec) scaledBlock(b *bitReader, dst []byte, stride int, coefIdx *[64]int) error {
	var ub [64]byte

	sub := c.value(srcSubBlockTypes)

	switch sub {
	case blkRun:
		if b.left() < 4 {
			return errBinkOverrun
		}

		scan := &binkPatterns[b.bits(4)]
		if err := c.runBlock(b, scan, func(pos int, v byte) { ub[pos] = v }); err != nil {
			return err
		}
	case blkIntra:
		var blockD [64]int32

		blockD[0] = int32(c.value(srcIntraDC))

		q, n, e := readDCTCoeffs(b, &blockD, coefIdx)
		if e != nil {
			return e
		}

		unquantize(&blockD, &binkIntraQuant[q], n, coefIdx)
		idct(&blockD)

		for i := range ub {
			ub[i] = byte(blockD[i])
		}
	case blkFill:
		fillBlock(dst, stride, 16, byte(c.value(srcColors)))
		return nil
	case blkPattern:
		col := [2]byte{byte(c.value(srcColors)), byte(c.value(srcColors))}

		for j := 0; j < 8; j++ {
			v := c.value(srcPattern)
			for i := 0; i < 8; i, v = i+1, v>>1 {
				ub[i+j*8] = col[v&1]
			}
		}
	case blkRaw:
		for j := 0; j < 8; j++ {
			for i := 0; i < 8; i++ {
				ub[i+j*8] = byte(c.value(srcColors))
			}
		}
	default:
		return fmt.Errorf("bink: bad 16x16 block type %d", sub)
	}

	scaleBlock(&ub, dst, stride)

	return nil
}

// decode decodes one video packet (the frame data after any audio packets) and returns the YUV 4:2:0 planes.
func (c *videoCodec) decode(data []byte) error {
	b := newBitReader(data)

	if c.hasAlpha {
		if c.version >= 'i' {
			b.pos += 32
		}

		if err := c.decodePlane(b, 3, false); err != nil {
			return err
		}
	}

	if c.version >= 'i' {
		b.pos += 32
	}

	c.frameNum++

	for p := 0; p < 3; p++ {
		idx := p
		if p != 0 && c.swapPlanes {
			idx = p ^ 3
		}

		if err := c.decodePlane(b, idx, p != 0); err != nil {
			return err
		}

		if b.pos >= b.end {
			break
		}
	}

	c.cur, c.prev = c.prev, c.cur

	return nil
}

// rgba converts the most recently decoded frame (now in prev) to RGBA (BT.601, limited range).
func (c *videoCodec) rgba(out []byte) {
	y, u, v := &c.prev[0], &c.prev[1], &c.prev[2]
	full := c.version == 'k'

	for row := 0; row < c.height; row++ {
		for col := 0; col < c.width; col++ {
			yy := int(y.pix[row*y.stride+col])
			uu := int(u.pix[(row>>1)*u.stride+(col>>1)]) - 128
			vv := int(v.pix[(row>>1)*v.stride+(col>>1)]) - 128

			var r, g, bl int

			if full {
				r = yy + (91881*vv)>>16
				g = yy - (22554*uu+46802*vv)>>16
				bl = yy + (116130*uu)>>16
			} else {
				yy = (yy - 16) * 76309
				r = (yy + 104597*vv) >> 16
				g = (yy - 25675*uu - 53279*vv) >> 16
				bl = (yy + 132201*uu) >> 16
			}

			o := (row*c.width + col) * 4
			out[o] = clamp8(r)
			out[o+1] = clamp8(g)
			out[o+2] = clamp8(bl)
			out[o+3] = 255
		}
	}

	if c.hasAlpha {
		a := &c.prev[3]
		for row := 0; row < c.height; row++ {
			for col := 0; col < c.width; col++ {
				out[(row*c.width+col)*4+3] = a.pix[row*a.stride+col]
			}
		}
	}
}

func clamp8(v int) byte {
	if v < 0 {
		return 0
	}

	if v > 255 {
		return 255
	}

	return byte(v)
}
