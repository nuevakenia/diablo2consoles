#!/usr/bin/env python3
"""DC-002.45: LA TABLA DEL RECORD MANAGER SE CONSUME POR TROZOS (no entera en memoria) -- el arreglo y su mutante.

POR QUE ES UNA SOLA VOLUNTAD (y no un parche a mano): el arreglo de esta tarjeta y su mutante son el MISMO
cambio al reves. El mutante tiene que dejar el motor como estaba -- `LoadDataDictionary` leyendo el asset
ENTERO con `LoadFile` (`ioutil.ReadAll`) y la tabla armada con esos bytes -- porque ESE es el estado que muere
adentro de la lectura de `LvlPrest.txt` (134.937 B contra 132.104 libres en el semiespacio del GC, medido en
DC-002.43). Si los dos textos no salen de aca, la vuelta se hace a mano y a mano se olvida: el mutante queda
pegado y la corrida siguiente mide cualquier cosa.

LO QUE CAMBIA (los DOS cuerpos de `d2core/d2asset/asset_manager.go`):

  * `LoadDataDictionary(path)`:  PRISTINO  `LoadFile(path)` (el asset entero a memoria) + `d2txt.LoadDataDictionary(bytes)`
                                SANO      el stream del asset tal cual (`LoadAsset`) + `d2txt.LoadDataDictionaryReader(stream)`
  * `LoadRecords(path)`:         PRISTINO  sin marcas propias (las de la lectura las ponia `LoadFile`)
                                SANO      las dos marcas de LA LECTURA (`lee-entra`/`lee-volvio`) envuelven el CONSUMO
                                          de la tabla, que es donde el stream se lee de verdad

Uso:  fix-tablas-trozos-245.py --estado       (que dice el arbol: arreglo, pristino u otro)
      fix-tablas-trozos-245.py --pristino     (el mutante: el asset entero en memoria)
      fix-tablas-trozos-245.py --sano         (el arreglo: la tabla por trozos)

Es IDEMPOTENTE: aplicarlo dos veces deja el mismo contenido, y cada modo verifica que el archivo quedo en el
estado pedido (si no, sale con error).
"""

import os
import re
import signal
import sys

# si el que llama corta la salida (`| head -1`), morir callado como cualquier herramienta de linea de comandos
signal.signal(signal.SIGPIPE, signal.SIG_DFL)

RAIZ = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
ARCHIVO = "d2core/d2asset/asset_manager.go"

os.chdir(RAIZ)

# la region que se reescribe: los DOS cuerpos (LoadDataDictionary y LoadRecords), que van pegados.
PATRON = re.compile(
    r"// LoadDataDictionary loads a txt data file.*?\nfunc \(am \*AssetManager\) LoadRecords\(path string\) error \{.*?\n\}\n",
    re.S,
)

SAN0 = "sano"
PRISTINO = "pristino"

TEXTO_SAN0 = r'''// LoadDataDictionary loads a txt data file.
//
// DC-002.45 -- LA TABLA SE CONSUME POR TROZOS, NO ENTERA EN MEMORIA. Antes esto era
// `LoadFile(path)` (el `ioutil.ReadAll` del asset completo) + `d2txt.LoadDataDictionary(bytes)`: el archivo
// ENTERO tenia que entrar en el semiespacio del GC de la consola ANTES de parsear la primera fila, y el
// arranque moria adentro de esa lectura (`LvlPrest.txt`: 134.937 B contra 132.104 libres al entrar;
// DC-002.43). El `csv.Reader` de la tabla NO necesita los bytes: los pide por trozos a medida que avanza, y
// el stream del MPQ ya entrega por SECTOR (4 KB). Se le pasa el stream del asset tal cual. Lo que queda vivo
// son las filas ya parseadas, no el archivo: el techo de memoria no cambia y sirve para cualquier tabla
// grande (MonStats.txt son 432.634 B, Skills.txt 173.535).
//
// OJO: el `DataDictionary` lee del stream MIENTRAS se lo recorre, asi que la tabla se tiene que consumir
// antes de olvidar el stream. El que la consume lo hace en el acto: `LoadRecords` la entrega a los loaders
// del RecordManager y vuelve. El comentario de mas abajo sobre el cacheo sigue valiendo (no se cachea el
// DataDictionary: no se puede recorrer dos veces, ni con un stream ni con los bytes de antes).
func (am *AssetManager) LoadDataDictionary(path string) (*d2txt.DataDictionary, error) {
	stream, err := am.LoadAsset(path)
	if err != nil {
		return nil, err
	}

	am.Debugf(fmtLoadDict, path)

	return d2txt.LoadDataDictionaryReader(stream), nil
}

// LoadRecords will load the records for the given path into the record manager.
// This is dependant on the record manager having bound a loader for the given path.
func (am *AssetManager) LoadRecords(path string) error {
	dict, err := am.LoadDataDictionary(path)
	if err != nil {
		return err
	}

	// DC-002.45: las dos marcas de la LECTURA envuelven EL CONSUMO del asset, que es donde el stream se lee
	// de verdad (los loaders del RecordManager recorren la tabla entera aca adentro). Con la tabla entera en
	// memoria (`ReadAll`) las dos marcas quedaban adentro de `LoadFile`; el significado es el MISMO -- la
	// ultima marca de la corrida sigue diciendo que asset estaba leyendo el motor cuando se murio -- pero
	// ahora la marca mira al consumidor, que es donde puede morir una tabla que se lee por trozos.
	d2loader.Marca(path, d2loader.MarcaLeeEntra)

	err = am.Records.Load(path, dict)

	d2loader.Marca(path, d2loader.MarcaLeeVolvio)

	if err != nil {
		return err
	}

	return nil
}
'''

TEXTO_PRISTINO = r'''// LoadDataDictionary loads a txt data file
func (am *AssetManager) LoadDataDictionary(path string) (*d2txt.DataDictionary, error) {
	// we purposefully do not cache data dictionaries because we are already
	// caching the file data. The underlying csv.Reader does not implement io.Seeker,
	// so after it has been iterated through, we cannot iterate through it again.
	//
	// The easy way around this is to not cache d2txt.DataDictionary objects, and just create
	// a new instance from cached file data if/when we ever need to reload the data dict
	data, err := am.LoadFile(path)
	if err != nil {
		return nil, err
	}

	am.Debugf(fmtLoadDict, path)

	return d2txt.LoadDataDictionary(data), nil
}

// LoadRecords will load the records for the given path into the record manager.
// This is dependant on the record manager having bound a loader for the given path.
func (am *AssetManager) LoadRecords(path string) error {
	dict, err := am.LoadDataDictionary(path)
	if err != nil {
		return err
	}

	err = am.Records.Load(path, dict)
	if err != nil {
		return err
	}

	return nil
}
'''


def estado() -> str:
    actual = open(ARCHIVO).read()
    tiene_sano = TEXTO_SAN0 in actual
    tiene_pristino = TEXTO_PRISTINO in actual
    if tiene_sano and not tiene_pristino:
        return "sano"
    if tiene_pristino and not tiene_sano:
        return "pristino"
    return "otro"


def aplica(texto: str, cual: str) -> None:
    actual = open(ARCHIVO).read()
    nuevo, n = PATRON.subn(lambda _m: texto, actual, count=1)
    if n != 1:
        raise SystemExit("no pude ubicar la region de LoadDataDictionary/LoadRecords en %s" % ARCHIVO)
    if nuevo == actual:
        print("ya estaba en el estado %s" % cual)
    else:
        open(ARCHIVO, "w").write(nuevo)
        print("%s -> estado %s" % (ARCHIVO, cual))
    if estado() != cual:
        raise SystemExit("el archivo NO quedo en el estado %s" % cual)


def main() -> int:
    args = sys.argv[1:]
    if args == ["--estado"]:
        e = estado()
        print("estado: %s" % e)
        print("  la tabla por trozos (el arreglo): %s" % ("SI" if TEXTO_SAN0 in open(ARCHIVO).read() else "no"))
        print("  el asset entero en memoria (el mutante): %s" % ("SI" if TEXTO_PRISTINO in open(ARCHIVO).read() else "no"))
        return 0 if e in ("sano", "pristino") else 1
    if args == ["--sano"]:
        aplica(TEXTO_SAN0, "sano")
        return 0
    if args == ["--pristino"]:
        aplica(TEXTO_PRISTINO, "pristino")
        return 0
    print(__doc__)
    return 2


if __name__ == "__main__":
    sys.exit(main())
