/* DC-002.32: el C del POC sonda-panic-232 (la consola serie y las marcas, lo mismo del POC motor-arranque:
 * son el mismo instrumento y no se reescribe). */
#include <kos.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

unsigned int d2c_arranque_marca(unsigned int que) {
    static const char *marcas[] = {
        "",
        "PANIC-232 marca 1: antes del panic\r\n",
        "PANIC-232 marca 2: despues del panic (no deberia salir)\r\n",
        "PANIC-232 marca 3: fin\r\n",
    };

    if (que >= 1 && que <= 3) {
        const char *m = marcas[que];
        int n = 0;
        while (m[n] != '\0') n++;
        dbgio_write_buffer_xlat((const uint8 *)m, (size_t)n);
    }

    return que;
}
