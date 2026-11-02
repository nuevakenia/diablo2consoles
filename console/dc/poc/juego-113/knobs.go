package main

// DC-002.113 -- LAS PALANCAS DEL POC DEL CONTROL QUE MUEVE AL JUGADOR.
//
// TODAS las del pueblo (DC-002.103...112) quedan con los MISMOS valores con los que el pueblo ya se dibuja y
// se presenta igual al host (crc32 1437022778): esta tarjeta no toca el camino del mapa. Lo que se agrega son
// las de esta tarjeta:
//
//	jugadorKnob      encendido/apagado  -- la corrida del criterio y la de referencia del pueblo solo
//	modoQuieto/dir   NU direccion 0     -- el cuadro de DC-002.112 (crc32 1363871674), que es la referencia
//	modoCaminando    TW                 -- la animacion de caminar EN EL PUEBLO (el motor usa TW en el pueblo)
//	dirsCaminando    cuantas de las 8 direcciones de la cruceta se componen (A/B de memoria)
//	camara           siguiendo | fija   -- la camara sigue al jugador (siguiendo) o se queda quieta (fija)
//	inyectarGuion    false | true       -- el guion de la cruceta lo pone el SHIM (no hay mando fisico)
//
// La leccion de DC-002.90 sigue valiendo: este archivo es la UNICA fuente de verdad; el entorno solo lo PISA y,
// cuando lo pisa, el build lo avisa en grande y el valor queda escrito en el knobs.go del directorio de build
// (el binario dice con cual se compilo).
const (
	// ============================== LAS DEL PUEBLO (DC-002.103/.112) ==============================
	//
	// orden: "contrato" | "prestamo" | "tarjeta". La referencia del POC es "prestamo" (el orden legal que llega
	// mas lejos: se devuelve el buffer prestado de la lectura de los `.dt1` y el arena de bloques se suelta
	// despues del primer cuadro, con la cache ya armada).
	orden = "prestamo"

	// cuadros: cuantos cuadros del GUION se dibujan (miden). Despues se dibujan tres mas, los del cierre: el que
	// vuelve al origen, el de la repetibilidad y el que devuelve al jugador a donde quedo.
	//
	// 22 Y NO 60 (medido en esta tarjeta): el cuadro del pueblo son ~670-760 ms de PVR en Flycast (1,3 cuadros/s),
	// asi que 60 cuadros son mas de un minuto de corrida y el cierre no se llega a ver. El guion de la cruceta
	// (8 direcciones de 1,5 s en run-flycast-113.sh) gasta ~21 cuadros: 22 deja el cierre dos cuadros despues.
	cuadros = 22

	// modoTablas: "stream" (el camino viejo) o "fuente" (el tramo de la fuente con dueno, DC-002.102).
	modoTablas = "fuente"

	// soltarLecturas: devolver el tramo de lecturas de la fuente antes y despues de componer.
	soltarLecturas = true

	// tramoDeLaCache: los pixeles de la cache en indices se piden a UN tramo de este tamano. En DC-002.112 eran
	// 2.723.840 B (86 imagenes, 0 derrames) y alcanzaban porque la camara NO se movia. Esta tarjeta camina: cada
	// celda nueva pide imagenes que no estaban (medido: ~26 KB por pieza) y lo que no entra en el tramo se DERRAMA
	// al monton de Go, donde es VIVO y empuja al recolector hasta tumbarlo ("GC to-space overflow"). Por eso el
	// tramo va al maximo que la VRAM servible deja (4.740.808 B disponibles; 12 bloques de 348.160 = 4.177.920 B).
	tramoDeLaCache = 4177920

	// tramoEnPedazos: el MISMO tramo pedido en bloques de este tamano (340 KB = 68 filas de 5.120 B).
	tramoEnPedazos = 348160

	// pedazoLoDiceElMonton: el tamano de cada bloque lo decide el monton (la sonda `d2c_servible`).
	pedazoLoDiceElMonton = true

	// cacheEnVram: los bloques de la cache se piden a la VRAM DEL PVR (`pvr_mem_malloc`) en vez del monton de C.
	cacheEnVram = true

	// pruebaDeVram: escribir un patron en un bloque de la VRAM y releerlo, al arrancar el video.
	pruebaDeVram = true

	// tramoAlArranque: reservar el tramo de la cache ANTES de cargar nada (con el tramo EN PEDAZOS no se
	// reserva nada al arranque: los bloques se piden en el primer cuadro).
	tramoAlArranque = false

	// soltarDespuesDeComponer: "nada" | "tablas" | "fuentes" | "todo". La referencia es "todo" (679.440 B).
	soltarDespuesDeComponer = "todo"

	// scratchPorArchivo: el buffer prestado con el que se lee cada `.dt1` se pide del tamano de ESE archivo.
	scratchPorArchivo = true

	// tramoDelArenaPorElMonton: el tamano de cada tramo del ARENA DE BLOQUES lo decide el monton.
	tramoDelArenaPorElMonton = true

	// soltarTramosAlCambiarDeNivel: los tramos del arena de bloques vuelven al monton de C al tirar un nivel.
	soltarTramosAlCambiarDeNivel = true

	// arenaDelDS1: las celdas de las capas de un `.ds1` en UN bloque del monton de C. Apagada (DC-002.110).
	arenaDelDS1 = "apagado"

	// ============================== LAS DEL JUGADOR (DC-002.112) ==============================
	//
	// jugadorKnob: "encendido" (el pueblo CON el jugador) | "apagado" (la referencia del pueblo solo).
	jugadorKnob = "encendido"

	// modoQuieto / dirQuieto: el cuadro del jugador parado. NU en la direccion 0 es el cuadro de DC-002.112
	// (caja 26x79, 896 pixeles, crc32 1857478036), y es la referencia contra la que se mide que el cuadro
	// CON el jugador sigue dando crc32 1363871674.
	modoQuieto = "NU"
	dirQuieto  = 0

	// msQuieto: 128,00 ms por cuadro del ciclo NU (medido del `animdata.d2` en DC-002.49).
	msQuieto = 128

	// ============================== LAS DE ESTA TARJETA (DC-002.113) ==============================
	//
	// modoCaminando: TW es la animacion de caminar DENTRO DEL PUEBLO (`AMTWhth.COF`, 8 cuadros por direccion a
	// 40,00 ms; medido en DC-002.49: `animdata(AMTWHTH) 40.00 ms/cuadro ciclo=320 ms`). El motor usa TW en el
	// pueblo (`d2player`: `ModeTownWalk`); WL es la de afuera del pueblo y no es la que corresponde aca.
	modoCaminando = "TW"

	// msCaminando: 40,00 ms por cuadro de TW (7,81 -> 25 cuadros/s), del mismo `animdata.d2`.
	msCaminando = 40

	// dirsCaminando: cuantas de las 8 direcciones que la cruceta puede pedir se componen al arranque. Las 8
	// cuestan 148.720 B de indices (medido en el host en DC-002.49b); con menos, el giro a una direccion no
	// compuesta se queda sin grafico, asi que este numero es el del A/B de memoria, no una opcion de uso.
	dirsCaminando = 8

	// camara: "siguiendo" = la camara se pone en el jugador en cada cuadro (lo que hace el motor con el heroe:
	// `d2gamescreen` le pasa `SetCameraTarget`), que es lo que deja al jugador EN PANTALLA mientras camina --
	// una celda son 160x160 px y la ventana de 640x480 muestra 4x3 celdas, asi que con la camara quieta el
	// jugador sale del cuadro en dos celdas (medido: ancla 813 con la pantalla de 640). "fija" = la camara se
	// queda en la celda de arranque (el A/B que mide cuanto se mueve EN PANTALLA).
	camara = "siguiendo"

	// inyectarGuion: cuando el emulador no tiene mando, el estado del maple lo pone el SHIM
	// (`d2c_control_inyectar`, en console/dc/std/csrc/control_dc.c) y el POC lo empuja cuadro a cuadro desde la
	// tabla `guionInyectado` de movimiento.go. Con esto encendido la evidencia lo DICE: no hay mando fisico en
	// el bus, el estado viene del shim. La corrida del criterio va con esto APAGADO y las teclas del teclado
	// virtual del emulador (el estado del maple lo arma el PROPIO Flycast).
	inyectarGuion = false

	// umbralDelRecolector: a que porcentaje del semiespacio salta el recolector (`d2c_gc_percent` ->
	// `debug.SetGCPercent` de libgodc). OJO CON LA ESCALA, que no es la de Go: `gc_heap.c` traduce 100 a "salta
	// al 75 % del semiespacio" (1.376.256 de 1.835.008) y cualquier otro valor a "salta en ESE porcentaje".
	//
	// LO QUE MIDIO ESTA TARJETA. Con 100 (el default de DC-002.112) el POC NO PASA de componer el pueblo: el
	// jugador de esta tarjeta trae 8 direcciones de TW en vez de una (148.720 B de indices) y el conjunto vivo
	// queda por encima del umbral del 75 %; a partir de ahi el recolector corre en CADA asignacion y no libera
	// nada. Medido en Flycast: 2.333 recolecciones seguidas (`ARQ-MARCA gc_collect`) y la composicion clavada
	// despues de leer el `.ds1`. No es que falte memoria (quedan 136 KB de monton de C y 36 % de semiespacio):
	// es el umbral. Con 95 el umbral queda en 1.743.257 B y la misma corrida compone.
	umbralDelRecolector = 95

	// gcCadaCuadros: NO se usa. Se dejo la palanca porque la medicion de esta tarjeta la descarto: pedirle un
	// `runtime.GC()` al recolector desde el callback del ciclo del PVR CLAVA el ciclo (dos corridas quedaron
	// en el cuadro 7 sin volver a presentar). El recolector entra solo por umbral, que es como funciona.
	gcCadaCuadros = 0

	// ============================== LAS DE DC-002.113b ==============================
	//
	// desalojoEnCuadros: la politica cuando el tramo de la cache se llena. Una imagen que no se uso en N cuadros
	// se SUELTA y sus bytes vuelven al tramo (se reusan). 0 = apagada, y entonces la cache es de solo agregar
	// como en DC-002.113 (27 derrames, 732.160 B y el `GC to-space overflow` de la corrida larga).
	// N=2 = un cuadro entero de margen sobre lo que el cuadro esta pintando.
	desalojoEnCuadros = 2

	// componerAlVuelo: la OTRA politica (se mide contra el desalojo). Con el tramo lleno, la imagen se compone
	// sobre un buffer de trabajo y se pinta SIN cachearse: no crece nada, pero el tile se decodifica en cada
	// cuadro que lo pida. Las dos juntas no tienen sentido: con desalojo casi siempre hay lugar.
	componerAlVuelo = false

	// bufferDelVuelo: el tamano del buffer de trabajo de la composicion al vuelo. La imagen mas grande de la
	// cache del pueblo son 71.680 B (160x448 a 1 B/pixel), asi que 80 KB con aire.
	bufferDelVuelo = 81920
)
