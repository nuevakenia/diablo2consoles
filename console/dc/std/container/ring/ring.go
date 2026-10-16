// Paquete ring: la lista circular, como la de Go (lo que el motor usa: New y Ring).
//
// Por que existe: en sh-elf no hay biblioteca estandar de Go; el motor guarda historiales
// circulares (1 archivo directo).
package ring

// Ring es una lista circular doblemente enlazada.
type Ring struct {
	next, prev *Ring
	Value      interface{}
}

// New crea un anillo de n elementos (todos con Value nil).
func New(n int) *Ring {
	if n <= 0 {
		return nil
	}
	r := new(Ring)
	p := r
	for i := 1; i < n; i++ {
		p.next = &Ring{prev: p}
		p = p.next
	}
	p.next = r
	r.prev = p
	return r
}

// Next devuelve el siguiente elemento.
func (r *Ring) Next() *Ring {
	if r.next == nil {
		return r.init()
	}
	return r.next
}

// Prev devuelve el anterior.
func (r *Ring) Prev() *Ring {
	if r.prev == nil {
		return r.init()
	}
	return r.prev
}

// Move adelanta n posiciones (atras si n es negativo).
func (r *Ring) Move(n int) *Ring {
	if r.next == nil {
		return r.init()
	}
	switch {
	case n < 0:
		for ; n < 0; n++ {
			r = r.prev
		}
	case n > 0:
		for ; n > 0; n-- {
			r = r.next
		}
	}
	return r
}

func (r *Ring) init() *Ring {
	r.next = r
	r.prev = r
	return r
}

// Link une r con s (el anillo que sigue a s) y devuelve el elemento siguiente al ultimo de r.
func (r *Ring) Link(s *Ring) *Ring {
	n := r.Next()
	if s != nil {
		p := s.Prev()
		r.next = s
		s.prev = r
		n.prev = p
		p.next = n
	}
	return n
}

// Unlink saca n elementos del anillo y devuelve el que quedo fuera.
func (r *Ring) Unlink(n int) *Ring {
	if n <= 0 {
		return nil
	}
	return r.Link(r.Move(n + 1))
}

// Len devuelve cuantos elementos tiene el anillo.
func (r *Ring) Len() int {
	n := 0
	if r != nil {
		n = 1
		for p := r.Next(); p != r; p = p.next {
			n++
		}
	}
	return n
}

// Do llama a f con cada elemento del anillo, en orden.
func (r *Ring) Do(f func(interface{})) {
	if r != nil {
		f(r.Value)
		for p := r.Next(); p != r; p = p.next {
			f(p.Value)
		}
	}
}
