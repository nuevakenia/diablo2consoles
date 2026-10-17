#!/usr/bin/env python3
"""Fija el SEMIESPACIO DEL GC de libgodc para la Dreamcast (el recorte que s¡ hace entrar el arranque).

SINTOMA
    El motor entero enlazado (205 .gox, 13.910.992 bytes cargados de los 16 MB) muere ANTES de una sola
    linea del motor:

        Out of memory. Requested sbrk_base 0x8d16b000, was 0x8cf6b000, diff 2097152
        fatal error: gc_init: alloc failed

    Y no es el binario solo: `gc_init` (runtime/gc_heap.c:37-38) pide DOS semiespacios del mismo tamano
    de una sola vez --

        gc_heap.space[0] = (uint8_t *)memalign(32, GC_SEMISPACE_SIZE);
        gc_heap.space[1] = (uint8_t *)memalign(32, GC_SEMISPACE_SIZE);

    -- asi que lo que necesita para arrancar es `2 * GC_SEMISPACE_SIZE`, no uno. La escalera medidas en
    esta tarjeta (mismo binario de 13.910.992 bytes cargados, scripts/console/medir-gc-semiespacio-214.sh):

        GC_SEMISPACE_SIZE_KB   el GC pide   el arranque
        2048 (default)          4 MB        muere en gc_init: alloc failed (diff 2097152)
        1024                    2 MB        pasa gc_init, muere despues en gc_external_alloc: OOM
                                          (diff 1228800)
        512                     1 MB        pasa las DOS paredes de RAM y llega al panic de Go del init
        256                     512 KB      igual que 512 (arena mas chica: 1.915.772 contra 2.440.060)

ARREGLO
    `GC_SEMISPACE_SIZE_KB` en runtime/godc_config.h, con el valor que se pida (por defecto 512 KB, que es
    el semiespacio mas grande que deja atras TODAS las paredes de RAM). El
    runtime lo lee para el semiespacio y para el umbral de objetos grandes; no se toca nada mas.

COSTO (declarado, no escondido)
    Un semiespacio de 256 KB es un heap chico: el GC copia cada 256 KB de allocacion, asi que la consola
    recolecta muchas mas veces. Sirve para el ARRANQUE (que es lo que esta tarjeta mide) y para saber que
    el motor llega a correr; para jugar hay que volver a subirlo a medida que el binario se recorte (o
    que el motor lea sus datos del disco en vez de tenerlos en .data, que es donde esta el peso real:
    d2core/d2records tiene UN dato de 1.672.892 bytes).

USO
    python3 scripts/console/fix-libgodc-gc.py [KB] [ruta_libgodc]
    python3 scripts/console/fix-libgodc-gc.py 256 --build      # fija y rehace libgodc.a
    Despues: reenlazar (scripts/console/recorte-214.sh, o build-motor-arranque.sh / build-motor-dc6.sh).

ORDEN E IDEMPOTENCIA
    Deja el arbol pristino con `git checkout`, aplica PRIMERO fix-libgodc-iface.py (que a su vez aplica el
    de ABI) y DESPUES este cambio: correrlo dos veces deja el mismo sha256.
"""
import hashlib
import os
import pathlib
import re
import subprocess
import sys

KB = 512
args = [a for a in sys.argv[1:] if not a.startswith("--")]
if args:
    KB = int(args[0])
LIB = args[1] if len(args) > 1 else "/home/dgalli/dreamcast/libgodc"
AQUI = os.path.dirname(os.path.abspath(__file__))
CONFIG = pathlib.Path(LIB + "/runtime/godc_config.h")


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()[:16]


print("== 1/3 el arbol pristino + los arreglos que ya existian (ABI e interfaces)")
r = subprocess.run([sys.executable, os.path.join(AQUI, "fix-libgodc-iface.py"), LIB],
                   capture_output=True, text=True)
print("   " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
if r.returncode != 0:
    print(r.stdout + r.stderr)
    sys.exit("fix-libgodc-iface.py fallo")

print("== 2/3 el semiespacio del GC")
texto = CONFIG.read_text()
patron = re.compile(r"#define GC_SEMISPACE_SIZE_KB\s+(\d+)")
m = patron.search(texto)
if not m:
    sys.exit(f"NO ENCONTRE '#define GC_SEMISPACE_SIZE_KB <n>' en {CONFIG}: revisar el vendor antes de seguir")
if int(m.group(1)) == KB:
    print(f"   ya estaba en {KB} KB (idempotente)")
else:
    anterior = int(m.group(1))
    CONFIG.write_text(patron.sub(f"#define GC_SEMISPACE_SIZE_KB {KB}", texto, count=1))
    print(f"   GC_SEMISPACE_SIZE_KB: {anterior} -> {KB}")

print(f"   {CONFIG}: sha256 {sha(CONFIG)}")
print(f"   lo que el GC pide al arrancar ahora: 2 x {KB} KB = {2 * KB * 1024} bytes")
print("   (medido en el arranque del binario de 13.910.992 bytes: quedan libres 610.304)")

print("== 3/3 el runtime")
if "--build" in sys.argv:
    # OJO (medido): el Makefile tiene `%.o: %.c` SIN dependencias de cabeceras, asi que cambiar
    # godc_config.h NO recompila nada y libgodc.a queda con el valor viejo. Se borran SOLO los objetos
    # del runtime (no `make clean`, que ademas hace `make -C kos clean` y se lleva el kos.gox que los
    # paquetes de Go importan).
    subprocess.run(["bash", "-c", f"rm -f {LIB}/runtime/*.o {LIB}/*.a"], check=True)
    # `all` y NO `libgodc.a`: el enlace necesita libgodc Y libgodcbegin (esta ultima trae el arranque del
    # runtime). Medido: con solo libgodc.a el enlace se cae con "cannot find -lgodcbegin".
    r = subprocess.run(["make", "-C", LIB, "all"], capture_output=True, text=True)
    print("   make: " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    if r.returncode != 0:
        print(r.stdout[-2000:] + r.stderr[-2000:])
        sys.exit("make fallo")
    print(f"   {LIB}/libgodc.a: {os.path.getsize(LIB + '/libgodc.a')} bytes, sha256 {sha(pathlib.Path(LIB + '/libgodc.a'))}")
else:
    print(f"   falta: rm -f {LIB}/runtime/*.o && make -C {LIB} libgodc.a   (o correr con --build)")
