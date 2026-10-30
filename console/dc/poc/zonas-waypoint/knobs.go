package main

// DC-002.78 -- LAS TRES FORMAS DE MEDIR, y por que hacen falta las tres.
//
// EL PROBLEMA QUE SEPARA ESTE ARCHIVO. El "pico de monton" de una carga de zona tiene DOS causas que se
// suman y que hay que separar para no echarle la culpa a la equivocada:
//
//  1. EL CACHE: lo que el motor se guarda a proposito (los .dt1 y el .ds1 vivos). Su tamano lo acota
//     `dt1Budget`/`ds1Budget` y se ve en `Cache.GetWeight()` y en `Cache.Evictions()`.
//  2. LO QUE EL RUNTIME NO DEVUELVE. Medido con `console/dc/poc/sonda-gc-277` en Flycast: en esta consola
//     toda asignacion de mas de 64 KB (`GC_LARGE_OBJECT_THRESHOLD`) se va al monton EXTERNO
//     (`gc_external_alloc` = `malloc` pelado, libgodc/runtime/gc_heap.c:116) y no la recolecta nadie: 12
//     trozos de 256 KB, tirados uno detras de otro, subieron el monton EXACTAMENTE 12 x 262.144 B, con y
//     sin `runtime.GC()`. Cada .dt1 del acto 1 mide entre 300 KB y 1 MB, o sea que cada archivo que se lee
//     queda tomado PARA SIEMPRE.
//
// De ahi las tres palancas (misma corrida, distinta palanca):
//
//	porBuffer (DC-002.78)      LA CURA. Los .dt1 los carga el MOTOR por su ruta de consola
//	                           (`am.LoadDT1` -> `d2asset/lectura_de_tiles.go`): un buffer UNICO que se
//	                           reusa entre archivos + copia de los bloques al parsear
//	                           (`d2dt1.LoadDT1CopiaBloques`), que asi quedan en el semiespacio del
//	                           recolector y no en el monton externo. Es lo que mide esta tarjeta.
//	sinCacheDt1                el camino IDEAL de DC-002.77: lee el .dt1, compone el mapa y LIBERA el crudo
//	                           con `free()` (el mismo `malloc` que uso `gc_external_alloc`), sin dejarlo en
//	                           el cache. Queda como control: con esto la (c) moria por fragmentacion.
//	gcEntreArchivos            pide `runtime.GC()` despues de cada archivo. En DC-002.77 dio IDENTICO al
//	                           camino de hoy: el recolector no devuelve nada de esto.
//
// OJO, y esto es una piedra medida: el .dt1 parseado NO se puede liberar impunemente. Los bloques de cada
// tile son SUBSLICES del archivo crudo (`d2common/d2datautils/stream_reader.go:117`: `ReadBytes` devuelve
// `data[pos:pos+n]`), no copias. Con `sinCacheDt1` se libera DESPUES de `Alimentar` (componer), que es el
// ultimo momento en que se tocan esos bytes. Con `porBuffer` los bloques apuntan al ARENA del motor
// (`d2asset/lectura_de_tiles.go`), que tampoco se libera: se reusa por cursor y se tira entero cuando no
// entra un archivo (y con el, el cache de dt1s).
//
// `gcInhibido` es la sonda de la causa: con el recolector FRENADO (el mismo freno que el runtime de libgodc
// se pone en sus secciones criticas) se ve si la muerte del camino nuevo es del recolector o del parseo.
const (
	porBuffer = false
	// gcInhibido: frena el recolector durante la carga de cada .dt1 (ver csrc/shim.c)
	gcInhibido      = false
	gcEntreArchivos = false
	sinCacheDt1     = false
	// leerConPrestamo (DC-002.80): lee cada .dt1 con la memoria de trabajo PRESTADA (d2mpq.ReadFileInto) en
	// vez de por el stream del MPQ. Apagada queda el camino de DC-002.78 -- el que se midio EN CONTRA -- para
	// poder correr las dos cosas con el mismo binario y el mismo arena.
	//
	// POR QUE ESTA PALANCA EXISTE. En DC-002.78 se midio que el DESTINO de la lectura ya no pedia memoria
	// (el arena) y que la LECTURA si: con el recolector frenado, el semiespacio de 2 MB se llenaba durante la
	// carga del PRIMER .dt1 y el runtime moria pidiendo 3.328 B. El prestamo es lo que saca a la lectura del
	// semiespacio; apagar esta palanca deja exactamente la corrida del .78 para comparar.
	leerConPrestamo = true
)
