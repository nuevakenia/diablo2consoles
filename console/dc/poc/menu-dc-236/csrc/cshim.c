/* DC-002.36: el compositor del MENU PRINCIPAL en la Dreamcast -- varias texturas paletizadas (8bpp) con
 * TRES paletas por hardware (los 4 bancos de 256 del PVR) y quads en posiciones distintas, con
 * transparencia (el indice 0 de la paleta) y blending NORMAL o ADITIVO (el fuego del logo).
 *
 * POR QUE EN C: el binding de Go de libgodc no expone pvr_txr_load_ex, pvr_set_pal_entry, pvr_prim ni
 * pvr_poly_compile, que son los que suben una textura paletizada y compilan un header. Es el mismo shim
 * del POC de DC-002.35 (una textura, un quad opaco) llevado a N texturas, 3 bancos de paleta y dos
 * modos de mezcla.
 *
 * LO QUE YA ESTABA RESUELTO Y SE RESPETA (ver la skill dreamcast-godc, cada punto costo una tarde):
 *   1. el lado de la textura tiene que ser POTENCIA DE DOS: la consola sube el lienzo rellenado con
 *      indice 0 y dibuja solo el pedazo visible, con las UV recortadas medio texel;
 *   2. las UV de las texturas twiddled van NORMALIZADAS (0..1), no en pixeles;
 *   3. el PVR dibuja TIRAS: el quad son cuatro vertices en zigzag y la marca de fin de tira
 *      (PVR_CMD_VERTEX_EOL) va en el CUARTO, no en el tercero.
 */
#include <kos.h>
#include <stdint.h>
#include <string.h>

#define MAX_TEX 16

static pvr_ptr_t tex[MAX_TEX];
static pvr_poly_hdr_t hdr_normal[MAX_TEX];
static pvr_poly_hdr_t hdr_aditivo[MAX_TEX];
static int tex_w[MAX_TEX];
static int tex_h[MAX_TEX];
static int hay[MAX_TEX];

/* d2c_serial_init: la consola serie es la UNICA salida de texto (sin esto KOS deja el handler en NULL y
 * el println de Go no sale a ninguna parte: medido en DC-001). */
void d2c_serial_init(void) {
    dbgio_dev_select("scif");
}

/* d2c_ms: milisegundos desde que arranco el programa (los contadores del SH-4 devuelven 0 en Flycast). */
unsigned int d2c_ms(void) {
    static int encendido = 0;

    if (!encendido) {
        timer_ms_enable();
        encendido = 1;
    }

    return (unsigned int)(timer_ms_gettime64() & 0xffffffffu);
}

/* d2c_vram_libre: la VRAM que queda (la mide el POC desde DC-002.9). */
unsigned int d2c_vram_libre(void) {
    return (unsigned int)pvr_mem_available();
}

/* d2c_pal: carga los 256 colores ARGB1555 de un BANCO. El PVR tiene 1024 entradas = 4 bancos de 256 y
 * cada textura elige el suyo con PVR_TXRFMT_8BPP_PAL(n): asi conviven en el mismo cuadro el fondo (sky),
 * los logos/botones (units) y el texto (static) sin remapear ni un indice. */
void d2c_pal(int banco, const unsigned short *entradas) {
    for (int i = 0; i < 256; i++) {
        pvr_set_pal_entry((uint32_t)(banco * 256 + i), entradas[i]);
    }
}

/* d2c_tex: sube el lienzo de indices de 8 bits (ya recortado a w x h y con el relleno de indice 0 hasta
 * pw x ph) y compila los DOS headers: normal (src alpha) y aditivo (el fuego del logo).
 * Devuelve 0 si todo bien, 1 si el id esta fuera de tabla, 2 si no hay VRAM. */
int d2c_tex(int id, const unsigned char *idx, int w, int h, int pw, int ph, int banco) {
    if (id < 0 || id >= MAX_TEX || idx == NULL) {
        return 1;
    }

    tex[id] = pvr_mem_malloc(pw * ph);
    if (tex[id] == NULL) {
        return 2;
    }

    tex_w[id] = pw;
    tex_h[id] = ph;
    hay[id] = 1;

    /* el twiddle y la subida: pvr_txr_load_ex con 8bpp (los flags SQ/DMA no hacen nada: medido) */
    pvr_txr_load_ex(idx, tex[id], pw, ph, PVR_TXRLOAD_8BPP);

    pvr_poly_cxt_t cxt;

    /* --- normal: src*alpha + dst*(1-alpha), y el indice 0 (alpha 0 en la paleta) no pinta --- */
    pvr_poly_cxt_txr(&cxt, PVR_LIST_TR_POLY,
        PVR_TXRFMT_PAL8BPP | PVR_TXRFMT_8BPP_PAL(banco) | PVR_TXRFMT_TWIDDLED,
        pw, ph, tex[id], PVR_FILTER_NONE);
    cxt.txr.alpha = PVR_TXRALPHA_ENABLE;
    cxt.blend.src = PVR_BLEND_SRCALPHA;
    cxt.blend.dst = PVR_BLEND_INVSRCALPHA;
    cxt.blend.src_enable = PVR_BLEND_ENABLE;
    cxt.blend.dst_enable = PVR_BLEND_ENABLE;
    pvr_poly_compile(&hdr_normal[id], &cxt);

    /* --- aditivo: el color del pixel se SUMA al de abajo (el fuego del logo, DrawEffectModulate) --- */
    pvr_poly_cxt_txr(&cxt, PVR_LIST_TR_POLY,
        PVR_TXRFMT_PAL8BPP | PVR_TXRFMT_8BPP_PAL(banco) | PVR_TXRFMT_TWIDDLED,
        pw, ph, tex[id], PVR_FILTER_NONE);
    cxt.txr.alpha = PVR_TXRALPHA_ENABLE;
    cxt.blend.src = PVR_BLEND_SRCALPHA;
    cxt.blend.dst = PVR_BLEND_ONE;
    cxt.blend.src_enable = PVR_BLEND_ENABLE;
    cxt.blend.dst_enable = PVR_BLEND_ENABLE;
    pvr_poly_compile(&hdr_aditivo[id], &cxt);

    return 0;
}

/* d2c_quad: dibuja el pedazo (u0,v0)-(u1,v1) del lienzo de `id` en (x,y) con tamano (w,h), pintado con
 * el color `argb` (el PVR multiplica el color del vertice por el de la paleta: asi se pinta el texto con
 * el color de la etiqueta). Las UV van en PIXELES de la textura: aca se normalizan y se les recorta medio
 * texel, que es lo que evita muestrear el relleno. */
void d2c_quad(int id, float x, float y, float w, float h, float u0, float v0, float u1, float v1,
              unsigned int argb, int modo) {
    pvr_vertex_t v;

    if (id < 0 || id >= MAX_TEX || !hay[id]) {
        return;
    }

    float tw = (float)tex_w[id];
    float th = (float)tex_h[id];

    float nu0 = (u0 + 0.5f) / tw;
    float nv0 = (v0 + 0.5f) / th;
    float nu1 = (u1 - 0.5f) / tw;
    float nv1 = (v1 - 0.5f) / th;

    pvr_prim(modo ? &hdr_aditivo[id] : &hdr_normal[id], sizeof(pvr_poly_hdr_t));

    v.argb = argb;
    v.oargb = 0;
    v.z = 1.0f;

    v.flags = PVR_CMD_VERTEX;
    v.u = nu0; v.v = nv0; v.x = x;         v.y = y;         pvr_prim(&v, sizeof(v));
    v.u = nu1; v.v = nv0; v.x = x + w;     v.y = y;         pvr_prim(&v, sizeof(v));
    v.u = nu0; v.v = nv1; v.x = x;         v.y = y + h;     pvr_prim(&v, sizeof(v));
    v.flags = PVR_CMD_VERTEX_EOL;
    v.u = nu1; v.v = nv1; v.x = x + w;     v.y = y + h;     pvr_prim(&v, sizeof(v));
}

/* d2c_tex_libre: la VRAM que ocupan las texturas subidas (para poder restarla de PvrMemAvailable y
 * declarar cuanto pide el menu). */
unsigned int d2c_tex_bytes(void) {
    unsigned int t = 0;

    for (int i = 0; i < MAX_TEX; i++) {
        if (hay[i]) {
            t += (unsigned int)(tex_w[i] * tex_h[i]);
        }
    }

    return t;
}

/* --- EL CONTROL: dos envoltorios finos sobre los shims de la stdlib (console/dc/std/csrc/control_dc.c).
 *
 * Por que envoltorios: el shim de la stdlib pide SIETE punteros (los botones y los seis ejes), y pasar
 * la direccion de un entero suelto desde Go obliga a guardar un unsafe.Pointer, que el runtime de la
 * consola no soporta (el enlace falla con "undefined reference to unsafe.Pointer..p": medido). Con un
 * envoltorio que recibe UN arreglo de int32, Go pasa &arreglo[0] y listo.
 */
int d2c_control_leer(unsigned int *botones, int *palanca_x, int *palanca_y,
                     int *gatillo_l, int *gatillo_r, int *palanca2_x, int *palanca2_y);

int d2c_control_buscar(void);

/* d2c_control_hay: 1 si hay control en el puerto 0. */
int d2c_control_hay(void) {
    return d2c_control_buscar();
}

/* d2c_control_todo: lee el control y deja los BOTONES en *todos (int32[0] del lado de Go).
 * Devuelve 0 si leyo, -1 si no hay control, -2 si no se pudo leer. */
int d2c_control_todo(int *todos) {
    unsigned int botones = 0;
    int px, py, gl, gr, p2x, p2y;

    int rc = d2c_control_leer(&botones, &px, &py, &gl, &gr, &p2x, &p2y);
    if (rc == 0 && todos != NULL) {
        *todos = (int)botones;
    }

    return rc;
}
