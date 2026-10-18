#!/usr/bin/env python3
"""DC-002.35, criterio 2 -- MEDIR la captura del emulador con numeros: cuantos pixeles del asset hay,
que color domina, y cuanto coincide con la referencia del host. Nada de "parece que".

POR QUE ASI (todo esto salio de medir, no de suponer)
-----------------------------------------------------
1. `spectacle -f` (KDE/Wayland) fotografia la PANTALLA COMPLETA: la captura trae el escritorio. Hay que
   UBICAR la ventana del emulador adentro. El primer intento de esta tarjeta recorto a ojo
   (210,210 640x480) y midio el escritorio: dio "color dominante 1,1,1" y un 14,91% de pixeles no negros
   que no significaban nada.
2. La ventana del emulador NO tiene una geometria fija: Flycast guarda la ultima en
   ~/.config/flycast/emu.cfg ([window] left/top/width/height) y KWin la puede mover. Por eso la geometria
   NO se supone: se pasa con --ventana (leida del emu.cfg) o se DETECTA por contenido.
3. El POC pinta su quad de 480x480 ARRIBA A LA IZQUIERDA de su pantalla de 640x480 (`d2cQuad(0,0,esc)`) y
   el resto lo deja en NEGRO PURO (`PvrSetBgColor 0,0,0`). Flycast dibuja esa pantalla CENTRADA en su
   area cliente, con barras negras a los costados. Como el asset ocupa EXACTAMENTE el alto de la pantalla
   (esc = min(640/256, 480/256) = 480/256), su lado en la captura es el alto del area cliente:
       pantalla = alto*640/480 de ancho, y el asset = alto x alto pegado arriba a la izquierda.
4. El ultimo paso no es a ojo: el bloque del asset se baja a 256x256 (el tamano de la referencia del
   host), se compara pixel a pixel y se busca la alineacion que mejor puntua. Si el modelo estuviera mal,
   el puntaje se cae y se ve en los numeros.

Uso:
  medir-captura-235.py <captura.png> [referencia.png] [--ventana L,T,W,H] [--busca N]

Sale 0 siempre: los numeros son el resultado (el gate lo hace el corpus).
"""
import subprocess
import sys

import numpy as np

UMBRAL_NEGRO = 12      # "negro puro": el fondo del PVR del POC es 0,0,0 exacto
MIN_CORRIDA = 60       # una banda de la pantalla mide cientos de px; 60 descarta negros de ventanas
REF_LADO = 256         # la referencia del host (console/dc/verif-dc6-235) es de 256x256
ANCHO_LOGICO, ALTO_LOGICO = 640, 480


def decodificar(png):
    info = subprocess.run(["ffprobe", "-v", "error", "-select_streams", "v", "-show_entries",
                           "stream=width,height", "-of", "csv=p=0", png],
                          capture_output=True, check=True, text=True).stdout.strip().split(",")
    w, h = int(info[0]), int(info[1])
    raw = subprocess.run(["ffmpeg", "-v", "error", "-i", png, "-f", "rawvideo", "-pix_fmt", "rgb24", "-"],
                         capture_output=True, check=True).stdout
    return np.frombuffer(raw, dtype=np.uint8).reshape(h, w, 3)


def banda_negra(img):
    """La corrida de negro puro mas larga de cada fila: (izq, der, largo) por fila."""
    h, w, _ = img.shape
    negro = (img <= UMBRAL_NEGRO).all(axis=2)
    salida = {}
    for y in range(h):
        fila = negro[y]
        if not fila.any():
            continue
        d = np.diff(np.concatenate(([0], fila.astype(np.int8), [0])))
        arranques = np.flatnonzero(d == 1)
        finales = np.flatnonzero(d == -1) - 1
        largos = finales - arranques + 1
        k = int(np.argmax(largos))
        if largos[k] >= MIN_CORRIDA:
            salida[y] = (int(arranques[k]), int(finales[k]), int(largos[k]))
    return salida


def pantalla_por_banda(filas):
    """Las filas de la pantalla del emulador: las que comparten el borde DERECHO de la banda.

    Medido, en dos intentos fallidos:
    - minimo/maximo de filas se estira cuando otra ventana oscura toca el borde (dio alto=571 en vez de 453);
    - agrupar por (izquierda, derecha) tampoco sirve: donde el ASSET es oscuro su negro se une al fondo negro
      de la pantalla y la corrida arranca mas a la izquierda (dio 135 filas en vez de 453 y el bloque salio
      de 135x135).
    El borde DERECHO si es constante: el POC pinta negro puro desde el borde del asset hasta el borde del
    area cliente, y ahi termina la ventana. Asi que la pantalla = las filas que comparten ese borde derecho.
    """
    if not filas:
        return None
    cuenta = {}
    for y in filas:
        der = filas[y][1]
        cuenta[der] = cuenta.get(der, 0) + 1
    der, alto = max(cuenta.items(), key=lambda kv: kv[1])
    ys = sorted(y for y in filas if filas[y][1] == der)
    izq = int(np.median([filas[y][0] for y in ys]))
    return {"arriba": ys[0], "abajo": ys[-1], "alto": alto, "izq": izq, "der": der,
            "seguidas": (ys[-1] - ys[0] + 1) == alto}


def bajar(img, x0, y0, lado, destino):
    """Box-downsample (promedio) del bloque (x0,y0,lado,lado) a destino x destino, con numpy."""
    bloque = img[y0:y0 + lado, x0:x0 + lado].astype(np.uint32)
    bordes = (np.arange(destino + 1) * lado) // destino
    filas = np.add.reduceat(bloque, bordes[:-1], axis=0)
    celdas = np.add.reduceat(filas, bordes[:-1], axis=1)
    cuenta = np.diff(bordes)
    peso = (cuenta[:, None] * cuenta[None, :])[:, :, None]
    return (celdas // peso).astype(np.int16)


def puntuar(img, ref, alto, x0, y0):
    h, w, _ = img.shape
    if x0 < 0 or y0 < 0 or x0 + alto > w or y0 + alto > h:
        return None
    b = bajar(img, x0, y0, alto, REF_LADO)
    dif = np.abs(b - ref).max(axis=2)
    n = dif.size
    return {"mas4": int((dif <= 4).sum()), "mas8": int((dif <= 8).sum()), "mas16": int((dif <= 16).sum()),
            "n": n, "err": float(dif.mean())}


def linea_puntaje(titulo, p):
    return ("  %s: +-4 %6d (%.2f%%)   +-8 %6d (%.2f%%)   +-16 %6d (%.2f%%)   error medio %.2f" %
            (titulo, p["mas4"], 100.0 * p["mas4"] / p["n"], p["mas8"], 100.0 * p["mas8"] / p["n"],
             p["mas16"], 100.0 * p["mas16"] / p["n"], p["err"]))


def main():
    args = [a for a in sys.argv[1:]]
    ventana = None
    busca = 40
    if "--ventana" in args:
        i = args.index("--ventana")
        ventana = [int(v) for v in args[i + 1].split(",")]
        del args[i:i + 2]
    if "--busca" in args:
        i = args.index("--busca")
        busca = int(args[i + 1])
        del args[i:i + 2]
    captura = args[0]
    ref_png = args[1] if len(args) > 1 else None

    img = decodificar(captura)
    h, w, _ = img.shape
    print("CAPTURA %s  %dx%d" % (captura, w, h))

    if ventana:
        izq, arriba, ancho_cliente, alto = ventana
        print("  area cliente del emulador (de emu.cfg [window]): x=%d..%d y=%d..%d (%dx%d)" %
              (izq, izq + ancho_cliente - 1, arriba, arriba + alto - 1, ancho_cliente, alto))
    else:
        filas = banda_negra(img)
        det = pantalla_por_banda(filas)
        if det is None or det["alto"] < 100:
            print("  SIN BANDA NEGRA: en esta captura no hay pantalla de consola (ni asset ni fondo negro)")
            return 0
        arriba, abajo, alto = det["arriba"], det["abajo"], det["alto"]
        der = det["der"]
        # Flycast deja el area cliente en la misma proporcion que la ultima corrida (682x453 con la
        # pantalla de 640x480): de ahi sale el ancho del cliente y, centrada, la pantalla.
        ancho_cliente = int(round(alto * 682.0 / 453.0))
        izq = der - ancho_cliente + 1
        print("  area cliente DETECTADA por la banda negra: y=%d..%d alto=%d banda x=%d..%d "
              "(la misma banda en %d filas, seguidas=%s)" %
              (arriba, abajo, alto, det["izq"], der, det["alto"], det["seguidas"]))

    escala = alto / float(ALTO_LOGICO)
    ancho_pantalla = int(round(ANCHO_LOGICO * escala))
    pantalla_izq = izq + (ancho_cliente - ancho_pantalla) // 2
    print("  pantalla de la consola (640x480 escalada x%.4f): x=%d..%d y=%d..%d (%dx%d)" %
          (escala, pantalla_izq, pantalla_izq + ancho_pantalla - 1, arriba, arriba + alto - 1,
           ancho_pantalla, alto))
    print("  bloque del asset (480x480 escalado = el alto entero, pegado arriba-izquierda): x=%d..%d (%dx%d)" %
          (pantalla_izq, pantalla_izq + alto - 1, alto, alto))

    bloque = img[arriba:arriba + alto, pantalla_izq:pantalla_izq + alto]
    no_negro = ~(bloque <= UMBRAL_NEGRO).all(axis=2)
    total = alto * alto
    print("  pixeles del bloque que NO son fondo negro: %d de %d (%.2f%%)" %
          (int(no_negro.sum()), total, 100.0 * no_negro.sum() / total))
    if no_negro.any():
        cuadros = bloque[no_negro] >> 4
        claves, cuenta = np.unique(cuadros, axis=0, return_counts=True)
        orden = np.argsort(-cuenta)[:5]
        for k in orden:
            c = claves[k]
            print("     color dominante (rango 16): %2d,%2d,%2d  pixeles=%d" % (c[0], c[1], c[2], cuenta[k]))

    # la pantalla entera: cuanto de negro puro (para el caso del mutante, que no pinta nada)
    pantalla = img[arriba:arriba + alto, pantalla_izq:pantalla_izq + ancho_pantalla]
    negros = int((pantalla <= UMBRAL_NEGRO).all(axis=2).sum())
    print("  pantalla completa: %d de %d pixeles en negro puro (%.2f%%)" %
          (negros, pantalla.shape[0] * pantalla.shape[1],
           100.0 * negros / (pantalla.shape[0] * pantalla.shape[1])))

    if not ref_png:
        return 0

    ref = decodificar(ref_png)
    print("  REFERENCIA %s (%dx%d)" % (ref_png, ref.shape[1], ref.shape[0]))

    modelo = puntuar(img, ref, alto, pantalla_izq, arriba)
    if modelo is not None:
        print(linea_puntaje("alineacion del MODELO (x=%d y=%d)" % (pantalla_izq, arriba), modelo))

    mejor = None
    for paso in (4, 1):
        rango = busca if mejor is None else paso * 2
        base_dx = 0 if mejor is None else mejor[0]
        base_dy = 0 if mejor is None else mejor[1]
        candidatos = [(dx, dy)
                      for dx in range(base_dx - rango, base_dx + rango + 1, paso)
                      for dy in range(base_dy - rango, base_dy + rango + 1, paso)]
        for dx, dy in candidatos:
            p = puntuar(img, ref, alto, pantalla_izq + dx, arriba + dy)
            if p is None:
                continue
            clave = (p["mas8"], p["mas4"], -p["err"])
            if mejor is None or clave > (mejor[2]["mas8"], mejor[2]["mas4"], -mejor[2]["err"]):
                mejor = (dx, dy, p)

    if mejor is not None:
        dx, dy, p = mejor
        print("  MEJOR ALINEACION (buscando +-%d px): dx=%+d dy=%+d -> x=%d y=%d" %
              (busca, dx, dy, pantalla_izq + dx, arriba + dy))
        print(linea_puntaje("contra la referencia del host", p))

    return 0


if __name__ == "__main__":
    sys.exit(main())
