/* DC-002.70: los puentes a C del POC del estado B (el mapa y el personaje por la interfaz).
 *
 * 1) LA CONSOLA SERIE. El `println` de Go en libgodc sale por el dispositivo de dbgio que este seleccionado,
 *    y el arranque de KOS no elige ninguno: sin esto la salida del POC no sale a ninguna parte (medido en
 *    DC-001.3). Es el mismo shim de los POC de enlace, del personaje y de la prueba de humo del backend.
 *
 * 2) EL MONTON, que el binding de Go no expone: mallinfo().fordblks es el MISMO numero que imprime el
 *    runtime cuando se queda sin memoria ("Memory: arena=%d used=%d free=%d"), asi que lo que se lee aca es
 *    comparable con el defecto. Es el numero con el que se negocia la memoria del cuadro.
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
