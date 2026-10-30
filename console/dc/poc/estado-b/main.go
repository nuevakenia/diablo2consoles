//go:build dreamcast

// DC-002.70 -- EL MAPA Y EL PERSONAJE DEL MOTOR DIBUJADOS POR LA INTERFAZ (Fase B.2).
//
// QUE PRUEBA, Y POR QUE ASI. El POC del personaje en el mapa (console/dc/poc/personaje-mapa, DC-002.38)
// dibuja con el shim de Ebitengine (una API de PC imitada sobre el PVR). Esta tarjeta es la otra mitad de la
// Fase B: el MISMO mapa y el MISMO personaje, pero dibujados contra d2interface.Surface con el backend de la
// consola (d2core/d2render/pvr) y su Renderer -- o sea el camino que va a usar el motor de verdad.
//
// LA CADENA ES LA DEL POC, SIN CAMBIOS, y por eso los numeros son comparables:
//
//	.ds1 del acto 1 en el MPQ -> d2ds1 (parseador del motor) -> los DT1 que el DS1 lista -> d2dt1 ->
//	el compositor `nivel.go` (EL MISMO archivo de DC-002.37: el build lo copia y compara su sha256) ->
//	el lienzo de INDICES del nivel y su paleta del acto;
//	el COF del heroe (AM/NU/HTH del d2char.mpq) -> sus capas .dcc/.dc6 -> d2dcc/d2dc6 -> el compuesto
//	`personaje.go` (EL MISMO archivo de DC-002.38) -> el sprite paletizado del personaje.
//
// EL DIBUJADO ES EL DE LA INTERFAZ, y esto es lo que cambia respecto del POC:
//
//	Renderer := pvr.NuevoRenderer()
//	Renderer.Run(dibujar, actualizar, 640, 480, "Diablo II (Dreamcast)")   <- el ciclo de cuadros del backend
//	dibujar(superficie d2interface.Surface)                                 <- el callback del motor
//	    superficie.Clear(negro)
//	    superficie.PushTranslation(mapaX, mapaY); superficie.RenderIndices(lienzo, 512, 512, paleta); Pop()
//	    superficie.PushTranslation(px, py); superficie.RenderSprite(personaje); Pop()
//
// NI EL POC NI EL MOTOR EN EL MEDIO: el cuadro lo compone la Surface del backend y lo pinta el Renderer.
//
// LO QUE NO ENTRA (y es la Fase D): d2gamescreen, el menu y los items. Tampoco se anima: es el PRIMER cuadro
// del POC (el de DC-002.38 con animar=0), que es contra el que se compara pixel a pixel.
//
// MEMORIA: el lienzo del cuadro vive en la VRAM (pvr.SuperficieDeLaPantalla) porque el monton de C no tiene
// el cuadro (1.228.800 B) y el mapa del nivel (262.144 B) al mismo tiempo. El POC del mapa por eso arranca
// el video ANTES de cargar el nivel: el arranque del PVR en esta consola necesita el monton vacio (medido en
// DC-002.50: se cuelga en "pvr: enabling vertical scaling" con 70.976 B libres).
package main

import (
	"hash/crc32"
	"image/color"
	"runtime"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_heap_libre
func d2cHeapLibre() uint

// init: la consola serie es la UNICA salida de texto en la Dreamcast (sin esto KOS deja el handler en NULL y
// el println de Go no sale a ninguna parte).
func init() { d2cSerialInit() }

const (
	// los DOS MPQ del juego que hacen falta (el mismo camino del POC: por ISO9660, desde el disco)
	rutaMPQ      = "/cd/JUEGO/D2DATA.MPQ"
	rutaMPQChars = "/cd/JUEGO/D2CHAR.MPQ"

	// el primer nivel del acto 1 (Rogue Encampment) y las dos paletas
	nivelEnMPQ     = "data\\global\\tiles\\ACT1\\TOWN\\townN1.ds1"
	palEnMPQ       = "data/global/palette/ACT1/pal.dat"
	palUnidadesMPQ = "data/global/palette/Units/pal.dat"

	// el COF del heroe: la amazona parada y desarmada (AM + NU + HTH)
	cofEnMPQ = "data\\global\\CHARS\\AM\\COF\\amnuhth.cof"

	// la ventana isometrica, su centro y la direccion/frame del personaje: LOS MISMOS del POC, para que las
	// capturas sean comparables
	ladoVentana      = 512
	centroX, centroY = 28, 20
	dirPersonaje     = 0
	framePersonaje   = 0

	// la pantalla logica
	ancho, alto = 640, 480
	mapaX       = (ancho - ladoVentana) / 2
	mapaY       = (alto - ladoVentana) / 2
)

// el estado del cuadro: el lienzo del nivel (indices), su paleta y el personaje. Viven fuera de main porque
// el callback de dibujado los usa cuadro a cuadro.
var (
	mapaIndices     []byte
	paletaDelMapa   [][3]uint8
	personajeSprite *pvr.Sprite
	personajeX      int
	personajeY      int

	mapaPintado bool
)

func main() {
	println("ESTADOB: arranca DC-002.70 -- el mapa y el personaje del motor por d2interface.Surface (backend PVR)")
	println("ESTADOB: monton libre al arrancar (bytes):", int(d2cHeapLibre()))

	// ============================== 0. EL VIDEO PRIMERO ==============================
	// El arranque del PVR necesita el monton vacio (medido en DC-002.50). El Renderer vuelve a llamar a
	// PantallaIniciar con el mismo tamano, que es idempotente (no hace nada).
	renderer := pvr.NuevoRenderer()

	if err := pvr.PantallaIniciar(ancho, alto); err != nil {
		println("ESTADOB: el video no arranco:", err.Error())

		return
	}

	println("ESTADOB: video andando, monton libre (bytes):", int(d2cHeapLibre()),
		"vram de la textura del cuadro:", pvr.VramDeLaPantalla())

	// ============================== 1. EL NIVEL ==============================
	mpq, err := d2mpq.FromFile(rutaMPQ)
	if err != nil {
		println("ESTADOB: MPQ-ABRIR FALLO:", err.Error())

		return
	}

	raw, err := mpq.ReadFile(nivelEnMPQ)
	if err != nil {
		println("ESTADOB: DS1-LEER FALLO:", err.Error())

		return
	}

	d, err := d2ds1.Unmarshal(raw)
	if err != nil {
		println("ESTADOB: DS1-PARSE FALLO:", err.Error())

		return
	}

	println("ESTADOB: DS1 bytes=", len(raw), "crc32=", crc32.ChecksumIEEE(raw), "version=", d.Version(),
		"act=", int(d.Act), "w=", d.Width(), "h=", d.Height(), "files=", len(d.Files))

	n := NuevoNivel(d, ladoVentana, ladoVentana, centroX, centroY)

	println("ESTADOB: VENTANA lado=", n.Ancho, "origen=", n.OrigenX, n.OrigenY,
		"pisos=", n.PisosPedidos, "paredes=", n.ParedesPedidas)

	for _, f := range d.Files {
		nombre := RutaDelDT1(f)

		rawDT1, err := LeerDelMPQ(mpq, nombre)
		if err != nil {
			println("ESTADOB: DT1", nombre, "FALLO:", err.Error())

			continue
		}

		dt1, err := d2dt1.LoadDT1(rawDT1)
		if err != nil {
			println("ESTADOB: DT1", nombre, "PARSE-FALLO:", err.Error())

			continue
		}

		println("ESTADOB: DT1", nombre, "bytes=", len(rawDT1), "tiles=", len(dt1.Tiles))

		n.Alimentar(dt1.Tiles)

		dt1 = nil
		rawDT1 = nil
	}

	n.Cerrar()

	println("ESTADOB: COMPUESTO pisos=", n.PisosCompuestos, "/", n.PisosPedidos,
		"paredes=", n.ParedesCompuestas, "/", n.ParedesPedidas,
		"sintilePiso=", n.PisosSinTile, "sintilePared=", n.ParedesSinTile, "fuera=", n.FueraDeVentana)
	println("ESTADOB: LIENZO pixelesNoVacios=", n.PixelesNoVacios(), "de", n.Ancho*n.Alto,
		"crc32=", n.CRC32())

	// la paleta del acto (768 bytes B,G,R) al formato del PVR
	pb, err := LeerDelMPQ(mpq, palEnMPQ)
	if err != nil || len(pb) < 768 {
		println("ESTADOB: PALETA-LEER FALLO:", palEnMPQ)

		return
	}

	palActo := PaletaRGB(pb)

	mapaIndices = n.Canvas
	// la paleta de 8 bits del acto: el mapa va por el camino por SOFTWARE del backend, que es RGBA, asi que
	// cuantizarla a los 5 bits del PVR solo perderia color (y haria que la captura no coincida con la del host)
	paletaDelMapa = palActo[:]

	println("ESTADOB: PALETA acto colores=256 crc32=", CRCPaleta(PaletaARGB1555(palActo)),
		"monton libre (bytes):", int(d2cHeapLibre()))

	// ============================== 2. EL PERSONAJE ==============================
	mc, err := d2mpq.FromFile(rutaMPQChars)
	if err != nil {
		println("ESTADOB: MPQ-CHARS-ABRIR FALLO:", err.Error())

		return
	}

	rawCof, err := mc.ReadFile(cofEnMPQ)
	if err != nil {
		println("ESTADOB: COF-LEER FALLO:", err.Error())

		return
	}

	cof, err := d2cof.Unmarshal(rawCof)
	if err != nil {
		println("ESTADOB: COF-PARSE FALLO:", err.Error())

		return
	}

	per := NuevoPersonaje(cof, dirPersonaje, framePersonaje)

	println("ESTADOB: COF bytes=", len(rawCof), "crc32=", crc32.ChecksumIEEE(rawCof),
		"capas=", cof.NumberOfLayers, "framesPorDir=", cof.FramesPerDirection,
		"dirs=", cof.NumberOfDirections)

	for i := range cof.CofLayers {
		encontrada := false
		ultimaRuta, ultimoErr := "", ""

		for _, ext := range []string{"dcc", "dc6"} {
			nombre := per.RutaDeLaCapa(i, ext)

			crudo, err := LeerDelMPQ(mc, nombre)
			if err != nil {
				ultimaRuta, ultimoErr = nombre, err.Error()

				continue
			}

			if ext == "dcc" {
				dd, err := d2dcc.Load(crudo)
				if err != nil {
					println("ESTADOB: CAPA", per.Claves[i], "PARSE-FALLO:", err.Error())

					break
				}

				caja := dd.Directions[dirPersonaje].Box
				pix := dd.Directions[dirPersonaje].Frames[framePersonaje].PixelData

				per.Agregar(per.Tipos[i], per.Claves[i], caja.Left, caja.Top, caja.Width, caja.Height, pix)
				encontrada = true

				dd = nil
				crudo = nil

				break
			}

			dc, err := d2dc6.Load(crudo)
			if err != nil {
				println("ESTADOB: CAPA", per.Claves[i], "PARSE-FALLO:", err.Error())

				break
			}

			fr := dc.Frames[framePersonaje]
			pix := dc.DecodeFrame(framePersonaje)

			per.Agregar(per.Tipos[i], per.Claves[i], int(fr.OffsetX), int(fr.OffsetY),
				int(fr.Width), int(fr.Height), pix)
			encontrada = true

			dc = nil
			crudo = nil

			break
		}

		if !encontrada {
			println("ESTADOB: CAPA", per.Claves[i], "SIN-GRAFICO ruta=", ultimaRuta, "err=", ultimoErr)
		}

		runtime.GC()
	}

	per.Componer()

	println("ESTADOB: PERSONAJE compuesto=", per.Ancho, "x", per.Alto, "esquina=", per.Izq, per.Arriba,
		"capasDibujadas=", per.CapasDibujadas, "sinGrafico=", per.CapasSinGrafico,
		"pixelesNoVacios=", per.PixelesNoVacios, "crc32=", per.CRC32())

	// la paleta de las UNIDADES (la de los personajes): el compuesto del COF esta en indices de ESA paleta
	pbu, err := LeerDelMPQ(mpq, palUnidadesMPQ)
	if err != nil || len(pbu) < 768 {
		println("ESTADOB: PALETA-UNIDADES FALLO:", palUnidadesMPQ)

		return
	}

	if per.Ancho > 0 && per.Alto > 0 {
		personajeSprite = pvr.NuevoSprite(per.Ancho, per.Alto, per.Lienzo, PaletaARGB1555(PaletaRGB(pbu)))
	}

	// el ancla del personaje: la MISMA cuenta del POC (la celda del motor proyectada al lienzo del nivel)
	est := NuevoEstado(float64(centroX), float64(centroY), false)
	cx, cy := est.Celda()
	ax, ay := PixelDeLaCelda(cx, cy, n.OrigenX, n.OrigenY)

	personajeX, personajeY = mapaX+ax+per.Izq, mapaY+ay+per.Arriba

	println("ESTADOB: POS-INICIAL celda=", cx, cy, "ancla=", ax, ay,
		"esquinaDelCompuesto=", per.Izq, per.Arriba, "pantalla=", personajeX, personajeY)
	println("ESTADOB: monton libre con el nivel y el personaje cargados (bytes):", int(d2cHeapLibre()))

	// ============================== 3. EL CICLO DE CUADROS DE LA INTERFAZ ==============================
	if err := renderer.Run(dibujar, actualizar, ancho, alto, "Diablo II (Dreamcast)"); err != nil {
		println("ESTADOB: el ciclo de cuadros salio con error:", err.Error())
	}
}

// dibujar es EL CALLBACK DEL MOTOR: compone el cuadro en la Surface del backend. El primer cuadro pinta el
// mapa y el personaje; los demas no repintan (la pantalla se conserva entre cuadros, igual que en el POC con
// animar=0: volver a copiar 512x512 del mapa 30 veces por segundo no mide nada nuevo).
func dibujar(superficie d2interface.Surface) error {
	if mapaPintado {
		return nil
	}

	malla, ok := superficie.(*pvr.Surface)
	if !ok {
		return errorDelPoc("el Renderer no entrego una Surface del backend PVR")
	}

	malla.Clear(color.RGBA{R: 0, G: 0, B: 0, A: 255})

	// 1) EL MAPA: el lienzo de indices del nivel, expandido al vuelo con su paleta (RenderIndices: el mapa es
	//    512x512 y su cache RGBA no entra en el monton)
	malla.PushTranslation(mapaX, mapaY)
	malla.RenderIndices(mapaIndices, ladoVentana, ladoVentana, paletaDelMapa)
	malla.Pop()

	// 2) EL PERSONAJE ENCIMA, en su esquina iso
	if personajeSprite != nil {
		malla.PushTranslation(personajeX, personajeY)
		malla.RenderSprite(personajeSprite)
		malla.Pop()
	}

	mapaPintado = true

	// ============================== las medidas del cuadro ==============================
	pixeles := malla.Imagen().Pixeles()
	llenos := 0

	for j := 0; j+3 < len(pixeles); j += 4 {
		if pixeles[j] != 0 || pixeles[j+1] != 0 || pixeles[j+2] != 0 {
			llenos++
		}
	}

	anchoReal, altoReal := superficie.GetSize()

	println("ESTADOB: CUADRO pixelesNoVacios=", llenos, "de", anchoReal*altoReal,
		"crc32=", int(crc32.ChecksumIEEE(pixeles)),
		"tamano=", anchoReal, altoReal,
		"lienzoEnVram=", pvr.VramDelLienzo(),
		"montonLibre=", int(d2cHeapLibre()))

	return nil
}

// actualizar es el otro callback del motor: aca no hay nada que avanzar (es el primer cuadro del POC).
func actualizar() error { return nil }

// errorDelPoc: un error con nombre, para que el que llama pueda distinguirlo de uno del motor.
type errorDelPoc string

func (e errorDelPoc) Error() string { return string(e) }
