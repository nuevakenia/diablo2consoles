package d2video

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// NewAudioDecoder returns a decoder for audio track i of the file (see AudioTracks and LastAudio).
func (v *BinkDecoder) NewAudioDecoder(i int) (*BinkAudioDecoder, error) {
	if i < 0 || i >= len(v.AudioTracks) {
		return nil, fmt.Errorf("bink: no audio track %d", i)
	}

	t := v.AudioTracks[i]
	channels := 1

	if t.Stereo {
		channels = 2
	}

	return NewBinkAudioDecoder(int(t.AudioSampleRateHz), channels, t.Algorithm == BinkAudioAlgorithmDCT,
		v.videoCodecRevision == 'b')
}

// AudioPacket returns the audio packet of one track stored in frame i, without decoding the frame's video. The
// packet is a slice of the file data (it starts with the 32 bit decoded size, as BinkAudioDecoder.Decode expects).
// Audio is not aligned with the video frames: a frame holds whatever whole blocks the encoder interleaved there
// (the first one several), so a player decodes packets ahead of the video and follows the audio clock.
func (v *BinkDecoder) AudioPacket(frame, track int) ([]byte, error) {
	data, err := v.frameData(frame)
	if err != nil {
		return nil, err
	}

	if track < 0 || track >= len(v.AudioTracks) {
		return nil, fmt.Errorf("bink: no audio track %d", track)
	}

	for t := 0; t <= track; t++ {
		if len(data) < 4 {
			return nil, errors.New("bink: truncated audio packet")
		}

		n := int(binary.LittleEndian.Uint32(data))
		if n < 0 || 4+n > len(data) {
			return nil, errors.New("bink: bad audio packet size")
		}

		if t == track {
			return data[4 : 4+n], nil
		}

		data = data[4+n:]
	}

	return nil, nil
}
