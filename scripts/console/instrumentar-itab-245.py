#!/usr/bin/env python3
"""DC-002.45 -- INSTRUMENTO (no arreglo): mide si el cache de itabs queda colgando del monton movible.

HIPOTESIS
    El crash del camino "por trozos" no es de memoria ni del csv: la CPU salta a EJECUTAR memoria del
    semiespacio (PC 8cbe3532 = semiespacio+0x114f2, PR = un `jsr @r10` con r10 dentro del semiespacio),
    justo despues de un `gc_collect`. El cache de itabs (`itab_cache`, estatico en `.bss`) guarda punteros
    a itabs que `get_itab` asigna con `runtime_malloc` = monton MOVIBLE de Go. El GC no escanea `.bss`
    (gc_copy.c: "gc_scan_globals() was removed - redundant with registerGCRoots()"), asi que tras una
    recoleccion el cache apunta a la mitad VIEJA del semiespacio, que se reusa. Un acierto de cache
    posterior devuelve un itab pisado y el `jsr` salta a basura.

QUE HACE
    Dos marcas en `get_itab` que NO cambian el comportamiento:
      itab-cache-acierto <dir>   cada acierto de cache
      itab-cache-VIEJO <dir>     el acierto apunta FUERA del semiespacio activo (colgante)
    Si `itab-cache-VIEJO` sale justo antes de la excepcion, la hipotesis esta medida.

Uso:  instrumentar-itab-245.py <libgodc> [--quitar]
"""
import sys

ANCLA = """    // Check cache first
    for (int i = 0; i < itab_cache.count; i++)
    {
        Itab *cached = itab_cache.itabs[i];
        if (cached && cached->inter == inter && itab_type(cached) == type)
        {
            return cached;
        }
    }
"""

NUEVO = """    // DC-002.45 (INSTRUMENTO): el cache de itabs es estatico (`.bss`) y guarda punteros a itabs que
    // `get_itab` asigna en el MONTON MOVIBLE de Go. El GC no escanea `.bss`, asi que despues de una
    // recoleccion el cache puede apuntar a la mitad VIEJA del semiespacio (que se reusa): un acierto
    // posterior devuelve un itab pisado y el `jsr` del metodo salta a basura.
    for (int i = 0; i < itab_cache.count; i++)
    {
        Itab *cached = itab_cache.itabs[i];
        if (cached && cached->inter == inter && itab_type(cached) == type)
        {
            godc_marca("itab-cache-acierto", (unsigned long)(uintptr_t)cached);

            if (itab_colgando(cached))
            {
                godc_marca("itab-cache-VIEJO", (unsigned long)(uintptr_t)cached);
            }

            return cached;
        }
    }
"""

AYUDA = """
// DC-002.45 (INSTRUMENTO): true si el itab NO cae en el semiespacio ACTIVO (o sea: el GC lo dejo atras).
static inline bool itab_colgando(const Itab *itab)
{
    uintptr_t p = (uintptr_t)itab;
    uintptr_t lo = (uintptr_t)gc_heap.space[gc_heap.active_space];
    uintptr_t hi = lo + gc_heap.space_size;

    return p < lo || p >= hi;
}

extern void godc_marca(const char *etiqueta, unsigned long valor);

"""


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        return 2

    lib = sys.argv[1].rstrip("/")
    quitar = "--quitar" in sys.argv
    ruta = lib + "/runtime/interface_dreamcast.c"

    actual = open(ruta).read()

    if quitar:
        if NUEVO not in actual:
            print("no estaba instrumentado")
            return 0
        actual = actual.replace(NUEVO, ANCLA).replace(AYUDA, "")
        open(ruta, "w").write(actual)
        print("instrumento quitado")
        return 0

    if NUEVO in actual:
        print("ya estaba instrumentado")
        return 0

    if ANCLA not in actual:
        print("no encontre el ancla del cache en", ruta)
        return 1

    actual = actual.replace(ANCLA, NUEVO, 1)

    # la ayuda va antes de get_itab
    ancla_get = "// Get or create interface table"
    if ancla_get not in actual:
        print("no encontre el comentario de get_itab")
        return 1
    actual = actual.replace(ancla_get, AYUDA + ancla_get, 1)

    open(ruta, "w").write(actual)
    print("instrumentado:", ruta)
    return 0


if __name__ == "__main__":
    sys.exit(main())
