// Package akara: la parte que el motor usa de la libreria akara (gravestench/akara), reescrita para la
// consola.
//
// Medido: de akara, el motor solo usa el BitSet (d2core/d2records/monster_unique_affix_*), y en DOS
// lugares (uno de ellos un test). El resto de akara es un motor de componentes (ECS) que el motor de
// Diablo II no usa: lo unico que entra al binario de la consola es el BitSet, asi que no tiene sentido
// portar la libreria entera.
//
// Se ve igual desde afuera (akara.NewBitSet(), *akara.BitSet), asi que el codigo del motor no cambia.
package akara

// BitSet es el conjunto de bits de akara: un arreglo de palabras de 64 bits, con las mismas operaciones
// que usa el motor (poner, sacar, preguntar, contar).
type BitSet struct {
	palabras []uint64
}

// NewBitSet arma un conjunto vacio, como el de akara.
func NewBitSet() *BitSet {
	return &BitSet{}
}

// Set prende el bit i y devuelve true si cambio (si ya estaba prendido, false).
func (b *BitSet) Set(i uint32) bool {
	p := int(i / 64)
	for len(b.palabras) <= p {
		b.palabras = append(b.palabras, 0)
	}
	mascara := uint64(1) << (i % 64)
	if b.palabras[p]&mascara != 0 {
		return false
	}
	b.palabras[p] |= mascara
	return true
}

// Clear apaga el bit i.
func (b *BitSet) Clear(i uint32) bool {
	p := int(i / 64)
	if p >= len(b.palabras) {
		return false
	}
	mascara := uint64(1) << (i % 64)
	if b.palabras[p]&mascara == 0 {
		return false
	}
	b.palabras[p] &^= mascara
	return true
}

// IsSet: si el bit i esta prendido.
func (b *BitSet) IsSet(i uint32) bool {
	p := int(i / 64)
	if p >= len(b.palabras) {
		return false
	}
	return b.palabras[p]&(uint64(1)<<(i%64)) != 0
}

// Len: la cantidad de bits que entran.
func (b *BitSet) Len() int { return len(b.palabras) * 64 }

// Count: cuantos bits estan prendidos.
func (b *BitSet) Count() int {
	total := 0
	for _, p := range b.palabras {
		v := p
		for v != 0 {
			total++
			v &= v - 1
		}
	}
	return total
}

// Reset: la deja vacia.
func (b *BitSet) Reset() { b.palabras = b.palabras[:0] }
