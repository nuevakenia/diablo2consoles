// Package paq1 existe para la sonda de DC-002.2f: define tipos con metodos para que los descriptores
// de tipo de esos metodos se generen EN ESTE PAQUETE, mientras las interfaces se declaran en main.
//
// Es el caso que hay que medir: si el descriptor del tipo del metodo (`func() error`) que genera este
// paquete es el MISMO puntero que el del metodo que pide la interfaz declarada en otro paquete. Si no
// lo fuera, comparar la firma por igualdad de puntero romperia todas las aserciones entre paquetes,
// que son las que hoy sostienen los 589 casos que pasan en la consola.
package paq1

// ConUno tiene la firma que la interfaz pide.
type ConUno struct{ msg string }

func (c *ConUno) Error() string { return c.msg }

// Unwrap devuelve un error solo (la firma de la interfaz buena).
func (c *ConUno) Unwrap() error { return nil }

// ConMuchos tiene el MISMO nombre de metodo con OTRA firma: es el defecto de implements_interface.
type ConMuchos struct{}

func (c *ConMuchos) Error() string { return "muchos" }

// Unwrap devuelve un arreglo de errores (firma distinta de la de la interfaz).
func (c *ConMuchos) Unwrap() []error { return nil }

// SinNada no tiene ningun metodo: no puede implementar nada no vacio.
type SinNada struct{}

// Valor() existe para que *SinNada no sea un tipo sin metodos y el compilador no lo colapse.
func (s *SinNada) Valor() int { return 7 }
