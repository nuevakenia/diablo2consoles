#!/usr/bin/env python3
"""Arma una imagen de GD-ROM arrancable para Dreamcast.

Reemplaza al mkisofs -G del flujo clasico (en este host no hay mkisofs ni genisoimage):
  1) construye un ISO9660 normal con pycdlib,
  2) inyecta el IP.BIN en los primeros 16 sectores (32 KB) = area de sistema del ISO,
que es exactamente lo que hace `mkisofs -G ip.bin`.

Uso:  python3 mkiso.py <salida.iso> <IP.BIN> <archivo1> [archivo2 ...]
Los archivos se agregan en la raiz del ISO con su nombre en mayusculas.

DC-002.27: un archivo se puede poner en una CARPETA del ISO con la forma
`origen:ruta_interna` (la ruta interna empieza con `/`, p.ej.
`.../d2data.mpq:/JUEGO/D2DATA.MPQ;1`). Las carpetas que falten se crean solas: pycdlib NO las
crea y contesta `PyCdlibInvalidInput: Could not find path` cuando el directorio padre no existe
(medido). Sin la forma con dos puntos, el comportamiento es el de antes.

DC-002.31: la lista se puede pasar en un ARCHIVO, con la forma `@/ruta/de/lista.txt` (una linea
por archivo, en las formas de arriba). Existe porque el arbol suelto del juego son 20.976 archivos
y la lista de argumentos se pasa del tope del kernel: el build del disco moria con "La lista de
argumentos es demasiado larga" (E2BIG medido; ver la evidencia de la tarjeta). Con la lista en un
archivo, y medido, pycdlib ACEPTA los nombres largos del arbol suelto (nivel 3): una entrada
`/JUEGO/DATA/GLOBAL/UI/FRONTEND/CINEMATICSELECTEXP.DC6;1` de 23 caracteres de nombre y cuatro
niveles de carpeta queda escrita en el disco con ese nombre.
"""
import os
import sys

import pycdlib


def partir(arg):
    """Separa `origen:ruta_interna`. El separador es el ULTIMO ':' cuyo resto empieza con '/' (asi
    una ruta del host con ':' adentro no se parte al medio). Sin dos puntos: raiz, nombre en
    mayusculas (el comportamiento de siempre)."""
    i = arg.rfind(":")
    if i > 0 and arg[i + 1:].startswith("/"):
        return arg[:i], arg[i + 1:]

    return arg, "/%s;1" % os.path.basename(arg).upper()


def expandir(args):
    """Los `@archivo` se cambian por las lineas de ese archivo (una linea = un archivo, los vacios se
    saltan). DC-002.31: sin esto la lista de 20.976 archivos del arbol suelto no entra en argv."""
    salida = []
    for a in args:
        if a.startswith("@"):
            with open(a[1:], encoding="utf-8") as fh:
                salida.extend(l.rstrip("\n") for l in fh if l.strip())
        else:
            salida.append(a)

    return salida


def main():
    if len(sys.argv) < 4:
        print(__doc__)
        return 2
    out, ipbin, files = sys.argv[1], sys.argv[2], expandir(sys.argv[3:])

    iso = pycdlib.PyCdlib()
    # DC-002.27: MKISO_JOLIET=0 arma la imagen SIN el arbol Joliet (solo el ISO9660 de nivel 3 con los
    # nombres en mayusculas y el `;1`). Existe porque el driver iso9660 de KOS busca el Joliet primero y se
    # midio (ver la evidencia de la tarjeta) que su busqueda de nombres falla con el arbol Joliet de
    # pycdlib: con el arbol pelado la ruta abre. Por defecto, Joliet como siempre (nivel 3).
    joliet = int(os.environ.get("MKISO_JOLIET", "3"))
    # sin Joliet: no se le pasa el parametro (pycdlib rechaza joliet=0 con "Invalid Joliet level")
    if joliet > 0:
        iso.new(interchange_level=3, joliet=joliet, vol_ident="D2CONSOLES",
                sys_ident="SEGA SEGAKATANA", vol_set_ident="SEGA GD-ROM")
    else:
        iso.new(interchange_level=3, vol_ident="D2CONSOLES",
                sys_ident="SEGA SEGAKATANA", vol_set_ident="SEGA GD-ROM")

    creados = set()

    for f in files:
        origen, destino = partir(f)
        if not os.path.isfile(origen):
            sys.exit("no encuentro el archivo %s" % origen)

        # las carpetas de arriba, de la raiz hacia abajo (pycdlib necesita el padre antes que el hijo)
        partes = destino.strip("/").split("/")[:-1]
        acumulado = ""
        for parte in partes:
            acumulado += "/" + parte
            if acumulado not in creados:
                iso.add_directory(acumulado)
                creados.add(acumulado)

        iso.add_file(origen, iso_path=destino)
        print("ISO+: %s -> %s (%d bytes)" % (origen, destino, os.path.getsize(origen)))

    iso.write(out)
    iso.close()

    with open(ipbin, "rb") as fh:
        ip = fh.read()
    with open(out, "rb") as fh:
        data = fh.read()
    if len(ip) != 32768:
        print("AVISO: IP.BIN mide %d bytes, se esperaban 32768" % len(ip))
    with open(out, "wb") as fh:
        fh.write(ip[:32768].ljust(32768, b"\0") + data[32768:])

    # Flycast no abre un .iso suelto (lo manda a libcdio y falla): hay que envolverlo en un .cue.
    # Con MODE1/2048 el emulador lee los sectores de 2048 tal cual, sin rearmar EDC/ECC.
    cue = os.path.splitext(out)[0] + ".cue"
    with open(cue, "w") as fh:
        fh.write('FILE "%s" BINARY\n  TRACK 01 MODE1/2048\n    INDEX 01 00:00:00\n' % os.path.basename(out))
    print("CUE: %s (TRACK 01 MODE1/2048 -> %s)" % (cue, os.path.basename(out)))
    print("ISO: %s (%d bytes, con IP.BIN en el area de sistema)" % (out, len(ip[:32768]) + len(data[32768:])))
    return 0


if __name__ == "__main__":
    sys.exit(main())
