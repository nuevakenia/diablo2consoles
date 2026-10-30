#!/usr/bin/env python3
"""DC-002.69: medir la captura de la prueba de humo del backend PVR (d2render/pvr), sin ojos.

QUE MIDE (y por que asi): la captura es una foto del escritorio con la ventana del emulador adentro, y el
color pasa por RGB565 (5-6-5), que pierde 3 bits por canal. Asi que no se compara pixel a pixel contra el
cuadro RGBA de la consola; lo que SI se prueba, y es el hito, es:

  1. la pantalla de la consola esta en la captura y mide 640x480, con el marco amarillo de 2 lineas que
     dibuja el POC: 2 x 640 = 1.280 pixeles de marco;
  2. el contenido pintado coincide con el numero que imprimio la consola por la serie (no-vacios), con
     tolerancia (el borde del RGB565 mueve el conteo);
  3. el sprite por HARDWARE (rombo verde a escala 3) esta en su caja, que el cuadro compuesto por software
     NO cubre: eso separa el camino del PVR del rasterizador;
  4. el rombo compuesto POR SOFTWARE (el "heroe") esta en la suya.

UBICAR LA PANTALLA SIN CLAVAR UNA POSICION: la ventana del emulador no abre siempre en el mismo lugar
(medido en DC-002.9: 2830,385 en una corrida y 210,210 en otra). Se busca el tramo de columnas con mas
pixeles del FONDO del POC (es el unico lugar con cientos seguidos) y despues la fila que menos "pintado
ajeno" mete en el recorte (abajo de la ventana esta el escritorio, que es brillante).

Uso:  checar-pvr-surface.py <captura.png> [--consola N] [--x X --y Y]
Devuelve 0 si todo cuadra, 1 si algo no.
"""
import argparse
import os
import subprocess
import sys

ANCHO, ALTO = 640, 480
FONDO = (8, 8, 16)        # el (8,10,16) del POC pasado por RGB565 (5-6-5) y de vuelta
AMARILLO = (255, 202, 0)  # el marco (255,200,0) pasado por RGB565
VERDE = (0, 249, 0)       # el indice 3 de la paleta (0,255,0) pasado por RGB565

# las cajas del POC, en coordenadas del cuadro de 640x480. NO se solapan, y por eso separan los dos caminos
# de dibujado: una la pinta el rasterizador por software, la otra el PVR.
CAJA_HW = (470, 90, 470 + 72, 90 + 72)
CAJA_SW = (488, 356, 512, 380)


def cerca(p, q, tol=10):
    return abs(p[0] - q[0]) <= tol and abs(p[1] - q[1]) <= tol and abs(p[2] - q[2]) <= tol


def decodificar(png):
    info = subprocess.run(["ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries",
                           "stream=width,height", "-of", "csv=p=0", png],
                          stdout=subprocess.PIPE, check=True).stdout.decode().strip()
    w, h = [int(v) for v in info.split(",")]
    datos = subprocess.run(["ffmpeg", "-y", "-loglevel", "error", "-i", png,
                            "-f", "rawvideo", "-pix_fmt", "rgb24", "-"],
                           stdout=subprocess.PIPE, check=True).stdout
    return datos, w, h


def buscar_x(datos, w, h):
    """las columnas de la pantalla: el tramo mas largo de columnas con muchos pixeles de fondo"""
    columnas = [0] * w
    for y in range(0, h, 2):
        base = y * w * 3
        for x in range(w):
            j = base + x * 3
            if cerca((datos[j], datos[j + 1], datos[j + 2]), FONDO, 6):
                columnas[x] += 1

    mejor_ini, mejor_largo, ini = 0, 0, -1
    for x in range(w + 1):
        lleno = x < w and columnas[x] >= h // 20
        if lleno and ini < 0:
            ini = x
        elif not lleno and ini >= 0:
            if x - ini > mejor_largo:
                mejor_largo, mejor_ini = x - ini, ini
            ini = -1

    return mejor_ini


def buscar_y(datos, w, h, x0):
    """la fila donde empieza la pantalla: la que menos pixeles ajenos mete en el recorte de 640x480"""
    pintado_fila = [0] * h

    for y in range(h):
        base = y * w * 3
        p = 0
        for x in range(x0, min(x0 + ANCHO, w)):
            j = base + x * 3
            c = (datos[j], datos[j + 1], datos[j + 2])
            if not cerca(c, FONDO, 6) and not cerca(c, AMARILLO, 60):
                p += 1
        pintado_fila[y] = p

    acumulado = [0]
    for v in pintado_fila:
        acumulado.append(acumulado[-1] + v)

    mejor_y, mejor = 0, 1 << 30
    for y in range(0, max(0, h - ALTO)):
        total = acumulado[y + ALTO] - acumulado[y]
        if total < mejor:
            mejor, mejor_y = total, y

    return mejor_y


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("png")
    ap.add_argument("--consola", type=int, default=None, help="pixeles no-vacios que imprimio la consola")
    ap.add_argument("--x", type=int, default=None)
    ap.add_argument("--y", type=int, default=None)
    a = ap.parse_args()

    if not os.path.exists(a.png):
        print("NO EXISTE", a.png)
        return 1

    datos, w, h = decodificar(a.png)
    print("captura %dx%d" % (w, h))

    x0 = a.x if a.x is not None else buscar_x(datos, w, h)
    y0 = a.y if a.y is not None else buscar_y(datos, w, h, x0)
    print("pantalla del POC en (%d,%d) %dx%d" % (x0, y0, ANCHO, ALTO))

    pintados = 0
    amarillos = 0
    filas_marco = {}

    for y in range(ALTO):
        base = ((y0 + y) * w + x0) * 3
        for x in range(ANCHO):
            j = base + x * 3
            c = (datos[j], datos[j + 1], datos[j + 2])
            if cerca(c, AMARILLO, 60):
                amarillos += 1
                filas_marco[y] = filas_marco.get(y, 0) + 1
            elif not cerca(c, FONDO, 6):
                pintados += 1

    print("pixeles pintados (ni fondo ni marco): %d" % pintados)
    print("pixeles del marco amarillo: %d (se esperan ~%d: 2 lineas de %d)" % (amarillos, 2 * ANCHO, ANCHO))
    print("filas del marco: %s" % sorted(filas_marco.items())[:6])

    def verdes(caja):
        xa, ya, xb, yb = caja
        n = 0
        for y in range(ya, min(yb, ALTO)):
            base = ((y0 + y) * w + x0) * 3
            for x in range(xa, min(xb, ANCHO)):
                j = base + x * 3
                if cerca((datos[j], datos[j + 1], datos[j + 2]), VERDE, 60):
                    n += 1
        return n

    hw = verdes(CAJA_HW)
    sw = verdes(CAJA_SW)
    print("verdes del sprite por HARDWARE en %s: %d" % (CAJA_HW, hw))
    print("verdes del rombo por SOFTWARE en %s: %d" % (CAJA_SW, sw))

    veredicto = True

    if amarillos < 2 * ANCHO - 40:
        print("  FALLA: el marco amarillo no esta: %d de %d pixeles" % (amarillos, 2 * ANCHO))
        veredicto = False

    if a.consola is not None:
        dif = abs(pintados - a.consola)
        rel = 100.0 * dif / max(a.consola, 1)
        print("consola dice %d pintados; la captura %d: diferencia %d (%.2f%%)" % (a.consola, pintados, dif, rel))
        if rel > 12.0:
            print("  FALLA: la diferencia pasa del 12%% (se esperan solo los bordes del RGB565)")
            veredicto = False

    if hw < 200:
        print("  FALLA: el sprite por HARDWARE no se ve (se esperaban >200 pixeles verdes)")
        veredicto = False

    if sw < 100:
        print("  FALLA: el rombo compuesto POR SOFTWARE no se ve (se esperaban >100 pixeles verdes)")
        veredicto = False

    print("RESULTADO:", "OK" if veredicto else "NO")
    return 0 if veredicto else 1


if __name__ == "__main__":
    sys.exit(main())
