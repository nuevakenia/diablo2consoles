//go:build dreamcast

// DC-002.45: EL LISTADO DE UN DIRECTORIO DEL DISCO POR EL CAMINO DIRECTO (`fs_readdir` del driver).
//
// DC-002.44 midio que el driver de KOS SI sabe listar (seccion 1-BIS de su evidencia, POC de C disco-c) y
// que el que no listaba era el PUENTE de `os`: `os.ReadDir("/cd/juego")` devolvia lista vacia con error nil
// porque solo miraba el `.manifiesto` del romdisk y el manifiesto del overlay de la RAM. El arreglo de esa
// tarjeta fue el MANIFIESTO que hornea `mkdisco.sh` en `/JUEGO/MANIFIESTO.TXT`, o sea la lista del
// HORNEADOR: un archivo que este en el disco y que el horneador no conozca NO SALE.
//
// Este POC mide el camino de fondo de esta tarjeta:
//
//	LISTA-DISCO <ruta> entradas= N                     <- el numero de os.ReadDir
//	LISTA-DISCO  [ i ] <nombre> dir= <b> bytes= <n>    <- nombre, tipo y TAMANO de cada entrada
//	LISTA-DISCO <ruta> ERROR <err>                     <- si el directorio no esta
//	LISTA-DISCO memoria ...                            <- el monton de C y el semiespacio del GC (criterio 5)
//
// EL BUFFER ES LOCAL: `os.ReadDir` arma el listado del disco en una pila de tamano fijo (`var bufISO`), no
// en el monton. Y eso se MIDE, no se promete: la memoria se imprime antes y despues de 40 vueltas del
// listado sobre el mismo directorio. Si el puente reservara del monton, 40 vueltas lo moverian.
package main

import (
	"os"
	"unsafe"
)

//extern d2c_marca_carga
func marcaCarga(punto int, ruta uintptr, largo int)

// memoria imprime el estado de la memoria. El punto 0 del instrumento es "instrumento", que es
// exactamente lo que es aca: una marca de control, no una carga del motor.
func memoria(etiqueta string) {
	var ruta [64]byte
	n := len(etiqueta)
	if n > len(ruta)-1 {
		n = len(ruta) - 1
	}
	for i := 0; i < n; i++ {
		ruta[i] = etiqueta[i]
	}
	marcaCarga(0, uintptr(unsafe.Pointer(&ruta[0])), n)
}

func main() {
	println("LISTA-DISCO arranca (DC-002.45: el ReadDir del disco por el fs_readdir del driver)")

	// (1) el listado de /cd/juego: el numero, y CADA entrada con su TAMANO. Los bytes importan: el
	// manifiesto del horneador los traia, y el driver los trae de `de->size`. Un 0 donde el archivo tiene
	// bytes seria un listado a medias.
	for _, ruta := range []string{"/cd/juego", "/cd/JUEGO", "/cd"} {
		entradas, err := os.ReadDir(ruta)

		if err != nil {
			println("LISTA-DISCO", ruta, "ERROR", err.Error())
			continue
		}

		println("LISTA-DISCO", ruta, "entradas=", len(entradas))

		for i, e := range entradas {
			tam := int64(-1)
			if fi, ferr := e.Info(); ferr == nil {
				tam = fi.Size()
			}
			println("LISTA-DISCO  [", i, "]", e.Name(), "dir=", e.IsDir(), "bytes=", tam)
		}
	}

	// (2) el Stat por nombre, que es el OTRO camino del motor (Complete) y que ya andaba: se imprime al lado
	// para que se vea que el archivo esta ahi aunque el listado no lo traiga.
	for _, nombre := range []string{"/cd/juego/patch_d2.mpq", "/cd/juego/d2data.mpq", "/cd/juego/d2char.mpq"} {
		if fi, err := os.Stat(nombre); err == nil {
			println("LISTA-DISCO stat", nombre, "bytes=", fi.Size())
		} else {
			println("LISTA-DISCO stat", nombre, "ERROR", err.Error())
		}
	}

	// (3) el manifiesto del horneador, LEIDO como archivo (no como listado): es la lista que el horneador
	// dejo, y sirve para ver que la marca suelta NO esta ahi. Si estuviera, el control del criterio 3 no
	// probaria nada: los dos caminos darian lo mismo.
	if datos, err := os.ReadFile("/cd/juego/MANIFIESTO.TXT"); err == nil {
		println("LISTA-DISCO manifiesto bytes=", len(datos), ":", string(datos))
	} else {
		println("LISTA-DISCO manifiesto ERROR", err.Error())
	}

	// (4) LA MEMORIA (criterio 5): 40 vueltas del listado del disco. El buffer es LOCAL y de tamano fijo;
	// si el puente reservara del monton, aca se veria.
	memoria("antes-de-40-listados")
	for i := 0; i < 40; i++ {
		_, _ = os.ReadDir("/cd/juego")
	}
	memoria("despues-de-40-listados")

	println("LISTA-DISCO fin")

	for {
	}
}
