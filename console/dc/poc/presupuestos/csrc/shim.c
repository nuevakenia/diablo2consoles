/* DC-002.75: los puentes a C del POC de los presupuestos de cache.
 *
 * 1) LA CONSOLA SERIE. El `println` de Go en libgodc sale por el dispositivo de dbgio que este
 *    seleccionado, y el arranque de KOS no elige ninguno: sin esto la salida del POC no sale a ninguna
 *    parte (medido en DC-001.3). Es el mismo shim de los POC del enlace, del personaje y del estado B.
 *
 * 2) EL MONTON, que el binding de Go no expone: mallinfo().fordblks es el MISMO numero que imprime el
 *    runtime cuando se queda sin memoria ("Memory: arena=%d used=%d free=%d"), asi que lo que se lee
 *    aca es comparable con el defecto. Es el numero con el que se mide cuanto dejo libre cada
 *    presupuesto de cache.
 */
#include <kos.h>
#include <malloc.h>
#include <stdio.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

unsigned int d2c_heap_libre(void) {
    struct mallinfo mi = mallinfo();

    return (unsigned int)mi.fordblks;
}
