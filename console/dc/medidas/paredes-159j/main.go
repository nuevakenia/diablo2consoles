// DC-002.159j -- LAS CUATRO FAMILIAS DE PARED Y SUS BLOQUES: el censo que mira lo que el renderer pide, y la
// comparacion de cada tile con el `.dt1` recien descomprimido.
//
// POR QUE EXISTE. DC-002.159i encontro que la consola pide una imagen de pared de 8.394.240 B (`estilo=0 sec=0
// tipo=1`, `minY=-29.648`) mientras el `.dt1` del pueblo declara `Height=-288` para ese mismo tile. El censo
// `sombra-159i` no lo veia por dos motivos distintos, y esta tarjeta cierra los dos:
//
//  1. EL CENSO MIRABA SOLO LOS TIPOS QUE EL `.ds1` DECLARA (en el pueblo, `tipo=14`). El renderer pide cuatro
//     tipos de pared por casilla (izquierda, derecha, y las dos mitades de la esquina norte), asi que un pedido
//     de `tipo=1` no estaba en el censo. Aca las cuatro familias se censan siempre.
//  2. EL CENSO CREIA LOS DATOS DEL MOTOR. Este medidor los COMPARA: el mismo tile, mirado por `GetTiles` (lo
//     que el renderer ve) y por un parseo NUEVO del `.dt1` (los bytes del MPQ descomprimidos otra vez, en un
//     AssetManager aparte). Si los dos coinciden, el dato del nivel esta sano en el host y lo que se rompe es
//     la lectura en la consola.
//
// Uso:  go run ./console/dc/medidas/paredes-159j [-dir <carpeta con los MPQ>] [-top 8]
//
// Que imprime (y que hay que leer):
//
//	PAREDES159J: CENSO 4 TIPOS ... paredes=N imagenes=... maxima=... B  -> el juez de "ninguna pasa de 81.920"
//	PAREDES159J:   tipo= 1 ... (una linea por tipo, con su imagen mas grande)
//	PAREDES159J: COMPARA motor vs .dt1 fresco: coinciden=X difieren=Y
//	PAREDES159J:   DIFIERE estilo=0 sec=0 tipo=1 var=0 motor(minY=.. maxY=.. n=.. real=..) dt1(minY=..)
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
)

// los cuatro tipos de pared que el renderer compone por casilla (d2enum): izquierda, derecha y las dos mitades
// de la esquina norte. El techo (`tipo=14`) NO entra: ese lo pide el `.ds1` directamente.
var losCuatro = []d2enum.TileType{
	d2enum.TileLeftWall,
	d2enum.TileRightWall,
	d2enum.TileRightPartOfNorthCornerWall,
	d2enum.TileLeftPartOfNorthCornerWall,
}

const (
	tileSurfaceWidth = 160 // el ancho FIJO de las paredes (tile_cache.go)
	blockOffsetY     = 32  // tile_cache.go
	techoDelBuffer   = 81920
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

// medida: lo que el renderer saca de un `d2dt1.Tile` para pedir su imagen (tile_cache.go, `generateWallCache`).
type medida struct {
	height int32 // `tileData.Height` declarado
	bloques int
	minY   int32
	maxY   int32
	real   int32 // realHeight = max(|Height|, maxY-minY)
}

func (m medida) imagen() int { return tileSurfaceWidth * int(m.real) }

func (m medida) String() string {
	return fmt.Sprintf("Height=%d bloques=%d minY=%d maxY=%d real=%d (%d B)",
		m.height, m.bloques, m.minY, m.maxY, m.real, m.imagen())
}

// medir: la MISMA cuenta que `generateWallCache`, linea por linea (el `target` es el propio tile si no hay
// pareja de esquina: aca se mide cada tile suelto, que es lo que el censo necesita).
func medir(t *d2dt1.Tile) medida {
	m := medida{height: t.Height, bloques: len(t.Blocks)}

	for _, b := range t.Blocks {
		if int32(b.Y) < m.minY {
			m.minY = int32(b.Y)
		}

		if int32(b.Y)+blockOffsetY > m.maxY {
			m.maxY = int32(b.Y) + blockOffsetY
		}
	}

	m.real = m.maxY - m.minY
	if abs := abs32(t.Height); abs > m.real {
		m.real = abs
	}

	return m
}

func abs32(n int32) int32 {
	if n < 0 {
		return -n
	}

	return n
}

func main() {
	dir := flag.String("dir", "", "carpeta con d2data.mpq y patch_d2.mpq")
	top := flag.Int("top", 8, "cuantas imagenes imprimir por lista")
	flag.Parse()

	if err := correr(*dir, *top); err != nil {
		fmt.Println("PAREDES159J: FALLO --", err)
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

	fmt.Println("PAREDES159J: patch_d2.mpq =", parche)
	fmt.Println("PAREDES159J: d2data.mpq  =", datos)

	am, err := nuevoAssetManager(parche, datos, true)
	if err != nil {
		return err
	}

	engine := d2mapengine.CreateMapEngine(d2util.LogLevelError, am)

	stamp := engine.LoadStamp(d2enum.RegionAct1Town, 1, 0)
	if stamp == nil {
		return fmt.Errorf("LoadStamp devolvio nil")
	}

	tamano := stamp.Size()

	fmt.Printf("PAREDES159J: pueblo %dx%d casillas, dt1s del .ds1 = %d, sizeof(d2dt1.Tile) en el host = %d B\n",
		tamano.Width, tamano.Height, len(stamp.TilesDelDS1()), int(unsafe.Sizeof(d2dt1.Tile{})))

	engine.ResetMapParaStamp(d2enum.RegionAct1Town, tamano.Width, tamano.Height, stamp)
	engine.PlaceStampClippedWhere(stamp, 0, 0, tamano.Width, tamano.Height, func(int) bool { return false }, false)

	// ============================== 1. las familias de pared que el nivel declara ==============================
	//
	// Una familia es `(estilo, secuencia)`. Para cada una se censan los CUATRO tipos (lo que el renderer pide
	// por casilla), se declare el tipo en el `.ds1` o no: es el agujero que tenia el censo de 159i.
	familias := map[[2]int]map[d2enum.TileType]int{} // (estilo,secuencia) -> tipo -> casillas que lo declaran

	tiles := *engine.Tiles()

	for idx := range tiles {
		for i := range tiles[idx].Components.Walls {
			w := &tiles[idx].Components.Walls[i]
			if w.Hidden() || w.Prop1 == 0 || w.Type.Special() {
				continue
			}

			k := [2]int{int(w.Style), int(w.Sequence)}
			if familias[k] == nil {
				familias[k] = map[d2enum.TileType]int{}
			}

			familias[k][w.Type]++
		}
	}

	// ============================== 2. el `.dt1` fresco, en OTRO AssetManager ==============================
	//
	// El motor ya tiene los `.dt1` parseados y cacheados; para comparar hace falta otro parseo de los MISMOS
	// bytes. Es un AssetManager aparte (mismas fuentes, mismo camino de la PC: `LoadDT1` de siempre) y los
	// bloques quedan apuntando al crudo de ESE manager, que no se suelta.
	fresco, err := nuevoAssetManager(parche, datos, false)
	if err != nil {
		return err
	}

	delDT1 := map[[3]int][]*d2dt1.Tile{} // (estilo,secuencia,tipo) -> tiles del .dt1 fresco

	for _, ruta := range stamp.TilesDelDS1() {
		dt1, err := fresco.LoadDT1(ruta)
		if err != nil {
			return fmt.Errorf("LoadDT1(%s): %w", ruta, err)
		}

		for i := range dt1.Tiles {
			t := &dt1.Tiles[i]
			k := [3]int{int(t.Style), int(t.Sequence), int(t.Type)}
			delDT1[k] = append(delDT1[k], t)
		}
	}

	// ============================== 3. el censo de los cuatro tipos ==============================
	claves := make([][2]int, 0, len(familias))
	for k := range familias {
		claves = append(claves, k)
	}

	sort.Slice(claves, func(i, j int) bool {
		if claves[i][0] != claves[j][0] {
			return claves[i][0] < claves[j][0]
		}

		return claves[i][1] < claves[j][1]
	})

	porTipo := map[d2enum.TileType]struct {
		imagenes int
		bytes    int
		maxima   int
		maxCual  string
	}{}

	imagenes, bytes, sobreElTecho := 0, 0, 0
	maxima, cualMaxima := 0, ""

	var difieren, coinciden int

	lista := make([]string, 0, 64)

	for _, fam := range claves {
		estilo, secuencia := fam[0], fam[1]

		for _, tipo := range append([]d2enum.TileType{}, losCuatro...) {
			// el techo tambien se mira si el `.ds1` lo declara: es la pared mas alta del pueblo (76.800 B) y
			// dejarla afuera haria que el censo no fuera el juez de "ninguna imagen pasa de X".
			opciones := engine.GetTiles(estilo, secuencia, tipo)
			if len(opciones) == 0 {
				continue
			}

			for v := range opciones {
				m := medir(&opciones[v])
				if m.real == 0 {
					continue // sin grafico (marcador de colision): el renderer no compone nada
				}

				n := m.imagen()

				imagenes++
				bytes += n

				acc := porTipo[tipo]
				acc.imagenes++
				acc.bytes += n
				if n > acc.maxima {
					acc.maxima = n
					acc.maxCual = fmt.Sprintf("estilo=%d sec=%d tipo=%d var=%d", estilo, secuencia, tipo, v)
				}

				porTipo[tipo] = acc

				if n > maxima {
					maxima = n
					cualMaxima = fmt.Sprintf("estilo=%d sec=%d tipo=%d var=%d %dx%d", estilo, secuencia, tipo, v,
						tileSurfaceWidth, m.real)
				}

				if n > techoDelBuffer {
					sobreElTecho++
					lista = append(lista, fmt.Sprintf("estilo=%d sec=%d tipo=%d var=%d %dx%d = %d B", estilo,
						secuencia, tipo, v, tileSurfaceWidth, m.real, n))
				}

				// ---- la comparacion con el `.dt1` recien descomprimido ----
				delArchivo := delDT1[[3]int{estilo, secuencia, int(tipo)}]
				if v >= len(delArchivo) {
					difieren++
					fmt.Printf("PAREDES159J:   DIFIERE estilo=%d sec=%d tipo=%d var=%d: el .dt1 fresco trae %d "+
						"tiles de esa clave (el motor, %d)\n", estilo, secuencia, tipo, v, len(delArchivo),
						len(opciones))

					continue
				}

				suya := medir(delArchivo[v])
				if suya == m {
					coinciden++

					continue
				}

				difieren++

				fmt.Printf("PAREDES159J:   DIFIERE estilo=%d sec=%d tipo=%d var=%d motor(%s) dt1(%s)\n",
					estilo, secuencia, tipo, v, m, suya)
			}
		}
	}

	fmt.Printf("PAREDES159J: CENSO 4 TIPOS familias=%d imagenes=%d bytes=%d (%.2f MB) sobreElTecho=%d\n",
		len(claves), imagenes, bytes, float64(bytes)/(1024*1024), sobreElTecho)

	tiposOrdenados := make([]d2enum.TileType, 0, len(porTipo))
	for t := range porTipo {
		tiposOrdenados = append(tiposOrdenados, t)
	}

	sort.Slice(tiposOrdenados, func(i, j int) bool { return tiposOrdenados[i] < tiposOrdenados[j] })

	for _, t := range tiposOrdenados {
		acc := porTipo[t]
		fmt.Printf("PAREDES159J:   tipo=%2d %-32s imagenes=%3d bytes=%8d maxima=%6d B (%s)\n", int(t),
			t.String(), acc.imagenes, acc.bytes, acc.maxima, acc.maxCual)
	}

	fmt.Printf("PAREDES159J: la imagen mas grande de las cuatro familias: %d B (%s)\n", maxima, cualMaxima)

	if sobreElTecho == 0 {
		fmt.Printf("PAREDES159J: NINGUNA imagen de las cuatro familias pasa el buffer al vuelo (%d B)\n",
			techoDelBuffer)
	} else {
		fmt.Printf("PAREDES159J: PASAN el buffer al vuelo (%d B): %d imagenes\n", techoDelBuffer, sobreElTecho)

		sort.Slice(lista, func(i, j int) bool { return lista[i] > lista[j] })
		for i, l := range lista {
			if i >= top {
				break
			}

			fmt.Println("PAREDES159J:   ", l)
		}
	}

	fmt.Printf("PAREDES159J: COMPARA motor vs .dt1 fresco: coinciden=%d difieren=%d\n", coinciden, difieren)

	// que tipos declara el `.ds1` (la razon por la que el censo de 159i no veia el `tipo=1`)
	declarados := map[d2enum.TileType]int{}
	for _, fam := range familias {
		for t, n := range fam {
			declarados[t] += n
		}
	}

	tiposDecl := make([]d2enum.TileType, 0, len(declarados))
	for t := range declarados {
		tiposDecl = append(tiposDecl, t)
	}

	sort.Slice(tiposDecl, func(i, j int) bool { return tiposDecl[i] < tiposDecl[j] })

	var partes []string
	for _, t := range tiposDecl {
		partes = append(partes, fmt.Sprintf("tipo=%d (%s) %d casillas", int(t), t.String(), declarados[t]))
	}

	fmt.Printf("PAREDES159J: el .ds1 declara %d claves de tipo: %s\n", len(declarados), strings.Join(partes, ", "))
	fmt.Println("PAREDES159J: FIN")

	return nil
}

// nuevoAssetManager: el arranque de la PC (`NewAssetManager`) o el de la consola (`NuevoAssetManagerDeConsola`,
// que prende el arena de bloques). El fresco es SIEMPRE el de la PC: se compara contra el camino sin arena.
func nuevoAssetManager(parche, datos string, consola bool) (*d2asset.AssetManager, error) {
	var (
		am  *d2asset.AssetManager
		err error
	)

	if consola {
		am, _, err = d2asset.NuevoAssetManagerDeConsola(d2util.LogLevelError, parche, datos)
	} else {
		am, err = d2asset.NewAssetManager(d2util.LogLevelError)

		if err == nil {
			for _, ruta := range []string{parche, datos} {
				if e := am.AddSource(ruta, types.AssetSourceMPQ); e != nil {
					return nil, fmt.Errorf("AddSource(%s): %w", ruta, e)
				}
			}
		}
	}

	if err != nil {
		return nil, err
	}

	for _, ruta := range tablas {
		_ = am.LoadRecords(ruta)
	}

	return am, nil
}

func buscarMPQ(dir, nombre string) string {
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
