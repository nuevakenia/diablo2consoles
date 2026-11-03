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
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* Los prototipos de las dos partes (el C las necesita ANTES: `subir` -- la funcion de siempre -- usa a las
 * dos, y estan definidas mas abajo). */
int d2c_pvr_pantalla_convertir(unsigned int rgba, int w, int h);
int d2c_pvr_pantalla_subir_vram(void);
/* DC-002.135: el reloj del backend, que ahora lo usan tambien las funciones de arriba (los cronometros de la
 * presentacion). Declarado aca porque su definicion esta al final del archivo. */
unsigned int d2c_pvr_pantalla_us(void);

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

/* ====================================================================================================
 * DC-002.135 -- LOS TRES PEDAZOS DE LA PRESENTACION, MEDIDOS POR DENTRO (y el camino rapido)
 * ====================================================================================================
 *
 * EL NUMERO QUE MANDA (DC-002.132): la presentacion cuesta 36.759 / 36.758 / 37.000 / 36.840 us en
 * cuatro tramos de dos corridas distintas, con el blit de tiles viejo y con el nuevo. Es CONSTANTE, o
 * sea que el techo del POC son 27 cuadros por segundo aunque el dibujado sea gratis. Y la linea
 * `PVR132: CUADRO` decia el total de la presentacion, pero NO de que estaba hecho: convertir 307.200
 * pixeles de RGBA a RGB565 (CPU), subirlos a la VRAM (Store Queues) y pintar el quad (PVR) son tres
 * cosas MUY distintas y sin partirlas no se sabe cual atacar. Estos tres cronometros las parten.
 *
 * COMO SE MIDE, Y POR QUE ASI. Cada bloque de 32x32 se cronometra con dos marcas: la del bucle de
 * pixeles (conversion) y la del `pvr_txr_load` (subida a la VRAM). Cronometrar LOS 300 BLOQUES de un
 * cuadro son 600 llamadas al reloj por cuadro, y el reloj de KOS no es gratis (se mide y se informa en
 * `d2c_pvr_pantalla_reloj_costo`): por eso se cronometra UNO DE CADA `PRES_MUESTREO` bloques y se
 * escala por la cuenta real de bloques. El trabajo por bloque es uniforme (misma cuenta de pixeles,
 * misma copia de 2048 B), asi que la muestra dice la proporcion de los tres pedazos sin ensuciar la
 * medida con el instrumento.
 *
 * EL CAMINO RAPIDO. Los dos cuellos que se atacan aca, y por que:
 *
 *   1. LA CONVERSION, pixel a pixel. El bucle de siempre lee el lienzo con TRES cargas de 8 bits
 *      (`p[0]`, `p[1]`, `p[2]`) contra la VRAM (area P2: sin cache, cada acceso es un viaje al chip),
 *      calcula el indice del twiddle con DOS lecturas de tabla y escribe de a 16 bits. El camino rapido
 *      hace UNA carga de 32 bits por pixel, arma el RGB565 con mascaras y corrimientos (7 cuentas, sin
 *      tablas de color) y escribe de a 32 bits (dos pixeles por deposito), recorriendo la textura en
 *      ORDEN para que los depositos sean seguidos: para eso hay una tabla de desplazamientos
 *      (`ofs_tile`, se arma una vez por ancho de lienzo) que dice, para cada posicion de la textura
 *      twiddleada, que pixel del bloque le toca. El resultado es BYTE A BYTE el mismo de siempre.
 *   2. LA SUBIDA. `pvr_txr_load` sube 2048 B por bloque (300 copias por cuadro). En la textura
 *      twiddleada un bloque de 32x32 SI es contiguo, pero los bloques vecinos NO: el twiddle los
 *      reparte en orden de Morton. Lo que si es contiguo es el GRUPO DE 2x2 BLOQUES (64x64 pixeles =
 *      8.192 B): verificado contra la formula, los cuatro bloques de un grupo caen en cuatro bloques
 *      SEGUIDOS de la textura. Asi que el camino rapido convierte el grupo entero en un buffer de 8 KiB
 *      y lo sube con UNA copia: 90 subidas por cuadro en vez de 300 (70 grupos de 2x2 = 280 bloques +
 *      los 20 bloques sueltos de la franja de abajo, 480 = 7x64 + 32).
 *
 * LAS DOS PALANCAS SE PUEDEN PROBAR POR SEPARADO (`presentarConvRapida` y `presentarVramAgrupada` del
 * POC, que llegan aca por `d2c_pvr_pantalla_pres_conv_rapida` y ..._vram_agrupada): con las dos en 0 el
 * camino es EXACTAMENTE el de DC-002.132, en el MISMO binario. Eso es el A/B.
 *
 * LO QUE NO SE TOCA: el formato de la textura (RGB565 twiddleada), la formula del twiddle, el quad de
 * pantalla y los bytes que llegan a la VRAM. Si los bytes cambian, la captura cambia: la prueba es que
 * la captura del pueblo salga identica pixel a pixel a la de DC-002.132 y el crc32 del mundo siga en
 * 1437022778.
 */
static int pres_conv_rapida = 0;
static int pres_vram_agrupada = 0;

/* Lo que costo la ULTIMA presentacion, ya escalado a los bloques reales del cuadro.
 *
 * SE ACUMULA POR LLAMADAS, NO SE PISA. Un cuadro puede presentarse en MAS DE UNA llamada: el split de
 * DC-002.116/118 llama a `d2c_pvr_pantalla_convertir_en` una vez por lienzo (dos en un cuadro de 2
 * jugadores, cuatro en el de 4). Si estas cuatro variables se reiniciaran en cada llamada, la linea
 * `PVR135: PRESENTAR` informaria SOLO LA ULTIMA MITAD del cuadro (conv de 150 bloques en vez de 300) y el
 * resto se veria como un `pintar` inflado. Por eso SUMAN, y el que reinicia es el ciclo del cuadro, con
 * `d2c_pvr_pantalla_pres_reiniciar()` (Go lo llama al empezar la presentacion del cuadro). */
static unsigned int pres_conv_us = 0;
static unsigned int pres_vram_us = 0;
static unsigned int pres_bloques = 0;  /* bloques de 32x32 convertidos */
static unsigned int pres_subidas = 0;  /* llamadas a pvr_txr_load (con la agrupada bajan de 300 a 90) */

/* Uno de cada PRES_MUESTREO bloques se cronometra: ver la cabecera (el reloj de KOS no es gratis). */
#define PRES_MUESTREO 8

/* LADO_GRUPO: el lado del grupo de bloques que se sube de una sola vez (2x2 bloques = 64x64 px).
 * El grupo es lo mas grande que la textura twiddleada guarda SEGUIDO: 4 bloques de 1.024 pixeles. */
#define LADO_GRUPO 64
static uint16_t conv_grupo[LADO_GRUPO * LADO_GRUPO] __attribute__((aligned(32)));

/* ofs_tile[i]: el byte del bloque (32x32) que va a la posicion i de la textura twiddleada.
 *
 * NO DEPENDE DEL BLOQUE, solo del ancho del lienzo (el paso de fila), asi que se arma una vez. La
 * cuenta sale de la MISMA formula del twiddle de `indice_tile`, despejada: en el indice, los bits de x
 * van a las posiciones IMPARES y los de y a las PARES, asi que para la posicion `i` de la textura,
 * x es `i` con los bits pares sacados e y es `i` con los impares sacados. */
static uint32_t ofs_tile[LADO_TILE * LADO_TILE];
static int ofs_tile_w = -1;

static void armar_ofs_tile(int w) {
    int i;

    if (ofs_tile_w == w) {
        return;
    }

    for (i = 0; i < LADO_TILE * LADO_TILE; i++) {
        int x = ((i & 0x002) >> 1) | ((i & 0x008) >> 2) | ((i & 0x020) >> 3) |
                ((i & 0x080) >> 4) | ((i & 0x200) >> 5);
        int y = (i & 0x001) | ((i & 0x004) >> 1) | ((i & 0x010) >> 2) |
                ((i & 0x040) >> 3) | ((i & 0x100) >> 4);

        ofs_tile[i] = (uint32_t)(y * w * 4 + x * 4);
    }

    ofs_tile_w = w;
}

/* El RGB565 de un pixel RGBA leido como una palabra de 32 bits (R en el byte 0, A en el 3): 5 bits de
 * rojo, 6 de verde y 5 de azul, igual que el bucle de siempre (no se redondea: se trunca). */
#define COLOR565(p) (uint32_t)((((p) & 0x000000F8u) << 8) | (((p) & 0x0000FC00u) >> 5) | \
                               (((p) & 0x00F80000u) >> 19))

/* Un bloque de 32x32 COMPLETO, del lienzo a `conv` en orden de textura, de a dos pixeles por deposito. */
static void bloque_rapido(const unsigned char *base, uint16_t *conv) {
    uint32_t *dst = (uint32_t *)conv;
    const uint32_t *ofs = ofs_tile;
    int i;

    for (i = 0; i < LADO_TILE * LADO_TILE; i += 4) {
        uint32_t a = *(const uint32_t *)(base + ofs[i]);
        uint32_t b = *(const uint32_t *)(base + ofs[i + 1]);
        uint32_t c = *(const uint32_t *)(base + ofs[i + 2]);
        uint32_t d = *(const uint32_t *)(base + ofs[i + 3]);

        dst[i >> 1] = COLOR565(a) | (COLOR565(b) << 16);
        dst[(i >> 1) + 1] = COLOR565(c) | (COLOR565(d) << 16);
    }
}

/* Las palancas y los numeros de la presentacion, para Go (cruzan como enteros: el comentario largo de
 * `pantalla.go` explica por que nada de punteros ni slices en una funcion //extern). */
void d2c_pvr_pantalla_pres_conv_rapida(int v) { pres_conv_rapida = (v != 0); }
void d2c_pvr_pantalla_pres_vram_agrupada(int v) { pres_vram_agrupada = (v != 0); }
int d2c_pvr_pantalla_pres_conv_rapida_on(void) { return pres_conv_rapida; }
int d2c_pvr_pantalla_pres_vram_agrupada_on(void) { return pres_vram_agrupada; }
unsigned int d2c_pvr_pantalla_pres_conv_us(void) { return pres_conv_us; }
unsigned int d2c_pvr_pantalla_pres_vram_us(void) { return pres_vram_us; }
unsigned int d2c_pvr_pantalla_pres_bloques(void) { return pres_bloques; }
unsigned int d2c_pvr_pantalla_pres_subidas(void) { return pres_subidas; }

/* Cuanto cuesta UNA llamada al reloj, medido aca mismo: 1.000 llamadas y el total en microsegundos.
 *
 * VA EN LA EVIDENCIA Y NO ES UN ADORNO: la presentacion se parte en tres con dos marcas por bloque
 * muestreado, asi que el que lee el desglose tiene que poder descontar el instrumento (si 1.000
 * llamadas cuestan, por ejemplo, 2.000 us, cada marca cuesta 2 us y el desglose trae ese ruido). */
unsigned int d2c_pvr_pantalla_reloj_costo(void) {
    unsigned int t0, t1;
    volatile unsigned int i, sumidero = 0;

    t0 = (unsigned int)timer_us_gettime64();
    for (i = 0; i < 1000; i++) {
        sumidero += (unsigned int)timer_us_gettime64();
    }
    t1 = (unsigned int)timer_us_gettime64();

    (void)sumidero;

    return t1 - t0;
}

/* La tabla del twiddle y sus constantes: ver el comentario largo mas abajo (seccion "el twiddle del
 * PVR"). Estan declaradas ACA arriba, y no mas abajo con el resto, porque el relleno del arranque
 * (`indice_tile`) las usa antes.
 *
 * LA TABLA ES DE 32 BITS, Y ESTO NO ES UN DETALLE: los bits de la coordenada se ESPARCEN (el bit i va al
 * bit 2i), asi que para 9 bits de coordenada (el lado menor de 512) el resultado ocupa 17 bits: TWIDTAB(511)
 * vale 87.381, que NO ENTRA en 16 bits. Con la tabla de 16 bits los valores de la mitad de arriba se
 * truncaban, y el resultado medido fue una pantalla con ruido en bloques y una franja de colores: los
 * pixeles escritos en el lugar equivocado. Las cuentas del indice final tampoco entran en 16 bits (la
 * textura tiene 524.288 pixeles).
 *
 * DC-002.26: LA TABLA TIENE QUE CUBRIR EL LADO MENOR, Y ERA DE 512. El indice del bloque sale de
 * `y0 & (lado_menor - 1)` y `x0 & (lado_menor - 1)`, o sea que la tabla necesita `lado_menor` entradas. Con
 * el POC de 640x480 la textura es 1024x512, el lado menor es 512 y la tabla de 512 alcanzaba justo. Con el
 * MOTOR (800x600) la textura es 1024x1024, el lado menor es 1024 y `twid_mask` vale 1023: las lecturas
 * `twid_tabla[x0 & 1023]` / `twid_tabla[y0 & 1023]` con x0 o y0 >= 512 SALIAN DE LA TABLA (leian lo que hay
 * detras en .bss) y el bloque se escribia en cualquier parte de la textura. Medido: de los 475 bloques de
 * 32x32 que tiene el cuadro del motor, 219 caian en esa zona y el cuadro salia con ARTEFACTOS (bloques
 * repetidos donde no van: la tinta de la captura sube de 3.196 a 5.831 pixeles). La tabla pasa a 1024
 * entradas (TWIDTAB ya cubre los 10 bits: su ultimo termino es (x&512)<<9). La ROTACION de 90 grados era
 * OTRO defecto de esta misma funcion: el lugar de los bits en `indice_tile` (ver ahi abajo). */
#define TWID_TABLA_N 1024
#define TWIDTAB(x) ( (x&1)|((x&2)<<1)|((x&4)<<2)|((x&8)<<3)|((x&16)<<4)| \
                     ((x&32)<<5)|((x&64)<<6)|((x&128)<<7)|((x&256)<<8)|((x&512)<<9) )
static uint32_t twid_tabla[TWID_TABLA_N];
static int twid_lista = 0;   /* si la tabla ya se armo */
static int twid_bits = 0;    /* log2 del lado menor (cuantos bits hay que correr para dividir por el) */
static int twid_mask = 0;    /* lado menor menos 1 (el modulo, que es un AND) */
static int twid_salto = 0;   /* lado menor al cuadrado: cuanto salta el indice de un bloque al siguiente */

static void armar_twiddle(void) {
    int i, min;

    if (twid_lista) {
        return;
    }

    for (i = 0; i < TWID_TABLA_N; i++) {
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
 * posiciones IMPARES del indice y los de y a las PARES, y el lado mayor parte la textura en bloques de
 * 512x512 (twid_salto). Con x0 e y0 multiplos de 32 los 5 bits bajos son cero, asi que los 1.024 pixeles
 * del bloque caen SEGUIDOS en la textura, desde este indice. Eso es lo que hace posible subir de a un
 * bloque sin tener el cuadro entero en memoria.
 *
 * DC-002.26: LAS DOS POSICIONES ESTABAN AL REVES, Y ESO ES UNA ROTACION DE 90 GRADOS. La convencion del
 * PVR es la del propio KOS (`pvr_texture.c:34`, la que usan todos los homebrew):
 *
 *     TWIDOUT(x, y) = TWIDTAB(y) | (TWIDTAB(x) << 1)      <- los bits de X van a los IMPARES
 *
 * y aca estaban al reves (los de y a los impares). Cambiar el lugar de los bits del indice es cambiar la
 * textura por su TRASPUESTA: el hardware lee el pixel que espera en (x, y) y se encuentra el que se escribio
 * en (y, x). Medido en la captura del motor: el texto del cuadro de error salia ROTADO 90 grados (las
 * lineas corriendo de arriba abajo) -- el mismo sintoma que se ve en la captura de Papu del 2026-10-17. */
static uint32_t indice_tile(int x0, int y0) {
    armar_twiddle();
    return (uint32_t)(twid_tabla[x0 & twid_mask] << 1)
         + (uint32_t)twid_tabla[y0 & twid_mask]
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
        /* DC-002.50 (instrumento de 2026-10-24): el POC se reinicia DENTRO de pvr_init_defaults y el ultimo
         * mensaje que deja KOS es "pvr: enabling vertical scaling for non-VGA". Estas dos marcas dicen si el
         * que se cae es el arranque del PVR o lo que viene despues (vblank_handler_add + las IRQ del ASIC). */
        printf("HITO250: PVR antes-de-pvr_init_defaults\n");
        pvr_init_defaults();
        printf("HITO250: PVR despues-de-pvr_init_defaults\n");
        pvr_listo = 1;
    }

    pant_tex = pvr_mem_malloc((size_t)PANT_PW * (size_t)PANT_PH * 2);
    if (pant_tex == NULL) {
        printf("HITO250: PVR SIN-VRAM\n");

        return -2;
    }

    printf("HITO250: PVR textura-de-pantalla OK\n");

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

    printf("HITO250: PVR relleno OK\n");

    /* contexto: lista OPACA (la pantalla tapa todo, no hay nada atras) y sin filtro (pixel exacto: con
     * filtro se ve borroso y en una pantalla de 640x480 se nota) */
    pvr_poly_cxt_t cxt;
    pvr_poly_cxt_txr(&cxt, PVR_LIST_OP_POLY,
                     PVR_TXRFMT_RGB565 | PVR_TXRFMT_TWIDDLED,
                     PANT_PW, PANT_PH, pant_tex, PVR_FILTER_NONE);
    pvr_poly_compile(&pant_hdr, &cxt);

    printf("HITO250: PVR contexto OK\n");

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

/* ============================== DC-002.116: la COLUMNA DE DESTINO ==============================
 *
 * Las tres funciones de conversion de abajo comparten la misma forma: `src` es un lienzo de w x h y el
 * resultado va a la textura de la pantalla a partir de la COLUMNA `x_dest` (y, en la de DC-002.118, de la
 * FILA `y_dest`). Hasta DC-002.115 el motor tenia UN lienzo de 640x480 y se subia entero a la textura, asi
 * que el destino siempre empezaba en la columna 0. El split vertical de DC-002.116 tiene DOS lienzos de
 * 320x480 (uno por jugador) y cada uno va a SU mitad: el de la izquierda a la columna 0 y el de la derecha
 * a la 320. La cuenta del pixel es la misma de siempre; lo unico que cambia es de donde se lee (`src` tiene
 * ancho propio, w) y donde se escribe (`x_dest + x0` en la textura).
 *
 * OJO CON EL STRIDE: `src` se recorre con SU ancho (w), no con PANT_W. Leer un lienzo de 320 con el paso de
 * 640 da una imagen cortada en diagonal.
 *
 * `x_dest` TIENE QUE SER MULTIPLO DE 32: cada bloque de 32x32 de la textura es contiguo (ver `indice_tile`),
 * y 320 lo es, asi que las dos mitades caen en bloques enteros. El que llama lo comprueba y devuelve -4. */
/* La conversion de siempre (el bucle de DC-002.132, copiado tal cual), con los cronometros de DC-002.135.
 *
 * ES EL CONTROL DEL A/B: con `pres_conv_rapida = 0` y `pres_vram_agrupada = 0` este es el camino que
 * midio 36,8 ms en DC-002.132, en el MISMO binario que el camino nuevo. Si se toca este bucle, el A/B
 * deja de decir la verdad. */
static int convertir_clasico(const unsigned char *src, int w, int h, int x_dest) {
    unsigned int reloj_conv = 0, reloj_vram = 0;
    int bloque = 0, muestras = 0;

    for (int y0 = 0; y0 < h; y0 += LADO_TILE) {
        int alto_tile = (h - y0 < LADO_TILE) ? (h - y0) : LADO_TILE;

        for (int x0 = 0; x0 < w; x0 += LADO_TILE) {
            int ancho_tile = (w - x0 < LADO_TILE) ? (w - x0) : LADO_TILE;
            int medir = ((bloque++ % PRES_MUESTREO) == 0);
            unsigned int t0 = 0, t1 = 0, t2 = 0;

            if (medir) {
                t0 = d2c_pvr_pantalla_us();
            }

            /* un bloque que NO esta completo (solo pasa si la pantalla no es multiplo de 32: 640x480 SI lo
             * es, asi que hoy nunca entra) se rellena con cero para no dejar pixeles de la vuelta
             * anterior en la parte que no se escribe */
            if (ancho_tile != LADO_TILE || alto_tile != LADO_TILE) {
                memset(conv_tile, 0, sizeof(conv_tile));
            }

            for (int yy = 0; yy < alto_tile; yy++) {
                const unsigned char *p = src + (size_t)(y0 + yy) * (size_t)w * 4 + (size_t)x0 * 4;
                uint32_t fila = (uint32_t)twid_tabla[yy];        /* solo depende de la fila del bloque */

                for (int xx = 0; xx < ancho_tile; xx++) {
                    unsigned int r = p[0];
                    unsigned int g = p[1];
                    unsigned int b = p[2];

                    /* el color, en el formato del hardware: 5 bits de rojo, 6 de verde y 5 de azul.
                     * DC-002.26: la x va a los bits IMPARES (<< 1) y la y a los pares, como `TWIDOUT`. */
                    conv_tile[(uint32_t)twid_tabla[xx] << 1 | fila] =
                        (uint16_t)(((r >> 3) << 11) | ((g >> 2) << 5) | (b >> 3));

                    p += 4;
                }
            }

            if (medir) {
                t1 = d2c_pvr_pantalla_us();
            }

            /* y el bloque a la VRAM. 1.024 pixeles = 2.048 bytes, de una sola copia por Store Queues. */
            pvr_txr_load(conv_tile, (pvr_ptr_t)((uint16_t *)pant_tex + indice_tile(x_dest + x0, y0)),
                         LADO_TILE * LADO_TILE * 2);
            pres_subidas++;

            if (medir) {
                t2 = d2c_pvr_pantalla_us();
                reloj_conv += t1 - t0;
                reloj_vram += t2 - t1;
                muestras++;
            }
        }
    }

    /* Aca la unidad es la misma que la del camino rapido: cada iteracion convierte UN bloque y sube 2.048 B,
     * asi que escalar por bloque (esto) o por iteracion da lo mismo. Se deja escrito como bloque para que
     * los dos caminos se lean con la misma vara (ver el escalado de `convertir_rapido`).
     * Y SUMAN, no pisan: un cuadro puede venir en varias llamadas (el split); el reinicio lo hace el ciclo del
     * cuadro con `d2c_pvr_pantalla_pres_reiniciar()`. */
    pres_bloques += (unsigned int)bloque;
    pres_conv_us += (muestras > 0) ? (unsigned int)((unsigned long long)reloj_conv * (unsigned int)bloque / (unsigned int)muestras) : 0;
    pres_vram_us += (muestras > 0) ? (unsigned int)((unsigned long long)reloj_vram * (unsigned int)bloque / (unsigned int)muestras) : 0;

    return 0;
}

/* El camino RAPIDO de DC-002.135: una carga de 32 bits por pixel, dos pixeles por deposito, y la subida
 * en grupos de 2x2 bloques (8.192 B de una sola copia). La cabecera del bloque de DC-002.135, arriba,
 * dice por que cada cosa.
 *
 * LOS BYTES QUE LLEGAN A LA VRAM SON LOS MISMOS: la cuenta del color es la de siempre (truncar a
 * 5-6-5) y el orden del twiddle tambien. Lo unico que cambia es cuantas instrucciones y cuantos viajes
 * cuesta. La prueba no es este comentario: es que la captura del pueblo salga identica pixel a pixel a
 * la de DC-002.132 y el crc32 del lienzo siga en 1437022778. */
static int convertir_rapido(const unsigned char *src, int w, int h, int x_dest) {
    unsigned int reloj_conv = 0, reloj_vram = 0;
    unsigned int bloques_inicio = pres_bloques;  /* para saber cuantos bloques convirtio ESTA llamada */
    unsigned int bloques_llamada = 0;
    unsigned int bloques_muestra = 0;  /* bloques convertidos en las iteraciones MUESTREADAS */
    unsigned int bytes_muestra = 0;    /* bytes subidos a la VRAM en las iteraciones MUESTREADAS */
    int iteraciones = 0, muestras = 0;

    /* el camino rapido solo sabe de bloques COMPLETOS (el bucle nuevo no tiene la cuenta del borde): un
     * lienzo que no sea multiplo de 32 -- las mitades de 240 de DC-002.118, por ejemplo -- se convierte
     * con el de siempre, que rellena el borde con cero. */
    if (((w % LADO_TILE) != 0) || ((h % LADO_TILE) != 0)) {
        return convertir_clasico(src, w, h, x_dest);
    }

    armar_ofs_tile(w);

    for (int y0 = 0; y0 < h; y0 += LADO_GRUPO) {
        int alto_grupo = (h - y0 < LADO_GRUPO) ? (h - y0) : LADO_GRUPO;

        for (int x0 = 0; x0 < w; x0 += LADO_GRUPO) {
            int ancho_grupo = (w - x0 < LADO_GRUPO) ? (w - x0) : LADO_GRUPO;
            int medir = ((iteraciones++ % PRES_MUESTREO) == 0);
            int cuantos = 1;   /* bloques que van en UNA copia: 4 (2x2), 2 (2x1) o 1 */
            unsigned int t0 = 0, t1 = 0, t2 = 0;
            unsigned int bloques_antes = pres_bloques;  /* cuantos bloques lleva esta iteracion */
            uint32_t i0;

            /* CUANTOS BLOQUES VAN EN UNA COPIA. La textura twiddleada guarda seguidos los cuatro bloques de
             * un grupo de 2x2 (64x64 px) y tambien un par VERTICAL (32x64), pero NO un par horizontal: entre
             * (x0,y0) y (x0+32,y0) esta el bloque (x0,y0+32), que es de otro grupo (el orden es de Morton).
             * Medido en esta tarjeta, subir un par horizontal como si fuera contiguo pisaba el bloque de al
             * lado y dejaba 1.024 palabras de la franja de abajo mal. Y un grupo que cruza la columna 512 cae
             * en dos superbloques y ahi los indices saltan: por eso se comprueba con `indice_tile` en vez de
             * suponerlo. Con la palanca apagada `cuantos` queda en 1 y cada bloque se sube a SU indice: la
             * conversion es la misma en los dos casos (los cuatro bloques del grupo se convierten SIEMPRE),
             * lo unico que cambia es cuantas copias se piden. */
            i0 = indice_tile(x_dest + x0, y0);
            cuantos = 1;

            if (pres_vram_agrupada && ancho_grupo == LADO_GRUPO && alto_grupo == LADO_GRUPO &&
                indice_tile(x_dest + x0 + LADO_TILE, y0) == i0 + 2 * 1024 &&
                indice_tile(x_dest + x0, y0 + LADO_TILE) == i0 + 1024 &&
                indice_tile(x_dest + x0 + LADO_TILE, y0 + LADO_TILE) == i0 + 3 * 1024) {
                cuantos = 4;
            } else if (pres_vram_agrupada && ancho_grupo == LADO_TILE && alto_grupo == LADO_GRUPO &&
                       indice_tile(x_dest + x0, y0 + LADO_TILE) == i0 + 1024) {
                cuantos = 2;
            }

            if (medir) {
                t0 = d2c_pvr_pantalla_us();
            }

            if (cuantos > 1) {
                /* el orden de los bloques dentro del grupo es el de la TEXTURA: primero la fila de abajo
                 * (k par: mismo y; k impar: y+32) y, en el grupo de 2x2, la columna de la derecha a partir
                 * de k=2. Con cuantos=2 (par vertical) solo se mueve la y. */
                for (int k = 0; k < cuantos; k++) {
                    int bx = ((cuantos == 4) && (k & 2)) ? LADO_TILE : 0;
                    int by = (k & 1) ? LADO_TILE : 0;

                    bloque_rapido(src + (size_t)(y0 + by) * (size_t)w * 4 + (size_t)(x0 + bx) * 4,
                                  conv_grupo + k * (LADO_TILE * LADO_TILE));
                }
            } else {
                /* los bloques del grupo, uno por uno, en orden de pantalla (fila de bloques) */
                int nbx = ancho_grupo / LADO_TILE;

                for (int by = 0; by < alto_grupo; by += LADO_TILE) {
                    for (int bx = 0; bx < ancho_grupo; bx += LADO_TILE) {
                        int k = (by / LADO_TILE) * nbx + (bx / LADO_TILE);

                        bloque_rapido(src + (size_t)(y0 + by) * (size_t)w * 4 + (size_t)(x0 + bx) * 4,
                                      conv_grupo + k * (LADO_TILE * LADO_TILE));
                    }
                }
            }

            if (medir) {
                t1 = d2c_pvr_pantalla_us();
            }

            if (cuantos > 1) {
                pvr_txr_load(conv_grupo, (pvr_ptr_t)((uint16_t *)pant_tex + i0),
                             cuantos * LADO_TILE * LADO_TILE * 2);
                pres_subidas++;
                pres_bloques += (unsigned int)cuantos;
            } else {
                int nbx = ancho_grupo / LADO_TILE;

                for (int by = 0; by < alto_grupo; by += LADO_TILE) {
                    for (int bx = 0; bx < ancho_grupo; bx += LADO_TILE) {
                        int k = (by / LADO_TILE) * nbx + (bx / LADO_TILE);

                        pvr_txr_load(conv_grupo + k * (LADO_TILE * LADO_TILE),
                                     (pvr_ptr_t)((uint16_t *)pant_tex +
                                                 indice_tile(x_dest + x0 + bx, y0 + by)),
                                     LADO_TILE * LADO_TILE * 2);
                        pres_subidas++;
                        pres_bloques++;
                    }
                }
            }

            if (medir) {
                t2 = d2c_pvr_pantalla_us();
                reloj_conv += t1 - t0;
                reloj_vram += t2 - t1;
                muestras++;

                /* LA UNIDAD DEL ESCALADO, que no es la iteracion (ver el comentario del final): */
                bloques_muestra += pres_bloques - bloques_antes;
                bytes_muestra += (pres_bloques - bloques_antes) * LADO_TILE * LADO_TILE * 2;
            }
        }
    }

    bloques_llamada = pres_bloques - bloques_inicio;

    /* EL ESCALADO, Y POR QUE NO ES POR ITERACION (ronda 2 de la revision de DC-002.135).
     *
     * Una iteracion NO siempre convierte lo mismo: convierte 4 bloques cuando el grupo de 2x2 se puede
     * agrupar, 2 cuando es un par vertical y 2 cuando va suelto en la franja de abajo (64 de ancho x 32 de
     * alto). Si se escala por ITERACION (lo que media la primera version) y el muestreo -- uno de cada 8 --
     * cae siempre en grupos completos, el numero sale ALTO: en 640x480, 70 iteraciones de 4 bloques + 10 de
     * 2 dan 300 bloques, y extrapolar 80 iteraciones del costo de un grupo completo da 320 (6,7 % de mas en
     * el peor caso; en estas corridas la muestra si toco la franja y el error fue 1,4 %). En el lienzo de
     * 320x480 del split el error se veia en el acto: la conversion se informaba 25.201 us contra un total de
     * 25.521 medido por el ciclo, o sea un `pintar` NEGATIVO.
     *
     * Asi que se escala por la unidad del trabajo: los BLOQUES para la conversion (cada bloque son los
     * mismos 1.024 pixeles) y los BYTES para la subida (cada byte cuesta lo mismo en la copia; el costo
     * fijo por llamada hace que esto subestime un pelo cuando hay copias chicas, ~3 % en el peor caso). El
     * resultado se SUMA a lo que ya costo el cuadro: un cuadro puede venir en varias llamadas (el split). */
    pres_conv_us += (muestras > 0 && bloques_muestra > 0) ? (unsigned int)((unsigned long long)reloj_conv * (unsigned int)bloques_llamada / (unsigned int)bloques_muestra) : 0;
    pres_vram_us += (muestras > 0 && bytes_muestra > 0) ? (unsigned int)((unsigned long long)reloj_vram * (unsigned int)(bloques_llamada * LADO_TILE * LADO_TILE * 2) / (unsigned int)bytes_muestra) : 0;

    return 0;
}

/* Los contadores de la presentacion a cero: lo llama GO al empezar la presentacion del cuadro.
 *
 * POR QUE LO LLAMA GO Y NO ESTA ACA. Un cuadro puede llegar en varias llamadas (dos lienzos de 320x480 en
 * el split de 2 jugadores, cuatro en el de 4): si la conversion reiniciara los contadores, el desglose
 * informaria solo el ULTIMO lienzo. Tampoco se pueden reiniciar en la primera conversion del cuadro sin
 * que el backend sepa cuando empieza un cuadro, y eso lo sabe el ciclo del Renderer: por eso es una funcion
 * aparte y explicita (ronda 2 de la revision de DC-002.135). */
void d2c_pvr_pantalla_pres_reiniciar(void) {
    pres_bloques = 0;
    pres_subidas = 0;
    pres_conv_us = 0;
    pres_vram_us = 0;
}

/* El que llaman las tres entradas de la conversion: elige camino y DEJA LOS NUMEROS DEL CUADRO EN LAS
 * VARIABLES QUE LEE GO, sumando lo que costo esta llamada a lo que ya costo el cuadro. */
static int convertir_a_columnas(const unsigned char *src, int w, int h, int x_dest) {
    if (pres_conv_rapida) {
        return convertir_rapido(src, w, h, x_dest);
    }

    return convertir_clasico(src, w, h, x_dest);
}

int d2c_pvr_pantalla_convertir(unsigned int rgba, int w, int h) {
    const unsigned char *src = (const unsigned char *)rgba;

    armar_twiddle();

    if (src == NULL || pant_tex == NULL) {
        return -1;
    }
    if (w != PANT_W || h != PANT_H) {
        return -2;
    }

    return convertir_a_columnas(src, w, h, 0);
}

/* DC-002.116: un lienzo de w x h a la COLUMNA x_dest de la textura de la pantalla (el split vertical).
 *
 * Es el MISMO camino que `d2c_pvr_pantalla_convertir` -- misma tabla del twiddle, mismo bloque de 32x32, mismas
 * Store Queues -- y por eso cuesta lo mismo POR PIXEL que la conversion de la pantalla entera: dos lienzos de
 * 320x480 son los mismos 307.200 pixeles que uno de 640x480. Lo que NO hace falta es el lienzo de 640x480:
 * los dos de 320x480 son, juntos, los mismos 1.228.800 B que antes vivian en la VRAM como un solo cuadro.
 *
 * Devuelve 0 si anduvo, -1 sin video, -2 tamano/columna invalidos, -4 columna que no es multiplo de 32. */
int d2c_pvr_pantalla_convertir_en(unsigned int rgba, int w, int h, int x_dest) {
    const unsigned char *src = (const unsigned char *)rgba;

    armar_twiddle();

    if (src == NULL || pant_tex == NULL) {
        return -1;
    }
    if (w <= 0 || h <= 0 || x_dest < 0) {
        return -2;
    }
    if (x_dest + w > PANT_W || h > PANT_H) {
        return -2;
    }
    if ((x_dest % LADO_TILE) != 0) {
        return -4;
    }

    return convertir_a_columnas(src, w, h, x_dest);
}

/* DC-002.118: un lienzo de w x h a la COLUMNA Y LA FILA (x_dest, y_dest) de la textura de la pantalla.
 *
 * POR QUE HACE FALTA UN y_dest. DC-002.116 partio la pantalla en DOS COLUMNAS de 320x480: alcanzaba con la
 * columna porque los dos lienzos empiezan en y=0. El split de TRES y CUATRO no: cuatro cuadrantes son dos
 * FILAS (0 y 240) y dos columnas, y sin la fila de destino los de abajo se escribian ENCIMA de los de arriba.
 *
 * ES EL MISMO CAMINO que `convertir_a_columnas` -- misma tabla del twiddle, mismo bloque de 32x32, mismas
 * Store Queues, mismo costo por pixel -- con la y del destino sumada al indice del bloque. Se deja como
 * funcion aparte (y no se le pone un parametro mas a la de DC-002.116) porque el POC de aquella tarjeta y su
 * evidencia tienen que seguir compilando y midiendo exactamente lo mismo.
 *
 * x_dest E y_dest TIENEN QUE SER MULTIPLOS DE 32: cada bloque de 32x32 de la textura es contiguo, y un lienzo
 * que empiece a mitad de bloque no se puede subir por bloques enteros sin leer y reescribir el bloque del
 * vecino. Los llamadores (el POC de DC-002.118) eligen las posiciones de la textura con esa regla: la
 * pantalla y la textura NO tienen por que coincidir pixel a pixel, y el que pinta sabe las dos.
 *
 * Devuelve 0 si anduvo, -1 sin video, -2 tamano/fila/columna invalidos, -4 destino que no es multiplo de 32. */
int d2c_pvr_pantalla_convertir_en_bloque(unsigned int rgba, int w, int h, int x_dest, int y_dest) {
    const unsigned char *src = (const unsigned char *)rgba;

    armar_twiddle();

    if (src == NULL || pant_tex == NULL) {
        return -1;
    }
    if (w <= 0 || h <= 0 || x_dest < 0 || y_dest < 0) {
        return -2;
    }
    /* contra la TEXTURA (potencia de dos), no contra la pantalla: un lienzo de 320x240 en la fila 256 de una
     * textura de 1024x512 entra, aunque 256+240 = 496 > 480 (la pantalla es mas chica que la textura). */
    if (x_dest + w > PANT_PW || y_dest + h > PANT_PH) {
        return -2;
    }
    if (((x_dest % LADO_TILE) != 0) || ((y_dest % LADO_TILE) != 0)) {
        return -4;
    }

    for (int y0 = 0; y0 < h; y0 += LADO_TILE) {
        int alto_tile = (h - y0 < LADO_TILE) ? (h - y0) : LADO_TILE;

        for (int x0 = 0; x0 < w; x0 += LADO_TILE) {
            int ancho_tile = (w - x0 < LADO_TILE) ? (w - x0) : LADO_TILE;

            /* un bloque que NO esta completo (el ultimo de una fila de tiles, que pasa con h=240: 7 bloques
             * enteros y uno de 16 filas) se rellena con cero para no dejar los pixeles de la vuelta anterior
             * en la parte que este lienzo no escribe */
            if (ancho_tile != LADO_TILE || alto_tile != LADO_TILE) {
                memset(conv_tile, 0, sizeof(conv_tile));
            }

            for (int yy = 0; yy < alto_tile; yy++) {
                const unsigned char *p = src + (size_t)(y0 + yy) * (size_t)w * 4 + (size_t)x0 * 4;
                uint32_t fila = (uint32_t)twid_tabla[yy];

                for (int xx = 0; xx < ancho_tile; xx++) {
                    unsigned int r = p[0];
                    unsigned int g = p[1];
                    unsigned int b = p[2];

                    conv_tile[(uint32_t)twid_tabla[xx] << 1 | fila] =
                        (uint16_t)(((r >> 3) << 11) | ((g >> 2) << 5) | (b >> 3));

                    p += 4;
                }
            }

            pvr_txr_load(conv_tile, (pvr_ptr_t)((uint16_t *)pant_tex + indice_tile(x_dest + x0, y_dest + y0)),
                         LADO_TILE * LADO_TILE * 2);
        }
    }

    return 0;
}

/* ============================== DC-002.139: LA PRESENTACION POR BLOQUES SUCIOS ==============================
 *
 * EL NUMERO QUE MANDA (DC-002.140, Flycast): la presentacion cuesta 25,5 ms por cuadro y 23,6 de esos son la
 * CONVERSION de los 307.200 pixeles (RGBA -> RGB565): el bucle lee el lienzo contra la VRAM (area P2, sin
 * cache) una palabra de 32 bits por pixel y esa lectura es la que manda. La subida a la VRAM son 1,8 ms.
 *
 * POR QUE SE PUEDE CONVERTIR MENOS. Con el mapa estatico (DC-002.139) el lienzo NO cambia entero entre dos
 * cuadros: cambian las cajas de los sprites y los paneles. Convertir un bloque de 32x32 cuesta lo mismo que
 * convertir cualquiera; lo que no cuesta nada es NO convertir el que quedo igual. El POC marca los bloques que
 * toco (jugador, bichos, NPCs, HUD, minimapa, panel) y aca se convierten solo esos, con el MISMO camino rapido
 * del bloque (`bloque_rapido` no se puede usar tal cual porque la fila del destino cambia: la tabla `ofs_tile`
 * esta pensada para un bloque entero desde su esquina).
 *
 * LOS BYTES QUE LLEGAN A LA VRAM SON LOS MISMOS que los del cuadro entero: misma cuenta del color (5-6-5
 * truncado), mismo twiddle, mismo `pvr_txr_load`. La prueba es el crc32 del mundo (1437022778) y que el cuadro
 * se vea igual.
 */
int d2c_pvr_pantalla_bloque(unsigned int rgba, int lienzo_w, int x0, int y0) {
    const unsigned char *src = (const unsigned char *)rgba;

    if (pant_tex == NULL || src == NULL) {
        return -1;
    }
    if (lienzo_w <= 0 || x0 < 0 || y0 < 0) {
        return -2;
    }

    armar_twiddle();
    /* el destino SIEMPRE es un bloque entero (el que llama pasa x0/y0 multiplos de 32), asi que el bloque se
     * arma con el mismo `bloque_rapido` del cuadro entero: cuatro pixeles por vuelta y dos por deposito. */
    armar_ofs_tile(lienzo_w);
    bloque_rapido(src + ((size_t)y0 * (size_t)lienzo_w + (size_t)x0) * 4, conv_tile);

    pvr_txr_load(conv_tile, (pvr_ptr_t)((uint16_t *)pant_tex + indice_tile(x0, y0)),
                 LADO_TILE * LADO_TILE * 2);
    pres_bloques++;

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

/* DC-002.118: pinta UN RECUADRO de la textura en UN RECUADRO de la pantalla.
 *
 * El cuadro partido de DOS (DC-002.116) se pintaba con un solo quad que tomaba la region (0,0)-(640,480) de la
 * textura y la ponia en toda la pantalla, porque los dos lienzos estaban donde mismo iban en la pantalla. Los
 * cuatro cuadrantes NO: van en dos filas, y la fila de abajo tiene que vivir en la textura en una fila que sea
 * MULTIPLO DE 32 (la 256, no la 240), asi que la textura ya no coincide con la pantalla y hay que decir,
 * recuadro por recuadro, de donde se lee y donde se pinta.
 *
 * Cada llamada emite el header del contexto y los cuatro vertices del quad (nearest, ver `pvr_init`: con
 * filtro los bordes del recuadro se mancharian con el vecino). El que llama abre la lista OPACA y cierra con
 * `pvr_wait_ready`, que ya lo hace el ciclo. */
void d2c_pvr_pantalla_pintar_bloque(int x, int y, int w, int h, int tu, int tv) {
    float u0 = (float)tu / (float)PANT_PW;
    float v0 = (float)tv / (float)PANT_PH;
    float u1 = (float)(tu + w) / (float)PANT_PW;
    float v1 = (float)(tv + h) / (float)PANT_PH;

    pvr_vertex_t v;
    pvr_prim(&pant_hdr, sizeof(pant_hdr));

    v.flags = PVR_CMD_VERTEX;
    v.argb = 0xffffffff;
    v.oargb = 0;
    v.z = 1.0f;

    v.u = u0; v.v = v0; v.x = (float)x;       v.y = (float)y;       pvr_prim(&v, sizeof(v));
    v.u = u1; v.v = v0; v.x = (float)(x + w); v.y = (float)y;       pvr_prim(&v, sizeof(v));
    v.u = u0; v.v = v1; v.x = (float)x;       v.y = (float)(y + h); pvr_prim(&v, sizeof(v));

    v.flags = PVR_CMD_VERTEX_EOL;
    v.u = u1; v.v = v1; v.x = (float)(x + w); v.y = (float)(y + h); pvr_prim(&v, sizeof(v));
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
