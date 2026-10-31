package d2mpq

// DC-002.105 -- las pruebas de SOLTAR LAS TABLAS DEL MPQ.
//
// QUE PROTEGEN, y por que asi:
//   - que se devuelvan SOLO los tramos que pasaron el umbral externo (los que el runtime mando a `malloc`):
//     liberar un tramo del semiespacio a mano corrompe el monton del recolector, asi que la prueba exige que
//     el liberador reciba EXACTAMENTE los tramos grandes, ni uno mas ni uno menos;
//   - que despues de soltar el MPQ NO se pueda leer (contesta `ErrTablasSoltadas` en vez de "file not found",
//     que seria mentir sobre la causa);
//   - que soltar dos veces no libere dos veces.
//
// La prueba NO necesita la instalacion del juego: arma un `MPQ` con las tablas del tamano que tienen las de
// verdad (las de `d2data.mpq`) y comprueba la aritmetica y las guardas. El liberador de la prueba NO libera
// memoria (en el host no hay monton externo que devolver): solo anota a quien le pidieron que liberara.

import "testing"

// liberadorDeMentira: anota los tramos y devuelve el tamano que le dicen (como el shim de la consola).
type liberadorDeMentira struct {
	tramos []int
	bytes  int
}

func (l *liberadorDeMentira) liberar(tramo uintptr, bytes int) int {
	if tramo == 0 {
		panic("liberador: tramo nulo")
	}

	l.tramos = append(l.tramos, bytes)
	l.bytes += bytes

	return bytes
}

// mpqDePrueba: las tablas con los tamanos medidos de `d2data.mpq` (65.536 ranuras de hash, 11.083 bloques).
func mpqDePrueba() *MPQ {
	return &MPQ{
		hashRanuras:  65536,
		hashOcupadas: make([]byte, 65536/8),
		hashEntradas: make([]byte, 10802*16), // las ranuras ocupadas de d2data, en 16 B cada una
		hashRangos:   make([]uint16, 256),
		blockTable:   make([]byte, 11083*16),
	}
}

func TestTamanosDeLasTablasDiceQueEsLoQueEstaEnElMontonDeC(t *testing.T) {
	mpq := mpqDePrueba()
	tn := mpq.TamanosDeLasTablas()

	quiénes := []struct {
		nombre string
		valor  int
		quiero int
	}{
		{"hash ocupadas", tn.HashOcupadas, 65536 / 8},
		{"hash entradas", tn.HashEntradas, 10802 * 16},
		{"hash rangos", tn.HashRangos, 256 * 2},
		{"tabla de bloque", tn.BloqueTabla, 11083 * 16},
	}

	total := 0

	for _, q := range quiénes {
		if q.valor != q.quiero {
			t.Errorf("%s: censo %d, quiero %d", q.nombre, q.valor, q.quiero)
		}

		total += q.quiero
	}

	if tn.Total != total {
		t.Errorf("total del censo: %d, quiero %d", tn.Total, total)
	}

	// solo las dos grandes pasan el umbral de 64 KB
	quieroExternas := 10802*16 + 11083*16
	if tn.Externas != quieroExternas {
		t.Errorf("bytes en el monton de C: %d, quiero %d", tn.Externas, quieroExternas)
	}

	if tn.Bloques != 11083 {
		t.Errorf("entradas de la tabla de bloque: %d, quiero 11083", tn.Bloques)
	}
}

func TestLiberarTablasDevuelveSoloLosTramosGrandes(t *testing.T) {
	mpq := mpqDePrueba()
	libre := &liberadorDeMentira{}

	bytes := mpq.LiberarTablas(libre.liberar)

	quiero := 10802*16 + 11083*16
	if bytes != quiero {
		t.Errorf("bytes devueltos: %d, quiero %d", bytes, quiero)
	}

	if len(libre.tramos) != 2 {
		t.Fatalf("tramos devueltos: %d %v, quiero 2 (los dos grandes)", len(libre.tramos), libre.tramos)
	}

	for _, n := range libre.tramos {
		if n <= umbralExterno {
			t.Errorf("se libero un tramo de %d B: esta debajo del umbral (%d), es del semiespacio", n, umbralExterno)
		}
	}

	if !mpq.Soltada() {
		t.Error("el MPQ dice que no esta soltado")
	}

	if mpq.hashOcupadas != nil || mpq.hashEntradas != nil || mpq.hashRangos != nil || mpq.blockTable != nil {
		t.Error("quedaron tablas vivas despues de soltarlas")
	}
}

func TestDespuesDeSoltarElMPQNoSePuedeLeer(t *testing.T) {
	mpq := mpqDePrueba()
	mpq.LiberarTablas((&liberadorDeMentira{}).liberar)

	if mpq.Contains("data\\global\\excel\\LvlPrest.txt") {
		t.Error("Contains contesta true con las tablas soltadas")
	}

	if _, ok := mpq.TamanoDeArchivo("data\\global\\excel\\LvlPrest.txt"); ok {
		t.Error("TamanoDeArchivo contesta que si con las tablas soltadas")
	}

	if _, err := mpq.ReadFile("data\\global\\excel\\LvlPrest.txt"); err != ErrTablasSoltadas {
		t.Errorf("ReadFile: %v, quiero ErrTablasSoltadas", err)
	}

	if _, err := mpq.getFileBlockData("cualquiera"); err != ErrTablasSoltadas {
		t.Errorf("getFileBlockData: %v, quiero ErrTablasSoltadas", err)
	}
}

func TestSoltarDosVecesNoLiberaDosVeces(t *testing.T) {
	mpq := mpqDePrueba()
	libre := &liberadorDeMentira{}

	mpq.LiberarTablas(libre.liberar)
	segunda := mpq.LiberarTablas(libre.liberar)

	if segunda != 0 {
		t.Errorf("la segunda vez devolvio %d B, quiero 0", segunda)
	}

	if len(libre.tramos) != 2 {
		t.Errorf("el liberador fue llamado %d veces, quiero 2 (la segunda pasada no libera nada)", len(libre.tramos))
	}
}

func TestSinLiberadorSeSueltanLasTablasYNoSeLiberaMemoria(t *testing.T) {
	mpq := mpqDePrueba()

	if bytes := mpq.LiberarTablas(nil); bytes != 0 {
		t.Errorf("sin liberador devolvio %d B, quiero 0 (en la PC no se libera a mano)", bytes)
	}

	if !mpq.Soltada() || mpq.blockTable != nil {
		t.Error("sin liberador las tablas tenian que quedar sueltas igual")
	}
}

func TestLiberarElPrestamoDevuelveSoloLoGrande(t *testing.T) {
	s := &Scratch{
		posiciones:    make([]uint32, 33),   // 132 B: semiespacio
		crudo:         make([]byte, 132),    // semiespacio
		comprimido:    make([]byte, 8192),   // semiespacio
		descomprimido: make([]byte, 120000), // 117 KB: monton de C
		reservado:     120000 + 8192 + 132 + 132,
	}

	libre := &liberadorDeMentira{}

	bytes := s.Liberar(libre.liberar)
	if bytes != 120000 {
		t.Errorf("bytes del prestamo: %d, quiero 120000", bytes)
	}

	if len(libre.tramos) != 1 {
		t.Errorf("tramos del prestamo: %v, quiero uno solo (el descompreso)", libre.tramos)
	}

	if s.posiciones != nil || s.crudo != nil || s.comprimido != nil || s.descomprimido != nil {
		t.Error("el prestamo quedo con tramos despues de liberarlo")
	}
}
