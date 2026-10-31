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
	cuadros := flag.Int("cuadros", 1, "cuantos cuadros componer (el 2do en adelante mide la cache ya armada)")
	flag.Parse()

	if err := correr(*dir, *preset, *archivo, *ancho, *alto, *cx, *cy, *salida, *perezoso, *cuadros); err != nil {
		fmt.Println("RENDER294: FALLO --", err)
		os.Exit(1)
	}
}

func correr(dir string, preset, archivo, ancho, alto int, cx, cy float64, salida string,
	perezoso bool, cuadros int) error {
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

	mr := d2maprenderer.CreateMapRenderer(am, r, engine, term, d2util.LogLevelError, cx, cy, opciones...)

	supCache := r.Estadisticas()
	fmt.Printf("RENDER294: CACHE superficies=%d bytesRGBA=%d (%.2f MB) picoPorSuperficie=%dx%d\n",
		supCache.Superficies, supCache.Bytes, float64(supCache.Bytes)/(1024*1024),
		supCache.AnchoMaximo, supCache.AltoMaximo)

	fmt.Println("RENDER294: ================ 4. EL CUADRO ================")

	sup := r.NewSurface(ancho, alto)

	var msTotal int64

	for cuadro := 1; cuadro <= cuadros; cuadro++ {
		antesDeRender := r.Estadisticas()

		t0 := time.Now()
		mr.Render(sup)
		ms := time.Since(t0).Milliseconds()

		despuesDeRender := r.Estadisticas()

		c, ok := sup.(*Cuadro)
		if !ok {
			return fmt.Errorf("la Surface del cuadro no es la de este medidor")
		}

		llenos := c.PixelesNoVacios()
		crudo := c.Pixeles()
		msTotal += ms

		espacio := mr.TamanoDelCuadro()
		fmt.Printf("RENDER294: CUADRO %d tamano=%dx%d pixelesNoVacios=%d de %d crc32=%d ms=%d\n",
			cuadro, espacio.W, espacio.H, llenos, ancho*alto, crc32.ChecksumIEEE(crudo), ms)
		fmt.Printf("RENDER294: CUADRO %d superficiesNuevas=%d bytesNuevos=%d cacheSuperficies=%d cacheBytes=%d\n",
			cuadro, despuesDeRender.Superficies-antesDeRender.Superficies,
			despuesDeRender.Bytes-antesDeRender.Bytes,
			despuesDeRender.Superficies, despuesDeRender.Bytes)
	}

	fmt.Printf("RENDER294: CUADRO total ms=%d (promedio %.1f)\n", msTotal, float64(msTotal)/float64(cuadros))

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
