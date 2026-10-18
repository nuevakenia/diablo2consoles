#!/usr/bin/env python3
"""DC-002.35 (criterio 1) -- SONDA DEL HOST: el registro del BLOQUE de un archivo adentro de un MPQ.

Por que existe: la consola lee el mismo DC6 del mismo MPQ y le sale OTRO contenido con el MISMO tamano
(42502455 vs 820453653). Para saber de que lado esta el defecto hay que mirar lo que dice el MPQ del
bloque: si el archivo esta COMPRIMIDO (implode de PKWARE 0x100 o zlib/combinado 0x200), entonces lo que
se lee NO son los bytes guardados sino el resultado de descomprimir, y el sospechoso pasa a ser ESE
camino (que es justo lo que mide DC-002.40: `zlib: invalid header` en el `(listfile)` en la consola, y
que en el host anda).

Esta sonda NO usa el codigo del motor: descifra las tablas del MPQ a mano (la tabla de criptografia de
0x500 palabras y el descifrado estandar), busca la entrada por nombre y reporta el registro del bloque.
O sea: una medida INDEPENDIENTE de la del motor.

Uso:  sonda-dc6-mpq-235.py <mpq> <ruta\interna\del\archivo> [bytes_crudos]
"""
import struct
import sys
import zlib

_CRYPT = None


def tabla_cript():
    """La tabla de criptografia del MPQ (0x500 palabras, el generador estandar de StormLib)."""
    global _CRYPT
    if _CRYPT is not None:
        return _CRYPT

    semilla = 0x00100001
    tabla = [0] * 0x500
    for indice1 in range(0x100):
        for indice2 in range(indice1, 0x500, 0x100):
            semilla = (semilla * 125 + 3) % 0x2AAAAB
            temporal1 = (semilla & 0xFFFF) << 16
            semilla = (semilla * 125 + 3) % 0x2AAAAB
            temporal2 = semilla & 0xFFFF
            tabla[indice2] = temporal1 | temporal2
    _CRYPT = tabla
    return tabla


def hash_nombre(nombre, tipo):
    """HashString COMO LO HACE EL MOTOR (crypto.go:hashString).

    OJO, medido: la variante del motor NO es la de StormLib -- indexa la tabla con
    `cryptoLookup(tipo*0x100 + char)` (el caracter pelado) y no con `tipo<<8 | (seed2 & 0xFF)`.
    La sonda espeja el codigo del motor a proposito: lo que se mide es el camino de la consola, y ese
    camino es este. (Con la variante de StormLib esta sonda NO encuentra ninguna entrada: medido.)
    """
    tabla = tabla_cript()
    semilla1 = 0x7FED7FED
    semilla2 = 0xEEEEEEEE
    for c in nombre.upper():
        val = ord(c)
        semilla1 = (tabla[(tipo * 0x100) + val] ^ ((semilla1 + semilla2) & 0xFFFFFFFF)) & 0xFFFFFFFF
        semilla2 = (val + semilla1 + semilla2 + (semilla2 << 5) + 3) & 0xFFFFFFFF
    return semilla1


def descifrar_tabla(datos, clave):
    """El descifrado de bloques del MPQ, palabra por palabra (asi vienen las dos tablas)."""
    tabla = tabla_cript()
    salida = bytearray()
    semilla2 = 0xEEEEEEEE
    for i in range(0, len(datos) - 3, 4):
        semilla2 = (semilla2 + tabla[0x400 + (clave & 0xFF)]) & 0xFFFFFFFF
        valor = struct.unpack_from("<I", datos, i)[0]
        valor = (valor ^ ((clave + semilla2) & 0xFFFFFFFF)) & 0xFFFFFFFF
        # la clave nueva usa el corrimiento sobre la clave VIEJA (asi esta en StormLib)
        nueva = ((((~clave & 0xFFFFFFFF) << 21) & 0xFFFFFFFF) + 0x11111111) & 0xFFFFFFFF
        clave = (nueva | (clave >> 11)) & 0xFFFFFFFF
        salida += struct.pack("<I", valor)
        semilla2 = (valor + semilla2 + (semilla2 << 5) + 3) & 0xFFFFFFFF
    return bytes(salida)


FLAGS = [
    (0x00000100, "IMPLODE (PKWARE Data Compression)"),
    (0x00000200, "COMPRESS (multi-metodo: zlib/PKWARE/bzip2 por sector)"),
    (0x00010000, "ENCRYPTED"),
    (0x00020000, "FIX_KEY"),
    (0x00040000, "PATCH_FILE"),
    (0x00100000, "SINGLE_UNIT (una sola unidad: sin tabla de offsets por sector)"),
    (0x01000000, "DELETE_MARKER"),
    (0x02000000, "SECTOR_CRC"),
    (0x04000000, "EXISTS"),
]


def main():
    mpq_path = sys.argv[1]
    interno = sys.argv[2]
    n_crudos = int(sys.argv[3]) if len(sys.argv) > 3 else 16

    with open(mpq_path, "rb") as f:
        datos = f.read()

    hdr = datos[:32]
    magic = hdr[:4]
    ver = struct.unpack_from("<H", hdr, 12)[0]
    ht_off, bt_off, ht_cnt, bt_cnt = struct.unpack_from("<IIII", hdr, 16)

    print("MPQ %s" % mpq_path)
    print("  magic=%r ver=%d hashEnt=%d blockEnt=%d hashOff=%d blockOff=%d" %
          (magic, ver, ht_cnt, bt_cnt, ht_off, bt_off))

    ht = descifrar_tabla(datos[ht_off:ht_off + ht_cnt * 16], hash_nombre("(hash table)", 3))
    bt = descifrar_tabla(datos[bt_off:bt_off + bt_cnt * 16], hash_nombre("(block table)", 3))

    ha = hash_nombre(interno, 0)
    a = hash_nombre(interno, 1)
    b = hash_nombre(interno, 2)
    mascara = ht_cnt - 1
    indice = ha & mascara
    entrada = None
    for paso in range(ht_cnt):
        idx = (indice + paso) & mascara
        ea, eb, loc, plat, bi = struct.unpack_from("<IIHHI", ht, idx * 16)
        if bi == 0xFFFFFFFF:
            break
        if ea == a and eb == b:
            entrada = (idx, ea, eb, loc, plat, bi)
            break

    print("  archivo: %s" % interno)
    print("  hash0=0x%08x hash1=0x%08x hash2=0x%08x -> arranca en la ranura %d" % (ha, a, b, indice))
    if entrada is None:
        print("  NO ESTA en la tabla de hash de este MPQ")
        return 0

    idx, ea, eb, loc, plat, bi = entrada
    print("  entrada hash %d: bloque=%d locale=0x%04x plataforma=0x%04x" % (idx, bi, loc, plat))
    pos, comp, desc, flags = struct.unpack_from("<IIII", bt, bi * 16)
    print("  REGISTRO DEL BLOQUE %d: filePosition=%d compressedSize=%d uncompressedSize=%d flags=0x%08x" %
          (bi, pos, comp, desc, flags))
    for bit, nombre in FLAGS:
        if flags & bit:
            print("     flag 0x%08x: %s" % (bit, nombre))
    if not (flags & 0xFFFF0000):
        print("     sin flags de cifrado")

    crudos = datos[pos:pos + max(n_crudos, 0)]
    print("  primeros %d bytes GUARDADOS: %s" % (n_crudos, " ".join("%02x" % b for b in crudos)))

    if flags & 0x00100000 and flags & 0x00000200:
        print("  (single unit + compress: el primer byte crudo dice el metodo: 0x%02x)" % crudos[0])

    if not (flags & (0x00000100 | 0x00000200)):
        print("  CRC-32 de los bytes guardados: %d" % (zlib.crc32(datos[pos:pos + desc]) & 0xFFFFFFFF))
    else:
        print("  COMPRIMIDO: el contenido que se lee es el resultado de DESCOMPRIMIR estos bytes")
    return 0


if __name__ == "__main__":
    sys.exit(main())
