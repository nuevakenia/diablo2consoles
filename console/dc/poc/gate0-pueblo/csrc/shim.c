/* DC-002.155: los puentes a C del POC del GATE 0 (el pueblo por el arranque del motor).
 *
 * Son los instrumentos del resto de los POC de la consola, recortados a lo que este arranque usa:
 *
 * 1) LA CONSOLA SERIE. El `println` de Go en libgodc sale por el dispositivo de dbgio que este elegido, y el
 *    arranque de KOS no elige ninguno: sin esto la salida no sale a ninguna parte (DC-001.3).
 * 2) EL RELOJ de KOS (los contadores del SH-4 dan 0 en Flycast, DC-002.36).
 * 3) EL MONTON: los numeros de `mallinfo` (el instrumento de DC-002.28/51/75) y el break.
 * 4) LA SONDA del pedazo que el monton puede servir DE UNA VEZ (DC-002.106): es la que dimensiona el arena de
 *    bloques de los `.dt1` cuando el tamano fijo no entra.
 * 5) EL SEMIESPACIO del recolector (`gc_stats`, DC-002.109): lo que ocupa, cuanto mide y cuantas recolecciones.
 *    `runtime.ReadMemStats` no sirve en libgodc (Alloc = 0, DC-002.9).
 * 6) EL UMBRAL del recolector (`debug.SetGCPercent`, DC-002.109).
 * 7) QUIEN PIDE los bloques grandes (DC-002.136), con `-Wl,--wrap=malloc`: cada pedido de 64 KB o mas imprime
 *    su tamano, lo que quedaba libre antes y la direccion de retorno (que se resuelve con sh-elf-addr2line
 *    sobre prog.elf). Es el instrumento que convierte "murio por OOM" en "murio este pedido, de esta funcion".
 *
 * OJO: `#include "gc_semispace.h"` NO va. El umbral del objeto grande es el mismo numero que usa el runtime
 * (65536) y se escribe aca con el nombre que tiene en libgodc, para no depender de una ruta de include que este
 * script no pasa.
 */
#include <kos.h>
#include <malloc.h>
#include <stdint.h>
#include <stdlib.h>
#include <unistd.h>
#include <reent.h>

/* ============================== 1) la consola serie ============================== */

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

/* ============================== 2) el reloj ============================== */

unsigned int d2c_ms(void) {
    static int encendido = 0;

    if (!encendido) {
        timer_ms_enable();
        encendido = 1;
    }

    return (unsigned int)(timer_ms_gettime64() & 0xffffffffu);
}

/* ============================== 3) el monton de C ============================== */

unsigned int d2c_sbrk_actual(void) {
    return (unsigned int)(uintptr_t)sbrk(0);
}

/* El techo del monton de KOS: `_arch_mem_top` menos la pila del kernel. */
unsigned int d2c_monton_techo(void) {
    return (unsigned int)(_arch_mem_top - THD_KERNEL_STACK_SIZE);
}

/* DC-002.155: LO QUE DE VERDAD QUEDA PARA PEDIR. `fordblks` (la lista de libres) NO es eso: al arrancar da
 * 1,6 KB aunque el monton tenga megabytes por delante, porque lo que queda es BREAK ROOM -- el espacio entre
 * el break y el techo, que dlmalloc usa cuando no hay hueco. Este numero (techo menos break) es el que dice si
 * una estructura entra o no, y es el que va al lado de `fordblks` en cada etapa. */
int d2c_break_libre(void) {
    unsigned int techo = (unsigned int)(_arch_mem_top - THD_KERNEL_STACK_SIZE);
    unsigned int breakActual = (unsigned int)(uintptr_t)sbrk(0);

    return (int)(techo - breakActual);
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

/* d2c_libre: libera un bloque pedido por el runtime para un objeto GRANDE (mas de 64 KB, o sea
 * `gc_external_alloc`, que es `malloc` pelado y no lo recolecta nadie). Es el `free` pelado, el mismo que
 * `runtime.FreeExternal` pero sin sacarlo de la tabla de externos: para lo que pide el ASSETMANAGER por su
 * cuenta (las tablas del MPQ, DC-002.105) es el que corresponde. */
void d2c_libre(void *p) {
    free(p);
}

/* ============================== 4) la sonda del monton (DC-002.106) ============================== */

/* El umbral del objeto grande del recolector de libgodc (`GC_LARGE_OBJECT_THRESHOLD`, gc_heap.h). Arriba de
 * esto, una asignacion de Go NO va al semiespacio: va a `malloc`. La sonda tiene que pedir MAS que esto o no
 * compra nada. */
#define UMBRAL_DEL_OBJETO_GRANDE 65536

unsigned int d2c_servible(unsigned int tope) {
    if (tope > (1u << 30))
        return 0;

    for (unsigned int t = tope; t > (unsigned)UMBRAL_DEL_OBJETO_GRANDE; t = (t / 2) & ~511u) {
        void *p = malloc(t);

        if (p != NULL) {
            free(p);

            return t;
        }
    }

    return 0;
}

/* ============================== 5) el semiespacio del recolector ============================== */

/* los tipos son los de libgodc: `used`/`total` son size_t y el contador es uint32_t (con los tipos cambiados
 * el numero sale basura sin avisar, medido en DC-002.100) */
void gc_stats(size_t *used, size_t *total, uint32_t *collections);

unsigned int d2c_gc_usado(void) {
    size_t usado = 0;

    gc_stats(&usado, NULL, NULL);

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

/* ============================== 6) el umbral del recolector (DC-002.109) ============================== */

int32_t debug_SetGCPercent(int32_t percent) __asm__("debug.SetGCPercent");

int d2c_gc_percent(int nuevo) {
    return (int)debug_SetGCPercent((int32_t)nuevo);
}

/* ============================== 7) quien pide los bloques grandes (DC-002.136) ============================== */

static int instrumentoDeBloques = 1;

void d2c_bloques_instrumento(int encendido) {
    instrumentoDeBloques = encendido;
}

/* ============================== 8) DONDE ESTA EL SEMIESPACIO VIVO (DC-002.159n) ==============================
 *
 * El recolector de libgodc COPIA: despues de cada recoleccion, lo que sigue vivo esta en la OTRA mitad y la
 * mitad vieja pasa a ser el area de reparto (o sea, se reusa). Un puntero que se quedo en la mitad vieja no
 * da un error: lee basura. Eso es lo que mata a `colorToColorM` (la tabla de metodos del color sale de
 * memoria prestada y el `jsr` se va a cualquier lado).
 *
 * Esta funcion le pasa a la sonda de Go los cinco numeros que hacen falta para decirlo: donde empieza la mitad
 * VIVA, donde esta el puntero de reparto (lo vivo esta entre esos dos), donde termina, donde empieza la otra
 * mitad (la que se reusa, para poder decir "esa direccion es de la mitad vieja") y el tope de la imagen.
 *
 * La estructura `gc_heap` la declara libgodc; `gc_imagen_hi` lo pone `gc_init()` (arriba de la imagen es
 * monton). El build pasa -I"$LIBGODC/runtime" (build-dc.sh), asi que el include resuelve sin copiar nada.
 */
#include "gc_semispace.h"

extern uintptr_t gc_imagen_hi;

void d2c_semi_rango(unsigned int *v) {
    if (v == NULL) {
        return;
    }

    v[0] = (unsigned int)(uintptr_t)gc_heap.space[gc_heap.active_space];
    v[1] = (unsigned int)(uintptr_t)gc_heap.alloc_ptr;
    v[2] = (unsigned int)(uintptr_t)gc_heap.alloc_limit;
    v[3] = (unsigned int)(uintptr_t)gc_heap.space[1 - gc_heap.active_space];
    v[4] = (unsigned int)(uintptr_t)gc_imagen_hi;
    v[5] = (unsigned int)gc_heap.gc_count;
    v[6] = (unsigned int)gc_heap.space_size;
}

void *__real_malloc(size_t n);

void *__wrap_malloc(size_t n) {
    struct mallinfo antes;
    void *p;

    if (!instrumentoDeBloques || n < UMBRAL_DEL_OBJETO_GRANDE) {
        return __real_malloc(n);
    }

    antes = mallinfo();

    p = __real_malloc(n);

    dbgio_printf("GATE0: PIDE %d libreAntes=%d tomadoAntes=%d ra=%08x quedo=%08x\n",
                 (int)n, (int)antes.fordblks, (int)antes.uordblks,
                 (unsigned int)(uintptr_t)__builtin_return_address(0), (unsigned int)(uintptr_t)p);

    return p;
}
