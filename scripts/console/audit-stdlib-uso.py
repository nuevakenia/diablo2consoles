#!/usr/bin/env python3
"""Que parte de la biblioteca estandar usa REALMENTE el motor.

No sirve implementar fmt entero: sirve implementar lo que el motor llama. Esto mide dos cosas:

  1. las funciones de fmt/strings/strconv/math/errors que el motor usa, con cuanta frecuencia
  2. los VERBOS de formato (%d, %s, %v ...) que aparecen en los textos de formato literales
     del motor, porque cada verbo es codigo que hay que escribir en el fmt de la consola

Los formatos NO literales (fmt.Sprintf(variable, ...)) quedan contados aparte: esos no se pueden
verificar de antemano, hay que correrlos.

Uso:  scripts/console/audit-stdlib-uso.py [raiz_del_repo]
"""
import os
import re
import sys
from collections import Counter

PAQUETES = ("fmt", "strings", "strconv", "math", "errors", "sort", "bytes",
            "unicode/utf8", "encoding/binary", "encoding/hex", "time", "os",
            "path/filepath", "io", "io/ioutil", "bufio", "log", "sync", "container/heap")

# %[flags][ancho][.precision][argumento]$[verbo]
VERBO = re.compile(r"%(?:\[[0-9]+\])?[-+# 0]*(?:[0-9]+|\*)?(?:\.[0-9]+|\.[*])?[a-zA-Z%]")
LITERAL = re.compile(r'^[`"](.*?)[`"]$', re.S)


def main():
    raiz = sys.argv[1] if len(sys.argv) > 1 else "."
    os.chdir(raiz)

    llamadas = Counter()          # fmt.Sprintf -> 344
    verbos = Counter()            # %d -> 512
    anchos = Counter()            # %-20s, %.2f, %08d ...
    no_literales = Counter()      # fmt.Sprintf(x, ...) : no se puede verificar de antemano
    ejemplos = {}

    archivos = []
    for base, dirs, files in os.walk("."):
        dirs[:] = [d for d in dirs if d not in (".git", "node_modules")]
        for f in files:
            if f.endswith(".go") and not f.endswith("_test.go"):
                p = os.path.join(base, f)
                if not p.startswith("./console/"):
                    archivos.append(p)

    llamada_re = re.compile(r"\b(%s)\.([A-Z]\w*)\(" % "|".join(re.escape(p.split("/")[-1])
                                                              for p in PAQUETES))
    # primer argumento de la llamada, si es un literal
    fmt_literal = re.compile(r"""\bfmt\.(Sprintf|Sprint|Sprintln|Printf|Println|Errorf|
                                   Print|Fprintf|Fprintln|Fprint)\(\s*([`"])(.*?)\2""",
                            re.X | re.S)

    for path in archivos:
        try:
            src = open(path, encoding="utf-8", errors="replace").read()
        except OSError:
            continue

        for imp, fn in llamada_re.findall(src):
            llamadas["%s.%s" % (imp, fn)] += 1

        for fn, _q, texto in fmt_literal.findall(src):
            if fn not in ("Sprintf", "Sprint", "Sprintln", "Printf", "Errorf", "Fprintf"):
                continue
            for v in VERBO.findall(texto):
                verbos[v[-1]] += 1
                if v != "%" + v[-1]:
                    anchos[v] += 1
                ejemplos.setdefault("%" + v[-1], (texto.strip()[:60], path))

        # llamadas con formato no literal: fmt.Sprintf(x) / fmt.Println(x) ...
        for fn in re.findall(r"\bfmt\.(?:Sprintf|Printf|Errorf|Fprintf)\(\s*[a-zA-Z_`(]", src):
            no_literales[fn] += 1

    print("== funciones mas usadas del motor (top 30) ==")
    for k, n in llamadas.most_common(30):
        print(f"  {n:>6}  {k}")

    print()
    print("== verbos de formato en textos literales ==")
    total = sum(verbos.values())
    print(f"  ({total} verbos en total)")
    for v, n in verbos.most_common():
        ej = ejemplos.get("%" + v, ("", ""))
        print(f"  {n:>6}  %{v}   ej: {ej[0]!r}")

    print()
    print("== modificadores de ancho/precision (los que cuestan codigo) ==")
    for v, n in anchos.most_common(20):
        print(f"  {n:>6}  {v}")

    print()
    print("== llamadas con formato NO literal (no se pueden verificar de antemano) ==")
    for k, n in no_literales.most_common():
        print(f"  {n:>6}  fmt.{k}(variable, ...)")


if __name__ == "__main__":
    main()
