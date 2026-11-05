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
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2input/maple"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2maprenderer"
	renderEbiten "github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/ebiten"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
	"github.com/OpenDiablo2/OpenDiablo2/d2game/d2gamescreen"
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

//extern d2c_gc_colecciones
func d2cGcColecciones() int

// DC-002.157: el reloj de microsegundos del backend del PVR, para el cronometro por tile del renderer
// (`d2maprenderer.FijarRelojDeLosTiles`). Es el mismo reloj con el que el shim mide las fases del cuadro.
//
//extern d2c_pvr_pantalla_us
func d2cRelojUs() uint

func init() { d2cSerialInit() }

const (
	// heroeEnElPueblo (DC-002.159): la palanca del heroe en este POC. DC-002.159b la deja PRENDIDA: el heroe
	// entra porque el mapa le devuelve la memoria (las 2.312 casillas del motor y el arena de bloques de los
	// `.dt1` se sueltan al cerrar el cuadro 1, cuando el mapa ya esta pintado y no se repinta; ver
	// `PantallaDelPueblo.soltarElMapaYEntrarElHeroe`). El muro de DC-002.159 esta medido en
	// console/dc/evidencia/dc-002.159-el-heroe-en-la-consola.md.
	heroeEnElPueblo = true

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

	// DC-002.159d: la SONDA DE CONTROL de la basura (ver `gc_159d.go`). Pide y tira esta cantidad de bytes por
	// cuadro sin guardarlos en ningun lado: si el recolector se los lleva, el neto de la fase no se mueve; si no,
	// el vivo crece a razon de estos bytes por cuadro. 1024 mientras se mide; 0 en la corrida buena.
	//
	// MEDIDO (DC-002.159d, 15 cuadros con 1024 B/cuadro): la fase `control` paso de 0 B/cuadro a 1032 B/cuadro.
	// O sea: el recolector de libgodc NO se lleva lo que ya nadie apunta mientras esta corriendo la ronda -- el
	// vivo sube con TODO lo que se pide y solo baja cuando recolecta. No hay una estructura que crezca: hay que
	// pedir menos.
	basuraDeControl159d = 0

	// LA POLITICA CUANDO LOS 4 BLOQUES SE LLENAN (DC-002.156), las mismas dos palancas del POC de DC-002.113b:
	// una imagen que no se uso en 2 cuadros se suelta y sus bytes vuelven al tramo, y lo que no entra se compone
	// sobre UN buffer de trabajo que se reusa en vez de pedirle un `make` al monton de Go por imagen. El buffer
	// es el de 80 KB del 113: la imagen mas grande de la cache del pueblo mide 71.680 B.
	desalojoEnCuadros = 2
	bufferDelVuelo    = 81920

	// DC-002.159i: EL TECHO DE UNA IMAGEN DE LA CACHE DEL MAPA. Es la pared que faltaba: con el runtime sano, la
	// corrida de 159h moria pidiendo UNA imagen de indices que no entra en la consola (4.737.760 B, y el buffer
	// al vuelo creciendo a 5.922.200 B para servirla => `makeslice: too large` / `gc_external_alloc: OOM`), y
	// ninguna de las dos palancas de arriba la ve venir: el desalojo y el al vuelo trabajan sobre imagenes que YA
	// entraron.
	//
	// EL NUMERO ES MEDIDO, NO ELEGIDO. `console/dc/medidas/sombra-159i` censa las 277 imagenes que el pueblo pide
	// de verdad (pisos, sombras y paredes, con las mismas cuentas de `tile_cache.go`): la mas grande es una pared
	// de 160x480 = 76.800 B y NINGUNA pasa los 81.920. O sea que el techo no le quita nada al nivel: corta el
	// pedido que no era de ninguna imagen y que tumbaba la corrida.
	limiteDeImagen159i = 81920

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
		"gcColecciones=", d2cGcColecciones(),
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

	// DC-002.157: EL CRONOMETRO POR TILE DEL RENDERER. Prendido, el renderer anota cuanto cuesta cada bloque
	// del mapa (cache / vista / estado / pintado) y cada una de sus cuatro pasadas, y la pantalla imprime el
	// censo al cerrar cada tramo del plan. El reloj es el del PVR (el mismo de las fases del cuadro), que es
	// un contador de 32 bits: da la vuelta cada 71 minutos, de sobra para una corrida.
	//
	// Va aca y no en el renderer porque en la PC el reloj lo pone el que mide; en la consola lo pone el
	// arranque, que es el unico que tiene el //extern del timer.
	d2maprenderer.FijarRelojDeLosTiles(func() uint { return d2cRelojUs() })

	println(frase, "cronometroDeLosTiles=", d2maprenderer.CronometroDeLosTilesPrendido())

	// ============================== 1b. LA CRUCETA (DC-002.159) ==============================
	// El servicio de entrada de la consola: `d2core/d2input/maple` lee el bus maple y traduce los botones a
	// lo que el motor pregunta (la cruceta son KeyUp/KeyDown/KeyLeft/KeyRight, y el cursor para los menus).
	// El refresco va por un callback porque el que puede llamar al bus es este main (es el unico archivo del
	// binario que lleva el //extern), y la pantalla lo llama al principio de cada cuadro, antes de preguntar
	// las teclas.
	// DC-002.159: las capas del heroe que entran en la consola. Medido: cada capa del compuesto son ~89 KB
	// VIVOS en el semiespacio del recolector (16 direcciones del modo Neutral) y el pueblo compuesto ya deja
	// 1,27 MB de 1,83 MB; con las cinco capas del equipamiento vacio el recolector muere con
	// `GC to-space overflow`. Entran cabeza, torso y piernas: el heroe se ve entero de silueta y camina.
	d2mapentity.CapasDelJugadorDeConsola = []d2enum.CompositeType{
		d2enum.CompositeTypeHead,
		d2enum.CompositeTypeTorso,
		d2enum.CompositeTypeLegs,
	}

	entrada := maple.NuevoServicio(cuadroAncho, cuadroAlto)

	// DC-002.159b: el paseo. Se envuelve el servicio del maple para que la cruceta tenga un guion (los ocho
	// sentidos de la pantalla) cuando no hay nadie apretando; el mando real sigue sumando. Ver paseo_159.go.
	paseo := nuevoPaseo159(entrada)

	println(frase, "cruceta: maple listo, hayControl=", maple.HayControl())

	// ============================== 1. LAS PALANCAS DE LA CONSOLA ==============================
	// El AssetManager del motor se arma DENTRO de `d2app.Create`, y los presupuestos de sus caches se congelan
	// al armarlos: por eso esto va antes de Create y no despues.
	d2asset.ArrancarComoConsola()

	// DC-002.159o: LAS PALANCAS DE ESTA TARJETA (ver `palancas_159o.go`): el camino del tinte forzado, el color
	// neutro que lo ejerce miles de veces sin cambiar un pixel, EL ARREGLO (resolver el color al empujarlo) y la
	// sonda. Van antes de `Create` para que el log diga con cual se corrio.
	palancas159o()

	// DC-002.159j: EL SELLO DE LOS BLOQUES DE CADA TILE. La pared `estilo=0 sec=0 tipo=1` pide 8.394.240 B en
	// la consola cuando su `.dt1` mide 288 de alto: el `Height` del tile esta bien y el `minY` de sus bloques no,
	// o sea que la rebanada `Blocks` del tile apunta a memoria que ya no es suya. Con esto prendido el motor
	// guarda, al cargar cada `.dt1`, lo que esa rebanada tenia (tamano, minY/maxY, direccion y una masa de los
	// `Y`) en un arreglo PROPIO, y el renderer compara al componer la pared: la diferencia se escribe en el log
	// con las dos versiones (ver d2core/d2map/d2mapengine/sellos_159j.go). Apagado no cambia nada del camino.
	d2mapengine.ActivarSellosDeLosTiles159j(true)

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

	// DC-002.159d: LA SONDA DE LA BASURA POR CUADRO. `runtime.ReadMemStats` de libgodc devuelve ceros (por eso
	// el instrumento de 159c se saco), pero el recolector si dice lo suyo por `gc_stats`: los bytes usados del
	// semiespacio y cuantas recolecciones hizo. La pantalla la muestrea en cada fase del cuadro y la imprime
	// cada 15 cuadros (ver d2game/d2gamescreen/gc_159d.go). En la consola el unico que tiene el //extern es este
	// main, asi que el puente se instala aca.
	d2gamescreen.SondaDelRecolector159d = func() (usado, colecciones int) {
		return d2cGcUsado(), d2cGcColecciones()
	}

	// DC-002.159d (SONDA DE CONTROL): los bytes que se piden y se tiran por cuadro, para saber si el recolector
	// se lleva lo que ya nadie apunta. 0 = apagada (el camino de la corrida buena).
	d2gamescreen.FijarBasuraDeControl159d(basuraDeControl159d)

	// DC-002.159h: las cuentas del cache HSV del renderer, para la linea `CUADRO` (`hsv=aciertos/fallos`).
	// Es la prueba positiva de que la sombra del compuesto pasa por el mapa que hasta esta tarjeta mataba
	// el SH-4; el mapa vive en d2core/d2render/ebiten y la pantalla no puede importarlo (el renderer es un
	// parametro), asi que el puente se instala aca, igual que la sonda del recolector.
	d2gamescreen.CuentasDelCaminoHSV159h = func() (pedidas, aciertos, fallos, sombras int) {
		pedidas, aciertos, fallos = renderEbiten.CuentasDelCaminoHSV159h()
		sombras = d2asset.SombrasDibujadas159h()

		return pedidas, aciertos, fallos, sombras
	}

	// DC-002.159f: LAS MARCAS FINAS. La cuenta vive en la pantalla (`d2gamescreen/marcas_159f.go`), pero los
	// paquetes que marcan no pueden importarla: cada uno deja su puente (`MarcaFina159f`) y aca se le suma la
	// BASE de su bloque de tramos. Con esto la linea `GC159D` parte `heroe`, `presente` y `plan` en las
	// operaciones que los pagan.
	d2asset.MarcaFina159f = func(id int) {
		d2gamescreen.Marcar159f(d2gamescreen.MarcasAssetBase159f + id)
	}
	d2app.MarcaFina159f = func(id int) {
		d2gamescreen.Marcar159f(d2gamescreen.MarcasAppBase159f + id)
	}
	ebiten.MarcaFina159f = func(id int) {
		d2gamescreen.Marcar159f(d2gamescreen.MarcasEbitenBase159f + id)
	}

	println(frase, "sondaDelRecolector=instalada gcVivo=", d2cGcUsado(), "gcTotal=", d2cGcTotal())

	// ============================== 2. EL MOTOR ==============================
	app := d2app.Create("DC-002.155", "gate0-pueblo")

	estado("despues-de-Create")

	// las palancas del arranque y la pantalla del pueblo
	app.ArrancarEnGate0(d2app.Gate0{
		// las tablas del nivel (DC-002.86: LvlTypes trae los tipos de nivel y LvlPrest los presets) y, para
		// el heroe (DC-002.159), `charstats.txt`: es la fila de la clase con la que se calculan la vida, el
		// mana y la estamina de nivel 1 (y la que `Player.Advance` consulta para el drenaje de estamina).
		Tablas: []string{d2resource.LevelType, d2resource.LevelPreset, d2resource.CharStats},

		SinAnimData: !heroeEnElPueblo, // con el heroe el animdata se carga solo con las claves de la amazona
		SinStrings:  true,             // el pueblo no dibuja texto
		SinGui:      true,             // sin cursor ni pantalla de carga (3,15 MB de RGBA)
		SinUi:       true,             // sin el sonido del boton (esta en d2sfx.mpq, que no va al disco)

		// DC-002.159: EL HEROE DEL MOTOR Y LA CRUCETA
		Heroe:            heroeEnElPueblo,
		Entrada:          paseo,
		RefrescarEntrada: func() { paseo.ActualizarDesdeElControl() },

		Region:       d2enum.RegionAct1Town,
		Preset:       1,
		ConEntidades: false, // las casillas solas: el pueblo, no sus habitantes

		OpcionesDelMapa: []d2maprenderer.Opcion{
			d2maprenderer.ComoConsola(cuadroAncho, cuadroAlto),
			d2maprenderer.CacheDeTilesPerezosa(),
			d2maprenderer.CacheDeTilesEnIndices(),
			d2maprenderer.CacheDeTilesEnBloquesDe(bloquesDeLaCache*bytesDelBloque, bytesDelBloque,
				pvr.VramReservar, "vram", sondaDelMonton),

			// DC-002.156: LA POLITICA CUANDO EL TRAMO SE LLENA. El primer cuadro del pueblo pide ~86 imagenes y en
			// los 4 bloques de VRAM entran ~44: lo que sobra NO puede irse al monton de Go de a un `make` por
			// imagen (medido en la corrida de esta tarjeta: 633.648 B en el primer cuadro y `out of memory` en el
			// recolector con el monton de C lleno). Las dos palancas son las mismas del POC de DC-002.113b, que es
			// el que dibuja este cuadro en el host: el desalojo devuelve los bytes de lo que no se uso en 2 cuadros
			// y la composicion al vuelo arma lo que no entra en UN buffer de trabajo que se reusa.
			d2maprenderer.ConDesalojoDeLaCache(desalojoEnCuadros),
			d2maprenderer.ConComposicionAlVuelo(bufferDelVuelo),

			// DC-002.159i: LA PARED QUE FALTABA. Una imagen de la cache que no entra en la consola (el pedido de
			// 4.737.760 B que mataba la corrida de 159h) se dice en el log UNA vez y el cuadro sigue sin ella;
			// sin este techo, el buffer al vuelo crece a `n + n/4` y el `make` es el que tumba el runtime.
			d2maprenderer.ConLimiteDeImagen(limiteDeImagen159i),
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
