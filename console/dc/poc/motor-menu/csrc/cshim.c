/* DC-002.9: un asset grande del juego (el fondo del menu) pintado por el PVR.
 *
 * Por que en C: el binding de Go de libgodc no expone pvr_txr_load_ex ni pvr_set_pal_entry, que son
 * los que suben una textura paletizada. El resto (decodificar el DC6) lo hace el CODIGO DEL MOTOR,
 * importado tal cual en la consola.
 *
 * Las dos cosas que cuestan una tarde cada una (medidas en DC-001.4) y que acá ya estan resueltas:
 *   1. el lado de la textura tiene que ser POTENCIA DE DOS: el asset se sube rellenado y el quad
 *      dibuja solo el pedazo visible (W x H reales, con las UV recortadas medio texel).
 *   2. las UV de las texturas twiddled van NORMALIZADAS (0..1), no en pixeles.
 */
#include <kos.h>
#include <stdio.h>
#include <stdint.h>
#include "menu.h" /* generado por scripts/console/bake-asset.sh en <build>/gen */

void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

static pvr_ptr_t tex = NULL;
static pvr_poly_hdr_t hdr;
static int ancho = 0, alto = 0;     /* los del asset */
static int tex_w = 0, tex_h = 0;    /* los de la textura (potencia de dos) */

/* d2c_menu_init: sube los indices de 8 bits y la paleta del asset. Devuelve 0 si todo bien. */
int d2c_menu_init(void) {
    pvr_set_pal_format(PVR_PAL_ARGB1555);
    for (int i = 0; i < 256; i++) {
        pvr_set_pal_entry(i, menu_pal[i]);
    }

    ancho = MENU_W;
    alto = MENU_H;
    /* MENU_PW / MENU_PH son la textura ya redondeada a potencia de dos y rellenada con indice 0, que
     * las calcula el horneador (bake-asset.sh) al generar el .h: acá no se recalcula. */
    tex_w = MENU_PW;
    tex_h = MENU_PH;

    if (tex_w > 1024 || tex_h > 1024) {
        printf("POC-MENU: textura demasiado grande (%dx%d)\n", tex_w, tex_h);
        return 1;
    }

    tex = pvr_mem_malloc(tex_w * tex_h);
    if (!tex) {
        printf("POC-MENU: sin VRAM para %d bytes\n", tex_w * tex_h);
        return 2;
    }
    pvr_txr_load_ex(menu_idx, tex, tex_w, tex_h, PVR_TXRLOAD_8BPP);

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

/* d2c_menu_quad: dibuja el asset en (x,y), opcionalmente escalado por s. */
void d2c_menu_quad(float x, float y, float s, unsigned int argb) {
    pvr_vertex_t v;
    float u0 = 0.5f / tex_w, v0 = 0.5f / tex_h;
    float u1 = (float)ancho / tex_w - 0.5f / tex_w;
    float v1 = (float)alto / tex_h - 0.5f / tex_h;

    (void)argb;
    pvr_prim(&hdr, sizeof(hdr));

    v.flags = PVR_CMD_VERTEX;
    v.argb = 0xFFFFFFFF;
    v.oargb = 0;
    v.z = 1.0f;
    v.u = u0;   v.v = v0;   v.x = x;                v.y = y;                pvr_prim(&v, sizeof(v));
    v.u = u1;   v.v = v0;   v.x = x + ancho * s;    v.y = y;                pvr_prim(&v, sizeof(v));
    v.flags = PVR_CMD_VERTEX_EOL;
    v.u = u0;   v.v = v1;   v.x = x;                v.y = y + alto * s;     pvr_prim(&v, sizeof(v));
    v.flags = PVR_CMD_VERTEX;
    v.u = u1;   v.v = v1;   v.x = x + ancho * s;    v.y = y + alto * s;     pvr_prim(&v, sizeof(v));
}

/* Estas cuatro las puede llamar el Go ANTES de subir la textura (para anunciar el tamano), asi que
 * devuelven las medidas del asset horneado, no las variables que recien se llenan en init. */
int d2c_menu_w(void) { return MENU_W; }
int d2c_menu_h(void) { return MENU_H; }
int d2c_menu_texw(void) { return MENU_PW; }
int d2c_menu_texh(void) { return MENU_PH; }
