//go:build dreamcast

// DC-002.9: el motor PINTANDO en la consola. El DC6 y la paleta los decodifica el codigo del motor tal
// como esta en el repo (d2dc6.Load + DecodeFrame, d2dat.Load, y d2util.ImgIndexToRGBA por dentro del
// resumen) y el resultado se sube al PVR como textura de 1 byte por pixel, con la paleta del motor en
// el banco 0 del hardware.
//
// No hay nada del motor copiado aca: los bytes entran horneados del MPQ (bake-dc6) y los paquetes se
// compilan con sh-elf-gccgo (build-motor-gox.sh). Lo unico que aporta la consola es el PVR.
//
// La evidencia son dos cosas:
//   - la consola serie: las lineas "MOTOR-PVR ..." (los indices y los RGBA finales con su hash) tienen
//     que ser identicas a las que imprime el comparador del host sobre los MISMOS bytes
//     (scripts/console/compare-motor-pvr.sh). El hash es lo que hace que "se ve igual" sea un numero.
//   - la captura del emulador: tiene que verse el logo, no una pantalla en negro.
package main

import (
	"kos"
	"strconv"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/motorresumen"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dat"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
)

//extern d2c_serial_init
func d2cSerialInit()

// La frontera con C va con uintptr y no con un slice: un slice de Go pasado a una funcion //extern
// tiene el ABI ambiguo (puntero, largo y capacidad como tres argumentos). Es la misma regla que sigue
// la stdlib en os.go, y el C lo vuelve a castear a puntero.
//
//extern d2c_pvr_init
func d2cPvrInit(indices uintptr, paleta uintptr, w int, h int) int

//extern d2c_pvr_quad
func d2cPvrQuad(x, y, s float32, argb uint32)

//extern d2c_pvr_ancho
func d2cPvrAncho() int

//extern d2c_pvr_alto
func d2cPvrAlto() int

func main() {
	d2cSerialInit()

	println("MOTOR-PVR arranca: el motor decodifica el DC6 y la paleta, el PVR lo pinta (DC-002.9)")
	println("MOTOR-PVR consola dc6bytes=", strconv.Itoa(len(assetDC6)), "palbytes=", strconv.Itoa(len(assetPAL)))

	d, err := d2dc6.Load(assetDC6)
	if err != nil {
		println("MOTOR-PVR FALLO parseando el DC6:", err.Error())
		colgado()
	}

	paleta, err := d2dat.Load(assetPAL)
	if err != nil {
		println("MOTOR-PVR FALLO parseando la paleta:", err.Error())
		colgado()
	}

	// el resumen: los indices (lo que ya medía DC-002.3) y el COLOR, que es lo nuevo de DC-002.9
	for _, l := range motorresumen.Lineas(d) {
		println(l)
	}

	for _, l := range motorresumen.LineasRGBA(d, paleta) {
		println(l)
	}

	if len(d.Frames) == 0 || d.Frames[0] == nil {
		println("MOTOR-PVR FALLO: el DC6 no tiene frame 0")
		colgado()
	}

	idx := d.DecodeFrame(0)
	if idx == nil {
		println("MOTOR-PVR FALLO: el frame 0 no decodifico")
		colgado()
	}

	// la paleta como 256 RGBA (4 bytes por color) para el C: sale del MISMO d2interface.Color que usa
	// el motor para convertir los indices, asi que lo que se ve en pantalla sale de la paleta del motor
	rgbaPal := make([]byte, 256*4)
	for i := 0; i < 256; i++ {
		c, err := paleta.GetColor(i)
		if err != nil {
			println("MOTOR-PVR FALLO pidiendo el color", strconv.Itoa(i), err.Error())
			colgado()
		}

		rgbaPal[i*4+0] = c.R()
		rgbaPal[i*4+1] = c.G()
		rgbaPal[i*4+2] = c.B()
		rgbaPal[i*4+3] = c.A()
	}

	kos.PvrInitDefaults()
	// violeta: se ve por los huecos transparentes (el indice 0 no se pinta)
	kos.PvrSetBgColor(0.35, 0.0, 0.55)

	f := d.Frames[0]
	r := d2cPvrInit(uintptr(unsafe.Pointer(&idx[0])), uintptr(unsafe.Pointer(&rgbaPal[0])), int(f.Width), int(f.Height))
	if r != 0 {
		println("MOTOR-PVR FALLO subiendo la textura al PVR, codigo", strconv.Itoa(r))
		colgado()
	}

	println("MOTOR-PVR textura PVR", d2cPvrAncho(), "x", d2cPvrAlto(),
		"pixeles del frame", len(idx), "bytes en VRAM (16 bits/pixel habrian sido", len(idx)*2, ")")

	blanco := kos.PlxPackColor(1, 1, 1, 1)

	var frames uint32

	for {
		kos.PvrWaitReady()
		kos.PvrSceneBegin()
		kos.PvrListBegin(kos.PVR_LIST_TR_POLY)

		// el logo: 3x arriba y 1:1 abajo, para ver el escalado del PVR sin costo de CPU
		d2cPvrQuad(40, 40, 3, blanco)
		d2cPvrQuad(40, 300, 1, blanco)

		kos.PvrListFinish()
		kos.PvrSceneFinish()

		frames++
		if frames%600 == 0 {
			println("MOTOR-PVR frames dibujados", frames)
		}
	}
}

// colgado: el programa se queda vivo para que la consola serie termine de vaciarse; el emulador se
// corta despues de los segundos que le pasa run-flycast.sh y el log se lee de ahi.
func colgado() {
	for {
	}
}
