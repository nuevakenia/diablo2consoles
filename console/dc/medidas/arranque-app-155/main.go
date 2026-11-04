// DC-002.155 -- EL PESO DEL ARRANQUE DEL MOTOR EN EL HOST, PASO POR PASO (Gate 0).
//
// PARA QUE. Gate 0 es "el motor pinta un cuadro del pueblo en la consola", y la consola tiene 2.450.496 B
// de monton de C con el binario entero adentro (DC-002.93). Antes de compilar nada hay que saber QUE PIDE
// cada paso del arranque de `d2app`, con el numero: si la suma no entra, la tarjeta no se compila.
//
// QUE MIDE, en el mismo orden en que los pide `d2app.initialize`:
//
//	1. los MPQ abiertos (`AddSource`, que lee cabecera + tablas de hash y de bloques)
//	2. el idioma (`LoadLanguage`)
//	3. SOLO las tablas del pueblo (LvlTypes + LvlPrest: el subconjunto de DC-002.86)
//	4. `animdata.d2` (lo que carga `initAnimationData`)
//	5. las tres tablas de strings (`loadStrings`)
//	6. las DOS animaciones del `d2gui` (cursor y pantalla de carga, expandidas a RGBA por el motor)
//
// El numero es `HeapAlloc` del runtime, que es lo que el motor pide; la consola ademas tiene el monton de C
// para las asignaciones grandes, asi que el host SUBESTIMA (punteros de 8 B) y hay que leerlo como piso.
//
// Uso:  go run ./console/dc/medidas/arranque-app-155 [-dir <carpeta con los MPQ>]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2animdata"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
)

// tablasDelPueblo: el subconjunto medido en DC-002.86 -- las dos que el compositor de casillas no puede
// dejar de tener (`LoadStamp` lee Presets y Types; `ResetMap` recorre `levelType.Files`). Todo lo demas
// (monstats, Objects, Levels, AutoMap...) es de las entidades o de los mapas automaticos, no del pueblo.
var tablasDelPueblo = []string{
	d2resource.LevelType,
	d2resource.LevelPreset,
}

func main() {
	dir := flag.String("dir", "", "carpeta con los MPQ (vacio: D2_DIR o ~/.wine-d2/drive_c/Games/Diablo II)")
	flag.Parse()

	gameDir := *dir
	if gameDir == "" {
		gameDir = os.Getenv("D2_DIR")
	}

	if gameDir == "" {
		home, _ := os.UserHomeDir()
		gameDir = filepath.Join(home, ".wine-d2", "drive_c", "Games", "Diablo II")
	}

	mpqs := []string{"patch_D2.mpq", "d2data.mpq", "d2char.mpq"}

	fmt.Println("ARRANQUE155: carpeta del juego:", gameDir)
	fmt.Println("ARRANQUE155: sizeof int =", 8, "(host amd64; en SH-4 son 4: los structs bajan)")

	base := heap()

	am, err := d2asset.NewAssetManager(d2util.LogLevelDefault)
	if err != nil {
		fmt.Println("ARRANQUE155: NewAssetManager FALLO:", err)

		return
	}

	paso("1. asset manager vacio", base)

	for _, nombre := range mpqs {
		ruta := filepath.Join(gameDir, nombre)
		if err := am.AddSource(ruta, types.AssetSourceMPQ); err != nil {
			fmt.Println("ARRANQUE155: AddSource FALLO:", ruta, err)

			return
		}

		paso("1. MPQ abierto "+nombre, base)
	}

	base = heap()

	lang := am.LoadLanguage(d2resource.LocalLanguage)
	am.Loader.SetLanguage(&lang)
	charset := am.FontDir()
	am.Loader.SetCharset(&charset)
	paso("2. idioma + charset", base)

	base = heap()

	for _, tabla := range tablasDelPueblo {
		if err := am.LoadRecords(tabla); err != nil {
			fmt.Println("ARRANQUE155: tabla FALLO:", tabla, err)

			return
		}

		paso("3. tabla "+tabla, base)
	}

	base = heap()

	datos, err := am.LoadFile(d2resource.AnimationData)
	if err != nil {
		fmt.Println("ARRANQUE155: animdata FALLO:", err)
	} else {
		fmt.Println("ARRANQUE155: animdata.d2 crudo =", len(datos), "B")

		anim, err := d2animdata.Load(datos)
		if err != nil {
			fmt.Println("ARRANQUE155: animdata Load FALLO:", err)
		} else {
			fmt.Println("ARRANQUE155: animdata registros =", anim.GetRecordsCount())
		}

		paso("4. animdata.d2 parseada", base)
	}

	base = heap()

	for _, tabla := range []string{d2resource.PatchStringTable, d2resource.ExpansionStringTable, d2resource.StringTable} {
		if _, err := am.LoadStringTable(tabla); err != nil {
			fmt.Println("ARRANQUE155: string table FALLO:", tabla, err)

			continue
		}

		paso("5. strings "+tabla, base)
	}

	base = heap()

	if anim, err := am.LoadAnimation(d2resource.CursorDefault, d2resource.PaletteUnits); err != nil {
		fmt.Println("ARRANQUE155: cursor FALLO:", err)
	} else {
		fmt.Println("ARRANQUE155: cursor   direcciones=", anim.GetDirectionCount(), "cuadros=", anim.GetFrameCount())
	}

	paso("6. animacion del cursor (RGBA)", base)

	base = heap()

	if anim, err := am.LoadAnimation(d2resource.LoadingScreen, d2resource.PaletteLoading); err != nil {
		fmt.Println("ARRANQUE155: loadingscreen FALLO:", err)
	} else {
		w, h := anim.GetFrameBounds()
		fmt.Println("ARRANQUE155: carga    direcciones=", anim.GetDirectionCount(), "cuadros=", anim.GetFrameCount(),
			"tamano=", w, "x", h)
	}

	paso("6. animacion de la pantalla de carga (RGBA)", base)

	fmt.Println("ARRANQUE155: FIN")
}

func heap() uint64 {
	var m runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&m)

	return m.HeapAlloc
}

func paso(nombre string, base uint64) {
	var m runtime.MemStats

	runtime.ReadMemStats(&m)

	fmt.Printf("ARRANQUE155: %-46s heap=%8d B  delta=%8d B\n", nombre, m.HeapAlloc, int64(m.HeapAlloc)-int64(base))
}
