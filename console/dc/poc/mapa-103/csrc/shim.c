/* DC-002.82: los puentes a C del POC de las tablas y el compositor del motor (copiados del POC de
 * las dos zonas, DC-002.77: son los mismos cuatro instrumentos).
 *
 * 1) LA CONSOLA SERIE. El `println` de Go en libgodc sale por el dispositivo de dbgio que este
 *    seleccionado, y el arranque de KOS no elige ninguno: sin esto la salida del POC no sale a ninguna
 *    parte (medido en DC-001.3). Es el mismo shim de los POC del enlace, del personaje, del estado B y
 *    del cooperativo.
 *
 * 2) EL MONTON, que el binding de Go no expone. Se leen los cuatro numeros de `mallinfo` mas el break del
 *    monton, que es el juego de instrumentos de DC-002.28/51/75:
 *      arena   lo que el malloc tiene PEDIDO del sistema (crece por sbrk a medida que hace falta)
 *      usado   uordblks: lo que esta tomado AHORA -- este es el numero cuyo PICO se compara entre zonas
 *      libre   fordblks: lo libre DENTRO del arena (el numero que imprimia DC-002.51/75)
 *      sbrk    el break del monton, al lado del techo de KOS: dice si el monton toco el techo.
 *    Son el mismo instrumento que el runtime imprime al quedarse sin memoria, asi que los numeros son
 *    comparables con el defecto medido.
 *
 * 3) EL RELOJ. Los contadores de rendimiento del SH-4 (PerfCntr*) devuelven 0 en Flycast (medido en
 *    DC-002.36), asi que el tiempo se mide con el timer de KOS, que si anda en el emulador.
 */
#include <kos.h>
#include <malloc.h>
#include <stdint.h>
#include <stdlib.h>
#include <unistd.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

/* Milisegundos desde el arranque (el timer de KOS, con el reloj encendido la primera vez). */
unsigned int d2c_ms(void) {
    static int encendido = 0;

    if (!encendido) {
        timer_ms_enable();
        encendido = 1;
    }

    return (unsigned int)(timer_ms_gettime64() & 0xffffffffu);
}

/* El break actual del monton (0 si el sistema no lo sabe). */
unsigned int d2c_sbrk_actual(void) {
    return (unsigned int)(uintptr_t)sbrk(0);
}

/* El techo del monton de KOS: `_arch_mem_top` menos la pila del kernel. */
unsigned int d2c_monton_techo(void) {
    return (unsigned int)(_arch_mem_top - THD_KERNEL_STACK_SIZE);
}

int d2c_monton_arena(void) {
    return mallinfo().arena;
}

int d2c_monton_libre(void) {
    return mallinfo().fordblks;
}

int d2c_monton_usado(void) {
    return mallinfo().uordblks;
}

/* d2c_libre: libera un bloque pedido por el runtime para un objeto GRANDE. En libgodc, toda asignacion de
 * mas de `GC_LARGE_OBJECT_THRESHOLD` (64 KB) va a `gc_external_alloc`, que es `malloc` pelado y NO la
 * recolecta nadie (libgodc/runtime/gc_heap.c). El puntero que devuelve es el de malloc, tal cual, asi que
 * el `free` de aca es el que corresponde -- es lo mismo que hace `runtime.FreeExternal`. */
void d2c_libre(void *p) {
    free(p);
}

/* DC-002.78: el recolector FRENADO durante la carga de un .dt1.
 *
 * Por que: el recolector de libgodc es de COPIA (gc_copy.c) y su barrido de la pila es conservador. Medido en
 * esta tarjeta: con 16.266 bloques copiados al semiespacio, el recolector salta por umbral EN MEDIO del
 * parseo (dos veces en los dos primeros archivos) y el SH-4 muere con PC en una direccion de MONTON (fuera
 * del segmento de codigo del ELF) -- o sea que alguien salto a un puntero que el recolector habia movido.
 *
 * `gc_inhibit_collection`/`gc_allow_collection` son las mismas que usa el propio runtime de libgodc para
 * sus secciones criticas (map_dreamcast.c:773, map_fast_internal.h:157, gc_copy.c:233). No son una invencion
 * de este POC: es el freno que el runtime ya se pone a si mismo.
 */
void gc_inhibit_collection(void);
void gc_allow_collection(void);

void d2c_gc_frena(void) {
    gc_inhibit_collection();
}

void d2c_gc_suelta(void) {
    gc_allow_collection();
}

/* ============================== 4) DC-002.105: SOLTAR LO QUE EL CUADRO NO LEE ==============================
 *
 * Dos puentes, uno por forma de bloque. En libgodc todo pedido de mas de `GC_LARGE_OBJECT_THRESHOLD` (64 KB)
 * va a `gc_external_alloc`, que es `malloc` pelado FUERA de los dos semiespacios: el recolector no lo copia,
 * no lo barre y NO lo devuelve nunca. Ese monton es el unico de donde puede salir la cache de tiles del cuadro
 * (DC-002.101/.103), asi que lo que el cuadro no usa se devuelve a mano despues de componer.
 *
 *   d2c_soltar_tramo    un tramo CRUDO y grande (una rebanada de bytes: las tablas del MPQ, la memoria de
 *                       trabajo de la lectura). Devuelve cuantos bytes se devolvieron. Se NIEGA si el tramo cae
 *                       adentro de un semiespacio (ahi el dueno es el recolector y un free() a mano corrompe
 *                       el monton) o si no paso el umbral: las dos cosas son el mismo error.
 *
 *   d2c_soltar_filas    el TABLERO (buckets) de un mapa de Go, que no es una rebanada: gccgo guarda el mapa
 *                       como puntero a un `GoMap` de 28 B (`map_dreamcast.h`, con sus `_Static_assert` del
 *                       layout) y el puntero a los buckets no esta en ningun lado visible desde Go. Se leen
 *                       `buckets` y `oldbuckets` y se devuelve cada uno SOLO SI esta en la tabla de externos
 *                       del runtime (`gc_externos`): esa tabla es la PRUEBA de que el runtime lo mando a
 *                       malloc, un tablero del semiespacio no esta ahi y no se toca. La misma tabla tiene el
 *                       TAMANO del bloque, asi que los bytes que se informan son los del bloque de verdad --
 *                       no una cuenta que el que llama suponga. Ademas se exige que el propio `GoMap` caiga
 *                       adentro de un semiespacio: un puntero de mapa que no cumpla eso no es un mapa.
 *
 * Medido en DC-002.105: el `LevelPresets` del pueblo son DOS tableros (108.032 B del crecimiento y 216.064 B
 * del mapa vivo) y las tablas de `d2data.mpq` son 354.656 B (177.328 de entradas de hash + 177.328 de tabla
 * de bloque).
 */
#include "gc_semispace.h"
#include "map_dreamcast.h"

/* la unica de gc_heap.c que no esta en un header (la usa stack.c contra las pilas de goroutine) */
bool gc_solapa_semiespacio(const void *lo, size_t n);

int d2c_soltar_tramo(void *tramo, unsigned tamano) {
    if (tramo == NULL || tamano <= (unsigned)GC_LARGE_OBJECT_THRESHOLD)
        return 0;

    if (gc_solapa_semiespacio(tramo, tamano))
        return 0;

    free(tramo);

    return (int)tamano;
}

int d2c_soltar_filas(void *pMapa) {
    if (pMapa == NULL)
        return 0;

    GoMap *h = *(GoMap **)pMapa;

    if (h == NULL)
        return 0;

    /* el GoMap lo pidio `makemap` por `gc_alloc` con 28 B: vive en un semiespacio. Si no cae ahi, esto no es
     * un mapa y no hay nada que soltar (es la guarda que evita escribirle a memoria ajena). */
    if (!gc_solapa_semiespacio(h, sizeof(GoMap)))
        return 0;

    int bytes = 0;
    void *tableros[2] = {h->buckets, h->oldbuckets};

    for (int i = 0; i < 2; i++) {
        if (tableros[i] == NULL)
            continue;

        for (int j = 0; j < gc_externos_count; j++) {
            if (gc_externos[j].ptr != tableros[i])
                continue;

            bytes += (int)gc_externos[j].size;
            gc_external_free(tableros[i]);   /* saca el bloque de la tabla y le hace free */
            break;
        }
    }

    /* el mapa queda como un mapa VACIO, no como un puntero a memoria devuelta: si alguien lo usa despues de
     * soltarlo, ve cero filas (y el recolector no sigue un puntero que ya no existe) */
    h->count = 0;
    h->B = 0;
    h->buckets = NULL;
    h->oldbuckets = NULL;

    return bytes;
}
