// Paquete de prueba para el minimo caso de dos paquetes: aca vive el tipo, en main la asercion.
package saludo

// Texto cumple la interfaz error sin importar nada.
type Texto struct{ S string }

func (t Texto) Error() string { return t.S }

// ComoError devuelve el tipo concreto como la interfaz error.
func ComoError(s string) error { return Texto{s} }

// Desenvuelto devuelve un puntero, para probar tambien el caso con puntero.
func Desenvuelto(s string) error { return &Texto{s} }

// Nombre es un metodo comun, sin relacion con error.
func (t Texto) Nombre() string { return t.S }
