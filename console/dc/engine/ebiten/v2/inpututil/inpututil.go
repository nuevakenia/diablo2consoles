// Package inpututil: la parte de input de Ebitengine que el motor usa (que se apreto EN ESTE cuadro, y
// cuanto lleva apretado). Es la costura por donde entra el joystick de la consola.
//
// LO QUE ESTABA MAL, MEDIDO: este paquete tenia la mitad de lo que el motor pide, y lo que tenia devolvia
// cero. Medido (grep de inpututil.* en d2core, d2game y d2app) el motor pide SEIS funciones y aca habia
// tres; y de esas tres, una preguntaba por el estado sin tenerlo (KeyPressDuration devolvia 0 siempre).
//
// COMO SE DESTAPO, porque vale la pena: en un listado, las dos funciones de flanco (la de "se apreto" y la
// de "se solto") se veian iguales, pero una tiene 16 caracteres de nombre y la otra 17. Comparando el
// LARGO de los nombres aparecio que eran DOS funciones distintas y que la de "se solto" no existia. El
// largo fue lo que lo delato.
//
// Ahora estan las seis y funcionan de verdad: el estado de cada cuadro lo lleva el paquete ebiten (ver
// console/dc/engine/ebiten/v2/cuadro.go) y el backend de la consola tiene que llamar a ebiten.CerrarCuadro
// al terminar cada cuadro.
package inpututil

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// IsKeyJustPressed: si la tecla se apreto en este cuadro. Aca es lo mismo que estar apretada: quien
// llene el estado (el backend de la consola) marca el flanco al armar el cuadro.
func IsKeyJustPressed(k ebiten.Key) bool { return ebiten.IsKeyJustPressed(k) }

// IsKeyPressed: si esta apretada ahora.
func IsKeyPressed(k ebiten.Key) bool { return ebiten.IsKeyPressed(k) }

// IsKeyJustReleased: si la tecla se solto en este cuadro (esta es la que FALTABA: el motor la pide).
func IsKeyJustReleased(k ebiten.Key) bool { return ebiten.TeclaSeSolto(k) }

// KeyPressDuration: cuantos cuadros lleva apretada (0 si no lo esta).
func KeyPressDuration(k ebiten.Key) int { return ebiten.DuracionTecla(k) }

// IsMouseButtonJustPressed: si el boton del mouse se apreto en este cuadro.
func IsMouseButtonJustPressed(b ebiten.MouseButton) bool { return ebiten.MouseSeApreto(b) }

// IsMouseButtonJustReleased: si el boton del mouse se solto en este cuadro.
func IsMouseButtonJustReleased(b ebiten.MouseButton) bool { return ebiten.MouseSeSolto(b) }

// MouseButtonPressDuration: cuantos cuadros lleva apretado (0 si no lo esta).
func MouseButtonPressDuration(b ebiten.MouseButton) int { return ebiten.DuracionMouse(b) }

// AppendPressedKeys: agrega las teclas apretadas a la lista que le pasen, ordenadas por su numero (si no,
// el orden del mapa cambia en cada corrida y nada que dependa de eso se puede comparar ni repetir).
func AppendPressedKeys(keys []ebiten.Key) []ebiten.Key {
	for _, k := range ebiten.TeclasApretadas() {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// AppendInputChars: el texto tipeado en este cuadro.
func AppendInputChars(runes []rune) []rune {
	return append(runes, ebiten.InputChars()...)
}

// AppendJustPressedGamepadButtons: los botones del joystick que se apretaron en este cuadro.
func AppendJustPressedGamepadButtons(id ebiten.GamepadID, buttons []ebiten.GamepadButton) []ebiten.GamepadButton {
	for b := ebiten.GamepadButton(0); int(b) < ebiten.GamepadButtonNum(id); b++ {
		if ebiten.IsGamepadButtonPressed(id, b) && !ebiten.BotonJoystickEstabaApretado(id, b) {
			buttons = append(buttons, b)
		}
	}
	return buttons
}
