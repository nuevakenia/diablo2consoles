package d2mapengine

// DC-002.159m -- LO QUE EL RENDERER PIDE Y LA TABLA COMPACTADA TIENE QUE TENER.
//
// EL DEFECTO, MEDIDO. El arena de bloques usados copia SOLO los tiles cuya clave `(estilo, secuencia, tipo)`
// esta en `Stamp.ClavesDeTiles()`, y esa lista era la que el `.ds1` DECLARA por casilla. Pero el renderer pide,
// ademas de la clave declarada, otras dos que el `.ds1` no declara nunca:
//
//   - la OTRA MITAD de la esquina norte (`tipo=3` pide `tipo=4`, `generateWallCache`), y
//   - las SOMBRAS con `tipo=13` (`PrepareTile` y `generateShadowCache`), porque las capas de sombra del `.ds1`
//     no llevan tipo (se decodifican con el registro del piso y quedan con `Type=0`).
//
// Medido en el pueblo con `console/dc/medidas/pedidos-159m`: 2695 pedidos, 247 claves distintas y 6 claves que
// el `.dt1` del nivel TIENE y la tabla compactada no tenia (2 sombras `0/0/13` y `0/1/13`, y las mitades
// izquierdas `0/1/4`, `0/2/4`, `0/3/4`, `2/0/4`): 6 tiles, 49.341 B de bloque. En la corrida del Gate 0 de
// DC-002.159j eso se veia como `Unknown tile ID [2 0 %!d(fmt.Stringer=Left part of an Upper-Left corner)]`
// (esquina norte dibujada con una sola mitad) y `Unknown tile ID [0 0 %!d(fmt.Stringer=shadow)]`.
//
// LO QUE ESTAS PRUEBAS FIJAN:
//
//  1. NINGUN pedido del renderer se queda sin tiles en el pueblo (paredes declaradas, la mitad izquierda de la
//     esquina norte y las sombras). Es el criterio 2 de la tarjeta mirado desde el host.
//  2. EL DESVIO DEL CENSO, con su numero: la tabla compactada censa 90 imagenes de las cuatro familias de pared
//     y la tabla entera 96. Las 6 que faltan son exactamente las claves `2/1/tipo 3`, `2/1/tipo 4`, `2/2/tipo
//     3`, `2/2/tipo 4`, `2/3/tipo 3`, `2/3/tipo 4`: familias que el `.ds1` usa como pared pero para las que NO
//     declara ninguna mitad de esquina norte, asi que el renderer no las pide nunca (163.840 B de imagen que
//     nadie compone). Es el criterio 1 por la segunda via ("o el desvio queda explicado con su numero").
//  3. EL AVISO del motor dice el tipo de verdad (estaba con `%d` sobre un `Stringer`) y el censo NO lo dispara
//     (el instrumento no puede tapar la medida: en 159j el censo imprimia ~160 lineas `Unknown tile ID` en el
//     log del Gate 0 y no se podia saber si el renderer se habia quedado sin un grafico).
//
// DATOS: la instalacion del juego (`D2_DIR`, o ~/.wine-d2/drive_c/Games/Diablo II). Sin los MPQ se SALTA.

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapstamp"
)

// los numeros medidos con `console/dc/medidas/pedidos-159m` y `sombra-159i` sobre el pueblo.
const (
	// imagenes y bytes del censo de las cuatro familias con la tabla compactada (la consola)
	censoConsola159m = 90
	bytesConsola159m = 3010560

	// con la tabla ENTERA (el camino de la PC, que es el que mide `sombra-159i`)
	censoHost159m = 96
	bytesHost159m = 3174400

	// las dos sombras del pueblo y las cuatro mitades izquierdas que el `.ds1` no declara
	sombrasDelPueblo159m = 5
	paresDeEsquina159m   = 7
)

// clavesDelDesvio159m: las 6 claves que solo estan en la tabla entera, con la imagen que el censo les cuenta
// (160 de ancho por el alto real de cada tile).
var clavesDelDesvio159m = []struct {
	estilo, secuencia, tipo int
	imagen                  int
}{
	{2, 1, 3, 30720},
	{2, 1, 4, 30720},
	{2, 2, 3, 25600},
	{2, 2, 4, 25600},
	{2, 3, 3, 25600},
	{2, 3, 4, 25600},
}

// puebloCompuestoComoLaConsola159m: el pueblo por el camino de la consola (arena de bloques usados, que
// COMPACTA los `.dt1` a lo que el renderer pide) y con las casillas puestas, que es lo que hace falta para
// recorrer los componentes de pared y de sombra.
func puebloCompuestoComoLaConsola159m(t *testing.T) (*MapEngine, *d2mapstamp.Stamp, *d2asset.AssetManager) {
	t.Helper()

	dir := dirDelJuego159l(t)

	parche := filepath.Join(dir, "patch_D2.mpq")
	datos := filepath.Join(dir, "d2data.mpq")

	am, _, err := d2asset.NuevoAssetManagerDeConsola(d2util.LogLevelError, parche, datos)
	if err != nil {
		t.Fatalf("NuevoAssetManagerDeConsola: %v", err)
	}

	for _, tabla := range []string{d2resource.LevelType, d2resource.LevelPreset} {
		if err := am.LoadRecords(tabla); err != nil {
			t.Fatalf("LoadRecords(%s): %v", tabla, err)
		}
	}

	t.Cleanup(func() {
		// el camino de la consola deja palancas prendidas en el paquete; la prueba no las deja para la siguiente
		d2asset.CargarSoloLosTilesDelDS1(false)
	})

	engine := CreateMapEngine(d2util.LogLevelError, am)

	stamp := engine.LoadStamp(d2enum.RegionAct1Town, 1, 0)
	if stamp == nil {
		t.Fatal("LoadStamp devolvio nil: el pueblo no esta en el MPQ")
	}

	tamano := stamp.Size()

	engine.ResetMapParaStamp(d2enum.RegionAct1Town, tamano.Width, tamano.Height, stamp)
	engine.PlaceStampClippedWhere(stamp, 0, 0, tamano.Width, tamano.Height, func(int) bool { return false }, false)

	return engine, stamp, am
}

// altoReal159m: el alto con el que el renderer compone una pared/techo (`generateWallCache`: 160 de ancho, y
// `max(|Height|, maxY-minY)` con 32 px por bloque).
func altoReal159m(t *d2dt1.Tile) int32 {
	minY, maxY := int32(0), int32(0)

	for _, b := range t.Blocks {
		if int32(b.Y) < minY {
			minY = int32(b.Y)
		}

		if int32(b.Y)+32 > maxY {
			maxY = int32(b.Y) + 32
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

// censoDeLaTablaEntera159m: la cuenta del censo de las cuatro familias sobre los `.dt1` del nivel
// descomprimidos OTRA VEZ y sin arena (el mismo camino de la PC). Devuelve tambien las claves de esos `.dt1`
// con sus tamanos de imagen, para poder explicar el desvio.
func censoDeLaTablaEntera159m(t *testing.T, am *d2asset.AssetManager, rutas []string,
	engine *MapEngine) (imagenes, bytesCenso int, crudas map[[3]int][]int32) {
	t.Helper()

	crudas = map[[3]int][]int32{}

	for _, ruta := range rutas {
		datos, err := am.LoadFile("/data/global/tiles/" + ruta)
		if err != nil {
			t.Fatalf("LoadFile(%s): %v", ruta, err)
		}

		dt1, err := d2dt1.LoadDT1(datos)
		if err != nil {
			t.Fatalf("LoadDT1(%s): %v", ruta, err)
		}

		for i := range dt1.Tiles {
			t := &dt1.Tiles[i]
			k := [3]int{int(t.Style), int(t.Sequence), int(t.Type)}
			crudas[k] = append(crudas[k], altoReal159m(t))
		}
	}

	familias := familiasDePared159m(engine)

	for f := range familias {
		for _, tipo := range losCuatroTiposDePared159j {
			for _, alto := range crudas[[3]int{f[0], f[1], int(tipo)}] {
				if alto == 0 {
					continue // marcador de colision: no hay imagen
				}

				imagenes++
				bytesCenso += anchoFijoDeLaPared159j * int(alto)
			}
		}
	}

	return imagenes, bytesCenso, crudas
}

// familiasDePared159m: las `(estilo, secuencia)` de las paredes que las casillas del nivel declaran (la misma
// lista con la que el censo arma su recorrido).
func familiasDePared159m(engine *MapEngine) map[[2]int]bool {
	familias := map[[2]int]bool{}

	tiles := *engine.Tiles()

	for idx := range tiles {
		for i := range tiles[idx].Components.Walls {
			w := &tiles[idx].Components.Walls[i]
			if w.Hidden() || w.Prop1 == 0 || w.Type.Special() {
				continue
			}

			familias[[2]int{int(w.Style), int(w.Sequence)}] = true
		}
	}

	return familias
}

// TestElRendererNoSeQuedaSinNingunaMitadNiSombra159m: criterio 2 de la tarjeta.
func TestElRendererNoSeQuedaSinNingunaMitadNiSombra159m(t *testing.T) {
	engine, _, _ := puebloCompuestoComoLaConsola159m(t)

	var (
		perdidas []string
		pares    int
		sombras  int
		paredes  int
	)

	tiles := *engine.Tiles()

	for idx := range tiles {
		for i := range tiles[idx].Components.Walls {
			w := &tiles[idx].Components.Walls[i]
			if w.Hidden() || w.Prop1 == 0 || w.Type.Special() {
				continue
			}

			paredes++

			if len(engine.GetTiles(int(w.Style), int(w.Sequence), w.Type)) == 0 {
				perdidas = append(perdidas, "pared "+itoa159j(int(w.Style))+"/"+itoa159j(int(w.Sequence))+
					"/tipo "+itoa159j(int(w.Type)))
			}

			// generateWallCache: la mitad derecha de la esquina norte pide la izquierda
			if w.Type == d2enum.TileRightPartOfNorthCornerWall {
				pares++

				izquierda := engine.GetTiles(int(w.Style), int(w.Sequence), d2enum.TileLeftPartOfNorthCornerWall)
				if len(izquierda) == 0 {
					perdidas = append(perdidas, "mitad izquierda de la esquina "+
						itoa159j(int(w.Style))+"/"+itoa159j(int(w.Sequence)))
				}
			}
		}

		for i := range tiles[idx].Components.Shadows {
			s := &tiles[idx].Components.Shadows[i]
			if s.Hidden() || s.Prop1 == 0 {
				continue
			}

			sombras++

			if len(engine.GetTiles(int(s.Style), int(s.Sequence), d2enum.TileShadow)) == 0 {
				perdidas = append(perdidas, "sombra "+itoa159j(int(s.Style))+"/"+itoa159j(int(s.Sequence)))
			}
		}
	}

	t.Logf("paredes pedidas=%d pares de esquina=%d sombras=%d", paredes, pares, sombras)

	if len(perdidas) > 0 {
		t.Fatalf("el renderer pide %d claves que la tabla compactada no tiene: %v (ver "+
			"d2mapstamp.ClavesDeTiles, DC-002.159m)", len(perdidas), perdidas)
	}

	if pares != paresDeEsquina159m {
		t.Fatalf("el pueblo declara %d paredes de la mitad derecha de la esquina norte, se esperaban %d "+
			"(es el numero con el que se midio el defecto)", pares, paresDeEsquina159m)
	}

	if sombras != sombrasDelPueblo159m {
		t.Fatalf("el pueblo pinta %d sombras, se esperaban %d", sombras, sombrasDelPueblo159m)
	}

	if paredes == 0 {
		t.Fatal("no se recorrio ni una pared: la prueba no midio nada")
	}
}

// TestElCensoExplicaSuDesvioContraLaTablaEntera159m: criterio 1, por la via del desvio explicado con su numero.
func TestElCensoExplicaSuDesvioContraLaTablaEntera159m(t *testing.T) {
	engine, stamp, am := puebloCompuestoComoLaConsola159m(t)

	deLaConsola := engine.CensoDeLasCuatroFamiliasDePared159j()

	imagenes, bytesCenso, crudas := censoDeLaTablaEntera159m(t, am, stamp.TilesDelDS1(), engine)

	t.Logf("censo consola: familias=%d imagenes=%d bytes=%d maxima=%d", deLaConsola.Familias,
		deLaConsola.Imagenes, deLaConsola.Bytes, deLaConsola.Maxima)
	t.Logf("censo tabla entera: familias=%d imagenes=%d bytes=%d maxima=%d", len(familiasDePared159m(engine)),
		imagenes, bytesCenso, deLaConsola.Maxima)

	if deLaConsola.Imagenes != censoConsola159m || deLaConsola.Bytes != bytesConsola159m {
		t.Fatalf("el censo de la consola da %d imagenes / %d B, se esperaban %d / %d (DC-002.159m)",
			deLaConsola.Imagenes, deLaConsola.Bytes, censoConsola159m, bytesConsola159m)
	}

	if imagenes != censoHost159m || bytesCenso != bytesHost159m {
		t.Fatalf("el censo con la tabla entera da %d imagenes / %d B, se esperaban %d / %d (los mismos "+
			"numeros que `console/dc/medidas/sombra-159i` en el host)", imagenes, bytesCenso, censoHost159m,
			bytesHost159m)
	}

	// el desvio, con su numero: 6 imagenes / 163.840 B que son exactamente estas claves
	imagenesDelDesvio := 0
	bytesDelDesvio := 0

	for _, k := range clavesDelDesvio159m {
		altos := crudas[[3]int{k.estilo, k.secuencia, k.tipo}]
		if len(altos) != 1 {
			t.Fatalf("el `.dt1` del nivel trae %d tile(s) de %d/%d/tipo %d, se esperaba 1", len(altos),
				k.estilo, k.secuencia, k.tipo)
		}

		if imagen := anchoFijoDeLaPared159j * int(altos[0]); imagen != k.imagen {
			t.Fatalf("%d/%d/tipo %d mide %d B de imagen, se esperaba %d B", k.estilo, k.secuencia, k.tipo,
				imagen, k.imagen)
		}

		imagenesDelDesvio++
		bytesDelDesvio += k.imagen

		// el renderer NO la pide (el `.ds1` no declara ninguna mitad de esquina en esa familia), por eso la
		// tabla compactada no la tiene
		if len(engine.GetTiles(k.estilo, k.secuencia, d2enum.TileType(k.tipo))) != 0 {
			t.Fatalf("la tabla compactada tiene %d/%d/tipo %d: entonces no es una clave del desvio",
				k.estilo, k.secuencia, k.tipo)
		}
	}

	if imagenes-deLaConsola.Imagenes != imagenesDelDesvio || bytesCenso-deLaConsola.Bytes != bytesDelDesvio {
		t.Fatalf("el desvio medido (%d imagenes / %d B) no coincide con las %d claves que lo explican (%d B): "+
			"el desvio tiene otra causa", imagenes-deLaConsola.Imagenes, bytesCenso-deLaConsola.Bytes,
			imagenesDelDesvio, bytesDelDesvio)
	}

	// y ninguna imagen de las cuatro familias pasa el buffer al vuelo (criterio 3), ni cambia de tamano entre
	// las dos tablas
	if deLaConsola.SobreTope != 0 || deLaConsola.Maxima > TechoDeUnaImagen159j {
		t.Fatalf("consola: %d imagenes pasan el techo de %d B (la mayor mide %d B)", deLaConsola.SobreTope,
			TechoDeUnaImagen159j, deLaConsola.Maxima)
	}

	if deLaConsola.Maxima != imagenDeLaPared159j {
		t.Fatalf("la imagen de pared mas grande del pueblo mide %d B, se esperaba %d B", deLaConsola.Maxima,
			imagenDeLaPared159j)
	}
}

// TestElAvisoDiceElTipoYElCensoNoLoSuelta159m: el formato del aviso y el instrumento callado.
func TestElAvisoDiceElTipoYElCensoNoLoSuelta159m(t *testing.T) {
	logs := &bytes.Buffer{}

	l := d2util.NewLogger()
	l.Writer = logs

	m := &MapEngine{Logger: l}
	m.dt1TileData = []d2dt1.Tile{{Style: 0, Sequence: 0, Type: int32(d2enum.TileLeftWall)}}

	if got := m.GetTiles(2, 0, d2enum.TileLeftPartOfNorthCornerWall); got != nil {
		t.Fatalf("GetTiles de una clave que no esta devolvio %d tile(s)", len(got))
	}

	// el numero Y el nombre del tipo: es lo que `%d` sobre un `Stringer` rompia en la corrida de 159j
	const esperado = "Unknown tile ID [2 0 4 (Left part of an Upper-Left corner)]"
	if !bytes.Contains(logs.Bytes(), []byte(esperado)) {
		t.Fatalf("el aviso no dice el tipo de verdad; se esperaba %q en el log:\n%s", esperado, logs.String())
	}

	// el censo pregunta lo mismo pero NO avisa: si avisara, sus 4 tipos por familia taparian en el log del
	// Gate 0 los avisos del renderer (medido en 159j: ~160 lineas del censo contra 7 del renderer)
	logs.Reset()

	if got := m.tilesDeLaClave(2, 0, d2enum.TileLeftPartOfNorthCornerWall); len(got) != 0 {
		t.Fatalf("tilesDeLaClave devolvio %d tile(s) de una clave que no esta", len(got))
	}

	if bytes.Contains(logs.Bytes(), []byte("Unknown tile ID")) {
		t.Fatalf("la busqueda del censo avisa al log:\n%s", logs.String())
	}
}
