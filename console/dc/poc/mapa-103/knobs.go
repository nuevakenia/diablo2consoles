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
)
