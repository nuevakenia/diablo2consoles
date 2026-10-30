package aica

import (
	"math"
)

// LAS DOS ESCALAS, Y POR QUE HAY QUE TRADUCIR.
//
// El motor habla en 0..1: `SetVolumes(bgmVolume, sfxVolume float64)` recibe lo que dicen los deslizadores de
// opciones y `SoundEffect.SetVolume(v float64)` lo mismo (es el contrato de d2core/d2audio/ebiten, que se lo
// pasa a ebiten tal cual). El AICA habla en 0..255 con 128 en el centro del pan:
//
//	snd_sfx_play(index, vol, pan)   -> vol 0..255 (snd_sfxmgr.h: el maximo es 0xff)
//	                                 -> pan 0 = todo a la izquierda, 128 = centro, 255 = todo a la derecha
//
// Y el motor da el pan en -1..1 (d2core/d2audio/ebiten/ebiten_sound_effect.go: "left is -1.0, center is 0.0,
// right is 1.0"). Las dos traducciones viven aca, en Go comun con prueba en el host, porque un error de
// escala no se ve: suena bajito o se va a un lado, y eso no lo dice ningun log.
const (
	// VolumenMaximo es el tope del registro de volumen del AICA (0xff, aica.c:calc_aica_vol).
	VolumenMaximo = 255
	// PanCentro es el valor del pan que suena igual por los dos parlantes.
	PanCentro = 128
	// PanMinimo y PanMaximo son los extremos del registro del pan.
	PanMinimo = 0
	PanMaximo = 255
)

// VolumenAICA traduce un volumen del motor (0..1, donde 1 es el tope) al registro del AICA (0..255).
//
// Se RECORTA en vez de envolver: un 1.5 o un -0.2 (que el motor puede pasar por una suma de factores) suena
// al tope o mudo, no al reves. Se redondea, no se trunca: truncar deja el 0.999 en 254 y el volumen maximo
// nunca llega al tope del chip.
func VolumenAICA(volumen float64) int {
	if math.IsNaN(volumen) || volumen <= 0 {
		return 0
	}

	if volumen >= 1 {
		return VolumenMaximo
	}

	return int(math.Round(volumen * VolumenMaximo))
}

// PanAICA traduce un pan del motor (-1 izquierda .. 0 centro .. +1 derecha) al registro del AICA
// (0 izquierda .. 128 centro .. 255 derecha).
//
// El centro es EXACTO: -1..1 es una recta de largo 2 que se mapea sobre 0..255, asi que el 0 del motor cae
// en 127,5 y se redondea a 128 (que es el centro que el chip trata como centro, no 127).
func PanAICA(pan float64) int {
	if math.IsNaN(pan) {
		return PanCentro
	}

	if pan <= -1 {
		return PanMinimo
	}

	if pan >= 1 {
		return PanMaximo
	}

	return int(math.Round((pan + 1) * PanMaximo / 2))
}
