#!/usr/bin/env python3
"""DC-002.45: EL TAMANO DEL SEMIESPACIO DEL GC -- la constante `GC_SEMISPACE_SIZE_KB` del runtime vendorizado.

POR QUE ES UNA VOLUNTAD Y NO UN PARCHE A MANO: `~/dreamcast/libgodc` esta vendorizado (fuera del repo) y sus
fuentes las tocan SOLO estos scripts; a mano se olvida volver atras y la medicion siguiente mide cualquier
cosa. Este deja el archivo en el estado pedido y lo VERIFICA (si no quedo, sale con error).

QUE CAMBIA: `runtime/godc_config.h`

    #ifndef GC_SEMISPACE_SIZE_KB
    #define GC_SEMISPACE_SIZE_KB 512        <- cada semiespacio (el heap de Go son DOS)
    #endif

Medido (DC-002.45): con 512 KB cada uno el arranque del motor muere con
`fatal error: GC to-space overflow - your live data doesn't fit in the heap` adentro de la lectura de
`LvlPrest.txt`: el espacio de COPIA son 262.144 B y el dato vivo de esa sola tabla mide 244.432 B en el host
(395.992 B las dos primeras tablas del arranque, con un pico de 684.696 B). El techo del monton de C medido en
la consola es 3.215.392 B y los semiespacios ya ocupan 1.048.576 de ahi: el pedido de esta tarjeta (opcion 2)
tiene que entrar en lo que queda.

Uso:  fix-libgodc-semiespacio-245.py [ruta_libgodc] [--estado | --kb N | --quitar]
      --estado   dice el valor de hoy (y si es el de fabrica)
      --kb N     deja GC_SEMISPACE_SIZE_KB en N (el pedido: 1024 = 1 MB por semiespacio)
      --quitar   vuelve al valor de fabrica (512)

Despues:  make -C <libgodc> -j8 libgodc.a   y reenlazar los POC.
"""
import re
import sys

FABRICA = 512
RUTA = 'runtime/godc_config.h'
PATRON = re.compile(r'(#define\s+GC_SEMISPACE_SIZE_KB\s+)(\d+)')


def main() -> int:
    args = [a for a in sys.argv[1:] if not a.startswith('--')]
    modo = [a for a in sys.argv[1:] if a.startswith('--')]

    lib = (args[0] if args else '/home/dgalli/dreamcast/libgodc').rstrip('/')
    ruta = lib + '/' + RUTA

    try:
        actual = open(ruta).read()
    except OSError as e:
        print('no pude leer', ruta, e)
        return 2

    m = PATRON.search(actual)
    if not m:
        print('no encontre el #define de GC_SEMISPACE_SIZE_KB en', ruta)
        return 1

    kb = int(m.group(2))

    if '--estado' in modo:
        print('estado: GC_SEMISPACE_SIZE_KB =', kb)
        print('  de fabrica:', 'SI' if kb == FABRICA else 'NO (se cambio a mano o por este script)')
        print('  cada semiespacio: %d B, el heap de Go: %d B' % (kb * 1024, 2 * kb * 1024))
        return 0

    if '--quitar' in modo:
        nuevo = FABRICA
    elif '--kb' in modo:
        i = modo.index('--kb')
        try:
            nuevo = int(modo[i + 1])
        except (IndexError, ValueError):
            print('--kb necesita un numero (KB por semiespacio)')
            return 2
    else:
        print(__doc__)
        return 2

    if kb == nuevo:
        print('ya estaba en %d KB' % nuevo)
    else:
        escrito = PATRON.sub(lambda mm: mm.group(1) + str(nuevo), actual, count=1)
        open(ruta, 'w').write(escrito)
        kb = nuevo
        print('%s -> GC_SEMISPACE_SIZE_KB = %d' % (ruta, nuevo))

    verificado = PATRON.search(open(ruta).read())
    if not verificado or int(verificado.group(2)) != nuevo:
        print('el archivo NO quedo en %d KB' % nuevo)
        return 1
    print('verificado: cada semiespacio %d B, el heap de Go %d B' % (nuevo * 1024, 2 * nuevo * 1024))
    print('ahora: make -C %s -j8 libgodc.a' % lib)
    return 0


if __name__ == '__main__':
    sys.exit(main())
