#!/usr/bin/env python3
"""DC-002.37, criterio 2 -- MEDIR la captura del acto 1 en la Dreamcast: cuantos pixeles pintados hay,
que color domina y cuanto coincide con la REFERENCIA del host (buscando la orientacion que gana).

POR QUE ASI (lo que costo cada regla, medido en DC-002.35 / DC-002.9)
---------------------------------------------------------------------
1. `spectacle -f` (KDE/Wayland) fotografia la PANTALLA COMPLETA: la ventana del emulador hay que UBICARLA
   adentro de la captura. Aca se ubica por la corrida de negro puro mas larga de cada fila (el POC pinta el
   fondo en 0,0,0 exacto) y la pantalla son las filas que comparten el borde DERECHO (ese borde no se mueve;
   el izquierdo se une al negro del asset cuando el asset es oscuro).
2. La geometria de la ventana NO se supone: se mide (`MEDIDOR237 ventana=...`) o se pasa con `--ventana`,
   y el corpus del mutante reusa JUSTO esa geometria (la pantalla del mutante es negra entera: ahi no hay
   banda que detectar, y recortar a ojo seria medir el escritorio).
3. El POC pinta su quad en (0,0) de su pantalla de 640x480 (`d2cQuad(0,0,esc)`): el lienzo es de 512 y
   esc = min(640/512, 480/512) = 0,9375, o sea el asset ocupa EXACTAMENTE el alto de la pantalla, pegado
   arriba a la izquierda.
4. **Una referencia que sale del mismo lienzo no puede validar la orientacion**: la referencia del host es
   el MISMO lienzo de indices (mismo `nivel.go`), asi que un espejado se compara espejado contra espejado y
   da 100%. Por eso se puntua en las CUATRO orientaciones (identidad, espejada H, espejada V, rotada 180) y
   se informa cual gana: si gana otra que la identidad, el sprite salio espejado y se ve en el numero.

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
    """Las filas de la pantalla del emulador: las que comparten el borde DERECHO de la banda."""
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


def orientar(ref, cual):
    if cual == "identidad":
        return ref
    if cual == "espejada-h":
        return ref[:, ::-1]
    if cual == "espejada-v":
        return ref[::-1]
    return ref[::-1, ::-1]


def puntuar(img, ref, alto, x0, y0):
    h, w, _ = img.shape
    if x0 < 0 or y0 < 0 or x0 + alto > w or y0 + alto > h:
        return None
    b = bajar(img, x0, y0, alto, REF_LADO)
    dif = np.abs(b - ref).max(axis=2)
    n = dif.size
    return {"mas4": int((dif <= 4).sum()), "mas8": int((dif <= 8).sum()), "mas16": int((dif <= 16).sum()),
            "n": n, "err": float(dif.mean())}


def puntuar_grueso(img, ref, alto, x0, y0, lado_grueso=128):
    """La comparacion a 128x128: el render de la consola escala el lienzo (512 -> 480 logicos -> el alto del
    area cliente en la captura) y volver a subirlo a 512 mezcla pixeles. Medido en la sintetica: a 512 el
    mismo asset derecho da 80% a +-8 y a 128 da el 99%. El numero fino sirve para la ORIENTACION (la
    diferencia entre orientaciones es enorme) y el grueso para decir "es el mismo mapa"."""
    h, w, _ = img.shape
    if x0 < 0 or y0 < 0 or x0 + alto > w or y0 + alto > h:
        return None
    b = bajar(img, x0, y0, alto, lado_grueso)
    r = bajar(ref, 0, 0, ref.shape[0], lado_grueso)
    dif = np.abs(b - r).max(axis=2)
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
        print("  area cliente del emulador (--ventana): x=%d..%d y=%d..%d (%dx%d)" %
              (izq, izq + ancho_cliente - 1, arriba, arriba + alto - 1, ancho_cliente, alto))
    else:
        filas = banda_negra(img)
        det = pantalla_por_banda(filas)
        if det is None or det["alto"] < 100:
            print("MEDIDOR237 SIN-BANDA")
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

    print("MEDIDOR237 ventana=%d,%d,%d,%d" % (izq, arriba, ancho_cliente, alto))

    escala = alto / float(ALTO_LOGICO)
    ancho_pantalla = int(round(ANCHO_LOGICO * escala))
    pantalla_izq = izq + (ancho_cliente - ancho_pantalla) // 2
    print("  pantalla de la consola (640x480 escalada x%.4f): x=%d..%d y=%d..%d (%dx%d)" %
          (escala, pantalla_izq, pantalla_izq + ancho_pantalla - 1, arriba, arriba + alto - 1,
           ancho_pantalla, alto))

    # el asset mide 512 * esc en la pantalla logica, y esc = min(640/512, 480/512) = 480/512: o sea que el
    # asset ocupa EXACTAMENTE el alto del area cliente (el mismo caso que DC-002.35, donde era 256 -> 480).
    lado_asset = int(round(min(ANCHO_LOGICO, ALTO_LOGICO) * escala))
    print("  bloque del asset (%dx%d escalado = el alto entero, pegado arriba-izquierda): x=%d..%d (%dx%d)" %
          (REF_LADO, REF_LADO, pantalla_izq, pantalla_izq + lado_asset - 1, lado_asset, lado_asset))

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

    # Cuanto del LIENZO de referencia no es negro: es la expectativa MEDIDA de pixeles pintados que tiene
    # que tener la captura (el corpus compara contra este numero, no contra un umbral inventado). OJO: la
    # referencia pinta el indice 0 con su color de paleta y en la consola el indice 0 es TRANSPARENTE (sale
    # el fondo negro), asi que la captura puede tener unos pocos pixeles menos: por eso hay tolerancia.
    ref_no_negro = int((~(ref <= UMBRAL_NEGRO).all(axis=2)).sum())
    ref_tot = ref.shape[0] * ref.shape[1]
    print("  la referencia tiene %d de %d pixeles que no son negro (%.2f%%)" %
          (ref_no_negro, ref_tot, 100.0 * ref_no_negro / ref_tot))
    print("MEDIDOR237 refNoFondo=%.2f" % (100.0 * ref_no_negro / ref_tot))

    # La alineacion se busca UNA vez (con la identidad) y despues se puntua cada orientacion en ESA
    # alineacion: asi la comparacion entre orientaciones es la misma ventana de pixeles.
    mejor = None
    for paso in (4, 1):
        rango = busca if mejor is None else paso * 2
        base_dx = 0 if mejor is None else mejor[0]
        base_dy = 0 if mejor is None else mejor[1]
        candidatos = [(dx, dy)
                      for dx in range(base_dx - rango, base_dx + rango + 1, paso)
                      for dy in range(base_dy - rango, base_dy + rango + 1, paso)]
        for dx, dy in candidatos:
            p = puntuar(img, ref, lado_asset, pantalla_izq + dx, arriba + dy)
            if p is None:
                continue
            clave = (p["mas8"], p["mas4"], -p["err"])
            if mejor is None or clave > (mejor[2]["mas8"], mejor[2]["mas4"], -mejor[2]["err"]):
                mejor = (dx, dy, p)

    if mejor is None:
        return 0

    dx, dy, p = mejor
    print("  MEJOR ALINEACION (buscando +-%d px): dx=%+d dy=%+d -> x=%d y=%d" %
          (busca, dx, dy, pantalla_izq + dx, arriba + dy))
    print(linea_puntaje("identidad, en la mejor alineacion", p))

    ganadora = "identidad"
    mejor_pct = 100.0 * p["mas8"] / p["n"]
    print("MEDIDOR237 orientacion=identidad mas8=%.2f mas4=%.2f error=%.2f" %
          (mejor_pct, 100.0 * p["mas4"] / p["n"], p["err"]))

    for cual in ORIENTACIONES[1:]:
        q = puntuar(img, orientar(ref, cual), lado_asset, pantalla_izq + dx, arriba + dy)
        if q is None:
            continue
        pct = 100.0 * q["mas8"] / q["n"]
        print(linea_puntaje("%s, misma alineacion" % cual, q))
        print("MEDIDOR237 orientacion=%s mas8=%.2f mas4=%.2f error=%.2f" %
              (cual, pct, 100.0 * q["mas4"] / q["n"], q["err"]))
        if pct > mejor_pct:
            mejor_pct, ganadora = pct, cual

    print("MEDIDOR237 MEJOR=%s mas8=%.2f" % (ganadora, mejor_pct))

    # la comparacion a 128x128 (la que dice "es el mismo mapa", sin el ruido del reescalado)
    gr = puntuar_grueso(img, ref, lado_asset, pantalla_izq + dx, arriba + dy)
    if gr is not None:
        print(linea_puntaje("a 128x128, contra la referencia", gr))
        print("MEDIDOR237 grueso mas8=%.2f mas16=%.2f error=%.2f" %
              (100.0 * gr["mas8"] / gr["n"], 100.0 * gr["mas16"] / gr["n"], gr["err"]))

    return 0


if __name__ == "__main__":
    sys.exit(main())
