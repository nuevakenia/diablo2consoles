package fuenteconsola

// DC-002.96 -- las pruebas de la SOURCE DE CONSOLA.
//
// QUE PROTEGEN. Es la pieza por la que el camino de consola lee TODO asset grande, asi que lo que hay que
// exigirle no es "anda": es que no se pueda romper en silencio ninguna de las cuatro cosas que la tarjeta
// pone como criterio.
//
//  1. LAS TRES INTERFACES EN UNA SOLA IMPLEMENTACION. Las aserciones de compilacion del paquete cubren eso;
//     aca ademas se mide en vivo que los TRES caminos (dst del que llama, memoria prestada y tramo con
//     dueno) devuelven EL MISMO CONTENIDO que el camino de siempre, para los seis formatos.
//  2. EL DUENO DEL BUFFER ES EXPLICITO. `Pedir` entrega un tramo y `Liberar` lo devuelve: despues de
//     liberar, `BytesVivos` vuelve a CERO (el monton vuelve al nivel de partida), `Datos` queda en nil y
//     `Pedidos == Liberados`.
//  3. SE LIBERA SOLO LO QUE SE PUEDE LIBERAR. Debajo del umbral externo (64 KB) el bloque vive en el
//     semiespacio del recolector y liberarlo a mano corrompe el monton: el liberador tiene que recibir
//     EXACTAMENTE los tramos que pasaron el umbral, ni uno mas ni uno menos.
//  4. LA LECTURA NO PIDE MEMORIA DE TRABAJO POR ARCHIVO. El prestamo se pide una vez: despues de la primera
//     lectura `Crecidas` no crece y `Reservado` no cambia.
//
// NECESITA LA INSTALACION DEL JUEGO (no hay MPQ de mentira: el contenido que se compara es el real). Sin
// instalacion las pruebas se SALTAN en vez de dar verde por la nada.

import (
	"bytes"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
)

// los seis formatos que el camino de consola tiene que cargar por la fuente nueva. Los mismos nombres que
// `console/dc/poc/fuente-consola` (y que el medidor del host): el .dt1 es el de ~1 MB del pueblo, y el .cof
// es el caso chico que NO hay que liberar a mano.
var losSeis = map[string]string{
	".dt1":     `data\global\tiles\ACT1\TOWN\floor.dt1`,
	".ds1":     `data\global\tiles\ACT1\TOWN\townN1.ds1`,
	".dcc":     `data\global\objects\AA\S2\aas2litnuhth.dcc`,
	".cof":     `data\global\objects\E2\COF\e2S1HTH.cof`,
	".dc6":     `data\global\ui\FrontEnd\ts11b.DC6`,
	"animdata": `data\global\AnimData.D2`,
}

// TestLaFuenteCumpleLasTresInterfacesYDaElMismoContenido: los tres caminos de la fuente nueva tienen que
// devolver el mismo archivo que `mpq.ReadFile`, para los seis formatos.
func TestLaFuenteCumpleLasTresInterfacesYDaElMismoContenido(t *testing.T) {
	f := abrirDePrueba(t)

	for formato, nombre := range losSeis {
		quiero, err := f.mpq.ReadFile(limpiar(nombre))
		if err != nil {
			t.Fatalf("%s %s: el camino de siempre fallo: %v", formato, nombre, err)
		}

		tamano, sabido := f.Tamano(nombre)
		if !sabido {
			t.Fatalf("%s %s: la fuente no sabe el tamano", formato, nombre)
		}

		if tamano != len(quiero) {
			t.Fatalf("%s %s: Tamano dice %d y el archivo mide %d", formato, nombre, tamano, len(quiero))
		}

		// camino 1: el destino lo pone el que llama
		dst := make([]byte, tamano)

		leidos, err := f.LeerEnteroEn(nombre, dst)
		if err != nil {
			t.Fatalf("%s %s: LeerEnteroEn: %v", formato, nombre, err)
		}

		if !bytes.Equal(dst[:leidos], quiero) {
			t.Fatalf("%s %s: LeerEnteroEn NO devuelve el archivo", formato, nombre)
		}

		// camino 2: con la memoria de trabajo prestada
		dst2 := make([]byte, tamano)

		leidos2, err := f.LeerEnteroEnConPrestamo(nombre, dst2, nil)
		if err != nil {
			t.Fatalf("%s %s: LeerEnteroEnConPrestamo: %v", formato, nombre, err)
		}

		if !bytes.Equal(dst2[:leidos2], quiero) {
			t.Fatalf("%s %s: LeerEnteroEnConPrestamo NO devuelve el archivo", formato, nombre)
		}

		// camino 3: el tramo con dueno
		tramo, err := f.Pedir(nombre)
		if err != nil {
			t.Fatalf("%s %s: Pedir: %v", formato, nombre, err)
		}

		if !bytes.Equal(tramo.Datos, quiero) {
			t.Fatalf("%s %s: el tramo NO devuelve el archivo", formato, nombre)
		}

		if crc32.ChecksumIEEE(tramo.Datos) != crc32.ChecksumIEEE(quiero) {
			t.Fatalf("%s %s: el crc32 del tramo NO es el del archivo", formato, nombre)
		}

		tramo.Liberar()
	}

	e := f.Estado()

	if e.Pedidos != len(losSeis) || e.Liberados != len(losSeis) {
		t.Fatalf("pedidos=%d liberados=%d de %d archivos", e.Pedidos, e.Liberados, len(losSeis))
	}

	if e.BytesVivos != 0 {
		t.Fatalf("quedaron %d bytes vivos despues de liberar todo", e.BytesVivos)
	}

	if e.SinTamano != 0 || e.PedidosFallidos != 0 {
		t.Fatalf("sinTamano=%d pedidosFallidos=%d", e.SinTamano, e.PedidosFallidos)
	}
}

// TestLiberarDevuelveTodoYRespetaElUmbralExterno: el liberador recibe EXACTAMENTE los tramos que pasaron el
// umbral de 64 KB (los que el runtime mando al monton externo), ni uno mas. Y despues de liberar, el que
// llamo ya no tiene el buffer.
func TestLiberarDevuelveTodoYRespetaElUmbralExterno(t *testing.T) {
	f := abrirDePrueba(t)

	var (
		liberados []uintptr
		vivos     []*Tramo
	)

	f.UsarLiberador(func(direccion uintptr) { liberados = append(liberados, direccion) })

	grandes := 0

	for formato, nombre := range losSeis {
		tramo, err := f.Pedir(nombre)
		if err != nil {
			t.Fatalf("%s %s: Pedir: %v", formato, nombre, err)
		}

		if len(tramo.Datos) > umbralExterno {
			grandes++
		}

		vivos = append(vivos, tramo)
	}

	// nada liberado todavia: los seis tramos estan vivos y el instrumento lo dice
	if e := f.Estado(); e.Liberados != 0 || int(e.BytesVivos) <= 0 {
		t.Fatalf("antes de liberar: liberados=%d bytesVivos=%d", e.Liberados, e.BytesVivos)
	}

	for _, tramo := range vivos {
		tramo.Liberar()
	}

	if len(liberados) != grandes {
		t.Fatalf("el liberador se llamo %d veces y los tramos que pasaron el umbral son %d", len(liberados), grandes)
	}

	e := f.Estado()

	if e.Liberados != len(losSeis) {
		t.Fatalf("liberados=%d de %d tramos", e.Liberados, len(losSeis))
	}

	if e.BytesVivos != 0 {
		t.Fatalf("quedaron %d bytes vivos: el dueno no devolvio todo", e.BytesVivos)
	}

	if e.LibresExternos != grandes {
		t.Fatalf("libresExternos=%d y habia %d tramos grandes", e.LibresExternos, grandes)
	}

	for i, tramo := range vivos {
		if tramo.Datos != nil {
			t.Fatalf("el tramo %d conserva su Datos despues de liberar", i)
		}

		if !tramo.Liberado() {
			t.Fatalf("el tramo %d no quedo marcado como liberado", i)
		}

		// liberar de nuevo NO libera de nuevo: se cuenta y se ignora
		tramo.Liberar()
	}

	if e2 := f.Estado(); e2.LiberadosDobles != len(vivos) || len(liberados) != grandes {
		t.Fatalf("la doble liberacion no se contuvo: dobles=%d liberadorLlamado=%d", e2.LiberadosDobles, len(liberados))
	}
}

// TestElPrestamoNoCreceEntreArchivos: el mismo archivo diez veces sobre el mismo prestamo. Despues de la
// primera lectura no se pide memoria de trabajo ni una vez mas.
func TestElPrestamoNoCreceEntreArchivos(t *testing.T) {
	f := abrirDePrueba(t)

	var (
		reservado int
		crecidas  int
		crc       = uint32(0)
	)

	for vuelta := 0; vuelta < 10; vuelta++ {
		tramo, err := f.Pedir(`data\global\tiles\ACT1\TOWN\floor.dt1`)
		if err != nil {
			t.Fatalf("vuelta %d: Pedir: %v", vuelta, err)
		}

		if vuelta == 0 {
			crc = crc32.ChecksumIEEE(tramo.Datos)

			e := f.Estado()
			reservado, crecidas = e.PrestamoReservado, e.PrestamoCrecidas
		}

		if crc32.ChecksumIEEE(tramo.Datos) != crc {
			t.Fatalf("vuelta %d: el contenido cambio entre vueltas", vuelta)
		}

		tramo.Liberar()
	}

	e := f.Estado()

	if e.PrestamoCrecidas != crecidas {
		t.Fatalf("el prestamo crecio de %d a %d reservas: se esta pidiendo memoria por archivo", crecidas, e.PrestamoCrecidas)
	}

	if e.PrestamoReservado != reservado {
		t.Fatalf("el prestamo reservado cambio de %d a %d bytes", reservado, e.PrestamoReservado)
	}
}

// TestRegistrarEnPoneLaFuentePrimeroYAtiendeLasCargas: puesta en la lista de fuentes del loader del motor,
// la Source de consola atiende las cargas (el loader prueba las fuentes de izquierda a derecha). Es lo que
// hace que las cargas del camino de consola pasen por aca sin cambiar una linea del motor.
func TestRegistrarEnPoneLaFuentePrimeroYAtiendeLasCargas(t *testing.T) {
	f := abrirDePrueba(t)

	l, err := d2loader.NewLoader(d2util.LogLevelDefault)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}

	// como en el arranque del motor: primero la fuente MPQ de siempre
	if err := l.AddSource(f.Path(), types.AssetSourceMPQ); err != nil {
		t.Fatalf("AddSource: %v", err)
	}

	f.RegistrarEn(l)

	if len(l.Sources) != 2 || l.Sources[0] != f {
		t.Fatalf("la fuente de consola no quedo primera: %d fuentes", len(l.Sources))
	}

	const rutaDS1 = "/data/global/tiles/act1/town/townn1.ds1"

	tamano, sabido := l.TamanoDeArchivo(rutaDS1)
	if !sabido || tamano == 0 {
		t.Fatalf("el loader no sabe el tamano del .ds1: %d %v", tamano, sabido)
	}

	dst := make([]byte, tamano)

	leidos, err := l.LeerEnteroEnConPrestamo(rutaDS1, dst, nil)
	if err != nil {
		t.Fatalf("LeerEnteroEnConPrestamo por el loader: %v", err)
	}

	if leidos != tamano {
		t.Fatalf("leyo %d de %d bytes", leidos, tamano)
	}

	// la prueba de que la carga la atendio ESTA fuente y no la del MPQ de siempre
	if e := f.Estado(); e.LecturasConPrestamo == 0 {
		t.Fatalf("la carga no paso por la fuente de consola: %+v", e)
	}
}

// abrirDePrueba abre el d2data.mpq de la instalacion del juego. Sin instalacion, la prueba se SALTA (no se
// inventa un MPQ: lo que se compara es el contenido real).
func abrirDePrueba(t *testing.T) *Fuente {
	t.Helper()

	ruta := rutaDePrueba(t)

	f, err := Abrir(ruta)
	if err != nil {
		t.Fatalf("no pude abrir %s: %v", ruta, err)
	}

	t.Cleanup(func() { _ = f.Cerrar() })

	return f
}

func rutaDePrueba(t *testing.T) string {
	t.Helper()

	if d := os.Getenv("D2_DIR"); d != "" {
		if st, err := os.Stat(filepath.Join(d, "d2data.mpq")); err == nil && !st.IsDir() {
			return filepath.Join(d, "d2data.mpq")
		}
	}

	candidata := filepath.Join(os.Getenv("HOME"), ".wine-d2", "drive_c", "Games", "Diablo II", "d2data.mpq")
	if st, err := os.Stat(candidata); err == nil && !st.IsDir() {
		return candidata
	}

	t.Skip("no hay instalacion de Diablo II (D2_DIR o ~/.wine-d2): la prueba necesita el d2data.mpq real")

	return ""
}
