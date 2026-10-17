#!/usr/bin/env python3
"""Traza las asignaciones GRANDES del motor en la Dreamcast: QUE pide y DESDE DONDE.

SINTOMA (DC-002.22, abierta al cerrar DC-002.21)
    Con el atlas de texto ya entrando (semiespacio del GC en 256 KB), el arranque del motor muere asi:

        Out of memory. Requested sbrk_base 0x8d18a000, was 0x8cfb6000, diff 1916928
        fatal error: gc_external_alloc: OOM
        Memory: arena=2415484 used=2372348 free=43136

    O sea: el motor pide 1.916.928 bytes CONTIGUOS (0x1D4000) y el monton de C tiene 43.136 libres. El
    mensaje dice CUANTO, no QUIEN ni QUE estructura. Y el candidato por parecido de numeros (la tabla de
    `d2core/d2records`, 1.672.892 bytes en `.data`) es del mismo orden pero NO es el mismo numero: hay que
    medirlo, no suponerlo.

POR QUE NO ALCANZA CON LEER EL MENSAJE
    `gc_external_alloc` (runtime/gc_heap.c) no recibe el tipo: el tamano ya esta calculado. El punto donde
    TODAVIA se sabe de quien es la asignacion es la entrada del runtime que llama el codigo Go:
    `runtime_makeslice`, `runtime_growslice`, `runtime_newobject` y `runtime_mallocgc`.

ARREGLO (traza, no arreglo: es el instrumento)
    Se instrumentan las cuatro entradas, SOLO cuando el tamano pasa el umbral, y cada linea imprime:

        ALLOC-TRAZA <sitio> size= <n> tipo= <nombre del tipo> elemsize= <n> len= <n> cap= <n> pc= <pc>

    El `pc` es el de RETORNO (`__builtin_return_address(0)`), o sea el llamado DESDE EL CODIGO GO; en SH-4 el
    registro de retorno apunta a la instruccion SIGUIENTE al llamado, asi que el llamado esta en pc-4 (las
    instrucciones del SH son de 2 bytes). Se resuelve contra el ELF enlazado (que lleva simbolos):

        sh-elf-addr2line -f -e prog.elf <pc-4>
        sh-elf-nm -n prog.elf | awk '$1<="<pc-4>"' | tail -1

    El nombre del tipo sale del descriptor (`type->__uncommon->__name`, que en gccgo es un puntero a
    `struct __go_string`): es lo que contesta "QUE estructura" sin adivinar. Si el tipo no tiene uncommon
    (tipos sin nombre) sale `-`.

UMBRAL Y TOPE
    Umbral: 256 KB (el pedido del defecto es 1,9 MB; 256 KB deja el log legible y no pierde nada grande).
    Tope de lineas: 60, para que un arranque que asigne mucho no inunde la consola serie.

MEDIDO CON ESTE INSTRUMENTO (DC-002.22, el arranque del motor en Flycast)
    El pedido del OOM NO es la tabla de `d2core/d2records` (1.672.892 bytes, el candidato por parecido):
    es `make([]uint8, 1920000)`, o sea una imagen de 800 x 600 x 4 (la PANTALLA logica del renderer:
    `d2display.BaseW/BaseH` = 800/600), en `NewImage` (`console/dc/engine/ebiten/v2/ebiten.go:43`), el de
    `RunGame` (`pantalla = NewImage(ancho, alto)`, ebiten.go:870). Y el 1.916.928 del mensaje de OOM no es
    el tamaño pedido: es el `diff` de `sbrk` (1.920.000 mas el redondeo del monton).

    OJO, MEDIDO: el instrumento COMPLETO (las cinco entradas) cambia el camino del arranque (el arranque
    termina en un panic de puntero nil y NO pide los 1,92 MB). El que reproduce la pared de la rama (el OOM
    en `gc_external_alloc`) y contesta la pregunta es el MINIMO: `--solo-grande`. Si hay que volver a mirar
    asignaciones, empezar por ahi.

USO
    python3 scripts/console/fix-libgodc-alloc-traza.py --solo-grande --build    # el instrumento minimo
    python3 scripts/console/fix-libgodc-alloc-traza.py --build                  # las cinco entradas
    # reenlazar y correr Flycast:
    POC=motor-arranque bash scripts/console/build-motor-arranque.sh /tmp/dcbuild/traza-222 /tmp/gox-00220
    bash scripts/console/run-flycast.sh /tmp/dcbuild/traza-222/prog-emu.elf 30 /tmp/dc-run-traza222
    grep -a 'ALLOC-TRAZA' /tmp/dc-run-traza222/flycast.log | tr -d '\r'
    # el pc se resuelve contra el ELF (el llamado esta en pc-4, las instrucciones del SH son de 2 bytes):
    sh-elf-addr2line -f -e /tmp/dcbuild/traza-222/prog.elf 8c03e52a
    # y DESPUES dejar el runtime sin la traza:
    python3 scripts/console/fix-libgodc-gc.py 256 --build

ORDEN E IDEMPOTENCIA
    Deja el arbol pristino con `git checkout` y reaplica la cadena completa (ABI -> interfaces -> GC en
    256 KB) antes de este cambio: correrlo dos veces deja el mismo sha256.
    OJO (medido): `fix-libgodc-gc.py` NO saca este instrumento (su cadena solo revierte SUS archivos). Para
    dejar el runtime como el de la rama hay que correr
    `git -C <lib> checkout -- runtime/gc_runtime.c runtime/runtime_stubs.c runtime/gc_heap.c` y despues
    `fix-libgodc-gc.py 256 --build`.
"""
import hashlib
import os
import pathlib
import subprocess
import sys

args = [a for a in sys.argv[1:] if not a.startswith("--")]
LIB = args[0] if args else "/home/dgalli/dreamcast/libgodc"
AQUI = os.path.dirname(os.path.abspath(__file__))
GC_RUNTIME = pathlib.Path(LIB + "/runtime/gc_runtime.c")
STUBS = pathlib.Path(LIB + "/runtime/runtime_stubs.c")
HEAP = pathlib.Path(LIB + "/runtime/gc_heap.c")
MIS_ARCHIVOS = ["runtime/gc_runtime.c", "runtime/runtime_stubs.c", "runtime/gc_heap.c"]

# ------------------------------------------------------------------ el texto que se inyecta
COMM = '''/* DC-002.22: la traza de las asignaciones grandes (ver scripts/console/fix-libgodc-alloc-traza.py).
 * Imprime el tamano, el tipo (cuando el descriptor lo tiene) y el pc del llamado DESDE EL CODIGO GO
 * (en SH-4 el llamado esta en pc-4). Tope de 60 lineas para no inundar la consola serie. */
#define TRZ_UMBRAL  (256 * 1024)
#define TRZ_TOPE    60
static int trz_lineas = 0;
static void trz_log(const char *sitio, unsigned int size, const struct __go_type_descriptor *t,
                    long len, long cap)
{
    if (size < TRZ_UMBRAL || trz_lineas >= TRZ_TOPE)
        return;
    trz_lineas++;
    if (t && t->__uncommon && t->__uncommon->__name)
        dbglog(DBG_CRITICAL, "ALLOC-TRAZA %s size=%u tipo=%.*s elemsize=%u len=%ld cap=%ld pc=%08x\\n",
               sitio, size, (int)t->__uncommon->__name->__length,
               (const char *)t->__uncommon->__name->__data, (unsigned int)t->__size, len, cap,
               (unsigned int)(uintptr_t)__builtin_return_address(0));
    else
        dbglog(DBG_CRITICAL, "ALLOC-TRAZA %s size=%u tipo=- elemsize=%u len=%ld cap=%ld pc=%08x\\n",
               sitio, size, t ? (unsigned int)t->__size : 0, len, cap,
               (unsigned int)(uintptr_t)__builtin_return_address(0));
}
'''

# 1. runtime_makeslice: la asignacion de un slice nuevo (make([]T, len, cap))
VIEJO_MAKESLICE = '''void *runtime_makeslice(struct __go_type_descriptor *elem_type, intptr_t len, intptr_t cap)
{'''
NUEVO_MAKESLICE = '''void *runtime_makeslice(struct __go_type_descriptor *elem_type, intptr_t len, intptr_t cap)
{
    if (elem_type) trz_log("makeslice", (unsigned int)(elem_type->__size * cap), elem_type,
                           (long)len, (long)cap);'''

# 2. runtime_mallocgc: el camino generico del compilador
VIEJO_MALLOCGC = '''    (void)needzero;
    return gc_alloc_impl(size, type);
}'''
NUEVO_MALLOCGC = '''    (void)needzero;
    trz_log("mallocgc", (unsigned int)size, type, -1, -1);
    return gc_alloc_impl(size, type);
}'''

# 3. runtime_growslice: el append que crece (recibe el tipo del ELEMENTO)
VIEJO_GROWSLICE = '''Slice runtime_growslice(void *elem_type, void *oldarray, intptr_t oldlen, intptr_t oldcap, intptr_t cap)
{
    struct __go_type_descriptor *et = (struct __go_type_descriptor *)elem_type;
    size_t elem_size = et ? et->__size : 0;
'''
NUEVO_GROWSLICE = '''Slice runtime_growslice(void *elem_type, void *oldarray, intptr_t oldlen, intptr_t oldcap, intptr_t cap)
{
    struct __go_type_descriptor *et = (struct __go_type_descriptor *)elem_type;
    size_t elem_size = et ? et->__size : 0;

    trz_log("growslice", (unsigned int)(elem_size * (cap > 0 ? cap : 0)), et, (long)oldcap, (long)cap);
'''

# 4. gc_external_alloc: el malloc del objeto grande (aqui ya no hay tipo, pero si el rastro del pc)
VIEJO_EXTERNAL = '''void *gc_external_alloc(size_t size)
{
    void *ptr = malloc(size);'''
NUEVO_EXTERNAL = '''void *gc_external_alloc(size_t size)
{
    trz_log("external", (unsigned int)size, NULL, -1, -1);
    void *ptr = malloc(size);'''

# 5. gc_alloc: la puerta por la que pasa TODO objeto grande (aca SI esta el descriptor del tipo)
VIEJO_GRANDE = '''    if (size > GC_LARGE_OBJECT_THRESHOLD)
    {
        gc_heap.large_alloc_count++;
        gc_heap.large_alloc_total += size;
        return gc_external_alloc(size);
    }'''
NUEVO_GRANDE = '''    if (size > GC_LARGE_OBJECT_THRESHOLD)
    {
        gc_heap.large_alloc_count++;
        gc_heap.large_alloc_total += size;
        /* DC-002.22: la traza de TODA asignacion que se va al monton de C. El descriptor del tipo viene del
         * compilador, asi que aca se sabe QUE se pide aunque el llamador no lo diga. */
        trz_log("gc_alloc-grande", (unsigned int)size, type, -1, -1);
        return gc_external_alloc(size);
    }'''


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()[:16]


print("== 1/3 el arbol pristino + los arreglos que ya existian (ABI, interfaces y GC en 256 KB)")
r = subprocess.run([sys.executable, os.path.join(AQUI, "fix-libgodc-gc.py"), "256", LIB],
                   capture_output=True, text=True)
print("   " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
if r.returncode != 0:
    print(r.stdout + r.stderr)
    sys.exit("fix-libgodc-gc.py fallo")

# Los archivos de ESTE script tambien vuelven al pristino: sin esto, correrlo dos veces no es idempotente
# (el texto a reemplazar ya no estaria). Los de fix-libgodc-iface.py no se tocan aca.
subprocess.run(["git", "-C", LIB, "checkout", "--"] + MIS_ARCHIVOS, check=True)
print("   pristinos: " + ", ".join(MIS_ARCHIVOS))

print("== 2/3 la traza")


def reemplazar(path, viejo, nuevo):
    t = path.read_text()
    n = t.count(viejo)
    if n != 1:
        sys.exit("ERROR: el texto a reemplazar aparece %d veces (esperaba 1) en %s" % (n, path))
    path.write_text(t.replace(viejo, nuevo, 1))
    print("  ok %s" % path.name)


# `--solo-grande`: SOLO la puerta de gc_alloc (una linea de log). Sirve para separar "el instrumento
# cambio el camino" de "el instrumento no llego a la asignacion": con una sola linea de log en el MINIMO
# de codigo (medido en DC-002.22: con las cinco entradas el arranque llegaba mas lejos y el pedido de
# 1.916.928 NO aparecia).
SOLO_GRANDE = "--solo-grande" in sys.argv
if SOLO_GRANDE:
    print("   SOLO la puerta de gc_alloc (una entrada, un log)")
    reemplazar(HEAP, '#include <kos/dbglog.h>\n', '#include <kos/dbglog.h>\n' + COMM)
    reemplazar(HEAP, VIEJO_GRANDE, NUEVO_GRANDE)
else:
    # el bloque de la traza va arriba de todo, en gc_runtime.c (que ya incluye <stdio.h>)
    reemplazar(GC_RUNTIME, '#include "runtime.h"\n#include "gc_semispace.h"',
               '#include "runtime.h"\n#include "gc_semispace.h"\n#include <kos/dbglog.h>\n')
    reemplazar(GC_RUNTIME, 'void *runtime_newobject(struct __go_type_descriptor *type) __asm__("_runtime.newobject");\n',
               COMM + '\nvoid *runtime_newobject(struct __go_type_descriptor *type) __asm__("_runtime.newobject");\n')
    reemplazar(GC_RUNTIME, VIEJO_MAKESLICE, NUEVO_MAKESLICE)
    reemplazar(GC_RUNTIME, VIEJO_MALLOCGC, NUEVO_MALLOCGC)

    # runtime_stubs.c (growslice) y gc_heap.c (el malloc del objeto grande)
    reemplazar(STUBS, '#include "gc_semispace.h"\n\n// Dreamcast-specific allocation limit',
               '#include "gc_semispace.h"\n' + COMM + '\n// Dreamcast-specific allocation limit')
    reemplazar(STUBS, VIEJO_GROWSLICE, NUEVO_GROWSLICE)
    reemplazar(HEAP, '#include <kos/dbglog.h>\n', '#include <kos/dbglog.h>\n' + COMM)
    reemplazar(HEAP, VIEJO_EXTERNAL, NUEVO_EXTERNAL)
    reemplazar(HEAP, VIEJO_GRANDE, NUEVO_GRANDE)

print("== 3/3 el runtime")
if "--build" in sys.argv:
    # OJO (medido): `%.o: %.c` sin dependencias de cabeceras y `make clean` se lleva el kos.gox.
    subprocess.run(["bash", "-c", f"rm -f {LIB}/runtime/*.o {LIB}/*.a"], check=True)
    r = subprocess.run(["make", "-C", LIB, "all"], capture_output=True, text=True)
    print("   make: " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    if r.returncode != 0:
        print(r.stdout[-3000:] + r.stderr[-3000:])
        sys.exit("make fallo (si es por dbglog: revisar el #include <kos/dbglog.h>)")
    print(f"   {LIB}/libgodc.a: {os.path.getsize(LIB + '/libgodc.a')} bytes, sha256 {sha(pathlib.Path(LIB + '/libgodc.a'))}")
else:
    print("   falta --build")
print(f"   gc_runtime.c sha256={sha(GC_RUNTIME)}   runtime_stubs.c sha256={sha(STUBS)}   gc_heap.c sha256={sha(HEAP)}")
print("   ahora: reenlazar el motor y correr Flycast; en el log buscar 'ALLOC-TRAZA'")
