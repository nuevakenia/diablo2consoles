package d2asset

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"

// DC-002.102 -- LAS TABLAS POR LA FUENTE CON DUENO: la lectura de una .txt deja de abrir un stream.
//
// EL DEFECTO QUE ATACA (medido en DC-002.101). Con el video encendido, el POC del pueblo muere despues de
// la SEGUNDA tabla: `Memory: used=5.033.808 free=2.160 arena=5.035.968` y `Out of memory. Requested
// sbrk_base 0x8cff8000, was 0x8cfe4000, diff 81920`. De esos 5,03 MB, 4.194.304 B son los DOS semiespacios
// del recolector; el resto es el AssetManager y lo que dejaron las tablas: **+324.112 B de monton de C**
// para LvlTypes.txt + LvlPrest.txt (DC-002.101 §4).
//
// POR QUE EL CAMINO DE HOY GASTA MONTON DE C. `LoadDataDictionary` NO lee el archivo: le pasa a
// `d2txt.LoadDataDictionaryReader` el STREAM del asset (`am.LoadAsset` -> `Loader.Load` -> la fuente), y el
// `csv.Reader` lo consume por trozos. En la consola ese stream es `Fuente.Open` -> `d2mpq.ReadFileStream`,
// que **prefetchea el archivo COMPRIMIDO ENTERO** (hasta 2 MB, `mpq_stream.go: raw`) y no lo devuelve
// nadie. Es el mismo camino que la cabecera de `Fuente.Open` avisa que es el que "NO sabe el tamano y NO
// libera nada".
//
// LA CURA, y es la que la rama ya eligio para los otros assets grandes (DC-002.96/.99): la tabla entra
// entera por la FUENTE, en el tramo prestado (monton de C, UNO para todo el motor, reusado archivo tras
// archivo, devuelto con `free` inyectado), y se parsea desde esos bytes. El contrato del tramo se cumple
// solo: la tabla se consume en el acto (`LoadRecords` la entrega a los loaders y vuelve) y entre la lectura
// y el fin del parseo no se lee ningun otro archivo.
//
// POR QUE NO VUELVE EL DEFECTO DE DC-002.43. Ahi el problema era `make([]byte, tamano)` DENTRO del
// semiespacio del recolector (134.937 B de LvlPrest contra 132.104 libres). El tramo NO es del recolector:
// sale del monton de C con dueno (`asset.SourceDeTramos`, `console/dc/fuenteconsola`), y es UNO para todas
// las tablas, asi que la segunda no pide memoria.
//
// CUANDO SE PRENDE. Lo prende `ActivarTablasPorLaFuente()`, que llama el arranque de la consola. La PC no
// lo llama nunca: su `LoadDataDictionary` sigue leyendo por stream, byte por byte, como siempre.
//
// LO QUE MIDIO DC-002.102 (Flycast, POC `mapa-102`, 640x480 con el video encendido), y hay que decirlo
// porque NO es lo que la tarjeta esperaba:
//
//	TABLA fuente LvlTypes.txt  bytes=  10719  montonDelta=      0
//	TABLA stream LvlPrest.txt  bytes= 134937  montonDelta= 324112   <- el camino de hoy
//	TABLA fuente LvlPrest.txt  bytes= 134937  montonDelta= 459056   <- tramo (134.937) + parseo
//	despues de `SoltarLecturasDeConsola`: menos 134.944 -> 324.112 netos, LOS MISMOS que por stream
//
// O sea: el TRAMO se devuelve entero (con `free` inyectado) y el camino queda neutro: los 324.112 B que
// DC-002.101 le atribuyo a la lectura de las tablas son, en realidad, las FILAS YA PARSEADAS de LvlPrest
// (asignaciones de mas de 64 KB del lado de Go, que libgodc manda a `gc_external_alloc`). Lo que SI gana
// el camino es que ya no abre el stream del MPQ (que prefetchea el comprimido y no lo devuelve), o sea que
// las tablas dejan de tener un camino propio con memoria sin dueno. El numero que mueve el monton en esta
// fase es el semiespacio del recolector, no la lectura (ver `medir-semiespacio-102.sh`).
var tablasPorLaFuente bool

// ActivarTablasPorLaFuente prende la lectura de las tablas por el tramo de la fuente (DC-002.102).
// Requiere `ActivarLecturaDeConsola`/`ActivarLecturasPorTramos`; si la fuente no sabe prestar memoria, la
// lectura se cae sola al camino de DC-002.79 (tamano reservado) y el instrumento lo cuenta.
func ActivarTablasPorLaFuente() { tablasPorLaFuente = true }

// TablasPorLaFuenteActivas dice si el camino esta prendido (para el censo del arranque).
func TablasPorLaFuenteActivas() bool { return tablasPorLaFuente }

// LoadDataDictionaryPorStream es el camino de siempre, expuesto para poder medirlo al lado del nuevo en la
// MISMA corrida (el A/B de DC-002.102: mismo archivo, dos caminos, dos deltas de monton). La PC usa este
// camino siempre, sin llamar a nada.
func (am *AssetManager) LoadDataDictionaryPorStream(path string) (*d2txt.DataDictionary, error) {
	stream, err := am.LoadAsset(path)
	if err != nil {
		return nil, err
	}

	am.Debugf(fmtLoadDict, path)

	return d2txt.LoadDataDictionaryReader(stream), nil
}
