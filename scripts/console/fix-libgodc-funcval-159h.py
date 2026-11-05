#!/usr/bin/env python3
"""DC-002.159h -- EL HASHER DE UN MAPA ES UN FuncVal LA MITAD DE LAS VECES, Y LIBGODC LO LLAMABA COMO CODIGO.

EL DEFECTO (medido)
    `runtime.mapassign` hace `MAPTYPE_HASHER(t)(key, h->hash0)`, y el campo `__hasher` del descriptor del tipo
    NO siempre es la direccion del CODIGO:

      * clave ESCALAR (uint32, uint64, uint8, string, float64): gccgo pone en el descriptor el hasher del
        runtime, y libgodc lo define como CODIGO (`_runtime.memhash32..f`, `_runtime.strhash..f`, ...). Llamarlo
        derecho anda -- y anda hoy: la sonda `console/dc/poc/mapas-159h` escribe y lee `map[uint32]int`,
        `map[float64]int` y `map[string]int` sin morir.
      * clave COMPUESTA o NOMBRADA ([2]float64, [3]int, un struct con dos floats): el hasher lo GENERA EL
        COMPILADOR, que emite el codigo `pkg.T..hash` y ADEMAS un objeto de DATOS `pkg.T..hash..f` en .rodata
        cuya primera palabra es el puntero al codigo; en el descriptor va el objeto de DATOS. Llamarlo derecho
        es saltar a .rodata, y el SH-4 muere:

            MAPA159H paso=4 tipo=map[[2]float64]int
            Unhandled exception: PC 8c058852, code 1, evt 0180     <- 2 bytes adentro de ..hash..f

        Es el mismo defecto que mataba el camino HSV del renderer (`map[[2]float64]ColorM`) y, con el, la
        sombra del compuesto (DC-002.159b). `__equalfn` tiene EXACTAMENTE la misma forma y se llamaba igual
        (asi que aun arreglando el hasher, la PRIMERA BUSQUEDA de una clave ya metida volvia a saltar a datos).

    NO es "el hasher de los floats": `map[[2]int]int` ANDA (8 bytes: gccgo lo manda al `memhash64` de libgodc)
    y `map[[3]int]int` MUERE (12 bytes: hasher generado). La frontera es "lo emite el compilador o no", no el
    tipo de la clave. Medido en la consola con la sonda de 7 casos.

EL ARREGLO
    Antes de llamar, el puntero se resuelve con `godc_codigo_gccgo`: si es una de las funciones que aporta
    libgodc (lista cerrada, las 26 de abajo) se llama DERECHO; si no lo es, es un FuncVal y se lee su primera
    palabra, que es el codigo. Igual para `__equalfn`.

    POR QUE UNA LISTA Y NO UN RANGO DE SECCIONES. Lo natural seria preguntar "¿el puntero cae en .text?", y el
    enlazador sabe el limite (`PROVIDE (__etext = .)` en el script de KOS). Medido en esta tarjeta: NO SIRVE.
    `__etext` queda sin definir cuando la unica referencia viene de un miembro de un archivo `.a` (el error es
    `undefined reference to __etext` desde `map_dreamcast.o`, con el script puesto y con el layout correcto):
    en el Gate 0 existe solo porque algun objeto de `libkallisti.a` tambien la referencia. Una lista de
    direcciones no depende de nadie, es la misma informacion (que funciones son codigo de libgodc) y se
    verifica: `scripts/console/checar-mapas-159h.sh` la contrasta contra los 338 descriptores de mapa del ELF.

    Se toca SOLO `map_dreamcast.h` (la lista, el ayudante y el macro) y `map_dreamcast.c` (`keyEqual`): son los
    dos archivos de este arreglo, y ninguno esta en la cadena de los otros `fix-libgodc-*.py`, asi que se puede
    aplicar y quitar sin pisar a nadie.

    OJO, LO QUE ESTE SCRIPT NO TOCA (medido, y anotado): `interface_dreamcast.c` llama `__equalfn` derecho en
    cinco lugares (lineas 635, 656, 690, 714, 739 del arbol pristino). Es el mismo defecto, pero es el camino de
    la IGUALDAD DE INTERFACES, no el de los mapas: no hay una corrida medida que lo muestre como crash.

USO
    python3 scripts/console/fix-libgodc-funcval-159h.py [ruta_libgodc] [--build] [--quitar]
    Despues: reenlazar (build-dc.sh para un POC, build-gate0-159.sh para el Gate 0) y correr Flycast.

ORDEN E IDEMPOTENCIA
    Aplica PRIMERO la cadena documentada del runtime (ABI -> interfaces -> GC -> copia por Store Queue, o sea
    `fix-libgodc-gc.py <KB>`), despues devuelve SUS DOS archivos al pristino con `git checkout` y aplica este
    arreglo. Correrlo dos veces deja el mismo sha256. `--quitar` deja el arbol como estaba antes de este
    script (la cadena de los demas queda puesta, igual que con `fix-libgodc-map-traza.py`).

    OJO CON EL SEMIESPACIO: el valor de la RAMA es 1792 KB, no los 512 KB de DC-002.25 que trae el script por
    defecto. Medido en esta tarjeta: correr la cadena con 512 deja la corrida del Gate 0 muriendo a los 19 s
    (`Unhandled exception: PC 00000001`) en la mitad de la carga de las tablas, porque el runtime pide
    `2 x 512 KB` y el arranque ya venia midiendo `gcTotal = 1.835.008 B`. El valor se toma de
    `scripts/console/medir-semiespacio-102.sh` ("para volver al valor de la rama: fix-libgodc-gc.py 1792") y se
    puede pisar con `KB_159H=<n>`.
"""
import hashlib
import os
import pathlib
import subprocess
import sys

args = [a for a in sys.argv[1:] if not a.startswith("--")]
LIB = args[0] if args else os.path.expanduser("~/dreamcast/libgodc")
AQUI = os.path.dirname(os.path.abspath(__file__))
HEADER = pathlib.Path(LIB + "/runtime/map_dreamcast.h")
MAPA = pathlib.Path(LIB + "/runtime/map_dreamcast.c")
MIS_ARCHIVOS = ["runtime/map_dreamcast.h", "runtime/map_dreamcast.c"]
QUITAR = "--quitar" in sys.argv
# el semiespacio de la RAMA (ver medir-semiespacio-102.sh). NO el 512 del default de fix-libgodc-gc.py.
KB = os.environ.get("KB_159H", "1792")

# EL TOOLCHAIN EN EL PATH. La cadena (`fix-libgodc-cadena.sh`) se carga `env.sh` sola, pero los `make` de este
# script corren en subprocesos: sin `$DC_ROOT/sh-elf/bin` en el PATH, `make` muere con
# "sh-elf-gcc: No existe el fichero o el directorio" (medido). Se arma desde el arbol del runtime:
# LIB es <dc_root>/libgodc.
_DC_ROOT = os.path.dirname(os.path.abspath(LIB))
_ENTORNO = dict(os.environ)
_ENTORNO["PATH"] = os.path.join(_DC_ROOT, "sh-elf", "bin") + os.pathsep + _ENTORNO.get("PATH", "")
_ENTORNO.setdefault("KOS_BASE", os.path.join(_DC_ROOT, "kos"))

# Las funciones que libgodc define como CODIGO y que pueden aparecer en `__hasher` o en `__equalfn`. Es la
# lista completa de `__asm__("_runtime.*..f")` de libgodc cuyo nombre es un hash o una igualdad (medida con
# `sh-elf-nm ~/dreamcast/libgodc/libgodc.a | grep -E " [TtW] _runtime\\."`). Todo lo que NO este aca y venga en
# un descriptor es un FuncVal del compilador.
FUNCIONES_DE_LIBGODC = [
    # hashers
    "memhash0..f", "memhash8..f", "memhash16..f", "memhash32..f", "memhash64..f", "memhash128..f",
    "strhash..f", "f32hash..f", "f64hash..f", "interhash", "interhash..f", "nilinterhash",
    "nilinterhash..f",
    # igualdades
    "memequal", "memequal0", "memequal0..f", "memequal8..f", "memequal16..f", "memequal32..f",
    "memequal64..f", "memequal128..f", "strequal..f", "f32equal..f", "f64equal..f", "c64equal..f",
    "c128equal..f", "interequal..f", "nilinterequal..f",
]


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()[:16]


# --estado: NO TOCA NADA. El 2026-11-05 correr este script con un argumento `--estado` que no entendia lo hizo
# ejecutar la cadena entera y dejar el arbol a medias a mitad de otra medicion; ahora el estado se contesta y se
# sale. Los pedazos que se miran son los tres del arreglo y los tres que la cadena se lleva.
if "--estado" in sys.argv:
    def hay(patron, archivo="runtime/map_dreamcast.h"):
        ruta = pathlib.Path(LIB, archivo)
        return patron in ruta.read_text() if ruta.exists() else False

    piezas = [
        ("este arreglo: el hasher resuelto (map_dreamcast.h)", "godc_codigo_gccgo", "runtime/map_dreamcast.h"),
        ("este arreglo: la igualdad resuelta (map_dreamcast.c)", "godc_codigo_gccgo(keytype->__equalfn)", "runtime/map_dreamcast.c"),
        ("este arreglo: el valor indirecto (map_dreamcast.c)", "MAPTYPE_INDIRECT_VALUE", "runtime/map_dreamcast.c"),
        ("la cadena: la marca gc-vivo (DC-002.45/47)", 'godc_marca("gc-vivo"', "runtime/gc_copy.c"),
        ("la cadena: el sensor fuera de RAM (DC-002.92)", "gc_tipo_en_imagen", "runtime/gc_copy.c"),
        ("la cadena: la tabla de tipos (DC-002.146d)", "gc_tipos_tabla", "runtime/gc_copy.c"),
        ("el semiespacio de la RAMA (1792 KB)", "#define GC_SEMISPACE_SIZE_KB 1792", "runtime/godc_config.h"),
    ]
    print(f"== estado de DC-002.159h en {LIB}")
    for nombre, patron, archivo in piezas:
        print(f"   {'PUESTO ' if hay(patron, archivo) else 'QUITADO'}  {nombre}")
    a = pathlib.Path(LIB, "libgodc.a")
    print(f"   libgodc.a: {a.stat().st_size} bytes, sha256 {sha(a)}" if a.exists() else "   libgodc.a: no esta")
    sys.exit(0)


print(f"== 1/4 el arbol pristino + los arreglos que ya existian (ABI, interfaces, GC {KB} KB y Store Queue)")
if QUITAR:
    print("   --quitar: no se corre la cadena (solo se saca este arreglo)")
else:
    # LA CADENA NO ES IDEMPOTENTE SI `gc_heap.c` TRAE PARCHEOS DE OTRAS TARJETAS: su paso de las marcas de
    # arranque (DC-002.29) busca el texto pristino del limite del monton y, si `gc_heap.c` ya trae la tabla de
    # tipos (DC-002.146d) o un instrumento, corta con "el texto ... aparece 0 veces" y deja el arbol a medias
    # (medido en esta tarjeta, dos veces). Se devuelven al pristino los dos archivos que la cadena reescribe.
    subprocess.run(["git", "-C", LIB, "checkout", "--",
                    "runtime/gc_copy.c", "runtime/gc_heap.c"], check=True)
    print("   pristinos antes de la cadena: runtime/gc_copy.c, runtime/gc_heap.c")
    # LA CADENA ENTERA, no solo gc.py: `fix-libgodc-gc.py` resetea `interface_dreamcast.c` y se lleva el
    # arreglo de los itabs (DC-002.39e / DC-002.45). Medido en esta tarjeta: correr gc.py a secas dejaba el
    # arbol con "FALTA el arreglo de los itabs" (`fix-libgodc-cadena.sh --verificar`). El punto de entrada de
    # la cadena es `fix-libgodc-cadena.sh` (gc -> candidato -> Store Queue -> itab -> itab-245).
    r = subprocess.run(["bash", os.path.join(AQUI, "fix-libgodc-cadena.sh"), LIB, "--kb", KB],
                       capture_output=True, text=True)
    print("   " + (r.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    if r.returncode != 0:
        print(r.stdout + r.stderr)
        sys.exit("fix-libgodc-cadena.sh fallo")
    v = subprocess.run(["bash", os.path.join(AQUI, "fix-libgodc-cadena.sh"), LIB, "--kb", KB, "--verificar"],
                       capture_output=True, text=True)
    print("   " + (v.stdout.strip().splitlines() or ["(sin salida)"])[-1])
    if v.returncode != 0:
        print(v.stdout + v.stderr)
        sys.exit("la cadena quedo INCOMPLETA")

    # LOS PARCHEOS QUE LA CADENA SE LLEVA. `fix-libgodc-cadena.sh` resetea `gc_copy.c` y `gc_heap.c`, y con eso
    # se van tres cosas que las tarjetas siguientes habian dejado puestas:
    #   * DC-002.45/47 -> la marca `gc-vivo` (el instrumento del dato vivo): `instrumentar-vivo-245.py`
    #   * DC-002.92    -> el sensor fuera de RAM y el tope de la imagen: `fix-libgodc-tipos.py`
    #   * DC-002.146d  -> la tabla de tipos del GC (el arreglo del panic por tipo invalido): `fix-libgodc-tipos-reales.py`
    # Sin las dos ultimas, `gc_heap.c` queda llamando `gc_tipo_registrar` y `libgodc.a` no enlaza
    # (`undefined reference to gc_tipo_registrar` / `gc_imagen_hi`, medido en esta tarjeta). Cada una se aplica
    # SOLO si su marca no esta: son idempotentes de verdad y asi no se aplican dos veces.
    def ya_esta(patron, archivo="runtime/gc_copy.c"):
        return subprocess.run(["grep", "-cF", patron, os.path.join(LIB, archivo)],
                              capture_output=True, text=True).stdout.strip()

    vivos = [
        ("la marca gc-vivo (DC-002.45/47)", 'godc_marca("gc-vivo"', "instrumentar-vivo-245.py"),
        ("el sensor fuera de RAM y el tope de la imagen (DC-002.92)", "gc_tipo_en_imagen", "fix-libgodc-tipos.py"),
        ("la tabla de tipos del GC (DC-002.146d)", "gc_tipos_tabla", "fix-libgodc-tipos-reales.py"),
    ]
    for nombre, marca, script in vivos:
        ruta = os.path.join(AQUI, script)
        if not os.path.exists(ruta):
            print(f"   AVISO: no esta {script}: queda sin {nombre}")
            continue
        if ya_esta(marca) not in ("", "0") and not (nombre.endswith("(DC-002.92)") and ya_esta('gc_scan_single_element(void *elem, struct __go_type_descriptor *type, size_t limite)') in ("", "0")):
            print(f"   ya puesto: {nombre}")
            continue
        p = subprocess.run([sys.executable, ruta, LIB], capture_output=True, text=True)
        print(f"   {nombre}: " + (p.stdout.strip().splitlines() or ["(sin salida)"])[-1])
        # DC-002.92 aborta en un TERCER sitio de llamada que este vendor no tiene (tiene dos: `gc_copy.c`
        # lineas 624/741/756 del vendor). Si el pago quedo puesto, se sigue: eso es lo que se comprueba.
        if p.returncode != 0 and ya_esta("gc_tipo_en_imagen") not in ("", "0"):
            print(f"     (el script corto con rc={p.returncode}, pero el pago esta puesto: sigo)")
        elif p.returncode != 0:
            print(p.stdout + p.stderr)
            sys.exit(f"{script} fallo")

# estos dos son del script: al pristino, para que correrlo dos veces sea lo mismo
subprocess.run(["git", "-C", LIB, "checkout", "--"] + MIS_ARCHIVOS, check=True)
print("   pristinos: " + ", ".join(MIS_ARCHIVOS))

if QUITAR:
    print("== 2/4 --quitar: el arbol queda sin este arreglo")
    if "--build" in sys.argv:
        subprocess.run(["bash", "-c", f"rm -f {LIB}/runtime/*.o {LIB}/*.a"], check=True)
        r = subprocess.run(["make", "-C", LIB, "all"], capture_output=True, text=True, env=_ENTORNO)
        if r.returncode != 0:
            print(r.stdout[-3000:] + r.stderr[-3000:])
            sys.exit("make fallo")
        print(f"   {LIB}/libgodc.a: {os.path.getsize(LIB + '/libgodc.a')} bytes (runtime SIN el arreglo)")
    sys.exit(0)

print("== 2/4 la lista, el ayudante y el macro del hasher (map_dreamcast.h)")

VIEJO_MACRO = """// gccgo hasher signature: uintptr_t hasher(void *key, uintptr_t seed)
#define MAPTYPE_HASHER(t) ((uintptr_t (*)(void *, uintptr_t))((t)->__hasher))"""

declaraciones = "\n".join(
    f'extern char __g159h_{n.replace(".", "_").replace("-", "_")}[] __asm__("_runtime.{n}");'
    for n in FUNCIONES_DE_LIBGODC)
entradas = "\n".join(
    f'    (const void *)__g159h_{n.replace(".", "_").replace("-", "_")},' for n in FUNCIONES_DE_LIBGODC)

NUEVO_MACRO = f"""/* DC-002.159h: LO QUE HAY EN __hasher/__equalfn ES UN FuncVal LA MITAD DE LAS VECES.
 *
 * El descriptor de tipo guarda un void* que NO siempre es el codigo de la funcion:
 *
 *   - cuando la funcion la aporta el RUNTIME (los hashers de clave escalar: _runtime.memhash32..f,
 *     _runtime.strhash..f, _runtime.f64hash..f) libgodc la define como CODIGO, y el descriptor apunta al .text:
 *     llamarla derecho anda;
 *   - cuando la funcion la GENERA EL COMPILADOR (toda clave compuesta o nombrada que no cae en un hasher de
 *     libgodc: [2]float64, [3]int, un struct con floats...) gccgo emite el codigo `X..hash` Y ADEMAS un objeto
 *     de DATOS `X..hash..f` en .rodata cuya PRIMERA palabra es el puntero al codigo, y en el descriptor va el
 *     objeto de DATOS. Llamarlo derecho es saltar a .rodata.
 *
 * Medido en DC-002.159h con `console/dc/poc/mapas-159h`: `map[uint32]int`, `map[float64]int` y
 * `map[string]int` andan (hasher de libgodc) y `map[[2]float64]int` mata el SH-4 con
 * `Unhandled exception: PC 8c058852` -- 2 bytes adentro de `_main._62_7float64..hash..f`, que esta en .rodata.
 *
 * El discriminador es la LISTA de las funciones que aporta libgodc (abajo). No es un rango de secciones:
 * `PROVIDE (__etext = .)` del script de KOS queda sin definir cuando la unica referencia viene de un miembro de
 * un archivo `.a` (medido: `undefined reference to __etext` desde este mismo map_dreamcast.o).
 */
{declaraciones}

// las funciones de libgodc que pueden venir en un descriptor de tipo: hash e igualdad
static const void *const godc_funciones_de_libgodc[] = {{
{entradas}
}};

MAP_INLINE void *godc_codigo_gccgo(void *funcval)
{{
    uintptr_t i;

    if (funcval == NULL)
    {{
        return NULL;
    }}

    for (i = 0; i < sizeof(godc_funciones_de_libgodc) / sizeof(godc_funciones_de_libgodc[0]); i++)
    {{
        if (funcval == godc_funciones_de_libgodc[i])
        {{
            return funcval; /* es codigo de libgodc: se llama derecho */
        }}
    }}

    return *(void **)funcval; /* es un FuncVal del compilador: su primera palabra es el codigo */
}}

// gccgo hasher signature: uintptr_t hasher(void *key, uintptr_t seed)
#define MAPTYPE_HASHER(t) ((uintptr_t (*)(void *, uintptr_t))godc_codigo_gccgo((t)->__hasher))"""

print("== 3/4 la igualdad de claves (map_dreamcast.c, keyEqual) Y EL VALOR INDIRECTO (mapassign)")

VIEJO_EQUAL = """\tif (keytype->__equalfn != NULL) {
\t\t_Bool (*fn)(void *, void *) = (_Bool (*)(void *, void *))keytype->__equalfn;"""

NUEVO_EQUAL = """\tif (keytype->__equalfn != NULL) {
\t\t// DC-002.159h: __equalfn tiene la MISMA forma que __hasher (FuncVal cuando la genera el compilador),
\t\t// y sin resolverla la primera BUSQUEDA de una clave ya metida vuelve a saltar a .rodata.
\t\t_Bool (*fn)(void *, void *) = (_Bool (*)(void *, void *))godc_codigo_gccgo(keytype->__equalfn);"""

# EL SEGUNDO DEFECTO (medido con `poc/mapas-159h`, pasos 8 a 11): un mapa cuyo VALOR no entra en una palabra
# guarda un PUNTERO al valor, y el descriptor del tipo lo dice asi: `__flags` con el bit 1 puesto y
# `__valuesize` = 4 (el tamano del puntero, NO el del valor; el valor de verdad esta en `__elem->__size`).
# `mapassign` copiaba `__valuesize` bytes (o sea nada) y devolvia la palabra del cajon, que esta en cero: el mapa
# "andaba" (la clave quedaba metida y `len` daba 1) pero TODO valor leido volvia cero. Es el caso de las dos
# caches del camino HSV de la consola (`map[[2]float64]ebiten.ColorM` y `map[tintHSVKey]ebiten.ColorM`, con
# `ColorM` de 20 float64 = 160 B): sin esto la sombra del compuesto no se dibuja mal... se dibuja con una matriz
# en cero. La celda la tiene que pedir el runtime, y el cajon la guarda como puntero (el tipo del cajon ya trae
# al valor como campo con puntero, asi que el recolector lo sigue).
VIEJO_VALOR = """    /* Insert new entry */
    th = bucketTophash(insertBucket);
    th[insertSlot] = top;
    keyCopy(t, bucketKey(t, insertBucket, insertSlot), key);
    h->count++;

    h->flags &= ~MAP_FLAG_WRITING;
    result = valuePtr(t, bucketValue(t, insertBucket, insertSlot));"""

NUEVO_VALOR = """    /* Insert new entry */
    th = bucketTophash(insertBucket);
    th[insertSlot] = top;
    keyCopy(t, bucketKey(t, insertBucket, insertSlot), key);
    h->count++;

    /* DC-002.159h: el VALOR INDIRECTO. El cajon guarda un puntero y hay que PEDIR la celda; sin esto el mapa
       devuelve cero en toda lectura (medido: `map[uint32]main.matriz`, valor de 160 B). */
    if (MAPTYPE_FLAGS(t) & MAPTYPE_INDIRECT_VALUE)
    {
        void **celda = (void **)bucketValue(t, insertBucket, insertSlot);

        if (*celda == NULL)
            *celda = gc_alloc(MAPTYPE_ELEM(t)->__size, MAPTYPE_ELEM(t));
    }

    h->flags &= ~MAP_FLAG_WRITING;
    result = valuePtr(t, bucketValue(t, insertBucket, insertSlot));"""


def reemplazar(path, viejo, nuevo):
    t = path.read_text()
    n = t.count(viejo)
    if n != 1:
        sys.exit("ERROR: el texto a reemplazar aparece %d veces (esperaba 1) en %s" % (n, path))
    path.write_text(t.replace(viejo, nuevo, 1))
    print("  ok %s (%d -> %d bytes)" % (path.name, len(viejo), len(nuevo)))


reemplazar(HEADER, VIEJO_MACRO, NUEVO_MACRO)
reemplazar(MAPA, VIEJO_EQUAL, NUEVO_EQUAL)
reemplazar(MAPA, VIEJO_VALOR, NUEVO_VALOR)

print("== 4/4 el runtime")
if "--build" in sys.argv:
    # OJO (medido en los otros fix): `%.o: %.c` no depende de las cabeceras, asi que hay que borrar los .o a mano
    subprocess.run(["bash", "-c", f"rm -f {LIB}/runtime/*.o {LIB}/*.a"], check=True)
    r = subprocess.run(["make", "-C", LIB, "all"], capture_output=True, text=True, env=_ENTORNO)
    if r.returncode != 0:
        print(r.stdout[-3000:] + r.stderr[-3000:])
        sys.exit("make fallo")
    print(f"   {LIB}/libgodc.a: {os.path.getsize(LIB + '/libgodc.a')} bytes, sha256 {sha(pathlib.Path(LIB + '/libgodc.a'))}")
else:
    print("   falta --build")

print(f"   map_dreamcast.h sha256={sha(HEADER)}   map_dreamcast.c sha256={sha(MAPA)}")
print(f"   funciones de libgodc en la lista: {len(FUNCIONES_DE_LIBGODC)}")
print("   ahora: reenlazar (build-dc.sh para un POC, build-gate0-159.sh para el Gate 0) y correr Flycast")
