#!/usr/bin/env python3
"""DC-002.45 -- INSTRUMENTO: cuanto dato VIVO queda despues de cada recoleccion (en la consola).

POR QUE
    `gc-por-umbral-bytes-usados` mide lo ASIGNADO, no lo VIVO. La tarjeta pide decidir entre "leer por
    trozos" y "subir el semiespacio", y con la lectura por trozos ya puesta el arranque sigue muriendo con
    `GC to-space overflow`. Ese error NO habla del archivo: habla del dato que sobrevive a la recoleccion
    (si no entra en el semiespacio de destino, no hay donde copiarlo).

    `gc_heap.bytes_copied` (gc_copy.c, al final de gc_collect) ES ese numero: lo que quedo vivo despues de
    copiar. Esta marca lo imprime en cada recoleccion, junto con el tope.

QUE HACE
    Una linea despues de `gc_heap.bytes_copied = after_size`:
        ARQ-MARCA gc-vivo v=<bytes_copied>/<space_size> gc=<n>
    No cambia el comportamiento: es una llamada a dbgio con numeros ya calculados.

Uso:  instrumentar-vivo-245.py <libgodc> [--quitar]
"""
import sys

ANCLA = """    size_t after_size = gc_heap.alloc_ptr - gc_heap.space[gc_heap.active_space];
    gc_heap.bytes_copied = after_size;
    gc_heap.bytes_allocated = after_size;
"""

NUEVO = """    size_t after_size = gc_heap.alloc_ptr - gc_heap.space[gc_heap.active_space];
    gc_heap.bytes_copied = after_size;
    gc_heap.bytes_allocated = after_size;

    /* DC-002.45 (INSTRUMENTO): el dato VIVO de la corrida -- lo que sobrevivio a la copia. Es el numero
       que decide si el semiespacio alcanza: `gc-por-umbral-bytes-usados` mide lo asignado, no lo vivo. */
    godc_marca("gc-vivo", (unsigned long)after_size);
"""


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        return 2

    lib = sys.argv[1].rstrip("/")
    ruta = lib + "/runtime/gc_copy.c"
    quitar = "--quitar" in sys.argv

    actual = open(ruta).read()

    if quitar:
        if NUEVO not in actual:
            print("no estaba instrumentado")
            return 0
        open(ruta, "w").write(actual.replace(NUEVO, ANCLA))
        print("instrumento quitado")
        return 0

    if NUEVO in actual:
        print("ya estaba instrumentado")
        return 0

    if ANCLA not in actual:
        print("no encontre el ancla de bytes_copied en", ruta)
        return 1

    open(ruta, "w").write(actual.replace(ANCLA, NUEVO, 1))
    print("instrumentado:", ruta)
    return 0


if __name__ == "__main__":
    sys.exit(main())
