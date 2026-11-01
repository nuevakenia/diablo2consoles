package d2ds1

// DC-002.110 -- EL `.ds1` DE UN NIVEL FUERA DEL SEMIESPACIO DEL RECOLECTOR.
//
// POR QUE EXISTE. Medido en DC-002.109 (Flycast, `gc_stats`): un nivel compuesto deja ~413.000 B vivos en el
// semiespacio del recolector de libgodc (1.835.008 B) despues de una recoleccion COMPLETA, y son los del
// `.ds1` del nivel: el mismo salto que da el parseo (+421.304 B). Con eso, la vuelta del cambio de nivel no
// entra (1.735.048 B despues de parsear su `.ds1` + ~382.632 B de las casillas > 1.835.008 B) aunque el
// monton de C este sano (949.688 B libres). La ida si entraba; lo que no entra es VOLVER.
//
// CUANTO ES, medido en el host (`console/dc/medidas/ds1-110`, que es la medida de esta tarjeta):
//
//	TownN1.ds1 / TownE1.ds1   57x41 = 2337 celdas, 4 capas (2 de pared, 1 de piso, 1 de sombra)
//	una celda = sizeof(d2ds1.Tile): 24 B en el host, 28 B en el SH-4
//	las casillas del `.ds1` = 2337 x 24 x 4 = 224.352 B en el host (261.744 B en la consola)
//
// O sea que lo que hay que sacar del semiespacio son las CUATRO rejillas de celdas de las capas. El resto del
// `.ds1` (la lista de archivos, los objetos, los caminos de los NPC) son unos pocos KB y se quedan donde
// estan: no son la pared.
//
// POR QUE NO SE VAN SOLAS, y esto es lo que la medicion del host deja dicho: en el host (recolector EXACTO) el
// `.ds1` soltado se libera en la recoleccion siguiente -- `console/dc/medidas/ds1-110` lo mide (el monton
// vuelve al nivel de partida). En libgodc el barrido de la pila es CONSERVADOR: el puntero al `.ds1` que quedo
// en un marco de pila ya muerto (el de `componerNivel`) cuenta como raiz y el recolector copia el `.ds1`
// entero al otro semiespacio en CADA recoleccion. Es la misma clase de fuga que DC-002.109 midio al pisar con
// ceros la basura de la pila (recupero 17.000 de 413.000: no alcanzaba con eso).
//
// LA CURA (el camino que ya usaron DC-002.101/.104/.105 para el arena de bloques y la cache de tiles): las
// celdas de las capas de un `.ds1` se piden en UN bloque del MONTON DE C, con liberador explicito inyectado.
// En libgodc todo pedido de mas de `GC_LARGE_OBJECT_THRESHOLD` (64 KB) va a `gc_external_alloc` (malloc
// pelado, fuera de los dos semiespacios): ese bloque NO lo copia el recolector, asi que el `.ds1` de un nivel
// deja de ocupar el semiespacio aunque una pila conservadora lo siga viendo.
//
// EL CONTRATO, dicho como es:
//
//   - El bloque se pide UNO por `.ds1` y del tamaño exacto (`capas x ancho x alto` celdas); las filas de cada
//     capa son rebanadas ADENTRO de ese bloque, no un `make` por fila.
//   - Se pide en `loadBody`, cuando ya se saben el tamaño y cuantas capas hay, y solo si la suma pasa el umbral
//     de los 64 KB: por debajo, el bloque caeria en el semiespacio y no compraria nada (se cuenta en
//     `ArenasChicas` y todo sigue por el camino de siempre).
//   - Se devuelve con `(*DS1).SoltarArenaDeTiles()`, que llama al liberador inyectado. SIN liberador instalado
//     no se toca nada (la PC: ahi no hay monton externo que devolver y el recolector hace su trabajo).
//   - DESPUES DE SOLTAR EL `.ds1` ESTA MUERTO: las capas quedan sin celdas, asi que un uso tardio se ve como
//     una capa vacia (`Layer.Tile` devuelve nil) y no como memoria de otro. Es el mismo contrato que el tramo
//     prestado de la fuente (DC-002.96) y el arena de bloques (DC-002.101).
//   - Quien decide CUANDO se suelta es el que compone el nivel: despues de `PlaceStampClippedWhere` el
//     `.ds1` no lo mira nadie (las casillas del mapa son COPIAS de las celdas, no punteros).
//
// POR QUE EL LIBERADOR SE INYECTA. `free` no esta en la libreria estandar de Go: en la consola es
// `d2c_soltar_bloque` del shim del POC (que es `gc_external_free`: saca el bloque de la tabla de externos del
// runtime y le hace free). Un paquete de `d2common` no puede llamarlo solo, asi que quien arranca la consola
// dice con que se libera (`UsarLiberadorDeLaArena`). En la PC no se instala nunca.
//
// POR QUE EL BLOQUE ES `[]Tile` Y NO `[]byte`. El tamaño de una asignacion lo decide el recolector por BYTES,
// asi que `make([]Tile, capas*ancho*alto)` pasa el umbral igual que un `make([]byte, ...)` de los mismos bytes,
// y no hace falta mirar el bloque como celdas armando una cabecera de rebanada a mano (`unsafe.Slice` no
// existe en gccgo y la cabecera a mano traia un Sizeof de mas por celda). El unico `unsafe` que queda es la
// direccion que se le pasa al liberador, que es lo que pide `free`.

import "unsafe"

// umbralArenaDeTiles: el mismo corte que usa libgodc para mandar una asignacion al monton de C
// (`GC_LARGE_OBJECT_THRESHOLD` = 64 KB, `gc_heap.c:116`). Debajo de esto el bloque vive en el semiespacio del
// recolector y sacarlo de ahi no compra nada.
const umbralArenaDeTiles = 64 * 1024

var (
	// arenaDeTilesEncendida: la palanca. La prende el arranque de la consola (`ActivarArenaDelDS1`).
	arenaDeTilesEncendida bool

	// liberadorDeLaArena: con que se devuelve el bloque al monton. nil = no se libera a mano (la PC).
	liberadorDeLaArena func(direccion uintptr, bytes int) int

	// el instrumento
	arenasPedidas  int // bloques pedidos (uno por `.ds1` con capas)
	arenasSoltadas int // bloques devueltos con el liberador
	bytesDeArenas  int // bytes pedidos en total
	bytesDevueltos int // bytes que el liberador dijo haber devuelto
	arenasChicas   int // `.ds1` que no llegaron al umbral (siguen en el camino de siempre)
	arenasSinLugar int // bloques que el monton no pudo dar (se sigue con el camino de siempre)
	celdasEnArenas int // celdas que viven fuera del monton de Go
	capasEnArenas  int // capas que viven fuera del monton de Go

	// bytesPorCelda: lo que ocupa una celda de una capa en ESTA plataforma (24 B en el host, 28 B en el SH-4).
	bytesPorCelda = int(unsafe.Sizeof(Tile{}))
)

// ActivarArenaDelDS1 prende el camino del bloque del monton de C. La llama el arranque de la consola; la PC no
// la llama nunca (y sin ella todo queda exactamente como estaba).
func ActivarArenaDelDS1() { arenaDeTilesEncendida = true }

// UsarLiberadorDeLaArena dice con que se devuelve al monton el bloque de celdas de un `.ds1`. En la consola se
// pasa `d2c_soltar_bloque` (ver la cabecera).
func UsarLiberadorDeLaArena(l func(direccion uintptr, bytes int) int) { liberadorDeLaArena = l }

// ArenaDelDS1Encendida dice si el camino esta prendido (lo consulta el instrumento del arranque).
func ArenaDelDS1Encendida() bool { return arenaDeTilesEncendida }

// HayLiberadorDeLaArena dice si el arranque instalo el liberador.
func HayLiberadorDeLaArena() bool { return liberadorDeLaArena != nil }

// BytesPorCelda es lo que ocupa una celda de una capa en esta plataforma (24 B en el host, 28 B en el SH-4).
func BytesPorCelda() int { return bytesPorCelda }

// CensoDeLaArena: el instrumento, para el log de la consola.
type CensoDeLaArena struct {
	Encendida    bool
	ConLiberador bool
	Pedidas      int
	Soltadas     int
	Bytes        int
	Devueltos    int
	Chicas       int
	SinLugar     int
	Celdas       int
	Capas        int
	Vivos        int // bytes pedidos que todavia no se devolvieron (tiene que volver a 0)
}

// EstadoDeLaArena devuelve el instrumento de la arena.
func EstadoDeLaArena() CensoDeLaArena {
	return CensoDeLaArena{
		Encendida:    arenaDeTilesEncendida,
		ConLiberador: liberadorDeLaArena != nil,
		Pedidas:      arenasPedidas,
		Soltadas:     arenasSoltadas,
		Bytes:        bytesDeArenas,
		Devueltos:    bytesDevueltos,
		Chicas:       arenasChicas,
		SinLugar:     arenasSinLugar,
		Celdas:       celdasEnArenas,
		Capas:        capasEnArenas,
		Vivos:        bytesDeArenas - bytesDevueltos,
	}
}

// reservarArenaDeTiles pide el bloque del `.ds1` y se lo reparte a las capas que ya existan. Devuelve los
// bytes pedidos (0 si la palanca esta apagada, si el bloque no llega al umbral o si el monton no lo dio).
//
// Se llama desde `loadBody` cuando el `.ds1` ya dijo su tamaño y cuantas capas tiene (ver ds1.go), ANTES de
// que las capas se llenen: asi las filas de cada capa son rebanadas de este bloque y no hay `make` por fila.
func (l *ds1Layers) reservarArenaDeTiles(capas int) int {
	if !arenaDeTilesEncendida || capas < 1 || l.width < 1 || l.height < 1 {
		return 0
	}

	celdas := capas * l.width * l.height
	total := celdas * bytesPorCelda

	if total < umbralArenaDeTiles {
		arenasChicas++

		return 0
	}

	bloque := make([]Tile, celdas) // > 64 KB: en libgodc esto es `gc_external_alloc`, fuera del semiespacio
	if len(bloque) < celdas {
		arenasSinLugar++

		return 0
	}

	l.arena = bloque
	l.arenaUsada = 0

	arenasPedidas++
	bytesDeArenas += total

	// las capas que ya estaban (la de sustitucion se empuja en la cabecera) se mudan al bloque
	for _, grupo := range []*layerGroup{&l.Floors, &l.Walls, &l.Shadows, &l.Substitutions} {
		for idx := range *grupo {
			l.ponerEnElArena((*grupo)[idx])
		}
	}

	return total
}

// ponerEnElArena le da a una capa su rebanada del bloque (si queda lugar). Sin bloque no hace nada y la capa
// sigue con su rejilla de siempre.
func (l *ds1Layers) ponerEnElArena(capa *Layer) {
	if capa == nil || l.arena == nil {
		return
	}

	necesita := l.width * l.height
	if necesita < 1 || l.arenaUsada+necesita > len(l.arena) {
		return
	}

	capa.usarCelda(l.arena[l.arenaUsada:l.arenaUsada+necesita], l.width, l.height)

	l.arenaUsada += necesita
	capasEnArenas++
	celdasEnArenas += necesita
}

// soltarArena devuelve el bloque del `.ds1` al monton y deja las capas sin celdas. Devuelve los bytes que se
// devolvieron DE VERDAD (0 si no habia bloque o si no hay liberador instalado).
func (l *ds1Layers) soltarArena() int {
	if l == nil || len(l.arena) == 0 {
		return 0
	}

	// las capas dejan de apuntar al bloque ANTES de liberarlo: un uso tardio se ve vacio, no como memoria ajena
	celdas := len(l.arena)
	bytes := celdas * bytesPorCelda
	direccion := uintptr(unsafe.Pointer(&l.arena[0]))

	for _, grupo := range []*layerGroup{&l.Floors, &l.Walls, &l.Shadows, &l.Substitutions} {
		for idx := range *grupo {
			if (*grupo)[idx] != nil {
				(*grupo)[idx].soltarCelda()
			}
		}
	}

	l.arena = nil
	l.arenaUsada = 0

	devueltos := 0
	if liberadorDeLaArena != nil {
		devueltos = liberadorDeLaArena(direccion, bytes)
		arenasSoltadas++
		bytesDevueltos += devueltos
	}

	// con el bloque se fue TODO lo que vivia fuera del monton de Go de este `.ds1`
	capasEnArenas = 0
	celdasEnArenas = 0

	return devueltos
}

// SoltarArenaDeTiles devuelve al monton el bloque de celdas de ESTE `.ds1` (DC-002.110). Devuelve los bytes
// que el liberador dijo haber devuelto (0 si el `.ds1` vive en el monton de Go o no hay liberador instalado).
//
// CONTRATO: despues de esto el `.ds1` esta MUERTO. Las capas quedan sin celdas, asi que `Tile` devuelve nil y
// el que lo use despues ve una capa vacia -- no memoria devuelta. El que compone el nivel lo llama cuando ya
// no lo mira nadie: despues de `PlaceStampClippedWhere` las casillas del mapa son copias de las celdas.
func (ds1 *DS1) SoltarArenaDeTiles() int {
	if ds1 == nil || ds1.ds1Layers == nil {
		return 0
	}

	return ds1.soltarArena()
}

// EnArena dice si las celdas de este `.ds1` viven fuera del monton de Go (DC-002.110).
func (ds1 *DS1) EnArena() bool {
	return ds1 != nil && ds1.ds1Layers != nil && ds1.ds1Layers.arena != nil
}

// BytesDeLaArena dice cuanto mide el bloque de celdas de este `.ds1` (0 si vive en el monton de Go).
func (ds1 *DS1) BytesDeLaArena() int {
	if ds1 == nil || ds1.ds1Layers == nil {
		return 0
	}

	return len(ds1.ds1Layers.arena) * bytesPorCelda
}

// SoltarArenaDeTilesDe es `(*DS1).SoltarArenaDeTiles` como FUNCION de paquete, para los llamadores de la consola.
// MEDIDO en esta tarjeta: gccgo 15.1 no resolvia `mr.ds1.SoltarArenaDeTiles()` desde `d2mapstamp` ("reference to
// undefined field or method" sobre un metodo que SI esta en el `.gox` de `d2ds1`, comprobado con `strings`), y la
// funcion de paquete si se resuelve. El mismo codigo compila sin problema en un paquete aparte, asi que la causa
// exacta del defecto no quedo aislada -- lo que queda es el camino que compila. Si algun dia se afeita, el metodo
// sigue ahi y esto se puede borrar.
func SoltarArenaDeTilesDe(ds1 *DS1) int {
	return ds1.SoltarArenaDeTiles()
}

// EnArenaDe es `(*DS1).EnArena` como funcion de paquete (mismo motivo que SoltarArenaDeTilesDe).
func EnArenaDe(ds1 *DS1) bool { return ds1.EnArena() }

// BytesDeLaArenaDe es `(*DS1).BytesDeLaArena` como funcion de paquete (mismo motivo que SoltarArenaDeTilesDe).
func BytesDeLaArenaDe(ds1 *DS1) int { return ds1.BytesDeLaArena() }
