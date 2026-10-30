package d2asset

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2tbl"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// NewAssetManager creates and assigns all necessary dependencies for the AssetManager top-level functions to work correctly
func NewAssetManager(logLevel d2util.LogLevel) (*AssetManager, error) {
	loader, err := d2loader.NewLoader(logLevel)
	if err != nil {
		return nil, err
	}

	records, err := d2records.NewRecordManager(logLevel)
	if err != nil {
		return nil, err
	}

	logger := d2util.NewLogger()
	logger.SetPrefix(logPrefix)
	logger.SetLevel(logLevel)

	// DC-002.75: los ocho caches se crean en UN solo lugar (presupuestos.go), con los presupuestos
	// vigentes: los de PC, o los de consola si el arranque llamo a AjustarPresupuestosDeConsola.
	caches := NuevosCachesDeAssets()

	manager := &AssetManager{
		Logger:     logger,
		Loader:     loader,
		tables:     make([]d2tbl.TextDictionary, 0),
		animations: caches.Animations,
		fonts:      caches.Fonts,
		palettes:   caches.Palettes,
		transforms: caches.Transforms,
		dt1s:       caches.Dt1s,
		ds1s:       caches.Ds1s,
		cofs:       caches.Cofs,
		dccs:       caches.Dccs,
		Records:    records,
	}

	return manager, err
}
