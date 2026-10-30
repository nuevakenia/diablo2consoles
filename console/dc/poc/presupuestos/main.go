//go:build dreamcast

// DC-002.75 -- LOS PRESUPUESTOS DE CACHE DE LA CONSOLA, MEDIDOS EN FLYCAST (Fase B.1c).
//
// QUE PRUEBA. DC-002.74 midio que el techo del cooperativo no es la imagen del binario sino lo que el
// motor se guarda: los ocho caches del AssetManager, configurados con numeros de PC (128 MB de
// animaciones, 1 GB por cache de datos) que en una consola de 16 MB son "sin limite". Esta tarjeta
// acota esos presupuestos SOLO en la consola y esta corrida mide, con numeros:
//
//  1. el monton libre antes y despues de acotar (d2c_heap_libre, el mismo instrumento de DC-002.51),
//  2. cuantas veces desaloja cada cache (Cache.Evictions, el numero que el modo verbose escribe),
//  3. que el cuadro NO cambia: el mapa y el personaje se dibujan IGUAL que en DC-002.70, con el mismo
//     nivel.go y el mismo personaje.go (el build los copia y compara su sha256), por el mismo
//     Renderer PVR, y la captura se compara pixel a pixel contra la de DC-002.70.
//
// LA UNICA DIFERENCIA CON EL POC DE DC-002.70 (console/dc/poc/estado-b) ES DE DONDE SALEN LOS DATOS:
// el estado B lee el .ds1, los .dt1, el COF y las capas .dcc del MPQ y los tira en cuanto los usa; aca
// pasan por los CACHES DEL MOTOR con los presupuestos de consola (Retrieve -> si falta, leer, parsear e
// Insert con el peso en bytes), que es lo que va a hacer el juego de verdad. El dibujado es el MISMO
// codigo: si el cuadro cambia, es el cache.
//
// LO QUE NO ENTRA: el cooperativo de 4 jugadores (DC-002.76), items, menu ni animacion: es el primer
// cuadro, el mismo de DC-002.70, que es contra el que se compara.
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
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_heap_libre
func d2cHeapLibre() uint

// init: la consola serie es la UNICA salida de texto en la Dreamcast.
func init() { d2cSerialInit() }

const (
	rutaMPQ      = "/cd/JUEGO/D2DATA.MPQ"
	rutaMPQChars = "/cd/JUEGO/D2CHAR.MPQ"

	nivelEnMPQ     = "data\\global\\tiles\\ACT1\\TOWN\\townN1.ds1"
	palEnMPQ       = "data/global/palette/ACT1/pal.dat"
	palUnidadesMPQ = "data/global/palette/Units/pal.dat"
	cofEnMPQ       = "data\\global\\CHARS\\AM\\COF\\amnuhth.cof"

	ladoVentana      = 512
	centroX, centroY = 28, 20
	dirPersonaje     = 0
	framePersonaje   = 0

	ancho, alto = 640, 480
	mapaX       = (ancho - ladoVentana) / 2
	mapaY       = (alto - ladoVentana) / 2

	// cuadrosCuadro: a los cuantos cuadros se informa el estado de los caches "durante N cuadros"
	cuadrosCuadro = 60
)

var (
	mapaIndices     []byte
	paletaDelMapa   [][3]uint8
	personajeSprite *pvr.Sprite
	personajeX      int
	personajeY      int

	mapaPintado bool
	cuadros     int
)

// contadorDeCache: aciertos y faltas de UN cache, contados por el POC (el Cache no los lleva porque el
// motor no los necesita; lo que si lleva es el desalojo, que es el numero de esta tarjeta).
type contadorDeCache struct {
	aciertos int
	faltas   int
}

var (
	caches *d2asset.CachesDeAssets

	cuentaDT1s        contadorDeCache
	cuentaDS1s        contadorDeCache
	cuentaCoFs        contadorDeCache
	cuentaDCCs        contadorDeCache
	cuentaAnimaciones contadorDeCache
)

func main() {
	println("PRESUPUESTOS: arranca DC-002.75 -- los caches del motor con presupuestos de consola")
	println("PRESUPUESTOS: monton libre al arrancar (bytes):", int(d2cHeapLibre()))

	// ============================== 0. LOS PRESUPUESTOS, ANTES Y DESPUES ==============================
	antes := d2asset.PresupuestosVigentes()
	informeDePresupuestos("DE PC (los de siempre)", antes)

	// lo que hace el arranque de la consola: acota los caches y los pone a pesar en BYTES
	d2asset.AjustarPresupuestosDeConsola()

	despues := d2asset.PresupuestosVigentes()
	informeDePresupuestos("DE CONSOLA (acotados)", despues)

	println("PRESUPUESTOS: la suma de los caches de DATOS paso de", antes.SumaDeLosCachesDeDatos(),
		"a", despues.SumaDeLosCachesDeDatos(), "bytes")

	caches = d2asset.NuevosCachesDeAssets()

	// ============================== 1. EL VIDEO PRIMERO ==============================
	// El arranque del PVR necesita el monton vacio (medido en DC-002.50): por eso va antes de cargar nada.
	renderer := pvr.NuevoRenderer()

	if err := pvr.PantallaIniciar(ancho, alto); err != nil {
		println("PRESUPUESTOS: el video no arranco:", err.Error())

		return
	}

	println("PRESUPUESTOS: video andando, monton libre (bytes):", int(d2cHeapLibre()),
		"vram de la textura del cuadro:", pvr.VramDeLaPantalla())

	// ============================== 2. LA ZONA, POR LOS CACHES ==============================
	mpq, err := d2mpq.FromFile(rutaMPQ)
	if err != nil {
		println("PRESUPUESTOS: MPQ-ABRIR FALLO:", err.Error())

		return
	}

	d := cargarDS1Cacheado(mpq, nivelEnMPQ)

	if d == nil {
		return
	}

	println("PRESUPUESTOS: DS1 version=", d.Version(), "act=", int(d.Act), "w=", d.Width(), "h=", d.Height(),
		"files=", len(d.Files), "ficultades=", cuentaDS1sString(),
		"evicto=", caches.Ds1s.Evictions(), "pesoEnCache=", caches.Ds1s.GetWeight(),
		"presupuesto=", caches.Ds1s.GetBudget(), "montonLibre=", int(d2cHeapLibre()))

	n := NuevoNivel(d, ladoVentana, ladoVentana, centroX, centroY)
	println("PRESUPUESTOS: VENTANA lado=", n.Ancho, "origen=", n.OrigenX, n.OrigenY,
		"pisos=", n.PisosPedidos, "paredes=", n.ParedesPedidas)

	for _, f := range d.Files {
		nombre := RutaDelDT1(f)

		dt1 := cargarDT1Cacheado(mpq, nombre)
		if dt1 == nil {
			continue
		}

		println("PRESUPUESTOS: DT1", nombre, "tiles=", len(dt1.Tiles),
			"evicto=", caches.Dt1s.Evictions(), "pesoEnCache=", caches.Dt1s.GetWeight(),
			"presupuesto=", caches.Dt1s.GetBudget(), "montonLibre=", int(d2cHeapLibre()))

		n.Alimentar(dt1.Tiles)

		dt1 = nil
	}

	n.Cerrar()

	println("PRESUPUESTOS: COMPUESTO pisos=", n.PisosCompuestos, "/", n.PisosPedidos,
		"paredes=", n.ParedesCompuestas, "/", n.ParedesPedidas,
		"sintilePiso=", n.PisosSinTile, "sintilePared=", n.ParedesSinTile, "fuera=", n.FueraDeVentana)
	println("PRESUPUESTOS: LIENZO pixelesNoVacios=", n.PixelesNoVacios(), "de", n.Ancho*n.Alto,
		"crc32=", n.CRC32())
	println("PRESUPUESTOS: DT1S al terminar la zona: aciertos=", cuentaDT1s.aciertos, "faltas=", cuentaDT1s.faltas,
		"evicto=", caches.Dt1s.Evictions(), "pesoEnCache=", caches.Dt1s.GetWeight())

	pb, err := LeerDelMPQ(mpq, palEnMPQ)
	if err != nil || len(pb) < 768 {
		println("PRESUPUESTOS: PALETA-LEER FALLO:", palEnMPQ)

		return
	}

	palActo := PaletaRGB(pb)
	mapaIndices = n.Canvas
	paletaDelMapa = palActo[:]

	println("PRESUPUESTOS: PALETA acto colores=256 crc32=", CRCPaleta(PaletaARGB1555(palActo)),
		"monton libre (bytes):", int(d2cHeapLibre()))

	// ============================== 3. EL PERSONAJE, POR LOS CACHES ==============================
	mc, err := d2mpq.FromFile(rutaMPQChars)
	if err != nil {
		println("PRESUPUESTOS: MPQ-CHARS-ABRIR FALLO:", err.Error())

		return
	}

	cof := cargarCOFCacheado(mc, cofEnMPQ)
	if cof == nil {
		return
	}

	println("PRESUPUESTOS: COF capas=", cof.NumberOfLayers, "framesPorDir=", cof.FramesPerDirection,
		"dirs=", cof.NumberOfDirections, "evicto=", caches.Cofs.Evictions(),
		"pesoEnCache=", caches.Cofs.GetWeight(), "presupuesto=", caches.Cofs.GetBudget())

	per := NuevoPersonaje(cof, dirPersonaje, framePersonaje)

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
					println("PRESUPUESTOS: CAPA", per.Claves[i], "PARSE-FALLO:", err.Error())

					break
				}

				tamano := d2asset.TamanoDCC(dd)
				insertarOReusarDCC(nombre, dd, tamano, caches)

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
				println("PRESUPUESTOS: CAPA", per.Claves[i], "PARSE-FALLO:", err.Error())

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
			println("PRESUPUESTOS: CAPA", per.Claves[i], "SIN-GRAFICO ruta=", ultimaRuta, "err=", ultimoErr)
		}

		runtime.GC()
	}

	per.Componer()

	println("PRESUPUESTOS: PERSONAJE compuesto=", per.Ancho, "x", per.Alto, "esquina=", per.Izq, per.Arriba,
		"capasDibujadas=", per.CapasDibujadas, "sinGrafico=", per.CapasSinGrafico,
		"pixelesNoVacios=", per.PixelesNoVacios, "crc32=", per.CRC32())
	println("PRESUPUESTOS: DCCS aciertos=", cuentaDCCs.aciertos, "faltas=", cuentaDCCs.faltas,
		"evicto=", caches.Dccs.Evictions(), "pesoEnCache=", caches.Dccs.GetWeight(),
		"presupuesto=", caches.Dccs.GetBudget())
	println("PRESUPUESTOS: ANIMACIONES evicto=", caches.Animations.Evictions(),
		"pesoEnCache=", caches.Animations.GetWeight(), "presupuesto=", caches.Animations.GetBudget())

	pbu, err := LeerDelMPQ(mpq, palUnidadesMPQ)
	if err != nil || len(pbu) < 768 {
		println("PRESUPUESTOS: PALETA-UNIDADES FALLO:", palUnidadesMPQ)

		return
	}

	if per.Ancho > 0 && per.Alto > 0 {
		personajeSprite = pvr.NuevoSprite(per.Ancho, per.Alto, per.Lienzo, PaletaARGB1555(PaletaRGB(pbu)))
	}

	est := NuevoEstado(float64(centroX), float64(centroY), false)
	cx, cy := est.Celda()
	ax, ay := PixelDeLaCelda(cx, cy, n.OrigenX, n.OrigenY)

	personajeX, personajeY = mapaX+ax+per.Izq, mapaY+ay+per.Arriba

	println("PRESUPUESTOS: POS-INICIAL celda=", cx, cy, "ancla=", ax, ay,
		"esquinaDelCompuesto=", per.Izq, per.Arriba, "pantalla=", personajeX, personajeY)
	println("PRESUPUESTOS: monton libre con la zona y el personaje cargados (bytes):", int(d2cHeapLibre()))

	// los caches ya tienen lo del cuadro: los desalojos de "durante N cuadros" se cuentan desde aca
	caches.Dt1s.ResetEvictions()
	caches.Ds1s.ResetEvictions()
	caches.Cofs.ResetEvictions()
	caches.Dccs.ResetEvictions()
	caches.Animations.ResetEvictions()

	// ============================== 4. EL CICLO DE CUADROS DE LA INTERFAZ ==============================
	if err := renderer.Run(dibujar, actualizar, ancho, alto, "Diablo II (Dreamcast)"); err != nil {
		println("PRESUPUESTOS: el ciclo de cuadros salio con error:", err.Error())
	}
}

// informeDePresupuestos imprime los ocho numeros, nombrados como en el codigo del motor.
func informeDePresupuestos(rotulo string, p d2asset.Presupuestos) {
	println("PRESUPUESTOS: budgets " + rotulo + ":")
	println("PRESUPUESTOS:   animaciones=", p.Animaciones, "cofs=", p.CoFs, "dccs=", p.DCCs,
		"dt1s=", p.DT1s, "ds1s=", p.DS1s, "fuentes=", p.Fuentes, "paletas=", p.Paletas,
		"transformaciones=", p.Transformaciones)
	println("PRESUPUESTOS:   suma de los caches de datos=", p.SumaDeLosCachesDeDatos())
}

func cuentaDS1sString() string {
	if cuentaDS1s.faltas == 0 {
		return "acierto"
	}

	return "falta"
}

// cargarDS1Cacheado: el camino del motor (Retrieve -> falta -> leer -> parsear -> Insert con el peso en
// bytes). Devuelve nil si no se pudo.
func cargarDS1Cacheado(mpq *d2mpq.MPQ, clave string) *d2ds1.DS1 {
	if valor, encontrado := caches.Ds1s.Retrieve(clave); encontrado {
		cuentaDS1s.aciertos++

		return valor.(*d2ds1.DS1)
	}

	cuentaDS1s.faltas++

	raw, err := LeerDelMPQ(mpq, clave)
	if err != nil {
		println("PRESUPUESTOS: DS1-LEER FALLO:", err.Error())

		return nil
	}

	d, err := d2ds1.Unmarshal(raw)
	if err != nil {
		println("PRESUPUESTOS: DS1-PARSE FALLO:", err.Error())

		return nil
	}

	peso := d2asset.TamanoDS1(d)

	if err := caches.Ds1s.Insert(clave, d, d2asset.Pesar(peso)); err != nil {
		println("PRESUPUESTOS: DS1-INSERT FALLO:", err.Error())

		return nil
	}

	println("PRESUPUESTOS: DS1-INSERT peso=", peso, "bytes (crudo", len(raw), ")")

	return d
}

// cargarDT1Cacheado: lo mismo para un .dt1.
func cargarDT1Cacheado(mpq *d2mpq.MPQ, clave string) *d2dt1.DT1 {
	if valor, encontrado := caches.Dt1s.Retrieve(clave); encontrado {
		cuentaDT1s.aciertos++

		return valor.(*d2dt1.DT1)
	}

	cuentaDT1s.faltas++

	raw, err := LeerDelMPQ(mpq, clave)
	if err != nil {
		println("PRESUPUESTOS: DT1", clave, "FALLO:", err.Error())

		return nil
	}

	dt1, err := d2dt1.LoadDT1(raw)
	if err != nil {
		println("PRESUPUESTOS: DT1", clave, "PARSE-FALLO:", err.Error())

		return nil
	}

	if err := caches.Dt1s.Insert(clave, dt1, d2asset.Pesar(d2asset.TamanoDT1(dt1))); err != nil {
		println("PRESUPUESTOS: DT1", clave, "INSERT-FALLO:", err.Error())

		return nil
	}

	return dt1
}

// cargarCOFCacheado: el COF por el cache de COFs.
func cargarCOFCacheado(mpq *d2mpq.MPQ, clave string) *d2cof.COF {
	if valor, encontrado := caches.Cofs.Retrieve(clave); encontrado {
		cuentaCoFs.aciertos++

		return valor.(*d2cof.COF)
	}

	cuentaCoFs.faltas++

	raw, err := LeerDelMPQ(mpq, clave)
	if err != nil {
		println("PRESUPUESTOS: COF-LEER FALLO:", err.Error())

		return nil
	}

	cof, err := d2cof.Unmarshal(raw)
	if err != nil {
		println("PRESUPUESTOS: COF-PARSE FALLO:", err.Error())

		return nil
	}

	if err := caches.Cofs.Insert(clave, cof, d2asset.Pesar(len(raw))); err != nil {
		println("PRESUPUESTOS: COF-INSERT FALLO:", err.Error())

		return nil
	}

	return cof
}

// insertarOReusarDCC: un DCC entra en DOS caches, igual que en el motor: el de DCCs y el de animaciones
// (una animacion de DCC se apoya en el DCC, no lo copia: el Animation y el DCC son la misma memoria).
func insertarOReusarDCC(clave string, dd *d2dcc.DCC, tamano int, c *d2asset.CachesDeAssets) {
	if _, encontrado := c.Dccs.Retrieve(clave); encontrado {
		cuentaDCCs.aciertos++

		return
	}

	cuentaDCCs.faltas++

	if err := c.Dccs.Insert(clave, dd, d2asset.Pesar(tamano)); err != nil {
		println("PRESUPUESTOS: DCC", clave, "INSERT-FALLO:", err.Error())
	}

	if _, encontrado := c.Animations.Retrieve(clave); encontrado {
		cuentaAnimaciones.aciertos++

		return
	}

	cuentaAnimaciones.faltas++

	if err := c.Animations.Insert(clave, dd, d2asset.Pesar(tamano)); err != nil {
		println("PRESUPUESTOS: ANIM", clave, "INSERT-FALLO:", err.Error())
	}
}

// dibujar es EL MISMO CALLBACK DEL MOTOR QUE EN DC-002.70: compone el cuadro en la Surface del backend.
func dibujar(superficie d2interface.Surface) error {
	if mapaPintado {
		return nil
	}

	malla, ok := superficie.(*pvr.Surface)
	if !ok {
		return errorDelPoc("el Renderer no entrego una Surface del backend PVR")
	}

	malla.Clear(color.RGBA{R: 0, G: 0, B: 0, A: 255})

	malla.PushTranslation(mapaX, mapaY)
	malla.RenderIndices(mapaIndices, ladoVentana, ladoVentana, paletaDelMapa)
	malla.Pop()

	if personajeSprite != nil {
		malla.PushTranslation(personajeX, personajeY)
		malla.RenderSprite(personajeSprite)
		malla.Pop()
	}

	mapaPintado = true

	pixeles := malla.Imagen().Pixeles()
	llenos := 0

	for j := 0; j+3 < len(pixeles); j += 4 {
		if pixeles[j] != 0 || pixeles[j+1] != 0 || pixeles[j+2] != 0 {
			llenos++
		}
	}

	anchoReal, altoReal := superficie.GetSize()

	println("PRESUPUESTOS: CUADRO pixelesNoVacios=", llenos, "de", anchoReal*altoReal,
		"crc32=", int(crc32.ChecksumIEEE(pixeles)),
		"tamano=", anchoReal, altoReal,
		"lienzoEnVram=", pvr.VramDelLienzo(),
		"montonLibre=", int(d2cHeapLibre()))

	return nil
}

// actualizar: a los cuadrosCuadro cuadros informa como quedaron los caches "durante N cuadros". El
// primer cuadro no repinta (el mapa se conserva), asi que lo unico que cambia entre cuadros es el
// contador de desalojos y el monton: eso es lo que se mide.
func actualizar() error {
	cuadros++

	if cuadros == cuadrosCuadro {
		println("PRESUPUESTOS: DURANTE", cuadros, "CUADROS:",
			"dt1s evicto=", caches.Dt1s.Evictions(), "peso=", caches.Dt1s.GetWeight(),
			"/ ds1s evicto=", caches.Ds1s.Evictions(), "peso=", caches.Ds1s.GetWeight(),
			"/ cofs evicto=", caches.Cofs.Evictions(), "peso=", caches.Cofs.GetWeight(),
			"/ dccs evicto=", caches.Dccs.Evictions(), "peso=", caches.Dccs.GetWeight(),
			"/ animaciones evicto=", caches.Animations.Evictions(), "peso=", caches.Animations.GetWeight())
		println("PRESUPUESTOS: DURANTE", cuadros, "CUADROS: montonLibre=", int(d2cHeapLibre()))
	}

	return nil
}

// errorDelPoc: un error con nombre, para que el que llama pueda distinguirlo de uno del motor.
type errorDelPoc string

func (e errorDelPoc) Error() string { return string(e) }
