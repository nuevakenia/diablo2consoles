#!/usr/bin/env python3
"""DC-002.45: cuanto pesa DE VERDAD una tabla del juego una vez parseada (en el host, con Go).

POR QUE: la tarjeta dice que la pared es el archivo entero en memoria. Pero con la tabla por trozos la
consola ya no muere en la lectura: muere con `GC to-space overflow`, que quiere decir que lo que NO entra
es el dato VIVO (las filas ya parseadas). Este script mide ese numero, que en la consola no se puede medir
(el runtime no tiene un contador de "vivo por tabla").
"""
import subprocess
import sys

PROGRAMA = r'''
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

func main() {
	ruta := os.Args[1]
	datos, err := os.ReadFile(ruta)
	if err != nil {
		fmt.Println("no pude leer", ruta, err)
		os.Exit(1)
	}

	var antes, despues runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&antes)

	// el MISMO camino que el motor: la tabla y despues su loader
	dict := d2txt.LoadDataDictionary(datos)
	rm, err := d2records.NewRecordManager(d2util.LogLevelNone)
	if err != nil {
		fmt.Println("no pude crear el RecordManager:", err)
		os.Exit(1)
	}

	if err := rm.Load("/data/global/excel/LvlPrest.txt", dict); err != nil {
		fmt.Println("el loader fallo:", err)
		os.Exit(1)
	}

	runtime.GC()
	runtime.ReadMemStats(&despues)

	fmt.Printf("archivo           %d B\n", len(datos))
	fmt.Printf("filas             %d\n", len(rm.Level.Presets))
	fmt.Printf("heap vivo antes   %d B\n", antes.HeapAlloc)
	fmt.Printf("heap vivo despues %d B\n", despues.HeapAlloc)
	fmt.Printf("LO QUE QUEDA VIVO %d B  (%.1f%% del semiespacio de 524288)\n",
		int64(despues.HeapAlloc)-int64(antes.HeapAlloc),
		100*float64(int64(despues.HeapAlloc)-int64(antes.HeapAlloc))/524288)
}
'''


def main():
    ruta_tabla = sys.argv[1] if len(sys.argv) > 1 else \
        "/home/dgalli/.hermes/cache/scratch/trozos-245/tablas/LvlPrest.txt"

    tmp = "/home/dgalli/Proyectos/diablo2consoles/console/dc/tmp-peso-vivo"
    subprocess.run(["mkdir", "-p", tmp], check=True)

    with open(tmp + "/main.go", "w") as f:
        f.write(PROGRAMA)

    rc = subprocess.run(["go", "run", "./console/dc/tmp-peso-vivo", ruta_tabla],
                        cwd="/home/dgalli/Proyectos/diablo2consoles")
    return rc.returncode


if __name__ == "__main__":
    sys.exit(main())
