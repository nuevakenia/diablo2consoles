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

// TramoDeLaCache: los buffers de los que se recortan los pixeles de las imagenes de indices.
//
// DOS FORMAS, la misma idea (un buffer con dueno del que se reparten las imagenes):
//
//	UN GOLPE (DC-002.104)  `bytes`: un solo `make` del tamano del censo entero. En la consola eso es UNA
//	                       asignacion del monton de C, y dlmalloc la sirve por `sbrk` de un tiron: si el
//	                       break room (techo - sbrk) no llega, no entra aunque "haya" memoria libre
//	                       repartida en huecos (medido en DC-002.105: 2.849.816 B disponibles, 1.204.224
//	                       de break, pedido de 2.723.840 de un golpe -> muere).
//	EN PEDAZOS (DC-002.106) `pedazos`: el mismo total pedido de a bloques de `pedazo` B, cada uno cuando
//	                       hace falta. Cada pedazo si se sirve de los huecos y del top, asi que la pared deja
//	                       de ser la CONTIGUIDAD. Lo que se pierde es la cola de cada pedazo (unos KB por
//	                       bloque): por eso el total es un TECHO (`total`) y lo que sobra se cuenta.
type TramoDeLaCache struct {
	bytes []byte // el tramo de UN GOLPE (nil en el modo en pedazos)

	// modo en pedazos (DC-002.106): `total` es el techo de bytes, `pedazo` el tamano de cada bloque,
	// `pedido` lo que se le pidio al monton hasta ahora y `usadoEn` cuanto se recorto de cada bloque.
	// `pedazos` arranca vacio: se pide lo que haga falta, cuando haga falta (asi el monton decide de donde
	// sale cada bloque con el estado que tiene EN ESE MOMENTO, no con el del arranque).
	pedazos [][]byte
	usadoEn []int
	pedazo  int
	total   int
	pedido  int

	// servible (DC-002.106): si esta puesto, el tamano de cada bloque nuevo lo dice el MONTON -- le pregunta
	// "cuanto podes servir de hasta `pedazo` B ahora" -- en vez de pedir siempre `pedazo`. Es la diferencia
	// entre cortarse en el ultimo bloque (medido en Flycast: 286.720 B pedidos con 159.744 de break y las
	// migas libres sin poder coserse) y seguir bajando de tamano hasta que entra. Nil = tamano fijo.
	servible func(tope int) int

	// reserva (DC-002.108): DE DONDE SALEN los bloques. Nil = `make` (el monton de C en la consola, el monton
	// de Go en la PC), que es el camino de DC-002.106. Puesto, cada bloque lo pide esta funcion y si devuelve
	// nil (el que reserva no tenia lugar) la imagen se derrama al camino de siempre y se cuenta. Es la palanca
	// que pone los 2,7 MB de la cache EN LA VRAM del PVR sin que este paquete sepa que existe un PVR.
	reserva func(bytes int) []byte

	// origen: de donde vienen los bloques, para el censo y el log ("monton" si `reserva` es nil, "vram" si el
	// que llama puso ahi la VRAM). Es un string y no un enum a proposito: este paquete no tiene por que
	// conocer los backends.
	origen string

	// pedazoMenor / pedazoMayor: el rango de tamanos con el que el monton fue sirviendo los bloques, para el
	// censo (0 = todavia no se pidio ninguno).
	pedazoMenor int
	pedazoMayor int

	usado           int
	imagenes        int
	derrames        int
	bytesDerramados int
}

// Bytes: el tamano del tramo (lo que se pidio de una sola vez, o lo que lleva pedido el modo en pedazos).
func (t *TramoDeLaCache) Bytes() int {
	if t == nil {
		return 0
	}

	if t.pedazo > 0 {
		return t.pedido
	}

	return len(t.bytes)
}

// Pedazos: cuantos bloques se pidieron (0 en el modo de un golpe).
func (t *TramoDeLaCache) Pedazos() int {
	if t == nil {
		return 0
	}

	return len(t.pedazos)
}

// Techo: el total que el modo en pedazos puede llegar a pedir (0 en el modo de un golpe, que ya lo pidio todo).
func (t *TramoDeLaCache) Techo() int {
	if t == nil {
		return 0
	}

	return t.total
}

// EnPedazos: si el tramo se pide por bloques (DC-002.106).
func (t *TramoDeLaCache) EnPedazos() bool { return t != nil && t.pedazo > 0 }

// origenDeLosBloques: de donde salen los bloques, para el censo (DC-002.108). Sin reservador es el monton.
func (t *TramoDeLaCache) origenDeLosBloques() string {
	if t == nil {
		return ""
	}

	if t.reserva == nil {
		return "monton"
	}

	if t.origen == "" {
		return "otro"
	}

	return t.origen
}

// nuevoBloque: UN bloque de `tam` B, del reservador si lo hay y del monton si no (DC-002.108). Devuelve nil
// cuando el reservador no tenia lugar: el que llama derrama la imagen al `make` de siempre y lo cuenta.
func (t *TramoDeLaCache) nuevoBloque(tam int) []byte {
	if t.reserva != nil {
		return t.reserva(tam)
	}

	return make([]byte, tam)
}

// CensoDelTramo: el censo del tramo, para el log de la consola.
type CensoDelTramo struct {
	Activa          bool
	EnPedazos       bool
	Origen          string // de donde salen los bloques (DC-002.108): "monton" o el nombre que puso el que llama ("vram")
	Pedazos         int    // cuantos bloques se pidieron (modo en pedazos)
	PedazoMenor     int    // el bloque mas chico que el monton sirvio (0 = todavia no se pidio ninguno)
	PedazoMayor     int    // el bloque mas grande
	Techo           int    // lo que el modo en pedazos puede llegar a pedir
	Bytes           int    // el tamano del tramo (0 = apagada); en pedazos, lo pedido hasta ahora
	Usado           int    // lo que se recorto para las imagenes
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

// ReservarTramoDeLaCacheEnPedazos: el tramo de la cache pedido en BLOQUES de `pedazo` B, con un techo de
// `total` B (DC-002.106).
//
// POR QUE EN PEDAZOS. Medido en DC-002.105: con todo lo que el cuadro no lee devuelto, el monton tiene
// 2.849.816 B disponibles (1.645.592 de libres + 1.204.224 de break room) y la cache pide 2.723.840, o sea que
// ALCANZA -- pero el tramo de un solo golpe muere igual, porque dlmalloc lo pide ENTERO por `sbrk` y el break
// room no llega a 2,7 MB. Pedido de a `pedazo` B, cada bloque si se sirve de los huecos y del top, y la pared
// deja de ser "faltan bytes contiguos".
//
// POR QUE NO SE PIDE NADA ACA. Los bloques se piden en `pedirIndicesDelTile`, a medida que las imagenes llegan
// (cache perezosa): asi el monton decide de donde sale cada bloque con el estado que tiene en ese momento y no
// se reserva de mas antes de saber si el cuadro lo necesita. Esta funcion solo deja escrito el plan.
//
// EL TECHO (`total`) ES LO QUE PROTEGE EL MONTON. La cola que se pierde en cada bloque (una imagen no entra en
// lo que queda y el bloque se cierra) es real, y pedir "hasta que alcance" terminaria en un pedido que el
// monton no puede servir -- que en la consola es fatal (`gc_external_alloc` tira OOM). Con techo, lo que no
// entra en el tramo se derrama al camino de siempre y se cuenta.
//
// Devuelve nil si algunos de los dos numeros no pasa el corte de 64 KB del objeto grande de libgodc: un tramo
// chico iria al semiespacio del recolector y no compra nada (lo mismo que `ReservarTramoDeLaCache`).
//
// `sonda` (opcional) es la que pregunta cuanto puede servir el monton (DC-002.106: en la consola el puente
// `d2c_servible` del shim, que sondea con `malloc` y se queda con el bloque mas grande que entra). Con sonda,
// el tamano de cada bloque es `min(pedazo, lo que el monton sirva)`; sin sonda, siempre `pedazo`.
func ReservarTramoDeLaCacheEnPedazos(total, pedazo int, sonda ...func(tope int) int) *TramoDeLaCache {
	if total <= UmbralDelObjetoGrande || pedazo <= UmbralDelObjetoGrande {
		return nil
	}

	if pedazo > total {
		pedazo = total
	}

	t := &TramoDeLaCache{pedazo: pedazo, total: total}

	if len(sonda) > 0 && sonda[0] != nil {
		t.servible = sonda[0]
	}

	return t
}

// ReservarTramoDeLaCacheDe (DC-002.108): UN tramo de `bytes` B pedido al RESERVADOR (la VRAM del PVR en la
// consola) en vez de un `make`. Es el modo de un solo golpe de DC-002.104 con el origen cambiado, y es la
// medida mas directa de la tarjeta: el pedido de 2.723.840 B que en el monton de C muere por contiguidad
// (`gc_external_alloc`, DC-002.104) entra en la VRAM de un golpe o no entra (en ese caso devuelve nil y el que
// llama se queda con el camino en bloques).
func ReservarTramoDeLaCacheDe(bytes int, reserva func(bytes int) []byte, origen string) *TramoDeLaCache {
	if bytes <= UmbralDelObjetoGrande || reserva == nil {
		return nil
	}

	b := reserva(bytes)
	if b == nil {
		return nil
	}

	if origen == "" {
		origen = "monton"
	}

	return &TramoDeLaCache{bytes: b, reserva: reserva, origen: origen}
}

// UsarTramoDeLaCache: la cache de indices pide sus pixeles a ESTE tramo (ya reservado).
func (mr *MapRenderer) UsarTramoDeLaCache(t *TramoDeLaCache) { mr.tramo = t }

// ActivarTramoDeLaCacheDe: reserva el tramo de UN golpe con reservador propio y lo usa (DC-002.108).
func (mr *MapRenderer) ActivarTramoDeLaCacheDe(bytes int, reserva func(bytes int) []byte, origen string) {
	t := ReservarTramoDeLaCacheDe(bytes, reserva, origen)

	if t == nil {
		mr.tramo = nil
		mr.Warningf("el tramo unico de la cache de %s no se activa con %d B (tiene que pasar los %d B del "+
			"objeto grande de libgodc, y el que reserva tiene que dar lugar; DC-002.108)", origen, bytes,
			UmbralDelObjetoGrande)

		return
	}

	mr.tramo = t

	mr.Debugf("tramo UNICO de la cache de %s: %d B en un solo pedido (DC-002.108)", origen, len(t.bytes))
}

// ReservarTramoDeLaCacheEnBloquesDe (DC-002.108): el tramo en bloques de `pedazo` B con techo `total`, pero
// los bloques los pide `reserva` en vez de un `make`. Es el mismo mecanismo de DC-002.106 con el ORIGEN
// cambiado: en la consola la cache de tiles vive en la VRAM del PVR (8 MB, de los que el motor tiene ~2,3 MB
// tomados), no en el monton de C -- que es lo que la tarjeta mide.
//
// POR QUE UNA FUNCION Y NO UN `import` DEL PVR. Este paquete es del MOTOR y se compila tambien para la PC:
// pedirle que conozca el backend de la consola rompe las dos cosas (el camino de la PC y la prueba del host).
// La reserva entra como funcion, que es el mismo patron de `servible` (DC-002.106) y del `PedirDeArena` del
// decodificador (DC-002.98).
//
// SI EL QUE RESERVA NO TIENE LUGAR devuelve nil: la imagen se derrama al `make` de siempre y se cuenta en el
// censo (`Derrames` / `BytesDerramados`). No es un error: es el numero que dice cuanto falto.
//
// `origen` es solo para el log y el censo ("vram"). Sin `origen`, se informa "monton".
func ReservarTramoDeLaCacheEnBloquesDe(total, pedazo int, reserva func(bytes int) []byte, origen string,
	sonda ...func(tope int) int) *TramoDeLaCache {
	if total <= UmbralDelObjetoGrande || pedazo <= UmbralDelObjetoGrande || reserva == nil {
		return nil
	}

	if pedazo > total {
		pedazo = total
	}

	if origen == "" {
		origen = "monton"
	}

	t := &TramoDeLaCache{pedazo: pedazo, total: total, reserva: reserva, origen: origen}

	if len(sonda) > 0 && sonda[0] != nil {
		t.servible = sonda[0]
	}

	return t
}

// ActivarTramoDeLaCacheEnBloquesDe: reserva el tramo de bloques con reservador propio y lo usa, en un paso
// (DC-002.108). Es la forma corta de `UsarTramoDeLaCache(ReservarTramoDeLaCacheEnBloquesDe(...))`.
func (mr *MapRenderer) ActivarTramoDeLaCacheEnBloquesDe(total, pedazo int, reserva func(bytes int) []byte,
	origen string, sonda ...func(tope int) int) {
	t := ReservarTramoDeLaCacheEnBloquesDe(total, pedazo, reserva, origen, sonda...)

	if t == nil {
		mr.tramo = nil
		mr.Warningf("el tramo de la cache en bloques de %s no se activa con total=%d pedazo=%d B (los dos tienen "+
			"que pasar los %d B del objeto grande de libgodc, DC-002.108)", origen, total, pedazo,
			UmbralDelObjetoGrande)

		return
	}

	mr.tramo = t

	mr.Debugf("tramo de la cache en bloques de %s: techo %d B en bloques de %d B (DC-002.108)", origen, total,
		pedazo)
}

// ActivarTramoDeLaCacheEnPedazos: reserva el tramo en pedazos y lo usa, en un paso. Es la forma corta de
// `UsarTramoDeLaCache(ReservarTramoDeLaCacheEnPedazos(total, pedazo))` (DC-002.106). ADITIVA igual que las
// otras: sin llamarla, la cache pide sus pixeles con `make` como siempre.
func (mr *MapRenderer) ActivarTramoDeLaCacheEnPedazos(total, pedazo int, sonda ...func(tope int) int) {
	t := ReservarTramoDeLaCacheEnPedazos(total, pedazo, sonda...)

	if t == nil {
		mr.tramo = nil
		mr.Warningf("el tramo en pedazos de la cache no se activa con total=%d pedazo=%d B (los dos tienen que "+
			"pasar los %d B del objeto grande de libgodc, DC-002.106)", total, pedazo, UmbralDelObjetoGrande)

		return
	}

	mr.tramo = t

	mr.Debugf("tramo de la cache en pedazos: techo %d B en bloques de %d B (DC-002.106)", total, pedazo)
}

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
		EnPedazos:       mr.tramo.EnPedazos(),
		Origen:          mr.tramo.origenDeLosBloques(),
		Pedazos:         mr.tramo.Pedazos(),
		PedazoMenor:     mr.tramo.pedazoMenor,
		PedazoMayor:     mr.tramo.pedazoMayor,
		Techo:           mr.tramo.Techo(),
		Bytes:           mr.tramo.Bytes(),
		Usado:           mr.tramo.usado,
		Libre:           mr.tramo.Bytes() - mr.tramo.usado,
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
		// modo en pedazos (DC-002.106): el bloque lo elige `pedirDelTramoEnPedazos`, que devuelve nil si derrama
		if mr.tramo.EnPedazos() {
			if p := mr.pedirDelTramoEnPedazos(n); p != nil {
				return p
			}

			return make([]byte, n)
		}

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

// pedirDelTramoEnPedazos: busca el bloque donde entra la imagen de `n` B y devuelve su pedazo; si no hay
// ninguno con lugar, PIDE un bloque nuevo (hasta el techo); si ya no hay techo, devuelve nil y la imagen se
// pide con `make` como siempre.
//
// POR QUE EL BLOQUE MAS JUSTO (best fit) Y NO EL DE EN CURSO. La cola de un bloque que no se llena se pierde,
// y los tamanos de las imagenes de la cache son MUY distintos (20.480 B las comunes, hasta 71.680 las cuatro
// grandes del pueblo): llenando el bloque en curso, una imagen que no entra en la cola abre otro bloque y deja
// la cola muerta. Con el bloque mas justo, la imagen chica cae en la cola que le sirve y el desperdicio total
// baja a la mitad, que es lo que decide si el tramo entra en el monton (DC-002.106: medido en el host, 61.440 B
// de cola contra 148.480 del camino en curso para 11 bloques de 250 KB).
func (mr *MapRenderer) pedirDelTramoEnPedazos(n int) []byte {
	t := mr.tramo

	if !t.EnPedazos() {
		return nil
	}

	// una imagen sola mas grande que un bloque no cabe con este plan: se pide como siempre y se cuenta
	if n > t.pedazo {
		t.derrames++
		t.bytesDerramados += n

		return nil
	}

	mejor := -1

	for i := range t.pedazos {
		libre := len(t.pedazos[i]) - t.usadoEn[i]
		if libre < n {
			continue
		}

		if mejor < 0 || libre < len(t.pedazos[mejor])-t.usadoEn[mejor] {
			mejor = i
		}
	}

	if mejor < 0 {
		restante := t.total - t.pedido
		if restante <= UmbralDelObjetoGrande {
			// no se pide un bloque que iria al semiespacio del recolector: seria un derrame disfrazado
			t.derrames++
			t.bytesDerramados += n

			return nil
		}

		tam := t.pedazo
		if restante < tam {
			tam = restante
		}

		// DC-002.106: el tamano lo dice el monton. Se pide el mayor bloque servible de hasta `tam`; si ni el
		// mas chico que sirve entra la imagen, se derrama (`make` como siempre) en vez de tirar el runtime.
		if t.servible != nil {
			s := t.servible(tam)
			if s < n || s <= UmbralDelObjetoGrande {
				t.derrames++
				t.bytesDerramados += n

				return nil
			}

			tam = s
		}

		// DC-002.108: el bloque lo da el reservador si hay uno (la VRAM del PVR, en la consola). Si el
		// reservador no tenia lugar, la imagen se derrama al `make` de siempre: no se tira el runtime.
		bloque := t.nuevoBloque(tam)
		if bloque == nil {
			t.derrames++
			t.bytesDerramados += n

			return nil
		}

		t.pedazos = append(t.pedazos, bloque)
		t.usadoEn = append(t.usadoEn, 0)
		t.pedido += len(bloque)
		mejor = len(t.pedazos) - 1

		if t.pedazoMenor == 0 || len(bloque) < t.pedazoMenor {
			t.pedazoMenor = len(bloque)
		}

		if len(bloque) > t.pedazoMayor {
			t.pedazoMayor = len(bloque)
		}
	}

	desde := t.usadoEn[mejor]
	t.usadoEn[mejor] += n
	t.usado += n
	t.imagenes++

	return t.pedazos[mejor][desde : desde+n : desde+n]
}

// reiniciarTramo: el nivel cambio, la cache se descarta y el tramo se reusa desde el byte 0. El tramo NO se
// libera (en la consola el monton externo no se devuelve, ADR 0001 §1.4): se reusa.
func (mr *MapRenderer) reiniciarTramo() {
	if mr.tramo == nil {
		return
	}

	for i := range mr.tramo.usadoEn {
		mr.tramo.usadoEn[i] = 0
	}

	mr.tramo.usado = 0
	mr.tramo.imagenes = 0
	mr.tramo.derrames = 0
	mr.tramo.bytesDerramados = 0
}
