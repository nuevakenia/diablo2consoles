//go:build dreamcast

// DC-002.155 -- GATE 0 CON EL MOTOR DE VERDAD: EL PUEBLO PINTADO POR `d2app` EN LA DREAMCAST.
//
// QUE ES. El Gate 0 del plan dice: "el motor pinta un cuadro del pueblo en la consola". Hasta hoy el pueblo se
// habia compuesto y dibujado en POCs (`mapa-101`, `juego-131`...) que arman su propio camino. Esto es OTRA
// COSA: arranca `d2app` -- el entry point del juego, el mismo `Create().Run()` que usa la PC -- con las
// palancas de la consola puestas, y en vez del menu carga la pantalla del pueblo (`d2gamescreen`), que compone
// el mapa con `d2mapengine` y lo dibuja con `d2maprenderer`, los paquetes del motor. Si esto pinta, el Gate 0
// esta cerrado con el camino del motor y no con un POC.
//
// LO QUE HACIA FALTA PARA QUE ENTRARA (y es el motivo de que el POC tenga este archivo):
//
//  1. LAS PALANCAS DEL ARRANQUE (`d2app.ArrancarEnGate0`, DC-002.155): el arranque de la PC carga las 85 tablas
//     (15-16,7 MB), el `animdata.d2` (2,6 MB), las tres tablas de strings (2,67 MB), la pantalla de carga
//     (3,11 MB de RGBA) y el sonido de la UI. En el monton de C de la consola (2.450.496 B) eso no entra. El
//     pueblo solo necesita `LvlTypes` y `LvlPrest` (DC-002.86) y sin entidades no hay animacion que consultar.
//  2. LA CACHE DEL MAPA EN INDICES Y EN VRAM (`d2maprenderer`): 29,36 MB de RGBA no existen; en indices son
//     7,70 MB y los bloques salen de la VRAM (DC-002.101/.104).
//  3. LA SURFACE DEL CUADRO TIENE QUE SABER PINTAR INDICES (`d2core/d2render/ebiten/indices_consola.go`): lo
//     tenia el backend PVR del POC y NO el shim de ebiten, que es por donde dibuja el motor.
//  4. EL BUFFER DE LECTURA DE LOS `.dt1` EN VRAM (DC-002.154): 1.051.532 B que si no son del monton de C.
//
// LO QUE SE MIDE, y en este orden: el monton de C y la VRAM en cada etapa del arranque (config, cada tabla,
// el mapa paso por paso, cada cuadro informado), el censo de casillas del pueblo y los ms del primer cuadro.
// El numero con el que se compara es el presupuesto de DC-002.93: 2.450.496 B de monton de C.
//
// LAS PALANCAS DE ESTE MAIN (arriba, en constantes: en la consola `os.Getenv` es un stub, asi que un knob por
// variable de entorno seria un knob que no se puede mover):
//
//	bloquesDeLaCache   cuantos tramos de 348.160 B se le piden a la VRAM para la cache de tiles
//	bytesDelScratch    el buffer de lectura de los `.dt1` (el `.dt1` mas grande del pueblo, DC-002.154)
//	cuadroAncho/Alto   640x480: el cuadro que compone el renderer (d2display no baja de 800x600)
package main

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2app"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2maprenderer"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// --- la consola serie y el monton de C (los mismos puentes que el resto de los POC) ---

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_monton_libre
func d2cMontonLibre() int

//extern d2c_monton_usado
func d2cMontonUsado() int

//extern d2c_monton_techo
func d2cMontonTecho() uint

//extern d2c_servible
func d2cServible(tope uint) int

//extern d2c_break_libre
func d2cBreakLibre() int

//extern d2c_gc_percent
func d2cGcPercent(nuevo int) int

//extern d2c_gc_usado
func d2cGcUsado() int

//extern d2c_gc_total
func d2cGcTotal() int

//extern d2c_libre
func d2cLibre(p uintptr)

func init() { d2cSerialInit() }

const (
	// el cuadro que COMPONE el renderer: 640x480 (el de la consola; d2display no baja de 800x600 y ese es el
	// tamano de la superficie del motor, pero la cache y la camara se calculan sobre este)
	cuadroAncho = 640
	cuadroAlto  = 480

	// la cache de tiles: tramos de 348.160 B (8 celdas de 64x64 en indices, DC-002.104) pedidos a la VRAM.
	// 4 tramos = 1.392.640 B: despues del cuadro (2.097.152) y del framebuffer del motor (1.920.000) a la
	// VRAM del PVR le quedan 1.772.200 B medidos, asi que 6 tramos (2.088.960) NO entran y 4 si. Con 4 la
	// cache no guarda el cuadro entero (86 imagenes del POC son 2,72 MB) y el desalojo recompone lo que falta:
	// cuesta ms por cuadro, no el cuadro.
	bloquesDeLaCache = 4
	bytesDelBloque   = 348160

	// el buffer prestado de la lectura de los `.dt1` (DC-002.154): el `.dt1` mas grande del pueblo. Si la VRAM
	// no lo da, el camino se degrada solo al `make` del monton de C y el arranque muere donde muere.
	bytesDelScratch = 1051532

	// el umbral del recolector: 95 % (el mismo del POC del juego, medido en DC-002.109: mas bajo, el
	// recolector se lleva el pueblo a pedazos; mas alto, el semiespacio se llena de basura)
	umbralDelRecolector = 95
)

// frase: lo que se imprime antes de cada etapa. Todo el log de esta corrida empieza con GATE0 para poder
// separarlo de lo que imprimen el motor y el shim.
const frase = "GATE0:"

// sondaDelMonton: cuantos bytes entran de verdad debajo de `tope` (el puente pide uint, el motor int).
func sondaDelMonton(tope int) int { return d2cServible(uint(tope)) }

// estado: el monton de C y la VRAM, en una linea, para la etapa que se pida. OJO con los dos numeros del
// monton: `montonLibre` es la lista de libres (al arrancar son 1,6 KB y NO significa que no haya memoria) y
// `breakLibre` es lo que queda entre el break y el techo, que es de donde salen los pedidos grandes. Lo que
// decide si una estructura entra es la SUMA de los dos.
func estado(etapa string) {
	censo := pvr.FotoDeLaVram()

	println(frase, "ESTADO", etapa,
		"montonLibre=", d2cMontonLibre(),
		"breakLibre=", d2cBreakLibre(),
		"montonUsado=", d2cMontonUsado(),
		"gcVivo=", d2cGcUsado(),
		"gcTotal=", d2cGcTotal(),
		"vramLibre=", censo.Disponible,
		"vramPedida=", censo.Pedida)
}

// instrumento: lo que `d2app` y la pantalla llaman en cada etapa del arranque.
func instrumento(etapa string) { estado(etapa) }

func main() {
	println(frase, "arranca DC-002.155 -- el pueblo del Gate 0 por el arranque del MOTOR (d2app)")
	println(frase, "palancas: cuadro=", cuadroAncho, "x", cuadroAlto, "bloquesDeLaCache=", bloquesDeLaCache,
		"bytesDelBloque=", bytesDelBloque, "bytesDelScratch=", bytesDelScratch,
		"gcPercent=", umbralDelRecolector)

	estado("al-arrancar")

	// ============================== 0. EL VIDEO PRIMERO ==============================
	// El PVR necesita el monton de C casi vacio para arrancar (DC-002.50): la textura de la pantalla y sus
	// buffers se piden con el monton limpio. El tamano es el que va a pedir el motor en `Layout`
	// (d2display.Clamp no baja de 800x600), asi que `RunGame` despues lo encuentra ya hecho.
	if err := ebiten.IniciarPantalla(800, 600); err != nil {
		println(frase, "el video NO arranco:", err.Error())

		return
	}

	println(frase, "PVR listo, VRAM de la textura del cuadro=", ebiten.VramDeLaPantalla())
	estado("despues-del-video")

	// ============================== 1. LAS PALANCAS DE LA CONSOLA ==============================
	// El AssetManager del motor se arma DENTRO de `d2app.Create`, y los presupuestos de sus caches se congelan
	// al armarlos: por eso esto va antes de Create y no despues.
	d2asset.ArrancarComoConsola()

	// el `free` del monton de C para los tramos que la consola pide con malloc: sin esto, soltar un tramo lo
	// devuelve al recolector de Go y el monton de C no lo recupera
	d2asset.UsarLiberadorDeExternos(func(p uintptr) { d2cLibre(p) })

	// la sonda y el medidor del monton de C: el arena de bloques de los `.dt1` se pide por tramos del tamano
	// que ENTRA, no de un tamano fijo que puede no entrar
	d2asset.UsarSondaDelMontonDeC(sondaDelMonton)
	d2asset.UsarMedidorDelMontonDeC(d2cMontonLibre)

	// DC-002.154: el buffer de lectura de los `.dt1` sale de la VRAM (1.051.532 B que si no son del monton)
	d2asset.UsarScratchDeFuera(
		func(bytes int) []byte { return pvr.VramReservar(bytes) },
		func(b []byte) int { return pvr.VramSoltar(b) },
	)

	previo := d2cGcPercent(umbralDelRecolector)
	println(frase, "gcPercent: antes=", previo, "ahora=", umbralDelRecolector,
		"scratchDeFuera=", d2asset.HayScratchDeFuera(),
		"techoDelMonton=", int(d2cMontonTecho()))

	// ============================== 2. EL MOTOR ==============================
	app := d2app.Create("DC-002.155", "gate0-pueblo")

	estado("despues-de-Create")

	// las palancas del arranque y la pantalla del pueblo
	app.ArrancarEnGate0(d2app.Gate0{
		// las dos tablas del nivel (DC-002.86: LvlTypes trae los tipos de nivel y LvlPrest los presets)
		Tablas: []string{d2resource.LevelType, d2resource.LevelPreset},

		SinAnimData: true, // sin entidades no se consulta ni una animacion
		SinStrings:  true, // el pueblo no dibuja texto
		SinGui:      true, // sin cursor ni pantalla de carga (3,15 MB de RGBA)
		SinUi:       true, // sin el sonido del boton (esta en d2sfx.mpq, que no va al disco)

		Region:       d2enum.RegionAct1Town,
		Preset:       1,
		ConEntidades: false, // las casillas solas: el pueblo, no sus habitantes

		OpcionesDelMapa: []d2maprenderer.Opcion{
			d2maprenderer.ComoConsola(cuadroAncho, cuadroAlto),
			d2maprenderer.CacheDeTilesPerezosa(),
			d2maprenderer.CacheDeTilesEnIndices(),
			d2maprenderer.CacheDeTilesEnBloquesDe(bloquesDeLaCache*bytesDelBloque, bytesDelBloque,
				pvr.VramReservar, "vram", sondaDelMonton),
		},

		Instrumento: instrumento,
	})

	println(frase, "llamando a Run(): initialize + el bucle del PVR")

	if err := app.Run(); err != nil {
		println(frase, "Run() termino con error:", err.Error())
		estado("al-morir")

		return
	}

	println(frase, "Run() volvio sin error")
	estado("al-final")
}
