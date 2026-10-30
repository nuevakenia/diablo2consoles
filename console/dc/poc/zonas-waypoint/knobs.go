package main

// DC-002.77 -- LAS DOS FORMAS DE MEDIR, y por que hacen falta las dos.
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
// De ahi las dos corridas (misma corrida, distinta palanca):
//
//	caminoCache (por defecto)  el camino del motor: Retrieve/Insert en los caches y el crudo se suelta al
//	                           parsear, pero como es un objeto grande NO se recupera. Es el "hoy".
//	sinCacheDt1                el camino IDEAL para medir la causa: lee el .dt1, compone el mapa y LIBERA
//	                           el crudo con `free()` (el mismo `malloc` que uso `gc_external_alloc`), sin
//	                           dejarlo en el cache. Si con esto el pico de las tres fases es el mismo, la
//	                           causa es la liberacion, no el cache.
//
// OJO, y esto es una piedra medida: el .dt1 parseado NO se puede liberar impunemente. Los bloques de cada
// tile son SUBSLICES del archivo crudo (`d2common/d2datautils/stream_reader.go:117`: `ReadBytes` devuelve
// `data[pos:pos+n]`), no copias. Por eso el camino sin cache libera DESPUES de `Alimentar` (componer), que
// es el ultimo momento en que se tocan esos bytes. Un cache que quisiera liberar al desalojar tiene el
// mismo requisito: o copia los bloques al parsear, o libera recien cuando la entrada sale del cache.
const (
	gcEntreArchivos = false
	sinCacheDt1     = false
)
