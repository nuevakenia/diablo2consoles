/* DC-002.33: los instrumentos del POC tablas-mpq.
 *
 * Tres funciones, las que el main.go declara con //extern:
 *
 *   d2c_serial_init   la consola serie. Tiene que ser C: en Go no se puede llamar a `dbgio_dev_select`.
 *   d2c_ms            el reloj (para decir cuanto tarda abrir cada MPQ en la consola).
 *   d2c_monton_libre  `mallinfo().fordblks`, el MISMO numero que el runtime de la consola imprime como
 *                     `free=` cuando se queda sin RAM. Aca sirve para ver que abrir los tres MPQ NO mueve
 *                     el monton de C: con las tablas guardadas como BYTES cada tabla es UNA reserva de mas
 *                     de 64 KiB (o sea del monton de C) y la de d2data.mpq son 1.048.576 bytes.
 *
 * El juego COMPLETO de instrumentos del monton (sbrk, techo, arena, usado y el bloque CONTIGUO mas grande)
 * esta en csrc/cshim.c del POC `motor-reservas` (DC-002.28), que es el que mide "entra o no entra esta
 * reserva": aca no se repite porque este POC no pregunta eso (a proposito: es otra tarjeta la que mide el
 * monton de C; aca se mide QUE DICEN LAS TABLAS).
 */
#include <kos.h>
#include <malloc.h>

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

int d2c_monton_libre(void) {
    return mallinfo().fordblks;
}
