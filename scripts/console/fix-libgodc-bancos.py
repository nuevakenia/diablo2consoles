#!/usr/bin/env python3
"""INSTRUMENTO DESCARTADO (NO ES EL ARREGLO): derrama los BANCOS DEL QUE LLAMA (r8..r13) a la pila en los
puntos de asignacion del runtime (DC-002.39b).

MEDIDO: NO ARREGLA NADA. Con este parche los fallos del POC `banco-vivo` son EXACTAMENTE los mismos que sin
el (184 de 2560, en los mismos indices: ronda= 1 i= 56, ronda= 2 i= 42, ronda= 3 i= 21, ...), asi que el
valor que se cae NO esta en esos bancos. Queda en el repo como el instrumento con el que se descarto la
hipotesis del registro (que era la que suponia DC-002.39), y porque volcarlos no cuesta nada.
EL ARREGLO DE VERDAD es `fix-libgodc-candidato.py` (el portero del barrido conservador rechazaba los
punteros INTERIORES): con ese, banco-vivo da 0 fallos de 2560. Evidencia: evidencia/dc-002.39b-candidato-
interior.txt.

SINTOMA (medido, DC-002.39 y DC-002.39b)
    La consulta de los 21.253 nombres del corpus de DC-002.33 pierde entre 0 y 6 nombres por corrida
    (control 2b), y el POC chiquito `console/dc/poc/banco-vivo` lo reproduce en 30 s: 184 de 2560
    comparaciones salen con el VALOR DEL BUCLE viejo.

        BANCO-VIVO-FALLO ronda= 1 i= 56 leidoPtr= 2350049200 leidoLen= 13 vivoPtr= 2350615008 vivoLen= 13

    El nombre que falla esta BIEN FORMADO y el MISMO nombre pedido de nuevo SI se encuentra
    (`segundo intento contiene= true`): el DATO esta bien; el que queda viejo es el VALOR que el bucle
    tiene EN LA MANO cuando el cuerpo (abrir y leer el MPQ) dispara una recoleccion. En la corrida del
    corpus el largo de ese valor se leyo de memoria que ya era de otro objeto (`largoNombre= -1795502946`).

QUE PASA, PASO A PASO
    1. `gc_collect()` es un recolector de COPIA (semiespacios de 512 KB): cuando mueve un objeto tiene que
       ACTUALIZAR todas las referencias que queden apuntandolo.
    2. Las raices que barre son: las globales registradas por el compilador, todas las pilas de goroutine y
       la pila del que recolecta (`gc_scan_stack`, desde el SP del recolector hasta el tope del stack).
    3. `gc_scan_registers()` se borro del runtime, con este comentario por escrito:
       "NOTE: gc_scan_registers was removed. It couldn't update registers after copying, so it was
        useless. Stack scanning catches spilled registers."
       Y es cierto a medias: el barrido de la pila caza los registros que ALGUIEN DERRAMO. Un valor vivo en
       un registro CALLE-SAVED (r8..r13) que ningun marco intermedio derrame no esta en la pila ni en
       ninguna otra parte que el recolector mire.
    4. Por que r8..r13 (y no r0..r7 ni r14): el ABI del SH-4 dice que r8..r14 los preserva el LLAMADO, y
       GCC los derrama SOLO si la funcion los usa. r0..r7 son de paso (el que llama no espera que
       sobrevivan a una llamada, asi que no puede tener un valor vivo ahi cuando llama). r14, con
       `-fno-omit-frame-pointer` como compila libgodc, es el puntero al marco: el prologo de CUALQUIER
       funcion lo empuja a la pila y lo repone al salir, o sea que su valor ya es visible
       (medido con `sh-elf-gcc -S`: el control sin el derrame empuja r14 y NO empuja r8..r13).
    5. Por eso derramar y recargar r8..r13 ALREDEDOR de `gc_collect` (lo que se probo en DC-002.39) no
       alcanza: es tarde. Para que el derrame sirva tiene que hacerse DONDE EL VALOR TODAVIA ES DEL QUE
       LLAMA y su reposicion llegar hasta el, o sea en la ENTRADA del camino que el codigo de Go llama.

EL ARREGLO
    En la entrada de los dos caminos que el codigo de Go llama DIRECTAMENTE y que pueden recolectar --
    `gc_alloc()` (toda asignacion: `runtime_newobject`, `runtime_mallocgc`, `runtime_makeslice`,
    `runtime_makemap`, los `append`, las cadenas...) y `runtime_GC()` (`runtime.GC()` del codigo) -- se
    le pide AL COMPILADOR que deje r8..r13 en la pila del propio marco:

        #define GC_BANCOS_DEL_QUE_LLAMA() __asm__ volatile("" ::: "r8", "r9", "r10", "r11", "r12", "r13")

    No es codigo que se ejecute: es el clobber de un `asm` vacio. GCC tiene que PRESERVAR los registros
    calle-saved a traves de el, asi que los empuja en el prologo y los repone en el epilogo (medido en el
    `.s`: `mov.l r8,@-r15 ... mov.l r13,@-r15` arriba y los seis `mov.l @r15+,rN` abajo). Con eso:
      * los valores del que llama pasan a estar EN LA PILA durante toda la asignacion (incluida la
        recoleccion que caiga adentro), asi que `gc_scan_stack` los VE y los ACTUALIZA como cualquier otra
        raiz;
      * el epilogo los repone YA ACTUALIZADOS en r8..r13, o sea que el marco de Go sigue con su valor
        bueno, no con el que apuntaba al semiespacio viejo.

    Costo declarado (medido con `sh-elf-gcc -S`): 6 `mov.l` de mas en el prologo y 6 en el epilogo de
    `gc_alloc`. Una asignacion es un camino caliente, asi que es un costo real y se dice: son 12 accesos a
    memoria por asignacion, a cambio de que la recoleccion no pierda valores vivos.

USO
    python3 scripts/console/fix-libgodc-bancos.py                 # aplica (idempotente)
    python3 scripts/console/fix-libgodc-bancos.py --build         # ademas rehace libgodc.a/libgodcbegin.a
    python3 scripts/console/fix-libgodc-bancos.py --sin-pristino  # encadenado (no re-resetea el arbol)
    python3 scripts/console/fix-libgodc-bancos.py --quitar --build # EL MUTANTE: lo saca y deja el resto
    python3 scripts/console/fix-libgodc-bancos.py --estado         # que dice el arbol (para el corpus)
    [ruta_libgodc]  (por defecto /home/dgalli/dreamcast/libgodc)

ORDEN E IDEMPOTENCIA
    Deja PRISTINOS los tres archivos que toca (`git checkout --` de gc_semispace.h, gc_heap.c y
    gc_runtime.c), vuelve a poner las MARCAS DEL ARRANQUE (que son parte de la cadena canonica y viven en
    dos de esos archivos: sin esto el reseteo se las llevaria) y recien despues aplica este arreglo. Los
    otros pasos de la cadena (punteros interiores, Store Queue, ABI, interfaces, semiespacio) no los toca:
    viven en otros archivos. Correrlo dos veces deja el mismo sha256.
"""
import hashlib
import os
import pathlib
import subprocess
import sys

QUITAR = "--quitar" in sys.argv
BUILD = "--build" in sys.argv
AQUI = os.path.dirname(os.path.abspath(__file__))
args = [a for a in sys.argv[1:] if not a.startswith("--")]
LIB = args[0] if args else "/home/dgalli/dreamcast/libgodc"

SEMISPACE = pathlib.Path(LIB + "/runtime/gc_semispace.h")
HEAP = pathlib.Path(LIB + "/runtime/gc_heap.c")
RUNTIME = pathlib.Path(LIB + "/runtime/gc_runtime.c")

MARCA = "DC-002.39b"

DEFINICION = '''/* ==================== LOS BANCOS DEL QUE LLAMA (DC-002.39b) ====================
 *
 * Pone los registros CALLE-SAVED del que llama (r8..r13) EN LA PILA del marco que asignó, para que el
 * barrido conservador del stack los vea y los ACTUALICE cuando el recolector de copia mueve el objeto.
 *
 * Un valor vivo en un registro que ningun marco intermedio derrama no esta en la pila: el recolector lo
 * mueve y el valor queda apuntando al semiespacio viejo (el defecto medido en DC-002.39/DC-002.39b:
 * `BANCO-VIVO-FALLO ... leidoPtr= 2350049200 vivoPtr= 2350615008`). El `asm` va VACIO a proposito: no es
 * codigo que corra, es el `clobber` que obliga a GCC a derramar esos registros en el prologo y a
 * reponerlos (ya actualizados) en el epilogo.
 *
 * r14 no hace falta (con -fno-omit-frame-pointer es el puntero al marco: cualquier prologo de C lo
 * derrama) y r0..r7 no pueden tener un valor vivo de Go cuando se llama (son de paso en el ABI del SH-4).
 */
#define GC_BANCOS_DEL_QUE_LLAMA() __asm__ volatile("" ::: "r8", "r9", "r10", "r11", "r12", "r13")
'''

ANCLA_DEF = "#define GC_SEMISPACE_SIZE ((GC_SEMISPACE_SIZE_KB) * 1024)"

LLAMADA = ("    GC_BANCOS_DEL_QUE_LLAMA();   /* %s: los r8..r13 del que llama, a la pila, para que el"
           " barrido del stack los vea */" % MARCA)

ANCLA_ALLOC = "void *gc_alloc(size_t size, struct __go_type_descriptor *type)\n{"
ANCLA_GC = "void runtime_GC(void)\n{"


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()[:16]


def reemplazar(path, viejo, nuevo):
    t = path.read_text()
    if nuevo in t:
        print("   ya estaba en %s" % path.name)
        return
    n = t.count(viejo)
    if n != 1:
        sys.exit("ERROR: el texto a reemplazar aparece %d veces (esperaba 1) en %s" % (n, path))
    path.write_text(t.replace(viejo, nuevo, 1))
    print("   ok %s" % path.name)


def sacar_linea(path, texto):
    lineas = path.read_text().splitlines(keepends=True)
    quedan = [l for l in lineas if texto not in l]
    if len(quedan) == len(lineas):
        print("   %s: ya no estaba" % path.name)
        return
    path.write_text("".join(quedan))
    print("   ok %s: linea del arreglo sacada" % path.name)


def estado():
    t = SEMISPACE.read_text() if SEMISPACE.exists() else ""
    aplicado = MARCA in t
    print("bancos: %s en %s (sha256 %s)" % ("APLICADO" if aplicado else "QUITADO",
                                            SEMISPACE, sha(SEMISPACE) if SEMISPACE.exists() else "-"))
    return aplicado


if "--estado" in sys.argv:
    print("== estado del arreglo de los bancos del que llama (DC-002.39b)")
    sys.exit(0 if estado() else 1)

if "--sin-pristino" in sys.argv:
    print("== 1/3 sin tocar el arbol (encadenado: el arbol ya lo dejo listo quien llamo)")
else:
    print("== 1/3 el vendor pristino de los TRES archivos de este arreglo")
    for f in ("runtime/gc_semispace.h", "runtime/gc_heap.c", "runtime/gc_runtime.c"):
        r = subprocess.run(["git", "-C", LIB, "checkout", "--", f], capture_output=True, text=True)
        if r.returncode != 0:
            sys.exit("git checkout de %s fallo: %s" % (f, r.stderr))
    print("   pristinos (git checkout): gc_semispace.h, gc_heap.c, gc_runtime.c")
    # Las MARCAS DEL ARRANQUE (DC-002.29) viven en gc_heap.c y gc_runtime.c: son parte de la cadena
    # canonica del vendor. Si no se reponen, este reseteo se las lleva y el corpus pierde las lineas
    # ARQ-MARCA que documenta la evidencia.
    r = subprocess.run([sys.executable, os.path.join(AQUI, "fix-libgodc-marcas-arranque.py"), LIB],
                       capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout + r.stderr)
        sys.exit("fix-libgodc-marcas-arranque.py fallo")
    print("   " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])

print("== 2/3 el arreglo: los bancos del que llama, a la pila")
if QUITAR:
    print("   --quitar: NO se aplica (el arbol queda con la cadena canonica y sin este arreglo: es el MUTANTE)")
    sacar_linea(SEMISPACE, "GC_BANCOS_DEL_QUE_LLAMA()")
    sacar_linea(HEAP, LLAMADA)
    sacar_linea(RUNTIME, LLAMADA)
    if MARCA in SEMISPACE.read_text():
        sys.exit("ERROR: quedaron restos del arreglo en %s" % SEMISPACE)
else:
    reemplazar(SEMISPACE, ANCLA_DEF, ANCLA_DEF + "\n\n" + DEFINICION.rstrip("\n"))
    reemplazar(HEAP, ANCLA_ALLOC + "\n", ANCLA_ALLOC + "\n" + LLAMADA + "\n")
    reemplazar(RUNTIME, ANCLA_GC + "\n", ANCLA_GC + "\n" + LLAMADA + "\n")

estado()

print("== 3/3 el runtime")
if BUILD:
    subprocess.run(["bash", "-c", "rm -f %s/runtime/*.o %s/*.a" % (LIB, LIB)], check=True)
    r = subprocess.run(["make", "-C", LIB, "all"], capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout[-4000:] + r.stderr[-4000:])
        sys.exit("make fallo")
    print("   make ok: " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    print("   %s/libgodc.a: %d bytes, sha256 %s" % (LIB, os.path.getsize(LIB + "/libgodc.a"),
                                                    sha(pathlib.Path(LIB + "/libgodc.a"))))
else:
    print("   falta: rm -f %s/runtime/*.o %s/*.a && make -C %s all   (o correr con --build)" % (LIB, LIB, LIB))
print("   ahora: reenlazar el POC (`build-dc.sh` o `build-motor-arranque.sh`) y correr Flycast;")
print("          el numero es `BANCO-VIVO-TOTAL ... fallos= 0` en la consola serie")
