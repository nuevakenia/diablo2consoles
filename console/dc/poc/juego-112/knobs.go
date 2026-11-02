package main

// DC-002.112 -- LAS PALANCAS DEL POC DEL PRIMER CUADRO DE JUEGO.
//
// TODAS son las mismas de DC-002.103...111, con los mismos valores (son la configuracion con la que el pueblo
// ya se dibuja y se presenta igual al host: crc32 1437022778), MAS las tres de esta tarjeta. Se dejan escritas
// aca y no en cada corrida por la leccion de DC-002.90: el `knobs.go` del POC es la unica fuente de verdad, el
// entorno solo puede PISARLO y, cuando lo pisa, el script lo avisa en grande y el valor queda en el knobs del
// directorio de build (el binario dice con cual se compilo).
const (
	// ============================== LAS DEL PUEBLO (DC-002.103/.111) ==============================
	//
	// orden: "contrato" | "prestamo" | "tarjeta". La referencia del POC es "prestamo": es el orden LEGAL que
	// llega mas lejos (se devuelve lo unico que se puede devolver sin invalidar nada: el buffer prestado de la
	// lectura de los `.dt1`) y el arena de bloques se suelta DESPUES del cuadro 1, con la cache ya armada.
	orden = "prestamo"

	// cuadros: cuantos se dibujan antes de dejar de imprimir (el ciclo del PVR no vuelve nunca).
	cuadros = 3

	// modoTablas: "stream" (el camino viejo) o "fuente" (el tramo de la fuente con dueno, DC-002.102).
	modoTablas = "fuente"

	// soltarLecturas: devolver el tramo de lecturas de la fuente antes de componer y despues de componer.
	soltarLecturas = true

	// tramoDeLaCache (DC-002.104): los pixeles de la cache en indices se piden a UN tramo de este tamano (el
	// censo medido en el host: 86 imagenes, 2.723.840 B, 0 derrames).
	tramoDeLaCache = 2723840

	// tramoEnPedazos (DC-002.106): el MISMO tramo pedido en bloques de este tamano (340 KB = 68 filas de
	// 5.120 B, el grano de los tamanos de la cache): asi el monton lo sirve de sus huecos y la pared deja de
	// ser la CONTIGUIDAD. 0 = de un solo golpe.
	tramoEnPedazos = 348160

	// pedazoLoDiceElMonton (DC-002.106): el tamano de cada bloque lo decide el monton (la sonda `d2c_servible`).
	pedazoLoDiceElMonton = true

	// cacheEnVram (DC-002.108): los bloques de la cache se piden a la VRAM DEL PVR (`pvr_mem_malloc`) en vez
	// del monton de C. El dibujado sigue leyendo los indices con la CPU: el cuadro sale identico al del host.
	cacheEnVram = true

	// pruebaDeVram (DC-002.108): escribir un patron en un bloque de la VRAM y releerlo, al arrancar el video.
	pruebaDeVram = true

	// tramoAlArranque (DC-002.104): reservar el tramo de la cache ANTES de cargar nada. Con el tramo EN PEDAZOS
	// no se reserva nada al arranque (los bloques se piden en el primer cuadro).
	tramoAlArranque = false

	// soltarDespuesDeComponer (DC-002.105): que se devuelve al monton de C en cuanto el pueblo esta compuesto y
	// ANTES de crear el renderer: "nada" | "tablas" | "fuentes" | "todo". La referencia es "todo" (679.440 B).
	// OJO CON EL CONTRATO: el cuadro SI lee dos archivos del MPQ al armar la cache (la paleta del acto y su
	// `pal.pl2`), por eso se PRECARGAN antes de soltar las fuentes.
	soltarDespuesDeComponer = "todo"

	// scratchPorArchivo (DC-002.111): el buffer prestado con el que se lee cada `.dt1` se pide del tamano de ESE
	// archivo y se devuelve en cuanto sus bloques estan copiados al arena: saca 1.008.003 B del pico.
	scratchPorArchivo = true

	// tramoDelArenaPorElMonton (DC-002.111): el tamano de cada tramo del ARENA DE BLOQUES lo decide el monton
	// (si no, el septimo tramo de 524.288 B muere por contiguidad con 1,4 MB libres).
	tramoDelArenaPorElMonton = true

	// soltarTramosAlCambiarDeNivel (DC-002.111): los tramos del arena de bloques vuelven al monton de C al
	// tirar un nivel. Con un solo nivel no cambia nada; se deja en el valor medido para que el binario sea el
	// mismo camino que el de DC-002.111.
	soltarTramosAlCambiarDeNivel = true

	// ============================== LAS DE ESTA TARJETA (DC-002.112) ==============================
	//
	// jugador: "encendido" (la corrida del criterio: el pueblo CON el jugador) | "apagado" (la referencia de
	// DC-002.106/.108 en el MISMO binario: el pueblo solo, mismo codigo y mismo arranque). Con "apagado" no se
	// abre d2char.mpq, no se compone el ciclo y el cuadro tiene que dar otra vez crc32 1437022778.
	// Se pisa por entorno con JUGADOR=.
	jugadorKnob = "encendido"

	// modoDelJugador y dirDelJugador: la animacion y la direccion que se componen. NU es el neutral de la
	// amazona (parada y desarmada, `amnuhth.cof`) y la direccion 0 es la que mira al sur-oeste, la misma que
	// usan las tablas de referencia de DC-002.38/.100.
	modoDelJugador = "NU"
	dirDelJugador  = 0

	// msDelJugador: 128,00 ms por cuadro (7,81 cuadros/s), medido del `animdata.d2` en DC-002.49. No lo usa el
	// cuadro (esta tarjeta no anima: DC-002.113 mueve al jugador), pero es el reloj del ciclo y se informa.
	msDelJugador = 128

	// cuadroDelJugador: cual de los cuadros compuestos se dibuja. 0 = el primero, que es el que compara el
	// corpus de DC-002.38 (caja 26x79, 896 pixeles, crc32 1857478036).
	cuadroDelJugador = 0

	// ============================== LA ARENA DEL .DS1 (DC-002.110), APAGADA ==============================
	//
	// arenaDelDS1: las celdas de las capas de un `.ds1` se piden en UN bloque del monton de C. Medido en
	// DC-002.110: saca 380.544 B del semiespacio pero el monton de C no tiene esos 186.960 B en el pico de
	// componer (OOM en la TRANSICION). Esta tarjeta compone UN nivel, asi que la palanca no cambia nada de lo
	// que se mide y se deja como estaba (apagada) para no moverle el piso a la referencia del pueblo.
	arenaDelDS1 = "apagado"
)
