//go:build dreamcast

// DC-002.37 -- EL ACTO 1 EN LA DREAMCAST: el primer nivel armado y pintado desde los .ds1 DEL MPQ DEL
// DISCO.
//
// LA CADENA QUE SE MIDE (la misma que usa el juego para armar un nivel):
//   MPQ del disco (ISO9660, /cd) -> d2mpq.FromFile + ReadFile -> d2ds1.Unmarshal (el parseador del
//   MOTOR, el mismo codigo que corre en el host) -> los DT1 que el PROPIO DS1 lista (ruta normalizada
//   como la normaliza el motor) -> d2dt1.LoadDT1 + d2dt1.DecodeTileGfxData (el decodificador del motor)
//   -> un lienzo de indices de 8 bits compuesto en la proyeccion ISOMETRICA del renderer del motor ->
//   paleta del ACTO leida del MPQ (768 bytes = 256 colores B,G,R) -> textura paletizada de 8 bits +
//   paleta ARGB1555 por hardware -> quad por el PVR (csrc/cshim.c).
//
// LA COMPOSICION ES EL MISMO ARCHIVO QUE EL DEL HOST (`nivel.go`): el POC de la consola y
// `console/dc/verif-acto1-237` comparten ese archivo byte a byte, y por eso el CRC-32 del lienzo de
// indices es comparable entre las dos plataformas.
//
// LOS NUMEROS (la consola serie es la unica salida):
//   - el .ds1: bytes, CRC-32, version, acto, tamano en celdas, objetos, cuantas capas trae cada grupo y
//     cuantos TILES NO VACIOS tiene cada capa;
//   - cada DT1 que el DS1 lista: bytes, CRC-32 y cuantos tiles trae;
//   - la composicion: cuantos pisos y paredes se pidieron, cuantos entraron, cuantos quedaron SIN TILE,
//     cuantos cayeron FUERA de la ventana, cuantos pixeles del lienzo no son el indice 0 y el CRC-32 del
//     lienzo;
//   - la paleta del acto y su CRC ARGB1555, y la VRAM libre;
//   - los cuadros dibujados.
//
// MUTANTE (criterio 3): con DS1_INEXISTENTE=1 (lo pone el script, recompilando) la ruta del nivel apunta a
// un .ds1 que no existe: no hay lienzo, no hay textura y la pantalla queda como estaba (el fondo del PVR).
package main

import (
	"hash/crc32"
	"kos"
	"runtime"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint

//extern d2c_tex_init
func d2cTexInit(idx uint, pal uint, w, h, pw, ph int32) int32

//extern d2c_quad
func d2cQuad(x, y, s float32)

//extern d2c_quad_solido
func d2cQuadSolido(x, y, w, h float32, argb uint)

//extern d2c_vram_libre
func d2cVramLibre() uint

//extern d2c_free_external
func d2cFreeExternal(p uintptr)

// umbralGrande: a partir de que tamaño el recolector de libgodc manda la asignacion al malloc del monton
// de C (GC_LARGE_OBJECT_THRESHOLD_KB = 64 en runtime/godc_config.h). Por encima de esto la memoria NO la
// devuelve el recolector: hay que devolverla con d2cFreeExternal o la arena se llena (medido: "fatal
// error: out of memory" despues de 3 o 4 DT1 de los 8 del town).
const umbralGrande = 64 * 1024

const (
	// El disco: el MPQ y las rutas del juego, tal como las escribe el ISO9660 (nombres exactos).
	rutaMPQ = "/cd/JUEGO/D2DATA.MPQ"

	// EL PRIMER NIVEL DEL ACTO 1: el Rogue Encampment (el nivel 1 del acto 1 en Levels.txt). Su mapa es
	// este .ds1 de la zona TOWN del acto 1, que vive en d2data.mpq junto con sus DT1.
	nivelEnMPQ = "data\\global\\tiles\\ACT1\\TOWN\\townN1.ds1"

	// La paleta del ACTO (768 bytes: 256 colores de 3 bytes en orden B, G, R).
	palEnMPQ = "data/global/palette/ACT1/pal.dat"

	// La ventana isometrica que se compone (potencia de dos: el PVR lo exige).
	ladoVentana = 512

	// La celda que queda en el centro de la ventana: la mitad del nivel (57x41), para no pintar solo una
	// punta.
	centroX, centroY = 28, 20

	// La pantalla del POC y los cuadros que avisa antes de seguir.
	ancho, alto = 640, 480
	cuadrosAMedir = 300
)

func main() {
	d2cSerialInit()

	println("HITO237: arranca (DC-002.37, el acto 1 armado y pintado desde los .ds1 del MPQ del disco)")
	println("HITO237: runtime", runtime.GOOS, runtime.GOARCH)

	// --- 1. el MPQ del disco ---
	t0 := int64(d2cMs())

	mpq, err := d2mpq.FromFile(rutaMPQ)
	if err != nil {
		println("HITO237: MPQ-ABRIR FALLO:", err.Error())
		return
	}

	defer mpq.Close()

	println("HITO237: MPQ-ABRIR OK", rutaMPQ, "ms=", int64(d2cMs())-t0)

	// --- 2. el .ds1 del nivel (el MUTANTE apunta a uno que no existe) ---
	t1 := int64(d2cMs())

	raw, err := mpq.ReadFile(nivelEnMPQ)
	if err != nil {
		println("HITO237: DS1-LEER FALLO:", nivelEnMPQ, ":", err.Error())
		println("HITO237: sin .ds1 no hay nivel: la pantalla queda como estaba (mutante)")
		pantallaVacia()

		return
	}

	println("HITO237: DS1-LEER OK bytes=", len(raw), "crc32=", crc32.ChecksumIEEE(raw), "ms=", int64(d2cMs())-t1)

	const trozoBytes = 65536

	for i := 0; i < len(raw); i += trozoBytes {
		fin := i + trozoBytes
		if fin > len(raw) {
			fin = len(raw)
		}

		println("HITO237: DS1 trozo", i/trozoBytes, "desde=", i, "bytes=", fin-i, "crc32=", crc32.ChecksumIEEE(raw[i:fin]))
	}

	d, err := d2ds1.Unmarshal(raw)
	if err != nil {
		println("HITO237: DS1-PARSE FALLO:", err.Error())
		return
	}

	println("HITO237: DS1 version=", d.Version(), "act=", int(d.Act), "w=", d.Width(), "h=", d.Height(),
		"objetos=", len(d.Objects), "files=", len(d.Files))

	cuentaCapa := func(cual string, idx int, l *d2ds1.Layer) {
		if l == nil {
			return
		}

		noVacios := 0

		for y := 0; y < l.Height(); y++ {
			for x := 0; x < l.Width(); x++ {
				t := l.Tile(x, y)
				if t == nil {
					continue
				}

				if t.Style != 0 || t.Sequence != 0 {
					noVacios++
				}
			}
		}

		println("HITO237: DS1 capa", cual, idx, "w=", l.Width(), "h=", l.Height(), "tilesNOvacios=", noVacios)
	}

	for i := 0; i < 2; i++ {
		cuentaCapa("PISO", i+1, d.GetFloor(i))
	}

	for i := 0; i < 4; i++ {
		cuentaCapa("PARED", i+1, d.GetWall(i))
	}

	for i := 0; i < 4; i++ {
		cuentaCapa("SOMBRA", i+1, d.GetShadow(i))
	}

	// --- 3. la ventana y los pedidos ---
	n := NuevoNivel(d, ladoVentana, ladoVentana, centroX, centroY)

	println("HITO237: VENTANA lado=", n.Ancho, "origen=", n.OrigenX, n.OrigenY,
		"centroCelda=", centroX, centroY)
	println("HITO237: PEDIDOS pisos=", n.PisosPedidos, "paredes=", n.ParedesPedidas, "total=", len(n.Pedidos))

	// --- 4. los DT1, DE A UNO (el town pide 4,35 MB de DT1: tenerlos todos vivos no entra en la consola) ---
	for _, f := range d.Files {
		nombre := RutaDelDT1(f)
		tDT1 := int64(d2cMs())

		rawDT1, err := LeerDelMPQ(mpq, nombre)
		if err != nil {
			println("HITO237: DT1", nombre, "FALLO:", err.Error())
			continue
		}

		dt1, err := d2dt1.LoadDT1(rawDT1)
		if err != nil {
			println("HITO237: DT1", nombre, "bytes=", len(rawDT1), "crc32=", crc32.ChecksumIEEE(rawDT1),
				"PARSE-FALLO:", err.Error())
			continue
		}

		println("HITO237: DT1", nombre, "bytes=", len(rawDT1), "crc32=", crc32.ChecksumIEEE(rawDT1),
			"tiles=", len(dt1.Tiles), "ms=", int64(d2cMs())-tDT1)

		n.Alimentar(dt1.Tiles)

		// Se sueltan, y el archivo CRUDO se devuelve AL MONTON DE C: por encima de 64 KB el recolector de
		// libgodc no lo recupera (el objeto va al malloc de C, ver d2c_free_external), asi que liberarlo a
		// mano es lo que deja la arena en el PICO DE UN ARCHIVO en vez de sumar los 4,35 MB de los ocho DT1
		// del town (medido: sin esto, "fatal error: out of memory" al tercer o cuarto archivo).
		dt1 = nil

		if len(rawDT1) > umbralGrande {
			d2cFreeExternal(uintptr(unsafe.Pointer(&rawDT1[0])))
		}

		rawDT1 = nil
	}

	n.Cerrar()

	println("HITO237: COMPUESTO pisos=", n.PisosCompuestos, "/", n.PisosPedidos,
		"paredes=", n.ParedesCompuestas, "/", n.ParedesPedidas,
		"sintilePiso=", n.PisosSinTile, "sintilePared=", n.ParedesSinTile, "fuera=", n.FueraDeVentana)
	println("HITO237: LIENZO pixelesNoVacios=", n.PixelesNoVacios(), "de", n.Ancho*n.Alto,
		"crc32=", n.CRC32())
	println("HITO237: LIENZO pixelesPiso=", n.PixelesDelPiso, "pixelesPared=", n.PixelesDePared)

	// --- 5. la paleta del acto, del mismo MPQ ---
	pb, err := LeerDelMPQ(mpq, palEnMPQ)
	if err != nil {
		println("HITO237: PALETA-LEER FALLO:", err.Error())
		return
	}

	if len(pb) < 768 {
		println("HITO237: PALETA CORTA bytes=", len(pb))
		return
	}

	pal := PaletaRGB(pb)
	pvrPal := PaletaARGB1555(pal)
	println("HITO237: PALETA bytes=", len(pb), "colores=256", "crc32=", CRCPaleta(pvrPal))

	// --- 6. el PVR: subir el lienzo como textura y pintarlo ---
	kos.PvrInitDefaults()
	kos.PvrSetBgColor(0.0, 0.0, 0.0)
	kos.PvrWaitReady()

	println("HITO237: VRAM libre antes=", int64(d2cVramLibre()))

	rc := d2cTexInit(
		uint(uintptr(unsafe.Pointer(&n.Canvas[0]))),
		uint(uintptr(unsafe.Pointer(&pvrPal[0]))),
		int32(n.Ancho), int32(n.Alto), int32(n.Ancho), int32(n.Alto),
	)
	if rc != 0 {
		println("HITO237: TEXTURA FALLO codigo=", rc)
		return
	}

	println("HITO237: TEXTURA OK w=", n.Ancho, "h=", n.Alto, "bytes=", n.Ancho*n.Alto)
	println("HITO237: VRAM libre despues=", int64(d2cVramLibre()))

	esc := float32(ancho) / float32(n.Ancho)
	if sy := float32(alto) / float32(n.Alto); sy < esc {
		esc = sy
	}

	println("HITO237: escala milesima=", int(esc*1000))
	println("HITO237: dibujando")

	cuadros := 0

	for {
		kos.PvrWaitReady()
		kos.PvrSceneBegin()
		kos.PvrListBegin(kos.PVR_LIST_TR_POLY)
		d2cQuad(0, 0, esc)
		// CONTROL del camino de dibujo: un quad de color en la esquina inferior derecha (fuera del asset de
		// 480x480). Si la captura muestra el asset negro pero este quad magenta, el que falla es la textura.
		d2cQuadSolido(520, 380, 110, 90, 0xFFFF00FF)
		kos.PvrListFinish()
		kos.PvrSceneFinish()

		cuadros++
		// una marca cada 30 cuadros CON el reloj: ademas de avisar que se esta pintando, deja medir los
		// cuadros por segundo de la consola (antes solo se avisaba a los 300 y en una corrida de 60 s el
		// aviso no llegaba: no se sabia si pintaba o se habia quedado).
		if cuadros%30 == 0 && cuadros <= cuadrosAMedir {
			println("HITO237: CUADROS", cuadros, "dibujados ms=", int64(d2cMs()))
		}
	}
}

// pantallaVacia: el camino del mutante (.ds1 que no existe). Deja el PVR con su fondo y NO sube nada.
func pantallaVacia() {
	kos.PvrInitDefaults()
	kos.PvrSetBgColor(0.0, 0.0, 0.0)
	kos.PvrWaitReady()

	println("HITO237: PANTALLA-VACIA (sin nivel, sin textura) vram=", int64(d2cVramLibre()))

	for {
		kos.PvrWaitReady()
		kos.PvrSceneBegin()
		kos.PvrListFinish()
		kos.PvrSceneFinish()
	}
}
