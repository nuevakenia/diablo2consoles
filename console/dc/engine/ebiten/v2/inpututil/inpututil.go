// Package inpututil: la parte de input de Ebitengine que el motor usa (teclas apretadas en este cuadro
// y textos tipeados). Es la costura por donde entra el joystick de la consola.
package inpututil

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// IsKeyJustPressed: si la tecla se apreto en este cuadro. Aca es lo mismo que estar apretada: quien
// llene el estado (el backend de la consola) marca el flanco al armar el cuadro.
func IsKeyJustPressed(k ebiten.Key) bool { return ebiten.IsKeyJustPressed(k) }

// IsKeyPressed: si esta apretada.
func IsKeyPressed(k ebiten.Key) bool { return ebiten.IsKeyPressed(k) }

// KeyPressDuration: cuanto lleva apretada (en cuadros). Sin backend devuelve 0.
func KeyPressDuration(k ebiten.Key) int { return 0 }

// AppendPressedKeys: las teclas apretadas. Sin backend no hay ninguna.
func AppendPressedKeys(keys []ebiten.Key) []ebiten.Key { return keys }

// AppendInputChars: el texto tipeado en este cuadro.
func AppendInputChars(runes []rune) []rune { return runes }

// AppendJustPressedGamepadButtons: los botones del joystick que se apretaron.
func AppendJustPressedGamepadButtons(id ebiten.GamepadID, buttons []ebiten.GamepadButton) []ebiten.GamepadButton {
	return buttons
}
