/* DC-002.69: el SPRITE PALETIZADO de 8 bits en el PVR -- la cara de C de d2render/pvr.
 *
 * QUE HACE: sube a la VRAM los indices de 8 bits de un sprite (un DC6/COF: 1 byte por pixel) y su paleta de
 * 256 colores ARGB1555, y lo dibuja como un quad texturado. La paleta va al BANCO 0 del hardware, que es el
 * que usa la textura (PVR_TXRFMT_8BPP_PAL(0)).
 *
 * POR QUE EN C: el binding de Go de libgodc NO expone pvr_txr_load_ex (la subida de una textura
 * paletizada) ni pvr_set_pal_entry (las entradas de la paleta). Es el mismo patron que el shim del POC
 * (console/dc/poc/personaje-mapa/csrc/cshim.c:79) y el de la pantalla: gccgo llama a C con el comentario
 * //extern y los punteros cruzan como uintptr.
 *
 * EL RELLENO LO HACE EL GO, Y ESTA ES LA DECISION DE ESTE ARCHIVO QUE MAS IMPORTA: una textura del PVR tiene
 * que medir una POTENCIA DE DOS en las dos dimensiones (con una de 118x52, pvr_poly_compile aborta con
 * "Invalid texture U size"), asi que un sprite de 118x52 se sube como 128x64. Rellenar aca pidiendo un
 * buffer temporal costaria hasta 256 KB de .bss (o del monton) POR SPRITE; rellenando en Go es un slice
 * temporal del tamano de la textura, que el recolector libera solo. Este C recibe `pw x ph` ya rellenado y
 * sube lo que le dan.
 *
 * LAS TRES COSAS QUE ESTAN RESUELTAS ACA Y COSTARON UNA TARDE CADA UNA (medidas, no supuestas):
 *   1. la potencia de dos (arriba);
 *   2. las UV de las texturas twiddled van NORMALIZADAS (0..1), no en pixeles: en pixeles el PVR repite el
 *      patron y se ve cualquier cosa;
 *   3. el PVR dibuja TIRAS: el quad son cuatro vertices en zigzag y la marca de fin de tira
 *      (PVR_CMD_VERTEX_EOL) va en el CUARTO, no en el tercero;
 *   4. medio texel de recorte en las UV, para no muestrear el relleno de la potencia de dos.
 */
#include <kos.h>
#include <stdint.h>
#include <stdio.h>

static pvr_ptr_t sprite_tex = NULL;
static pvr_poly_hdr_t sprite_hdr;
static int sprite_bytes = 0;                    /* lo que ocupa en la VRAM (0 = no hay textura) */
static int sprite_ancho = 0, sprite_alto = 0;   /* los del asset (el pedazo visible) */
static int sprite_pw = 0, sprite_ph = 0;        /* los de la textura (potencia de dos) */

/* Sube los indices (8 bits/pixel, YA rellenados a pw x ph) y la paleta (256 x ARGB1555) a la VRAM.
 * Devuelve los bytes de VRAM que ocupa, o un codigo negativo: -1 argumentos, -2 la textura no entra,
 * -3 tamano invalido.
 *
 * UNA SOLA TEXTURA A LA VEZ: el banco de paleta 0 es uno, y esta funcion lo deja con la paleta del sprite
 * que sube. Subir otro libera el anterior (d2render/pvr lo dice en SubirSpritePVR). */
int d2c_pvr_sprite_subir(unsigned int idx, unsigned int pal, int w, int h, int pw, int ph) {
    const unsigned char *indices = (const unsigned char *)(uintptr_t)idx;
    const unsigned short *paleta = (const unsigned short *)(uintptr_t)pal;

    if (indices == NULL || paleta == NULL || w <= 0 || h <= 0 || pw < w || ph < h) {
        return -1;
    }

    if (pw > 1024 || ph > 1024 || (pw & (pw - 1)) != 0 || (ph & (ph - 1)) != 0) {
        printf("PVR-SPRITE textura invalida (%dx%d)\n", pw, ph);
        return -3;
    }

    /* la paleta del hardware, banco 0 */
    pvr_set_pal_format(PVR_PAL_ARGB1555);
    for (int i = 0; i < 256; i++) {
        pvr_set_pal_entry(i, paleta[i]);
    }

    if (sprite_tex != NULL) {
        pvr_mem_free(sprite_tex);
        sprite_tex = NULL;
        sprite_bytes = 0;
    }

    sprite_tex = pvr_mem_malloc(pw * ph);
    if (sprite_tex == NULL) {
        printf("PVR-SPRITE sin VRAM para %d bytes\n", pw * ph);
        return -2;
    }

    pvr_txr_load_ex(indices, sprite_tex, pw, ph, PVR_TXRLOAD_8BPP);

    /* el contexto: lista TRANSPARENTE (un sprite se compone sobre lo que hay), textura paletizada 8bpp en
     * el banco 0 y sin filtro (pixel exacto: el arte de 1998 ya viene al tamano que se ve) */
    pvr_poly_cxt_t cxt;
    pvr_poly_cxt_txr(&cxt, PVR_LIST_TR_POLY,
        PVR_TXRFMT_PAL8BPP | PVR_TXRFMT_8BPP_PAL(0) | PVR_TXRFMT_TWIDDLED,
        pw, ph, sprite_tex, PVR_FILTER_NONE);
    cxt.txr.alpha = PVR_TXRALPHA_ENABLE;    /* el indice 0 (alfa 0) es transparente */
    cxt.blend.src = PVR_BLEND_SRCALPHA;
    cxt.blend.dst = PVR_BLEND_INVSRCALPHA;
    cxt.blend.src_enable = PVR_BLEND_ENABLE;
    cxt.blend.dst_enable = PVR_BLEND_ENABLE;
    pvr_poly_compile(&sprite_hdr, &cxt);

    sprite_ancho = w;
    sprite_alto = h;
    sprite_pw = pw;
    sprite_ph = ph;
    sprite_bytes = pw * ph;

    return sprite_bytes;
}

/* Dibuja el sprite YA SUBIDO en (x, y) con la escala pedida. El llamador tiene que tener abierta la lista
 * TRANSPARENTE (PVR_LIST_TR_POLY): aca no se abre ni se cierra nada, porque el ciclo de cuadros es del
 * Renderer y no de este backend.
 *
 * OJO: `escala` multiplica el tamano VISIBLE (no las UV): el sprite se agranda en pantalla, no se muestrea
 * mas grande de la textura. */
void d2c_pvr_sprite_dibujar_go(float x, float y, float ancho, float alto, float escala) {
    pvr_vertex_t v;
    float u0, v0, u1, v1;

    if (sprite_bytes == 0) {
        return;
    }

    /* medio texel adentro: asi el muestreo no toca el relleno de la potencia de dos */
    u0 = 0.5f / (float)sprite_pw;
    v0 = 0.5f / (float)sprite_ph;
    u1 = (float)sprite_ancho / (float)sprite_pw - 0.5f / (float)sprite_pw;
    v1 = (float)sprite_alto / (float)sprite_ph - 0.5f / (float)sprite_ph;

    ancho *= escala;
    alto *= escala;

    pvr_prim(&sprite_hdr, sizeof(sprite_hdr));

    v.argb = 0xFFFFFFFF;
    v.oargb = 0;
    v.z = 1.0f;

    v.flags = PVR_CMD_VERTEX;
    v.u = u0;   v.v = v0;   v.x = x;         v.y = y;         pvr_prim(&v, sizeof(v));
    v.u = u1;   v.v = v0;   v.x = x + ancho; v.y = y;         pvr_prim(&v, sizeof(v));
    v.u = u0;   v.v = v1;   v.x = x;         v.y = y + alto;  pvr_prim(&v, sizeof(v));
    v.flags = PVR_CMD_VERTEX_EOL;
    v.u = u1;   v.v = v1;   v.x = x + ancho; v.y = y + alto;  pvr_prim(&v, sizeof(v));
}

/* Devuelve la textura a la VRAM. */
void d2c_pvr_sprite_liberar(void) {
    if (sprite_tex != NULL) {
        pvr_mem_free(sprite_tex);
        sprite_tex = NULL;
    }

    sprite_bytes = 0;
}
