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
//
// DC-002.81: y desde aca se llama a `ArrancarComoConsola`, que es el arranque UNICO de la consola (ver
// abajo): asi el AssetManager de consola y el arranque del motor prenden exactamente las mismas palancas.

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2tbl"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// ArrancarComoConsola es EL ARRANQUE DE LA CONSOLA EN UNA SOLA LLAMADA (DC-002.81).
//
// POR QUE EXISTE. El `main` del motor no arma el AssetManager: lo arma `d2app` (`d2app/app.go:134`) con
// `NewAssetManager`, que es el camino de la PC, y en la consola eso deja los presupuestos de PC (128 MB de
// animaciones, 1 GB por cache de datos, peso 1 por entrada) y las TRES palancas apagadas. Las tres vivian en
// tres funciones sueltas que cada POC se acordaba de llamar por su cuenta:
//
//  1. `AjustarPresupuestosDeConsola` (DC-002.75) -- los caches acotados y pesando en bytes.
//  2. `ActivarLecturaDeConsola`     (DC-002.79) -- `LoadFile` reserva el tamano exacto en vez de `ReadAll`,
//     y apaga el prefetch del MPQ (hasta 2 MB por archivo). Sin esta, el motor se cae leyendo el animdata o
//     cualquier archivo de mas de 64 KB.
//  3. `ActivarCaminoDeTilesDeConsola` (DC-002.78/.80) -- cada .dt1 se lee ADENTRO de un arena pedido UNA
//     vez, con la lectura del MPQ por memoria prestada (`lecturaPrestadaDeTiles`).
//  4. `ActivarLecturasPorTramos`   (DC-002.99) -- `LoadFile` deja de pedir un buffer por archivo: lee en un
//     tramo de la fuente y lo devuelve con `SoltarLecturasDeConsola` (el ultimo `make` por archivo del motor).
//
// ACA quedan juntas, para que el arranque de la consola sea UNA linea y no tres que se pueden olvidar (que es
// justo lo que esta tarjeta viene a comprobar: que nada se pierda al arrancar el motor entero). La PC no la
// llama NUNCA: `NewAssetManager`, el camino del host, queda exactamente como estaba.
func ArrancarComoConsola() {
	AjustarPresupuestosDeConsola()
	ActivarLecturaDeConsola()
	ActivarCaminoDeTilesDeConsola()

	// DC-002.88: el nivel carga solo los `.dt1` que su `.ds1` lista (ver `lectura_de_tiles.go`). El
	// compositor del motor (`LoadStamp` + `ResetMap`) cargaba los 32 campos del level type y el pueblo tiene
	// dos `.dt1` ahi que el `.ds1` no lista: 960.096 B de arena que ninguna casilla mira.
	CargarSoloLosTilesDelDS1(true)

	// DC-002.99: la lectura del motor deja de pedir un buffer por archivo. `LoadFile` lee en UN tramo de la
	// fuente (`console/dc/fuenteconsola`, que lo saca del monton de C con `free` inyectado) y lo devuelve con
	// `SoltarLecturasDeConsola`. OJO CON EL CONTRATO: con esta palanca prendida, lo que devuelve `LoadFile`
	// vale hasta la proxima lectura (es el precio de no pedir memoria; ver la cabecera de lectura_consola.go).
	ActivarLecturasPorTramos()

	// DC-002.159b: las direcciones de los `.dcc` de personaje se decodifican cuando se dibujan, no todas al cargar.
	// Un modo del heroe son 7 capas y las 16 direcciones de la capa `LA` de la amazona son 419.184 B de pixeles
	// descomprimidos (medido en el Gate 0): con el mapa compuesto no entran en el semiespacio. Ver
	// dcc_animation.go.
	UsarDireccionesPerezosas(true)

	// DC-002.159b: las sombras del compuesto se apagan. El camino de la sombra (escala + sesgo + brillo cero) en el
	// renderer del PVR mata el SH-4 en el primer cuadro del heroe; medido y localizado en esta tarjeta (ver
	// `sombrasDelCompuesto` en composite.go y la evidencia de DC-002.159b). El ente se dibuja igual, sin la mancha
	// del piso. Cuando el camino de la sombra este arreglado, se saca esta linea.
	UsarSombrasDelCompuesto(false)
}

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

	// DC-002.81: el arranque de consola de UNA sola llamada (presupuestos + lectura completa + arena de
	// tiles). Antes aca se prendia solo la ruta del arena; la lectura con tamano reservado (DC-002.79) se
	// quedaba afuera, o sea que este constructor NO dejaba el AssetManager completo para el motor.
	ArrancarComoConsola()

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
