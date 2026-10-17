/* DC-002.27: lo minimo del C del POC motor-disco.
 *
 *   - la consola serie: sin dbgio_dev_select("scif") KOS deja el handler en NULL y no sale nada (medido
 *     desde DC-001.3). Es la unica salida que tiene la consola.
 *   - las marcas de avance: el motor puede quedarse en su cuadro de error, en el bucle de dibujo o
 *     colgado. Las marcas dicen POR DONDE IBA cuando eso pase.
 *   - el reloj (d2c_ms): para medir cuanto cuesta leer el MPQ del disco, con el temporizador de KOS
 *     (`TimerUsGettime64`, `TimerMsEnable`). Los contadores del SH-4 (PerfCntr*) devuelven 0 en Flycast:
 *     no se usan aca.
 */
#include <kos.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

/* d2c_ms: milisegundos desde que arranco el programa, con el temporizador de KOS. Se escribe con
 * dbgio_write_buffer_xlat y no con printf por lo mismo que las marcas: si el motor se cuelga en la linea
 * siguiente, la marca ya tiene que estar fuera. */
unsigned int d2c_ms(void) {
    static int encendido = 0;

    if (!encendido) {
        timer_ms_enable();
        encendido = 1;
    }

    return (unsigned int)(timer_ms_gettime64() & 0xffffffffu);
}

unsigned int d2c_disco_marca(unsigned int que) {
    static const char *marcas[] = {
        "",
        "MOTOR-DISCO marca 1: la sonda del disco (antes del motor)\r\n",
        "MOTOR-DISCO marca 2: entrando a d2app.Create()\r\n",
        "MOTOR-DISCO marca 3: Create() volvio, entrando a Run()\r\n",
        "MOTOR-DISCO marca 4: Run() volvio\r\n",
    };

    if (que >= 1 && que <= 4) {
        const char *m = marcas[que];
        int n = 0;
        while (m[n] != '\0') n++;
        dbgio_write_buffer_xlat((const uint8 *)m, (size_t)n);
    }

    return que;
}
