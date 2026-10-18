#!/usr/bin/env python3
"""HUELLA DE CAMADA DE UN DIR DE .gox (DC-002.29).

POR QUE: el `.gox` de gccgo NO es autocontenido. Su seccion `.go_export` lleva los CUERPOS INLINEADOS de las
funciones que importo, y esos cuerpos van con la RUTA DEL ARCHIVO FUENTE con que se compilo aquel paquete.
O sea: cada `.gox` guarda la huella de EN QUE DIRECTORIO se construyo la stdlib contra la que se lo compilo.
Medido (d2mpq recompilado contra la camada de DC-002.27): su export lleva
    //  /tmp/gox-00220/src/strings/strings.go:396
mientras el mismo paquete recompilado hoy lleva
    //  /tmp/gox-229/src/strings/strings.go:396
(el mismo fuente, el mismo numero de linea: lo unico distinto es la CAMADA).

Uso:  python3 scripts/console/huella-camada-gox.py <dir_gox> [...]

Imprime, por camada, cuantas referencias `/.../src/<algo>.go` tiene cada `.gox` y cual es el directorio raiz
dominante. Un dir de .gox con paquetes de DOS huellas distintas esta MEZCLADO: es el estado que mata el
arranque del motor (medido en DC-002.29).
"""
import collections
import os
import re
import subprocess
import sys

RX = re.compile(rb"(/[A-Za-z0-9_./+-]*?)/(?:src|gox)/")


def export_de(f):
    """(offset, tamano) de .go_export segun readelf -S -W"""
    out = subprocess.run(["sh-elf-readelf", "-S", "-W", f], capture_output=True, text=True).stdout
    for l in out.splitlines():
        m = re.match(r"\s*\[\s*\d+\]\s+(\S+)\s+(\S+)\s+([0-9a-f]+)\s+([0-9a-f]+)\s+([0-9a-f]+)", l)
        if m and m.group(1) == ".go_export":
            return int(m.group(4), 16), int(m.group(5), 16)
    return None, None


def huella(f):
    off, tam = export_de(f)
    if off is None:
        return None, 0, 0
    datos = open(f, "rb").read()[off:off + tam]
    raices = collections.Counter(m.group(1).decode() for m in RX.finditer(datos))
    if not raices:
        return None, 0, len(datos)
    return raices.most_common(1)[0][0], sum(raices.values()), len(datos)


def camada(d):
    print("== camada %s" % d)
    res = {}
    for base, _, archivos in os.walk(d):
        for a in sorted(archivos):
            if not a.endswith(".gox"):
                continue
            f = os.path.join(base, a)
            rel = os.path.relpath(f, d)
            h, n, tam = huella(f)
            res[rel] = (h, n, tam)
    cuenta = collections.Counter(v[0] for v in res.values() if v[0])
    for h, n in cuenta.most_common():
        print("   huella %-28s en %d paquetes (de %d con referencias)" % (h, n, sum(cuenta.values())))
    conref = {k: v for k, v in res.items() if v[0]}
    if cuenta:
        dom = cuenta.most_common(1)[0][0]
        otros = sorted((k, v) for k, v in conref.items() if v[0] != dom)
        print("   paquetes cuya huella NO es la dominante (%s): %d" % (dom, len(otros)))
        for k, v in otros:
            print("      %-72s %s (%d refs)" % (k, v[0], v[1]))
    for clave in ("github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq.gox",
                  "github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/mpq.gox",
                  "github.com/OpenDiablo2/OpenDiablo2/d2app.gox"):
        if clave in res:
            h, n, tam = res[clave]
            print("   [clave] %-60s huella=%s refs=%d export=%d" % (clave.split('/')[-1], h, n, tam))
    return res


if __name__ == "__main__":
    for d in sys.argv[1:]:
        camada(d)
        print()
