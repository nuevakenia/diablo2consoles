/* DC-002.72: los dos puentes a C que necesita el POC del audio.
 *
 * NO HAY NADA DEL SONIDO ACA: el camino al AICA (d2audio_*) vive en console/dc/std/csrc/sonido_dc.c, que
 * build-dc.sh compila y enlaza SIEMPRE, y el paquete d2core/d2audio/aica lo llama por //extern. Es el mismo
 * criterio que el POC del control (DC-002.71) con el lector del maple: el hardware, en un solo lugar.
 *
 * Estos dos son del POC y no del paquete: la consola serie (sin esto el println de Go no sale a ninguna
 * parte) y el reloj para las marcas de tiempo (los contadores de rendimiento del SH-4 devuelven 0 en
 * Flycast: medido).
 */
#include <kos.h>
#include <stdio.h>

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
