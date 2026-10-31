// DC-002.86 (instrumento del HOST): CUANTO SE USA DE VERDAD, TABLA POR TABLA, al cargar el pueblo con
// `d2mapengine`. Es el medidor de DC-002.82 (`console/dc/medidas/tablas`, que mide lo CARGADO) con el otro
// lado del numero: lo USADO.
//
// POR QUE HACE FALTA EL OTRO LADO. DC-002.82 midio lo cargado (las 12 tablas del compositor 4.243.176 B,
// 405 % del semiespacio) y en la consola entran 4 de 12. Pero cargar una tabla de 574 filas para usar 14
// filas no es lo mismo que cargarla porque se usan las 574: sin el numero de lo usado no se puede decidir
// entre recortar la carga (camino a), pedir por registro (b) o mudar la tabla al monton externo (c).
//
// DE DONDE SALE "LO USADO", SIN SUPOSICIONES. El instrumento carga las 12 tablas por el camino del motor
// (igual que DC-002.82) y despues HACE LAS MISMAS LECTURAS QUE HACE EL COMPOSITOR, con el pueblo real:
//
//   - `d2mapengine.ResetMap` / `LoadStamp`: `Records.Level.Types[RegionAct1Town]` y
//     `Records.Level.Presets[preset]` (engine.go:113, d2mapstamp/factory.go:49 y :79) -> 1 fila de cada una.
//   - `d2mapstamp.Stamp.EntitiesWhere` (el camino que crea entidades): por cada objeto del .ds1 del pueblo,
//     si es Character -> `Records.Monster.Presets[act][id]` -> `Records.Monster.Stats[nombre]` ->
//     `Records.Monster.Stats2[monstat.ExtraDataKey]` (stamp.go:145-146, d2mapentity/factory.go:233); si es
//     Item -> `Records.FindObject(act, typ, id).ObjectsTxtId` -> `Records.Object.Details[id]`
//     (stamp.go:165-171). Se cuentan las filas DISTINTAS que el pueblo pide.
//   - `LvlWarp`, `AutoMap`, `Levels`, `LvlMaze`, `LvlSub`, `objtype`: el compositor de casillas y de
//     entidades NO las toca (no hay una sola lectura de esos Records en d2mapengine/d2mapstamp; el
//     `grep` esta en la evidencia). Se informan con 0 filas usadas y con la cita del porque.
//
// Hay DOS numeros de "usado", porque son dos cosas distintas y confundirlas fue lo que dejo a DC-002.82 sin
// respuesta:
//
//	CASILLAS   lo que pide la configuracion medida hoy (`PlaceStampClippedWhere` con keep=false, o sea sin
//	           entidades): Types + Presets, y nada mas.
//	ENTIDADES  lo que pediria la misma fase con las entidades prendidas (`conEntidades=true`): ademas
//	           monpreset/monstats/monstats2/Objects, con las filas del .ds1 del pueblo.
//
// Uso:
//
//	go run ./console/dc/medidas/uso-tablas -dir ~/.wine-d2/drive_c/Games/'Diablo II'
//	go run ./console/dc/medidas/uso-tablas -dir ... -csv /tmp/uso.csv
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// el pueblo del acto 1, los mismos numeros que el POC de consola (tablas-mapengine/main.go).
const (
	regionPueblo = d2enum.RegionAct1Town // 1 -> Level.Types[1] "Act 1 - Town"
	presetPueblo = 1                     // Level.Presets[1] "Act 1 - Town 1"
	ds1Pueblo    = `data\global\tiles\act1\town\townn1.ds1`
)

// tablasDelCompositor: las 12 de DC-002.82, con el porque de cada una (las que usa y las que no).
var tablasDelCompositor = []struct {
	path  string
	uso   string // quien la lee al cargar el pueblo (codigo, no suposicion)
	usada bool
}{
	{d2resource.LevelType, "ResetMap/LoadStamp: Types[1]", true},
	{d2resource.LevelPreset, "LoadStamp: Presets[1]", true},
	{d2resource.LevelWarp, "NADIE en el compositor (d2mapengine/warps.go usa los warps del DS1, no la tabla)", false},
	{d2resource.AutoMap, "NADIE en el compositor (lo usa d2gamescreen para el automapa)", false},
	{d2resource.LevelDetails, "NADIE en el compositor (lo usa d2level/d2gamescreen)", false},
	{d2resource.LevelMaze, "NADIE en el compositor (solo los generadores de laberinto)", false},
	{d2resource.LevelSubstitutions, "NADIE en el compositor (lo usa d2mapgen)", false},
	{d2resource.ObjectType, "NADIE en el compositor (objtype lo usa d2mapentity para la colision, no la carga)", false},
	{d2resource.ObjectDetails, "Stamp.EntitiesWhere: Details[lookup.ObjectsTxtId]", true},
	{d2resource.MonPreset, "Stamp.EntitiesWhere: Presets[act][id]", true},
	{d2resource.MonStats, "Stamp.EntitiesWhere: Stats[preset]", true},
	{d2resource.MonStats2, "d2mapentity.NewNPC: Stats2[monstat.ExtraDataKey]", true},
}

type fila struct {
	tabla    string
	uso      string
	filas    int   // filas que tiene la tabla cargada
	usadas   int   // filas distintas que el pueblo pide (casillas + entidades)
	usadasSo int   // filas distintas que pide solo la composicion de casillas
	bytes    int64 // lo que ocupa la tabla cargada en el host (delta de HeapAlloc)
	ms       int64
	detalle  string
	fallo    string
}

func vivo() (int64, int64) {
	var ms runtime.MemStats

	runtime.GC()
	runtime.GC()
	runtime.ReadMemStats(&ms)

	return int64(ms.HeapAlloc), int64(ms.HeapObjects)
}

var mpqs []*d2mpq.MPQ

func leer(nombre string) ([]byte, error) {
	clave := strings.TrimPrefix(nombre, "/")
	clave = strings.ReplaceAll(clave, "/", "\\")

	var ultimo error

	for _, m := range mpqs {
		datos, err := m.ReadFile(clave)
		if err == nil {
			return datos, nil
		}

		ultimo = err
	}

	return nil, ultimo
}

func main() {
	dir := flag.String("dir", "", "carpeta con d2data.mpq, patch_d2.mpq y d2char.mpq")
	csv := flag.String("csv", "", "escribir las filas medidas a este archivo")
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

	// el orden del motor: el parche primero (es donde viven las tablas), los datos despues (el .ds1).
	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq"} {
		real, hay := enDisco[nombre]
		if !hay {
			fmt.Fprintf(os.Stderr, "no hay %s en %s\n", nombre, *dir)
			os.Exit(1)
		}

		m, e := d2mpq.FromFile(*dir + "/" + real)
		if e != nil {
			fmt.Fprintf(os.Stderr, "no pude abrir %s: %v\n", real, e)
			os.Exit(1)
		}

		fmt.Println("MPQ:", real)

		mpqs = append(mpqs, m)
	}

	rm, err := d2records.NewRecordManager(d2util.LogLevelNone)
	if err != nil {
		fmt.Fprintln(os.Stderr, "NewRecordManager:", err)
		os.Exit(1)
	}

	// ============================== 1. lo CARGADO, tabla por tabla ==============================
	fmt.Printf("\n== lo CARGADO (delta de HeapAlloc por tabla, el medidor de DC-002.82)\n")
	fmt.Printf("%-32s %8s %10s %10s %6s\n", "tabla", "filas", "vivo B", "objetos", "ms")

	var filas []*fila

	for i := range tablasDelCompositor {
		t := tablasDelCompositor[i]
		f := &fila{tabla: t.path, uso: t.uso}

		antes, _ := vivo()
		t0 := time.Now()

		errCarga := func() error {
			datos, errLeer := leer(t.path)
			if errLeer != nil {
				return errLeer
			}

			dict := d2txt.LoadDataDictionary(datos)
			errLoad := rm.Load(t.path, dict)

			dict = nil
			datos = nil

			return errLoad
		}()

		f.ms = time.Since(t0).Milliseconds()
		despues, _ := vivo()

		if errCarga != nil {
			f.fallo = errCarga.Error()
			fmt.Printf("%-32s  FALLO: %v\n", t.path, errCarga)

			filas = append(filas, f)

			continue
		}

		f.bytes = despues - antes
		f.filas = cuenta(rm, t.path)

		fmt.Printf("%-32s %8d %10d %10s %6d\n", t.path, f.filas, f.bytes, "", f.ms)

		filas = append(filas, f)
	}

	// ============================== 2. lo USADO por el pueblo ==============================
	//
	// El .ds1 del pueblo: los objetos que el compositor va a pedir cuando las entidades esten prendidas.
	crudoDS1, errDS1 := leer(ds1Pueblo)
	if errDS1 != nil {
		fmt.Fprintf(os.Stderr, "no pude leer el .ds1 del pueblo (%s): %v\n", ds1Pueblo, errDS1)
		os.Exit(1)
	}

	ds1, errUnmarshal := d2ds1.Unmarshal(crudoDS1)
	if errUnmarshal != nil {
		fmt.Fprintf(os.Stderr, "no pude desarmar el .ds1 del pueblo: %v\n", errUnmarshal)
		os.Exit(1)
	}

	monstruos, objetos, detalle := filasQuePideElPueblo(rm, ds1)

	usadas := map[string]int{
		d2resource.LevelType:     1, // Types[1]
		d2resource.LevelPreset:   1, // Presets[1]
		d2resource.MonPreset:     monstruos.presets,
		d2resource.MonStats:      monstruos.stats,
		d2resource.MonStats2:     monstruos.stats2,
		d2resource.ObjectDetails: objetos,
	}

	soloCasillas := map[string]int{
		d2resource.LevelType:   1,
		d2resource.LevelPreset: 1,
	}

	// ============================== 3. el veredicto, tabla por tabla ==============================
	fmt.Printf("\n== lo USADO vs lo CARGADO, al cargar el pueblo (RegionAct1Town, preset %d, %s)\n",
		presetPueblo, ds1Pueblo)
	fmt.Printf("   el .ds1 del pueblo: %dx%d, %d archivos .dt1, %d objetos\n",
		ds1.Width(), ds1.Height(), len(ds1.Files), len(ds1.Objects))
	fmt.Printf("   act=%d  %s\n\n", ds1.Act, detalle)

	fmt.Printf("%-24s %7s %7s %6s %7s %11s %9s %11s\n",
		"tabla", "filas", "usadas", "%uso", "casillas", "cargada B", "usado B", "por fila B")

	var totalCargado, totalUsado, totalCasillas int64

	orden := append([]*fila(nil), filas...)
	sort.Slice(orden, func(a, b int) bool { return orden[a].bytes > orden[b].bytes })

	csvB := strings.Builder{}
	csvB.WriteString("tabla;filas;usadas;usadas_casillas;cargada_B;usado_B;bytes_por_fila;uso\n")

	for _, f := range orden {
		if f.fallo != "" {
			continue
		}

		f.usadas = usadas[f.tabla]
		f.usadasSo = soloCasillas[f.tabla]

		porFila := int64(0)
		if f.filas > 0 {
			porFila = f.bytes / int64(f.filas)
		}

		usadoBytes := porFila * int64(f.usadas)
		pctUso := 0.0
		if f.filas > 0 {
			pctUso = 100 * float64(f.usadas) / float64(f.filas)
		}

		totalCargado += f.bytes
		totalUsado += usadoBytes

		casillasBytes := porFila * int64(f.usadasSo)
		totalCasillas += casillasBytes

		fmt.Printf("%-24s %7d %7d %5.1f%% %7d %11d %9d %11d\n",
			strings.TrimPrefix(f.tabla, "/data/global/excel/"), f.filas, f.usadas, pctUso,
			f.usadasSo, f.bytes, usadoBytes, porFila)

		fmt.Fprintf(&csvB, "%s;%d;%d;%d;%d;%d;%d;%s\n",
			f.tabla, f.filas, f.usadas, f.usadasSo, f.bytes, usadoBytes, porFila, f.uso)
	}

	fmt.Printf("\n   TOTAL de las 12 tablas cargadas: %d B (%.2f MB) = %.0f %% del semiespacio de 1024 KB\n",
		totalCargado, float64(totalCargado)/1048576.0, 100*float64(totalCargado)/1048576.0)
	fmt.Printf("   TOTAL con SOLO las filas del pueblo (casillas + entidades): %d B (%.1f %% de lo cargado)\n",
		totalUsado, 100*float64(totalUsado)/float64(totalCargado))
	fmt.Printf("   TOTAL con SOLO las filas de la composicion de casillas:     %d B (participacion de Types+Presets)\n",
		totalCasillas)

	fmt.Printf("\n== la lista de filas que pide el pueblo\n")
	fmt.Printf("   %s\n   %s\n   %s\n", detalle, monstruos.detalle, objetosDetalle(rm, ds1))

	// ============================== 2b. el PRESUPUESTO del compositor ==============================
	//
	// No es lo usado por las tablas: es lo que el compositor pide ADEMAS de las tablas, medido con los
	// archivos del pueblo y los sizeof del binario de consola (los imprime el POC: MapTile 264, d2dt1.Tile
	// 272, d2ds1.Tile 20 en 32 bits). Sin este numero no se puede decir si las tablas son la pared o no.
	presupuesto(rm, ds1)

	// el detalle de las que NO se usan: se dice con el codigo en la mano, no por omision
	fmt.Printf("\n== las que el compositor NO toca (0 filas usadas), con la cita\n")
	for i := range tablasDelCompositor {
		if !tablasDelCompositor[i].usada {
			fmt.Printf("   %-34s %s\n", strings.TrimPrefix(tablasDelCompositor[i].path, "/data/global/excel/"),
				tablasDelCompositor[i].uso)
		}
	}

	runtime.KeepAlive(rm)

	if *csv != "" {
		if errW := os.WriteFile(*csv, []byte(csvB.String()), 0o644); errW != nil {
			fmt.Fprintln(os.Stderr, "no pude escribir el csv:", errW)
		} else {
			fmt.Println("\ncsv:", *csv)
		}
	}
}

// cuenta: las filas que quedaron en los Records para esa tabla (es lo que la tabla CARGO, no lo que el
// archivo tiene: con un fallo de carga el numero baja y eso es informacion).
func cuenta(rm *d2records.RecordManager, path string) int {
	switch path {
	case d2resource.LevelType:
		return len(rm.Level.Types)
	case d2resource.LevelPreset:
		return len(rm.Level.Presets)
	case d2resource.LevelWarp:
		return len(rm.Level.Warp)
	case d2resource.AutoMap:
		return len(rm.Level.AutoMaps)
	case d2resource.LevelDetails:
		return len(rm.Level.Details)
	case d2resource.LevelMaze:
		return len(rm.Level.Maze)
	case d2resource.LevelSubstitutions:
		return len(rm.Level.Sub)
	case d2resource.ObjectType:
		return len(rm.Object.Types)
	case d2resource.ObjectDetails:
		return len(rm.Object.Details)
	case d2resource.MonPreset:
		n := 0
		for _, lista := range rm.Monster.Presets {
			n += len(lista)
		}

		return n
	case d2resource.MonStats:
		return len(rm.Monster.Stats)
	case d2resource.MonStats2:
		return len(rm.Monster.Stats2)
	}

	return 0
}

type pideMonstruos struct {
	presets, stats, stats2 int
	detalle                string
}

// filasQuePideElPueblo: hace EXACTAMENTE las lecturas de `Stamp.EntitiesWhere` con los objetos del .ds1
// del pueblo y devuelve cuantas filas DISTINTAS toca de cada tabla. Los objetos Character son NPCs/monstruos
// (monpreset -> monstats -> monstats2) y los Item son objetos del juego (lookups -> Objects.txt).
func filasQuePideElPueblo(rm *d2records.RecordManager, ds1 *d2ds1.DS1) (m pideMonstruos, objetos int, detalle string) {
	presets := map[string]bool{}
	stats := map[string]bool{}
	stats2 := map[string]bool{}
	idsObjetos := map[int]bool{}
	caracteres, items, otros := 0, 0, 0

	for i := range ds1.Objects {
		o := &ds1.Objects[i]

		switch o.Type {
		case int(d2enum.ObjectTypeCharacter):
			caracteres++

			lista, hay := rm.Monster.Presets[ds1.Act]
			if !hay || o.ID >= len(lista) {
				continue
			}

			nombre := lista[o.ID]
			if nombre == "" {
				continue
			}

			presets[nombre] = true

			mon := rm.Monster.Stats[nombre]
			if mon == nil {
				continue
			}

			stats[nombre] = true

			if _, hay2 := rm.Monster.Stats2[mon.ExtraDataKey]; hay2 {
				stats2[mon.ExtraDataKey] = true
			}
		case int(d2enum.ObjectTypeItem):
			items++

			lookup := rm.FindObject(int(ds1.Act), o.Type, o.ID)
			if lookup == nil {
				continue
			}

			if _, hay := rm.Object.Details[lookup.ObjectsTxtId]; hay {
				idsObjetos[lookup.ObjectsTxtId] = true
			}
		default:
			otros++
		}
	}

	m.presets = len(presets)
	m.stats = len(stats)
	m.stats2 = len(stats2)
	m.detalle = fmt.Sprintf("objetos del .ds1: %d Character (monstruos/NPC), %d Item, %d de otro tipo",
		caracteres, items, otros)

	objetos = len(idsObjetos)

	return m, objetos, m.detalle
}

// objetosDetalle: que ids de Objects.txt pide el pueblo (para que el numero se pueda auditar fila por fila).
func objetosDetalle(rm *d2records.RecordManager, ds1 *d2ds1.DS1) string {
	ids := map[int]bool{}

	for i := range ds1.Objects {
		o := &ds1.Objects[i]
		if o.Type != int(d2enum.ObjectTypeItem) {
			continue
		}

		if lookup := rm.FindObject(int(ds1.Act), o.Type, o.ID); lookup != nil {
			ids[lookup.ObjectsTxtId] = true
		}
	}

	lista := make([]int, 0, len(ids))
	for id := range ids {
		lista = append(lista, id)
	}

	sort.Ints(lista)

	return fmt.Sprintf("Objects.txt: %d filas distintas %v", len(lista), lista)
}

// presupuesto: lo que el compositor pide ADEMAS de las tablas, con los archivos del pueblo y los sizeof del
// binario de consola (los imprime el POC: en 32 bits MapTile 264 B, d2dt1.Tile 272 B, d2ds1.Tile 20 B).
//
// Es el numero que hace falta para saber si las tablas son la pared: si el arena de tiles sola ya no entra,
// recortar las tablas no cambia nada.
func presupuesto(rm *d2records.RecordManager, ds1 *d2ds1.DS1) {
	lt := rm.Level.Types[regionPueblo]
	if lt == nil {
		fmt.Println("\n   (no hay level type para el pueblo: sin presupuesto)")

		return
	}

	fmt.Printf("\n== el PRESUPUESTO del compositor, con los archivos del pueblo\n")

	var crudos, tiles int

	for i := range lt.Files {
		nombre := lt.Files[i]
		if nombre == "" || nombre == "0" {
			continue
		}

		clave := "data\\global\\tiles\\" + strings.ReplaceAll(strings.ToLower(nombre), "/", "\\")

		datos, err := leer(clave)
		if err != nil {
			fmt.Printf("   %-46s FALLO: %v\n", nombre, err)

			continue
		}

		crudos += len(datos)

		dt1, errDT1 := d2dt1.LoadDT1(datos)
		if errDT1 != nil {
			fmt.Printf("   %-46s %8d B  FALLO: %v\n", nombre, len(datos), errDT1)

			continue
		}

		tiles += len(dt1.Tiles)

		fmt.Printf("   %-46s %8d B  %4d tiles\n", nombre, len(datos), len(dt1.Tiles))
	}

	// dc002.82: el .ds1 se desarma con 4 capas (2 de piso + 2 de pared) de casillas de 20 B cada una,
	// dimensionadas al tamanio del mapa (el `refresh` del DS1 pide una casilla por celda y capa).
	capasDS1 := 4

	// Los sizeof son los que IMPRIME el POC en la consola (32 bits): `sizeof int= 4 puntero= 4
	// d2ds1.Tile= 20 d2dt1.Tile= 272 MapTile= 264` (log dc-002.82-consola-tablas-12.log). No se sacan de
	// `unsafe.Sizeof` aca porque este instrumento corre en el host (64 bits) y daria el doble.
	mapTile := 264
	tileDT1 := 272
	tileDS1 := 20

	casillas := ds1.Width() * ds1.Height()

	arena := crudos + crudos/10    // el 10 % de aire de capacidadDeLaArenaDeTiles
	const arenaDelCodigo = 4848148 // capacidadDeLaArenaDeTiles de d2asset (dimensionada con el POC de zonas)
	dt1Data := tiles * tileDT1
	mapa := casillas * mapTile
	ds1Bytes := capasDS1 * casillas * tileDS1

	fmt.Printf("\n   arena de tiles dimensionada al pueblo (crudos + 10 %% de aire):   %8d B (%.2f MB)\n",
		arena, float64(arena)/1048576.0)
	fmt.Printf("   arena de tiles que pide HOY el codigo (constante de d2asset):      %8d B\n", arenaDelCodigo)
	fmt.Printf("   dt1TileData (delta de casillas .dt1 x %d B):                      %8d B\n", tileDT1, dt1Data)
	fmt.Printf("   casillas del mapa (%dx%d = %d x %d B):                          %8d B\n",
		ds1.Width(), ds1.Height(), casillas, mapTile, mapa)
	fmt.Printf("   .ds1 desarmado (%d capas x %d casillas x %d B):                  %8d B\n",
		capasDS1, casillas, tileDS1, ds1Bytes)
	fmt.Printf("   --------------------------------------------------------------------------\n")
	fmt.Printf("   estructuras del compositor (dt1TileData + casillas + .ds1):       %8d B\n", dt1Data+mapa+ds1Bytes)

	for _, caso := range []struct {
		rotulo string
		arena  int
	}{
		{"con el arena CONSTANTE del codigo", arenaDelCodigo},
		{"con el arena dimensionado al pueblo", arena},
	} {
		total := caso.arena + dt1Data + mapa + ds1Bytes + 352256

		fmt.Printf("   %-38s con CERO tablas: %8d B (%.2f MB) -> faltan %8d B (margen 5.230.592)\n",
			caso.rotulo, total, float64(total)/1048576.0, total-5230592)
	}

}
