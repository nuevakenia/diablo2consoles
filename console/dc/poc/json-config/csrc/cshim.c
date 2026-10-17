/* DC-002.18: lo minimo del C para el corpus del config.json: la consola serie.
 *
 * Sin dbgio_dev_select("scif") KOS deja el handler del debug en NULL y no sale NADA por la serial (medido
 * desde DC-001.3), o sea que el corpus no se podria leer. Lo demas lo hace Go: el sistema de archivos
 * escribible de la consola (el overlay en RAM de DC-002.17) y el serializador del motor.
 */
#include <kos.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}
