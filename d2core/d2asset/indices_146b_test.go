package d2asset

// DC-002.146b -- LA TRADUCCION DEL INDICE DEL TILE (la que faltaba).
//
// `cargarDT1EnBloquesUsados` tira los tiles cuya clave el nivel no usa y CORRE a los que quedan; el
// `ExactTile.Index` que guarda el plan del DRLG es el del `.dt1` ORIGINAL. Este test fija el contrato de
// `IndiceDeTileCargado` en los tres casos que importan: sin compactar (la PC) el indice vale igual, con
// compactado vale el que le toco, y un tile que se tiro no vale nada (el registro se descarta).

import "testing"

func TestIndiceDeTileCargado(t *testing.T) {
	// sin compactar: la PC, o el arena de crudos. Cualquier indice vale igual.
	indicesDelArchivo = nil

	for _, idx := range []int{0, 7, 137} {
		if got, ok := IndiceDeTileCargado("/data/global/tiles/act1/outdoors/swamp.dt1", idx); !ok || got != idx {
			t.Fatalf("sin compactar, indice %d: got (%d, %v), queria (%d, true)", idx, got, ok, idx)
		}
	}

	// compactado: el `.dt1` tenia 6 tiles y sobrevivieron el 0 y el 3 (en ese orden).
	indicesDelArchivo = map[string][]int{
		"/data/global/tiles/act1/outdoors/swamp.dt1": {0, -1, -1, 1, -1, -1},
	}

	casos := []struct {
		idx    int
		quiero int
		hay    bool
		porQue string
	}{
		{0, 0, true, "el primero se queda donde estaba"},
		{3, 1, true, "el que estaba cuarto pasa a ser el segundo"},
		{1, 0, false, "un tile tirado no vale nada: el registro se descarta"},
		{5, 0, false, "un tile tirado no vale nada"},
		{6, 0, false, "un indice que el .dt1 no tenia tampoco"},
		{-1, 0, false, "un indice negativo no vale nada"},
	}

	for _, c := range casos {
		got, ok := IndiceDeTileCargado("/data/global/tiles/act1/outdoors/swamp.dt1", c.idx)

		if ok != c.hay || (c.hay && got != c.quiero) {
			t.Fatalf("indice %d (%s): got (%d, %v), queria (%d, %v)", c.idx, c.porQue, got, ok, c.quiero, c.hay)
		}
	}

	// la ruta se normaliza igual que la pide el motor: mayusculas y barras invertidas dan lo mismo
	if got, ok := IndiceDeTileCargado(`ACT1\OUTDOORS\Swamp.dt1`, 3); !ok || got != 1 {
		t.Fatalf("ruta sin normalizar: got (%d, %v), queria (1, true)", got, ok)
	}

	// otro archivo (que no paso por el camino que compacta) usa el indice tal cual
	if got, ok := IndiceDeTileCargado("/data/global/tiles/act1/town/floor.dt1", 3); !ok || got != 3 {
		t.Fatalf("otro archivo: got (%d, %v), queria (3, true)", got, ok)
	}

	indicesDelArchivo = nil
}
