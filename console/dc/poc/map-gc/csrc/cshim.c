/* DC-002.83 -- el puente minimo de la sonda del mapa: la consola serie, el reloj y la FORMA del mapa.
 *
 * La sonda no dibuja nada (no toca el PVR ni la VRAM): mide el recolector de basura de Go.
 *   d2c_serial_init: sin `dbgio_dev_select("scif")` KOS deja el handler del dbgio en NULL y el println de Go
 *                    no sale a ninguna parte (medido en DC-001). Mismo cuerpo que la sonda de cadenas.
 *   d2c_ms:          el reloj en milisegundos (los contadores del SH-4 devuelven 0 en Flycast).
 *   d2c_mapa_info:   lee el header del mapa (GoMap) y devuelve count, puntero del array de buckets, B y los
 *                    desbordes. El layout esta copiado de libgodc/runtime/map_dreamcast.h, que lo verifica
 *                    con _Static_assert contra el que usa gccgo.
 */
#include <kos.h>
#include <stdint.h>
#include <malloc.h>

/* layout de libgodc/runtime/map_dreamcast.h (28 bytes en SH-4 de 32 bits) */
typedef struct {
    uintptr_t count;
    uint8_t   flags;
    uint8_t   B;
    uint16_t  noverflow;
    uint32_t  hash0;
    void     *buckets;
    void     *oldbuckets;
    uintptr_t nevacuate;
    void     *extra;
} GoMap;

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

/* p es la direccion de la variable de Go (un map en gccgo es un puntero a GoMap). */
void d2c_mapa_info(void *p, int *cuenta, unsigned int *buckets, int *b, int *desbordes) {
    GoMap *m;

    if (p == 0) {
        return;
    }

    m = *(GoMap **)p;

    if (m == 0) {
        *cuenta = -1;
        return;
    }

    *cuenta = (int)m->count;
    *buckets = (unsigned int)(uintptr_t)m->buckets;
    *b = (int)m->B;
    *desbordes = (int)m->noverflow;
}

/* el mismo instrumento que usa el POC del cooperativo: el libre del monton de C (en KB). */
unsigned int d2c_heap_libre(void) {
    struct mallinfo mi = mallinfo();

    return (unsigned int)(mi.fordblks / 1024);
}

/* cuantas recolecciones lleva el GC (el contador de libgodc que ya usa el POC del cooperativo). */
extern unsigned long marca_gc_n;

unsigned int d2c_gc_n(void) {
    return (unsigned int)marca_gc_n;
}
