package main

// DC-002.103 -- LOS TRES PEDAZOS DEL CUADRO Y EN QUE ORDEN SE PIDEN.
//
// POR QUE UNA PALANCA Y NO UNA CORRIDA. El criterio de esta tarjeta es "el pueblo en pantalla" juntando tres
// cosas que YA estan medidas cada una por su lado: el camino de dibujo (`d2maprenderer` + PVR, 640x480), la
// cache de tiles en INDICES (2.723.840 B en vez de 10.895.360 B) y el arena de bloques que se suelta
// (3.629.443 B). Lo que nadie midio es SI SE PUEDEN JUNTAR, y el orden decide: los `EncodedData` de cada tile
// apuntan al arena de bloques, asi que soltarlo antes de armar la cache es leer memoria devuelta
// (`d2asset/arena_soltar_101.go`), pero soltarlo despues puede ser tarde -- despues de componer quedan
// 65.736 B y el renderer pide 442.368 B para crearse. Los dos ordenes son plausibles y hay que medirlos.
//
//	contrato  componer -> renderer -> cuadro 1 (arma la cache) -> soltar el arena -> cuadros 2 y 3.
//	          Es el orden que RESPETA el contrato de DC-002.101. Se espera que muera al crear el renderer.
//	prestamo  componer -> soltar SOLO el buffer prestado de lectura (1.008.003 B, muerto en cuanto el nivel
//	          termino de cargar) -> renderer -> cuadro 1 -> soltar el arena -> cuadros 2 y 3.
//	          Es el primer pedazo que se puede devolver sin invalidar nada. Mide la SEGUNDA pared: la cache
//	          de tiles en el semiespacio del recolector.
//	tarjeta   componer -> soltar TODO el arena (3.629.443 B) -> renderer -> cuadro 1 -> cuadros 2 y 3.
//	          Es el orden que pide la tarjeta. Va EN CONTRA del contrato (los `EncodedData` apuntan a memoria
//	          devuelta): se corre para medir que pasa de verdad, no para quedarse con ese camino.
//
// LO QUE HACE FALTA PARA LEER EL LOG. Cada paso imprime `MAPA103: MEMORIA` con usado/libre/arena/sbrk/techo y
// el pico, cada cuadro imprime su crc32 (el del host es 1437022778) y su ms, y el censo de la cache dice
// cuantas imagenes se armaron. Donde se corta el log ES la respuesta.
const (
	// orden: "contrato" | "prestamo" | "tarjeta" (ver la cabecera). Lo escribe build-mapa-103.sh desde aca;
	// cualquier corrida puede pisarlo por entorno ORDEN=... y el build lo avisa.
	//
	// La referencia del POC es "prestamo": es el orden LEGAL que llega mas lejos (el renderer entra y el
	// cuadro arranca; muere armando la cache). "tarjeta" existe para medir el orden que pide la tarjeta, y va
	// contra el contrato de DC-002.101. Ver la tabla de los tres en console/dc/evidencia/dc-002.103-*.md.
	orden = "prestamo"

	// cuadros: cuantos se dibujan antes de dejar de imprimir (el ciclo del PVR no vuelve nunca).
	cuadros = 3

	// modoTablas: "stream" (el camino de hoy) o "fuente" (el tramo de la fuente con dueno, DC-002.102).
	modoTablas = "fuente"

	// soltarLecturas: devolver el tramo de lecturas de la fuente antes de componer y despues de componer.
	soltarLecturas = true

	// tramoDeLaCache (DC-002.104): los pixeles de la cache en indices se piden a UN tramo de este tamano en vez
	// de un `make` por imagen. En libgodc, un `make` de mas de 64 KB se va al monton de C
	// (`gc_external_alloc`) y el recolector no lo copia; las 82 imagenes chicas de la cache (2.447.360 B) hoy
	// van al SEMIESPACIO y mueren ahi (DC-002.103). El numero es el censo medido en el host con
	// `console/dc/medidas/render-294 -perezoso -indices`: 86 imagenes, 2.723.840 B, 0 derrames.
	//
	// 0 = apagado (cada imagen con su `make`, el camino de la .103 y el de la PC). El build imprime con cual se
	// compilo el binario.
	tramoDeLaCache = 2723840

	// tramoEnPedazos (DC-002.106): el MISMO tramo de arriba pero pedido EN BLOQUES de este tamano en vez de un
	// solo golpe. 0 = de un solo golpe (el camino de DC-002.104).
	//
	// POR QUE. Medido en DC-002.105: con las tablas y las fuentes soltadas el monton tiene 2.849.816 B
	// disponibles (1.645.592 de libres + 1.204.224 de break room) y la cache pide 2.723.840: ALCANZA. Pero el
	// tramo de un solo golpe muere igual porque dlmalloc lo pide ENTERO por `sbrk` y el break room no llega a
	// 2,7 MB (`Requested sbrk_base 0x8d163000, was 0x8ceca000`). Pedido de a `pedazo` B, cada bloque si se
	// sirve de los huecos del monton y del top, y la pared deja de ser la CONTIGUIDAD.
	//
	// EL NUMERO. 348.160 B = 340 KB = 68 filas de 5.120 B, el grano de los tamanos de la cache (el censo por
	// imagen del host: 20.480 B las comunes, hasta 71.680 las cuatro grandes del pueblo). Con los 86 tamanos
	// de verdad y el reparto al bloque mas justo el tramo se llena en 8 bloques y deja 71.680 B de cola
	// perdida, que es el 2,6 % del censo: el pedido total queda en 2.723.840 (el techo) y lo que no entra se
	// derrama al camino de siempre (`make`), contado en el log. Medido en el host con
	// `render-294 -perezoso -indices -tramo 2723840 -pedazo 348160`: mismo crc32 1437022778 y mismo sha256 de
	// PNG que el tramo de un golpe; pedazos=8 usado=2.652.160 derrames=2 bytesDerramados=71.680.
	tramoEnPedazos = 348160

	// pedazoLoDiceElMonton (DC-002.106): el tamano de cada bloque lo decide el MONTON (la sonda `d2c_servible`
	// del shim: el mayor bloque de hasta `tramoEnPedazos` que puede servir AHORA) en vez de pedir ese tamano
	// fijo. Con la sonda, cuando el monton se queda con migas y poco break, los ultimos bloques salen mas
	// chicos en vez de morir -- que es lo que paso medido con tamano fijo (7 bloques de 340 KB puestos, y el
	// ultimo de 286.720 B murio con 409.184 B todavia disponibles pero en pedazos que dlmalloc no cose).
	pedazoLoDiceElMonton = true

	// cacheEnVram (DC-002.108): los bloques de la cache se piden a la VRAM DEL PVR en vez del monton de C
	// (`pvr.VramReservar`, que es `pvr_mem_malloc` del pool de texturas de KOS).
	//
	// POR QUE. La cache de tiles en indices son 2.723.840 B y el monton de C los aguanta solo en pedazos: con
	// las tablas y las fuentes soltadas quedan 2.849.816 B disponibles (DC-002.105) pero no contiguos, y el
	// camino en pedazos (DC-002.106) entra dejando el monton en 12.288 B de break room. En una Dreamcast de
	// verdad esos pixeles viven en la VRAM: son 8 MB de los que el motor usa ~2,3 MB (la textura del cuadro
	// 1.048.576 B + el lienzo RGBA 640x480x4 1.228.800 B), y lo que sobra lo reparte el PVR con su propio
	// dlmalloc.
	//
	// LO QUE NO CAMBIA. El dibujado sigue leyendo los indices con la CPU y expandiendolos con la paleta del
	// acto: el cuadro tiene que salir identico al byte al del host (crc32 1437022778). Lo que cambia es DONDE
	// VIVE el buffer. El costo (la VRAM se toca sin cache) se mide en los ms de cada cuadro.
	cacheEnVram = true

	// pruebaDeVram (DC-002.108): escribir un patron conocido en un bloque de la VRAM y releerlo, al arrancar el
	// video. Es el instrumento que dice si la VRAM se puede usar como RAM (escrituras que se pierden) antes de
	// culpar a la cache. No deja nada reservado: el bloque se suelta en el acto.
	pruebaDeVram = true

	// tramoAlArranque (DC-002.104): reservar el tramo de la cache ANTES de cargar nada, en vez de cuando se
	// crea el renderer. Medido en esta tarjeta:
	//   false (el de la referencia) el monton de C ya esta fragmentado al crear el renderer y el pedido de
	//                               2.723.840 B de un solo golpe muere en `gc_external_alloc`
	//                               (`Requested sbrk_base 0x8d086000, was 0x8cded000`): faltan 548.864 B de break
	//   true                         el tramo entra de una al arranque, pero entonces la COMPOSICION del pueblo
	//                               muere en el monton de C pidiendo 524.288 B con 77.824 de techo: faltan
	//                               446.464 B. Ese es el numero de DC-002.105.
	// Con las tablas y las fuentes soltadas (.105) el camino es este: reservar temprano y colgarlo despues.
	tramoAlArranque = false

	// soltarDespuesDeComponer (DC-002.105): que se devuelve al monton de C en cuanto el pueblo esta compuesto,
	// ANTES de crear el renderer y de armar la cache. Son las dos piezas que el cuadro NO usa:
	//
	//	nada     el camino de DC-002.103/.104 (la referencia): no se suelta nada y la cuenta de lo que falta
	//	         queda como la midio la .103
	//	tablas   las filas ya parseadas de las tablas del compositor (LvlTypes + LvlPrest): 324.112 B medidos,
	//	         de los cuales los que viven en el monton de C son los tableros del mapa de `LvlPrest`
	//	fuentes  las dos Source de consola (las tablas del MPQ + la memoria de trabajo de la lectura): 355.328 B
	//	         medidos al abrirlas (354.656 son de `d2data.mpq`)
	//	todo     las dos cosas: es lo que la .103 llama "las dos piezas que se pueden devolver" (679.440 B)
	//
	// OJO, y es parte del contrato: el cuadro SI lee DOS archivos del MPQ cuando el renderer arma la cache -- la
	// paleta del acto (`pal.dat`) y su tabla de sombreado (`pal.pl2`). Por eso, cuando se sueltan las fuentes,
	// las dos se PRECARGAN antes (ver `main.go`): despues de soltar, el MPQ no se puede leer.
	//
	// El valor de la referencia del POC es "todo": desde DC-002.106 el binario de referencia es el que dibuja
	// el pueblo, y para eso hay que soltar. Las corridas de .103/.104/.105 se hicieron pisandolo por entorno
	// (SOLTAR=nada para sus referencias).
	soltarDespuesDeComponer = "todo"

	// ============================== DC-002.109: LAS TRES FASES ==============================
	//
	// cicloDeFases: el ciclo ENTRAR / TRANSICION / VOLVER de una sola corrida (ver `fases_109.go`).
	//
	//	apagado     el default. La corrida es EXACTAMENTE la de DC-002.106/.108 (un nivel, tres cuadros):
	//	            es la referencia con la que se comparan los numeros, y por eso la palanca es aditiva.
	//	encendido   las tres fases. El ORDEN lo dice `ordenDeLasFases` (abajo), no esta palanca: son dos
	//	            preguntas distintas ("hay ciclo" y "en que orden") y el log tiene que poder decir cual de
	//	            las dos se movio.
	cicloDeFases = "apagado"

	// ordenDeLasFases: cual de los dos ordenes de la tarjeta usa el ciclo ("a" o "b"):
	//
	//	a  soltar la cache del nivel que se deja (el tramo, con el liberador de verdad: `pvr.VramSoltar` si los
	//	   bloques viven en la VRAM / `d2c_soltar_bloque` si salieron del monton de C) ANTES de reabrir las
	//	   fuentes y recargar las tablas.
	//	b  no soltar nada: la cache vieja sigue tomada durante la reapertura y la composicion, y el reinicio del
	//	   tramo pasa recien al colgar el nivel nuevo en el renderer.
	//
	// Se pisa por entorno con ORDEN_FASES=. Con `cicloDeFases` apagado no se mira.
	ordenDeLasFases = "a"

	// diagnosticoDeLasImagenes (DC-002.109, INSTRUMENTO): prende el censo por imagen de la cache (clave y
	// cuantas variantes devolvio el motor). Es el numero que dice si el nivel pide una variante que su `.dt1` no
	// trae. Apagado = el log de referencia no cambia.
	diagnosticoDeLasImagenes = false

	// gcPercentDelCiclo (DC-002.109): el umbral del recolector mientras corre el ciclo de las tres fases
	// (`debug.SetGCPercent` de libgodc: 100 = recolecta con el semiespacio lleno, o sea al 75 % -- ver
	// `gc_heap.c`; negativo = solo con runtime.GC()).
	//
	// POR QUE, CON EL NUMERO. Un nivel compuesto deja ~1,17 MB vivos en el semiespacio (1.835.008 B) y el
	// `.ds1` del nivel siguiente agrega ~255 KB mientras se parsea: 1,43 MB, arriba del 75 % (1.376.256). Desde
	// ahi el recolector corre en CADA asignacion sin liberar nada y la TRANSICION no avanza (medido: 1.585
	// recolecciones seguidas, cero progreso, con 3,58 MB libres en el monton de C). Con 95 el umbral pasa a
	// 1,74 MB: el parseo del `.ds1` entra sin una sola recoleccion.
	//
	// Se aplica SOLO si el ciclo esta prendido y se devuelve al valor anterior al terminar; la corrida de
	// referencia (palanca apagada) no lo toca nunca.
	gcPercentDelCiclo = 95

	// transicionPreset / transicionArchivo: el `.ds1` que compone la fase TRANSICION. El preset 1 del pueblo
	// (region 1, "Act 1 - Town") trae CUATRO archivos -- TownN1/TownE1/TownS1/TownW1, medido en el host con
	// `go run ./console/dc/medidas/presets-scratch ~/.wine-d2/drive_c/Games/'Diablo II'` -- asi que el archivo
	// 1 es un mapa DISTINTO del mismo level type: no cambia la paleta (y el crc32 del cuadro de VOLVER se
	// puede comparar contra el de ENTRAR sin que el acto meta ruido) pero si las casillas, los .dt1 usados y
	// la cache de tiles.
	transicionPreset  = 1
	transicionArchivo = 1

	// ============================== DC-002.110: EL .DS1 FUERA DEL SEMIESPACIO ==============================
	//
	// arenaDelDS1: las celdas de las capas de un `.ds1` se piden en UN bloque del monton de C (con liberador
	// explicito) en vez de una rejilla por fila en el semiespacio del recolector (ver
	// `d2common/d2fileformats/d2ds1/arena_110.go`).
	//
	// POR QUE. Medido en DC-002.109: un nivel compuesto deja ~413.000 B vivos en el semiespacio despues de una
	// recoleccion COMPLETA (el barrido de pila de libgodc es conservador y el puntero al `.ds1` de un marco ya
	// muerto cuenta como raiz), asi que VOLVER pide 1.735.048 + 382.632 contra 1.835.008 y no entra. Las
	// casillas del `.ds1` de un nivel son 2337 celdas x 20 B x 4 capas = 186.960 B en el SH-4 (224.352 B en el
	// host), y son lo que se saca del semiespacio.
	//
	//	encendido  el bloque del monton de C. MEDIDO en DC-002.110: la carga del `.ds1` pasa de +422.776 B a
	//	           +42.232 B de semiespacio (380.544 B menos), o sea que por semiespacio la vuelta ya entraria
	//	           (1.309.984 + 42.232 + 382.632 = 1.734.848 de 1.835.008) -- pero el monton de C NO tiene esos
	//	           186.960 B en el pico de componer (el arena de bloques de DC-002.101 son 4,04 MB y el techo se
	//	           toca justo: la corrida muere en la TRANSICION con `gc_external_alloc: OOM`).
	//	apagado    el camino de siempre, el de DC-002.109 (donde VOLVER muere en el semiespacio).
	//
	// POR ESO EL DEFAULT ES APAGADO: encendido es medible pero hoy NO COMPLETA LAS TRES FASES. Cuando el monton de
	// C suelte ~200.000 B antes del pico de componer (los candidatos estan en la evidencia de la tarjeta, seccion
	// 6), se prende y la vuelta entra. Se pisa por entorno con ARENA_DS1=.
	arenaDelDS1 = "apagado"
)
