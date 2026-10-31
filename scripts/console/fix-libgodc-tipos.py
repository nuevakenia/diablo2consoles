#!/usr/bin/env python3
"""DC-002.92 -- EL BARRIDO DEL GC FUERA DE RAM: el tipo tiene que estar en la imagen y el limite es el objeto.

SINTOMA (medido, DC-002.87c)
    Los MISMOS `src/` del POC cooperativo con 1.440 B mas de sonda pintan 360 cuadros; sin la sonda, el mismo
    codigo muere: H con `jsr @r9` dentro de `gc_scan_gcdata_bitmap` y el PC en `0x3fd17b4f` (fuera de RAM), I
    clavado 30 s en `gc_update_pointer_field` / `gc_scan_range_conservative`, E reinicia la consola. La
    diferencia entre morir y pintar es el ACOMODO del binario: o sea, que palabra de basura cae donde.

QUE PASA (leido en `runtime/gc_copy.c`, y por eso el arreglo va aca)
    1. `gc_scan_object()` toma el descriptor de tipo del ENCABEZADO del objeto y lo valida asi:

           bool in_p1_ram = (type_addr >= 0x8c000000 && type_addr < 0x8d000000);

       Eso no valida nada: el MONTON y la PILA viven ahi. Cualquier palabra del monton alineada a 4 pasa por
       "descriptor de tipo", y entonces `type->__size`, `type->__ptrdata` y `type->__gcdata` son lo que haya
       en esa direccion.
    2. El barrido preciso no tiene tope por objeto: `gc_scan_gcdata_bitmap(obj, gcdata, ptrdata/4)` recorre
       `ptrdata/4` palabras desde `obj`. Con un `__ptrdata` de basura eso es leer (y escribir, cuando la
       palabra parece un puntero del from-space) FUERA DE RAM. Es el salto a `0x3fd17b4f`.
    3. Si el "tipo" sale con `GC_KIND_GCPROG`, o sin gcdata, el camino es `gc_scan_range_conservative(elem,
       ptrdata)`: 16 MB de memoria barridos palabra por palabra y, por cada candidato, un
       `gc_interior_base()` que recorre el from-space desde abajo. Es el lazo de 30 s de I.
    4. Los encabezados falsos los produce el propio barrido conservador de la pila, que interpreta cualquier
       palabra como "aqui empieza un objeto y su tipo es esta otra palabra". Que eso pase o no depende de que
       basura quedo en la pila: el ACOMODO.

EL ARREGLO (dos invariantes, ambas con marca en la consola serie)
    A. UN DESCRIPTOR DE TIPO VIVE EN LA IMAGEN. Los descriptores son estructuras compiladas: estan en
       `.rodata`/`.data`, o sea entre el arranque de la imagen (`0x8c010000`) y `_end` (simbolo del
       enlazador). Fuera de ese rango NO hay descriptores: si el tipo no cae ahi, el encabezado es falso y el
       objeto se barre conservador, acotado a su tamano (`gc-tipo-fuera-imagen`). Lo mismo para el `gcdata`,
       que tambien es dato compilado (`gc-gcdata-fuera-imagen`).
    B. EL BARRIDO NO PASA DEL OBJETO. `gc_scan_single_element()` recibe el tamano del objeto/elemento y el
       barrido preciso se recorta a el: `nwords = min(__ptrdata, limite)/4` (`gc-ptrdata-recortado`). Un
       `__ptrdata` mayor que el objeto ya no puede sacar el barrido de RAM.

    Las dos marcas son la SONDA de la tarjeta (`console/dc/poc/gc-fuera-ram`): el sensor de
    `gc_scan_gcdata_bitmap` dice `GC-FUERA-RAM obj=0x... palabras=N gcdata=0x...` cuando el barrido se iba a
    salir de RAM, y con el arreglo puesto esa linea no sale nunca.

USO
    python3 scripts/console/fix-libgodc-tipos.py --sonda --build        # el instrumento (sensor), sin arreglo
    python3 scripts/console/fix-libgodc-tipos.py --build                # el arreglo + el sensor
    python3 scripts/console/fix-libgodc-tipos.py --quitar --build       # EL MUTANTE: saca el arreglo, deja el sensor
    python3 scripts/console/fix-libgodc-tipos.py --estado

    El primer argumento sin guiones es el arbol de libgodc (por defecto ~/dreamcast/libgodc). Para no tocar el
    toolchain del host, se apunta a una copia:  /tmp/dc306/dcroot/libgodc
"""
import hashlib
import os
import pathlib
import subprocess
import sys

args = [a for a in sys.argv[1:] if not a.startswith("--")]
LIB = args[0] if args else os.path.expanduser("~/dreamcast/libgodc")
COPY = pathlib.Path(LIB + "/runtime/gc_copy.c")
QUITAR = "--quitar" in sys.argv
BUILD = "--build" in sys.argv
SOLO_SONDA = "--sonda" in sys.argv


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


# ============================== 1. EL SENSOR (siempre) ==============================
ANCLA_SENSOR = """static void gc_scan_gcdata_bitmap(void *obj, const uint8_t *gcdata, size_t nwords)
{
    size_t word_idx = 0;
    size_t byte_idx = 0;
"""

NUEVO_SENSOR = """/* SONDA (DC-002.92): ¿este barrido preciso se sale de RAM?
 *
 * El barrido recorre `nwords` palabras desde `obj` y lee el mapa de bits desde `gcdata`. Ni el objeto ni el
 * mapa se validaban contra nada: `obj` es lo que diga un encabezado (que puede ser falso, ver
 * `gc_tipo_en_imagen`) y `nwords` lo que diga el `__ptrdata` de ese "tipo". Aca se mide ANTES de tocar
 * memoria: si el barrido se saldria de RAM, se dice con la direccion y el numero de palabras, se recorta a lo
 * que si esta en RAM y se sigue. La linea `GC-FUERA-RAM` es el testigo de la tarjeta: con el arreglo puesto
 * no sale nunca, sin el arreglo sale y dice por donde se iba.
 */
static unsigned long sonda_fuera_ram_n = 0;

/* DC-002.92: el tope de la imagen (texto+datos+bss). Lo pone `gc_init()` antes de pedir los semiespacios; de
 * ahi para arriba es monton, y una palabra del monton no es un descriptor de tipo. */
uintptr_t gc_imagen_hi = 0;

static void sonda_fuera_ram(void *obj, size_t nwords, const uint8_t *gcdata)
{
    sonda_fuera_ram_n++;
    godc_marca("GC-FUERA-RAM-obj", (unsigned long)(uintptr_t)obj);
    godc_marca("GC-FUERA-RAM-palabras", (unsigned long)nwords);
    godc_marca("GC-FUERA-RAM-gcdata", (unsigned long)(uintptr_t)gcdata);
    godc_marca("GC-FUERA-RAM-barra", (unsigned long)((uintptr_t)obj + nwords * sizeof(void *)));
}

static void gc_scan_gcdata_bitmap(void *obj, const uint8_t *gcdata, size_t nwords)
{
    size_t word_idx = 0;
    size_t byte_idx = 0;

    {
        uintptr_t inicio = (uintptr_t)obj;
        uintptr_t fin = inicio + nwords * sizeof(void *);

        if (nwords > 0 && (inicio < DC_RAM_START || fin > DC_RAM_END))
        {
            sonda_fuera_ram(obj, nwords, gcdata);
            return; /* el barrido se sale de RAM: no se toca nada y la marca queda */
        }

        if (nwords > 0 && ((uintptr_t)gcdata < DC_RAM_START ||
                           (uintptr_t)gcdata >= DC_RAM_END))
        {
            sonda_fuera_ram(obj, nwords, gcdata);
            return;
        }
    }
"""

# ============================== 2. EL ARREGLO ==============================
ANCLA_HELPERS = """/* Scan element using gcdata bitmap (precise scanning). */
static void gc_scan_element_with_gcdata(void *obj, struct __go_type_descriptor *type)
{
    if (!type->__gcdata || type->__ptrdata == 0)
        return;
"""

NUEVO_HELPERS = """/* ==================== EL TIPO TIENE QUE ESTAR EN LA IMAGEN (DC-002.92) ====================
 *
 * Los descriptores de tipo son estructuras COMPILADAS: viven en `.rodata`/`.data`, o sea en la IMAGEN. El heap
 * y la pila viven MAS ARRIBA, tambien en 0x8c......, asi que la comprobacion vieja (`0x8c000000 <= t <
 * 0x8d000000`) dejaba pasar como "tipo" cualquier palabra del monton alineada a 4. De ahi salen los
 * `__ptrdata` de basura que hacian que el barrido se fuera de RAM (DC-002.87c).
 *
 * El tope de la imagen se toma en `gc_init()` (gc_heap.c), ANTES de pedir los semiespacios: de ahi para arriba
 * es monton. `_end` NO sirve: este enlace no lo define (medido).
 */
extern uintptr_t gc_imagen_hi;

static bool gc_en_imagen(uintptr_t a, size_t n)
{
    if (gc_imagen_hi == 0)
        return false;   /* sin tope no se acepta nada: se barre conservador, que es lo seguro */

    return a >= 0x8c010000u && (a + n) <= gc_imagen_hi;
}

static bool gc_tipo_en_imagen(const struct __go_type_descriptor *t)
{
    if (t == NULL || ((uintptr_t)t & 3) != 0)
        return false;

    return gc_en_imagen((uintptr_t)t, sizeof(struct __go_type_descriptor));
}

/* El gcdata es un dato compilado mas (bitmap o programa), byte a byte: sin exigir alineacion. */
static bool gc_dato_en_imagen(const void *p)
{
    return p != NULL && gc_en_imagen((uintptr_t)p, 1);
}


/* Scan element using gcdata bitmap (precise scanning).
 *
 * DC-002.92: `limite` es el tamano del OBJETO (o del elemento). El barrido preciso nunca pasa de ahi: si el
 * `__ptrdata` del tipo dice mas que el objeto, lo que sobra no se toca (es un tipo mal formado o un
 * encabezado falso, y en los dos casos la memoria de adelante no es del objeto).
 */
static void gc_scan_element_with_gcdata(void *obj, struct __go_type_descriptor *type, size_t limite)
{
    size_t ptrdata;

    if (!type->__gcdata || type->__ptrdata == 0)
        return;

    ptrdata = type->__ptrdata;

    if (limite != 0 && ptrdata > limite)
    {
        godc_marca("gc-ptrdata-recortado", (unsigned long)ptrdata);
        ptrdata = limite;
    }
"""

ANCLA_GCDATA_CHECK = """    {
        uintptr_t gcdata_addr = (uintptr_t)type->__gcdata;
        bool in_p1_ram = (gcdata_addr >= 0x8c000000 && gcdata_addr < 0x8d000000);
        bool in_p2_ram = (gcdata_addr >= 0xac000000 && gcdata_addr < 0xad000000);

        if (!in_p1_ram && !in_p2_ram)
        {
            return;
        }
    }

    gc_scan_gcdata_bitmap(obj, type->__gcdata, type->__ptrdata / sizeof(void *));
"""

NUEVO_GCDATA_CHECK = """    if (!gc_dato_en_imagen(type->__gcdata))
    {
        godc_marca("gc-gcdata-fuera-imagen", (unsigned long)(uintptr_t)type->__gcdata);
        gc_scan_range_conservative(obj, ptrdata);
        return;
    }

    gc_scan_gcdata_bitmap(obj, type->__gcdata, ptrdata / sizeof(void *));
"""

ANCLA_SINGLE = """// Scan a single element using its type descriptor
static void gc_scan_single_element(void *elem, struct __go_type_descriptor *type)
{
    if (!type || type->__ptrdata == 0)
    {
        return; // No pointers
    }
"""

NUEVO_SINGLE = """// Scan a single element using its type descriptor
// DC-002.92: `limite` = bytes que de verdad tiene el objeto/elemento (0 = no se conoce).
static void gc_scan_single_element(void *elem, struct __go_type_descriptor *type, size_t limite)
{
    if (!type || type->__ptrdata == 0)
    {
        return; // No pointers
    }
"""

ANCLA_SINGLE_CHECK = """    {
        uintptr_t type_addr = (uintptr_t)type;
        bool in_p1_ram = (type_addr >= 0x8c000000 && type_addr < 0x8d000000);
        bool in_p2_ram = (type_addr >= 0xac000000 && type_addr < 0xad000000);

        if (!in_p1_ram && !in_p2_ram)
        {
            return;
        }
    }
"""

NUEVO_SINGLE_CHECK = """    /*
     * DC-002.92: UN DESCRIPTOR DE TIPO VIVE EN LA IMAGEN. El chequeo viejo aceptaba cualquier direccion de
     * RAM, y el monton tambien es RAM: un encabezado falso (el barrido conservador de la pila los inventa)
     * dejaba que `__ptrdata` se leyera del monton y que el barrido se fuera de RAM. Fuera de la imagen el
     * tipo es basura y no hay nada que barrer con el: se avisa y no se toca esa memoria.
     */
    if (!gc_tipo_en_imagen(type))
    {
        godc_marca("gc-tipo-fuera-imagen", (unsigned long)(uintptr_t)type);
        return;
    }
"""

ANCLA_SINGLE_CALLS = [
    ("""            void *elem = (uint8_t *)obj + i * type->__size;
            gc_scan_single_element(elem, type);""",
     """            void *elem = (uint8_t *)obj + i * type->__size;
            gc_scan_single_element(elem, type, type->__size);"""),
    ("""    gc_scan_single_element(obj, type);
}""",
     """    gc_scan_single_element(obj, type, obj_size);
}"""),
    ("""        for (size_t k = 0; k < n; k++)
            gc_scan_single_element((uint8_t *)e->ptr + k * tsz, e->type);""",
     """        for (size_t k = 0; k < n; k++)
            gc_scan_single_element((uint8_t *)e->ptr + k * tsz, e->type, tsz);"""),
]

ANCLA_OBJECT_TYPE = """    {
        uintptr_t type_addr = (uintptr_t)type;
        bool in_p1_ram = (type_addr >= 0x8c000000 && type_addr < 0x8d000000);
        bool in_p2_ram = (type_addr >= 0xac000000 && type_addr < 0xad000000);

        if (!in_p1_ram && !in_p2_ram)
        {
            gc_scan_range_conservative(obj, obj_size);
            return;
        }
    }
"""

NUEVO_OBJECT_TYPE = """    if (!gc_tipo_en_imagen(type))
    {
        /* DC-002.92: el encabezado dice un tipo que no esta en la imagen -> encabezado falso (o tipo
         * reciclado). No se puede confiar en `__ptrdata`: se barre conservador y acotado al objeto. */
        godc_marca("gc-tipo-fuera-imagen", (unsigned long)(uintptr_t)type);
        gc_scan_range_conservative(obj, obj_size);
        return;
    }
"""

ANCLA_ROOT = """static void gc_scan_root_variable(gc_root_t *root)
{
    if (!root->decl || root->ptrdata == 0)
        return;

    if (root->gcdata)
    {
        /* Use precise scanning with gcdata bitmap (optimized for SH-4) */
        gc_scan_gcdata_bitmap(root->decl, root->gcdata,
                              root->ptrdata / sizeof(void *));
    }
    else
    {
        /* Fallback to conservative scan */
        gc_scan_range_conservative(root->decl, root->ptrdata);
    }
}
"""

NUEVO_ROOT = """static void gc_scan_root_variable(gc_root_t *root)
{
    size_t ptrdata;

    if (!root->decl || root->ptrdata == 0)
        return;

    /* DC-002.92: una raiz tambien es un registro de tipo, y se usa igual de crudo. Se exige lo mismo que a
     * un objeto: la variable en RAM, el gcdata en la imagen y `ptrdata` que no pase de la variable. */
    if ((uintptr_t)root->decl < DC_RAM_START || (uintptr_t)root->decl >= DC_RAM_END)
    {
        godc_marca("gc-raiz-fuera-ram", (unsigned long)(uintptr_t)root->decl);
        return;
    }

    ptrdata = root->ptrdata;

    if (root->size != 0 && ptrdata > root->size)
    {
        godc_marca("gc-raiz-ptrdata-recortado", (unsigned long)ptrdata);
        ptrdata = root->size;
    }

    if (root->gcdata && gc_dato_en_imagen(root->gcdata))
    {
        /* Use precise scanning with gcdata bitmap (optimized for SH-4) */
        gc_scan_gcdata_bitmap(root->decl, root->gcdata,
                              ptrdata / sizeof(void *));
    }
    else
    {
        if (root->gcdata)
            godc_marca("gc-raiz-gcdata-fuera-imagen", (unsigned long)(uintptr_t)root->gcdata);

        /* Fallback to conservative scan */
        gc_scan_range_conservative(root->decl, ptrdata);
    }
}
"""

ANCLA_SINGLE_TAIL = """    // Check if type uses GC program (KindGCProg) instead of bitmap
    // GC programs are used for large types (typically >128 bytes of ptrdata)
    // For Dreamcast, fall back to conservative scan for these rare cases
    if (GC_TYPE_USES_GCPROG(type))
    {
        gc_scan_range_conservative(elem, type->__ptrdata);
        return;
    }

    // Use precise scanning with gcdata bitmap
    if (type->__gcdata)
    {
        gc_scan_element_with_gcdata(elem, type);
    }
    else
    {
        // No gcdata available - conservative scan
        gc_scan_range_conservative(elem, type->__ptrdata);
    }
"""

NUEVO_SINGLE_TAIL = """    /* DC-002.92: lo que se barre no puede pasar del objeto/elemento, diga lo que diga el tipo. */
    {
        size_t ptrdata = type->__ptrdata;

        if (limite != 0 && ptrdata > limite)
        {
            godc_marca("gc-ptrdata-recortado", (unsigned long)ptrdata);
            ptrdata = limite;
        }

        // Check if type uses GC program (KindGCProg) instead of bitmap
        // GC programs are used for large types (typically >128 bytes of ptrdata)
        // For Dreamcast, fall back to conservative scan for these rare cases
        if (GC_TYPE_USES_GCPROG(type))
        {
            gc_scan_range_conservative(elem, ptrdata);
            return;
        }

        // Use precise scanning with gcdata bitmap
        if (type->__gcdata)
        {
            gc_scan_element_with_gcdata(elem, type, limite);
        }
        else
        {
            // No gcdata available - conservative scan
            gc_scan_range_conservative(elem, ptrdata);
        }
    }
"""

FIX = [
    ("helpers", ANCLA_HELPERS, NUEVO_HELPERS),
    ("gcdata-check", ANCLA_GCDATA_CHECK, NUEVO_GCDATA_CHECK),
    ("single", ANCLA_SINGLE, NUEVO_SINGLE),
    ("single-check", ANCLA_SINGLE_CHECK, NUEVO_SINGLE_CHECK),
    ("single-cola", ANCLA_SINGLE_TAIL, NUEVO_SINGLE_TAIL),
    ("objeto-tipo", ANCLA_OBJECT_TYPE, NUEVO_OBJECT_TYPE),
    ("raiz", ANCLA_ROOT, NUEVO_ROOT),
]


def estado():
    t = COPY.read_text()
    sensor = "sonda_fuera_ram" in t
    arreglo = ("gc_tipo_en_imagen" in t and "gc_scan_single_element(void *elem, struct __go_type_descriptor *type, size_t limite)" in t)
    a = pathlib.Path(LIB + "/libgodc.a")
    print("sensor: %s | arreglo: %s | %s (sha256 %s)" %
          ("PUESTO" if sensor else "QUITADO", "PUESTO" if arreglo else "QUITADO", LIB,
           sha(a) if a.exists() else "-"))
    return sensor, arreglo


# ============================== 3. EL TOPE DE LA IMAGEN (gc_heap.c, siempre) ==============================
HEAP = pathlib.Path(LIB + "/runtime/gc_heap.c")

ANCLA_GC_INIT = """void gc_init(void)
{
    godc_marca("  gc_init-entra", 0);   /* ARQ-MARCA (DC-002.29) */
"""

NUEVO_GC_INIT = """extern uintptr_t gc_imagen_hi;   /* DC-002.92: lo define gc_copy.c, lo pone este arranque */
extern void *sbrk(intptr_t);

void gc_init(void)
{
    godc_marca("  gc_init-entra", 0);   /* ARQ-MARCA (DC-002.29) */

    /* DC-002.92: el tope de la IMAGEN, tomado ANTES de pedir los semiespacios (o sea antes de que este monton
     * tenga nada). Es lo que separa un descriptor de tipo -- dato compilado, adentro de la imagen -- de una
     * palabra cualquiera del monton. Ver `gc_tipo_en_imagen()` en gc_copy.c. */
    if (gc_imagen_hi == 0)
    {
        gc_imagen_hi = (uintptr_t)sbrk(0);
        godc_marca("  gc-imagen-hi", (unsigned long)gc_imagen_hi);
    }
"""



if "--estado" in sys.argv:
    print("== estado de la tarjeta DC-002.92")
    sys.exit(0)

print("== DC-002.92: el barrido del GC fuera de RAM (libgodc %s)" % LIB)

print("   el sensor y el tope de la imagen (siempre)")
reemplazar(COPY, ANCLA_SENSOR, NUEVO_SENSOR, "sensor", marca="static void sonda_fuera_ram(")
reemplazar(HEAP, ANCLA_GC_INIT, NUEVO_GC_INIT, "gc_init (tope de imagen)")

if QUITAR:
    print("   --quitar: se saca SOLO el arreglo; el sensor queda (es el testigo del mutante)")
    for et, viejo, nuevo in FIX:
        deshacer(COPY, nuevo, viejo, et)
    # los tres sitios de llamada vuelven a la firma vieja: se deshace al reves de como se aplico
else:
    print("   el sensor (siempre)")
    reemplazar(COPY, ANCLA_SENSOR, NUEVO_SENSOR, "sensor", marca="static void sonda_fuera_ram(")
    if not SOLO_SONDA:
        print("   el arreglo")
        for et, viejo, nuevo in FIX:
            reemplazar(COPY, viejo, nuevo, et)
        for viejo, nuevo in ANCLA_SINGLE_CALLS:
            reemplazar(COPY, viejo, nuevo, "sitio")

if SOLO_SONDA or QUITAR:
    # con la firma vieja de gc_scan_single_element, los tres sitios tienen que estar sin `limite`
    t = COPY.read_text()
    for viejo, nuevo in ANCLA_SINGLE_CALLS:
        if nuevo in t:
            COPY.write_text(t.replace(nuevo, viejo, 1))
            t = COPY.read_text()
            print("  firma vieja en un sitio de llamada")

sensor, arreglo = estado()

if BUILD:
    subprocess.run(["bash", "-c", "rm -f %s/runtime/*.o %s/*.a" % (LIB, LIB)], check=True)
    r = subprocess.run(["make", "-C", LIB, "all"], capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout[-4000:] + r.stderr[-4000:])
        sys.exit("make fallo")
    print("   make ok: " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    print("   %s/libgodc.a: %d bytes, sha256 %s" %
          (LIB, os.path.getsize(LIB + "/libgodc.a"), sha(pathlib.Path(LIB + "/libgodc.a"))))
else:
    print("   falta: correr con --build (rm -f %s/runtime/*.o && make -C %s all)" % (LIB, LIB))
print("   despues: reenlazar  scripts/console/build-gc-fuera-ram.sh  y correr Flycast")
