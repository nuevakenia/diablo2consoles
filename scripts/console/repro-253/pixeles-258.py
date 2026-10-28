#!/usr/bin/env python3
"""DC-002.58 -- el instrumento del veredicto "PINTA" del eslabon del NIVEL.

QUE MIDE: cuantos pixeles NO negros hay DENTRO DEL AREA DE VIDEO del emulador, en la captura de pantalla
completa que saca `spectacle -f`. El area de video la da el gestor de ventanas (`flycast-adelante.sh geo`,
que imprime `cli=x,y,w,h`), NO un recorte adivinado: en este host la ventana del emulador no queda en una
posicion fija, y con un recorte fijo lo que se mide es el fondo de pantalla (medido el 2026-10-27: 425.600
"no negros" en TODAS las capturas, o sea el escritorio entero, con el nivel pintado en otra parte).

POR QUE ESTE NUMERO SIRVE: el eslabon pinta con el PVR, que dibuja el fondo NEGRO; lo unico que deja pixeles de
color es el nivel (sus 193.538 pixeles de indice distinto de 0, que en la ventana de 640x480 y escalados a la
ventana real del emulador son una fraccion conocida del area). Una pantalla en negro da ~0 %; la corrida del
2026-10-27 dio 54,4 %. El umbral es 20 % del area (y un piso absoluto de 5.000 pixeles, para que una ventana
minimizada no pase por el area chica).

USO:  pixeles-258.py <captura.png> <x,y,w,h>     (la geometria del area de video del emulador)
SALE: 0 = PINTA, 1 = NO-PINTA, 2 = uso, 3 = no pude leer la imagen
"""
import sys

UMBRAL_NEGRO = 24     # suma R+G+B por debajo de esto = negro
FRACCION_MINIMA = 0.20
PISO_PIXELES = 5000


def main():
    if len(sys.argv) != 3:
        print(__doc__)
        return 2

    ruta = sys.argv[1]
    try:
        x, y, w, h = (int(v) for v in sys.argv[2].split(","))
    except ValueError:
        print("PIXELES: geometria invalida %r (se espera x,y,w,h)" % sys.argv[2])
        return 2

    from PIL import Image

    try:
        img = Image.open(ruta).convert("RGB")
    except Exception as e:  # noqa: BLE001 - el error se informa tal cual
        print("PIXELES: no pude abrir %s: %s" % (ruta, e))
        return 3

    datos = img.crop((x, y, x + w, y + h)).tobytes()
    cuenta = 0
    for i in range(0, len(datos), 3):
        if datos[i] + datos[i + 1] + datos[i + 2] > UMBRAL_NEGRO:
            cuenta += 1

    total = w * h
    fraccion = cuenta / total if total else 0.0
    print("PIXELES: area de video %dx%d en (%d,%d) de una captura de %dx%d"
          % (w, h, x, y, img.size[0], img.size[1]))
    print("PIXELES: no negros=%d de %d (%.1f%%), umbral=%.0f%% y piso %d"
          % (cuenta, total, 100.0 * fraccion, 100.0 * FRACCION_MINIMA, PISO_PIXELES))

    if cuenta >= PISO_PIXELES and fraccion >= FRACCION_MINIMA:
        print("VEREDICTO: PINTA (el area de video del emulador tiene %d pixeles pintados)" % cuenta)
        return 0

    print("VEREDICTO: NO-PINTA (el area de video del emulador quedo negra o casi negra)")
    return 1


if __name__ == "__main__":
    sys.exit(main())
