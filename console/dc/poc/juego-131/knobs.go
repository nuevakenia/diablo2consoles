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

	// celdaP1X / celdaP1Y (DC-002.122): donde arranca el JUGADOR 1, en celdas desde la celda del POC (28,20).
	// En (0,0) es el cuadro de DC-002.112 de siempre. La palanca es de esta tarjeta porque el alcance del golpe se
	// mide por la posicion: en (0,0) el jugador queda en DIAGONAL del bicho (que nace en (29,21)) y la tabla del
	// exe da 8, fuera del alcance de 7; con (0,1) queda en la celda de al lado, da 5 y SI alcanza.
	celdaP1X = 0.0
	celdaP1Y = 0.0

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
	splitKnob = "no"

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
	faseKnob = "ciclo"

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

	// ============================== LAS DE ESTA TARJETA (DC-002.121): EL MONSTRUO ==============================
	//
	// monstruosKnob: cuantos monstruos patrullan el mapa. 0 = APAGADO, y la corrida es la de DC-002.120 bit a
	// bit (es el 0 de la tabla "0 / 1 / N monstruos" de los ms por cuadro). El tope es 4 (`maxBichosBicho`):
	// mas bichos que carriles de patrulla no caben en la pantalla que la camara esta mirando.
	//
	// LOS CUATRO BICHOS SON DEL MISMO TIPO Y COMPARTEN LOS CUADROS: es la medicion que la tarjeta pide ("los
	// .dcc de monstruo se comparten?"). El que quiera el A/B de NO compartirlos tiene el numero en el log:
	// `noCompartidoSiCadaUnoComponia`.
	monstruosKnob = 1

	// celdaBichoX / celdaBichoY: la celda donde nace el bicho 1, en CELDAS del mapa: la celda del POC (28,20)
	// corrida uno en cada eje -- la MISMA que eligio DC-002.50. Y no es un detalle: es una de las celdas por las
	// que pasa el jugador cuando la cruceta pide ABAJO (`DireccionDelPiso(0,1)` = (+x,+y) en celdas, que con el
	// paso de un cuadro de este POC llega justo a esa celda), asi que el guion `choque` la mide de verdad. El
	// `.ds1` del pueblo no trae monstruos -- solo 41 Objects --, asi que no hay otra posicion que usar.
	// DC-002.127: `celdaBichoX/Y` se fueron con `celdaX/Y` a `main.go` (ahora dependen de las palancas vistaX/Y).
	// La nota de arriba decia que el `.ds1` del pueblo no trae monstruos: MEDIDO en esta tarjeta, es FALSO --
	// townN1 trae 15 Objects de tipo 1 (monstruos: warriv1, charsi, kashya, rogue1, chicken, cow) y townE1 18.

	// patrullaDx / patrullaDy / patrullaLargo: el eje del paseo y cuanto mide, en celdas. El eje (1,-1) es el que
	// mas ancho se ve en la proyeccion iso (160 px por celda, en horizontal), asi que es el que deja el paseo
	// DENTRO del lienzo de 640x480 con la camara quieta; y va cruzado al camino del jugador, para que el bicho se
	// le ponga delante y la colision de celda se vea. El largo son 2 celdas de mapa (2,83 de recorrido, 226 px
	// de pantalla): es el paseo mas largo que entra en el lienzo de 640x480 con la camara quieta en la celda del
	// POC (medido: con el eje (1,0) y 4 celdas el bicho se salia por la esquina -- ancla 614,510 al cuadro 3 --,
	// y con 1 celda el paso de un cuadro lo cruzaba entero y el paseo se veia como un salto). Con
	// PATRULLA="<dx> <dy> <largo>" se cambia desde el build.
	patrullaDx    = 1.0
	patrullaDy    = -1.0
	patrullaLargo = 2.0

	// separacionEntreBichos: la distancia entre carriles, en celdas y sobre la PERPENDICULAR del eje del paseo
	// (o sea (+0,5,+0,5) por bicho, que en la pantalla son 40 px de alto), para que N bichos no patrullen uno
	// arriba del otro y los cuatro entren en el lienzo.
	separacionEntreBichos = 0.5

	// direccionesCOFBicho: las direcciones del COF del monstruo. Medido con `sonda-monstruo-250`: los COF de
	// los monstruos traen 8 (`dirs=8`), no las 16 del heroe. Es lo que se le pasa a `d2cof.Dir64ToCof`.
	direccionesCOFBicho = 8

	// ============================== LAS DE ESTA TARJETA (DC-002.122): EL GOLPE ==============================
	//
	// combateKnob: "si" = el bicho tiene vida y el boton Y le pega; "no" = la corrida es la de DC-002.121 bit a
	// bit (el bicho patrulla y no hay vida ni golpe). Es el A/B de la tarjeta.
	combateKnob = "si"

	// combateCercaKnob (DC-002.141): "si" = el boton Y le pega al bicho VIVO MAS CERCANO de cada jugador y los
	// relojes del modo (GH/DT/DD) y de la muerte corren para TODOS los bichos. Es lo que hace falta para matar el
	// segundo bicho de la demo (`MONSTRUOS=2`): con "no" (el defecto) el bicho de pelea es SIEMPRE el primero de la
	// corrida (`bichoDePelea`), que es la corrida de DC-002.126 bit a bit -- y con el primer bicho muerto no arranca
	// ni un golpe mas.
	combateCercaKnob = "no"

	// alcanceGolpe: la distancia de borde, EN LAS UNIDADES DE LA TABLA DEL EXE, a la que llega el golpe del heroe
	// desarmado. El 7 es `meleeInRange`, el valor VERIFICADO del exe (`d2core/d2monsters/director.go`); con la
	// tabla de `UNIT_GetTableDistance` (`d2common/d2skill`, que es la que se usa en `alcance_122.go`) eso deja
	// pegarle al bicho de la celda de al lado (4-5) y NO al de dos celdas (18). La palanca esta para el A/B: con
	// ALCANCE=2 ningun golpe llega (es el gate estricto del exe sin el paseo fino, ver `alcance_122.go`).
	alcanceGolpe = 7

	// dirsAtaque: cuantas de las direcciones de la cruceta tienen compuesto el A1 del heroe. Cada direccion son
	// 13 cuadros (medido: 520 ms el ciclo y 58..62 KB de indices) y son lo que mas RAM pide de esta tarjeta: es la
	// palanca del A/B de memoria, igual que `dirsCaminando` en DC-002.113.
	//
	// MEDIDO (2026-11-02): con las 8 direcciones (499.525 B de indices) el POC NO ENTRA: despues de componerlas el
	// semiespacio del recolector queda al 96 % de su imagen viva (1.768.880 de 1.835.008 B) y el paso siguiente
	// (las casillas del DS1) muere con `GC to-space overflow`. Con 2 direcciones entra con el semiespacio como
	// estaba en DC-002.121.
	dirsAtaque = 2

	// dirsAtaqueLista: CUALES direcciones del COF del heroe se componen, separadas por comas. Manda sobre
	// `dirsAtaque`. Los numeros salen de la cadena del motor, medidos con `sonda-dir-122` (host):
	//   E = 14, W = 6, N = 10, S = 2, NE = 12, SE = 0, SW = 4, NW = 8
	// Por omision la 14 (el jugador 1 arranca al oeste del bicho y le pega mirando al este: `CELDA_P1="0 1"`).
	dirsAtaqueLista = "14"

	// dirsModoBicho: en cuantas direcciones de la patrulla se componen GH (recibiendo), DT (muriendo) y DD
	// (muerto). 1 = la de ida (entra: 86 % del semiespacio); 2 = las dos (NO entra: 98 % y el recolector se
	// atasca -- medido). La palanca queda para el A/B de memoria, como las otras dos de esta tarjeta.
	dirsModoBicho = 1

	// vidaMultiplicada: EL INSTRUMENTO de la corrida de los cuatro. El Returned de la tabla aguanta 2..3 HP, o sea
	// uno o dos golpes, asi que con un solo bicho los cuatro jugadores no llegan a pegarle. Con VIDA_X=2 la vida
	// (y su maximo) se multiplica y el log lo dice (`COMBATE VIDA INSTRUMENTO deLaTabla= 3 factor= 2 HP= 6`): la
	// vida de la tabla no se esconde. Con 1 la corrida es la de la tabla.
	vidaMultiplicada = 1

	// golpeAuto: "si" = EL INSTRUMENTO. El emulador corre sin ventana y sin teclado, asi que el boton Y no se
	// aprieta nunca; con esto el POC simula el FLANCO del boton (apretado 0,9 s de cada 2,4 s) y el resto del
	// camino es el real (el mismo flanco, el mismo A1, la misma regla del dano a la mitad y el mismo alcance).
	// "no" = el golpe solo sale si el boton Y se aprieta de verdad en el control.
	golpeAuto = "si"

	// ============================== LAS DE ESTA TARJETA (DC-002.123): EL DROP Y LA RECOGIDA ==============================
	//
	// dropKnob: "si" = al morir el bicho se resuelve su drop con la tabla (monstats -> TreasureClassEx -> weapons/
	// armor/misc) y su DC6 del MPQ queda listo para dibujarse en el piso. "no" = la corrida es la de DC-002.122
	// bit a bit (sin tablas de items, sin DC6 y sin suelo): es el A/B de la tarjeta y el que dice cuanto suman.
	dropKnob = "si"

	// semillaDrop: la semilla del dado del drop (el `d2rand` del motor, el mismo generador de la vida y del dano).
	// Fija para que la corrida sea REPRODUCIBLE: con otra semilla la cadena de TreasureClassEx tira otro objeto (o
	// ninguno), y la evidencia tiene que decir de que tirada salio el objeto que se ve.
	semillaDrop = 8

	// recogerAuto: "si" = EL INSTRUMENTO de la recogida. El emulador de estas corridas no tiene teclado, asi que la
	// cruceta no se aprieta nunca y el jugador no caminaria nunca hasta el objeto. Con esto encendido el POC le
	// empuja al jugador 1 los BITS de la direccion hacia el objeto que esta en el piso (el camino es el real: los
	// bits entran por el maple, el servicio los traduce, `Paso` mueve al jugador con la velocidad del exe y la
	// recogida la decide la regla del motor). "no" = el jugador solo camina con la cruceta de verdad.
	recogerAuto = "si"

	// ============================== LAS DE ESTA TARJETA (DC-002.124): EL INVENTARIO Y EL ALIJO ==============================
	//
	// inventarioKnob: "si" = el objeto recogido entra a la grilla del jugador (10x4 = las celdas que dice
	// `d2inventory.Dims(ContainerInventory)`), la grilla se dibuja y el objeto se ve en SU celda. "no" = la corrida
	// es la de DC-002.123 bit a bit (el objeto se recoge y no va a ninguna parte): es el A/B del costo.
	inventarioKnob = "si"

	// alijoKnob: "si" = ademas del inventario hay ALIJO del pueblo (6x8 = `d2inventory.Dims(ContainerStash, true)`),
	// con el guardar y el devolver. "no" = la corrida mide solo la grilla del jugador.
	alijoKnob = "si"

	// inventarioAuto: EL INSTRUMENTO. El emulador de estas corridas no tiene teclado ni puntero, asi que el panel
	// no se abre con un boton: el POC lo abre solo en el cuadro que dice `cuadroAbreElInventario` (el mismo
	// instrumento que `golpeAuto` y `recogerAuto`, y por el mismo motivo). Lo que NO es instrumento es el camino:
	// el objeto lo pone en la grilla la regla del motor (`d2inventory`), el panel se pinta con los pixeles del
	// `invfile` de su fila y el cuadro del pueblo se devuelve al cerrarse.
	inventarioAuto = "si"

	// cuadroAbreElInventario: en que cuadro del guion se abre el panel del jugador. 8 = DESPUES de la recogida de
	// DC-002.123 (medido: el bicho muere en el cuadro 6, el objeto cae en el 7 y el jugador lo recoge cuando el
	// instrumento de la cruceta lo lleva hasta el).
	//
	// DC-002.138: paso a VAR (`var` al final del archivo) porque el MVP JUGABLE no puede tener un cuadro fijo: lo
	// arma su guion, cuando la etapa anterior ya cerro. El valor de aca es el de la corrida de DC-002.124.

	// alijoAuto: EL INSTRUMENTO del alijo (mismo motivo). Abre el cofre en el cuadro `cuadroAbreElAlijo`, guarda el
	// objeto del inventario, lo devuelve al inventario en el cuadro siguiente y cierra.
	alijoAuto = "si"

	// cuadroAbreElAlijo: el alijo abre DESPUES de que el inventario mostro el objeto en su celda. DC-002.138: VAR
	// (ver el final del archivo), por el mismo motivo que `cuadroAbreElInventario`.

	// cerrarLosPaneles: "si" = el panel del jugador se cierra tres cuadros despues de abrirse y el alijo dos
	// despues del devolver (el A/B de "el cuadro del pueblo intacto con el panel cerrado DESPUES de haberlo
	// abierto"). "no" = los paneles quedan abiertos hasta el final de la corrida, que es como se ve en la captura:
	// medido en esta tarjeta, los cuadros con el panel son ~1 s y la captura del script es cada 2 s, asi que un
	// panel que se cierra deja la corrida sin ninguna captura con el inventario a la vista.
	cerrarLosPaneles = "si"

	// radioDelAlijo: a que distancia (en celdas) el jugador "toca" el alijo. En la PC el cofre se abre con el
	// PUNTERO (un clic sobre el objeto); en la consola no hay puntero, asi que el POC usa PROXIMIDAD. 2,0 celdas es
	// la distancia medida entre la celda del alijo y la celda donde queda el jugador despues de recoger el objeto
	// (1,41 celdas = la diagonal): con 0,9 (el `pickupRange` de los objetos del piso) el alijo no abriria nunca en
	// esta corrida y con 2,0 abre con el jugador parado al lado.
	radioDelAlijo = 2.0

	// ============================== LAS DE ESTA TARJETA (DC-002.126): LA MUERTE Y EL CADAVER ==============================
	//
	// muertePorCuadro: EL INSTRUMENTO del reloj de la muerte. "si" = el DT avanza UN cuadro de animacion por cuadro
	// del POC. Motivo medido: el cuadro del POC tarda ~1,3 s (el mapa se dibuja dos veces, 1,9 s medidos en el
	// split) y el DT del Returned son 19 cuadros a 49 ms = 935 ms, asi que con el reloj de pared de DC-002.122 la
	// caida se muestrea UNA vez -- el bicho pasa de parado a cadaver sin que se vea caer. Con "si" se dibujan los 19
	// cuadros uno por uno (uno por cuadro del POC) y se cuentan. Lo que NO cambia: el ciclo, sus 19 cuadros, su caja,
	// sus pixeles y su crc32 salen del animdata y del COF del motor; lo unico que cambia es cada cuanto se pide el
	// cuadro siguiente. "no" = el reloj de pared del motor (la corrida de DC-002.122, el A/B).
	muertePorCuadro = "si"

	// cadaverKnob: "si" = el cadaver (el DD) se queda lo que lo deja el MOTOR: 12,0 s (`corpseSeconds` de
	// `d2core/d2monsters/director.go`) y despues desaparece del cuadro. "no" = el cadaver no expira nunca (el A/B
	// de "desaparece del cuadro al final").
	cadaverKnob = "si"

	// ============================== LAS DE ESTA TARJETA (DC-002.125): EL HUD ==============================
	//
	// hudKnob: "si" = el HUD se dibuja encima del pueblo (globos de vida y mana con los valores reales, cinturon
	// de 4 pociones y el nombre de la zona). "no" = la corrida es la de DC-002.126 bit a bit, sin HUD: es el A/B
	// del costo (pixeles pintados, ms por cuadro y RAM).
	hudKnob = "si"

	// hudInstrumento: EL INSTRUMENTO de las barras. El POC no tiene dano al heroe todavia, asi que sin esto la
	// vida y el mana se quedarian en el maximo y no habria forma de ver que el relleno del globo sigue al VALOR
	// (que es lo que la tarjeta pide medir). Con "si" la vida y el mana BAJAN un punto por cuadro hasta 1; los
	// maximos no se tocan: son los de `charstats.txt` por el camino del motor. "no" = las barras quedan llenas.
	hudInstrumento = "si"

	// nivelesDistintos: "si" = cada jugador va con SU nivel (1, 2, 3, 4) y su vida y su mana salen de
	// `d2statlist.Class.BaseMax` con ese nivel. Es lo que hace visible que en el split cada mitad tiene SU HUD y
	// no una copia del primero (los cuatro heroes son la misma amazona; lo que cambia es el nivel). "no" = los
	// cuatro con el nivel de `nivelDelHeroe`.
	nivelesDistintos = "si"

	// hudPiezas: "todas" | "esenciales".
	//
	//   "todas"      = las DIEZ piezas del motor: los dos globos, sus dos TAPAS (`overlap.DC6`), el PANEL
	//                  (`ctrlpnl7.DC6` cuadro 0) y el cinturon con sus cuatro pociones. Medido: 49.448 B vivos.
	//   "esenciales" = las SEIS que la tarjeta pide: los dos globos, el cinturon, las cuatro pociones y la fuente.
	//                  Medido: 22.848 B vivos.
	//
	// POR QUE ES UNA PALANCA Y NO UNA ELECCION DE GUSTO. Con las diez piezas el semiespacio del POC queda al 93 %
	// en la fase ENTRAR y la fase TRANSICION se queda sin monton de C ("gc_external_alloc: OOM", arena 10216192,
	// libre 1248920, pidio 1007616). Con las seis la misma corrida cierra. El numero manda: las tapas son 14.432 B
	// para 807 pixeles (el brillo del vidrio) y el panel 12.168 B para 7.137 (el marco de piedra): son LO MAS
	// CARO POR PIXEL de todo el HUD, y ninguna de las dos es lo que la tarjeta pide ver.
	hudPiezas = "esenciales"

	// claseDelHeroe / nivelDelHeroe: la fila de `charstats.txt` y el nivel con el que se calculan la vida y el
	// mana. La amazona es el heroe del POC desde DC-002.38 (el COF `amnuhth.cof`).
	claseDelHeroe = "Amazon"
	nivelDelHeroe = 1

	// ============================== LAS DE ESTA TARJETA (DC-002.127): LOS NPCs Y EL DIALOGO ==============================
	//
	// npcKnob: "si" = los NPCs del pueblo se dibujan en su celda y se les puede hablar (el panel de dialogo con su
	// texto). "no" = la corrida es la de DC-002.125 bit a bit, sin un solo NPC ni un pixel de panel: es el A/B del
	// costo (bytes vivos, px por cuadro, ms por cuadro).
	npcKnob = "si"

	// npcLista: que NPCs se dibujan, por la columna Id de monstats.txt. La tarjeta pide Warriv, Charsi y Kashya.
	// Los otros dos del pueblo (akara, gheed) entran con la palanca; `rogue1`/`chicken`/`cow` no son gente con la
	// que se hable (no tienen entrada en d2npc) y nunca entran.
	npcLista = "warriv1,charsi,kashya"

	// hablarAuto: EL INSTRUMENTO del boton. El emulador corre sin ventana y sin teclado, asi que el boton X no se
	// aprieta nunca y la cadena boton -> panel no se podria medir. Con "si" el POC aprieta el boton en el cuadro
	// `cuadroAbreElDialogo`, pasando por el MISMO camino (`elBotonDeHablar`): el log dice `boton= true` cuando lo
	// aprieta el instrumento y `false` cuando lo aprieta el control.
	hablarAuto = "si"

	// cuadroAbreElDialogo: en que cuadro del POC se aprieta el boton. El POC nace en la celda (28,20) y Warriv
	// esta en la (29,15) del townE1 (subtiles 146,75 del .ds1): 5 celdas al norte, asi que el jugador camina antes
	// (el guion del POC) o el POC arranca pegado a el con `CELDA_P1="29 16"`. DC-002.138: VAR (ver el final del
	// archivo), por el mismo motivo que `cuadroAbreElInventario`.

	// vistaX / vistaY: LA CELDA DEL PUEBLO DONDE ARRANCA EL JUGADOR 1 Y DONDE SE PONE LA CAMARA. Era la constante
	// (28,20) del centro del townN1 hasta esta tarjeta. La ventana del POC mide 640x480 y el renderer dibuja a 80 px
	// por celda en un eje y 40 en el otro: el rombo visible son ~8x12 celdas. Los NPCs del pueblo estan a 10-15
	// celdas del centro, asi que para verlos (y poder hablarles) hay que apuntar la camara a su celda:
	//   Warriv  (25.2, 9.0)  -> VISTA_X=25 VISTA_Y=10
	//   Charsi  (18.8, 5.4)  -> VISTA_X=19 VISTA_Y=6
	//   Kashya  (30.6, 9.2)  -> VISTA_X=31 VISTA_Y=10
	// Con los valores por omision (28,20) la corrida es exactamente la de DC-002.125.
	vistaX = 28.0
	vistaY = 20.0

	// cerrarElDialogo: "si" = el panel se cierra dos cuadros despues de abrirse. Es la segunda mitad del criterio:
	// cerrado el panel, el lienzo tiene que volver al crc32 del pueblo solo.
	cerrarElDialogo = "si"

	// ============================== LAS DE ESTA TARJETA (DC-002.128): LA DEN OF EVIL ==============================
	//
	// questKnob: "si" = la primera quest del Acto 1 corre en el POC (aceptar hablando con Akara, matar los bichos de
	// la cueva con el contador avanzando, completarla y cobrar el punto de habilidad). "no" = la corrida es la de
	// DC-002.127 bit a bit: es el A/B del costo (bytes del paquete y ms por cuadro).
	questKnob = "si"

	// EL GUION DE LA QUEST, cuadro por cuadro. El emulador de estas corridas no tiene teclado y el A1 del heroe
	// tarda 13 cuadros (520 ms) por golpe, asi que el guion pone los eventos en cuadros FIJOS y el log dice en cual
	// paso cada cosa. Los cuadros son los del POC (los que cuenta `dibujar`).
	//
	//	2  hablar con Akara (el jugador arranca en su celda con VISTA_X/Y) y aceptar: Hear(64) -> estado 2
	//	3  cerrar el panel del aceptar
	//	4  a la cueva: EvAreaChanged 1 -> 8 -> estado 3, "hay que matarlos"
	//	5  el primer bicho de la cueva (uno por cuadro, `MONSTRUOS` bichos)
	//	8  la cueva quedo limpia: Tick(8) -> la pagina 5, recompensa pendiente
	//	9  volver al pueblo: EvAreaChanged 8 -> 1
	//	10 hablar con Akara otra vez y COBRAR: Hear(76) -> estado 5, EffectSkillPoint(1)
	//	11 la prueba de la supervivencia: un juego NUEVO sobre el registro del guardado + Start
	//
	// Con MONSTRUOS=3 los bichos mueren en los cuadros 5, 6 y 7. Con otro numero, `questCuadroLimpia` tiene que
	// quedar despues del ultimo: el guion lo avisa en el log si se pisan.
	questCuadroAcepta   = 2
	questCuadroCierra   = 3
	questCuadroCueva    = 4
	questCuadroMata     = 5
	questCuadroLimpia   = 8
	questCuadroVuelve   = 9
	questCuadroCobra    = 10
	questCuadroVerifica = 11

	// questClaseBicho: la clase del bicho que el POC pone en la cueva (6 = Returned en monstats.txt). Es la que va
	// en el evento `EvMonsterKilled`, que es como el exe cuenta las bajas de la cueva.
	questClaseBicho = 6

	// ============================== LAS DE ESTA TARJETA (DC-002.130): EL MINIMAPA ==============================
	//
	// minimapaKnob: "si" = el minimapa del pueblo se dibuja en la caja del motor (arriba a la derecha) con el modelo
	// de `d2automap` portado. "no" = la corrida es la de DC-002.128 bit a bit (el A/B del costo).
	minimapaKnob = "si"

	// minimapaCuanto: "siempre" = se pinta en cada cuadro (lo que hace el juego). "cambio" = se pinta solo cuando el
	// revelado agrego celdas o el heroe se movio en la caja; los demas cuadros se salta. Es el A/B de la palanca que
	// la tarjeta tiene que decidir con el numero: si "siempre" cuesta poco del cuadro, se usa siempre.
	minimapaCuando = "siempre"

	// minimapaTodo: "si" = ademas del reveal que sigue al jugador, se revela el pueblo ENTERO en el cuadro 1. Es el
	// caso peor (las 610 celdas que caen en la caja) y el numero que pide la tarjeta: ms por cuadro de dibujar el
	// pueblo entero. "no" = solo el reveal que sigue al jugador (se ve crecer el mapa cuadro a cuadro).
	minimapaTodo = "si"

	// minimapaSiembra (DC-002.146b): que hace el revelado con un tile que NO tiene fila en `AutoMap.txt`.
	//
	//	"cercana"  le da la celda de la fila MAS CERCANA del mismo tipo (`d2automap.Table.CelCercana`: mismo
	//	           estilo con la secuencia mas proxima, o el estilo mas proximo). Es lo que llena el automapa
	//	           del afuera generado por el DRLG, cuyos pisos son `act1/town/floor.dt1` estilo 0 y 5 con
	//	           secuencia 0 y las filas del "1 Wilderness" solo cubren el estilo 0 para las secuencias 1..47.
	//	"no"       el camino del MOTOR: el tile sin fila no tiene celda (es el A/B de la siembra, y el numero
	//	           que dice cuanto llena).
	minimapaSiembra = "cercana"

	// ============================== LAS DE ESTA TARJETA (DC-002.137): EL MINIMAPA POR DENTRO ==============================
	//
	// minimapaRapido: TRES caminos, y el A/B es una corrida (los pixeles son los mismos en los tres).
	//	"no"    el de DC-002.130: una tabla de 256 palabras por celda, dentro de `RenderIndicesTintado`, y un
	//	        `PushTranslation`/`Pop` por celda (610 tablas por cuadro, 156.160 entradas)
	//	"tabla" la tabla armada UNA vez por cuadro (`pvr.TablaDelBlit`, del POC) y los indices leidos de a cuatro
	//	"si"    ademas la lista COMPACTA de los pixeles que pintan (tercetos x,y,indice armados al cargar el arte):
	//	        ~8 vueltas por celda en vez de los 128 indices del tile
	minimapaRapido = "si"

	// minimapaDiag: las corridas de MEDIR del minimapa, por dentro (parten el costo entre la caja, las celdas y
	// la tabla). "no" = el camino del juego.
	//
	//	"caja"     pinta la caja negra y NADA mas: mide la caja sola (ROMPE el dibujo a proposito: es de medir)
	//	"celdas"   pinta las celdas y el marcador SIN la caja negra (ROMPE el dibujo: mide las celdas solas)
	//	"sinarte"  el bucle de las celdas SIN la busqueda del cuadro en el mapa (dibuja otro arte: ROMPE el dibujo)
	//	"sinblit"  el bucle entero (busqueda, proyeccion, recorte) SIN pegar la celda (ROMPE el dibujo)
	//	"tabla610" pinta el cuadro IDENTICO y ademas arma la tabla 610 veces mas: es lo que costaba la tabla por
	//	           celda del camino viejo, medido sin cambiar un pixel (es el control del instrumento)
	//	"flujo"    DC-002.146, el A/B del BLINDAJE DEL REARME: relee `AutoMap.txt` por el flujo del MPQ (el camino
	//	           viejo, el que pedia memoria por sector) y NO frena el recolector. Es la corrida que reproduce el
	//	           `slice bounds out of range` intermitente de DC-002.145 y la que mide lo que cuesta el camino
	//	           viejo (recolecciones adentro del rearme). El dibujo es el mismo.
	//	"panico"   DC-002.146, la PRUEBA EN CONTRA del blindaje: el rearme tira un panic de rebanada DE VERDAD en
	//	           el paso de la relectura de la tabla. El cruce tiene que cerrar igual y el log tiene que decir que
	//	           el minimapa se quedo en el nivel viejo. El dibujo no cambia (el rearme no se hace).
	//	"sololectura" DC-002.146, la lectura NUEVA sin el freno del recolector: mide cuanto pide la lectura sola
	//	           (tiene que dar 0 recolecciones adentro del rearme, contra las 4 del camino viejo).
	//	"carga"    DC-002.146, LA CARGA: doce relecturas de la tabla por el camino viejo (sin freno) antes de la de
	//	           verdad, para reproducir A VOLUNTAD el `slice bounds out of range` de DC-002.145 (~48 ventanas del
	//	           defecto en una corrida). El dibujo es el mismo.
	minimapaDiag = "no"

	// ============================== LAS DE ESTA TARJETA (DC-002.131): LO QUE NO ENTRA EN EL SPLIT ==============================
	//
	// medirVivo: "si" = el instrumento de esta tarjeta. En cada hito de la carga se fuerza una recoleccion
	// (`runtime.GC()`) y se informa lo que QUEDO VIVO, que es el numero con el que se decide si el semiespacio
	// alcanza (ver medidor_131.go). "no" = el instrumento se apaga ENTERO, recolecciones incluidas: es el A/B que
	// separa "lo vivo bajo" de "el instrumento recolecta basura y de paso salva la corrida".
	//
	// OJO, Y ES LA TRAMPA DE ESTA TARJETA: una recoleccion a pedido en el hito cambia el estado del monton cuando
	// empieza el bucle de cuadros. Con "si" el pico baja aunque no se toque nada mas. Por eso la prueba del arreglo
	// de verdad se corre con "no".
	medirVivo = "si"

	// arteEnMontonDeC: "si" = los cuadros YA COMPUESTOS del jugador y del bicho se copian al monton de C (fuera del
	// semiespacio del recolector), como los del HUD desde DC-002.125 y los de los NPCs desde DC-002.127. Es el
	// arreglo de esta tarjeta: 216.000 B de arte que dejan de ser 216.000 B de lo VIVO que el recolector tiene que
	// copiar. "no" = el A/B: el mismo binario con el arte en el semiespacio, que es lo que moria.
	arteEnMontonDeC = "si"

	// cuadroDesdeElBorde / cuadroHastaElBorde: la ventana de cuadros del instrumento fino (`vivoDeUnPaso`). El pico
	// de esta tarjeta tiene cuadro -- el split con el HUD muere en el 4 con la 125 y hoy muere mas tarde -- y una
	// recoleccion por paso en una corrida entera costaria el cuadro entero. Con la ventana vacia (desde > hasta)
	// el instrumento no cuesta nada.
	cuadroDesdeElBorde = 12
	cuadroHastaElBorde = 16

	// gcPorCuadro: "si" = el split hace UNA recoleccion en el borde del cuadro (antes de los lienzos) cuando el
	// semiespacio esta apretado (ver gc_del_cuadro_131.go). Queda en "no": corre y salva la corrida, pero cuesta
	// 86-92 ms por cuadro (el cuadro del mapa sube de 701 a 1.136 ms) y desde el cuadro 5 el ancho del texto de la
	// zona sale corrupto (-2147483507) -- el A/B esta en la evidencia. El arreglo que se usa es el de arte_131.go.
	gcPorCuadro = "no"

	// umbralDelGcDeCuadro: el % de semiespacio usado a partir del cual la recoleccion del borde corre. 70 porque el
	// vivo del split ronda el 76-85 % (medido): con menos, el recolector no llega con aire al cuadro siguiente y
	// corta adentro del dibujado, que es justo lo que esta palanca evita (y con "si" lo paga caro).
	umbralDelGcDeCuadro = 70

	// ============================== LAS DE ESTA TARJETA (DC-002.132): LOS CUADROS POR SEGUNDO ==============================
	//
	// medirCuadro: "si" = el instrumento por cuadro del POC (el crc32 del lienzo 16 veces por cuadro y el conteo
	// de pixeles, ~1,2 MB de lectura cada uno). "no" = se apaga DESPUES de los tres primeros cuadros, que se
	// miden siempre porque ahi vive la comparacion del cuadro 1 contra el host (crc32 1437022778). Es la palanca
	// que separa el costo de DIBUJAR del costo de MEDIRSE: con "si" el numero de cuadros por segundo es el de un
	// POC instrumentado, con "no" es el del juego.
	medirCuadro = "si"

	// ============================== LAS DE ESTA TARJETA (DC-002.139): LOS FPS DEL MVP ==============================
	//
	// mapaEstatico: "si" = el mapa es ESTATICO y no se repinta cuando la camara esta en el mismo pixel que el
	// cuadro anterior: se devuelve el mapa debajo de la caja de cada ente (lo que guarda `cajas139`) y el cuadro
	// sigue igual. "no" = el mapa se pinta entero cada cuadro, que es el camino de DC-002.140, bit a bit.
	//
	// cajas139: "si" = dejar encendido el guardar/devolver la caja de cada ente cuando el instrumento por
	// cuadro esta apagado. El salto del mapa lo necesita (es lo que devuelve el mapa debajo de los sprites) y
	// NO toca el dibujo: el sprite se pinta en el mismo orden y en el mismo sitio; lo que se paga son las
	// copias de la caja.
	mapaEstatico = "no"

	cajas139 = "no"

	// convBloques139: "si" = la presentacion convierte SOLO los bloques de 32x32 que el cuadro toco (las cajas
	// de los sprites y los paneles) en vez de los 307.200 pixeles del lienzo. Depende de `mapaEstatico`: con el
	// mapa repintandose entero no hay nada que ahorrar (se marca todo).
	convBloques139 = "si"

	// scroll141: "si" = EL CUADRO CAMINANDO (DC-002.141). Cuando la camara se movio unos pixeles, el lienzo se
	// CORRE con una copia en el lugar (`pvr.MoverLienzo`) y el mapa se repinta SOLO en las franjas que entran
	// por el borde, en las cajas de los entes del cuadro anterior y en las bandas de los paneles -- todo con
	// la lista de recortes del backend. "no" = el camino de DC-002.139, bit a bit (caminando repinta los 248
	// tiles: 11-13 fps medidos). La cabecera de `mapa_141.go` tiene el porque y los numeros.
	scroll141 = "si"

	// comparar141: "si" = en los primeros cuadros que caminan se pinta ADEMAS el mapa entero y se comparan los
	// dos crc32 del lienzo (la prueba de que el corrimiento y las franjas dan el mismo cuadro). Cuesta un
	// repintado entero por cuadro comparado: va apagada salvo en la corrida de la prueba.
	comparar141 = "no"

	// convFilas141: "si" = la presentacion convierte el lienzo recorriendo el LIENZO en orden (lecturas
	// seguidas contra la VRAM, el twiddle se paga escribiendo al buffer de 2 KiB en RAM) en vez de recorrer la
	// textura y leer el lienzo salteado, que es lo que hacia DC-002.135. Misma cuenta de color, mismos bytes:
	// "no" vuelve al camino viejo en el mismo binario (es el A/B). La cabecera de `conversion_141.go` lo cuenta.
	convFilas141 = "si"

	// presentarGrupo142: "si" = EL CAMINO POR BLOQUES SUCIOS SUBE EN GRUPOS DE 2x2 (DC-002.142). Los cuatro
	// bloques de un grupo (64x64 px = 8.192 B) son contiguos en la textura twiddleada, asi que van en UNA copia;
	// con "no" el camino es el de DC-002.139, una copia de 2 KiB por bloque (300 viajes cuando camina), en el
	// MISMO binario. La cabecera de `presentar_142.go` tiene los numeros.
	presentarGrupo142 = "si"

	// presentar142: "si" = el informe POR CUADRO de la presentacion del cuadro que camina (`JUEGO142:
	// PRESENTAR-CAMINANDO`). La linea `PVR135: PRESENTAR` es un promedio de 30 cuadros y ahi el cuadro caro se
	// diluye entre los baratos; este informe dice cuadro por cuadro cuanto costo presentar el que camino.
	presentar142 = "si"

	// pruebaConv142: "si" = la PRUEBA de la conversion (DC-002.142): convierte 20 bloques ANTES y DESPUES del
	// corrimiento del lienzo, en el mismo cuadro, y los dos numeros salen en `JUEGO142: PRUEBA-CONV`. Existe
	// porque el mismo bucle cuesta 9 us por bloque en un cuadro quieto y 80 en un cuadro que camina, y hay que
	// saber si es el bucle o el estado de la memoria despues de la copia. Cuesta ~2 ms por cuadro: va apagada.
	pruebaConv142 = "no"

	// blitRapido: "si" = el camino nuevo de dibujado de tiles del backend PVR (DC-002.132): el recorte se calcula
	// UNA vez por tile (no una vez por pixel) y con el estado neutro (sin color puesto) no se hace la cuenta del
	// tinte (3 multiplicaciones y 3 divisiones por 255 por pixel). "no" = el bucle viejo, TAL CUAL, que es el que
	// midio 675-931 ms por cuadro de mapa. Las dos ramas viven en el mismo binario: el A/B es una corrida.
	blitRapido = "si"

	// ============================== LAS DE ESTA TARJETA (DC-002.134): LAS ETAPAS DEL CUADRO ==============================
	//
	// etapas: "si" = el cronometro por etapa del dibujado del POC (jugador, bichos, HUD, minimapa, actualizar) y
	// la linea `JUEGO134: ETAPAS` cada 30 cuadros. Cuesta dos llamadas a d2c_us por etapa: nada al lado del
	// cuadro que mide. "no" = el instrumento no suma nada (el binario es el de DC-002.132 otra vez).
	etapasKnob = "si"

	// verbosoEtapas: "si" = las lineas de evidencia POR CUADRO del POC (la de comparacion contra el host, la de
	// cada jugador, la del bicho con su detalle, la del HUD, la del minimapa). Son ~1,5 KB por cuadro por el
	// puerto serie y en la consola real no las va a leer nadie: "no" deja solo los TRES primeros cuadros (donde
	// vive el crc32 del mundo contra el host) y los resumenes del cierre. Es el A/B de cuanto del cuadro es el
	// instrumento impreso.
	verbosoEtapas = "si"

	// blitPalabras134: "si" = el backend PVR escribe el pixel con UN deposito de 32 bits y arma la paleta (con
	// su tinte) en una tabla de 256 palabras UNA vez por tile, en vez de cuatro depositos de 8 bits y tres
	// multiplicaciones mas tres divisiones por pixel (DC-002.134, d2core/d2render/pvr/blit_palabras.go). "no" =
	// el bucle de bytes de DC-002.132, tal cual: es el A/B dentro del mismo binario.
	//
	// LAS DOS REGLAS DEL CAMINO DE BYTES SE RESPETAN: el indice 0 no pinta y el alfa intermedio hace el mismo
	// source-over con el mismo cociente por 255. La prueba es el crc32 del cuadro 1 contra el host
	// (1437022778) mas el verificador `console/dc/verif-blit-134` con los tres caminos sobre 1.000 casos.
	blitPalabras134 = "si"

	// pintarUnaVez: "si" = con el instrumento POR CUADRO apagado (`medirCuadro=no` y cuadro > 3) el jugador y el
	// bicho se pintan UNA sola vez y sin el baile de guardar la caja, medir los pixeles y devolver el mapa: ese
	// baile existe para MEDIR (el crc32 del cuadro con los sprites y la prueba de que el mapa se restaura), no
	// para dibujar. El cuadro que queda en pantalla es el MISMO (mismo orden de pintado, mismos pixeles); lo que
	// se va son las copias de la caja a y desde la VRAM y el segundo pintado.
	pintarUnaVez = "si"

	// ============================== LAS DE ESTA TARJETA (DC-002.135): LA PRESENTACION ==============================
	//
	// presentarConvRapida: "si" = el bucle de conversion RGBA -> RGB565 del backend PVR es el nuevo (DC-002.135):
	// UNA carga de 32 bits por pixel en vez de tres cargas de 8 bits contra la VRAM, el color con mascaras y
	// corrimientos (sin tablas de color) y DOS pixeles por deposito de 32 bits, recorriendo la textura en orden
	// con una tabla de desplazamientos armada una vez. "no" = el bucle de DC-002.132, TAL CUAL: es el A/B dentro
	// del mismo binario.
	//
	// EL RESULTADO TIENE QUE SER EL MISMO BYTE A BYTE: la cuenta del color (5-6-5 truncado) y el orden del
	// twiddle no cambian. La prueba es el crc32 del cuadro 1 contra el host (1437022778) y la captura del pueblo
	// comparada pixel a pixel con la de DC-002.132.
	presentarConvRapida = "si"

	// presentarVramAgrupada: "si" = los bloques de 32x32 se suben a la VRAM en GRUPOS de 2x2 (64x64 px = 8.192 B
	// de una sola copia) porque en la textura twiddleada esos cuatro bloques son CONTIGUOS: 90 copias por cuadro
	// en vez de 300 (70 grupos de 2x2 = 280 bloques + los 20 bloques sueltos de la franja de abajo, que con
	// 480 = 7x64 + 32 no tienen par vertical ni horizontal que agrupar). "no" = una copia de 2.048 B por bloque,
	// como siempre. Se puede probar por separado de la conversion: son dos cuellos distintos (la CPU y las Store
	// Queues).
	presentarVramAgrupada = "si"

	// ============================== LAS DE ESTA TARJETA (DC-002.136): EL ARRANQUE EN LOS 16 MB ==============================
	//
	// medirCarga: "si" = la TABLA DE CONSUMO DEL ARRANQUE (instrumento_136.go). Cada hito de la carga (los mismos de
	// siempre: `memoria()`) deja una fila con el monton de C tomado, lo que queda libre, el delta contra el hito
	// anterior y el semiespacio del recolector; el cierre imprime el pico con nombre, el margen mas bajo y las ocho
	// etapas que mas pidieron. No reserva memoria de mas y no fuerza recolecciones: mide el arranque tal cual.
	//
	// Y en el shim de C (`csrc/shim.c`, seccion 12) el otro instrumento de la tarjeta: `malloc` interpuesto, que
	// imprime cada pedido de 64 KB o mas con el monton que habia antes (`ARQ-MARCA ext-bloque`). Es el que dice
	// QUIEN pide los bloques grandes; la tabla de aca dice CUANDO. Con los dos, el OOM deja de ser a ciegas.
	//
	// "no" = el binario es el de DC-002.135 otra vez (es el A/B del costo: la tabla son unas decenas de lineas por el
	// puerto serie y nada mas).
	medirCarga = "si"

	// ============================== LAS DE ESTA TARJETA (DC-002.135, EL MAPA): EL TILE POR DENTRO ==============================
	//
	// tileCron: "si" = el cronometro por bloque DENTRO del renderer del mapa (`cronometro_135.go`): por cada tile
	// se abren y cierran cuatro marcas (cache, vista, estado, pintado) y por cada pasada del renderer una mas, y
	// cada 30 cuadros sale la linea `JUEGO135M: MAPA-POR-TILE` con el promedio por cuadro. Es el instrumento que
	// contesta la pregunta de la tarjeta: de los 382 ms del mapa, cuanto se va FUERA del bucle de pixeles y en
	// cual de los cuatro bloques. "no" = el reloj no se enchufa y el motor no cuenta nada (el binario es el de
	// DC-002.134 otra vez).
	//
	// OJO CON LA LECTURA: el instrumento cuesta ocho llamadas al reloj por tile (~2 ms por cuadro de 242 tiles,
	// medido al arrancar con `costo_de_1000_llamadas_us`), y la linea trae descontado ese costo en `instrumento=`.
	// Sin descontarlo, el bloque del pintado se queda con el 0,5 % del instrumento encima.
	tileCron = "no"

	// bucle135: la corrida de DIAGNOSTICO del bucle de pixeles del mapa (`diagnostico_135.go`), para partir el
	// costo por pixel en sus tres pedazos. Con "no" corre el camino del juego.
	//
	//   "lectura"   -- lee el indice y la tabla y NO escribe el pixel  (lo que cuesta leer)
	//   "escritura" -- NO lee el indice: escribe una palabra fija en los mismos destinos  (lo que cuesta escribir)
	//   "tabla2"    -- el bucle normal MAS una segunda armada de la tabla de 256 palabras  (lo que cuesta la tabla)
	//
	// "lectura" y "escritura" ROMPEN el dibujo a proposito: son corridas de medir, el `crc32` del cuadro 1 no se
	// compara en ellas. "tabla2" pinta el cuadro identico (es el control: si cambia, el instrumento toca algo que
	// no debe).
	bucle135 = "no"

	// cacheRapida135: "si" = con la imagen del tile YA armada, el renderer no le pide los datos del `.dt1` al
	// motor (`mapEngine.GetTiles`) para nada: el `YAdjust` de paredes, sombras y techos va guardado con la
	// imagen y se repone tal cual, y los pisos lo traen del `.ds1`. Medido en esta tarjeta: ese pedido, que se
	// pagaba en CADA tile y CADA cuadro acertara o no la cache, era 185 de los 382 ms del mapa.
	//
	// "no" = el camino de DC-002.134 tal cual. La prueba es doble: el `crc32` del cuadro 1 contra el host
	// (1437022778) con las dos, y la captura del pueblo comparada pixel a pixel.
	cacheRapida135 = "si"

	// plano135: "si" = el camino PLANO del blit de indices (estado neutro y paleta opaca): una rama por pixel
	// en vez de tres y cuatro pixeles por palabra leida de los indices. Es el caso del mapa del nivel y escribe
	// los MISMOS bytes que el camino general. "no" = el bucle general de DC-002.134 tal cual (el A/B, con la
	// prueba de que el cuadro no cambia: mismo `crc32` 1437022778).
	plano135 = "si"
)

// ============================== LAS PALANCAS QUE SON `var` (DC-002.138, EL MVP JUGABLE) ==============================
//
// POR QUE DEJARON DE SER `const`. Las tres las lee SU tarjeta en el cuadro `n`: `pasoDelInventario` abre el panel
// cuando `cuadro+1 == cuadroAbreElInventario`, `pasoDelAlijo` cuando `cuadro+1 == cuadroAbreElAlijo` y
// `pasoDelNPC` cuando `cuadro+1 == cuadroAbreElDialogo`. Eso funciona cuando la corrida ES de esa tarjeta, porque
// el cuadro se sabe de antemano. El MVP JUGABLE encadena nueve acciones y no puede saberlo: el panel del
// inventario se abre CUANDO EL JUGADOR YA RECOGIO EL OBJETO, y eso depende de cuantos golpes aguanto el bicho y de
// cuanto tardo en caer (19 cuadros de DT). Asi que el guion del MVP las ARMA un cuadro antes de usarlas
// (`mvp_138.go`) y con eso el camino de la tarjeta original no cambia una linea: sin MVP valen lo que dice aca,
// que es lo que valian cuando eran `const`.
var (
	// el panel del jugador, en el cuadro que el guion del MVP arma (8 en la corrida de DC-002.124)
	cuadroAbreElInventario = 8

	// el cofre del alijo, un cuadro despues de que el inventario mostro el objeto (10 en la de DC-002.124)
	cuadroAbreElAlijo = 10

	// el boton de hablar con el NPC mas cercano (2 en la de DC-002.127)
	cuadroAbreElDialogo = 2

	// mvpKnob: "si" = corre EL GUION DEL MVP JUGABLE (`mvp_138.go`): las nueve acciones de una partida, en UNA
	// corrida y en el orden del juego, con su linea de log y su crc32 por etapa. "no" = la corrida de la tarjeta
	// que se pida con las otras palancas (nada de esto existe para ella: el guion del MVP, cuando esta apagado, no
	// toca una sola palanca ni un solo cuadro).
	mvpKnob = "no"

	// demoKnob (DC-002.140): "si" = LA DEMO JUGABLE (`demo_140.go`). El POC deja de cortarse en `cuadros+3` (la
	// demo corre hasta que se sale del emulador), la quest avanza con lo que el JUGADOR hace (hablar con Akara con
	// el boton X, matar los bichos con Y, volver a Akara a cobrar) en vez de por numero de cuadro, y cada 30
	// cuadros se imprime el ritmo real de la corrida (`DEMO140: FPS`). Con "no" (el defecto) este archivo no hace
	// NADA y la corrida es la de la tarjeta que se pida, bit a bit.
	demoKnob = "no"

	// demoMoor (DC-002.140): "si" = cruzar el borde del campamento compone el nivel de AFUERA como nivel nuevo (el
	// orden de DC-002.109: soltar la cache y las casillas del que se deja ANTES de componer el que sigue). Va
	// aparte de `demoKnob` porque el nivel de afuera pide RAM y ese numero se mide antes de dejarlo encendido.
	//
	// DC-002.144: SE MIDIO Y NO ENTRA, asi que queda en "no" para siempre hasta que haya otra palanca de memoria.
	// Corrido en Flycast: con las tablas y las fuentes vivas (sin ellas el `.ds1` del nivel nuevo no se puede ni
	// leer) el cruce llega a componer y se muere en la arena de bloques de los `.dt1` del afuera -- pide
	// 4.368.642 B contra 2.995.872 B libres (`fatal error: gc_external_alloc: OOM`). El semiespacio no es la
	// pared (68,4 %). Los numeros y el porque, en `README-140.md` y en
	// `console/dc/evidencia/dc-002.144-el-blood-moor.txt`.
	demoMoor = "no"

	// moorScratchPrimero (DC-002.144b): EL ORDEN DE LOS DOS PEDIDOS GRANDES AL CRUZAR. "si" = el buffer de
	// lectura del `.dt1` MAS GRANDE del nivel de afuera se pide ANTES de los tramos del arena de bloques
	// (`scratchPorArchivo = false`, el camino de DC-002.101 que DC-002.111 dejo como A/B); "no" = el camino de
	// DC-002.111 (un buffer por archivo, del tamano de ESE archivo, devuelto en cuanto sus bloques estan
	// copiados).
	//
	// POR QUE ES UNA PALANCA Y NO UN DEFECTO. DC-002.144 midio el muro del cruce: 2.995.872 B libres y
	// `Out of memory. Requested sbrk_base ... diff 1007616` cuando el nivel de afuera fue a pedir el buffer de
	// 1.008.003 B. Lo libre estaba en trozos, y los trozos los armo el propio cruce: los tramos del arena
	// (2 x 524.288) y los buffers chicos (318.414, 505.504, 439.569) entraron DENTRO del hueco fresco que dejo
	// el arena de bloques del pueblo (2.621.440 B, cinco tramos devueltos juntos). Pidiendo el buffer grande
	// PRIMERO, cae en ese hueco entero y los tramos crecen detras. Solo aplica con `demoMoor = "si"`: el camino
	// del pueblo (la demo de DC-002.141) no cambia ni un byte.
	moorScratchPrimero = "si"

	// demoMoorDRLG (DC-002.145): QUE ENTRA AL CRUZAR. "si" = el afuera es una REGION del Blood Moor DE VERDAD
	// (el nivel 2, 56 x 96 en total), compuesta con el plan que el DRLG calculo en el host
	// (`moor_region_145.go`) y escrita casilla por casilla con `SetExactTiles`; "no" (el defecto) = el camino de
	// DC-002.140b/.144b, la pieza de `.ds1` de 17 x 17 (`Act1/Outdoors/Wild1.ds1`).
	//
	// POR QUE EL PLAN VIENE DEL HOST Y NO SE GENERA EN LA CONSOLA. Medido: el nivel entero pide 3.047.626 B de
	// arena de bloques + 1.141.544 B de buffer contra 2.994.912 B libres (no entra: faltan 1,2 MB), y la region
	// de 32 x 32 que quedo pide 1.534.112 B + 1.051.532 B = 2.585.644 B, o sea entra con 409 KB de sobra.
	// Correr el DRLG adentro de la SH-4 pediria mas RAM que el nivel (tablas, patrones de `.ds1` y los registros
	// de `BuildTiles`), asi que la consola COMPONE lo que el host GENERO. Solo actua con `demoMoor = "si"`.
	demoMoorDRLG = "no"

	// demoMoorStream (DC-002.147): EL MUNDO CAMBIA DE REGION SIN RECOMPILAR. Con "si", el afuera son las SEIS
	// regiones que cubren el nivel (`moor_regiones_147.go`, generadas por `moor-drlg -plan-regiones`), la demo
	// arranca en la de DC-002.145 (32 x 32 desde 24,32) y cada cuadro mira si el jugador se salio de esa caja: si
	// se salio, compone la region que contiene su celda del nivel, con lo del anterior soltado antes. Con "no"
	// (el defecto) el camino es el de DC-002.145: una sola region, sin borde, y el binario no cambia de conducta.
	// Solo actua con `demoMoor = "si"` y `demoMoorDRLG = "si"`.
	demoMoorStream = "no"
)
