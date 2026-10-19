#!/usr/bin/env python3
"""MUTANTE del arreglo de la COPIA POR STORE QUEUE (DC-002.24) que DC-002.39c destapo como la causa de que
la consulta del corpus pierda los nombres.

QUE HACE: sobre una copia PRIVADA del runtime (nunca el vendor compartido, que se toca con
`fix-libgodc-sq.py`), vuelve a poner la formula VIEJA en los dos QACR de `runtime/gc_copy.c`:

    *QACR0/1 = (dst_addr >> 24) & 0x1C;     <- el arreglo (la formula del KOS, sq.c)
    *QACR0/1 = (dst_addr >> 26) & 0x1C;     <- el MUTANTE (el estado del vendor vivo)

Es UN cambio de una palabra (`24` -> `26`). Con `>> 26` sobre una direccion del monton (0x8C......) el area
sale SIEMPRE 0 (la BIOS, de solo lectura): las escrituras de la Store Queue se DESCARTAN, el destino de la
copia queda con la memoria vieja y el barrido de Cheney se corta en la primera cabecera invalida. El
`dcache_inval_range` que agrega el arreglo se deja puesto a proposito: asi el mutante aisla SOLO la formula.

POR QUE EXISTE: una suite verde no prueba nada. Con este mutante el corpus de la consulta
(`scripts/console/checar-consulta-233.sh`) tiene que ponerse ROJO (por debajo de 21.253 nombres), y el
control sano (sin mutar) tiene que seguir verde. Medido en DC-002.39c:

    runtime con el arreglo (>>24):  checar-consulta-233.sh 3 120 -> 3/3 vueltas con 21253
    runtime mutante      (>>26):    checar-consulta-233.sh 1 120 -> ROJO

USO
    python3 scripts/console/mutar-libgodc-sq.py [LIB]              # aplica el mutante (idempotente)
    python3 scripts/console/mutar-libgodc-sq.py --estado [LIB]     # dice como esta la copia
    python3 scripts/console/mutar-libgodc-sq.py --build [LIB]      # ademas rehace libgodc.a
    (LIB por defecto: $LIBGODC si esta definido, si no <dir_del_repo>/... ; aca se exige explicito para no
     tocar nunca el vendor por accidente)
"""
import os
import pathlib
import subprocess
import sys

args = [a for a in sys.argv[1:] if not a.startswith("--")]
if not args:
    print("ERROR: falta la copia privada del runtime (LIB). Este script NO se corre sobre el vendor a mano.")
    sys.exit(2)

LIB = args[0]
COPY = pathlib.Path(LIB + "/runtime/gc_copy.c")

if not COPY.exists():
    print("ERROR: no existe %s" % COPY)
    sys.exit(2)

ARREGLADO = "(dst_addr >> 24) & 0x1C"
MUTANTE = "(dst_addr >> 26) & 0x1C"
texto = COPY.read_text()
n_arr = texto.count(ARREGLADO)
n_mut = texto.count(MUTANTE)

if "--estado" in sys.argv:
    print("mutante-sq: %s -> arreglado(>>24) x%d, mutante(>>26) x%d" % (LIB, n_arr, n_mut))
    sys.exit(0 if n_mut == 2 else 1)

if n_mut == 2:
    print("  ya estaba mutado (>>26 x2) %s" % COPY)
elif n_arr == 2:
    COPY.write_text(texto.replace(ARREGLADO, MUTANTE))
    print("  ok %s: >>24 -> >>26 en los dos QACR (MUTANTE)" % COPY)
else:
    sys.exit("ERROR: esperaba 2 ocurrencias de la formula (arreglada x%d, mutada x%d) en %s" % (n_arr, n_mut, COPY))

if "--build" in sys.argv:
    for f in (pathlib.Path(LIB) / "runtime").glob("*.o"):
        f.unlink()
    for f in pathlib.Path(LIB).glob("*.a"):
        f.unlink()
    r = subprocess.run(["make", "-C", LIB, "-s", "all"])
    if r.returncode != 0:
        sys.exit("make fallo")
    print("  libgodc.a: %d bytes, sha256 %s" % (
        os.path.getsize(LIB + "/libgodc.a"),
        subprocess.run(["sha256sum", LIB + "/libgodc.a"], capture_output=True, text=True).stdout[:16]))
