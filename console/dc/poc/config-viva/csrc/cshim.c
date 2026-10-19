/* DC-002.39: los instrumentos del POC config-viva.
 *
 * Uno solo, y tiene que ser C: la consola serie se abre con `dbgio_dev_select`, que no se puede llamar
 * desde Go (el binding de libgodc no lo expone). Sin esto la corrida entera es muda.
 *
 * El otro numero es el monton de C (`mallinfo().fordblks`), el MISMO que el runtime de la consola imprime
 * como `free=` cuando se queda sin RAM: sirve para ver que vigilar los nombres NO se lleva el monton de C
 * puesto (el que se llena es el semiespacio del GC, que no es este numero).
 */
#include <kos.h>
#include <malloc.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

int d2c_monton_libre(void) {
    return mallinfo().fordblks;
}
