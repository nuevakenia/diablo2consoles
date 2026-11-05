package d2maprenderer

// DC-002.159i -- la prueba en contra del techo por imagen: lo que no entra en la consola se DICE y se sigue.
//
// Las cuatro cosas que fija la tarjeta, cada una con su mutante (si se saca el guard, la prueba cae):
//
//  1. una imagen que pasa el techo NO pide un byte -- ni del tramo, ni del buffer al vuelo, ni del monton (el
//     `make` del derrame era justo el que moria con `makeslice: too large` / `gc_external_alloc: OOM`);
//  2. se anota UNA vez por clave (el log no puede inundarse con 31 avisos por cuadro) y el censo la cuenta;
//  3. el dibujado la saltea en el acto (`noEntraLaImagen`), que es lo que evita pagar el `GetTiles` y el aviso
//     "Render called on uncached ..." en cada cuadro;
//  4. sin techo (la PC, los POCs del host) el camino es EXACTAMENTE el de siempre: la imagen se pide igual.
//
// Ademas el techo del buffer al vuelo: `ActivarComposicionAlVuelo(81.920)` declara un buffer de trabajo y el
// buffer NO crece por encima (antes crecia a `n + n/4`, que es el pedido de 5,9 MB de la evidencia de 159h).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
)

func rendererConLog() (*MapRenderer, *bytes.Buffer) {
	logs := &bytes.Buffer{}
	l := d2util.NewLogger()
	l.Writer = logs
	l.SetLevel(d2util.LogLevelWarning)

	return &MapRenderer{Logger: l}, logs
}

// TestElTechoPorImagenNoPideUnByte: el pedido que tumbaba la corrida del Gate 0 (4,7 MB de indices) no toca el
// monton y queda anotado para el dibujado.
func TestElTechoPorImagenNoPideUnByte(t *testing.T) {
	mr, _ := rendererConLog()
	mr.ActivarLimiteDeImagen(81920)

	got := mr.pedirTrozoDeIndices(4737760, true)

	if got.indices != nil {
		t.Fatalf("una imagen de 4.737.760 B con techo de 81.920 devolvio %d B: se pidio memoria igual", len(got.indices))
	}

	if got.contenedor != contenedorNoEntra {
		t.Fatalf("contenedor=%d, queria contenedorNoEntra (%d)", got.contenedor, contenedorNoEntra)
	}
}

// TestElTechoNoCambiaElCaminoSinTecho: sin techo, la misma imagen se sirve como siempre (la PC no cambia).
func TestElTechoNoCambiaElCaminoSinTecho(t *testing.T) {
	mr, _ := rendererConLog()

	got := mr.pedirTrozoDeIndices(1000, true)
	if len(got.indices) != 1000 {
		t.Fatalf("sin techo una imagen de 1000 B devolvio %d B", len(got.indices))
	}

	if mr.sobreElTecho(4737760) {
		t.Fatal("sin techo nada esta sobre el techo")
	}
}

// TestElTechoDiceUnaVezPorImagen: el aviso sale UNA vez por clave, con el tamano y el techo, y el censo cuadra.
func TestElTechoDiceUnaVezPorImagen(t *testing.T) {
	mr, logs := rendererConLog()
	mr.ActivarLimiteDeImagen(81920)

	clave := claveDeCache(3, 4, 13, 7)

	for i := 0; i < 25; i++ {
		// el que llama (generate*Cache) es el que sabe de quien es la imagen: aca se imita su contrato
		if trozo := mr.pedirTrozoDeIndices(300000, true); trozo.indices == nil {
			mr.imagenRechazada(clave, "sombra", 300000, 500, 600)
		}
	}

	if n := strings.Count(logs.String(), "MAPA159I:"); n != 1 {
		t.Fatalf("el aviso salio %d veces, queria 1 (una por imagen)\n%s", n, logs.String())
	}

	if !strings.Contains(logs.String(), "300000 B") || !strings.Contains(logs.String(), "81920 B") {
		t.Fatalf("el aviso no dice el tamano ni el techo:\n%s", logs.String())
	}

	if !mr.noEntraLaImagen(clave) {
		t.Fatal("la imagen rechazada no quedo anotada: el dibujado la volveria a pedir por cuadro")
	}

	if imagenes, bytesRechazados, _ := mr.CensoDelLimite159i(); imagenes != 1 || bytesRechazados != 300000 {
		t.Fatalf("censo del techo: imagenes=%d bytes=%d, queria 1 y 300000", imagenes, bytesRechazados)
	}
}

// TestElBufferAlVueloNoCrecePorEncimaDeLoTechado: el buffer de trabajo declarado es un TECHO. Antes, una imagen
// mas grande lo agrandaba a n + n/4 (el 5.922.200 B de la evidencia) y ese `make` es el que no entra en la consola.
func TestElBufferAlVueloNoCrecePorEncimaDeLoTechado(t *testing.T) {
	mr, _ := rendererConLog()
	mr.tramo = ReservarTramoDeLaCache(200 * 1024)
	mr.ActivarComposicionAlVuelo(81920)

	if b := mr.bufferAlVuelo(300000); b != nil {
		t.Fatalf("el buffer al vuelo sirvio %d B con un techo declarado de 81.920", len(b))
	}

	if len(mr.bufferDelVuelo) != 81920 {
		t.Fatalf("el buffer al vuelo crecio a %d B (declarado 81.920)", len(mr.bufferDelVuelo))
	}

	if _, _, vuelos := mr.CensoDelLimite159i(); vuelos != 1 {
		t.Fatalf("vuelosRechazados=%d, queria 1", vuelos)
	}

	// y lo que si entra sigue entrando, con el buffer puesto y en cero
	if b := mr.bufferAlVuelo(76800); len(b) != 76800 {
		t.Fatalf("una imagen de 76.800 B (la mas grande del pueblo) no salio al vuelo: len=%d", len(b))
	}
}

// TestElTechoCortaElDerrame: con techo, ni el paso 5 (`make` del derrame) pide la imagen. Es el mutante del
// defecto: sin el guard del paso 0, este pedido llega al `make([]byte, n)` y en la consola mata la corrida.
func TestElTechoCortaElDerrame(t *testing.T) {
	mr, _ := rendererLogYTramo(t)
	mr.ActivarLimiteDeImagen(81920)

	trozo := mr.pedirTrozoDeIndices(4737760, true)
	if trozo.indices != nil || trozo.contenedor != contenedorNoEntra {
		t.Fatalf("con tramo y techo, una imagen de 4,7 MB salio por contenedor=%d con %d B",
			trozo.contenedor, len(trozo.indices))
	}

	// y el censo del tramo no la cuenta como derrame (no se pidio nada)
	if e := mr.CensoDelTramo(); e.Derrames != 0 || e.AlVuelo != 0 {
		t.Fatalf("censo del tramo: derrames=%d alVuelo=%d, queria 0 y 0", e.Derrames, e.AlVuelo)
	}
}

func rendererLogYTramo(t *testing.T) (*MapRenderer, *bytes.Buffer) {
	t.Helper()

	mr, logs := rendererConLog()
	// bloques de 512 KB: el techo por imagen es 81.920, o sea que la imagen grande no entra en NINGUN camino
	mr.ActivarTramoDeLaCacheEnPedazos(1024*1024, 512*1024)
	mr.ActivarComposicionAlVuelo(81920)

	return mr, logs
}
