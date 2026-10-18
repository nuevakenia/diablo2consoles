/* DC-002.28: los instrumentos del MONTON de C del POC motor-reservas.
 *
 * Por que hacen falta: hasta DC-002.27 la sonda imprimia `mallinfo().fordblks` y eso se leyo como "la RAM
 * que queda". NO es eso: `fordblks` es lo libre DENTRO del arena que el `malloc` ya pidio, y el arena
 * crece por `sbrk` a medida que hace falta (medido en DC-002.22: pedir 1,2 MB costo 200 bytes de
 * `fordblks`). Y la reserva que revienta no es "mas bytes que libres", es "un bloque CONTIGUO de N bytes
 * cuando no hay ninguno tan grande": el mapa del motor, por ejemplo, se lleva su tabla de \
 * cubetas en UNA sola asignacion.
 *
 * De ahi los tres numeros:
 *
 *   d2c_sbrk_actual  el break del monton (a donde llego `sbrk`). Con el `_arch_mem_top` de KOS al lado
 *                    dice si todavia queda RAM o si el monton ya toco el techo.
 *   d2c_monton_arena lo que el malloc tiene pedido del sistema (`mallinfo().arena`).
 *   d2c_bloque_max   EL NUMERO QUE IMPORTA: el bloque CONTIGUO mas grande que el monton acepta AHORA,
 *                    medido pidiendolo y devolviendolo (busqueda binaria). Es lo que contesta "entra esta
 *                    reserva?" sin suponer nada sobre fragmentacion.
 *
 * El `bloque_max` pide y libera en el acto: no deja nada tomado, pero SI puede mover el arena un poco
 * (el pedido de la busqueda es el que el malloc necesita para encontrar el bloque). Por eso se mide en los
 * mismos puntos de las dos corridas (antes y despues de cada carga) y no en medio de una.
 */
#include <kos.h>
#include <malloc.h>
#include <stdint.h>
#include <stdlib.h>
#include <unistd.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

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

/* El bloque CONTIGUO mas grande que el monton acepta ahora, en bytes (busqueda binaria sobre malloc/free,
 * precision 1 KB). El pedido se libera en el acto: no queda tomado. */
unsigned int d2c_bloque_max(void) {
    unsigned int lo = 0;
    unsigned int hi = 16u << 20;

    while (lo + 1024 < hi) {
        unsigned int medio = lo + (hi - lo) / 2;
        void *p = malloc(medio);

        if (p) {
            free(p);
            lo = medio;
        } else {
            hi = medio;
        }
    }

    return lo;
}
