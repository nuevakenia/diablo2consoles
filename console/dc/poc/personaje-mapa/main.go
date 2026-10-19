//go:build dreamcast

// DC-002.38 -- EL PERSONAJE EN EL MAPA, MOVIENDOSE CON EL CONTROL DE LA DREAMCAST.
//
// LA CADENA QUE SE MIDE (la misma que usa el juego, con los decodificadores y el compositor del MOTOR):
//
//	1. el NIVEL: el .ds1 del acto 1 del MPQ del disco -> d2ds1 (el parseador del motor) -> los DT1 que el
//	   propio DS1 lista -> d2dt1 -> el compositor `nivel.go` (el MISMO archivo que el verificador del host);
//	2. el PERSONAJE: el COF del heroe (data\global\CHARS\AM\COF\amnuhth.cof, del d2char.mpq) -> las capas y
//	   su ORDEN DE DIBUJO -> cada capa a su .dcc (o .dc6) -> d2dcc/d2dc6 (los decodificadores del motor) ->
//	   el compuesto `personaje.go` (el MISMO archivo que el verificador del host);
//	3. la POSICION: d2vector.Position (subtiles, 5 por celda) que avanza con la velocidad del motor
//	   (d2path: 9,375 subtiles por segundo caminando) en la direccion que pide el CONTROL de la Dreamcast;
//	4. el DIBUJO: el mapa y el personaje ENCIMA, por el camino del motor (ebiten.Image + DrawImage), que es
//	   el loop que va a correr el juego (ebiten.RunGame).
//
// LOS NUMEROS (la consola serie es la unica salida; son los mismos que imprime
// `console/dc/verif-personaje-238`, uno por uno):
//
//	- el .ds1 y los DT1 (bytes y CRC-32) y la composicion del nivel (pedidos/compuestos/sin tile/fuera,
//	  pixeles no vacios y CRC-32 del lienzo);
//	- el COF (bytes, CRC-32, capas, frames, direcciones) y cada capa del personaje (bytes, CRC-32, caja,
//	  pixeles no vacios);
//	- el compuesto del personaje (tamano, esquina, capas dibujadas, pixeles no vacios, CRC-32);
//	- la velocidad del motor y la POSICION cuadro a cuadro: subtiles, celda, tile y el pixel del ancla en la
//	  ventana y en la pantalla;
//	- la tabla del CONTROL (los bits del control de la Dreamcast, como en DC-002.11), cada vez que cambia.
//
// MUTANTE (criterio 3): con CONTROL_SORDO=true (lo pone el script, recompilando) el control SE LEE y SE
// INFORMA, pero NO se engancha al movimiento: con las mismas teclas apretadas, la posicion del motor no
// cambia (la distancia caminada queda en 0). Es el mutante de la PIEZA que se esta probando (el enganche
// control -> movimiento), no del dibujado.
package main

import (
	"hash/crc32"
	"runtime"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint

//extern d2c_vram_libre
func d2cVramLibre() uint

//extern d2c_free_external
func d2cFreeExternal(p uintptr)

const (
	// El disco: los DOS MPQ del juego que hacen falta, por ISO9660. El mapa y las paletas estan en
	// d2data.mpq; los graficos de los personajes, en d2char.mpq (medido: en d2data.mpq la carpeta chars
	// solo trae los COF de los mercenarios y las pantallas de seleccion).
	rutaMPQ      = "/cd/JUEGO/D2DATA.MPQ"
	rutaMPQChars = "/cd/JUEGO/D2CHAR.MPQ"

	// EL PRIMER NIVEL DEL ACTO 1 (el Rogue Encampment): el mismo .ds1 de DC-002.37.
	nivelEnMPQ = "data\\global\\tiles\\ACT1\\TOWN\\townN1.ds1"

	// La paleta del ACTO (mapa) y la de las UNIDADES (personajes): 768 bytes cada una, B,G,R.
	palEnMPQ      = "data/global/palette/ACT1/pal.dat"
	palUnidadesMPQ = "data/global/palette/Units/pal.dat"

	// El COF del personaje: la amazona parada y desarmada (AM + NU + HTH).
	cofEnMPQ = "data\\global\\CHARS\\AM\\COF\\amnuhth.cof"

	// La ventana isometrica y la celda que queda en el centro (las mismas de DC-002.37, para que los
	// numeros del nivel sean comparables).
	ladoVentana     = 512
	centroX, centroY = 28, 20

	// La pantalla logica del POC y donde se pega la ventana del nivel (centrada: (640-512)/2, (480-512)/2).
	ancho, alto = 640, 480
	mapaX       = (ancho - ladoVentana) / 2
	mapaY       = (alto - ladoVentana) / 2

	// La direccion y el frame del personaje que se componen.
	dirPersonaje   = 0
	framePersonaje = 0

	// cuadrosAMedir: cada cuantos cuadros se deja la marca de la POSICION en la consola serie (30 cuadros
	// son medio segundo a 60, o tres segundos a 10: el ritmo real lo dice el ms de la marca).
	cuadrosAMedir = 30

	// topePaso: el paso mas grande que se acepta en un cuadro (0,1 s). Sin esto, un cuadro que tarde mas
	// (una pausa del emulador, una recoleccion) mueve al personaje de un salto y el numero de la velocidad
	// sale de cualquier lado.
	topePaso = 0.1
)

// controlSordo: EL MUTANTE del criterio 3. El script lo pone en true con un sed y recompila: el control se
// sigue leyendo y se sigue informando, pero no mueve al personaje.
var controlSordo = false

// umbralGrande: a partir de que tamaño el recolector de libgodc manda la asignacion al monton de C (64 KB,
// ver DC-002.37): esas no las recupera el recolector.
const umbralGrande = 64 * 1024

// juego: lo que el motor le pide a una pantalla (Update y Draw). Es la interfaz Game, tal cual.
type juego struct {
	mapa        *ebiten.Image // la ventana del nivel (piso + paredes), ya con los colores del acto
	personaje   *ebiten.Image // el compuesto del personaje, con el indice 0 transparente
	per         *Personaje
	est         *Estado
	cuadros     int
	msAnterior  int64
	pasosCero   int
	pasosGrandes int
	botonesAntes uint32
	movido      bool
}

// Layout: el tamano de la pantalla.
func (j *juego) Layout(ancho, alto int) (int, int) { return ancho, alto }

// Update: un cuadro mas. Lee el control, avanza la posicion con las cuentas del MOTOR y deja la marca.
func (j *juego) Update() error {
	j.cuadros++

	ahora := int64(d2cMs())
	paso := float64(ahora-j.msAnterior) / 1000.0
	j.msAnterior = ahora

	if paso <= 0 {
		j.pasosCero++
	}

	if paso < 0 {
		paso = 0
	}

	// EL TOPE DEL PASO: se ACOTA, no se descarta. Medido en la consola: los cuadros que repintan el mapa
	// tardan ~0,17 s de tiempo de la consola (el resto, ~0,04 s), asi que descartando el paso de esos cuadros
	// el personaje avanzaba la mitad de lo que le tocaba. Acotando a topePaso, un cuadro lento aporta como
	// mucho topePaso de caminata: el juego se frena bajo carga, que es el comportamiento sano, en vez de
	// perder el movimiento.
	if paso > topePaso {
		j.pasosGrandes++
		paso = topePaso
	}

	// ---- el control: la cruceta y la palanca llegan como las flechas del teclado (control.go del backend
	// de la consola), asi que se pregunta por ellas y no por el bus maple de abajo
	ejeX, ejeY := 0.0, 0.0

	if ebiten.IsKeyPressed(ebiten.KeyLeft) {
		ejeX--
	}

	if ebiten.IsKeyPressed(ebiten.KeyRight) {
		ejeX++
	}

	if ebiten.IsKeyPressed(ebiten.KeyUp) {
		ejeY--
	}

	if ebiten.IsKeyPressed(ebiten.KeyDown) {
		ejeY++
	}

	// ---- el movimiento del MOTOR (d2path): la velocidad de la tabla del exe, en la direccion pedida
	j.movido = false

	if !controlSordo && (ejeX != 0 || ejeY != 0) && paso > 0 {
		dx, dy := DireccionDelPiso(ejeX, ejeY)
		if caminado := j.est.Avanzar(dx, dy, paso); caminado > 0 {
			j.movido = true
		}
	}

	// ---- la tabla del control (el instrumento de DC-002.11): se informa CUANDO CAMBIA, no cada cuadro
	b := ebiten.BotonesParaLaEvidencia()
	if b != j.botonesAntes {
		cx, cy := j.est.Celda()
		println("HITO238: CONTROL botones", int(b), "ejes", int(ejeX), int(ejeY),
			"celda", cx, cy, "cuadro", j.cuadros)
		j.botonesAntes = b
	}

	// ---- la marca de la POSICION: subtiles, celda, tile y el pixel del ancla
	if j.cuadros%cuadrosAMedir == 0 || j.cuadros == 1 {
		j.marca()
	}

	return nil
}

// marca: una linea con la posicion del motor, que es lo que el corpus compara.
func (j *juego) marca() {
	cx, cy := j.est.Celda()
	tx, ty := j.est.CeldaEntera()
	ax, ay := PixelDeLaCelda(cx, cy, origenX, origenY)

	println("HITO238: POS cuadro", j.cuadros, "ms", int64(d2cMs()),
		"botones", int(ebiten.BotonesParaLaEvidencia()),
		"subtiles", j.est.Pos.X(), j.est.Pos.Y(),
		"celda", cx, cy, "tile", tx, ty,
		"ancla", ax, ay, "pantalla", mapaX+ax, mapaY+ay,
		"caminado", j.est.Distancia, "tiempo", j.est.Tiempo, "pasos", j.est.Pasos,
		"dtCero", j.pasosCero, "dtGrande", j.pasosGrandes)
}

// Draw: el cuadro. El MAPA y el PERSONAJE ENCIMA (el orden de dibujo), por el camino del motor.
//
// El mapa se vuelve a pintar solo cuando el personaje se movio (o en el primer cuadro): la pantalla del
// backend se conserva entre cuadros, asi que volver a pintar lo mismo seria gastar el cuadro entero en una
// copia de 512x512. Cuando el personaje se mueve, el mapa tapapea el lugar donde estaba y despues se pinta
// el personaje en el lugar nuevo: el orden de dibujo es el que se ve.
func (j *juego) Draw(pantalla *ebiten.Image) {
	if j.cuadros > 1 && !j.movido {
		return
	}

	var op ebiten.DrawImageOptions
	op.GeoM.Translate(mapaX, mapaY)
	pantalla.DrawImage(j.mapa, &op)

	cx, cy := j.est.Celda()
	ax, ay := PixelDeLaCelda(cx, cy, origenX, origenY)

	var op2 ebiten.DrawImageOptions
	op2.GeoM.Translate(float64(mapaX+ax+j.per.Izq), float64(mapaY+ay+j.per.Arriba))
	pantalla.DrawImage(j.personaje, &op2)
}

// origenX/origenY: donde cayo la ventana del nivel (los imprime el compositor).
var origenX, origenY int

func main() {
	d2cSerialInit()

	println("HITO238: arranca (DC-002.38, el personaje en el mapa moviendose con el control de la Dreamcast)")
	println("HITO238: runtime", runtime.GOOS, runtime.GOARCH)

	if controlSordo {
		println("HITO238: MUTANTE controlSordo=true (el control se lee y se informa, pero no mueve)")
	}

	// ============================== 1. el nivel ==============================
	t0 := int64(d2cMs())

	mpq, err := d2mpq.FromFile(rutaMPQ)
	if err != nil {
		println("HITO238: MPQ-ABRIR FALLO:", err.Error())
		return
	}

	defer mpq.Close()

	println("HITO238: MPQ-ABRIR OK", rutaMPQ, "ms=", int64(d2cMs())-t0)

	t1 := int64(d2cMs())

	raw, err := mpq.ReadFile(nivelEnMPQ)
	if err != nil {
		println("HITO238: DS1-LEER FALLO:", err.Error())
		return
	}

	println("HITO238: DS1-LEER OK bytes=", len(raw), "crc32=", crc32.ChecksumIEEE(raw), "ms=", int64(d2cMs())-t1)

	d, err := d2ds1.Unmarshal(raw)
	if err != nil {
		println("HITO238: DS1-PARSE FALLO:", err.Error())
		return
	}

	println("HITO238: DS1 version=", d.Version(), "act=", int(d.Act), "w=", d.Width(), "h=", d.Height(),
		"objetos=", len(d.Objects), "files=", len(d.Files))

	n := NuevoNivel(d, ladoVentana, ladoVentana, centroX, centroY)
	origenX, origenY = n.OrigenX, n.OrigenY

	println("HITO238: VENTANA lado=", n.Ancho, "origen=", n.OrigenX, n.OrigenY, "centroCelda=", centroX, centroY)
	println("HITO238: PEDIDOS pisos=", n.PisosPedidos, "paredes=", n.ParedesPedidas, "total=", len(n.Pedidos))

	// los DT1, DE A UNO (el town pide 4,35 MB de DT1: tenerlos todos vivos no entra en la consola)
	for _, f := range d.Files {
		nombre := RutaDelDT1(f)
		tDT1 := int64(d2cMs())

		rawDT1, err := LeerDelMPQ(mpq, nombre)
		if err != nil {
			println("HITO238: DT1", nombre, "FALLO:", err.Error())
			continue
		}

		dt1, err := d2dt1.LoadDT1(rawDT1)
		if err != nil {
			println("HITO238: DT1", nombre, "bytes=", len(rawDT1), "PARSE-FALLO:", err.Error())
			continue
		}

		println("HITO238: DT1", nombre, "bytes=", len(rawDT1), "crc32=", crc32.ChecksumIEEE(rawDT1),
			"tiles=", len(dt1.Tiles), "ms=", int64(d2cMs())-tDT1)

		n.Alimentar(dt1.Tiles)

		dt1 = nil

		if len(rawDT1) > umbralGrande {
			d2cFreeExternal(uintptrOf(rawDT1))
		}

		rawDT1 = nil
	}

	n.Cerrar()

	println("HITO238: COMPUESTO pisos=", n.PisosCompuestos, "/", n.PisosPedidos,
		"paredes=", n.ParedesCompuestas, "/", n.ParedesPedidas,
		"sintilePiso=", n.PisosSinTile, "sintilePared=", n.ParedesSinTile, "fuera=", n.FueraDeVentana)
	println("HITO238: LIENZO pixelesNoVacios=", n.PixelesNoVacios(), "de", n.Ancho*n.Alto, "crc32=", n.CRC32())
	println("HITO238: LIENZO pixelesPiso=", n.PixelesDelPiso, "pixelesPared=", n.PixelesDePared)

	// ============================== 2. el mapa a una imagen del motor ==============================
	tPal := int64(d2cMs())

	pb, err := LeerDelMPQ(mpq, palEnMPQ)
	if err != nil || len(pb) < 768 {
		println("HITO238: PALETA-LEER FALLO:", palEnMPQ)
		return
	}

	palActo := PaletaRGB(pb)
	pvrPal := PaletaARGB1555(palActo)
	println("HITO238: PALETA acto bytes=", len(pb), "colores=256", "crc32=", CRCPaleta(pvrPal), "ms=", int64(d2cMs())-tPal)

	mapa := ebiten.NewImage(n.Ancho, n.Alto)
	rgba := make([]byte, n.Ancho*n.Alto*4)

	for i, idx := range n.Canvas {
		r, g, b := ColorDelIndice(palActo, idx)
		rgba[i*4+0], rgba[i*4+1], rgba[i*4+2], rgba[i*4+3] = r, g, b, 255
	}

	mapa.ReplacePixels(rgba)
	rgba = nil

	// el lienzo de indices ya no hace falta: se suelta para que el recolector tenga lugar para el personaje
	n.Canvas = nil
	n.Scratch = nil

	runtime.GC()

	println("HITO238: MAPA-IMAGEN bytes=", n.Ancho*n.Alto*4, "vram_libre=", int64(d2cVramLibre()))

	// ============================== 3. el personaje ==============================
	tCoF := int64(d2cMs())

	mc, err := d2mpq.FromFile(rutaMPQChars)
	if err != nil {
		println("HITO238: MPQ-CHARS-ABRIR FALLO:", err.Error())
		return
	}

	defer mc.Close()

	println("HITO238: MPQ-CHARS-ABRIR OK", rutaMPQChars, "ms=", int64(d2cMs())-tCoF)

	rawCof, err := mc.ReadFile(cofEnMPQ)
	if err != nil {
		println("HITO238: COF-LEER FALLO:", err.Error())
		return
	}

	cof, err := d2cof.Unmarshal(rawCof)
	if err != nil {
		println("HITO238: COF-PARSE FALLO:", err.Error())
		return
	}

	println("HITO238: COF", cofEnMPQ, "bytes=", len(rawCof), "crc32=", crc32.ChecksumIEEE(rawCof),
		"capas=", cof.NumberOfLayers, "framesPorDir=", cof.FramesPerDirection,
		"dirs=", cof.NumberOfDirections, "speed=", cof.Speed)

	per := NuevoPersonaje(cof, dirPersonaje, framePersonaje)
	println("HITO238: ORDEN capas=", len(per.Claves), "orden=", len(per.Orden))

	for i := range cof.CofLayers {
		tCapa := int64(d2cMs())
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
					println("HITO238: CAPA", per.Claves[i], "PARSE-FALLO:", err.Error())
					break
				}

				caja := dd.Directions[dirPersonaje].Box
				pix := dd.Directions[dirPersonaje].Frames[framePersonaje].PixelData

				println("HITO238: CAPA", per.Claves[i], "bytes=", len(crudo), "crc32=", crc32.ChecksumIEEE(crudo),
					"caja=", caja.Left, caja.Top, caja.Width, caja.Height, "pixeles=", noCero(pix),
					"ms=", int64(d2cMs())-tCapa)

				per.Agregar(per.Tipos[i], per.Claves[i], caja.Left, caja.Top, caja.Width, caja.Height, pix)
				encontrada = true

				// el DCC entero (16 direcciones x 8 frames) se suelta: se queda SOLO el frame que se dibuja
				dd = nil
				crudo = nil

				break
			}

			dc, err := d2dc6.Load(crudo)
			if err != nil {
				println("HITO238: CAPA", per.Claves[i], "PARSE-FALLO:", err.Error())
				break
			}

			fr := dc.Frames[framePersonaje]
			pix := dc.DecodeFrame(framePersonaje)

			println("HITO238: CAPA", per.Claves[i], "bytes=", len(crudo), "crc32=", crc32.ChecksumIEEE(crudo),
				"caja=", fr.OffsetX, fr.OffsetY, fr.Width, fr.Height, "pixeles=", noCero(pix),
				"ms=", int64(d2cMs())-tCapa)

			per.Agregar(per.Tipos[i], per.Claves[i], int(fr.OffsetX), int(fr.OffsetY),
				int(fr.Width), int(fr.Height), pix)
			encontrada = true

			dc = nil
			crudo = nil

			break
		}

		if !encontrada {
			println("HITO238: CAPA", per.Claves[i], "SIN-GRAFICO ruta=", ultimaRuta, "err=", ultimoErr)
		}

		runtime.GC()
	}

	per.Componer()

	println("HITO238: PERSONAJE compuesto=", per.Ancho, "x", per.Alto, "esquina=", per.Izq, per.Arriba,
		"capasDibujadas=", per.CapasDibujadas, "sinGrafico=", per.CapasSinGrafico,
		"pixelesNoVacios=", per.PixelesNoVacios, "crc32=", per.CRC32())

	// la paleta de las UNIDADES (la de los personajes) y el compuesto a una imagen con alfa
	pbu, err := LeerDelMPQ(mpq, palUnidadesMPQ)
	if err != nil || len(pbu) < 768 {
		println("HITO238: PALETA-UNIDADES FALLO:", palUnidadesMPQ)
		return
	}

	palUni := PaletaRGB(pbu)

	var personaje *ebiten.Image

	if per.Ancho > 0 && per.Alto > 0 {
		personaje = ebiten.NewImage(per.Ancho, per.Alto)

		crgba := make([]byte, per.Ancho*per.Alto*4)

		for i, idx := range per.Lienzo {
			if idx == 0 {
				continue // el indice 0 es TRANSPARENTE: el mapa se ve por ahi
			}

			crgba[i*4+0], crgba[i*4+1], crgba[i*4+2], crgba[i*4+3] =
				palUni[idx][0], palUni[idx][1], palUni[idx][2], 255
		}

		personaje.ReplacePixels(crgba)
	}

	println("HITO238: PERSONAJE-IMAGEN bytes=", per.Ancho*per.Alto*4, "vram_libre=", int64(d2cVramLibre()))

	// ============================== 4. el personaje y el control ==============================
	est := NuevoEstado(float64(centroX), float64(centroY), false)

	println("HITO238: MOTOR velocidad=subtilesPorSegundo=", est.Velocidad,
		"celdasPorSegundo=", est.Velocidad/subtilesPorCelda)

	cx, cy := est.Celda()
	ax, ay := PixelDeLaCelda(cx, cy, n.OrigenX, n.OrigenY)

	println("HITO238: POS-INICIAL subtiles=", est.Pos.X(), est.Pos.Y(), "celda=", cx, cy,
		"ancla=", ax, ay, "pantalla=", mapaX+ax, mapaY+ay)

	if ebiten.HayControl() {
		println("HITO238: hay control en el puerto 0")
	} else {
		println("HITO238: NO hay control en el puerto 0 (el loop corre igual)")
	}

	j := &juego{mapa: mapa, personaje: personaje, per: per, est: est, msAnterior: int64(d2cMs())}

	if err := ebiten.IniciarPantalla(ancho, alto); err != nil {
		println("HITO238: no pude arrancar el video:", err.Error())
	}

	println("HITO238: dibujando (mapa", n.Ancho, "x", n.Alto, "en", mapaX, mapaY, "; personaje", per.Ancho, "x", per.Alto, ")")

	if err := ebiten.RunGame(j); err != nil {
		println("HITO238: el loop termino con error:", err.Error())
	}
}

// noCero: cuantos pixeles del frame NO son el indice 0 (el instrumento del personaje).
func noCero(pix []byte) int {
	c := 0
	for _, v := range pix {
		if v != 0 {
			c++
		}
	}

	return c
}

// uintptrOf: la direccion de los bytes de un slice, para devolverlos al monton de C (ver DC-002.37).
func uintptrOf(b []byte) uintptr {
	if len(b) == 0 {
		return 0
	}

	return uintptr(unsafe.Pointer(&b[0]))
}
