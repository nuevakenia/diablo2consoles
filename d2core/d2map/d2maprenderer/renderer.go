package d2maprenderer

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"strconv"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2display"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2lightmap"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
)

const (
	logPrefix = "Map Renderer"
)

const (
	// extraViewTop and extraViewBottom are the pixels above and below the screen that the tile range covers
	// (tall wall and roof pieces reach into the screen from tiles that stand outside it).
	extraViewTop    = 200
	extraViewBottom = 450
	two             = 2

	dbgOffsetXY   = 40
	dbgBoxWidth   = 220
	dbgBoxHeight  = 60
	dbgBoxPadding = 10

	dbgCollisionSize    = 5
	dbgCollisionOffsetX = -3
	dbgCollisionOffsetY = 4

	whiteHalfOpacity        = 0xffffff7f
	blackQuarterOpacity     = 0x00000040
	lightGreenFullOpacity   = 0x40ff00ff
	magentaFullOpacity      = 0xff00ffff
	yellowFullOpacity       = 0xffff00ff
	lightBlueQuarterOpacity = 0x5050ff32
	whiteQuarterOpacity     = 0xffffff64
	redQuarterOpacity       = 0x74000064

	subtilesPerTile    = 5
	orthoSubTileWidth  = 16
	orthoSubTileHeight = 8
	orthoTileWidth     = subtilesPerTile * orthoSubTileWidth
	orthoTileHeight    = subtilesPerTile * orthoSubTileHeight
)

// MapRenderer manages the game viewport and Camera. It requests tile and entity data from MapEngine and renders it.
type MapRenderer struct {
	asset               *d2asset.AssetManager
	renderer            d2interface.Renderer   // Used for drawing operations
	mapEngine           *d2mapengine.MapEngine // The map engine that is being rendered
	palette             d2interface.Palette    // The palette used for this map
	viewport            *Viewport              // Used for rendering offsets
	Camera              Camera                 // Used to determine where on the map we are rendering
	imageCacheRecords   map[uint32]d2interface.Surface
	blankWalls          map[uint32]bool // wall tiles without graphics (marker DT1 files, collision placeholders)
	blankShadows        map[uint32]bool // shadow tiles without graphics (never cached, never drawn)
	mapDebugVisLevel    int             // Map debug visibility index (0=none, 1=tiles, 2=sub-tiles)
	entityDebugVisLevel int             // Entity Debug visibility index (0=none, 1=vectors)
	lastFrameTime       float64         // The last time the map was rendered
	currentFrame        int             // Current render frame (for animations)
	animTicks           int             // client world ticks (25 Hz) for animated floors, see tile_anim.go
	animAcc             float64
	animFrames          map[uint16]int // frame count of animated floor sequences by style<<8|sequence
	light               *lighting
	entBuckets          map[[2]int]*entityBucket // per-frame entity index, see indexEntities
	entFree             []*entityBucket
	subKeys             []int8                        // scratch of renderEntitiesBySubtile
	shadeVals           []color.RGBA                  // scratch for renderShadedImage
	shadeGrids          map[shadeGridKey][]shadePoint // subtile offsets of the vertices of a grid, per image size
	stats               DrawStats
	roofCover           map[[2]int]bool  // roof tiles fading out because the hero is under them
	shake               screenShake      // view shake of a Siege Beast stomp, see shake.go
	backdrops           []*backdropLayer // neighbouring levels drawn beyond the map edge, see backdrop.go
	offX, offY          int              // a backdrop renderer: tile offset to the main map (light and fade keys)

	// tamanoDelCuadro: si esta puesto, Render usa ESTE tamano en vez del de d2display (DC-002.94).
	// Existe porque el cuadro de la consola es 640x480 y d2display.Clamp sube todo a 800x600: sin esta
	// palanca el renderer hierve en un viewport que no es el de la Surface. En la PC queda en cero y el
	// camino es el de siempre (d2display.Get()).
	tamanoDelCuadro d2display.Size
	tieneTamano     bool

	// cachePerezoso: la cache de tiles se arma a medida que se dibuja, no entera al crear el renderer
	// (DC-002.94). Ver ActivarCachePerezoso.
	cachePerezoso bool

	// cacheEnIndices: la cache guarda INDICES de 8 bits (1 B/pixel) en vez de RGBA (4 B/pixel) y el dibujado
	// va por `lienzoDeIndices.RenderIndicesTintado` (DC-002.101). Aditivo: sin la palanca, todo como antes.
	cacheEnIndices      bool
	indicesCacheRecords map[uint32]*imagenDeIndices
	paletaDeIndices     [][4]uint8 // la paleta del nivel expandida para las imagenes en indices

	// tramo: de donde salen los pixeles de las imagenes de indices cuando la palanca esta puesta: UN solo
	// buffer de mas de 64 KB (en la consola, monton de C) repartido entre las imagenes (DC-002.104). Nil = la
	// cache pide sus pixeles con `make` como siempre, que es el camino de la PC.
	tramo *TramoDeLaCache

	// ============================== EL DESALOJO (DC-002.113b) ==============================
	//
	// desalojoEnCuadros: cuantos cuadros tiene que llevar una imagen SIN pedirse para que el desalojo la pueda
	// soltar cuando el tramo se llena. 0 = APAGADO, y entonces la cache es de solo agregar como en DC-002.113
	// (que es lo que la dejo crecer hasta tumbar al recolector). Es la palanca que separa esta tarjeta de la
	// anterior en el A/B: mismo binario, misma corrida, solo cambia este numero.
	desalojoEnCuadros int

	// componerAlVuelo: la otra salida cuando el tramo esta lleno -- en vez de derramar al monton de Go, la imagen
	// se compone sobre `bufferDelVuelo`, se pinta y NO se guarda. Cuesta decodificar el tile en cada cuadro que
	// lo pida, y eso es lo que se mide contra el desalojo (ms por cuadro).
	componerAlVuelo       bool
	bufferDelVuelo        []byte
	crecimientosDelBuffer int

	// topeDelVuelo (DC-002.159i): el buffer de trabajo NO crece mas alla de esto. Es el `maxBytes` que declaro
	// `ActivarComposicionAlVuelo`; 0 = sin tope (crece como siempre, el camino de la PC y de los POCs del host).
	topeDelVuelo int

	// ============================== EL TECHO POR IMAGEN (DC-002.159i) ==============================
	// Ver `limite_de_imagen_159i.go`: en la consola UNA imagen de la cache del mapa puede no entrar en la RAM
	// (medido en DC-002.159h: 4.737.760 B de indices, y el buffer al vuelo creciendo por encima). Con el techo
	// puesto, la que no entra se dice en el log UNA vez y no se pinta; el cuadro sigue. Cero = sin techo (la PC).
	limiteDeImagen int

	// noEntran: las claves de cache de las imagenes que el techo dejo afuera; el dibujado las pregunta antes de
	// pedir nada (asi el cuadro no paga ni el `GetTiles` ni un aviso por cuadro).
	noEntran map[uint32]bool

	// las cuentas del rechazo, para el log de la consola y la evidencia
	imagenesNoEntran int
	bytesNoEntran    int
	vuelosRechazados int

	// cacheSinGetTiles (DC-002.135): con la cache en indices y la imagen YA armada, el tile no necesita los
	// datos del `.dt1` para nada (ni para el `YAdjust`, que va guardado con la imagen), asi que se saltea el
	// `mapEngine.GetTiles` que `generate*Cache` pedia SIEMPRE, acertara o no la cache. Es la palanca del A/B:
	// apagada, el camino es el de DC-002.134 tal cual (y el cuadro tiene que salir identico con las dos).
	cacheSinGetTiles bool

	// cuadroDeLaCache: el numero de cuadro del dibujado (lo cuenta `Render`), que es la unidad con la que se
	// mide "esta imagen no se usa hace N cuadros".
	cuadroDeLaCache int

	// recogidaAlVuelo: la ultima imagen compuesta al vuelo, valida SOLO en el cuadro en que se compuso. Es el
	// unico puntero a `bufferDelVuelo`, y por eso no hay dos al vuelo a la vez.
	recogidaAlVuelo *imagenDeIndices

	// ============================== EL MAPA ESTATICO (DC-002.157) ==============================
	// Ver `mapa_estatico_157.go`: con la palanca puesta, el mapa NO se repinta si la vista no se movio y no
	// hay nada (tiles animados, entes, luz, shake, debug) que cambie el cuadro. Es el salto de DC-002.139,
	// que en el POC jugable dejo el mapa en 4,2 ms y en 0 los cuadros quietos.
	mapaEstatico157    bool
	mapa157Valido      bool
	mapa157X, mapa157Y int
	saltos157          int
	repintados157      int

	// ============================== LOS ENTES APARTE (DC-002.159) ==============================
	// Ver `entes_aparte_159.go`: con la palanca puesta, el renderer pinta SOLO el mapa y no indexa ni
	// dibuja entes; el que llama los dibuja encima (y los borra con la memoria del lienzo). Es lo que
	// deja que el mapa estatico siga valiendo con el heroe caminando: sin esto, un solo ente apaga el
	// salto y el cuadro vuelve a los 818 ms de DC-002.157.
	entesAparte159 bool

	// DC-002.159c (instrumento): donde quedo el origen del ultimo ente dibujado con `DibujarEnte159`, en pixeles
	// del lienzo. Tiene que coincidir con `PieDeLaPantalla` del que lo dibujo; si no coincide, el borrado limpia
	// un lugar y el sprite queda en otro (el rastro del caminar).
	dibujoX159 int
	dibujoY159 int

	// ============================== EL MAPA SOLTADO (DC-002.159b) ==============================
	// Ver `mapa_soltado_159.go`: cuando el que llama devuelve la memoria del mapa (las casillas del motor y
	// el arena de bloques de los `.dt1`), el mapa del lienzo es el unico que hay: `puedeSaltarseElMapa157`
	// devuelve verdadero siempre y el renderer no vuelve a pintar un mundo vacio encima.
	mapaSoltado159 bool

	*d2util.Logger
}

// ActivarDesalojoDeLaCache (DC-002.113b): cuando el tramo se llena, suelta las imagenes que no se usaron en los
// ultimos `cuadros` cuadros y reusa esos bytes. ADITIVA: sin llamarla, la cache es de solo agregar.
//
// POR QUE N CUADROS Y NO UN TAMANO. El tramo no se puede agrandar en la consola (la VRAM servible son ~4,7 MB y
// ya esta pedida) y lo que hay que acotar es CUANTO VIVE una imagen, no cuanta memoria hay: con N cuadros de
// gracia, lo que se fue de pantalla se puede soltar y lo que el cuadro esta pintando no. N=1 ya es seguro (la
// pintura copia los pixeles en el acto) y N=2 deja un cuadro entero de margen.
func (mr *MapRenderer) ActivarDesalojoDeLaCache(cuadros int) {
	if cuadros < 1 {
		mr.desalojoEnCuadros = 0

		return
	}

	mr.desalojoEnCuadros = cuadros
}

// DesalojoEnCuadros: cuantos cuadros de gracia tiene una imagen antes de que el desalojo la pueda soltar (0 =
// apagado).
func (mr *MapRenderer) DesalojoEnCuadros() int { return mr.desalojoEnCuadros }

// ActivarComposicionAlVuelo (DC-002.113b): cuando el tramo esta lleno, la imagen se compone sobre un buffer de
// trabajo de hasta `maxBytes` B y se pinta sin cachearse. ADITIVA: sin llamarla, lo que no entra se derrama.
//
// DC-002.159i: `maxBytes` es ademas el TECHO del buffer -- no crece por encima de lo declarado. Con 0 (el camino de
// antes) el buffer crece cuando la imagen no entra.
func (mr *MapRenderer) ActivarComposicionAlVuelo(maxBytes int) {
	mr.componerAlVuelo = true
	mr.topeDelVuelo = maxBytes

	if maxBytes > 0 && len(mr.bufferDelVuelo) < maxBytes {
		mr.bufferDelVuelo = make([]byte, maxBytes)
	}
}

// ComposicionAlVuelo: si el tramo lleno compone al vuelo en vez de derramar.
func (mr *MapRenderer) ComposicionAlVuelo() bool { return mr.componerAlVuelo }

// CuadroDeLaCache: cuantos cuadros se dibujaron desde que existe este renderer (lo que mide el desalojo).
func (mr *MapRenderer) CuadroDeLaCache() int { return mr.cuadroDeLaCache }

// ActivarCachePerezoso: la cache de tiles se arma al dibujar (DC-002.94).
//
// POR QUE EXISTE. `generateTileCache` arma UNA superficie por grafico distinto del nivel entero: en el
// pueblo son 277 superficies y 29,36 MB de RGBA (medido en el host con console/dc/medidas/render-294), y el
// monton de la Dreamcast no tiene eso ni de lejos. Con esta palanca la cache arranca vacia y cada imagen se
// arma la primera vez que un tile visible la pide. Es ADITIVA: sin llamarla, el camino es el de siempre.
func (mr *MapRenderer) ActivarCachePerezoso() { mr.cachePerezoso = true }

// CachePerezosa: si la cache se arma al dibujar.
func (mr *MapRenderer) CachePerezosa() bool { return mr.cachePerezoso }

// ActivarCacheDeIndices: la cache de tiles guarda indices de 8 bits (1 B/pixel) y no RGBA (4 B/pixel)
// (DC-002.101). El dibujado usa `RenderIndicesTintado` del destino; la PC no la llama nunca.
func (mr *MapRenderer) ActivarCacheDeIndices() {
	mr.cacheEnIndices = true

	if mr.indicesCacheRecords == nil {
		mr.indicesCacheRecords = make(map[uint32]*imagenDeIndices)
	}
}

// CacheDeIndices: si la cache guarda indices en vez de RGBA.
func (mr *MapRenderer) CacheDeIndices() bool { return mr.cacheEnIndices }

// ActivarCacheSinGetTiles (DC-002.135): con la imagen ya armada, `generate*Cache` NO pide los datos del tile al
// motor (`mapEngine.GetTiles`). El `YAdjust` que esas funciones calculaban se guarda con la imagen y se repone
// tal cual, asi que el dibujado es el mismo; lo que se va es una llamada por tile y por cuadro que se pagaba
// acertara o no la cache. Es la palanca del A/B: apagada, el camino es el de DC-002.134 tal cual.
func (mr *MapRenderer) ActivarCacheSinGetTiles(si bool) { mr.cacheSinGetTiles = si }

// CacheSinGetTiles: si el atajo esta puesto (para el log del arranque).
func (mr *MapRenderer) CacheSinGetTiles() bool { return mr.cacheSinGetTiles }

// FijarTamanoDelCuadro: el tamano del cuadro que este renderer compone, en pixeles, para la consola
// (DC-002.94). Sin llamarla, el tamano sale de d2display como siempre.
func (mr *MapRenderer) FijarTamanoDelCuadro(ancho, alto int) {
	mr.tamanoDelCuadro = d2display.Size{W: ancho, H: alto}
	mr.tieneTamano = ancho > 0 && alto > 0

	if mr.tieneTamano {
		mr.viewport.Resize(ancho, alto)
	}
}

// TamanoDelCuadro: el tamano efectivo del cuadro (el de la consola si se fijo, el de d2display si no).
func (mr *MapRenderer) TamanoDelCuadro() d2display.Size {
	if mr.tieneTamano {
		return mr.tamanoDelCuadro
	}

	return d2display.Get()
}

// Opcion es un ajuste que se aplica al renderer ANTES de armar la cache de tiles (DC-002.94). Es variadica
// a proposito: los llamadores de la PC (`d2gamescreen`) no cambian una linea.
type Opcion func(*MapRenderer)

// ComoConsola: el cuadro es de 640x480 (o el tamano que se pida) y no el de d2display (DC-002.94).
func ComoConsola(ancho, alto int) Opcion {
	return func(mr *MapRenderer) { mr.FijarTamanoDelCuadro(ancho, alto) }
}

// CacheDeTilesPerezosa: las imagenes de los tiles se arman al dibujar, no todas al crear el renderer
// (DC-002.94: la cache entera del pueblo son 29,36 MB de RGBA y en la consola no entran).
func CacheDeTilesPerezosa() Opcion {
	return func(mr *MapRenderer) { mr.ActivarCachePerezoso() }
}

// CacheDeTilesEnIndices: las imagenes de los tiles se guardan como indices de 8 bits en vez de RGBA
// (DC-002.101: 1 B/pixel contra 4). El destino del dibujado tiene que implementar `lienzoDeIndices`.
func CacheDeTilesEnIndices() Opcion {
	return func(mr *MapRenderer) { mr.ActivarCacheDeIndices() }
}

// CacheDeTilesEnUnTramo: los pixeles de la cache en indices salen de UN tramo de `bytes` B (un solo `make`
// grande = en la consola una asignacion del monton de C) repartido entre las imagenes (DC-002.104, ADR 0001).
// Es aditiva y va con `CacheDeTilesEnIndices`: sin ella, una `make` por imagen, que es el camino de la PC.
func CacheDeTilesEnUnTramo(bytes int) Opcion {
	return func(mr *MapRenderer) { mr.ActivarTramoDeLaCache(bytes) }
}

// ConDesalojoDeLaCache (DC-002.113b): cuando el tramo de la cache se llena, suelta las imagenes que no se usaron
// en los ultimos `cuadros` cuadros y reusa esos bytes. Es ADITIVA: sin ella, la cache es de solo agregar y lo que
// no entra se derrama al monton (que es lo que tumbaba al recolector en la consola).
func ConDesalojoDeLaCache(cuadros int) Opcion {
	return func(mr *MapRenderer) { mr.ActivarDesalojoDeLaCache(cuadros) }
}

// ConComposicionAlVuelo (DC-002.113b): cuando el tramo de la cache se llena, la imagen se compone sobre un buffer
// de trabajo de hasta `maxBytes` B y se pinta sin cachearse. Es la otra salida, la que se mide contra el desalojo.
func ConComposicionAlVuelo(maxBytes int) Opcion {
	return func(mr *MapRenderer) { mr.ActivarComposicionAlVuelo(maxBytes) }
}

// CacheDeTilesEnTramo: igual que `CacheDeTilesEnUnTramo`, pero con el tramo YA reservado
// (`ReservarTramoDeLaCache`), que es lo que permite pedirlo cuando el monton de C todavia no esta fragmentado.
func CacheDeTilesEnTramo(t *TramoDeLaCache) Opcion {
	return func(mr *MapRenderer) { mr.UsarTramoDeLaCache(t) }
}

// CacheDeTilesEnTramoEnPedazos: los pixeles de la cache en indices salen de bloques de `pedazo` B pedidos a
// medida, con un techo de `total` B (DC-002.106). Es la forma que entra en la consola cuando el tramo de un
// solo golpe no: cada bloque si se sirve de los huecos del monton de C y del break room, mientras que el pedido
// entero tiene que ser contiguo y no lo es (medido en DC-002.105). Aditiva como las demas.
func CacheDeTilesEnTramoEnPedazos(total, pedazo int, sonda ...func(tope int) int) Opcion {
	return func(mr *MapRenderer) { mr.ActivarTramoDeLaCacheEnPedazos(total, pedazo, sonda...) }
}

// CacheDeTilesEnUnTramoDe (DC-002.108): UN tramo de `bytes` B para los pixeles de la cache, reservado por
// `reserva` en vez de un `make`. Es el modo de un solo golpe con el origen cambiado -- con la VRAM del PVR, el
// pedido entero de 2.723.840 B que en el monton de C muere por contiguidad.
func CacheDeTilesEnUnTramoDe(bytes int, reserva func(bytes int) []byte, origen string) Opcion {
	return func(mr *MapRenderer) { mr.ActivarTramoDeLaCacheDe(bytes, reserva, origen) }
}

// CacheDeTilesEnBloquesDe (DC-002.108): los mismos bloques de la anterior, pero el que los da es `reserva` y
// no el monton de C. Es la palanca con la que la cache de tiles del pueblo (86 imagenes, 2.723.840 B) vive en
// la VRAM del PVR en vez de pelear con dlmalloc: el monton de C no la pide y la pared de la contiguidad
// desaparece. `origen` es el nombre del lugar ("vram") y viaja al censo.
//
// ADITIVA: sin llamarla, todo es como antes. Si `reserva` devuelve nil (no habia lugar) la imagen se derrama al
// `make` de siempre y se cuenta en el censo.
func CacheDeTilesEnBloquesDe(total, pedazo int, reserva func(bytes int) []byte, origen string,
	sonda ...func(tope int) int) Opcion {
	return func(mr *MapRenderer) { mr.ActivarTramoDeLaCacheEnBloquesDe(total, pedazo, reserva, origen, sonda...) }
}

// CreateMapRenderer creates a new MapRenderer, sets the required fields and returns a pointer to it.
func CreateMapRenderer(asset *d2asset.AssetManager, renderer d2interface.Renderer,
	mapEngine *d2mapengine.MapEngine,
	term d2interface.Terminal, l d2util.LogLevel, startX, startY float64, opciones ...Opcion) *MapRenderer {
	result := &MapRenderer{
		asset:     asset,
		renderer:  renderer,
		mapEngine: mapEngine,
		viewport:  NewViewport(0, 0, d2display.W(), d2display.H()),
		light:     newLighting(),
	}

	result.Logger = d2util.NewLogger()
	result.Logger.SetPrefix(logPrefix)
	result.Logger.SetLevel(l)

	result.Camera = Camera{}
	rx, ry := result.WorldToOrtho(startX, startY)
	startPosition := d2vector.NewPosition(rx, ry)
	result.Camera.position = &startPosition
	result.viewport.SetCamera(&result.Camera)

	// las palancas van ANTES de la cache: `CacheDeTilesPerezosa` tiene que estar puesta cuando se llama a
	// generateTileCache, o la cache se arma entera igual
	for _, opcion := range opciones {
		opcion(result)
	}

	var name, desc, level string

	name, desc = "mapdebugvis", "set map debug visualization level"
	level = "level"

	if err := term.Bind(name, desc, []string{level}, result.commandMapDebugVis); err != nil {
		result.Errorf("could not bind the mapdebugvis action, err: %v", err)
	}

	name, desc = "entitydebugvis", "set entity debug visualization level"

	if err := term.Bind(name, desc, []string{level}, result.commandEntityDebugVis); err != nil {
		result.Errorf("could not bind the entitydebugvis action, err: %v", err)
	}

	if mapEngine.LevelType().ID != 0 {
		result.generateTileCache()
	}

	return result
}

// UnbindTerminalCommands unbinds commands from the terminal
func (mr *MapRenderer) UnbindTerminalCommands(term d2interface.Terminal) error {
	return term.Unbind("mapdebugvis", "entitydebugvis")
}

func (mr *MapRenderer) commandMapDebugVis(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("invalid argument supplied")
	}

	level, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid argument supplied")
	}

	mr.mapDebugVisLevel = level

	return nil
}

func (mr *MapRenderer) commandEntityDebugVis(args []string) error {
	level, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid argument supplied")
	}

	mr.entityDebugVisLevel = level

	return nil
}

// RegenerateTileCache calls MapRenderer.generateTileCache().
func (mr *MapRenderer) RegenerateTileCache() {
	mr.generateTileCache()
}

// SetMapEngine sets the MapEngine this renderer is rendering.
func (mr *MapRenderer) SetMapEngine(mapEngine *d2mapengine.MapEngine) {
	mr.mapEngine = mapEngine
	mr.generateTileCache()
}

// Render determines the width and height of map tiles that should be rendered. The following render passes are
// made in succession (the real game's order, renderer.md b1: all floors before any wall):
//
// Pass 1: Floor tiles of the whole view, then lower wall tiles of the whole view.
//
// Pass 2: Per tile: shadows, then entities below walls.
//
// Pass 3: Upper wall tiles and entities above walls.
//
// Pass 4: Roof tiles.
func (mr *MapRenderer) Render(target d2interface.Surface) {
	// https://github.com/OpenDiablo2/OpenDiablo2/issues/789
	// Prevents concurrent map read & write exceptions that otherwise occur when we join a TCP game
	// as a remote client, due to rendering before we have handled the GenerateMapPacket.
	if mr.mapEngine.IsLoading {
		return
	}

	// DC-002.113b: el cuadro es la unidad con la que el desalojo mide si una imagen esta fria ("no se uso en N
	// cuadros"), asi que se cuenta aca, que es el unico lugar por donde pasa un cuadro dibujado.
	// DC-002.157: OJO, EL SALTO VA ANTES DE ESTE CONTADOR. Si el mapa se salta, la cache no se toco: contar el
	// cuadro igual haria que el desalojo fuera envejeciendo imagenes que nadie volvio a pedir y, cuando la
	// camara se mueva, la cache se recomponga entera sin necesidad. El salto congela la cache con el mapa.
	if mr.puedeSaltarseElMapa157() {
		mr.recogidaAlVuelo = nil
		mr.contarSalto157()
		contarCuadro135()

		return
	}

	mr.cuadroDeLaCache++
	mr.recogidaAlVuelo = nil

	// DC-002.139: el censo de tiles animados es de ESTE cuadro (el POC no puede saltarse el mapa si hay alguno).
	reiniciarTilesAnimados139()

	mapSize := mr.mapEngine.Size()

	sz := mr.TamanoDelCuadro()
	mr.viewport.Resize(sz.W, sz.H)

	startX, startY, endX, endY := TileRange(mr.viewport, sz.W, sz.H, mapSize.Width, mapSize.Height)

	mr.stats = DrawStats{Lit: mr.light.active()}
	if mr.light.active() {
		mr.stats.LightSources = 1 + len(mr.light.input.Extra)
	}

	if dx, dy := mr.shake.offset(shakeRand.Intn); dx != 0 || dy != 0 {
		target.PushTranslation(dx, dy)
		defer target.Pop()
	}

	// DC-002.135: el cronometro por PASADA, para cerrar la cuenta del mapa sin dejar nada afuera.
	tPaso := ahora135()

	mr.indexEntities()

	anotarPaso135(PasoIndice135, tPaso)

	// only the tiles that can be seen (the range is a rectangle around the isometric diamond of the screen)
	rows := VisibleTileRows(mr.viewport, sz.W, sz.H, startX, startY, endX, endY)

	tPaso = ahora135()

	mr.renderBackdrops(target)

	anotarPaso135(PasoFondos135, tPaso)

	tPaso = ahora135()

	mr.renderPass1(target, startY, endY, rows)

	anotarPaso135(Paso1Plantas135, tPaso)

	tPaso = ahora135()

	mr.renderPass2(target, startY, endY, rows)

	anotarPaso135(Paso2Sombras135, tPaso)

	if mr.mapDebugVisLevel > 0 {
		mr.renderMapDebug(mr.mapDebugVisLevel, target, startX, startY, endX, endY)
	}

	tPaso = ahora135()

	mr.renderPass3(target, startY, endY, rows)

	anotarPaso135(Paso3Paredes135, tPaso)

	tPaso = ahora135()

	mr.renderPass4(target, startY, endY, rows)

	anotarPaso135(Paso4Techos135, tPaso)

	if mr.entityDebugVisLevel > 0 {
		mr.renderEntityDebug(target)
	}

	contarCuadro135()

	// DC-002.157: el mapa quedo pintado en ESTE lienzo y en ESTA traslacion. El cuadro siguiente se lo puede
	// saltar si nada de eso cambio.
	mr.anotarElMapaPintado157()
}

// TileRange is the range of tiles [startX,endX) x [startY,endY) that covers a screen of width x height pixels
// (clamped to a map of mapW x mapH tiles). The range is taken from the top and bottom middle of the screen
// as the 800x600 game always did, and widened by the extra half width on both sides for wider screens
// (a screen pixel to the right is +1/160 tile in x and -1/160 in y). At 800x600 it is exactly the old range.
func TileRange(v *Viewport, width, height, mapW, mapH int) (startX, startY, endX, endY int) {
	stxf, styf := v.ScreenToWorld(width/two, -extraViewTop)
	etxf, etyf := v.ScreenToWorld(width/two, height+extraViewBottom)

	grow := float64(width-d2display.BaseW) / two / (orthoTileWidth * two)
	if grow < 0 {
		grow = 0
	} else if width > d2display.BaseW {
		grow += float64(alignedWidth) / half / (orthoTileWidth * two) // the view shifts by 200 px beside open panels
	}

	startX = int(math.Max(0, math.Floor(stxf-grow)))
	startY = int(math.Max(0, math.Floor(styf-grow)))
	endX = int(math.Min(float64(mapW), math.Ceil(etxf+grow)))
	endY = int(math.Min(float64(mapH), math.Ceil(etyf+grow)))

	return startX, startY, endX, endY
}

// MoveCameraTo sets the position of the Camera to the given x and y coordinates.
func (mr *MapRenderer) MoveCameraTo(position *d2vector.Position) {
	mr.Camera.MoveTo(position)
}

// MoveCameraBy adds the given vector to the current position of the Camera.
func (mr *MapRenderer) MoveCameraBy(vector *d2vector.Vector) {
	mr.Camera.MoveBy(vector)
}

// MoveCameraTargetBy adds the given vector to the current position of the Camera.
func (mr *MapRenderer) MoveCameraTargetBy(vector *d2vector.Vector) {
	mr.Camera.MoveTargetBy(vector)
}

// ScreenToWorld returns the world position for the given screen (pixel) position.
func (mr *MapRenderer) ScreenToWorld(x, y int) (worldX, worldY float64) {
	return mr.viewport.ScreenToWorld(x, y)
}

// ScreenToOrtho returns the orthogonal position, without accounting for the isometric angle, for the given screen
// (pixel) position.
func (mr *MapRenderer) ScreenToOrtho(x, y int) (orthoX, orthoY float64) {
	return mr.viewport.ScreenToOrtho(x, y)
}

// WorldToOrtho returns the orthogonal position for the given isometric world position.
func (mr *MapRenderer) WorldToOrtho(x, y float64) (orthoX, orthoY float64) {
	return mr.viewport.WorldToOrtho(x, y)
}

// Floors of the whole view first, then lower walls (a wall must never be
// overpainted by the floor of a later tile).
func (mr *MapRenderer) renderPass1(target d2interface.Surface, startY, endY int, rows *TileRows) {
	for tileY := startY; tileY < endY; tileY++ {
		from, to := rows.Row(tileY)

		for tileX := from; tileX < to; tileX++ {
			tile := mr.mapEngine.TileAt(tileX, tileY)
			mr.viewport.PushTranslationWorld(float64(tileX), float64(tileY))

			for _, floor := range tile.Components.Floors {
				if !floor.Hidden() && floor.Prop1 != 0 {
					mr.renderFloor(floor, target, tileX, tileY)
				}
			}

			mr.viewport.PopTranslation()
		}
	}

	for tileY := startY; tileY < endY; tileY++ {
		from, to := rows.Row(tileY)

		for tileX := from; tileX < to; tileX++ {
			tile := mr.mapEngine.TileAt(tileX, tileY)
			mr.viewport.PushTranslationWorld(float64(tileX), float64(tileY))

			for idx, wall := range tile.Components.Walls {
				if !wall.Hidden() && wall.Prop1 != 0 && wall.Type.LowerWall() {
					mr.renderWall(wall, mr.viewport, target, tileX, tileY, idx, false)
				}
			}

			mr.viewport.PopTranslation()
		}
	}
}

// Entities below walls.
func (mr *MapRenderer) renderPass2(target d2interface.Surface, startY, endY int, rows *TileRows) {
	for tileY := startY; tileY < endY; tileY++ {
		from, to := rows.Row(tileY)

		for tileX := from; tileX < to; tileX++ {
			mr.viewport.PushTranslationWorld(float64(tileX), float64(tileY))

			tile := mr.mapEngine.TileAt(tileX, tileY)
			for _, shadow := range tile.Components.Shadows {
				if !shadow.Hidden() && shadow.Prop1 != 0 {
					mr.renderShadow(shadow, target, tileX, tileY)
				}
			}

			tileEnt := mr.getEntitiesBelowWalls(tileX, tileY)

			mr.renderEntitiesBySubtile(target, tileEnt)

			mr.viewport.PopTranslation()
		}
	}
}

// entityBucket holds the entities standing on one tile, split by layer.
type entityBucket struct {
	below []d2interface.MapEntity // layer 1: drawn below the walls
	above []d2interface.MapEntity // every other layer: drawn above the walls
}

// indexEntities sorts all entities into per-tile buckets once per frame. The passes used to scan
// every entity for every visible tile (tiles x entities map iterations per frame). The buckets
// and their slices are reused between frames. Entities are visited in map order, as before, so
// the order of entities that share a tile and a subtile is as unspecified as it always was.
func (mr *MapRenderer) indexEntities() {
	// DC-002.159: con los entes aparte el renderer no los indexa ni los dibuja (los pinta el que llama,
	// encima del mapa): el indice por casilla se queda vacio y las pasadas 2 y 3 no dibujan ningun ente.
	if mr.entesAparte159 {
		return
	}

	if mr.entBuckets == nil {
		mr.entBuckets = make(map[[2]int]*entityBucket)
	}

	for key, b := range mr.entBuckets {
		b.below, b.above = b.below[:0], b.above[:0]

		mr.entFree = append(mr.entFree, b)

		delete(mr.entBuckets, key)
	}

	for _, mapEntity := range mr.mapEngine.Entities() {
		pos := mapEntity.GetPosition()
		key := [2]int{int(pos.X() / subtilesPerTile), int(pos.Y() / subtilesPerTile)} // = int(pos.World()), no allocation

		b := mr.entBuckets[key]
		if b == nil {
			if n := len(mr.entFree); n > 0 {
				b, mr.entFree = mr.entFree[n-1], mr.entFree[:n-1]
			} else {
				b = &entityBucket{}
			}

			mr.entBuckets[key] = b
		}

		if mapEntity.GetLayer() == 1 {
			b.below = append(b.below, mapEntity)
		} else {
			b.above = append(b.above, mapEntity)
		}
	}
}

// renderEntitiesBySubtile draws the entities of one tile ordered by sub tile (row by row, entities in
// bucket order inside a sub tile). Entities whose sub tile offset is outside 0..4 are not drawn, as
// before. The offsets are computed once per entity, without allocating (this used to cost 25 x 2
// vector allocations per entity and tile).
func (mr *MapRenderer) renderEntitiesBySubtile(target d2interface.Surface, ents []d2interface.MapEntity) {
	// DC-002.159: con los entes aparte ninguno se dibuja en las pasadas (los pinta el que llama).
	if mr.entesAparte159 || len(ents) == 0 {
		return
	}

	keys := mr.subKeys[:0]

	for _, e := range ents {
		pos := e.GetPosition()
		ox, oy := pos.SubTileXY()
		sx, sy := int(ox), int(oy)

		if sx < 0 || sx > 4 || sy < 0 || sy > 4 {
			keys = append(keys, -1)
		} else {
			keys = append(keys, int8(sy*5+sx))
		}
	}

	mr.subKeys = keys

	for k := int8(0); k < 25; k++ {
		for i, e := range ents {
			if keys[i] != k {
				continue
			}

			target.PushTranslation(mr.viewport.GetTranslationScreen())
			mr.renderLitEntity(target, e)
			target.Pop()
		}
	}
}

func (mr *MapRenderer) getEntitiesBelowWalls(tileX, tileY int) []d2interface.MapEntity {
	if b := mr.entBuckets[[2]int{tileX, tileY}]; b != nil {
		return b.below
	}

	return nil
}

// Upper wall tiles and entities above walls.
func (mr *MapRenderer) renderPass3(target d2interface.Surface, startY, endY int, rows *TileRows) {
	for tileY := startY; tileY < endY; tileY++ {
		from, to := rows.Row(tileY)

		for tileX := from; tileX < to; tileX++ {
			tile := mr.mapEngine.TileAt(tileX, tileY)
			mr.viewport.PushTranslationWorld(float64(tileX), float64(tileY))
			mr.renderTilePass2(tile, target, tileX, tileY)

			entities := mr.getEntitiesAboveWalls(tileX, tileY)

			mr.renderEntitiesBySubtile(target, entities)

			mr.viewport.PopTranslation()
		}
	}
}

func (mr *MapRenderer) getEntitiesAboveWalls(tileX, tileY int) []d2interface.MapEntity {
	if b := mr.entBuckets[[2]int{tileX, tileY}]; b != nil {
		return b.above
	}

	return nil
}

// Roof tiles.
func (mr *MapRenderer) renderPass4(target d2interface.Surface, startY, endY int, rows *TileRows) {
	mr.updateRoofCover()

	for tileY := startY; tileY < endY; tileY++ {
		from, to := rows.Row(tileY)

		for tileX := from; tileX < to; tileX++ {
			tile := mr.mapEngine.TileAt(tileX, tileY)
			mr.viewport.PushTranslationWorld(float64(tileX), float64(tileY))
			mr.renderTilePass3(tile, target, tileX, tileY)
			mr.viewport.PopTranslation()
		}
	}
}

func (mr *MapRenderer) renderTilePass2(tile *d2mapengine.MapTile, target d2interface.Surface, tileX, tileY int) {
	for idx, wall := range tile.Components.Walls {
		if !wall.Hidden() && wall.Type.UpperWall() {
			mr.renderWall(wall, mr.viewport, target, tileX, tileY, idx, true)
		}
	}
}

func (mr *MapRenderer) renderTilePass3(tile *d2mapengine.MapTile, target d2interface.Surface, tileX, tileY int) {
	for idx, wall := range tile.Components.Walls {
		if wall.Type == d2enum.TileRoof {
			mr.renderRoof(wall, mr.viewport, target, tileX, tileY, idx)
		}
	}
}

func (mr *MapRenderer) renderFloor(tile d2ds1.Tile, target d2interface.Surface, tileX, tileY int) {
	// DC-002.159i: la imagen que el techo dejo afuera no se pide ni se avisa por cuadro (el log ya lo dijo). Las
	// animadas quedan afuera de este atajo: su clave es el cuadro de la animacion, no el del tile.
	if !tile.Animated && mr.noEntraLaImagen(claveDeCache(tile.Style, tile.Sequence, 0, tile.RandomIndex)) {
		return
	}

	// DC-002.135: el cronometro del bloque de la CACHE (hipotesis (a) de la tarjeta).
	tCache := ahora135()

	// DC-002.94: con la cache perezosa, la imagen se arma aca la primera vez
	mr.asegurarImagen(&tile, 0)

	// DC-002.135: el pedazo `cacheBuscar` (la busqueda en la cache, ya con la imagen armada o no).
	tBuscar := ahora135()

	var (
		img    d2interface.Surface
		indice *imagenDeIndices
	)

	clave := tile.RandomIndex
	if tile.Animated {
		// DC-002.139: un tile animado se pinta con el reloj, no con la camara: el POC no puede saltarse el mapa.
		contarTileAnimado139()

		clave = byte(tileAnimFrame(mr.animTicks, 0, mr.animFrames[uint16(tile.Style)<<8|uint16(tile.Sequence)]))
	}

	if mr.cacheEnIndices {
		indice = mr.getCacheDeIndices(tile.Style, tile.Sequence, 0, clave)
	} else {
		img = mr.getImageCacheRecord(tile.Style, tile.Sequence, 0, clave)
	}

	anotar135(BloqueCacheBuscar135, tBuscar)

	if img == nil && indice == nil {
		mr.Warningf("Render called on uncached floor {%v,%v}", tile.Style, tile.Sequence)
		return
	}

	mr.stats.Floors++

	anotar135(BloqueCache135, tCache)

	// DC-002.135: el bloque de la VISTA (la aritmetica de la posicion en pantalla, hipotesis (b)).
	tVista := ahora135()

	mr.viewport.PushTranslationOrtho(-80, float64(tile.YAdjust))
	defer mr.viewport.PopTranslation()

	anotar135(BloqueVista135, tVista)

	// DC-002.135: el bloque del ESTADO del Surface (hipotesis (c)).
	tEstado := ahora135()

	target.PushTranslation(mr.viewport.GetTranslationScreen())
	defer target.Pop()

	anotar135(BloqueEstado135, tEstado)

	// DC-002.135: el bloque del PINTADO (el `RenderIndicesTintado` y lo que lo rodea).
	tPintado := ahora135()

	if mr.light.active() {
		contarTile135()
		anotar135(BloquePintado135, tPintado)

		if indice != nil {
			mr.Warningf("la cache en indices no pinta con la luz prendida (DC-002.101)")

			return
		}

		mr.renderShadedImage(target, img, floorShadeCols, floorShadeRows, 1, shadeFloor, tileX, tileY)

		return
	}

	mr.pintarTile(target, img, indice)

	contarTile135()
	anotar135(BloquePintado135, tPintado)
}

// pintarTile dibuja la imagen cacheada del tile: los indices por el camino de DC-002.101 o la superficie
// RGBA por el de siempre. Es UN punto para las cuatro capas (piso, sombra, pared y techo).
func (mr *MapRenderer) pintarTile(target d2interface.Surface, img d2interface.Surface, indice *imagenDeIndices) {
	if indice != nil {
		mr.dibujarImagenDeIndices(target, indice)

		return
	}

	target.Render(img)
}

func (mr *MapRenderer) renderWall(tile d2ds1.Tile, viewport *Viewport, target d2interface.Surface,
	tileX, tileY, idx int, upper bool) {
	// DC-002.159i: la imagen que el techo dejo afuera no se pide ni se avisa por cuadro (el log ya lo dijo).
	if mr.noEntraLaImagen(claveDeCache(tile.Style, tile.Sequence, int(tile.Type), tile.RandomIndex)) {
		return
	}

	// DC-002.135: el bloque de la CACHE (ver `cronometro_135.go`).
	tCache := ahora135()

	// DC-002.94: con la cache perezosa, la imagen (y el YAdjust que esta funcion usa) se arman aca la primera vez
	mr.asegurarImagen(&tile, tile.Type)

	// DC-002.135: el pedazo `cacheBuscar`.
	tBuscar := ahora135()

	img := mr.getImageCacheRecord(tile.Style, tile.Sequence, tile.Type, tile.RandomIndex)
	indice := (*imagenDeIndices)(nil)

	if mr.cacheEnIndices {
		indice = mr.getCacheDeIndices(tile.Style, tile.Sequence, int(tile.Type), tile.RandomIndex)
	}

	anotar135(BloqueCacheBuscar135, tBuscar)

	if img == nil && indice == nil {
		if !mr.blankWalls[blankWallKey(&tile)] {
			mr.Warningf("Render called on uncached wall {%v,%v,%v}", tile.Style, tile.Sequence, tile.Type)
		}

		return
	}

	if upper {
		mr.stats.UpperWalls++
	} else {
		mr.stats.Walls++
	}

	anotar135(BloqueCache135, tCache)

	// DC-002.135: el bloque de la VISTA.
	tVista := ahora135()

	viewport.PushTranslationOrtho(-80, float64(tile.YAdjust))
	defer viewport.PopTranslation()

	anotar135(BloqueVista135, tVista)

	// DC-002.135: el bloque del ESTADO, con la cuenta de la luz/alfa adentro (es lo que se hacia entre el
	// `PushTranslation` y el pintado).
	tEstado := ahora135()

	target.PushTranslation(viewport.GetTranslationScreen())
	defer target.Pop()

	if !mr.light.active() {
		anotar135(BloqueEstado135, tEstado)

		// DC-002.135: el bloque del PINTADO.
		tPintado := ahora135()

		mr.pintarTile(target, img, indice)

		contarTile135()
		anotar135(BloquePintado135, tPintado)

		return
	}

	if indice != nil {
		mr.Warningf("la cache en indices no pinta con la luz prendida (DC-002.101)")
		anotar135(BloqueEstado135, tEstado)

		return
	}

	alpha := 1.0
	if upper {
		alpha = mr.light.fadeFor(wallKey{tileX + mr.offX, tileY + mr.offY, idx},
			mr.heroBehind(tile.Type, tileX+mr.offX, tileY+mr.offY))
		if alpha < 1 {
			mr.stats.WallsFading++
		}
	}

	anotar135(BloqueEstado135, tEstado)

	tPintado := ahora135()

	mr.renderShadedImage(target, img, wallShadeCols, wallShadeRows, alpha, shadeWall, tileX, tileY)

	contarTile135()
	anotar135(BloquePintado135, tPintado)
}

// renderRoof draws a roof tile with the flat frame ambient (roofs sample no
// light map in the real game).
func (mr *MapRenderer) renderRoof(tile d2ds1.Tile, viewport *Viewport, target d2interface.Surface,
	tileX, tileY, idx int) {
	// DC-002.159i: una imagen que el techo dejo afuera no se pide ni se avisa por cuadro (ver
	// `limite_de_imagen_159i.go`); el log ya dijo cual es, una vez, cuando se rechazo.
	if mr.noEntraLaImagen(claveDeCache(tile.Style, tile.Sequence, int(tile.Type), tile.RandomIndex)) {
		return
	}

	// DC-002.135: el bloque de la CACHE.
	tCache := ahora135()

	// DC-002.94: con la cache perezosa, la imagen se arma aca la primera vez
	mr.asegurarImagen(&tile, tile.Type)

	// DC-002.135: el pedazo `cacheBuscar`.
	tBuscar := ahora135()

	img := mr.getImageCacheRecord(tile.Style, tile.Sequence, tile.Type, tile.RandomIndex)
	indice := (*imagenDeIndices)(nil)

	if mr.cacheEnIndices {
		indice = mr.getCacheDeIndices(tile.Style, tile.Sequence, int(tile.Type), tile.RandomIndex)
	}

	anotar135(BloqueCacheBuscar135, tBuscar)

	if img == nil && indice == nil {
		mr.Warningf("Render called on uncached wall {%v,%v,%v}", tile.Style, tile.Sequence, tile.Type)
		return
	}

	mr.stats.Roofs++

	anotar135(BloqueCache135, tCache)

	// DC-002.135: el bloque de la VISTA.
	tVista := ahora135()

	viewport.PushTranslationOrtho(-80, float64(tile.YAdjust))
	defer viewport.PopTranslation()

	anotar135(BloqueVista135, tVista)

	// DC-002.135: el bloque del ESTADO (con el alfa del techo adentro, que es lo que se paga entre el
	// `PushTranslation` y el pintado).
	tEstado := ahora135()

	target.PushTranslation(viewport.GetTranslationScreen())
	defer target.Pop()

	if mr.light.active() {
		if indice != nil {
			mr.Warningf("la cache en indices no pinta con la luz prendida (DC-002.101)")
			anotar135(BloqueEstado135, tEstado)

			return
		}

		alpha := mr.light.roofAlpha(wallKey{tileX + mr.offX, tileY + mr.offY, idx},
			mr.roofCover[[2]int{tileX + mr.offX, tileY + mr.offY}])
		if alpha < 1 {
			mr.stats.RoofsFading++
		}

		anotar135(BloqueEstado135, tEstado)

		if alpha <= 0 {
			return
		}

		tPintado := ahora135()

		target.PushColor(withAlpha(mr.light.ambientTint(), alpha))
		defer target.Pop()

		mr.pintarTile(target, img, indice)

		contarTile135()
		anotar135(BloquePintado135, tPintado)

		return
	}

	anotar135(BloqueEstado135, tEstado)

	// DC-002.135: el bloque del PINTADO.
	tPintado := ahora135()

	mr.pintarTile(target, img, indice)

	contarTile135()
	anotar135(BloquePintado135, tPintado)
}

// updateRoofCover recomputes which roof tiles the hero is under (see roof_fade.go).
func (mr *MapRenderer) updateRoofCover() {
	mr.roofCover = nil
	if !mr.light.active() {
		return
	}

	hx, hy := int(mr.light.input.HeroX), int(mr.light.input.HeroY)
	size := mr.mapEngine.Size()

	mr.roofCover = roofRegion(func(x, y int) bool {
		if x < 0 || y < 0 || x >= size.Width || y >= size.Height {
			return false
		}

		for _, w := range mr.mapEngine.TileAt(x, y).Components.Walls {
			if w.Type == d2enum.TileRoof && !w.Hidden() && w.Prop1 != 0 {
				return true
			}
		}

		return false
	}, hx, hy)
}

// wallFadeRule is the exact test of the real game for a wall that fades (Game.exe 1.14b, verified in Ghidra:
// DRAWLIST_CollectRoomEntries 0x4da580 sends wall tile records with sprite-priority bits to
// DRAWLIST_AddTimedRoomEntry 0x4d9f60, which asks DRAWLIST_IsRoomEntryOnScreen 0x4d9e40; hero position is
// the path position / 5 in DRAWLIST_BuildForFrame 0x4da970). In tile units, with (hx, hy) the hero's tile:
// orientations 1,4,5,7,8,10,12 fade when hx < wallX < hx+4, orientations 2,3,6,7,9,11,12 fade when
// hy < wallY < hy+4 (each axis tested alone, the other axis is not looked at). Other orientations (trees,
// 14) never fade. The target alpha is 0x80/255 and the fade is linear over 500 ms.
func wallFadeRule(t d2enum.TileType, wallX, wallY, heroX, heroY int) bool {
	switch t {
	case d2enum.TileLeftWall, d2enum.TileLeftPartOfNorthCornerWall, d2enum.TileLeftEndWall,
		d2enum.TileSouthCornerWall, d2enum.TileLeftWallWithDoor, d2enum.TileSpecialTile1,
		d2enum.TilePillarsColumnsAndStandaloneObjects:
		if wallX > heroX && wallX < heroX+4 {
			return true
		}
	}

	switch t {
	case d2enum.TileRightWall, d2enum.TileRightPartOfNorthCornerWall, d2enum.TileRightEndWall,
		d2enum.TileSouthCornerWall, d2enum.TileRightWallWithDoor, d2enum.TileSpecialTile2,
		d2enum.TilePillarsColumnsAndStandaloneObjects:
		return wallY > heroY && wallY < heroY+4
	}

	return false
}

// heroBehind reports whether the wall tile fades for the hero (see wallFadeRule).
func (mr *MapRenderer) heroBehind(t d2enum.TileType, tileX, tileY int) bool {
	in := mr.light.input

	return wallFadeRule(t, tileX, tileY, int(math.Floor(in.HeroX)), int(math.Floor(in.HeroY)))
}

// renderLitEntity draws an entity tinted by one light map sample at its feet.
func (mr *MapRenderer) renderLitEntity(target d2interface.Surface, e d2interface.MapEntity) {
	mr.stats.Entities++

	if !mr.light.active() {
		e.Render(target)
		return
	}

	x, y := e.GetPositionF()

	cell := mr.light.lm.Sample(x*subtilesPerTile, y*subtilesPerTile)

	// the hovered unit's light byte is doubled (min 0x40, max 0xff), 0x46dab0 / 0x46c9a0
	if h, ok := e.(interface{ Highlighted() bool }); ok && h.Highlighted() {
		cell.Intensity = d2lightmap.HoverIntensity(cell.Intensity)
	}

	target.PushColor(mr.light.tint(cell))
	e.Render(target)
	target.Pop()
}

// shadowAlpha is the opacity of DT1 shadow tiles (the original uses a PL2 blend table; 160/255 is
// the approximation chosen in renderer.md section c, item 4).
const shadowAlpha = 160.0 / 255.0

func (mr *MapRenderer) renderShadow(tile d2ds1.Tile, target d2interface.Surface, tileX, tileY int) {
	// DC-002.159i: la imagen que el techo dejo afuera no se pide ni se avisa por cuadro (el log ya lo dijo). Es
	// el camino que la evidencia de DC-002.159h señalo como el que muere (`renderShadow` -> `asegurarImagen` ->
	// `pedirTrozoDeIndices`), y el que hay que dejar barato cuando la imagen no entra: ni `GetTiles` ni aviso.
	if mr.noEntraLaImagen(claveDeCache(tile.Style, tile.Sequence, int(d2enum.TileShadow), tile.RandomIndex)) {
		return
	}

	mr.asegurarImagen(&tile, d2enum.TileShadow)

	img := mr.getImageCacheRecord(tile.Style, tile.Sequence, 13, tile.RandomIndex)
	indice := (*imagenDeIndices)(nil)

	if mr.cacheEnIndices {
		indice = mr.getCacheDeIndices(tile.Style, tile.Sequence, int(d2enum.TileShadow), tile.RandomIndex)
	}

	if img == nil && indice == nil {
		// shadow tiles without graphics (width or height 0) are never cached;
		// the exact room tiles of the outdoor generator contain such markers
		key := uint32(tile.Style)<<16 | uint32(tile.Sequence)<<8 | uint32(tile.RandomIndex)

		if mr.blankShadows == nil {
			mr.blankShadows = map[uint32]bool{}
		}

		blank, known := mr.blankShadows[key]
		if !known {
			opts := mr.mapEngine.GetTiles(int(tile.Style), int(tile.Sequence), d2enum.TileShadow)
			blank = opts != nil && int(tile.RandomIndex) < len(opts) &&
				(opts[tile.RandomIndex].Width == 0 || opts[tile.RandomIndex].Height == 0)
			mr.blankShadows[key] = blank
		}

		if blank {
			return
		}

		mr.Warningf("Render called on uncached shadow {%v,%v}", tile.Style, tile.Sequence)

		return
	}

	mr.stats.Shadows++

	defer mr.viewport.PushTranslationOrtho(-80, float64(tile.YAdjust)).PopTranslation()

	target.PushTranslation(mr.viewport.GetTranslationScreen())
	defer target.Pop()

	if mr.light.active() {
		if indice != nil {
			mr.Warningf("la cache en indices no pinta con la luz prendida (DC-002.101)")

			return
		}

		// lit like the floor underneath
		mr.renderShadedImage(target, img, floorShadeCols, floorShadeRows, shadowAlpha, shadeFloor, tileX, tileY)

		return
	}

	target.PushColor(color.RGBA{R: 255, G: 255, B: 255, A: 160}) //nolint:gomnd // Not a magic number...
	defer target.Pop()

	mr.pintarTile(target, img, indice)
}

func (mr *MapRenderer) renderMapDebug(mapDebugVisLevel int, target d2interface.Surface, startX, startY, endX, endY int) {
	for tileY := startY; tileY < endY; tileY++ {
		for tileX := startX; tileX < endX; tileX++ {
			mr.viewport.PushTranslationWorld(float64(tileX), float64(tileY))
			mr.renderTileDebug(tileX, tileY, mapDebugVisLevel, target)
			mr.viewport.PopTranslation()
		}
	}
}

//nolint:funlen // doesn't make sense to split this function
func (mr *MapRenderer) renderEntityDebug(target d2interface.Surface) {
	entities := mr.mapEngine.Entities()

	for idx := range entities {
		e := entities[idx]
		pos := e.GetPosition()
		world := pos
		x, y := world.X()/subtilesPerTile, world.Y()/subtilesPerTile
		velocity := e.GetVelocity()
		velocity = *velocity.Clone()
		vx, vy := mr.viewport.WorldToOrtho(velocity.X(), velocity.Y())
		screenX, screenY := mr.viewport.WorldToScreen(x, y)

		offX, offY := dbgOffsetXY, -dbgOffsetXY

		entScreenXf, entScreenYf := mr.WorldToScreenF(e.GetPositionF())
		entScreenX := int(math.Floor(entScreenXf))
		entScreenY := int(math.Floor(entScreenYf))
		entityWidth, entityHeight := e.GetSize()
		halfWidth, halfHeight := entityWidth/two, entityHeight/two
		l, r := entScreenX-halfWidth, entScreenX+halfWidth
		t, b := entScreenY-halfHeight, entScreenY+halfHeight
		mx, my := mr.renderer.GetCursorPos()
		xWithin := (l <= mx) && (r >= mx)
		yWithin := (t <= my) && (b >= my)
		within := xWithin && yWithin

		boxLineColor := d2util.Color(magentaFullOpacity)
		boxHoverColor := d2util.Color(yellowFullOpacity)

		boxColor := boxLineColor

		if within {
			boxColor = boxHoverColor
		}

		stack := 0
		// box
		mr.viewport.PushTranslationWorld(x, y)

		target.PushTranslation(screenX, screenY)
		stack++

		target.PushTranslation(-halfWidth, -halfHeight)
		stack++

		target.DrawLine(0, entityHeight, boxColor)
		target.DrawLine(entityWidth, 0, boxColor)

		target.PushTranslation(entityWidth, entityHeight)
		stack++

		target.DrawLine(-entityWidth, 0, boxColor)
		target.DrawLine(0, -entityHeight, boxColor)
		target.PopN(stack)
		mr.viewport.PopTranslation()

		// hover
		if within {
			mr.viewport.PushTranslationWorld(x, y)
			target.PushTranslation(screenX, screenY)
			target.DrawLine(offX, offY, d2util.Color(whiteHalfOpacity))
			target.PushTranslation(offX+dbgBoxPadding, offY-dbgBoxPadding*two)
			target.PushTranslation(-dbgOffsetXY, -dbgOffsetXY)
			target.DrawRect(dbgBoxWidth, dbgBoxHeight, d2util.Color(blackQuarterOpacity))
			target.Pop()
			target.DrawTextf("World (%.2f, %.2f)\nVelocity (%.2f, %.2f)", x, y, vx, vy)
			target.Pop()
			target.DrawLine(int(vx), int(vy), d2util.Color(lightGreenFullOpacity))
			target.Pop()
			mr.viewport.PopTranslation()
		}
	}
}

// WorldToScreen returns the screen (pixel) position for the given isometric world position as two ints.
func (mr *MapRenderer) WorldToScreen(x, y float64) (screenX, screenY int) {
	return mr.viewport.WorldToScreen(x, y)
}

// WorldToScreenF returns the screen (pixel) position for the given isometric world position as two float64s.
func (mr *MapRenderer) WorldToScreenF(x, y float64) (screenX, screenY float64) {
	return mr.viewport.WorldToScreenF(x, y)
}

func (mr *MapRenderer) renderTileDebug(ax, ay, debugVisLevel int, target d2interface.Surface) {
	subTileColor := d2util.Color(lightBlueQuarterOpacity)
	tileColor := d2util.Color(whiteQuarterOpacity)
	tileCollisionColor := d2util.Color(redQuarterOpacity)

	screenX1, screenY1 := mr.viewport.WorldToScreen(float64(ax), float64(ay))
	screenX2, screenY2 := mr.viewport.WorldToScreen(float64(ax+1), float64(ay))
	screenX3, screenY3 := mr.viewport.WorldToScreen(float64(ax), float64(ay+1))

	target.PushTranslation(screenX1, screenY1)
	defer target.Pop()

	target.DrawLine(screenX2-screenX1, screenY2-screenY1, tileColor)
	target.DrawLine(screenX3-screenX1, screenY3-screenY1, tileColor)
	target.PushTranslation(-10, 10)
	target.DrawTextf("%v, %v", ax, ay)
	target.Pop()

	if debugVisLevel > 1 {
		for i := 1; i <= 4; i++ {
			x2 := i * orthoSubTileWidth
			y2 := i * orthoSubTileHeight

			target.PushTranslation(-x2, y2)
			target.DrawLine(orthoTileWidth, orthoTileHeight, subTileColor)
			target.Pop()

			target.PushTranslation(x2, y2)
			target.DrawLine(-orthoTileWidth, orthoTileHeight, subTileColor)
			target.Pop()
		}

		tile := mr.mapEngine.TileAt(ax, ay)

		for i, wall := range tile.Components.Walls {
			if wall.Type.Special() {
				target.PushTranslation(-20, 10+(i+1)*14) // nolint:gomnd // just for debug
				target.DrawTextf("s: %v-%v", wall.Style, wall.Sequence)
				target.Pop()
			}
		}

		for yy := 0; yy < 5; yy++ {
			for xx := 0; xx < 5; xx++ {
				isoX := (xx - yy) * orthoSubTileWidth
				isoY := (xx + yy) * orthoSubTileHeight

				blocked := tile.GetSubTileFlags(xx, yy).BlockWalk

				if blocked {
					target.PushTranslation(isoX+dbgCollisionOffsetX, isoY+dbgCollisionOffsetY)
					target.DrawRect(dbgCollisionSize, dbgCollisionSize, tileCollisionColor)
					target.Pop()
				}
			}
		}
	}
}

const (
	frameOverflow = 10
	frameLength   = 1.0 / frameOverflow
)

// Advance is called once per frame and maintains the MapRenderer's previous
// render timestamp and current frame.
func (mr *MapRenderer) Advance(elapsed float64) {
	mr.lastFrameTime += elapsed
	framesAdvanced := int(mr.lastFrameTime / frameLength)
	mr.lastFrameTime -= float64(framesAdvanced) * frameLength

	mr.currentFrame += framesAdvanced
	if mr.currentFrame >= frameOverflow {
		mr.currentFrame = 0
	}

	mr.animAcc += elapsed
	if n := int(mr.animAcc / tileAnimTickSeconds); n > 0 {
		mr.animAcc -= float64(n) * tileAnimTickSeconds
		mr.animTicks += n
	}

	mr.Camera.Advance(elapsed)
	mr.shake.advance(elapsed)
	mr.advanceLighting(elapsed)
}

func (mr *MapRenderer) loadPaletteForAct(levelType d2enum.RegionIdType) (d2interface.Palette,
	error) {
	var palettePath string

	switch levelType {
	case d2enum.RegionAct1Town, d2enum.RegionAct1Wilderness, d2enum.RegionAct1Cave, d2enum.RegionAct1Crypt,
		d2enum.RegionAct1Monestary, d2enum.RegionAct1Courtyard, d2enum.RegionAct1Barracks,
		d2enum.RegionAct1Jail, d2enum.RegionAct1Cathedral, d2enum.RegionAct1Catacombs, d2enum.RegionAct1Tristram:
		palettePath = d2resource.PaletteAct1
	case d2enum.RegionAct2Town, d2enum.RegionAct2Sewer, d2enum.RegionAct2Harem, d2enum.RegionAct2Basement,
		d2enum.RegionAct2Desert, d2enum.RegionAct2Tomb, d2enum.RegionAct2Lair, d2enum.RegionAct2Arcane:
		palettePath = d2resource.PaletteAct2
	case d2enum.RegionAct3Town, d2enum.RegionAct3Jungle, d2enum.RegionAct3Kurast, d2enum.RegionAct3Spider,
		d2enum.RegionAct3Dungeon, d2enum.RegionAct3Sewer:
		palettePath = d2resource.PaletteAct3
	case d2enum.RegionAct4Town, d2enum.RegionAct4Mesa, d2enum.RegionAct4Lava, d2enum.RegionAct5Lava:
		palettePath = d2resource.PaletteAct4
	case d2enum.RegonAct5Town, d2enum.RegionAct5Siege, d2enum.RegionAct5Barricade, d2enum.RegionAct5Temple,
		d2enum.RegionAct5IceCaves, d2enum.RegionAct5Baal:
		palettePath = d2resource.PaletteAct5
	default:
		return nil, errors.New("failed to find palette for region")
	}

	mr.loadShadeTable(palettePath)

	return mr.asset.LoadPalette(palettePath)
}

// ViewportToLeft moves the viewport to the left.
func (mr *MapRenderer) ViewportToLeft() {
	mr.viewport.toLeft()
}

// ViewportToRight moves the viewport to the right.
func (mr *MapRenderer) ViewportToRight() {
	mr.viewport.toRight()
}

// ViewportDefault resets the viewport to it's default position.
func (mr *MapRenderer) ViewportDefault() {
	mr.viewport.resetAlign()
}

// SetCameraTarget sets the Camera target
func (mr *MapRenderer) SetCameraTarget(position *d2vector.Position) {
	mr.Camera.SetTarget(position)
}

// SetCameraPosition sets the Camera position
func (mr *MapRenderer) SetCameraPosition(position *d2vector.Position) {
	mr.Camera.MoveTo(position)
}

// InvalidateImageCache the global region image cache. Call this when you are changing regions.
func (mr *MapRenderer) InvalidateImageCache() {
	mr.imageCacheRecords = nil
}

func (mr *MapRenderer) getImageCacheRecord(style, sequence byte, tileType d2enum.TileType, randomIndex byte) d2interface.Surface {
	lookupIndex := uint32(style)<<24 | uint32(sequence)<<16 | uint32(tileType)<<8 | uint32(randomIndex)
	return mr.imageCacheRecords[lookupIndex]
}

func (mr *MapRenderer) setImageCacheRecord(style, sequence byte, tileType d2enum.TileType, randomIndex byte, image d2interface.Surface) {
	lookupIndex := uint32(style)<<24 | uint32(sequence)<<16 | uint32(tileType)<<8 | uint32(randomIndex)

	if mr.imageCacheRecords == nil {
		mr.imageCacheRecords = make(map[uint32]d2interface.Surface)
	}

	mr.imageCacheRecords[lookupIndex] = image

	// DC-002.135: una imagen compuesta es un fallo de cache (ver `contarImagen135`).
	contarImagen135()
}
