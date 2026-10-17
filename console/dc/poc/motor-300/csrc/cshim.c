/* DC-001.5: el puente a C del POC de los tres numeros del go/no-go.
 *
 * Por que en C: el binding de Go de libgodc no expone (a) el reloj de microsegundos, (b) mallinfo DE
 * KOS y la VRAM libre del PVR, (c) vid_set_mode/vid_mode (el modo de video), ni (d) pvr_txr_load_ex ni
 * pvr_set_pal_entry, que son las dos piezas del PVR que hacen falta para 1 byte por pixel + paleta de
 * 256 en hardware. El patron es el de los POCs de DC-001.4 (pal8) y DC-002.9 (motor-pvr): gccgo llama
 * a C con el comentario //extern y los punteros cruzan como uintptr.
 *
 * Lo que agrega este POC: los datos son del motor (d2dc6.Load + DecodeFrame y d2dat.Load, los mismos
 * bytes crudos del MPQ), la textura es PAL8 con la paleta del motor en el banco 0, y ademas de pintar
 * se MIDEN el heap de KOS antes y despues de cargarlo, la VRAM, y el frame con 300 sprites en dos
 * modos de video (el que quedo en efecto se lee de vid_mode, no se supone).
 */
#include <kos.h>
#include <malloc.h>
#include <stdlib.h>
#include <string.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

/* Microsegundos desde el arranque: el reloj con el que se mide el frame (la misma fuente que usa el
 * POC de la stdlib). Los contadores de rendimiento del SH-4 NO son la fuente: en el emulador no estan
 * implementados y dan 0, asi que el POC los lee y los reporta aparte. */
unsigned long long d2c_us(void) {
    return (unsigned long long)timer_us_gettime64();
}

/* El heap de KOS (newlib): arena = lo que hay de heap, fordblks = libre, uordblks = en uso. Es el
 * numero que responde "cuanta RAM de los 16 MB queda con el motor cargado". */
unsigned int d2c_mem_arena(void) { return (unsigned int)mallinfo().arena; }
unsigned int d2c_mem_libre(void) { return (unsigned int)mallinfo().fordblks; }
unsigned int d2c_mem_usada(void) { return (unsigned int)mallinfo().uordblks; }

/* La VRAM del PVR (8 MB, aparte de los 16 de RAM principal). */
unsigned int d2c_vram_libre(void) { return (unsigned int)pvr_mem_available(); }

/* El modo de video. d2c_modo pide el cambio y REINICIA el PVR (el cambio de modo rearma la VRAM, asi
 * que el llamador tiene que volver a subir su textura). d2c_modo_ancho/alto leen el modo EN EFECTO:
 * eso es lo que se imprime, para no confundir "se pidio" con "quedo". */
int d2c_modo(int dm, int pm) {
    pvr_shutdown();
    /* vid_set_mode del KOS toma el modo por int y el pixel mode por su enum */
    vid_set_mode(dm, (vid_pixel_mode_t)pm);
    return (int)pvr_init_defaults();
}

int d2c_modo_ancho(void) { return (vid_mode != NULL) ? (int)vid_mode->width : -1; }
int d2c_modo_alto(void) { return (vid_mode != NULL) ? (int)vid_mode->height : -1; }

static pvr_ptr_t tex = NULL;
static pvr_poly_hdr_t hdr;
static int W = 0, H = 0, PW = 0, PH = 0;
static float u0 = 0.0f, v0 = 0.0f, u1 = 0.0f, v1 = 0.0f;

int d2c_pvr_init(unsigned int indices, unsigned int palRGBA, int w, int h) {
    const unsigned char *idx = (const unsigned char *)indices;
    const unsigned char *pal = (const unsigned char *)palRGBA;

    if (idx == NULL || pal == NULL || w <= 0 || h <= 0) {
        return -1;
    }

    /* se puede llamar dos veces (cada modo de video): se libera la textura anterior, si la hay */
    if (tex != NULL) {
        pvr_mem_free(tex);
        tex = NULL;
    }

    W = w;
    H = h;

    /* 1) la paleta del motor (256 RGBA de 8 bits) al banco 0 del PVR */
    pvr_set_pal_format(PVR_PAL_ARGB1555);
    for (int i = 0; i < 256; i++) {
        unsigned int r = pal[i * 4 + 0];
        unsigned int g = pal[i * 4 + 1];
        unsigned int b = pal[i * 4 + 2];
        /* el indice 0 es transparente (la convencion de Diablo II y la del motor), el resto opaco */
        unsigned int alpha = (i == 0) ? 0u : 1u;
        pvr_set_pal_entry(i, (uint16)((alpha << 15) | ((r >> 3) << 10) | ((g >> 3) << 5) | (b >> 3)));
    }

    /* 2) la textura de 8 bits: un byte por pixel, rellenada a potencia de dos (lo exige el PVR) */
    PW = 1;
    while (PW < w) PW <<= 1;
    PH = 1;
    while (PH < h) PH <<= 1;

    tex = pvr_mem_malloc((size_t)PW * (size_t)PH);
    if (tex == NULL) {
        return -2;
    }

    unsigned char *tmp = (unsigned char *)calloc((size_t)PW * (size_t)PH, 1);
    if (tmp == NULL) {
        return -3;
    }

    for (int y = 0; y < h; y++) {
        memcpy(tmp + (size_t)y * (size_t)PW, idx + (size_t)y * (size_t)w, (size_t)w);
    }

    pvr_txr_load_ex(tmp, tex, PW, PH, PVR_TXRLOAD_8BPP);
    free(tmp);

    /* 3) contexto: lista translucida + alpha de textura, para que el indice 0 se vea como hueco */
    pvr_poly_cxt_t cxt;
    pvr_poly_cxt_txr(&cxt, PVR_LIST_TR_POLY,
                     PVR_TXRFMT_PAL8BPP | PVR_TXRFMT_8BPP_PAL(0) | PVR_TXRFMT_TWIDDLED,
                     PW, PH, tex, PVR_FILTER_NONE);
    cxt.txr.alpha = PVR_TXRALPHA_ENABLE;
    cxt.blend.src = PVR_BLEND_SRCALPHA;
    cxt.blend.dst = PVR_BLEND_INVSRCALPHA;
    cxt.blend.src_enable = PVR_BLEND_ENABLE;
    cxt.blend.dst_enable = PVR_BLEND_ENABLE;
    pvr_poly_compile(&hdr, &cxt);

    /* las UV cubren solo el arte visible dentro de la textura rellenada, normalizadas */
    u0 = 0.5f / (float)PW;
    v0 = 0.5f / (float)PH;
    u1 = ((float)W - 0.5f) / (float)PW;
    v1 = ((float)H - 0.5f) / (float)PH;

    return 0;
}

/* Dibuja la textura en (x, y) con escala s. El llamador tiene que tener abierta PVR_LIST_TR_POLY.
 * Un sprite = una cabecera de textura + 4 vertices, que es lo que cuesta cada sprite del motor. */
void d2c_pvr_quad(float x, float y, float s, unsigned int argb) {
    pvr_vertex_t v;

    pvr_prim(&hdr, sizeof(hdr));

    v.flags = PVR_CMD_VERTEX;
    v.argb = argb;
    v.oargb = 0;
    v.z = 1.0f;

    v.u = u0;   v.v = v0;   v.x = x;                v.y = y;                pvr_prim(&v, sizeof(v));
    v.u = u1;   v.v = v0;   v.x = x + W * s;        v.y = y;                pvr_prim(&v, sizeof(v));
    v.u = u0;   v.v = v1;   v.x = x;                v.y = y + H * s;        pvr_prim(&v, sizeof(v));

    v.flags = PVR_CMD_VERTEX_EOL;
    v.u = u1;   v.v = v1;   v.x = x + W * s;        v.y = y + H * s;        pvr_prim(&v, sizeof(v));
}

int d2c_pvr_ancho(void) { return W; }
int d2c_pvr_alto(void) { return H; }

/* Cuanta RAM PRINCIPAL le queda al programa (que es la pregunta de la tarjeta): se piden bloques de
 * 1 MB a malloc hasta que uno falle y se suman, y despues se liberan. Es la unica forma de contestar
 * "cuanto de los 16 MB queda" sin suponer cuanto se reserva el sistema ni el runtime, y se mide CON EL
 * MOTOR CARGADO (el POC ya decodifico el DC6 y la paleta y el PVR ya tiene la textura). Ojo: es una
 * medida del heap de newlib, o sea de lo que el programa puede pedir por malloc; no incluye lo que el
 * ELF ya ocupa (eso se mide en el host con sh-elf-size). */
unsigned int d2c_ram_tope(void) {
    const size_t bloque = 1024u * 1024u;
    const size_t maximo = 64u; /* tope de cordura: 64 bloques = 64 MB, mas que la RAM de la consola */
    void *punteros[64];
    size_t n = 0;
    unsigned int total = 0;

    while (n < maximo) {
        void *p = malloc(bloque);
        if (p == NULL) {
            break;
        }
        punteros[n++] = p;
        total += (unsigned int)bloque;
    }

    for (size_t i = 0; i < n; i++) {
        free(punteros[i]);
    }

    return total;
}
