/* DC-001.4: textura paletizada (PAL8 + paleta por hardware del PVR) con un asset real de
 * Diablo II horneado en build (gen/d2logo.h).
 *
 * Por que en C: el binding de libgodc no expone pvr_txr_load_ex ni pvr_set_pal_entry, que son
 * justo las dos piezas de la textura paletizada. Es el mismo patron que la consola serie:
 * gccgo llama a C con el comentario //extern.
 *
 * Lo que prueba:
 *   - indices de 8 bits en VRAM (1 byte por pixel) en vez de 16 (ARGB1555) -> la mitad de VRAM
 *   - la paleta del PowerVR2 (256 entradas ARGB1555 en hardware, banco 0)
 *   - transparencia por indice 0 (la convencion de Diablo II y la del motor)
 */
#include <kos.h>
#include <stdio.h>
#include "d2logo.h" /* generado por scripts/console/bake-asset.sh en <build>/gen */

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

static pvr_ptr_t tex = NULL;
static pvr_poly_hdr_t hdr;
/* UV normalizadas (0..1): asi las quiere el PVR para texturas twiddled. El 0.5 de mas es
 * medio texel, para no muestrear el relleno del borde (ver examples/dreamcast/sound/
 * ghettoplay-vorbis/bkg.c de KOS, que usa u de 0.0 a 1.0). */
static float u0 = 0.0f, v0 = 0.0f, u1 = 0.0f, v1 = 0.0f;

int d2c_pal8_init(void) {
    /* 1) paleta por hardware: banco 0 con las 256 entradas ARGB1555 del asset */
    pvr_set_pal_format(PVR_PAL_ARGB1555);
    for (int i = 0; i < 256; i++) {
        pvr_set_pal_entry(i, d2logo_pal[i]);
    }

    /* 2) textura 8 bpp: un byte por pixel; el PVR la twiddlea al cargarla. El tamano de la
     *    textura tiene que ser potencia de dos (lo exige KOS y el propio PVR), asi que se
     *    sube rellenada (PW x PH) y el quad dibuja solo el arte visible (W x H). */
    tex = pvr_mem_malloc(D2LOGO_PW * D2LOGO_PH);
    if (tex == NULL) {
        return -1;
    }
    pvr_txr_load_ex(d2logo_idx, tex, D2LOGO_PW, D2LOGO_PH, PVR_TXRLOAD_8BPP);

    /* 3) contexto de poligono: lista translucida + alpha de textura, para que el indice 0
     *    se vea por transparencia en vez de como color */
    pvr_poly_cxt_t cxt;
    pvr_poly_cxt_txr(&cxt, PVR_LIST_TR_POLY,
                     PVR_TXRFMT_PAL8BPP | PVR_TXRFMT_8BPP_PAL(0) | PVR_TXRFMT_TWIDDLED,
                     D2LOGO_PW, D2LOGO_PH, tex, PVR_FILTER_NONE);
    cxt.txr.alpha = PVR_TXRALPHA_ENABLE;
    cxt.blend.src = PVR_BLEND_SRCALPHA;
    cxt.blend.dst = PVR_BLEND_INVSRCALPHA;
    cxt.blend.src_enable = PVR_BLEND_ENABLE;
    cxt.blend.dst_enable = PVR_BLEND_ENABLE;
    pvr_poly_compile(&hdr, &cxt);

    /* las UV cubren solo el arte visible dentro de la textura rellenada, normalizadas */
    u0 = 0.5f / (float)D2LOGO_PW;
    v0 = 0.5f / (float)D2LOGO_PH;
    u1 = ((float)D2LOGO_W - 0.5f) / (float)D2LOGO_PW;
    v1 = ((float)D2LOGO_H - 0.5f) / (float)D2LOGO_PH;

    return 0;
}

/* Dibuja el asset en (x, y) con escala s. El llamador tiene que tener abierta la lista
 * PVR_LIST_TR_POLY. */
void d2c_pal8_quad(float x, float y, float s, unsigned int argb) {
    pvr_vertex_t v;

    pvr_prim(&hdr, sizeof(hdr));

    v.flags = PVR_CMD_VERTEX;
    v.argb = argb;
    v.oargb = 0;
    v.z = 1.0f;

    v.u = u0;   v.v = v0;   v.x = x;                    v.y = y;                    pvr_prim(&v, sizeof(v));
    v.u = u1;   v.v = v0;   v.x = x + D2LOGO_W * s;     v.y = y;                    pvr_prim(&v, sizeof(v));
    v.u = u0;   v.v = v1;   v.x = x;                    v.y = y + D2LOGO_H * s;     pvr_prim(&v, sizeof(v));

    v.flags = PVR_CMD_VERTEX_EOL;
    v.u = u1;   v.v = v1;   v.x = x + D2LOGO_W * s;     v.y = y + D2LOGO_H * s;     pvr_prim(&v, sizeof(v));
}

int d2c_asset_w(void) { return D2LOGO_W; }
int d2c_asset_h(void) { return D2LOGO_H; }
