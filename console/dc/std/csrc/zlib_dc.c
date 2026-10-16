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

int d2c_zlib_inflate(const unsigned char *src, int src_len,
                     unsigned char *dst, int dst_cap) {
    z_stream zs;
    memset(&zs, 0, sizeof(zs));

    /* 15 + 32: ventana de 32 KB y que la zlib decida entre cabecera zlib o gzip. Es lo que hace el
     * zlib.NewReader de Go, y al MPQ le llegan las dos formas segun el bloque. */
    if (inflateInit2(&zs, 15 + 32) != Z_OK) {
        return -1;
    }

    zs.next_in = (Bytef *)src;
    zs.avail_in = (uInt)src_len;
    zs.next_out = dst;
    zs.avail_out = (uInt)dst_cap;

    int rc = inflate(&zs, Z_FINISH);

    if (rc != Z_STREAM_END) {
        inflateEnd(&zs);
        return -1;
    }

    int escrito = (int)zs.total_out;
    inflateEnd(&zs);
    return escrito;
}
