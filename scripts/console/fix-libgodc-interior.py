#!/usr/bin/env python3
"""Pone al dia los PUNTEROS INTERIORES del GC de libgodc (las cadenas que apuntan al medio de otro objeto).

SINTOMA (medido, DC-002.36)
    El POC del MENU PRINCIPAL en la Dreamcast lee su plan de dibujo del disco (PLAN.TXT, 19.187 B) y
    despues pide los 11 assets que el plan nombra. Lo que imprime la consola:

        MENU236: TEXTURA id= 0 FONDO tipo= fondo tam= 800 x 600 ... crc32= 1622281050
        MENU236: TEXTURA id= 1 LOGO_NEGRO_IZQ tipo= logo tam= 159 x 122 ... crc32= 71136314
        MENU236: TEXTURA id= 2 <basura> tipo= <basura> ...
        MENU236: TEXTURA FALLO id= 3 <basura> file does not exist
        ...                            (y lo mismo para los id 4 a 10)
        MENU236: LIENZOS listos ms= 7714 faltan= 8

    O sea: los NOMBRES de las texturas (que salen del plan) pasan a ser basura a mitad de la corrida y el
    POC no encuentra 8 de los 11 archivos. Antes de eso la corrida hizo recolecciones del GC
    (`ARQ-MARCA gc-por-threshold-bytes-usados`).

QUE PASA, PASO A PASO
    1. `strings.Fields(string(datos))` (y lo mismo el decodificador de json) devuelve CADENAS QUE SON
       PEDAZOS del buffer que se leyo: el puntero de la cadena cae en el MEDIO de un objeto del monton
       (un puntero INTERIOR). Es Go legal: el recolector tiene que mantener vivo el objeto que CONTIENE
       el puntero.
    2. El GC de libgodc no lo hace, a proposito y por escrito:
       `gc_is_valid_object_start()` ("Check if pointer is to a valid object start, REJECTING INTERIOR
       POINTERS") rechaza cualquier direccion que no sea un comienzo de objeto valido.
    3. Entonces `gc_update_pointer_field()` (`gc_copy_object()`) lee el "encabezado" de la MITAD de un
       objeto, no lo valida y deja el puntero como estaba: el buffer grande se queda sin ninguna
       referencia valida, la recoleccion siguiente se lo lleva y su memoria se reusa.
    4. Las cadenas siguen apuntando a esa direccion -- que ahora tiene los bytes de OTRO objeto -- y
       muestran basura. Medido: el largo del nombre del tercero del POC `tablas-mpq` salio
       `-1795502946` (DC-002.39, el mismo defecto).

EL ARREGLO
    El from-space de este GC es una SECUENCIA CONTIGUA de objetos (el asignador es un bump: cada objeto
    se asienta a continuacion del anterior y la recoleccion reinicia el puntero al comienzo). Entonces,
    desde el comienzo del from-space se puede CAMINAR objeto por objeto -- con el tamano que trae cada
    encabezado -- hasta dar con el que CONTIENE la direccion del puntero interior. Con el objeto
    encontrado se hace la copia normal (que deja su puntero de reenvio en el encabezado viejo) y el
    puntero nuevo es `nuevo_comienzo + desplazamiento`. El recorrido se acota con un CURSOR que se guarda
    entre punteros: las cadenas de un mismo buffer caen en el mismo objeto y el cursor ya esta ahi.

    Ademas se saca el filtro de ALINEACION a 8 bytes del barrido conservador de la pila: las cadenas
    cortadas con `strings.Fields` empiezan en cualquier byte, asi que el candidato tiene que llegar al
    resolutor (que es el que sabe decidir) y no descartarse por alineacion.

LIMITE DECLARADO (y por que se acepta)
    * El GC de esta consola es CONSERVADOR: tambien retiene objetos cuando una palabra de la pila solo lo
      PARECE. Aceptar punteros interiores amplia esa ambiguedad (una palabra que cae dentro de un objeto
      retiene ese objeto). Es el comportamiento normal de un recolector conservador.
    * Si un puntero interior cae, ademas, en una direccion alineada a 8 bytes y los bytes de ahi PARECEN un
      encabezado valido, se lo toma por comienzo de objeto (es el caso que ya existia antes de este arreglo
      para cualquier palabra dentro del rango del monton).

USO
    python3 scripts/console/fix-libgodc-interior.py                 # aplica (idempotente) sobre el pristino
    python3 scripts/console/fix-libgodc-interior.py --build         # ademas rehace libgodc.a
    python3 scripts/console/fix-libgodc-interior.py --sin-pristino  # para encadenar con otro instrumento
    python3 scripts/console/fix-libgodc-interior.py --quitar --build # EL MUTANTE: lo saca y deja el resto
    python3 scripts/console/fix-libgodc-interior.py --estado         # que dice el arbol (para el corpus)

ORDEN E IDEMPOTENCIA
    La cadena del runtime es: fix-libgodc-gc.py (pristino + ABI -> interfaces -> semiespacio) y despues
    fix-libgodc-sq.py (la copia por Store Queue). Este arreglo va DESPUES de los dos porque toca el mismo
    archivo (runtime/gc_copy.c) y se apoya en que las copias escriben bien. Correrlo dos veces deja el
    mismo sha256 del archivo (el reemplazo mira primero si el texto nuevo ya esta, la leccion de DC-002.25).

MEDIDO (POC sonda-cadenas y POC menu-dc-236, Flycast, rama DC-002)
    ANTES: `sonda-cadenas A ROJO B ROJO C OK` (las cadenas cortadas de un buffer grande se reciclan) y el
           POC del menu carga 3 de 11 texturas.
    DESPUES: `sonda-cadenas A OK B OK C OK` y el POC del menu carga 11 de 11.
"""
import hashlib
import os
import pathlib
import subprocess
import sys

args = [a for a in sys.argv[1:] if not a.startswith("--")]
LIB = args[0] if args else os.path.expanduser("~/dreamcast/libgodc")
AQUI = os.path.dirname(os.path.abspath(__file__))
COPY = pathlib.Path(LIB + "/runtime/gc_copy.c")
QUITAR = "--quitar" in sys.argv
BUILD = "--build" in sys.argv

# --- 1. el from-space y su tope, guardados al comienzo de la recoleccion ------------------------------
VIEJO_ESPACIOS = """    int old_space = gc_heap.active_space;
    int new_space = 1 - old_space;
"""
NUEVO_ESPACIOS = """    int old_space = gc_heap.active_space;
    int new_space = 1 - old_space;

    /* DC-002.36: el from-space y su TOPE, que es lo que hace falta para ubicar los punteros INTERIORES
       (las cadenas que apuntan al medio de otro objeto). Se guardan ANTES de dar vuelta el semiespacio:
       en cuanto el recolector sigue, `alloc_ptr` ya es el del espacio nuevo. */
    gc_from_lo = (uintptr_t)gc_heap.space[old_space];
    gc_from_hi = (uintptr_t)gc_heap.alloc_ptr;
    gc_walk_pos = gc_from_lo;
"""

# --- 2. la declaracion de las tres variables (arriba del archivo: gc_collect las usa y en C tienen que
#        estar declaradas antes) -----------------------------------------------------------------------
ANCLA_STATICS = """static void *gc_copy_object(void *ptr);
static void gc_scan_object(void *obj);"""

STATICS = """/* ==================== PUNTEROS INTERIORES (DC-002.36) ====================
 *
 * Una cadena de Go puede apuntar al MEDIO de un objeto mas grande: `strings.Fields`, `strings.Split` y el
 * decodificador de json devuelven PEDAZOS del buffer que se leyo. Es Go legal y el recolector tiene que
 * mantener vivo el objeto que CONTIENE el puntero, pero el GC de libgodc los rechazaba (el comentario de
 * `gc_is_valid_object_start` lo dice: "rejecting interior pointers"): el buffer grande quedaba sin
 * referencias validas, se lo llevaba la recoleccion y las cadenas pasaban a mostrar basura.
 *
 * El from-space es una SECUENCIA CONTIGUA de objetos (asignador bump), asi que desde su comienzo se puede
 * CAMINAR objeto por objeto hasta el que contiene la direccion. El cursor (`gc_walk_pos`) evita repetir el
 * recorrido cuando los punteros interiores caen en el mismo objeto, que es el caso normal (las cadenas de
 * un mismo archivo).
 *
 * LIMITE DECLARADO: el GC es CONSERVADOR y aceptar interiores amplia la ambiguedad (una palabra de la pila
 * que cae dentro de un objeto retiene ese objeto). Es el comportamiento normal de un recolector conservador.
 */
static uintptr_t gc_from_lo = 0;  /* comienzo del from-space (primer objeto) */
static uintptr_t gc_from_hi = 0;  /* tope del from-space (fin del ultimo objeto) */
static uintptr_t gc_walk_pos = 0; /* cursor del recorrido de objetos */

static void *gc_copy_object(void *ptr);
static void gc_scan_object(void *obj);"""

# --- 3. los dos ayudantes (el recorrido de objetos y el resolutor) ------------------------------------
ANCLA_HELPERS = """    return new_ptr;
}

/* --- Object Scanning --- */"""

HELPERS = """    return new_ptr;
}

/* Devuelve el comienzo del objeto que CONTIENE `addr` y deja el desplazamiento en `off`, o NULL si ahi no
 * hay ningun objeto (o si el recorrido encontro un encabezado que no cierra: entonces no se inventa nada).
 * Ver el comentario de las variables, arriba del archivo: es el arreglo de DC-002.36. */
static void *gc_interior_base(uintptr_t addr, uintptr_t *off)
{
    uintptr_t p;

    if (addr < gc_from_lo + GC_HEADER_SIZE || addr >= gc_from_hi)
        return NULL;

    p = gc_from_lo;

    if (gc_walk_pos > p && gc_walk_pos <= addr)
        p = gc_walk_pos;

    while (p + GC_HEADER_SIZE <= gc_from_hi)
    {
        gc_header_t *h = (gc_header_t *)p;
        size_t sz = GC_HEADER_GET_SIZE(h);

        if (sz < GC_HEADER_SIZE || sz > gc_heap.space_size || (sz & GC_ALIGN_MASK))
            return NULL;

        if (addr < p + sz)
        {
            /* la direccion cae en el ENCABEZADO y no en los datos: no es un puntero a datos */
            if (addr < p + GC_HEADER_SIZE)
                return NULL;

            gc_walk_pos = p;
            *off = addr - (p + GC_HEADER_SIZE);

            return (void *)(p + GC_HEADER_SIZE);
        }

        p += sz;
    }

    return NULL;
}

/* Pone al dia un puntero que cae en el from-space, sea el COMIENZO de un objeto o un puntero INTERIOR.
 * Deja `*actualizado = 1` solo si el puntero cambio de direccion. */
static void *gc_relocate(void *ptr, int *actualizado)
{
    uintptr_t addr = (uintptr_t)ptr;
    uintptr_t off = 0;
    void *base;

    *actualizado = 0;

    if (addr < gc_from_lo || addr >= gc_from_hi)
        return ptr;

    if (gc_is_valid_object_start(ptr))
    {
        void *nuevo = gc_copy_object(ptr);

        if (nuevo != ptr)
            *actualizado = 1;

        return nuevo;
    }

    base = gc_interior_base(addr, &off);
    if (base == NULL)
        return ptr;

    *actualizado = 1;

    return (void *)((uintptr_t)gc_copy_object(base) + off);
}

/* --- Object Scanning --- */"""

# --- 4. el resolutor en el unico lugar donde se pone al dia un puntero ---------------------------------
VIEJO_UPDATE = """    /*
     * Pointer IS in from-space. This is a heap pointer that needs to
     * be copied/forwarded. Call gc_copy_object to handle it.
     */
    void *new_ptr = gc_copy_object(old_ptr);

    // Update the field if the object was moved
    if (new_ptr != old_ptr)
    {
        *field = new_ptr;
    }
}"""

NUEVO_UPDATE = """    /*
     * El puntero esta EN el from-space: puede ser el COMIENZO de un objeto o un puntero INTERIOR (una
     * cadena que apunta al medio de un buffer mas grande). `gc_relocate` resuelve los dos casos: si es un
     * comienzo copia el objeto, y si es interior UBICA el objeto que lo contiene, lo copia y devuelve
     * `nuevo_comienzo + desplazamiento`. Antes de esto el interior se descartaba y la cadena quedaba
     * apuntando a memoria reciclada (DC-002.36).
     */
    int actualizado = 0;
    void *new_ptr = gc_relocate(old_ptr, &actualizado);

    if (actualizado)
    {
        *field = new_ptr;
    }
}"""

# --- 4. el filtro rapido del barrido conservador: sin el requisito de alineacion a 8 -------------------
VIEJO_FILTRO = """static inline bool gc_might_be_heap_ptr(uintptr_t val, heap_bounds_t bounds)
{
    /*
     * Single comparison trick.
     *
     * Instead of: val >= lo && val < hi
     * We use:     (val - lo) < (hi - lo)
     *
     * This works because if val < lo, the subtraction wraps around to a
     * huge positive number, which fails the < check. Two comparisons
     * become one subtraction + one comparison.
     *
     * Also checks alignment in the same expression.
     */
    return ((val - bounds.lo) < (bounds.hi - bounds.lo)) &&
           ((val & GC_ALIGN_MASK) == 0);
}"""

NUEVO_FILTRO = """static inline bool gc_might_be_heap_ptr(uintptr_t val, heap_bounds_t bounds)
{
    /*
     * Single comparison trick.
     *
     * Instead of: val >= lo && val < hi
     * We use:     (val - lo) < (hi - lo)
     *
     * This works because if val < lo, the subtraction wraps around to a
     * huge positive number, which fails the < check. Two comparisons
     * become one subtraction + one comparison.
     *
     * DC-002.36: ACA ESTABA EL FILTRO DE ALINEACION A 8 BYTES (`(val & GC_ALIGN_MASK) == 0`) y se saco:
     * las cadenas que salen de `strings.Fields` empiezan en CUALQUIER byte, asi que una cadena interior
     * no alineada quedaba descartada antes de que nadie la mirara. El candidato va al resolutor
     * (`gc_relocate`), que es el que sabe si ahi hay un objeto y si el puntero es interior. Lo que se
     * pierde es solo velocidad en el filtro: las palabras que caen DENTRO del rango del from-space son
     * pocas (~0,01% de una palabra al azar) y el resolutor las rechaza sin copiar nada.
     */
    return (val - bounds.lo) < (bounds.hi - bounds.lo);
}"""


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()[:16]


def reemplazar(path, viejo, nuevo):
    t = path.read_text()
    # DC-002.25: mirar SIEMPRE primero si el texto nuevo ya esta (el viejo puede ser una subcadena del
    # nuevo) o la cadena se reaplica en cada corrida y el archivo crece.
    if nuevo in t:
        print("  ya estaba %s" % path.name)
        return
    n = t.count(viejo)
    if n != 1:
        sys.exit("ERROR: el texto a reemplazar aparece %d veces (esperaba 1) en %s" % (n, path))
    path.write_text(t.replace(viejo, nuevo, 1))
    print("  ok %s" % path.name)


FIX_APLICADO = "static void *gc_relocate(void *ptr, int *actualizado)"


def estado():
    t = COPY.read_text() if COPY.exists() else ""
    aplicado = FIX_APLICADO in t
    print("interior: %s en %s (sha256 %s)" % ("APLICADO" if aplicado else "QUITADO",
                                             COPY, sha(COPY) if COPY.exists() else "-"))
    return aplicado


if "--estado" in sys.argv:
    print("== estado del arreglo de punteros interiores")
    sys.exit(0 if estado() else 1)

if "--sin-pristino" in sys.argv:
    print("== 1/2 sin tocar el arbol (encadenado: ya lo dejo listo quien llamo)")
else:
    # El vendor pristino de gc_copy.c. Lo hace ESTE script y no fix-libgodc-gc.py a proposito: su
    # encabezado dice, por escrito, que para los archivos de la copia por Store Queue (runtime/gc_copy.c)
    # no hace el `git checkout` y que "quien llama lo deja pristino". Es lo que hace
    # fix-libgodc-iface.py con sus dos archivos, y es lo que hace falta aca: sin esto, un estado a medias
    # del archivo (medido: quedo escrito el bloque del from-space sin la declaracion de las variables) no
    # lo arregla ninguna idempotencia, porque el texto no coincide ni con el viejo ni con el nuevo.
    print("== 1/2 el vendor pristino de gc_copy.c + la cadena documentada del runtime")
    subprocess.run(["git", "-C", LIB, "checkout", "--", "runtime/gc_copy.c"], check=True)
    print("   gc_copy.c: pristino (git checkout), sha256 %s" % sha(COPY))

    r = subprocess.run([sys.executable, os.path.join(AQUI, "fix-libgodc-sq.py"), LIB],
                       capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout + r.stderr)
        sys.exit("fix-libgodc-sq.py fallo")
    print("   " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])

print("== 2/2 los punteros interiores del GC")
if QUITAR:
    print("   --quitar: NO se aplica (el arbol queda con la cadena y sin este arreglo: es el MUTANTE)")
else:
    reemplazar(COPY, VIEJO_ESPACIOS, NUEVO_ESPACIOS)
    reemplazar(COPY, ANCLA_STATICS, STATICS)
    reemplazar(COPY, ANCLA_HELPERS, HELPERS)
    reemplazar(COPY, VIEJO_UPDATE, NUEVO_UPDATE)
    reemplazar(COPY, VIEJO_FILTRO, NUEVO_FILTRO)

estado()

if BUILD:
    subprocess.run(["bash", "-c", f"rm -f {LIB}/runtime/*.o {LIB}/*.a"], check=True)
    r = subprocess.run(["make", "-C", LIB, "all"], capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout[-4000:] + r.stderr[-4000:])
        sys.exit("make fallo")
    print("   make ok: " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    print(f"   {LIB}/libgodc.a: {os.path.getsize(LIB + '/libgodc.a')} bytes, "
          f"sha256 {sha(pathlib.Path(LIB + '/libgodc.a'))}")
else:
    print(f"   falta: rm -f {LIB}/runtime/*.o {LIB}/*.a && make -C {LIB} all   (o correr con --build)")
print("   ahora: reenlazar el POC y correr Flycast; el numero es `SONDA-CADENAS A OK` en la consola serie")
