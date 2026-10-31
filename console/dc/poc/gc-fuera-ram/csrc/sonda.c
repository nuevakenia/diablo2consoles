/* DC-002.92 -- el puente C de la sonda del barrido del GC.
 *
 * La sonda no dibuja nada: mide el recolector. Lo unico que necesita de C es lo que Go no puede decir:
 * direcciones, el encabezado del objeto y la memoria de malloc (que NO se mueve en una recoleccion, cosa que
 * el monton de Go si hace y que romperia el experimento: el descriptor falso tiene que quedarse quieto).
 *
 *   d2c_serial_init      sin `dbgio_dev_select("scif")` el println de Go no sale a ninguna parte.
 *   d2c_ms               reloj en ms (los contadores del SH-4 dan 0 en Flycast).
 *   d2c_usado            mallinfo().uordblks: el "usado" del monton de C, que es donde pide memoria el runtime.
 *   d2c_sbrk/d2c_techo   el break del monton y el tope de KOS (de aca sale el mapa de secciones).
 *   d2c_fin_imagen       `_end`: el fin de la imagen (texto+datos+bss). Los descriptores de tipo viven ADENTRO.
 *   d2c_malloco          pide n bytes por malloc, los llena con `relleno` y devuelve la direccion.
 *   d2c_header_de        el encabezado de un objeto del monton de Go: ptr-8 (GC_HEADER_SIZE).
 *   d2c_falsificar_tipo  escribe el campo `type` del encabezado. Esto es lo que arma el caso.
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

/* El techo de RAM de KOS: el mismo que usa el runtime para no pasarse. */
#define D2C_TECHO ((unsigned int)0x8d000000u)

unsigned int d2c_techo(void) {
    return D2C_TECHO;
}

/* La pila del hilo actual, para el mapa (solo informativo). */
unsigned int d2c_pila(void) {
    char aqui;
    return (unsigned int)(uintptr_t)&aqui;
}

unsigned int d2c_malloco(unsigned int n, int relleno) {
    unsigned char *p = (unsigned char *)malloc(n);

    if (p == NULL) {
        return 0;
    }

    memset(p, relleno, n);

    return (unsigned int)(uintptr_t)p;
}

unsigned int d2c_header_de(unsigned int p) {
    return p - 8;   /* GC_HEADER_SIZE = 8 (ver libgodc/runtime/gc_semispace.h) */
}

unsigned int d2c_tamano_header(unsigned int p) {
    return *(unsigned int *)(uintptr_t)(p - 8);
}

unsigned int d2c_tipo_header(unsigned int p) {
    return *(unsigned int *)(uintptr_t)(p - 4);
}

void d2c_falsificar_tipo(unsigned int p, unsigned int tipo) {
    *(unsigned int *)(uintptr_t)(p - 4) = tipo;
}

unsigned int d2c_poner32(unsigned int addr, unsigned int off, unsigned int val) {
    *(unsigned int *)(uintptr_t)(addr + off) = val;
    return *(unsigned int *)(uintptr_t)(addr + off);
}

unsigned int d2c_leer32(unsigned int addr, unsigned int off) {
    return *(unsigned int *)(uintptr_t)(addr + off);
}

unsigned int d2c_addr(void *p) {
    return (unsigned int)(uintptr_t)p;
}
