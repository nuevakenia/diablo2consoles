package d2mapengine

// DC-002.159j -- LA PRUEBA DEL HOST: LOS BLOQUES DE UN TILE SE LEEN DE LA RAIZ, NO DE LA TABLA EXTERNA.
//
// QUE FIJA, Y POR QUE EN EL HOST. En la consola `dt1TileData` es un bloque EXTERNO del monton de C (91.392 B
// con los 311 tiles del pueblo: `SELLOS159J` lo imprimio en la corrida del Gate 0) y el recolector de libgodc no
// escanea su interior (`gc_heap.c:107` + `gc_copy.c`), asi que las rebanadas `Blocks` que cuelgan de adentro
// quedaban SIN RAIZ: la primera recoleccion se las llevaba y el tile seguia con el `len`/`ptr` viejos apuntando
// a memoria reciclada. Se vio como la pared `estilo=0 sec=0 tipo=1` pidiendo 8.394.240 B (`minY=-29.648` sobre
// un `Height=-288` correcto) en DC-002.159i.
//
// El arreglo (`bloques_vivos_159j.go`) es una rebanada propia del motor, `raizDeBloques`, con la MISMA rebanada
// de cada tile en el semiespacio, que el recolector SI recorre y por lo tanto conserva y corrige. La tabla
// externa queda con un puntero viejo, asi que el que manda es la raiz: `GetTiles` repone la rebanada desde ahi.
//
// En el host el recolector es exacto y esto no se puede reproducir: lo que la prueba hace es CORROMPER la tabla
// como la corrompe el recolector de la consola (dejarle una rebanada ajena) y exigir que `GetTiles` siga
// devolviendo los bloques del `.dt1`. Sin la raiz la prueba se pone roja -- el segundo subtest lo demuestra en
// el mismo binario, que es la unica forma de que la prueba cace su propia regresion.
//
// DATOS: la instalacion del juego (`D2_DIR`, o ~/.wine-d2/drive_c/Games/Diablo II). Sin los MPQ se SALTA.

import (
	"path/filepath"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
)

// la pared que delato el defecto, con los numeros del host (console/dc/medidas/paredes-159j).
const (
	estiloDeLaPared159j   = 0
	secuenciaDeLaPared159j = 0
	tipoDeLaPared159j      = 1

	// 160 de ancho (tileSurfaceWidth) por 288 de alto: 46.080 B, el techo legitimo medido en el host
	imagenDeLaPared159j = 46080

	// el pueblo compone 311 tiles (los que el `.ds1` referencia) y 6.737 bloques: los mismos numeros que la
	// corrida de la consola imprimio en `SELLOS159J`
	tilesDelPueblo159j   = 311
	bloquesDelPueblo159j = 6737
)

// puebloComoLaConsola159j: el pueblo compuesto por el camino de la consola (arena de bloques usados), que es el
// que compacta los `.dt1` y el que dejo la tabla en un bloque externo. Devuelve el motor y el AssetManager del
// MISMO nivel por el camino de la PC (para comparar contra el `.dt1` sin arena).
func puebloComoLaConsola159j(t *testing.T) (*MapEngine, *d2asset.AssetManager, string) {
	t.Helper()

	dir := dirDelJuego159l(t)

	// los MPQ: patch_D2 primero (es el que trae las tablas del compositor)
	parche := filepath.Join(dir, "patch_D2.mpq")
	datos := filepath.Join(dir, "d2data.mpq")

	for _, ruta := range []string{parche, datos} {
		if _, err := filepath.Glob(ruta); err != nil {
			t.Fatalf("ruta del MPQ %s: %v", ruta, err)
		}
	}

	am, _, err := d2asset.NuevoAssetManagerDeConsola(d2util.LogLevelError, parche, datos)
	if err != nil {
		t.Fatalf("NuevoAssetManagerDeConsola: %v", err)
	}

	for _, tabla := range []string{d2resource.LevelType, d2resource.LevelPreset} {
		if err := am.LoadRecords(tabla); err != nil {
			t.Fatalf("LoadRecords(%s): %v", tabla, err)
		}
	}

	engine := CreateMapEngine(d2util.LogLevelError, am)

	stamp := engine.LoadStamp(d2enum.RegionAct1Town, 1, 0)
	if stamp == nil {
		t.Fatal("LoadStamp devolvio nil: el pueblo no esta en el MPQ")
	}

	tamano := stamp.Size()

	t.Cleanup(func() {
		// el camino de la consola deja palancas prendidas en el paquete; la prueba no las deja para la siguiente
		d2asset.CargarSoloLosTilesDelDS1(false)
	})

	engine.ResetMapParaStamp(d2enum.RegionAct1Town, tamano.Width, tamano.Height, stamp)

	return engine, am, dir
}

// medirLosBloques159j: la cuenta de `generateWallCache` sobre los bloques de un tile.
func medirLosBloques159j(t *d2dt1.Tile) (bloques int, minY, maxY int32) {
	bloques = len(t.Blocks)

	for _, b := range t.Blocks {
		if int32(b.Y) < minY {
			minY = int32(b.Y)
		}

		if int32(b.Y)+32 > maxY {
			maxY = int32(b.Y) + 32
		}
	}

	return bloques, minY, maxY
}

func TestLosBloquesDeLasParedesSeLeenDeLaRaiz159j(t *testing.T) {
	engine, _, _ := puebloComoLaConsola159j(t)

	// ============================== 1. la raiz cubre la tabla ==============================
	if len(engine.dt1TileData) != tilesDelPueblo159j {
		t.Fatalf("la tabla tiene %d tiles, se esperaban %d (el camino de la consola compacta los `.dt1`)",
			len(engine.dt1TileData), tilesDelPueblo159j)
	}

	if len(engine.raizDeBloques) != len(engine.dt1TileData) {
		t.Fatalf("la raiz tiene %d rebanadas y la tabla %d tiles: cada tile tiene que tener la suya",
			len(engine.raizDeBloques), len(engine.dt1TileData))
	}

	if rebanadas, bloques := engine.EstadoDeLaRaizDeBloques159j(); rebanadas != tilesDelPueblo159j ||
		bloques != bloquesDelPueblo159j {
		t.Fatalf("la raiz guarda %d rebanadas / %d bloques, se esperaban %d / %d",
			rebanadas, bloques, tilesDelPueblo159j, bloquesDelPueblo159j)
	}

	// ============================== 2. la pared 0/0/1, medida de la raiz ==============================
	opciones := engine.GetTiles(estiloDeLaPared159j, secuenciaDeLaPared159j, tipoDeLaPared159j)
	if len(opciones) != 1 {
		t.Fatalf("GetTiles(0,0,1) trajo %d tiles, se esperaba 1 (es la pared izquierda del pueblo)", len(opciones))
	}

	bloques, minY, maxY := medirLosBloques159j(&opciones[0])

	t.Logf("pared 0/0/1: bloques=%d minY=%d maxY=%d Height=%d", bloques, minY, maxY, opciones[0].Height)

	// 23 bloques es el dato REAL del `.ds1` (la corrida de la consola imprimio `bloques=23`): lo que estaba
	// roto eran los `Y` de esos bloques, no su cantidad.
	if bloques != 23 {
		t.Fatalf("la pared 0/0/1 trae %d bloques, se esperaban 23", bloques)
	}

	if minY != -256 || maxY != 0 {
		t.Fatalf("la pared 0/0/1: minY=%d maxY=%d (se esperaba minY=-256, maxY=0; el alto real lo da su "+
			"Height=-288: 288)", minY, maxY)
	}

	real := maxY - minY
	if h := opciones[0].Height; h < 0 {
		if -h > real {
			real = -h
		}
	} else if h > real {
		real = h
	}

	if imagen := 160 * int(real); imagen != imagenDeLaPared159j {
		t.Fatalf("la imagen de la pared 0/0/1 mide %d B, se esperaba %d B (160x288)", imagen, imagenDeLaPared159j)
	}

	// ============================== 3. el mutador: la tabla queda con una rebanada ajena ==============================
	//
	// Esto es lo que hace el recolector de libgodc con la tabla externa: el `len` y el `ptr` de la rebanada se
	// quedan como estaban y la memoria de los bloques pasa a ser de otro. Se emula dejandole una rebanada de
	// basura (23 bloques con un `Y` imposible, el numero que salio en la consola: -29.648).
	ajena := make([]d2dt1.Block, 23)
	for i := range ajena {
		ajena[i].Y = -29648
	}

	for i := range engine.dt1TileData {
		if engine.dt1TileData[i].Style == estiloDeLaPared159j &&
			engine.dt1TileData[i].Sequence == secuenciaDeLaPared159j &&
			engine.dt1TileData[i].Type == tipoDeLaPared159j {
			engine.dt1TileData[i].Blocks = ajena
		}
	}

	despues := engine.GetTiles(estiloDeLaPared159j, secuenciaDeLaPared159j, tipoDeLaPared159j)
	bloquesDespues, minYDespues, maxYDespues := medirLosBloques159j(&despues[0])

	if bloquesDespues != bloques || minYDespues != minY || maxYDespues != maxY {
		t.Fatalf("con la tabla corrompida GetTiles devolvio bloques=%d minY=%d maxY=%d; la raiz tiene "+
			"bloques=%d minY=%d maxY=%d (el tile se lee de la RAIZ: ver bloques_vivos_159j.go)",
			bloquesDespues, minYDespues, maxYDespues, bloques, minY, maxY)
	}

	// ============================== 4. y la prueba caza su regresion ==============================
	//
	// Sin la raiz (que es el estado del arbol antes de esta tarjeta) el mismo GetTiles devuelve la basura. Si
	// algun dia la raiz deja de ser la fuente, el paso 3 se pone rojo; esto fija que el paso 3 mide algo.
	engine.raizDeBloques = nil

	sinRaiz := engine.GetTiles(estiloDeLaPared159j, secuenciaDeLaPared159j, tipoDeLaPared159j)
	bloquesSin, minYSin, _ := medirLosBloques159j(&sinRaiz[0])

	if bloquesSin != 23 || minYSin != -29648 {
		t.Fatalf("sin la raiz se esperaba leer la rebanada ajena (23 bloques, minY=-29648) y se leyo "+
			"bloques=%d minY=%d: la prueba NO esta midiendo la raiz", bloquesSin, minYSin)
	}
}

// TestNingunaParedDelPuebloPasaElTecho159j: el invariante que la consola tiene que poder sostener -- ninguna
// imagen de las cuatro familias de pared del pueblo pasa el buffer al vuelo (81.920 B). Es el criterio 2 de la
// tarjeta mirado desde el host, con los mismos numeros que imprime `console/dc/medidas/paredes-159j`.
func TestNingunaParedDelPuebloPasaElTecho159j(t *testing.T) {
	engine, _, _ := puebloComoLaConsola159j(t)

	techo := 81920
	maxima, cual := 0, ""

	for _, tipo := range []d2enum.TileType{d2enum.TileLeftWall, d2enum.TileRightWall,
		d2enum.TileRightPartOfNorthCornerWall, d2enum.TileLeftPartOfNorthCornerWall} {
		for estilo := 0; estilo < 32; estilo++ {
			for secuencia := 0; secuencia < 64; secuencia++ {
				for _, tile := range engine.GetTiles(estilo, secuencia, tipo) {
					_, minY, maxY := medirLosBloques159j(&tile)

					alto := maxY - minY
					if abs := tile.Height; abs < 0 {
						if -abs > alto {
							alto = -abs
						}
					} else if abs > alto {
						alto = abs
					}

					n := 160 * int(alto)
					if n > maxima {
						maxima, cual = n, ""
						cual = " TIPO=" + tipo.String()
					}

					if n > techo {
						t.Fatalf("la pared estilo=%d sec=%d tipo=%d pide %d B (tipo%s), mas que el buffer al "+
							"vuelo (%d B): el dato del `.dt1` esta roto", estilo, secuencia, int(tipo), n, cual, techo)
					}
				}
			}
		}
	}

	t.Logf("la imagen de pared mas grande del pueblo: %d B", maxima)

	if maxima != imagenDeLaPared159j {
		t.Fatalf("la imagen de pared mas grande del pueblo mide %d B, se esperaba %d B (160x288, la pared "+
			"estilo=0 sec=0 tipo=1)", maxima, imagenDeLaPared159j)
	}
}
