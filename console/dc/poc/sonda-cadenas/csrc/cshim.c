/* DC-002.36 -- el puente minimo de la sonda de cadenas: SOLO la consola serie y el reloj.
 *
 * La sonda no dibuja nada (no toca el PVR ni la VRAM): mide el recolector de basura de Go. Se copian de la
 * stdlib minima las dos unicas funciones que hacen falta, con el mismo cuerpo que ya esta medido:
 *   d2c_serial_init: sin `dbgio_dev_select("scif")` KOS deja el handler del dbgio en NULL y el println de Go
 *                    no sale a ninguna parte (medido en DC-001).
 *   d2c_ms:          el reloj en milisegundos. Los contadores del SH-4 (PerfCntr) devuelven 0 en Flycast,
 *                    asi que el tiempo se mide con el temporizador de KOS.
 */
#include <kos.h>
#include <stdint.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

unsigned int d2c_ms(void) {
    static int encendido = 0;

    if (!encendido) {
        timer_ms_enable();
        encendido = 1;
    }

    return (unsigned int)(timer_ms_gettime64() & 0xffffffffu);
}
