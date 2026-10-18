#!/usr/bin/env python3
"""MARCAS DEL ARRANQUE DEL RUNTIME (DC-002.29): la LINEA exacta donde muere el arranque.

SINTOMA (tarjeta DC-002.29, medido en DC-002.28)
    El binario del motor rearmado (recompilar UN paquete del motor y reenlazar) NO arranca: con los gox de
    DC-002.27 el mismo POC corre hasta el cuadro 2; con cualquier gox rearmado no imprime NADA. La tarjeta
    lo declara como "cuelgue mudo adentro del arranque del runtime de Go".

LO QUE ESTAS MARCAS MIDIERON (corrida del 2026-10-17, Flycast)
    El runtime NO es el que muere: `main()` corre ENTERO. El binario enfermo imprime, por la consola serie
    que el constructor de C ya abrio:

        ARQ-MARCA ctor-kos_startup v=0
        C-CONSTRUCTOR (antes de main): ... arena=94368 libre=1864
        ARQ-MARCA main-entra / runtime_init-entra
        ARQ-MARCA   gc_init-entra / gc_init-semiespacio0 0x8cd808c0 / gc_init-semiespacio1 0x8ce008e0
        ARQ-MARCA   gc_init-OK / map_init-OK / sudog_pool_init-OK / stack_pool_preallocate-OK /
                    proc_init-OK / panic_init-OK
        ARQ-MARCA runtime_init-volvio v=0
        ARQ-MARCA go_go-volvio-goroutines v=2
        ARQ-MARCA scheduler-entra v=2
        ARQ-MARCA main_wrapper-entra v=0
        (y de ahi: REINICIO del bus, sin una linea mas)

    O sea: la muerte esta ADENTRO de `___go_init_main` (los `init` de los ~118 paquetes que el enlazador
    genera a partir de los .gox). Para el indice exacto esta scripts/console/marcas-init-elf.py, que no
    toca Go: parchea el ELF enlazado.

    OJO CON LA FORMA DEL SINTOMA: en el binario sin instrumentar el mismo arreglo de gox SALE por el final
    de main (KOS imprime `arch: exit return code 0` y reinicia), y con las marcas puestas muere antes, con
    un reinicio sin mensaje. Las dos formas son el mismo defecto de layout: NO clavarse en una.

QUE ESCRIBE (todo lineas propias, idempotente y conviviente con los otros instrumentos)
    runtime/marcas_arranque.c  -- `godc_marca(etiqueta, valor)` y `godc_init_marca(idx)`: abren la consola
                                  serie si hace falta y escriben con dbgio (sin printf de Go).
    y lineas de enganche en:
      runtime/kos_startup.c   un constructor: la marca MAS TEMPRANA posible (antes de main)
      runtime/go-main.c       main(): entrada, runtime_init de vuelta, __go_go de vuelta (con el contador
                              de goroutines), entrada y VUELTA del scheduler; main_wrapper: antes de los
                              `init` de los paquetes, despues de ellos, antes de main.main
      runtime/gc_runtime.c    runtime_init(): una marca por paso
      runtime/gc_heap.c       gc_init(): la marca y las dos direcciones del semiespacio (memalign)

    Nada de esto toca el lado de Go: las marcas son C del runtime, para no mover el monton de Go (la
    leccion medida de DC-002.22: un `println` de mas en Go cambia el defecto).

USO
    python3 scripts/console/fix-libgodc-marcas-arranque.py --build            # escribe + rehace libgodc
    python3 scripts/console/fix-libgodc-marcas-arranque.py --quitar --build   # lo saca
    # despues: reenlazar el POC (build-motor-arranque.sh) y correr Flycast; buscar 'ARQ-MARCA'

    Cada linea que agrega lleva `ARQ-MARCA` en un comentario, y `--quitar` borra exactamente esas lineas:
    no toca las de la sonda del PC (DC-002.24b) ni las de los arreglos del vendor.
"""
import hashlib
import os
import pathlib
import subprocess
import sys

args = [a for a in sys.argv[1:] if not a.startswith("--")]
LIB = args[0] if args else os.environ.get("LIBGODC", "/home/dgalli/dreamcast/libgodc")
QUITAR = "--quitar" in sys.argv

MARCA = "ARQ-MARCA (DC-002.29)"

NUEVO = pathlib.Path(LIB + "/runtime/marcas_arranque.c")
MAIN = pathlib.Path(LIB + "/runtime/go-main.c")
RUNTIME = pathlib.Path(LIB + "/runtime/gc_runtime.c")
HEAP = pathlib.Path(LIB + "/runtime/gc_heap.c")
STARTUP = pathlib.Path(LIB + "/runtime/kos_startup.c")
COPY = pathlib.Path(LIB + "/runtime/gc_copy.c")

# las declaraciones sueltas que la version anterior del script dejaba sin marca: se sacan igual
SUELTAS = (
    "extern void godc_marca(const char *etiqueta, unsigned long valor);",
    "extern uint32_t goroutine_count;",
    "extern unsigned int goroutine_count;",
    "extern void godc_init_marca(unsigned long idx);",
)

CABECERA = """/* libgodc/runtime/marcas_arranque.c -- DC-002.29
 *
 * LAS MARCAS DEL ARRANQUE DEL RUNTIME. Lo escribe scripts/console/fix-libgodc-marcas-arranque.py (NO
 * editar a mano: se pisa). El porque y como se lee estan en la cabecera de ese script.
 *
 * Escribe con el dispositivo de consola de KOS (dbgio) y no con el printf de newlib: el arranque del
 * runtime corre antes que cualquier otra cosa y lo unico probado ahi es lo que uso la sonda de DC-002.24b.
 */
#include <kos/dbgio.h>
#include <kos.h>

static int marcas_listas = 0;

static void marcas_abre_consola(void)
{
    if (!marcas_listas) {
        dbgio_dev_select("scif");
        marcas_listas = 1;
    }
}

void godc_marca(const char *etiqueta, unsigned long valor)
{
    marcas_abre_consola();
    dbgio_printf("ARQ-MARCA %s v=%lu\\n", etiqueta, valor);
}

/* La marca del INDICE de `init` de paquete: la llaman los trozos que scripts/console/marcas-init-elf.py
 * mete en el pool de literales de `___go_init_main` (parcheando el ELF enlazado: no se toca una linea de
 * Go). El indice es el orden en que el enlazador genera las llamadas. */
void godc_init_marca(unsigned long idx)
{
    marcas_abre_consola();
    dbgio_printf("ARQ-INIT idx = %lu\\n", idx);
}

/* Cuantas recolecciones lleva el GC: la marca de gc_collect() la incrementa (ver gox de marcas). */
unsigned long marca_gc_n = 0;
unsigned long marca_gc_bytes = 0;
"""

CTOR = [
    "",
    "/* %s: la marca mas temprana posible." % MARCA,
    "   Un constructor de C de los primeros objetos que el linker pone en .ctors (libgodcbegin va con",
    "   --whole-archive y ANTES de las demas libs): si esta sale y la de main() no, la muerte esta entre",
    "   los constructores y main. */",
    "extern void godc_marca(const char *etiqueta, unsigned long valor);   /* %s */" % MARCA,
    "__attribute__((constructor))   /* %s */" % MARCA,
    "static void marcas_ctor_kos_startup(void)   /* %s */" % MARCA,
    "{",
    '    godc_marca("ctor-kos_startup", 0);   /* %s */' % MARCA,
    "}",
]


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()[:16]


def quitar_lineas(p):
    """borra TODAS las lineas que agrego este instrumento (llevan `ARQ-MARCA` en un comentario)"""
    if not p.exists():
        return 0
    ls = p.read_text().splitlines(True)
    fuera = set()
    for i, l in enumerate(ls):
        if "ARQ-MARCA" in l or l.strip() in SUELTAS:
            fuera.add(i)
    # el bloque del constructor: el comentario de varias lineas (solo la primera lleva la marca) y la
    # funcion que le sigue, hasta su llave de cierre
    for i, l in enumerate(ls):
        if "mas temprana posible" in l or "pone en .ctors" in l:
            j = i
            while j < len(ls) and ls[j].strip() != "}":
                fuera.add(j)
                j += 1
            if j < len(ls):
                fuera.add(j)
    quedan = [l for i, l in enumerate(ls) if i not in fuera]
    p.write_text("".join(quedan))
    return len(ls) - len(quedan)


def normaliza(p):
    """el instrumento inserta y saca lineas: deja el archivo sin las lineas en blanco que quedan sueltas
    (para que el diff del vendor sea exactamente el que era cuando el instrumento no estaba)"""
    if not p.exists():
        return
    t = p.read_text()
    while "\n\n\n" in t:
        t = t.replace("\n\n\n", "\n\n")
    p.write_text(t.rstrip("\n") + "\n")


def inserta_despues(p, ancla, lineas, cuantas=1):
    """inserta `lineas` justo despues de la linea `ancla` (exacta)"""
    ls = p.read_text().splitlines(True)
    idx = [i for i, l in enumerate(ls) if l.rstrip("\n") == ancla]
    if len(idx) != cuantas:
        sys.exit("ERROR: el ancla %r de %s aparece %d veces, esperaba %d" % (ancla, p.name, len(idx), cuantas))
    for i in reversed(idx):
        ls[i + 1:i + 1] = [l if l.endswith("\n") else l + "\n" for l in lineas]
    p.write_text("".join(ls))


def inserta_tras_llave(p, cabecera, lineas, cuantas=1):
    """inserta `lineas` justo despues de la llave `{` de la funcion cuya cabecera es `cabecera`"""
    ls = p.read_text().splitlines(True)
    idx = [i for i, l in enumerate(ls) if l.rstrip("\n") == cabecera]
    if len(idx) != cuantas:
        sys.exit("ERROR: la cabecera %r de %s aparece %d veces" % (cabecera, p.name, len(idx)))
    for i in reversed(idx):
        j = i
        while ls[j].strip() != "{":
            j += 1
        ls[j + 1:j + 1] = [l + "\n" for l in lineas]
    p.write_text("".join(ls))


def marca(txt, valor="0"):
    return ['    godc_marca("%s", %s);   /* %s */' % (txt, valor, MARCA)]


def reemplaza(p, viejo, nuevo, cuantas=1):
    """cambia un texto exacto por otro (para meter una marca DENTRO de un bloque, sin romper las llaves)"""
    t = p.read_text()
    n = t.count(viejo)
    if n != cuantas:
        sys.exit("ERROR: el texto %r de %s aparece %d veces, esperaba %d" % (viejo[:40], p.name, n, cuantas))
    p.write_text(t.replace(viejo, nuevo))


def poner():
    if NUEVO.exists() and NUEVO.read_text() == CABECERA:
        print("   marcas_arranque.c ya estaba igual (idempotente)")
    else:
        NUEVO.write_text(CABECERA)
        print("   ok runtime/marcas_arranque.c")

    for p in (STARTUP, MAIN, RUNTIME, HEAP, COPY):
        n = quitar_lineas(p)
        if n:
            print("   %s: %d lineas viejas del instrumento sacadas" % (p.name, n))

    inserta_despues(STARTUP, "uint32_t _main_thread_stack_size = KOS_MAIN_STACK_SIZE;", CTOR)
    print("   ok kos_startup.c: constructor de marcas")

    inserta_despues(MAIN, "#include <kos.h>",
                    ["", "extern void godc_marca(const char *etiqueta, unsigned long valor);   /* %s */" % MARCA,
                     "/* goroutine_count ya lo declara runtime/goroutine.h */   /* %s */" % MARCA])
    inserta_tras_llave(MAIN, "__attribute__((no_split_stack)) int main(int argc, char **argv)", marca("main-entra"))
    inserta_despues(MAIN, "    runtime_init();", marca("runtime_init-volvio"))
    inserta_despues(MAIN, "    __go_go(main_wrapper, NULL);",
                    ['    godc_marca("go_go-volvio-goroutines", (unsigned long)goroutine_count);   /* %s */' % MARCA,
                     '    godc_marca("scheduler-entra", (unsigned long)goroutine_count);   /* %s */' % MARCA])
    inserta_despues(MAIN, "    scheduler_run_loop();",
                    ['    godc_marca("scheduler-VOLVIO", (unsigned long)goroutine_count);   /* %s */' % MARCA])
    inserta_despues(MAIN, '    extern void go_init_main(void) __asm__("___go_init_main");',
                    marca("main_wrapper-entra"))
    inserta_despues(MAIN, "    go_init_main();", marca("inits-de-paquetes-OK"))
    inserta_despues(MAIN, '    extern void main_dot_main(void) __asm__("_main.main");',
                    marca("main.main-entra"))
    inserta_despues(MAIN, "    main_dot_main();", marca("main.main-volvio"))
    print("   ok go-main.c: main() y main_wrapper")

    inserta_despues(RUNTIME, "extern void sudog_pool_init(void);",
                    ["extern void godc_marca(const char *etiqueta, unsigned long valor);   /* %s */" % MARCA])
    inserta_tras_llave(RUNTIME, "__attribute__((no_split_stack)) void runtime_init(void)",
                       marca("runtime_init-entra"))
    for paso in ("gc_init_impl();", "map_init();", "sudog_pool_init();", "stack_pool_preallocate();",
                 "proc_init();", "panic_init();"):
        inserta_despues(RUNTIME, "    " + paso, marca("  " + paso[:-1] + "-OK"))
    print("   ok gc_runtime.c: runtime_init paso por paso")

    inserta_despues(HEAP, "#include <kos/dbglog.h>",
                    ["extern void godc_marca(const char *etiqueta, unsigned long valor);   /* %s */" % MARCA])
    inserta_tras_llave(HEAP, "void gc_init(void)", marca("  gc_init-entra"))
    inserta_despues(HEAP, "    gc_heap.space[1] = (uint8_t *)memalign(32, GC_SEMISPACE_SIZE);",
                    ['    godc_marca("  gc_init-semiespacio0", (unsigned long)(uintptr_t)gc_heap.space[0]);   /* %s */' % MARCA,
                     '    godc_marca("  gc_init-semiespacio1", (unsigned long)(uintptr_t)gc_heap.space[1]);   /* %s */' % MARCA])
    print("   ok gc_heap.c: gc_init")

    # --- 5. gc_copy.c y gc_heap.c: LA COLECCION (el GC que corre durante los `init` del motor)
    inserta_despues(COPY, '#include "dc_platform.h"',
                    ["extern void godc_marca(const char *etiqueta, unsigned long valor);   /* %s */" % MARCA,
                     "extern unsigned long marca_gc_n;   /* %s */" % MARCA])
    inserta_tras_llave(COPY, "void gc_collect(void)",
                       ['    godc_marca("gc_collect", (unsigned long)(++marca_gc_n));   /* %s */' % MARCA])
    inserta_despues(COPY, "    size_t remaining = gc_heap.alloc_limit - gc_heap.alloc_ptr;",
                    ['    godc_marca("gc_if_needed-pidio", (unsigned long)requested_size);   /* %s */' % MARCA])
    # OJO: los dos `if` de abajo van SIN llaves en el original. Meter la marca "detras de la linea del if"
    # dejaria la sentencia siguiente fuera del if (el GC recolectaria en CADA asignacion): se reescribe el
    # bloque completo.
    reemplaza(HEAP, "        if (used > threshold)\n            gc_collect();",
              "        if (used > threshold)\n"
              "        {   /* %s */\n" % MARCA +
              '            godc_marca("gc-por-umbral-bytes-usados", (unsigned long)used);   /* %s */\n' % MARCA +
              "            gc_collect();\n"
              "        }   /* %s */" % MARCA)
    reemplaza(HEAP,
              "        if (gc_heap.alloc_ptr + total_size > gc_heap.alloc_limit)\n"
              '            runtime_throw("out of memory");',
              "        if (gc_heap.alloc_ptr + total_size > gc_heap.alloc_limit)\n"
              "        {   /* %s */\n" % MARCA +
              '            godc_marca("OOM-throw", (unsigned long)total_size);   /* %s */\n' % MARCA +
              '            runtime_throw("out of memory");\n'
              "        }   /* %s */" % MARCA)
    print("   ok gc_copy.c + gc_heap.c: el GC (cuantas colecciones y con cuanto heap)")

    for p in (STARTUP, MAIN, RUNTIME, HEAP, COPY):
        normaliza(p)


def sacar():
    if NUEVO.exists():
        NUEVO.unlink()
        print("   runtime/marcas_arranque.c borrado")
    else:
        print("   marcas_arranque.c ya no estaba")
    for p in (STARTUP, MAIN, RUNTIME, HEAP, COPY):
        print("   ok %s: %d lineas del instrumento sacadas" % (p.name, quitar_lineas(p)))
        normaliza(p)


print("== las marcas del arranque del runtime (DC-002.29)")
if QUITAR:
    sacar()
else:
    poner()

if "--build" in sys.argv:
    subprocess.run(["bash", "-c", "rm -f %s/runtime/*.o %s/*.a" % (LIB, LIB)], check=True)
    r = subprocess.run(["make", "-C", LIB, "all"], capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout[-4000:] + r.stderr[-4000:])
        sys.exit("make fallo")
    print("   make ok")
    for lib in ("libgodc.a", "libgodcbegin.a"):
        print("   %s: %d bytes, sha256 %s" % (lib, os.path.getsize(LIB + "/" + lib),
                                              sha(pathlib.Path(LIB + "/" + lib))))

print("   ahora: reenlazar el POC (build-motor-arranque.sh) y correr Flycast; buscar 'ARQ-MARCA' en el log")
