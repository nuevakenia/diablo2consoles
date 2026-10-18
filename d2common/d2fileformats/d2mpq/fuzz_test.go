package d2mpq

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// synthMPQ builds a small, valid MPQ archive in memory (nothing from the game): a plain file, a
// zlib compressed multi-sector file and a single unit file. mutate may alter the table words before
// they are encrypted, to build hostile archives.
func synthMPQ(mutate func(hash, block []uint32)) []byte {
	return synthMPQSized(8, 2, mutate)
}

// synthMPQSized is synthMPQ with the size of the two tables as parameters (DC-002.28): the tables of the
// game are tens of thousands of entries (d2data.mpq: 65.536 de hash) and las tablas se leen POR TROZOS, asi
// que hace falta poder armar un MPQ cuyas tablas crucen el marco del trozo (1.024 entradas) para comparar
// el camino por trozos contra el de la tabla entera.
func synthMPQSized(hashEntries, blockEntries uint32, mutate func(hash, block []uint32)) []byte {
	if hashEntries < 2 || blockEntries < 2 {
		panic("synthMPQSized: hacen falta al menos las 2 entradas de los dos archivos")
	}

	cryptoLookup(0)

	const blockShift = 3 // 4 KiB sectors

	sector := 0x200 << blockShift

	plain := bytes.Repeat([]byte("plain file "), 40)

	big := bytes.Repeat([]byte("compressed sector data "), 400) // 9200 bytes: three sectors

	var body bytes.Buffer

	body.Write(make([]byte, 32)) // header goes here

	// file 0: stored as is
	pos0 := uint32(body.Len())
	body.Write(plain)

	// file 1: offset table + zlib sectors
	pos1 := uint32(body.Len())
	nsec := (len(big) + sector - 1) / sector
	offsets := make([]uint32, nsec+1)

	var sectors [][]byte

	for i := 0; i < nsec; i++ {
		end := (i + 1) * sector
		if end > len(big) {
			end = len(big)
		}

		var z bytes.Buffer

		z.WriteByte(2) // zlib

		w := zlib.NewWriter(&z)
		_, _ = w.Write(big[i*sector : end])
		_ = w.Close()

		sectors = append(sectors, z.Bytes())
	}

	offsets[0] = uint32(len(offsets) * 4)
	for i, s := range sectors {
		offsets[i+1] = offsets[i] + uint32(len(s))
	}

	_ = binary.Write(&body, binary.LittleEndian, offsets)

	for _, s := range sectors {
		body.Write(s)
	}

	compSize1 := uint32(body.Len()) - pos1

	hashOff := uint32(body.Len())
	blockOff := hashOff + hashEntries*16

	hash := make([]uint32, hashEntries*4)
	for i := range hash {
		hash[i] = 0xFFFFFFFF
	}

	// DC-002.33: cada entrada va en la ranura donde el SONdeo de su nombre la deja: eso es lo que hacen los
	// MPQ de verdad y lo que ahora usa `buscarHash`. Antes cada entrada iba en una ranura fija (0, 1, ...)
	// porque la consulta era contra un mapa que indexaba por las dos mitades del nombre y la posicion no
	// importaba: con una tabla de bytes la posicion ES la consulta.
	setHash := func(name string, block uint32) {
		slot := int(hashString(name, 0) & (hashEntries - 1))

		for i := 0; i < int(hashEntries); i++ {
			s := (slot + i) & int(hashEntries-1)
			if hash[s*4+3] != 0xFFFFFFFF { // ranura ocupada: sigue el sondeo
				continue
			}

			hash[s*4] = hashString(name, 1)
			hash[s*4+1] = hashString(name, 2)
			hash[s*4+2] = 0
			hash[s*4+3] = block

			return
		}

		panic("synthMPQSized: no queda ranura libre en la tabla de hash")
	}

	setHash("plain.txt", 0)
	setHash("big.txt", 1)

	blocks := make([]uint32, blockEntries*4)
	copy(blocks, []uint32{
		pos0, uint32(len(plain)), uint32(len(plain)), uint32(FileExists),
		pos1, compSize1, uint32(len(big)), uint32(FileExists | FileCompress),
	})

	if mutate != nil {
		mutate(hash, blocks)
	}

	encrypt(hash, hashString("(hash table)", 3))
	encrypt(blocks, hashString("(block table)", 3))

	_ = binary.Write(&body, binary.LittleEndian, hash)
	_ = binary.Write(&body, binary.LittleEndian, blocks)

	out := body.Bytes()
	copy(out, "MPQ\x1a")
	binary.LittleEndian.PutUint32(out[4:], 32)
	binary.LittleEndian.PutUint32(out[8:], uint32(len(out)))
	binary.LittleEndian.PutUint16(out[12:], 0)
	binary.LittleEndian.PutUint16(out[14:], blockShift)
	binary.LittleEndian.PutUint32(out[16:], hashOff)
	binary.LittleEndian.PutUint32(out[20:], blockOff)
	binary.LittleEndian.PutUint32(out[24:], hashEntries)
	binary.LittleEndian.PutUint32(out[28:], blockEntries)

	return out
}

// readAll reads a bounded amount of every block of the archive.
func readAll(t testing.TB, path string) {
	m, err := FromFile(path)
	if err != nil {
		return
	}
	defer m.Close()

	// DC-002.33: la tabla de bloque son BYTES y se recorre por indice; cada consulta arma la `Block` de ese
	// numero (antes se recorria el mapa de hash y se seguia el BlockIndex de cada ranura).
	for bi, entradas := uint32(0), m.blockEntries(); bi < entradas; bi++ {
		b := m.bloqueEn(bi)

		s, err := CreateStream(m, b, "file")
		if err != nil {
			continue
		}

		buf := make([]byte, 1<<16)
		for i := 0; i < 64; i++ {
			n, err := s.Read(buf, 0, uint32(len(buf)))
			if err != nil || n == 0 {
				break
			}
		}
	}

	_, _ = m.ReadFile("plain.txt")
	_, _ = m.ReadFile("big.txt")
	_, _ = m.ReadFile("(listfile)")
}

func TestSynthMPQ(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.mpq")
	if err := os.WriteFile(p, synthMPQ(nil), 0o600); err != nil {
		t.Fatal(err)
	}

	m, err := FromFile(p)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	d, err := m.ReadFile("plain.txt")
	if err != nil || !bytes.HasPrefix(d, []byte("plain file ")) || len(d) != 440 {
		t.Fatalf("plain: %v %d", err, len(d))
	}

	d, err = m.ReadFile("big.txt")
	if err != nil || len(d) != 9200 || !bytes.HasPrefix(d, []byte("compressed sector data ")) {
		t.Fatalf("big: %v %d", err, len(d))
	}
}

// TestHostileMPQ: hand-made hostile headers and tables must fail with errors, quickly, without
// allocating by their claimed sizes.
func TestHostileMPQ(t *testing.T) {
	huge := uint32(0xFFFFFFF0)

	cases := map[string]func([]byte) []byte{
		"huge hash table": func(b []byte) []byte { binary.LittleEndian.PutUint32(b[24:], huge); return b },
		"huge block table": func(b []byte) []byte {
			binary.LittleEndian.PutUint32(b[28:], 0x40000000)
			return b
		},
		"block size shift 40": func(b []byte) []byte { binary.LittleEndian.PutUint16(b[14:], 40); return b },
		"table offsets beyond file": func(b []byte) []byte {
			binary.LittleEndian.PutUint32(b[16:], huge)
			return b
		},
		"truncated":   func(b []byte) []byte { return b[:len(b)/2] },
		"header only": func(b []byte) []byte { return b[:32] },
	}

	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "h.mpq")
			if err := os.WriteFile(p, f(synthMPQ(nil)), 0o600); err != nil {
				t.Fatal(err)
			}

			readAll(t, p)
		})
	}

	// block table entries claiming huge or inconsistent sizes
	for name, mut := range map[string]func(hash, block []uint32){
		"huge size":       func(h, b []uint32) { b[2] = 0xFFFFFFFF },
		"huge compressed": func(h, b []uint32) { b[5] = 0xFFFFFFFF; b[6] = 0xFFFFFFF0 },
		"position beyond": func(h, b []uint32) { b[0] = 0xFFFFFFFF },
		"hash to block 99": func(h, b []uint32) {
			h[3] = 99
		},
		"colliding names": func(h, b []uint32) {
			h[4], h[5], h[6], h[7] = h[0], h[1], h[2], 1
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "h.mpq")
			if err := os.WriteFile(p, synthMPQ(mut), 0o600); err != nil {
				t.Fatal(err)
			}

			readAll(t, p)
		})
	}
}

// FuzzArchive opens arbitrary bytes as an MPQ and reads every file; it must never panic, hang or
// allocate by a size read from the file.
func FuzzArchive(f *testing.F) {
	f.Add(synthMPQ(nil))
	f.Add(synthMPQ(func(h, b []uint32) { b[2] = 0xFFFFFFFF }))
	f.Add(synthMPQ(func(h, b []uint32) { b[3] |= uint32(FileEncrypted | FileFixKey) }))
	f.Add(synthMPQ(func(h, b []uint32) { b[7] |= uint32(FileSingleUnit) }))
	f.Add(synthMPQ(func(h, b []uint32) { b[3] = uint32(FileExists | FileImplode) }))
	f.Add([]byte("MPQ\x1a"))
	f.Add([]byte{})

	dir := f.TempDir()

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			return
		}

		p := filepath.Join(dir, "fuzz.mpq")
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Skip()
		}

		readAll(t, p)
	})
}

// FuzzSector feeds arbitrary bytes to the sector decompressors.
func FuzzSector(f *testing.F) {
	f.Add([]byte{2, 0x78, 0x9c, 3, 0}, uint32(16))
	f.Add([]byte{8, 0, 4}, uint32(16))
	f.Add([]byte{0x40, 0, 0, 0, 0}, uint32(16))
	f.Add([]byte{0x41, 0, 0, 0, 0}, uint32(16))
	f.Add([]byte{0x81, 0, 0, 0, 0, 0, 0}, uint32(16))
	f.Add([]byte{}, uint32(0))

	f.Fuzz(func(t *testing.T, data []byte, expected uint32) {
		if expected > 1<<20 {
			expected = 1 << 20
		}

		_, _ = decompressMulti(data, expected)
		_, _ = pkDecompressHint(data, int(expected))
	})
}
