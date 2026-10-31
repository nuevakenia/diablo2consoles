package d2asset

// DC-002.105 -- las pruebas del pase por el AssetManager de SOLTAR LAS FILAS DE LAS TABLAS.
//
// QUE PROTEGEN. La funcion es la puerta por la que el POC de la consola suelta las filas del compositor: lo que
// se exige es que llegue al `RecordManager` sin tener que exponerlo, que aguante un `AssetManager` sin
// `Records` (la PC, un arranque a medias) y que el liberador del arranque se pueda instalar y consultar.

import (
	"testing"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

func TestSoltarFilasDeLasTablasLlegaAlRecordManager(t *testing.T) {
	rm := &d2records.RecordManager{}
	rm.Level.Presets = d2records.LevelPresets{1: {Name: "Act 1 - Town", DefinitionID: 1}}

	am := &AssetManager{Records: rm}

	// liberador de mentira: no libera (en el host no hay monton externo), solo contesta el tamano
	UsarLiberadorDeFilas(&d2records.LiberadorDeFilas{
		SoltarMapa: func(unsafe.Pointer) int { return 216064 },
	})
	t.Cleanup(func() { UsarLiberadorDeFilas(nil) })

	if !HayLiberadorDeFilas() {
		t.Error("el liberador de filas no quedo instalado")
	}

	res := am.SoltarFilasDeLasTablas([]string{d2resource.LevelPreset})

	if rm.Level.Presets != nil {
		t.Error("Records.Level.Presets quedo con filas")
	}

	if res.Tablas != 1 || res.Bytes != 216064 {
		t.Errorf("resultado: %d tablas, %d B; quiero 1 y 216064", res.Tablas, res.Bytes)
	}
}

func TestSoltarFilasDeLasTablasAguantaUnManagerSinRecords(t *testing.T) {
	am := &AssetManager{}

	res := am.SoltarFilasDeLasTablas([]string{d2resource.LevelPreset})
	if res.Tablas != 0 || res.Bytes != 0 {
		t.Errorf("sin Records devolvio %+v, quiero el cero", res)
	}

	var sinNada *AssetManager

	res = sinNada.SoltarFilasDeLasTablas([]string{d2resource.LevelPreset})
	if res.Tablas != 0 {
		t.Errorf("con el AssetManager en nil devolvio %+v, quiero el cero", res)
	}
}

func TestTablasQueSePuedenSoltarNoEstaVacia(t *testing.T) {
	if len(TablasQueSePuedenSoltar()) == 0 {
		t.Fatal("TablasQueSePuedenSoltar volvio vacia")
	}
}
