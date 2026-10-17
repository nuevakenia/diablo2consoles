/* DC-002.9: la textura del motor subida al PVR como PAL8 (1 byte por pixel) con la paleta del motor en
 * el banco 0 del hardware.
 *
 * Por que en C: el binding de libgodc no expone pvr_txr_load_ex ni pvr_set_pal_entry, que son las dos
 * piezas del PVR que hacen falta para 1 byte por pixel + paleta de 256 en hardware. Es el mismo patron
 * que el POC de DC-001.4 (csrc/cshim.c de pal8) y el de la consola serie: gccgo llama a C con el
 * comentario //extern, y los punteros cruzan como uintptr.
 *
 * Lo que agrega DC-002.9: LOS DATOS SON DEL MOTOR. Los indices salen de d2dc6.DecodeFrame y la paleta
 * de d2dat.Load; aca solo se convierte el formato (los RGBA de 8 bits del motor a las 256 entradas
 * ARGB1555 del PVR, con la misma conversion que usa el repo: alpha<<15 | (R>>3)<<10 | (G>>3)<<5 |
 * (B>>3) y el indice 0 transparente) y se sube la textura.
 */
#include <kos.h>
#include <stdlib.h>
#include <string.h>

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

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

    /* 2) la textura de 8 bits: un byte por pixel, rellenada a potencia de dos (lo exige el PVR). El
     *    quad dibuja solo el arte visible (W x H) de la textura rellenada (PW x PH). */
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

    /* 3) contexto: lista translucida + alpha de textura, para que el indice 0 se vea como hueco y no
     *    como un color mas */
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

/* Dibuja la textura en (x, y) con escala s. El llamador tiene que tener abierta PVR_LIST_TR_POLY. */
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
