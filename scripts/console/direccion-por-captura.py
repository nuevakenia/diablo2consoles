#!/usr/bin/env python3
"""DC-002.49b / DC-002.64: de que direccion es CADA captura de la corrida del giro, y con que celda se arman
las referencias del host.

Lee la corrida (flycast.log + geo.txt) y la tabla del host (crc249b.txt) y, por captura, dice:
 * el modo y el cuadro que imprimio el POC en ese momento (linea HITO249: ANIM),
 * la direccion del COF que le corresponde segun la tabla del host (canal del log),
 * la CELDA EXACTA del personaje en ese momento y que testigo del log la dice,
 * la caja del cuadro (para cruzarla con la del host).

LAS DOS LECCIONES DE DC-002.64 (las dos medidas, ver evidencia dc-002.64-pixeles-direccion.txt)
---------------------------------------------------------------------------------------------
1. LA CELDA VA CON DECIMALES. El POC mueve el personaje en SUBTILES y la marca imprime
   `celda 29.3152 18.6848`. Recortarla a `29,18` corre el ancla (dx-dy)*80 en x y (dx+dy)*40 en y: medio
   subtile de error son ~30 px en x y ~40 px en y, o sea el personaje entero fuera de su caja. Con la celda
   recortada TODAS las direcciones caen a ~70-74 % de identidad y el margen se vuelve ruido (medido: c02 daba
   dir 07 74,38 % margen 0,78). Con la celda exacta la MISMA captura da su direccion con margen 8 pts.
2. EL TESTIGO DE POSICION NO ES SOLO `HITO238: POS`. El POC imprime POS cada 30 cuadros (a ~10 cuadros/s:
   cada ~3 s) y ademas `HITO238: CONTROL` en cada cambio de botones/direccion. El ultimo testigo anterior a la
   marca ANIM puede ser un CONTROL, y cuando el personaje acaba de girar o de volver, el POS anterior ya esta
   viejo (medido en c07: el POS decia 26.8319,18.8054 y el CONTROL dos lineas antes del ANIM decia
   28.0199,19.9934; los pixeles de la captura dicen 28.09,20.04 -> manda el CONTROL).

Uso:
  direccion-por-captura.py <corrida> <tabla-crc249b.txt> [--silencioso]

Las lineas `CANAL249B ...` de la salida son la entrada de `canal-pixeles-249b.py`.
"""
import re
import sys
from pathlib import Path

RE_ANIM = re.compile(r"HITO249: ANIM modo (\S+) cuadro (\d+) de (\d+) caja (\d+) (\d+) esquina (-?\d+) (-?\d+) "
                     r"pixeles (\d+) crc32 (\d+)")
RE_POS = re.compile(r"HITO238: POS cuadro (\d+) ms (\d+) .*?celda ([-0-9.]+) ([-0-9.]+) tile ([-0-9]+) ([-0-9]+)")
RE_CONTROL = re.compile(r"HITO238: CONTROL botones (-?\d+) ejes (-?[0-9.]+) (-?[0-9.]+) celda ([-0-9.]+) ([-0-9.]+) "
                        r"cuadro (\d+)")
RE_CRC = re.compile(r"CRC249B: (\S+) dir=(\d+) cuadro=(\d+) caja=(\d+x\d+) esquina=(-?\d+),(-?\d+) "
                    r"pixeles=(\d+) crc32=(\d+)")
RE_GEO = re.compile(r"GEO captura=(\S+) cliente=[0-9,]*(?: modo=(\S+) cuadro=(\d+).*?caja=(\d+x\d+) "
                    r"crc32=(\d+))?")


def leer_tabla(ruta):
    """(modo, crc32) -> (dir, cuadro, 'cajaWxH', izq, arriba) de la tabla CRC249B del host."""
    tabla = {}
    for l in Path(ruta).read_text(errors="replace").splitlines():
        m = RE_CRC.search(l)
        if m:
            tabla[(m.group(1), int(m.group(8)))] = (int(m.group(2)), int(m.group(3)), m.group(4),
                                                   int(m.group(5)), int(m.group(6)))
    return tabla


def capturas_de_la_corrida(corrida, tabla):
    """Una fila por captura, con la celda EXACTA y el testigo que la dice (POS o CONTROL, el ultimo)."""
    anim, testigos = [], []
    for n, l in enumerate((Path(corrida) / "flycast.log").read_text(errors="replace").splitlines(), 1):
        if (m := RE_ANIM.search(l)):
            anim.append((n, m.group(1), int(m.group(2)), (int(m.group(4)), int(m.group(5))), int(m.group(9))))
        elif (m := RE_CONTROL.search(l)):
            testigos.append((n, "CONTROL", m.group(4), m.group(5)))
        elif (m := RE_POS.search(l)):
            testigos.append((n, "POS", m.group(3), m.group(4)))
    testigos.sort()

    geo = []
    ruta_geo = Path(corrida) / "geo.txt"
    if ruta_geo.exists():
        for l in ruta_geo.read_text(errors="replace").splitlines():
            m = RE_GEO.search(l)
            if m:
                geo.append((m.group(1), m.group(2), int(m.group(3) or -1), m.group(4), int(m.group(5) or -1)))

    filas, cursor = [], 0
    for nombre, modo, cuadro, caja_geo, crc in geo:
        if modo is None:
            filas.append({"nombre": nombre, "modo": None, "nota": "la captura no cayo sobre una marca ANIM"})
            continue
        i = cursor
        while i < len(anim) and not (anim[i][1] == modo and anim[i][2] == cuadro and anim[i][4] == crc):
            i += 1
        hallado = None
        if i < len(anim):
            # el mismo cuadro sigue en pantalla mientras el reloj no lo cambie: la captura cae en la ULTIMA
            # de esas lineas seguidas, no en la primera.
            j = i
            while (j + 1 < len(anim) and anim[j + 1][1] == modo and anim[j + 1][2] == cuadro
                   and anim[j + 1][4] == crc):
                j += 1
            hallado = anim[j]
        if hallado is None:
            for k in range(cursor, len(anim)):
                if anim[k][1] == modo and anim[k][4] == crc:
                    hallado = anim[k]
                    break
        if hallado is None:
            # el crc32 que quedo en geo.txt puede venir CORTADO (medido en DC-002.64: la corrida lee el log
            # mientras la consola escribe y se llevo "crc32=5718" de un 57183000). Si el numero de geo es
            # PREFIJO del crc32 del log y coinciden modo, cuadro y caja, es la misma marca.
            for k in range(cursor, len(anim)):
                if (anim[k][1] == modo and anim[k][2] == cuadro
                        and "%dx%d" % anim[k][3] == caja_geo
                        and str(anim[k][4]).startswith(str(crc))):
                    hallado = anim[k]
                    break
        if hallado is None:
            filas.append({"nombre": nombre, "modo": modo, "nota": "NO ENCONTRADA en el log"})
            continue
        cursor = anim.index(hallado)
        linea = hallado[0]
        crc_real = hallado[4]
        previos = [t for t in testigos if t[0] < linea]
        testigo, celda_x, celda_y = (previos[-1][1], previos[-1][2], previos[-1][3]) if previos else ("-", None, None)
        d = tabla.get((modo, crc_real)) or tabla.get((modo, crc))
        filas.append({
            "nombre": nombre, "modo": modo, "cuadro": cuadro, "caja": "%dx%d" % hallado[3],
            "caja_geo": caja_geo, "crc32": crc_real, "dir": d[0] if d else None,
            "caja_host": d[2] if d else None, "celda": ("%s,%s" % (celda_x, celda_y)) if celda_x else None,
            "testigo": testigo, "linea_testigo": previos[-1][0] if previos else None, "linea_anim": linea,
        })
    return filas


def main():
    if len(sys.argv) < 3:
        raise SystemExit("uso: direccion-por-captura.py <corrida> <tabla-crc249b.txt> [--silencioso]")

    corrida, tabla_txt = sys.argv[1], sys.argv[2]
    silencioso = "--silencioso" in sys.argv[3:]
    filas = capturas_de_la_corrida(corrida, leer_tabla(tabla_txt))

    if not silencioso:
        print("%-14s %-3s %-5s %-6s %-7s %-8s %-22s %-8s %s"
              % ("captura", "modo", "cuadro", "caja", "dir", "testigo", "celda exacta", "cajaHost", "veredicto"))
        for f in filas:
            if f["modo"] is None or f.get("nota"):
                print("%-14s %s" % (f["nombre"], f.get("nota") or "sin ANIM"))
                continue
            dir_txt = "%02d" % f["dir"] if f["dir"] is not None else "?"
            veredicto = "caja del POC %s == la del host %s" % (f["caja"], f["caja_host"] or "-")
            if f["caja_host"] and f["caja"] != f["caja_host"]:
                veredicto = "OJO la caja del POC %s NO es la del host %s" % (f["caja"], f["caja_host"])
            print("%-14s %-3s %-5d %-6s %-7s %-8s %-22s %-8s %s"
                  % (f["nombre"], f["modo"], f["cuadro"], f["caja"], dir_txt,
                     f["testigo"], f["celda"] or "?", f["caja_host"] or "-", veredicto))
        print()

    # la forma que consume canal-pixeles-249b.py
    for f in filas:
        if f["modo"] is None or f.get("nota") or not f["celda"]:
            print("CANAL249B captura=%s SIN-DATOS %s" % (f["nombre"], f.get("nota") or "sin marca ANIM"))
            continue
        print("CANAL249B captura=%s modo=%s cuadro=%d caja=%s crc32=%d dir=%s celda=%s testigo=%s"
              % (f["nombre"], f["modo"], f["cuadro"], f["caja"], f["crc32"],
                 "%02d" % f["dir"] if f["dir"] is not None else "?", f["celda"], f["testigo"]))
    return 0


if __name__ == "__main__":
    sys.exit(main())
