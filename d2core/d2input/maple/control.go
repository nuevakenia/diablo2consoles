//go:build dreamcast

package maple

import (
	"unsafe"
)

// ============================ EL LECTOR DEL MAPLE (solo la consola) ============================
//
// Este archivo es el UNICO que sabe como se lee un control en la Dreamcast. El resto del paquete (la tabla y
// los nueve metodos) es Go comun: compila y se prueba en el host, contra el mismo estado que produce este
// lector.
//
// LAS DOS FUNCIONES DE C son las de console/dc/std/csrc/control_dc.c -- las mismas que usa el shim de
// Ebitengine de la consola desde DC-002.11, no unas nuevas. Ese .c lo compila build-dc.sh SIEMPRE (los
// shims de la stdlib se enlazan con cualquier POC), asi que un POC que importe este paquete ya tiene el
// lector sin tocar nada.
//
// EL PUNTERO CRUZA COMO uintptr y no como unsafe.Pointer: un puntero de Go pasado a un //extern tiene el ABI
// ambiguo (medido en DC-002.9/22), y ademas el runtime de la consola no tiene el descriptor del tipo
// unsafe.Pointer, asi que guardarlo en un campo o capturarlo en una clausura rompe el enlace. Se convierte a
// puntero solo en la llamada.

//extern d2c_control_buscar
func d2cControlBuscar() int

//extern d2c_control_leer
func d2cControlLeer(botones, palancaX, palancaY, gatilloL, gatilloR, palanca2X, palanca2Y uintptr) int

// buscado/hay: el control se busca UNA vez (maple_enum_type recorre el bus) y se recuerda. Que el aparato
// siga enchufado lo dice el propio .c en cada lectura: si se desconecta, d2c_control_leer devuelve -1.
var (
	buscado bool
	hay     bool
)

// HayControl: si la consola encontro un control en el puerto 0. Se pregunta una sola vez.
func HayControl() bool {
	if !buscado {
		hay = d2cControlBuscar() != 0
		buscado = true
	}

	return hay
}

// ActualizarDesdeElControl: lee el control AHORA y deja el cuadro traducido en el servicio. Se llama una vez
// por cuadro, antes de que el motor pregunte nada.
//
// SI NO HAY CONTROL NO ES UN ERROR: el estado queda en "nada apretado" y el servicio sigue contestando (el
// juego tiene que poder mostrar su pantalla sin control, igual que un juego de PC sin mouse). Devolver false
// es solo para que el que llama lo pueda informar; el motor no se entera.
func (s *Servicio) ActualizarDesdeElControl() bool {
	if !HayControl() {
		s.Actualizar(0, 0, 0, 0, 0)

		return false
	}

	var (
		botones              uint32
		palancaX, palancaY   int32
		gatilloL, gatilloR   int32
		palanca2X, palanca2Y int32
	)

	if d2cControlLeer(
		uintptr(unsafe.Pointer(&botones)),
		uintptr(unsafe.Pointer(&palancaX)), uintptr(unsafe.Pointer(&palancaY)),
		uintptr(unsafe.Pointer(&gatilloL)), uintptr(unsafe.Pointer(&gatilloR)),
		uintptr(unsafe.Pointer(&palanca2X)), uintptr(unsafe.Pointer(&palanca2Y)),
	) != 0 {
		// el aparato estaba y ya no esta (o el estado no estaba listo): el cuadro es "nada apretado", no el
		// cuadro anterior. Si se dejara el estado viejo, una tecla quedaria apretada para siempre.
		s.Actualizar(0, 0, 0, 0, 0)

		return false
	}

	s.Actualizar(botones, palancaX, palancaY, gatilloL, gatilloR)

	return true
}
