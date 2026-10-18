#!/usr/bin/env python3
"""CIERRE DE DEPENDIENTES DE UN PAQUETE DEL MOTOR (DC-002.29).

PARA QUE: si un `.gox` recompilado solo queda consistente con la camada cuando TODOS los que lo importan se
recompilan tambien (porque el `.gox` lleva adentro los cuerpos inlineados y los numeros de tipo de sus
imports, medido en DC-002.29), el rearme tiene que rehacer el CIERRE, no un paquete solo. Esto lo cuenta:
descubre los paquetes del motor que importan (directa o transitivamente) al paquete pedido.

Uso: python3 scripts/console/cierre-dependientes.py <paquete> [<paquete> ...]
     (p.ej. d2common/d2fileformats/d2mpq) -- imprime el cierre en orden de capas (los de adentro primero).
"""
import os
import re
import subprocess
import sys

REPO = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
PREFIJO = "github.com/OpenDiablo2/OpenDiablo2/"

RX_IMP = re.compile(r'"' + re.escape(PREFIJO) + r'([^"]+)"')


def paquetes_del_motor():
    """los paquetes que build-motor-all.sh compila: los .go del motor, menos vendor/poc/std/test"""
    out = subprocess.run(["bash", "-c",
                          "cd %s && find . -name '*.go' -not -name '*_test.go' | "
                          "grep -vE '^./vendor|^./console|^./testdata' | xargs -n1 dirname | sort -u" % REPO],
                         capture_output=True, text=True).stdout.split()
    return sorted(p[2:] for p in out if p.startswith("./"))


def imports(p):
    d = os.path.join(REPO, p)
    if not os.path.isdir(d):
        return set()
    s = set()
    for a in os.listdir(d):
        if not a.endswith(".go") or a.endswith("_test.go"):
            continue
        try:
            texto = open(os.path.join(d, a), encoding="utf-8", errors="replace").read()
        except OSError:
            continue
        for m in RX_IMP.finditer(texto):
            s.add(m.group(1))
    return s


def main():
    modo_capas = "--capas" in sys.argv
    objetivo = set(a for a in sys.argv[1:] if not a.startswith("--"))
    if not objetivo:
        sys.exit("uso: cierre-dependientes.py <paquete> [...] [--capas]")
    todos = paquetes_del_motor()
    imp = {p: imports(p) for p in todos}
    # cierre hacia atras: quien importa a un paquete del cierre (transitivo)
    cierre = set()
    capa = 1
    actual = set(objetivo)
    while actual:
        nuevos = set()
        for p in todos:
            if p in cierre or p in objetivo:
                continue
            if imp[p] & actual:
                nuevos.add(p)
        if not nuevos:
            break
        if not modo_capas:
            print("capa %d (%d paquetes): %s" % (capa, len(nuevos), " ".join(sorted(nuevos))))
        else:
            print(" ".join(sorted(nuevos)))
        cierre |= nuevos
        actual = nuevos
        capa += 1
    if modo_capas:
        return
    print("\nTOTAL: %d de %d paquetes del motor dependen (transitivamente) de %s"
          % (len(cierre), len(todos), ",".join(sorted(objetivo))))
    print("orden de compilacion (capas de adentro hacia afuera): el de la lista de capas")
    print("sin dependientes (hojas): %d" % len([p for p in todos if p not in cierre and p not in objetivo
                                                and not any(p in imp[q] for q in todos)]))


if __name__ == "__main__":
    main()
