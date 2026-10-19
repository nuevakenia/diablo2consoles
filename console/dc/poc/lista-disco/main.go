//go:build dreamcast

// DC-002.44: EL LISTADO DE UN DIRECTORIO DEL DISCO, por el puente de `os` (el que usa el motor).
//
// El motor, con los MPQ en el disco, ve CERO archivos en /cd/juego: `d2setup.Complete` (os.Stat por nombre)
// contesta true y `d2setup.Validate` (que LISTA el directorio) ve 0 archivos y 7 faltantes. Este POC mide
// el camino del listado solo, sin el motor arriba, para poder decir en que llamada se pierde:
//
//	LISTA-DISCO entradas= N      <- el numero de os.ReadDir
//	LISTA-DISCO [i] nombre dir= <- cada entrada, tal como la devuelve el puente
//
// Con el disco armado por `mkdisco.sh` (que ahora hornea /JUEGO/MANIFIESTO.TXT con lo que hornea) y sin el
// manifiesto (mutante) el mismo binario tiene que dar 0 entradas: ese es el control negativo.
package main

import (
	"os"
)

func main() {
	println("LISTA-DISCO arranca (DC-002.44: el ReadDir de /cd/juego)")

	for _, ruta := range []string{"/cd/juego", "/cd/JUEGO", "/cd"} {
		entradas, err := os.ReadDir(ruta)

		if err != nil {
			println("LISTA-DISCO", ruta, "ERROR", err.Error())
			continue
		}

		println("LISTA-DISCO", ruta, "entradas=", len(entradas))

		for i, e := range entradas {
			println("LISTA-DISCO  [", i, "]", e.Name(), "dir=", e.IsDir())
		}
	}

	// el Stat por nombre, que es el OTRO camino del motor (Complete) y que ya andaba: se imprime al lado
	// para que se vea que el archivo esta ahi aunque el listado no lo traiga.
	for _, nombre := range []string{"/cd/juego/patch_d2.mpq", "/cd/juego/d2data.mpq", "/cd/juego/d2char.mpq"} {
		if fi, err := os.Stat(nombre); err == nil {
			println("LISTA-DISCO stat", nombre, "bytes=", fi.Size())
		} else {
			println("LISTA-DISCO stat", nombre, "ERROR", err.Error())
		}
	}

	println("LISTA-DISCO fin")

	for {
	}
}
