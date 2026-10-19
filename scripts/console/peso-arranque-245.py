#!/usr/bin/env python3
"""DC-002.45: cuanto dato VIVO deja el arranque COMPLETO (todas las tablas, en el orden del motor).

POR QUE ES EL NUMERO QUE DECIDE: la tarjeta pide elegir entre "leer por trozos" y "subir el semiespacio".
Leer por trozos ya esta hecho y la lectura de `LvlPrest.txt` pasa -- pero el arranque sigue muriendo, ahora
con `GC to-space overflow`. Eso quiere decir que lo que no entra NO es el archivo sino el DATO YA PARSEADO.
Este script mide ese dato: carga todas las tablas de `d2resource.DataDictionaries()` en orden, con el mismo
camino que el motor (`LoadDataDictionary` + `RecordManager.Load`), y mide el heap vivo al final.

El numero se compara contra el semiespacio de la consola (524.288 B) y contra lo que la consola tiene libre
(el monton de C, medido en la propia corrida: `cfree`).
"""
import os
import subprocess
import sys

PROGRAMA = r'''
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

func main() {
	dir := os.Args[1]

	rm, err := d2records.NewRecordManager(d2util.LogLevelNone)
	if err != nil {
		fmt.Println("no pude crear el RecordManager:", err)
		os.Exit(1)
	}

	var antes runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&antes)

	total := 0
	cargadas := 0
	fallidas := 0
	pico := antes.HeapAlloc

	for _, ruta := range d2resource.DataDictionaries() {
		// el archivo plano: el nombre base, como lo dejo sacar-tabla
		base := filepath.Base(strings.ReplaceAll(ruta, "\\", "/"))
		datos, err := os.ReadFile(filepath.Join(dir, base))
		if err != nil {
			fmt.Printf("%-42s SIN ARCHIVO\n", base)
			fallidas++
			continue
		}

		total += len(datos)

		dict := d2txt.LoadDataDictionary(datos)
		if err := rm.Load(ruta, dict); err != nil {
			fmt.Printf("%-42s %8d B  el loader fallo: %v\n", base, len(datos), err)
			fallidas++
			continue
		}

		var ahora runtime.MemStats
		runtime.ReadMemStats(&ahora)

		vivo := ahora.HeapAlloc - antes.HeapAlloc
		if ahora.HeapAlloc > pico {
			pico = ahora.HeapAlloc
		}

		fmt.Printf("%-42s %8d B  vivo acumulado %8d B\n", base, len(datos), vivo)
		cargadas++
	}

	runtime.GC()

	// el RecordManager TIENE que seguir vivo: sin esto el GC se lleva todo y el numero final es mentira
	runtime.KeepAlive(rm)

	var despues runtime.MemStats
	runtime.ReadMemStats(&despues)

	vivo := int64(despues.HeapAlloc) - int64(antes.HeapAlloc)

	fmt.Println()
	fmt.Printf("tablas cargadas   %d (fallidas %d)\n", cargadas, fallidas)
	fmt.Printf("bytes de archivos %d B\n", total)
	fmt.Printf("DATO VIVO         %d B  (%.0f%% del semiespacio de 524288)\n",
		vivo, 100*float64(vivo)/524288)
	fmt.Printf("PICO del heap     %d B  (%.0f%% del semiespacio de 524288)\n",
		pico-antes.HeapAlloc, 100*float64(int64(pico)-int64(antes.HeapAlloc))/524288)
}
'''


def main():
    dir_tablas = sys.argv[1] if len(sys.argv) > 1 else \
        "/home/dgalli/.hermes/cache/scratch/trozos-245/tablas"

    tmp = "/home/dgalli/Proyectos/diablo2consoles/console/dc/tmp-peso-arranque"
    os.makedirs(tmp, exist_ok=True)

    with open(tmp + "/main.go", "w") as f:
        f.write(PROGRAMA)

    rc = subprocess.run(["go", "run", "./console/dc/tmp-peso-arranque", dir_tablas],
                        cwd="/home/dgalli/Proyectos/diablo2consoles")
    return rc.returncode


if __name__ == "__main__":
    sys.exit(main())
