/* DC-002.39e: los instrumentos del POC itab-raiz.
 *
 * Uno solo, y tiene que ser C: la consola serie se abre con `dbgio_dev_select`, que no se puede llamar
 * desde Go (el binding de libgodc no lo expone). Sin esto la corrida entera es muda.
 *
 * Los otros dos numeros son el monton de C (`mallinfo().fordblks`, el mismo que el runtime imprime como
 * `free=` cuando se queda sin RAM) y la CUENTA DE RECOLECCIONES del runtime (`gc_stats`), que es lo que
 * dice si el itab llego a copiarse o no: sin recolecciones este POC no prueba nada.
 */
#include <kos.h>
#include <malloc.h>
#include <stdint.h>
#include <stddef.h>

void gc_stats(size_t *used, size_t *total, uint32_t *collections);

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

int d2c_monton_libre(void) {
    return mallinfo().fordblks;
}

unsigned long d2c_ms(void) {
    return (unsigned long)timer_ms_gettime64();
}

int d2c_gc_cuenta(void) {
    size_t usado = 0, tope = 0;
    uint32_t cuenta = 0;

    gc_stats(&usado, &tope, &cuenta);

    return (int)cuenta;
}

/* DC-002.39e: la BASE del semiespacio ACTIVO la expone la sonda del runtime (`d2c_semi_activo`, en el
 * interface_dreamcast.c de la copia privada): este archivo no ve gc_semispace.h. Aca no va nada mas. */
