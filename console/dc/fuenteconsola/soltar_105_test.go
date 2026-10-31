package fuenteconsola

// DC-002.105 -- la prueba de SOLTAR LA FUENTE despues de componer.
//
// QUE PROTEGE. Es la pieza que devuelve al monton las tablas del MPQ (355.328 B medidos al abrir los dos, con
// el `d2data.mpq` de verdad). Lo que se exige:
//   - que el censo de las tablas salga del MPQ REAL y que los bytes que el liberador devuelve sean los tramos
//     grandes, ni uno mas (los chicos son del semiespacio y liberarlos a mano corrompe el monton);
//   - que despues de soltar la fuente NO se pueda leer y que un uso tardio se vea (`errSinMPQ`), no que
//     devuelva basura;
//   - que soltar dos veces no haga nada la segunda (es idempotente, como `Tramo.Liberar`).
//
// NECESITA LA INSTALACION DEL JUEGO, como el resto de las pruebas de este paquete: sin ella se SALTA.

import (
	"errors"
	"testing"
)

func TestSoltarDevuelveLasTablasDelMPQYDejaLaFuenteMuerta(t *testing.T) {
	f := abrirDePrueba(t)

	antes := f.mpq.TamanosDeLasTablas()
	if antes.Total == 0 {
		t.Fatal("el MPQ no tiene tablas: no hay nada que medir")
	}

	var tramos []int

	libre := func(tramo uintptr, bytes int) int {
		if tramo == 0 {
			t.Fatalf("liberador: tramo nulo (bytes=%d)", bytes)
		}

		tramos = append(tramos, bytes)

		return bytes
	}

	res := f.Soltar(libre)

	if res.YaSoltada || res.Cedido {
		t.Fatalf("resultado inesperado: %+v", res)
	}

	if res.Tablas.Total != antes.Total {
		t.Errorf("el censo cambio entre las dos lecturas: %d y %d", antes.Total, res.Tablas.Total)
	}

	if res.BytesTablas != antes.Externas {
		t.Errorf("bytes de las tablas: %d, quiero %d (los que pasaron el umbral)", res.BytesTablas, antes.Externas)
	}

	suma := 0
	for _, n := range tramos {
		if n <= umbralExterno {
			t.Errorf("se libero un tramo de %d B, debajo del umbral %d: es del semiespacio", n, umbralExterno)
		}

		suma += n
	}

	if suma != res.BytesTablas {
		t.Errorf("los tramos suman %d B y el resultado dice %d", suma, res.BytesTablas)
	}

	t.Logf("d2data.mpq: tablas %d B (hash %d+%d+%d, bloque %d), en el monton de C %d B; devueltos %d B en %d tramos",
		res.Tablas.Total, res.Tablas.HashOcupadas, res.Tablas.HashEntradas, res.Tablas.HashRangos,
		res.Tablas.BloqueTabla, res.Tablas.Externas, res.Bytes, len(tramos))

	// la fuente quedo muerta: nada de lo que contesta puede ser "hay algo"
	if f.mpq != nil {
		t.Error("la fuente se quedo con el MPQ")
	}

	if f.Exists(`data\global\excel\LvlPrest.txt`) {
		t.Error("Exists contesta true con la fuente soltada")
	}

	if _, ok := f.Tamano(`data\global\excel\LvlPrest.txt`); ok {
		t.Error("Tamano contesta que si con la fuente soltada")
	}

	if _, err := f.Pedir(`data\global\excel\LvlPrest.txt`); !errors.Is(err, errSinMPQ) {
		t.Errorf("Pedir: %v, quiero errSinMPQ", err)
	}

	if _, err := f.Open(`data\global\excel\LvlPrest.txt`); !errors.Is(err, errSinMPQ) {
		t.Errorf("Open: %v, quiero errSinMPQ", err)
	}

	// idempotente
	segunda := f.Soltar(libre)
	if !segunda.YaSoltada || segunda.Bytes != 0 {
		t.Errorf("la segunda vez devolvio %+v, quiero YaSoltada y 0 B", segunda)
	}
}

// TestSoltarUnMPQCedidoNoLeTocaLasTablas: `Envolver` presta un MPQ que abrio otro (DC-002.100). Sus tablas
// las puede estar mirando el dueno, asi que soltar la fuente NO puede liberarlas: lo dice con `Cedido`.
func TestSoltarUnMPQCedidoNoLeTocaLasTablas(t *testing.T) {
	f := abrirDePrueba(t)

	prestada := Envolver(f.mpq, "prestada")

	res := prestada.Soltar(func(uintptr, int) int {
		t.Error("se le pidio al liberador que tocara las tablas de un MPQ cedido")

		return 0
	})

	if !res.Cedido {
		t.Errorf("resultado %+v: tenia que decir que el MPQ era cedido", res)
	}

	if res.BytesTablas != 0 {
		t.Errorf("bytes de las tablas de un MPQ cedido: %d, quiero 0", res.BytesTablas)
	}

	// el MPQ del dueno sigue sirviendo
	if f.mpq.Soltada() {
		t.Error("se soltaron las tablas de un MPQ cedido")
	}
}
