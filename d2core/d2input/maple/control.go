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

// DC-002.114: las mismas dos cosas, pero de UN PUERTO del bus (el jugador 2 va en el puerto 1). `maple_enum_dev`
// es la direccion fisica del bus; `maple_enum_type` es "el n-esimo aparato de ese tipo", que no es lo mismo.
//
//extern d2c_control_buscar_puerto
func d2cControlBuscarPuerto(puerto uint) int

//extern d2c_control_leer_puerto
func d2cControlLeerPuerto(puerto uint,
	botones, palancaX, palancaY, gatilloL, gatilloR, palanca2X, palanca2Y uintptr) int

//extern d2c_control_aparato
func d2cControlAparato(puerto uint) int

// puertosDelMaple: los cuatro puertos del bus (A, B, C, D). El arreglo de abajo es del mismo tamano.
const puertosDelMaple = 4

// buscado/hay: el control se busca UNA vez (maple_enum_type recorre el bus) y se recuerda. Que el aparato
// siga enchufado lo dice el propio .c en cada lectura: si se desconecta, d2c_control_leer devuelve -1.
var (
	buscado bool
	hay     bool

	// DC-002.114: lo mismo, puerto por puerto.
	buscadoPuerto [puertosDelMaple]bool
	hayPuerto     [puertosDelMaple]bool
)

// HayControl: si la consola encontro un control en el puerto 0. Se pregunta una sola vez... salvo que el estado lo
// este poniendo el shim: en ese caso hay estado aunque no haya aparato, y el .c lo dice (`d2c_control_inyectando`).
//
// POR QUE EL CHEQUEO DE LA INYECCION VA PRIMERO (medido en DC-002.114): el cache guarda "no hay aparato" la primera
// vez que se pregunta, y en el cooperativo eso pasa ANTES de encender la inyeccion del jugador 2. Sin esta rama, el
// puerto quedaba cacheado como "sin control" y el estado inyectado nunca se leia: el jugador 2 no se movia.
func HayControl() bool {
	if d2cControlInyectando() == 1 {
		return true
	}

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

// ============================ DC-002.114: UN JUGADOR POR PUERTO ============================

// HayControlEn: si hay un control en ESE puerto del bus. Se pregunta una sola vez por puerto (como HayControl) y
// el .c vuelve a verificar en cada lectura que el aparato siga ahi.
func HayControlEn(puerto int) bool {
	if puerto < 0 || puerto >= puertosDelMaple {
		return false
	}

	// EL SHIM VA PRIMERO (medido en DC-002.114): si ese puerto lo pone el guion, hay estado aunque no haya aparato.
	// Sin esta rama el cache de abajo guardaba "no hay control" antes de encender la inyeccion y el jugador 2 no se
	// movia nunca (el .c tenia el estado inyectado, pero el Go ni se lo pedia).
	if d2cControlInyectandoPuerto(uint(puerto)) == 1 {
		return true
	}

	if !buscadoPuerto[puerto] {
		hayPuerto[puerto] = d2cControlBuscarPuerto(uint(puerto)) != 0
		buscadoPuerto[puerto] = true
	}

	return hayPuerto[puerto]
}

// AparatoEnPuerto: que aparato hay en ese puerto, como numero (los bits de `functions` del maple; 0 = nada). Es el
// instrumento de la evidencia: con esto se puede decir si el emulador tiene un segundo control o si el estado del
// jugador 2 viene del shim, sin adivinar.
func AparatoEnPuerto(puerto int) int {
	if puerto < 0 || puerto >= puertosDelMaple {
		return 0
	}

	// se fuerza la busqueda: `d2c_control_aparato` informa del aparato que encontro la busqueda
	HayControlEn(puerto)

	return d2cControlAparato(uint(puerto))
}

// ActualizarDesdeElControlEn: igual que ActualizarDesdeElControl, pero lee el aparato de ESE puerto del bus. Es la
// funcion que hace posible el cooperativo local: cada jugador tiene su `Servicio` y su puerto, y ninguno se entera
// del estado del otro.
//
// Si en ese puerto no hay control no es un error: el estado queda en "nada apretado" (el jugador 2 puede estar
// desconectado y el juego sigue con el jugador 1).
func (s *Servicio) ActualizarDesdeElControlEn(puerto int) bool {
	if !HayControlEn(puerto) {
		s.Actualizar(0, 0, 0, 0, 0)

		return false
	}

	var (
		botones              uint32
		palancaX, palancaY   int32
		gatilloL, gatilloR   int32
		palanca2X, palanca2Y int32
	)

	if d2cControlLeerPuerto(uint(puerto),
		uintptr(unsafe.Pointer(&botones)),
		uintptr(unsafe.Pointer(&palancaX)), uintptr(unsafe.Pointer(&palancaY)),
		uintptr(unsafe.Pointer(&gatilloL)), uintptr(unsafe.Pointer(&gatilloR)),
		uintptr(unsafe.Pointer(&palanca2X)), uintptr(unsafe.Pointer(&palanca2Y)),
	) != 0 {
		s.Actualizar(0, 0, 0, 0, 0)

		return false
	}

	s.Actualizar(botones, palancaX, palancaY, gatilloL, gatilloR)

	return true
}
