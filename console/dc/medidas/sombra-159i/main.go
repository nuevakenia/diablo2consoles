// DC-002.159i -- QUE IMAGEN ES LA QUE NO ENTRA: el censo de TODO lo que el renderer del mapa pediria.
//
// POR QUE EXISTE. DC-002.159h dejo el runtime sano y la corrida del Gate 0 igual muere, ~50 s, en la pantalla
// del mapa, con un pedido de imagen de 4.737.760 B que no entra en la RAM de la consola (`makeslice: too
// large` / `gc_external_alloc: OOM` desde `MapRenderer.bufferAlVuelo`). Este medidor contesta, sobre los
// MISMOS datos del pueblo y sin emulador, la pregunta de la tarjeta: QUE imagen es (estilo/secuencia/tipo/
// variante), cuanto mide, DONDE esta en el pueblo y cuantas casillas la piden.
//
// NO USA EL RENDERER. Repite las tres cuentas de tamano de `tile_cache.go` (`generateFloorCache`,
// `generateShadowCache`, `generateWallCache`) sobre el `.dt1` del nivel y los censos del motor. Es a proposito:
// el renderer pide la imagen al dibujar y para reproducir la corrida habria que correr la caminata entera; el
// tamano, en cambio, sale de los datos y no de la camara. La cuenta es la MISMA linea por linea:
//
//	piso    w = tiles[RandomIndex].Width           h = abs(Height)
//	sombra  w = tiles[RandomIndex].Width           h = maxY-minY sobre los bloques (+32 c/u)
//	pared   w = 160 (tileSurfaceWidth)             h = max(abs(Height), maxY-minY)
//
// y `n = w*h` es exactamente lo que se le pasa a `pedirTrozoDeIndices` (1 B por pixel en la cache de indices).
//
// Uso:  go run ./console/dc/medidas/sombra-159i [-dir <carpeta con los MPQ>] [-top 12]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
)

var tablas = []string{
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

const (
	tileSurfaceWidth = 160 // el ancho FIJO de las paredes y techos (tile_cache.go)
	blockOffsetY     = 32
	tileMinHeight    = 32
	shadowAdjustY    = 80
)

// imagen: una imagen que el renderer pediria, con su clave de cache y sus tamanos.
type imagen struct {
	que      string // "piso", "sombra" o "pared"
	style    int
	seq      int
	tipo     int
	variant  int
	ancho    int
	alto     int
	yAjuste  int
	casillas int // cuantas casillas del pueblo la piden
	ejemplo  string
}

func (im imagen) bytes() int { return im.ancho * im.alto }

func main() {
	dir := flag.String("dir", "", "carpeta con d2data.mpq y patch_d2.mpq")
	top := flag.Int("top", 12, "cuantas imagenes imprimir por lista")
	flag.Parse()

	if err := correr(*dir, *top); err != nil {
		fmt.Println("SOMBRA159I: FALLO --", err)
		os.Exit(1)
	}
}

func correr(dir string, top int) error {
	if dir == "" {
		dir = autodetectarD2()
	}

	parche := buscarMPQ(dir, "patch_d2.mpq")
	datos := buscarMPQ(dir, "d2data.mpq")
	if parche == "" || datos == "" {
		return fmt.Errorf("faltan los MPQ en %q (patch_d2.mpq=%q d2data.mpq=%q)", dir, parche, datos)
	}

	fmt.Println("SOMBRA159I: patch_d2.mpq =", parche)
	fmt.Println("SOMBRA159I: d2data.mpq  =", datos)

	am, err := d2asset.NewAssetManager(d2util.LogLevelError)
	if err != nil {
		return fmt.Errorf("NewAssetManager: %w", err)
	}

	for _, ruta := range []string{parche, datos} {
		if err := am.AddSource(ruta, types.AssetSourceMPQ); err != nil {
			return fmt.Errorf("AddSource(%s): %w", ruta, err)
		}
	}

	for _, ruta := range tablas {
		_ = am.LoadRecords(ruta)
	}

	engine := d2mapengine.CreateMapEngine(d2util.LogLevelError, am)

	stamp := engine.LoadStamp(d2enum.RegionAct1Town, 1, 0)
	if stamp == nil {
		return fmt.Errorf("LoadStamp devolvio nil")
	}

	tamano := stamp.Size()
	fmt.Printf("SOMBRA159I: pueblo %dx%d casillas, dt1s del .ds1 = %d\n", tamano.Width, tamano.Height,
		len(stamp.TilesDelDS1()))

	engine.ResetMapParaStamp(d2enum.RegionAct1Town, tamano.Width, tamano.Height, stamp)
	engine.PlaceStampClippedWhere(stamp, 0, 0, tamano.Width, tamano.Height, func(int) bool { return false }, false)

	tiles := *engine.Tiles()

	vistas := map[string]*imagen{}
	fuera := map[string]int{} // variantes que se salen del rango del .dt1 (la imagen NO existe)
	ejFuera := map[string]string{}

	anotarFuera := func(que string, style, seq, tipo, variant, opciones, x, y int) {
		clave := fmt.Sprintf("%-6s estilo=%3d sec=%3d tipo=%3d var=%2d (opciones=%d)", que, style, seq, tipo,
			variant, opciones)
		fuera[clave]++

		if _, ok := ejFuera[clave]; !ok {
			ejFuera[clave] = fmt.Sprintf("(%d,%d)", x, y)
		}
	}

	anotar := func(im imagen, x, y int) {
		clave := fmt.Sprintf("%s|%d|%d|%d|%d", im.que, im.style, im.seq, im.tipo, im.variant)

		if v, ok := vistas[clave]; ok {
			v.casillas++

			return
		}

		im.casillas = 1
		im.ejemplo = fmt.Sprintf("(%d,%d)", x, y)
		vistas[clave] = &im
	}

	for idx := range tiles {
		tile := &tiles[idx]
		x, y := idx%tamano.Width, idx/tamano.Width

		for i := range tile.Components.Floors {
			f := &tile.Components.Floors[i]
			if f.Hidden() || f.Prop1 == 0 {
				continue
			}

			opciones := engine.GetTiles(int(f.Style), int(f.Sequence), 0)
			if opciones == nil {
				continue
			}

			if int(f.RandomIndex) >= len(opciones) {
				anotarFuera("piso", int(f.Style), int(f.Sequence), 0, int(f.RandomIndex), len(opciones), x, y)

				continue
			}

			d := &opciones[f.RandomIndex]
			ancho := int(d.Width)
			alto := absInt(int(d.Height))
			anotar(imagen{que: "piso", style: int(f.Style), seq: int(f.Sequence), tipo: 0,
				variant: int(f.RandomIndex), ancho: ancho, alto: alto}, x, y)
		}

		for i := range tile.Components.Shadows {
			s := &tile.Components.Shadows[i]
			if s.Hidden() || s.Prop1 == 0 {
				continue
			}

			opciones := engine.GetTiles(int(s.Style), int(s.Sequence), d2enum.TileShadow)
			if opciones == nil {
				continue
			}

			if int(s.RandomIndex) >= len(opciones) {
				anotarFuera("sombra", int(s.Style), int(s.Sequence), int(d2enum.TileShadow), int(s.RandomIndex),
					len(opciones), x, y)

				continue
			}

			d := &opciones[s.RandomIndex]
			if d.Width == 0 || d.Height == 0 {
				continue
			}

			minY, maxY := int32(0), int32(0)

			for _, b := range d.Blocks {
				if int32(b.Y) < minY {
					minY = int32(b.Y)
				}

				if int32(b.Y)+tileMinHeight > maxY {
					maxY = int32(b.Y) + tileMinHeight
				}
			}

			anotar(imagen{que: "sombra", style: int(s.Style), seq: int(s.Sequence),
				tipo: int(d2enum.TileShadow), variant: int(s.RandomIndex),
				ancho: int(d.Width), alto: int(maxY - minY), yAjuste: int(minY + shadowAdjustY)}, x, y)
		}

		for i := range tile.Components.Walls {
			w := &tile.Components.Walls[i]
			if w.Hidden() || w.Prop1 == 0 || w.Type.Special() {
				continue
			}

			opciones := engine.GetTiles(int(w.Style), int(w.Sequence), w.Type)
			if opciones == nil {
				continue
			}

			if int(w.RandomIndex) >= len(opciones) {
				anotarFuera("pared", int(w.Style), int(w.Sequence), int(w.Type), int(w.RandomIndex),
					len(opciones), x, y)

				continue
			}

			d := &opciones[w.RandomIndex]

			minY, maxY := int32(0), int32(0)

			for _, b := range d.Blocks {
				if int32(b.Y) < minY {
					minY = int32(b.Y)
				}

				if int32(b.Y)+blockOffsetY > maxY {
					maxY = int32(b.Y) + blockOffsetY
				}
			}

			alto := absInt(int(d.Height))
			if rango := int(maxY - minY); rango > alto {
				alto = rango
			}

			if alto == 0 {
				continue
			}

			anotar(imagen{que: "pared", style: int(w.Style), seq: int(w.Sequence), tipo: int(w.Type),
				variant: int(w.RandomIndex), ancho: tileSurfaceWidth, alto: alto}, x, y)
		}
	}

	lista := make([]*imagen, 0, len(vistas))
	total, porTipo := 0, map[string]int{}
	cuantas := map[string]int{}

	for _, im := range vistas {
		lista = append(lista, im)
		total += im.bytes()
		porTipo[im.que] += im.bytes()
		cuantas[im.que]++
	}

	sort.Slice(lista, func(i, j int) bool { return lista[i].bytes() > lista[j].bytes() })

	fmt.Printf("SOMBRA159I: CENSO imagenes=%d bytes=%d (%.2f MB)\n", len(lista), total,
		float64(total)/(1024*1024))

	for _, que := range []string{"piso", "sombra", "pared"} {
		fmt.Printf("SOMBRA159I:   %-7s %3d imagenes %9d B (%.2f MB)\n", que, cuantas[que], porTipo[que],
			float64(porTipo[que])/(1024*1024))
	}

	fmt.Printf("SOMBRA159I: LAS %d MAS GRANDES\n", top)

	for i, im := range lista {
		if i >= top {
			break
		}

		fmt.Printf("SOMBRA159I:   %2d. %-6s estilo=%3d sec=%3d tipo=%3d var=%2d  %dx%d = %d B  casillas=%d  ej=%s\n",
			i+1, im.que, im.style, im.seq, im.tipo, im.variant, im.ancho, im.alto, im.bytes(), im.casillas,
			im.ejemplo)
	}

	// el corte que importa: lo que pasa el buffer al vuelo del Gate 0 (81.920 B) y lo que pasa el umbral del
	// objeto grande de libgodc (64 KB, o sea lo que se va al monton de C y no al semiespacio).
	sobre := func(limite int) (int, int) {
		n, b := 0, 0

		for _, im := range lista {
			if im.bytes() > limite {
				n++
				b += im.bytes()
			}
		}

		return n, b
	}

	n, b := sobre(81920)
	fmt.Printf("SOMBRA159I: sobre el buffer al vuelo (81.920 B): %d imagenes, %d B\n", n, b)

	n, b = sobre(64 * 1024)
	fmt.Printf("SOMBRA159I: sobre el objeto grande de libgodc (64 KB): %d imagenes, %d B\n", n, b)

	// LAS VARIANTES FUERA DE RANGO -- la hipotesis de por que la consola pide una imagen que el host no ve.
	// `generateWallCache`/`generateShadowCache` indexan `tileOptions[tile.RandomIndex]` SIN comprobar el rango
	// (ver DC-002.109: el `[variante]` que se sale del rango es un dato del nivel, no un defecto del motor).
	// En la PC eso es un panic; en la consola, si el indice no se comprueba, es LEER MEMORIA AJENA: un
	// `Width`/`Height` de basura y un pedido de imagen que no es de ninguna imagen.
	if len(fuera) == 0 {
		fmt.Println("SOMBRA159I: VARIANTES fuera de rango: ninguna (todas las casillas caen dentro de su .dt1)")
	} else {
		claves := make([]string, 0, len(fuera))
		totalCasillas := 0

		for k := range fuera {
			claves = append(claves, k)
			totalCasillas += fuera[k]
		}

		sort.Strings(claves)

		fmt.Printf("SOMBRA159I: VARIANTES fuera de rango: %d claves, %d casillas\n", len(claves), totalCasillas)

		for _, k := range claves {
			fmt.Printf("SOMBRA159I:   %s  casillas=%d  ej=%s\n", k, fuera[k], ejFuera[k])
		}
	}

	fmt.Println("SOMBRA159I: FIN")

	return nil
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}

	return n
}

func buscarMPQ(dir, nombre string) string {
	// OJO CON LAS MAYUSCULAS: la instalacion del juego trae `patch_D2.mpq` y `d2data.mpq`, y en Linux el
	// nombre es lo que es (medido: pedir "patch_d2.mpq" a secas devuelve vacio y el medidor se planta).
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, e := range entradas {
		if strings.EqualFold(e.Name(), nombre) {
			return filepath.Join(dir, e.Name())
		}
	}

	return ""
}

func autodetectarD2() string {
	candidatos := []string{
		os.Getenv("D2_DIR"),
		filepath.Join(os.Getenv("HOME"), ".wine-d2/drive_c/Games/Diablo II"),
		filepath.Join(os.Getenv("HOME"), "Games/diablo2"),
	}

	for _, c := range candidatos {
		if c == "" {
			continue
		}

		if buscarMPQ(c, "d2data.mpq") != "" && buscarMPQ(c, "patch_d2.mpq") != "" {
			return c
		}
	}

	return ""
}
