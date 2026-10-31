#!/usr/bin/env python3
"""DC-002.89 -- ATRIBUIR LA IMAGEN DE LA CONSOLA POR RAMA, con el mapa del enlazador.

QUE PROBLEMA RESUELVE. `nm` solo ve simbolos con nombre y tamano: en este binario los simbolos del
blob que genera gccgo (`_go_..C*`, descriptores de tipo y tablas de itabs) no llevan el paquete en el
nombre, asi que una atribucion por simbolo deja un cajon "sin dueno". El mapa del enlazador
(`-Wl,-Map=...`, lo pide `relink-con-mapa.sh` o `DC_LDMAP=1` en build-dc.sh) si dice, seccion de
entrada por seccion de entrada, DE QUE OBJETO salieron los bytes: con eso TODO se atribuye a su
paquete (un `.gox` por paquete de Go), y lo que no es del repo queda separado (libgodc, KOS, libc...).

QUE CUENTA COMO IMAGEN. Solo las secciones de salida con direccion != 0 (las que se cargan en la
consola): .text .rodata .data .eh_frame .gcc_except_table .ctors .dtors .got .init .fini. Las que van
a direccion 0 no viajan en el 1ST_READ y se listan aparte como "no cargado" (.go_export, .debug_*),
que es justo lo que hace que la suma de los objetos (16 MB) no sea el tamano del binario (6,8 MB).

Uso:
    scripts/console/atribuir-imagen.py <mapa> [--por-objeto] [--csv <archivo>]
"""
import collections
import csv
import re
import sys

HEX = re.compile(r'^0x[0-9a-fA-F]+$')
REPO = 'github.com/OpenDiablo2/OpenDiablo2/'

# las ramas de la imagen: paquetes del repo EMPAREJADOS por su prefijo de import. El orden importa
# (gana el primero que empareja), y lo que no empareja queda como su propio paquete.
RAMAS = [
    ('entidades: mapa/entidad y tipos', ('d2core/d2map/d2mapentity',)),
    ('entidades: heroe y sus records', ('d2core/d2hero',)),
    ('entidades: monstruos', ('d2common/d2monster', 'd2core/d2map/d2mapgen')),
    ('entidades: objetos/items', ('d2core/d2item', 'd2common/d2item', 'd2core/d2inventory')),
    ('entidades: skills/stats/combate', ('d2common/d2skill', 'd2core/d2stats', 'd2common/d2combat',
                                         'd2common/d2statlist', 'd2common/d2equip',
                                         'd2common/d2missile', 'd2core/d2stats',
                                         'd2common/d2fileformats/d2s')),
    ('entidades: quests', ('d2common/d2quest',)),
    ('mapa: compositor y stamp', ('d2core/d2map/d2mapengine', 'd2core/d2map/d2mapstamp',
                                  'd2core/d2map/d2mapgen')),
    ('records/tablas', ('d2core/d2records',)),
    ('assets y formato de archivos', ('d2core/d2asset', 'd2common/d2fileformats', 'd2common/d2loader',
                                      'd2common/d2cache', 'd2common/d2datautils')),
    ('render/UI', ('d2core/d2render', 'd2core/d2ui', 'd2core/d2screen', 'd2core/d2interface',
                   'd2core/d2input')),
    ('resto del repo', ('d2core/', 'd2common/', 'd2game/', 'd2app/')),
]


def rama_de(paquete):
    for nombre, prefijos in RAMAS:
        for p in prefijos:
            if paquete == p or paquete.startswith(p + '/'):
                return nombre
    return 'resto del repo'


def paquete_de(obj):
    """Del path de un objeto del enlace al paquete (Go) o a la biblioteca C (todo lo demas)."""
    if '/gox-enlace/' in obj or obj.endswith('.gox'):
        i = obj.rfind(REPO)
        if i >= 0:
            return ('repo', obj[i + len(REPO):].replace('.gox', ''))
        # un .gox fuera del repo (stdlib de Go, terceros)
        j = obj.rfind('/', 0, obj.rfind('.gox'))
        return ('terceros', obj[max(0, j - 40):].replace('.gox', ''))
    m = re.search(r'/([^/]+\.a)\(([^)]+)\)$', obj)
    if m:
        return ('C', m.group(1) + '(' + m.group(2) + ')')
    m = re.search(r'/([^/]+\.(?:o|obj))$', obj)
    if m:
        base = m.group(1)
        if base in ('main.o',):
            return ('repo', 'main (el POC)')
        if base.startswith('stdlib-'):
            return ('C', base)
        return ('C', base)
    return ('otros', obj)


def parse(mapa):
    lines = open(mapa, errors='replace').read().split('\n')
    ini = 0
    for i, l in enumerate(lines):
        if l.startswith('Linker script and memory map'):
            ini = i + 1
            break

    secs = {}                   # seccion de salida -> direccion
    por_obj = collections.Counter()
    por_obj_sec = collections.defaultdict(collections.Counter)
    pendiente = None
    sec_actual = None

    for l in lines[ini:]:
        if not l.strip():
            continue
        if l[0] not in ' \t':
            m = re.match(r'^(\S+)\s+0x([0-9a-fA-F]+)', l)
            if m and m.group(1) not in ('LOAD', 'START', 'END'):
                sec_actual = m.group(1)
                secs[sec_actual] = int(m.group(2), 16)
            continue
        if l.startswith(' *fill*') or l.startswith(' *(*'):
            continue
        t = l.split()
        if len(t) >= 4 and HEX.match(t[-3]) and HEX.match(t[-2]):     # nombre addr size objeto
            obj, tam = t[-1], int(t[-2], 16)
            pendiente = None
        elif t and not HEX.match(t[0]) and t[0].startswith('.'):       # nombre solo, sigue en la otra linea
            pendiente = ' '.join(t)
            continue
        elif len(t) == 3 and HEX.match(t[0]) and HEX.match(t[1]):      # addr size objeto
            obj, tam = t[2], int(t[1], 16)
            pendiente = None
        else:
            continue                                                   # simbolos
        if HEX.match(obj):
            continue
        por_obj[obj] += tam
        por_obj_sec[obj][sec_actual] += tam

    return secs, por_obj, por_obj_sec


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        return 2
    mapa = sys.argv[1]
    ver_objetos = '--por-objeto' in sys.argv
    csvout = None
    if '--csv' in sys.argv:
        csvout = sys.argv[sys.argv.index('--csv') + 1]

    secs, por_obj, por_obj_sec = parse(mapa)
    cargadas = {s for s, a in secs.items() if a != 0 and not s.startswith('.debug')}
    no_cargadas = {s for s in secs if s not in cargadas}
    # .bss NO viaja en el 1ST_READ (se pone a cero en la consola al arrancar): se cuenta aparte para que
    # el total de imagen se pueda comparar contra el tamano del binario.
    en_binario = {s for s in cargadas if s != '.bss'}

    sec_tot = collections.Counter()
    img = collections.Counter()
    no_img = collections.Counter()
    ramas = collections.Counter()
    paquetes = collections.Counter()
    paq_rama = {}

    for obj, secmap in por_obj_sec.items():
        tipo, nombre = paquete_de(obj)
        for sec, tam in secmap.items():
            sec_tot[sec] += tam
            if sec in cargadas:
                if sec not in en_binario:
                    continue
                img[obj] += tam
                if tipo == 'repo':
                    ramas[rama_de(nombre)] += tam
                    paquetes[nombre] += tam
                    paq_rama[nombre] = rama_de(nombre)
                else:
                    ramas[('C/KOS/libgodc: ' + tipo) if tipo != 'C' else 'C: libc/KOS/libgodc/otros'] += tam
            else:
                no_img[obj] += tam

    total_img = sum(img.values())
    total_no = sum(no_img.values())
    print('mapa: %s' % mapa)
    print('secciones cargadas: %s' % ' '.join(sorted(cargadas)))
    print('secciones NO cargadas (direccion 0): %s' % ' '.join(sorted(no_cargadas)))
    print()
    print('== POR SECCION ==')
    for s, v in sec_tot.most_common():
        marca = 'EN EL 1ST_READ' if s in en_binario else ('bss (no viaja, se pone a cero)' if s == '.bss' else 'fuera')
        print('%11d  %-20s %s' % (v, s, marca))
    print()
    print('imagen atribuida a objetos (sin .bss): %d B (%.3f MB)' % (total_img, total_img / 1048576))
    print('no cargado (fuera del 1ST_READ): %d B (%.2f MB)' % (total_no, total_no / 1048576))
    print()

    print('== POR RAMA ==')
    print('%11s %6s  %s' % ('bytes', '%', 'rama'))
    for k, v in ramas.most_common():
        print('%11d %5.1f%%  %s' % (v, 100.0 * v / total_img, k))
    print()

    print('== POR PAQUETE DEL REPO (top 30) ==')
    print('%11s %6s  %-40s %s' % ('bytes', '%img', 'paquete', 'rama'))
    for k, v in paquetes.most_common(30):
        print('%11d %5.1f%%  %-40s %s' % (v, 100.0 * v / total_img, k, paq_rama[k]))
    print()

    if ver_objetos:
        print('== POR OBJETO (top 40) ==')
        for obj, v in img.most_common(40):
            print('%11d  %s' % (v, obj))
        print()

    if csvout:
        with open(csvout, 'w', newline='') as f:
            w = csv.writer(f)
            w.writerow(['bytes', 'paquete', 'rama'])
            for k, v in paquetes.most_common():
                w.writerow([v, k, paq_rama[k]])
        print('csv: %s' % csvout)

    return 0


if __name__ == '__main__':
    sys.exit(main())
