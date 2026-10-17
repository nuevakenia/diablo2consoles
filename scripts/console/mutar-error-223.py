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
                    no mide lo que dice medir. (DC-002.23)
  --sq-roto         (DC-002.24) le devuelve a `gc_sq_copy` la formula VIEJA del area de la Store Queue
                    (`>> 26`, la que escribe siempre en la BIOS: se descarta) y rehace libgodc.a. Con el
                    defecto puesto el arranque tiene que volver al rojo de DC-002.23 (0 cuadros completos
                    y `Unhandled exception: PC 0000000b`): si el arranque siguiera andando, el arreglo no
                    seria el que arregla. Se deshace con --volver (que rehace la cadena y el arreglo).

El mutador es idempotente: deja el archivo con el sha256 del principio y lo verifica.

Uso:  mutar-error-223.py --offset-vecino      (aplica, compila y deja el arbol mutado)
      mutar-error-223.py --sq-roto            (defecto de la Store Queue + libgodc.a rehecha)
      mutar-error-223.py --volver             (restaura el sha256 original y lo comprueba)
      mutar-error-223.py --control            (control sano: el original ya esta puesto)
"""
import argparse
import hashlib
import os
import subprocess
import sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(os.path.dirname(__file__))))
POC = os.path.join(REPO, "console/dc/poc/motor-error/main.go")
SHA = os.path.join(os.path.dirname(os.path.abspath(__file__)), ".error-223.sha")
LIB = os.environ.get("LIBGODC", "/home/dgalli/dreamcast/libgodc")
COPY = os.path.join(LIB, "runtime/gc_copy.c")
AQUI = os.path.dirname(os.path.abspath(__file__))

# El texto del arreglo (fix-libgodc-sq.py) y su mutacion (la formula vieja de la Store Queue). El mutante
# escribe EXACTAMENTE el texto pristino (sin comentarios): asi la cadena lo puede volver a reemplazar.
QACR_ARREGLO = """    *QACR0 = (dst_addr >> 24) & 0x1C;
    *QACR1 = (dst_addr >> 24) & 0x1C;"""
QACR_MUTADO = """    *QACR0 = (dst_addr >> 26) & 0x1C;
    *QACR1 = (dst_addr >> 26) & 0x1C;"""

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
    if actual != original:
        print("   FALLO: el sha256 quedo en %s y debia ser %s" % (actual, original))
        return 1
    print("   OK: el POC quedo con el sha256 original (%s)" % actual)
    # y el runtime, con el arreglo de la Store Queue puesto (la cadena lo vuelve a aplicar)
    r = subprocess.run([sys.executable, os.path.join(AQUI, "fix-libgodc-gc.py"), "512", LIB, "--build"],
                       capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout[-2000:] + r.stderr[-2000:])
        return 1
    print("   " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    return 0


def sq_roto(kb):
    """El mutante del arreglo: la formula vieja del area de la Store Queue, con libgodc.a rehecha."""
    # 1. la cadena + el arreglo (para partir del estado bueno) y su .a
    r = subprocess.run([sys.executable, os.path.join(AQUI, "fix-libgodc-gc.py"), str(kb), LIB, "--build"],
                       capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout[-2000:] + r.stderr[-2000:])
        return 1
    # 2. y AHORA si, el defecto viejo encima (despues de la cadena, que es la que lo aplica)
    with open(COPY, "r", encoding="utf-8") as f:
        t = f.read()
    if QACR_MUTADO in t:
        print("   el mutante de la Store Queue ya estaba puesto")
    elif QACR_ARREGLO in t:
        with open(COPY, "w", encoding="utf-8") as f:
            f.write(t.replace(QACR_ARREGLO, QACR_MUTADO, 1))
        print("   mutante de la Store Queue puesto: el area del monton vuelve a salir 0 (BIOS)")
    else:
        print("   FALLO: no encontre el arreglo de la Store Queue en %s" % COPY)
        return 1
    # 3. rehacer el .a con el defecto (el Makefile no mira cabeceras: se borran los objetos)
    subprocess.run(["bash", "-c", "rm -f %s/runtime/*.o %s/*.a" % (LIB, LIB)], check=True)
    r = subprocess.run(["make", "-C", LIB, "all"], capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout[-2000:] + r.stderr[-2000:])
        return 1
    print("   libgodc.a rehecha con el defecto: " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    return 0


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--offset-vecino", action="store_true")
    ap.add_argument("--sq-roto", type=int, nargs="?", const=512, default=None,
                    help="el semiespacio en KB con el que se rehace libgodc (por defecto 512)")
    ap.add_argument("--volver", action="store_true")
    ap.add_argument("--control", action="store_true")
    args = ap.parse_args()

    guardar_sha()

    if args.volver:
        return volver()

    if args.control:
        print("   control sano: el POC esta tal cual (offsetError = 156)")
        return 0

    if args.sq_roto:
        return sq_roto(args.sq_roto)

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
