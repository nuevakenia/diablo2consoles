#!/usr/bin/env python3
"""DC-002.45: LOS ITABS NO VIVEN EN EL MONTON MOVIBLE (el cache de `.bss` los dejaba colgando).

SINTOMA (medido en la consola, no deducido)
    El arranque del motor se cae con `Unhandled exception: PC 8cbe3532` (o `PC 6c000000`), o sea la CPU
    salta a EJECUTAR memoria que no es codigo, siempre DENTRO de un `gc_collect`. La direccion del PC cae
    adentro del semiespacio del GC (8cbe3532 = semiespacio0 + 0x114f2) y el PR es un `jsr @r10` con r10
    apuntando tambien al semiespacio: no es una llamada a una funcion, es un salto a basura.

    La marca del instrumento (`instrumentar-itab-245.py`) lo nombra sin deducir nada:

        ARQ-MARCA itab-cache-acierto v=2362071496
        ARQ-MARCA itab-cache-VIEJO  v=2362071496     <-- FUERA del semiespacio ACTIVO
        Unhandled exception: PC 6c000000, code 1, evt 0180

CAUSA
    `get_itab()` (runtime/interface_dreamcast.c) arma cada tabla de metodos con `runtime_malloc`, que es el
    monton MOVIBLE de Go, y la guarda en `itab_cache`, un arreglo ESTATICO que vive en `.bss`. El GC NO
    escanea `.bss`: el comentario de gc_copy.c lo dice explicito ("gc_scan_globals() was removed - redundant
    with registerGCRoots()"). Cuando el GC copia, actualiza las raices que conoce (pila, globales de gccgo,
    gc_add_root) pero NO el cache: el cache sigue apuntando a la mitad VIEJA del semiespacio, que en la
    misma recoleccion pasa a ser el espacio de ASIGNACION y se pisa. El siguiente acierto de cache devuelve
    un itab con la tabla de metodos hecha basura, y el `jsr` del metodo salta a lo que haya.

    No se veia antes porque el camino viejo (el asset entero en memoria) hacia MENOS recolecciones y menos
    conversiones de interfaz: el cache se llenaba y se usaba antes de que el GC lo invalidara. El camino por
    trozos corre muchas mas recolecciones (una cada 4 KB leidos de la tabla), y ahi el defecto sale siempre.

ARREGLO
    Los itabs NO tienen nada que actualizar cuando el GC copia: sus palabras son descriptores de tipo
    (estan en `.rodata`) y punteros a funcion (estan en `.text`), nunca punteros al monton. Lo unico que
    hace falta es que el itab NO se mueva: se asigna con `gc_external_alloc` (malloc de KOS, fuera del
    monton de Go) en vez de `runtime_malloc`. Con eso el cache puede seguir siendo un arreglo estatico y
    ningun itab cambia de direccion.

    Es el mismo criterio que el runtime ya usa para lo que no puede moverse (texturas, buffers de DMA: ver
    la cabecera de gc_semispace.h).

USO
    python3 scripts/console/fix-libgodc-itab-245.py [ruta_libgodc] [--estado|--quitar]
    Despues: make -C <libgodc> libgodc.a

ORDEN EN LA CADENA
    Va DESPUES de fix-libgodc-iface.py (ese resetea el archivo con `git checkout`). En
    fix-libgodc-cadena.sh es el paso 4.

QUIEN LO JUZGA
    scripts/console/checar-trozos-245.sh: el arranque pasa `LvlPrest.txt` y sigue con la tabla siguiente,
    y el mutante del instrumento (sin este arreglo) se cae adentro de la lectura.
"""
import sys

ANCLA = """    Itab *itab = (Itab *)runtime_malloc(total_size);
    if (!itab)
        return NULL;
"""

NUEVO = """    // DC-002.45: FUERA DEL MONTON MOVIBLE. Este itab lo guarda `itab_cache` (estatico, en `.bss`) y el GC
    // NO escanea `.bss`: si el itab vive en el monton de Go, la recoleccion lo copia y deja el cache
    // apuntando a la mitad vieja del semiespacio, que se reusa (medido: `itab-cache-VIEJO` justo antes del
    // `Unhandled exception` con el PC dentro del semiespacio). Un itab no tiene NADA que actualizar cuando
    // el GC copia -- sus palabras son descriptores de tipo (`.rodata`) y punteros a funcion (`.text`) -- asi
    // que lo unico que hace falta es que no se mueva: se pide al monton de C (`gc_external_alloc`), el mismo
    // camino que el runtime usa para lo que no puede moverse (ver la cabecera de gc_semispace.h).
    Itab *itab = (Itab *)gc_external_alloc(total_size);
    if (!itab)
        return NULL;
"""


def main():
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    modo = [a for a in sys.argv[1:] if a.startswith("--")]

    lib = (args[0] if args else "/home/dgalli/dreamcast/libgodc").rstrip("/")
    ruta = lib + "/runtime/interface_dreamcast.c"

    try:
        actual = open(ruta).read()
    except OSError as e:
        print("no pude leer", ruta, e)
        return 2

    aplicado = NUEVO in actual

    if "--estado" in modo:
        print("estado:", "arreglado (el itab fuera del monton movible)" if aplicado else "pristino (el itab en el monton de Go)")
        return 0 if aplicado else 1

    if "--quitar" in modo:
        if not aplicado:
            print("ya estaba pristino")
            return 0
        open(ruta, "w").write(actual.replace(NUEVO, ANCLA))
        print("-> estado pristino")
        return 0

    if aplicado:
        print("ya estaba arreglado:", ruta)
        return 0

    if ANCLA not in actual:
        print("no encontre el ancla de get_itab en", ruta)
        return 1

    open(ruta, "w").write(actual.replace(ANCLA, NUEVO, 1))
    print(ruta, "-> el itab se pide a gc_external_alloc (fuera del monton movible)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
