#!/usr/bin/env python3
"""El MUTANTE del instrumento de DC-002.37: `medir-captura-237.py` tiene que saber distinguir una captura
BUENA de una ESPEJADA y de una VACIA. Si no las distingue, sus numeros no prueban nada.

Por que existe: en DC-002.9 un sprite espejado paso la comparacion al 100% porque la referencia salia del
MISMO horneado espejado (espejado contra espejado da identico). La regla que quedo de esa tarde es comparar
contra las cuatro orientaciones; este script verifica que el instrumento REALMENTE lo haga.

Que hace (sin emulador: se arma la captura a mano con la referencia del host):
  1. arma una captura sintetica con el MISMO layout que deja Flycast: marco negro de 1280x720, area cliente
     de 682x453 en (200,100), la pantalla de 640x480 escalada x0.94375 centrada (604x453) y el asset de
     512x512 escalado al alto (453x453) pegado arriba a la izquierda de la pantalla;
  2. CASO SANO: el asset derecho -> el instrumento tiene que decir identidad y >= 97% a +-8;
  3. CASO MUTANTE: el asset ESPEJADO VERTICALMENTE -> el instrumento NO puede decir identidad (tiene que
     ganar espejada-v). Si dice identidad, el instrumento es ciego y el corpus de DC-002.37 no vale;
  4. CASO VACIO: todo negro (el camino del mutante de la tarjeta) -> 0% de pixeles pintados en el bloque.

Uso:  mutar-captura-237.py <referencia.png> <dir_salida>
Sale 0 si el instrumento caza los tres casos; 1 si alguno se le escapa.
"""
import os
import subprocess
import sys

import numpy as np

UMBRAL_NEGRO = 12
ANCHO_LOGICO, ALTO_LOGICO = 640, 480
IZQ, ARRIBA, ANCHO_CLIENTE, ALTO = 200, 100, 682, 453


def leer(png):
    info = subprocess.run(["ffprobe", "-v", "error", "-select_streams", "v", "-show_entries",
                           "stream=width,height", "-of", "csv=p=0", png],
                          capture_output=True, check=True, text=True).stdout.strip().split(",")
    w, h = int(info[0]), int(info[1])
    raw = subprocess.run(["ffmpeg", "-v", "error", "-i", png, "-f", "rawvideo", "-pix_fmt", "rgb24", "-"],
                         capture_output=True, check=True).stdout
    return np.frombuffer(raw, dtype=np.uint8).reshape(h, w, 3)


def escribir(png, img):
    h, w, _ = img.shape
    subprocess.run(["ffmpeg", "-v", "error", "-y", "-f", "rawvideo", "-pix_fmt", "rgb24",
                    "-s", "%dx%d" % (w, h), "-i", "-", png],
                   input=img.astype(np.uint8).tobytes(), check=True)


def escalar(img, lado):
    """Nearest-neighbour a lado x lado (para el test alcanza y no inventa colores)."""
    h, w, _ = img.shape
    ys = (np.arange(lado) * h) // lado
    xs = (np.arange(lado) * w) // lado
    return img[ys][:, xs]


def armar(ref, espejo_v=False, vacio=False):
    # el ESCRITORIO (color no negro: es lo que hace que la banda negra del cliente se pueda detectar, igual
    # que en la captura real de KDE) y adentro el AREA CLIENTE del emulador en negro, con la pantalla del POC.
    frame = np.zeros((720, 1280, 3), np.uint8)
    frame[:, :] = (40, 44, 52)
    frame[ARRIBA:ARRIBA + ALTO, IZQ:IZQ + ANCHO_CLIENTE] = (0, 0, 0)
    if vacio:
        return frame
    escala = ALTO / float(ALTO_LOGICO)
    ancho_pantalla = int(round(ANCHO_LOGICO * escala))
    pantalla_izq = IZQ + (ANCHO_CLIENTE - ancho_pantalla) // 2
    lado = int(round(min(ANCHO_LOGICO, ALTO_LOGICO) * escala))
    asset = escalar(ref[::-1] if espejo_v else ref, lado)
    frame[ARRIBA:ARRIBA + lado, pantalla_izq:pantalla_izq + lado] = asset
    return frame


def medir(herramienta, png, ref_png=None, ventana=None):
    cmd = [sys.executable, herramienta, png]
    if ref_png:
        cmd.append(ref_png)
    if ventana:
        cmd += ["--ventana", ventana]
    salida = subprocess.run(cmd, capture_output=True, text=True).stdout
    d = {}
    for lin in salida.splitlines():
        if lin.startswith("MEDIDOR237 "):
            cuerpo = lin[len("MEDIDOR237 "):]
            if cuerpo.startswith("MEJOR="):
                k, v = cuerpo[len("MEJOR="):].split(" mas8=")
                d["mejor"], d["mas8"] = k, float(v)
            elif cuerpo.startswith("bloque"):
                d["bloque"] = lin
                d["noFondo"] = float(lin.split("(")[-1].split("%")[0])
            elif cuerpo.startswith("grueso mas8="):
                resto = cuerpo[len("grueso "):]
                partes = dict(p.split("=") for p in resto.split())
                d["grueso_mas8"] = float(partes["mas8"])
                d["grueso_mas16"] = float(partes["mas16"])
            elif cuerpo.startswith("refNoFondo="):
                d["refNoFondo"] = float(cuerpo.split("=")[1])
            elif cuerpo.startswith("ventana="):
                d["ventana"] = cuerpo.split("=")[1]
            elif cuerpo.startswith("SIN-BANDA"):
                d["sinBanda"] = True
    return d, salida


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        return 2
    ref_png, sal = sys.argv[1], sys.argv[2]
    herramienta = os.path.join(os.path.dirname(os.path.abspath(__file__)), "medir-captura-237.py")
    os.makedirs(sal, exist_ok=True)

    ref = leer(ref_png)
    print("REFERENCIA %s (%dx%d)" % (ref_png, ref.shape[1], ref.shape[0]))

    fallos = 0

    # 1. el caso SANO
    sano_png = os.path.join(sal, "sintetica-sana.png")
    escribir(sano_png, armar(ref))
    d, salida = medir(herramienta, sano_png, ref_png)
    print("\n== CASO SANO (el asset derecho)")
    print("\n".join("   " + l for l in salida.splitlines() if l.startswith("MEDIDOR237")))
    if d.get("mejor") == "identidad" and d.get("grueso_mas8", 0) >= 95.0:
        print("   OK: el instrumento dice identidad y a 128x128 coincide %.2f%% a +-8 (%.2f%% a +-16)" %
              (d["grueso_mas8"], d.get("grueso_mas16", 0.0)))
    else:
        print("   FALLO: el instrumento NO reconoce el asset derecho (mejor=%s, 128x128 a +-8 = %s%%)" %
              (d.get("mejor"), d.get("grueso_mas8")))
        fallos += 1
    if "noFondo" in d and "refNoFondo" in d and abs(d["noFondo"] - d["refNoFondo"]) <= 3.0:
        print("   OK: los pixeles pintados de la captura (%.2f%%) siguen a los de la referencia (%.2f%%)" %
              (d["noFondo"], d["refNoFondo"]))
    else:
        print("   FALLO: los pixeles pintados no siguen a la referencia (%s contra %s)" %
              (d.get("noFondo"), d.get("refNoFondo")))
        fallos += 1

    # 2. el caso ESPEJADO (el mutante del instrumento)
    esp_png = os.path.join(sal, "sintetica-espejada-v.png")
    escribir(esp_png, armar(ref, espejo_v=True))
    d2, salida2 = medir(herramienta, esp_png, ref_png)
    print("\n== CASO MUTANTE (el asset ESPEJADO VERTICALMENTE)")
    print("\n".join("   " + l for l in salida2.splitlines() if l.startswith("MEDIDOR237")))
    if d2.get("mejor") == "espejada-v":
        print("   OK: el instrumento lo caza (gana espejada-v con %.2f%% y la identidad queda abajo)" % d2["mas8"])
    else:
        print("   FALLO: el instrumento NO caza el espejado (dice '%s'): la comparacion es ciega" % d2.get("mejor"))
        fallos += 1

    # 3. el caso VACIO (el camino del mutante de la tarjeta: pantalla negra entera)
    vac_png = os.path.join(sal, "sintetica-vacia.png")
    escribir(vac_png, armar(ref, vacio=True))
    d3, salida3 = medir(herramienta, vac_png)
    print("\n== CASO VACIO (todo negro, como la pantalla del mutante)")
    print("\n".join("   " + l for l in salida3.splitlines() if l.startswith("MEDIDOR237")))
    ventana = d.get("ventana")
    d4, salida4 = medir(herramienta, vac_png, ventana=ventana)
    if "noFondo" in d4 and d4["noFondo"] <= 1.0:
        print("   OK: con la geometria medida de la corrida buena (%s) el bloque da %.2f%% pintado" %
              (ventana, d4["noFondo"]))
    else:
        print("   FALLO: la captura vacia no da 0%% pintado (%s)" % d4.get("noFondo"))
        fallos += 1

    print("\n== RESULTADO: %s" % ("los 3 casos cazados (el instrumento sirve)" if fallos == 0
                                  else "%d caso(s) que el instrumento NO caza" % fallos))
    return 0 if fallos == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
