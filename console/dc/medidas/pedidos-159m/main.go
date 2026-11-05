// DC-002.159m -- QUE PIDE EL RENDERER Y QUE TIENE LA TABLA COMPACTADA.
//
// EL DEFECTO QUE MIDE. La consola carga los `.dt1` del nivel por el camino del arena de bloques usados, que
// copia SOLO los tiles cuya clave `(estilo, secuencia, tipo)` esta en `Stamp.ClavesDeTiles()` -- o sea la que
// el `.ds1` DECLARA por casilla. Pero el renderer no pide solo eso: pide el tipo de la pared declarada, la
// OTRA mitad de la esquina norte (`tipo=3` pide `tipo=4`), y las sombras (`tipo=13`) que vienen de las capas
// de sombra del `.ds1`. Este medidor levanta las tres demandas sobre el pueblo ya compuesto y las clasifica
// contra el `.dt1` CRUDO (otro AssetManager, otro parseo):
//
//	el .dt1 lo tiene y la tabla compactada NO -> perdida de grafico (el `Unknown tile ID` de la consola)
//	el .dt1 tampoco lo tiene                  -> pedido legitimo sin grafico (nada que arreglar)
//
// Y dice cuanto costaria cada politica de claves (bytes de bloque al arena) y cuanto daria el censo de las
// cuatro familias con cada una: es el numero con el que se decide, no una opinion.
//
// Uso:  go run ./console/dc/medidas/pedidos-159m [-dir <carpeta con los MPQ>]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
)

const (
	anchoDeLaPared = 160
	offsetDeBloque = 32
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

var losCuatro = []d2enum.TileType{
	d2enum.TileLeftWall,
	d2enum.TileRightWall,
	d2enum.TileRightPartOfNorthCornerWall,
	d2enum.TileLeftPartOfNorthCornerWall,
}

// clave: (estilo, secuencia, tipo) de un tile.
type clave [3]int

func (c clave) String() string {
	return fmt.Sprintf("%d/%d/tipo %d", c[0], c[1], c[2])
}

// tileCrudo: un tile del `.dt1` recien parseado, con lo que el censo necesita (bytes de bloque y alto real).
type tileCrudo struct {
	bloques int // bytes de bloque (lo que cuesta copiarlo al arena)
	alto    int32
}

func altoReal(t *d2dt1.Tile) int32 {
	minY, maxY := int32(0), int32(0)

	for _, b := range t.Blocks {
		if int32(b.Y) < minY {
			minY = int32(b.Y)
		}

		if int32(b.Y)+offsetDeBloque > maxY {
			maxY = int32(b.Y) + offsetDeBloque
		}
	}

	alto := t.Height
	if alto < 0 {
		alto = -alto
	}

	if rango := maxY - minY; rango > alto {
		alto = rango
	}

	return alto
}

func bytesDeBloques(t *d2dt1.Tile) int {
	n := 0

	for b := range t.Blocks {
		n += len(t.Blocks[b].EncodedData)
	}

	return n
}

func main() {
	dir := flag.String("dir", "", "carpeta con d2data.mpq y patch_d2.mpq")
	flag.Parse()

	if err := correr(*dir); err != nil {
		fmt.Println("PEDIDOS159M: FALLO --", err)
		os.Exit(1)
	}
}

func correr(dir string) error {
	if dir == "" {
		dir = autodetectarD2()
	}

	parche := buscarMPQ(dir, "patch_d2.mpq")
	datos := buscarMPQ(dir, "d2data.mpq")

	if parche == "" || datos == "" {
		return fmt.Errorf("faltan los MPQ en %q", dir)
	}

	// el motor de la PC: la tabla de tiles COMPLETA (el camino del arena de bloques solo lo prende el
	// arranque de la consola, o sea que aca no se compacta nada y `GetTiles` ve los 8 `.dt1` enteros).
	am, err := nuevoAssetManager(parche, datos)
	if err != nil {
		return err
	}

	engine := d2mapengine.CreateMapEngine(d2util.LogLevelError, am)

	stamp := engine.LoadStamp(d2enum.RegionAct1Town, 1, 0)
	if stamp == nil {
		return fmt.Errorf("LoadStamp devolvio nil")
	}

	tamano := stamp.Size()

	engine.ResetMapParaStamp(d2enum.RegionAct1Town, tamano.Width, tamano.Height, stamp)
	engine.PlaceStampClippedWhere(stamp, 0, 0, tamano.Width, tamano.Height, func(int) bool { return false }, false)

	// ============ 1. las claves que el `.ds1` declara (lo que la consola conserva) ============
	declaradas := map[clave]bool{}
	for _, k := range stamp.ClavesDeTiles() {
		declaradas[desempacar(k)] = true
	}

	// ============ 2. el `.dt1` crudo: que claves TIENE el nivel, y cuanto cuesta cada tile ============
	crudas := map[clave][]tileCrudo{}

	for _, ruta := range stamp.TilesDelDS1() {
		dt1, err := am.LoadDT1(ruta)
		if err != nil {
			return fmt.Errorf("LoadDT1(%s): %w", ruta, err)
		}

		for i := range dt1.Tiles {
			t := &dt1.Tiles[i]
			k := clave{int(t.Style), int(t.Sequence), int(t.Type)}
			crudas[k] = append(crudas[k], tileCrudo{bloques: bytesDeBloques(t), alto: altoReal(t)})
		}
	}

	// ============ 3. lo que el renderer PIDE sobre el pueblo compuesto ============
	demandas := map[clave]int{} // clave -> cuantas casillas la piden
	familias := map[[2]int]bool{}
	pares := 0

	tiles := *engine.Tiles()

	for idx := range tiles {
		for i := range tiles[idx].Components.Walls {
			w := &tiles[idx].Components.Walls[i]
			if w.Hidden() || w.Prop1 == 0 || w.Type.Special() {
				continue
			}

			k := clave{int(w.Style), int(w.Sequence), int(w.Type)}
			demandas[k]++

			familias[[2]int{k[0], k[1]}] = true

			// generateWallCache: la mitad derecha de la esquina norte pide la izquierda para elegir la
			// mas baja de las dos.
			if w.Type == d2enum.TileRightPartOfNorthCornerWall {
				demandas[clave{k[0], k[1], int(d2enum.TileLeftPartOfNorthCornerWall)}]++
				pares++
			}
		}

		for i := range tiles[idx].Components.Shadows {
			s := &tiles[idx].Components.Shadows[i]
			if s.Hidden() || s.Prop1 == 0 {
				continue
			}

			demandas[clave{int(s.Style), int(s.Sequence), int(d2enum.TileShadow)}]++
		}

		for i := range tiles[idx].Components.Floors {
			f := &tiles[idx].Components.Floors[i]
			if f.Hidden() || f.Prop1 == 0 {
				continue
			}

			demandas[clave{int(f.Style), int(f.Sequence), 0}]++
		}
	}

	// ============ 4. la clasificacion ============
	fmt.Printf("PEDIDOS159M: pueblo %dx%d, .dt1 del .ds1 = %d, familias de pared = %d\n",
		tamano.Width, tamano.Height, len(stamp.TilesDelDS1()), len(familias))

	perdidas := []clave{}
	legitimas := []clave{}
	bytesPerdidos := 0
	tilesPerdidos := 0

	claves := ordenar(demandas)

	for _, k := range claves {
		crudo, hayCrudo := crudas[k]
		nCrudo := len(crudo)

		_, declarada := declaradas[k]

		switch {
		case hayCrudo && !declarada:
			perdidas = append(perdidas, k)

			bytesDeLaClave := 0
			for _, t := range crudo {
				bytesDeLaClave += t.bloques
				tilesPerdidos++
			}

			bytesPerdidos += bytesDeLaClave

			fmt.Printf("PEDIDOS159M:   PERDIDA %s: el .dt1 trae %d tile(s) = %d B de bloque, la tabla "+
				"compactada 0 (casillas=%d)\n", k, nCrudo, bytesDeLaClave, demandas[k])
		case !hayCrudo:
			legitimas = append(legitimas, k)

			fmt.Printf("PEDIDOS159M:   legitima %s: el .dt1 del nivel NO lo trae (casillas=%d)\n", k, demandas[k])
		}
	}

	fmt.Printf("PEDIDOS159M: demandas=%d claves distintas=%d; PERDIDAS=%d (%d tile(s), %d B de bloque); "+
		"legitimas=%d; pares de esquina pedidos=%d\n", cuantasDemandas(demandas), len(claves), len(perdidas),
		tilesPerdidos, bytesPerdidos, len(legitimas), pares)

	// ============ 5. las dos politicas de claves: costo y censo ============
	// politica A (la demanda): lo declarado + lo que el renderer pide
	politicaA := map[clave]bool{}
	for _, k := range claves {
		politicaA[k] = true
	}

	// politica B (la familia completa): las cuatro familias de pared, por cada familia que el `.ds1` usa
	politicaB := map[clave]bool{}
	for k := range politicaA {
		politicaB[k] = true
	}

	for f := range familias {
		for _, tipo := range losCuatro {
			politicaB[clave{f[0], f[1], int(tipo)}] = true
		}
	}

	// el censo con cada politica: las imagenes de los cuatro tipos por familia (la MISMA cuenta del censo)
	for _, caso := range []struct {
		nombre string
		claves map[clave]bool
	}{
		{"A demanda (.ds1 + lo que pide el renderer)", politicaA},
		{"B familia completa (.ds1 + las cuatro familias)", politicaB},
	} {
		imagenes, bytes, extraTiles, extraBytes := 0, 0, 0, 0

		for f := range familias {
			for _, tipo := range losCuatro {
				k := clave{f[0], f[1], int(tipo)}

				if !caso.claves[k] {
					continue // la tabla compactada no tiene esa clave: el censo no la ve
				}

				crudo, ok := crudas[k]
				if !ok {
					continue
				}

				for _, t := range crudo {
					if t.alto > 0 {
						imagenes++
						bytes += anchoDeLaPared * int(t.alto)
					}

					// lo que la politica mete al arena sin que el renderer lo pida
					if demandas[k] == 0 {
						extraTiles++
						extraBytes += t.bloques
					}
				}
			}
		}

		fmt.Printf("PEDIDOS159M: politica %s -> censo 4 tipos: imagenes=%d bytes=%d; tiles que no pide nadie "+
			"que igual entran: %d var(tile)s / %d B de bloque\n", caso.nombre, imagenes, bytes, extraTiles, extraBytes)
	}

	// el desvio entre las dos politicas, clave por clave: es la explicacion del criterio 1 de la tarjeta
	imagenesDelDesvio, bytesDelDesvio := 0, 0

	for f := range familias {
		for _, tipo := range losCuatro {
			k := clave{f[0], f[1], int(tipo)}

			if demandas[k] != 0 {
				continue
			}

			crudo, ok := crudas[k]
			if !ok {
				continue
			}

			for _, t := range crudo {
				if t.alto == 0 {
					continue
				}

				imagenesDelDesvio++
				bytesDelDesvio += anchoDeLaPared * int(t.alto)

				fmt.Printf("PEDIDOS159M:   sin pedido %s: %d B (%dx%d) que el censo cuenta en el host y que el "+
					"renderer no pide\n", k, anchoDeLaPared*int(t.alto), anchoDeLaPared, t.alto)
			}
		}
	}

	fmt.Printf("PEDIDOS159M: desvio del censo entre las dos politicas: %d imagenes / %d B\n", imagenesDelDesvio,
		bytesDelDesvio)

	// ============ 5b. que claves de la lista NO las pide nadie (y cuanto cuestan) ============
	//
	// La lista de claves se arma con las capas del `.ds1`, celdas vacias incluidas: una casilla con `Prop1=0` o
	// oculta no la compone nadie (`PrepareTile` las saltea), asi que sus claves no las pide el renderer. Aca se
	// mide cuanto ocupan, que es la unica perilla para no cruzar el tramo del arena (DC-002.159m: +49.341 B de
	// bloques cruzaron de 5 a 6 tramos y el cuadro 1 murio en la paleta).
	soloDeclaradas, soloDeclaradasBytes := 0, 0

	for k := range declaradas {
		if demandas[k] != 0 {
			continue
		}

		bytesDeLaClave := 0
		for _, t := range crudas[k] {
			bytesDeLaClave += t.bloques
		}

		fmt.Printf("PEDIDOS159M:   sin pedido %s: %d tile(s), %d B de bloque, %d casillas que la usan\n", k,
			len(crudas[k]), bytesDeLaClave, casillasQueLaUsan159m(engine, k))

		for _, t := range crudas[k] {
			soloDeclaradas++
			soloDeclaradasBytes += t.bloques
		}
	}

	fmt.Printf("PEDIDOS159M: claves en la lista que NO pide nadie: %d, %d var(tile)s, %d B de bloque\n",
		cuantasSinPedido(declaradas, demandas), soloDeclaradas, soloDeclaradasBytes)

	// ============ 6. que declara el `.ds1`, por capa ============
	declaradasPorTipo := map[int]int{}
	for k := range declaradas {
		declaradasPorTipo[k[2]]++
	}

	tipos := []int{}
	for t := range declaradasPorTipo {
		tipos = append(tipos, t)
	}

	sort.Ints(tipos)

	partes := []string{}
	for _, t := range tipos {
		partes = append(partes, fmt.Sprintf("tipo=%d %d clave(s)", t, declaradasPorTipo[t]))
	}

	fmt.Printf("PEDIDOS159M: el .ds1 declara %d claves: %s\n", len(declaradas), strings.Join(partes, ", "))
	fmt.Println("PEDIDOS159M: FIN")

	return nil
}

// casillasQueLaUsan159m: cuantas casillas del nivel compuesto declaran esa clave (0 = no la pinta nadie).
func casillasQueLaUsan159m(engine *d2mapengine.MapEngine, k clave) int {
	n := 0

	tiles := *engine.Tiles()

	for idx := range tiles {
		for i := range tiles[idx].Components.Walls {
			w := &tiles[idx].Components.Walls[i]
			if int(w.Style) == k[0] && int(w.Sequence) == k[1] && int(w.Type) == k[2] && !w.Hidden() &&
				w.Prop1 != 0 {
				n++
			}
		}

		for i := range tiles[idx].Components.Floors {
			f := &tiles[idx].Components.Floors[i]
			if k[2] == 0 && int(f.Style) == k[0] && int(f.Sequence) == k[1] && !f.Hidden() && f.Prop1 != 0 {
				n++
			}
		}
	}

	return n
}

// cuantasSinPedido: cuantas claves de la lista no las pide ningun componente.
func cuantasSinPedido(claves map[clave]bool, demandas map[clave]int) int {
	n := 0

	for k := range claves {
		if demandas[k] == 0 {
			n++
		}
	}

	return n
}

// cuantasDemandas: cuantas casillas piden algo al motor, en total.
func cuantasDemandas(m map[clave]int) int {
	total := 0

	for _, n := range m {
		total += n
	}

	return total
}

// desempacar: la clave de `d2asset` (21 bits por campo) de vuelta a (estilo, secuencia, tipo).
func desempacar(k int64) clave {
	const mascara = (1 << 21) - 1

	return clave{int(k >> 42 & mascara), int(k >> 21 & mascara), int(k & mascara)}
}

func ordenar(m map[clave]int) []clave {
	claves := make([]clave, 0, len(m))
	for k := range m {
		claves = append(claves, k)
	}

	sort.Slice(claves, func(i, j int) bool {
		if claves[i][0] != claves[j][0] {
			return claves[i][0] < claves[j][0]
		}

		if claves[i][1] != claves[j][1] {
			return claves[i][1] < claves[j][1]
		}

		return claves[i][2] < claves[j][2]
	})

	return claves
}

func nuevoAssetManager(parche, datos string) (*d2asset.AssetManager, error) {
	am, err := d2asset.NewAssetManager(d2util.LogLevelError)
	if err != nil {
		return nil, err
	}

	for _, ruta := range []string{parche, datos} {
		if e := am.AddSource(ruta, types.AssetSourceMPQ); e != nil {
			return nil, fmt.Errorf("AddSource(%s): %w", ruta, e)
		}
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
