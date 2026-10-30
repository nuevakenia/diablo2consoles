/* DC-002.77: los puentes a C del POC de las dos zonas y el waypoint.
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
