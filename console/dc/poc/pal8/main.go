//go:build dreamcast
// +build dreamcast

// DC-001.4: un asset REAL de Diablo II (un DC6 horneado desde el MPQ en tiempo de build)
// dibujado en la Dreamcast como textura paletizada: indices de 8 bits en VRAM + la paleta
// por hardware del PowerVR2.
//
// Es el paso que decide el port: si el arte del juego entra en 8 bits con su paleta, la
// memoria de texturas cae a la mitad (1 byte por pixel en vez de 16/32) y el PVR sigue
// haciendo el blending por hardware.
//
// Evidencia que deja:
//   - texto por la consola serie (println -> SCIF, se lee en el log de Flycast)
//   - el asset dibujado dos veces: 1:1 (para comparar pixel a pixel contra la referencia del
//     host que genera el horneador) y escalado (para ver que el PVR escala sin costo de CPU)
//   - el fondo violeta se ve a traves de las zonas transparentes del asset (indice 0)
package main

import (
	"kos"
	"runtime"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_pal8_init
func d2cPal8Init() int32

//extern d2c_pal8_quad
func d2cPal8Quad(x, y, s float32, argb uint32)

//extern d2c_asset_w
func d2cAssetW() int32

//extern d2c_asset_h
func d2cAssetH() int32

func main() {
	d2cSerialInit()
	println("POC-PAL8: arranco (DC-001.4, asset real de Diablo II en 8 bits)")
	println("POC-PAL8: runtime", runtime.GOOS, runtime.GOARCH)

	kos.PvrInitDefaults()
	// violeta: tiene que verse alrededor del logo (zonas transparentes del asset)
	kos.PvrSetBgColor(0.35, 0.0, 0.55)

	aw := d2cAssetW()
	ah := d2cAssetH()

	if r := d2cPal8Init(); r != 0 {
		println("POC-PAL8: FALLO la textura paletizada, codigo", r)
	} else {
		println("POC-PAL8: asset", aw, "x", ah,
			"= indices", aw*ah, "bytes en VRAM (16 bits/pixel habrian sido", aw*ah*2, ")")
		println("POC-PAL8: vram libre en bytes:", int32(kos.PvrMemAvailable()))
	}

	// copia A: 1:1, fija. Es la que se compara contra la referencia del host.
	ax := float32(48)
	ay := float32(64)

	// copia B: escalada 4x y movida por el mando.
	bx := float32(48)
	by := float32(220)
	escala := float32(4)

	blanco := kos.PlxPackColor(1, 1, 1, 1)

	kos.TimerMsEnable()
	usPrevios := kos.TimerUsGettime64()

	var frames uint32

	for {
		kos.PvrWaitReady()
		kos.PvrSceneBegin()
		kos.PvrListBegin(kos.PVR_LIST_TR_POLY)

		d2cPal8Quad(ax, ay, 1, blanco)
		d2cPal8Quad(bx, by, escala, blanco)

		kos.PvrListFinish()
		kos.PvrSceneFinish()

		// mando: mueve la copia grande
		if c := kos.GetFirstController(); c != nil {
			if c.ButtonPressed(kos.CONT_DPAD_LEFT) {
				bx -= 4
			}
			if c.ButtonPressed(kos.CONT_DPAD_RIGHT) {
				bx += 4
			}
			if c.ButtonPressed(kos.CONT_DPAD_UP) {
				by -= 4
			}
			if c.ButtonPressed(kos.CONT_DPAD_DOWN) {
				by += 4
			}
		}

		frames++
		if frames%120 == 0 {
			us := kos.TimerUsGettime64()
			deltaUs := us - usPrevios
			usPrevios = us
			println("POC-PAL8: frame", frames, "us/frame", deltaUs/120, "fps", 120000000/deltaUs,
				"vram libre", int32(kos.PvrMemAvailable()),
				"B en", int32(bx), int32(by))
		}
	}
}
