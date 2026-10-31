package d2asset

import (
	"fmt"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

// SondaDeCapas (DC-002.84): la foto de los campos de una capa .dcc cargada, para poder comparar la misma
// capa ANTES y DESPUES de una recoleccion del GC.
//
// POR QUE. En la consola el primer cuadro que pinta el `Composite` muere (salto a basura) y se midio que
// pasa sin ninguna recoleccion EN EL CUADRO: la corrupcion ya estaba puesta por la recoleccion forzada que
// el POC hace justo antes. Esta sonda imprime las palabras que el GC tendria que poner al dia -- los dos
// punteros de las clausuras (`onBindRenderer`, `ensureDirection`), el `dcc`, el `palette`, el `renderer` y
// la tira `directions` -- para ver CUAL de ellas cambia de valor cuando el objeto se copia.
//
// Se lee con `unsafe` porque son campos sin exportar: es un instrumento de medicion, no codigo del motor,
// y no cambia nada de lo que hace `Render`.
func SondaDeCapas(capas []d2interface.Animation) string {
	salida := ""

	for i, capa := range capas {
		dcc, ok := capa.(*DCCAnimation)
		if !ok || dcc == nil {
			salida += fmt.Sprintf(" capa=%d noEsDCC", i)

			continue
		}

		a := &dcc.Animation

		clausura := func(f *func(d2interface.Renderer) error) [2]uintptr {
			return *(*[2]uintptr)(unsafe.Pointer(f))
		}

		asegurar := *(*[2]uintptr)(unsafe.Pointer(&a.ensureDirection))
		enlazar := clausura(&a.onBindRenderer)

		salida += fmt.Sprintf(" capa=%d dcc=%d pal=%d rend=%d enlaza=%d/%d asegura=%d/%d dirs=%d/%d/%d dir=%d",
			i, int(uintptr(unsafe.Pointer(dcc.dcc))), int(uintptr(unsafe.Pointer(&dcc.palette))),
			int(uintptr(unsafe.Pointer(&a.renderer))),
			int(enlazar[0]), int(enlazar[1]), int(asegurar[0]), int(asegurar[1]),
			int(uintptr(unsafe.Pointer(&a.directions[0]))), len(a.directions), cap(a.directions), a.directionIndex)
	}

	return salida
}

// Capas devuelve las capas cargadas del modo actual (para el instrumento de DC-002.84: son los objetos que
// la recoleccion del GC tiene que reubicar cuando se pinta el primer cuadro).
func (c *Composite) Capas() []d2interface.Animation {
	if c.mode == nil {
		return nil
	}

	return c.mode.layers
}
