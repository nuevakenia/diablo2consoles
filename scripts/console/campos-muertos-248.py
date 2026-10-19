#!/usr/bin/env python3
"""DC-002.48: sacar del record de Objects los campos que el motor NUNCA lee.

Medido: de los 77 campos de `ObjectDetailRecord`, 36 no tienen UNA sola lectura en los 142 paquetes del
motor (buscando `.Campo` en todo el arbol, fuera del loader y del archivo del struct). Son bytes VIVOS que
el GC copia en cada recoleccion (medido: 573 filas x 96 B = 54.936 B en la consola).

Uso:  campos-muertos-248.py --estado      (cuantos campos tiene hoy el struct)
      campos-muertos-248.py --podar       (los saca del struct y del loader)
"""
import os
import re
import sys

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
STRUCT = REPO + "/d2core/d2records/object_details_record.go"
LOADER = REPO + "/d2core/d2records/object_details_loader.go"

# los 36 campos medidos sin uso (fuera de su loader y de su archivo de struct)
MUERTOS = [
    "NTgtFX", "NTgtFY", "NTgtBX", "NTgtBY", "Orientation", "Trans", "TotalPieces",
    "XSpace", "YSpace", "NameOffset", "ShrineFunction", "ClientFn",
    "BlocksLight", "HasCollision", "SelS",
    "EnvEffect", "BlockVisibility", "PreOperate",
    "SelHD", "SelTR", "SelLG", "SelRA", "SelLA", "SelRH", "SelLH", "SelSH",
    "MonsterOk", "Gore", "Beta", "CollisionSubst", "BlockMissile", "DrawUnder", "OpenWarp",
    "LightRed", "LightGreen", "LightBlue",
]


def quitar_del_struct(texto):
    m = re.search(r"(type ObjectDetailRecord struct \{)(.*?)(\n\})", texto, re.S)
    cuerpo = m.group(2)
    salida = []
    for linea in cuerpo.split("\n"):
        nombre = linea.split(":")[0].split("//")[0].strip().split(" ")[0].split("\t")[0]
        if nombre in MUERTOS:
            continue
        salida.append(linea)
    return texto[:m.start(2)] + "\n".join(salida) + texto[m.end(2):]


def quitar_del_loader(texto):
    salida = []
    lineas = texto.split("\n")
    i = 0
    while i < len(lineas):
        linea = lineas[i]
        campo = re.match(r"^\s*([A-Za-z_][A-Za-z0-9_]*):", linea)
        if campo and campo.group(1) in MUERTOS:
            # un bloque `Campo: [8]bool{ ... },` se lleva hasta la linea del cierre
            if re.search(r"\[8\]\w+\{", linea):
                while i < len(lineas) and not re.match(r"^\s*\},?\s*$", lineas[i]):
                    i += 1
            i += 1
            continue
        salida.append(linea)
        i += 1
    return "\n".join(salida)


def main():
    struct_txt = open(STRUCT).read()
    loader_txt = open(LOADER).read()

    campos = re.findall(r"^\t([A-Za-z_][A-Za-z0-9_]*)\s", struct_txt.split("type ObjectDetailRecord struct {")[1], re.M)

    if "--estado" in sys.argv:
        print("campos del struct hoy: %d; de la lista de muertos quedan %d" %
              (len(campos), len([c for c in campos if c in MUERTOS])))
        return 0

    if "--podar" in sys.argv:
        open(STRUCT, "w").write(quitar_del_struct(struct_txt))
        open(LOADER, "w").write(quitar_del_loader(loader_txt))
        print("podado: se sacaron %d campos del struct y sus asignaciones del loader" % len(MUERTOS))
        return 0

    print(__doc__)
    return 2


if __name__ == "__main__":
    sys.exit(main())
