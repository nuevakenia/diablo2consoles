/* DC-002.98: los puentes a C del POC del .dcc en la arena.
 *
 * 1) LA CONSOLA SERIE. El `println` de Go en libgodc sale por el dispositivo de dbgio que este seleccionado,
 *    y el arranque de KOS no elige ninguno: sin esto la salida del POC no sale a ninguna parte (DC-001.3).
 *
 * 2) EL MONTON DE C (`mallinfo`): `arena` / `uordblks` (usado) / `fordblks` (libre). En esta tarjeta el
 *    numero que importa es `usado`: la arena del decodificador pide tramos de mas de 64 KB, o sea `malloc`
 *    pelado fuera de los semiespacios del recolector, y al liberarlos el `usado` tiene que VOLVER.
 *
 * 3) EL `free` DEL BLOQUE GRANDE (`d2c_libre`). En libgodc toda asignacion de mas de
 *    `GC_LARGE_OBJECT_THRESHOLD` (64 KB) va a `gc_external_alloc` (malloc pelado) y no la recolecta nadie
 *    (libgodc/runtime/gc_heap.c). El puntero es el de malloc tal cual, asi que el `free` de aca es el que
 *    corresponde -- es lo mismo que hace `runtime.FreeExternal`.
 *
 * 4) EL SEMIESPACIO DEL RECOLECTOR (`gc_stats`, runtime/gc_heap.c:328): `used` (lo tomado del semiespacio
 *    activo), `total` (el tamano de UN semiespacio, 2 MB en la consola) y `collections` (el contador del
 *    recolector). Es el instrumento con el que se cuenta el defecto: 45 recolecciones seguidas y muerte.
 *
 * 5) EL RELOJ (el timer de KOS; los contadores del SH-4 dan 0 en Flycast, DC-002.36).
 */
#include <kos.h>
#include <malloc.h>
#include <stdint.h>
#include <stddef.h>
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

/* d2c_libre: devuelve al monton un bloque que el runtime pidio para un objeto GRANDE (> 64 KB). */
void d2c_libre(void *p) {
    free(p);
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

/* gc_stats: el semiespacio del recolector de libgodc y su contador de recolecciones. OJO con los tipos:
 * `used` es una cuenta (size_t), no un puntero, y el valor no entra en un int de 32 bits con signo. */
extern void gc_stats(size_t *usado, size_t *total, uint32_t *colecciones);

static size_t gc_usado, gc_total;
static uint32_t gc_colecciones;

unsigned int d2c_gc_n(void) {
    gc_stats(&gc_usado, &gc_total, &gc_colecciones);

    return (unsigned int)gc_colecciones;
}

unsigned int d2c_gc_heap_usado(void) {
    gc_stats(&gc_usado, &gc_total, &gc_colecciones);

    return (unsigned int)gc_usado;
}

unsigned int d2c_gc_heap_total(void) {
    gc_stats(&gc_usado, &gc_total, &gc_colecciones);

    return (unsigned int)gc_total;
}
