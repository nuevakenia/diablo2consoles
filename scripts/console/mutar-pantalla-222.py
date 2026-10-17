#!/usr/bin/env python3
"""DC-002.22: los MUTANTES de la pantalla del motor, aplicados y sacados de forma idempotente.

POR QUE UN SCRIPT Y NO TOCAR A MANO: los dos mutantes de esta tarjeta son reveses de las dos palancas
del arreglo (el framebuffer en la VRAM y la pantalla del paquete sin crear), y el corpus los necesita
poner y sacar varias veces por corrida. A mano, el riesgo es dejar el arbol mutado en el commit (que es
peor que no medir). Aca cada mutante dice si esta puesto, se aplica una sola vez y `--restaurar` deja
los dos archivos exactamente como estaban (los saltos de linea incluidos: el archivo se abre con
newline='' porque leer/escribir en modo texto normal los normaliza y el sha256 cambiaria solo).

LOS DOS MUTANTES

  --pantalla-al-monton   el framebuffer NO se reserva en la VRAM: `d2c_pvr_pantalla_fb_init` devuelve
                         -2 de entrada (el mismo camino que "la VRAM no dio"), asi que el shim se queda
                         con la pantalla del MONTON, como antes de esta tarjeta. Es el mutante del
                         DEFECTO: tiene que volver el OOM de 1.920.000 bytes.
  --paquete-al-arranque  la pantalla del paquete vuelve a crearse al arrancar (`var pantalla =
                         NewImage(640, 480)`), o sea los 1.228.800 bytes que el motor no usa. Es el
                         mutante de la PALANCA DE RAM: con el monton al tope (4.128 bytes libres medidos),
                         el arranque NO llega a main(): se cae pidiendo esos 1.228.800.

Uso:
    python3 scripts/console/mutar-pantalla-222.py --estado
    python3 scripts/console/mutar-pantalla-222.py --pantalla-al-monton --paquete-al-arranque
    python3 scripts/console/mutar-pantalla-222.py --restaurar
"""

import argparse
import hashlib
import pathlib
import sys

REPO = pathlib.Path(__file__).resolve().parents[2]
PVR_C = REPO / "console/dc/std/csrc/pvr_pantalla.c"
SHIM_GO = REPO / "console/dc/engine/ebiten/v2/ebiten.go"

# El C: la primera linea del cuerpo de d2c_pvr_pantalla_fb_init.
C_ANCLA = "int d2c_pvr_pantalla_fb_init(int w, int h) {\n"
C_MUTANTE = "    return -2; /* MUTANTE DC-002.22: como si la VRAM no diera */\n"

# El Go: la pantalla del paquete, creada al arrancar (el defecto viejo) contra perezosa (el arreglo).
GO_SANO = "var pantalla *Image\n"
GO_MUTADO = "var pantalla = NewImage(PantallaAncho, PantallaAlto)\n"


def leer(path):
    with open(path, "r", newline="") as f:
        return f.read()


def escribir(path, texto):
    with open(path, "w", newline="") as f:
        f.write(texto)


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()[:16]


def estado(verboso=True):
    c = leer(PVR_C)
    g = leer(SHIM_GO)
    monton = C_MUTANTE in c
    paquete = GO_MUTADO in g
    if verboso:
        print(f"   {PVR_C.relative_to(REPO)}  sha {sha(PVR_C)}")
        print(f"      pantalla:  {'EN EL MONTON (mutante del defecto)' if monton else 'en la VRAM (sano)'}")
        print(f"   {SHIM_GO.relative_to(REPO)}  sha {sha(SHIM_GO)}")
        print(f"      paquete:   {'creada al arrancar (mutante de la RAM)' if paquete else 'perezosa (sano)'}")
    return monton, paquete


def pantalla_al_monton(puesto):
    c = leer(PVR_C)
    if puesto:
        if C_MUTANTE in c:
            print("   pantalla: ya esta en el monton")
        else:
            if C_ANCLA not in c:
                sys.exit("no encuentro d2c_pvr_pantalla_fb_init en pvr_pantalla.c")
            escribir(PVR_C, c.replace(C_ANCLA, C_ANCLA + C_MUTANTE, 1))
            print("   pantalla: AL MONTON (fb_init devuelve -2)")
    else:
        if C_MUTANTE not in c:
            print("   pantalla: ya estaba en la VRAM")
        else:
            escribir(PVR_C, c.replace(C_MUTANTE, "", 1))
            print("   pantalla: A LA VRAM")


def paquete_al_arranque(puesto):
    g = leer(SHIM_GO)
    if puesto:
        if GO_MUTADO in g:
            print("   paquete: ya se crea al arrancar")
        else:
            if GO_SANO not in g:
                sys.exit("no encuentro `var pantalla *Image` en ebiten.go")
            escribir(SHIM_GO, g.replace(GO_SANO, GO_MUTADO, 1))
            print("   paquete: AL ARRANQUE (NewImage(640, 480) al inicializar)")
    else:
        if GO_MUTADO not in g:
            print("   paquete: ya era perezosa")
        else:
            escribir(SHIM_GO, g.replace(GO_MUTADO, GO_SANO, 1))
            print("   paquete: PEREZOSA")


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--pantalla-al-monton", action="store_true", help="mutante A: la pantalla vuelve al monton")
    p.add_argument("--pantalla-a-la-vram", action="store_true", help="sano")
    p.add_argument("--paquete-al-arranque", action="store_true", help="mutante B: la pantalla del paquete al arrancar")
    p.add_argument("--paquete-perezoso", action="store_true", help="sano")
    p.add_argument("--estado", action="store_true")
    p.add_argument("--restaurar", action="store_true", help="los dos archivos al estado sano")
    args = p.parse_args()

    if args.estado:
        estado()
        return

    hizo = False
    if args.pantalla_al_monton or args.pantalla_a_la_vram:
        pantalla_al_monton(args.pantalla_al_monton)
        hizo = True
    if args.paquete_al_arranque or args.paquete_perezoso:
        paquete_al_arranque(args.paquete_al_arranque)
        hizo = True

    if args.restaurar:
        monton, paquete = estado(verboso=False)
        if monton:
            pantalla_al_monton(False)
        if paquete:
            paquete_al_arranque(False)
        hizo = True

    if not hizo:
        p.print_help()
        return

    print()
    estado()


if __name__ == "__main__":
    main()
