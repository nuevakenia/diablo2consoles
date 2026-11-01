package d2asset

// DC-002.111 -- PRUEBAS DEL BUFFER DE LECTURA DE LOS `.dt1` POR ARCHIVO.
//
// QUE SE PRUEBA, Y POR QUE ALCANZA. La tarjeta no pregunta si el buffer se libera "bien" (eso lo mide la
// corrida en Flycast, con el monton de C de verdad): pregunta si el buffer deja de estar TOMADO durante la
// carga del nivel, que es lo que lo saca del pico. Eso son dos invariantes que si se pueden fijar en el host:
//
//  1. con la palanca encendida, `PrepararArenaDeBloquesUsados` NO pide el buffer (antes pedia uno del tamano
//     del `.dt1` mas grande: 1.008.003 B en el pueblo) y `cargarDT1EnBloquesUsados` lo devuelve al terminar
//     cada archivo;
//  2. el liberador es el INYECTADO (el `free` del shim en la consola), una vez por buffer, y sin liberador
//     instalado no se toca nada -- que es lo que deja a la PC con el camino de siempre.
//
// Lo que estas pruebas NO pueden ver: que el puntero liberado sea del monton de C (en el host es memoria del
// recolector y `free` no esta en la libreria estandar). Por eso el liberador de la prueba solo CUENTA.

import (
	"sync"
	"testing"
)

// liberadorDePrueba: cuenta las llamadas y los punteros, sin liberar nada (ver la cabecera).
type liberadorDePrueba struct {
	candado  sync.Mutex
	llamadas int
	punteros []uintptr
}

func (l *liberadorDePrueba) soltar(p uintptr) {
	l.candado.Lock()
	defer l.candado.Unlock()

	l.llamadas++
	l.punteros = append(l.punteros, p)
}

// prepararLaPalanca deja el paquete como lo encuentra la prueba y lo restaura al salir.
func prepararLaPalanca(t *testing.T, porArchivo bool) *liberadorDePrueba {
	t.Helper()

	candadoDeTiles.Lock()
	scratchAntes, scratchDelNivelAntes := scratchDeLectura, scratchDelNivel
	tramosAntes, actualAntes, cursorAntes := arenaDeBloques, tramoActual, cursorDelBloque
	clavesAntes := clavesDelNivel
	candadoDeTiles.Unlock()

	usoAntes := ScratchPorArchivoActivo()
	soltadosAntes, bytesAntes := scratchSoltados, scratchBytesSoltados

	l := &liberadorDePrueba{}
	UsarLiberadorDeExternos(l.soltar)
	UsarScratchPorArchivo(porArchivo)

	candadoDeTiles.Lock()
	scratchDeLectura, scratchDelNivel = nil, 0
	arenaDeBloques, tramoActual, cursorDelBloque = nil, 0, 0
	clavesDelNivel = nil
	scratchSoltados, scratchBytesSoltados = 0, 0
	candadoDeTiles.Unlock()

	t.Cleanup(func() {
		candadoDeTiles.Lock()
		scratchDeLectura, scratchDelNivel = scratchAntes, scratchDelNivelAntes
		arenaDeBloques, tramoActual, cursorDelBloque = tramosAntes, actualAntes, cursorAntes
		clavesDelNivel = clavesAntes
		scratchSoltados, scratchBytesSoltados = soltadosAntes, bytesAntes
		candadoDeTiles.Unlock()

		UsarScratchPorArchivo(usoAntes)
		UsarLiberadorDeExternos(nil)
	})

	return l
}

// TestScratchPorArchivoNoSePideEnPreparar: con la palanca encendida la carga del nivel NO arranca con el
// buffer tomado. Es el defecto que se arregla: antes el buffer del `.dt1` mas grande estaba pedido desde
// `PrepararArenaDeBloquesUsados` (o sea, desde ANTES del primer archivo) y se soltaba recien despues de
// armar la cache de imagenes.
func TestScratchPorArchivoNoSePideEnPreparar(t *testing.T) {
	l := prepararLaPalanca(t, true)

	PrepararArenaDeBloquesUsados(nil, 1008003)

	candadoDeTiles.Lock()
	pedido, tamano := len(scratchDeLectura), scratchDelNivel
	candadoDeTiles.Unlock()

	if pedido != 0 {
		t.Fatalf("con la palanca por archivo no se pide nada en Preparar: quedo un buffer de %d B", pedido)
	}

	if tamano != 1008003 {
		t.Fatalf("scratchDelNivel = %d, quiero el del archivo mas grande (1008003)", tamano)
	}

	if l.llamadas != 0 {
		t.Fatalf("no habia nada que liberar y el liberador se llamo %d veces", l.llamadas)
	}
}

// TestScratchDelNivelSePideEnPreparar: el camino de DC-002.101 (palanca apagada) sigue pidiendo el buffer del
// archivo mas grande antes del primer `.dt1`. Es el A/B: si esto cambia, la corrida de referencia no es la
// misma y los numeros no se pueden comparar.
func TestScratchDelNivelSePideEnPreparar(t *testing.T) {
	prepararLaPalanca(t, false)

	PrepararArenaDeBloquesUsados(nil, 1008003)

	candadoDeTiles.Lock()
	pedido := len(scratchDeLectura)
	candadoDeTiles.Unlock()

	if pedido != 1008003 {
		t.Fatalf("con la palanca apagada el buffer es del archivo mas grande: quedo en %d B", pedido)
	}
}

// TestSoltarElBufferDevuelveAlLiberador: soltar devuelve el buffer con el liberador INYECTADO (una vez), deja
// el estado en nil y lo cuenta en el instrumento. Es el mismo camino que usa el final de cada `.dt1`.
func TestSoltarElBufferDevuelveAlLiberador(t *testing.T) {
	l := prepararLaPalanca(t, true)

	candadoDeTiles.Lock()
	scratchDeLectura = make([]byte, 661515)
	scratchDelNivel = 661515
	candadoDeTiles.Unlock()

	bytes := SoltarPrestamoDeLecturaDeTiles()

	if bytes != 661515 {
		t.Fatalf("SoltarPrestamoDeLecturaDeTiles devolvio %d B, quiero 661515", bytes)
	}

	if l.llamadas != 1 {
		t.Fatalf("el liberador se llamo %d veces, quiero exactamente 1", l.llamadas)
	}

	candadoDeTiles.Lock()
	quedo, tamano := len(scratchDeLectura), scratchDelNivel
	soltados, bytesSoltados := scratchSoltados, scratchBytesSoltados
	candadoDeTiles.Unlock()

	if quedo != 0 || tamano != 0 {
		t.Fatalf("despues de soltar quedo buffer=%d scratchDelNivel=%d, quiero 0 y 0", quedo, tamano)
	}

	if soltados != 1 || bytesSoltados != 661515 {
		t.Fatalf("el instrumento dice soltados=%d bytes=%d, quiero 1 y 661515", soltados, bytesSoltados)
	}

	// soltar dos veces no puede devolver el mismo puntero dos veces (doble free en la consola)
	if otra := SoltarPrestamoDeLecturaDeTiles(); otra != 0 || l.llamadas != 1 {
		t.Fatalf("el segundo soltar devolvio %d B con %d llamadas al liberador, quiero 0 y 1", otra, l.llamadas)
	}
}

// TestSinLiberadorNoSeTocaNada: en la PC (y en cualquier arranque sin `UsarLiberadorDeExternos`) soltar el
// buffer no hace nada. Es lo que deja el camino del host igual que antes de esta tarjeta.
func TestSinLiberadorNoSeTocaNada(t *testing.T) {
	prepararLaPalanca(t, true)
	UsarLiberadorDeExternos(nil)

	candadoDeTiles.Lock()
	scratchDeLectura = make([]byte, 505504)
	scratchDelNivel = 505504
	candadoDeTiles.Unlock()

	if bytes := SoltarPrestamoDeLecturaDeTiles(); bytes != 0 {
		t.Fatalf("sin liberador no se suelta nada: devolvio %d B", bytes)
	}

	candadoDeTiles.Lock()
	quedo := len(scratchDeLectura)
	candadoDeTiles.Unlock()

	if quedo != 505504 {
		t.Fatalf("sin liberador el buffer tiene que quedar como estaba: quedo en %d B", quedo)
	}
}
