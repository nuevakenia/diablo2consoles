#!/usr/bin/env python3
"""DC-002.89 -- atribuir los SIMBOLOS (tambien el blob de gccgo) a su paquete, con el mapa.

POR QUE. `nm -S` da nombre + tamano + direccion de 65.024 simbolos, pero el nombre no siempre trae el
paquete: gccgo junta los datos en simbolos locales como `_go..C3075` (uno solo, 1.672.892 B en este
binario) y los descriptores de tipo en `_type...`. Una atribucion por nombre deja ese cajon sin dueno
(medido: el 61 % de los bytes). El mapa del enlazador no lista los simbolos locales, pero SI lista
cada seccion de entrada con su objeto y su rango de direcciones: cruzando la DIRECCION del simbolo con
ese rango, cada simbolo -local o no- cae en su objeto, y el objeto ya se sabe de que paquete es.

Uso:  scripts/console/atribuir-simbolos.py <mapa> <nm.txt>
      el nm.txt se saca con:  sh-elf-nm -S <prog.elf> | grep '^[0-9a-f]'
"""
import bisect
import collections
import re
import sys

HEX = re.compile(r'^0x[0-9a-fA-F]+$')
REPO = 'github.com/OpenDiablo2/OpenDiablo2/'


def secciones(mapa):
    """[(inicio, fin, objeto)] de todas las secciones de entrada, ordenadas por direccion."""
    salida = []
    lineas = open(mapa, errors='replace').read().split('\n')
    ini = 0
    for i, l in enumerate(lineas):
        if l.startswith('Linker script and memory map'):
            ini = i + 1
            break

    obj_actual = None
    for l in lineas[ini:]:
        if not l.strip() or l[0] not in ' \t':
            continue
        t = l.split()
        if len(t) >= 4 and HEX.match(t[-3]) and HEX.match(t[-2]):
            inicio, tam, obj_actual = int(t[-3], 16), int(t[-2], 16), t[-1]
        elif len(t) == 3 and HEX.match(t[0]) and HEX.match(t[1]):
            inicio, tam = int(t[0], 16), int(t[1], 16)
        else:
            continue
        if obj_actual and inicio:
            salida.append((inicio, inicio + tam, obj_actual))

    salida.sort()
    return salida


def paquete_de(obj):
    if obj.endswith('.gox'):
        i = obj.rfind(REPO)
        if i >= 0:
            return obj[i + len(REPO):].replace('.gox', '')
        return 'stdlib/terceros: ' + obj.rsplit('/', 1)[-1].replace('.gox', '')
    return 'C: ' + obj


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        return 2

    secs = secciones(sys.argv[1])
    inicios = [s[0] for s in secs]

    def objeto_de(direccion):
        i = bisect.bisect_right(inicios, direccion) - 1
        if i >= 0 and secs[i][0] <= direccion < secs[i][1]:
            return secs[i][2]
        return None

    por_paquete = collections.Counter()
    blob = collections.Counter()
    blob_total = 0
    sin = 0
    sin_bytes = 0
    total = 0
    n = 0
    recortados = 0
    recortado_bytes = 0

    for linea in open(sys.argv[2], errors='replace'):
        p = linea.split()
        if len(p) < 4 or not re.match(r'^[0-9a-fA-F]+$', p[0]):
            continue
        try:
            direccion, tam = int(p[0], 16), int(p[1], 16)
        except ValueError:
            continue
        nombre = p[3]
        n += 1
        obj = objeto_de(direccion)
        if not obj:
            sin += 1
            sin_bytes += tam
            total += tam
            continue
        # OJO, MEDIDO: nm MIENTE con el blob de gccgo. `_go..C3075` sale con 1.672.892 B y vive en una
        # seccion de 6.240 B de d2skill.gox: nm da el tamano del GRUPO COMDAT, no el de la seccion que el
        # linker dejo. El tamano se recorta a lo que queda de seccion, y lo recortado se informa.
        fin_sec = secs[bisect.bisect_right(inicios, direccion) - 1][1]
        if direccion + tam > fin_sec:
            recortado_bytes += tam - (fin_sec - direccion)
            recortados += 1
            tam = fin_sec - direccion
        total += tam
        paq = paquete_de(obj)
        por_paquete[paq] += tam
        # EL BLOB: simbolo cuyo NOMBRE no trae el paquete. Es exactamente lo que una atribucion por
        # nombre deja en el cajon "sin dueno"; aca se reparte por objeto.
        if not re.search(r'github_0com_1OpenDiablo2|^_?main\.', nombre):
            blob[paq] += tam
            blob_total += tam

    print('simbolos: %d, bytes: %d (%.2f MB)' % (n, total, total / 1048576))
    print('simbolos recortados a su seccion (nm daba mas): %d, %d B de mas' % (recortados, recortado_bytes))
    print('sin objeto (no caen en ninguna seccion del mapa): %d simbolos, %d B (%.2f %%)' %
          (sin, sin_bytes, 100.0 * sin_bytes / total))
    print('blob (simbolos sin el paquete en el nombre): %d B (%.1f %% de los simbolos)' %
          (blob_total, 100.0 * blob_total / total))
    print()
    print('== EL BLOB, POR PAQUETE (lo que nm NO podia atribuir) ==')
    print('%11s %6s  %s' % ('bytes', '%blob', 'paquete'))
    for k, v in blob.most_common(20):
        print('%11d %5.1f%%  %s' % (v, 100.0 * v / blob_total, k))
    print()
    print('== TODOS LOS SIMBOLOS, POR PAQUETE (top 25) ==')
    print('%11s %6s  %s' % ('bytes', '%', 'paquete'))
    for k, v in por_paquete.most_common(25):
        print('%11d %5.1f%%  %s' % (v, 100.0 * v / total, k))
    return 0


if __name__ == '__main__':
    sys.exit(main())
