#!/usr/bin/env python3
"""DC-002.39e: EL CACHE DE ITABS de libgodc -- el arreglo (ranuras como raices del GC) y su MUTANTE.

EL DEFECTO (medido, evidencia console/dc/evidencia/dc-002.39e-itab-raiz.txt)
    `get_itab` (runtime/interface_dreamcast.c) arma los `Itab *` de las conversiones entre INTERFACES en el
    monton de Go y los guarda en `static struct { Itab *itabs[32]; int count; } itab_cache`, un array en
    `.bss` de C. El recolector de la consola es copiativo y sus raices son SOLO `gc_root_table`, las listas
    de gccgo y las pilas (`gc_scan_roots` de gc_copy.c, con el comentario explicito "not conservative
    .data/.bss scan"). Un puntero guardado en `.bss` es invisible: cuando el recolector COPIA el itab al otro
    semiespacio la ranura del cache sigue apuntando al viejo, y en la recoleccion siguiente ese puntero se
    reubica como INTERIOR (DC-002.36) -- la tabla de metodos termina siendo la de otro objeto y la llamada
    al metodo salta a un pedazo de DATO. Eso es `PC 656a626f` ("obje" leido como direccion) con el `PR` en
    `bytes.Buffer.ReadFrom`.

    MEDIDO en el POC `itab-raiz` (console/dc/poc/itab-raiz), mismo binario/disco/gox, un solo cambio:
      sin el arreglo  la consola se REINICIA en la primera vuelta (72 arranques, 0 lineas de progreso)
      con el arreglo  completa las 40 vueltas, 820 comprobaciones despues de la recoleccion, v2mal= 0
    Y en el corpus del motor (POC `tablas-mpq-listfile`, con la lectura del `(listfile)` puesta):
      sin el arreglo  11244 de 21253 (d2char.mpq no termina)
      con el arreglo  21253 de 21253 (patch_d2 162/162, d2data 11082/11082, d2char 10009/10009)

MODOS
    (por defecto)  EL ARREGLO. Registra las 32 ranuras del cache con `gc_add_root` (una sola vez, al guardar
                   el primer itab) y deja puestos los CONTADORES de observabilidad (`d2c_itab_llamadas`,
                   `d2c_itab_armados`, `d2c_itab_ultimo`, y el rango del semiespacio activo). Los contadores
                   no cambian el comportamiento: son lo que deja al POC decir si este camino se ejerce, si
                   el itab cae en el monton de Go y cuantos itabs se reservan en total.
    --sin-raices   EL MUTANTE. Lo mismo SIN el `gc_add_root`. Es el control de la suite: con esto la corrida
                   del POC tiene que ponerse ROJA (reinicio en la primera vuelta). Si el mutante queda verde,
                   el hueco es de la suite, no del arreglo.

    Se registran las 32 ranuras de una vez (y no la que se acaba de guardar) porque `gc_add_root` no se puede
    llamar por cada itab: la tabla de raices del runtime tiene tope 256 y el cache son 32.

USO
    python3 scripts/console/fix-libgodc-itab.py <ruta_libgodc>              # el arreglo
    python3 scripts/console/fix-libgodc-itab.py <ruta_libgodc> --sin-raices # el mutante
    Despues:  make -C <ruta_libgodc> libgodc.a

    Lo llama `fix-libgodc-cadena.sh` como paso 4 (siempre el arreglo), asi que en la practica no se corre a
    mano: la cadena es el punto de entrada unico (DC-002.39d). El mutante lo arma `checar-itab-39e.sh`.

IDEMPOTENCIA
    Deja el archivo PRISTINO con `git checkout --` y vuelve a aplicar lo pedido; despues reaplica el ABI
    (`fix-libgodc-abi.py` toca el MISMO archivo: sin reaplicarlo el compilador falla con "conflicting types
    for runtime_ifaceI2T2P", medido). El sha256 se imprime.

OJO CON EL VENDOR
    ~/dreamcast/libgodc es el arbol vendorizado y NO se toca a mano. Para medir, una copia privada
    (`bash scripts/console/setup-dcroot.sh <dir>`) y ahi la cadena completa.
"""
import hashlib
import pathlib
import subprocess
import sys

ARCH = "runtime/interface_dreamcast.c"

CACHE_ANCLA = """// Simple itab cache (linear search for now)
#define MAX_CACHED_ITABS 32
static struct
{
    Itab *itabs[MAX_CACHED_ITABS];
    int count;
} itab_cache = {.count = 0};
"""

# la observabilidad va SIEMPRE (los dos modos): es lo que deja medir sin adivinar
CONTADORES = """
/* DC-002.39e: LA OBSERVABILIDAD DEL CACHE. Tres numeros que dicen si este camino se ejerce y donde vive el
 * itab (no cambian el comportamiento: solo cuentan):
 *   - d2c_itab_llamadas: cuantas veces entro `get_itab`. Si queda en 0, la conversion la resolvio el
 *     COMPILADOR con un itab estatico de `.rodata` y este cache no tiene nada que ver;
 *   - d2c_itab_armados: cuantos itabs se reservaron en el MONTON DE GO (los que el recolector puede mover).
 *     Con el arreglo queda en 1 (el cache acierta); sin el arreglo sube (el cache falla y rearma);
 *   - d2c_itab_ultimo: la direccion del ultimo armado, para compararla con el rango del semiespacio.
 * El rango del semiespacio se expone aca (y no en el POC) porque este archivo ya incluye gc_semispace.h, que
 * es el que declara `gc_heap`. */
static volatile int d2c_itab_llamadas = 0;
static volatile int d2c_itab_armados = 0;
static void *d2c_itab_ultimo = NULL;

int d2c_itab_llamadas_cuenta(void) { return d2c_itab_llamadas; }
int d2c_itab_armados_cuenta(void) { return d2c_itab_armados; }
void *d2c_itab_ultimo_ptr(void) { return d2c_itab_ultimo; }

void *d2c_semi_activo(void) { return gc_heap.space[gc_heap.active_space]; }
void *d2c_semi_activo_fin(void) { return gc_heap.alloc_limit; }
"""

RAICES = """
/* DC-002.39e: EL ARREGLO. Las ranuras del cache son punteros a objetos del MONTON DE GO, pero el cache vive
 * en `.bss` de C y el recolector de la consola no barre `.bss` (ver `gc_scan_roots` en gc_copy.c: "not
 * conservative .data/.bss scan"). Registrandolas con `gc_add_root`, el recolector las trata como una
 * variable de Go mas y las actualiza cuando copia el itab. Se registran las 32 de una vez y una sola vez. */
void gc_add_root(void **root_ptr);

static int d2c_itab_raices_puestas = 0;

static void d2c_itab_registrar_raices(void)
{
    if (d2c_itab_raices_puestas)
        return;

    for (int i = 0; i < MAX_CACHED_ITABS; i++)
        gc_add_root((void **)&itab_cache.itabs[i]);

    d2c_itab_raices_puestas = 1;
}
"""

CONTEOS_ANCLA = [
    (
        """    // Check cache first
    for (int i = 0; i < itab_cache.count; i++)
""",
        """    d2c_itab_llamadas++;

    // Check cache first
    for (int i = 0; i < itab_cache.count; i++)
""",
        "el conteo de llamadas",
    ),
    (
        """    Itab *itab = (Itab *)runtime_malloc(total_size);
    if (!itab)
        return NULL;
""",
        """    Itab *itab = (Itab *)runtime_malloc(total_size);
    if (!itab)
        return NULL;

    d2c_itab_armados++;
    d2c_itab_ultimo = itab;
""",
        "el conteo de armados",
    ),
]

GUARDADO_ANCLA = """    // Add to cache if space available
    if (itab_cache.count < MAX_CACHED_ITABS)
    {
        itab_cache.itabs[itab_cache.count++] = itab;
    }
"""

GUARDADO_ARREGLO = """    // Add to cache if space available
    if (itab_cache.count < MAX_CACHED_ITABS)
    {
        itab_cache.itabs[itab_cache.count++] = itab;

        // DC-002.39e: las ranuras del cache son raices del GC (ver arriba)
        d2c_itab_registrar_raices();
    }
"""


def aplicar(texto, ancla, nuevo, que):
    if texto.count(ancla) != 1:
        print(f"ERROR: el ancla de {que} aparece {texto.count(ancla)} veces (tiene que ser 1)", file=sys.stderr)
        sys.exit(2)

    return texto.replace(ancla, nuevo)


def main():
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    modo = "mutante" if "--sin-raices" in sys.argv else "arreglo"

    if not args:
        print(__doc__)
        sys.exit(2)

    lib = pathlib.Path(args[0])
    arch = lib / ARCH

    if not arch.exists():
        print(f"ERROR: no existe {arch}", file=sys.stderr)
        sys.exit(2)

    # pristino primero (idempotencia). NO se resetea a ciegas el arbol: el vendor tiene la cadena encima
    # (DC-002.39d) y lo que se resetea es SOLO este archivo; despues se reaplica el ABI porque
    # `fix-libgodc-abi.py` toca el mismo archivo (firma de ifaceI2I2/ifaceI2T2P) y hay que dejarlo alineado.
    subprocess.run(["git", "-C", str(lib), "checkout", "--", ARCH], check=True)

    abi = pathlib.Path(__file__).resolve().parent / "fix-libgodc-abi.py"
    if abi.exists():
        subprocess.run([sys.executable, str(abi), str(lib)], check=True)

    texto = arch.read_text()
    original = texto

    bloque = CACHE_ANCLA + CONTADORES + (RAICES if modo == "arreglo" else "")
    texto = aplicar(texto, CACHE_ANCLA, bloque, "el bloque del cache")

    for ancla, nuevo, que in CONTEOS_ANCLA:
        texto = aplicar(texto, ancla, nuevo, que)

    if modo == "arreglo":
        texto = aplicar(texto, GUARDADO_ANCLA, GUARDADO_ARREGLO, "el registro de las raices")

    arch.write_text(texto)

    h = hashlib.sha256(arch.read_bytes()).hexdigest()
    print(f"modo:      {modo}")
    print(f"archivo:   {arch}")
    print(f"sha256:    {h[:32]}")
    print(f"cambio:    {len(texto) - len(original):+d} bytes")
    print(f"raices:    {texto.count('d2c_itab_registrar_raices')} marcas (0 = mutante, 2 = definicion + llamada)")
    print(f"siguiente: make -C {lib} libgodc.a")


if __name__ == "__main__":
    main()
