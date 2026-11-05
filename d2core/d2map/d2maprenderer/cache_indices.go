package d2maprenderer

// DC-002.101 -- LA CACHE DE TILES EN INDICES: 1 B por pixel en vez de 4.
//
// POR QUE EXISTE. La cache de tiles guarda UNA superficie RGBA por grafico distinto del nivel: en el pueblo
// el primer cuadro pide 86 superficies y 10.895.360 B (10,4 MB) con la palanca perezosa, medido en el host
// con `console/dc/medidas/render-294`. En la consola ese pedido no entra: el monton de C del build del POC
// arranca con ~5 MB de techo (DC-002.94). El dato que pide el tile ya sale en indices (8 bits) del `.dt1`
// (`DecodeTileGfxData`) y lo unico que hace falta para pintarlo es una paleta de 256 colores, asi que
// guardarlo como RGBA es CUATRO VECES lo necesario.
//
// QUE ES. Una imagen de tile en indices: el buffer de 1 B/pixel tal cual sale del decodificador, su tamano y
// la paleta del acto (compartida, no se copia por tile). La paleta NO es del tile: es la del nivel, y por eso
// `Paleta` se guarda como referencia.
//
// COMO SE DIBUJA. El destino tiene que saber pintar indices (`lienzoDeIndices`): el backend PVR ya lo sabia
// desde DC-002.70 (`RenderIndices`) y aca se le pide la variante que respeta el estado de dibujado (color y
// alfa del `PushColor`, que es como se pintan las sombras al 62,7 %). El cuadro del host del medidor
// implementa lo mismo, asi la comparacion de pixeles es entre dos caminos que hacen la misma cuenta.
//
// LO QUE NO HACE. No aplica efectos del PL2 ni filtros: la cache del mapa se pinta sin `PushEffect` en el
// camino del pueblo (las sombras usan alfa del color, no efecto) y con la luz apagada. Con la luz prendida
// (`renderShadedImage`) NO se usa el camino de indices: ahi el pixel se mezcla con el mapa de luz y hace falta
// RGBA. Por eso la palanca es ADITIVA: sin llamarla, la cache es la de siempre.

// imagenDeIndices: una imagen de tile guardada como indices de 8 bits (1 B por pixel).
type imagenDeIndices struct {
	indices []byte
	ancho   int
	alto    int
	paleta  [][4]uint8 // 256 entradas RGBA, la del acto (compartida entre todos los tiles)

	// DC-002.113b: DONDE vive la imagen y CUANDO se uso por ultima vez. Es lo que hace falta para devolverla al
	// tramo (desalojo) y para no soltar una que el cuadro todavia puede pintar.
	//
	// `contenedor` es el mismo numero de `trozoDeIndices`: -1 fuera del tramo (un `make`), >= 0 el bloque del
	// tramo. `desde` es el byte del bloque donde arranca la imagen y `len(indices)` lo que ocupa.
	clave        uint32
	contenedor   int
	desde        int
	ultimoCuadro int

	// DC-002.135: EL YADJUST DE LA IMAGEN. Para las paredes, las sombras y los techos el `YAdjust` del tile NO
	// viene del `.ds1`: lo CALCULA `generateWallCache`/`generateShadowCache` mirando los bloques del `.dt1`
	// (`tileMinY`, `RoofHeight`). Como la imagen ya lo pago al armarse, se guarda aca y el camino de la cache
	// rapida lo repone sin volver a pedir los datos del tile. Para los pisos queda en 0 y no se usa: ahi el
	// `YAdjust` es del registro del `.ds1` y cambia de casilla a casilla.
	yAjuste int
}

// lienzoDeIndices: lo que el destino del dibujado tiene que saber hacer para pintar una imagen de indices con
// el estado actual (traslacion, color y alfa). Lo implementan `pvr.Surface` y el cuadro del medidor del host.
type lienzoDeIndices interface {
	RenderIndicesTintado(indices []byte, ancho, alto int, paleta [][4]uint8)
}

// claveDeCache: la MISMA clave que usa `getImageCacheRecord` (estilo, secuencia, tipo, variante).
func claveDeCache(style, sequence byte, tileType int, randomIndex byte) uint32 {
	return uint32(style)<<24 | uint32(sequence)<<16 | uint32(tileType)<<8 | uint32(randomIndex)
}

// Bytes: lo que ocupa esta imagen en la cache (1 B por pixel, sin contar la cabecera del arreglo).
func (im *imagenDeIndices) Bytes() int {
	if im == nil {
		return 0
	}

	return len(im.indices)
}

// EstadoDeLaCache: el censo de la cache de tiles, sea RGBA o de indices. Es el numero que decide si el camino
// de dibujo entra en el monton de la consola.
type EstadoDeLaCache struct {
	EnIndices   bool
	Superficies int
	Bytes       int // lo que ocupan las imagenes (RGBA: 4 B/px; indices: 1 B/px + el mapa)
	Pixeles     int // pixeles totales de la cache (lo que costaria en indices: Pixeles B)
}

// EstadoDeLaCache devuelve el censo de la cache de tiles armada hasta ahora.
func (mr *MapRenderer) EstadoDeLaCache() EstadoDeLaCache {
	e := EstadoDeLaCache{EnIndices: mr.cacheEnIndices}

	if mr.cacheEnIndices {
		for _, im := range mr.indicesCacheRecords {
			e.Superficies++
			e.Bytes += im.Bytes()
			e.Pixeles += im.ancho * im.alto
		}

		return e
	}

	for _, s := range mr.imageCacheRecords {
		if s == nil {
			continue
		}

		ancho, alto := s.GetSize()
		e.Superficies++
		e.Bytes += ancho * alto * 4
		e.Pixeles += ancho * alto
	}

	return e
}

// getCacheDeIndices: la imagen de indices ya armada, o nil.
//
// DC-002.113b: al que la PIDE se le anota el cuadro, que es lo que hace que el desalojo sepa cuales estan frias.
// Y si la imagen se acaba de componer AL VUELO (no esta en la cache porque su buffer se reusa) se devuelve el
// slot del cuadro: el dibujado la pide UNA vez, en el acto, y no queda guardada en ningun lado.
func (mr *MapRenderer) getCacheDeIndices(style, sequence byte, tileType int, randomIndex byte) *imagenDeIndices {
	k := claveDeCache(style, sequence, tileType, randomIndex)

	if im := mr.indicesCacheRecords[k]; im != nil {
		im.ultimoCuadro = mr.cuadroDeLaCache

		return im
	}

	if im := mr.recogidaAlVuelo; im != nil && im.clave == k && im.ultimoCuadro == mr.cuadroDeLaCache {
		return im
	}

	return nil
}

func (mr *MapRenderer) setCacheDeIndices(style, sequence byte, tileType int, randomIndex byte, im *imagenDeIndices) {
	k := claveDeCache(style, sequence, tileType, randomIndex)
	im.clave = k
	im.ultimoCuadro = mr.cuadroDeLaCache

	// DC-002.135: una imagen COMPUESTA es un fallo de cache. Si en un tramo de 30 cuadros hay 242 tiles por
	// cuadro y las imagenes compuestas son decenas, la cache no se rearma por cuadro (hipotesis (a) de la
	// tarjeta, medida).
	contarImagen135()

	// DC-002.113b: la imagen que se compuso AL VUELO no entra en la cache (el buffer de trabajo se reusa en el
	// mismo cuadro): se deja en el slot del cuadro para que el dibujado la encuentre y no se guarda.
	if im.contenedor == contenedorAlVuelo {
		mr.recogidaAlVuelo = im

		return
	}

	if mr.indicesCacheRecords == nil {
		mr.indicesCacheRecords = make(map[uint32]*imagenDeIndices)
	}

	mr.indicesCacheRecords[k] = im
}

// desalojarDeLaCache (DC-002.113b): hace lugar para una imagen de `n` B soltando las imagenes que NO se usaron en
// los ultimos `desalojoEnCuadros` cuadros, de la mas vieja a la mas nueva. Devuelve si al final hay lugar.
//
// POR QUE POR CUADROS Y NO POR DISTANCIA A LA CAMARA. Las dos sirven y la distancia es mas cara (hay que mirar la
// posicion de cada tile del nivel para cada imagen). "No se uso en N cuadros" es exactamente lo que el dibujado
// ya sabe -- cada `getCacheDeIndices` anota el cuadro -- y con la camara siguiendo el que se fue de pantalla es,
// justo, el que no se volvio a pedir. Medido en Flycast: ver LEEME de la tarjeta.
//
// NO HACE FALTA PROTEGER EL DIBUJADO EN CURSO: la pintura de los indices COPIA los pixeles al lienzo en el acto
// (`pvr.Surface.RenderIndicesTintado`), asi que una imagen ya pintada no tiene nada en vuelo. Aun asi el umbral
// N >= 1 deja un cuadro entero de margen: soltar algo que se pinto en ESTE cuadro seria volver a componerlo.
//
// NO RESERVA NADA: recorre el mapa buscando la mas vieja en cada vuelta (la cache del pueblo son ~110 imagenes y
// una vuelta es un `for` sobre el mapa), asi el desalojo no deja basura en el monton de Go -- que es, justo, lo
// que la tarjeta viene a arreglar.
func (mr *MapRenderer) desalojarDeLaCache(n int) bool {
	t := mr.tramo

	if t == nil || mr.desalojoEnCuadros <= 0 || n <= 0 {
		return false
	}

	// PASADA 1 -- LAS FRIAS, de la mas vieja a la mas nueva: las que no se usaron en los ultimos N cuadros.
	if mr.soltarLasMasViejasHastaQueEntre(n, mr.cuadroDeLaCache-mr.desalojoEnCuadros) {
		return true
	}

	// PASADA 2 -- AGRANDAR EL HUECO MAS GRANDE con su vecino (el unico que lo puede coser).
	if mr.agrandarLosHuecosHastaQueEntre(n) {
		return true
	}

	// PASADA 3 -- ULTIMA VUELTA: no queda ninguna fria y ningun hueco se puede agrandar. Se sueltan las que no
	// se pidieron en ESTE cuadro (las de este cuadro las suelta solo la pasada 2, si no hay otra).
	return mr.soltarLasMasViejasHastaQueEntre(n, mr.cuadroDeLaCache-1)
}

// soltarLasMasViejasHastaQueEntre: suelta, de la mas vieja a la mas nueva, las imagenes que no se usaron desde el
// cuadro `limite` (inclusive) hasta que una imagen de `n` B entre en lo que el tramo ya tiene. Devuelve si entro.
//
// NO RESERVA NADA: recorre el mapa buscando la mas vieja en cada vuelta (la cache del pueblo son ~200 imagenes y
// una vuelta es un `for` sobre el mapa), asi el desalojo no deja basura en el monton de Go -- que es, justo, lo
// que la tarjeta viene a arreglar.
func (mr *MapRenderer) soltarLasMasViejasHastaQueEntre(n, limite int) bool {
	t := mr.tramo

	for {
		if t.hayLugarPara(n) {
			return true
		}

		var (
			masViejaClave uint32
			masVieja      *imagenDeIndices
			hay           bool
		)

		for k, im := range mr.indicesCacheRecords {
			if im == nil || im.ultimoCuadro > limite {
				continue
			}

			if !hay || im.ultimoCuadro < masVieja.ultimoCuadro {
				masViejaClave, masVieja, hay = k, im, true
			}
		}

		if !hay {
			return false
		}

		mr.soltarImagenDeLaCache(masViejaClave, masVieja)
	}
}

// agrandarLosHuecosHastaQueEntre: mientras no entre, se suelta el VECINO del hueco mas grande para coserlo con el
// de al lado. Es lo que hace que una imagen grande (76.800 B, la mas grande de la cache del pueblo) entre en un
// tramo lleno de huecos chicos de imagenes comunes (20.480 B).
func (mr *MapRenderer) agrandarLosHuecosHastaQueEntre(n int) bool {
	t := mr.tramo

	for {
		if t.hayLugarPara(n) {
			return true
		}

		mayor := t.huecoMayor()
		if mayor < 0 {
			return false
		}

		vecinaClave, vecina, hayVecina := mr.vecinaDelHueco(t.huecos[mayor])
		if !hayVecina {
			return false
		}

		mr.soltarImagenDeLaCache(vecinaClave, vecina)
		t.agrandamientos++
	}
}

// huecoMayor: el indice del hueco mas grande del tramo (-1 si no hay ninguno).
func (t *TramoDeLaCache) huecoMayor() int {
	mejor := -1

	for i := range t.huecos {
		if mejor < 0 || t.huecos[i].Tamano() > t.huecos[mejor].Tamano() {
			mejor = i
		}
	}

	return mejor
}

// vecinaDelHueco: la imagen de la cache que EMPIEZA justo donde termina el hueco (o TERMINA justo donde arranca),
// que es la unica que puede agrandarlo -- los pedazos de una imagen son contiguos, asi que dos huecos solo se
// cosen si el pedazo de entre medio se va.
//
// DOS VUELTAS, y el orden importa: primero se busca una vecina que NO se haya pedido en el cuadro en curso (la
// caliente de antes); solo si no hay ninguna se acepta una del cuadro en curso. Soltar una del cuadro en curso es
// SEGURO -- la pintura de los indices COPIA los pixeles al lienzo en el acto (`RenderIndicesTintado` escribe el
// RGBA ahi mismo, no deja nada en vuelo) y el que la vuelva a pedir la recompone igual -- pero cuesta una
// decodificacion, asi que se paga solo cuando no hay otra: es lo que separa "la imagen grande se derrama" de
// "entra" (medido en Flycast: el derrame unico de 76.800 B de DC-002.113b desaparece con esta vuelta).
func (mr *MapRenderer) vecinaDelHueco(h hueco) (uint32, *imagenDeIndices, bool) {
	var (
		calienteClave uint32
		caliente      *imagenDeIndices
		hayCaliente   bool
	)

	for k, im := range mr.indicesCacheRecords {
		if im == nil || im.contenedor != h.contenedor {
			continue
		}

		fin := im.desde + len(im.indices)
		if im.desde != h.hasta && fin != h.desde {
			continue
		}

		if im.ultimoCuadro < mr.cuadroDeLaCache {
			return k, im, true
		}

		if !hayCaliente {
			calienteClave, caliente, hayCaliente = k, im, true
		}
	}

	return calienteClave, caliente, hayCaliente
}

// soltarImagenDeLaCache: saca la imagen del mapa y DEVUELVE sus bytes al tramo (el hueco se cose con sus vecinos).
//
// Lo que se suelta se vuelve a componer si el dibujado lo pide otra vez, y da los MISMOS pixeles: el
// decodificador del `.dt1` es determinista y la paleta es la del nivel. Por eso el crc32 del cuadro no cambia
// por desalojar (es lo que mide la tarjeta).
func (mr *MapRenderer) soltarImagenDeLaCache(k uint32, im *imagenDeIndices) {
	if im == nil {
		return
	}

	delete(mr.indicesCacheRecords, k)

	t := mr.tramo
	if t == nil || im.contenedor < 0 {
		return
	}

	t.devolverHueco(im.contenedor, im.desde, im.desde+len(im.indices))
	t.usado -= len(im.indices)
	t.desalojos++
	t.bytesDesalojados += len(im.indices)
}

// bufferAlVuelo (DC-002.113b): el buffer de trabajo para una imagen de `n` B que NO se cachea.
//
// ES UNO SOLO Y SE PONE EN CERO ANTES DE CADA USO. El decodificador del `.dt1` escribe solo donde el tile pinta y
// el indice 0 es el transparente, asi que el buffer tiene que llegar en cero igual que un `make`. Y es uno solo
// porque la imagen se pinta EN EL ACTO (`RenderIndicesTintado` copia al lienzo): no hay dos al vuelo a la vez.
//
// DC-002.159i -- NO CRECE MAS ALLA DE LO DECLARADO. `ActivarComposicionAlVuelo(maxBytes)` declara el buffer de
// trabajo, y antes este metodo lo AGRANDABA (`n + n/4`) cuando la imagen no entraba: con una imagen de 4,7 MB
// eso es un pedido de 5,9 MB que en la consola no entra y mata la corrida (medido en DC-002.159h). Ahora, si la
// imagen pasa el buffer declarado, NO hay buffer al vuelo: se devuelve nil, se cuenta (`vuelosRechazados`) y el
// que llama sigue con la politica de siempre (el derrame, que con el techo por imagen tampoco puede ser enorme).
func (mr *MapRenderer) bufferAlVuelo(n int) []byte {
	if n <= 0 {
		return nil
	}

	if mr.topeDelVuelo > 0 && n > mr.topeDelVuelo {
		mr.vuelosRechazados++

		return nil
	}

	if len(mr.bufferDelVuelo) < n {
		// se pide de a un cuarto mas para no crecer en cada imagen distinta
		tam := n + n/4
		mr.bufferDelVuelo = make([]byte, tam)
		mr.crecimientosDelBuffer++
	}

	b := mr.bufferDelVuelo[:n:n]

	for i := range b {
		b[i] = 0
	}

	return b
}

// TamanosDeLaCache devuelve el tamano de CADA imagen de la cache en indices, uno por entrada (DC-002.103).
//
// POR QUE EXISTE. `EstadoDeLaCache` da el total, y el total no dice DONDE puede vivir la cache: en libgodc
// toda asignacion de mas de 64 KB (`GC_LARGE_OBJECT_THRESHOLD_KB`, libgodc/runtime/gc_heap.c) se va al monton
// de C y el recolector no la toca, mientras que las de menos van al SEMIESPACIO (1.792 KB en el vendor de
// DC-002.102) y hay que copiarlas en cada recoleccion. Con el total solo no se puede decidir si la cache
// entra en el semiespacio; con la lista de tamanos si.
//
// Es un INSTRUMENTO: solo lo lee el medidor del host. La consola no lo llama (una lista de 86 enteros no
// entra gratis en el monton, y alli el numero que importa es el que ya da `EstadoDeLaCache`).
func (mr *MapRenderer) TamanosDeLaCache() []int {
	tamanos := make([]int, 0, len(mr.indicesCacheRecords))

	for _, im := range mr.indicesCacheRecords {
		tamanos = append(tamanos, im.Bytes())
	}

	return tamanos
}

// HuellaDeLaCache (DC-002.108): un FNV-1a de los pixeles de la cache en indices -- clave, tamano y bytes de
// cada imagen, por clave ORDENADA, para que dos corridas den el mismo numero.
//
// POR QUE EXISTE. Cuando la cache se muda de lugar (del monton de C a la VRAM del PVR) hay dos preguntas
// distintas y el crc32 del cuadro no las separa: ¿son los MISMOS pixeles? y ¿los lee bien el dibujado? Con la
// huella se responde la primera sola: si dos corridas dan la misma huella, la cache es la misma byte a byte
// aunque uno de los dos haya escrito en VRAM; lo que cambie despues es del camino de dibujado.
//
// ES A MANO Y SIN IMPORTS (FNV-1a y una ordenacion por insercion): este paquete entra al binario de la consola
// y no vale la pena arrastrar `hash/crc32` y `sort` a su cierre por un instrumento.
func (mr *MapRenderer) HuellaDeLaCache() (uint32, int, int) {
	claves := make([]uint32, 0, len(mr.indicesCacheRecords))

	for k := range mr.indicesCacheRecords {
		claves = append(claves, k)
	}

	// ordenacion por insercion: son 86 claves en el pueblo
	for i := 1; i < len(claves); i++ {
		for j := i; j > 0 && claves[j] < claves[j-1]; j-- {
			claves[j], claves[j-1] = claves[j-1], claves[j]
		}
	}

	const (
		fnvBase  = 2166136261
		fnvPrimo = 16777619
	)

	h := uint32(fnvBase)
	bytes := 0

	mezclar := func(b byte) {
		h ^= uint32(b)
		h *= fnvPrimo
	}

	for _, k := range claves {
		im := mr.indicesCacheRecords[k]
		if im == nil {
			continue
		}

		for s := 0; s < 4; s++ {
			mezclar(byte(k >> (8 * s)))
		}

		n := len(im.indices)
		for s := 0; s < 4; s++ {
			mezclar(byte(n >> (8 * s)))
		}

		for _, b := range im.indices {
			mezclar(b)
		}

		bytes += n
	}

	return h, len(claves), bytes
}

// dibujarImagenDeIndices pinta una imagen de indices en el destino, si el destino sabe hacerlo.
func (mr *MapRenderer) dibujarImagenDeIndices(target interface{}, im *imagenDeIndices) {
	lienzo, ok := target.(lienzoDeIndices)
	if !ok {
		mr.Warningf("la Surface del cuadro no sabe dibujar indices (DC-002.101)")

		return
	}

	lienzo.RenderIndicesTintado(im.indices, im.ancho, im.alto, im.paleta)
}
