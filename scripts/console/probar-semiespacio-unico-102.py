#!/usr/bin/env python3
"""SONDA DC-002.102 (a) -- UN SOLO SEMIESPACIO: medir si el recolector de libgodc funciona con uno.

LA PREGUNTA. El monton de C de la consola, con el video encendido, son 5.113.792 B y de esos los DOS
semiespacios del recolector se llevan 4.194.304 B (2 MB cada uno, pedidos de una en `gc_init`). Si el
recolector anduviera con UNO, serian 2.097.152 B menos de monton tomado: la palanca mas grande que hay.
La tarjeta pide medirlo, no razonarlo.

LO QUE DICE EL CODIGO (y por que igual hay que medirlo). `gc_copy.c` es un recolector de COPIA tipo
Cheney: guarda el semiespacio VIEJO (`space[active]`) y copia a la MITAD nueva,

    int old_space = gc_heap.active_space;      /* gc_copy.c:174 */
    int new_space = 1 - old_space;             /* gc_copy.c:175 */
    gc_heap.alloc_ptr = gc_heap.space[new_space];   /* 184 */

y `gc_init` (gc_heap.c:107-108) pide SIEMPRE los dos. Con uno, la mitad nueva no existe y el recolector
escribe en NULL en la primera recoleccion -- que es lo que esta sonda deja medido: el arranque llega a
`gc_collect` y muere ahi, con la direccion 0x00000000 en el registro.

QUE HACE (y que NO toca). Copia `runtime/gc_heap.c` a `gc_heap.c.antes-102`, le inserta el `#ifdef
GC_UN_SEMIESPACIO` en `gc_init` (un solo `memalign`, `space[1] = NULL`, y el chequeo del par salteado),
rehace libgodc y lo deja asi. `--restaurar` vuelve el archivo original y rehace libgodc: el arbol queda
como estaba (nada de esto se commitea).

OJO CON EL ORDEN. Antes de correr esto, el runtime tiene que estar en el estado de la rama:
    python3 scripts/console/fix-libgodc-gc.py <KB> --build
(que hace `git checkout` del vendor y reaplica los arreglos). Esta sonda se aplica DESPUES y se retira
con `--restaurar`; no reemplaza la cadena de fix-*.py.

USO
    python3 scripts/console/probar-semiespacio-unico-102.py            # aplica y rehace libgodc
    python3 scripts/console/probar-semiespacio-unico-102.py --restaurar # vuelve atras y rehace
"""
import hashlib
import os
import pathlib
import subprocess
import sys

LIB = os.environ.get("LIBGODC", "/home/dgalli/dreamcast/libgodc")
HEAP = pathlib.Path(LIB + "/runtime/gc_heap.c")
RESPALDO = pathlib.Path(LIB + "/runtime/gc_heap.c.antes-102")

MARCA_INI = "/* DC-002.102 SONDA: UN SOLO SEMIESPACIO */"
MARCA_FIN = "/* DC-002.102 SONDA: FIN */"


def sha(p):
    return hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest()[:16]


# el toolchain de KOS no esta en el PATH de una sesion no interactiva: make lo necesita para sh-elf-gcc
_DC = os.path.expanduser("~/dreamcast")
os.environ["PATH"] = _DC + "/sh-elf/bin:" + os.environ.get("PATH", "")
os.environ.setdefault("KOS_BASE", _DC + "/kos")


def rehacer():
    subprocess.run(["bash", "-c", "rm -f %s/runtime/*.o %s/*.a" % (LIB, LIB)], check=True)
    r = subprocess.run(["make", "-C", LIB, "all"], capture_output=True, text=True)
    print("   make: " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    if r.returncode != 0:
        print(r.stdout[-2000:] + r.stderr[-2000:])
        sys.exit("make fallo")
    print("   %s/libgodc.a: %s bytes, sha256 %s" % (LIB, os.path.getsize(LIB + "/libgodc.a"),
                                                    sha(LIB + "/libgodc.a")))


if "--restaurar" in sys.argv:
    print("== 1/2 devolver runtime/gc_heap.c")
    if not RESPALDO.exists():
        sys.exit("no hay respaldo en %s: nada que restaurar" % RESPALDO)
    HEAP.write_bytes(RESPALDO.read_bytes())
    RESPALDO.unlink()
    print("   %s restaurado, sha256 %s" % (HEAP, sha(HEAP)))
    CONFIG = pathlib.Path(LIB + "/runtime/godc_config.h")
    cfg = CONFIG.read_text()
    if "GC_UN_SEMIESPACIO" in cfg:
        cfg = "\n".join(l for l in cfg.splitlines()
                        if "GC_UN_SEMIESPACIO" not in l and "SONDA: un solo semiespacio" not in l)
        CONFIG.write_text(cfg.rstrip() + "\n")
        print("   godc_config.h sin la bandera, sha256 %s" % sha(CONFIG))
    print("== 2/2 rehacer libgodc")
    rehacer()
    sys.exit(0)

print("== 1/3 el arbol tiene que estar sin la sonda puesta")
if RESPALDO.exists():
    sys.exit("ya hay un respaldo en %s: corre --restaurar antes" % RESPALDO)

texto = HEAP.read_text()
if MARCA_INI in texto:
    sys.exit("gc_heap.c ya trae la sonda: corre --restaurar antes")

viejo = """    gc_heap.space[0] = (uint8_t *)memalign(32, GC_SEMISPACE_SIZE);
    gc_heap.space[1] = (uint8_t *)memalign(32, GC_SEMISPACE_SIZE);"""
nuevo = """    gc_heap.space[0] = (uint8_t *)memalign(32, GC_SEMISPACE_SIZE);
""" + MARCA_INI + """
    /* UN SOLO SEMIESPACIO (medicion, no arreglo): el segundo NO se pide y queda en NULL. */
#ifdef GC_UN_SEMIESPACIO
    gc_heap.space[1] = NULL;
#else
    gc_heap.space[1] = (uint8_t *)memalign(32, GC_SEMISPACE_SIZE);
#endif
    """ + MARCA_FIN

if viejo not in texto:
    sys.exit("NO ENCONTRE el par de memalign de gc_init en %s: revisar el vendor antes de seguir" % HEAP)

texto = texto.replace(viejo, nuevo, 1)

# con un solo semiespacio el chequeo del PAR tiene que saltar el segundo (si no, muere en gc_init y no se
# mide lo que la tarjeta pregunta: que pasa cuando el recolector tiene que copiar)
viejo_chk = """    if (!gc_heap.space[0] || !gc_heap.space[1])"""
nuevo_chk = """#ifdef GC_UN_SEMIESPACIO
    if (!gc_heap.space[0])
#else
    if (!gc_heap.space[0] || !gc_heap.space[1])
#endif"""
if viejo_chk not in texto:
    sys.exit("NO ENCONTRE el chequeo de los dos semiespacios en %s" % HEAP)
texto = texto.replace(viejo_chk, nuevo_chk, 1)

# el memset del segundo tampoco puede correr con NULL
viejo_set = """    memset(gc_heap.space[0], 0, GC_SEMISPACE_SIZE);
    memset(gc_heap.space[1], 0, GC_SEMISPACE_SIZE);"""
nuevo_set = """    memset(gc_heap.space[0], 0, GC_SEMISPACE_SIZE);
#ifndef GC_UN_SEMIESPACIO
    memset(gc_heap.space[1], 0, GC_SEMISPACE_SIZE);
#endif"""
if viejo_set not in texto:
    sys.exit("NO ENCONTRE los dos memset de gc_init en %s" % HEAP)
texto = texto.replace(viejo_set, nuevo_set, 1)

RESPALDO.write_bytes(HEAP.read_bytes())
HEAP.write_text(texto)
print("   respaldo: %s" % RESPALDO)
print("   gc_heap.c con la sonda, sha256 %s" % sha(HEAP))

# la bandera, en la configuracion del runtime (el Makefile la incluye por cabecera)
CONFIG = pathlib.Path(LIB + "/runtime/godc_config.h")
cfg = CONFIG.read_text()
if "GC_UN_SEMIESPACIO" not in cfg:
    CONFIG.write_text(cfg.rstrip() + "\n\n/* DC-002.102 SONDA: un solo semiespacio (ver probar-semiespacio-unico-102.py) */\n"
                      "#define GC_UN_SEMIESPACIO 1\n")
    print("   godc_config.h: + GC_UN_SEMIESPACIO 1, sha256 %s" % sha(CONFIG))

print("== 2/3 rehacer libgodc")
rehacer()

print("== 3/3 listo: reenlazar el POC y correr")
print("   SALTAR_MOTOR=1 bash scripts/console/build-mapa-102.sh /tmp/dcbuild/m102-un-sem")
print("   (el arranque tiene que morir en la PRIMERA recoleccion: es el veredicto, no un fallo de la sonda)")
