package d2mapstamp

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
)

const logPrefix = "Map Stamp"

// NewStampFactory creates a MapStamp factory instance
func NewStampFactory(asset *d2asset.AssetManager, l d2util.LogLevel, entity *d2mapentity.MapEntityFactory) *StampFactory {
	result := &StampFactory{
		asset:  asset,
		entity: entity,
		rng:    d2rand.New(0),
	}

	result.Logger = d2util.NewLogger()
	result.Logger.SetLevel(l)
	result.Logger.SetPrefix(logPrefix)

	return result
}

// StampFactory is responsible for loading map stamps. A stamp can be thought of like a
// preset map configuration, like the various configurations of Act 1 town.
type StampFactory struct {
	asset  *d2asset.AssetManager
	entity *d2mapentity.MapEntityFactory
	rng    *d2rand.Seed // picks among a preset's files; reseeded with the map seed

	*d2util.Logger
}

// Reseed restarts the file-picking stream; the map engine calls it with the map seed.
func (f *StampFactory) Reseed(seed int64) {
	f.rng.Init(uint32(seed))
}

// LoadStamp loads the Stamp data from file, using the given level type, level preset index, and
// level file index.
func (f *StampFactory) LoadStamp(levelType d2enum.RegionIdType, levelPreset, fileIndex int) *Stamp {
	preset := f.asset.Records.Level.Presets[levelPreset]

	var levelFilesToPick []string

	for _, fileRecord := range preset.Files {
		if fileRecord != "" && fileRecord != "0" {
			levelFilesToPick = append(levelFilesToPick, fileRecord)
		}
	}

	levelIndex := fileIndex
	if fileIndex < 0 || fileIndex >= len(levelFilesToPick) {
		levelIndex = int(f.rng.Roll(int32(len(levelFilesToPick))))
	}

	if levelFilesToPick == nil {
		panic("no level files to pick from")
	}

	return f.LoadStampPath(levelType, levelPreset, levelFilesToPick[levelIndex])
}

// LoadStampPath is LoadStamp for an explicit DS1 file (relative to
// data/global/tiles). The DRLG outdoor generator uses it because the compiled
// lvlprest.bin lists preset files the txt-based records lack.
func (f *StampFactory) LoadStampPath(levelType d2enum.RegionIdType, levelPreset int, path string) *Stamp {
	stamp := &Stamp{
		factory:     f,
		entity:      f.entity,
		regionID:    levelType,
		levelType:   *f.asset.Records.Level.Types[levelType],
		levelPreset: f.asset.Records.Level.Presets[levelPreset],
	}

	// DC-002.88: en la consola los `.dt1` NO se cargan aca. `stamp.tiles` solo lo lee `Stamp.TileData`, que
	// no tiene un solo llamador en todo el repo (`grep -rn 'TileData('`), y cargar los 32 campos del level
	// type para llenarlo pedia 603 tiles (164.016 B) y metia al arena dos archivos que el `.ds1` no lista.
	// El nivel los carga en `ResetMap`, que es donde se arman los tiles del mapa (`dt1TileData`).
	if !d2asset.SoloLosTilesDelDS1() {
		for _, levelTypeDt1 := range &stamp.levelType.Files {
			if levelTypeDt1 == "" || levelTypeDt1 == "0" {
				continue
			}

			dt1, err := f.asset.LoadDT1(levelTypeDt1)
			if err != nil {
				f.Error(err.Error())
				return nil
			}

			stamp.tiles = append(stamp.tiles, dt1.Tiles...)
		}
	}

	stamp.regionPath = path
	fileData, err := f.asset.LoadFile("/data/global/tiles/" + stamp.regionPath)

	if err != nil {
		panic(err)
	}

	stamp.ds1, err = d2ds1.Unmarshal(fileData)
	if err != nil {
		f.Error(err.Error())
		return nil
	}

	return stamp
}

// TilesDelDS1 (DC-002.88) devuelve los `.dt1` que el `.ds1` de este stamp lista, con el nombre que usa el
// motor (minusculas, sin la ruta de instalacion del juego, `.tg1` como `.dt1`). Es la MISMA traduccion que
// hace `MapEngine.AddDS1`, y es la lista con la que la consola dimensiona el arena y carga los tiles.
func (mr *Stamp) TilesDelDS1() []string {
	if mr.ds1 == nil {
		return nil
	}

	rutas := make([]string, 0, len(mr.ds1.Files))
	vistas := make(map[string]bool, len(mr.ds1.Files))

	for idx := range mr.ds1.Files {
		ruta := normalizarRutaDT1(mr.ds1.Files[idx])
		if ruta == "" || vistas[ruta] {
			continue
		}

		vistas[ruta] = true
		rutas = append(rutas, ruta)
	}

	return rutas
}

// normalizarRutaDT1: el `.ds1` guarda la ruta del DISCO de instalacion del juego (medido en DC-002.77):
//
//	\d2\data\global	iles\act1	own\floor.tg1   ->   act1/town/floor.dt1
//
// Se baja a minusculas, se le saca el prefijo de la instalacion, la extension `.tg1` pasa a `.dt1` y las
// barras invertidas a barras. Es la misma cuenta que `MapEngine.AddDS1` hace hoy, movida a una funcion para
// que la usen los dos.
func normalizarRutaDT1(nombre string) string {
	nombre = strings.ToLower(nombre)
	nombre = strings.ReplaceAll(nombre, "c:", "")
	nombre = strings.ReplaceAll(nombre, ".tg1", ".dt1")
	nombre = strings.ReplaceAll(nombre, barraInvertida+"d2"+barraInvertida+"data"+barraInvertida+
		"global"+barraInvertida+"tiles"+barraInvertida, "")
	nombre = strings.ReplaceAll(nombre, "data"+barraInvertida+"global"+barraInvertida+"tiles"+
		barraInvertida, "")
	nombre = strings.ReplaceAll(nombre, barraInvertida, "/")

	return strings.TrimPrefix(nombre, "/")
}

// barraInvertida: el separador de path del MPQ, armado por su codigo (92) para no depender del escape.
const barraInvertida = string(rune(92))

// ClavesDeTiles (DC-002.88) devuelve las `(estilo, secuencia, tipo)` que este `.ds1` referencia de verdad, en
// sus capas de piso y de pared, EMPAQUETADAS en un `int64` y ORDENADAS. Es lo que necesita el arena de bloques
// usados: de los 507 tiles de los 8 `.dt1` que el pueblo lista, sus casillas solo referencian 311 (2.586.489 B
// de bloque contra 4.360.271).
//
// POR QUE UNA LISTA ORDENADA Y NO UN `map[[3]int32]bool`. Medido en la consola de esta tarjeta: un mapa con
// `make(map[[3]int32]bool, 57*41)` mata la SH-4 en `runtime.mapassign` (illegal instruction) al primer
// `claves[k] = true`. Los mapas del resto del POC (los caches de d2asset, con hint 0) si funcionan: lo que
// rompe es el mapa pedido de una vez con un hint grande. Una lista ordenada + busqueda binaria no toca el
// recolector ni el `mapassign`.
//
// OJO: NO mira las capas de sombra ni las de sustitucion; el compositor del motor arma `MapTile.Components`
// con piso y pared (ver `MapEngine.PlaceStampClippedWhere`), que es lo que se pinta.
//
// Devuelve `nil` si alguna clave no entra en 21 bits por campo (no pasa con los `.dt1` del juego: los estilos
// llegan a unos miles y las secuencias a decenas): ahi el que llama tiene que dejar de filtrar, porque una
// clave truncada se comeria tiles buenos.
func (mr *Stamp) ClavesDeTiles() []int64 {
	if mr.ds1 == nil {
		return nil
	}

	ancho, alto := mr.ds1.Width(), mr.ds1.Height()
	claves := make([]int64, 0, ancho*alto)
	noEntra := false

	agregar := func(t *d2ds1.Tile) {
		if t == nil {
			return
		}

		claves = append(claves, d2asset.ClaveDeTile(int64(t.Style), int64(t.Sequence), int64(t.Type)))
	}

	for capa := 0; capa < 2; capa++ {
		if mr.ds1.GetFloor(capa) == nil {
			continue
		}

		for y := 0; y < alto; y++ {
			for x := 0; x < ancho; x++ {
				agregar(mr.ds1.GetFloor(capa).Tile(x, y))
			}
		}
	}

	for capa := 0; capa < 4; capa++ {
		if mr.ds1.GetWall(capa) == nil {
			continue
		}

		for y := 0; y < alto; y++ {
			for x := 0; x < ancho; x++ {
				agregar(mr.ds1.GetWall(capa).Tile(x, y))
			}
		}
	}

	for i := range claves {
		if claves[i] < 0 {
			noEntra = true
		}
	}

	if noEntra {
		return nil
	}

	d2asset.OrdenarClavesDeTiles(claves)

	return claves
}
