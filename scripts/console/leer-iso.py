#!/usr/bin/env python3
"""DC-002.27: lee un archivo DE ADENTRO de la ISO y dice su tamano y su sha256.

Para que: el criterio de aceptacion del disco es que el MPQ que viaja en la ISO sea el MISMO
archivo que el del PC (no un pedazo cortado ni corrido). Eso se mide del lado del HOST leyendo la
ISO con pycdlib y comparando la suma de comprobacion contra el archivo original; la consola mide
lo mismo del otro lado (abre el MPQ por /cd y suma sus bytes).

Uso:
    scripts/console/leer-iso.py <iso> <ruta_interna> [<ruta_interna> ...]
Las rutas internas son las de ISO9660 (las que escribe mkiso.py, p.ej. /JUEGO/PATCH_D2.MPQ;1).
Sin `;1` tambien las encuentra (pycdlib acepta las dos formas).
"""
import hashlib
import sys

import pycdlib


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        return 2

    iso_path, rutas = sys.argv[1], sys.argv[2:]
    iso = pycdlib.PyCdlib()
    iso.open(iso_path)

    fallos = 0

    for ruta in rutas:
        destino = "/tmp/leer-iso-227.bin"
        try:
            iso.get_file_from_iso(destino, iso_path=ruta)
        except Exception as e:  # noqa: BLE001 -- el mensaje de pycdlib es el dato
            print("ISO-LECTURA %s FALLO: %s" % (ruta, e))
            fallos += 1
            continue

        with open(destino, "rb") as fh:
            datos = fh.read()

        print("ISO-LECTURA %s bytes=%d sha256=%s" % (ruta, len(datos), hashlib.sha256(datos).hexdigest()))

    iso.close()

    return 1 if fallos else 0


if __name__ == "__main__":
    sys.exit(main())
