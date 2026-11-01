// DC-002.94 -- EL CUADRO DEL PUEBLO EN EL HOST: la referencia contra la que se compara la Dreamcast.
//
// QUE ES. El camino de dibujo del MOTOR (`d2maprenderer`) compone el pueblo del acto 1 sobre una
// `d2interface.Surface`. En la consola esa Surface es la del backend PVR; en el host no habia ninguna que
// se pudiera usar sin ventana (la de `d2core/d2render/ebiten` pide Ebitengine y un contexto de video), asi que
// este medidor trae la suya (`cuadro.go`): un rasterizador de software en Go puro que cumple los 21 metodos
// del contrato (`var _ d2interface.Surface`) y el `Renderer` completo. Con eso el MISMO `MapRenderer`, el
// MISMO `.ds1` y la MISMA camara dan un cuadro de 640x480 que se guarda como PNG: esa es la referencia.
//
// QUE MIDE, en orden:
//
//  1. La composicion del pueblo por el motor (`d2mapengine`): el censo de casillas (pisos/paredes) y el crc32
//     de la grilla de identidades, que tiene que dar el mismo numero que la consola (DC-002.88: -1596781041).
//  2. La CACHE DE TILES del renderer: cuantas superficies pide `generateTileCache` y cuantos bytes son. Es el
//     numero que decide si el camino de dibujo entra en los 16 MB de la consola antes de compilar nada.
//  3. El cuadro: pixeles no vacios, crc32 del cuadro y ms, mas el PNG de referencia.
//
// Uso:  go run ./console/dc/medidas/render-294 [-dir <carpeta con los MPQ>] [-out /tmp/cuadro-host-294.png]
package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2maprenderer"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2term"
)

// tablasDelCompositor: las mismas doce que carga el POC de la consola (console/dc/poc/tablas-mapengine).
// `LoadStamp` lee Presets y Types; `ResetMap` recorre levelType.Files; las demas las pide el camino de
// entidades y mapas automaticos. Se cargan igual para que el cuadro sea el mismo del otro lado.
var tablasDelCompositor = []string{
	d2resource.LevelType,
	d2resource.LevelPreset,
	d2resource.LevelWarp,
	d2resource.AutoMap,
	d2resource.LevelDetails,
	d2resource.LevelMaze,
	d2resource.LevelSubstitutions,
	d2resource.ObjectType,
	d2resource.ObjectDetails,
	d2resource.MonPreset,
	d2resource.MonStats,
	d2resource.MonStats2,
}

// pedazoDelTramo (DC-002.106): si no es 0, el tramo de `-tramo` se pide en bloques de este tamano en vez de
// un solo golpe. Se resuelve con un global y no con un parametro mas de `correr` porque el medidor ya lleva una
// docena y este knob no cambia NADA del camino del cuadro (solo de donde salen sus pixeles).
var pedazoDelTramo int

// cambioDeNivel (DC-002.109): si no es 0, DESPUES de los cuadros de `-cuadros` el medidor hace el cambio de
// nivel del POC de la consola sobre el MISMO motor y el MISMO renderer: suelta la cache del nivel que se deja
// (`SoltarLaCacheDelNivel`, sin liberador: en el host no hay monton externo que devolver), compone el `.ds1`
// de `-cambio-preset`/`-cambio-archivo`, lo cuelga con `SetMapEngine` y dibuja un cuadro mas. Es la
// reproduccion en el host del ciclo de las tres fases: si algo del renderer no aguanta un cambio de nivel, se
// ve aca con la pila de Go completa (en la consola el panic no trae traza: sin frame pointers).
var cambioDeNivel bool
var cambioPreset, cambioArchivo int

func main() {
	dir := flag.String("dir", "", "carpeta con d2data.mpq y patch_d2.mpq (por defecto la instalacion del juego)")
	preset := flag.Int("preset", 1, "preset del nivel en LvlPrest (1 = Act 1 - Town 1)")
	archivo := flag.Int("archivo", 0, "archivo del preset (0 = TownN1.ds1)")
	ancho := flag.Int("ancho", 640, "ancho del cuadro (el de la consola)")
	alto := flag.Int("alto", 480, "alto del cuadro (el de la consola)")
	cx := flag.Float64("cx", 28, "celda X del centro de la camara")
	cy := flag.Float64("cy", 20, "celda Y del centro de la camara")
	salida := flag.String("out", "/tmp/cuadro-host-294.png", "PNG de referencia del cuadro")
	perezoso := flag.Bool("perezoso", false, "cache de tiles perezosa (la de la consola: se arma al dibujar)")
	indices := flag.Bool("indices", false, "cache de tiles en INDICES de 8 bits (1 B/px) en vez de RGBA (4 B/px)")
	tramo := flag.Int("tramo", 0, "DC-002.104: los pixeles de la cache salen de UN tramo de N B (> 64 KB = monton "+
		"de C en la consola); 0 = apagado (make por imagen, el camino de la PC)")
	cuadros := flag.Int("cuadros", 1, "cuantos cuadros componer (el 2do en adelante mide la cache ya armada)")
	histograma := flag.Bool("histograma", false, "censo de tamanos de la cache de indices contra el umbral de 64 KB de libgodc")
	pedazo := flag.Int("pedazo", 0, "DC-002.106: el MISMO tramo de -tramo pero pedido EN BLOQUES de N B (> 64 KB); "+
		"0 = de un solo golpe. En la consola el de un golpe no entra -- cada bloque si se sirve de los huecos")
	cambio := flag.Bool("cambio", false, "DC-002.109: despues de los cuadros, hace el cambio de nivel (soltar la "+
		"cache + componer -cambio-preset/-cambio-archivo + SetMapEngine) y dibuja un cuadro mas")
	presetDelCambio := flag.Int("cambio-preset", 1, "DC-002.109: preset del nivel del cambio")
	archivoDelCambio := flag.Int("cambio-archivo", 1, "DC-002.109: archivo del preset del cambio")
	flag.Parse()

	pedazoDelTramo = *pedazo
	cambioDeNivel = *cambio
	cambioPreset, cambioArchivo = *presetDelCambio, *archivoDelCambio

	if err := correr(*dir, *preset, *archivo, *ancho, *alto, *cx, *cy, *salida, *perezoso, *indices, *tramo,
		*cuadros, *histograma); err != nil {
		fmt.Println("RENDER294: FALLO --", err)
		os.Exit(1)
	}
}

func correr(dir string, preset, archivo, ancho, alto int, cx, cy float64, salida string,
	perezoso, indices bool, tramo int, cuadros int, histograma bool) error {
	if dir == "" {
		dir = autodetectarD2()
	}

	parche, datos, err := losDosMPQ(dir)
	if err != nil {
		return err
	}

	fmt.Println("RENDER294: patch_d2.mpq =", parche)
	fmt.Println("RENDER294: d2data.mpq  =", datos)

	am, err := d2asset.NewAssetManager(d2util.LogLevelError)
	if err != nil {
		return fmt.Errorf("NewAssetManager: %w", err)
	}

	for _, ruta := range []string{parche, datos} {
		if err := am.AddSource(ruta, types.AssetSourceMPQ); err != nil {
			return fmt.Errorf("AddSource(%s): %w", ruta, err)
		}
	}

	fmt.Println("RENDER294: ================ 1. LAS TABLAS ================")

	for _, ruta := range tablasDelCompositor {
		if err := am.LoadRecords(ruta); err != nil {
			fmt.Printf("RENDER294: TABLA %-58s FALLO: %v\n", ruta, err)

			continue
		}

		fmt.Printf("RENDER294: TABLA %-58s OK (types=%d presets=%d)\n", ruta,
			len(am.Records.Level.Types), len(am.Records.Level.Presets))
	}

	fmt.Println("RENDER294: ================ 2. EL PUEBLO POR EL MOTOR ================")

	engine := d2mapengine.CreateMapEngine(d2util.LogLevelError, am)

	stamp := engine.LoadStamp(d2enum.RegionAct1Town, preset, archivo)
	if stamp == nil {
		return fmt.Errorf("LoadStamp devolvio nil (preset %d archivo %d)", preset, archivo)
	}

	tamano := stamp.Size()
	fmt.Printf("RENDER294: LoadStamp OK ds1 w=%d h=%d dt1sDelDS1=%d\n", tamano.Width, tamano.Height,
		len(stamp.TilesDelDS1()))

	tComposicion := time.Now()

	engine.ResetMapParaStamp(d2enum.RegionAct1Town, tamano.Width, tamano.Height, stamp)
	engine.PlaceStampClippedWhere(stamp, 0, 0, tamano.Width, tamano.Height, func(int) bool { return false }, false)

	msComposicion := time.Since(tComposicion).Milliseconds()

	pisos, paredes, crcGrilla := censo(engine)
	fmt.Printf("RENDER294: MAPA tiles=%d pisos=%d paredes=%d crcGrilla=%d ms=%d\n",
		len(*engine.Tiles()), pisos, paredes, crcGrilla, msComposicion)

	fmt.Println("RENDER294: ================ 3. EL RENDERER DEL MAPA Y SU CACHE ================")

	r := NuevoRendererHost()

	term, err := d2term.NewTerminal()
	if err != nil {
		return fmt.Errorf("NewTerminal: %w", err)
	}

	// la cache de tiles se arma ACA adentro (generateTileCache): lo que pida el renderer es exactamente lo
	// que pediria en la consola.
	var opciones []d2maprenderer.Opcion
	opciones = append(opciones, d2maprenderer.ComoConsola(ancho, alto))

	if perezoso {
		opciones = append(opciones, d2maprenderer.CacheDeTilesPerezosa())
		fmt.Println("RENDER294: cache PEREZOSA (la de la consola: las imagenes se arman al dibujar)")
	}

	if indices {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnIndices())
		fmt.Println("RENDER294: cache en INDICES (DC-002.101: 1 B por pixel en vez de 4)")
	}

	if tramo > 0 {
		if pedazoDelTramo > 0 {
			opciones = append(opciones, d2maprenderer.CacheDeTilesEnTramoEnPedazos(tramo, pedazoDelTramo))
			fmt.Printf("RENDER294: cache en TRAMO EN PEDAZOS techo=%d B pedazo=%d B (DC-002.106: cada bloque se "+
				"sirve de los huecos del monton de C)\n", tramo, pedazoDelTramo)
		} else {
			opciones = append(opciones, d2maprenderer.CacheDeTilesEnUnTramo(tramo))
			fmt.Printf("RENDER294: cache en UN TRAMO de %d B (DC-002.104: un solo make grande; en la consola es el "+
				"monton de C y no el semiespacio)\n", tramo)
		}
	}

	mr := d2maprenderer.CreateMapRenderer(am, r, engine, term, d2util.LogLevelError, cx, cy, opciones...)

	supCache := r.Estadisticas()
	fmt.Printf("RENDER294: CACHE superficies=%d bytesRGBA=%d (%.2f MB) picoPorSuperficie=%dx%d\n",
		supCache.Superficies, supCache.Bytes, float64(supCache.Bytes)/(1024*1024),
		supCache.AnchoMaximo, supCache.AltoMaximo)

	cacheInicial := mr.EstadoDeLaCache()
	fmt.Printf("RENDER294: CACHE101 enIndices=%v superficies=%d bytes=%d (%.2f MB) pixeles=%d\n",
		cacheInicial.EnIndices, cacheInicial.Superficies, cacheInicial.Bytes,
		float64(cacheInicial.Bytes)/(1024*1024), cacheInicial.Pixeles)

	fmt.Println("RENDER294: ================ 4. EL CUADRO ================")

	sup := r.NewSurface(ancho, alto)

	var msTotal int64
	var crcPrimero uint32

	for cuadro := 1; cuadro <= cuadros; cuadro++ {
		antesDeRender := r.Estadisticas()
		antes101 := mr.EstadoDeLaCache()

		t0 := time.Now()
		mr.Render(sup)
		ms := time.Since(t0).Milliseconds()

		despuesDeRender := r.Estadisticas()
		despues101 := mr.EstadoDeLaCache()

		c, ok := sup.(*Cuadro)
		if !ok {
			return fmt.Errorf("la Surface del cuadro no es la de este medidor")
		}

		llenos := c.PixelesNoVacios()
		crudo := c.Pixeles()
		msTotal += ms

		espacio := mr.TamanoDelCuadro()
		if cuadro == 1 {
			crcPrimero = crc32.ChecksumIEEE(crudo)
		}

		fmt.Printf("RENDER294: CUADRO %d tamano=%dx%d pixelesNoVacios=%d de %d crc32=%d crc32RGB=%d ms=%d\n",
			cuadro, espacio.W, espacio.H, llenos, ancho*alto, crc32.ChecksumIEEE(crudo), crcSoloRGB(crudo), ms)
		fmt.Printf("RENDER294: CUADRO %d superficiesNuevas=%d bytesNuevos=%d cacheSuperficies=%d cacheBytes=%d\n",
			cuadro, despuesDeRender.Superficies-antesDeRender.Superficies,
			despuesDeRender.Bytes-antesDeRender.Bytes,
			despuesDeRender.Superficies, despuesDeRender.Bytes)
		fmt.Printf("RENDER294: CUADRO %d cache101 nuevas=%d bytesNuevos=%d cacheSuperficies=%d cacheBytes=%d (%.2f MB)\n",
			cuadro, despues101.Superficies-antes101.Superficies, despues101.Bytes-antes101.Bytes,
			despues101.Superficies, despues101.Bytes, float64(despues101.Bytes)/(1024*1024))
	}

	fmt.Printf("RENDER294: CUADRO total ms=%d (promedio %.1f)\n", msTotal, float64(msTotal)/float64(cuadros))

	tramoEstado := mr.CensoDelTramo()
	fmt.Printf("RENDER294: TRAMO104 activa=%v bytes=%d usado=%d libre=%d imagenes=%d derrames=%d bytesDerramados=%d "+
		"enPedazos=%v pedazos=%d techo=%d\n",
		tramoEstado.Activa, tramoEstado.Bytes, tramoEstado.Usado, tramoEstado.Libre, tramoEstado.Imagenes,
		tramoEstado.Derrames, tramoEstado.BytesDerramados, tramoEstado.EnPedazos, tramoEstado.Pedazos,
		tramoEstado.Techo)

	if histograma {
		fmt.Print(censoDeLaCache(mr.TamanosDeLaCache()))
		fmt.Print(listaDeTamanos(mr.TamanosDeLaCache()))
	}

	if cambioDeNivel {
		if err := elCambioDeNivel(mr, engine, sup.(*Cuadro), ancho, alto, crcPrimero); err != nil {
			return err
		}
	}

	final, ok := sup.(*Cuadro)
	if !ok {
		return fmt.Errorf("la Surface del cuadro no es la de este medidor")
	}

	if err := escribirPNG(salida, final); err != nil {
		return err
	}

	fmt.Println("RENDER294: PNG de referencia =", salida)
	fmt.Println("RENDER294: FIN")

	return nil
}

// elCambioDeNivel (DC-002.109): el ciclo del POC de la consola, en el host y con las mismas llamadas que hace
// `fases_109.go`: soltar la cache del nivel que se deja, componer el `.ds1` nuevo sobre el mismo motor,
// colgarlo con `SetMapEngine` y dibujar un cuadro con el lienzo limpio. Sin liberador de bloques (`nil`): en
// este medidor el tramo es un `make` de Go y lo devuelve el recolector.
//
// LO QUE MIDE: el crc32 del cuadro del nivel nuevo y el del nivel de vuelta (los dos se imprimen), que es lo
// que en la consola no se pudo leer porque el panic no trae traza.
func elCambioDeNivel(mr *d2maprenderer.MapRenderer, engine *d2mapengine.MapEngine, sup *Cuadro,
	ancho, alto int, crcDelPrimero uint32) error {
	fmt.Println("RENDER294: ================ 5. EL CAMBIO DE NIVEL (DC-002.109) ================")

	// DC-002.109: el POC de la consola suelta el ARENA DE BLOQUES despues del cuadro 1 (contrato de
	// DC-002.101: con la cache armada ya nadie mira los tiles crudos). El liberador de aca NO libera nada (en
	// el host esa memoria la devuelve el recolector), pero el CAMINO se ejecuta igual: `arenaDeBloques = nil`,
	// cursores en cero, `bloquesUsados = false` y el buffer prestado tirado -- que es lo que deja el estado en
	// el que el nivel nuevo se tiene que volver a componer ENTERO. Sin esto el medidor no ve el mismo camino
	// que la consola.
	d2asset.UsarLiberadorDeExternos(func(uintptr) {})

	tramos, bytesDelArena := d2asset.SoltarArenaDeBloques()
	fmt.Printf("RENDER294: CAMBIO soltarElArena tramos=%d bytes=%d (liberador no-op: es el CAMINO, no la memoria)\n",
		tramos, bytesDelArena)

	soltado := mr.SoltarLaCacheDelNivel(nil)
	e := mr.CensoDelTramo()
	fmt.Printf("RENDER294: CAMBIO soltarLaCache bytes=%d (pedazos=%d usado=%d) -- sin liberador: los bloques se "+
		"reusan desde el byte 0\n", soltado, e.Pedazos, e.Usado)

	for _, fase := range []struct {
		nombre  string
		preset  int
		archivo int
	}{
		{"TRANSICION", cambioPreset, cambioArchivo},
		{"VOLVER", 1, 0},
	} {
		stamp := engine.LoadStamp(d2enum.RegionAct1Town, fase.preset, fase.archivo)
		if stamp == nil {
			return fmt.Errorf("el cambio de nivel: LoadStamp nil (%d/%d)", fase.preset, fase.archivo)
		}

		tamano := stamp.Size()
		engine.ResetMapParaStamp(d2enum.RegionAct1Town, tamano.Width, tamano.Height, stamp)
		engine.PlaceStampClippedWhere(stamp, 0, 0, tamano.Width, tamano.Height, func(int) bool { return false }, false)

		pisos, paredes, crcGrilla := censo(engine)
		fmt.Printf("RENDER294: CAMBIO %s ds1=%s w=%d h=%d pisos=%d paredes=%d crcGrilla=%d\n", fase.nombre,
			stamp.RegionPath(), tamano.Width, tamano.Height, pisos, paredes, crcGrilla)

		mr.SetMapEngine(engine)

		// el lienzo limpio: el del cuadro 1 arranco en cero, y el crc32 solo se compara contra la misma
		// condicion (DC-002.109: la consola borra el suyo con ClearRGBA(0,0,0,0))
		for i := range sup.Pixeles() {
			sup.Pixeles()[i] = 0
		}

		t0 := time.Now()
		mr.Render(sup)
		ms := time.Since(t0).Milliseconds()

		cache := mr.EstadoDeLaCache()
		tramo := mr.CensoDelTramo()
		fmt.Printf("RENDER294: CAMBIO %s CUADRO pixelesNoVacios=%d de %d crc32=%d crc32RGB=%d ms=%d "+
			"cacheSuperficies=%d cacheBytes=%d tramoUsado=%d pedazos=%d\n", fase.nombre, sup.PixelesNoVacios(),
			ancho*alto, crc32.ChecksumIEEE(sup.Pixeles()), crcSoloRGB(sup.Pixeles()), ms, cache.Superficies,
			cache.Bytes, tramo.Usado, tramo.Pedazos)

		if fase.nombre == "VOLVER" {
			fmt.Printf("RENDER294: CAMBIO VOLVER contra ENTRAR: crc32 %d contra %d (igual=%v)\n",
				crc32.ChecksumIEEE(sup.Pixeles()), crcDelPrimero, crc32.ChecksumIEEE(sup.Pixeles()) == crcDelPrimero)
		}
	}

	return nil
}

// crcSoloRGB (DC-002.106): el crc32 del cuadro mirando SOLO los canales de color, sin el alfa. Es el mismo
// instrumento del POC de la consola: si el RGB coincide entre los dos lados y el RGBA no, lo que difiere es el
// canal alfa de la mezcla y no el dibujo.
func crcSoloRGB(pixeles []byte) uint32 {
	dig := crc32.NewIEEE()

	for j := 0; j+3 < len(pixeles); j += 4 {
		_, _ = dig.Write(pixeles[j : j+3])
	}

	return dig.Sum32()
}

// listaDeTamanos (DC-002.106): los tamanos de las 86 imagenes de la cache, en una sola linea con comas.
//
// POR QUE HACE FALTA. El tramo de la cache en PEDAZOS (DC-002.106) reparte las imagenes en bloques de `pedazo`
// B; cuanto se desperdicia en la cola de cada bloque depende del TAMANO de cada imagen y del ORDEN en que se
// piden, y eso no se puede adivinar: se simula con los tamanos de verdad. Esta linea es la entrada de esa
// simulacion (y la prueba de que la consola pide los mismos 86 tamanos).
func listaDeTamanos(tamanos []int) string {
	partes := make([]string, len(tamanos))
	for i, t := range tamanos {
		partes[i] = strconv.Itoa(t)
	}

	return fmt.Sprintf("RENDER294: TAMANOS101 n=%d bytes=%s\n", len(tamanos), strings.Join(partes, ","))
}

// censoDeLaCache: los tamanos de las imagenes de la cache contra el umbral de 64 KB de libgodc.
//
// POR QUE IMPORTA EL UMBRAL. En libgodc (`runtime/gc_heap.c:116`) toda asignacion de mas de
// `GC_LARGE_OBJECT_THRESHOLD_KB` (64 KB) va a `gc_external_alloc` = monton de C, que el recolector NO toca ni
// devuelve; las de menos van al SEMIESPACIO del recolector, que en la consola del pueblo mide 1.792 KB
// (vendor de DC-002.102) y por lo tanto es el techo REAL de lo que puede vivir ahi. Este censo dice cuantas
// imagenes caen de cada lado, que es lo que decide si la cache puede vivir en el semiespacio o tiene que
// pedirse en un tramo propio (ADR 0001).
func censoDeLaCache(tamanos []int) string {
	const umbral = 64 * 1024

	chicas, bytesChicas, grandes, bytesGrandes, mayor := 0, 0, 0, 0, 0

	for _, t := range tamanos {
		if t > umbral {
			grandes++
			bytesGrandes += t
		} else {
			chicas++
			bytesChicas += t
		}

		if t > mayor {
			mayor = t
		}
	}

	return fmt.Sprintf("RENDER294: CENSO101 total=%d bytes=%d | enSemiespacio(<64KB)=%d bytes=%d | "+
		"externas(>64KB)=%d bytes=%d | mayor=%d\n",
		len(tamanos), bytesChicas+bytesGrandes, chicas, bytesChicas, grandes, bytesGrandes, mayor)
}

// escribirPNG: el cuadro a disco, para compararlo despues contra la captura de Flycast.
func escribirPNG(ruta string, c *Cuadro) error {
	f, err := os.Create(ruta)
	if err != nil {
		return fmt.Errorf("creando %s: %w", ruta, err)
	}

	defer func() { _ = f.Close() }()

	if err := png.Encode(f, c.Screenshot()); err != nil {
		return fmt.Errorf("codificando el PNG: %w", err)
	}

	return nil
}

// censo: pisos y paredes del mapa compuesto, mas el crc32 de la grilla de identidades (estilo, secuencia,
// tipo, prop1 de la primera capa de piso y de la primera de pared). Es el MISMO numero que reporta la consola
// (DC-002.88: -1596781041 para el pueblo), asi la comparacion de pixeles se hace sobre el mismo mapa.
func censo(engine *d2mapengine.MapEngine) (pisos, paredes int, crc uint32) {
	var buf [8]byte

	juego := *engine.Tiles()
	tabla := crc32.MakeTable(crc32.IEEE)
	acumulado := uint32(0)

	for i := range juego {
		t := &juego[i]

		// las OCHO posiciones se escriben siempre, tambien las del componente que no existe (van en cero):
		// es la misma cuenta del censo de la consola (console/dc/poc/tablas-mapengine), para que el crc32
		// se pueda comparar numero a numero entre los dos lados.
		for j := range buf {
			buf[j] = 0
		}

		if len(t.Components.Floors) > 0 {
			fl := &t.Components.Floors[0]
			buf[0], buf[1], buf[2], buf[3] = fl.Style, fl.Sequence, byte(fl.Type), fl.Prop1

			if !fl.Hidden() && fl.Prop1 != 0 {
				pisos++
			}
		}

		if len(t.Components.Walls) > 0 {
			w := &t.Components.Walls[0]
			buf[4], buf[5], buf[6], buf[7] = w.Style, w.Sequence, byte(w.Type), w.Prop1

			if !w.Hidden() && w.Prop1 != 0 {
				paredes++
			}
		}

		acumulado = crc32.Update(acumulado, tabla, buf[:])
	}

	return pisos, paredes, acumulado
}

// losDosMPQ: las dos fuentes que el compositor necesita (las tablas en el parche, los .ds1/.dt1 en los datos),
// con el nombre real del archivo en el disco (el juego los tiene en mayusculas o minusculas segun la copia).
func losDosMPQ(dir string) (string, string, error) {
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return "", "", fmt.Errorf("listando %s: %w", dir, err)
	}

	enDisco := map[string]string{}

	for _, e := range entradas {
		enDisco[strings.ToLower(e.Name())] = e.Name()
	}

	buscar := func(nombre string) (string, error) {
		real, hay := enDisco[nombre]
		if !hay {
			return "", fmt.Errorf("no hay %s en %s", nombre, dir)
		}

		return filepath.Join(dir, real), nil
	}

	parche, err := buscar("patch_d2.mpq")
	if err != nil {
		return "", "", err
	}

	datos, err := buscar("d2data.mpq")
	if err != nil {
		return "", "", err
	}

	return parche, datos, nil
}

// autodetectarD2: la instalacion de siempre en este host (la misma de los otros medidores).
func autodetectarD2() string {
	candidatas := []string{
		filepath.Join(os.Getenv("HOME"), ".wine-d2/drive_c/Games/Diablo II"),
	}

	for _, c := range candidatas {
		if entradas, err := os.ReadDir(c); err == nil && len(entradas) > 0 {
			return c
		}
	}

	return candidatas[0]
}
