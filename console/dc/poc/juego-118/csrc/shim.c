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

/* ============================== 5) DC-002.106: EL PEDAZO QUE EL MONTON PUEDE SERVIR ==============================
 *
 * POR QUE. El tramo de la cache pedido en pedazos de tamano FIJO (DC-002.106, `tramoEnPedazos`) se corto en el
 * ultimo bloque medido en Flycast: con 7 bloques de 348.160 B puestos, al monton le quedaban 249.440 B libres
 * (en migas, repartidas) y 159.744 B de break room, y el ultimo bloque (286.720 B, lo que faltaba del techo)
 * murio en `gc_external_alloc` porque NI un hueco ni el break alcanzaban para 286.720 DE UNA VEZ -- pero la
 * memoria total SI estaba (409.184 B). dlmalloc no cose un pedido con trozos.
 *
 * QUE HACE. Le pregunta al monton, en C (donde un `malloc` que falla devuelve NULL en vez de tirar el runtime),
 * cual es el bloque mas grande que puede servir AHORA sin pasarse de `tope`. Baja de a mitades hasta que el
 * pedido entra. Cada sonda se libera en el acto, asi que no cuesta memoria: lo que vino de un hueco se vuelve a
 * fundir con el, y lo que vino del top se queda como top (que es de donde saldria el pedido de verdad).
 *
 * OJO CON EL ORDEN. Esto NO elige un tamano "comodo": dice el MAXIMO que entra. El que llama decide si ese
 * tamano le sirve (tiene que pasar los 64 KB del objeto grande, si no la asignacion se iria al semiespacio del
 * recolector y no compraria nada).
 */
unsigned int d2c_servible(unsigned int tope) {
    if (tope > (1u << 30))
        return 0;

    for (unsigned int t = tope; t > (unsigned)GC_LARGE_OBJECT_THRESHOLD; t = (t / 2) & ~511u) {
        void *p = malloc(t);

        if (p != NULL) {
            free(p);

            return t;
        }
    }

    return 0;
}

/* ============================== 6) DC-002.109: SOLTAR UN BLOQUE DE LA CACHE ==============================
 *
 * POR QUE. Las tres fases (entrar / transicion / volver) piden reabrir las dos fuentes MPQ y recargar las
 * tablas -- 570.720 B -- y despues COMPONER el nivel nuevo, que son ~3,63 MB de arena de bloques + buffer
 * prestado. Con el tramo de la cache tomado (2.723.840 B que el recolector no devuelve) el monton de C no
 * llega. La decision "a" de la tarjeta es soltar la cache del nivel que se deja ANTES de reabrir: estos
 * bloques son pedazos de un tramo, o sea rebanadas de Go que fueron a `gc_external_alloc` por tamano.
 *
 * POR QUE `gc_external_free` Y NO `free` PELADO. El bloque puede estar en la tabla de externos del runtime
 * (`gc_externos`, la que se barre en cada recoleccion desde DC-002.83). `gc_external_free` lo SACA de la tabla
 * antes de liberarlo; un `free` suelto deja una entrada con un puntero a memoria devuelta que despues se barre.
 * `d2c_soltar_tramo` (arriba) existe para los tramos que NO son de Go (las tablas del MPQ, que se piden en C).
 *
 * LAS DOS GUARDAS SON LAS MISMAS DE `d2c_soltar_tramo`: si el bloque no paso el umbral del objeto grande o cae
 * adentro de un semiespacio, su dueno es el recolector y no se toca (se devuelve 0 y el tramo lo reusa).
 */
int d2c_soltar_bloque(void *p, unsigned tamano) {
    if (p == NULL || tamano <= (unsigned)GC_LARGE_OBJECT_THRESHOLD)
        return 0;

    if (gc_solapa_semiespacio(p, tamano))
        return 0;

    gc_external_free(p);

    return (int)tamano;
}

/* ============================== 7) DC-002.109: EL SEMIESPACIO DEL RECOLECTOR ==============================
 *
 * POR QUE. La primera corrida del ciclo de las tres fases murio en la TRANSICION con el monton de C sano
 * (3,58 MB libres) y el log lleno de `ARQ-MARCA gc_collect`: el que se quedo sin lugar fue el SEMIESPACIO del
 * recolector (1.835.008 B, y a partir del 75 % recolecta en CADA asignacion). `runtime.ReadMemStats` no sirve
 * en libgodc (`ms.Alloc` = 0, medido en DC-002.9); `gc_stats` si, y da los tres numeros que importan: lo que
 * ocupa el semiespacio activo, cuanto mide y cuantas recolecciones hubo. Es el instrumento que separa "no hay
 * memoria" de "no hay memoria DONDE se pide" -- las dos paredes que ya midieron DC-002.101/.103, ahora en el
 * cambio de nivel.
 */
void gc_stats(size_t *used, size_t *total, uint32_t *collections);

unsigned int d2c_gc_usado(void) {
    size_t usado = 0;
    uint32_t colecciones = 0;

    gc_stats(&usado, NULL, &colecciones);

    return (unsigned int)usado;
}

unsigned int d2c_gc_total(void) {
    size_t total = 0;

    gc_stats(NULL, &total, NULL);

    return (unsigned int)total;
}

unsigned int d2c_gc_colecciones(void) {
    uint32_t colecciones = 0;

    gc_stats(NULL, NULL, &colecciones);

    return (unsigned int)colecciones;
}

/* d2c_gc_percent: el umbral al que el recolector de libgodc salta (el `debug.SetGCPercent` del runtime, que
 * ya existe en libgodc -- gc_runtime.c: 100 = recolecta cuando el semiespacio esta lleno, 50 = a la mitad,
 * negativo = solo con runtime.GC()). Devuelve el valor anterior.
 *
 * POR QUE HACE FALTA (medido en DC-002.109). El default del runtime en el POC es 100, y el codigo de
 * `gc_heap.c` lo traduce a "recolecta a partir del 75 % del semiespacio" (1.376.256 de 1.835.008). Un nivel
 * compuesto deja ~1,17 MB VIVOS en el semiespacio; el `.ds1` del nivel siguiente agrega ~255 KB mientras se
 * parsea, o sea 1,43 MB: por arriba del 75 %. A partir de ahi el recolector corre en CADA asignacion y no
 * libera nada (el conjunto vivo no baja del umbral): medido en Flycast, 1.585 recolecciones seguidas y la
 * composicion sin avanzar. No es que falte memoria -- sobran 400 KB --: es el umbral.
 */
int32_t debug_SetGCPercent(int32_t percent) __asm__("debug.SetGCPercent");

int d2c_gc_percent(int nuevo) {
    return (int)debug_SetGCPercent((int32_t)nuevo);
}

/* ============================== 8) DC-002.112: EL JUGADOR EN EL MISMO CUADRO QUE EL PUEBLO ==============================
 *
 * Tres puentes mas, los del POC del personaje (`console/dc/poc/personaje-mapa/csrc/cshim.c`), que es donde estan
 * medidos y explicados:
 *
 *   d2c_free_external  devuelve al monton de C un bloque que el runtime mando a `gc_external_alloc` (> 64 KB).
 *                      Es la diferencia con `d2c_libre` de arriba: ese es un `free` PELADO y deja la entrada en la
 *                      tabla de externos del runtime; `runtime_FreeExternal` la saca y despues libera. El arena
 *                      del decodificador del .dcc (DC-002.100) son tramos de `make` de 256 KB, o sea bloques de GO:
 *                      por eso el jugador usa este y no el otro.
 *
 *   d2c_gc_n           las recolecciones de libgodc (`gc_stats`). El criterio del jugador es que la decodificacion
 *                      del .dcc cueste CERO recolecciones (DC-002.98/.100): este contador es el que lo prueba.
 *
 *   d2c_heap_libre     `fordblks` (lo libre dentro del arena). Es el numero con el que el runtime cuenta lo que le
 *                      queda cuando se queda sin memoria, asi que el "antes/despues del jugador" se lee con el.
 *
 * OJO CON LOS TIPOS: `used`/`total` de `gc_stats` son size_t y el contador es uint32_t (medido en DC-002.100: con
 * los tipos cambiados el numero sale basura sin avisar).
 */
void runtime_FreeExternal(void *ptr) __asm__("_runtime.FreeExternal");

void d2c_free_external(void *ptr) {
    if (ptr)
        runtime_FreeExternal(ptr);
}

unsigned int d2c_gc_n(void) {
    size_t usado = 0, total = 0;
    uint32_t colecciones = 0;

    gc_stats(&usado, &total, &colecciones);

    return (unsigned int)colecciones;
}

unsigned int d2c_heap_libre(void) {
    return (unsigned int)mallinfo().fordblks;
}
