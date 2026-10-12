package d2tbl

import (
	"fmt"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2datautils"
)

// IDStrings maps the numeric string id the exe uses (the "Index" field of a hash node, which is the
// node's own id: VERIFIED against a real string.tbl, where id 0x1055 is the automap "Game: " line,
// 0x1056 "Password: ", 0x1057 the difficulty template, 0x1422 / 0x1423 the difficulty names and
// 0xf9b a single space) to its text. It is separate from TextDictionary, whose "#n" keys of unnamed
// entries are hash slot numbers, not ids.
type IDStrings map[int]string

// LoadIDStrings reads every active node of a .tbl and keys its text by the node's id.
func LoadIDStrings(data []byte, dec Decoder) (IDStrings, error) {
	br := d2datautils.CreateStreamReader(data)

	_, _ = br.ReadBytes(crcByteCount)

	numberOfElements, err := br.ReadUInt16()
	if err != nil {
		return nil, fmt.Errorf("reading number of elements: %v", err)
	}

	hashTableSize, err := br.ReadUInt32()
	if err != nil {
		return nil, fmt.Errorf("reading hash table size: %v", err)
	}

	if _, err = br.ReadByte(); err != nil { // version
		return nil, fmt.Errorf("reading version: %v", err)
	}

	_, _ = br.ReadUInt32() // string offset
	_, _ = br.ReadUInt32() // max tries
	_, _ = br.ReadUInt32() // file size

	if _, err = br.ReadBytes(int(numberOfElements) * 2); err != nil { // the index array
		return nil, fmt.Errorf("reading element index: %v", err)
	}

	if err = d2datautils.CheckCount("hash table", int64(hashTableSize), hashEntrySize, br.Remaining()); err != nil {
		return nil, err
	}

	entries := make([]*textDictionaryHashEntry, hashTableSize)
	scratch := make(TextDictionary)

	// the existing loader reads the node layout (and the values, which are re-read below by node)
	if err = scratch.loadHashEntries(entries, br, dec); err != nil {
		return nil, fmt.Errorf("loading hash entries: %v", err)
	}

	out := make(IDStrings, len(entries))

	for _, e := range entries {
		if e == nil || !e.IsActive {
			continue
		}

		val, verr := nodeValue(data, e, dec)
		if verr != nil {
			return nil, verr
		}

		out[int(e.Index)] = val
	}

	return out, nil
}

func nodeValue(data []byte, e *textDictionaryHashEntry, dec Decoder) (string, error) {
	if e.NameLength == 0 {
		return "", nil
	}

	start, end := int(e.NameString), int(e.NameString)+int(e.NameLength)-1
	if start < 0 || end > len(data) || end < start {
		return "", fmt.Errorf("string of node %d out of range", e.Index)
	}

	raw := data[start:end]
	if dec != nil {
		return dec(raw), nil
	}

	return string(raw), nil
}
