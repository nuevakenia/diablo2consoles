#!/usr/bin/env python3
"""DC-002.40: LA CABECERA CRUDA DE UN ARCHIVO COMPRIMIDO DE UN MPQ (el instrumento del HOST).

Para que: `ReadFile("(listfile)")` FALLA en la consola con `zlib: invalid header` y ANDA en el host con el
MISMO codigo del motor. Antes de tocar nada hay que saber, del propio archivo:

  1. los FLAGS del bloque (FileImplode 0x100 / FileCompress 0x200 / FileEncrypted 0x10000 /
     FileSingleUnit 0x10000 -- ver `mpq_block.go`), que son los que eligen el CAMINO en `mpq_stream.go`;
  2. el tamano guardado contra el tamano real, y la TABLA DE OFFSETS de sectores;
  3. el PRIMER BYTE DE CADA SECTOR, que es el tipo de compresion del bloque (`decompressMulti`:
     2 = zlib, 8 = PKWARE implode, ...). Ese byte es lo que decide si el `zlib: invalid header` es
     "el bloque no es zlib" o "el dato llego mal".

Y de paso descomprime el archivo ENTERO con la zlib de Python y cuenta las lineas: si los nombres salen
igual que en la consola/host, la lectura cruda del archivo esta verificada de punta a punta y el defecto
esta SOLO del lado de la consola.

Este script NO usa el codigo del motor: lee el MPQ a mano (tablas descifradas con la tabla de criptografia
de Storm). Es a proposito: el instrumento del host tiene que poder contradecir al motor.

Uso: sonda-mpq-compr.py [--mpqdir DIR] [--mpq a.mpq,b.mpq] [--archivo "(listfile)" | --archivo X.dc6]
                        [--sectores N] [--crudo]
"""
import argparse
import binascii
import os
import struct
import sys
import zlib

# ---------------------------------------------------------------------------------------------
# La tabla de criptografia de Storm (0x500 palabras): las 0x100 primeras filas son las 5 tablas de
# hash (tipos 0..4) y desde 0x400 esta la que usa el descifrado.
# ---------------------------------------------------------------------------------------------
def tabla_cripto():
    seed = 0x00100001
    t = [0] * 0x500
    for i1 in range(0x100):
        for i2 in range(5):
            seed = (seed * 125 + 3) % 0x2AAAAB
            a = (seed & 0xFFFF) << 16
            seed = (seed * 125 + 3) % 0x2AAAAB
            b = seed & 0xFFFF
            t[i1 + i2 * 0x100] = (a | b) & 0xFFFFFFFF
    return t


CRYPT = tabla_cripto()
M = 0xFFFFFFFF


def hash_string(s, tipo):
    seed1 = 0x7FED7FED
    seed2 = 0xEEEEEEEE
    for ch in s.upper():
        c = ord(ch)
        seed1 = (CRYPT[tipo * 0x100 + c] ^ ((seed1 + seed2) & M)) & M
        seed2 = (c + seed1 + seed2 + (seed2 << 5) + 3) & M
    return seed1


def descifrar(palabras, key):
    seed = 0xEEEEEEEE
    out = []
    for w in palabras:
        seed = (seed + CRYPT[0x400 + (key & 0xFF)]) & M
        v = (w ^ ((key + seed) & M)) & M
        key = ((((~key) & M) << 0x15) + 0x11111111 | (key >> 0x0B)) & M
        seed = (v + seed + (seed << 5) + 3) & M
        out.append(v)
    return out


def descifrar_bytes(datos, key):
    # OJO: solo se descifran las palabras completas; los 0..3 bytes de la cola quedan COMO ESTAN (es lo que
    # hace StormLib: `dwLength & ~0x03`). Descartarlos corta el flujo de zlib del sector.
    n = len(datos) // 4
    palabras = list(struct.unpack("<%dI" % n, datos[: n * 4]))
    return struct.pack("<%dI" % n, *descifrar(palabras, key)) + datos[n * 4:]


def leer_tabla(f, offset, entradas, key_str):
    f.seek(offset)
    crudo = f.read(entradas * 16)
    if len(crudo) != entradas * 16:
        raise SystemExit("tabla corta: %d de %d bytes" % (len(crudo), entradas * 16))
    return descifrar_bytes(crudo, hash_string(key_str, 3))


# ---------------------------------------------------------------------------------------------
FLAGS = [(0x00000100, "Implode"), (0x00000200, "Compress"), (0x00010000, "Encrypted"),
         (0x00020000, "FixKey"), (0x00100000, "PatchFile"), (0x01000000, "SingleUnit"),
         (0x02000000, "DeleteMarker"), (0x04000000, "SectorCrc"), (0x80000000, "Exists")]

TIPOS = {1: "Huffman", 2: "zlib", 8: "PKWARE implode", 0x10: "BZip2", 0x40: "IMA ADPCM mono",
         0x80: "IMA ADPCM stereo", 0x12: "LZMA", 0x22: "sparse+zlib", 0x30: "sparse+bzip2",
         0x41: "huffman+adpcm", 0x81: "adpcm"}


def decodificar_flags(v):
    nombres = [n for bit, n in FLAGS if v & bit]
    return "0x%08X (%s)" % (v, " | ".join(nombres) if nombres else "ninguna")


def bloque_de(f, hdr, nombre):
    """Devuelve el registro del bloque (16 bytes) para `nombre`, buscando en la tabla de hash."""
    ht_off, bt_off, ht_cnt, bt_cnt = hdr["ht_off"], hdr["bt_off"], hdr["ht_cnt"], hdr["bt_cnt"]
    ht = leer_tabla(f, ht_off, ht_cnt, "(hash table)")
    hA, hB = hash_string(nombre, 1), hash_string(nombre, 2)
    idx = hash_string(nombre, 0) & (ht_cnt - 1)
    for _ in range(0x100):
        e = struct.unpack_from("<IIII", ht, idx * 16)
        if e[3] == 0xFFFFFFFF:
            return None, ht
        if e[0] == hA and e[1] == hB:
            b = e[3]
            bt = leer_tabla(f, bt_off, bt_cnt, "(block table)")
            reg = struct.unpack_from("<IIII", bt, b * 16)
            return {"hash": e, "bloque": reg, "idx": idx, "bt_idx": b}, ht
        idx = (idx + 1) & (ht_cnt - 1)
    return None, ht


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--mpqdir", default=os.path.expanduser("~/.wine-d2/drive_c/Games/Diablo II"))
    ap.add_argument("--mpq", default="patch_d2.mpq,d2data.mpq,d2char.mpq")
    ap.add_argument("--archivo", default="(listfile)")
    ap.add_argument("--sectores", type=int, default=4)
    ap.add_argument("--crudo", action="store_true", help="volcar los primeros 32 bytes de cada sector")
    ap.add_argument("--extraer", default=None,
                    help="dir donde dejar los SECTORES DESCIFRADOS (los bytes guardados, tal como los ve "
                         "el motor tras descifrar) para hornearlos en un POC de la consola")
    args = ap.parse_args()

    for nombre_mpq in [x.strip() for x in args.mpq.split(",") if x.strip()]:
        ruta = os.path.join(args.mpqdir, nombre_mpq)
        if not os.path.exists(ruta):
            alt = [n for n in os.listdir(args.mpqdir) if n.lower() == nombre_mpq.lower()]
            if not alt:
                print("%-14s NO ESTA en %s" % (nombre_mpq, args.mpqdir))
                continue
            ruta = os.path.join(args.mpqdir, alt[0])

        with open(ruta, "rb") as f:
            h = f.read(32)
            magic = h[:4]
            hdr_size, archive_size, ver, blk_size = struct.unpack_from("<IIHH", h, 4)
            ht_off, bt_off, ht_cnt, bt_cnt = struct.unpack_from("<IIII", h, 16)
            hdr = dict(ht_off=ht_off, bt_off=bt_off, ht_cnt=ht_cnt, bt_cnt=bt_cnt,
                       hdr_size=hdr_size, blk_size=blk_size)

            print("=== %s  (%d bytes, magic=%r ver=%d, sector=0x200<<%d = %d B)"
                  % (nombre_mpq, os.path.getsize(ruta), magic, ver, blk_size, 0x200 << blk_size))
            print("    hashEnt=%d blockEnt=%d" % (ht_cnt, bt_cnt))

            r, _ = bloque_de(f, hdr, args.archivo)
            if r is None:
                print("    %s: NO ESTA en el archivo" % args.archivo)
                continue

            pos, csize, usize, flags = r["bloque"]
            print("    ARCHIVO %s  hashIdx=%d blockIdx=%d" % (args.archivo, r["idx"], r["bt_idx"]))
            print("      FilePosition=%d  CompressedFileSize=%d  UncompressedFileSize=%d" % (pos, csize, usize))
            print("      Flags=%s" % decodificar_flags(flags))

            # la clave del bloque: hash del nombre del ARCHIVO (tipo 3) y, con FixKey, corrida por
            # la posicion y el tamano real (`Block.calculateEncryptionSeed`). La tabla de offsets
            # usa esa clave MENOS UNO y los DATOS la clave MAS el indice del bloque.
            clave = (hash_string(os.path.basename(args.archivo), 3) + pos) ^ usize

            single = bool(flags & 0x01000000)
            comp = bool(flags & 0x00000200)
            impl = bool(flags & 0x00000100)
            print("      camino del motor: %s%s%s"
                  % ("SingleUnit -> loadSingleUnit (descomprime el archivo ENTERO de una)"
                     if single else "por SECTORES",
                     " + Compress" if comp else "",
                     " + Implode" if impl else ""))

            if not (comp or impl) or single:
                if single:
                    print("      OJO single-unit: el tamano guardado es el comprimido entero (%d)" % csize)
                print("      (sin tabla de offsets de sectores: el archivo se guarda de un tiron)")
            else:
                sector = 0x200 << blk_size
                nsect = (usize + sector - 1) // sector
                npos = nsect + 1
                print("      sectores=%d (tabla de offsets = %d palabras)" % (nsect, npos))
                f.seek(pos)
                crudo = f.read(npos * 4)
                offs = list(struct.unpack("<%dI" % (npos), crudo))
                if flags & 0x00010000:
                    offs = descifrar(offs, (clave - 1) & M)
                    print("      (offsets DESCIFRADOS: el bloque esta encriptado, clave=0x%08X)" % clave)
                print("      offsets(primeros 8)=%s" % offs[:8])
                for i in range(min(args.sectores, nsect)):
                    a, b = offs[i], offs[i + 1]
                    f.seek(pos + a)
                    datos = f.read(min(b - a, 64))
                    if flags & 0x00010000:
                        # el sector va ENCRIPTADO: la clave es la del bloque MAS el indice del sector
                        # (`mpq_stream.go`: `decryptBytes(data, blockIndex + v.Block.EncryptionSeed)`).
                        datos = descifrar_bytes(datos, (clave + i) & M)
                    guardado = b - a
                    esperado = min(sector, usize - i * sector)
                    tipo = datos[0] if datos else -1
                    forma = ("CRUDO (guardado == esperado, la compresion no achico)"
                             if guardado == esperado else "comprimido: %s" % TIPOS.get(tipo, "tipo 0x%X" % tipo))
                    print("      sector %d: guardado=%d esperado=%d  primer byte=0x%02X -> %s"
                          % (i, guardado, esperado, tipo & 0xFF, forma))
                    if args.crudo:
                        print("         %s" % binascii.hexlify(datos[:32]).decode())

            # LA PRUEBA DE PUNTA A PUNTA: descomprimir el archivo con la zlib de Python y contarlo.
            # Solo si el bloque es por sectores y zlib (que es el caso del (listfile)); para el resto se
            # dice lo que se puede decir sin inventar. Hace el camino COMPLETO igual que el motor:
            # offsets (descifrados), sector (descifrado si hace falta), y zlib sobre los datos sin el
            # primer byte.
            if comp and not single and not impl:
                sector = 0x200 << blk_size
                nsect = (usize + sector - 1) // sector
                cifrado = bool(flags & 0x00010000)
                f.seek(pos)
                offs = list(struct.unpack("<%dI" % (nsect + 1), f.read((nsect + 1) * 4)))
                if cifrado:
                    offs = descifrar(offs, (clave - 1) & M)
                salida = bytearray()
                fallos = 0
                crudos = 0
                for i in range(nsect):
                    a, b = offs[i], offs[i + 1]
                    esperado = min(sector, usize - i * sector)
                    f.seek(pos + a)
                    datos = f.read(b - a)
                    crudo_sec = datos
                    if cifrado:
                        datos = descifrar_bytes(datos, (clave + i) & M)
                    if b - a == esperado:
                        crudos += 1
                        salida += datos
                    else:
                        try:
                            salida += zlib.decompress(datos[1:])
                        except Exception as e:  # noqa: BLE001
                            fallos += 1
                            if fallos <= 2:
                                print("      sector %d: la zlib de Python TAMBIEN falla: %s" % (i, e))
                    if args.extraer:
                        os.makedirs(args.extraer, exist_ok=True)
                        nom = "%s_s%d.bin" % (os.path.splitext(nombre_mpq)[0], i)
                        with open(os.path.join(args.extraer, nom), "wb") as g:
                            g.write(datos)
                        if i == 0:
                            des = (datos if b - a == esperado else zlib.decompress(datos[1:]))
                            info = {
                                "mpq": nombre_mpq, "archivo": args.archivo,
                                "FilePosition": pos, "Flags": flags,
                                "sector0_stored_bytes": len(datos),
                                "sector0_stored_crc32": zlib.crc32(datos) & M,
                                "sector0_raw_crc32": zlib.crc32(crudo_sec) & M,
                                "sector0_inflado_bytes": len(des),
                                "sector0_inflado_crc32": zlib.crc32(des) & M,
                                "sector0_offset": a, "sector0_esperado": esperado,
                                "sectores": nsect, "uncompressed": usize,
                            }
                            with open(os.path.join(args.extraer, "host-%s.txt" % os.path.splitext(nombre_mpq)[0]),
                                      "w") as g2:
                                for k, v in info.items():
                                    g2.write("%s\t%s\n" % (k, v))
                            print("      EXTRAIDO %s: guardado=%d crc32=%d  inflado=%d crc32=%d  (offset=%d)"
                                  % (nom, len(datos), zlib.crc32(datos) & M, len(des),
                                     zlib.crc32(des) & M, a))
                texto = bytes(salida)
                lineas = [x for x in texto.split(b"\r\n") if x]
                print("      DESCOMPRIMIDO (zlib de Python): %d bytes de %d esperados, %d fallos, %d sectores "
                      "crudos, %d nombres, crc32=%d"
                      % (len(texto), usize, fallos, crudos, len(lineas), zlib.crc32(texto) & M))
                if lineas:
                    print("      primeros 3 nombres: %s" % [x.decode("latin1") for x in lineas[:3]])
            print()


if __name__ == "__main__":
    sys.exit(main())
