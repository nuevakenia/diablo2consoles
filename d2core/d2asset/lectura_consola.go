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
func (am *AssetManager) loadFileDeConsola(filePath string) ([]byte, error) {
	tamano, sabido := am.Loader.TamanoDeArchivo(filePath)
	if !sabido {
		lecturasSinTamano++

		return am.loadFileConReadAll(filePath)
	}

	return am.loadFileDeConsolaEn(filePath, make([]byte, tamano), tamano)
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
