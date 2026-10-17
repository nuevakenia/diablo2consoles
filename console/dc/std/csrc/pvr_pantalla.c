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
static int PANT_W = 0, PANT_H = 0, PANT_PW = 0, PANT_PH = 0;
/* el PVR se inicializa UNA sola vez (DC-002.22): `pvr_init_defaults()` avisa "pvr_init called twice!" y
 * volver a inicializar el chip con el video ya andando es la clase de cosa que rompe el estado del PVR
 * sin decir nada. Medido: con el video re-dimensionable (el POC arranca 640x480 y el motor despues
 * 800x600) se llamaba dos veces. */
static int pvr_listo = 0;

/* DC-002.17: el buffer del cuadro convertido. Antes era `pant_conv`, 1.048.576 bytes del MONTON (el
 * `calloc` de abajo); ahora es un bloque de 32x32 en .bss. Alineado a 32 bytes porque `pvr_txr_load` va
 * por Store Queues (sq_cpy) y eso exige origen y destino alineados a 32 y un largo multiplo de 32. */
#define LADO_TILE 32
static uint16_t conv_tile[LADO_TILE * LADO_TILE] __attribute__((aligned(32)));

/* La tabla del twiddle y sus constantes: ver el comentario largo mas abajo (seccion "el twiddle del
 * PVR"). Estan declaradas ACA arriba, y no mas abajo con el resto, porque el relleno del arranque
 * (`indice_tile`) las usa antes.
 *
 * LA TABLA ES DE 32 BITS, Y ESTO NO ES UN DETALLE: los bits de la coordenada se ESPARCEN (el bit i va al
 * bit 2i), asi que para 9 bits de coordenada (el lado menor de 512) el resultado ocupa 17 bits: TWIDTAB(511)
 * vale 87.381, que NO ENTRA en 16 bits. Con la tabla de 16 bits los valores de la mitad de arriba se
 * truncaban, y el resultado medido fue una pantalla con ruido en bloques y una franja de colores: los
 * pixeles escritos en el lugar equivocado. Las cuentas del indice final tampoco entran en 16 bits (la
 * textura tiene 524.288 pixeles). */
#define TWIDTAB(x) ( (x&1)|((x&2)<<1)|((x&4)<<2)|((x&8)<<3)|((x&16)<<4)| \
                     ((x&32)<<5)|((x&64)<<6)|((x&128)<<7)|((x&256)<<8)|((x&512)<<9) )
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

/* El indice, DENTRO DE LA TEXTURA, del primer pixel del bloque que empieza en (x0, y0).
 *
 * La formula es la de la conversion de siempre (ver abajo) aplicada a la esquina: los bits de x van a las
 * posiciones PARES del indice y los de y a las impares, y el lado mayor parte la textura en bloques de
 * 512x512 (twid_salto). Con x0 e y0 multiplos de 32 los 5 bits bajos son cero, asi que los 1.024 pixeles
 * del bloque caen SEGUIDOS en la textura, desde este indice. Eso es lo que hace posible subir de a un
 * bloque sin tener el cuadro entero en memoria. */
static uint32_t indice_tile(int x0, int y0) {
    armar_twiddle();
    return (uint32_t)(twid_tabla[y0 & twid_mask] << 1)
         + (uint32_t)twid_tabla[x0 & twid_mask]
         + (uint32_t)(x0 >> twid_bits) * (uint32_t)twid_salto
         + (uint32_t)(y0 >> twid_bits) * (uint32_t)twid_salto;
}

/* Arranca el video y deja la textura de la pantalla lista. Se llama antes del loop.
 * Devuelve 0 si anduvo; negativo si no (sin reserva de VRAM, sin memoria, tamanos invalidos).
 *
 * DC-002.22: ES RE-DIMENSIONABLE, y no era. Antes el `pantallaIniciada` del binding de Go la volvia
 * idempotente para siempre, asi que el arranque del motor (que pide 640x480 al inicio y despues el
 * tamano de la pantalla DEL MOTOR, 800x600) se quedaba con la textura de 640x480 y la conversion del
 * cuadro devolvia -2 ("no se pudo convertir el cuadro") sin decir por que. Medido con el numero: con
 * el framebuffer de 800x600 y la textura de 640x480, `d2c_pvr_pantalla_convertir` corta en
 * `w != PANT_W` y devuelve -2 (el error es de TAMANO, no de VRAM).
 *
 * Si el tamano cambia se libera la textura vieja y se rehace todo (los tamanos potencia de dos y el
 * contexto del quad). El twiddle se rearma porque sus constantes salen de PANT_PW/PANT_PH. */
int d2c_pvr_pantalla_init(int w, int h) {
    if (w <= 0 || h <= 0) {
        return -1;
    }

    /* el mismo tamano que ya esta: no hay nada que hacer */
    if (pant_tex != NULL && w == PANT_W && h == PANT_H) {
        return 0;
    }

    /* otro tamano: la textura vieja se devuelve a la VRAM antes de pedir la nueva */
    if (pant_tex != NULL) {
        pvr_mem_free(pant_tex);
        pant_tex = NULL;
        twid_lista = 0;
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
     * pantalla muestra algo, asi que parece que anda.
     *
     * DC-002.22: se llama UNA sola vez (ver `pvr_listo`). */
    if (!pvr_listo) {
        pvr_init_defaults();
        pvr_listo = 1;
    }

    pant_tex = pvr_mem_malloc((size_t)PANT_PW * (size_t)PANT_PH * 2);
    if (pant_tex == NULL) {
        return -2;
    }

    /* DC-002.17: ACA HABIA UN `calloc(PANT_PW * PANT_PH, 2)` = 1.048.576 bytes (1 MiB exactos) del
     * monton, y era el `codigo -3` que dejaba al motor sin video. Medido en el arranque del motor:
     *
     *   Out of memory. Requested sbrk_base 0x8d0a9000, was 0x8cfa9000, diff 1048576
     *   MOTOR-ARRANQUE el video no arranco: no se pudo arrancar la pantalla del PVR (codigo -3)
     *
     * 1048576 es EXACTAMENTE PANT_PW * PANT_PH * 2 (1024x512x2): el pedido que no entraba. En el motor
     * entero, el monton ya tiene 2.430.080 bytes usados de los 2.731.136 que hay entre el fin del .bss y
     * el tope de sbrk (_arch_mem_top - THD_KERNEL_STACK_SIZE), o sea quedaban 301.056 libres y se pedian
     * 1.048.576. Los POC del PVR SI pintaban porque pesan una fraccion de eso y ahi el 1 MiB entraba.
     *
     * El reemplazo es SUBIR POR BLOQUES: la textura se arma en un buffer estatico de 32x32 pixeles
     * (2.048 bytes en .bss, no en el monton) y cada bloque se manda a la VRAM con pvr_txr_load. En el
     * PVR un bloque de 32x32 SI es contiguo en la textura twiddleada (el twiddle intercala los bits de x
     * con los de y: los 5 bits bajos de cada coordenada dan 10 bits de indice, o sea 1.024 pixeles
     * seguidos), asi que 1.048.576 bytes del monton pasan a 2.048 bytes de .bss.
     *
     * El relleno (de 640 a 1023 y de 480 a 511) se deja en CERO una sola vez, aca: antes lo ponia el
     * calloc, y la comparacion pixel a pixel del POC (DC-002.10) cuenta con eso. Cuesta 512 bloques de
     * 2.048 bytes una vez en el arranque (~1 MiB de DMA, medido abajo en la evidencia). */
    for (int y0 = 0; y0 < PANT_PH; y0 += LADO_TILE) {
        for (int x0 = 0; x0 < PANT_PW; x0 += LADO_TILE) {
            memset(conv_tile, 0, sizeof(conv_tile));
            pvr_txr_load(conv_tile, (pvr_ptr_t)((uint16_t *)pant_tex + indice_tile(x0, y0)),
                         LADO_TILE * LADO_TILE * 2);
        }
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
 * Con la tabla de arriba (512 valores, calculada una sola vez) y la potencia de dos (que el minimo lado es
 * 512, asi que en vez de dividir se CORRE A LA DERECHA), el twiddle queda en dos lecturas de tabla y un OR
 * por pixel, y la conversion entera sigue costando lo que costaba. Y la subida a la VRAM se hace con
 * pvr_txr_load, que es una copia por las Store Queues con el bus de 64 bits (el camino rapido del hardware).
 *
 * DC-002.17: el twiddle NO cambio, lo que cambio es DONDE se arma. Antes se armaba el cuadro entero en un
 * buffer de 1 MiB del monton; ahora se arma de a un bloque de 32x32 (conv_tile, 2 KiB en .bss) y cada
 * bloque se sube apenas se termina. La cuenta del pixel es la misma: dentro del bloque, los 5 bits bajos de
 * (x, y) dan el indice (twid_tabla[x] << 1 | twid_tabla[y]), y el bloque arranca en indice_tile(x0, y0).
 */

int d2c_pvr_pantalla_convertir(unsigned int rgba, int w, int h) {
    const unsigned char *src = (const unsigned char *)rgba;

    armar_twiddle();

    if (src == NULL || pant_tex == NULL) {
        return -1;
    }
    if (w != PANT_W || h != PANT_H) {
        return -2;
    }

    for (int y0 = 0; y0 < h; y0 += LADO_TILE) {
        int alto_tile = (h - y0 < LADO_TILE) ? (h - y0) : LADO_TILE;

        for (int x0 = 0; x0 < w; x0 += LADO_TILE) {
            int ancho_tile = (w - x0 < LADO_TILE) ? (w - x0) : LADO_TILE;

            /* un bloque que NO esta completo (solo pasa si la pantalla no es multiplo de 32: 640x480 SI
             * lo es, asi que hoy nunca entra) se rellena con cero para no dejar pixeles de la vuelta
             * anterior en la parte que no se escribe */
            if (ancho_tile != LADO_TILE || alto_tile != LADO_TILE) {
                memset(conv_tile, 0, sizeof(conv_tile));
            }

            for (int yy = 0; yy < alto_tile; yy++) {
                const unsigned char *p = src + (size_t)(y0 + yy) * (size_t)w * 4 + (size_t)x0 * 4;
                uint32_t fila = (uint32_t)twid_tabla[yy] << 1;   /* solo depende de la fila del bloque */

                for (int xx = 0; xx < ancho_tile; xx++) {
                    unsigned int r = p[0];
                    unsigned int g = p[1];
                    unsigned int b = p[2];

                    /* el color, en el formato del hardware: 5 bits de rojo, 6 de verde y 5 de azul */
                    conv_tile[fila | (uint32_t)twid_tabla[xx]] =
                        (uint16_t)(((r >> 3) << 11) | ((g >> 2) << 5) | (b >> 3));

                    p += 4;
                }
            }

            /* y el bloque a la VRAM. 1.024 pixeles = 2.048 bytes, de una sola copia por Store Queues. */
            pvr_txr_load(conv_tile, (pvr_ptr_t)((uint16_t *)pant_tex + indice_tile(x0, y0)),
                         LADO_TILE * LADO_TILE * 2);
        }
    }
    return 0;
}

int d2c_pvr_pantalla_subir_vram(void) {
    if (pant_tex == NULL) {
        return -1;
    }
    /* DC-002.17: ESTA FUNCION YA NO TIENE QUE HACER NADA, y no es un olvido. La conversion ahora sube cada
     * bloque a la VRAM apenas lo termina (no hay cuadro entero convertido que subir despues: justamente
     * ese buffer de 1 MiB era el que no entraba en los 16 MB). Queda devolviendo 0 para no romper a los
     * llamadores ni a las mediciones: `SubirPantalla` sigue llamando convertir + subir, y el que mide por
     * separado sigue midiendo convertir (que ahora incluye la subida) y subir (que cuesta cero).
     *
     * ESTA ES AHORA UNA COPIA PLANA, sin twiddlear: el twiddle ya se hizo en la conversion. Se usa
     * pvr_txr_load, que copia con las Store Queues y el bus de 64 bits (es lo que el propio KOS recomienda:
     * "unless you need to twiddle your texture, just use that instead").
     *
     * Queda anotado porque costo medirlo: los flags PVR_TXRLOAD_SQ y PVR_TXRLOAD_DMA de pvr_txr_load_ex NO
     * HACEN NADA -- con los dos el tiempo fue exactamente el mismo (302.549 microsegundos), y el header del
     * propio KOS lo avisa ("many of the parameters are just plain not supported at all"). Probar un flag
     * que existe en el header y creer que mejoro, sin medir, es la forma de pasar una tarde. */
    return 0;
}

/* Cuanto buffer propio usa el backend, en bytes (para la evidencia: el pedido al monton era 1.048.576 y
 * ahora es este numero, en .bss). */
int d2c_pvr_pantalla_ram(void) { return (int)sizeof(conv_tile); }

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

/* ------------------------------------------------------------------ DC-002.22: el framebuffer en la VRAM
 *
 * QUE ES: el bloque de memoria donde el MOTOR dibuja su cuadro en RGBA (8 bits por canal), reservado
 * en la VRAM en vez del monton de C. Es memoria de trabajo, no una textura: el PVR nunca la lee como
 * textura (el cuadro se convierte a RGB565 en `pant_tex`, que es la que se pinta).
 *
 * POR QUE (medido en DC-002.22, no supuesto). El motor pide su pantalla logica -- 800x600x4 =
 * 1.920.000 bytes -- y esa peticion NO ENTRA en la RAM principal:
 *
 *   - la RAM son 16.777.216 bytes y el binario que Flycast carga ya ocupa 13.919.684;
 *   - el monton, cuando llega el pedido, tiene arena=2415484 used=2372348 free=43136;
 *   - o sea que faltan 1.876.864 bytes, y no hay recorte del compilador que los pague: DC-002.14 ya
 *     midio que -ffunction-sections/-fdata-sections/--gc-sections NO bajan el binario (sale 2.316
 *     bytes MAS grande) y que -fno-asynchronous-unwind-tables no cambia el .eh_frame de gccgo.
 *   - la VRAM, en cambio, son 8 MB y el PVR usa ~1 MB para la textura del cuadro: sobran ~5,7 MB.
 *
 * El que decide no es la intuicion sino la cuenta: 1.920.000 bytes que NO pueden vivir en los 16 MB
 * de RAM principal y sí viven en los 8 MB de VRAM.
 *
 * COMO LO USA EL SHIM: `NuevaPantallaVram(w, h)` (pvr.go) toma el puntero y lo convierte en el slice
 * de pixeles de la imagen. El resto del shim no cambia una linea: escribir un pixel sigue siendo
 * `pixeles[i] = v`.
 *
 * LO QUE CUESTA, DICHO DE FRENTE: la VRAM se escribe por el bus de 64 bits del PVR y sin cache (el
 * area P2 es cache-through), asi que el compositor de software del shim (pixel a pixel) es mas lento
 * que contra la RAM. Es un intercambio declarado: RAM que no habia, a cambio de velocidad de
 * dibujado. El costo se mide en la tarjeta de rendimiento, no aca.
 *
 * Si no hay VRAM, `fb_init` devuelve negativo y el shim se queda con la imagen del monton (como
 * estaba antes): el POC chico sigue andando y el que mide dice cual de los dos caminos tomo. */
static void *pant_fb = NULL;
static int pant_fb_bytes = 0;

int d2c_pvr_pantalla_fb_init(int w, int h) {
    size_t bytes;

    if (w <= 0 || h <= 0) {
        return -1;
    }
    if (pant_tex == NULL) {
        /* el PVR todavia no arranco: sin pvr_init_defaults no hay VRAM que repartir */
        return -3;
    }

    bytes = (size_t)w * (size_t)h * 4;

    if (pant_fb != NULL && (size_t)pant_fb_bytes == bytes) {
        return 0;
    }
    if (pant_fb != NULL) {
        pvr_mem_free(pant_fb);
        pant_fb = NULL;
        pant_fb_bytes = 0;
    }

    pant_fb = pvr_mem_malloc(bytes);
    if (pant_fb == NULL) {
        return -2;
    }

    /* el cuadro arranca en negro: la pantalla se pinta entera una vez para no leer basura de la VRAM */
    memset(pant_fb, 0, bytes);
    pant_fb_bytes = (int)bytes;

    return 0;
}

/* El puntero al framebuffer de la pantalla (0 si no hay). Cruza a Go como uintptr. */
unsigned int d2c_pvr_pantalla_fb(void) { return (unsigned int)pant_fb; }

/* Cuantos bytes de VRAM ocupa el framebuffer (0 si vive en el monton: el numero que lo dice). */
int d2c_pvr_pantalla_fb_bytes(void) { return pant_fb_bytes; }

/* Cuantos bytes LIBRES tiene el MONTON DE C en este momento.
 *
 * Es mallinfo(), la MISMA fuente que el "Memory: arena=%d used=%d free=%d" con el que el runtime de la
 * consola cuenta el OOM (runtime/defer_dreamcast.c:138). O sea: el numero que se lee aca es comparable
 * con el que dio el defecto. Se expone a Go porque el hito de esta tarjeta ES ese numero: cuanto queda
 * en el monton cuando el motor pide su pantalla. */
int d2c_pvr_pantalla_mem_libre(void) {
    struct mallinfo mi = mallinfo();

    return mi.fordblks;
}

/* El reloj, en microsegundos. No es parte del dibujado: es para poder DECIR cuanto tarda cada etapa en vez
 * de suponerlo. Un cuadro de 640x480 en un SH-4 tiene tres partes que pueden ser la lenta (dibujar en
 * memoria, convertir y subir a la VRAM, y el PVR pintando) y adivinar cual de las tres es la que no da los
 * cuadros por segundo es justo lo que no se puede hacer. */
unsigned int d2c_pvr_pantalla_us(void) { return (unsigned int)timer_us_gettime64(); }
