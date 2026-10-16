// Paquete fnv: el hash FNV-1a, como el de Go.
//
// Por que existe: el motor hashea claves y rutas (9 archivos usan New32a / New64a; adentro del
// motor se usa para los indices del MPQ y las tablas de textos).
//
// El algoritmo es publico y no tiene datos generados: con los mismos primos da exactamente lo
// mismo que Go, y el corpus lo compara contra la implementacion real de Go con las cadenas que
// usa el motor. (El hash de 32 bits se uso tambien en DC-002.3 para resumir los indices de un DC6,
// escrito a mano en el comparador; de aca en adelante sale de este paquete.)
package fnv

import "d2std/hash"

const (
	offset32 = 2166136261
	prime32  = 16777619
	offset64 = 14695981039346656037
	prime64  = 1099511628211
)

type sum32a uint32

// New32a devuelve un hash FNV-1a de 32 bits.
func New32a() hash.Hash32 {
	s := sum32a(offset32)
	return &s
}

func (s *sum32a) Write(p []byte) (int, error) {
	h := *s
	for _, b := range p {
		h ^= sum32a(b)
		h *= prime32
	}
	*s = h
	return len(p), nil
}

func (s *sum32a) Sum(b []byte) []byte {
	v := uint32(*s)
	return append(b, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func (s *sum32a) Reset()         { *s = offset32 }
func (s *sum32a) Size() int      { return 4 }
func (s *sum32a) BlockSize() int { return 1 }
func (s *sum32a) Sum32() uint32  { return uint32(*s) }

type sum64a uint64

// New64a devuelve un hash FNV-1a de 64 bits.
func New64a() hash.Hash64 {
	s := sum64a(offset64)
	return &s
}

func (s *sum64a) Write(p []byte) (int, error) {
	h := *s
	for _, b := range p {
		h ^= sum64a(b)
		h *= prime64
	}
	*s = h
	return len(p), nil
}

func (s *sum64a) Sum(b []byte) []byte {
	v := uint64(*s)
	return append(b, byte(v>>56), byte(v>>48), byte(v>>40), byte(v>>32),
		byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func (s *sum64a) Reset()         { *s = offset64 }
func (s *sum64a) Size() int      { return 8 }
func (s *sum64a) BlockSize() int { return 1 }
func (s *sum64a) Sum64() uint64  { return uint64(*s) }
