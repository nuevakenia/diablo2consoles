/* Sonda de goroutines (DC-002.7): la consola serie y el ceder del hilo de KOS.
 *
 * thd_pass() es lo que el scheduler de libgodc usa en su bucle: cede el hilo de KOS al scheduler de
 * KOS y vuelve. Desde Go lo llamo por //extern para medir si con eso las otras goroutines progresan.
 */
#include <kos.h>
#include <stdint.h>
#include <arch/timer.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

void d2c_thd_pass(void) {
    thd_pass();
}
