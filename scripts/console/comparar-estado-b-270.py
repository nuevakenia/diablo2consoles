#!/usr/bin/env python3
"""DC-002.70: comparar dos capturas de la pantalla de la consola, pixel a pixel.

QUE COMPARA
  Dos fotos del escritorio con la ventana de Flycast adentro: la del POC de DC-002.38 (mapa + personaje
  dibujados por el shim de Ebitengine) y la del POC del ESTADO B (el MISMO mapa y el MISMO personaje
  dibujados por d2interface.Surface con el backend PVR y su Renderer). Las dos pasan por el mismo emulador,
  asi que la comparacion mide el DIBUJADO, no el video.

COMO SE UBICA LA PANTALLA (sin clavar una posicion)
  La pantalla de la PRIMERA captura se ubica por su firma geometrica: el mapa de 512x512 se pega en
  (64, -16), asi que quedan DOS BANDAS NEGRAS de 64 columnas (x 0..63 y x 576..639). Se busca el tramo de 64
  columnas negro en (casi) todas las filas y su simetrico a la derecha: eso da x0 (y dice que la escala es
  1:1, medido: la ventana de Flycast no reescala), y el tramo de filas con las dos bandas negras da y0.

  La SEGUNDA captura se alinea contra la primera por correlacion: se busca el desplazamiento que maximiza los
  pixeles que coinciden (tolerancia 8) en una franja de 200x512 del cuadro. Se hace asi porque la ventana del
  emulador no siempre queda limpia: medido en DC-002.70, en una de las dos corridas el borde de la ventana
  tapaba las primeras 26 filas de las bandas y el ubicador por bandas se equivocaba de fila (daba 146 en vez
  de 120) -- la correlacion encontro el lugar exacto (100 % de coincidencia).

LAS ULTIMAS 6 FILAS NO SE COMPARAN Y SE DICE POR QUE
  La ventana del emulador dibuja su borde inferior sobre las ultimas filas del cuadro (medido: en la fila 600
  de la captura, las columnas de las bandas son (83,85,88), el gris del borde). Se comparan las 474 filas que
  el emulador muestra enteras y se informa el numero.

QUE REPORTA
  1. donde quedo la pantalla en cada captura;
  2. pixeles IDENTICOS (los tres canales iguales) sobre las filas comparadas;
  3. de los que difieren: cuantos <= 8 por canal (la cuantizacion del framebuffer: el PVR convierte a RGB565
     y Flycast lo devuelve, con su dithering) y cuantos MAS de 8 (esos son contenido distinto: hay que
     buscarlos);
  4. la caja de los que difieren mas de 8 y una muestra de ellos.

Uso:  comparar-estado-b-270.py <captura-estado-b.png> <captura-poc.png>
Devuelve 0 si cierra (ningun pixel fuera de la cuantizacion), 1 si no.
"""
import subprocess
import sys

ANCHO, ALTO = 640, 480
BANDA = 64      # (640 - 512) / 2
FILAS = 474     # las que el emulador muestra enteras (ver la cabecera)
TOL = 8         # lo que puede mover el RGB565 + el dithering de Flycast


def decodificar(png):
    info = subprocess.run(["ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries",
                           "stream=width,height", "-of", "csv=p=0", png],
                          stdout=subprocess.PIPE, check=True).stdout.decode().strip()
    w, h = [int(v) for v in info.split(",")[:2]]
    datos = subprocess.run(["ffmpeg", "-y", "-loglevel", "error", "-i", png,
                            "-f", "rawvideo", "-pix_fmt", "rgb24", "-"],
                           stdout=subprocess.PIPE, check=True).stdout
    return datos, w, h


def es_negro(datos, w, x, y):
    j = (y * w + x) * 3
    return datos[j] == 0 and datos[j + 1] == 0 and datos[j + 2] == 0


def ubicar(datos, w, h):
    """(x0, y0, filas) de la pantalla: tramo de 64 columnas negras y su simetrico a la derecha"""
    negras = [0] * w
    for y in range(h):
        base = y * w * 3
        for x in range(w):
            j = base + x * 3
            if datos[j] == 0 and datos[j + 1] == 0 and datos[j + 2] == 0:
                negras[x] += 1

    mejor = None
    for x0 in range(0, w - ANCHO + 1):
        izq = min(negras[x0:x0 + BANDA])
        der = min(negras[x0 + ANCHO - BANDA:x0 + ANCHO])
        alto = min(izq, der)
        if alto < FILAS * 0.9:
            continue
        if mejor is None or alto > mejor[1]:
            mejor = (x0, alto)

    if mejor is None:
        return None

    x0 = mejor[0]

    empieza, largo, mejorY, mejorLargo = -1, 0, 0, 0
    for y in range(h):
        limpia = es_negro(datos, w, x0, y) and es_negro(datos, w, x0 + ANCHO - 1, y)
        if limpia:
            if empieza < 0:
                empieza, largo = y, 0
            largo += 1
        else:
            if largo > mejorLargo:
                mejorLargo, mejorY = largo, empieza
            empieza = -1

    if largo > mejorLargo:
        mejorLargo, mejorY = largo, empieza

    return x0, mejorY, mejorLargo


def alinear(datos_a, wa, xa, ya, datos_b, wb, hb, radio=140):
    """el (x0, y0) de la captura B que mejor coincide con la franja superior de la A"""
    mejor = None

    for dy in range(-radio, radio + 1, 2):
        for dx in range(-radio, radio + 1, 2):
            xb, yb = xa + dx, ya + dy
            if xb < 0 or yb < 0 or xb + ANCHO > wb or yb + ALTO > hb:
                continue

            acuerdo = 0
            for y in range(60, 260, 2):
                for x in range(BANDA, ANCHO - BANDA, 2):
                    ja = (ya + y) * wa * 3 + (xa + x) * 3
                    jb = (yb + y) * wb * 3 + (xb + x) * 3
                    if abs(datos_a[ja] - datos_b[jb]) <= TOL and \
                       abs(datos_a[ja + 1] - datos_b[jb + 1]) <= TOL and \
                       abs(datos_a[ja + 2] - datos_b[jb + 2]) <= TOL:
                        acuerdo += 1

            if mejor is None or acuerdo > mejor[2]:
                mejor = (xb, yb, acuerdo)

    return mejor


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        return 2

    datos_a, wa, ha = decodificar(sys.argv[1])
    datos_b, wb, hb = decodificar(sys.argv[2])

    a = ubicar(datos_a, wa, ha)
    if a is None:
        print("NO PUDE UBICAR LA PANTALLA en %s" % sys.argv[1])
        return 1

    xa, ya = a[0], a[1]
    print("estado B %s (%dx%d) -> pantalla en %s" % (sys.argv[1], wa, ha, a))

    b = alinear(datos_a, wa, xa, ya, datos_b, wb, hb)
    if b is None:
        print("NO PUDE ALINEAR la segunda captura contra la primera")
        return 1

    xb, yb, acuerdo = b
    print("POC      %s (%dx%d) -> pantalla en (%d, %d)  [acuerdo del alineado: %d pixeles]"
          % (sys.argv[2], wb, hb, xb, yb, acuerdo))

    identicos = 0
    cuantizacion = 0
    otra = 0
    suma = 0
    caja = [ANCHO, ALTO, -1, -1]
    total = 0
    muestra = []

    for y in range(FILAS):
        for x in range(ANCHO):
            ja = (ya + y) * wa * 3 + (xa + x) * 3
            jb = (yb + y) * wb * 3 + (xb + x) * 3
            d0 = abs(datos_a[ja] - datos_b[jb])
            d1 = abs(datos_a[ja + 1] - datos_b[jb + 1])
            d2 = abs(datos_a[ja + 2] - datos_b[jb + 2])
            peor = max(d0, d1, d2)
            total += 1
            suma += d0 + d1 + d2

            if peor == 0:
                identicos += 1
            elif peor <= TOL:
                cuantizacion += 1
            else:
                otra += 1
                caja[0] = min(caja[0], x)
                caja[1] = min(caja[1], y)
                caja[2] = max(caja[2], x)
                caja[3] = max(caja[3], y)
                if len(muestra) < 8:
                    muestra.append((x, y, (datos_a[ja], datos_a[ja + 1], datos_a[ja + 2]),
                                    (datos_b[jb], datos_b[jb + 1], datos_b[jb + 2])))

    print("")
    print("filas comparadas  %d de %d (las ultimas 6 son el borde de la ventana del emulador)" % (FILAS, ALTO))
    print("pixeles           %d" % total)
    print("IDENTICOS         %d  (%.2f %%)" % (identicos, 100.0 * identicos / total))
    print("  cuantizacion    %d  (%.2f %%): difieren <= %d por canal (RGB565 del PVR + dithering)" %
          (cuantizacion, 100.0 * cuantizacion / total, TOL))
    print("  CONTENIDO       %d  (%.2f %%)" % (otra, 100.0 * otra / total))
    print("diferencia media por pixel (suma de 3 canales): %.3f" % (suma / total))

    if otra:
        print("caja de los pixeles de contenido: x %d..%d  y %d..%d" % (caja[0], caja[2], caja[1], caja[3]))
        for d in muestra:
            print("   (%d,%d) estado B %s  vs  POC %s" % d)
        print("RESULTADO: revisar (hay pixeles de CONTENIDO distinto)")
        return 1

    print("RESULTADO: OK -- el contenido coincide; la diferencia es solo la profundidad de color del video")
    return 0


if __name__ == "__main__":
    sys.exit(main())
