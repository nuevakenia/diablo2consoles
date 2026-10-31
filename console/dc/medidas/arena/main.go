// DC-002.88 (instrumento del HOST): EL ARENA DE TILES Y LAS ESTRUCTURAS DEL MAPA, MEDIDOS ARCHIVO POR
// ARCHIVO, ANTES DE TOCAR EL COMPOSITOR.
//
// POR QUE EXISTE. DC-002.86 dejo medido que las tablas de records NO son la pared (0,9 % de uso) y que el
// compositor del motor `d2mapengine` muere pidiendo el arena de tiles: `make([]byte, 4.849.148)` de UNA sola
// vez contra un margen de monton de 4.550.656 B. El presupuesto que quedo para esta tarjeta es:
//
//	arena de tiles (constante de d2asset) ......... 4.848.148 B   <- lo que pide HOY
//	dt1TileData (603 tiles x 272 B) ...............   164.016 B
//	casillas del mapa (57x41 x 264 B) .............   616.968 B
//	.ds1 desarmado (4 capas x 2337 x 20 B) ........   186.960 B
//	AssetManager ..................................   352.352 B
//	= 6.168.348 B contra 5.230.592 B de margen -> faltan 937.756 B
//
// Lo que faltaba era el OTRO lado: cuanto de ese arena es de verdad necesario. El arena se dimensiono en
// DC-002.78 con los 8 `.dt1` del POC de ZONAS (4.408.317 B + 10 % de aire); el pueblo tiene 9 archivos en
// su level type y 4.652.603 B crudos, pero el `.ds1` del pueblo NO lista los 9. Este instrumento mide:
//
//  1. LOS `.dt1` DEL LEVEL TYPE, uno por uno: tamano descomprimido (de la tabla de bloques del MPQ, sin leer
//     el archivo), tamano comprimido, tiles, bytes de bloques codificados (lo que de verdad hay que tener
//     vivo para pintar) y bytes de cabecera (el resto del archivo).
//  2. CUALES USA EL `.ds1`: la lista de archivos del `townn1.ds1` contra la lista del level type. Los que
//     sobran se cargan y ocupan arena sin que ninguna casilla los mire.
//  3. EL ARENA QUE ALCANZA: la suma de los tamanos de los `.dt1` que el nivel carga de verdad, con y sin el
//     aire del 10 % de la constante, y contra la constante de hoy.
//  4. LAS ESTRUCTURAS: `MapTile` por casilla, `d2dt1.Tile` por tile (el `dt1TileData`), y las capas del
//     `.ds1` desarmado. Con el `sizeof` del HOST (el de la consola lo imprime el POC: MapTile 264, Tile 272,
//     d2ds1.Tile 20).
//
// NO MIDE LA CONSOLA. Los numeros del monton de la consola salen del POC (`build-tablas-282.sh`); aca se
// mide el TAMANO de las cosas, que es lo que decide que recorte tiene sentido.
//
// Uso:
//
//	go run ./console/dc/medidas/arena -dir ~/.wine-d2/drive_c/Games/'Diablo II'
//	go run ./console/dc/medidas/arena -dir ... -levelType 1 -ds1 act1/town/townn1.ds1
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// la constante del codigo, copiada a proposito: si cambia en d2asset, este instrumento tiene que decir que
// la copia quedo vieja en vez de mentir con un numero que ya no es el que pide la consola.
const arenaDelCodigo = 4408317 + 440831 // d2core/d2asset/lectura_de_tiles.go

// aireDeLaConstante: el 10 % que se agrego en DC-002.78 (440.831 B sobre 4.408.317).
const aireDeLaConstante = 440831

var mpqs []*d2mpq.MPQ

func leer(nombre string) ([]byte, int, bool) {
	clave := strings.TrimPrefix(nombre, "/")
	clave = strings.ReplaceAll(clave, "/", "\\")

	for _, m := range mpqs {
		if tam, hay := m.TamanoDeArchivo(clave); hay {
			if datos, err := m.ReadFile(clave); err == nil {
				return datos, tam, true
			}
		}
	}

	return nil, 0, false
}

// barraInvertida: la barra del MPQ, armada por su codigo (92) para no depender del escape.
const barraInvertida = string(rune(92))

// claveMPQ: nombre de level type ("act1/town/floor.dt1") -> nombre dentro del MPQ.
func claveMPQ(nombre string) string {
	nombre = strings.ToLower(nombre)
	nombre = strings.ReplaceAll(nombre, "c:", "")
	nombre = strings.ReplaceAll(nombre, ".tg1", ".dt1")
	nombre = strings.ReplaceAll(nombre, barraInvertida+"d2"+barraInvertida+"data"+barraInvertida+
		"global"+barraInvertida+"tiles"+barraInvertida, "")
	nombre = strings.ReplaceAll(nombre, barraInvertida, "/")

	return "data/global/tiles/" + nombre
}

// sinPrefijo: para comparar nombres de distinta procedencia ("act1/town/floor.dt1" contra
// "d2/data/global/tiles/act1/town/floor.tg1" del .ds1).
func sinPrefijo(nombre string) string {
	n := strings.ToLower(nombre)
	n = strings.ReplaceAll(n, barraInvertida, "/")
	n = strings.ReplaceAll(n, "c:", "")
	n = strings.ReplaceAll(n, ".tg1", ".dt1")

	if i := strings.LastIndex(n, "tiles/"); i >= 0 {
		n = n[i+len("tiles/"):]
	}

	return n
}

type archivo struct {
	nombre          string
	crudo           int
	comprimido      int
	tiles           int
	bloquesBy       int
	bloques         int
	usados          int // tiles cuyo (estilo, secuencia, tipo) aparece en alguna capa del .ds1
	bloquesUsadosBy int // bytes de bloque de esos tiles
	enElDS1         bool
	entro           bool
}

// tablaDelArchivoMasGrande: el .dt1 mas grande de los que el nivel carga. Es lo que hay que prestar para
// leer un archivo entero cuando el arena ya no guarda el crudo: el prestamo es UNO y se reusa entre archivos.
func tablaDelArchivoMasGrande(archivos []*archivo) int {
	masGrande := 0

	for i := range archivos {
		if archivos[i].enElDS1 && archivos[i].crudo > masGrande {
			masGrande = archivos[i].crudo
		}
	}

	return masGrande
}

// clavesDelDS1: las (estilo, secuencia, tipo) que el `.ds1` referencia de verdad, en la capa de piso y en
// las de pared. Es el numero que dice si un `.dt1` de la lista se usa o solo se carga.
func clavesDelDS1(d *d2ds1.DS1) map[[3]int32]bool {
	usadas := map[[3]int32]bool{}

	for capa := 0; capa < 2; capa++ {
		if d.GetFloor(capa) != nil {
			for y := 0; y < d.Height(); y++ {
				for x := 0; x < d.Width(); x++ {
					if t := d.GetFloor(capa).Tile(x, y); t != nil {
						usadas[[3]int32{int32(t.Style), int32(t.Sequence), int32(t.Type)}] = true
					}
				}
			}
		}
	}

	for capa := 0; capa < 4; capa++ {
		if d.GetWall(capa) != nil {
			for y := 0; y < d.Height(); y++ {
				for x := 0; x < d.Width(); x++ {
					if t := d.GetWall(capa).Tile(x, y); t != nil {
						usadas[[3]int32{int32(t.Style), int32(t.Sequence), int32(t.Type)}] = true
					}
				}
			}
		}
	}

	return usadas
}

func main() {
	dir := flag.String("dir", "", "carpeta con d2data.mpq y patch_d2.mpq")
	levelType := flag.Int("levelType", int(d2enum.RegionAct1Town), "id del level type (LvlTypes)")
	ds1Pedido := flag.String("ds1", "act1/town/townn1.ds1", "el .ds1 del nivel (relativo a data/global/tiles)")
	flag.Parse()

	if *dir == "" {
		fmt.Fprintln(os.Stderr, "falta -dir (la carpeta con los MPQ)")
		os.Exit(2)
	}

	entradas, errDir := os.ReadDir(*dir)
	if errDir != nil {
		fmt.Fprintln(os.Stderr, "no pude listar", *dir, errDir)
		os.Exit(1)
	}

	enDisco := map[string]string{}
	for _, e := range entradas {
		enDisco[strings.ToLower(e.Name())] = e.Name()
	}

	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq"} {
		real, hay := enDisco[nombre]
		if !hay {
			fmt.Fprintf(os.Stderr, "no hay %s en %s\n", nombre, *dir)
			os.Exit(1)
		}

		m, e := d2mpq.FromFile(filepath.Join(*dir, real))
		if e != nil {
			fmt.Fprintf(os.Stderr, "no pude abrir %s: %v\n", real, e)
			os.Exit(1)
		}

		mpqs = append(mpqs, m)
	}

	fmt.Printf("sizeof del HOST: d2dt1.Tile=%d d2ds1.Tile=%d (consola: 272 / 20); MapTile: 264 B en la consola\n",
		int(unsafe.Sizeof(d2dt1.Tile{})), int(unsafe.Sizeof(d2ds1.Tile{})))

	// ============================== el level type ==============================
	rm, err := d2records.NewRecordManager(d2util.LogLevelNone)
	if err != nil {
		fmt.Fprintln(os.Stderr, "NewRecordManager:", err)
		os.Exit(1)
	}

	datos, _, hay := leer(d2resource.LevelType)
	if !hay {
		fmt.Fprintln(os.Stderr, "no pude leer", d2resource.LevelType)
		os.Exit(1)
	}

	if err := rm.Load(d2resource.LevelType, d2txt.LoadDataDictionary(datos)); err != nil {
		fmt.Fprintln(os.Stderr, "no pude cargar LvlTypes:", err)
		os.Exit(1)
	}

	tipo := rm.Level.Types[*levelType]
	if tipo == nil {
		fmt.Fprintf(os.Stderr, "el level type %d no existe en LvlTypes\n", *levelType)
		os.Exit(1)
	}

	fmt.Printf("\n== el level type %d: %q (%d campos de archivo)\n", *levelType, tipo.Name, len(tipo.Files))

	// ============================== el .ds1 del nivel ==============================
	usa := map[string]bool{}
	ancho, alto := 0, 0
	crudoDS1 := 0
	capas := 0
	objetos := 0
	filesDS1 := []string{}
	var ds1DelNivel *d2ds1.DS1

	if *ds1Pedido != "" {
		datosDS1, tam, hayDS1 := leer("data/global/tiles/" + *ds1Pedido)
		if !hayDS1 {
			fmt.Fprintf(os.Stderr, "no pude leer el .ds1 %s\n", *ds1Pedido)
		} else {
			crudoDS1 = tam

			d, errDS1 := d2ds1.Unmarshal(datosDS1)
			if errDS1 != nil {
				fmt.Fprintf(os.Stderr, "no pude parsear %s: %v\n", *ds1Pedido, errDS1)
			} else {
				ds1DelNivel = d
				ancho, alto = d.Width(), d.Height()
				objetos = len(d.Objects)
				filesDS1 = d.Files

				for i := range d.Files {
					usa[sinPrefijo(d.Files[i])] = true
				}

				for i := 0; i < 4; i++ {
					if d.GetFloor(i) != nil {
						capas++
					}

					if d.GetWall(i) != nil {
						capas++
					}
				}

				for i := 0; i < 2; i++ {
					if d.GetShadow(i) != nil {
						capas++
					}
				}
			}
		}
	}

	fmt.Printf("\n== los .dt1 del level type, uno por uno\n")
	fmt.Printf("%-28s %10s %10s %6s %10s %10s %6s %6s  %s\n",
		"archivo", "crudo", "comprim.", "tiles", "bloques B", "cabecera B", "usados", "enDS1", "en el .ds1")

	claves := map[[3]int32]bool{}
	if ds1DelNivel != nil {
		claves = clavesDelDS1(ds1DelNivel)
	}

	// el censo del `.ds1` con el que se compara la grilla que compone el motor en la consola (DC-002.88: el
	// pueblo compuso pisos= 2337 paredes= 338 y grilla crc -1596781041)
	if ds1DelNivel != nil {
		conPiso, conPared, celdas := 0, 0, 0

		for y := 0; y < ds1DelNivel.Height(); y++ {
			for x := 0; x < ds1DelNivel.Width(); x++ {
				celdas++

				piso := false
				for capa := 0; capa < 2 && !piso; capa++ {
					if f := ds1DelNivel.GetFloor(capa); f != nil && f.Tile(x, y) != nil {
						piso = true
					}
				}

				pared := false
				for capa := 0; capa < 4 && !pared; capa++ {
					if w := ds1DelNivel.GetWall(capa); w != nil && w.Tile(x, y) != nil {
						pared = true
					}
				}

				if piso {
					conPiso++
				}

				if pared {
					conPared++
				}
			}
		}

		fmt.Printf("\n== el censo del .ds1 (lo que el motor tiene que copiar)\n")
		fmt.Printf("celdas=%d conPiso=%d conPared=%d claves=%d\n", celdas, conPiso, conPared, len(claves))
	}

	var (
		archivos           []*archivo
		totalCrudo         int
		totalTiles         int
		totalBloques       int
		totalComprimido    int
		totalDelDS1        int
		tilesDelDS1        int
		totalUsadoDelDS1   int
		tilesUsados        int
		totalBloquesUsados int
		sobrantes          []string
	)

	// la lista a medir: los archivos del level type PRIMERO y despues los que el `.ds1` lista y el level type
	// no tiene (el pueblo: `act1/barracks/warp.dt1`, 658.158 B, que el compositor carga hoy porque el `.ds1`
	// lo pide y el level type lo ignora).
	nombres := make([]string, 0, 16)
	visto := map[string]bool{}

	for i := range tipo.Files {
		nombre := strings.ToLower(tipo.Files[i])
		if nombre == "" || nombre == "0" || visto[nombre] {
			continue
		}

		visto[nombre] = true
		nombres = append(nombres, nombre)
	}

	enLevelType := map[string]bool{}

	for i := range nombres {
		enLevelType[nombres[i]] = true
	}

	for i := range filesDS1 {
		nombre := sinPrefijo(filesDS1[i])
		if nombre == "" || visto[nombre] {
			continue
		}

		visto[nombre] = true
		nombres = append(nombres, nombre)
	}

	for i := range nombres {
		nombre := nombres[i]

		a := &archivo{nombre: nombre, enElDS1: usa[sinPrefijo(nombre)]}

		clave := claveMPQ(nombre)
		for _, m := range mpqs {
			if tam, ok := m.TamanoDeArchivo(strings.ReplaceAll(clave, "/", "\\")); ok {
				a.crudo = tam
				a.comprimido, _ = m.TamanoComprimido(strings.ReplaceAll(clave, "/", "\\"))

				break
			}
		}

		if datos, _, ok := leer(clave); ok {
			a.entro = true

			if dt1, errDT1 := d2dt1.LoadDT1(datos); errDT1 == nil {
				a.tiles = len(dt1.Tiles)

				for t := range dt1.Tiles {
					bytesDelTile := 0

					for b := range dt1.Tiles[t].Blocks {
						a.bloques++
						bytesDelTile += len(dt1.Tiles[t].Blocks[b].EncodedData)
					}

					a.bloquesBy += bytesDelTile

					if claves[[3]int32{dt1.Tiles[t].Style, dt1.Tiles[t].Sequence, dt1.Tiles[t].Type}] {
						a.usados++
						a.bloquesUsadosBy += bytesDelTile
					}
				}
			}
		}

		cabecera := a.crudo - a.bloquesBy

		marca := ""
		if !a.enElDS1 {
			marca = "NO (sobra)"
			sobrantes = append(sobrantes, nombre)
		} else if a.tiles > 0 && a.usados == 0 {
			marca = "SI, pero 0 tiles usados"
		}

		fmt.Printf("%-28s %10d %10d %6d %10d %10d %8d %6d %6s  %s\n",
			nombre, a.crudo, a.comprimido, a.tiles, a.bloquesBy, a.bloquesUsadosBy, cabecera, a.usados,
			boolTexto(a.enElDS1), marca)

		archivos = append(archivos, a)
		totalComprimido += a.comprimido

		if enLevelType[nombre] {
			totalCrudo += a.crudo
			totalTiles += a.tiles
			totalBloques += a.bloquesBy
		}

		if a.enElDS1 {
			totalDelDS1 += a.crudo
			tilesDelDS1 += a.tiles
			totalBloquesUsados += a.bloquesUsadosBy

			if a.usados > 0 {
				totalUsadoDelDS1 += a.crudo
				tilesUsados += a.tiles
			}
		}
	}

	fmt.Printf("%-28s %10d %10d %6d %10d %10d\n", "TOTAL (el level type)", totalCrudo, totalComprimido,
		totalTiles, totalBloques, totalCrudo-totalBloques)
	fmt.Printf("%-28s %10d %10d %6d %10d %10d\n", "TOTAL (los que lista el .ds1)", totalDelDS1, 0,
		tilesDelDS1, 0, 0)
	fmt.Printf("%-28s %10d %10d %6d %10d %10d\n", "TOTAL (los que el .ds1 USA de verdad)", totalUsadoDelDS1, 0,
		tilesUsados, 0, 0)

	// ============================== el arena que alcanza ==============================
	fmt.Printf("\n== el arena de tiles\n")
	fmt.Printf("%-58s %10d B\n", "la constante de d2asset (lo que pide hoy, de una vez)", arenaDelCodigo)
	fmt.Printf("%-58s %10d B\n", "los .dt1 del level type, crudos (lo que carga hoy ResetMap)", totalCrudo)
	fmt.Printf("%-58s %10d B\n", "los .dt1 que el .ds1 lista, crudos (ResetMapParaStamp)", totalDelDS1)
	fmt.Printf("%-58s %10d B\n", "  con el mismo aire del 10 %", totalDelDS1+totalDelDS1/10)
	fmt.Printf("%-58s %10d B\n", "los .dt1 que el .ds1 USA de verdad, crudos", totalUsadoDelDS1)

	fmt.Printf("\n== el piso del arena: solo los bloques de los tiles que el .ds1 usa\n")
	fmt.Printf("%-58s %10d B\n", "bloques codificados de TODOS los tiles del .ds1", totalBloquesUsados)
	fmt.Printf("%-58s %10d B\n", "  + las cabeceras de esos tiles (272 B x %d)", tilesUsados)
	fmt.Printf("%-58s %10d B\n", "  = arena de bloques usados + cabeceras",
		totalBloquesUsados+tilesUsados*272)
	fmt.Printf("%-58s %10d B\n", "  (mas el prestamo de lectura: el .dt1 mas grande)",
		tablaDelArchivoMasGrande(archivos))
	fmt.Printf("%-58s %10d B\n", "  = TOTAL con el arena de bloques usados",
		totalBloquesUsados+tilesUsados*272+tablaDelArchivoMasGrande(archivos))
	fmt.Printf("%-58s %10d B\n", "recorte contra los .dt1 del .ds1",
		totalDelDS1-(totalBloquesUsados+tilesUsados*272)-tablaDelArchivoMasGrande(archivos))

	if len(sobrantes) > 0 {
		fmt.Printf("\n  .dt1 que el level type carga y el .ds1 NO lista: %s\n", strings.Join(sobrantes, ", "))
	}

	fmt.Printf("\n  de los %d B crudos (level type), %d B son bloques codificados (el pixel RLE: lo que hay\n"+
		"  que tener vivo para pintar) y %d B son cabeceras (16 B de archivo + 272 B por tile + indices)\n",
		totalCrudo, totalBloques, totalCrudo-totalBloques)

	// ============================== las estructuras ==============================
	fmt.Printf("\n== las estructuras del mapa\n")

	casillas := ancho * alto
	mapTileB := tamanoDeMapTileConsola()
	dt1TileDataB := totalTiles * 272
	ds1B := capas * casillas * 20

	fmt.Printf("%-58s %10d B  (%dx%d=%d casillas x %d B, UNA make en ResetMap)\n",
		"casillas del mapa (MapTile)", casillas*mapTileB, ancho, alto, casillas, mapTileB)
	fmt.Printf("%-58s %10d B  (%d tiles x 272 B, append por .dt1)\n",
		"dt1TileData (d2dt1.Tile)", dt1TileDataB, totalTiles)
	fmt.Printf("%-58s %10d B  (%d capas x %d casillas x 20 B)\n",
		".ds1 desarmado (d2ds1.Tile)", ds1B, capas, casillas)
	fmt.Printf("%-58s %10d B  (medido en la consola, DC-002.86)\n", "AssetManager del motor", 352352)
	fmt.Printf("%-58s %10d B\n", "estructuras (dt1TileData + casillas + .ds1)",
		dt1TileDataB+casillas*mapTileB+ds1B)

	// ============================== el presupuesto completo ==============================
	margenConsola := 5230592 // medido: techo - sbrk al arrancar (DC-002.86)
	assetManager := 352352

	fmt.Printf("\n== el presupuesto del compositor, con cada caso\n")
	fmt.Printf("%-58s %10d B\n", "margen del monton en la consola (DC-002.86)", margenConsola)
	fmt.Printf("%-58s %10d B\n", "  menos el AssetManager", margenConsola-assetManager)

	casos := []struct {
		rotulo string
		arena  int
	}{
		{"arena = constante de hoy", arenaDelCodigo},
		{"arena = crudo del level type", totalCrudo},
		{"arena = crudo de los .dt1 que usa el .ds1", totalDelDS1},
	}

	for _, c := range casos {
		total := c.arena + dt1TileDataB + casillas*mapTileB + ds1B
		queda := margenConsola - assetManager - total

		veredicto := "ENTRA, quedan"
		if queda < 0 {
			veredicto = "NO ENTRA, faltan"
		}

		fmt.Printf("%-58s %10d B total -> %s %d B\n", "  "+c.rotulo, total, veredicto, abs(queda))
	}

	// ============================== los .dt1 que el .ds1 lista ==============================
	if len(filesDS1) > 0 {
		fmt.Printf("\n== los archivos que el .ds1 del nivel lista (%d)\n", len(filesDS1))

		for i := range filesDS1 {
			nombre := sinPrefijo(filesDS1[i])
			tam := 0

			for _, a := range archivos {
				if a.nombre == nombre {
					tam = a.crudo
				}
			}

			fmt.Printf("  %-40s %10d B\n", nombre, tam)
		}
	}

	fmt.Printf("\n== el .ds1 del nivel\n")
	fmt.Printf("  %s: %d B crudos, %dx%d, %d capas, %d objetos\n", *ds1Pedido, crudoDS1, ancho, alto, capas, objetos)

	fmt.Printf("\nFIN\n")
}

// tamanoDeMapTileConsola: el `unsafe.Sizeof` de `d2mapengine.MapTile` en la consola, medido en DC-002.82
// (el paquete no se importa aca para no arrastrar el host a ebiten).
func tamanoDeMapTileConsola() int { return 264 }

func boolTexto(v bool) string {
	if v {
		return "si"
	}

	return "no"
}

func abs(v int) int {
	if v < 0 {
		return -v
	}

	return v
}
