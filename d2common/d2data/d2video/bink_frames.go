package d2video

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Revision returns the Bink revision letter from the file header ('i' for Diablo II's "BIKi" files).
func (v *BinkDecoder) Revision() byte { return v.videoCodecRevision }

// NumFrames returns the number of video frames in the file.
func (v *BinkDecoder) NumFrames() int { return int(v.numberOfFrames) }

// FrameIndex returns the index of the next frame NextFrame will decode.
func (v *BinkDecoder) FrameIndex() int { return int(v.frameIndex) }

// Reset rewinds to the first frame (the first frame of a Bink file is a keyframe).
func (v *BinkDecoder) Reset() error {
	v.frameIndex = 0

	return v.initVideo()
}

func (v *BinkDecoder) initVideo() error {
	codec, err := newVideoCodec(v.videoCodecRevision, int(v.VideoWidth), int(v.VideoHeight), v.HasAlphaPlane)
	if err != nil {
		return err
	}

	codec.gray = v.Grayscale
	v.codec = codec

	return nil
}

// frameData returns the raw bytes of frame i.
func (v *BinkDecoder) frameData(i int) ([]byte, error) {
	if i < 0 || i >= int(v.numberOfFrames) {
		return nil, io.EOF
	}

	start := int(v.FrameIndexTable[i] &^ 1)
	end := int(v.FrameIndexTable[i+1] &^ 1)

	if start < 0 || end > len(v.source) || start >= end {
		return nil, fmt.Errorf("bink: frame %d has bad extent %d-%d", i, start, end)
	}

	return v.source[start:end], nil
}

// NextFrame decodes the next frame and returns it as tightly packed RGBA pixels (Width*Height*4 bytes).
// The returned slice is reused by the following call; copy it if it must be kept. It returns io.EOF after
// the last frame. Audio packets of the frame are left in LastAudio.
func (v *BinkDecoder) NextFrame() ([]byte, error) {
	data, err := v.frameData(int(v.frameIndex))
	if err != nil {
		return nil, err
	}

	v.LastAudio = v.LastAudio[:0]

	for range v.AudioTracks {
		if len(data) < 4 {
			return nil, errors.New("bink: truncated audio packet")
		}

		n := int(binary.LittleEndian.Uint32(data))
		if n < 0 || 4+n > len(data) {
			return nil, errors.New("bink: bad audio packet size")
		}

		v.LastAudio = append(v.LastAudio, data[4:4+n])
		data = data[4+n:]
	}

	if err = v.codec.decode(data); err != nil {
		return nil, fmt.Errorf("frame %d: %w", v.frameIndex, err)
	}

	need := int(v.VideoWidth) * int(v.VideoHeight) * 4
	if cap(v.rgbaBuf) < need {
		v.rgbaBuf = make([]byte, need)
	}

	v.rgbaBuf = v.rgbaBuf[:need]
	v.codec.rgba(v.rgbaBuf)
	v.frameIndex++

	return v.rgbaBuf, nil
}
