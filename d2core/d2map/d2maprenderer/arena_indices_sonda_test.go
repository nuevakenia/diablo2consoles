package d2maprenderer

// DC-002.106 -- la prueba en contra del tramo EN PEDAZOS: que reparta igual que el de un solo golpe, que use
// la cola de los bloques ya abiertos (best fit), que respete el techo, y que cuando la sonda del monton dice
// que no entra un bloque del tamano pedido, los siguientes salgan DEL TAMANO QUE EL MONTON SIRVE.

import (
	"testing"
)

// TestElTramoEnPedazosPreguntaAlMonton: con sonda, el tamano de cada bloque lo dice la sonda (el monton) y no
// `pedazo`. Es la diferencia entre cortarse en el ultimo bloque y seguir con bloques mas chicos.
func TestElTramoEnPedazosPreguntaAlMonton(t *testing.T) {
	// la sonda: el monton solo sirve hasta 100 KB, aunque se le pidan 300 KB
	sirve := func(tope int) int {
		if tope > 100*1024 {
			return 100 * 1024
		}

		return tope
	}

	mr := &MapRenderer{}
	mr.ActivarTramoDeLaCacheEnPedazos(1024*1024, 300*1024, sirve)

	for i := 0; i < 6; i++ {
		if p := mr.pedirIndicesDelTile(60 * 1024); len(p) != 60*1024 {
			t.Fatalf("imagen %d: len=%d", i, len(p))
		}
	}

	e := mr.CensoDelTramo()

	if e.PedazoMayor != 100*1024 || e.PedazoMenor != 100*1024 {
		t.Fatalf("los bloques tienen que salir del tamano que sirve la sonda (100 KB): %+v", e)
	}

	if e.Pedazos != 6 || e.Usado != 360*1024 || e.Derrames != 0 {
		t.Fatalf("censo del tramo: %+v (6 bloques, 360 KB usados, sin derrames)", e)
	}
}

// TestElTramoEnPedazosSeDetieneSiLaSondaNoSirve: si la sonda no puede servir ni el bloque mas chico util (por
// debajo del corte de 64 KB del objeto grande), la imagen se derrama y se cuenta -- NO se pide un bloque que
// iria al semiespacio del recolector.
func TestElTramoEnPedazosSeDetieneSiLaSondaNoSirve(t *testing.T) {
	nada := func(int) int { return 0 }

	mr := &MapRenderer{}
	mr.ActivarTramoDeLaCacheEnPedazos(1024*1024, 300*1024, nada)

	p := mr.pedirIndicesDelTile(50 * 1024)

	if len(p) != 50*1024 || cap(p) != 50*1024 {
		t.Fatalf("la imagen tiene que salir igual (por make): len=%d cap=%d", len(p), cap(p))
	}

	e := mr.CensoDelTramo()

	if e.Pedazos != 0 || e.Derrames != 1 || e.BytesDerramados != 50*1024 {
		t.Fatalf("censo del tramo: %+v (cero bloques, un derrame de 50 KB)", e)
	}
}
