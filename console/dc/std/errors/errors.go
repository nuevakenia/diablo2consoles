// Package errors: lo minimo que usa el motor (errors.New 375 veces, y el envoltorio de %w).
//
// Diferencias con la de Go, a proposito y documentadas:
//   - Is compara con == sin comprobar antes si el tipo es comparable (eso en Go necesita
//     reflect). Con tipos de error que contienen un slice, la comparacion paniquea igual que en
//     Go; con los que no, da el mismo resultado.
//   - As NO esta: asignar a un puntero cualquiera necesita reflect, que no existe en la consola.
//     El motor lo usa? ver scripts/console/audit-stdlib-uso.py. Si aparece, en la consola ese
//     camino se reescribe con un type switch.
package errors

// New devuelve un error con el texto dado.
func New(text string) error {
	return &errorString{text}
}

type errorString struct {
	s string
}

func (e *errorString) Error() string { return e.s }

// Unwrap devuelve el error envuelto por err, o nil.
func Unwrap(err error) error {
	u, ok := err.(interface{ Unwrap() error })
	if !ok {
		return nil
	}
	return u.Unwrap()
}

// Is informa si algun error de la cadena de err es target.
func Is(err, target error) bool {
	if target == nil {
		return err == target
	}
	for {
		if err == target {
			return true
		}
		if x, ok := err.(interface{ Is(error) bool }); ok && x.Is(target) {
			return true
		}
		switch x := err.(type) {
		case interface{ Unwrap() error }:
			err = x.Unwrap()
			if err == nil {
				return false
			}
		case interface{ Unwrap() []error }:
			for _, e := range x.Unwrap() {
				if Is(e, target) {
					return true
				}
			}
			return false
		default:
			return false
		}
	}
}

// Join envuelve varios errores en uno solo (lo que hace Go desde 1.20).
func Join(errs ...error) error {
	n := 0
	for _, err := range errs {
		if err != nil {
			n++
		}
	}
	if n == 0 {
		return nil
	}
	e := &joinError{errs: make([]error, 0, n)}
	for _, err := range errs {
		if err != nil {
			e.errs = append(e.errs, err)
		}
	}
	return e
}

type joinError struct {
	errs []error
}

func (e *joinError) Error() string {
	var b []byte
	for i, err := range e.errs {
		if i > 0 {
			b = append(b, '\n')
		}
		b = append(b, err.Error()...)
	}
	return string(b)
}

func (e *joinError) Unwrap() []error { return e.errs }
