// El cuadro: lo que distingue "esta apretada" de "se apreto ahora", y cuanto lleva apretada.
//
// POR QUE HACE FALTA: el motor pregunta dos cosas distintas, y no son la misma:
//
//	ebiten.IsKeyPressed(k)            -> esta apretada AHORA (sirve para caminar)
//	inpututil.IsKeyJustPressed(k)     -> se apreto EN ESTE CUADRO (sirve para los menus: si no, un toque
//	                                     corto se pierde, o un toque largo dispara mil veces)
//
// Eso quiere decir que el estado TIENE que saber que paso en el cuadro anterior. En la PC eso lo hace
// ebiten por dentro; aca lo hace esta parte, y el backend de la consola solo tiene que avisar cuando el
// cuadro termina (CerrarCuadro).
package ebiten

// teclasAntes: como estaba cada tecla al cerrar el cuadro anterior.
var teclasAntes = map[Key]bool{}

// duracionTeclas: cuantos cuadros lleva apretada cada una.
var duracionTeclas = map[Key]int{}

// mouseAntes y duracionMouse: lo mismo para los botones del mouse.
var (
	mouseAntes    [MouseButtonNum]bool
	duracionMouse [MouseButtonNum]int
)

// CerrarCuadro: hay que llamarla al terminar cada cuadro. Guarda como estaban las cosas, para que en el
// cuadro que viene "JustPressed" y "JustReleased" sepan que cambio.
func CerrarCuadro() {
	for k, apretada := range teclas {
		teclasAntes[k] = apretada
		if apretada {
			duracionTeclas[k]++
		} else {
			delete(duracionTeclas, k)
		}
	}

	for b := 0; b < MouseButtonNum; b++ {
		mouseAntes[b] = mouseApretados[b]
		if mouseApretados[b] {
			duracionMouse[b]++
		} else {
			duracionMouse[b] = 0
		}
	}

	for b := 0; b < 8; b++ {
		botonesAntes[b] = botonesJP[b]
	}
}

// TeclaSeApreto: si la tecla paso de soltada a apretada entre el cuadro anterior y este.
func TeclaSeApreto(k Key) bool { return teclas[k] && !teclasAntes[k] }

// TeclaSeSolto: si paso de apretada a soltada.
func TeclaSeSolto(k Key) bool { return !teclas[k] && teclasAntes[k] }

// DuracionTecla: cuantos cuadros lleva apretada (0 si no lo esta).
func DuracionTecla(k Key) int { return duracionTeclas[k] }

// MouseSeApreto: si el boton paso de suelto a apretado.
func MouseSeApreto(b MouseButton) bool {
	if int(b) < 0 || int(b) >= MouseButtonNum {
		return false
	}
	return mouseApretados[b] && !mouseAntes[b]
}

// MouseSeSolto: si paso de apretado a suelto.
func MouseSeSolto(b MouseButton) bool {
	if int(b) < 0 || int(b) >= MouseButtonNum {
		return false
	}
	return !mouseApretados[b] && mouseAntes[b]
}

// DuracionMouse: cuantos cuadros lleva apretado (0 si no lo esta).
func DuracionMouse(b MouseButton) int {
	if int(b) < 0 || int(b) >= MouseButtonNum {
		return 0
	}
	return duracionMouse[b]
}

// botonesAntes: como estaba cada boton del joystick al cerrar el cuadro anterior.
var botonesAntes [8]bool

// BotonJoystickEstabaApretado: si el boton ya estaba apretado en el cuadro anterior (lo usa inpututil
// para armar la lista de "se apreto ahora").
func BotonJoystickEstabaApretado(id GamepadID, b GamepadButton) bool {
	if id != 0 || int(b) < 0 || int(b) >= 8 {
		return false
	}
	return botonesAntes[b]
}

// TeclasApretadas: cuales estan apretadas (el backend lo usa para lo que necesite; el motor tiene
// AppendPressedKeys).
func TeclasApretadas() []Key {
	salida := make([]Key, 0, len(teclas))
	for k, apretada := range teclas {
		if apretada {
			salida = append(salida, k)
		}
	}
	return salida
}
