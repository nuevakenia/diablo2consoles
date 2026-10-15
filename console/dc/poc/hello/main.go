//go:build dreamcast
// +build dreamcast

// POC DC-001.1: el "hola mundo" de Go en Dreamcast.
// Prueba que el camino completo funciona: gccgo (sh-elf) + runtime libgodc + arranque desde
// la imagen (KOS) + salida por consola serie (println -> dc-tool / pantalla del emulador).
package main

import "kos"

func main() {
	println("=== DC-001.1 hola desde Go en Dreamcast ===")
	println("si lees esto, gccgo+libgodc+KOS arrancaron")

	kos.PvrInitDefaults()
	kos.PvrSetBgColor(0.05, 0.02, 0.15)

	println("PVR inicializado; entrando al bucle de video")

	for {
		kos.PvrSceneBegin()
		kos.PvrSceneFinish()
		kos.PvrWaitReady()
	}
}
