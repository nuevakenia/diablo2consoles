#!/usr/bin/env python3
"""Verificador de la regla de codegen del port a Dreamcast (DC-002.52).

El defecto que costo ocho intentos en DC-002.50: gccgo para SH-4 miscompila las operaciones sobre una
rebanada de struct ANONIMO. El sintoma no se parece a la causa (la consola se reinicia o tira una
excepcion adentro de `pvr_init_defaults` / `pvr_mem_malloc`, con una direccion del monton en el PC, como
si fuera falta de memoria), asi que no se puede dejar librado a la memoria de quien escriba el codigo.

Lo MEDIDO (evidencia de DC-002.50, seccion 12):
  * `append(lista, struct{...}{...})` -> el binario deja de arrancar el video (dibujando=0, reinicio dentro
    del PVR). Con el mismo codigo y un tipo CON NOMBRE: dibujando=1.
  * el recorte `lista[:1]` sobre la MISMA rebanada anonima se comporta igual de mal (medido en la variante
    /tmp/poc-v7: pasa el init del PVR y muere despues).
  * en cambio el literal `[]struct{...}{...}` recorrido con `range` y SOLO leido esta en el arbol desde
    DC-002.38 y no rompe nada: por eso no se prohibe.

Lo que este verificador persigue, entonces:
  A) `append` de un struct anonimo (en cualquier parte del programa, en una o varias lineas);
  B) recorte (`s[:n]`, `s[n:]`) o asignacion de elemento (`s[3] = ...`) sobre una variable declarada como
     rebanada de struct anonimo.

Uso:
    scripts/console/checar-codegen-252.py [--root DIR] [--verbose]

Sale 0 si no hay violaciones (e imprime cuantos .go miro) y 1 si encuentra alguna, con archivo:linea.
El mutador `mutar-codegen-252.sh` es el que prueba que este control CAZA su regresion.
"""
import argparse
import re
import sys
from pathlib import Path

# donde vive el codigo que se compila para la consola (los .go del host no pasan por gccgo, pero se miran
# igual: el mismo archivo suele compilarse para los dos lados -- nivel.go, personaje.go y monstruo.go son
# literalmente el mismo archivo en el POC y en el verificador)
SUBDIRS = ("poc", "std", "verif-", "sonda-", "motor-")
EXCLUIR = (".git", "vendor", "gox", "testdata")

RE_APPEND_ANON = re.compile(r"\bappend\s*\((?:[^()]|\([^()]*\))*struct\s*\{", re.S)
RE_DECL_ANON = re.compile(r"\b(\w+)\s*(?::=|=|\s)\s*\[\]struct\s*\{")
RE_VAR_ANON = re.compile(r"\bvar\s+(\w+)\s+\[\]struct\s*\{")


def archivos(raiz: Path):
    for p in sorted(raiz.rglob("*.go")):
        texto = str(p)
        if any(x in texto for x in EXCLUIR):
            continue
        if not any(f"/{s}" in texto for s in SUBDIRS):
            continue
        yield p


def linea_de(texto: str, pos: int) -> int:
    return texto.count("\n", 0, pos) + 1


def revisar(p: Path, verbose: bool):
    texto = p.read_text(errors="replace")
    violaciones = []

    for m in RE_APPEND_ANON.finditer(texto):
        violaciones.append((linea_de(texto, m.start()), "append de struct ANONIMO (usa un tipo con nombre)"))

    nombres = set(m.group(1) for m in RE_DECL_ANON.finditer(texto))
    nombres |= set(m.group(1) for m in RE_VAR_ANON.finditer(texto))

    for nombre in sorted(nombres):
        # recorte: nombre[...:...]
        for m in re.finditer(r"\b%s\s*\[([^\]]*:[^\]]*)\]" % re.escape(nombre), texto):
            violaciones.append((linea_de(texto, m.start()),
                                "recorte de rebanada de struct ANONIMO (%s[%s])" % (nombre, m.group(1))))
        # asignacion de un elemento: nombre[3] = ...
        for m in re.finditer(r"\b%s\s*\[[^\]]+\]\s*=[^=]" % re.escape(nombre), texto):
            violaciones.append((linea_de(texto, m.start()),
                                "asignacion de elemento en rebanada de struct ANONIMO (%s)" % nombre))

    if verbose and nombres:
        for n in sorted(nombres):
            print("      (declara []struct{ anonimo: %s -- solo se permite recorrerlo y leerlo)" % n)

    violaciones.sort()
    return violaciones


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", default=None, help="raiz del arbol (por defecto, el repo de este script)")
    ap.add_argument("--verbose", action="store_true")
    args = ap.parse_args()

    raiz = Path(args.root) if args.root else Path(__file__).resolve().parents[2] / "console" / "dc"
    raiz = raiz.resolve()
    if not raiz.is_dir():
        print("FALLO: no existe %s" % raiz)
        return 2

    vistos = 0
    total = []
    for p in archivos(raiz):
        vistos += 1
        for linea, que in revisar(p, args.verbose):
            total.append((p, linea, que))

    print("== DC-002.52 regla de codegen: rebanadas de struct anonimo ==")
    print("   raiz: %s" % raiz)
    print("   .go mirados: %d" % vistos)

    if not total:
        print("   VIOLACIONES: 0 -> el arbol cumple (ningun append ni recorte sobre un struct anonimo)")
        return 0

    print("   VIOLACIONES: %d" % len(total))
    for p, linea, que in total:
        print("   %s:%d  %s" % (p, linea, que))
    return 1


if __name__ == "__main__":
    sys.exit(main())
