#!/usr/bin/env python3
"""DC-002.17: comprueba en la CAPTURA que el marco amarillo que dibuja el POC motor-pantalla esta entero.

POR QUE ESTE CHEQUEO Y NO "SE VE BIEN": el arreglo del video (DC-002.17) cambio la subida del cuadro a la
VRAM: antes se convertia el cuadro entero en un buffer de 1 MiB y se subia de una, ahora se arman bloques de
32x32 y se sube cada uno por separado (300 subidas por cuadro). Un error en la cuenta del indice de un bloque
NO se nota en la consola serie: el video "arranca" igual y la huella del motor se calcula sobre la imagen de
Go, antes de la conversion. Lo unico que lo delata es la PANTALLA.

El POC dibuja, ademas del logo, un MARCO de 1 pixel en los cuatro bordes de la pantalla (640x480) en amarillo
puro (255,255,0). Ese marco cruza 20 bloques por arriba y por abajo y 15 por cada lado: si un bloque se
subiera al indice equivocado, el marco se cortaria o apareceria corrido, y este chequeo lo dice con un numero
(la cantidad de pixeles del marco que NO son amarillos).

Uso:  python3 scripts/console/medir-marco-217.py <captura.png> [--fondo R G B]
Sale con 0 si el marco esta entero y 1 si no.
"""
import argparse
import subprocess
import sys

AMARILLO = (255, 255, 0)
TOL = 40            # el PVR guarda 5-6-5: el amarillo puro vuelve con +/-8 por canal, se deja margen


def decodificar(png):
    out = subprocess.run(
        ["ffmpeg", "-v", "error", "-i", png, "-f", "rawvideo", "-pix_fmt", "rgb24", "-"],
        capture_output=True)
    if out.returncode != 0:
        sys.exit("ffmpeg fallo: " + out.stderr.decode()[:300])
    datos = out.stdout

    info = subprocess.run(
        ["ffprobe", "-v", "error", "-select_streams", "v",
         "-show_entries", "stream=width,height", "-of", "csv=p=0", png],
        capture_output=True, text=True).stdout.strip()
    ancho, alto = (int(v) for v in info.split(",")[:2])
    return datos, ancho, alto


def es_amarillo(datos, ancho, x, y):
    i = (y * ancho + x) * 3
    r, g, b = datos[i], datos[i + 1], datos[i + 2]
    return (abs(r - AMARILLO[0]) <= TOL and abs(g - AMARILLO[1]) <= TOL
            and abs(b - AMARILLO[2]) <= TOL)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("captura")
    ap.add_argument("--ancho-pantalla", type=int, default=640)
    ap.add_argument("--alto-pantalla", type=int, default=480)
    args = ap.parse_args()

    datos, ancho, alto = decodificar(args.captura)
    print("captura:", args.captura, ancho, "x", alto)

    # 1) ubicar la pantalla de la consola: la fila que tenga el tramo amarillo mas largo (el borde de arriba)
    mejor = (0, -1, -1, -1)      # largo, x0, x1, y
    for y in range(alto):
        x = 0
        while x < ancho:
            if es_amarillo(datos, ancho, x, y):
                x0 = x
                while x < ancho and es_amarillo(datos, ancho, x, y):
                    x += 1
                if x - x0 > mejor[0]:
                    mejor = (x - x0, x0, x - 1, y)
            else:
                x += 1

    largo, x0, x1, y0 = mejor
    print("el tramo amarillo mas largo de la captura:", largo, "px en x", x0, "..", x1, "y", y0)
    if largo < args.ancho_pantalla // 2:
        print("RESULTADO: NO ENCUENTRO EL MARCO (el tramo mas largo no llega a la mitad de", args.ancho_pantalla, "px)")
        return 1

    # 1:1 o escalado. La ventana de Flycast no siempre mide 640x480 (medido: 446 px de ancho, o sea escala
    # 0,697), asi que la escala se DEDUCE del tramo encontrado y no se supone.
    if largo == args.ancho_pantalla:
        escala = 1.0
    else:
        escala = largo / float(args.ancho_pantalla)
    alto_pantalla = int(round(args.alto_pantalla * escala))
    print("escala deducida del marco:", round(escala, 4),
          "-> la pantalla ocupa", largo, "x", alto_pantalla, "px en la captura")

    # 2) el marco entero: los 4 bordes (con la escala deducida). La posicion del borde de abajo y del de la
    # derecha se busca en +-3 px: la ventana de Flycast no mide 640x480 y el redondeo de la escala mueve el
    # borde un pixel (medido: escala 0,6969 -> 334,4 px de alto). NO se afloja el color: un bloque subido al
    # indice equivocado deja un tramo de 22 px sin amarillo, que se ve igual con cualquier tolerancia.
    def cubierto(puntos):
        bien = 0
        for (x, y) in puntos:
            if 0 <= x < ancho and 0 <= y < alto and es_amarillo(datos, ancho, x, y):
                bien += 1
        return bien, len(puntos)

    def mejor_fila(y_esperado, xs):
        mejor = (0, y_esperado)
        for y in range(y_esperado - 3, y_esperado + 4):
            b, n = cubierto([(x, y) for x in xs])
            if b > mejor[0]:
                mejor = (b, y)
        return mejor[0], mejor[1], len(xs)

    def mejor_columna(x_esperado, ys):
        mejor = (0, x_esperado)
        for x in range(x_esperado - 3, x_esperado + 4):
            b, n = cubierto([(x, y) for y in ys])
            if b > mejor[0]:
                mejor = (b, x)
        return mejor[0], mejor[1], len(ys)

    xs = list(range(x0, x0 + largo))
    ys = list(range(y0, y0 + alto_pantalla))

    resultados = []
    b, n = cubierto([(x, y0) for x in xs])
    resultados.append(("arriba", b, n))
    b, y, n = mejor_fila(y0 + alto_pantalla - 1, xs)
    resultados.append(("abajo(y=%d)" % y, b, n))
    b, n = cubierto([(x0, y) for y in ys])
    resultados.append(("izquierda", b, n))
    b, x, n = mejor_columna(x0 + largo - 1, ys)
    resultados.append(("derecha(x=%d)" % x, b, n))

    total, con = 0, 0
    for nombre, b, n in resultados:
        total += n
        con += b
        print("  borde", nombre, ":", b, "/", n, "pixeles amarillos")

    print("pixeles del marco revisados:", total)
    print("pixeles del marco que NO son amarillos:", total - con)
    if con != total:
        print("RESULTADO: MARCO ROTO (los bloques de 32x32 no estan donde tienen que estar)")
        return 1

    print("RESULTADO: MARCO ENTERO", con, "/", total, "pixeles amarillos")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
