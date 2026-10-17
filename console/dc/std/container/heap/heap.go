// Package heap: la cola de prioridad de la biblioteca estandar, para la consola.
//
// Es el mismo algoritmo que el de Go (monticulo binario sobre Interface) y se puede probar contra el
// de verdad en el host: la interfaz es identica, asi que es el MISMO codigo corriendo en los dos
// lados con el mismo resultado.
//
// Límite declarado: es el algoritmo, no una copia; el orden de los elementos IGUALES dentro del
// monticulo no tiene por que coincidir con el de Go (el de Go tampoco lo garantiza).
package heap

import "d2std/sort"

// Interface es la interfaz de Go, igual: el tipo que se guarda tiene que saber ordenarse.
type Interface interface {
	sort.Interface
	Push(x interface{}) // agrega x como elemento len()
	Pop() interface{}   // saca y devuelve el ultimo elemento
}

// Init establece las invariantes del monticulo (se llama una vez, antes de usar el monticulo).
func Init(h Interface) {
	n := h.Len()
	for i := n/2 - 1; i >= 0; i-- {
		down(h, i, n)
	}
}

// Push mete el elemento x en el monticulo.
func Push(h Interface, x interface{}) {
	h.Push(x)
	up(h, h.Len()-1)
}

// Pop saca el elemento minimo (segun Less) y lo devuelve.
func Pop(h Interface) interface{} {
	n := h.Len() - 1
	h.Swap(0, n)
	down(h, 0, n)
	return h.Pop()
}

// Remove saca el elemento i.
func Remove(h Interface, i int) interface{} {
	n := h.Len() - 1
	if n != i {
		h.Swap(i, n)
		if !down(h, i, n) {
			up(h, i)
		}
	}
	return h.Pop()
}

// Fix reacomoda el monticulo cuando el elemento i cambio de valor.
func Fix(h Interface, i int) {
	if !down(h, i, h.Len()) {
		up(h, i)
	}
}

func up(h Interface, j int) {
	for {
		i := (j - 1) / 2 // padre
		if i == j || !h.Less(j, i) {
			break
		}
		h.Swap(i, j)
		j = i
	}
}

func down(h Interface, i0, n int) bool {
	i := i0
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 { // j1 < 0 despues del desborde
			break
		}
		j := j1 // hijo izquierdo
		if j2 := j1 + 1; j2 < n && h.Less(j2, j1) {
			j = j2 // = hijo derecho
		}
		if !h.Less(j, i) {
			break
		}
		h.Swap(i, j)
		i = j
	}
	return i > i0
}
