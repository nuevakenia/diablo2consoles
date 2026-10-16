/* Sonda DC-002.3: solo la consola serie (la evidencia es el texto del motor decodificando). */
#include <kos.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}
