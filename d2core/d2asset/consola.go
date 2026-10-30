package d2asset

// DC-002.76 -- EL ASSETMANAGER DE LA CONSOLA, CON SUS CACHES A LA VISTA.
//
// POR QUE EXISTE. El POC del cooperativo (console/dc/poc/coop) tiene que medir CUANTO CUESTA CADA
// JUGADOR con el motor de verdad: los jugadores de `d2mapentity` sobre un `AssetManager` real, con los
// caches reales y el COF real del MPQ. Para eso hacen falta dos cosas que `NewAssetManager` no da:
//
//  1. LAS FUENTES. `NewAssetManager` crea el loader sin fuentes: en la PC las agrega `d2app` desde la
//     configuracion. En la consola los dos MPQ estan en el disco de la imagen (`/cd/JUEGO/...`), asi
//     que el que arranca tiene que poder decir "estas son las fuentes".
//  2. LOS CACHES. El numero que mide la tarjeta (desalojos, peso en cada cache) vive en los ocho
//     caches, que son campos NO exportados. Se devuelven como `*CachesDeAssets` -- el mismo tipo que
//     usa `NuevosCachesDeAssets` -- para que quien mida los lea sin tocar el motor ni duplicarlo.
//
// NO CAMBIA EL AssetManager: la estructura queda igual y `NewAssetManager` sigue siendo el camino de la
// PC. Esta es una funcion NUEVA que arma el mismo objeto con las fuentes y devuelve los caches.
//
// El llamador tiene que haber llamado a `AjustarPresupuestosDeConsola()` ANTES (los presupuestos se
// congelan al crear los caches): es el mismo contrato que documenta presupuestos.go.
//
// DC-002.80: aca tambien se prende el camino de tiles de la consola (`ActivarCaminoDeTilesDeConsola`). Si
// hay un AssetManager de consola, la carga de un .dt1 tiene que ir por el arena con la lectura prestada: se
// midio en Flycast y la ruta cierra (tres fases, `pico(c) - pico(a) = 0`, crc32 3507647456, con el
// recolector normal y con el recolector frenado). Prenderlo ACA y no en `AjustarPresupuestosDeConsola` deja
// el contrato claro: la ruta va con el AssetManager, no con los numeros de los presupuestos.

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2tbl"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// NuevoAssetManagerDeConsola arma el AssetManager de la consola: los ocho caches con los presupuestos
// vigentes y una fuente MPQ por cada ruta que se le pasa (en el orden en que las recibe: el loader
// prueba las fuentes de izquierda a derecha). Devuelve tambien los caches para poder medirlos.
func NuevoAssetManagerDeConsola(nivelDeLog d2util.LogLevel, rutasMPQ ...string) (*AssetManager, *CachesDeAssets, error) {
	loader, err := d2loader.NewLoader(nivelDeLog)
	if err != nil {
		return nil, nil, err
	}

	for _, ruta := range rutasMPQ {
		if err := loader.AddSource(ruta, types.AssetSourceMPQ); err != nil {
			return nil, nil, err
		}
	}

	records, err := d2records.NewRecordManager(nivelDeLog)
	if err != nil {
		return nil, nil, err
	}

	logger := d2util.NewLogger()
	logger.SetPrefix(logPrefix)
	logger.SetLevel(nivelDeLog)

	caches := NuevosCachesDeAssets()

	// DC-002.80: con AssetManager de consola, los .dt1 se cargan por el arena y se leen con memoria
	// prestada. Medido: la ruta cierra (ver el comentario de arriba).
	ActivarCaminoDeTilesDeConsola()

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

	return manager, caches, nil
}
