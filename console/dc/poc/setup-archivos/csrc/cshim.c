/* DC-002.13: lo minimo del C para ARRANCAR EL MOTOR DE VERDAD en la consola.
 *
 * El POC llama a d2app.Create(...).Run(), que es el arranque del juego tal cual (el de main.go de la raiz).
 * Lo unico que aporta este C es lo que en el host hace el sistema operativo:
 *
 *   - la consola serie: sin dbgio_dev_select("scif") KOS deja el handler en NULL y no sale nada (medido
 *     desde DC-001.3). Todo lo que imprima el motor (su `log` de la stdlib minima sale por os.Stdout, que
 *     es d2c_escribir_consola) tiene que poder leerse.
 *   - las marcas de avance: el motor puede irse a un dialogo de error, a un bucle o a un cuelgue. Las
 *     marcas dicen POR DONDE IBA cuando eso pase, que es el numero que se mide en esta tarjeta.
 *
 * El video NO se arranca aca: lo arranca Go con ebiten.IniciarPantalla (el backend de la consola), que es
 * el camino que usa el motor para su pantalla.
 */
#include <kos.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

/* d2c_arranque_marca: deja escrito en la serial por que punto del arranque va el POC. Se escribe con
 * dbgio_write_buffer_xlat y no con printf para que no dependa del buffering de la libc de KOS: si el motor
 * se cuelga en la linea siguiente, la marca ya tiene que estar fuera. */
unsigned int d2c_arranque_marca(unsigned int que) {
    static const char *marcas[] = {
        "",
        "MOTOR-ARRANQUE marca 1: entrando a d2app.Create()\r\n",
        "MOTOR-ARRANQUE marca 2: Create() volvio, entrando a Run()\r\n",
        "MOTOR-ARRANQUE marca 3: Run() volvio\r\n",
    };

    if (que >= 1 && que <= 3) {
        const char *m = marcas[que];
        int n = 0;
        while (m[n] != '\0') n++;
        dbgio_write_buffer_xlat((const uint8 *)m, (size_t)n);
    }

    return que;
}
