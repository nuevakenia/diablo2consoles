#!/usr/bin/env python3
"""Traza el mapa nil de libgodc: imprime el pc del que llama antes de paniquear.

SINTOMA (DC-002.15)
    Con el semiespacio del GC en 512 KB el arranque del motor ya pasa las paredes de RAM y muere asi:

        panic: assignment to entry in nil map

        goroutine 1 [running]:
        Stack Trace: frame pointers not enabled!

        Memory: arena=2440060 used=2436340 free=3720

    y el panic es ANTES de `main` (main.go imprime `MOTOR-ARRANQUE arranca` en su primera linea y el log no
    tiene ni una linea MOTOR-ARRANQUE): lo dispara un `init` o un inicializador de paquete.

POR QUE NO ALCANZA CON LEER EL MENSAJE
    El mensaje lo imprime libgodc (runtime/map_dreamcast.c:1051 y el macro de map_fast_internal.h:245), no el
    runtime de Go, y el runtime minimo no sabe armar la pila ("frame pointers not enabled"). O sea que el
    panic dice QUE paso (escritura en un mapa nil) pero no QUIEN lo hizo.

ARREGLO (traza, no arreglo: es el instrumento)
    Los dos sitios que paniquean imprimen por la consola serie el pc de RETORNO (`__builtin_return_address(0)`)
    antes de paniquear. Ese pc se resuelve contra el ELF enlazado (que lleva simbolos):

        sh-elf-nm prog.elf | sort | awk '$1<="<pc>"' | tail -1     # o
        sh-elf-addr2line -f -e prog.elf <pc>

    Nota de SH-4: el registro de retorno (PR) apunta a la instruccion SIGUIENTE al llamado, asi que el
    llamado esta en pc-4 (las instrucciones del SH son de 2 bytes).

USO
    python3 scripts/console/fix-libgodc-map-traza.py [--build] [ruta_libgodc]
    Despues: reenlazar (scripts/console/build-motor-arranque.sh <dir> /tmp/gox-0028) y correr Flycast.

ORDEN E IDEMPOTENCIA
    Deja el arbol pristino con `git checkout` y reaplica la cadena completa (ABI -> interfaces -> GC 512 KB)
    antes de este cambio: correrlo dos veces deja el mismo sha256.
"""
import hashlib
import os
import pathlib
import subprocess
import sys

args = [a for a in sys.argv[1:] if not a.startswith("--")]
LIB = args[0] if args else "/home/dgalli/dreamcast/libgodc"
AQUI = os.path.dirname(os.path.abspath(__file__))
MAPA = pathlib.Path(LIB + "/runtime/map_dreamcast.c")
HEADER = pathlib.Path(LIB + "/runtime/map_fast_internal.h")
DEFER = pathlib.Path(LIB + "/runtime/defer_dreamcast.c")
MIS_ARCHIVOS = ["runtime/map_dreamcast.c", "runtime/map_fast_internal.h", "runtime/defer_dreamcast.c"]


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()[:16]


print("== 1/3 el arbol pristino + los arreglos que ya existian (ABI, interfaces y GC en 512 KB)")
r = subprocess.run([sys.executable, os.path.join(AQUI, "fix-libgodc-gc.py"), "512", LIB],
                   capture_output=True, text=True)
print("   " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
if r.returncode != 0:
    print(r.stdout + r.stderr)
    sys.exit("fix-libgodc-gc.py fallo")

# Los archivos de ESTE script tambien vuelven al pristino: sin esto, correrlo dos veces no es idempotente
# (el texto a reemplazar ya no estaria). Los de fix-libgodc-iface.py no se tocan aca.
subprocess.run(["git", "-C", LIB, "checkout", "--"] + MIS_ARCHIVOS, check=True)
print("   pristinos: " + ", ".join(MIS_ARCHIVOS))

print("== 2/3 la traza del panic (los tres sitios)")
VIEJO_PANIC = '''void runtime_gopanic_impl(struct __go_type_descriptor *type, void *data)
{'''
NUEVO_PANIC = '''void runtime_gopanic_impl(struct __go_type_descriptor *type, void *data)
{
    /* DC-002.15: la traza del panic. El pc de retorno es el del que llamo a panic() (en SH-4 el llamado
     * esta en pc-4); el kind dice QUE se paniqueo. */
    dbglog(DBG_CRITICAL, "PANIC-TRAZA pc=%08x tipo=%08x kind=%d dato=%08x\\n",
           (unsigned int)(uintptr_t)__builtin_return_address(0),
           (unsigned int)(uintptr_t)type, type ? (int)type->__code : -1,
           (unsigned int)(uintptr_t)data);'''

print("== 2/3 la traza del mapa nil (los dos sitios que paniquean)")
VIEJO_FUNC = '''    if (h == NULL) {
        runtime_panicstring("assignment to entry in nil map");
        return g_zero_value;
    }'''
NUEVO_FUNC = '''    if (h == NULL) {
        /* DC-002.15: la traza. El pc de retorno dice QUIEN escribio en el mapa nil: se resuelve con
         * sh-elf-addr2line contra el ELF enlazado (en SH-4 el llamado esta en pc-4). */
        dbglog(DBG_CRITICAL, "MAP-NIL-TRAZA runtime_mapassign pc=%08x t=%08x\\n",
               (unsigned int)(uintptr_t)__builtin_return_address(0), (unsigned int)(uintptr_t)t);
        runtime_panicstring("assignment to entry in nil map");
        return g_zero_value;
    }'''
VIEJO_MACRO = '''    if (h == NULL) {                                                             \\
        runtime_panicstring("assignment to entry in nil map");                   \\
        return g_zero_value;                                                     \\
    }                                                                            \\'''
NUEVO_MACRO = '''    if (h == NULL) {                                                             \\
        /* DC-002.15: la traza (mismo motivo que en runtime_mapassign). */        \\
        dbglog(DBG_CRITICAL, "MAP-NIL-TRAZA %s pc=%08x t=%08x\\n",                \\
               __func__, (unsigned int)(uintptr_t)__builtin_return_address(0),    \\
               (unsigned int)(uintptr_t)t);                                      \\
        runtime_panicstring("assignment to entry in nil map");                   \\
        return g_zero_value;                                                     \\
    }                                                                            \\'''


def reemplazar(path, viejo, nuevo):
    t = path.read_text()
    n = t.count(viejo)
    if n != 1:
        sys.exit("ERROR: el texto a reemplazar aparece %d veces (esperaba 1) en %s" % (n, path))
    path.write_text(t.replace(viejo, nuevo, 1))
    print("  ok %s" % path.name)


reemplazar(DEFER, VIEJO_PANIC, NUEVO_PANIC)
reemplazar(MAPA, VIEJO_FUNC, NUEVO_FUNC)
reemplazar(HEADER, VIEJO_MACRO, NUEVO_MACRO)

print("== 3/3 el runtime")
if "--build" in sys.argv:
    # OJO (medido): `%.o: %.c` sin dependencias de cabeceras y `make clean` se lleva el kos.gox.
    subprocess.run(["bash", "-c", f"rm -f {LIB}/runtime/*.o {LIB}/*.a"], check=True)
    r = subprocess.run(["make", "-C", LIB, "all"], capture_output=True, text=True)
    print("   make: " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    if r.returncode != 0:
        print(r.stdout[-3000:] + r.stderr[-3000:])
        sys.exit("make fallo (si es por dbglog: agregar #include <kos/dbglog.h> en map_dreamcast.c)")
    print(f"   {LIB}/libgodc.a: {os.path.getsize(LIB + '/libgodc.a')} bytes, sha256 {sha(pathlib.Path(LIB + '/libgodc.a'))}")
else:
    print("   falta --build")
print(f"   map_dreamcast.c sha256={sha(MAPA)}   map_fast_internal.h sha256={sha(HEADER)}")
print("   ahora: reenlazar el motor y correr Flycast; en el log buscar 'MAP-NIL-TRAZA'")
