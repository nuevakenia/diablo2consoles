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

// NewBitSet arma un conjunto. En akara recibe los indices que van prendidos desde el arranque
// (NewBitSet(indicesToSetTrue ...int)): se respeta esa firma, porque el motor la usa asi.
func NewBitSet(indicesToSetTrue ...int) *BitSet {
	b := &BitSet{}
	for _, i := range indicesToSetTrue {
		b.Set(i, true)
	}
	return b
}

// Set prende o apaga el bit idx (la firma de akara: Set(idx int, b bool)).
func (b *BitSet) Set(idx int, v bool) {
	if idx < 0 {
		return
	}
	p := idx / 64
	for len(b.palabras) <= p {
		b.palabras = append(b.palabras, 0)
	}
	mascara := uint64(1) << uint(idx%64)
	if v {
		b.palabras[p] |= mascara
	} else {
		b.palabras[p] &^= mascara
	}
}

// Get: si el bit idx esta prendido (asi se llama en akara).
func (b *BitSet) Get(idx int) bool {
	if idx < 0 {
		return false
	}
	p := idx / 64
	if p >= len(b.palabras) {
		return false
	}
	return b.palabras[p]&(uint64(1)<<uint(idx%64)) != 0
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

// Clear: la deja vacia (la firma de akara, sin argumentos).
func (b *BitSet) Clear() { b.palabras = b.palabras[:0] }

// Flip: da vuelta el bit idx.
func (b *BitSet) Flip(idx int) {
	b.Set(idx, !b.Get(idx))
}

// Clone: una copia independiente.
func (b *BitSet) Clone() *BitSet {
	nuevo := &BitSet{palabras: make([]uint64, len(b.palabras))}
	copy(nuevo.palabras, b.palabras)
	return nuevo
}
