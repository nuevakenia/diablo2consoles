package d2maprenderer

// DC-002.135 -- EL MAPA POR TILE: LOS 1,2 ms POR TILE QUE NO SON PIXELES.
//
// DE DONDE VIENE. En DC-002.134 el cuadro del pueblo (Flycast, 1 jugador, HUD y minimapa) son 501 ms y el mapa
// 382.115 us, o sea el 83 %. El backend conto los pixeles que pinta el mapa por el camino de indices:
// 1.539.584 por cuadro (242 tiles, 6.360 px por tile). 382 ms / 1,54 M px = 248 ns por pixel, y el bucle de
// deposito por palabras del backend (un solo almacenamiento de 32 bits por pixel, DC-002.134) se suponia de
// 12-15 ciclos. De ahi la conclusion que esta tarjeta viene a verificar:
//
//	~0,38 ms por tile es el bucle de pixeles  =>  ~1,2 ms por tile se van FUERA del bucle
//
// QUE MIDE ESTE ARCHIVO. Un cronometro por bloque DENTRO del renderer del mapa, con la misma forma que el de
// las etapas del POC (`instrumento_134.go`): se abre una marca antes de cada bloque, se cierra despues, y lo que
// se acumula es el tiempo del bloque en el tramo. Los bloques son los cuatro que el renderer hace POR TILE, en
// el orden en que los paga:
//
//	cache    -- `asegurarImagen` (con la cache perezosa, la primera vez arma la imagen) mas la busqueda de la
//	            imagen en la cache (`getCacheDeIndices` / `getImageCacheRecord`). Es la hipotesis (a) de la
//	            tarjeta: que la cache se rearme por cuadro.
//	vista    -- la cuenta de la posicion en pantalla del tile: `PushTranslationOrtho` (aritmetica de punto
//	            flotante del viewport, que en esta consola va por software) y su `PopTranslation`. Hipotesis (b).
//	estado   -- el estado del `Surface`: `PushTranslation` con la traslacion ya calculada (y su `Pop`), que es
//	            donde se apila el estado de dibujado por tile. Hipotesis (c).
//	pintado  -- el `RenderIndicesTintado` del tile (la luz, si estuviera prendida, tambien cae aca). Es el bucle
//	            de pixeles y lo que lo rodea (la tabla de 256 palabras, el recorte).
//
// Y ADEMAS, por PASADA del renderer (`Render`), para poder cerrar la cuenta sin dejar nada afuera:
//
//	indice   -- `indexEntities` (los bichos y el jugador a sus casillas)
//	fondos   -- `renderBackdrops`
//	p1       -- `renderPass1` (pisos y paredes de abajo)
//	p2       -- `renderPass2` (sombras y entidades bajo las paredes)
//	p3       -- `renderPass3` (paredes de arriba y entidades sobre ellas)
//	p4       -- `renderPass4` (techos)
//
// LA RESTA ES EL DATO. `p1 - (tiles de p1 x lo que cuesta cada uno)` es lo que la pasada paga FUERA de los
// tiles (el `TileAt`, el `PushTranslationWorld`, los bucles). Si `suma(bloques)` no explica `suma(pasadas)` mas
// los fondos y el indice, lo que falta esta en el resto del renderer y no en el tile.
//
// PRECIO DEL INSTRUMENTO: dos llamadas al reloj por bloque, o sea ocho por tile. Medido en la consola, una
// llamada a `d2c_us` cuesta menos de 1 us (`JUEGO135M: RELOJ costo_de_1000_llamadas_us`), asi que el
// instrumento de un cuadro de 242 tiles son ~2 ms: el 0,5 % de los 382. Se resta antes de comparar.
//
// CON EL RELOJ SIN PONER (`FijarRelojDeLosTiles(nil)`, el default) este archivo no cuenta nada y no toca el
// dibujado: las marcas valen 0 y `anotar` corta en la primera comparacion. El binario sin la palanca es el de
// DC-002.134 otra vez.

// Reloj es la funcion que le da la hora al cronometro, en microsegundos. La inyecta el POC (que tiene el
// `d2c_us` de `csrc/shim.c`): el motor no sabe de relojes de consola, y asi este mismo paquete se compila en el
// host (el medidor y los verificadores) sin arrastrar simbolos de C.
type Reloj func() uint

var reloj135 Reloj

// Los bloques POR TILE, en el orden en que el renderer los paga.
const (
	BloqueCache135 = iota
	BloqueVista135
	BloqueEstado135
	BloquePintado135
	// BloqueCacheTiles135 y BloqueCacheBuscar135 son dos PEDAZOS del bloque de la cache, para no tener que
	// adivinar cual de las dos mitades pesa (el `GetTiles` del motor o la busqueda en la cache del renderer):
	//
	//	cache = cacheTiles (GetTiles dentro de generate*Cache) + cacheBuscar (la busqueda posterior)
	//	        + el resto de generate*Cache (el censo, el `Hidden`/`Prop1`, el `append` de la lista)
	BloqueCacheTiles135
	BloqueCacheBuscar135
	bloques135
)

// Las PASADAS del renderer.
const (
	PasoIndice135 = iota
	PasoFondos135
	Paso1Plantas135
	Paso2Sombras135
	Paso3Paredes135
	Paso4Techos135
	pasos135
)

// CensoDeLosTiles: lo acumulado en el tramo, en el orden de las constantes de arriba.
type CensoDeLosTiles struct {
	Cuadros     int
	Tiles       int               // tiles pintados (los mismos que suma `DrawStats`)
	Us          [bloques135]int64 // microsegundos por bloque por tile
	UsPaso      [pasos135]int64   // microsegundos por pasada
	Imagenes    int64             // imagenes de la cache COMPUESTAS en el tramo (fallos de cache)
	SumaPasos   int64
	SumaBloques int64
}

var (
	us135            [bloques135]int64
	usPaso135        [pasos135]int64
	cuadrosCenso135  int
	tilesCenso135    int
	imagenesCenso135 int64
)

// FijarRelojDeLosTiles prende (con una funcion) o apaga (con nil) el cronometro por tile.
func FijarRelojDeLosTiles(r Reloj) { reloj135 = r }

// CronometroDeLosTilesPrendido dice si esta prendido (para el arranque del POC).
func CronometroDeLosTilesPrendido() bool { return reloj135 != nil }

// ahora135: la marca de arranque de un bloque. Devuelve 0 con el cronometro apagado, y el 0 es la senal de
// "no anotes nada" para `anotar135` (una marca de 0 us no existe: el reloj de la consola arranca en el arranque).
func ahora135() int64 {
	if reloj135 == nil {
		return 0
	}

	return int64(reloj135())
}

// anotar135 cierra un bloque abierto con `ahora135`.
func anotar135(bloque int, desde int64) {
	if desde == 0 {
		return
	}

	t := int64(reloj135())
	if t < desde { // el contador de la consola da la vuelta cada ~71 minutos: no se resta un negativo
		return
	}

	us135[bloque] += t - desde
}

// anotarPaso135 cierra una pasada del renderer.
func anotarPaso135(paso int, desde int64) {
	if desde == 0 {
		return
	}

	t := int64(reloj135())
	if t < desde {
		return
	}

	usPaso135[paso] += t - desde
}

// contarTile135: un tile pintado mas en el tramo (lo llaman las cuatro capas, una vez cada una).
func contarTile135() {
	if reloj135 == nil {
		return
	}

	tilesCenso135++
}

// contarImagen135: una imagen COMPUESTA (un fallo de cache). Es la hipotesis (a) hecha numero: si en un tramo de
// 30 cuadros hay 86 imagenes y 242 tiles por cuadro, la cache no se rearma por cuadro; si hay ~7.260, si.
func contarImagen135() {
	if reloj135 == nil {
		return
	}

	imagenesCenso135++
}

// contarCuadro135: el renderer cerro un cuadro (lo llama `Render` al final).
func contarCuadro135() {
	if reloj135 == nil {
		return
	}

	cuadrosCenso135++
}

// CensoDeLosTiles135: una copia de lo acumulado (para imprimir fuera del paquete).
func CensoDeLosTiles135() CensoDeLosTiles {
	c := CensoDeLosTiles{
		Cuadros:  cuadrosCenso135,
		Tiles:    tilesCenso135,
		Us:       us135,
		UsPaso:   usPaso135,
		Imagenes: imagenesCenso135,
	}

	for _, us := range us135 {
		c.SumaBloques += us
	}

	for _, us := range usPaso135 {
		c.SumaPasos += us
	}

	return c
}

// ReiniciarCensoDeLosTiles135 pone el tramo en cero (lo hace el informe despues de imprimirlo).
func ReiniciarCensoDeLosTiles135() {
	us135 = [bloques135]int64{}
	usPaso135 = [pasos135]int64{}
	cuadrosCenso135, tilesCenso135, imagenesCenso135 = 0, 0, 0
}
