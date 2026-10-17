// Prueba del Sscanf de la consola contra el de la biblioteca estandar de verdad.
//
// El motor lo usa en dos lugares, y los dos formatos estan aca ("place_group%d" y "%d"), mas variaciones
// para cubrir el resto de lo que soporta: literales, varios verbos, ancho, comodin, %x, %c.
package difftest

import (
	gofmt "fmt"
	"testing"

	"d2std/fmt"
)

func TestSscanfContraGo(t *testing.T) {
	casos := []struct {
		texto   string
		formato string
		// los tipos de los destinos, para armar la misma cantidad en los dos lados
		tipos []string
	}{
		// los dos del motor
		{"place_group42", "place_group%d", []string{"int"}},
		{"17", "%d", []string{"int"}},
		// literales y varios verbos
		{"x=12 y=34", "x=%d y=%d", []string{"int", "int"}},
		{"hola mundo", "%s %s", []string{"string", "string"}},
		{"3.5 2.25", "%f %f", []string{"float64", "float64"}},
		{"ff 10", "%x %x", []string{"int", "int"}},
		// el ancho y el comodin
		{"12345", "%2d", []string{"int"}},
		// el comodin NO es de go (es de C): los dos lados tienen que fallar igual
		{"99 42", "%*d %d", []string{"int"}},
		// espacios de mas en el texto y en el formato
		{"   7   ", " %d ", []string{"int"}},
		{"negativo: -15", "negativo: %d", []string{"int"}},
	}

	for i, c := range casos {
		// lado consola
		nuestros := armarDestinos(c.tipos)
		nNuestro, eNuestro := fmt.Sscanf(c.texto, c.formato, nuestros...)

		// lado go
		deGo := armarDestinos(c.tipos)
		nGo, eGo := gofmt.Sscanf(c.texto, c.formato, deGo...)

		if nNuestro != nGo {
			t.Errorf("caso %d (%q con %q): leidos nuestro=%d go=%d", i, c.texto, c.formato, nNuestro, nGo)
		}
		if (eNuestro == nil) != (eGo == nil) {
			t.Errorf("caso %d (%q con %q): error nuestro=%v go=%v", i, c.texto, c.formato, eNuestro, eGo)
		}
		if nNuestro == nGo && eNuestro == nil {
			for j := range nuestros {
				a, b := valorApuntadoSscanf(nuestros[j]), valorApuntadoSscanf(deGo[j])
				if a != b {
					t.Errorf("caso %d (%q con %q) valor %d: nuestro=%v go=%v",
						i, c.texto, c.formato, j, a, b)
				}
			}
		}
	}
}

// armarDestinos: un valor por tipo, para pasarle a los dos Sscanf.
func armarDestinos(tipos []string) []interface{} {
	salida := make([]interface{}, len(tipos))
	for i, tp := range tipos {
		switch tp {
		case "int":
			v := new(int)
			salida[i] = v
		case "float64":
			v := new(float64)
			salida[i] = v
		case "string":
			v := new(string)
			salida[i] = v
		}
	}
	return salida
}

// valorDe: el valor apuntado, para poder comparar los destinos de los dos lados.
func valorApuntadoSscanf(p interface{}) interface{} {
	switch v := p.(type) {
	case *int:
		return *v
	case *float64:
		return *v
	case *string:
		return *v
	}
	return nil
}
