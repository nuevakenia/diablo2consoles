/* DC-002.32: el C del POC sonda-peso-232.
 *
 * Trae, sin cambiarle una linea, los DOS instrumentos que la tarjeta pide por nombre:
 *
 *   - el del arranque (DC-002.13/27/28): la consola serie y las tres marcas de avance del POC, con
 *     dbgio_write_buffer_xlat para que no dependan del buffering de la libc de KOS (si el motor se cuelga
 *     en la linea siguiente, la marca ya tiene que estar fuera).
 *   - el del MONTON de C (DC-002.28, POC motor-reservas/csrc/cshim.c): `sbrk`, `arena`, `fordblks` y
 *     `bloqueMax` (el bloque CONTIGUO mas grande, medido pidiendolo y devolviendolo). `fordblks` NO es "la
 *     RAM que queda" (el arena crece por sbrk), asi que el numero que dice lo que cuesta una reserva es el
 *     SALTO de `sbrk` y el de `arena`, no el de `libre`.
 *
 * Lo que agrega esta tarjeta: `d2c_reserva` / `d2c_reserva_libre`, que piden N bytes del monton (tocando
 * las dos puntas para que el sistema realmente los entregue) y los liberan. Es lo que mide CUANTO CUESTA,
 * en el monton de C, tener la tabla de d2records en RAM: el mismo numero que la tabla ocupa hoy adentro del
 * `.data` del binario (1.672.892 bytes, medidos con `nm` sobre el ELF enlazado).
 */
#include <kos.h>
#include <malloc.h>
#include <stdint.h>
#include <stdlib.h>
#include <unistd.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

/* d2c_arranque_marca: deja escrito en la serial por que punto del arranque va el POC. */
unsigned int d2c_arranque_marca(unsigned int que) {
    static const char *marcas[] = {
        "",
        "MOTOR-ARRANQUE marca 1: entrando a d2app.Create()\r\n",
        "MOTOR-ARRANQUE marca 2: Create() volvio, entrando a Run()\r\n",
        "MOTOR-ARRANQUE marca 3: Run() volvio\r\n",
    };

    if (que >= 1 && que <= 3) {
        const char *m = marcas[que];
        int n = 0;
        while (m[n] != '\0') n++;
        dbgio_write_buffer_xlat((const uint8 *)m, (size_t)n);
    }

    return que;
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

/* d2c_reserva: pide N bytes (una sola asignacion, el caso de la tabla) y los TOCA en las dos puntas, para
 * que el sistema entregue la memoria de verdad y no solo anote el pedido. Devuelve el puntero (0 = no
 * entro). */
unsigned int d2c_reserva(unsigned int bytes) {
    unsigned char *p = (unsigned char *)malloc(bytes);

    if (p == 0) {
        return 0;
    }

    p[0] = 0x5a;
    p[bytes - 1] = 0xa5;

    return (unsigned int)(uintptr_t)p;
}

/* d2c_reserva_libre: libera lo que pidio d2c_reserva. */
void d2c_reserva_libre(unsigned int ptr) {
    if (ptr != 0) {
        free((void *)(uintptr_t)ptr);
    }
}
