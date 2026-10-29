#!/usr/bin/env python3
"""DC-002.49b -- ¿QUE direccion del COF se ve en la CAPTURA? (el canal de los pixeles)

POR QUE EXISTE
--------------
El canal del log (`HITO249: ANIM ... crc32`) dice que compuesto se dibujo, pero es el POC diciendo que hizo.
Este dice que se VE. Los dos tienen que coincidir, y son independientes: el del log compara crc32 contra la
tabla del host; este compara PIXELES contra las referencias del host.

COMO COMPARA (la leccion de DC-002.49)
--------------------------------------
La region NO es la pantalla entera (que da ~68 % de identidad entre dos cuadros cualesquiera y esconde todo):
es la UNION de lo que cada referencia cambia respecto del mapa solo, o sea el personaje y nada mas. Cada
referencia se baja al tamano del recorte de la captura (no se reescala la captura: no se inventan pixeles) y
se mide la identidad pixel a pixel con tolerancia por canal.

LA REGLA QUE HACE QUE ESTO NO SE ENGANE A SI MISMO
--------------------------------------------------
La captura se tiene que parecer a SU direccion MUCHO mas que a la segunda: se informa el margen
(mejor - segundo). Si el margen es chico, el numero no dice nada y el corpus lo tiene que declarar.

EL ANCLA (DC-002.64, medido): las referencias tienen que estar compuestas EN EL MISMO ANCLA que la captura, y
el ancla del POC es `-centro 28,20` (la ventana de 512 del nivel, pegada en 64,-16 de la pantalla logica: o
sea origen 304,1664) con `-pos <celda>` EXACTA del personaje. Eso lo arma `canal-pixeles-249b.py`, que ademas
verifica la linea VENTANA del host antes de creerle a las referencias.

OJO (DC-002.64): la version anterior de este comentario culpaba al `-pos` del host de no pintar donde pinta el
POC. Medido: era falso — con `-centro 28,20` y la celda exacta el host pinta EXACTAMENTE donde el POC (misma
cuenta: `PixelDeLaCelda(celda, origen) + (mapaX,mapaY)`). Lo que fallaba era quien armaba la referencia: le
pasaba la celda RECORTADA a entero (medio subtile = 80 px de error). Ver `direccion-por-captura.py`.

Uso:
  clasificar-captura-249b.py --host REFS --mapa REFS/mapa.png --modo TN --ventana L,T,W,H captura.png ...
"""
import argparse
import importlib.util
import os
import sys

import numpy as np

AQUI = os.path.dirname(os.path.abspath(__file__))
DIRS_DEL_CONTROL = [0, 1, 4, 7, 8, 9, 12, 15]


def cargar(nombre, archivo):
    spec = importlib.util.spec_from_file_location(nombre, os.path.join(AQUI, archivo))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


MED = cargar("medidor238", "medir-captura-238.py")
ANIM = cargar("medidor249", "medir-anim-249.py")


def region_de_las_referencias(refs, mapa, tolerancia):
    """La union de lo que CADA direccion (y CADA cuadro de su ciclo) pinta encima del mapa solo, en pixeles
    logicos 640x480."""
    x0 = y0 = 10 ** 9
    x1 = y1 = -1
    for d, imagenes in refs.items():
        for img in imagenes:
            cambio = (np.abs(img.astype(np.int16) - mapa.astype(np.int16)).max(axis=2) > tolerancia)
            ys, xs = np.where(cambio)
            if len(xs) == 0:
                raise SystemExit(f"VACIA dir={d:02d}: no cambia ningun pixel respecto del mapa")
            x0, y0 = min(x0, int(xs.min())), min(y0, int(ys.min()))
            x1, y1 = max(x1, int(xs.max()) + 1), max(y1, int(ys.max()) + 1)
    return x0, y0, x1, y1


def imagenes_de_la_direccion(host, modo, d):
    """Los PNG de una direccion: el CICLO ENTERO si esta (`-cNN`), o el cuadro 0 solo si es de antes (DC-002.64)."""
    prefijo = "pantalla-%s-d%02d" % (modo, d)
    ciclo = sorted(n for n in os.listdir(host) if n.startswith(prefijo + "-c") and n.endswith(".png"))
    if ciclo:
        return [os.path.join(host, n) for n in ciclo]
    suelto = os.path.join(host, prefijo + ".png")
    return [suelto] if os.path.exists(suelto) else []


def cargar_referencias(host, modo, dirs):
    """{dir: [imagenes del ciclo]} y cuantos cuadros trae cada direccion."""
    refs, ruta = {}, {}
    faltan = []
    for d in dirs:
        rutas = imagenes_de_la_direccion(host, modo, d)
        if not rutas:
            faltan.append("pantalla-%s-d%02d*.png" % (modo, d))
            continue
        refs[d] = [MED.decodificar(r) for r in rutas]
        ruta[d] = rutas
    if faltan:
        raise SystemExit("FALTAN las referencias: " + ", ".join(faltan))
    return refs, ruta


def clasificar(host, mapa, modo, ventana, capturas, dirs=None, tolerancia=40):
    """Una fila por captura: el ranking de identidad contra cada direccion, el mejor, el segundo y el margen.

    La identidad de una DIRECCION es la mejor de su ciclo entero (DC-002.64): la captura cayo en un cuadro
    cualquiera de la animacion, y comparar solo contra el cuadro 0 castigaba a todas las direcciones por igual
    (medido: 81 % en vez de 93 % para la direccion correcta).

    Devuelve (region, filas). Cada fila es un dict con: captura, recorte, ranking (lista de (ident, dir)),
    mejor, segundo, margen, y el cuadro que gano por direccion; o `falla` con el motivo.
    """
    dirs = dirs or DIRS_DEL_CONTROL
    mapa_img = MED.decodificar(mapa)
    refs, rutas = cargar_referencias(host, modo, dirs)
    region = region_de_las_referencias(refs, mapa_img, 8)
    cuadros = {d: len(refs[d]) for d in dirs}

    filas = []
    for ruta in capturas:
        fila = {"captura": os.path.basename(ruta), "region": region, "cuadros": cuadros}
        img = MED.decodificar(ruta)
        recorte = ANIM.recorte_en_la_captura(img, ventana, region)
        if recorte is None:
            fila["falla"] = "la region del personaje no cae dentro de la captura"
            filas.append(fila)
            continue

        alto_d, ancho_d = recorte.shape[0], recorte.shape[1]
        ranking, ganador = [], {}
        for d in dirs:
            mejor_d = None
            for k, ref in enumerate(refs[d]):
                ref_crop = ANIM.recorte_de_la_referencia(ref, region, (ancho_d, alto_d))
                if ref_crop is None:
                    continue
                ident = ANIM.identidad(recorte, ref_crop)
                if mejor_d is None or ident > mejor_d[0]:
                    mejor_d = (ident, k)
            if mejor_d is None:
                continue
            ranking.append((mejor_d[0], d))
            ganador[d] = mejor_d
        if not ranking:
            fila["falla"] = "ninguna referencia entra en el recorte"
            filas.append(fila)
            continue

        ranking.sort(reverse=True)
        fila["recorte"] = (ancho_d, alto_d)
        fila["ranking"] = ranking
        fila["ganador"] = ganador
        fila["mejor"] = ranking[0]
        fila["segundo"] = ranking[1] if len(ranking) > 1 else (0.0, -1)
        fila["margen"] = fila["mejor"][0] - fila["segundo"][0]
        fila["cuadro_del_mejor"] = ganador[ranking[0][1]][1]
        filas.append(fila)
    return region, filas


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--host", required=True, help="carpeta con pantalla-<MODO>-dNN.png de ESTE ancla")
    ap.add_argument("--mapa", required=True, help="la pantalla SIN el personaje, de ESTE ancla (-png-mapa)")
    ap.add_argument("--modo", default="TN")
    ap.add_argument("--ventana", default="", help="area cliente del emulador: L,T,W,H (la linea GEO de la corrida)")
    ap.add_argument("--tolerancia", type=int, default=40,
                    help="diferencia maxima por canal para 'mismo pixel' (40 = el reescalado del emulador, DC-002.49)")
    ap.add_argument("--dirs", default=",".join(str(d) for d in DIRS_DEL_CONTROL))
    ap.add_argument("capturas", nargs="+")
    args = ap.parse_args()

    ventana = None
    if args.ventana:
        ventana = tuple(int(x) for x in args.ventana.split(","))
        if len(ventana) != 4:
            raise SystemExit("--ventana tiene que ser L,T,W,H")

    dirs = [int(x) for x in args.dirs.split(",")]
    region, filas = clasificar(args.host, args.mapa, args.modo, ventana, args.capturas, dirs, args.tolerancia)
    print("region del personaje (union de las %d direcciones, pixeles logicos): %d,%d -> %d,%d  (%dx%d)"
          % (len(dirs), region[0], region[1], region[2], region[3], region[2] - region[0], region[3] - region[1]))

    for f in filas:
        if f.get("falla"):
            print("%-28s FALLA: %s" % (f["captura"], f["falla"]))
            continue
        mejor, segundo = f["mejor"], f["segundo"]
        print("%-28s mejor=dir %02d (cuadro %02d)  identidad=%.2f %%  segundo=dir %02d (%.2f %%)  margen=%.2f pts  "
              "recorte=%dx%d"
              % (f["captura"], mejor[1], f["cuadro_del_mejor"], mejor[0], segundo[1], segundo[0], f["margen"],
                 f["recorte"][0], f["recorte"][1]))
        print("    ranking: " + "  ".join("%02d:%.2f" % (d, i) for i, d in f["ranking"]))

    return 0


if __name__ == "__main__":
    sys.exit(main())
