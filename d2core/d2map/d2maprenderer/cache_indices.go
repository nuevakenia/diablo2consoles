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

// dibujarImagenDeIndices pinta una imagen de indices en el destino, si el destino sabe hacerlo.
func (mr *MapRenderer) dibujarImagenDeIndices(target interface{}, im *imagenDeIndices) {
	lienzo, ok := target.(lienzoDeIndices)
	if !ok {
		mr.Warningf("la Surface del cuadro no sabe dibujar indices (DC-002.101)")

		return
	}

	lienzo.RenderIndicesTintado(im.indices, im.ancho, im.alto, im.paleta)
}
