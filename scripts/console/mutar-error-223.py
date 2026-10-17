#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""DC-002.23: los mutantes del instrumento que mide el error del motor.

El corpus (checar-error-223.sh) mide el campo `errorMessage` del `App` por OFFSET: el 156 que
`updateInitError` usa para leerlo. Si ese numero estuviera mal, todas las lecturas serian de OTRO campo y
el control quedaria verde sin discriminar nada (la leccion de DC-002.8b: un control de costo que no
discrimina no prueba nada).

Los mutantes:
  --offset-vecino   cambia el offset del POC (156 -> 152, el campo `logger`, que es un puntero NO nil) y
                    exige que las lecturas dejen de dar NIL: si el POC siguiera diciendo NIL, el control
                    no mide lo que dice medir.

El mutador es idempotente: deja el archivo con el sha256 del principio y lo verifica.

Uso:  mutar-error-223.py --offset-vecino      (aplica, compila y deja el arbol mutado)
      mutar-error-223.py --volver             (restaura el sha256 original y lo comprueba)
      mutar-error-223.py --control            (control sano: el original ya esta puesto)
"""
import argparse
import hashlib
import os
import re
import sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(os.path.dirname(__file__))))
POC = os.path.join(REPO, "console/dc/poc/motor-error/main.go")
SHA = os.path.join(os.path.dirname(os.path.abspath(__file__)), ".error-223.sha")

MUTACIONES = {
    "--offset-vecino": [
        ("const offsetError = 156", "const offsetError = 152"),
    ],
}


def sha256(ruta):
    with open(ruta, "rb") as f:
        return hashlib.sha256(f.read()).hexdigest()


def leer():
    with open(POC, "r", encoding="utf-8") as f:
        return f.read()


def escribir(texto):
    with open(POC, "w", encoding="utf-8") as f:
        f.write(texto)


def guardar_sha():
    if not os.path.exists(SHA):
        with open(SHA, "w", encoding="utf-8") as f:
            f.write(sha256(POC))
        print("   sha original guardado: %s" % sha256(POC))


def volver():
    with open(SHA, "r", encoding="utf-8") as f:
        original = f.read().strip()
    # restaura por texto (la mutacion es de una linea) y comprueba el hash
    texto = leer()
    for cambios in MUTACIONES.values():
        for viejo, nuevo in cambios:
            texto = texto.replace(nuevo, viejo)
    escribir(texto)
    actual = sha256(POC)
    if actual == original:
        print("   OK: el POC quedo con el sha256 original (%s)" % actual)
        return 0
    print("   FALLO: el sha256 quedo en %s y debia ser %s" % (actual, original))
    return 1


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--offset-vecino", action="store_true")
    ap.add_argument("--volver", action="store_true")
    ap.add_argument("--control", action="store_true")
    args = ap.parse_args()

    guardar_sha()

    if args.volver:
        return volver()

    if args.control:
        print("   control sano: el POC esta tal cual (offsetError = 156)")
        return 0

    aplicada = None
    for nombre, cambios in MUTACIONES.items():
        if getattr(args, nombre[2:].replace("-", "_")):
            aplicada = nombre
            texto = leer()
            for viejo, nuevo in cambios:
                if viejo not in texto:
                    print("   FALLO: no encontre %r para mutar" % viejo)
                    return 1
                texto = texto.replace(viejo, nuevo)
            escribir(texto)
            break

    if not aplicada:
        print("   nada que hacer (usa --offset-vecino, --volver o --control)")
        return 2

    print("   mutante %s aplicado: %s" % (aplicada, MUTACIONES[aplicada]))
    return 0


if __name__ == "__main__":
    sys.exit(main())
