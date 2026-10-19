#!/usr/bin/env python3
"""DC-002.49 -- MEDIR la animacion del personaje en las capturas de Flycast.

QUE CONTESTA (los tres criterios de la tarjeta, con numeros):

  1. `identidad` de la CAJA DEL PERSONAJE contra cada cuadro del ciclo del host: para cada captura se
     recorta la caja del personaje (la misma que reporta el POC en su marca HITO249: ANIM) y se compara
     contra los 16 (TN) u 8 (TW) cuadros que compuso el host con el MISMO compositor. La captura se queda
     con el cuadro que mas se le parece, y se informa el segundo: si el primero no le gana por lejos al
     segundo, el numero no significa nada (es la leccion de DC-002.35: una referencia horneada del mismo
     lado puede validar un espejado).

  2. `distintos` entre DOS capturas: cuantos pixeles cambian DENTRO de la caja del personaje. Con el
     personaje QUIETO ese numero es la ANIMACION (no hay desplazamiento que lo explique). Es el numero del
     criterio 1, y el del criterio 3 cuando tiene que dar 0 (el mutante).

  3. `cuadros vistos`: que cuadros del ciclo aparecieron en las capturas (los distintos, y cuantos).

POR QUE LA CAJA Y NO LA PANTALLA ENTERA: medido en el host (y en DC-002.49), el cuadro entero cambia
~87 % de sus pixeles entre dos instantes por el re-sombreado de luces; la pantalla del POC ni siquiera se
repinta si el personaje no se mueve, asi que el cambio que sobrevive es el de la caja del personaje. Y para
distinguir ANIMACION de DESPLAZAMIENTO no hay otra forma que medir con el personaje quieto: si se mueve, la
caja cambia por moverse.

Uso:
  medir-anim-249.py --host DIR --modo TN --caja "izq,arriba,ancho,alto" --ancla 320,240 \\
                    [--ventana L,T,W,H] captura1.png [captura2.png ...]

Sale 0 siempre: los numeros son el resultado (el veredicto lo da el corpus checar-anim-249.py).
"""
import importlib.util
import os
import sys

import numpy as np

AQUI = os.path.dirname(os.path.abspath(__file__))
ANCHO_LOGICO, ALTO_LOGICO = 640, 480
TOLERANCIA = 40          # diferencia maxima por canal para decir "este pixel es el mismo" (reescalado del emulador)


def cargar_medidor_238():
    """Reusa decodificar() y bajar() de medir-captura-238.py (el mismo instrumento de la tarjeta anterior)."""
    ruta = os.path.join(AQUI, "medir-captura-238.py")
    spec = importlib.util.spec_from_file_location("medidor238", ruta)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


MED = cargar_medidor_238()


def logica(img, ventana):
    """El area cliente de la captura y la escala: la pantalla logica del POC esta metida ahi."""
    alto, ancho, _ = img.shape
    if ventana:
        izq, arriba, ancho_cliente, alto_cliente = ventana
    else:
        izq, arriba, ancho_cliente, alto_cliente = 0, 0, ancho, alto

    escala = alto_cliente / float(ALTO_LOGICO)
    ancho_pantalla = int(round(ANCHO_LOGICO * escala))
    pantalla_izq = izq + (ancho_cliente - ancho_pantalla) // 2

    return pantalla_izq, arriba, escala


def recorte_en_la_captura(img, ventana, caja_logica):
    """La caja del personaje RECORTADA EN LA RESOLUCION DE LA CAPTURA (no se reescala la captura: se compara
    en su propia grilla, que es la que no inventa pixeles). La caja se recorta a la pantalla: si el personaje
    se fue de cuadro, lo que se compara es lo que se ve (y el corpus lo declara con el ancla)."""
    pantalla_izq, arriba, escala = logica(img, ventana)
    x0, y0, x1, y1 = caja_logica
    alto, ancho, _ = img.shape
    cx0 = max(pantalla_izq, pantalla_izq + int(round(x0 * escala)))
    cy0 = max(arriba, arriba + int(round(y0 * escala)))
    cx1 = min(pantalla_izq + int(round(ANCHO_LOGICO * escala)), pantalla_izq + int(round(x1 * escala)))
    cy1 = min(arriba + int(round(ALTO_LOGICO * escala)), arriba + int(round(y1 * escala)))
    cx1 = min(cx1, ancho)
    cy1 = min(cy1, alto)
    if cx1 <= cx0 or cy1 <= cy0:
        return None
    return img[cy0:cy1, cx0:cx1].astype(np.int16)


def recorte_de_la_referencia(ref, caja_logica, tamano):
    """La caja del personaje de la referencia del host (640x480), bajada al tamano de la captura."""
    x0, y0, x1, y1 = caja_logica
    alto, ancho, _ = ref.shape
    x0, x1 = max(0, min(x0, ancho - 1)), max(1, min(x1, ancho))
    y0, y1 = max(0, min(y0, alto - 1)), max(1, min(y1, alto))
    if x1 <= x0 or y1 <= y0:
        return None
    ancho_d, alto_d = tamano
    return MED.bajar(ref, x0, y0, x1 - x0, y1 - y0, ancho_d, alto_d).astype(np.int16)


def caja_de(ancla, caja):
    """La caja del personaje en pixeles de la pantalla logica: (ancla + esquina, tamano)."""
    izq, arriba, ancho, alto = caja
    x0 = ancla[0] + izq
    y0 = ancla[1] + arriba
    return x0, y0, x0 + ancho, y0 + alto


def identidad(a, b):
    """% de pixeles iguales (+-TOLERANCIA por canal) entre dos recortes del mismo tamano."""
    dif = np.abs(a.astype(np.int16) - b.astype(np.int16)).max(axis=2)
    return 100.0 * float((dif <= TOLERANCIA).sum()) / dif.size


def difieren(a, b):
    """Cuantos pixeles del recorte NO son el mismo (+-TOLERANCIA por canal), y la caja del cambio."""
    dif = np.abs(a.astype(np.int16) - b.astype(np.int16)).max(axis=2)
    mascara = dif > TOLERANCIA
    n = int(mascara.sum())
    if n == 0:
        return 0, None
    ys, xs = np.nonzero(mascara)
    return n, (int(xs.min()), int(ys.min()), int(xs.max() - xs.min() + 1), int(ys.max() - ys.min() + 1))


def main():
    args = sys.argv[1:]
    opciones = {"--host": None, "--modo": "TN", "--caja": None, "--ancla": "320,240", "--ventana": None}
    resto = []
    i = 0
    while i < len(args):
        if args[i] in opciones:
            opciones[args[i]] = args[i + 1]
            i += 2
        else:
            resto.append(args[i])
            i += 1

    modo = opciones["--modo"]
    caja = [int(v) for v in opciones["--caja"].split(",")]
    ancla = [int(v) for v in opciones["--ancla"].split(",")]
    ventana = [int(v) for v in opciones["--ventana"].split(",")] if opciones["--ventana"] else None

    print("ANIM249 modo=%s caja=%d,%d %dx%d ancla=%d,%d capturas=%d" %
          (modo, caja[0], caja[1], caja[2], caja[3], ancla[0], ancla[1], len(resto)))

    # ---- las referencias del host: la pantalla entera con cada cuadro del ciclo encima
    host = opciones["--host"]
    refs = []
    if host:
        for f in sorted(os.listdir(host)):
            if f.startswith("pantalla-%s-" % modo) and f.endswith(".png"):
                cuadro = int(f.rsplit("-", 1)[1].split(".")[0])
                img = MED.decodificar(os.path.join(host, f))
                if img.shape[0] != ALTO_LOGICO or img.shape[1] != ANCHO_LOGICO:
                    continue
                refs.append((cuadro, img))
        refs.sort()
        print("ANIM249 referencias=%d cuadros=%s" % (len(refs), ",".join(str(c) for c, _ in refs)))

    caja_logica = caja_de(ancla, caja)

    # ---- las capturas: la caja del personaje de cada una, EN SU PROPIA RESOLUCION
    recortes = []
    for ruta in resto:
        img = MED.decodificar(ruta)
        recorte = recorte_en_la_captura(img, ventana, caja_logica)
        nombre = os.path.basename(ruta)

        if recorte is None or recorte.size == 0:
            print("ANIM249 %s FUERA-DE-CUADRO (la caja del personaje no entra en la pantalla)" % nombre)
            continue

        linea = "ANIM249 %s caja=%dx%d" % (nombre, recorte.shape[1], recorte.shape[0])
        if refs:
            referencias = [(c, recorte_de_la_referencia(r, caja_logica, (recorte.shape[1], recorte.shape[0])))
                           for c, r in refs]
            referencias = [(c, r) for c, r in referencias if r is not None and r.shape == recorte.shape]
            puntajes = sorted(((identidad(recorte, r), c) for c, r in referencias), reverse=True)
            if puntajes:
                mejor, peor = puntajes[0], puntajes[-1]
                segunda = puntajes[1] if len(puntajes) > 1 else (0.0, -1)
                linea += " cuadro=%d identidad=%.2f%% segundo=%d:%.2f%% ventaja=%.2f peor=%d:%.2f%%" % (
                    mejor[1], mejor[0], segunda[1], segunda[0], mejor[0] - segunda[0], peor[1], peor[0])
        print(linea)
        recortes.append((nombre, recorte))

    # ---- dos capturas distintas: cuantos pixeles cambian dentro de la caja
    if len(recortes) >= 2:
        base_nombre, base = recortes[0]
        for nombre, recorte in recortes[1:]:
            n, caja_cambio = difieren(base, recorte)
            print("ANIM249 DISTINTOS %s vs %s pixelesCambiados=%d de=%d cajaDelCambio=%s" %
                  (base_nombre, nombre, n, base.size,
                   ("%d,%d %dx%d" % caja_cambio) if caja_cambio else "-"))

        # todos contra todos: el minimo y el maximo (con el ciclo andando el minimo tiene que ser > 0,
        # con el mutante el maximo tiene que ser 0)
        valores = []
        for i in range(len(recortes)):
            for j in range(i + 1, len(recortes)):
                n, _ = difieren(recortes[i][1], recortes[j][1])
                valores.append(n)
        print("ANIM249 PARES pares=%d minimo=%d maximo=%d promedio=%.1f" %
              (len(valores), min(valores), max(valores), float(sum(valores)) / len(valores)))

    if recortes:
        # los dibujos DISTINTOS que se vieron en las capturas: dos capturas son el mismo dibujo si su caja
        # es igual pixel a pixel (+-TOLERANCIA). Es el "cuantos se ven de verdad" del criterio 2, medido en
        # la pantalla y no en el log.
        grupos = []
        for nombre, recorte in recortes:
            for g in grupos:
                if identidad(recorte, g[0][1]) >= 99.0:
                    g.append((nombre, recorte))
                    break
            else:
                grupos.append([(nombre, recorte)])

        print("ANIM249 VISTOS capturas=%d dibujosDistintos=%d grupos=%s" %
              (len(recortes), len(grupos),
               " | ".join(",".join(n for n, _ in g) for g in grupos)))

    return 0


if __name__ == "__main__":
    sys.exit(main())
