//go:build dreamcast

// DC-002.10: UN CUADRO DEL MOTOR en la Dreamcast -- no una textura suelta pintada por el C, sino el camino
// completo del juego:
//
//	el asset del motor -> una imagen del motor -> DrawImage sobre la pantalla del juego -> la pantalla a la
//	VRAM -> el PVR la pinta
//
// El asset es el mismo del POC motor-menu (el fondo del menu de seleccion de personaje, 256x256), asi que
// los dos POCs pintan LO MISMO: compararlos dice si el camino del motor da el mismo resultado que el camino
// directo. Si el dibujado de la consola tuviera el rojo y el azul cambiados, o el quad al reves, se ve en la
// comparacion y no "en un rato mirando la pantalla".
//
// LO QUE SE MUEVE: el logo cruza la pantalla de izquierda a derecha y vuelve a entrar. Eso no es adorno: es
// la unica forma de probar que la MATRIZ DE GEOMETRIA de verdad posiciona. Con la matriz de mentira (la que
// habia antes, que solo guardaba "hubo un cambio") el logo apareceria siempre en el mismo lugar, y el
// defecto no se notaria hasta tener el juego entero andando.
//
// Y el LOOP es el del motor (ebiten.RunGame): esperar el cuadro, actualizar, dibujar, cerrar la lista del
// PVR. Es el mismo loop que va a correr el juego.
package main

import (
	"image/color"
	"runtime"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_asset_w
func d2cAssetW() int

//extern d2c_asset_h
func d2cAssetH() int

//extern d2c_asset_idx
func d2cAssetIdx() uint

//extern d2c_asset_pal
func d2cAssetPal(i int) uint

// juego: lo que el motor le pide a una pantalla (Update y Draw). Es la interfaz Game, tal cual.
type juego struct {
	logo    *ebiten.Image
	x       float64
	cuadros int
	vueltas int
}

// Layout: el tamano de la pantalla.
func (j *juego) Layout(ancho, alto int) (int, int) { return 640, 480 }

// Update: un cuadro mas. Mueve el logo y cada 60 cuadros deja constancia (60 cuadros son un segundo, asi
// que en la consola serie se ve el ritmo al que va).
func (j *juego) Update() error {
	j.cuadros++

	j.x += 3
	if j.x > 640 {
		j.x = -256
		j.vueltas++
	}

	if j.cuadros%60 == 0 {
		println("MOTOR-PANTALLA cuadros", j.cuadros, "x", int(j.x), "vueltas", j.vueltas,
			"dibujos", ebiten.Dibujos, "huella", int(ebiten.HuellaDePantalla()))
	}

	return nil
}

// Draw: el cuadro. Pinta el logo en su posicion y un marco fijo.
//
// EL MARCO FIJO NO ES ADORNO: si lo unico que se dibujara fuera el logo, un cuadro donde el logo esta fuera
// de la pantalla se veria IGUAL que un cuadro donde no se dibujo nada. El marco da una referencia que no se
// mueve, y asi "se movio el logo" se distingue de "se dejo de dibujar".
func (j *juego) Draw(pantalla *ebiten.Image) {
	var op ebiten.DrawImageOptions
	op.GeoM.Translate(j.x, 112)
	pantalla.DrawImage(j.logo, &op)

	marca := color.RGBA{R: 255, G: 255, B: 0, A: 255}
	for x := 0; x < 640; x++ {
		pantalla.SetRGBA(x, 0, marca)
		pantalla.SetRGBA(x, 479, marca)
	}
	for y := 0; y < 480; y++ {
		pantalla.SetRGBA(0, y, marca)
		pantalla.SetRGBA(639, y, marca)
	}
}

func main() {
	d2cSerialInit()

	println("MOTOR-PANTALLA arranca (DC-002.10: el cuadro del motor en la consola)")
	println("MOTOR-PANTALLA runtime", runtime.GOOS, runtime.GOARCH)

	ancho, alto := d2cAssetW(), d2cAssetH()
	println("MOTOR-PANTALLA asset", ancho, "x", alto)

	logo := armarImagen(ancho, alto)
	println("MOTOR-PANTALLA imagen del motor armada, bytes de RGBA", ancho*alto*4)

	// el video se arranca ACA y no dentro de RunGame: asi el numero de la VRAM que se informa es el de
	// verdad. La primera version lo pedia antes de arrancar el video y imprimia 0 -- un cero seguro, sin
	// sentido, que parecia un dato. (RunGame lo vuelve a pedir; arrancar dos veces no hace nada.)
	if err := ebiten.IniciarPantalla(640, 480); err != nil {
		println("MOTOR-PANTALLA no pude arrancar el video:", err.Error())
	}
	println("MOTOR-PANTALLA la pantalla del juego ocupa en la VRAM, bytes", ebiten.VramDeLaPantalla())

	if err := ebiten.RunGame(&juego{logo: logo}); err != nil {
		println("MOTOR-PANTALLA el loop termino con error:", err.Error())
	}
}

// armarImagen: los indices del asset + su paleta -> una imagen del motor (RGBA de 8 bits por canal).
//
// OJO CON LA PALETA: el horneador la deja como ARGB1555 (5 bits de rojo, 5 de verde, 5 de azul y 1 de
// transparencia), que es el formato del PVR. Al pasarla a RGBA de 8 bits hay que VOLVER A ESTIRAR cada
// canal: el rojo de 5 bits se multiplica por 255/31 y el verde por 255/31 (tambien 5 bits). Usar 255/31 para
// los dos y 255/31 para el azul es correcto; lo que NO se puede hacer es copiar los bits como si ya fueran
// de 8 (sale todo oscuro y con los colores corridos).
func armarImagen(ancho, alto int) *ebiten.Image {
	img := ebiten.NewImage(ancho, alto)

	base := d2cAssetIdx()

	for i := 0; i < ancho*alto; i++ {
		indice := *(*uint8)(unsafe.Pointer(uintptr(base) + uintptr(i)))
		argb := d2cAssetPal(int(indice))

		r5 := uint8((argb >> 10) & 0x1f)
		g5 := uint8((argb >> 5) & 0x1f)
		b5 := uint8(argb & 0x1f)

		// el indice 0 es transparente (la convencion de Diablo II): asi el marco del fondo se ve
		alfa := uint8(255)
		transparente := (argb>>15)&1 == 0
		if indice == 0 || transparente {
			alfa = 0
		}

		img.SetRGBA(i%ancho, i/ancho, color.RGBA{
			R: uint8(uint16(r5) * 255 / 31),
			G: uint8(uint16(g5) * 255 / 31),
			B: uint8(uint16(b5) * 255 / 31),
			A: alfa,
		})
	}

	return img
}
