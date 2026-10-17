// Entrada de la consola para el ebiten: mouse, joystick y cursor.
//
// POR QUE ESTA SEPARADO: ebiten.go tiene la imagen y el dibujo; esto es lo otro, la entrada. En la
// Dreamcast el mouse NO EXISTE y el teclado tampoco: hay un joystick con sus botones y su palanca. Pero el
// motor fue escrito para un juego de PC, asi que pregunta por mouse y teclado. La forma de resolverlo es la
// misma que con el dibujo: la consola le MIENTE con la misma cara que la PC, y por dentro traduce.
//
// LA TRADUCCION (esto es lo que hace que el juego se pueda jugar):
//
//	la palanca del joystick  -> CursorPosition (el mouse)
//	el boton A               -> MouseButtonLeft (el clic izquierdo, que es "agarrar/hablar")
//	el boton B               -> MouseButtonRight (el derecho, que es "usar la skill")
//	la cruceta               -> las flechas del teclado (los menus)
//
// MEDIDO: el motor pide estos nombres (grep de ebiten.* en los paquetes ebiten del motor): MouseButton,
// MouseButtonLeft, MouseButtonMiddle, MouseButtonRight, CursorPosition, CursorModeHidden, Wheel,
// GamepadAxis, GamepadAxisNum, GamepadButton, GamepadButtonNum, GamepadID, GamepadIDs, GamepadName,
// GamepadSDLID, IsGamepadButtonPressed, InputChars.
//
// A DONDE ENTRA EL JOYSTICK: por PonerTecla (el teclado) y por las funciones Poner* de este archivo (el
// mouse y el joystick). El motor pregunta y no se entera de quien aprieta. Hoy el que llena esto es el POC
// (console/dc/poc/motor-menu), y cuando el juego arranque de verdad lo va a llenar el backend de la
// consola leyendo el control con el API de KOS.
package ebiten

// ================================================================ el mouse

// MouseButton: el boton del mouse.
type MouseButton int

const (
	MouseButtonLeft MouseButton = iota
	MouseButtonRight
	MouseButtonMiddle

	MouseButtonNum = 3
)

// MouseButtonLeft / Right / Middle ya estan arriba, y "Num" dice cuantos hay.

// CursorMode: el modo del cursor.
type CursorMode int

const (
	CursorModeVisible CursorMode = iota
	CursorModeHidden
	CursorModeLocked
)

// el estado del mouse: la posicion y que botones estan apretados.
var (
	mouseX, mouseY  int
	mouseApretados  [MouseButtonNum]bool
	ruedaX, ruedaY  float64
	cursorModo      = CursorModeVisible
	caracteresTipeo []rune
)

// CursorPosition: donde esta el cursor. En la consola lo mueve la palanca del joystick.
func CursorPosition() (int, int) { return mouseX, mouseY }

// SetCursorPosition: lo mueve.
func SetCursorPosition(x, y int) { mouseX, mouseY = x, y }

// IsMouseButtonPressed: si el boton esta apretado. En la consola, el boton A es el clic izquierdo.
func IsMouseButtonPressed(b MouseButton) bool {
	if int(b) < 0 || int(b) >= MouseButtonNum {
		return false
	}
	return mouseApretados[b]
}

// Wheel: el giro de la rueda. En la consola no hay rueda (los gatillos podrian serlo, y se ve al medir si
// el juego los necesita).
func Wheel() (float64, float64) { return ruedaX, ruedaY }

// CursorModeActual: en que modo esta (lo usa el backend).
func CursorModeActual() CursorMode { return cursorModo }

// PonerMouse: la usa el backend de la consola para mover el cursor y apretar botones. Es la costura por
// donde entra el joystick, igual que PonerTecla para el teclado.
func PonerMouse(x, y int) { mouseX, mouseY = x, y }

// PonerMouseBoton: marca un boton del mouse apretado o soltado.
func PonerMouseBoton(b MouseButton, apretado bool) {
	if int(b) >= 0 && int(b) < MouseButtonNum {
		mouseApretados[b] = apretado
	}
}

// PonerRueda: la rueda (para el backend).
func PonerRueda(x, y float64) { ruedaX, ruedaY = x, y }

// ================================================================ el joystick

// GamepadAxis: cuanto esta movido un eje de la palanca.
//
// MEDIDO EN EL EBITEN DE VERDAD (v2.0.2, input.go:157): esto NO es un tipo, es una FUNCION
// `func GamepadAxis(id GamepadID, axis int) float64`, y el eje se pide con un int. Aca estaba escrito
// como un tipo, y el motor que lo llama como funcion (d2input/ebiten/gamepad_service.go:59:
// `ebiten.GamepadAxis(id, i)`) no compilaba.
func GamepadAxis(id GamepadID, eje int) float64 {
	if id != 0 || eje < 0 || eje >= 4 {
		return 0
	}
	return ejes[eje]
}

// Los ejes, con los mismos nombres que usa el motor. Son int, como en ebiten.
const (
	AxisLeftX = iota
	AxisLeftY
	AxisRightX
	AxisRightY
)

// GamepadAxisNum: cuantos ejes tiene un joystick. Toma el id porque asi es en ebiten (medido en el motor:
// d2core/d2input/ebiten/gamepad_service.go:50 lo llama GamepadButtonNum(id), GamepadAxisNum(id)).
func GamepadAxisNum(id GamepadID) int {
	if id != 0 {
		return 0
	}
	return 4
}

// GamepadButtonNum: cuantos botones tiene el joystick. Toma el id por lo mismo que GamepadAxisNum. Un
// control de Dreamcast tiene 8: A, B, X, Y, Start, y los gatillos L/R como botones (ademas de que L y R
// son analogicos por su cuenta).
func GamepadButtonNum(id GamepadID) int {
	if id != 0 {
		return 0
	}
	return 8
}

// GamepadIDs: los joysticks conectados. En la consola hay uno solo, y el control esta siempre puesto: no
// se conecta ni se desconecta, es parte de la maquina.
func GamepadIDs() []GamepadID { return []GamepadID{0} }

// GamepadName: el nombre del joystick.
func GamepadName(id GamepadID) string {
	if id != 0 {
		return ""
	}
	return "Dreamcast Controller"
}

// GamepadSDLID: el identificador de SDL. En la consola no hay SDL: se devuelve algo fijo para que el
// codigo del motor que lo use para distinguir controles tenga un valor estable.
func GamepadSDLID(id GamepadID) string {
	if id != 0 {
		return ""
	}
	return "dreamcast-0"
}

// el estado del joystick: los ejes y los botones.
var (
	ejes        [4]float64
	botonesJP   [8]bool
)

// GamepadAxisValue: cuanto esta movido un eje (de -1 a 1, como ebiten). En la consola lo llena el backend
// con la palanca.
func GamepadAxisValue(id GamepadID, eje int) float64 {
	if id != 0 || int(eje) < 0 || int(eje) >= 4 {
		return 0
	}
	return ejes[eje]
}

// GamepadButtonValue: cuanto esta apretado un boton analogico (los gatillos). 0 o 1 los digitales.
func GamepadButtonValue(id GamepadID, b GamepadButton) float64 {
	if id != 0 || int(b) < 0 || int(b) >= 8 {
		return 0
	}
	if botonesJP[b] {
		return 1
	}
	return 0
}

// IsGamepadButtonPressed: si el boton esta apretado.
func IsGamepadButtonPressed(id GamepadID, b GamepadButton) bool {
	if id != 0 || int(b) < 0 || int(b) >= 8 {
		return false
	}
	return botonesJP[b]
}

// PonerEje: la usa el backend para mover un eje del joystick.
func PonerEje(eje int, valor float64) {
	if int(eje) >= 0 && int(eje) < 4 {
		ejes[eje] = valor
	}
	// la palanca tambien mueve el cursor: el motor de PC espera un mouse, y en la consola el que mueve es
	// la palanca. Se suma el movimiento del eje izquierdo.
	if eje == AxisLeftX || eje == AxisLeftY {
		PonerMouse(mouseX+int(round(valor*8)), mouseY+int(round(valor*8)))
	}
}

// PonerBotonJoystick: la usa el backend para apretar o soltar un boton.
func PonerBotonJoystick(b GamepadButton, apretado bool) {
	if int(b) >= 0 && int(b) < 8 {
		botonesJP[b] = apretado
	}
}

// round: redondear a entero (no se usa el de math para no arrastrar el paquete entero por esto).
func round(v float64) float64 {
	if v < 0 {
		return -float64(int(-v + 0.5))
	}
	return float64(int(v + 0.5))
}

// ================================================================ el texto tipeado

// InputChars: las letras que se tipearon desde el cuadro anterior (para los campos de texto: el nombre del
// personaje, los nombres de las partidas). En la consola se llena con los botones: no hay teclado de
// verdad.
func InputChars() []rune { return caracteresTipeo }

// PonerCaracter: la usa el backend para agregar una letra tipeada.
func PonerCaracter(r rune) { caracteresTipeo = append(caracteresTipeo, r) }

// LimpiarCaracteres: la usa el backend al cerrar el cuadro (ebiten devuelve solo lo del cuadro).
func LimpiarCaracteres() { caracteresTipeo = nil }

// ================================================================ la ventana (lo que falta)

// IsVsyncEnabled: si esta prendido el vsync.
func IsVsyncEnabled() bool { return true }

// SetMaxTPS: el tope de cuadros por segundo (0 = sin tope).
func SetMaxTPS(tps int) {}

// UncappedTPS: la constante de "sin tope" de ebiten.
const UncappedTPS = 0

// SetRunnableOnUnfocused: en la consola el juego siempre esta enfocado (es la maquina entera).
func SetRunnableOnUnfocused(v bool) {}

// SetWindowResizable: no hay ventana que redimensionar (la pantalla de la consola es fija).
func SetWindowResizable(v bool) {}

// SetWindowSize: el tamano de la ventana. En la consola es fijo.
func SetWindowSize(w, h int) {}

// ScreenSizeInFullscreen: el tamano de la pantalla en pantalla completa (640x480, lo que da el PVR).
func ScreenSizeInFullscreen() (int, int) { return 640, 480 }
