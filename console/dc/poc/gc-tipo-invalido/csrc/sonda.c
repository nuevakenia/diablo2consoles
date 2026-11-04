/* DC-002.146d -- el puente C de la sonda del tipo invalido del GC.
 *
 * La sonda no dibuja nada: mide el recolector. De C necesita solo lo que Go no puede decir: direcciones, el
 * encabezado del objeto, y las marcas de tiempo/comodidad del arranque.
 *
 *   d2c_serial_init      sin `dbgio_dev_select("scif")` el println de Go no sale a ninguna parte.
 *   d2c_ms               reloj en ms (los contadores del SH-4 dan 0 en Flycast).
 *   d2c_usado            mallinfo().uordblks: el "usado" del monton de C.
 *   d2c_sbrk             el break del monton: la PRACTICA es el tope de la imagen en el arranque, que es el mismo
 *                        valor que usa `gc_init()` de libgodc para `gc_imagen_hi`.
 *   d2c_dir              la direccion de un global de Go (el descriptor falso vive en .bss, o sea en la imagen).
 *   d2c_header_de        el encabezado de un objeto del monton de Go: ptr-8 (GC_HEADER_SIZE).
 *   d2c_tipo_header      el campo `type` del encabezado (el que se falsifica).
 *   d2c_falsificar_tipo  escribe ese campo. Esto es lo que arma el caso.
 *   d2c_addr             la direccion de un *nodo (el objeto vivo).
 */
#include <kos.h>
#include <stdint.h>
#include <malloc.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

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

int d2c_usado(void) {
    struct mallinfo mi = mallinfo();
    return (int)mi.uordblks;
}

unsigned int d2c_sbrk(void) {
    return (unsigned int)(uintptr_t)sbrk(0);
}

unsigned int d2c_dir(void *p) {
    return (unsigned int)(uintptr_t)p;
}

unsigned int d2c_header_de(unsigned int p) {
    return p - 8;   /* GC_HEADER_SIZE = 8 (ver libgodc/runtime/gc_semispace.h) */
}

unsigned int d2c_tipo_header(unsigned int p) {
    return *(unsigned int *)(uintptr_t)(p - 4);
}

void d2c_falsificar_tipo(unsigned int p, unsigned int tipo) {
    *(unsigned int *)(uintptr_t)(p - 4) = tipo;
}

unsigned int d2c_addr(void *p) {
    return (unsigned int)(uintptr_t)p;
}
