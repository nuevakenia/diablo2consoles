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
      --sin-build  solo deja el header (no rehace libgodc.a: el .a seguiria con el valor VIEJO)

Despues: reenlazar los POC (build-motor-arranque.sh / build-dc6.sh).
"""
import hashlib
import os
import pathlib
import re
import shutil
import subprocess
import sys

FABRICA = 512
RUTA = 'runtime/godc_config.h'
PATRON = re.compile(r'(#define\s+GC_SEMISPACE_SIZE_KB\s+)(\d+)')


def sha(ruta) -> str:
    return hashlib.sha256(pathlib.Path(ruta).read_bytes()).hexdigest()[:16]


def main() -> int:
    argv = sys.argv[1:]
    args = [a for a in argv if not a.startswith('--')]
    modo = [a for a in argv if a.startswith('--')]

    # DC-002.47: `--kb N` se lee de argv, NO de la lista de `--opciones`. La version anterior buscaba el
    # numero adentro de `modo` (que solo tiene las palabras que empiezan con `--`), asi que
    # `--kb 1024` moria siempre con "--kb necesita un numero" y el numero nunca se aplicaba: la unica
    # forma de mover el semiespacio era --quitar (volver a fabrica). Medido en esta tarjeta.
    kb_pedido = None
    if '--kb' in argv:
        i = argv.index('--kb')
        try:
            kb_pedido = int(argv[i + 1])
        except (IndexError, ValueError):
            print('--kb necesita un numero (KB por semiespacio)')
            return 2
        # el numero no es una ruta: no puede quedar como args[0]
        if argv[i + 1] in args:
            args.remove(argv[i + 1])

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
    elif kb_pedido is not None:
        nuevo = kb_pedido
    else:
        print(__doc__)
        return 2

    if kb == nuevo:
        print('ya estaba en %d KB' % nuevo)
        cambio = False
    else:
        escrito = PATRON.sub(lambda mm: mm.group(1) + str(nuevo), actual, count=1)
        open(ruta, 'w').write(escrito)
        kb = nuevo
        cambio = True
        print('%s -> GC_SEMISPACE_SIZE_KB = %d' % (ruta, nuevo))
    verificado = PATRON.search(open(ruta).read())
    if not verificado or int(verificado.group(2)) != nuevo:
        print('el archivo NO quedo en %d KB' % nuevo)
        return 1
    print('verificado: cada semiespacio %d B, el heap de Go %d B' % (nuevo * 1024, 2 * nuevo * 1024))

    # DC-002.47: SE REHACE ACA. El Makefile del vendor es `%.o: %.c` SIN dependencias de cabeceras, asi que
    # tocar `godc_config.h` NO recompila NADA: `make libgodc.a` contesta "esta actualizado" y `libgodc.a`
    # sigue con el valor VIEJO del semiespacio (medido en esta tarjeta: 1024 en el header y 512 adentro del
    # .a, sin un solo error). Se borran SOLO los objetos del runtime (no `make clean`, que ademas limpia KOS y
    # se lleva `kos.gox`, que los paquetes de Go importan) y se hace `make all`.
    if '--sin-build' in modo:
        print('--sin-build: no rehago libgodc.a (el binario va a seguir con el valor viejo si cambio)')
        return 0

    # DC-002.47: si NO hubo cambio y el .a esta, NO se rehace. Rehacerlo cuesta ~1 minuto y deja el .a con
    # otro sha256 (el `ar` graba la fecha), lo que rompe cualquier comparacion "es el mismo binario".
    # --rehacer lo fuerza.
    if not cambio and os.path.exists(lib + '/libgodc.a') and '--rehacer' not in modo:
        print('== libgodc.a ya esta en %d KB: no lo rehago (--rehacer para forzarlo)' % nuevo)
        return 0

    print('== rehaciendo libgodc.a (los .o del runtime, porque el Makefile no mira las cabeceras)')
    # DC-002.47: EL TOOLCHAIN PRIMERO. Este script lo puede llamar cualquiera (un script de medicion, un
    # agente, a mano) y `make` necesita `sh-elf-gcc` en el PATH. Medido en esta tarjeta: sin el PATH, el `rm`
    # de abajo ya borro los .o y el .a, `make` muere con «sh-elf-gcc: No existe el fichero o el directorio» y
    # deja a libgodc SIN biblioteca (el arranque no enlaza y el sintoma no dice nada del PATH). Se arregla de
    # dos maneras: se agrega el bin del toolchain al PATH, y si igual no aparece, se aborta ANTES de borrar.
    dc_root = os.environ.get('DC_ROOT', os.path.expanduser('~/dreamcast'))
    sh_elf = os.path.join(dc_root, 'sh-elf', 'bin')
    if os.path.isdir(sh_elf) and sh_elf not in os.environ.get('PATH', '').split(':'):
        os.environ['PATH'] = sh_elf + ':' + os.environ.get('PATH', '')
    if not shutil.which('sh-elf-gcc'):
        print('no encuentro sh-elf-gcc en el PATH (%s): no borro nada' % os.environ.get('PATH', ''))
        return 1
    subprocess.run(['bash', '-c', 'rm -f %s/runtime/*.o %s/*.a' % (lib, lib)], check=True)
    r = subprocess.run(['make', '-C', lib, '-j8', 'all'], capture_output=True, text=True)
    print('   make: ' + ((r.stdout.strip().splitlines() or ['(sin salida)'])[-1]))
    if r.returncode != 0:
        print(r.stdout[-2000:] + r.stderr[-2000:])
        print('make fallo')
        return 1
    peso = os.path.getsize(lib + '/libgodc.a')
    print('   %s/libgodc.a: %d bytes, sha256 %s' % (lib, peso, sha(pathlib.Path(lib + '/libgodc.a'))))
    return 0


if __name__ == '__main__':
    sys.exit(main())
