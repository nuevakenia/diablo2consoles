#!/usr/bin/env python3
"""DC-002.36 -- MEDIR la pantalla del MENU PRINCIPAL de la Dreamcast con numeros (y la navegacion).

Nada de "se ve el menu": este instrumento da los pixeles.

POR QUE ASI (cada punto salio de medir, no de suponer)
------------------------------------------------------
1. `spectacle -f` (KDE/Wayland) fotografia la PANTALLA COMPLETA (en este host 3584x1080, dos monitores):
   la ventana del emulador hay que UBICARLA adentro. Aca no sirve el truco de DC-002.35 (ubicar la pantalla
   por la banda de negro puro al lado del quad): el menu pinta los 640x480 ENTEROS, no deja banda negra.
   Lo que si se sabe es la geometria de la VENTANA: `run-flycast-236.sh` la pide y la VERIFICA contra KWin
   (linea `ventana: Flycast - dc236|flycast|...|geo=L,T,W,H`, que es el marco: el area cliente empieza en
   T+28 por la barra de titulo).
2. La pantalla de la consola (640x480) va CENTRADA en el area cliente y escalada por el alto (Flycast
   respeta la proporcion): escala = alto_cliente/480, ancho = 640*escala, y el resto son barras negras a
   los costados. Medido con la ventana por defecto de este host (cliente 682x453): escala 0.94375, pantalla
   604x453 en x=1439.
3. La geometria NO se cree de palabra: se contrasta con la referencia. Si el rectangulo estuviera mal, la
   coincidencia se cae -- y el instrumento busca ademas el mejor desplazamiento (+-4 px), asi que un error
   de 1 o 2 px no se confunde con una pantalla mal pintada.
4. La referencia del host (`referencia-foco<N>-640.png`, 640x480) la armó el compositor con el CODIGO DEL
   MOTOR y el mismo PVR simulado: nearest, indice 0 transparente. La captura se baja a 640x480 con nearest
   por lo mismo (interpolar inventaria colores que el PVR no puede dar).

Uso:
  medir-menu-236.py pinta  <captura.png> [--referencia REF.png] [--ventana L,T,W,H] [--busca N] [--titulo T]
  medir-menu-236.py navega <a.png> <b.png> [--referencia-a REF.png] [--referencia-b REF.png]
                           [--ventana L,T,W,H] [--umbral D]

Sale 0 siempre (los numeros son el resultado; el gate lo hace el corpus).
"""
import subprocess
import sys

import numpy as np

ANCHO_CONSOLA, ALTO_CONSOLA = 640, 480
BARRA_TITULO = 28          # medido: `-config window:left/top` deja el cliente 28 px abajo del marco
UMBRAL_DIF = 16            # diferencia por canal para contar un pixel como "de otro color"
UMBRAL_CAMBIO = 12         # diferencia por canal para contar un pixel como "que cambio"


def decodificar(png):
    info = subprocess.run(["ffprobe", "-v", "error", "-select_streams", "v", "-show_entries",
                           "stream=width,height", "-of", "csv=p=0", png],
                          capture_output=True, check=True, text=True).stdout.strip().split(",")
    w, h = int(info[0]), int(info[1])
    raw = subprocess.run(["ffmpeg", "-v", "error", "-i", png, "-f", "rawvideo", "-pix_fmt", "rgb24", "-"],
                         capture_output=True, check=True).stdout
    return np.frombuffer(raw, dtype=np.uint8).reshape(h, w, 3)


def rect_pantalla(img, ventana):
    """El rectangulo de la PANTALLA de la consola dentro de la captura, a partir del marco de la ventana.

    Devuelve (x0, y0, ancho, alto) en pixeles de la captura, o None si la ventana no esta adentro (o si el
    marco que le pasaron no da una pantalla creible).
    """
    if ventana is None:
        return None
    marco_x, marco_y, marco_w, marco_h = ventana
    x0c, y0c = marco_x, marco_y + BARRA_TITULO
    cliente_w, cliente_h = marco_w, marco_h - BARRA_TITULO   # la barra de titulo NO es area cliente

    alto = cliente_h
    ancho = int(round(ANCHO_CONSOLA * (cliente_h / float(ALTO_CONSOLA))))
    x0 = x0c + (cliente_w - ancho) // 2

    h, w, _ = img.shape
    if ancho < 100 or alto < 100 or x0 < 0 or y0c < 0 or x0 + ancho > w or y0c + alto > h:
        return None

    return (x0, y0c, ancho, alto)


def a_consola(bloque, ancho=ANCHO_CONSOLA, alto=ALTO_CONSOLA):
    """Baja el bloque de la captura al tamano de la pantalla de la consola con NEAREST (sin inventar color)."""
    h, w, _ = bloque.shape
    ys = (np.arange(alto) * h) // alto
    xs = (np.arange(ancho) * w) // ancho
    return bloque[ys][:, xs]


def fondo_de(ref):
    """El color de fondo: el mas comun de la referencia (cuantizado a 16 niveles, como los dominantes)."""
    claves, cuenta = np.unique((ref >> 4).reshape(-1, 3), axis=0, return_counts=True)
    k = claves[int(np.argmax(cuenta))]
    return (k.astype(np.int32) << 4) + 8


def dominantes(img, n=5):
    claves, cuenta = np.unique((img >> 4).reshape(-1, 3), axis=0, return_counts=True)
    orden = np.argsort(-cuenta)[:n]
    return [((claves[k].astype(int) << 4), int(cuenta[k])) for k in orden]


def caja_diferencia(a, b, umbral=UMBRAL_CAMBIO):
    dif = np.abs(a.astype(np.int32) - b.astype(np.int32)).max(axis=2)
    mascara = dif > umbral
    n = int(mascara.sum())
    if n == 0:
        return 0, None
    ys, xs = np.nonzero(mascara)
    return n, (int(xs.min()), int(ys.min()), int(xs.max()), int(ys.max()))


def puntuar(a, b, dx=0, dy=0):
    """Compara `a` (pantalla de la captura, 640x480) contra `b` (la referencia) desplazada."""
    h, w, _ = a.shape
    x0, y0 = max(0, dx), max(0, dy)
    x1, y1 = min(w, w + dx), min(h, h + dy)
    if x1 - x0 < 100 or y1 - y0 < 100:
        return None
    pa = a[y0:y1, x0:x1].astype(np.int32)
    pb = b[y0 - dy:y1 - dy, x0 - dx:x1 - dx].astype(np.int32)
    dif = np.abs(pa - pb).max(axis=2)
    n = dif.size
    return {"mas4": int((dif <= 4).sum()), "mas8": int((dif <= 8).sum()), "mas16": int((dif <= 16).sum()),
            "n": n, "err": float(dif.mean())}


def linea(titulo, p):
    return ("  %s: +-4 %7d (%.2f%%)   +-8 %7d (%.2f%%)   +-16 %7d (%.2f%%)   error medio %.2f" %
            (titulo, p["mas4"], 100.0 * p["mas4"] / p["n"], p["mas8"], 100.0 * p["mas8"] / p["n"],
             p["mas16"], 100.0 * p["mas16"] / p["n"], p["err"]))


def mejor_alineacion(a, b, busca):
    mejor = None
    for paso in (4, 1):
        rango = busca if mejor is None else paso * 2
        bdx = 0 if mejor is None else mejor[0]
        bdy = 0 if mejor is None else mejor[1]
        for dx in range(bdx - rango, bdx + rango + 1, paso):
            for dy in range(bdy - rango, bdy + rango + 1, paso):
                p = puntuar(a, b, dx, dy)
                if p is None:
                    continue
                if mejor is None or (p["mas8"], p["mas4"], -p["err"]) > (mejor[2]["mas8"], mejor[2]["mas4"],
                                                                        -mejor[2]["err"]):
                    mejor = (dx, dy, p)
    return mejor


def args_comunes(argv):
    opciones = {"referencia": None, "referencia-a": None, "referencia-b": None, "ventana": None,
                "busca": 4, "umbral": UMBRAL_CAMBIO, "titulo": ""}
    libres = []
    i = 0
    while i < len(argv):
        a = argv[i]
        if a.startswith("--"):
            clave = a[2:]
            if clave == "busca" or clave == "umbral":
                opciones[clave] = int(argv[i + 1])
            elif clave == "ventana":
                opciones[clave] = [int(v) for v in argv[i + 1].split(",")]
            else:
                opciones[clave] = argv[i + 1]
            i += 2
            continue
        libres.append(a)
        i += 1
    return libres, opciones


def pantalla_de(png, ventana):
    img = decodificar(png)
    rect = rect_pantalla(img, ventana)
    if rect is None:
        print("  SIN PANTALLA: no pude ubicar la ventana del emulador en %s %dx%d (ventana=%s)" %
              (png, img.shape[1], img.shape[0], ventana))
        return None, None
    x0, y0, ancho, alto = rect
    bloque = img[y0:y0 + alto, x0:x0 + ancho]
    return a_consola(bloque), rect


def pinta(png, op):
    print("CAPTURA %s" % png)
    pantalla, rect = pantalla_de(png, op["ventana"])
    if pantalla is None:
        return 0
    x0, y0, ancho, alto = rect
    print("  ventana (marco de KWin): %s  -> area cliente en y=%d, alto cliente=%d" %
          (op["ventana"], op["ventana"][1] + BARRA_TITULO, op["ventana"][3]))
    print("  pantalla de la consola dentro de la captura: x=%d..%d y=%d..%d (%dx%d, escala %.4f)" %
          (x0, x0 + ancho - 1, y0, y0 + alto - 1, ancho, alto, alto / float(ALTO_CONSOLA)))

    ref = None
    if op["referencia"]:
        ref = decodificar(op["referencia"])
        print("  REFERENCIA %s (%dx%d)" % (op["referencia"], ref.shape[1], ref.shape[0]))

    if ref is not None:
        fondo = fondo_de(ref)
        no_fondo_ref = (np.abs(ref.astype(np.int32) - fondo).max(axis=2) > UMBRAL_DIF).sum()
        print("  (referencia) color de fondo %d,%d,%d -> pixeles que NO son fondo: %d de %d (%.2f%%)" %
              (fondo[0], fondo[1], fondo[2], no_fondo_ref, ref.shape[0] * ref.shape[1],
               100.0 * no_fondo_ref / ref.shape[0] / ref.shape[1]))
    else:
        fondo = fondo_de(pantalla)

    no_fondo = (np.abs(pantalla.astype(np.int32) - fondo).max(axis=2) > UMBRAL_DIF)
    total = pantalla.shape[0] * pantalla.shape[1]
    print("  CAPTURA: pixeles que NO son el fondo %d,%d,%d: %d de %d (%.2f%%)" %
          (fondo[0], fondo[1], fondo[2], int(no_fondo.sum()), total, 100.0 * no_fondo.sum() / total))
    print("  colores dominantes de la pantalla (cuantizados a 16, con su cuenta):")
    for c, cuenta in dominantes(pantalla):
        print("     %3d,%3d,%3d  pixeles=%d (%.2f%%)" % (c[0], c[1], c[2], cuenta, 100.0 * cuenta / total))

    if ref is None:
        return 0

    base = puntuar(pantalla, ref)
    if base is not None:
        print(linea("pantalla tal cual (dx=0 dy=0)", base))
    mejor = mejor_alineacion(pantalla, ref, op["busca"])
    if mejor is not None:
        dx, dy, p = mejor
        print("  MEJOR ALINEACION (buscando +-%d px): dx=%+d dy=%+d" % (op["busca"], dx, dy))
        print(linea("contra la referencia del host", p))
    return 0


def navega(a, b, op):
    print("NAVEGACION entre %s y %s" % (a, b))
    pa, recta = pantalla_de(a, op["ventana"])
    pb, rectb = pantalla_de(b, op["ventana"])
    if pa is None or pb is None:
        return 0
    print("  la misma pantalla de la consola en las dos capturas: %s y %s" % (recta, rectb))

    n, caja = caja_diferencia(pa, pb, op["umbral"])
    total = pa.shape[0] * pa.shape[1]
    print("  pixeles que CAMBIAN entre las dos capturas (diferencia > %d por canal): %d de %d (%.2f%%)" %
          (op["umbral"], n, total, 100.0 * n / total))
    if caja:
        print("  la caja donde cambian: x=%d..%d y=%d..%d (%dx%d)" %
              (caja[0], caja[2], caja[1], caja[3], caja[2] - caja[0] + 1, caja[3] - caja[1] + 1))
        print("  esa caja en el lienzo LOGICO del motor (x5/4, 800x600): x=%d..%d y=%d..%d" %
              (caja[0] * 5 // 4, caja[2] * 5 // 4, caja[1] * 5 // 4, caja[3] * 5 // 4))
    else:
        print("  la caja donde cambian: ninguna (las dos capturas son iguales)")

    for etiqueta, pantalla, ref_png in (("A", pa, op["referencia-a"]), ("B", pb, op["referencia-b"])):
        if not ref_png:
            continue
        ref = decodificar(ref_png)
        mejor = mejor_alineacion(pantalla, ref, op["busca"])
        if mejor is not None:
            dx, dy, p = mejor
            print("  captura %s contra %s: dx=%+d dy=%+d" % (etiqueta, ref_png, dx, dy))
            print(linea("  captura %s" % etiqueta, p))
    return 0


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        return 0
    modo = sys.argv[1]
    libres, op = args_comunes(sys.argv[2:])
    if modo == "pinta" and libres:
        return pinta(libres[0], op)
    if modo == "navega" and len(libres) >= 2:
        return navega(libres[0], libres[1], op)
    print(__doc__)
    return 0


if __name__ == "__main__":
    sys.exit(main())
