// Package otto: el motor de JavaScript de github.com/robertkrimen/otto, para la consola.
//
// PARA QUE SE USA: d2script (el scripting del motor: d2script/engine.go) y d2networking/d2server (las
// ordenes del servidor). Otto es un interprete de JavaScript escrito en Go puro.
//
// POR QUE ES UN STUB Y NO OTTO: porque esto es JavaScript, y nada del juego DE UN JUGADOR pasa por aca. El
// scripting es la parte que corre codigo que alguien mas escribio (mods, ordenes del servidor de la red);
// el juego de un jugador entero -- menus, personajes, mapas, pelear -- no lo usa ni una vez. Traer un
// interprete de JavaScript a la consola seria pagar el tamano (miles de lineas), la memoria y la
// verificacion de un lenguaje entero para una parte que este puerto no toca.
//
// QUE PASA SI ALGUIEN LO USA: no se cae ni miente. `New` devuelve un interprete vacio, correr un script
// devuelve el error con el motivo, y las funciones que se registran quedan guardadas (asi el que registra
// "debugPrint" no se lleva una sorpresa). Todo lo que devuelve un valor devuelve el "no hay valor" de
// JavaScript, que es lo unico honesto sin interprete.
//
// SI ALGUN DIA HACE FALTA: el camino no es escribir JavaScript a mano -- es compilar otto tal cual para
// SH-4 (es Go puro, entraria por el mismo camino que los demas paquetes del motor). Se hace el dia que se
// mida que el scripting hace falta, no antes.
package otto

import (
	"errors"
	"fmt"
	"sort"
)

// ErrSinInterprete: el motivo, para que el error lo diga.
var ErrSinInterprete = errors.New("otto: en la consola no hay interprete de JavaScript (el scripting no es parte del juego de un jugador)")

// Value: un valor de JavaScript. Aca no hay valores (no hay interprete), asi que es un valor vacio.
type Value struct {
	valor interface{}
	// vacio: si esto es el "no hay valor" de JavaScript (undefined).
	vacio bool
}

// UndefinedValue: el "no hay valor" de JavaScript.
func UndefinedValue() Value { return Value{vacio: true} }

// NullValue: el "nulo" de JavaScript.
func NullValue() Value { return Value{vacio: true} }

// String: el valor en texto ("undefined" si no hay valor, como en JavaScript).
func (v Value) String() string {
	if v.vacio || v.valor == nil {
		return "undefined"
	}
	return fmt.Sprintf("%v", v.valor)
}

// ToInteger: el valor como entero.
func (v Value) ToInteger() int64 { return 0 }

// ToFloat: el valor como decimal.
func (v Value) ToFloat() float64 { return 0 }

// ToBoolean: el valor como si/no (en JavaScript todo lo vacio es falso).
func (v Value) ToBoolean() bool { return false }

// IsUndefined: si es el "no hay valor".
func (v Value) IsUndefined() bool { return v.vacio }

// IsNull: si es el "nulo".
func (v Value) IsNull() bool { return v.vacio }

// Export: devuelve el valor como Go.
func (v Value) Export() interface{} { return v.valor }

// FunctionCall: los datos de una llamada a una funcion de Go desde JavaScript.
type FunctionCall struct {
	ArgumentList []Value
	Otto         *Otto
}

// Argument: el argumento numero i de la llamada.
func (c FunctionCall) Argument(i int) Value {
	if i < 0 || i >= len(c.ArgumentList) {
		return UndefinedValue()
	}
	return c.ArgumentList[i]
}

// Otto: el interprete. Aca es una caja vacia: guarda las funciones que se registran para que el que las
// registra vea que quedaron guardadas.
type Otto struct {
	funciones map[string]interface{}
}

// New: arma un interprete.
func New() *Otto {
	return &Otto{funciones: make(map[string]interface{})}
}

// Set: registra una funcion o un valor con un nombre. El error es nil porque REGISTRAR no es correr: el
// registro anda igual, lo que no hay es quien lo llame.
func (o *Otto) Set(nombre string, valor interface{}) error {
	if o.funciones == nil {
		o.funciones = make(map[string]interface{})
	}
	o.funciones[nombre] = valor
	return nil
}

// Get: lee un valor registrado.
func (o *Otto) Get(nombre string) Value {
	if o.funciones == nil {
		return UndefinedValue()
	}
	return Value{valor: o.funciones[nombre]}
}

// ToValue: pasa un valor de Go a un valor de JavaScript.
func (o *Otto) ToValue(fuente interface{}) (Value, error) {
	return Value{valor: fuente}, nil
}

// Run: corre un script. No se puede (ver el comentario del paquete).
func (o *Otto) Run(script string) (Value, error) {
	return UndefinedValue(), ErrSinInterprete
}

// Eval: lo mismo, con una expresion suelta.
func (o *Otto) Eval(codigo string) (Value, error) {
	return UndefinedValue(), ErrSinInterprete
}

// Call: llamar a una funcion registrada.
func (o *Otto) Call(nombre string, argumentos ...interface{}) (Value, error) {
	return UndefinedValue(), ErrSinInterprete
}

// NombresRegistrados: que se registro (ordenado, para que la evidencia sea la misma en cada corrida). Lo
// usa el POC para mostrar que el registro quedo.
func (o *Otto) NombresRegistrados() []string {
	var nombres []string
	for n := range o.funciones {
		nombres = append(nombres, n)
	}
	sort.Strings(nombres)
	return nombres
}
