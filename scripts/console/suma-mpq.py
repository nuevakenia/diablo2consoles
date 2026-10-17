#!/usr/bin/env python3
"""DC-002.27: la MISMA suma que calcula la consola, del lado del host.

Para que: la suma de los bytes de un MPQ se calcula en los dos lados con el mismo algoritmo
(`suma = suma*31 + byte`, aritmetica de 32 bits) y se comparan los numeros. Si la consola leyera un
pedazo corrido, o corto, o de otro archivo, las sumas no pueden coincidir.

La consola suma DOS trozos de cada archivo (el principio y el final, `topeSuma` = 4 MiB cada uno),
porque el MPQ tiene su indice al final: aca se suman los mismos dos trozos.

Uso:  scripts/console/suma-mpq.py <archivo> [bytes_por_trozo]
Salida (una linea por trozo, como la de la consola):
    SUMA-HOST principio <archivo> desde=0 leidos=N de=TOTAL suma=S
    SUMA-HOST final <archivo> desde=X leidos=N de=TOTAL suma=S
"""
import sys

TOPE = 4 << 20
MASCARA = 0xFFFFFFFF


def suma(datos):
    s = 0
    for b in datos:
        s = (s * 31 + b) & MASCARA

    return s


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        return 2

    ruta = sys.argv[1]
    tope = int(sys.argv[2]) if len(sys.argv) > 2 else TOPE

    with open(ruta, "rb") as fh:
        fh.seek(0, 2)
        total = fh.tell()

        cuantos = min(tope, total)

        fh.seek(0)
        cabeza = fh.read(cuantos)
        print("SUMA-HOST principio %s desde=0 leidos=%d de=%d suma=%d" %
              (ruta, len(cabeza), total, suma(cabeza)))

        fh.seek(total - cuantos)
        cola = fh.read(cuantos)
        print("SUMA-HOST final %s desde=%d leidos=%d de=%d suma=%d" %
              (ruta, total - cuantos, len(cola), total, suma(cola)))

    return 0


if __name__ == "__main__":
    sys.exit(main())
