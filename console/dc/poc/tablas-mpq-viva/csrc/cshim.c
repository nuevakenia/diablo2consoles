/* DC-002.39: los instrumentos del POC tablas-mpq-viva (el mismo cshim.c de tablas-mpq, DC-002.33).
 *
 * Tres funciones, las que el main.go declara con //extern:
 *
 *   d2c_serial_init   la consola serie. Tiene que ser C: en Go no se puede llamar a `dbgio_dev_select`.
 *   d2c_ms            el reloj (para decir cuanto tarda abrir cada MPQ en la consola).
 *   d2c_monton_libre  `mallinfo().fordblks`, el MISMO numero que el runtime de la consola imprime como
 *                     `free=`. Aca sirve para ver que abrir los tres MPQ NO mueve el monton de C.
 *
 * El vigia del arreglo (`d2c_vigia_dir`, `d2c_fase`, `d2c_mira`) NO vive aca: vive en el runtime del
 * proyecto (`libgodc/runtime/vigia_viva.c`, copia privada del runtime usada para medir DC-002.39), porque
 * tiene que imprimir las palabras DENTRO de cada recoleccion.
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
