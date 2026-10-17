//go:build dreamcast

// DC-001.5: los tres numeros del go/no-go, MEDIDOS en la emulada (no deducidos).
//
//   (a) el tamano del ELF: se mide en el host con sh-elf-size sobre ESTE mismo ELF (los segmentos que
//       tienen que caber en los 16 MB). El POC no lo puede saber de si mismo.
//   (b) la RAM libre con el motor cargado: los MISMOS bytes entran al motor (d2dc6.Load + DecodeFrame y
//       d2dat.Load, la paleta) y el POC imprime el heap de KOS ANTES y DESPUES (mallinfo: arena, libre,
//       usada) mas la VRAM libre del PVR. El "motor cargado" de esta fase es el asset real del motor
//       (140.740 y 768 bytes del MPQ) decodificado por el codigo del motor compilado para SH-4.
//   (c) el frame dibujando 300 sprites paletizados: 300 quads de la textura PAL8 del motor por frame
//       (rejilla de 20x15, todos dentro de la pantalla en los dos modos), 5 s de reloj a 640x480 y 5 s a
//       320x240, con el modo que quedo EN EFECTO leido del hardware (vid_mode) y con el reloj de KOS.
//       Los contadores de rendimiento del SH-4 se arrancan y se leen: si el emulador no los implementa,
//       el numero que sale es 0 y eso tambien es un dato medido, no una suposicion.
//
// El asset entra CRUDO del MPQ (lo hornea build-motor-dc6.sh en el directorio de build): aca no hay nada
// del motor copiado ni ningun numero escrito a mano.
package main

import (
	"kos"
	"strconv"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dat"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_us
func d2cUs() uint64

//extern d2c_mem_libre
func d2cMemLibre() uint32

//extern d2c_mem_arena
func d2cMemArena() uint32

//extern d2c_mem_usada
func d2cMemUsada() uint32

//extern d2c_vram_libre
func d2cVramLibre() uint32

//extern d2c_pvr_init
func d2cPvrInit(indices uintptr, paleta uintptr, w int, h int) int

//extern d2c_pvr_quad
func d2cPvrQuad(x, y, s float32, argb uint32)

//extern d2c_pvr_ancho
func d2cPvrAncho() int

//extern d2c_pvr_alto
func d2cPvrAlto() int

//extern d2c_ram_tope
func d2cRamTope() uint32

//extern d2c_modo
func d2cModo(dm int, pm int) int

//extern d2c_modo_ancho
func d2cModoAncho() int

//extern d2c_modo_alto
func d2cModoAlto() int

const (
	spritesPorFrame = 300
	columnas        = 20
	filas           = 15 // 20 x 15 = 300
	pasoX           = 15 // px entre sprites: la rejilla entera (300x105 px) entra en 320x240
	pasoY           = 15
	escala          = 0.13
	microsFase      = 5 * 1000000 // 5 s de reloj por modo
	// La CPU del SH-4 son 200 MHz: los ciclos que se imprimen salen de multiplicar el tiempo medido por
	// esto. Va marcado como DERIVADO en la linea, al lado del contador del hardware (que en el emulador
	// no esta implementado y da 0).
	ciclosPorMicrosegundo = 200
)

// el modo de video de KOS (dc/video.h): DM_320x240 y PM_RGB565. Se pasan por numero porque son los
// valores del hardware, y el POC despues LEE el modo que quedo en efecto.
const (
	dm320x240 = 0x1000
	pmRGB565  = 1
)

func itoa(n int) string      { return strconv.Itoa(n) }
func u64s(n uint64) string   { return strconv.FormatUint(n, 10) }

// memoria imprime el estado del heap de KOS y de la VRAM del PVR. mallinfo va en bytes.
func memoria(etiqueta string) {
	println("MOTOR-300 mem", etiqueta,
		"libre_bytes=", u64s(uint64(d2cMemLibre())),
		"arena_bytes=", u64s(uint64(d2cMemArena())),
		"usada_bytes=", u64s(uint64(d2cMemUsada())),
		"vram_libre_bytes=", u64s(uint64(d2cVramLibre())))
}

// correrFase dibuja 300 sprites por frame y pone numero al frame. El modo que se imprime es el que quedo
// en efecto (vid_mode del hardware), no el que se pidio.
//
// vblank=true espera el vblank entre frames (asi corre un juego sincronizado: el tope es la tasa de la
// pantalla) y vblank=false NO espera: mide el frame A FONDO, que es el numero que contesta "entra en 60
// Hz / en 30 Hz" (lo que la maquina puede empujar, no lo que la pantalla deja).
func correrFase(nombre string, vblank bool) {
	ancho, alto := d2cModoAncho(), d2cModoAlto()
	blanco := kos.PlxPackColor(1, 1, 1, 1)
	ritmo := "a_fondo"
	if vblank {
		ritmo = "vblank"
	}

	println("MOTOR-300 FASE", nombre, ritmo, "arranca modo_ancho=", itoa(ancho), "modo_alto=", itoa(alto),
		"sprites=", itoa(spritesPorFrame))

	kos.PerfCntrStart()

	t0 := d2cUs()
	var frames uint64
	var usEnSprites uint64
	for d2cUs()-t0 < microsFase {
		if vblank {
			kos.PvrWaitReady()
		}
		kos.PvrSceneBegin()
		kos.PvrListBegin(kos.PVR_LIST_TR_POLY)

		// lo que la CPU se gasta en SOMETER los 300 sprites se mide aparte: el resto del frame lo
		// manda el render del PVR (que en el emulador no es el del hardware), y confundir los dos
		// seria mentir con un numero que parece de CPU y es del emulador.
		ts0 := d2cUs()
		for i := 0; i < spritesPorFrame; i++ {
			x := float32(8 + (i%columnas)*pasoX)
			y := float32(8 + (i/columnas)*pasoY)
			d2cPvrQuad(x, y, escala, blanco)
		}
		usEnSprites += d2cUs() - ts0

		kos.PvrListFinish()
		kos.PvrSceneFinish()
		frames++
	}

	t1 := d2cUs()
	contador := kos.PerfCntrCycles()
	total := t1 - t0
	usPorFrame := total / frames

	println("MOTOR-300 FASE", nombre, ritmo, "fin",
		"modo_ancho=", itoa(ancho), "modo_alto=", itoa(alto),
		"sprites_por_frame=", itoa(spritesPorFrame),
		"frames=", u64s(frames),
		"us_total=", u64s(total),
		"us_por_frame=", u64s(usPorFrame),
		"ms_por_frame=", u64s(usPorFrame/1000),
		"fps=", u64s(1000000/usPorFrame),
		"us_cpu_sometiendo_300_sprites=", u64s(usEnSprites/frames),
		"ciclos_por_frame_derivados=", u64s(usPorFrame*ciclosPorMicrosegundo),
		"contador_sh4_ciclos=", u64s(contador))
}

func main() {
	d2cSerialInit()

	println("MOTOR-300 arranca: los tres numeros del go/no-go (DC-001.5)")
	println("MOTOR-300 sprites_por_frame=", itoa(spritesPorFrame),
		"rejilla=", itoa(columnas), "x", itoa(filas),
		"paso_px=", itoa(pasoX), "x", itoa(pasoY),
		"escala=", itoa(int(escala*100)), "centesimas")

	// (b) antes de cargar nada
	memoria("antes")

	// el motor cargado: los mismos bytes crudos del MPQ y el mismo codigo del motor que en el host
	d, err := d2dc6.Load(assetDC6)
	if err != nil {
		println("MOTOR-300 FALLO parseando el DC6:", err.Error())
		colgado()
	}

	paleta, err := d2dat.Load(assetPAL)
	if err != nil {
		println("MOTOR-300 FALLO parseando la paleta:", err.Error())
		colgado()
	}

	idx := d.DecodeFrame(0)
	if idx == nil {
		println("MOTOR-300 FALLO: el frame 0 no decodifico")
		colgado()
	}

	rgbaPal := make([]byte, 256*4)
	for i := 0; i < 256; i++ {
		c, err := paleta.GetColor(i)
		if err != nil {
			println("MOTOR-300 FALLO pidiendo el color", itoa(i), err.Error())
			colgado()
		}

		rgbaPal[i*4+0] = c.R()
		rgbaPal[i*4+1] = c.G()
		rgbaPal[i*4+2] = c.B()
		rgbaPal[i*4+3] = c.A()
	}

	f := d.Frames[0]

	// (b) con el motor cargado: el DC6 en memoria, su frame 0 decodificado y la paleta
	memoria("con_el_motor_cargado")
	println("MOTOR-300 motor dc6_bytes=", itoa(len(assetDC6)), "pal_bytes=", itoa(len(assetPAL)),
		"frame0_pixeles=", itoa(len(idx)), "frame0_px=", itoa(int(f.Width)), "x", itoa(int(f.Height)))

	// el PVR con la textura PAL8 del motor (1 byte por pixel + la paleta del motor en el banco 0)
	kos.PvrInitDefaults()
	kos.PvrSetBgColor(0.35, 0.0, 0.55)

	if r := d2cPvrInit(uintptr(unsafe.Pointer(&idx[0])), uintptr(unsafe.Pointer(&rgbaPal[0])), int(f.Width), int(f.Height)); r != 0 {
		println("MOTOR-300 FALLO subiendo la textura al PVR, codigo", itoa(r))
		colgado()
	}

	println("MOTOR-300 textura PVR", itoa(d2cPvrAncho()), "x", itoa(d2cPvrAlto()),
		"sprite_px=", itoa(int(float32(d2cPvrAncho())*escala)), "x", itoa(int(float32(d2cPvrAlto())*escala)),
		"vram_libre_despues_de_la_textura_bytes=", u64s(uint64(d2cVramLibre())))

	// (c) fase 1: el modo por defecto del POC (el mismo que fijo DC-002.9), primero sincronizado al
	// vblank (como corre un juego) y despues a fondo (lo que la maquina puede empujar)
	correrFase("default", true)
	correrFase("default", false)

	// (c) fase 2: a 320x240, que es la pregunta que el plan del go/no-go quiere contestar (la mitad de
	// pixeles de la pantalla). El cambio de modo reinicia la VRAM, asi que la textura se vuelve a subir.
	println("MOTOR-300 cambiando a 320x240")
	if r := d2cModo(dm320x240, pmRGB565); r != 0 {
		println("MOTOR-300 FALLO cambiando de modo, codigo", itoa(r))
		colgado()
	}

	if r := d2cPvrInit(uintptr(unsafe.Pointer(&idx[0])), uintptr(unsafe.Pointer(&rgbaPal[0])), int(f.Width), int(f.Height)); r != 0 {
		println("MOTOR-300 FALLO subiendo la textura al PVR despues del cambio de modo, codigo", itoa(r))
		colgado()
	}

	correrFase("320x240", true)
	correrFase("320x240", false)

	// (b) lo ultimo: cuanto le queda de RAM al programa CON EL MOTOR CARGADO y el PVR andando. Se mide
	// al final a proposito (pedirla toda fragmenta el heap) y se libera enseguida.
	println("MOTOR-300 RAM-TOPE con_el_motor_cargado_y_el_pvr_andando libre_bytes=", u64s(uint64(d2cRamTope())),
		"vram_libre_bytes=", u64s(uint64(d2cVramLibre())))

	println("MOTOR-300 fin")
	colgado()
}

// colgado: el programa se queda vivo para que la consola serie termine de vaciarse (el emulador se corta
// despues de los segundos que le pasa run-flycast.sh y el log se lee de ahi).
func colgado() {
	for {
	}
}
