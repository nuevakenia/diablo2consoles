package d2asset

// DC-002.79 -- LA LECTURA COMPLETA DE LA CONSOLA: RESERVAR EL TAMANO, NO ADIVINARLO (Fase C.1c).
//
// EL DEFECTO QUE ARREGLA (medido en DC-002.76, bloqueante para el motor en la consola). `LoadFile` es un
// `ioutil.ReadAll` sobre el stream del MPQ, y en la consola eso muere con los archivos grandes:
//
//	.ds1 de 57.652 B            ReadAll OK
//	primer .dt1 de 505.000 B    ReadAll -> "Unhandled exception ... Data address error (read)", PC en io.ReadAll
//	animdata.d2 de 411.104 B    idem
//
// con `mpq.ReadFile` (buffer del tamano EXACTO, una lectura) los mismos archivos se leen sin problema. La
// diferencia es cuanta memoria pide cada camino: `io.ReadAll` no sabe cuanto mide el archivo, va duplicando
// el buffer (512 B, 1 KB, 2 KB...) y en libgodc todo pedido de mas de 64 KB se va al monton externo que no
// devuelve nadie: de esa cadena quedan ~2 MB tomados por archivo de 505 KB (`poc/sonda-gc-277`, DC-002.77).
//
// LA CURA. Pedir el tamano a la fuente ANTES de leer (`d2loader.TamanoDeArchivo`, que sale de la tabla de
// bloques del MPQ), reservar ese tamano UNA vez y leer ahi adentro por trozos
// (`d2loader.LeerEnteroEn`). Ni duplicaciones ni buffer de mas. Y de yapa el prefetch del stream se apaga
// (`d2mpq.InhibirPrefetch`): ese buffer es el archivo comprimido entero, hasta 2 MB, pedido por archivo.
//
// POR QUE NO SE TOCA `LoadAsset`. `LoadAsset` devuelve un `io.ReadSeeker`, no un archivo en memoria: no
// reserva nada y no es el que muere. Lo que muere es el que lo consume con `ReadAll`. La API publica de
// los dos queda igual; el arreglo es por dentro, y en la PC ni se entera (el camino de siempre sigue).
//
// CUANDO SE PRENDE. Lo prende `ActivarLecturaDeConsola()`, que tiene que llamar el arranque de la consola
// (hoy el POC de esta tarjeta; en la Fase D, el main del motor). La PC no la llama NUNCA.
//
// POR QUE VARIABLE Y NO BUILD TAG: gccgo NO filtra por build tag (compila todos los .go del paquete,
// medido en DC-002.2). Es la misma razon que documenta presupuestos.go.

import (
	"io/ioutil"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset"
)

var (
	// caminoDeLecturaDeConsola: prende la lectura con tamano reservado. En la PC queda apagada.
	caminoDeLecturaDeConsola bool

	// el instrumento: cuantas lecturas completas se hicieron, cuantos bytes se llevaron y cuantas se
	// cayeron al camino viejo (fuente que no sabe el tamano).
	lecturasCompletas    int
	bytesLeidosCompletos int64
	lecturasSinTamano    int
)

// DC-002.99 -- LA LECTURA DEL MOTOR CON DUENO: EL ULTIMO `make` POR ARCHIVO.
//
// EL DEFECTO QUE ARREGLA (medido en DC-002.96, seccion 6.1). Con `ActivarLecturaDeConsola` el motor ya
// reservaba el TAMANO exacto, pero lo reservaba POR ARCHIVO (`make([]byte, tamano)` en
// `loadFileDeConsola`). En libgodc todo pedido de mas de 64 KB va a `gc_external_alloc` (malloc pelado) y el
// recolector NO lo devuelve jamas: leer los ocho `.dt1` del pueblo por `d2asset.LoadFile` dejaba tomado lo que
// mide cada archivo, para siempre (DC-002.96 §3.5: los seis formatos de prueba dejaban 3.633.432 B).
//
// LA CURA. `LoadFile` no reserva: pide UN tramo a la fuente (`asset.SourceDeTramos`, opcional) y lee adentro.
// El tramo es UNO y se reusa archivo tras archivo (crece cuando aparece un archivo mas grande que el tramo), y
// lo devuelve `SoltarLecturasDeConsola` -- o el proximo crecimiento. En la consola el tramo sale del monton de
// C con `free` inyectado (`console/dc/fuenteconsola`), asi que devolverlo es devolverlo DE VERDAD.
//
// EL CONTRATO, Y ES EL QUE HAY QUE RESPETAR AL USAR ESTO. Lo que devuelve `LoadFile` con este camino deja de
// ser un buffer propio: **vale hasta la proxima lectura del motor**. Es el mismo contrato que ya tiene el
// arena de tiles (DC-002.78: los bloques de un `.dt1` apuntan al arena) y por eso el que GUARDA los bytes para
// un segundo consumidor (`LoadFileHandoff`, que es lo que usan el `.ds1` y el `.dt1` del camino de la PC) NO
// los pasa por el tramo: los lee con buffer propio (ver `leerParaElHandoff`).
//
// LA PC NO SE ENTERA: el camino se prende con `ActivarLecturasPorTramos`, que `ArrancarComoConsola` llama y
// `NewAssetManager` (el de la PC) nunca.
var (
	// caminoDeTramosDeConsola: prende la lectura por tramo prestado (DC-002.99).
	caminoDeTramosDeConsola bool

	// tramoDeLecturas: EL reservorio. Uno solo para todo el motor, reusado archivo tras archivo.
	tramoDeLecturas asset.TramoPrestado

	// el instrumento: los tramos pedidos, los reusos y lo que mide el tramo vigente.
	tramosPedidos       int
	tramosLiberados     int
	tramosReusados      int
	lecturasPorTramo    int
	bytesPorTramo       int64
	bytesDelTramoActual int
)

// ActivarLecturasPorTramos prende la lectura del motor por el tramo prestado de la fuente (DC-002.99). La
// llama el arranque de la consola; la PC no. Requiere que la fuente implemente `asset.SourceDeTramos` (la de
// consola lo hace): si no, cada carga se va al camino de DC-002.79 y el instrumento lo cuenta.
func ActivarLecturasPorTramos() {
	caminoDeTramosDeConsola = true
}

// EstadoDeLasLecturasPorTramo: el instrumento de DC-002.99, aparte del de DC-002.79 para no cambiar el
// tamano de `EstadoDeLaLectura` (que ya leen otros binarios).
type EstadoDeLasLecturasPorTramo struct {
	Camino        bool  // el camino esta prendido
	Pedidos       int   // tramos pedidos a la fuente (el reservorio y sus crecimientos)
	Liberados     int   // tramos devueltos con Liberar
	Reusos        int   // lecturas hechas en el tramo que ya estaba (memoria no pedida)
	BytesDelTramo int   // bytes del tramo vigente (0 = no hay tramo)
	Lecturas      int   // lecturas que pasaron por el tramo
	Bytes         int64 // bytes leidos por el tramo
}

// EstadoDeLosTramos devuelve el estado de la lectura por tramo (para medir en la consola).
func EstadoDeLosTramos() EstadoDeLasLecturasPorTramo {
	return EstadoDeLasLecturasPorTramo{
		Camino:        caminoDeTramosDeConsola,
		Pedidos:       tramosPedidos,
		Liberados:     tramosLiberados,
		Reusos:        tramosReusados,
		BytesDelTramo: bytesDelTramoActual,
		Lecturas:      lecturasPorTramo,
		Bytes:         bytesPorTramo,
	}
}

// SoltarLecturasDeConsola devuelve al monton el tramo de las lecturas del motor (DC-002.99): despues de esto
// el monton de C vuelve al nivel de partida. Se llama al terminar de cargar (o al tirar el nivel); la proxima
// `LoadFile` pide un tramo nuevo. En la PC no hace nada porque nunca hay tramo.
func SoltarLecturasDeConsola() {
	soltarTramoDeLecturas()
}

// soltarTramoDeLecturas: el cuerpo, sin ceremonia, para usarlo adentro del paquete.
func soltarTramoDeLecturas() {
	if tramoDeLecturas == nil {
		return
	}

	tramoDeLecturas.Liberar()
	tramoDeLecturas = nil
	bytesDelTramoActual = 0
	tramosLiberados++
}

// ActivarLecturaDeConsola prende la lectura completa con el tamano reservado de antemano y APAGA el
// prefetch del MPQ (ver la cabecera de este archivo). La llama el arranque de la consola; la PC no.
func ActivarLecturaDeConsola() {
	caminoDeLecturaDeConsola = true
	d2mpq.InhibirPrefetch(true)
}

// EstadoDeLaLectura: el instrumento, para informar en la consola.
type EstadoDeLaLectura struct {
	CaminoDeConsola bool
	Lecturas        int   // cuantas lecturas completas se hicieron por el camino nuevo
	Bytes           int64 // cuantos bytes se llevaron esas lecturas
	SinTamano       int   // cuantas veces la fuente no dijo el tamano y se volvio al camino viejo
	PrefetchApagado bool  // el prefetch del MPQ (el archivo comprimido entero, hasta 2 MB por archivo)
}

// EstadoDeLosArchivos devuelve el estado de la lectura completa (para medir en la consola).
func EstadoDeLosArchivos() EstadoDeLaLectura {
	return EstadoDeLaLectura{
		CaminoDeConsola: caminoDeLecturaDeConsola,
		Lecturas:        lecturasCompletas,
		Bytes:           bytesLeidosCompletos,
		SinTamano:       lecturasSinTamano,
		PrefetchApagado: d2mpq.PrefetchInhibido(),
	}
}

// loadFileDeConsola: el camino de la consola -- tamano primero, buffer exacto, lectura por trozos dentro
// de ese buffer. Si la fuente no sabe el tamano (una carpeta del disco, por ejemplo), se vuelve al camino
// de siempre en vez de inventar un numero.
//
// DC-002.99: si ademas esta prendida la lectura por tramo y la fuente sabe prestar memoria, el buffer NO se
// pide por archivo: se pide un tramo y se reusa (ver la cabecera de DC-002.99).
func (am *AssetManager) loadFileDeConsola(filePath string) ([]byte, error) {
	if caminoDeTramosDeConsola && am.Loader.TieneTramos(filePath) {
		return am.loadFilePorTramo(filePath)
	}

	return am.loadFileDeConsolaConTamano(filePath)
}

// loadFileDeConsolaConTamano: el camino de DC-002.79, sin tramo -- `make` del tamano exacto y lectura adentro.
// Es el que se usa cuando la fuente no sabe prestar memoria, y el que usa el handoff (que se queda con los
// bytes, ver `leerParaElHandoff`).
func (am *AssetManager) loadFileDeConsolaConTamano(filePath string) ([]byte, error) {
	tamano, sabido := am.Loader.TamanoDeArchivo(filePath)
	if !sabido {
		lecturasSinTamano++

		return am.loadFileConReadAll(filePath)
	}

	return am.loadFileDeConsolaEn(filePath, make([]byte, tamano), tamano)
}

// loadFilePorTramo: la lectura del motor por el tramo prestado de la fuente (DC-002.99).
//
// EL TRAMO ES UNO. La primera lectura pide a la fuente un tramo del tamano del archivo; la siguiente lee
// adentro del mismo si entra, y solo cuando aparece un archivo MAS GRANDE se devuelve el tramo viejo y se pide
// uno nuevo. O sea: `LoadFile` deja de pedir memoria por archivo, que es el defecto que cierra esta tarjeta.
//
// LO QUE DEVUELVE VALE HASTA LA PROXIMA LECTURA (ver la cabecera de DC-002.99): es el precio de no pedir
// memoria, y el que lo consume lo tiene que saber. Los que GUARDAN los bytes (el handoff) no pasan por aca.
func (am *AssetManager) loadFilePorTramo(filePath string) ([]byte, error) {
	tamano, sabido := am.Loader.TamanoDeArchivo(filePath)
	if !sabido {
		lecturasSinTamano++

		return am.loadFileDeConsolaConTamano(filePath)
	}

	// el tramo vigente sirve si entra el archivo; si no, se devuelve y se pide uno del tamano nuevo
	tramo := tramoDeLecturas
	reuso := tramo != nil && len(tramo.Bytes()) >= tamano

	if !reuso {
		soltarTramoDeLecturas()

		nuevo, err := am.Loader.Prestar(filePath, tamano)
		if err != nil {
			// la fuente no sabe prestar (o fallo): no se inventa nada, se lee con buffer propio
			return am.loadFileDeConsolaConTamano(filePath)
		}

		tramo = nuevo
		tramoDeLecturas = nuevo
		tramosPedidos++
		bytesDelTramoActual = len(nuevo.Bytes())
	} else {
		tramosReusados++
	}

	dst := tramo.Bytes()[:tamano]

	d2loader.Marca(filePath, d2loader.MarcaLeeEntra)

	var (
		leidos int
		err    error
	)

	// la lectura no pide memoria de trabajo si la fuente sabe leer con prestamo (DC-002.80); si no, se lee
	// igual al `dst` del que llama, que es lo que importa aca
	if am.Loader.TieneLecturaConPrestamo(filePath) {
		leidos, err = am.Loader.LeerEnteroEnConPrestamo(filePath, dst, nil)
	} else {
		leidos, err = am.Loader.LeerEnteroEn(filePath, dst)
	}

	if err != nil {
		return nil, err
	}

	d2loader.Marca(filePath, d2loader.MarcaLeeVolvio)

	lecturasCompletas++
	bytesLeidosCompletos += int64(leidos)
	lecturasPorTramo++
	bytesPorTramo += int64(leidos)

	return dst[:leidos], nil
}

// leerParaElHandoff: la lectura de los archivos que un SEGUNDO consumidor va a leer despues (`LoadFileHandoff`
// guarda los bytes). No puede ir por el tramo -- el tramo se reusa en la proxima lectura y el segundo
// consumidor leeria memoria de otro archivo -- asi que se reserva un buffer propio, que es el camino de hoy.
// En la PC es el mismo `io.ReadAll` de siempre.
func (am *AssetManager) leerParaElHandoff(filePath string) ([]byte, error) {
	if caminoDeLecturaDeConsola {
		return am.loadFileDeConsolaConTamano(filePath)
	}

	return am.loadFileConReadAll(filePath)
}

// loadFileConReadAll es el camino de siempre (`io.ReadAll` sobre el stream del MPQ), y es al que se vuelve
// cuando no hay tamano que reservar. En la PC es el UNICO camino: nada cambia.
func (am *AssetManager) loadFileConReadAll(filePath string) ([]byte, error) {
	fileAsset, err := am.LoadAsset(filePath)
	if err != nil {
		return nil, err
	}

	// DC-002.43: las dos marcas de la LECTURA COMPLETA. `LoadFile` es el que se lleva el archivo entero a la
	// memoria: aca es donde una lectura grande puede morir por memoria, y sin marca la muerte no decia si
	// habia sido el MPQ o este `ReadAll`.
	d2loader.Marca(filePath, d2loader.MarcaLeeEntra)

	data, err := ioutil.ReadAll(fileAsset)
	if err != nil {
		return nil, err
	}

	d2loader.Marca(filePath, d2loader.MarcaLeeVolvio)

	return data, err
}

// LoadFileEn lee el archivo entero DENTRO de `dst` y devuelve la vista de lo leido (DC-002.79).
//
// Es `LoadFile` para quien ya tiene el buffer: se reserva UNA vez el tamano del archivo mas grande que se
// va a leer y cada archivo se lee ahi adentro. Asi la carga de una zona deja de pedir memoria POR ARCHIVO.
// La vista que devuelve APUNTA A `dst`: vale hasta que se vuelva a usar.
//
// Si `dst` no alcanza, crece (y eso se cuenta como una lectura sin buffer: es lo que hay que evitar).
// No cambia nada en la PC: es una funcion nueva, aditiva.
func (am *AssetManager) LoadFileEn(filePath string, dst []byte) ([]byte, error) {
	tamano, sabido := am.Loader.TamanoDeArchivo(filePath)
	if !sabido {
		lecturasSinTamano++

		datos, err := am.loadFileConReadAll(filePath)
		if err != nil {
			return nil, err
		}

		return datos, nil
	}

	if tamano > len(dst) {
		dst = make([]byte, tamano)
	}

	datos, err := am.loadFileDeConsolaEn(filePath, dst, tamano)
	if err != nil {
		return nil, err
	}

	return datos, nil
}

// loadFileDeConsolaEn: el cuerpo de LoadFileEn, ya con el buffer del tamano correcto.
func (am *AssetManager) loadFileDeConsolaEn(filePath string, dst []byte, tamano int) ([]byte, error) {
	d2loader.Marca(filePath, d2loader.MarcaLeeEntra)

	leidos, err := am.Loader.LeerEnteroEn(filePath, dst[:tamano])
	if err != nil {
		return nil, err
	}

	d2loader.Marca(filePath, d2loader.MarcaLeeVolvio)

	lecturasCompletas++
	bytesLeidosCompletos += int64(leidos)

	return dst[:leidos], nil
}
