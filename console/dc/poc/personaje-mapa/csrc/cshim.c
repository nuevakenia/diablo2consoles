/* DC-002.35: la textura paletizada del asset DEL DISCO, subida al PVR.
 *
 * Por que en C: el binding de Go de libgodc no expone pvr_txr_load_ex ni pvr_set_pal_entry, que son
 * los dos que suben una textura paletizada (con la paleta por hardware del PowerVR2). El mismo shim
 * que usa motor-menu (DC-002.9), con una diferencia: alla los bytes venian HORNEADOS en un .h, aca
 * los pasa el Go, que los acaba de leer del MPQ del disco y decodificar con el decodificador del
 * motor. Por eso la entrada es un puntero (direccion, pasada como uint) y no un arreglo del .h.
 *
 * Las tres cosas que ya estan resueltas aca (y costaron una tarde cada una, ver la skill):
 *   1. el lado de la textura tiene que ser POTENCIA DE DOS: el asset se sube rellenado (con indice 0)
 *      y el quad dibuja solo el pedazo visible, con las UV recortadas medio texel;
 *   2. las UV de las texturas twiddled van NORMALIZADAS (0..1), no en pixeles;
 *   3. el PVR dibuja TIRAS: el quad son cuatro vertices en zigzag y la marca de fin de tira
 *      (PVR_CMD_VERTEX_EOL) va en el CUARTO, no en el tercero.
 */
#include <kos.h>
#include <stdio.h>
#include <stdint.h>
#include <malloc.h>

static pvr_ptr_t tex = NULL;
static pvr_poly_hdr_t hdr;
static int ancho = 0, alto = 0;     /* los del asset */
static int tex_w = 0, tex_h = 0;    /* los de la textura (potencia de dos) */

/* d2c_serial_init: la consola serie es la UNICA salida de texto (sin esto, KOS deja el handler en
 * NULL y el println de Go no sale a ninguna parte: medido en DC-001). */
void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

/* d2c_ms: milisegundos desde que arranco el programa, con el temporizador de KOS (los contadores de
 * rendimiento del SH-4 devuelven 0 en Flycast: medido). */
unsigned int d2c_ms(void) {
    static int encendido = 0;

    if (!encendido) {
        timer_ms_enable();
        encendido = 1;
    }

    return (unsigned int)(timer_ms_gettime64() & 0xffffffffu);
}

/* d2c_vram_libre: la VRAM que queda, igual que la mide el POC del motor.
 *
 * DC-002.56 (medido): SOLO TIENE SENTIDO CON EL PVR ENCENDIDO. `pvr_mem_available()` (KOS
 * kernel/arch/dreamcast/hardware/pvr/pvr_mem.c:176) arranca con `if(!pvr_mem_base) return 0;` y
 * `pvr_mem_base` es un estatico en .bss que pone en cero el arranque y solo llena `pvr_init`/
 * `pvr_mem_reset`. Con el PVR apagado devuelve 0 sin tocar el allocator ni pvr_state (en este KOS
 * el allocator de VRAM es un dlmalloc propio y `pvr_state.ram_free_list` ya no existe). O sea:
 * leerlo antes del encendido no ensucia nada, pero el 0 no informa nada tampoco. Las marcas del
 * camino previo al video dicen "vram_libre= sin-init" y la unica lectura viva es
 * "HITO250: VRAM-POSTINIT", despues de `pvr_init_defaults`.
 */
unsigned int d2c_vram_libre(void) {
    return (unsigned int)pvr_mem_available();
}

/* d2c_heap_libre: el monton que queda (bytes libres), el MISMO numero que imprime el runtime cuando se queda
 * sin memoria ("Memory: arena=%d used=%d free=%d" en libgodc/runtime/defer_dreamcast.c). En la Dreamcast la
 * RAM es un solo monton: el heap de Go se pide de aca con memalign, y las asignaciones GRANDES del runtime van
 * a parar aca y no las recupera nadie (ver d2c_free_external mas abajo). O sea que este numero es el que dice
 * si un POC entra o no, y sin el se mide a ciegas (medido en DC-002.50: se paso de "entra" a "no entra" por
 * 150 KB). */
unsigned int d2c_heap_libre(void) {
    struct mallinfo mi = mallinfo();

    return (unsigned int)mi.fordblks;
}

/* d2c_tex_init: sube los indices de 8 bits y la paleta ARGB1555 que le pasa el Go.
 * Devuelve 0 si todo bien; 1 textura demasiado grande; 2 sin VRAM. */
int d2c_tex_init(unsigned int idx, unsigned int pal, int w, int h, int pw, int ph) {
    const unsigned char *indices = (const unsigned char *)(uintptr_t)idx;
    const unsigned short *paleta = (const unsigned short *)(uintptr_t)pal;

    if (indices == NULL || paleta == NULL || w <= 0 || h <= 0) {
        return 3;
    }

    pvr_set_pal_format(PVR_PAL_ARGB1555);
    for (int i = 0; i < 256; i++) {
        pvr_set_pal_entry(i, paleta[i]);
    }

    ancho = w;
    alto = h;
    tex_w = pw;
    tex_h = ph;

    if (tex_w > 1024 || tex_h > 1024) {
        printf("HITO237: textura demasiado grande (%dx%d)\n", tex_w, tex_h);
        return 1;
    }

    tex = pvr_mem_malloc(tex_w * tex_h);
    if (!tex) {
        printf("HITO237: sin VRAM para %d bytes\n", tex_w * tex_h);
        return 2;
    }

    pvr_txr_load_ex(indices, tex, tex_w, tex_h, PVR_TXRLOAD_8BPP);

    pvr_poly_cxt_t cxt;
    pvr_poly_cxt_txr(&cxt, PVR_LIST_TR_POLY,
        PVR_TXRFMT_PAL8BPP | PVR_TXRFMT_8BPP_PAL(0) | PVR_TXRFMT_TWIDDLED,
        tex_w, tex_h, tex, PVR_FILTER_NONE);
    cxt.txr.alpha = PVR_TXRALPHA_ENABLE;    /* el indice 0 es transparente */
    cxt.blend.src = PVR_BLEND_SRCALPHA;
    cxt.blend.dst = PVR_BLEND_INVSRCALPHA;
    cxt.blend.src_enable = PVR_BLEND_ENABLE;
    cxt.blend.dst_enable = PVR_BLEND_ENABLE;
    pvr_poly_compile(&hdr, &cxt);

    return 0;
}

/* d2c_free_external: DEVUELVE al monton de C una asignacion GRANDE del runtime.
 *
 * POR QUE EXISTE (medido en DC-002.37): el recolector de libgodc manda TODO objeto de mas de
 * GC_LARGE_OBJECT_THRESHOLD_KB (64 KB, ver runtime/godc_config.h) al `malloc` del monton de C
 * (`gc_heap.c: gc_external_alloc`) y el recolector NO los vuelve a tocar: no tienen cabecera de GC, asi
 * que solo salen por `runtime.FreeExternal`. Sin esto, leer los 8 DT1 del town (4,35 MB, uno por vez) no
 * devuelve nada y el arranque muere con
 *     fatal error: out of memory   (arena=6075776 used=6075584 free=192)
 * despues de 3 o 4 archivos. Con esto la arena se mantiene en el pico de UN archivo.
 *
 * OJO: solo vale para punteros de asignaciones de mas de 64 KB. Una asignacion chica vive en el
 * semiespacio del recolector y liberarla a mano rompe la memoria.
 */
extern void runtime_FreeExternal(void *ptr) __asm__("_runtime.FreeExternal");

void d2c_free_external(void *ptr) {
    if (ptr) {
        runtime_FreeExternal(ptr);
    }
}

/* d2c_quad_solido: un quad de COLOR, SIN textura. Es el CONTROL del camino de dibujo (DC-002.37): si
 * este quad se ve en la pantalla y el texturado no, el que falla es la subida de la textura, no el PVR ni
 * el refresco. Va aparte a proposito: no toca el estado de la textura. */
void d2c_quad_solido(float x, float y, float w, float h, unsigned int argb) {
    pvr_poly_cxt_t cxt;
    pvr_poly_hdr_t h2;
    pvr_vertex_t v;

    pvr_poly_cxt_col(&cxt, PVR_LIST_TR_POLY);
    cxt.gen.culling = PVR_CULLING_NONE;
    pvr_poly_compile(&h2, &cxt);
    pvr_prim(&h2, sizeof(h2));

    v.argb = argb;
    v.oargb = 0;
    v.z = 1.0f;
    v.u = 0.0f;
    v.v = 0.0f;

    v.flags = PVR_CMD_VERTEX;
    v.x = x;         v.y = y;         pvr_prim(&v, sizeof(v));
    v.x = x + w;     v.y = y;         pvr_prim(&v, sizeof(v));
    v.x = x;         v.y = y + h;     pvr_prim(&v, sizeof(v));
    v.flags = PVR_CMD_VERTEX_EOL;
    v.x = x + w;     v.y = y + h;     pvr_prim(&v, sizeof(v));
}

/* d2c_quad: dibuja el asset en (x,y), escalado por s. */
void d2c_quad(float x, float y, float s) {
    pvr_vertex_t v;
    float u0 = 0.5f / tex_w, v0 = 0.5f / tex_h;
    float u1 = (float)ancho / tex_w - 0.5f / tex_w;
    float v1 = (float)alto / tex_h - 0.5f / tex_h;

    pvr_prim(&hdr, sizeof(hdr));

    v.argb = 0xFFFFFFFF;
    v.oargb = 0;
    v.z = 1.0f;

    v.flags = PVR_CMD_VERTEX;
    v.u = u0;   v.v = v0;   v.x = x;                v.y = y;                pvr_prim(&v, sizeof(v));
    v.u = u1;   v.v = v0;   v.x = x + ancho * s;    v.y = y;                pvr_prim(&v, sizeof(v));
    v.u = u0;   v.v = v1;   v.x = x;                v.y = y + alto * s;     pvr_prim(&v, sizeof(v));
    v.flags = PVR_CMD_VERTEX_EOL;
    v.u = u1;   v.v = v1;   v.x = x + ancho * s;    v.y = y + alto * s;     pvr_prim(&v, sizeof(v));
}
