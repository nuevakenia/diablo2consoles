#!/usr/bin/env python3
"""DC-002.37, criterio 2 -- MEDIR la captura del acto 1 en la Dreamcast: cuantos pixeles no son fondo,
que color domina y si la figura es la MISMA que la referencia del host.

LA GEOMETRIA (lo que costo esta tarjeta)
----------------------------------------
El POC pinta su quad en (0,0) de una pantalla logica de 640x480 y el lienzo (512x512) se escala por
esc = min(640/512, 480/512) = 0,9375: el asset ocupa EXACTAMENTE el alto de la pantalla logica, pegado
arriba a la izquierda. La pantalla logica, a su vez, el emulador la mete en el AREA CLIENTE de su ventana
manteniendo la proporcion (640x480 -> 604x453 en una ventana de 682x453, o sea 39 px de banda negra a
cada lado). Tres cosas medidas aca:

1. La geometria NO se adivina: se pasa con `--ventana L,T,W,H` (el AREA CLIENTE que reporta el gestor de
   ventanas; la mide `run-flycast-237.sh` con KWin y la imprime como `GEO ... cliente=...`).
2. `--ventana` se CONTRASTA con el quad de control magenta que el POC dibuja en coordenadas logicas
   conocidas (520,380 110x90): si el quad cae donde la caja dice (residuo <= 2 px) la caja es la buena,
   y si no, el instrumento lo avisa. El quad se dibuja sin textura: es independiente del asset.
3. Sin `--ventana` se cae a la deteccion por banda negra (la usa el MUTANTE, que no pinta quad de
   control). Esa deteccion se arreglo: la banda puede estar CORTADA por el propio quad de control (medido:
   el quad magenta parte la banda y la deteccion vieja leia alto=361 donde el cliente mide 453).

POR QUE EL VEREDICTO ES A 64x64 (y no a 512)
--------------------------------------------
La captura trae el lienzo reescalado a 453 px (y el escalado del emulador mezcla pixeles), asi que
comparar 1:1 contra la referencia de 512 es ruido: medido, con la caja EXACTA, a 512 la identidad da
44,7 % a +-8. A 128 da 93,5 % y a 64 da 98,8 %. Y a 128 una caja corrida 1 px cambia el numero entre
93,5 % y 97,1 % (medido), mientras que a 64 se queda entre 95,8 % y 99,8 %: por eso el "es el mismo mapa"
se mide a 64x64 y las otras tres orientaciones (espejada H, espejada V, rotada 180) quedan en 31-36 %.

Uso:
  medir-captura-237.py <captura.png> [referencia.png] [--ventana L,T,W,H] [--busca N]

Sale 0 siempre: los numeros son el resultado (el gate lo hace el corpus `checar-acto1-237.sh`).
"""
import subprocess
import sys

import numpy as np

UMBRAL_NEGRO = 12      # "negro puro": el fondo del PVR del POC es 0,0,0 exacto
MIN_CORRIDA = 60       # una banda de la pantalla mide cientos de px; 60 descarta negros de ventanas
REF_LADO = 512         # el lienzo del acto 1 (verif-acto1-237 -png) es de 512x512
ANCHO_LOGICO, ALTO_LOGICO = 640, 480
ORIENTACIONES = ("identidad", "espejada-h", "espejada-v", "rotada-180")
# el quad de control SIN textura que dibuja el POC, en coordenadas logicas (main.go: d2cQuadSolido)
QUAD_X, QUAD_Y, QUAD_W, QUAD_H = 520, 380, 110, 90
TOLERANCIA_QUAD = 2.0  # px: la caja del gestor de ventanas se acepta si el quad cae a esta distancia
LADO_VEREDICTO = 64    # la resolucion a la que se decide "es el mismo mapa" (ver arriba)


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
    """El area cliente del emulador segun la banda negra.

    OJO (medido): el quad de control magenta PARTE la banda en las filas donde cae (la banda de la derecha
    se queda en 49 px y no llega al minimo), asi que las filas del cliente NO son contiguas. La version
    vieja contaba filas y leia alto=361 donde el cliente mide 453; esta toma el EXTREMO (min..max) de las
    filas que comparten el borde derecho y el borde izquierdo MENOR (el borde del cliente)."""
    if not filas:
        return None
    cuenta = {}
    for y in filas:
        der = filas[y][1]
        cuenta[der] = cuenta.get(der, 0) + 1
    der, n = max(cuenta.items(), key=lambda kv: kv[1])
    ys = sorted(y for y in filas if filas[y][1] == der)
    izq = min(filas[y][0] for y in ys)
    span = ys[-1] - ys[0] + 1
    return {"arriba": ys[0], "abajo": ys[-1], "alto": span, "izq": izq, "der": der,
            "filas_vistas": n, "seguidas": n == span}


def bajar(img, x0, y0, lado, destino):
    """Box-downsample (promedio) del bloque (x0,y0,lado,lado) a destino x destino, con numpy."""
    bloque = img[y0:y0 + lado, x0:x0 + lado].astype(np.uint32)
    bordes = (np.arange(destino + 1) * lado) // destino
    filas = np.add.reduceat(bloque, bordes[:-1], axis=0)
    celdas = np.add.reduceat(filas, bordes[:-1], axis=1)
    cuenta = np.diff(bordes)
    peso = (cuenta[:, None] * cuenta[None, :])[:, :, None]
    return (celdas // peso).astype(np.int16)


def orientar(ref, cual):
    if cual == "identidad":
        return ref
    if cual == "espejada-h":
        return ref[:, ::-1]
    if cual == "espejada-v":
        return ref[::-1]
    return ref[::-1, ::-1]


def puntuar(img, ref, alto, x0, y0, destino=REF_LADO):
    h, w, _ = img.shape
    if x0 < 0 or y0 < 0 or x0 + alto > w or y0 + alto > h:
        return None
    b = bajar(img, x0, y0, alto, destino)
    r = bajar(ref, 0, 0, ref.shape[0], destino)
    dif = np.abs(b - r).max(axis=2)
    n = dif.size
    return {"mas4": int((dif <= 4).sum()), "mas8": int((dif <= 8).sum()), "mas16": int((dif <= 16).sum()),
            "n": n, "err": float(dif.mean())}


def linea_puntaje(titulo, p):
    return ("  %s: +-4 %6d (%.2f%%)   +-8 %6d (%.2f%%)   +-16 %6d (%.2f%%)   error medio %.2f" %
            (titulo, p["mas4"], 100.0 * p["mas4"] / p["n"], p["mas8"], 100.0 * p["mas8"] / p["n"],
             p["mas16"], 100.0 * p["mas16"] / p["n"], p["err"]))


def quad_medido(img):
    """El bbox del quad de control magenta (0xFFFF00FF) en la captura, o None."""
    r, g, b = (img[:, :, i].astype(np.int16) for i in range(3))
    mag = (r >= 200) & (g <= 60) & (b >= 200)
    if not mag.any():
        return None
    ys, xs = np.nonzero(mag)
    return {"x0": int(xs.min()), "x1": int(xs.max()), "y0": int(ys.min()), "y1": int(ys.max()),
            "px": int(mag.sum())}


def main():
    args = [a for a in sys.argv[1:]]
    ventana = None
    busca = 12
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
        print("  area cliente del emulador (--ventana, la de KWin): x=%d..%d y=%d..%d (%dx%d)" %
              (izq, izq + ancho_cliente - 1, arriba, arriba + alto - 1, ancho_cliente, alto))
        fuente = "gestor-de-ventanas"
    else:
        filas = banda_negra(img)
        det = pantalla_por_banda(filas)
        if det is None or det["alto"] < 100:
            print("MEDIDOR237 SIN-BANDA")
            print("  SIN BANDA NEGRA: en esta captura no hay pantalla de consola (ni asset ni fondo negro)")
            return 0
        arriba, alto, der = det["arriba"], det["alto"], det["der"]
        # El borde IZQUIERDO del cliente no siempre se ve (cuando el asset ya esta pintado, la banda negra
        # de la izquierda se une al negro del asset y la corrida mas larga arranca adentro del asset). Asi
        # que el ancho se toma de la VENTANA que usa este banco (682x453, la de emu.cfg) y el borde
        # izquierdo sale del derecho: es una SUPOSICION, y por eso el corpus mide la geometria con KWin y
        # exige el quad de control (control 2j) en vez de confiar en esta via.
        ancho_cliente = int(round(alto * 682.0 / 453.0))
        izq = der - ancho_cliente + 1
        print("  area cliente DETECTADA por la banda negra: y=%d..%d alto=%d x=%d..%d ancho=%d "
              "(filas con banda=%d, contiguas=%s, ancho de la ventana 682x453 SUPUESTO)" %
              (arriba, arriba + alto - 1, alto, izq, der, ancho_cliente,
               det["filas_vistas"], det["seguidas"]))
        fuente = "banda-negra"

    print("MEDIDOR237 fuente=%s" % fuente)
    print("MEDIDOR237 ventana=%d,%d,%d,%d" % (izq, arriba, ancho_cliente, alto))

    escala = alto / float(ALTO_LOGICO)
    ancho_pantalla = int(round(ANCHO_LOGICO * escala))
    pantalla_izq = izq + (ancho_cliente - ancho_pantalla) // 2
    print("  pantalla de la consola (640x480 escalada x%.4f, centrada en el cliente): x=%d..%d y=%d..%d "
          "(%dx%d)" % (escala, pantalla_izq, pantalla_izq + ancho_pantalla - 1, arriba, arriba + alto - 1,
                       ancho_pantalla, alto))

    # el asset mide 512 * esc en la pantalla logica, y esc = min(640/512, 480/512) = 480/512: el asset
    # ocupa EXACTAMENTE el alto del area cliente, pegado arriba-izquierda de la pantalla logica.
    lado_asset = int(round(min(ANCHO_LOGICO, ALTO_LOGICO) * escala))
    print("  bloque del asset (%dx%d escalado = el alto entero): x=%d..%d (%dx%d)" %
          (REF_LADO, REF_LADO, pantalla_izq, pantalla_izq + lado_asset - 1, lado_asset, lado_asset))

    # --- el quad de control: la caja del gestor de ventanas se contrasta con el asset que el POC dibuja en
    # coordenadas logicas conocidas. Si el quad cae donde la caja dice, la caja es la buena.
    q = quad_medido(img)
    pred_x0 = pantalla_izq + QUAD_X * escala
    pred_y0 = arriba + QUAD_Y * escala
    if q is None:
        print("MEDIDOR237 quad=sin (no hay quad magenta: es la pantalla del MUTANTE o no llego a pintar)")
    else:
        dx = q["x0"] - pred_x0
        dy = q["y0"] - pred_y0
        dw = (q["x1"] - q["x0"] + 1) - QUAD_W * escala
        dh = (q["y1"] - q["y0"] + 1) - QUAD_H * escala
        estado = "ok" if max(abs(dx), abs(dy), abs(dw), abs(dh)) <= TOLERANCIA_QUAD else "corrido"
        print("  quad de control magenta (%d,%d %dx%d logicos): medido x=%d..%d y=%d..%d (%dx%d), "
              "predicho x=%.1f y=%.1f (%.1fx%.1f)" %
              (QUAD_X, QUAD_Y, QUAD_W, QUAD_H, q["x0"], q["x1"], q["y0"], q["y1"],
               q["x1"] - q["x0"] + 1, q["y1"] - q["y0"] + 1, pred_x0, pred_y0,
               QUAD_W * escala, QUAD_H * escala))
        print("MEDIDOR237 quad=%s dx=%+.1f dy=%+.1f dw=%+.1f dh=%+.1f px=%d" %
              (estado, dx, dy, dw, dh, q["px"]))

    bloque = img[arriba:arriba + lado_asset, pantalla_izq:pantalla_izq + lado_asset]
    no_negro = ~(bloque <= UMBRAL_NEGRO).all(axis=2)
    total = lado_asset * lado_asset
    n_no_negro = int(no_negro.sum())
    print("  pixeles del bloque que NO son fondo negro: %d de %d (%.2f%%)" %
          (n_no_negro, total, 100.0 * n_no_negro / total))
    print("MEDIDOR237 bloque %dx%d noFondo=%d de %d (%.2f%%)" %
          (lado_asset, lado_asset, n_no_negro, total, 100.0 * n_no_negro / total))

    if no_negro.any():
        cuadros = bloque[no_negro] >> 4
        claves, cuenta = np.unique(cuadros, axis=0, return_counts=True)
        orden = np.argsort(-cuenta)[:5]
        for k in orden:
            c = claves[k]
            print("     color dominante (rango 16): %2d,%2d,%2d  pixeles=%d" % (c[0], c[1], c[2], cuenta[k]))
        c = claves[orden[0]]
        print("MEDIDOR237 dominante=%d,%d,%d pixeles=%d" % (c[0], c[1], c[2], cuenta[orden[0]]))
        media = bloque[no_negro].astype(np.float64).mean(axis=0)
        print("  color medio del bloque (sin fondo): R=%.1f G=%.1f B=%.1f max=%d" %
              (media[0], media[1], media[2], int(bloque.max())))

    # la pantalla entera: cuanto de negro puro (para el caso del mutante, que no pinta nada)
    pantalla = img[arriba:arriba + alto, pantalla_izq:pantalla_izq + ancho_pantalla]
    negros = int((pantalla <= UMBRAL_NEGRO).all(axis=2).sum())
    tot_pant = pantalla.shape[0] * pantalla.shape[1]
    print("  pantalla completa: %d de %d pixeles en negro puro (%.2f%%)" %
          (negros, tot_pant, 100.0 * negros / tot_pant))
    print("MEDIDOR237 pantalla negros=%d de %d (%.2f%%)" % (negros, tot_pant, 100.0 * negros / tot_pant))

    if not ref_png:
        return 0

    ref = decodificar(ref_png)
    print("  REFERENCIA %s (%dx%d)" % (ref_png, ref.shape[1], ref.shape[0]))

    # Cuanto del LIENZO de referencia no es negro: es la expectativa MEDIDA de pixeles pintados. OJO: la
    # referencia pinta el indice 0 con su color de paleta y en la consola el indice 0 es TRANSPARENTE, asi
    # que los dos numeros no tienen por que ser iguales: la diferencia medida con la caja buena es de 2,6
    # puntos (89,0 % de la consola contra 86,4 % de la referencia) y el corpus la tolera con margen.
    ref_no_negro = int((~(ref <= UMBRAL_NEGRO).all(axis=2)).sum())
    ref_tot = ref.shape[0] * ref.shape[1]
    print("  la referencia tiene %d de %d pixeles que no son negro (%.2f%%)" %
          (ref_no_negro, ref_tot, 100.0 * ref_no_negro / ref_tot))
    print("MEDIDOR237 refNoFondo=%.2f" % (100.0 * ref_no_negro / ref_tot))

    # --- el veredicto SIN BUSCAR: la caja es la del gestor de ventanas (medida aparte), el bloque se toma
    # tal cual y se compara a 64x64 en las cuatro orientaciones.
    print("  == sin buscar alineacion (la caja es la medida, no se ajusta)")
    veredicto = {}
    for cual in ORIENTACIONES:
        p = puntuar(img, orientar(ref, cual), lado_asset, pantalla_izq, arriba, destino=LADO_VEREDICTO)
        if p is None:
            continue
        veredicto[cual] = 100.0 * p["mas8"] / p["n"]
        print(linea_puntaje("%-11s a %dx%d" % (cual, LADO_VEREDICTO, LADO_VEREDICTO), p))
    for cual, pct in veredicto.items():
        if cual == "identidad":
            print("MEDIDOR237 identidad64 mas8=%.2f" % pct)
        else:
            print("MEDIDOR237 orient64 %s mas8=%.2f" % (cual, pct))
    p = puntuar(img, ref, lado_asset, pantalla_izq, arriba, destino=128)
    if p is not None:
        print(linea_puntaje("identidad a 128x128", p))
        print("MEDIDOR237 grueso mas8=%.2f mas16=%.2f error=%.2f" %
              (100.0 * p["mas8"] / p["n"], 100.0 * p["mas16"] / p["n"], p["err"]))
    p = puntuar(img, ref, lado_asset, pantalla_izq, arriba, destino=REF_LADO)
    if p is not None:
        print(linea_puntaje("identidad a 512x512 (referencia, con el ruido del reescalado)", p))
        print("MEDIDOR237 fino mas8=%.2f error=%.2f" % (100.0 * p["mas8"] / p["n"], p["err"]))

    # --- ademas, la mejor alineacion buscando +-busca px a 128x128 (informativo: dice si la caja esta
    # corrida; NO es el numero del veredicto)
    mejor = None
    for paso in (4, 1):
        rango = busca if mejor is None else paso * 2
        base_dx = 0 if mejor is None else mejor[0]
        base_dy = 0 if mejor is None else mejor[1]
        for dx in range(base_dx - rango, base_dx + rango + 1, paso):
            for dy in range(base_dy - rango, base_dy + rango + 1, paso):
                q2 = puntuar(img, ref, lado_asset, pantalla_izq + dx, arriba + dy, destino=128)
                if q2 is None:
                    continue
                clave = (q2["mas8"], q2["mas4"], -q2["err"])
                if mejor is None or clave > (mejor[2]["mas8"], mejor[2]["mas4"], -mejor[2]["err"]):
                    mejor = (dx, dy, q2)
    if mejor is not None:
        print("  mejor alineacion buscando +-%d px (a 128x128): dx=%+d dy=%+d -> %.2f%% a +-8 (la caja "
              "medida da %.2f%% sin buscar)" %
              (busca, mejor[0], mejor[1], 100.0 * mejor[2]["mas8"] / mejor[2]["n"],
               veredicto.get("identidad", float("nan"))))

    ident = veredicto.get("identidad", 0.0)
    otras = [v for k, v in veredicto.items() if k != "identidad"]
    ganadora = max(veredicto.items(), key=lambda kv: kv[1])[0] if veredicto else "ninguna"
    print("MEDIDOR237 MEJOR=%s mas8=%.2f" %
          (ganadora, max(veredicto.values()) if veredicto else 0.0))
    print("MEDIDOR237 orientaciones identidad=%.2f otras=%s" %
          (ident, ",".join("%.2f" % v for v in otras)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
