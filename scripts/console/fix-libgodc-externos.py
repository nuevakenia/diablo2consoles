#!/usr/bin/env python3
"""Los bloques EXTERNOS del GC: los grandes que llevan punteros adentro ahora se BARREN.

SINTOMA (medido, DC-002.76 / DC-002.83)
    El POC del cooperativo crea el jugador 1 y NO puede crear el jugador 2. La consola dice:

        COOP276: ANIMDATA vivo registros= 2556 gcTotal= 8
        COOP276: JUGADOR J2 FALLO: could not find Animation data

    O sea: `GetRecordsCount()` sigue diciendo 2.556 (el `count` del header del mapa vive) pero
    `GetRecords("AMNUHTH")`, que busca POR CLAVE, no encuentra nada. Y pasa igual en cada cuadro, despues
    de las recolecciones que dispara la creacion del jugador 1.

QUE PASA, PASO A PASO (medido, no supuesto)
    1. `gc_alloc()` manda TODO pedido de mas de `GC_LARGE_OBJECT_THRESHOLD` (64 KB) a
       `gc_external_alloc()`, que es `malloc()` pelado: un bloque FUERA de los dos semiespacios, sin
       encabezado de GC. El recolector nunca lo copia y nunca lo barre -- no existe para el.
    2. El mapa `map[string][]*AnimationDataRecord` de AnimData.d2 tiene 2.556 claves y su bucket mide 172 B
       (`topbits[8] + 8 claves string(8) + 8 valores slice(12) + overflow(4)`), asi que con B=9 el array
       pesa 512 * 172 = 88.064 B > 64 KB -> se va al camino externo. El tipo del bucket SI trae su mapa de
       punteros (leido del ELF con `scripts/console/leer-tipos-elf.py`: size=172 ptrdata=172
       gcdata=0x8c056120), pero nadie lo usa.
    3. Todo lo que cuelga SOLO de esos buckets -- los bytes de los nombres de 8 letras y los `[]*registro`
       de cada clave -- queda invisible. Medido con la sonda `console/dc/poc/map-gc` (Flycast): con el mapa
       lleno (2.556 claves + 2.556 registros) el recolector ve `gc-vivo = 134.640 B`, que son los 96 KB del
       conjunto vivo de prueba mas los ~36 KB del runtime. El mapa no aporta ni un byte.
    4. Cuando el espacio viejo se reusa (en el cooperativo lo reusa la creacion del jugador 1, que son 2 s
       de decodificar .dcc), los bytes de las claves se pisan: la busqueda por clave falla y el `count`
       sigue intacto porque el header del mapa es un objeto chico que SI vive en el monton Go.

    La sonda del mapa lo mide en tres rondas de recoleccion (misma corrida, Flycast):
        ronda 0   A_bien= 2556   clavesRotas= 0
        ronda 1   A_bien= 2556   clavesRotas= 0     (los bytes viejos todavia estan)
        ronda 2   A_bien= 2347   clavesRotas= 416   (el espacio viejo se reuso)
        ronda 3   A_bien= 1856   clavesRotas= 541
    y las camas de control se quedan enteras: B (claves constantes, bucket chico) 16/16, C (map[int]int)
    255/255, D (map[string]string chico) 256/256 y E (mapa armado despues del GC) 256/256. O sea: el
    defecto es del camino EXTERNO, no del mapa.

EL ARREGLO
    El bloque grande tiene que seguir fuera del monton (en una consola de 16 MB copiar 88 KB en cada
    recoleccion es caro), pero sus PUNTEROS tienen que seguirse: se registra en una tabla chica
    (`gc_externos`, hasta 48 entradas) junto con el tipo y el tamano, y en cada recoleccion se barre igual
    que un objeto del monton -- `gc_scan_single_element` por elemento -- antes del barrido de Cheney. Los
    punteros adentro se ponen al dia EN EL LUGAR (el bloque no se mueve), asi que lo que cuelga de el se
    copia y sobrevive.

    Solo se registran los bloques cuyo tipo TIENE punteros (`__ptrdata > 0`): las imagenes, los .dcc y los
    buffers crudos del MPQ (que son `[]uint8`) siguen yendo a `malloc` sin tabla, como hasta ahora. Si la
    tabla se llena se cae al camino de siempre (el bloque se pide al monton Go): mas lento y con mas
    presion sobre el semiespacio, pero correcto, y la marca `gc-externo-sin-lugar` lo dice en la consola.

LIMITE DECLARADO
    Un bloque externo que se registra NO se libera nunca (el mapa no libera los arrays de buckets viejos y
    el recolector tampoco los puede liberar: no los copia). Es la misma fuga que ya estaba declarada en
    `allocBuckets`, ahora acotada a 48 bloques. El mapa de animdata usa UNO solo.

USO
    python3 scripts/console/fix-libgodc-externos.py                  # aplica (idempotente)
    python3 scripts/console/fix-libgodc-externos.py --build          # ademas rehace libgodc.a
    python3 scripts/console/fix-libgodc-externos.py --quitar --build # EL MUTANTE: lo saca y deja el resto
    python3 scripts/console/fix-libgodc-externos.py --estado         # que dice el arbol

ORDEN E IDEMPOTENCIA
    Va DESPUES de la cadena que ya existe (fix-libgodc-gc.py, fix-libgodc-sq.py, fix-libgodc-interior.py):
    toca `runtime/gc_copy.c` (el barrido) y `runtime/gc_heap.c` (el registro). NO hace `git checkout`: eso
    borraria las marcas de arranque y `gc_solapa_semiespacio` (DC-002.53) que viven en gc_heap.c sin
    script propio. Los tres reemplazos miran primero si el texto nuevo ya esta, asi que correrlo dos veces
    deja el mismo sha256.

MEDIDO (sonda console/dc/poc/map-gc y POC coop, Flycast, rama DC-002)
    ANTES:  A_bien 2556 -> 2347 -> 1856 (las claves se reciclan) y J2 no se crea.
    DESPUES: A_bien 2556 en las cuatro rondas (0, 1, 2 y 3) y el cooperativo crea J2.
"""
import hashlib
import os
import pathlib
import subprocess
import sys

args = [a for a in sys.argv[1:] if not a.startswith("--")]
LIB = args[0] if args else os.path.expanduser("~/dreamcast/libgodc")
AQUI = os.path.dirname(os.path.abspath(__file__))
HEADER = pathlib.Path(LIB + "/runtime/gc_semispace.h")
HEAP = pathlib.Path(LIB + "/runtime/gc_heap.c")
COPY = pathlib.Path(LIB + "/runtime/gc_copy.c")
QUITAR = "--quitar" in sys.argv
BUILD = "--build" in sys.argv


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()[:16]


def reemplazar(path, viejo, nuevo):
    t = path.read_text()
    if nuevo in t:
        print("  ya estaba en %s" % path.name)
        return
    n = t.count(viejo)
    if n != 1:
        sys.exit("ERROR: el texto a reemplazar aparece %d veces (esperaba 1) en %s" % (n, path))
    path.write_text(t.replace(viejo, nuevo, 1))
    print("  ok %s" % path.name)


def deshacer(path, nuevo, viejo):
    """El camino inverso de `reemplazar`: saca el texto nuevo y deja el viejo (para el mutante)."""
    t = path.read_text()
    if nuevo not in t:
        print("  ya estaba deshecho en %s" % path.name)
        return
    n = t.count(nuevo)
    if n != 1:
        sys.exit("ERROR: el texto a deshacer aparece %d veces (esperaba 1) en %s" % (n, path))
    path.write_text(t.replace(nuevo, viejo, 1))
    print("  deshecho %s" % path.name)


# --- 1. la tabla, en el header que ya incluyen los dos archivos ---------------------------------------
ANCLA_HEADER = """// Map and channel allocation declarations are in map_dreamcast.c
"""

TABLA_HEADER = """// Map and channel allocation declarations are in map_dreamcast.c

/* ==================== BLOQUES EXTERNOS CON PUNTEROS (DC-002.83) ====================
 *
 * `gc_alloc()` manda todo pedido de mas de GC_LARGE_OBJECT_THRESHOLD a `gc_external_alloc()`, que es
 * `malloc()` pelado: un bloque FUERA de los semiespacios, sin encabezado, que el recolector nunca barre.
 * Mientras el bloque no tenga punteros (una imagen, un .dcc, un buffer crudo) eso es correcto y barato.
 * Con punteros adentro NO: todo lo que cuelga de el se recicla aunque este vivo. Medido en DC-002.83: el
 * array de buckets de un `map[string][]*T` de 2.556 claves pesa 88.064 B, se va al camino externo y las
 * claves del mapa mueren en la segunda recoleccion ("could not find Animation data" en el cooperativo).
 *
 * Por eso los externos que llevan punteros se REGISTRAN aca (tipo + tamano) y en cada recoleccion se
 * barren como un objeto del monton, antes del barrido de Cheney. El bloque no se mueve: sus punteros se
 * ponen al dia en el lugar.
 */
#define GC_MAX_EXTERNOS 48

typedef struct gc_externo
{
    void *ptr;                              /* el bloque de malloc */
    struct __go_type_descriptor *type;       /* el tipo con el que se pidio (NULL = sin tipo) */
    size_t size;                            /* bytes pedidos */
} gc_externo_t;

extern gc_externo_t gc_externos[GC_MAX_EXTERNOS];
extern int gc_externos_count;
"""

# --- 2. la tabla y el registro, en gc_heap.c ----------------------------------------------------------
ANCLA_GLOBALS = """gc_roots_t gc_root_table = {.roots = {NULL}, .count = 0};
"""

TABLA_GLOBALS = """gc_roots_t gc_root_table = {.roots = {NULL}, .count = 0};

/* DC-002.83: los bloques externos con punteros (ver el comentario de la tabla en gc_semispace.h). */
gc_externo_t gc_externos[GC_MAX_EXTERNOS];
int gc_externos_count = 0;

/* Registrar/olvidar un bloque externo. Se registra SOLO si el tipo dice que hay punteros adentro: los
 * buffers crudos (ptrdata == 0) no necesitan tabla y siguen yendo a malloc sin costo. */
static void gc_registrar_externo(void *ptr, size_t size, struct __go_type_descriptor *type)
{
    if (gc_externos_count >= GC_MAX_EXTERNOS)
        return;

    gc_externos[gc_externos_count].ptr = ptr;
    gc_externos[gc_externos_count].type = type;
    gc_externos[gc_externos_count].size = size;
    gc_externos_count++;
}

static void gc_olvidar_externo(void *ptr)
{
    for (int i = 0; i < gc_externos_count; i++)
    {
        if (gc_externos[i].ptr != ptr)
            continue;

        gc_externos[i] = gc_externos[gc_externos_count - 1];
        gc_externos_count--;

        return;
    }
}
"""

# --- 3. el camino de objetos grandes: con punteros, a la tabla ----------------------------------------
ANCLA_GRANDE = """    /* Large objects bypass GC heap */
    if (size > GC_LARGE_OBJECT_THRESHOLD)
    {
        gc_heap.large_alloc_count++;
        gc_heap.large_alloc_total += size;
        return gc_external_alloc(size);
    }
"""

NUEVO_GRANDE = """    /* Large objects bypass GC heap */
    if (size > GC_LARGE_OBJECT_THRESHOLD)
    {
        /*
         * DC-002.83: un bloque grande SIN punteros (una imagen, un .dcc, un buffer crudo del MPQ) puede
         * vivir fuera del monton: nadie tiene que barrerlo, nada se muere por el. Uno CON punteros NO:
         * fuera del monton no existe para el recolector, y todo lo que cuelgue solo de el se recicla
         * (medido: el array de buckets de un mapa de 2.556 claves, 88.064 B, y las claves del mapa mueren
         * en la segunda recoleccion). Los que llevan punteros se piden a malloc y se REGISTRAN, para que
         * el barrido los recorra.
         */
        bool con_punteros = (type != NULL) && (type->__ptrdata > 0);

        if (con_punteros && gc_externos_count < GC_MAX_EXTERNOS)
        {
            void *bloque = gc_external_alloc(size);

            gc_registrar_externo(bloque, size, type);
            gc_heap.large_alloc_count++;
            gc_heap.large_alloc_total += size;
            godc_marca("gc-externo-con-punteros", (unsigned long)size);

            return bloque;
        }

        if (con_punteros)
        {
            /* la tabla esta llena: se pide al monton Go (mas presion, pero correcto) y se avisa */
            godc_marca("gc-externo-sin-lugar", (unsigned long)size);
        }

        gc_heap.large_alloc_count++;
        gc_heap.large_alloc_total += size;
        return gc_external_alloc(size);
    }
"""

# --- 4. el free se desregistra --------------------------------------------------------------
ANCLA_FREE = """void gc_external_free(void *ptr)
{
    if (ptr)
        free(ptr);
}
"""

NUEVO_FREE = """void gc_external_free(void *ptr)
{
    if (ptr)
    {
        gc_olvidar_externo(ptr);   /* DC-002.83: si estaba en la tabla, sale antes del free */
        free(ptr);
    }
}
"""

# --- 5. el barrido de los externos, en gc_copy.c -----------------------------------------------------
ANCLA_FWD = """static void gc_scan_roots(void);
static void gc_scan_stack(void);
"""

FWD = """static void gc_scan_roots(void);
static void gc_scan_stack(void);
static void gc_scan_externos(void);
"""

ANCLA_LLAMADA = """    gc_scan_roots();
"""

LLAMADA = """    gc_scan_roots();

    /* DC-002.83: los bloques externos que llevan punteros adentro. No se copian (no viven en el
     * semiespacio) pero sus punteros SI se ponen al dia, que es lo que mantiene vivo lo que cuelga de
     * ellos: sin esto el array de buckets de un mapa grande es invisible para el recolector y el mapa se
     * vacia por clave en la segunda recoleccion. */
    gc_scan_externos();
"""

ANCLA_DEF = """// Scan an object for pointer fields
static void gc_scan_object(void *obj)
"""

DEF = """/*
 * DC-002.83: barre los bloques EXTERNOS registrados (los que viven fuera de los semiespacios y llevan
 * punteros adentro). Cada bloque se recorre elemento por elemento con su tipo, igual que un array del
 * monton: los punteros se ponen al dia EN EL LUGAR (el bloque no se mueve) y lo que apuntan se copia.
 */
static void gc_scan_externos(void)
{
    for (int i = 0; i < gc_externos_count; i++)
    {
        gc_externo_t *e = &gc_externos[i];

        if (e->ptr == NULL || e->type == NULL || e->type->__ptrdata == 0)
            continue;

        size_t tsz = e->type->__size;

        if (tsz == 0 || e->size < tsz)
        {
            gc_scan_range_conservative(e->ptr, e->size);
            continue;
        }

        size_t n = e->size / tsz;

        if (n > gc_heap.space_size / sizeof(void *))
        {
            gc_scan_range_conservative(e->ptr, e->size);
            continue;
        }

        for (size_t k = 0; k < n; k++)
            gc_scan_single_element((uint8_t *)e->ptr + k * tsz, e->type);
    }
}

// Scan an object for pointer fields
static void gc_scan_object(void *obj)
"""


def estado():
    h = HEADER.read_text() if HEADER.exists() else ""
    k = HEAP.read_text() if HEAP.exists() else ""
    c = COPY.read_text() if COPY.exists() else ""
    aplicado = ("gc_externo_t" in h and "gc_registrar_externo" in k and "gc_scan_externos();" in c)
    print("externos: %s en %s (sha256 %s)" % ("APLICADO" if aplicado else "QUITADO", LIB,
                                              sha(pathlib.Path(LIB + "/libgodc.a"))
                                              if os.path.exists(LIB + "/libgodc.a") else "-"))
    return aplicado


if "--estado" in sys.argv:
    print("== estado del arreglo de bloques externos con punteros")
    sys.exit(0 if estado() else 1)

print("== el arreglo de bloques externos del GC (DC-002.83)")

if QUITAR:
    # El MUTANTE: se deshacen los tres reemplazos (de nuevo -> viejo), asi el arbol queda con la cadena del
    # runtime y SIN este arreglo. Es lo que exige el criterio de un arreglo del runtime: sacarlo tiene que
    # poner ROJO el mismo comando que con el arreglo da VERDE (scripts/console/checar-mapgc-283.py).
    print("   --quitar: se deshace el arreglo (el arbol queda como el mutante)")
    deshacer(HEADER, TABLA_HEADER, ANCLA_HEADER)
    deshacer(HEAP, TABLA_GLOBALS, ANCLA_GLOBALS)
    deshacer(HEAP, NUEVO_GRANDE, ANCLA_GRANDE)
    deshacer(HEAP, NUEVO_FREE, ANCLA_FREE)
    deshacer(COPY, FWD, ANCLA_FWD)
    deshacer(COPY, LLAMADA, ANCLA_LLAMADA)
    deshacer(COPY, DEF, ANCLA_DEF)
else:
    reemplazar(HEADER, ANCLA_HEADER, TABLA_HEADER)
    reemplazar(HEAP, ANCLA_GLOBALS, TABLA_GLOBALS)
    reemplazar(HEAP, ANCLA_GRANDE, NUEVO_GRANDE)
    reemplazar(HEAP, ANCLA_FREE, NUEVO_FREE)
    reemplazar(COPY, ANCLA_FWD, FWD)
    reemplazar(COPY, ANCLA_LLAMADA, LLAMADA)
    reemplazar(COPY, ANCLA_DEF, DEF)

estado()

if BUILD:
    subprocess.run(["bash", "-c", f"rm -f {LIB}/runtime/*.o {LIB}/*.a"], check=True)
    r = subprocess.run(["make", "-C", LIB, "all"], capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout[-4000:] + r.stderr[-4000:])
        sys.exit("make fallo")
    print("   make ok: " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    print("   %s/libgodc.a: %d bytes, sha256 %s" % (LIB, os.path.getsize(LIB + "/libgodc.a"),
                                                    sha(pathlib.Path(LIB + "/libgodc.a"))))
else:
    print("   falta: rm -f %s/runtime/*.o %s/*.a && make -C %s all   (o correr con --build)"
          % (LIB, LIB, LIB))
print("   ahora: reenlazar la sonda map-gc y correr Flycast; el numero es `A_faltan= 0` en las rondas 2 y 3")
