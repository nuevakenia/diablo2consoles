#!/usr/bin/env python3
"""DC-002.64 -- EL CANAL DE LOS PIXELES POR DIRECCION, de una: la captura dice que direccion se dibujo.

QUE HACE
--------
Para cada captura de una corrida del guion del giro:
 1. saca del log el modo, el cuadro, la caja, el crc32 y la CELDA EXACTA del personaje (con el testigo de
    posicion mas nuevo: `HITO238: POS` o `HITO238: CONTROL`), y de la tabla del host la direccion que dice el
    canal del log (`direccion-por-captura.py`);
 2. arma las referencias del host EN EL ANCLA DEL POC para ESA celda (`verif-personaje-238 -centro 28,20
    -pos <celda exacta> -censo-dirs <MODO> -png-dirs <MODO> -png-ciclo ... -png-mapa ...`) y VERIFICA la linea
    VENTANA antes de creerle a las referencias (si el ancla no es la del POC, no se clasifica: se aborta);
 3. clasifica la captura contra las 8 direcciones (`clasificar-captura-249b.py`) y exige que gane SU direccion
    POR MARGEN (>= --margen puntos sobre la segunda), no por un pelo;
 4. mide DONDE cae el personaje en la captura (los pixeles que no son el mapa del host) y de ahi despeja la
    CELDA DE LOS PIXELES: si las referencias caen donde cae la captura, esa celda tiene que ser la del testigo.
    Es el control de que el ancla coincide, con pixeles y no con la cuenta del que arma la referencia.

LAS CINCO LECCIONES QUE ESTE INSTRUMENTO TIENE METIDAS (todas medidas, ninguna elegida)
--------------------------------------------------------------------------------------
 1. LA CELDA VA CON DECIMALES. El POC camina en sub-tiles (`celda 29.3152 18.6848`) y el ancla se mueve
    (dx-dy)*80 en x y (dx+dy)*40 en y: truncar a 29,18 son ~30 px en x y ~40 en y, el personaje se sale de su
    caja y TODAS las direcciones caen a 70-74 % con margen de ruido (medido: c02 daba dir 07 74,38 %).
 2. SE COMPARA CONTRA EL CICLO ENTERO, no contra el cuadro 0: la captura cae en un cuadro cualquiera. Con el
    cuadro 0 la direccion correcta daba ~81 % y el margen 0,8-3,9 pts; con el ciclo entero, 92-99 % y 4-22 pts.
 3. LA CAPTURA VA A 1:1 (ventana 640x480), porque el emulador con 320x240 reduce el sprite de 26x77 a 13x38 y
    borra el detalle que separa direcciones finas (00/08/09): el margen cae a 0,5-1,5 pts.
 4. LA REGION ES LA UNION DE LO QUE CAMBIA, no la pantalla entera (que da ~68 % entre dos cuadros cualesquiera
    y esconde todo) y no la caja entera: se compara la union de lo que cada referencia cambia respecto del
    MAPA SOLO (el personaje y nada mas). El emulador tiene pixeles propios que el host no dibuja (franja de
    texto arriba): se miden como la interseccion de las mascaras de TODAS las capturas de la corrida (lo que no
    es el mapa en todas, no es el personaje) y se restan.
 5. EL MODO Y EL CUADRO DEL LOG NO SON LOS DE LA IMAGEN. La marca `HITO249: ANIM` se lee DESPUES del recorte
    (medido: 3,06 s entre capturas con 1,5 s de espera en el guion), asi que la imagen es ~1,5 s mas vieja y
    caminando eso son celdas y cuadros de diferencia. Por eso: el modo (TN/TW) y el cuadro se dejan libres, y si
    los pixeles dicen que el personaje no esta donde dice el testigo, se vuelven a armar las referencias en la
    celda de los pixeles (el mapa del host no depende del ancla: la ventana del nivel esta fija en 28,20).

CRITERIOS DE VERDE (los de la tarjeta)
--------------------------------------
 * `dir` del pixel == `dir` del log  -> la captura dice la misma direccion que el compositor;
 * margen >= --margen (4,0 pts)     -> el numero discrimina;
 * |celda de los pixeles - celda testigo| <= --celda-tolerancia (0,30 celda) en x y en y -> las referencias del
   host caen donde cae la captura.

Sobre las capturas REALES de la corrida (`/tmp/dc249b-run/pixeles`, guion derecha/arriba/izquierda/abajo).
No toca la tabla CRC249B ni el corpus de DC-002.49: solo lee la tabla y las capturas.

Uso:
  canal-pixeles-249b.py --run <corrida> --tabla <crc249b.txt> [--salida corpus.txt] [--refs carpeta]
                       [--margen 4.0] [--celda-tolerancia 0.30] [--dirs 0,1,4,7,8,9,12,15]
Sale 0 si TODAS las capturas evaluables dieron veredicto verde, y 1 si alguna quedo roja (el corpus se escribe
igual, con el motivo de cada roja).
"""
import argparse
import importlib.util
import os
import subprocess
import sys

import numpy as np

AQUI = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(os.path.dirname(AQUI))
TOLERANCIA_PIXEL = 40

# El ancla del POC (medida, no elegida): ventana del nivel de 512 centrada en la celda 28,20, pegada en
# (640-512)/2, (480-512)/2 = 64,-16 de la pantalla logica. El origen del nivel sale de la misma cuenta que
# hace `verif-personaje-238` y que hace el POC: (28-20)*80-80 = 560 y (28+20)*40 = 1920, menos lado/2 = 256.
CENTRO_POC = (28, 20)
LADO_VENTANA = 512
ORIGEN_POC = (304, 1664)
MAPA_X, MAPA_Y = 64, -16
LADO_CELDA_X, LADO_CELDA_Y, OFFSET_ISO = 80, 40, 80

# cuanto se le tolera al ancla del testigo antes de volver a armar las referencias en la celda MEDIDA: la marca
# ANIM se lee ~1,5 s despues de la captura (medido en la corrida) y caminando son ~2,8 celdas.
ANCLA_TOLERANCIA = 0.10

# los dos modos de la tabla: la captura es ~1,5 s MAS VIEJA que la marca ANIM, asi que el modo del log no es
# necesariamente el de la imagen
MODOS = ("TN", "TW")


def cargar(nombre, archivo):
    spec = importlib.util.spec_from_file_location(nombre, os.path.join(AQUI, archivo))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


DPC = cargar("dpc249b", "direccion-por-captura.py")
CLAS = cargar("clasif249b", "clasificar-captura-249b.py")
MED = CLAS.MED
ANIM = CLAS.ANIM


def ventana_de_la_corrida(corrida):
    ruta = os.path.join(corrida, "geo.txt")
    if not os.path.exists(ruta):
        raise SystemExit("la corrida no tiene geo.txt (lo escribe run-flycast-249b.sh): no se puede saber el "
                         "area cliente del emulador")
    for l in open(ruta, errors="replace"):
        if "cliente=" in l:
            return tuple(int(x) for x in l.split("cliente=")[1].split()[0].split(","))
    raise SystemExit("geo.txt sin lineas GEO con cliente=")


def armar_referencias(celda, modo, salida, dirs):
    """Corre el verificador del host en el ancla del POC para ESA celda exacta y devuelve su censo."""
    os.makedirs(salida, exist_ok=True)
    cmd = ["go", "run", "./console/dc/verif-personaje-238",
           "-centro", "%d,%d" % CENTRO_POC, "-ventana", str(LADO_VENTANA), "-pos", celda,
           "-censo-dirs", modo, "-png-dirs", modo,
           "-png-ciclo", salida, "-png-mapa", os.path.join(salida, "mapa.png")]
    r = subprocess.run(cmd, cwd=REPO, capture_output=True, text=True)
    if r.returncode != 0:
        raise SystemExit("el verificador del host fallo (celda %s):\n%s\n%s" % (celda, r.stdout[-1500:], r.stderr[-1500:]))
    with open(os.path.join(salida, "censo.txt"), "w") as f:
        f.write(r.stdout)
    return r.stdout


def verificar_ancla(censo, celda):
    """El host tiene que estar en la ventana del POC; si no, las referencias no sirven para esta captura."""
    linea = [l for l in censo.splitlines() if "VERIF238: VENTANA" in l]
    if not linea:
        raise SystemExit("el censo del host no trae la linea VENTANA")
    l = linea[0]
    esperado = "lado=%d origen=%d,%d centroCelda=%d,%d" % (LADO_VENTANA, ORIGEN_POC[0], ORIGEN_POC[1],
                                                            CENTRO_POC[0], CENTRO_POC[1])
    if esperado not in l:
        raise SystemExit("ANCLA MAL: el host no esta en la ventana del POC.\n  esperado: %s\n  el host:  %s"
                         % (esperado, l.strip()))
    pos = [x for x in censo.splitlines() if "VERIF238: POS " in x]
    if pos and ("celda=" + celda) not in pos[0].replace(",", ",") and celda not in pos[0]:
        # el host imprime la celda con 4 decimales: se compara por numero, no por texto
        try:
            cx, cy = (float(v) for v in celda.split(","))
            import re
            m = re.search(r"celda=([-0-9.]+),([-0-9.]+)", pos[0])
            if not m or abs(float(m.group(1)) - cx) > 0.01 or abs(float(m.group(2)) - cy) > 0.01:
                raise SystemExit("el host no tomo la celda: pedi %s y puso %s" % (celda, pos[0].strip()))
        except ValueError:
            pass
    return l.strip()


def componentes_de_la_mascara(mascara):
    """Componentes conexas (4-vecinos) de la mascara, como listas de (fila, columna)."""
    alto, ancho = mascara.shape
    visto = np.zeros_like(mascara)
    comps = []
    for y0, x0 in zip(*np.nonzero(mascara)):
        if visto[y0, x0]:
            continue
        visto[y0, x0] = True
        pila, pts = [(int(y0), int(x0))], []
        while pila:
            y, x = pila.pop()
            pts.append((y, x))
            for dy, dx in ((1, 0), (-1, 0), (0, 1), (0, -1)):
                ny, nx = y + dy, x + dx
                if 0 <= ny < alto and 0 <= nx < ancho and mascara[ny, nx] and not visto[ny, nx]:
                    visto[ny, nx] = True
                    pila.append((ny, nx))
        comps.append(pts)
    return comps


def caja_de(pts):
    ys = [p[0] for p in pts]
    xs = [p[1] for p in pts]
    return min(xs), min(ys), max(xs) + 1, max(ys) + 1


def mascara_de_la_captura(captura, mapa_png, ventana):
    """Los pixeles de la captura que NO son el mapa del host, ya en la grilla de la captura.

    Devuelve (mascara booleana, escala) o (None, escala).
    """
    img = MED.decodificar(captura)
    ref = MED.decodificar(mapa_png)
    pantalla_izq, arriba, escala = ANIM.logica(img, ventana)
    ancho_sup = int(round(ANIM.ANCHO_LOGICO * escala))
    alto_sup = int(round(ANIM.ALTO_LOGICO * escala))
    mapa_bajo = MED.bajar(ref, 0, 0, ANIM.ANCHO_LOGICO, ANIM.ALTO_LOGICO, ancho_sup, alto_sup)
    zona = img[arriba:arriba + alto_sup, pantalla_izq:pantalla_izq + ancho_sup]
    if zona.shape[:2] != mapa_bajo.shape[:2]:
        return None, escala
    dif = np.abs(zona.astype(np.int16) - mapa_bajo.astype(np.int16)).max(axis=2)
    m = dif > TOLERANCIA_PIXEL
    m[:2, :] = False
    m[-2:, :] = False
    m[:, :2] = False
    m[:, -2:] = False
    return m, escala


def celda_implicita(mascara, escala, esquina, artefacto=None):
    """DONDE cae el personaje en la captura, medido: las componentes conexas que no son el mapa del host.

    Los pixeles que difieren del mapa son el personaje MAS lo que dibuja el emulador y el host no (en este
    host: una franja de texto arriba y el marco del recorte). Eso NO se adivina: se mide. `artefacto` es la
    interseccion de las mascaras de TODAS las capturas de la corrida, y lo que esta ahi no es el personaje
    (el personaje se mueve entre capturas; la franja del emulador no). Se resta, y del resto se toma la
    componente mas grande que pueda ser un personaje (<= 90x120) junto con las que la tocan: el sprite suele
    venir partido en varias (un color de fondo lo separa).

    Devuelve (cx, cy, caja, pixeles) o None. La esquina del cuadro sale de la tabla del host (la de ESE crc32),
    asi que de la caja medida se despeja el ancla y de ahi la celda.
    """
    if mascara is None:
        return None
    m = mascara if artefacto is None else (mascara & ~artefacto)
    if not m.any():
        return None

    candidatas = []
    for pts in componentes_de_la_mascara(m):
        if len(pts) < 20:
            continue
        x0, y0, x1, y1 = caja_de(pts)
        if (x1 - x0) > 90 or (y1 - y0) > 120:
            continue
        candidatas.append((len(pts), pts))
    if not candidatas:
        return None

    candidatas.sort(reverse=True, key=lambda c: c[0])
    x0, y0, x1, y1 = caja_de(candidatas[0][1])
    juntos = list(candidatas[0][1])
    for n, pts in candidatas[1:]:
        bx0, by0, bx1, by1 = caja_de(pts)
        if bx0 - 6 <= x1 and bx1 + 6 >= x0 and by0 - 6 <= y1 and by1 + 6 >= y0:
            juntos += pts
    x0, y0, x1, y1 = caja_de(juntos)

    izq, arr = esquina
    ax = int(round(x0 / escala)) - MAPA_X - izq
    ay = int(round(y0 / escala)) - MAPA_Y - arr
    dxmy = (ax + OFFSET_ISO + ORIGEN_POC[0]) / float(LADO_CELDA_X)
    dxpy = (ay + ORIGEN_POC[1]) / float(LADO_CELDA_Y)
    return ((dxmy + dxpy) / 2.0, (dxpy - dxmy) / 2.0, (x1 - x0, y1 - y0), len(juntos))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--run", required=True, help="la corrida (flycast.log + geo.txt + capturas)")
    ap.add_argument("--tabla", required=True, help="la tabla CRC249B del host (go run ... -crc-dirs)")
    ap.add_argument("--salida", default="", help="el corpus de salida (por defecto <run>/canal-pixeles.txt)")
    ap.add_argument("--refs", default="/tmp/dc264-refs", help="carpeta de referencias (se reusan por celda)")
    ap.add_argument("--margen", type=float, default=4.0)
    ap.add_argument("--celda-tolerancia", type=float, default=0.30)
    ap.add_argument("--dirs", default=",".join(str(d) for d in CLAS.DIRS_DEL_CONTROL))
    ap.add_argument("--ver", action="store_true", help="deja todo el detalle en pantalla")
    args = ap.parse_args()

    tabla_txt = DPC.leer_tabla(args.tabla)                 # (modo, crc) -> (dir, cuadro, caja, izq, arriba)
    filas = DPC.capturas_de_la_corrida(args.run, tabla_txt)
    ventana = ventana_de_la_corrida(args.run)
    dirs = [int(x) for x in args.dirs.split(",")]
    salida = args.salida or os.path.join(args.run, "canal-pixeles.txt")

    print("CANAL-PIXELES 249B corrida=%s ventana=%s ancla del POC: centro=%d,%d origen=%d,%d"
          % (args.run, ",".join(str(v) for v in ventana), CENTRO_POC[0], CENTRO_POC[1],
             ORIGEN_POC[0], ORIGEN_POC[1]))
    print("criterios: dir_pixel == dir_log, margen >= %.1f pts, |celda de los pixeles - testigo| <= %.2f celda"
          % (args.margen, args.celda_tolerancia))
    print("libres: el MODO (%s) y el CUADRO — la marca ANIM se lee despues de la captura, no es la de la imagen"
          % ",".join(MODOS))

    lineas = []
    verdes = evaluadas = 0
    refs_por_celda = {}
    evaluables, mascaras = [], {}

    def refs_de(modo, celda):
        """Las referencias del host en ESTA celda, armadas o reusadas. Verifica el ancla antes de usarlas."""
        clave = (modo, celda)
        if clave in refs_por_celda:
            return refs_por_celda[clave]
        carpeta = os.path.join(args.refs, "%s-%s" % (modo, celda.replace(",", "_")))
        censo = ""
        if os.path.exists(os.path.join(carpeta, "censo.txt")):
            censo = open(os.path.join(carpeta, "censo.txt"), errors="replace").read()
        pngs = [n for n in (os.listdir(carpeta) if os.path.isdir(carpeta) else [])
                if n.startswith("pantalla-%s-d" % modo) and "-c" in n]
        if censo and "PNG-DIRS" in censo and len(pngs) >= len(dirs) and \
                os.path.exists(os.path.join(carpeta, "mapa.png")):
            ancla = verificar_ancla(censo, celda)
            print("  REF %-4s celda=%-16s -> %s  (%s, REUSADAS %d PNG)" % (modo, celda, ancla, carpeta, len(pngs)))
        else:
            censo = armar_referencias(celda, modo, carpeta, dirs)
            ancla = verificar_ancla(censo, celda)
            print("  REF %-4s celda=%-16s -> %s  (%s)" % (modo, celda, ancla, carpeta))
        refs_por_celda[clave] = carpeta
        return carpeta

    def clasificar_todos(nombre, celda):
        """Pregunta la DIRECCION en los dos modos y se queda con el mejor cuadro de cada direccion.

        El MODO y el CUADRO del log no son los de la imagen: la marca ANIM se lee ~1,5 s despues de la captura
        (medido: 3,06 s entre capturas con 1,5 s de espera), y caminando son ~2,8 celdas y varios cuadros de
        diferencia. Lo que se pregunta es la direccion; el modo y el cuadro se dejan libres.
        """
        png = [os.path.join(args.run, nombre + ".png")]
        por_dir, recorte = {}, None
        for modo in MODOS:
            carpeta = refs_de(modo, celda)
            _, medido = CLAS.clasificar(carpeta, os.path.join(carpeta, "mapa.png"), modo, ventana, png, dirs)
            r = medido[0]
            if r.get("falla"):
                continue
            recorte = r["recorte"]
            for ident, d in r["ranking"]:
                cuadro = r["ganador"][d][1]
                if d not in por_dir or ident > por_dir[d][0]:
                    por_dir[d] = (ident, modo, cuadro)
        if not por_dir:
            return {"falla": "no se pudo clasificar"}
        ranking = sorted(((v[0], d) for d, v in por_dir.items()), reverse=True)
        mejor, segundo = ranking[0], (ranking[1] if len(ranking) > 1 else (0.0, -1))
        return {"mejor": mejor, "segundo": segundo, "margen": mejor[0] - segundo[0], "ranking": ranking,
                "recorte": recorte or (0, 0), "ganador": por_dir,
                "cuadro_del_mejor": por_dir[mejor[1]][2], "modo_del_mejor": por_dir[mejor[1]][1]}

    for f in filas:
        if f["modo"] is None or f.get("nota") or not f["celda"] or f["dir"] is None:
            lineas.append("SIN-DATOS %-14s %s" % (f["nombre"], f.get("nota") or "sin direccion en la tabla"))
            continue
        evaluadas += 1
        r = clasificar_todos(f["nombre"], f["celda"])
        if r.get("falla"):
            lineas.append("ROJO     %-14s %s" % (f["nombre"], r["falla"]))
            continue

        # la mascara de esta captura (para medir DONDE cae el personaje) se guarda para despues: primero hay
        # que juntar las de TODAS para saber que pixeles son del emulador y no del personaje
        carpeta = refs_de(f["modo"], f["celda"])
        mascara, escala = mascara_de_la_captura(os.path.join(args.run, f["nombre"] + ".png"),
                                                os.path.join(carpeta, "mapa.png"), ventana)
        mascaras[f["nombre"]] = mascara
        evaluables.append((f, r))

    # el artefacto del emulador: lo que NO es el mapa en TODAS las capturas (el personaje se mueve, la franja
    # de texto del emulador no)
    artefacto = None
    for nombre, m in mascaras.items():
        if m is None:
            continue
        artefacto = m.copy() if artefacto is None else (artefacto & m)
    if artefacto is not None:
        print("  artefacto del emulador (en todas las capturas, fuera del mapa): %d px" % int(artefacto.sum()))

    for f, r in evaluables:
        esquina = (tabla_txt[(f["modo"], f["crc32"])][3], tabla_txt[(f["modo"], f["crc32"])][4])
        escala = ventana[3] / float(ANIM.ALTO_LOGICO)
        implicita = celda_implicita(mascaras.get(f["nombre"]), escala, esquina, artefacto)
        cx_t, cy_t = (float(v) for v in f["celda"].split(","))
        if implicita:
            cx_i, cy_i, caja_med, npx = implicita
            desvio = max(abs(cx_i - cx_t), abs(cy_i - cy_t))
        else:
            cx_i = cy_i = float("nan")
            caja_med, npx, desvio = None, 0, float("inf")

        # EL ANCLA SE ITERA (DC-002.64): la marca ANIM se lee ~1,5 s DESPUES de la captura, asi que mientras el
        # personaje camina el ancla del testigo puede estar media celda o mas alla. Si los pixeles dicen que el
        # personaje no esta donde dice el testigo, se arman las referencias en la celda MEDIDA y se vuelve a
        # preguntar la direccion (el mapa del host no depende del ancla: la ventana del nivel esta fija en 28,20).
        ancla = f["celda"]
        if implicita and r["margen"] < args.margen and desvio > ANCLA_TOLERANCIA:
            celda_medida = "%.4f,%.4f" % (cx_i, cy_i)
            r2 = clasificar_todos(f["nombre"], celda_medida)
            # el ancla medida arrastra el sesgo del borde de la mascara (~0,05 celda) y ademas el redondeo del
            # host es a pixel: solo se cambia si el margen MEJORA, y si no, se queda el testigo (el del log)
            if not r2.get("falla") and r2["margen"] > r["margen"]:
                r, ancla = r2, celda_medida

        dir_pixel = r["mejor"][1]
        ok_dir = (dir_pixel == f["dir"])
        ok_margen = (r["margen"] >= args.margen)
        ok_celda = (desvio <= args.celda_tolerancia)
        veredicto = "VERDE" if (ok_dir and ok_margen and ok_celda) else "ROJO"
        if veredicto == "VERDE":
            verdes += 1

        lineas.append(
            "%s %-14s %-4s c%02d  dir_log=%02d dir_pixel=%02d (modo %s cuadro %02d)  identidad=%.2f%% "
            "segundo=%02d:%.2f%% margen=%5.2f  celda_testigo=%s (%s)  celda_pixeles=%.2f,%.2f desvio=%.2f "
            "ancla=%s caja_medida=%s pixeles=%d recorte=%dx%d"
            % (veredicto, f["nombre"], f["modo"], f["cuadro"], f["dir"], dir_pixel, r["modo_del_mejor"],
               r["cuadro_del_mejor"], r["mejor"][0], r["segundo"][1], r["segundo"][0], r["margen"], f["celda"],
               f["testigo"], cx_i, cy_i, desvio, ancla, ("%dx%d" % caja_med) if caja_med else "-", npx,
               r["recorte"][0], r["recorte"][1]))

        if args.ver:
            print("    ranking: " + "  ".join("%02d:%.2f" % (d, i) for i, d in r["ranking"]))

    lineas.append("")
    lineas.append("CANAL-PIXELES 249B %d/%d" % (verdes, evaluadas))
    texto = "\n".join(lineas)
    print()
    print(texto)
    with open(salida, "w") as fh:
        fh.write(texto + "\n")
    print()
    print("corpus en " + salida)
    return 0 if verdes == evaluadas and evaluadas > 0 else 1


if __name__ == "__main__":
    sys.exit(main())
