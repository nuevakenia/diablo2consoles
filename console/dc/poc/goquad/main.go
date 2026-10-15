//go:build dreamcast
// +build dreamcast

// DC-001.3: primer POC en Go sobre Dreamcast.
// Prueba la cadena completa sin nada de C propio mas que el puente a la consola serie:
//
//	Go (gccgo sh-elf) -> runtime libgodc -> KOS -> PVR (dibujo) + maple (mando) + perf (ciclos)
//
// Evidencia que deja:
//   - texto por la consola serie (println -> SCIF, se lee en el log de Flycast)
//   - un cuadro blanco que rebota solo (dos capturas en momentos distintos = el bucle de
//     frames esta vivo y el PVR dibuja)
//   - el estado del mando (si hay control conectado y que botones estan apretados)
package main

import "kos"

//extern d2c_serial_init
func d2cSerialInit()

func main() {
	d2cSerialInit()
	println("POC-GO: arranco Go en Dreamcast (DC-001.3)")

	kos.PvrInitDefaults()
	kos.PvrSetBgColor(0.05, 0.05, 0.20)
	println("POC-GO: PVR listo; vram libre en bytes:", int32(kos.PvrMemAvailable()))

	cx := float32(320)
	cy := float32(240)
	dx := float32(3)
	dy := float32(2)

	var frames uint32
	// OJO: los contadores de rendimiento del SH-4 (PerfCntrStart/PerfCntrCycles) devuelven 0
	// en Flycast: el emulador no implementa PMCR/PMCTR. Para medir en el emulador se usa el
	// temporizador (TMU), que si emula. En consola real los contadores son la medida buena.
	kos.TimerMsEnable()
	usPrevios := kos.TimerUsGettime64()
	kos.PerfCntrStart()

	for {
		// ---- dibujo ----
		kos.PvrWaitReady()
		kos.PvrSceneBegin()
		kos.PvrListBegin(kos.PVR_LIST_OP_POLY)
		kos.PlxCxtInit()
		kos.PlxCxtTexture(nil)
		kos.PlxCxtCulling(kos.PLX_CULL_NONE)
		kos.PlxCxtSend(int32(kos.PVR_LIST_OP_POLY))

		blanco := kos.PlxPackColor(1, 1, 1, 1)
		kos.PlxVertInp(kos.PLX_VERT, cx-24, cy-24, 10, blanco)
		kos.PlxVertInp(kos.PLX_VERT, cx+24, cy-24, 10, blanco)
		kos.PlxVertInp(kos.PLX_VERT, cx-24, cy+24, 10, blanco)
		kos.PlxVertInp(kos.PLX_VERT_EOS, cx+24, cy+24, 10, blanco)

		kos.PvrListFinish()
		kos.PvrSceneFinish()

		// ---- rebote solo (para que las capturas muestren movimiento sin tocar nada) ----
		cx += dx
		cy += dy
		if cx < 24 || cx > 616 {
			dx = -dx
		}
		if cy < 24 || cy > 456 {
			dy = -dy
		}

		// ---- mando ----
		if c := kos.GetFirstController(); c != nil {
			if c.ButtonPressed(kos.CONT_DPAD_LEFT) {
				cx -= 6
			}
			if c.ButtonPressed(kos.CONT_DPAD_RIGHT) {
				cx += 6
			}
			if c.ButtonPressed(kos.CONT_DPAD_UP) {
				cy -= 6
			}
			if c.ButtonPressed(kos.CONT_DPAD_DOWN) {
				cy += 6
			}
		}

		// ---- medicion ----
		frames++
		if frames%120 == 0 {
			us := kos.TimerUsGettime64()
			deltaUs := us - usPrevios
			usPrevios = us
			println("POC-GO: frame", frames, "us/frame", deltaUs/120, "fps", 120000000/deltaUs,
				"ciclos SH4", kos.PerfCntrCycles())
			if c := kos.GetFirstController(); c != nil {
				println("POC-GO: mando botones", c.Buttons, "joy", c.Joyx, c.Joyy)
			} else {
				println("POC-GO: sin mando en el puerto 0")
			}
		}
	}
}
