/* DC-002.10: la PANTALLA del motor subida a la VRAM del PVR y pintada a pantalla completa.
 *
 * POR QUE ESTA ACA Y NO EN UN POC: esto es el backend de video de la consola, o sea la pieza que reemplaza
 * a Ebitengine en la Dreamcast. El motor dibuja sus cuadros en una imagen RGBA de 8 bits por canal (lo que
 * hace Ebitengine en la PC) y aca se convierten a lo que el hardware entiende: RGB565 (2 bytes por pixel),
 * se suben a la VRAM y se pintan ocupando los 640x480.
 *
 * POR QUE EN C: el binding de libgodc no expone pvr_txr_load_ex (la subida de una textura con su twiddle),
 * que es la pieza que hace falta. Es el mismo patron que el shim del POC de DC-002.9 y el de la consola
 * serie: gccgo llama a C con el comentario //extern y los punteros cruzan como uintptr.
 *
 * LA CONVERSION (RGBA -> RGB565) es la de siempre: el rojo y el azul se quedan con 5 bits y el verde con 6
 * (el ojo distingue mas el verde, y el hardware tiene el verde en el medio: 5-6-5). Ojo, NO es la misma que
 * la del POC de DC-002.9: esa arma una PALETA (ARGB1555, con transparencia) y esta arma los pixeles de la
 * pantalla (RGB565, sin transparencia, porque la pantalla se ve entera). Confundirlas da colores cambiados
 * -- el rojo y el azul se permutan.
 *
 * LA PANTALLA ES 640x480 Y LA TEXTURA ES 1024x512 (potencia de dos, como exige el PVR), asi que se pinta
 * solo la parte de arriba a la izquierda: las coordenadas de textura van por el cociente (640/1024 y
 * 480/512), no por 1. Si se pintara con 1, el cuadro saldria estirado y con basura en el borde.
 */
#include <kos.h>
#include <stdlib.h>
#include <string.h>

/* Los prototipos de las dos partes (el C las necesita ANTES: `subir` -- la funcion de siempre -- usa a las
 * dos, y estan definidas mas abajo). */
int d2c_pvr_pantalla_convertir(unsigned int rgba, int w, int h);
int d2c_pvr_pantalla_subir_vram(void);

static pvr_ptr_t pant_tex = NULL;
static pvr_poly_hdr_t pant_hdr;
static uint16_t *pant_conv = NULL;
static int PANT_W = 0, PANT_H = 0, PANT_PW = 0, PANT_PH = 0;

/* Arranca el video y deja la textura de la pantalla lista. Se llama UNA vez, antes del loop.
 * Devuelve 0 si anduvo; negativo si no (sin reserva de VRAM, sin memoria, tamanos invalidos). */
int d2c_pvr_pantalla_init(int w, int h) {
    if (w <= 0 || h <= 0) {
        return -1;
    }

    PANT_W = w;
    PANT_H = h;

    /* la textura tiene que ser potencia de dos en las dos dimensiones */
    PANT_PW = 1;
    while (PANT_PW < w) PANT_PW <<= 1;
    PANT_PH = 1;
    while (PANT_PH < h) PANT_PH <<= 1;

    /* OJO CON ESTAS DOS LINEAS: aca estaba `vid_set_mode(DM_640x480, PM_RGB565)` y NO VA.
     *
     * El PVR se encarga de la pantalla el solo: pvr_init_defaults() ya elige el modo de video y lo deja
     * listo. Cambiar el modo de video DESPUES deja al PVR dibujando sobre un framebuffer y esperando el
     * vsync de otro, y pvr_wait_ready() NO VUELVE NUNCA MAS. Medido en la consola serie: el juego imprimia
     * sus cinco lineas de arranque, dibujaba UN cuadro (el marco amarillo se veia en la captura) y de ahi
     * no salia mas -- ni un cuadro mas, ni un mensaje de error. Un cuelgue silencioso de los peores: la
     * pantalla muestra algo, asi que parece que anda. */
    pvr_init_defaults();

    pant_tex = pvr_mem_malloc((size_t)PANT_PW * (size_t)PANT_PH * 2);
    if (pant_tex == NULL) {
        return -2;
    }

    /* el buffer donde se arma el cuadro con el pixel ya en el formato del hardware */
    pant_conv = (uint16_t *)calloc((size_t)PANT_PW * (size_t)PANT_PH, sizeof(uint16_t));
    if (pant_conv == NULL) {
        return -3;
    }

    /* contexto: lista OPACA (la pantalla tapa todo, no hay nada atras) y sin filtro (pixel exacto: con
     * filtro se ve borroso y en una pantalla de 640x480 se nota) */
    pvr_poly_cxt_t cxt;
    pvr_poly_cxt_txr(&cxt, PVR_LIST_OP_POLY,
                     PVR_TXRFMT_RGB565 | PVR_TXRFMT_TWIDDLED,
                     PANT_PW, PANT_PH, pant_tex, PVR_FILTER_NONE);
    pvr_poly_compile(&pant_hdr, &cxt);

    return 0;
}

/* Sube el cuadro del motor (RGBA de 8 bits por canal) a la VRAM: convertir y despues subir.
 *
 * Lo que sobra a la derecha de la textura (1024 - 640) queda en negro: el quad no lo pinta. */
int d2c_pvr_pantalla_subir(unsigned int rgba, int w, int h) {
    int r = d2c_pvr_pantalla_convertir(rgba, w, h);
    if (r != 0) {
        return r;
    }
    return d2c_pvr_pantalla_subir_vram();
}

/* La conversion (RGBA -> RGB565 en memoria) y la subida a la VRAM, SEPARADAS Y MEDIBLES.
 *
 * Por que separadas: el cuadro entero costaba 590.087 microsegundos y la mitad se iba en "subir". Eso son
 * dos cosas MUY distintas -- recorrer 307.200 pixeles en la CPU, y mandar 1 MB por DMA a la VRAM -- y
 * "subir" no dice cual de las dos es. Con estas dos funciones se mide cada una y se deja de adivinar. */
/* ------------------------------------------------------------------ el twiddle del PVR
 *
 * EL TWIDDLE es el orden en el que el chip de video guarda los pixeles de una textura: no van uno al lado
 * del otro, van MEZCLADOS, intercalando los bits de la coordenada x con los de la y (asi el hardware
 * encuentra juntos los pixeles de un cuadradito y el dibujado aprovecha mejor la memoria). La formula es la
 * del propio KOS (pvr_texture.c:32) y esta aca porque EL TWIDDLE SE HACE UNA VEZ, EN LA CONVERSION, y no se
 * deja para despues:
 *
 *   pvr_txr_load_ex (el que twiddlea solo) tarda 302.549 microsegundos por cuadro -- el 77% de los 391.434
 *   del cuadro entero -- para una conversion que tarda 29.303. La razon esta en su bucle: hace una DIVISION
 *   y un MODULO POR PIXEL para calcular el bloque, y el SH-4 no tiene instruccion de division (es una
 *   subrutina). Aca la textura es de 1024x512, o sea que la mitad del trabajo de esos 302 ms es dividir.
 *
 * Con la tabla de abajo (512 valores, calculada una sola vez) y la potencia de dos (que el minimo lado es
 * 512, asi que en vez de dividir se CORRE A LA DERECHA), el twiddle queda en dos lecturas de tabla y un OR
 * por pixel, y la conversion entera sigue costando lo que costaba. Y la subida a la VRAM se hace con
 * pvr_txr_load, que es una copia por las Store Queues con el bus de 64 bits (el camino rapido del hardware).
 */
#define TWIDTAB(x) ( (x&1)|((x&2)<<1)|((x&4)<<2)|((x&8)<<3)|((x&16)<<4)| \
                     ((x&32)<<5)|((x&64)<<6)|((x&128)<<7)|((x&256)<<8)|((x&512)<<9) )

/* LA TABLA ES DE 32 BITS, Y ESTO NO ES UN DETALLE: los bits de la coordenada se ESPARCEN (el bit i va al
 * bit 2i), asi que para 9 bits de coordenada (el lado menor de 512) el resultado ocupa 17 bits: TWIDTAB(511)
 * vale 87.381, que NO ENTRA en 16 bits. Con la tabla de 16 bits los valores de la mitad de arriba se
 * truncaban, y el resultado medido fue una pantalla con ruido en bloques y una franja de colores: los
 * pixeles escritos en el lugar equivocado. Las cuentas del indice final tampoco entran en 16 bits (la
 * textura tiene 524.288 pixeles). */
static uint32_t twid_tabla[512];
static int twid_lista = 0;   /* si la tabla ya se armo */
static int twid_bits = 0;    /* log2 del lado menor (cuantos bits hay que correr para dividir por el) */
static int twid_mask = 0;    /* lado menor menos 1 (el modulo, que es un AND) */
static int twid_salto = 0;   /* lado menor al cuadrado: cuanto salta el indice de un bloque al siguiente */

static void armar_twiddle(void) {
    int i, min;

    if (twid_lista) {
        return;
    }

    for (i = 0; i < 512; i++) {
        twid_tabla[i] = (uint32_t)TWIDTAB(i);
    }

    min = (PANT_PW < PANT_PH) ? PANT_PW : PANT_PH;
    twid_bits = 0;
    while ((1 << twid_bits) < min) {
        twid_bits++;
    }
    twid_mask = min - 1;
    twid_salto = min * min;

    twid_lista = 1;
}

int d2c_pvr_pantalla_convertir(unsigned int rgba, int w, int h) {
    armar_twiddle();

    const unsigned char *src = (const unsigned char *)rgba;

    if (src == NULL || pant_conv == NULL) {
        return -1;
    }
    if (w != PANT_W || h != PANT_H) {
        return -2;
    }

    for (int y = 0; y < h; y++) {
        const unsigned char *p = src + (size_t)y * (size_t)w * 4;
        /* lo que solo depende de la fila se calcula UNA vez por fila y no por pixel */
        int ytw = twid_tabla[y & twid_mask];
        int ybloque = (y >> twid_bits) * twid_salto;

        for (int x = 0; x < w; x++) {
            unsigned int r = p[0];
            unsigned int g = p[1];
            unsigned int b = p[2];

            /* el color, en el formato del hardware: 5 bits de rojo, 6 de verde y 5 de azul */
            int indice = (ytw | (twid_tabla[x & twid_mask] << 1))
                       + (x >> twid_bits) * twid_salto + ybloque;
            pant_conv[indice] = (uint16_t)(((r >> 3) << 11) | ((g >> 2) << 5) | (b >> 3));

            p += 4;
        }
    }
    return 0;
}

int d2c_pvr_pantalla_subir_vram(void) {
    if (pant_tex == NULL) {
        return -1;
    }
    /* ESTA ES AHORA UNA COPIA PLANA, sin twiddlear: el twiddle ya se hizo en la conversion. Se usa
     * pvr_txr_load, que copia con las Store Queues y el bus de 64 bits (es lo que el propio KOS recomienda:
     * "unless you need to twiddle your texture, just use that instead").
     *
     * Queda anotado porque costo medirlo: los flags PVR_TXRLOAD_SQ y PVR_TXRLOAD_DMA de pvr_txr_load_ex NO
     * HACEN NADA -- con los dos el tiempo fue exactamente el mismo (302.549 microsegundos), y el header del
     * propio KOS lo avisa ("many of the parameters are just plain not supported at all"). Probar un flag
     * que existe en el header y creer que mejoro, sin medir, es la forma de pasar una tarde. */
    pvr_txr_load(pant_conv, pant_tex, (uint32_t)(PANT_PW * PANT_PH * 2));
    return 0;
}

/* Pinta el cuadro en toda la pantalla. El llamador tiene que tener abierta PVR_LIST_OP_POLY. */
void d2c_pvr_pantalla_pintar(void) {
    float u1 = (float)PANT_W / (float)PANT_PW;
    float v1 = (float)PANT_H / (float)PANT_PH;

    pvr_vertex_t v;
    pvr_prim(&pant_hdr, sizeof(pant_hdr));

    v.flags = PVR_CMD_VERTEX;
    v.argb = 0xffffffff;
    v.oargb = 0;
    v.z = 1.0f;

    v.u = 0.0f; v.v = 0.0f; v.x = 0.0f;        v.y = 0.0f;         pvr_prim(&v, sizeof(v));
    v.u = u1;   v.v = 0.0f; v.x = (float)PANT_W; v.y = 0.0f;         pvr_prim(&v, sizeof(v));
    v.u = 0.0f; v.v = v1;   v.x = 0.0f;        v.y = (float)PANT_H; pvr_prim(&v, sizeof(v));

    v.flags = PVR_CMD_VERTEX_EOL;
    v.u = u1;   v.v = v1;   v.x = (float)PANT_W; v.y = (float)PANT_H; pvr_prim(&v, sizeof(v));
}

/* Cuantos bytes de VRAM se reservaron (para poder decir un numero en la evidencia). */
int d2c_pvr_pantalla_vram(void) { return PANT_PW * PANT_PH * 2; }

/* El reloj, en microsegundos. No es parte del dibujado: es para poder DECIR cuanto tarda cada etapa en vez
 * de suponerlo. Un cuadro de 640x480 en un SH-4 tiene tres partes que pueden ser la lenta (dibujar en
 * memoria, convertir y subir a la VRAM, y el PVR pintando) y adivinar cual de las tres es la que no da los
 * cuadros por segundo es justo lo que no se puede hacer. */
unsigned int d2c_pvr_pantalla_us(void) { return (unsigned int)timer_us_gettime64(); }
