package d2mpq

import "io"

// Hash represents a hashed file entry in the MPQ file
type Hash struct { // 16 bytes
	A          uint32
	B          uint32
	Locale     uint16
	Platform   uint16
	BlockIndex uint32
}

// Name64 returns part A and B as uint64
func (h *Hash) Name64() uint64 {
	return uint64(h.A)<<32 | uint64(h.B)
}

//nolint:gomnd // number
func (mpq *MPQ) readHashTable() error {
	if _, err := mpq.file.Seek(int64(mpq.header.HashTableOffset), io.SeekStart); err != nil {
		return err
	}

	mpq.hashes = make(map[uint64]*Hash)

	// DC-002.28: la tabla se descifra POR TROZOS y cada trozo va derecho al mapa, sin materializar la
	// tabla entera. Antes se pedia la tabla completa (dos reservas contiguas de `entradas*16` bytes) y en
	// la Dreamcast eso no entra: la de hash de d2data.mpq son 1.048.576 bytes y el monton de C de la
	// consola queda en pocos cientos de bytes libres (medido: el motor muere en `gc_external_alloc: OOM`
	// al abrir ese archivo). El unico que crece con el tamano de la tabla es el mapa, que es el
	// instrumento del motor (una entrada por nombre).
	return decryptTablePorTrozos(mpq.file, mpq.header.HashTableEntries, "(hash table)",
		func(trozo []uint32) error {
			for n := 0; n+4 <= len(trozo); n += 4 {
				e := &Hash{
					A: trozo[n],
					B: trozo[n+1],
					// https://github.com/OpenDiablo2/OpenDiablo2/issues/812
					Locale:     uint16(trozo[n+2] >> 16),    //nolint:gomnd // binary data
					Platform:   uint16(trozo[n+2] & 0xFFFF), //nolint:gomnd // binary data
					BlockIndex: trozo[n+3],
				}
				mpq.hashes[e.Name64()] = e
			}

			return nil
		})
}
