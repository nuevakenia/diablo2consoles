#!/usr/bin/env python3
"""DC-002.38 -- MEDIR una captura del POC del personaje en el mapa, contra la referencia del host.

QUE CONTESTA (los numeros del criterio 1, medidos, no mirados):

  1. `identidad`: la pantalla de la consola es la MISMA imagen que la referencia del host (el mapa compuesto
     por el motor + el personaje encima), comparada a 64x60 para que el reescalado del emulador no sea ruido
     (medido en DC-002.37: a 512 la identidad es ruido, a 64 es estable). Y se contrasta con las otras tres
     orientaciones (espejada H, espejada V, rotada 180), que tienen que quedar MUY abajo: si la identidad no
     se despega de ellas, el numero no significa nada.
  2. `personaje`: cuantos pixeles de la captura NO son el mapa solo (la referencia `-png-mapa`) y donde esta
     su centro de masa. Eso mide dos cosas de una: que el personaje SE PINTA (px > 0) y DONDE quedo. El
     centro tiene que coincidir con el pixel del ancla que el POC imprime en la consola serie (HITO238: POS),
     y tiene que MOVERSE en la direccion de la tecla: es el criterio de que el control mueve al personaje del
     motor sobre el mapa.

LA GEOMETRIA (por que hay cuentas de por medio)
-----------------------------------------------
El POC pinta en una pantalla logica de 640x480 el mapa de 512x512 en (64,-16) -- o sea: 64 px de negro a cada
lado y las 16 filas de arriba y de abajo del mapa fuera de la pantalla -- y el personaje encima. El emulador
mete esa pantalla en el AREA CLIENTE de su ventana manteniendo la proporcion, asi que:

    escala   = alto_cliente / 480
    pantalla = 640*escala x alto_cliente, centrada en el cliente
    bloque   = la parte del mapa visible: x 64..576 de la pantalla logica, todo el alto

El bloque se compara con la referencia recortada igual. La geometria del cliente la pasa `--ventana` (la mide
`run-flycast-238.sh` preguntandole a KWin); sin ella se cae a la deteccion por banda negra, que en este POC
es MUY solida: el mapa deja 64 px de negro a cada lado (y el fondo es negro puro), asi que las dos bandas
verticales son el propio margen del mapa.

Uso:
  medir-captura-238.py <captura.png> --ref referencia.png --ref-mapa mapa.png [--ventana L,T,W,H]

Sale 0 siempre: los numeros son el resultado (el veredicto lo da el corpus `checar-personaje-238.sh`).
"""
import subprocess
import sys

import numpy as np

UMBRAL_NEGRO = 12        # negro puro: el fondo del PVR es 0,0,0 exacto
ANCHO_LOGICO, ALTO_LOGICO = 640, 480
MAPA_X, MAPA_Y, MAPA_LADO = 64, -16, 512   # donde pinta el mapa el POC (ver main.go)
LADO_VEREDICTO = 64      # el lado mayor de la comparacion de identidad
ORIENTACIONES = ("identidad", "espejada-h", "espejada-v", "rotada-180")
UMBRAL_DIF = 32          # "este pixel no es el mismo": diferencia maxima por canal, en 0..255
MIN_PIXELES_PERSONAJE = 200  # un personaje de 26x79 tiene 896 pixeles; menos de 200 es ruido del reescalado


def decodificar(png):
    info = subprocess.run(["ffprobe", "-v", "error", "-select_streams", "v", "-show_entries",
                           "stream=width,height", "-of", "csv=p=0", png],
                          capture_output=True, check=True, text=True).stdout.strip().split(",")
    w, h = int(info[0]), int(info[1])
    raw = subprocess.run(["ffmpeg", "-v", "error", "-i", png, "-f", "rawvideo", "-pix_fmt", "rgb24", "-"],
                         capture_output=True, check=True).stdout
    return np.frombuffer(raw, dtype=np.uint8).reshape(h, w, 3)


def bajar(img, x0, y0, ancho, alto, destino_x, destino_y):
    """Box-downsample (promedio) del bloque (x0,y0,ancho,alto) al tamano pedido.

    OJO: si el destino es MAS GRANDE que el origen (re-escalar 483 px a 512), hay celdas del destino que no
    reciben ninguna columna del origen: el peso queda en 0 y la division da NaN. Se acota el peso a 1 (queda
    el promedio de lo poco que cayo ahi, que es el comportamiento util).
    """
    bloque = img[y0:y0 + alto, x0:x0 + ancho].astype(np.uint32)
    bx = (np.arange(destino_x + 1) * ancho) // destino_x
    by = (np.arange(destino_y + 1) * alto) // destino_y
    filas = np.add.reduceat(bloque, by[:-1], axis=0)
    celdas = np.add.reduceat(filas, bx[:-1], axis=1)
    peso = (np.diff(bx)[None, :] * np.diff(by)[:, None])[:, :, None]
    return (celdas // np.maximum(peso, 1)).astype(np.int16)


def orientar(ref, cual):
    if cual == "identidad":
        return ref
    if cual == "espejada-h":
        return ref[:, ::-1]
    if cual == "espejada-v":
        return ref[::-1]
    return ref[::-1, ::-1]


def comparar(a, b, tolerancia=4):
    """Porcentaje de pixeles iguales (+-tolerancia por canal) entre dos bloques del MISMO tamano."""
    dif = np.abs(a.astype(np.int16) - b.astype(np.int16)).max(axis=2)
    return 100.0 * float((dif <= tolerancia).sum()) / dif.size


def diferencia_con_vecindad(a, b, radio=2):
    """Por cada pixel de `a`, cuanto se aparta de LO MAS PARECIDO de `b` en su vecindad de radio `radio`.

    POR QUE LA VECINDAD Y NO EL PIXEL JUSTO: la captura viene del emulador reescalada (640x480 -> ~604x453)
    y despues re-muestreada por este medidor; el error de registro de uno o dos pixeles hace que TODOS los
    bordes del mapa difieran, y el "cuantos pixeles no son el mapa" contaba 22.000 en vez de ~600. Comparando
    contra la mejor coincidencia de la vecindad, un codo del mapa corrido 1 px sigue siendo el mismo codo, y
    lo que sobrevive es lo que NO esta en el mapa: el personaje.
    """
    mejor = None
    for dy in range(-radio, radio + 1):
        for dx in range(-radio, radio + 1):
            desplazada = np.roll(np.roll(b, dy, axis=0), dx, axis=1)
            dif = np.abs(a.astype(np.int16) - desplazada.astype(np.int16)).max(axis=2)
            mejor = dif if mejor is None else np.minimum(mejor, dif)
    return mejor


def bandas_negras(img):
    """Los extremos del bloque del mapa en la captura, por las bandas negras de los costados.

    El mapa deja 64 px logicos de negro a cada lado; ademas el contenido del mapa tiene pixeles negros, asi
    que NO se busca "todo el mapa es no negro": se busca, por columna, la fraccion de pixeles negros. Las
    columnas de la banda son casi todas negras (>90%); las del mapa, no (el mapa esta lleno de color).
    """
    negro = (img <= UMBRAL_NEGRO).all(axis=2)
    fraccion = negro.mean(axis=0)
    columnas = np.flatnonzero(fraccion > 0.9)
    if columnas.size == 0:
        return None
    return int(columnas.min()), int(columnas.max())


def main():
    args = sys.argv[1:]
    ventana = None
    ref_png = ref_mapa_png = None
    if "--ventana" in args:
        i = args.index("--ventana")
        ventana = [int(v) for v in args[i + 1].split(",")]
        del args[i:i + 2]
    if "--ref" in args:
        i = args.index("--ref")
        ref_png = args[i + 1]
        del args[i:i + 2]
    if "--ref-mapa" in args:
        i = args.index("--ref-mapa")
        ref_mapa_png = args[i + 1]
        del args[i:i + 2]
    captura = args[0]

    img = decodificar(captura)
    h, w, _ = img.shape
    print("MED238 captura=%s tamano=%dx%d" % (captura, w, h))

    # ---- 1. el area cliente del emulador
    if ventana:
        izq, arriba, ancho_cliente, alto = ventana
        fuente = "kwin"
    else:
        b = bandas_negras(img)
        if b is None:
            print("MED238 SIN-PANTALLA (no hay bandas negras: el emulador no esta en la captura)")
            return 0
        # las columnas del mapa (no negras) mas el margen del propio mapa: 64/512 de su ancho a cada lado
        ancho_mapa_px = b[1] - b[0] + 1
        margen = int(round(ancho_mapa_px * MAPA_X / float(MAPA_LADO)))
        izq, arriba = max(0, b[0] - margen), 0
        ancho_cliente, alto = img.shape[1], img.shape[0]
        fuente = "banda-negra"

    print("MED238 cliente=%d,%d,%d,%d fuente=%s" % (izq, arriba, ancho_cliente, alto, fuente))

    escala = alto / float(ALTO_LOGICO)
    ancho_pantalla = int(round(ANCHO_LOGICO * escala))
    pantalla_izq = izq + (ancho_cliente - ancho_pantalla) // 2
    print("MED238 pantalla=%d,%d,%d,%d escala=%.4f" %
          (pantalla_izq, arriba, ancho_pantalla, alto, escala))

    # ---- 2. el bloque del mapa: de la pantalla logica a la captura
    #     x: 64..576 de la pantalla logica; y: todo el alto (el mapa mide 512 de alto y se pinta en -16, asi
    #     que en pantalla quedan visibles sus filas 16..495)
    bx0 = pantalla_izq + int(round(MAPA_X * escala))
    by0 = arriba
    bancho = int(round(MAPA_LADO * escala))
    balto = alto
    if bx0 < 0 or by0 < 0 or bx0 + bancho > w or by0 + balto > h:
        print("MED238 BLOQUE-FUERA (%d,%d %dx%d no entra en %dx%d)" % (bx0, by0, bancho, balto, w, h))
        return 0
    print("MED238 bloque=%d,%d,%d,%d" % (bx0, by0, bancho, balto))

    if not ref_png:
        return 0

    # ---- 3. la referencia: recortada igual (64..576 de 640, todo el alto)
    ref = decodificar(ref_png)
    ref_bloque = ref[0:ALTO_LOGICO, MAPA_X:MAPA_X + MAPA_LADO]
    print("MED238 referencia=%s bloque=%dx%d" % (ref_png, ref_bloque.shape[1], ref_bloque.shape[0]))

    # ---- 4. identidad a 64x60 (el bloque es 512x480: 64x60 mantiene la proporcion)
    dx = int(round(LADO_VEREDICTO * bancho / float(balto)))
    cap_b = bajar(img, bx0, by0, bancho, balto, dx, LADO_VEREDICTO)
    ref_b = bajar(ref_bloque, 0, 0, ref_bloque.shape[1], ref_bloque.shape[0], dx, LADO_VEREDICTO)

    puntajes = {}
    for cual in ORIENTACIONES:
        puntajes[cual] = comparar(cap_b, orientar(ref_b, cual))
    print("MED238 identidad=" + " ".join("%s:%.2f%%" % (k, v) for k, v in puntajes.items()))
    print("MED238 identidad_amplia=" + " ".join(
        "%s:%.2f%%" % (cual, comparar(cap_b, orientar(ref_b, cual), 16)) for cual in ORIENTACIONES))

    # ---- 5. el personaje: lo que NO es el mapa solo
    if ref_mapa_png:
        mapa = decodificar(ref_mapa_png)
        mapa_bloque = mapa[0:ALTO_LOGICO, MAPA_X:MAPA_X + MAPA_LADO]

        # a la resolucion del bloque del mapa (512x480), que es la de la referencia
        cap_full = bajar(img, bx0, by0, bancho, balto, MAPA_LADO, ALTO_LOGICO)
        dif = diferencia_con_vecindad(cap_full, mapa_bloque)
        mascara = dif > UMBRAL_DIF
        n = int(mascara.sum())

        # el host: los mismos dos PNG, a la MISMA resolucion (el numero que se contrasta)
        dif_host = diferencia_con_vecindad(ref_bloque, mapa_bloque)
        mascara_host = dif_host > UMBRAL_DIF
        n_host = int(mascara_host.sum())

        linea = "MED238 personaje=px=%d px_host=%d" % (n, n_host)
        if n > 0:
            ys, xs = np.nonzero(mascara)
            linea += " centro=%.1f,%.1f caja=%d,%d,%d,%d" % (
                float(xs.mean()), float(ys.mean()), int(xs.min()), int(ys.min()),
                int(xs.max() - xs.min() + 1), int(ys.max() - ys.min() + 1))
        if n_host > 0:
            ys, xs = np.nonzero(mascara_host)
            linea += " centro_host=%.1f,%.1f" % (float(xs.mean()), float(ys.mean()))
        print(linea)

        if n < MIN_PIXELES_PERSONAJE:
            print("MED238 personaje=NO-SE-VE (%d px, el umbral es %d)" % (n, MIN_PIXELES_PERSONAJE))
        else:
            print("MED238 personaje=SE-VE")

    return 0


if __name__ == "__main__":
    sys.exit(main())
