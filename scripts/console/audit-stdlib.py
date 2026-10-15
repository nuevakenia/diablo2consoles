#!/usr/bin/env python3
"""Mide que le falta al motor para compilar para Dreamcast: la biblioteca estandar de Go.

libgodc no trae stdlib (sus ejemplos solo importan "kos" y "unsafe"), asi que cada paquete
estandar que use el motor es trabajo a mano. Esto cuenta ese trabajo.

Uso:  scripts/console/audit-stdlib.py [raiz_del_repo]

Salida:
  - paquetes estandar que el motor usa DIRECTO, con cuantos archivos los importan y cuanta
    superficie de su API se toca (proxy: cuantos "pkg.Simbolo" distintos aparecen)
  - paquetes estandar TRANSITIVOS (los que entran por terceros: ebiten, etc.)
  - los paquetes del proyecto que NO se pueden traer (los que dependen de Ebitengine)
"""
import os
import re
import subprocess
import sys
from collections import defaultdict

# paquetes de terceros: se reemplazan por el backend de consola, no se portean
TERCEROS = ("github.com/", "golang.org/", "ebiten.org/", "go.uber.org/", "gopkg.in/")


def es_stdlib(path):
    return "." not in path.split("/")[0]


def main():
    raiz = sys.argv[1] if len(sys.argv) > 1 else "."
    os.chdir(raiz)

    archivos_por_paquete = defaultdict(int)
    usos_por_paquete = defaultdict(set)
    paquetes_proyecto = set()
    archivos = []

    for base, dirs, files in os.walk("."):
        dirs[:] = [d for d in dirs if d not in (".git", "node_modules")]
        for f in files:
            if f.endswith(".go"):
                archivos.append(os.path.join(base, f))

    imp_re = re.compile(r'^\s*(?:[\w\.]+\s+)?"([^"]+)"', re.M)
    bloque_re = re.compile(r"import\s*\((.*?)\)", re.S)

    for path in archivos:
        # los POC de consola importan "kos" (no es stdlib ni tercero: es el runtime de la DC)
        if path.startswith("./console/"):
            continue
        # los _test.go no entran en el binario de la consola: no cuentan para el porteo
        if path.endswith("_test.go"):
            continue
        try:
            src = open(path, encoding="utf-8", errors="replace").read()
        except OSError:
            continue

        imports = set()
        for m in bloque_re.finditer(src):
            imports.update(imp_re.findall(m.group(1)))
        for m in re.finditer(r'^import\s+"([^"]+)"', src, re.M):
            imports.add(m.group(1))

        for imp in imports:
            if imp.startswith("github.com/OpenDiablo2/") or imp.startswith("kos"):
                paquetes_proyecto.add(imp)
                continue
            if not es_stdlib(imp):
                continue
            nombre = imp.split("/")[-1]
            archivos_por_paquete[imp] += 1
            # proxy de superficie: cuantos simbolos distintos se usan como pkg.Simbolo
            usos_por_paquete[imp].update(re.findall(r"\b%s\.([A-Z]\w*)" % re.escape(nombre), src))

    total_simbolos = sum(len(v) for v in usos_por_paquete.values())

    print(f"archivos .go del motor: {len(archivos)}")
    print(f"paquetes estandar usados directo: {len(archivos_por_paquete)}")
    print(f"simbolos estandar distintos tocados (proxy de superficie): {total_simbolos}")
    print()
    print(f"{'paquete':<28} {'archivos':>8} {'simbolos':>9}  simbolos")
    print("-" * 100)
    for p, n in sorted(archivos_por_paquete.items(), key=lambda kv: (-kv[1], kv[0])):
        sims = sorted(usos_por_paquete[p])
        muestra = ", ".join(sims[:6]) + (" ..." if len(sims) > 6 else "")
        print(f"{p:<28} {n:>8} {len(sims):>9}  {muestra}")

    print()
    print("== paquetes estandar transitivos (entran por terceros: no cuentan para el porteo,")
    print("== pero dicen que trae Ebitengine que el backend de consola no va a necesitar) ==")
    out = subprocess.run(["go", "list", "-deps", "."], capture_output=True, text=True)
    if out.returncode == 0:
        props = {l.strip() for l in out.stdout.splitlines() if l.strip()}
        trans = sorted(p for p in props if es_stdlib(p) and p not in archivos_por_paquete)
        print(f"({len(trans)} paquetes)")
        print(", ".join(trans))
    else:
        print("go list fallo:", out.stderr.strip()[:300])


if __name__ == "__main__":
    main()
