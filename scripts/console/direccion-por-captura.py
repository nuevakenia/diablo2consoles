#!/usr/bin/env python3
"""DC-002.49b: de que direccion es CADA captura de la corrida del giro, y la celda para armar las referencias.

Lee la corrida (flycast.log + geo.txt) y la tabla del host (crc249b.txt) y, por captura, dice:
 * el modo y el cuadro que imprimio el POC en ese momento (linea HITO249: ANIM),
 * la direccion del COF que le corresponde segun la tabla del host (canal del log),
 * la celda del personaje en ese momento (la ultima marca HITO238: POS anterior en el log),
 * que es lo que necesita `verif-personaje-238 -pos <celda> -png-dirs TN` para armar la referencia
   con la que el canal de PIXELES clasifica la captura.

Uso: direccion-por-captura.py <corrida> <tabla-crc249b.txt>
"""
import re
import sys
from pathlib import Path

RE_ANIM = re.compile(r"HITO249: ANIM modo (\S+) cuadro (\d+) de (\d+) caja (\d+) (\d+) esquina (-?\d+) (-?\d+) "
                     r"pixeles (\d+) crc32 (\d+)")
RE_POS = re.compile(r"HITO238: POS cuadro (\d+) ms (\d+) .*? celda ([-0-9.]+) ([-0-9.]+) tile (\d+) (\d+)")
RE_CRC = re.compile(r"CRC249B: (\S+) dir=(\d+) cuadro=(\d+) caja=(\d+x\d+) esquina=(-?\d+),(-?\d+) "
                    r"pixeles=(\d+) crc32=(\d+)")
RE_GEO = re.compile(r"GEO captura=(\S+) cliente=[0-9,]*(?: modo=(\S+) cuadro=(\d+).*?caja=(\d+x\d+) "
                    r"crc32=(\d+))?")


def main():
    corrida = Path(sys.argv[1])
    tabla = {}
    for l in Path(sys.argv[2]).read_text(errors="replace").splitlines():
        m = RE_CRC.search(l)
        if m:
            tabla[(m.group(1), int(m.group(8)))] = (int(m.group(2)), int(m.group(3)), m.group(4))

    anim, pos = [], []
    for n, l in enumerate((corrida / "flycast.log").read_text(errors="replace").splitlines(), 1):
        if (m := RE_ANIM.search(l)):
            anim.append((n, m.group(1), int(m.group(2)), (int(m.group(4)), int(m.group(5))), int(m.group(9))))
        elif (m := RE_POS.search(l)):
            pos.append((n, int(m.group(5)), int(m.group(6))))

    geo = []
    ruta_geo = corrida / "geo.txt"
    if ruta_geo.exists():
        for l in ruta_geo.read_text(errors="replace").splitlines():
            m = RE_GEO.search(l)
            if m:
                geo.append((m.group(1), m.group(2), int(m.group(3) or -1), int(m.group(5) or -1)))

    cursor, salida = 0, []
    print("%-14s %-3s %-5s %-6s %-7s %-9s %s" % ("captura", "modo", "cuadro", "caja", "dir", "celda", "veredicto"))
    for nombre, modo, cuadro, crc in geo:
        if modo is None:
            print("%-14s (la captura no cayo sobre una marca ANIM del personaje)" % nombre)
            continue
        hallado = None
        i = cursor
        while i < len(anim) and not (anim[i][1] == modo and anim[i][2] == cuadro and anim[i][4] == crc):
            i += 1
        if i < len(anim):
            # el mismo cuadro sigue en pantalla mientras el reloj no lo cambie: la captura cae en la ULTIMA
            # de esas lineas seguidas, no en la primera.
            j = i
            while (j + 1 < len(anim) and anim[j + 1][1] == modo and anim[j + 1][2] == cuadro
                   and anim[j + 1][4] == crc):
                j += 1
            hallado = (j, anim[j][3])
        if hallado is None:
            for i in range(cursor, len(anim)):
                n, m_modo, m_cuadro, caja, m_crc = anim[i]
                if m_modo == modo and m_crc == crc:
                    hallado = (i, caja)

                    break
        if hallado is None:
            print("%-14s %-3s %-5d %-6s %-7s %-9s NO ENCONTRADA en el log" % (nombre, modo, cuadro, "?", "?", "?"))
            continue
        idx, caja = hallado
        cursor = idx
        linea = anim[idx][0]
        prev = [p for p in pos if p[0] < linea]
        celda = "%d,%d" % (prev[-1][1], prev[-1][2]) if prev else "?"
        d = tabla.get((modo, crc))
        dir_txt = "%02d" % d[0] if d else "?"
        caja_host = d[2] if d else "-"
        veredicto = "caja del POC %dx%d == la del host %s" % (caja[0], caja[1], caja_host)
        if d and "%dx%d" % (caja[0], caja[1]) != caja_host:
            veredicto = "OJO la caja del POC %dx%d NO es la del host %s" % (caja[0], caja[1], caja_host)
        print("%-14s %-3s %-5d %-6s %-7s %-9s %s" % (nombre, modo, cuadro, "%dx%d" % caja, dir_txt, celda, veredicto))
        salida.append((nombre, modo, dir_txt, celda))

    print()
    print("== las capturas que sirven para el canal de pixeles (dir, celda)")
    for nombre, modo, d, celda in salida:
        print("   %-14s modo=%s dir=%s celda=%s" % (nombre, modo, d, celda))
    return 0


if __name__ == "__main__":
    sys.exit(main())
