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
	// 24 Y NO 22 (DC-002.114): el guion del cooperativo son tres tramos de 4 s con 1 s de pausa (14 s, ver
	// `guionCoop` en coop.go) y el cuadro del pueblo son ~670-760 ms de PVR en Flycast (1,3 cuadros/s): 14 s son
	// ~19 cuadros. Con los 22 de DC-002.113 sobraba margen pero los tramos de 6 s separaban demasiado a los dos
	// jugadores; con tramos de 4 s, 24 cuadros dejan el guion completo mas el cierre.
	//
	// 84 PARA EL GUION DEL LEASH (DC-002.115): son 8 tramos de 7 s con 1 s de pausa = 63 s, y el cuadro de este
	// POC tarda ~1 s en Flycast (medido en DC-002.114: 1,0..1,2 s por cuadro con dos jugadores). 63 s son ~63
	// cuadros; 84 dejan el guion entero mas el cierre aunque una corrida salga mas lenta que la anterior.
	//
	// 12 PARA EL GUION DEL SPLIT (DC-002.116): el guion de esta tarjeta son dos tramos de 2 s (IDA y VUELTA
	// caminando los dos para el mismo lado) = 4 s mas las pausas, y el cuadro del split tarda ~1,2-2 s en
	// Flycast (dos pasadas del mapa). 12 cuadros cubren el guion entero con margen y dejan los tres del cierre.
	// No hacen falta mas: lo que se mide es el costo por cuadro, no una caminata larga.
	//
	// 14 PARA EL GUION DE LOS CUATRO (DC-002.117): el guion de esta tarjeta es un QUIETO de 1 s (el cuadro 1, el de
	// DC-002.112 con un solo jugador) mas tres tramos de 2 s (los cuatro juntos para un lado, cada uno en SU
	// direccion, y los cuatro de vuelta) = 7 s mas las pausas (11 s), y el cuadro tarda ~1,1-1,4 s en Flycast (una
	// pasada del mapa, cuatro recuadros que guardar/pintar/restaurar). 14 cuadros cubren el guion y dejan los tres
	// del cierre. NO SE SUBE: cada cuadro de caminata mete imagenes nuevas en la cache (sobre medido: ~111 imagenes;
	// con 113 empieza a derramar y el emulador se cae, ver tramoDeLaCache).
	// 6 PARA EL CICLO CON LOS CUATRO (DC-002.120): el guion de esta tarjeta son los cuatro caminando en el
	// pueblo (los tramos de `guionCuatro`: QUIETO, los cuatro juntos para un lado, cada uno en su direccion y
	// los cuatro de vuelta) y despues LAS TRES FASES del cambio de nivel, que no son cuadros del guion: cada
	// fase compone un nivel (~25 s en Flycast) y se queda `cuadrosPorFase` cuadros en pantalla. Con 6 cuadros
	// de guion (~7 s) los cuatro caminan de sobra y la corrida entra en la ventana de captura.
	cuadros = 6

	// modoTablas: "stream" (el camino viejo) o "fuente" (el tramo de la fuente con dueno, DC-002.102).
	modoTablas = "fuente"

	// soltarLecturas: devolver el tramo de lecturas de la fuente antes y despues de componer.
	soltarLecturas = true

	// tramoDeLaCache: los pixeles de la cache en indices se piden a UN tramo de este tamano. En DC-002.112 eran
	// 2.723.840 B (86 imagenes, 0 derrames) y alcanzaban porque la camara NO se movia. Esta tarjeta camina: cada
	// celda nueva pide imagenes que no estaban (medido: ~26 KB por pieza) y lo que no entra en el tramo se DERRAMA
	// al monton de Go, donde es VIVO y empuja al recolector hasta tumbarlo ("GC to-space overflow"). Por eso el
	// tramo va al maximo que la VRAM servible deja.
	//
	// 10 BLOQUES Y NO 12 (DC-002.116), Y ESTO ES LO QUE CUESTA EL SPLIT. 12 bloques (4.177.920) era el techo
	// cuando el lienzo de 640x480 vivia en la VRAM y la camara era UNA. Con el split los dos lienzos de 320x480 se
	// llevan 1.228.800 B de la misma VRAM (los mismos que el de 640x480), pero la cache tiene que tener las DOS
	// zonas del mapa, y ahi ya no entra todo:
	//   disponible despues de los lienzos = 4.740.808 - 1.228.800 = 3.511.944 B
	//   10 bloques = 3.481.600 B (deja 30.344 B al PVR); 13 (4.526.080) no entran y el PVR contesta
	//   "out of PVR memory; texture heap probably corrupted" y el programa se cae (medido).
	// Los 3.276.800 B que cuestan las dos ventanas de la corrida del criterio entran en los 10 bloques. Con las dos
	// zonas de (28,20)+(50,20) (3.694.080 B) NO entran: eso se midio derramando al monton de Go y muriendo con
	// "GC to-space overflow", que es el motivo de que el jugador 2 arranque en una zona barata (ver celdaP2X).
	//
	// 10 BLOQUES EN DC-002.117, MEDIDO Y NO SUPUESTO. La cuenta prometia 12 (4.177.920 B) porque sin split los
	// lienzos de 320x480 no se piden; el POC lo pidio y la ARENA le dio 10 (pedazos= 10, bytes= 3.481.600, medido
	// en la primera corrida de esta tarjeta): el tramo no lo limita la cuenta del renderer sino lo que el PVR
	// servible entrega de verdad. Se deja el numero que la arena sostiene, y el techo queda anotado arriba para que
	// nadie vuelva a pedir 12 creyendo que entran.
	//
	// Y EL SOBRE DE LA CACHE ES ~111 IMAGENES. Con 16 cuadros y el guion de tres tramos de 3 s la corrida cerro con
	// 111 imagenes y 0 derrames; con 18 cuadros llego a 113, empezo a DERRAMAR (25 derrames, 558.080 B al monton de
	// Go) y a los dos cuadros el emulador se cayo con "Fatal: SH4 exception when blocked" (medido). Por eso el guion
	// de esta tarjeta camina menos celdas: lo que se mide es el costo de CUATRO en el cuadro, no una caminata larga.
	tramoDeLaCache = 3481600

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

	// arenaDelDS1: las celdas de las capas de un `.ds1` en UN bloque del monton de C. Apagada en DC-002.110 (con
	// las palancas de aquella tarjeta el monton de C se quedaba sin lugar), pero ENCENDIDA aca: con el ciclo y los
	// cuatro jugadores el monton de C tiene lugar (pico 10.199.216 B de 10.632.224) y las celdas del `.ds1`
	// (186.960 B) se sacan del semiespacio, que es el recurso que falta. `ARENA_DS1=apagado` deja el A/B.
	arenaDelDS1 = "encendido"

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

	// ============================== LAS DE ESTA TARJETA (DC-002.114) ==============================
	//
	// jugadoresKnob: cuantos jugadores entran (1..4). Es LA palanca del A/B de memoria y de ms por cuadro: el
	// binario es el mismo y lo unico que cambia es este numero, asi que los cuatro casos de la tabla de la tarjeta
	// salen de la MISMA compilacion con `JUGADORES=1|2|3|4`.
	//
	// DC-002.117: 4 por omision. El tope es 4 porque son los cuatro puertos del bus maple (y el tope del cooperativo
	// del juego); 1 y 2 son las referencias de DC-002.113 y DC-002.115.
	jugadoresKnob = 4

	// puertoJugador1..4: el puerto del bus maple de cada jugador (0 = A, 1 = B, 2 = C, 3 = D). Cada uno lee SU
	// puerto con `d2c_control_leer_puerto` (maple_enum_dev): no es "el primer control del bus", es la direccion
	// fisica. Con un solo control en el bus (la corrida de Flycast) los otros tres puertos van por el guion.
	puertoJugador1 = 0
	puertoJugador2 = 1
	puertoJugador3 = 2
	puertoJugador4 = 3

	// celdaP2X / celdaP2Y: donde arranca el jugador 2, en CELDAS del mapa, respecto de la celda de arranque (28,20).
	//
	// DC-002.116: (52,12), o sea 24 celdas en x y 8 en y -- distancia 25,32 celdas, mas de las 20 que pide la
	// tarjeta -- y es una zona BARATA del pueblo: un cuadro de 320x480 ahi pide 33 imagenes (675.840 B) contra las
	// las 51 (1.093.120 B) de (50,20). Importa porque con el split la cache tiene que tener las DOS zonas: 2.600.960
	// (P1 en (28,20)) + 675.840 = 3.276.800 B, que entran en los 10 bloques del tramo; con (50,20) eran 3.694.080
	// y no entraban (ver tramoDeLaCache). Las dos mitades muestran mapa lleno: 153.600 de 153.600 pixeles.
	//
	// DC-002.117: (3,0) y NO (24,-8). Con tres y cuatro jugadores NO HAY SPLIT (es otra tarjeta) y el leash los
	// tiene que tener a TODOS en un solo lienzo de 640x480, asi que el grupo arranca junto: los cuatro en un cuadrado
	// de 3x3 celdas alrededor de (29,5;21,5). Medido con la proyeccion del motor ((x-y)*80, (x+y)*40): los cuatro
	// quedan entre -240 y +240 px en x y entre -120 y +199 px en y respecto de la camara del centro, o sea adentro
	// de los 640x480 con la caja de 26x79 de cada uno. Ademas el grupo entero cabe en la MISMA ventana de tiles que
	// P1 (una sola zona del mapa, no dos), que es lo que la VRAM de la cache necesita.
	celdaP2X = 3.0
	celdaP2Y = 0.0

	// celdaP3X/Y y celdaP4X/Y: donde arrancan el jugador 3 y el 4, en CELDAS desde la celda del POC (28,20), con la
	// misma cuenta que el 2: el cuadrado de 3x3 celdas. La separacion entre los dos mas lejanos (la diagonal del
	// cuadrado) son 4,24 celdas, dentro del leash medido (5,25 celdas, el peor caso de las 8 direcciones).
	celdaP3X = 0.0
	celdaP3Y = 3.0
	celdaP4X = 3.0
	celdaP4Y = 3.0

	// compartirJugador2: "si" = el jugador 2 dibuja con los MISMOS cuadros compuestos del jugador 1 (los `.dcc`
	// decodificados y pegados se comparten: 0 B de assets para el segundo jugador; lo que es por jugador es el
	// estado). "no" no esta implementado y el POC lo dice: medir el camino caro es otra tarjeta.
	compartirJugador2 = "si"

	// guionJugador2: "si" = el estado del puerto del jugador 2 lo pone el guion del shim (`botonesCoop`, coop.go)
	// porque en la corrida de Flycast hay UN solo control en el bus (el teclado va al puerto A). "no" = se lee el bus
	// y el jugador 2 se queda quieto si no hay aparato. El log dice siempre cual de los dos caminos corrio.
	guionJugador2 = "si"

	// inyectarJugador1: "si" = el estado del puerto del jugador 1 tambien lo pone el guion (corrida con los DOS
	// guionados, que es la que mide los tramos SOLO-P1 / SOLO-P2 / LOS-DOS alineados al reloj de la consola). "no" =
	// el jugador 1 lee su control del bus (la corrida del criterio de DC-002.113, con el teclado virtual del host).
	//
	// DC-002.116: "si" por omision. La corrida del criterio del split no depende del teclado del emulador: los dos
	// jugadores van por el guion y las dos camaras se mueven solas. Es el mismo camino del input (bits del maple ->
	// tabla del backend -> teclas del motor), y el log dice en cada linea que el estado viene del shim.
	inyectarJugador1 = "si"

	// ============================== LAS DE ESTA TARJETA (DC-002.115) ==============================
	//
	// leashCeldas: la DISTANCIA MAXIMA entre jugadores, en CELDAS del mapa. 0 = la que se MIDA al arranque con la
	// proyeccion del motor y los cuadros compuestos (leash.go: el barrido de las 8 direcciones de separacion, y se
	// usa la PEOR). Con un numero puesto a mano, el log avisa si es mayor que la medida, porque entonces los
	// jugadores se pueden salir del lienzo: la palanca sirve para el A/B, no para "elegir" el resultado.
	leashCeldas = 0.0

	// guionPoc: "coop" = el guion de DC-002.114 (SOLO-P1 / SOLO-P2 / LOS-DOS: la independencia del input) |
	// "leash" = los 8 tramos de 7 s de DC-002.115, en los que el jugador 1 camina en una direccion de la cruceta
	// y el 2 en la CONTRARIA | "split" = 2 tramos de 2 s con los DOS caminando para el mismo lado | "split-quieto" =
	// los dos quietos (LA CORRIDA DEL CRITERIO: la cache del split no aguanta el movimiento, ver guionSplit) |
	// "cuatro" = los tres tramos de 3 s de DC-002.117 (los cuatro juntos, cada uno en SU direccion, y los cuatro de
	// vuelta; ver guionCuatro).
	guionPoc = "cuatro"

	// ============================== LAS DE ESTA TARJETA (DC-002.116) ==============================
	//
	// splitKnob: "si" = la pantalla partida en DOS lienzos de 320x480, uno por jugador, con una camara cada uno
	// (lo de esta tarjeta) | "no" = el cuadro de DC-002.115 (un lienzo de 640x480 con la camara en el centro del
	// grupo y el leash). Es LA palanca del A/B que pide la tarjeta: el binario es el mismo y lo unico que cambia
	// es este valor, asi que los ms por cuadro con UN lienzo y con DOS salen de la misma corrida del motor.
	//
	// CON EL SPLIT PUESTO, EL LEASH NO SE APLICA: cada jugador se va con su camara. El leash sigue siendo el
	// camino de "no" (y su medida se sigue imprimiendo en esa corrida).
	//
	// DC-002.117: "no" POR OMISION. Esta tarjeta mide TRES Y CUATRO jugadores en UNA pantalla (con el leash): el
	// split llega hasta dos lienzos y el de 3 y 4 es otra tarjeta (`prepararElSplit` rechaza cualquier numero de
	// jugadores que no sea 2, y lo dice). Las palancas y el codigo del split quedan enteros y CORREN (medido):
	// `SPLIT=si JUGADORES=2` da los dos lienzos de 320x480, las dos camaras y el ancla 160,283 en las dos mitades.
	// Para que sea EL MISMO cuadro de DC-002.116 falta el arranque de aquella tarjeta, que aca cambio a proposito
	// (el grupo de los cuatro arranca a 3 celdas): con `CELDA_P2="24 -8"` los dos vuelven a estar a 25 celdas.
	//
	// DC-002.118: "si" POR OMISION, y ahora el split llega a UNO, DOS, TRES y CUATRO (`repartirElSplit`). El A/B de
	// esta tarjeta es la MISMA compilacion con otra palanca: "si" (la rejilla de recuadros) | "no" (la pantalla
	// entera de DC-002.115/117 con el leash). Con `JUGADORES=2` y "si" el reparto es el de DC-002.116 sin tocar una
	// linea; con 4 son los cuatro cuadrantes de 320x240, uno por camara.
	splitKnob = "si"

	// ============================== LAS DE ESTA TARJETA (DC-002.118) ==============================
	//
	// repartoDeTresKnob: la forma del split de TRES, que es el unico que tiene dos razonables.
	//
	//	"grande"  (por omision) 320x480 para el jugador 1 y dos de 320x240 apiladas a la derecha.
	//	"tiras"                  tres tiras de 640x160, una por jugador, de arriba abajo.
	//
	// LAS DOS CUESTAN LO MISMO -- los mismos 307.200 pixeles, los mismos bytes de VRAM y los mismos ms de mapa por
	// cuadro, porque el costo es por pixel y no por lienzo (ver la tabla de la evidencia) --, asi que lo que decide
	// es lo que VE cada jugador: 640x160 son 4x1 celdas, una franja donde no se ve el piso de adelante ni el de
	// atras; 320x240 son 2x1,5 celdas con el piso alrededor. Queda la palanca para el A/B medido.
	repartoDeTresKnob = "grande"

	// ============================== LAS DE ESTA TARJETA (DC-002.117) ==============================
	//
	// leashDeGrupo: el leash de 3 y 4 jugadores. Con dos se aplica el de DC-002.115 (radial contra el OTRO
	// jugador); con tres o cuatro el que manda es el GRUPO: cada jugador no se puede alejar del centro del grupo
	// mas que `leashCeldas/2` celdas -- el mismo radio con el que DC-002.115 midio que la caja entra en el lienzo
	// (alli los dos se ponian a +-d/2 del centro, o sea al radio d/2). Puesto en "no" el POC aplica el leash de a
	// pares de DC-002.115 tambien con tres y cuatro, que NO alcanza para el grupo: es el A/B que dice por que el
	// de grupo existe, y el log lo avisa.
	leashDeGrupo = true

	// ============================== LAS DE ESTA TARJETA (DC-002.120) ==============================
	//
	// faseKnob: "ciclo" = despues del guion de los cuatro se corren LAS TRES FASES del cambio de nivel
	// (ENTRAR / TRANSICION / VOLVER) con los cuatro DIBUJADOS y el leash puesto, que es lo que la tarjeta mide:
	// el mundo cambiando con la gente adentro. "apagado" = la corrida de DC-002.117 tal cual (el ciclo no toca
	// una linea de ese camino).
	//
	// DC-002.118: "apagado" POR OMISION, y el codigo del ciclo queda ENTERO. Esta tarjeta mide el REPARTO de la
	// pantalla: con las fases encendidas la corrida se va ~75 s en componer tres niveles y lo que se mide (ms por
	// cuadro con 1, 2, 3 y 4 lienzos) queda mezclado con la composicion. `FASE=ciclo` sigue corriendo el ciclo de
	// DC-002.120 con la rejilla puesta, que es el siguiente paso y no lo de esta tarjeta.
	faseKnob = "apagado"

	// transicionPreset / transicionArchivo: el `.ds1` que compone la fase TRANSICION. El preset 1 del pueblo
	// trae cuatro archivos (TownN1/TownE1/TownS1/TownW1, medido en el host en DC-002.111), asi que el archivo 1
	// es un mapa DISTINTO --TownE1, 57x41 como el pueblo-- sin cambiar de level type ni de paleta. La VUELTA
	// compone otra vez el archivo 0.
	transicionPreset  = 1
	transicionArchivo = 1

	// ordenDeLasFases: cual de los dos ordenes del cambio de nivel usa el ciclo.
	//
	//	a  soltar la cache del nivel que se deja ANTES de tocar el nivel nuevo. Es el que DC-002.109 MIDIO que
	//	   gana (en el modo del monton es el unico que existe, y con la cache en la VRAM deja el doble de lugar).
	//	b  no soltar nada: la cache vieja sigue tomada mientras se compone el nivel nuevo. Queda para el A/B.
	ordenDeLasFases = "a"

	// cuadrosPorFase: cuantos cuadros se queda dibujada cada fase. No es un detalle de la medicion sino de la
	// EVIDENCIA: el cuadro de este POC tarda ~1,1-1,4 s en Flycast y una captura cada 2 s necesita mas de un
	// cuadro por fase para caerle adentro. Los ms por cuadro se miden en cada uno y el resumen da el medio.
	cuadrosPorFase = 3

	// recolectarDespuesDeCargarElDs1 (DC-002.111): un `runtime.GC()` DENTRO de `componerNivelDeLaFase`, con el
	// `.ds1` nuevo ya cargado y antes de tocar un `.dt1` del nivel nuevo. El barrido de pila de libgodc es
	// conservador: el `.ds1` del nivel que se deja sigue en el marco muerto de la fase anterior y cuenta como
	// raiz. La recoleccion que corre con el marco ya reescrito es la que lo tira (medido: 409.096 B).
	recolectarDespuesDeCargarElDs1 = true

	// soltarEnElCiclo: si en el ciclo se sueltan (o no) las fuentes MPQ y las tablas del compositor despues de
	// componer. `false`, que es el default medido de DC-002.111: no se sueltan, o sea la fase siguiente NO
	// tiene que reabrir nada, y esos bytes (679.440 B en el monton de C, ~120-146 KB de vivos en el semiespacio)
	// no se piden en cada fase. Es la palanca que hizo entrar la VUELTA de DC-002.111.
	soltarEnElCiclo = false

	// soltarCaminandoEnElCiclo: antes del primer cuadro de fase se SUELTAN los 8 ciclos de caminar del jugador
	// (148.720 B de indices, el unico vivo que los cuatro le agregan al semiespacio del recolector) y se
	// recolecta. Las fases son QUIETAS: los cuatro quedan en la formacion con el ciclo NU, que es lo que se
	// dibuja igual. En "no" queda el A/B: el mismo binario con los ciclos vivos.
	//
	// MEDIDO Y APAGADA: el recolector de libgodc es conservador con la pila, y los ciclos que quedan en ranuras
	// muertas de la pila NO se sueltan (medido: soltar 148.720 B movio el semiespacio 40 B). Con las casillas
	// reusadas (`reusarLasCasillas`) no hace falta: la VUELTA entra igual. Se deja la palanca y el log, porque el
	// numero es parte de lo aprendido.
	soltarCaminandoEnElCiclo = false

	// reusarLasCasillas: las casillas del nivel nuevo se escriben ENCIMA del nivel que se deja (`Stamp.TileEn`) en
	// vez de pedir ~650.000 B nuevos y tirar los del anterior. Es lo que hace entrar la VUELTA con los cuatro
	// dibujados encima. Con "no", la corrida muere con `GC to-space overflow` (medido: 1.760.000 de 1.835.008).
	// El contenido de las casillas es identico byte a byte: los crc32 del cuadro (1437022778 / 584380401) lo
	// confirman, y son los mismos del host.
	reusarLasCasillas = true

	// ============================== LAS DE ESTA TARJETA (DC-002.119) ==============================
	//
	// tramosPropios: LAS CATEGORIAS GORDAS DEL POC VIVEN EN UN TRAMO DEL MONTON DE C, CON DUENO (decision 1b del
	// ADR 0001). Encendida, el POC pide dos tramos -- los 72 cuadros compuestos del jugador (165.152 B) y las cajas
	// de guardado de cada jugador (~8,8 KB cada una) -- y los reparte adentro; apagada, todo queda en el semiespacio
	// como en DC-002.118.
	//
	// ES LA PALANCA DEL A/B DE LA TARJETA y por eso es un booleano y no un tamano: el binario es el MISMO y lo unico
	// que cambia entre las dos corridas es de donde salen esos bytes (el semiespacio del recolector o el monton de C
	// con `malloc`). El cuadro tiene que dar el mismo crc32 en las dos: los bytes son los mismos, mudados.
	tramosPropios = true

	// censoPorEtapas: el censo del semiespacio por origen (censo.go) con recoleccion A PEDIDO en cada etapa del
	// arranque. Encendido por omision: es el instrumento de la tarjeta y no toca el ciclo (la recoleccion a pedido
	// solo corre en el arranque, antes del primer cuadro). Apagado, el POC queda con los `SEMIESPACIO` de siempre.
	censoPorEtapas = true
)
