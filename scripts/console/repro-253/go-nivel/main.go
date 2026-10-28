//go:build dreamcast

// DC-002.58 -- EL ESLABON DEL NIVEL de la escalera de DC-002.53 (`scripts/console/repro-253/`).
//
// QUE MIDE: el TRABAJO PREVIO AL VIDEO del POC, solo. Abre el `D2DATA.MPQ` del disco por ISO9660, lee el
// DS1 del acto 1, carga sus DT1 uno por uno, compone el nivel con el compositor del POC (`nivel.go`, copiado
// TAL CUAL) y recien despues enciende el PVR y pinta. Sin personaje, sin bicho, sin control.
//
// POR QUE: los tres eslabones anteriores (`kos`, `minimo`, `churn`) PASAN. O sea que ni KOS, ni Flycast, ni
// libgodc, ni el estado del monton alcanzan para romper el arranque del PVR. Lo que queda por partir en dos es
// el camino del POC: lo que hace ANTES de encender el video, o el tamano/forma de SU binario. Este eslabon
// hace exactamente el trabajo previo al video (que es el sospechoso) con un binario chiquito.
//
// LAS MARCAS SON LAS DEL POC, con el prefijo HITO258 en vez de HITO238 para no confundir los dos logs:
//
//	HITO258: MPQ-ABRIR OK ... ms=
//	HITO258: DS1 version= act= w= h= objetos= files=
//	HITO258: COMPUESTO pisos= / paredes=
//	HITO258: LIENZO pixelesNoVacios= de
//	HITO258: MEMORIA etapa= libre=
//	HITO258: PVR-INIT / PVR-SUBIR / CUADRO n= de 60 dibujando=1
//
// Los numeros del nivel tienen que ser LOS MISMOS que los del POC del disco (medidos el 2026-10-25:
// DS1 act=1 w=57 h=41 objetos=41, COMPUESTO pisos=1982/1982 paredes=330/330, LIENZO pixelesNoVacios=193538
// de 262144); si no dan igual, el eslabon no esta midiendo el mismo camino y hay que decirlo, no taparlo.
//
// QUE NO HACE: no modifica el POC (`console/dc/poc/personaje-mapa`), no toca `~/dreamcast/libgodc` y no usa
// ebiten: pinta con el shim de pantalla de la stdlib (`console/dc/std/csrc/pvr_pantalla.c`), que es el mismo
// que usa `go-minimo` y el que va a seguir andando cuando el resto se caiga.
package main

import (
	"hash/crc32"
	"runtime"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"kos"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint

//extern d2c_vram_libre
func d2cVramLibre() uint

//extern d2c_heap_libre
func d2cHeapLibre() uint

//extern d2c_free_external
func d2cFreeExternal(p uintptr)

//extern d2c_pvr_pantalla_init
func d2cPvrPantallaInit(w, h int) int

//extern d2c_pvr_pantalla_subir
func d2cPvrPantallaSubir(rgba uintptr, w, h int) int

//extern d2c_pvr_pantalla_pintar
func d2cPvrPantallaPintar()

const (
	// las MISMAS rutas y la MISMA ventana que el POC: si cambian, los numeros no son comparables.
	rutaMPQ    = "/cd/JUEGO/D2DATA.MPQ"
	nivelEnMPQ = "data" + barraInvertida + "global" + barraInvertida + "tiles" + barraInvertida + "ACT1" + barraInvertida + "TOWN" + barraInvertida + "townN1.ds1"
	palEnMPQ   = "data/global/palette/ACT1/pal.dat"

	ladoVentana      = 512
	centroX, centroY = 28, 20

	// la pantalla logica del POC y donde se pega la ventana del nivel (centrada).
	ancho, alto = 640, 480
	mapaX       = (ancho - ladoVentana) / 2
	mapaY       = (alto - ladoVentana) / 2

	// umbralGrande: a partir de que tamano el recolector de libgodc manda la asignacion al monton de C (64 KB,
	// los mismos que usa el POC para devolver los DT1 leidos).
	umbralGrande = 64 * 1024

	// cuadrosConMarca: cuantos cuadros se marcan uno por uno (el criterio del eslabon). Despues sigue
	// pintando y deja un latido cada latidoCuadros, para que la captura tenga algo que ver: un eslabon que
	// sale limpio a los 2 s no deja ni una captura con el nivel.
	cuadrosConMarca = 60
	latidoCuadros   = 60
)

func main() {
	d2cSerialInit()

	println("HITO258: arranca (DC-002.58, el eslabon del NIVEL: mpq + DS1 compuestos y despues el PVR)")
	println("HITO258: runtime", runtime.GOOS, runtime.GOARCH)

	// ============================== 1. el nivel (lo mismo que el POC, en el mismo orden) ==============================
	t0 := int64(d2cMs())

	mpq, err := d2mpq.FromFile(rutaMPQ)
	if err != nil {
		println("HITO258: MPQ-ABRIR FALLO:", err.Error())

		return
	}

	println("HITO258: MPQ-ABRIR OK", rutaMPQ, "ms=", int64(d2cMs())-t0)

	t1 := int64(d2cMs())

	raw, err := mpq.ReadFile(nivelEnMPQ)
	if err != nil {
		println("HITO258: DS1-LEER FALLO:", err.Error())

		return
	}

	println("HITO258: DS1-LEER OK bytes=", len(raw), "crc32=", crc32.ChecksumIEEE(raw), "ms=", int64(d2cMs())-t1)

	d, err := d2ds1.Unmarshal(raw)
	if err != nil {
		println("HITO258: DS1-PARSE FALLO:", err.Error())

		return
	}

	println("HITO258: DS1 version=", d.Version(), "act=", int(d.Act), "w=", d.Width(), "h=", d.Height(),
		"objetos=", len(d.Objects), "files=", len(d.Files))

	n := NuevoNivel(d, ladoVentana, ladoVentana, centroX, centroY)

	println("HITO258: VENTANA lado=", n.Ancho, "origen=", n.OrigenX, n.OrigenY, "centroCelda=", centroX, centroY)
	println("HITO258: PEDIDOS pisos=", n.PisosPedidos, "paredes=", n.ParedesPedidas, "total=", len(n.Pedidos))

	// los DT1, DE A UNO (el town pide 4,35 MB de DT1: tenerlos todos vivos no entra en la consola)
	for _, f := range d.Files {
		nombre := RutaDelDT1(f)
		tDT1 := int64(d2cMs())

		rawDT1, err := LeerDelMPQ(mpq, nombre)
		if err != nil {
			println("HITO258: DT1", nombre, "FALLO:", err.Error())

			continue
		}

		dt1, err := d2dt1.LoadDT1(rawDT1)
		if err != nil {
			println("HITO258: DT1", nombre, "bytes=", len(rawDT1), "PARSE-FALLO:", err.Error())

			continue
		}

		println("HITO258: DT1", nombre, "bytes=", len(rawDT1), "crc32=", crc32.ChecksumIEEE(rawDT1),
			"tiles=", len(dt1.Tiles), "ms=", int64(d2cMs())-tDT1)

		n.Alimentar(dt1.Tiles)

		dt1 = nil

		if len(rawDT1) > umbralGrande {
			d2cFreeExternal(uintptrOf(rawDT1))
		}

		rawDT1 = nil
	}

	n.Cerrar()

	memoria("nivel")

	println("HITO258: COMPUESTO pisos=", n.PisosCompuestos, "/", n.PisosPedidos,
		"paredes=", n.ParedesCompuestas, "/", n.ParedesPedidas,
		"sintilePiso=", n.PisosSinTile, "sintilePared=", n.ParedesSinTile, "fuera=", n.FueraDeVentana)
	println("HITO258: LIENZO pixelesNoVacios=", n.PixelesNoVacios(), "de", n.Ancho*n.Alto, "crc32=", n.CRC32())
	println("HITO258: LIENZO pixelesPiso=", n.PixelesDelPiso, "pixelesPared=", n.PixelesDePared)

	// ============================== 2. el nivel a los pixeles de la pantalla ==============================
	tPal := int64(d2cMs())

	pb, err := LeerDelMPQ(mpq, palEnMPQ)
	if err != nil || len(pb) < 768 {
		println("HITO258: PALETA-LEER FALLO:", palEnMPQ)

		return
	}

	palActo := PaletaRGB(pb)

	println("HITO258: PALETA acto bytes=", len(pb), "colores=256", "crc32=", CRCPaleta(PaletaARGB1555(palActo)),
		"ms=", int64(d2cMs())-tPal)

	// el MPQ ya no hace falta: se cierra EXPLICITAMENTE (en la consola la RAM es una sola y el indice del
	// d2data tiene que volver antes de pedir el resto). El POC hace lo mismo.
	mpq.Close()

	memoria("mpq-cerrado")

	// la ventana del nivel en una pantalla de 640x480, recortando lo que se sale (mapaY es NEGATIVO: la
	// ventana de 512 no entra en los 480 de alto, igual que en el POC).
	rgba := make([]byte, ancho*alto*4)

	for y := 0; y < ladoVentana; y++ {
		sy := mapaY + y
		if sy < 0 || sy >= alto {
			continue
		}

		for x := 0; x < ladoVentana; x++ {
			sx := mapaX + x
			if sx < 0 || sx >= ancho {
				continue
			}

			// ColorDelIndice del POC: el indice 0 va negro (en la consola es transparente sobre fondo negro).
			r, g, b := ColorDelIndice(palActo, n.Canvas[y*ladoVentana+x])
			o := (sy*ancho + sx) * 4
			rgba[o+0], rgba[o+1], rgba[o+2], rgba[o+3] = r, g, b, 255
		}
	}

	// cuantos pixeles de la pantalla quedaron pintados: el mismo instrumento del LIENZO, pero sobre el buffer que
	// se sube a la VRAM. El nivel solo ocupa una parte de los 640x480; el resto queda negro (el fondo del PVR).
	noVacios := 0
	for i := 0; i < len(rgba); i += 4 {
		if rgba[i] != 0 || rgba[i+1] != 0 || rgba[i+2] != 0 {
			noVacios++
		}
	}

	println("HITO258: PANTALLA pixelesNoVacios=", noVacios, "de", ancho*alto)

	// DC-002.56: con el PVR APAGADO esta lectura devuelve 0 (pvr_mem_available corta en `if(!pvr_mem_base)`).
	// Se deja igual que en el POC para que la marca sea comparable, y se documenta aca para que nadie la lea
	// como "no hay VRAM".
	println("HITO258: MAPA-IMAGEN bytes=", ancho*alto*4, "vram_libre=", int64(d2cVramLibre()))

	// los pixeles ya estan en el buffer de la pantalla y el lienzo de indices no se toca mas.
	n.Canvas = nil
	n.Scratch = nil

	runtime.GC()

	memoria("mapa-en-imagen")

	// ============================== 3. recien ahora el PVR ==============================
	tPVR := int64(d2cMs())

	if r := d2cPvrPantallaInit(ancho, alto); r != 0 {
		println("HITO258: PVR-INIT FALLO r=", r, "ms=", int64(d2cMs())-tPVR)

		return
	}

	println("HITO258: PVR-INIT OK ms=", int64(d2cMs())-tPVR)

	kos.PvrSetBgColor(0, 0, 0)

	// el orden de un cuadro es el del backend del motor (`console/dc/engine/ebiten/v2/ebiten.go`): esperar el
	// cuadro, abrir la escena y la lista OPACA (la pantalla tapa todo), pintar y cerrar.
	for i := 0; ; i++ {
		kos.PvrWaitReady()
		kos.PvrSceneBegin()
		kos.PvrListBegin(kos.PVR_LIST_OP_POLY)

		if i == 0 {
			r := d2cPvrPantallaSubir(uintptrOf(rgba), ancho, alto)
			println("HITO258: PVR-SUBIR r=", r, "ms=", int64(d2cMs())-tPVR)

			if r != 0 {
				kos.PvrListFinish()
				kos.PvrSceneFinish()

				return
			}
		}

		d2cPvrPantallaPintar()

		kos.PvrListFinish()
		kos.PvrSceneFinish()

		switch {
		case i < cuadrosConMarca:
			println("HITO258: CUADRO n=", i+1, "de", cuadrosConMarca, "dibujando=1 ms=", int64(d2cMs()))
		case i%latidoCuadros == 0:
			println("HITO258: LATIDO cuadros=", i+1, "dibujando=1 ms=", int64(d2cMs()))
		}
	}
}

// memoria: cuanta RAM queda libre (bytes) al empezar una etapa. Es el mismo instrumento del POC: en la consola
// hay UN solo monton y este numero es el que dice si algo entra o no.
func memoria(etapa string) {
	println("HITO258: MEMORIA etapa=", etapa, "libre=", int64(d2cHeapLibre()))
}

// uintptrOf: la direccion de los bytes de un slice (para devolverlos al monton de C, como el POC).
func uintptrOf(b []byte) uintptr {
	if len(b) == 0 {
		return 0
	}

	return uintptr(unsafe.Pointer(&b[0]))
}
