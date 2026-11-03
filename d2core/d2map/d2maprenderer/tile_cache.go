package d2maprenderer

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
)

const (
	tileMinHeight          int16 = 32
	shadowAdjustY          int32 = 80
	defaultFloorTileWidth        = 10
	defaultFloorTileHeight       = 10
)

const (
	blockOffsetY      = 32
	tileSurfaceWidth  = 160
	tileSurfaceHeight = 80
)

// resetLevelCaches drops everything cached for the previous level.
func (mr *MapRenderer) resetLevelCaches() {
	// The cached tile images are keyed by (style, sequence, type, variant) only, and the same key means a
	// different graphic in every level type and palette. Without this reset a level change kept the
	// previous level's images: Lut Gholein drew Rogue Encampment grass on the tiles both share
	// (Act-1 looking green tiles in later acts).
	mr.imageCacheRecords = nil
	// DC-002.101: la cache de indices y la paleta expandida son del mismo nivel (la paleta cambia por acto).
	mr.indicesCacheRecords = nil
	mr.paletaDeIndices = nil
	// DC-002.113b: el slot de la imagen compuesta al vuelo es de un cuadro, y el cuadro se fue con el nivel
	mr.recogidaAlVuelo = nil
	// DC-002.104: el tramo de la cache tampoco se libera (en la consola el monton externo no se devuelve): se
	// reusa desde el byte 0, que es lo mismo que decir que el nivel nuevo pide sus imagenes de nuevo.
	mr.reiniciarTramo()
	mr.blankShadows = nil
	mr.blankWalls = nil
	mr.light.resetFades()
}

// paletaEnRGBA: la paleta del nivel expandida a [][4]uint8, UNA vez, para las imagenes en indices. Es la
// misma paleta que usa `ImgIndexToRGBA` (R,G,B,A de cada color), asi el camino de indices pinta los mismos
// bytes que el de RGBA.
func (mr *MapRenderer) paletaEnRGBA() [][4]uint8 {
	if mr.paletaDeIndices != nil {
		return mr.paletaDeIndices
	}

	pal := make([][4]uint8, 256)

	for i := 0; i < 256; i++ {
		c, err := mr.palette.GetColor(i)
		if err != nil || c == nil {
			continue
		}

		pal[i] = [4]uint8{c.R(), c.G(), c.B(), c.A()}
	}

	mr.paletaDeIndices = pal

	return pal
}

func (mr *MapRenderer) generateTileCache() {
	var err error

	mr.resetLevelCaches()
	mr.palette, err = mr.loadPaletteForAct(d2enum.RegionIdType(mr.mapEngine.LevelType().ID))

	if err != nil {
		mr.Error(err.Error())
	}

	// DC-002.94: con la cache perezosa NO se arma nada aca. La cache entera del pueblo son 277
	// superficies (29,36 MB de RGBA, medidos en el host con console/dc/medidas/render-294) y en la consola
	// no entran en 16 MB: cada imagen se arma la primera vez que se dibuja (ver asegurarImagen).
	if mr.cachePerezoso {
		return
	}

	tiles := *mr.mapEngine.Tiles()
	for idx := range tiles {
		tile := &tiles[idx]

		for i := range tile.Components.Floors {
			if !tile.Components.Floors[i].Hidden() && tile.Components.Floors[i].Prop1 != 0 {
				mr.generateFloorCache(&tile.Components.Floors[i])
			}
		}

		for i := range tile.Components.Shadows {
			if !tile.Components.Shadows[i].Hidden() && tile.Components.Shadows[i].Prop1 != 0 {
				mr.generateShadowCache(&tile.Components.Shadows[i])
			}
		}

		for i := range tile.Components.Walls {
			// special tiles (warps, entry markers) have no graphic and are never drawn
			if !tile.Components.Walls[i].Hidden() && tile.Components.Walls[i].Prop1 != 0 &&
				!tile.Components.Walls[i].Type.Special() {
				mr.generateWallCache(&tile.Components.Walls[i])
			}
		}
	}
}

func (mr *MapRenderer) generateFloorCache(tile *d2ds1.Tile) {
	// DC-002.135 -- LA CACHE RAPIDA: SI LA IMAGEN YA ESTA, NO SE LE PIDE NADA AL MOTOR.
	//
	// MEDIDO (evidencia de la tarjeta): con la cache ya armada y CERO imagenes nuevas, este bloque era el que
	// mas pesaba del mapa -- 187 ms de los 528 de la corrida del cronometro, 248 veces por cuadro -- y casi
	// todo era el `mapEngine.GetTiles` de abajo, que se pedia SIEMPRE, acertara o no la cache. Con la imagen en
	// la mano el `.dt1` no hace falta para nada: el `YAdjust` del piso viene del `.ds1` (no se calcula aca) y el
	// resto de la funcion es la composicion que no hay que hacer. Las flojas (lava, `tile.Animated`) siguen por
	// el camino de siempre: ahi la clave de la imagen sale de los datos del tile.
	if mr.cacheSinGetTiles && mr.cacheEnIndices && !tile.Animated {
		if mr.getCacheDeIndices(tile.Style, tile.Sequence, 0, tile.RandomIndex) != nil {
			return
		}
	}

	// DC-002.135: el pedazo `cacheTiles` del bloque de la cache (el `GetTiles` del motor).
	tTiles := ahora135()

	tileOptions := mr.mapEngine.GetTiles(int(tile.Style), int(tile.Sequence), 0)

	anotar135(BloqueCacheTiles135, tTiles)

	diagImagen("piso", tile, len(tileOptions))

	var tileData []*d2dt1.Tile

	if tileOptions == nil {
		mr.Errorf("Could not locate tile Style:%d, Seq: %d, Type: %d", tile.Style, tile.Sequence, 0)

		tileData = append(tileData, &d2dt1.Tile{})
		tileData[0].Width = defaultFloorTileWidth
		tileData[0].Height = defaultFloorTileHeight
	} else {
		if !tileOptions[0].MaterialFlags.Lava {
			tileData = append(tileData, &tileOptions[tile.RandomIndex])
		} else {
			tile.Animated = true

			if mr.animFrames == nil {
				mr.animFrames = map[uint16]int{}
			}

			mr.animFrames[uint16(tile.Style)<<8|uint16(tile.Sequence)] = len(tileOptions)
			for i := range tileOptions {
				tileData = append(tileData, &tileOptions[i])
			}
		}
	}

	var tileIndex byte

	for i := range tileData {
		if tileData[i].MaterialFlags.Lava {
			tileIndex = byte(tileData[i].RarityFrameIndex)
		} else {
			tileIndex = tile.RandomIndex
		}

		cachedImage := mr.getImageCacheRecord(tile.Style, tile.Sequence, 0, tileIndex)

		if mr.cacheEnIndices {
			// DC-002.101: cache en indices -- el "ya esta" es la entrada de indices
			if mr.getCacheDeIndices(tile.Style, tile.Sequence, 0, tileIndex) != nil {
				return
			}
		} else if cachedImage != nil {
			return
		}

		tileYMinimum := int32(0)

		for _, block := range tileData[i].Blocks {
			tileYMinimum = d2math.MinInt32(tileYMinimum, int32(block.Y))
		}

		tileYOffset := d2math.AbsInt32(tileYMinimum)
		tileHeight := d2math.AbsInt32(tileData[i].Height)

		// DC-002.113b: el trozo dice DONDE quedo la imagen, que es lo que despues permite desalojarla.
		// `cacheable` es falso para las animadas: arman varias imagenes en la MISMA llamada y el buffer al
		// vuelo es uno solo (se pisarian entre ellas antes de pintarse).
		trozo := mr.pedirTrozoDeIndices(int(tileData[i].Width*tileHeight), len(tileData) == 1)
		indexData := trozo.indices
		d2dt1.DecodeTileGfxData(tileData[i].Blocks, &indexData, tileYOffset, tileData[i].Width)

		if mr.cacheEnIndices {
			mr.setCacheDeIndices(tile.Style, tile.Sequence, 0, tileIndex, &imagenDeIndices{
				indices: indexData, ancho: int(tileData[i].Width), alto: int(tileHeight), paleta: mr.paletaEnRGBA(),
				contenedor: trozo.contenedor, desde: trozo.desde,
			})

			continue
		}

		image := mr.renderer.NewSurface(int(tileData[i].Width), int(tileHeight))
		pixels := d2util.ImgIndexToRGBA(indexData, mr.palette)

		image.ReplacePixels(pixels)

		mr.setImageCacheRecord(tile.Style, tile.Sequence, 0, tileIndex, image)
	}
}

func (mr *MapRenderer) generateShadowCache(tile *d2ds1.Tile) {
	// DC-002.135: la cache rapida (ver `generateFloorCache`). Aca el `YAdjust` SI lo calcula esta funcion, asi
	// que se repone el que quedo guardado con la imagen.
	if mr.cacheSinGetTiles && mr.cacheEnIndices {
		if im := mr.getCacheDeIndices(tile.Style, tile.Sequence, int(d2enum.TileShadow),
			tile.RandomIndex); im != nil {
			tile.YAdjust = im.yAjuste

			return
		}
	}

	// DC-002.135: el pedazo `cacheTiles` (ver `generateFloorCache`).
	tTiles := ahora135()

	tileOptions := mr.mapEngine.GetTiles(int(tile.Style), int(tile.Sequence), d2enum.TileShadow)

	anotar135(BloqueCacheTiles135, tTiles)

	diagImagen("sombra", tile, len(tileOptions))

	var tileData *d2dt1.Tile

	if tileOptions == nil {
		return
	}

	tileData = &tileOptions[tile.RandomIndex]

	if tileData.Width == 0 || tileData.Height == 0 {
		return
	}

	tileMinY := int32(0)
	tileMaxY := int32(0)

	for _, block := range tileData.Blocks {
		tileMinY = d2math.MinInt32(tileMinY, int32(block.Y))
		tileMaxY = d2math.MaxInt32(tileMaxY, int32(block.Y+tileMinHeight))
	}

	tileYOffset := -tileMinY
	tileHeight := int(tileMaxY - tileMinY)
	tile.YAdjust = int(tileMinY + shadowAdjustY)

	cachedImage := mr.getImageCacheRecord(tile.Style, tile.Sequence, d2enum.TileShadow, tile.RandomIndex)

	if mr.cacheEnIndices {
		if mr.getCacheDeIndices(tile.Style, tile.Sequence, int(d2enum.TileShadow), tile.RandomIndex) != nil {
			return
		}
	} else if cachedImage != nil {
		return
	}

	trozo := mr.pedirTrozoDeIndices(int(tileData.Width)*tileHeight, true)
	indexData := trozo.indices
	d2dt1.DecodeTileGfxData(tileData.Blocks, &indexData, tileYOffset, tileData.Width)

	if mr.cacheEnIndices {
		mr.setCacheDeIndices(tile.Style, tile.Sequence, int(d2enum.TileShadow), tile.RandomIndex, &imagenDeIndices{
			indices: indexData, ancho: int(tileData.Width), alto: tileHeight, paleta: mr.paletaEnRGBA(),
			contenedor: trozo.contenedor, desde: trozo.desde, yAjuste: tile.YAdjust,
		})

		return
	}

	image := mr.renderer.NewSurface(int(tileData.Width), tileHeight)
	pixels := d2util.ImgIndexToRGBA(indexData, mr.palette)

	image.ReplacePixels(pixels)

	mr.setImageCacheRecord(tile.Style, tile.Sequence, d2enum.TileShadow, tile.RandomIndex, image)
}

func (mr *MapRenderer) generateWallCache(tile *d2ds1.Tile) {
	// DC-002.135: la cache rapida (ver `generateFloorCache`). El `YAdjust` de la pared/techo lo calcula esta
	// funcion con los bloques del `.dt1`, asi que el camino rapido repone el que quedo guardado con la imagen:
	// el dibujado sale en el mismo sitio. Las paredes sin grafico (colision sin dibujo) ya estan en
	// `blankWalls`: esas tampoco necesitan el motor, porque el renderer las saltea antes de usar el `YAdjust`.
	if mr.cacheSinGetTiles && mr.cacheEnIndices {
		if im := mr.getCacheDeIndices(tile.Style, tile.Sequence, int(tile.Type), tile.RandomIndex); im != nil {
			tile.YAdjust = im.yAjuste

			return
		}

		if mr.blankWalls != nil && mr.blankWalls[blankWallKey(tile)] {
			return
		}
	}

	// DC-002.135: el pedazo `cacheTiles` (ver `generateFloorCache`).
	tTiles := ahora135()

	tileOptions := mr.mapEngine.GetTiles(int(tile.Style), int(tile.Sequence), tile.Type)

	anotar135(BloqueCacheTiles135, tTiles)

	diagImagen("pared", tile, len(tileOptions))

	var tileData *d2dt1.Tile

	if tileOptions == nil {
		return
	}

	tileData = &tileOptions[tile.RandomIndex]

	var newTileData *d2dt1.Tile = nil

	if tile.Type == d2enum.TileRightPartOfNorthCornerWall {
		newTileOptions := mr.mapEngine.GetTiles(
			int(tile.Style), int(tile.Sequence),
			d2enum.TileLeftPartOfNorthCornerWall,
		)
		diagImagen("pared-par-de-la-esquina", tile, len(newTileOptions))

		// DC-002.109: `GetTiles` devuelve nil cuando el `.dt1` del nivel no tiene esa variante, y aca se
		// indexaba a ciegas: era un "index out of range" esperando al nivel que no la trae (la TRANSICION del
		// pueblo cambio de nivel y murio en este punto en la consola). Sin la pieza izquierda se arma la pared
		// con la derecha, que es el mismo camino que toma el renderer cuando `newTileData` sale mas alta.
		if int(tile.RandomIndex) < len(newTileOptions) {
			newTileData = &newTileOptions[tile.RandomIndex]
		}
	}

	tileMinY := int32(0)
	tileMaxY := int32(0)

	target := tileData

	if newTileData != nil && newTileData.Height < tileData.Height {
		target = newTileData
	}

	for _, block := range target.Blocks {
		tileMinY = d2math.MinInt32(tileMinY, int32(block.Y))
		tileMaxY = d2math.MaxInt32(tileMaxY, int32(block.Y+blockOffsetY))
	}

	realHeight := d2math.MaxInt32(d2math.AbsInt32(tileData.Height), tileMaxY-tileMinY)
	tileYOffset := -tileMinY

	if tile.Type == d2enum.TileRoof {
		tile.YAdjust = -int(tileData.RoofHeight)
	} else {
		tile.YAdjust = int(tileMinY) + tileSurfaceHeight
	}

	cachedImage := mr.getImageCacheRecord(tile.Style, tile.Sequence, tile.Type, tile.RandomIndex)

	if mr.cacheEnIndices {
		if mr.getCacheDeIndices(tile.Style, tile.Sequence, int(tile.Type), tile.RandomIndex) != nil {
			return
		}
	} else if cachedImage != nil {
		return
	}

	if realHeight == 0 {
		// graphic-less collision placeholders exist in the DT1 files (e.g. barracks pillars 9/25 and 9/26,
		// type 12): they only carry sub-tile flags, so there is nothing to draw
		mr.Debugf("wall tile [%d %d %d] has no graphic", tile.Style, tile.Sequence, tile.Type)

		if mr.blankWalls == nil {
			mr.blankWalls = map[uint32]bool{}
		}

		mr.blankWalls[blankWallKey(tile)] = true
		return
	}

	trozo := mr.pedirTrozoDeIndices(int(tileSurfaceWidth*realHeight), true)

	indexData := trozo.indices

	d2dt1.DecodeTileGfxData(tileData.Blocks, &indexData, tileYOffset, tileSurfaceWidth)

	if newTileData != nil {
		d2dt1.DecodeTileGfxData(newTileData.Blocks, &indexData, tileYOffset, tileSurfaceWidth)
	}

	if mr.cacheEnIndices {
		mr.setCacheDeIndices(tile.Style, tile.Sequence, int(tile.Type), tile.RandomIndex, &imagenDeIndices{
			indices: indexData, ancho: tileSurfaceWidth, alto: int(realHeight), paleta: mr.paletaEnRGBA(),
			contenedor: trozo.contenedor, desde: trozo.desde, yAjuste: tile.YAdjust,
		})

		return
	}

	image := mr.renderer.NewSurface(tileSurfaceWidth, int(realHeight))
	pixels := d2util.ImgIndexToRGBA(indexData, mr.palette)

	image.ReplacePixels(pixels)

	mr.setImageCacheRecord(tile.Style, tile.Sequence, tile.Type, tile.RandomIndex, image)
}

// asegurarImagen (DC-002.94): con la cache perezosa, arma la imagen del tile que se va a dibujar si todavia
// no esta. Las condiciones son LAS MISMAS de generateTileCache, para que la cache acabe con lo mismo que
// tendria armada de entrada (y el cuadro sea identico).
func (mr *MapRenderer) asegurarImagen(tile *d2ds1.Tile, clase d2enum.TileType) {
	if !mr.cachePerezoso {
		return
	}

	switch clase {
	case d2enum.TileShadow:
		if !tile.Hidden() && tile.Prop1 != 0 {
			mr.generateShadowCache(tile)
		}
	case 0: // piso: el tipo 0 de la capa de pisos
		if !tile.Hidden() && tile.Prop1 != 0 {
			mr.generateFloorCache(tile)
		}
	default: // pared, techo, arbol, columna...
		if !tile.Hidden() && tile.Prop1 != 0 && !clase.Special() {
			mr.generateWallCache(tile)
		}
	}
}

// blankWallKey: la clave de una pared sin grafico (colision sin dibujo).
func blankWallKey(tile *d2ds1.Tile) uint32 {
	return uint32(tile.Style)<<24 | uint32(tile.Sequence)<<16 | uint32(tile.Type)<<8 | uint32(tile.RandomIndex)
}

// DiagnosticoDeLasImagenes (DC-002.109, INSTRUMENTO): prende el censo por imagen de la cache en el log de la
// consola. Dice, para cada imagen que se arma, la clave (estilo/secuencia/tipo/variante) y CUANTAS variantes
// devolvio el motor para esa clave: es lo que separa "el nivel pide una variante que su `.dt1` no trae" (un
// dato del nivel, y el `[variante]` de arriba se sale de rango) de "la imagen se armo y el cuadro murio
// despues". Apagado por default: sin prenderlo no se imprime una sola linea.
var DiagnosticoDeLasImagenes bool

func diagImagen(que string, tile *d2ds1.Tile, opciones int) {
	if !DiagnosticoDeLasImagenes || tile == nil {
		return
	}

	println("MAPA103: IMAGEN", que, "estilo=", int(tile.Style), "sec=", int(tile.Sequence),
		"tipo=", int(tile.Type), "variante=", int(tile.RandomIndex), "opciones=", opciones)
}
