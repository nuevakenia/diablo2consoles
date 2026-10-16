/* Sonda DC-002.2f: solo la consola serie. */
#include <kos.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}
