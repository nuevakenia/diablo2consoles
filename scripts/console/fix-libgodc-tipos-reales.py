#!/usr/bin/env python3
"""DC-002.146d -- EL RECOLECTOR BARRIA TIPOS CON `__ptrdata` BASURA: la tabla de los descriptores REALES.

SINTOMA (medido en DC-002.145, commit de DC-002.146)
    El `panic: slice bounds out of range` del cruce de nivel aparecia LEJOS del recolector, pero la firma estaba en
    el log del recolector de libgodc:

        ARQ-MARCA gc-ptrdata-recortado   v=2349166024   (0x8c056dc8)
        ARQ-MARCA gc-gcdata-fuera-imagen v= 906232840   (0x36040408)

    Los dos valores son DIRECCIONES, no tamanos: el recolector estaba leyendo un "descriptor de tipo" que no lo era.
    0x8c056dc8 cae dentro de `.rodata` (medido en el ELF del POC de DC-002.146c: .text 0x8c010000..0x8c054f70,
    .rodata 0x8c054fd4..0x8c0607c8), o sea que el supuesto `__ptrdata` era una palabra de datos de la imagen.

POR QUE PASA (leido en `runtime/gc_copy.c`)
    El barrido conservador de la pila INVENTA encabezados: cualquier palabra que caiga en el from-space y que tenga
    cuatro bytes de "tamano" razonables por delante es, para el recolector, un objeto; su campo `type` es otra
    palabra cualquiera. DC-002.92 exigio que el tipo cayera en la IMAGEN (`gc_tipo_en_imagen`) y acoto el barrido al
    tamano del objeto, lo que impidio que el barrido se saliera de RAM. Pero la imagen es texto+datos+bss: adentro
    tambien hay codigo, cadenas y tablas, asi que "cae en la imagen" NO es "es un descriptor". Un tipo de basura que
    caiga ahi se barre con lo que diga su `__ptrdata` (una direccion de .rodata leida como tamano) y el objeto se
    llena de punteros fantasma; el panic de Go aparece despues, en el codigo que corria.

EL ARREGLO (una tabla, tres veredictos, y una marca por veredicto)
    A. LA TABLA DE LOS TIPOS REALES. El compilador le pasa el descriptor al asignador en CADA `gc_alloc()`, asi que
       la tabla se llena en el asignador (`gc_heap.c`) y todo objeto del monton tiene su tipo anotado antes de que
       el recolector lo vea (el objeto no existe hasta que `gc_alloc` vuelve). Sin heap propio: 4096 ranuras fijas
       (16 KB) y sondeo lineal; al 75 % deja de tomar tipos y lo avisa (`gc-tipos-tabla-llena`).
    B. LOS TRES VEREDICTOS (`gc_tipo_veredicto`, en `gc_copy.c`):
         REAL        esta en la tabla -> barrido preciso, como siempre.
         CON FORMA   no esta en la tabla pero su contenido tiene forma de descriptor -> barrido CONSERVADOR acotado
                     al objeto, con aviso (`gc-tipo-sin-registrar`): red de seguridad para un tipo compilado que
                     nunca asigno. Si aparece, la tabla tiene un hueco.
         INVALIDO    ni en la tabla ni con forma -> el encabezado es FALSO: se avisa (`gc-tipo-invalido`) y NO SE
                     BARRE (NOSCAN). Ya no se lee su `__ptrdata`, asi que la firma del defecto desaparece.
       La FORMA sola no alcanza y se midio: en el ELF del POC de DC-002.146c (343.872 B de imagen) hay 6 trozos de
       .text/.data que la pasan sin ser descriptores. Por eso el que manda es la tabla; la forma solo separa "hay que
       mirarlo" de "esto es basura".

    El parte de cada recoleccion (solo si hay algo que avisar): `gc-tipos-invalidos-total`,
    `gc-tipos-sin-registrar-total`; al arrancar: `gc-tipos-tabla` (ranuras) y `gc-tipos-registrados` (tipos distintos).

USO
    python3 scripts/console/fix-libgodc-tipos-reales.py --build             # el arreglo
    python3 scripts/console/fix-libgodc-tipos-reales.py --quitar --build    # EL MUTANTE: vuelve al arreglo de .92
    python3 scripts/console/fix-libgodc-tipos-reales.py --estado
    (el primer argumento sin guiones es el arbol de libgodc; por defecto ~/dreamcast/libgodc)

ORDEN: este parche se apoya en el de DC-002.92 (`fix-libgodc-tipos.py`), que es el que agrega el sensor y el
barrido conservador acotado; `--quitar` devuelve el arbol exactamente a ese estado. Si el arbol no tiene el de .92,
`reemplazar` corta con "el texto aparece 0 veces" -- correr ese primero.

LO QUE SE MIDE
    `console/dc/poc/gc-tipo-invalido` (una sonda de 90 lineas de Go, sin motor): falsifica el tipo de un objeto VIVO con
    un descriptor que SI cae en la imagen y cuyo `__ptrdata` es una direccion de la imagen y `__gcdata` basura fuera
    de RAM (la firma exacta). Con el arreglo: `gc-tipo-invalido`, CERO `gc-ptrdata-recortado`, CERO
    `gc-gcdata-fuera-imagen` y el objeto de CONTROL (sano, con su tipo registrado) sale entero. Con el mutante: los
    tres marcadores del defecto, en la misma corrida.
    Comando:  scripts/console/checar-gc-tipo-invalido-146d.sh arreglo|mutante|veredicto

    Y la corrida GRANDE (la carga de la prueba, 12 relecturas del AutoMap.txt por el flujo del MPQ, plano de
    17 x 17 con `LIENZO_EN_RAM=no`): con el mutante 2 de 2 corridas mueren en la vuelta 0 (`REARME SE CAYO` +
    `FATAL: recover without checkpoint`); con el arreglo 2 de 2 terminan las 12 vueltas y cruzan (`CRUZA OK`) con
    CERO `gc-ptrdata-recortado`. Ver `console/dc/evidencia/dc-002.146d-el-tipo-invalido-del-recolector.txt`.
"""
import hashlib
import os
import pathlib
import subprocess
import sys

args = [a for a in sys.argv[1:] if not a.startswith("--")]
LIB = args[0] if args else os.path.expanduser("~/dreamcast/libgodc")
COPY = pathlib.Path(LIB + "/runtime/gc_copy.c")
HEAP = pathlib.Path(LIB + "/runtime/gc_heap.c")
CABEZA = pathlib.Path(LIB + "/runtime/gc_semispace.h")
QUITAR = "--quitar" in sys.argv
BUILD = "--build" in sys.argv


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()[:16]


def reemplazar(path, viejo, nuevo, etiqueta="", marca=None):
    t = path.read_text()
    clave = marca if marca is not None else nuevo
    if clave in t:
        print("  ya estaba %s" % (etiqueta or path.name))
        return
    n = t.count(viejo)
    if n != 1:
        sys.exit("ERROR: el texto de %s aparece %d veces (esperaba 1) en %s" % (etiqueta, n, path))
    path.write_text(t.replace(viejo, nuevo, 1))
    print("  ok %s" % (etiqueta or path.name))


def deshacer(path, nuevo, viejo, etiqueta=""):
    t = path.read_text()
    if nuevo not in t:
        print("  ya estaba deshecho %s" % (etiqueta or path.name))
        return
    n = t.count(nuevo)
    if n != 1:
        sys.exit("ERROR: el texto de %s aparece %d veces (esperaba 1) en %s" % (etiqueta, n, path))
    path.write_text(t.replace(nuevo, viejo, 1))
    print("  deshecho %s" % (etiqueta or path.name))


# ============================== 1. LA CABECERA ==============================
ANCLA_CABEZA = """struct __go_type_descriptor;

void runtime_throw(const char *s) __attribute__((noreturn));
"""

NUEVO_CABEZA = """struct __go_type_descriptor;

void runtime_throw(const char *s) __attribute__((noreturn));

/* DC-002.146d: LA TABLA DE LOS DESCRIPTORES DE TIPO REALES. La llena el asignador (`gc_heap.c`, con el descriptor
 * que le pasa el compilador en cada `gc_alloc`) y la consulta el recolector (`gc_copy.c`): un tipo que no esta en
 * la tabla y que no tiene ni forma de descriptor es un ENCABEZADO FALSO, y no se barre con el `__ptrdata` que
 * diga. Ver el comentario largo en `gc_copy.c`. */
void gc_tipo_registrar(struct __go_type_descriptor *t);
unsigned long gc_tipos_registrados(void);
"""

# ============================== 2. EL ASIGNADOR ==============================
ANCLA_ALLOC = """void *gc_alloc(size_t size, struct __go_type_descriptor *type)
{
    if (!gc_heap.initialized)
        gc_init();
"""

NUEVO_ALLOC = """void *gc_alloc(size_t size, struct __go_type_descriptor *type)
{
    /* DC-002.146d: el descriptor que manda el compilador se anota ANTES de que el objeto exista: asi el
     * recolector no puede ver nunca un objeto cuyo tipo no este en la tabla. */
    gc_tipo_registrar(type);

    if (!gc_heap.initialized)
        gc_init();
"""

ANCLA_ALLOC_NO_GC = """void *gc_alloc_no_gc(size_t size, struct __go_type_descriptor *type)
{
    if (!gc_heap.initialized)
        return NULL;
"""

NUEVO_ALLOC_NO_GC = """void *gc_alloc_no_gc(size_t size, struct __go_type_descriptor *type)
{
    gc_tipo_registrar(type);   /* DC-002.146d: igual que gc_alloc */

    if (!gc_heap.initialized)
        return NULL;
"""

# ============================== 3. LA TABLA Y LOS TRES VEREDICTOS ==============================
ANCLA_TABLA = """/* El gcdata es un dato compilado mas (bitmap o programa), byte a byte: sin exigir alineacion. */
static bool gc_dato_en_imagen(const void *p)
{
    return p != NULL && gc_en_imagen((uintptr_t)p, 1);
}
"""

NUEVO_TABLA = """/* El gcdata es un dato compilado mas (bitmap o programa), byte a byte: sin exigir alineacion. */
static bool gc_dato_en_imagen(const void *p)
{
    return p != NULL && gc_en_imagen((uintptr_t)p, 1);
}

/* ==================== LA TABLA DE LOS TIPOS REALES (DC-002.146d) ====================
 *
 * POR QUE EXISTE (medido en DC-002.145 / DC-002.146). El barrido conservador de la pila INVENTA encabezados: una
 * palabra que caiga en el from-space y que tenga cuatro bytes de "tamano" razonables por delante es, para el
 * recolector, el comienzo de un objeto. Su campo `type` es otra palabra cualquiera, y si esa palabra cae DENTRO DE
 * LA IMAGEN, `gc_tipo_en_imagen` la tiene por descriptor. Pero la imagen es texto+datos+bss, y ahi adentro tambien
 * hay codigo, cadenas y tablas: no todo lo que cae en la imagen es un descriptor. La firma del defecto lo dice con
 * dos numeros que son DIRECCIONES leidas como tamanos:
 *
 *     gc-ptrdata-recortado    v=2349166024 (0x8c056dc8)   <- el supuesto __ptrdata, una palabra de .rodata
 *     gc-gcdata-fuera-imagen  v= 906232840 (0x36040408)   <- y el supuesto __gcdata, basura fuera de RAM
 *
 * El rango de la imagen no alcanza para decidir; lo que decide es la lista de los descriptores que el PROGRAMA usa.
 * El compilador se lo pasa al asignador en cada `gc_alloc()`, asi que esta tabla tiene TODO descriptor de todo
 * objeto del monton anotado antes de que el recolector lo pueda ver.
 *
 * TRES VEREDICTOS (`gc_tipo_veredicto`) y que se hace con cada uno:
 *   REAL          esta en la tabla -> barrido preciso, como siempre.
 *   CON FORMA     no esta en la tabla pero su contenido tiene forma de descriptor (y la tabla no se lleno): se barre
 *                 CONSERVADOR y acotado al objeto, con aviso. Red de seguridad para un tipo compilado que nunca
 *                 asigno (uno que solo vive en la pila); si aparece, la tabla tiene un hueco y hay que mirarla.
 *   INVALIDO      ni en la tabla ni con forma -> el encabezado es FALSO (el objeto no es de nadie): se avisa con
 *                 `gc-tipo-invalido` y NO SE BARRE (NOSCAN). Barrerlo con un `__ptrdata` de basura es lo que metia
 *                 objetos fantasma en el semiespacio, y lo que despues reventaba lejos, en el codigo de Go.
 */
#define GC_TIPOS_SLOTS 4096   /* 16 KB: se llena con los TIPOS del programa, no con los objetos */

static struct __go_type_descriptor *gc_tipos_tabla[GC_TIPOS_SLOTS];
static struct __go_type_descriptor *gc_tipo_ultimo = NULL;   /* el de la asignacion anterior: el caso normal */
static unsigned long gc_tipos_n = 0;
static bool gc_tipos_llena = false;
static unsigned long gc_tipos_invalidos_n = 0;
static unsigned long gc_tipos_sin_registro_n = 0;

static unsigned gc_tipo_slot(const struct __go_type_descriptor *t)
{
    return ((unsigned)(uintptr_t)t >> 3) & (GC_TIPOS_SLOTS - 1);
}

/* Lo llama el ASIGNADOR (gc_heap.c) con el descriptor del compilador. Idempotente: el mismo tipo se pide millones
 * de veces y solo la primera entra. */
void gc_tipo_registrar(struct __go_type_descriptor *t)
{
    unsigned h;

    if (t == NULL || t == gc_tipo_ultimo)
        return;

    h = gc_tipo_slot(t);

    for (unsigned i = 0; i < GC_TIPOS_SLOTS; i++)
    {
        unsigned s = (h + i) & (GC_TIPOS_SLOTS - 1);

        if (gc_tipos_tabla[s] == t)
        {
            gc_tipo_ultimo = t;

            return;   /* ya estaba */
        }

        if (gc_tipos_tabla[s] == NULL)
        {
            if (gc_tipos_n * 4 >= (unsigned long)GC_TIPOS_SLOTS * 3)
            {
                /* 75 % llena: se deja de meter y se avisa UNA vez. Desde ahi un tipo desconocido no se puede
                 * acusar (`gc_tipo_veredicto` devuelve CON FORMA). */
                if (!gc_tipos_llena)
                    godc_marca("gc-tipos-tabla-llena", gc_tipos_n);

                gc_tipos_llena = true;

                return;
            }

            gc_tipos_tabla[s] = t;
            gc_tipos_n++;
            gc_tipo_ultimo = t;

            return;
        }
    }

    gc_tipos_llena = true;
}

unsigned long gc_tipos_registrados(void)
{
    return gc_tipos_n;
}

static bool gc_tipo_registrado(const struct __go_type_descriptor *t)
{
    unsigned h = gc_tipo_slot(t);

    for (unsigned i = 0; i < GC_TIPOS_SLOTS; i++)
    {
        unsigned s = (h + i) & (GC_TIPOS_SLOTS - 1);

        if (gc_tipos_tabla[s] == t)
            return true;

        if (gc_tipos_tabla[s] == NULL)
            return false;
    }

    return false;
}

/* LA FORMA de un descriptor. NO alcanza sola -- medido: en el ELF del POC de DC-002.146c (343.872 B de imagen) hay
 * 6 trozos de .text y .data que la pasan sin ser descriptores -- pero separa "hay que mirarlo" de "esto es basura":
 *   - el tamano de un tipo del motor no pasa de 1 MB (los objetos grandes van a malloc y su tipo sigue siendo chico);
 *   - `__ptrdata` es un PREFIJO del tipo: nunca mas grande que el tipo (en el defecto, 0x8c056dc8 >> __size);
 *   - `__code` es un Kind de Go (0..26) en los cinco bits de abajo; gccgo ADEMAS prende el bit 0x80 (y a veces
 *     0x20/0x40), asi que se mira `__code & 0x1F`. Medido: con la mascara `~(0x20|0x40)` se rechazaban 585 de los
 *     1684 descriptores REALES del ELF del demo de DC-002.146 (los de `__code` 130/145/153); con `& 0x1F`, cero.
 *   - `__align` es 1, 2, 4 u 8 (medido en los descriptores reales del ELF: 4, y 1 en los tipos de 1 byte);
 *   - el `gcdata`, si lo hay, es dato compilado: vive en la imagen.
 */
#define GC_TIPO_SIZE_MAX (1024 * 1024)
#define GC_KIND_ID_MAX 26

static bool gc_tipo_con_forma(const struct __go_type_descriptor *t)
{
    unsigned kind = (unsigned)t->__code & 0x1F;   /* el Kind son los 5 bits de abajo: gccgo prende 0x80 ademas */

    if (t->__size > GC_TIPO_SIZE_MAX)
        return false;

    if (t->__ptrdata > t->__size)
        return false;

    if (kind > GC_KIND_ID_MAX)
        return false;

    if (t->__align != 1 && t->__align != 2 && t->__align != 4 && t->__align != 8)
        return false;

    if (t->__gcdata != NULL && !gc_dato_en_imagen(t->__gcdata))
        return false;

    return true;
}

#define GC_TIPO_REAL 0
#define GC_TIPO_CON_FORMA 1
#define GC_TIPO_INVALIDO 2

static int gc_tipo_veredicto(const struct __go_type_descriptor *t)
{
    if (!gc_tipo_en_imagen(t))
        return GC_TIPO_INVALIDO;

    if (gc_tipo_registrado(t))
        return GC_TIPO_REAL;

    if (gc_tipos_llena)
        return GC_TIPO_CON_FORMA;   /* la tabla no tomo el tipo: no se puede acusar a nadie */

    return gc_tipo_con_forma(t) ? GC_TIPO_CON_FORMA : GC_TIPO_INVALIDO;
}

/* El parte de la tabla: la capacidad y los tipos distintos UNA vez, y despues solo si hay algo que contar. */
static void gc_tipos_parte(void)
{
    static bool capacidad_avisada = false;

    if (!capacidad_avisada)
    {
        godc_marca("gc-tipos-tabla", GC_TIPOS_SLOTS);
        godc_marca("gc-tipos-registrados", gc_tipos_n);
        capacidad_avisada = true;
    }

    if (gc_tipos_invalidos_n != 0)
        godc_marca("gc-tipos-invalidos-total", gc_tipos_invalidos_n);

    if (gc_tipos_sin_registro_n != 0)
        godc_marca("gc-tipos-sin-registrar-total", gc_tipos_sin_registro_n);
}
"""

# ============================== 4. LOS DOS SITIOS QUE BARREN ==============================
ANCLA_OBJETO = """    if (!gc_tipo_en_imagen(type))
    {
        /* DC-002.92: el encabezado dice un tipo que no esta en la imagen -> encabezado falso (o tipo
         * reciclado). No se puede confiar en `__ptrdata`: se barre conservador y acotado al objeto. */
        godc_marca("gc-tipo-fuera-imagen", (unsigned long)(uintptr_t)type);
        gc_scan_range_conservative(obj, obj_size);
        return;
    }
"""

NUEVO_OBJETO = """    {
        int veredicto = gc_tipo_veredicto(type);

        if (veredicto == GC_TIPO_INVALIDO)
        {
            /* DC-002.146d: el tipo no esta en la tabla y no tiene ni forma de descriptor: el encabezado es FALSO
             * (lo invento el barrido conservador de la pila) o el tipo se reciclo. Se avisa y NO SE BARRE: leer su
             * `__ptrdata` es leer una direccion de la imagen como si fuera un tamano, que es la firma del defecto
             * (gc-ptrdata-recortado v=0x8c056dc8) y la que metia objetos fantasma en el semiespacio. */
            gc_tipos_invalidos_n++;
            godc_marca("gc-tipo-invalido", (unsigned long)(uintptr_t)type);

            return;
        }

        if (veredicto == GC_TIPO_CON_FORMA)
        {
            /* Un descriptor con forma que nunca asigno: no se le cree el mapa de bits, pero SI se puede barrer su
             * memoria conservador y acotada al objeto (lo que hacia DC-002.92). Si esto sale, la tabla tiene un
             * hueco: hay un tipo compilado que asigna por otro camino. */
            gc_tipos_sin_registro_n++;
            godc_marca("gc-tipo-sin-registrar", (unsigned long)(uintptr_t)type);
            gc_scan_range_conservative(obj, obj_size);

            return;
        }
    }
"""

ANCLA_ELEMENTO = """    if (!gc_tipo_en_imagen(type))
    {
        godc_marca("gc-tipo-fuera-imagen", (unsigned long)(uintptr_t)type);
        return;
    }
"""

NUEVO_ELEMENTO = """    {
        /* DC-002.146d: aca solo se barre con un descriptor REAL, el que el compilador le paso al asignador. Uno con
         * forma pero sin registrar se barre conservador y acotado; uno sin forma es un encabezado falso y no se
         * barre (leerle el `__ptrdata` es leer basura como tamano). */
        int veredicto = gc_tipo_veredicto(type);

        if (veredicto != GC_TIPO_REAL)
        {
            if (veredicto == GC_TIPO_CON_FORMA)
            {
                gc_tipos_sin_registro_n++;
                godc_marca("gc-tipo-sin-registrar", (unsigned long)(uintptr_t)type);
                gc_scan_range_conservative(elem, limite != 0 ? limite : type->__size);
            }
            else
            {
                gc_tipos_invalidos_n++;
                godc_marca("gc-tipo-invalido", (unsigned long)(uintptr_t)type);
            }

            return;
        }
    }
"""

ANCLA_FORWARD = """static bool gc_candidato_del_barrido(void *ptr);   /* DC-002.39b: el barrido conservador acepta los INTERIORES */
void gc_scan_range_conservative(void *start, size_t size);
"""

NUEVO_FORWARD = """static bool gc_candidato_del_barrido(void *ptr);   /* DC-002.39b: el barrido conservador acepta los INTERIORES */
void gc_scan_range_conservative(void *start, size_t size);
static void gc_tipos_parte(void);   /* DC-002.146d: el parte de la tabla de tipos, al cerrar cada recoleccion */
static int gc_tipo_veredicto(const struct __go_type_descriptor *t);   /* DC-002.146d: REAL / CON FORMA / INVALIDO */
"""

ANCLA_CABECERA = """            /*
             * Type pointer should be reasonably aligned.
             * Type descriptors are structs with pointer-sized fields,
             * so they should be at least 4-byte aligned.
             */
            if (type_addr & 0x3)
            {
                return false;
            }
        }
    }

    return true;
}
"""

NUEVO_CABECERA = """            /*
             * Type pointer should be reasonably aligned.
             * Type descriptors are structs with pointer-sized fields,
             * so they should be at least 4-byte aligned.
             */
            if (type_addr & 0x3)
            {
                return false;
            }

            /* DC-002.146d: EL TIPO TIENE QUE SER UN DESCRIPTOR DE VERDAD. `gc_tipo_en_imagen` solo pide que caiga
             * en la imagen, y la imagen tiene adentro codigo, cadenas y tablas: un `type` que caiga en .rodata
             * (medido: 0x8c2dea5c en el demo de DC-002.146) pasaba el chequeo y este encabezado era un FALSO
             * comienzo de objeto inventado por el barrido conservador. Copiarlo le escribe un puntero de reenvio
             * ADENTRO del objeto vivo que lo contiene -- se pisa un campo de un objeto sano -- y el codigo de Go
             * revienta despues, lejos, con `slice bounds out of range`. Rechazarlo ACA corta la cadena en el primer
             * eslabon: el objeto fantasma no se copia, no se barre y no se le pisa la memoria a nadie.
             *
             * Se aceptan los REALES (una fila de la tabla que llena el asignador) y los que tienen FORMA de
             * descriptor (red de seguridad para un tipo compilado que nunca asigno). Ver `gc_tipo_veredicto`. */
            if (gc_tipo_veredicto(type) == GC_TIPO_INVALIDO)
            {
                gc_tipos_invalidos_n++;
                godc_marca("gc-tipo-invalido", (unsigned long)type_addr);

                return false;
            }
        }
    }

    return true;
}
"""

ANCLA_VIVO = """    godc_marca("gc-vivo", (unsigned long)after_size);
"""

NUEVO_VIVO = """    godc_marca("gc-vivo", (unsigned long)after_size);

    /* DC-002.146d: el parte de la tabla de tipos (solo si hay algo que avisar) */
    gc_tipos_parte();
"""

FIX = [
    ("cabecera", CABEZA, ANCLA_CABEZA, NUEVO_CABEZA),
    ("tabla", COPY, ANCLA_TABLA, NUEVO_TABLA),
    ("forward", COPY, ANCLA_FORWARD, NUEVO_FORWARD),
    ("compuerta", COPY, ANCLA_CABECERA, NUEVO_CABECERA),
    ("objeto", COPY, ANCLA_OBJETO, NUEVO_OBJETO),
    ("elemento", COPY, ANCLA_ELEMENTO, NUEVO_ELEMENTO),
    ("parte", COPY, ANCLA_VIVO, NUEVO_VIVO),
    ("alloc", HEAP, ANCLA_ALLOC, NUEVO_ALLOC),
    ("alloc_no_gc", HEAP, ANCLA_ALLOC_NO_GC, NUEVO_ALLOC_NO_GC),
]


def estado():
    t = COPY.read_text()
    puesto = "gc_tipo_veredicto" in t and "gc_tipos_registrados" in t
    a = pathlib.Path(LIB + "/libgodc.a")
    print("arreglo 146d: %s | %s (libgodc.a sha256 %s)" %
          ("PUESTO" if puesto else "QUITADO", LIB, sha(a) if a.exists() else "-"))
    return puesto


if "--estado" in sys.argv:
    print("== estado de la tarjeta DC-002.146d")
    estado()
    sys.exit(0)

print("== DC-002.146d: la tabla de los descriptores de tipo reales (libgodc %s)" % LIB)

if QUITAR:
    print("   --quitar: EL MUTANTE (vuelve al arreglo de DC-002.92, que es el 'hoy' de la tarjeta)")
    for et, path, viejo, nuevo in FIX:
        deshacer(path, nuevo, viejo, et)
else:
    for et, path, viejo, nuevo in FIX:
        reemplazar(path, viejo, nuevo, et)

if BUILD:
    # El `make` de libgodc necesita `sh-elf-gcc` y el entorno de KOS en el PATH: se sourcea el env.sh del repo
    # (con `set +u`, porque el environ.sh de KOS referencia variables sin definir).
    REPO = pathlib.Path(__file__).resolve().parents[2]
    entorno = "%s/scripts/console/env.sh" % REPO
    subprocess.run(["bash", "-c", "rm -f %s/runtime/*.o %s/*.a" % (LIB, LIB)], check=True)
    r = subprocess.run(["bash", "-c", "set +u; . %s; make -C %s all" % (entorno, LIB)],
                       capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout[-4000:] + r.stderr[-4000:])
        sys.exit("make fallo")
    print("   make ok: " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    estado()
else:
    estado()
    print("   falta: correr con --build (rm -f %s/runtime/*.o && make -C %s all)" % (LIB, LIB))
print("   despues: scripts/console/checar-gc-tipo-invalido-146d.sh arreglo|mutante")
