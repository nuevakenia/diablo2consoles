package d2asset

// DC-002.99 -- las pruebas del ULTIMO `make` POR ARCHIVO: la lectura del motor por el tramo prestado.
//
// QUE PROTEGEN. `LoadFile` es el punto de entrada de TODAS las cargas grandes del motor, y esta tarjeta le
// cambia el contrato: lo que devuelve vale hasta la proxima lectura. Lo que hay que exigirle, entonces, no es
// "anda" sino cuatro cosas que se pueden romper en silencio:
//
//  1. EL TRAMO ES UNO. Diez lecturas de archivos que entran en el mismo tramo tienen que pedir UN tramo, no
//     diez (`Pedidos == 1`, `Reusos == 9`). Es el numero de la tarjeta: deja de haber un buffer por archivo.
//  2. DEVOLVER ES DEVOLVER. `SoltarLecturasDeConsola` tiene que dejar el tramo devuelto (`Pedidos ==
//     Liberados`, `BytesDelTramo == 0`), y un archivo MAS GRANDE tiene que devolver el tramo viejo antes de
//     pedir el nuevo (si no, el reservorio crece y no vuelve nunca).
//  3. EL CONTRATO ES EL QUE SE DICE. Con el camino prendido, el `[]byte` de una lectura se PISA con la
//     siguiente: la prueba lo fija para que nadie se entere por un bug en produccion.
//  4. EL HANDOFF NO SE ROMPE. Los dos archivos que un segundo consumidor vuelve a leer (el `.ds1` y el `.dt1`)
//     se GUARDAN en la tabla del handoff, asi que NO pueden pasar por el tramo: se leen con buffer propio y el
//     segundo consumidor tiene que encontrar el archivo, no memoria de otro.
//
// Y la PC: con las palancas apagadas no se pide ni un tramo y cada lectura trae SU buffer (el camino de
// siempre, palabra por palabra).

import (
	"bytes"
	"io"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset"
)

// tramoDePrueba: un tramo de memoria de verdad (un `[]byte`) que cuenta si lo devolvieron.
type tramoDePrueba struct {
	datos    []byte
	fuente   *fuenteDePrueba
	liberado bool
}

func (t *tramoDePrueba) Bytes() []byte {
	if t.liberado {
		return nil
	}

	return t.datos
}

func (t *tramoDePrueba) Liberar() {
	if t.liberado {
		return
	}

	t.liberado = true
	t.fuente.liberados++
}

// fuenteDePrueba: las cuatro interfaces de `asset` en un solo objeto, sin MPQ. Presta memoria de verdad y
// cuenta lo que el motor le pide, que es lo que miden las pruebas.
type fuenteDePrueba struct {
	archivos map[string][]byte

	prestados int
	liberados int
	lecturas  int
	conPresta int
}

var (
	_ asset.Source                     = (*fuenteDePrueba)(nil)
	_ asset.SourceDeLecturaEntera      = (*fuenteDePrueba)(nil)
	_ asset.SourceDeLecturaConPrestamo = (*fuenteDePrueba)(nil)
	_ asset.SourceDeTramos             = (*fuenteDePrueba)(nil)
)

func (f *fuenteDePrueba) Path() string   { return "prueba" }
func (f *fuenteDePrueba) String() string { return "prueba" }

func (f *fuenteDePrueba) Exists(nombre string) bool {
	_, hay := f.archivos[nombre]

	return hay
}

func (f *fuenteDePrueba) Open(nombre string) (io.ReadSeeker, error) {
	datos, hay := f.archivos[nombre]
	if !hay {
		return nil, io.EOF
	}

	return bytes.NewReader(datos), nil
}

func (f *fuenteDePrueba) Tamano(nombre string) (int, bool) {
	datos, hay := f.archivos[nombre]
	if !hay {
		return 0, false
	}

	return len(datos), true
}

func (f *fuenteDePrueba) LeerEnteroEn(nombre string, dst []byte) (int, error) {
	datos, hay := f.archivos[nombre]
	if !hay {
		return 0, io.EOF
	}

	f.lecturas++

	return copy(dst, datos), nil
}

func (f *fuenteDePrueba) LeerEnteroEnConPrestamo(nombre string, dst []byte, _ *d2mpq.Scratch) (int, error) {
	f.conPresta++

	return f.LeerEnteroEn(nombre, dst)
}

func (f *fuenteDePrueba) Prestar(nombre string, minimo int) (asset.TramoPrestado, error) {
	if minimo < 1 {
		minimo = 1
	}

	f.prestados++

	return &tramoDePrueba{datos: make([]byte, minimo), fuente: f}, nil
}

// gerenteDePrueba: el AssetManager minimo que necesita `LoadFile` (el loader con la fuente de prueba).
func gerenteDePrueba(f *fuenteDePrueba) *AssetManager {
	l, err := d2loader.NewLoader(0)
	if err != nil {
		panic("el loader no se pudo armar: " + err.Error())
	}

	l.Sources = []asset.Source{f}

	return &AssetManager{Loader: l}
}

// palancas: deja el paquete como estaba. Las palancas son variables de paquete (es el estilo del camino de
// consola, ver presupuestos.go), asi que una prueba que las prenda tiene que apagarlas al salir o la siguiente
// prueba mide otra cosa.
func palancas(t *testing.T, lectura bool, tramos bool) {
	t.Helper()

	antesLectura, antesTramos := caminoDeLecturaDeConsola, caminoDeTramosDeConsola

	caminoDeLecturaDeConsola, caminoDeTramosDeConsola = lectura, tramos

	t.Cleanup(func() {
		soltarTramoDeLecturas()

		caminoDeLecturaDeConsola, caminoDeTramosDeConsola = antesLectura, antesTramos
	})
}

// TestElTramoEsUnoYSeReusa: diez archivos del mismo tamano piden UN tramo. Es el numero de la tarjeta.
func TestElTramoEsUnoYSeReusa(t *testing.T) {
	palancas(t, true, true)
	soltarTramoDeLecturas()

	// el contador del instrumento arranca donde lo dejo otra prueba: se mide el delta
	antes := EstadoDeLosTramos()

	f := &fuenteDePrueba{archivos: map[string][]byte{
		"data/global/a.ds1": bytes.Repeat([]byte{1}, 100),
		"data/global/b.ds1": bytes.Repeat([]byte{2}, 90),
	}}

	am := gerenteDePrueba(f)

	for i := 0; i < 10; i++ {
		datos, err := am.LoadFile("data/global/a.ds1")
		if err != nil {
			t.Fatalf("LoadFile: %v", err)
		}

		if len(datos) != 100 || datos[0] != 1 {
			t.Fatalf("LoadFile devolvio %d bytes que empiezan con %d", len(datos), datos[0])
		}
	}

	// y un archivo mas chico entra en el mismo tramo
	if datos, err := am.LoadFile("data/global/b.ds1"); err != nil || len(datos) != 90 || datos[0] != 2 {
		t.Fatalf("el archivo chico no se leyo en el tramo: %v", err)
	}

	despues := EstadoDeLosTramos()

	if pedidos := despues.Pedidos - antes.Pedidos; pedidos != 1 {
		t.Fatalf("se pidieron %d tramos para 11 lecturas, quiero 1", pedidos)
	}

	if reusos := despues.Reusos - antes.Reusos; reusos != 10 {
		t.Fatalf("reusos=%d, quiero 10 (todas menos la que pidio el tramo)", reusos)
	}

	if f.lecturas != 11 {
		t.Fatalf("la fuente hizo %d lecturas, quiero 11", f.lecturas)
	}

	// la lectura fue con el prestamo del MPQ (no pide memoria de trabajo)
	if f.conPresta != 11 {
		t.Fatalf("lecturas con prestamo=%d, quiero 11", f.conPresta)
	}
}

// TestSoltarDevuelveElTramoYElCrecimientoLoDevuelveAntes: devolver es devolver.
func TestSoltarDevuelveElTramoYElCrecimientoLoDevuelveAntes(t *testing.T) {
	palancas(t, true, true)
	soltarTramoDeLecturas()

	antes := EstadoDeLosTramos()

	f := &fuenteDePrueba{archivos: map[string][]byte{
		"data/global/chico.ds1":  bytes.Repeat([]byte{1}, 100),
		"data/global/grande.ds1": bytes.Repeat([]byte{2}, 400),
	}}

	am := gerenteDePrueba(f)

	if _, err := am.LoadFile("data/global/chico.ds1"); err != nil {
		t.Fatalf("LoadFile chico: %v", err)
	}

	// el archivo grande NO entra en el tramo de 100: el viejo se devuelve y se pide uno de 400
	datos, err := am.LoadFile("data/global/grande.ds1")
	if err != nil {
		t.Fatalf("LoadFile grande: %v", err)
	}

	if len(datos) != 400 || datos[0] != 2 {
		t.Fatalf("el grande no se leyo: %d bytes que empiezan con %d", len(datos), datos[0])
	}

	e := EstadoDeLosTramos()
	if pedidos, liberados := e.Pedidos-antes.Pedidos, e.Liberados-antes.Liberados; pedidos != 2 || liberados != 1 {
		t.Fatalf("pedidos=%d liberados=%d, quiero 2 y 1 (el crecimiento devuelve el tramo viejo)", pedidos, liberados)
	}

	if e.BytesDelTramo != 400 {
		t.Fatalf("el tramo vigente mide %d, quiero 400", e.BytesDelTramo)
	}

	if f.liberados != 1 {
		t.Fatalf("la fuente conto %d liberaciones, quiero 1", f.liberados)
	}

	// soltar: el tramo se devuelve y no queda nada pedido
	SoltarLecturasDeConsola()

	e = EstadoDeLosTramos()

	if e.BytesDelTramo != 0 {
		t.Fatalf("despues de soltar el tramo sigue midiendo %d", e.BytesDelTramo)
	}

	if e.Pedidos != e.Liberados {
		t.Fatalf("pedidos=%d liberados=%d: quedo un tramo vivo", e.Pedidos, e.Liberados)
	}

	if f.liberados != 2 {
		t.Fatalf("la fuente conto %d liberaciones, quiero 2", f.liberados)
	}
}

// TestElContratoEsElQueSeDice: con el tramo prendido, el `[]byte` de una lectura se pisa con la siguiente. La
// prueba lo fija a proposito: es el precio de no pedir memoria, y esta escrito en la cabecera del archivo.
func TestElContratoEsElQueSeDice(t *testing.T) {
	palancas(t, true, true)
	soltarTramoDeLecturas()

	f := &fuenteDePrueba{archivos: map[string][]byte{
		"data/global/uno.ds1": bytes.Repeat([]byte{1}, 100),
		"data/global/dos.ds1": bytes.Repeat([]byte{2}, 100),
	}}

	am := gerenteDePrueba(f)

	primero, err := am.LoadFile("data/global/uno.ds1")
	if err != nil {
		t.Fatalf("LoadFile uno: %v", err)
	}

	guardado := primero[0]

	segundo, err := am.LoadFile("data/global/dos.ds1")
	if err != nil {
		t.Fatalf("LoadFile dos: %v", err)
	}

	if segundo[0] != 2 {
		t.Fatalf("la segunda lectura no trae el segundo archivo: %d", segundo[0])
	}

	if guardado != 1 {
		t.Fatalf("la primera lectura no traia el primer archivo: %d", guardado)
	}

	if primero[0] != 2 {
		t.Fatalf("el tramo no se reuso: la primera vista trae %d y deveria traer el archivo nuevo (2)", primero[0])
	}
}

// TestElHandoffNoPasaPorElTramo: el archivo que se guarda para un segundo consumidor tiene que seguir siendo
// el mismo despues de que el motor lea otro.
func TestElHandoffNoPasaPorElTramo(t *testing.T) {
	palancas(t, true, true)
	soltarTramoDeLecturas()

	f := &fuenteDePrueba{archivos: map[string][]byte{
		"data/global/tiles/town.ds1": bytes.Repeat([]byte{7}, 120),
		"data/global/otro.ds1":       bytes.Repeat([]byte{9}, 120),
	}}

	am := gerenteDePrueba(f)

	// el primer consumidor lee el .ds1: el handoff se queda con los bytes
	if _, err := am.LoadFileHandoff("data/global/tiles/town.ds1"); err != nil {
		t.Fatalf("LoadFileHandoff: %v", err)
	}

	// el motor sigue cargando otras cosas (esto es lo que pisa el tramo)
	if _, err := am.LoadFile("data/global/otro.ds1"); err != nil {
		t.Fatalf("LoadFile otro: %v", err)
	}

	// el segundo consumidor tiene que encontrar SU archivo, no memoria de otro
	segundo, err := am.LoadFileHandoff("data/global/tiles/town.ds1")
	if err != nil {
		t.Fatalf("LoadFileHandoff (segundo): %v", err)
	}

	for i, b := range segundo {
		if b != 7 {
			t.Fatalf("el handoff devolvio memoria de otro archivo en el byte %d: %d", i, b)
		}
	}

	// y no se pidio ni un tramo para el handoff
	if f.prestados != 1 {
		t.Fatalf("la fuente presto %d tramos, quiero 1 (solo la lectura que no es del handoff)", f.prestados)
	}
}

// TestEnLaPCNoSePideNiUnTramo: con las palancas apagadas el camino es el de siempre, palabra por palabra: ni
// un tramo, y cada lectura con SU buffer.
func TestEnLaPCNoSePideNiUnTramo(t *testing.T) {
	palancas(t, false, false)

	f := &fuenteDePrueba{archivos: map[string][]byte{
		"data/global/a.ds1": bytes.Repeat([]byte{1}, 100),
		"data/global/b.ds1": bytes.Repeat([]byte{2}, 100),
	}}

	am := gerenteDePrueba(f)

	primero, err := am.LoadFile("data/global/a.ds1")
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	if _, err := am.LoadFile("data/global/b.ds1"); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	if f.prestados != 0 {
		t.Fatalf("en la PC se pidieron %d tramos, quiero 0", f.prestados)
	}

	// el buffer de la primera lectura NO se pisa (cada uno trae el suyo)
	if primero[0] != 1 {
		t.Fatalf("la primera lectura se piso: trae %d", primero[0])
	}
}
