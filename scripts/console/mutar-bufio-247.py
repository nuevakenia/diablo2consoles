#!/usr/bin/env python3
"""DC-002.47: EL ULTIMO TROZO DEL STREAM NO SE PIERDE -- el arreglo de `bufio.ReadBytes` y su MUTANTE.

POR QUE ES UNA SOLA VOLUNTAD: el arreglo de esta tarjeta y su mutante son el MISMO cambio al reves, y el
mutante tiene que dejar el lector como estaba (devolviendo el error de `rellenar` ANTES de mirar si el buffer
quedo con bytes) porque ESE es el estado que mide la tarjeta: con el mutante puesto el arranque del motor se
queda en las 3 tablas de DC-002.45 (LvlWarp.txt son 3.935 B, entra en un pedido de 4.096 B y el stream del
MPQ lo entrega JUNTO con `io.EOF`; el lector viejo tiraba esos bytes y la tabla se leia vacia).

LO QUE CAMBIA (el cuerpo del bucle de `console/dc/std/bufio/bufio.go`):

  MUTANTE (el de antes de esta tarjeta):
        salida = append(salida, trozo...)
        b.r = b.w
        if err := b.rellenar(); err != nil {
            return salida, err
        }

  SANO (el de esta tarjeta, igual que el `ReadSlice` de Go):
        salida = append(salida, trozo...)
        b.r = b.w
        // SOLO se pide mas si el lector no fallo todavia: `rellenar` deja el error en b.err y los bytes
        // (si los hubo) en el buffer, y la vuelta de arriba los devuelve con el error.
        if b.err == nil {
            b.rellenar()
        }

Uso:  mutar-bufio-247.py --estado     (que dice el arbol: sano, mutante u otro)
      mutar-bufio-247.py --mutante    (el lector viejo; es el que mide el corpus)
      mutar-bufio-247.py --sano       (el arreglo)

Es IDEMPOTENTE y cada modo verifica que el archivo quedo como se pidio (si no, sale con error).
"""

import os
import signal
import sys

signal.signal(signal.SIGPIPE, signal.SIG_DFL)

RAIZ = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
ARCHIVO = "console/dc/std/bufio/bufio.go"

os.chdir(RAIZ)

SANO = """\t\tsalida = append(salida, trozo...)
\t\tb.r = b.w
\t\t// SOLO se pide mas si el lector no fallo todavia: `rellenar` deja el error en b.err y los bytes
\t\t// (si los hubo) en el buffer, y la vuelta de arriba los devuelve con el error.
\t\tif b.err == nil {
\t\t\tb.rellenar()
\t\t}
"""

MUTANTE = """\t\tsalida = append(salida, trozo...)
\t\tb.r = b.w
\t\tif err := b.rellenar(); err != nil {
\t\t\treturn salida, err
\t\t}
"""


def main() -> int:
    modo = [a for a in sys.argv[1:] if a.startswith("--")]
    ruta = os.path.join(RAIZ, ARCHIVO)
    if not os.path.exists(ruta):
        print("no encuentro %s" % ruta)
        return 1

    texto = open(ruta).read()
    tiene_sano = texto.count(SANO) == 1
    tiene_mutante = texto.count(MUTANTE) == 1

    if "--estado" in modo:
        if tiene_sano:
            print("estado: sano (el ultimo trozo se devuelve con el error, como el ReadSlice de Go)")
            return 0
        if tiene_mutante:
            print("estado: mutante (el error de rellenar se devuelve antes de mirar el buffer)")
            return 0
        print("estado: OTRO (no encuentro ni el bloque del arreglo ni el del mutante)")
        return 1

    if not tiene_sano and not tiene_mutante:
        print("no encuentro el bloque que hay que cambiar en %s: el archivo no esta en ninguno de los dos "
              "estados (se edito a mano?)" % ARCHIVO)
        return 1

    if "--mutante" in modo:
        nuevo, esperado, nombre = texto.replace(SANO, MUTANTE), MUTANTE, "mutante"
    elif "--sano" in modo:
        nuevo, esperado, nombre = texto.replace(MUTANTE, SANO), SANO, "sano"
    else:
        print(__doc__)
        return 2

    if nuevo != texto:
        open(ruta, "w").write(nuevo)
        print("%s -> %s" % (ARCHIVO, nombre))
    else:
        print("%s ya estaba en %s" % (ARCHIVO, nombre))

    if open(ruta).read().count(esperado) != 1:
        print("el archivo NO quedo en %s" % nombre)
        return 1
    print("verificado: %s quedo %s" % (ARCHIVO, nombre))
    return 0


if __name__ == "__main__":
    sys.exit(main())
