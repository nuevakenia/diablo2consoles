// Package context: la senal de "cortala" que atraviesa una cadena de llamadas.
//
// PARA QUE SE USA: d2networking lo usa para poder terminar el servidor y los clientes cuando se cierra el
// juego (context.WithCancel + Background, y el Context que va de mano en mano). Tambien lo usa el motor
// para las esperas con limite de tiempo.
//
// ESTE ES DE VERDAD: no es un stub. Un Context es un canal que se cierra y una causa que se guarda, y eso
// la consola lo puede hacer (ya tiene canales y goroutines probadas). Que sea de verdad importa: si
// WithCancel devolviera algo que nunca se cierra, el servidor no se apagaria nunca y no se veria como un
// error hasta que hubiera que salir del juego.
//
// LO QUE NO HAY: el limite por tiempo (WithTimeout/WithDeadline) no esta porque nadie lo pide; si aparece,
// se escribe con time.After (que la consola ya tiene).
package context

import (
	"d2std/errors"
	"d2std/sync"
	"d2std/time"
)

// Canceled: el motivo que devuelve Err() cuando alguien llamo al cancelar.
var Canceled = errors.New("context canceled")

// DeadlineExceeded: lo mismo para el limite por tiempo (aca no se usa todavia, pero el motor lo nombra).
var DeadlineExceeded = errors.New("context deadline exceeded")

// Context: la senal. La forma es la de Go, tal cual.
type Context interface {
	// Deadline: cuando vence, si tiene limite. En la consola nunca hay limite.
	Deadline() (time.Time, bool)
	// Done: el canal que se CIERRA cuando hay que cortar.
	Done() <-chan struct{}
	// Err: por que se corto (nil si sigue andando).
	Err() error
	// Value: los datos que viajan con el contexto (una clave por dato).
	Value(clave any) any
}

// vacio: el contexto que no se puede cancelar (Background y TODO). Es el Context de verdad mas simple:
// un canal que nunca se cierra.
type vacio struct{}

func (vacio) Deadline() (time.Time, bool) { return time.Time{}, false }
func (vacio) Done() <-chan struct{}       { return nil }
func (vacio) Err() error                  { return nil }
func (vacio) Value(clave any) any         { return nil }
func (vacio) String() string              { return "context.Background" }

// el unico contexto de fondo (no hace falta mas de uno)
var fondo = vacio{}

// Background: el contexto de arriba de todo (del que cuelgan los demas).
func Background() Context { return fondo }

// TODO: igual que Background, para cuando todavia no se sabe cual va (Go lo tiene con ese nombre).
func TODO() Context { return fondo }

// CancelFunc: lo que devuelve WithCancel. Llamarlo corta el contexto y los que cuelgan de el.
type CancelFunc func()

// cancelable: un contexto que se puede cortar.
type cancelable struct {
	Context          // el de arriba (para heredar Value y Deadline)
	hecho   chan struct{}
	unaVez  sync.Once
	error   error
	cerrado bool
}

// Done: el canal que se cierra al cancelar.
func (c *cancelable) Done() <-chan struct{} { return c.hecho }

// Err: el motivo del corte (nil mientras anda). OJO: no se puede leer el campo sin candado en una consola
// con goroutines, pero se lee DESPUES de que el canal esta cerrado, y ahi el valor ya no cambia (el cierre
// del canal es la barrera). Es el mismo razonamiento que usa Go.
func (c *cancelable) Err() error {
	select {
	case <-c.hecho:
		return c.error
	default:
		return nil
	}
}

// Value: pasa la pregunta al contexto de arriba (aca no se guardan datos).
func (c *cancelable) Value(clave any) any { return c.Context.Value(clave) }

// Deadline: lo mismo.
func (c *cancelable) Deadline() (time.Time, bool) { return c.Context.Deadline() }

// WithCancel: un contexto que se corta cuando se llame a la funcion que devuelve.
//
// Devuelve las dos cosas, como Go: el contexto y el boton. Si el de arriba ya estaba cortado, este nace
// cortado (esa parte importa: los que cuelgan de un servidor apagado no pueden quedar vivos).
func WithCancel(arriba Context) (Context, CancelFunc) {
	if arriba == nil {
		panic("context: WithCancel(nil)")
	}

	c := &cancelable{Context: arriba, hecho: make(chan struct{})}

	cancelar := func() {
		c.unaVez.Do(func() {
			c.error = Canceled
			close(c.hecho)
		})
	}

	// si el de arriba ya se corto, este tambien
	if arriba.Done() != nil {
		select {
		case <-arriba.Done():
			cancelar()
		default:
		}
	}

	return c, cancelar
}

// WithValue: un contexto que lleva un dato. Se usa poco en el motor, pero es parte de la forma de Go y
// cuesta cuatro lineas.
func WithValue(arriba Context, clave, valor any) Context {
	if clave == nil {
		panic("context: WithValue con clave nil")
	}
	return &conValor{Context: arriba, clave: clave, valor: valor}
}

// conValor: el contexto con un dato adentro.
type conValor struct {
	Context
	clave any
	valor any
}

func (c *conValor) Value(clave any) any {
	if c.clave == clave {
		return c.valor
	}
	return c.Context.Value(clave)
}
