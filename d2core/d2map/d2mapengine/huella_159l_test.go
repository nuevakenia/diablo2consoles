package d2mapengine

// DC-002.159l -- LA PRUEBA DEL HOST QUE FIJA LA HUELLA DEL PUEBLO COMPUESTO.
//
// QUE FIJA, Y POR QUE EN EL HOST. En la consola el recolector de libgodc NO recorre el interior de los bloques
// externos (los pedidos de mas de 64 KB, `gc_heap.c:107`), asi que las rebanadas de cada casilla compuesta
// quedaban sin raiz y la primera recoleccion que caia durante la composicion se las llevaba. Medido en
// DC-002.159k: la huella del pueblo cambiaba sola entre `casillas-puestas` y `tras-recolector` (803 -> 801
// pisos, hash distinto, y valores distintos entre corridas del mismo binario).
//
// Esta prueba compone el pueblo DE VERDAD (los `.ds1`/`.dt1` del juego, por el mismo camino del motor que la
// consola: `LoadStamp` -> `PrepararLasCasillasEnElArena159l` -> `ResetMapParaStamp` -> `PlaceStampClippedWhere`)
// y exige tres cosas:
//
//  1. la huella del host es la de referencia: 2.337 pisos, 338 paredes (los numeros con los que se compara la
//     consola desde DC-002.159k);
//  2. TODAS las rebanadas de las casillas caen ADENTRO del arena de casillas -- o sea, fuera del semiespacio
//     del recolector, que es la propiedad que hace que la consola no las pierda;
//  3. la huella NO se mueve cuando el recolector pasa (dos `runtime.GC()` completos) ni cuando el `.ds1` se
//     suelta despues de componer.
//
// En el host el recolector es exacto: la prueba no puede reproducir el barrido conservador de libgodc. Lo que
// si puede -- y es lo que hace -- es fijar el contenido y la propiedad estructural que lo protege, para que
// cualquier cambio futuro que devuelva las rebanadas al semiespacio sin raiz rompa aca y no en el emulador.
//
// DATOS: se corre con la instalacion del juego (`D2_DIR`, o ~/.wine-d2/drive_c/Games/Diablo II). Sin los MPQ
// la prueba se SALTA (no se inventa un mapa sintetico: la huella de referencia es la del pueblo de verdad).

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
)

const (
	// los tres numeros de la huella de referencia del pueblo (DC-002.159k, host)
	pisosDelPueblo159l   = 2337
	paredesDelPueblo159l = 338

	// el FNV del host de DC-002.159k (= -1590544905 con signo). Es el numero que la consola tiene que repetir
	fnvDelPueblo159l = 2704422391

	// el arena del pueblo: 2.337 casillas x 4 rebanadas
	tilesDelArenaPueblo159l = 9348
)

func dirDelJuego159l(t *testing.T) string {
	t.Helper()

	dir := os.Getenv("D2_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("sin HOME: %v", err)
		}

		dir = filepath.Join(home, ".wine-d2", "drive_c", "Games", "Diablo II")
	}

	if _, err := os.Stat(filepath.Join(dir, "d2data.mpq")); err != nil {
		t.Skipf("datos del juego en %s: %v (pone D2_DIR)", dir, err)
	}

	return dir
}

func TestHuellaDelPuebloEnElArena159l(t *testing.T) {
	dir := dirDelJuego159l(t)

	am, err := d2asset.NewAssetManager(d2util.LogLevelError)
	if err != nil {
		t.Fatalf("NewAssetManager: %v", err)
	}

	// el ORDEN importa: patch_D2 primero (es el que trae las tablas del compositor)
	for _, nombre := range []string{"patch_d2.mpq", "d2data.mpq"} {
		if err := am.AddSource(filepath.Join(dir, nombre), types.AssetSourceMPQ); err != nil {
			t.Fatalf("AddSource(%s): %v", nombre, err)
		}
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

	t.Cleanup(func() { ReusarLasCasillasEnSuLugar(false) })

	tamano := stamp.Size()
	if tamano.Width != 57 || tamano.Height != 41 {
		t.Fatalf("el pueblo del MPQ mide %dx%d, se esperaba 57x41", tamano.Width, tamano.Height)
	}

	// ============================== el arreglo de DC-002.159l ==============================
	tiles, prendido := PrepararLasCasillasEnElArena159l(tamano.Width, tamano.Height)
	if !prendido || tiles != tilesDelArenaPueblo159l {
		t.Fatalf("arena de casillas: tiles=%d prendido=%v (se esperaba %d y prendido)",
			tiles, prendido, tilesDelArenaPueblo159l)
	}

	engine.ResetMapParaStamp(d2enum.RegionAct1Town, tamano.Width, tamano.Height, stamp)
	engine.PlaceStampClippedWhere(stamp, 0, 0, tamano.Width, tamano.Height, func(int) bool { return false }, false)

	if _, usados, fallas := EstadoDelArenaDeCasillas(); usados != tilesDelArenaPueblo159l || fallas != 0 {
		t.Fatalf("el arena se uso mal: usados=%d fallas=%d (se esperaba %d y 0)",
			usados, fallas, tilesDelArenaPueblo159l)
	}

	pisos1, paredes1, fnv1 := engine.HuellaDelMapa159l()

	if pisos1 != pisosDelPueblo159l || paredes1 != paredesDelPueblo159l {
		t.Fatalf("huella del pueblo: pisos=%d paredes=%d (referencia: %d/%d)",
			pisos1, paredes1, pisosDelPueblo159l, paredesDelPueblo159l)
	}

	if fnv1 != fnvDelPueblo159l {
		t.Fatalf("huella del pueblo: fnv=%d (referencia: %d)", fnv1, fnvDelPueblo159l)
	}

	t.Logf("huella casillas-puestas pisos=%d paredes=%d fnv=%d", pisos1, paredes1, fnv1)

	// ============================== las rebanadas, adentro del arena ==============================
	//
	// Es la propiedad que el defecto de 159k rompia: si alguna rebanada cae en el semiespacio, el recolector de
	// libgodc puede llevarsela sin que nadie la vea (la unica referencia esta adentro del bloque externo de
	// `m.tiles`, que el Cheney no escanea).
	desde := uintptr(unsafe.Pointer(&arenaDeCasillas[0]))
	hasta := desde + uintptr(len(arenaDeCasillas))*unsafe.Sizeof(arenaDeCasillas[0])

	for i := range engine.tiles {
		c := &engine.tiles[i].Components

		rebanadas := []struct {
			nombre string
			datos  []d2ds1.Tile
		}{
			{"Walls", c.Walls},
			{"Floors", c.Floors},
			{"Shadows", c.Shadows},
			{"Substitutions", c.Substitutions},
		}

		for _, r := range rebanadas {
			if len(r.datos) == 0 {
				continue
			}

			primera := uintptr(unsafe.Pointer(&r.datos[0]))
			if primera < desde || primera >= hasta {
				t.Fatalf("casilla %d: la rebanada %s (0x%x) NO esta en el arena [0x%x,0x%x)",
					i, r.nombre, primera, desde, hasta)
			}
		}
	}

	// ============================== el recolector y el .ds1 suelto ==============================
	//
	// Soltar el `.ds1` no puede tocar las casillas (son COPIAS de las celdas, no punteros). En el host no hay
	// liberador instalado y devuelve 0: se llama igual porque es el orden de la consola (DC-002.110).
	if liberados := stamp.SoltarLosTilesDelDS1(); liberados != 0 {
		t.Fatalf("en el host no hay liberador de la arena del .ds1 y devolvio %d B", liberados)
	}

	runtime.GC()
	runtime.GC()

	pisos2, paredes2, fnv2 := engine.HuellaDelMapa159l()

	t.Logf("huella tras-recolector pisos=%d paredes=%d fnv=%d", pisos2, paredes2, fnv2)

	if pisos1 != pisos2 || paredes1 != paredes2 || fnv1 != fnv2 {
		t.Fatalf("la huella se movio con el recolector: antes %d/%d/%d, despues %d/%d/%d",
			pisos1, paredes1, fnv1, pisos2, paredes2, fnv2)
	}
}
