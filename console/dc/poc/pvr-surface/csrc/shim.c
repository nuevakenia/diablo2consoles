/* DC-002.69: los puentes a C de la prueba de humo del backend PVR.
 *
 * 1) LA CONSOLA SERIE. El `println` de Go en libgodc sale por el dispositivo de dbgio que este seleccionado,
 *    y el arranque de KOS no elige ninguno: sin esto la salida del POC no sale a ninguna parte (medido en
 *    DC-001.3). Es el mismo shim que los POC de enlace y del personaje.
 *
 * 2) EL MONTON, que el binding de Go no expone: el numero que dice si la pantalla (640x480x4 = 1.228.800 B)
 *    entra o no. mallinfo().fordblks es el MISMO numero que imprime el runtime cuando se queda sin memoria
 *    ("Memory: arena=%d used=%d free=%d"), asi que lo que se lee aca es comparable con el defecto.
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
