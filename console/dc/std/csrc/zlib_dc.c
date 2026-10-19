/* Puente a la zlib de KOS para d2std/compress/zlib.
 *
 * Por que en C: la zlib que trae KOS (kos-ports) ya esta compilada y probada para SH-4. Aca se la
 * envuelve detras de dos funciones, y el paquete de Go las llama con //extern. El mismo patron que el
 * PVR: lo que la consola ya sabe hacer, se usa; no se reescribe.
 *
 * d2c_zlib_inflate_bound: cuanto puede llegar a ocupar el resultado. zlib trae inflateBound() para
 * esto, pero pide el tamano comprimido, y para datos de tamano desconocido se usa la regla de siempre:
 * 4x el comprimido + 64 KB de margen.
 *
 * d2c_zlib_inflate: descomprime src en dst. Devuelve la cantidad de bytes escritos, o -1 si la zlib
 * dice que los datos no son validos o que no entran en dst.
 */
#include <zlib.h>
#include <stdint.h>
#include <string.h>

int d2c_zlib_inflate_bound(int src_len) {
    if (src_len <= 0) return 1024;
    return src_len * 4 + 65536;
}

/* DC-002.40: el ULTIMO codigo de retorno de la zlib (`inflate`/`inflateInit2`), para poder decir POR QUE
 * fallo en vez de suponerlo. Los codigos no son lo mismo: Z_DATA_ERROR (-3) = los datos no son un flujo
 * zlib valido; Z_MEM_ERROR (-4) = no hubo memoria para la ventana de 32 KB; Z_STREAM_ERROR (-2) = el
 * estado se corrompio. Una medicion que solo dice "devolvio -1" no distingue un defecto de datos de uno
 * de memoria, y la diferencia decide si el arreglo va en el puente o en el monton. */
static int d2c_ultimo_rc = 0;

int d2c_zlib_ultimo_rc(void) { return d2c_ultimo_rc; }
const char *d2c_zlib_version(void) { return zlibVersion(); }

int d2c_zlib_inflate(const unsigned char *src, int src_len,
                     unsigned char *dst, int dst_cap) {
    z_stream zs;
    memset(&zs, 0, sizeof(zs));

    /* 15 + 32: ventana de 32 KB y que la zlib decida entre cabecera zlib o gzip. Es lo que hace el
     * zlib.NewReader de Go, y al MPQ le llegan las dos formas segun el bloque. */
    if (inflateInit2(&zs, 15 + 32) != Z_OK) {
        d2c_ultimo_rc = -100;   /* DC-002.40: no fue ni un dato ni un paso: fallo el init (memoria/version) */
        return -1;
    }

    zs.next_in = (Bytef *)src;
    zs.avail_in = (uInt)src_len;
    zs.next_out = dst;
    zs.avail_out = (uInt)dst_cap;

    int rc = inflate(&zs, Z_FINISH);
    d2c_ultimo_rc = rc;

    if (rc != Z_STREAM_END) {
        inflateEnd(&zs);
        return -1;
    }

    int escrito = (int)zs.total_out;
    inflateEnd(&zs);
    return escrito;
}

/* ================================================================================================
 * DC-002.20: INFLATE EN STREAMING (por pedazos), para no pedirle al monton el resultado entero.
 *
 * POR QUE: `d2c_zlib_inflate` descomprime TODO en un destino y el destino se calcula con
 * `d2c_zlib_inflate_bound` = 4 x comprimido + 64 KB, que es una REGLA, no una cota. Medido en DC-002.20
 * con el asset EMBEBIDO del motor (`assets.CompressedDebugText`): comprimido 2.209 B, la regla da 74.372 B
 * y el asset descomprimido son 98.358 B, asi que la zlib devolvia error y el `gzip` de la consola lo
 * traducia a `gzip: invalid header`. El mensaje mentia sobre la causa: era TAMANO, no cabecera.
 *
 * QUE HACE ESTO: el mismo `inflateInit2(15 + 32)` (que decide entre zlib y gzip), pero el estado de la zlib
 * queda VIVO entre llamadas y el que llama pide los bytes de a poco, en el buffer que ya tiene. Asi el
 * destino es el buffer del que lee (512 B en el corpus, lo que pida el motor) y no el resultado entero.
 *
 * LOS SLOTS: el estado son 4 z_stream en .bss, prestados por indice (init devuelve el indice, end lo
 * libera). Es a proposito: en la consola no se guardan punteros a estado de C en el monton de Go, se
 * guardan INDICES (los portadores uintptr del proyecto). 4 alcanza para el motor, que lee un asset a la vez.
 *
 * EL ABI (el mismo cuidado que costo caro en DC-002.19): los escalares primero y las rutas/buffers despues,
 * y el resultado del paso se lee con funciones APARTE (consumido/fin) en vez de punteros de salida: un
 * puntero de salida mal pasado no se nota, un contador que no avanza si.
 * ================================================================================================ */
#define D2C_ZLIB_SLOTS 4

static z_stream d2c_zs[D2C_ZLIB_SLOTS];
static int d2c_zs_libre[D2C_ZLIB_SLOTS] = {1, 1, 1, 1};
static int d2c_zs_fin[D2C_ZLIB_SLOTS];
static int d2c_zs_consumido[D2C_ZLIB_SLOTS];

int d2c_zlib_stream_init(void) {
    int i;
    for (i = 0; i < D2C_ZLIB_SLOTS; i++) {
        if (!d2c_zs_libre[i]) continue;
        memset(&d2c_zs[i], 0, sizeof(z_stream));
        if (inflateInit2(&d2c_zs[i], 15 + 32) != Z_OK) {
            d2c_ultimo_rc = -100;   /* el init: memoria o version, no los datos */
            return -1;
        }
        d2c_zs_libre[i] = 0;
        d2c_zs_fin[i] = 0;
        d2c_zs_consumido[i] = 0;
        return i;
    }
    return -1;   /* los 4 slots estan tomados: es un defecto de quien llama, no de la zlib */
}

/* Devuelve los bytes ESCRITOS en dst (0 si no hubo salida todavia), o -1 si la zlib dice que no son datos
 * validos. El final del flujo se pregunta con d2c_zlib_stream_fin, no se adivina con el 0. */
int d2c_zlib_stream_step(int slot, const unsigned char *src, int src_len,
                         unsigned char *dst, int dst_cap) {
    int rc, escrito;

    if (slot < 0 || slot >= D2C_ZLIB_SLOTS || d2c_zs_libre[slot]) return -1;
    d2c_zs_consumido[slot] = 0;
    d2c_zs[slot].next_in = (Bytef *)src;
    d2c_zs[slot].avail_in = (uInt)src_len;
    d2c_zs[slot].next_out = dst;
    d2c_zs[slot].avail_out = (uInt)dst_cap;

    rc = inflate(&d2c_zs[slot], Z_NO_FLUSH);
    d2c_ultimo_rc = rc;

    escrito = dst_cap - (int)d2c_zs[slot].avail_out;
    d2c_zs_consumido[slot] = src_len - (int)d2c_zs[slot].avail_in;

    if (rc == Z_STREAM_END) {
        d2c_zs_fin[slot] = 1;
        return escrito;
    }
    if (rc != Z_OK && rc != Z_BUF_ERROR) {
        return -1;   /* datos invalidos: esto SI es un error de cabecera o de flujo */
    }
    return escrito;
}

int d2c_zlib_stream_consumido(int slot) {
    if (slot < 0 || slot >= D2C_ZLIB_SLOTS) return 0;
    return d2c_zs_consumido[slot];
}

int d2c_zlib_stream_fin(int slot) {
    if (slot < 0 || slot >= D2C_ZLIB_SLOTS) return 1;
    return d2c_zs_fin[slot];
}

int d2c_zlib_stream_end(int slot) {
    if (slot < 0 || slot >= D2C_ZLIB_SLOTS || d2c_zs_libre[slot]) return -1;
    inflateEnd(&d2c_zs[slot]);
    d2c_zs_libre[slot] = 1;
    d2c_zs_fin[slot] = 0;
    d2c_zs_consumido[slot] = 0;
    return 0;
}

