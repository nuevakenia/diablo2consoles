/* DC-002.96: los puentes a C del POC de la Source de consola.
 *
 * 1) LA CONSOLA SERIE. El `println` de Go en libgodc sale por el dispositivo de dbgio que este seleccionado,
 *    y el arranque de KOS no elige ninguno: sin esto la salida del POC no sale a ninguna parte (medido en
 *    DC-001.3). Es el mismo shim de los POC del enlace, del personaje y de las zonas.
 *
 * 2) EL MONTON, que el binding de Go no expone: `mallinfo` (arena / uordblks / fordblks) mas el break del
 *    monton, que es el juego de instrumentos de DC-002.28/51/75/77. `uordblks` (usado) es EL numero de esta
 *    tarjeta: dice si el tramo que la fuente presto volvio al monton o se quedo tomado.
 *
 * 3) EL `free` DEL BLOQUE GRANDE (`d2c_libre`). En libgodc toda asignacion de mas de `GC_LARGE_OBJECT_THRESHOLD`
 *    (64 KB) va a `gc_external_alloc`, que es `malloc` pelado y NO la recolecta nadie
 *    (libgodc/runtime/gc_heap.c). El puntero que devuelve es el de malloc, tal cual, asi que el `free` de aca
 *    es el que corresponde -- es lo mismo que hace `runtime.FreeExternal`. Es el liberador que el POC le
 *    inyecta a la fuente (`Fuente.UsarLiberador`): sin el, el tramo se queda tomado.
 *
 * 4) EL RELOJ. Los contadores de rendimiento del SH-4 devuelven 0 en Flycast (medido en DC-002.36), asi que
 *    el tiempo se mide con el timer de KOS.
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

/* d2c_libre: devuelve al monton un bloque que el runtime pidio para un objeto GRANDE (> 64 KB). */
void d2c_libre(void *p) {
    free(p);
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
