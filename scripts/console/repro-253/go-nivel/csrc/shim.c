/* DC-002.58 -- el C del eslabon del NIVEL (`scripts/console/repro-253/go-nivel/`).
 *
 * SON LAS CUATRO FUNCIONES DE ARRANQUE DEL POC, COPIADAS DE
 * `console/dc/poc/personaje-mapa/csrc/cshim.c` (medido con `cmp` sobre los cuerpos):
 *
 *   d2c_serial_init   la consola serie (sin esto el println de Go no sale a ninguna parte)
 *   d2c_ms            los milisegundos del temporizador de KOS (los contadores del SH-4 dan 0 en Flycast)
 *   d2c_vram_libre    la VRAM que queda: SOLO tiene sentido con el PVR encendido (ver DC-002.56)
 *   d2c_heap_libre    el monton que queda, el MISMO numero que imprime el runtime al quedarse sin memoria
 *   d2c_free_external devuelve al monton de C una asignacion GRANDE del runtime (los DT1 del town son 4,35 MB)
 *
 * POR QUE UNA COPIA Y NO EL ARCHIVO DEL POC: el POC no se toca desde aca a proposito. Su layout es justo lo
 * que se esta midiendo (DC-002.53: 64 bytes mas o menos en 1ST_READ.BIN mueven el punto de muerte), asi que
 * ni se edita ni se le agrega un archivo. El C que sobra del POC (la textura paletizada y los quads) no se
 * copia: este eslabon pinta con el shim de la stdlib (`console/dc/std/csrc/pvr_pantalla.c`).
 *
 * OJO (medido en DC-002.56): con el PVR apagado `pvr_mem_available()` devuelve 0 porque
 * `pvr_mem_base` es un estatico en .bss que solo llenan `pvr_init`/`pvr_mem_reset`
 * (`kernel/arch/dreamcast/hardware/pvr/pvr_mem.c:176`). El 0 de las marcas previas al video NO es
 * "no hay VRAM": es "el PVR no arranco todavia".
 */
#include <kos.h>
#include <stdio.h>
#include <stdint.h>
#include <malloc.h>

/* la consola serie es la UNICA salida de texto: sin esto, KOS deja el handler en NULL. */
void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

/* milisegundos desde que arranco el programa (temporizador de KOS: los contadores de rendimiento del SH-4
 * devuelven 0 en Flycast, medido). */
unsigned int d2c_ms(void) {
    static int encendido = 0;

    if (!encendido) {
        timer_ms_enable();
        encendido = 1;
    }

    return (unsigned int)(timer_ms_gettime64() & 0xffffffffu);
}

/* la VRAM que queda. Con el PVR apagado devuelve 0 (ver el comentario de arriba). */
unsigned int d2c_vram_libre(void) {
    return (unsigned int)pvr_mem_available();
}

/* el monton que queda (bytes libres): el mismo numero que imprime el runtime cuando se queda sin memoria
 * ("Memory: arena=%d used=%d free=%d" en libgodc/runtime/defer_dreamcast.c). */
unsigned int d2c_heap_libre(void) {
    struct mallinfo mi = mallinfo();

    return (unsigned int)mi.fordblks;
}

/* devuelve al monton de C una asignacion GRANDE del runtime (mas de 64 KB: GC_LARGE_OBJECT_THRESHOLD_KB en
 * runtime/godc_config.h). El recolector no las vuelve a tocar: solo salen por runtime.FreeExternal. Sin esto,
 * leer los 8 DT1 del town (4,35 MB, uno por vez) no devuelve nada y el arranque muere con
 * "fatal error: out of memory" despues de 3 o 4 archivos (medido en DC-002.37). */
extern void runtime_FreeExternal(void *ptr) __asm__("_runtime.FreeExternal");

void d2c_free_external(void *ptr) {
    if (ptr) {
        runtime_FreeExternal(ptr);
    }
}
