/* DC-002.67: los dos puentes a C del POC de enlace.
 *
 * 1) LA CONSOLA SERIE. El `println` de Go en libgodc sale por el dispositivo de dbgio que este seleccionado,
 *    y el arranque de KOS no elige ninguno. Con esto la salida va a la SCIF y Flycast la escribe en su log
 *    (Debug:SerialConsoleEnabled=true). Es el mismo shim que usan los POC de DC-001.3 en adelante
 *    (console/dc/poc/goquad/csrc/cshim.c).
 *
 * 2) LA RAM DEL MONTON, que el binding de Go no expone. En una consola de 16 MB el dato que decide si el
 *    motor entero entra no es el tamano del 1ST_READ.BIN sino lo que queda libre despues del arranque: el
 *    runtime de libgodc se lleva 2 espacios de GC de 2 MB y KOS su propio monton. mallinfo() del dlmalloc de
 *    KOS dice el arena y lo libre ADENTRO del arena.
 */
#include <kos.h>
#include <malloc.h>
#include <stdio.h>
#include <unistd.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

unsigned int d2c_ram_arena(void) {
    struct mallinfo mi = mallinfo();

    return (unsigned int)mi.arena;
}

unsigned int d2c_ram_libre(void) {
    struct mallinfo mi = mallinfo();

    return (unsigned int)mi.fordblks;
}

/* OJO, lo que NO se puede medir asi: la RAM libre ARRIBA del monton. El techo que KOS usa para ubicar el
 * monton (la etiqueta __arch_mem_top de su startup.S) NO resuelve al enlazar desde C -- medido: el enlazador
 * da "undefined reference to __arch_mem_top" aunque la etiqueta este en libkallisti.
 *
 * Por eso la RAM se mide AL REVES, que ademas es lo que le importa al motor: se le PIDE al monton un bloque
 * del tamano que el motor va a necesitar y se informa si lo dio. d2c_ram_pedir devuelve los bytes pedidos si
 * el malloc salio bien y 0 si no. */

unsigned int d2c_ram_pedir(unsigned int bytes) {
    void *p = malloc(bytes);

    if (p == NULL) {
        return 0;
    }

    free(p);

    return bytes;
}
