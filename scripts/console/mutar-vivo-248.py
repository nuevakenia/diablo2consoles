#!/usr/bin/env python3
"""DC-002.48: el MUTANTE de las palancas del dato vivo, y su control.

LAS DOS PALANCAS DE ESTA TARJETA
  1. `ObjectDetailRecord` deja de guardar los 36 campos que el motor NUNCA lee (medidos: ni una lectura de
     `.Campo` en los 142 paquetes, fuera de su loader y del archivo del struct). Medido en el host: el record
     pasa de 768 a 632 B y la tabla de 654.424 a 408.976 B vivos.
  2. `d2txt.DataDictionary.String` COPIA el valor fuera del buffer del csv (los campos del csv son subslices
     del buffer del lector, asi que una tabla que guarda una cadena se lleva vivo el TEXTO del archivo).

EL MUTANTE
  Vuelve los tres archivos al estado de `HEAD` en un arbol de trabajo aparte (`--pristino <dir>`). Ese arbol,
  pasado por el MISMO corpus (`checar-trozos-245.sh`), tiene que devolver el dato vivo a 1.037.096 B y poner
  ROJO el control `checar-vivo-248.sh` (techo 950.000 B). Si el mutante quedara verde, el control no mide
  nada y el hueco seria del control, no de las palancas.

Uso:
  mutar-vivo-248.py --estado                 (que palancas estan puestas en el checkout)
  mutar-vivo-248.py --pristino <dir>         (copia los 3 archivos de HEAD adentro de <dir>)

OJO CON EL ARBOL DESDE DONDE SE CORRE (medido el 2026-10-19): los tres archivos se sacan con `git show HEAD:`
del arbol DONDE VIVE ESTE SCRIPT, asi que hay que correrlo DESDE EL CHECKOUT apuntando a la copia:
  rsync -a --exclude .git <checkout>/ <copia>/
  python3 <checkout>/scripts/console/mutar-vivo-248.py --pristino <copia>
Corrido desde la copia (que no tiene .git, porque el rsync lo excluye) el `git show` falla a la mitad y deja
el primero de los tres archivos escrito y los otros dos como estaban: un "pristino" a medias.
"""
import os
import subprocess
import sys

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
ARCHIVOS = [
    "d2core/d2records/object_details_record.go",
    "d2core/d2records/object_details_loader.go",
    "d2common/d2fileformats/d2txt/data_dictionary.go",
]


def git_show(archivo):
    return subprocess.run(["git", "-C", REPO, "show", "HEAD:" + archivo],
                          capture_output=True, text=True, check=True).stdout


def estado_de(raiz):
    rec = open(os.path.join(raiz, ARCHIVOS[0])).read()
    txt = open(os.path.join(raiz, ARCHIVOS[2])).read()

    podado = "BlocksLight" not in rec
    copia = "return copiar(d.record[i])" in txt

    if podado and copia:
        return "sano (campos muertos fuera: si; cadenas copiadas: si)"

    return "pristino (campos muertos fuera: %s; cadenas copiadas: %s)" % ("si" if podado else "NO",
                                                                        "si" if copia else "NO")


def main():
    if "--estado" in sys.argv:
        print("estado: " + estado_de(REPO))
        return 0

    if "--pristino" in sys.argv:
        destino = sys.argv[sys.argv.index("--pristino") + 1]
        for archivo in ARCHIVOS:
            ruta = os.path.join(destino, archivo)
            os.makedirs(os.path.dirname(ruta), exist_ok=True)
            open(ruta, "w").write(git_show(archivo))
        print("pristino: 3 archivos de HEAD escritos en " + destino)
        print("estado: " + estado_de(destino))
        return 0

    print(__doc__)
    return 2


if __name__ == "__main__":
    sys.exit(main())
