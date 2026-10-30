package d2asset

// DC-002.78 -- LA CARGA DE TILES DE LA CONSOLA NO PIDE MEMORIA POR ARCHIVO (Fase B.1e).
//
// EL DEFECTO QUE ARREGLA (medido en DC-002.77, no supuesto). Entrar al pueblo deja el monton en
// 12.073.160 B y la segunda zona muere con `gc_external_alloc: OOM` pidiendo 368.640 B. El cache no era el
// culpable (desaloja 6 veces y se queda en 721.552 B de 1.048.576): el culpable es que en libgodc toda
// asignacion de mas de 64 KB se va a `gc_external_alloc` (`gc_heap.c:116`), que es `malloc` pelado y no lo
// recolecta nadie -- medido con `console/dc/poc/sonda-gc-277`: 12 trozos de 256 KB suman +262.152 B cada
// uno, con y sin `runtime.GC()`. Y cada .dt1 del acto 1 mide 318.414 B a 1.008.003 B.
//
// LA CURA: UN ARENA, no un buffer por archivo.
//
// El camino de siempre pide un arreglo NUEVO por archivo (`ioutil.ReadAll` / `mpq.ReadFile`) y, como el
// archivo pasa de 64 KB, ese arreglo queda tomado para siempre. Aca el motor pide UNA vez un arena del
// tamano de los .dt1 de una zona entera (4.408.317 B crudos, medidos en
// `console/dc/evidencia/dc-002.77-zona-host.txt`) y cada archivo se lee ADENTRO, uno detras de otro (bump).
// La carga de una zona deja de pedir memoria: el arena se pide una vez y despues solo avanza un cursor.
//
// POR QUE NO SE COPIAN LOS BLOQUES (el camino (a) de la tarjeta, implementado y medido EN CONTRA). Copiar
// cada bloque a memoria propia (`d2dt1.LoadDT1CopiaBloques`) deja los bloques en el SEMIESPACIO del
// recolector (miden 8..1.181 B, todos bajo el umbral) y en el papel eso es lo correcto. Medido en Flycast:
// NO AGUANTA. Una zona son 16.266 copias; el recolector salta por umbral EN MEDIO del parseo (dos veces en
// los dos primeros archivos) y el SH-4 muere con `Unhandled exception: PC 7daed713, code 1, evt 00e0` /
// `Data address error`: el recolector de libgodc es de copia y mueve objetos mientras el parser tiene
// punteros a los pedazos que esta armando. Con el arena no hay copias ni presion sobre el semiespacio: la
// carga se comporta, en el semiespacio, como la de DC-002.70/.75/.77 (que corrian bien hasta quedarse sin
// monton externo).
//
// EL CONTRATO (lo que hay que respetar al usar esto):
//   - los bloques de un .dt1 cargado asi APUNTAN AL ARENA: valen mientras el arena no se reinicie.
//   - el arena se reinicia solo cuando un archivo no entra, y al reiniciarlo se VACIA el cache de dt1s
//     (sus entradas apuntaban adentro). Tambien se puede reiniciar a mano con `ReiniciarArenaDeTiles`, que
//     es lo que tiene que hacer el motor cuando TIRA UN NIVEL.
//   - quien se guarde los tiles de un .dt1 mas alla del nivel (el `dt1TileData` del MapEngine) tiene que
//     copiarlos a su propio arreglo, o reiniciar el arena solo al tirar el nivel.
//
// POR QUE UN ARENA Y NO UN POOL DE N BUFFERS. Con N buffers hay que decidir cual se puede reusar, y eso es
// llevar la cuenta de quien tiene el .dt1: un error ahi se ve como un mapa dibujado con pedazos de otro
// archivo. El arena no tiene esa decision: avanza y, cuando no entra, se tira entero (y con el, el cache).
//
// POR QUE NO SE LIBERA NADA CON `runtime.FreeExternal`. El simbolo existe en libgodc (`gc_heap.c:283`,
// `_runtime.FreeExternal`) pero NO esta declarado en Go: solo se puede llamar desde C
// (`tests/c/test_free_external.c`). Y la corrida de DC-002.77 con `free()` mostro la otra punta del
// problema: liberar y volver a pedir tamanos distintos FRAGMENTA el monton (arena 13,4 MB con 928 KB libres
// y sin bloque contiguo de 397 KB). El arena no libera nada: no fragmenta nada.
//
// CUANDO SE PRENDE. Lo prende `AjustarPresupuestosDeConsola()` (presupuestos.go), que es el arranque de la
// consola. La PC no la llama NUNCA: en el host todo esto queda apagado y el camino es el de siempre.
//
// POR QUE VARIABLE Y NO BUILD TAG: gccgo NO filtra por build tag (compila todos los .go del paquete, medido
// en DC-002.2): dos archivos, uno con tag, entrarian los dos. Es la misma razon que documenta
// presupuestos.go.

import (
	"errors"
	"io"
	"sync"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2datautils"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
)

const (
	// capacidadDeLaArenaDeTiles: los .dt1 de la zona mas pesada del acto 1 (el pueblo: 8 archivos,
	// 4.408.317 B crudos, medido en DC-002.77) mas un 10 % de aire. Es el numero que hace que entrar a
	// una zona no pida memoria: los 4,41 MB entran de una y los archivos se leen uno detras de otro.
	capacidadDeLaArenaDeTiles = 4408317 + 440831 // 4.848.148 B
)

// errores del camino de la consola
var (
	// errDT1NoEntraEnLaArena: el archivo es mas grande que el arena entero. No pasa con los .dt1 del acto
	// 1 (el mayor mide 1.008.003 B); si pasara, se dice en vez de morir.
	errDT1NoEntraEnLaArena = errors.New("d2asset: el .dt1 no entra en el arena de tiles")

	// errSinArena: la ruta de la consola esta prendida pero el arena no se pudo pedir.
	errSinArena = errors.New("d2asset: no hay arena de tiles")
)

var (
	// las dos mitades del camino de la consola. En la PC quedan apagadas.
	caminoDeTilesDeConsola bool

	// lecturaPrestadaDeTiles (DC-002.80): leer cada .dt1 con la memoria de trabajo PRESTADA por el arena
	// (`d2mpq.ReadFileInto`), en vez de por el stream del MPQ. Es lo que hace que la lectura no pida
	// memoria por archivo: medido en DC-002.78, leer UN .dt1 de 505 KB por el stream pedia ~2 MB del
	// semiespacio del recolector y el runtime moria con `gc_alloc: out of memory`. Apagada queda el camino
	// de DC-002.78 (que se midio EN CONTRA); la palanca existe para poder medir los dos en la misma corrida.
	lecturaPrestadaDeTiles = true

	// el prestamo: la tabla de posiciones, el bloque comprimido, el descompreso y el descompresor de zlib
	// de la lectura. Se pide la primera vez y se reusa entre archivos y entre zonas.
	prestamoDeTiles *d2mpq.Scratch

	// el arena: pedido UNA vez, con la medida de una zona. Los .dt1 se leen adentro.
	arenaDeTiles      []byte
	cursorDelArena    int
	reiniciosDelArena int
	archivosDeTiles   int

	// el lector por trozos (no pide memoria propia: lee ADENTRO del tramo que se le presta). Es el camino
	// de DC-002.78, el que se mide en contra.
	lectorDeTiles  d2datautils.BufferGrande
	candadoDeTiles sync.Mutex
)

// UsarLecturaPrestadaDeTiles prende/apaga la lectura con prestamo del camino de tiles (DC-002.80). Lo usa la
// palanca `LEER=` del POC de zonas para medir los dos caminos con el mismo binario.
func UsarLecturaPrestadaDeTiles(v bool) { lecturaPrestadaDeTiles = v }

// LecturaPrestadaDeTiles dice si la ruta de tiles esta leyendo el MPQ con memoria prestada (DC-002.81). Es el
// estado REAL de la palanca -- no el que el que llama creia haber pedido --, que es lo que hay que ver en el
// log del motor: `lecturaPrestada= true`.
func LecturaPrestadaDeTiles() bool { return lecturaPrestadaDeTiles }

// EstadoDelPrestamoDeTiles devuelve el instrumento del prestamo de la lectura (y si hay prestamo armado).
func EstadoDelPrestamoDeTiles() (d2mpq.EstadoDelPrestamo, bool) {
	candadoDeTiles.Lock()
	defer candadoDeTiles.Unlock()

	if prestamoDeTiles == nil {
		return d2mpq.EstadoDelPrestamo{}, false
	}

	return prestamoDeTiles.Estado(), true
}

// ActivarCaminoDeTilesDeConsola prende la ruta del arena: cada .dt1 se lee adentro de un arena pedido una
// sola vez, con la lectura del MPQ por memoria prestada (DC-002.80). La llama
// `NuevoAssetManagerDeConsola`, o sea que en la consola va prendida siempre; sigue siendo una funcion (y no
// una constante) porque la palanca `BUFFER=` del POC de zonas mide con y sin la ruta, y porque el arena se
// pide en la PRIMERA carga de un .dt1 y no aca (el arranque del PVR necesita el monton ordenado, DC-002.50).
func ActivarCaminoDeTilesDeConsola() {
	caminoDeTilesDeConsola = true
}

// EstadoDelArenaDeTiles: el instrumento, para informar en la consola.
type EstadoDelArenaDeTiles struct {
	CaminoDeConsola bool
	Capacidad       int // lo que se pidio UNA vez (0 = todavia no se pidio)
	Usado           int // lo que avanzo el cursor con los .dt1 vivos
	Reinicios       int // cuantas veces se tiro el arena (cada uno vacia el cache de dt1s)
	ArchivosLeidos  int // cuantos .dt1 se cargaron por este camino
}

// EstadoDeLosTiles devuelve el estado del arena (para medir en la consola).
func EstadoDeLosTiles() EstadoDelArenaDeTiles {
	candadoDeTiles.Lock()
	defer candadoDeTiles.Unlock()

	return EstadoDelArenaDeTiles{
		CaminoDeConsola: caminoDeTilesDeConsola,
		Capacidad:       cap(arenaDeTiles),
		Usado:           cursorDelArena,
		Reinicios:       reiniciosDelArena,
		ArchivosLeidos:  archivosDeTiles,
	}
}

// ReiniciarArenaDeTiles tira el arena y VACIA el cache de dt1s (sus entradas apuntan adentro del arena).
// Es lo que hay que llamar cuando el motor tira un nivel: ahi nadie usa mas los tiles del anterior.
func (am *AssetManager) ReiniciarArenaDeTiles() {
	candadoDeTiles.Lock()
	defer candadoDeTiles.Unlock()

	am.reiniciarArenaDeTilesSinCandado()
}

func (am *AssetManager) reiniciarArenaDeTilesSinCandado() {
	cursorDelArena = 0
	reiniciosDelArena++

	if am != nil && am.dt1s != nil {
		am.dt1s.Clear()
	}
}

// cargarDT1EnLaArena: la ruta de la consola para un .dt1 -- el archivo se lee ADENTRO del arena, sin pedir
// memoria, y los bloques quedan apuntando ahi (ver el contrato en la cabecera del archivo).
func (am *AssetManager) cargarDT1EnLaArena(filePath string) (*d2dt1.DT1, error) {
	candadoDeTiles.Lock()
	defer candadoDeTiles.Unlock()

	dt1, entro, err := am.parsearDT1EnLaArena(filePath)
	if err != nil {
		return nil, err
	}

	if entro {
		archivosDeTiles++

		return dt1, nil
	}

	// no entro: se tira el arena (y con el, el cache de dt1s) y se reintenta con el arena vacio
	am.reiniciarArenaDeTilesSinCandado()

	dt1, entro, err = am.parsearDT1EnLaArena(filePath)
	if err != nil {
		return nil, err
	}

	if !entro {
		return nil, errDT1NoEntraEnLaArena
	}

	archivosDeTiles++

	return dt1, nil
}

// parsearDT1EnLaArena lee UN .dt1 en el tramo libre del arena y lo parsea ahi mismo. Devuelve `entro` en
// false cuando el archivo no entra en lo que queda del arena (sin pedir memoria de mas).
//
// DC-002.80: la lectura del archivo tambien es prestada -- `d2mpq.ReadFileInto(nombre, tramo, prestamo)`, con
// el prestamo que se reusa entre archivos. Antes (DC-002.78) se leia por el STREAM del MPQ, que pide ~2 MB
// del semiespacio del recolector por archivo y mataba la consola en medio de la carga.
func (am *AssetManager) parsearDT1EnLaArena(filePath string) (dt1 *d2dt1.DT1, entro bool, err error) {
	if arenaDeTiles == nil {
		arenaDeTiles = make([]byte, capacidadDeLaArenaDeTiles)
	}

	if arenaDeTiles == nil {
		return nil, false, errSinArena
	}

	tramo := arenaDeTiles[cursorDelArena:]
	if len(tramo) == 0 {
		return nil, false, nil
	}

	var datos []byte

	if lecturaPrestadaDeTiles {
		datos, entro, err = am.leerDT1ConPrestamo(filePath, tramo)
	} else {
		datos, entro, err = am.leerDT1PorElStream(filePath, tramo)
	}

	if err != nil {
		return nil, false, err
	}

	if !entro {
		return nil, false, nil
	}

	// los bloques apuntan a `datos`, o sea al arena: no se copia nada
	dt1, err = d2dt1.LoadDT1(datos)
	if err != nil {
		return nil, false, err
	}

	cursorDelArena += len(datos)

	return dt1, true, nil
}

// leerDT1ConPrestamo lee el archivo ADENTRO del tramo del arena con la memoria de trabajo prestada: ni el
// destino ni la lectura piden nada. `entro` en false = el archivo no cabe en lo que queda del arena (se
// devuelve sin pedir memoria; el que llama tira el arena y reintenta).
func (am *AssetManager) leerDT1ConPrestamo(filePath string, tramo []byte) (datos []byte, entro bool, err error) {
	// si ninguna fuente sabe leer con prestamo (una carpeta del disco, por ejemplo), se vuelve al camino de
	// siempre: ahi no hay MPQ del que prestar nada
	if !am.Loader.TieneLecturaConPrestamo(filePath) {
		return am.leerDT1PorElStream(filePath, tramo)
	}

	if prestamoDeTiles == nil {
		prestamoDeTiles = &d2mpq.Scratch{}
	}

	leidos, err := am.Loader.LeerEnteroEnConPrestamo(filePath, tramo, prestamoDeTiles)
	if err != nil {
		// no entra: no es un fallo, es que hay que tirar el arena y reintentar
		if err == d2mpq.ErrNoEntraEnElBuffer {
			return nil, false, nil
		}

		return nil, false, err
	}

	return tramo[:leidos], true, nil
}

// leerDT1PorElStream es el camino de DC-002.78: se pide el archivo como stream (`LoadAsset`) y se lee adentro
// del tramo con un buffer por trozos. El DESTINO no pide memoria, pero la lectura si (~2 MB de semiespacio por
// archivo, medido). Queda como palanca de medicion.
func (am *AssetManager) leerDT1PorElStream(filePath string, tramo []byte) (datos []byte, entro bool, err error) {
	archivo, err := am.LoadAsset(filePath)
	if err != nil {
		return nil, false, err
	}

	if cerrable, ok := archivo.(io.Closer); ok {
		defer func() { _ = cerrable.Close() }()
	}

	return lectorDeTiles.LeerEn(tramo, archivo)
}
