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
func (mr *MapRenderer) getCacheDeIndices(style, sequence byte, tileType int, randomIndex byte) *imagenDeIndices {
	if mr.indicesCacheRecords == nil {
		return nil
	}

	return mr.indicesCacheRecords[claveDeCache(style, sequence, tileType, randomIndex)]
}

func (mr *MapRenderer) setCacheDeIndices(style, sequence byte, tileType int, randomIndex byte, im *imagenDeIndices) {
	if mr.indicesCacheRecords == nil {
		mr.indicesCacheRecords = make(map[uint32]*imagenDeIndices)
	}

	mr.indicesCacheRecords[claveDeCache(style, sequence, tileType, randomIndex)] = im
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
