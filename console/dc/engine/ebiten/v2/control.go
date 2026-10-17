// El backend de ENTRADA de la consola: el control de la Dreamcast, traducido a lo que el motor pregunta.
//
// POR QUE ESTA ACA Y NO EN entrada.go: entrada.go tiene las PREGUNTAS del motor (IsKeyPressed, CursorPosition,
// IsMouseButtonPressed) y las costuras por donde se las responde (PonerTecla, PonerMouse, PonerBotonJoystick).
// Este archivo es EL QUE RESPONDE: lee el control de verdad (por el bus maple, ver control_dc.c) y llena esas
// costuras. Son dos cosas distintas: una es el mostrador, la otra es el que atiende.
//
// LA TRADUCCION (y por que es esta): el motor de Diablo II es de PC y pregunta por mouse y teclado.
//
//	la palanca y la cruceta -> el CURSOR y los ejes       (el juego se maneja con el mouse)
//	el boton A              -> el clic IZQUIERDO + Enter  ("camina hasta aca" / aceptar)
//	el boton B              -> el clic DERECHO + Escape   (la skill / volver)
//	el boton X              -> el clic del MEDIO + X
//	el boton Y              -> Y
//	el boton Start          -> Enter                      (arrancar, el boton de siempre)
//	los gatillos L y R      -> Q y E                      (los atajos que en la PC estan al lado de la mano)
//
// Y ADEMAS los botones se llenan como BOTONES DE JOYSTICK (GamepadButton), porque el motor tiene tambien un
// servicio de joystick (d2gamepad) que los pregunta por ahi. Se llenan los dos caminos con el mismo boton: el
// motor pregunta por uno o por otro segun de donde venga, y asi los dos ven lo mismo.
//
// LA CRUCETA SON LAS FLECHAS DEL TECLADO: es lo que hace que los MENUS se puedan navegar. Sin esto el juego
// arranca y no se puede elegir nada.
package ebiten

import (
	"unsafe"
)

// Las funciones del backend de C (console/dc/std/csrc/control_dc.c). Los punteros cruzan como uintptr (la
// misma regla que el backend de video: un puntero de Go pasado a un //extern tiene el ABI ambiguo).
//
//extern d2c_control_buscar
func d2cControlBuscar() int

//extern d2c_control_leer
func d2cControlLeer(botones, palancaX, palancaY, gatilloL, gatilloR, palanca2X, palanca2Y uintptr) int

// Los bits de los botones del control de la Dreamcast, COPIADOS del header de KOS (dc/maple/controller.h:
// 103-118) y no de memoria. El orden importa: no es el orden en que estan escritos los botones en el
// control, es el orden de los bits.
const (
	contC     = 1 << 0
	contB     = 1 << 1
	contA     = 1 << 2
	contStart = 1 << 3
	contUp    = 1 << 4
	contDown  = 1 << 5
	contLeft  = 1 << 6
	contRight = 1 << 7
	contZ     = 1 << 8
	contY     = 1 << 9
	contX     = 1 << 10
	contD     = 1 << 11
)

// El estado del backend de entrada: si hay control, donde esta el cursor, cuanto se movio la palanca el
// cuadro pasado (para saber si la palanca sigue quieta) y que se apreto en este cuadro.
var (
	controlHay      bool
	controlBuscado  bool
	cursorX         = PantallaAncho / 2
	cursorY         = PantallaAlto / 2
	palancaAnterior [2]int
)

// La velocidad del cursor: cuantos pixeles se mueve por cuadro con la palanca a fondo (128) y con la
// cruceta (que es digital, todo o nada).
//
// SON DOS NUMEROS DISTINTOS Y ES A PROPOSITO: la palanca es analogica y con 8 pixeles por cuadro a fondo el
// cursor cruza la pantalla en 80 cuadros (segundo y medio), que es manejable; la cruceta con el mismo numero
// seria lentisima porque no tiene medio movimiento. LA CRUCETA VA MAS RAPIDO PARA COMPENSAR, y asi los dos
// se sienten parecidos.
const (
	velocidadPalanca = 8
	velocidadCruceta = 12
)

// HayControl: si la consola encontro un control. Se busca una vez y se recuerda.
func HayControl() bool {
	if !controlBuscado {
		controlHay = d2cControlBuscar() != 0
		controlBuscado = true
	}
	return controlHay
}

// LeerControl: lee el control AHORA y llena todo lo que el motor pregunta (el cursor, el mouse, el teclado
// y el joystick). La llama el loop del motor, una vez por cuadro, antes de Update.
//
// SI NO HAY CONTROL NO HACE NADA (y no rompe nada): la consola arranca sin control, y el juego tiene que
// poder mostrar su pantalla igual. Es lo mismo que hace un juego de PC cuando no hay mouse conectado.
func LeerControl() {
	if !HayControl() {
		return
	}

	var botones uint32
	var palancaX, palancaY, gatilloL, gatilloR, palanca2X, palanca2Y int32

	if d2cControlLeer(
		uintptr(unsafe.Pointer(&botones)),
		uintptr(unsafe.Pointer(&palancaX)), uintptr(unsafe.Pointer(&palancaY)),
		uintptr(unsafe.Pointer(&gatilloL)), uintptr(unsafe.Pointer(&gatilloR)),
		uintptr(unsafe.Pointer(&palanca2X)), uintptr(unsafe.Pointer(&palanca2Y)),
	) != 0 {
		return
	}

	// ---- el cursor: la palanca (analogica) y la cruceta (digital), sumadas
	movX, movY := 0, 0

	// la palanca: -128 a 127, con una zona muerta en el medio (un control viejo no vuelve exacto al centro,
	// y sin zona muerta el cursor se va solo)
	if palancaX > 16 || palancaX < -16 {
		movX += int(palancaX) * velocidadPalanca / 128
	}
	if palancaY > 16 || palancaY < -16 {
		movY += int(palancaY) * velocidadPalanca / 128
	}

	// la cruceta
	if botones&contLeft != 0 {
		movX -= velocidadCruceta
	}
	if botones&contRight != 0 {
		movX += velocidadCruceta
	}
	if botones&contUp != 0 {
		movY -= velocidadCruceta
	}
	if botones&contDown != 0 {
		movY += velocidadCruceta
	}

	cursorX += movX
	cursorY += movY

	// y el cursor se queda adentro de la pantalla: si se fuera, el motor seguiria recibiendo una posicion
	// valida pero el jugador perderia de vista donde esta apuntando, y con el mouse no hay forma de volver.
	if cursorX < 0 {
		cursorX = 0
	}
	if cursorX >= PantallaAncho {
		cursorX = PantallaAncho - 1
	}
	if cursorY < 0 {
		cursorY = 0
	}
	if cursorY >= PantallaAlto {
		cursorY = PantallaAlto - 1
	}

	PonerMouse(cursorX, cursorY)

	// ---- la palanca, tambien como EJES del joystick (el motor tiene un servicio de joystick que los
	// pregunta por ahi, ademas del mouse)
	PonerEje(AxisLeftX, float64(palancaX)/128.0)
	PonerEje(AxisLeftY, float64(palancaY)/128.0)
	PonerEje(AxisRightX, float64(palanca2X)/128.0)
	PonerEje(AxisRightY, float64(palanca2Y)/128.0)

	// ---- los botones, por los DOS caminos: como botones del mouse/teclado (lo que pregunta el motor de PC)
	// y como botones del joystick (lo que pregunta el servicio de joystick del motor)
	apretado := func(bit uint32) bool { return botones&bit != 0 }

	PonerMouseBoton(MouseButtonLeft, apretado(contA))
	PonerTecla(KeyEnter, apretado(contA) || apretado(contStart))

	PonerMouseBoton(MouseButtonRight, apretado(contB))
	PonerTecla(KeyEscape, apretado(contB))

	PonerMouseBoton(MouseButtonMiddle, apretado(contX))
	PonerTecla(KeyX, apretado(contX))
	PonerTecla(KeyY, apretado(contY))

	// la cruceta, como las flechas del teclado: ES LO QUE HACE QUE LOS MENUS SE PUEDAN NAVEGAR
	PonerTecla(KeyUp, apretado(contUp))
	PonerTecla(KeyDown, apretado(contDown))
	PonerTecla(KeyLeft, apretado(contLeft))
	PonerTecla(KeyRight, apretado(contRight))

	// los gatillos, como los atajos que en la PC estan al lado de la mano izquierda
	PonerTecla(KeyQ, gatilloL > 32)
	PonerTecla(KeyE, gatilloR > 32)

	// y los botones del joystick, con el orden de Ebitengine (0=A, 1=B, 2=X, 3=Y, 4=L, 5=R, 6=Back, 7=Start)
	PonerBotonJoystick(0, apretado(contA))
	PonerBotonJoystick(1, apretado(contB))
	PonerBotonJoystick(2, apretado(contX))
	PonerBotonJoystick(3, apretado(contY))
	PonerBotonJoystick(4, gatilloL > 32)
	PonerBotonJoystick(5, gatilloR > 32)
	PonerBotonJoystick(6, apretado(contZ))
	PonerBotonJoystick(7, apretado(contStart))

	// LOS FLANCOS NO SE GUARDAN ACA: "esta apretada" y "se apreto en este cuadro" son dos cosas distintas, y
	// la segunda la lleva cuadro.go. El backend solo dice como esta el control AHORA; quien cierra el cuadro
	// (CerrarCuadro, al final de RunGame) es el que deja el estado anterior guardado para el cuadro que
	// viene. Si esto guardara los flancos por su cuenta, los dos lugares dirian cosas distintas.
}

// PosicionDelCursor: donde esta el cursor del juego (para la evidencia: el numero dice si el control movio
// algo, que es la unica forma de saberlo sin mirar la pantalla).
func PosicionDelCursor() (int, int) { return cursorX, cursorY }

// BotonesParaLaEvidencia: el estado del control en un numero, para poder comparar dos cuadros.
//
// POR QUE UN NUMERO Y NO UN TEXTO: un "el control anda" no se puede comparar entre dos corridas; un numero
// si. Los bits son los del control de KOS, asi que el que lee el numero puede decir exactamente que boton
// estaba apretado.
func BotonesParaLaEvidencia() uint32 {
	if !HayControl() {
		return 0
	}

	var botones uint32
	if d2cControlLeer(
		uintptr(unsafe.Pointer(&botones)),
		0, 0, 0, 0, 0, 0,
	) != 0 {
		return 0
	}

	return botones
}
