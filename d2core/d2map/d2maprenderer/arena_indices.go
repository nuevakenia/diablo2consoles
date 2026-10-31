package d2maprenderer

// DC-002.104 -- LOS PIXELES DE LA CACHE DE TILES VIENEN DE UN SOLO TRAMO (ADR 0001).
//
// POR QUE EXISTE. DC-002.101 dejo la cache de tiles en INDICES de 8 bits: en el pueblo son 86 imagenes y
// 2.723.840 B, cuatro veces menos que los 10.895.360 B de RGBA. Aun asi la consola no la puede armar, y
// DC-002.103 midio POR QUE, con el censo por imagen (`TamanosDeLaCache`):
//
//	86 imagenes = 2.723.840 B | 82 pesan menos de 64 KB (2.447.360 B) | 4 pasan de 64 KB (276.480 B)
//
// En libgodc (`libgodc/runtime/gc_heap.c`, `GC_LARGE_OBJECT_THRESHOLD_KB`) toda asignacion de MAS de 64 KB se
// va a `gc_external_alloc` (= `malloc` sobre el monton de C, `mm.c`) y el recolector NO la toca ni la
// devuelve; las de menos van al SEMIESPACIO del recolector, que mide 1.792 KB y ya trae ~1,35 MB de conjunto
// vivo. Con las imagenes chicas, la cache pide 2,44 MB AL SEMIESPACIO y los tres ordenes de DC-002.103 mueren
// ahi (livelock de recolecciones, `gc-vivo` al tope con 2,8 MB libres en el monton de C: el problema no es
// CUANTA memoria hay, es DONDE se pide).
//
// QUE ES. UN tramo: un solo `make` de mas de 64 KB (o sea, en la consola, una sola asignacion del monton de C,
// que el recolector no copia) y las imagenes de la cache se REPARTEN adentro, cada una con su pedazo. Es la
// decision 1b del ADR 0001 aplicada a la cache de tiles: el buffer tiene dueno (este tramo) y vive lo que vive
// el nivel, no lo que vive una recoleccion.
//
// SE APAGA SOLA EN LA PC. La palanca es ADITIVA y por defecto esta apagada: sin llamar a
// `ActivarTramoDeLaCache` el camino es EXACTAMENTE el de siempre (`make` por imagen) y la PC no cambia una
// linea -- el camino de la PC (`d2gamescreen`) no la llama nunca. Lo que la hace "de consola" no es una rama
// por plataforma: es el tamano. Un `make` de 2,7 MB en la PC es un buffer de Go como cualquier otro; en
// libgodc el mismo `make` es una asignacion externa. Por eso el umbral se declara y se respeta: un tramo de
// 64 KB o menos no se activa (seria una imagen chica mas en el semiespacio y no compra nada).
//
// CUANDO SE RESERVA IMPORTA. En la consola el monton de C es dlmalloc sobre `mm_sbrk`
// (`kos/kernel/arch/dreamcast/kernel/mm.c`): crece hacia arriba hasta `_arch_mem_top - THD_KERNEL_STACK_SIZE`
// y no arma un bloque grande cosiendo huecos libres chicos. Medido en DC-002.104: con 3.693.432 B "libres"
// repartidos en el monton, un pedido de 2.723.840 B de un solo golpe NO entra (fatal en `gc_external_alloc`).
// De ahi que se pueda RESERVAR el tramo TEMPRANO (`ReservarTramoDeLaCache`) y colgarlo del renderer despues
// (`UsarTramoDeLaCache`), cuando el monton todavia no esta fragmentado.
//
// LO QUE NO HACE. No libera el tramo: `gc_external_alloc` no se devuelve (ADR 0001 §1.4) y el tramo vive lo
// que vive el nivel; al cambiar de nivel (`resetLevelCaches`) se REUSA desde el byte 0. No toca la paleta ni
// el dibujado: los pixeles y su orden son los mismos, y por eso el cuadro del host con el tramo puesto tiene
// que dar el MISMO crc32 que sin el. Si una imagen no entra (el tramo se quedo corto), se pide como siempre y
// se cuenta el derrame -- el numero que dice cuanto habria que agrandar el tramo.

// UmbralDelObjetoGrande: el corte de libgodc (`GC_LARGE_OBJECT_THRESHOLD_KB`, 64 KB). Lo que pasa de aca va
// al monton de C; lo que no, al semiespacio del recolector.
const UmbralDelObjetoGrande = 64 * 1024

// TramoDeLaCache: UN buffer del que se recortan los pixeles de las imagenes de indices.
type TramoDeLaCache struct {
	bytes           []byte
	usado           int
	imagenes        int
	derrames        int
	bytesDerramados int
}

// Bytes: el tamano del tramo (lo que se pidio de una sola vez).
func (t *TramoDeLaCache) Bytes() int {
	if t == nil {
		return 0
	}

	return len(t.bytes)
}

// CensoDelTramo: el censo del tramo, para el log de la consola.
type CensoDelTramo struct {
	Activa          bool
	Bytes           int // el tamano del tramo (0 = apagada)
	Usado           int // lo que se recorto para las imagenes
	Libre           int
	Imagenes        int // imagenes servidas del tramo
	Derrames        int // imagenes que no entraron y se pidieron por `make` (como siempre)
	BytesDerramados int
}

// ReservarTramoDeLaCache: UN buffer de `bytes` B para los pixeles de la cache en indices (DC-002.104, ADR 0001).
//
// POR QUE UN TAMANO Y NO UN "SI/NO". El tramo tiene que ser MAYOR que 64 KB para irse al monton de C (que es
// todo el punto), y cuanto mide depende del nivel: en el pueblo la cache entera son 2.723.840 B (DC-002.101).
// El que llama dice el tamano porque es el que sabe cuantas imagenes va a querer; lo que no entra se derrama y
// se cuenta (`CensoDelTramo`), para que el numero que falta salga del mismo log.
//
// Un valor de 64 KB o menos devuelve nil: ese tamano se queda en el semiespacio del recolector y no compra
// nada. Se puede reservar ANTES de que exista el renderer (el monton de C se fragmenta y el momento importa) y
// colgarlo despues con `UsarTramoDeLaCache`.
func ReservarTramoDeLaCache(bytes int) *TramoDeLaCache {
	if bytes <= UmbralDelObjetoGrande {
		return nil
	}

	return &TramoDeLaCache{bytes: make([]byte, bytes)}
}

// UsarTramoDeLaCache: la cache de indices pide sus pixeles a ESTE tramo (ya reservado).
func (mr *MapRenderer) UsarTramoDeLaCache(t *TramoDeLaCache) { mr.tramo = t }

// ActivarTramoDeLaCache: reserva el tramo y lo usa, en un paso. Es la forma corta de
// `UsarTramoDeLaCache(ReservarTramoDeLaCache(bytes))`. Es ADITIVA: sin llamarla, cada imagen se pide con
// `make` como siempre.
func (mr *MapRenderer) ActivarTramoDeLaCache(bytes int) {
	t := ReservarTramoDeLaCache(bytes)

	if t == nil {
		mr.tramo = nil
		mr.Warningf("el tramo de la cache de tiles no se activa con %d B (tiene que pasar los %d B del objeto "+
			"grande de libgodc, DC-002.104)", bytes, UmbralDelObjetoGrande)

		return
	}

	mr.tramo = t

	mr.Debugf("tramo de la cache de tiles activo: %d B (DC-002.104)", len(t.bytes))
}

// CensoDelTramo: el censo del tramo (apagado si no se activo).
func (mr *MapRenderer) CensoDelTramo() CensoDelTramo {
	if mr.tramo == nil {
		return CensoDelTramo{}
	}

	return CensoDelTramo{
		Activa:          true,
		Bytes:           len(mr.tramo.bytes),
		Usado:           mr.tramo.usado,
		Libre:           len(mr.tramo.bytes) - mr.tramo.usado,
		Imagenes:        mr.tramo.imagenes,
		Derrames:        mr.tramo.derrames,
		BytesDerramados: mr.tramo.bytesDerramados,
	}
}

// pedirIndicesDelTile: los `n` bytes de pixeles de UNA imagen de la cache.
//
// Con el tramo prendido, los pixeles son un pedazo del tramo (misma len, misma cap: nadie puede crecerlo, que
// es lo que hace que dos imagenes no se pisen) y el decodificador del `.dt1` escribe adentro, igual que sobre
// un `make`. Sin tramo -- o si no entra -- se pide un `make` de siempre.
func (mr *MapRenderer) pedirIndicesDelTile(n int) []byte {
	if n <= 0 {
		return nil
	}

	if mr.tramo != nil {
		if mr.tramo.usado+n <= len(mr.tramo.bytes) {
			desde := mr.tramo.usado
			mr.tramo.usado += n
			mr.tramo.imagenes++

			return mr.tramo.bytes[desde : desde+n : desde+n]
		}

		mr.tramo.derrames++
		mr.tramo.bytesDerramados += n
	}

	return make([]byte, n)
}

// reiniciarTramo: el nivel cambio, la cache se descarta y el tramo se reusa desde el byte 0. El tramo NO se
// libera (en la consola el monton externo no se devuelve, ADR 0001 §1.4): se reusa.
func (mr *MapRenderer) reiniciarTramo() {
	if mr.tramo == nil {
		return
	}

	mr.tramo.usado = 0
	mr.tramo.imagenes = 0
	mr.tramo.derrames = 0
	mr.tramo.bytesDerramados = 0
}
