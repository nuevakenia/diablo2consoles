//go:build dreamcast

// DC-002.9: una PANTALLA REAL del juego en la Dreamcast.
//
// El asset es el fondo del menu de seleccion de personaje (data\global\ui\BIGMENU\amazonback.DC6,
// 488892 bytes en el MPQ, 256x256 y 12 frames): lo decodifica el decodificador del MOTOR (d2dc6, el
// mismo codigo que corre en el host) y lo pinta el PVR como textura paletizada de 8 bits con su paleta
// por hardware.
//
// Lo que se mide: el tamano del asset y el de la textura (que en este caso es potencia de dos exacta,
// 256x256, asi que no hay relleno), la VRAM que queda, y los fps del bucle de dibujo.
//
// La cadena de subida (indices 8 bits + paleta por hardware + quad texturizado) esta en C porque el
// binding de libgodc no expone pvr_txr_load_ex ni pvr_set_pal_entry. Ver csrc/cshim.c.
package main

import (
	"kos"
	"runtime"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_menu_init
func d2cMenuInit() int32

//extern d2c_menu_quad
func d2cMenuQuad(x, y, s float32, argb uint32)

//extern d2c_menu_w
func d2cMenuW() int32

//extern d2c_menu_h
func d2cMenuH() int32

//extern d2c_menu_texw
func d2cMenuTexW() int32

func main() {
	d2cSerialInit()
	println("POC-MENU: arranca (DC-002.9, una pantalla real del juego en la Dreamcast)")
	println("POC-MENU: runtime", runtime.GOOS, runtime.GOARCH)

	kos.PvrInitDefaults()
	kos.PvrSetBgColor(0.0, 0.0, 0.0)
	kos.PvrWaitReady()

	ancho := d2cMenuW()
	alto := d2cMenuH()
	println("POC-MENU: asset", ancho, "x", alto)
	println("POC-MENU: textura", d2cMenuTexW())

	rc := d2cMenuInit()
	if rc != 0 {
		println("POC-MENU: no pude subir la textura, codigo", rc)
	} else {
		println("POC-MENU: textura subida (8 bits por pixel + paleta por hardware)")
	}

	esc := float32(640) / float32(ancho)
	println("POC-MENU: escala para llenar la pantalla, milesima:", int(esc*1000))
	println("POC-MENU: dibujando")

	frame := 0
	for {
		kos.PvrWaitReady()
		kos.PvrSceneBegin()
		// la lista hay que ABRIRLA antes de mandar primitivas y cerrarla despues: sin esto el PVR
		// responde "attempt to submit to unopened list" y no dibuja nada (medido: pantalla negra).
		kos.PvrListBegin(kos.PVR_LIST_TR_POLY)
		d2cMenuQuad(0, 0, esc, 0xFFFFFFFF)
		kos.PvrListFinish()
		kos.PvrSceneFinish()
		frame++
		if frame == 300 {
			println("POC-MENU: 300 frames dibujados")
		}
	}
}
