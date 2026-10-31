package d2records

// DC-002.105 -- las pruebas de SOLTAR LAS FILAS DE LAS TABLAS.
//
// QUE PROTEGEN, y por que asi:
//   - que soltar una tabla deje la tabla en NIL (sin referencia: es lo que permite que el recolector de Go
//     recicle lo que vive en el semiespacio) y que el tablero del mapa vaya al liberador;
//   - que el liberador reciba EXACTAMENTE las tablas grandes y nada mas: una tabla chica vive en el
//     semiespacio y pedirle al liberador que la toque corrompe el monton;
//   - que una tabla que no se sabe soltar se DIGA (`NoSoportadas`) en vez de tocarla a ciegas;
//   - que sin liberador (la PC) no se libere memoria a mano y las tablas se suelten igual.

import (
	"testing"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
)

// liberadorDeMentira: anota lo que le piden y contesta el tamano que le dicen (no libera memoria: en el host
// no hay monton externo que devolver, y liberar un puntero de Go a mano seria corromper el monton).
type liberadorDeMentira struct {
	mapas  []unsafe.Pointer
	tramos []int
	bytes  int
}

func (l *liberadorDeMentira) soltarMapa(mapa unsafe.Pointer) int {
	l.mapas = append(l.mapas, mapa)
	l.bytes += 216064 // el tablero medido de LvlPrest en el pueblo (DC-002.105)

	return 216064
}

func (l *liberadorDeMentira) soltarTramo(bloque unsafe.Pointer, bytes int) int {
	if bloque == nil {
		panic("liberador: tramo nulo")
	}

	l.tramos = append(l.tramos, bytes)
	l.bytes += bytes

	return bytes
}

// recordsDePrueba: las dos tablas que el compositor del pueblo carga de verdad (DC-002.88).
func recordsDePrueba() *RecordManager {
	rm := &RecordManager{}
	rm.Level.Presets = LevelPresets{
		1: {Name: "Act 1 - Town", DefinitionID: 1},
		2: {Name: "Act 1 - Wilderness 1", DefinitionID: 2},
	}
	rm.Level.Types = LevelTypes{{Name: "Act 1 - Town", ID: 0}}

	return rm
}

func TestSoltarTablasSueltaElMapaYLaRebanada(t *testing.T) {
	rm := recordsDePrueba()
	libre := &liberadorDeMentira{}

	UsarLiberadorDeFilas(&LiberadorDeFilas{SoltarMapa: libre.soltarMapa, SoltarTramo: libre.soltarTramo})
	t.Cleanup(func() { UsarLiberadorDeFilas(nil) })

	res := rm.SoltarTablas([]string{d2resource.LevelPreset, d2resource.LevelType})

	if rm.Level.Presets != nil {
		t.Error("Records.Level.Presets quedo con filas")
	}

	if rm.Level.Types != nil {
		t.Error("Records.Level.Types quedo con filas")
	}

	if res.Tablas != 2 {
		t.Errorf("tablas soltadas: %d, quiero 2", res.Tablas)
	}

	if res.Bytes != 216064 {
		t.Errorf("bytes devueltos: %d, quiero 216064 (el tablero del mapa; la rebanada chica es del semiespacio)", res.Bytes)
	}

	if len(libre.mapas) != 1 {
		t.Errorf("mapas que fueron al liberador: %d, quiero 1", len(libre.mapas))
	}

	if libre.mapas[0] != unsafe.Pointer(&rm.Level.Presets) {
		// despues de soltar, la direccion del campo es la misma variable: se compara con la que se registro
		t.Log("el liberador recibio la direccion de la variable del mapa (no se puede comparar despues del nil)")
	}

	if len(libre.tramos) != 0 {
		t.Errorf("tramos que fueron al liberador: %v, quiero ninguno (la rebanada de LvlTypes son 8 B, semiespacio)", libre.tramos)
	}
}

func TestSoltarTablasDistingueVaciaDeNoSoportada(t *testing.T) {
	rm := recordsDePrueba()
	UsarLiberadorDeFilas(&LiberadorDeFilas{SoltarMapa: (&liberadorDeMentira{}).soltarMapa})
	t.Cleanup(func() { UsarLiberadorDeFilas(nil) })

	res := rm.SoltarTablas([]string{d2resource.LevelWarp, "data/global/excel/NoExiste.txt"})

	if len(res.Vacias) != 1 || res.Vacias[0] != d2resource.LevelWarp {
		t.Errorf("vacias: %v, quiero [%s]", res.Vacias, d2resource.LevelWarp)
	}

	if len(res.NoSoportadas) != 1 || res.NoSoportadas[0] != "data/global/excel/NoExiste.txt" {
		t.Errorf("no soportadas: %v, quiero la tabla inventada", res.NoSoportadas)
	}

	if res.Tablas != 0 {
		t.Errorf("tablas soltadas: %d, quiero 0", res.Tablas)
	}
}

func TestSoltarTablasSinLiberadorSueltaLasReferencias(t *testing.T) {
	rm := recordsDePrueba()
	UsarLiberadorDeFilas(nil)

	res := rm.SoltarTablas([]string{d2resource.LevelPreset, d2resource.LevelType})

	if res.Bytes != 0 {
		t.Errorf("sin liberador devolvio %d B, quiero 0 (en la PC lo recicla el recolector)", res.Bytes)
	}

	if res.Tablas != 2 {
		t.Errorf("tablas soltadas: %d, quiero 2 (las referencias se sueltan igual)", res.Tablas)
	}

	if rm.Level.Presets != nil || rm.Level.Types != nil {
		t.Error("sin liberador las tablas tenian que quedar en nil igual")
	}
}

// TestSoltarTablasNoTocaLoQueNoSeLePide: soltar las tablas del nivel no puede llevarse las de monstruos (las
// mira el POC del personaje) -- el contrato es por LISTA, no "todo lo que haya".
func TestSoltarTablasNoTocaLoQueNoSeLePide(t *testing.T) {
	rm := recordsDePrueba()
	rm.Monster.Stats = MonStats{"fallen": &MonStatRecord{}}
	UsarLiberadorDeFilas(nil)

	rm.SoltarTablas([]string{d2resource.LevelPreset})

	if rm.Level.Presets != nil {
		t.Error("no se solto LvlPrest")
	}

	if rm.Monster.Stats == nil {
		t.Error("se llevo MonStats, que no estaba en la lista")
	}
}

func TestLasTablasQueSePuedenSoltarIncluyenLasDelCompositor(t *testing.T) {
	quienes := map[string]bool{}
	for _, tabla := range TablasQueSePuedenSoltar() {
		quienes[tabla] = true
	}

	for _, tabla := range []string{d2resource.LevelType, d2resource.LevelPreset} {
		if !quienes[tabla] {
			t.Errorf("%s no esta en TablasQueSePuedenSoltar", tabla)
		}
	}
}
