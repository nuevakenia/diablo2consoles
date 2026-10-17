#!/usr/bin/env python3
"""DC-002.14: DE DONDE SALE EL PESO DEL BINARIO DEL MOTOR (medido, por seccion y por paquete).

Por que existe: DC-002.13 midio que el motor entero enlazado NO entra en los 16 MB de la consola (faltan
1.486.848 bytes de RAM para el GC). Para recortar hay que saber DONDE esta el peso, y eso NO se mide con el
tamano de los .gox (traen debug y enganan): se mide en el ELF enlazado, con el tamano de cada simbolo,
atribuido a la seccion y al paquete que lo define.

Uso:  scripts/console/pesar-motor.py <elf>                  (peso por PAQUETE)
      scripts/console/pesar-motor.py <elf> --secciones       (peso por SECCION x paquete)

Por que las dos vistas: el recorte se hace por SECCION (gc-sections tira .text/.data/.rodata que nadie
referencia; -fno-asynchronous-unwind-tables tira .eh_frame entero), pero el que decide que recortar es el
PAQUETE (una seccion grande puede ser un paquete entero que el arranque no usa).

EL INSTRUMENTO SE CONTROLA A SI MISMO, y esto costo una medicion falsa (medido, 2026-10-17): en el ELF
enlazado SIN -ffunction-sections los tamanos de .text se SOLAPAN -- 21.242 de 21.266 simbolos arrancan
dentro del anterior, la suma da 14.215.564 bytes en una seccion de 4.962.328 (2,86x) mientras la UNION de
los intervalos da 4.971.334 (100,2%). O sea: los tamanos son de fiar pero NO son exclusivos, y sumarlos por
paquete reparte bytes que pertenecen a otro. La tabla que salia de ahi (la de DC-002.8/.13/.14, con d2game
1098252) NO es una atribucion: es una suma de intervalos solapados. Por eso, si en alguna seccion la suma
supera el tamano de la seccion, el tool NO imprime ninguna tabla: dice el solape y sale con exit 2. El
corpus que lo exige es scripts/console/checar-recorte-214.sh.

Los nombres de gccgo vienen mangleados: `_github_0com_1OpenDiablo2_1OpenDiablo2_1d2core_1d2records.Tabla`
es `github.com/OpenDiablo2/OpenDiablo2/d2core/d2records` + el simbolo (`_0` -> `.`, `_1` -> `/`).
"""
import subprocess
import sys
from collections import defaultdict

ARGV = [a for a in sys.argv[1:] if not a.startswith("--")]
POR_SECCION = "--secciones" in sys.argv
ELF = ARGV[0] if ARGV else "/tmp/dcbuild/motor-arranque/prog-completo.elf"
NM = "sh-elf-nm"
READELF = "sh-elf-readelf"
REPO = "github.com/OpenDiablo2/OpenDiablo2/"


def desmanglear(simbolo: str) -> str:
    if not simbolo.startswith("_"):
        return simbolo
    s = simbolo[1:]
    return s.replace("_1", "/").replace("_0", ".")


def paquete(simbolo: str) -> str:
    """Devuelve la etiqueta a la que se le carga el peso de este simbolo."""
    s = desmanglear(simbolo)

    if s.startswith(REPO):
        resto = s[len(REPO):]
        if "/" in resto:
            return REPO + resto.rsplit("/", 1)[0]
        return REPO + resto.split(".")[0]

    # los blobs generados por el compilador (constantes, descriptores de tipo, gcbits): OJO, esta regla va
    # ANTES de la de la stdlib, porque `go..C3111` tambien tiene un punto y si no cae en "go"
    if s.startswith("go.") or s.startswith("type..") or s.startswith("gcbits") or s.startswith("pimt"):
        return "datos generados por el compilador (tipos, constantes, gcbits)"

    # la stdlib minima y los terceros: `fmt.Fprintf`, `bufio.Scan`, `ebiten.xyz`
    if "." in s:
        cabeza = s.split(".")[0]
        if cabeza and cabeza[0].islower() and not cabeza.startswith("_") and "/" not in cabeza:
            return cabeza

    if s.startswith("runtime") or s.startswith("_runtime"):
        return "runtime de Go (libgodc)"

    return "C (libc / KOS / libgodc)"


def tamano_de_secciones():
    """Tamano REAL de cada seccion (lo que ocupa en los 16 MB son las ALLOC)."""
    salida = subprocess.run([READELF, "-SW", ELF], capture_output=True, text=True).stdout
    tam = {}
    for linea in salida.splitlines():
        # formato: [ 4] .rodata  PROGBITS  8c4cb87c 4bb8fc 59fa70 00  A  0 0 4
        p = linea.replace("[", " ").replace("]", " ").split()
        if len(p) < 7 or not p[0].isdigit():
            continue
        try:
            tam[p[1]] = int(p[5], 16)
        except ValueError:
            continue
    return tam


def leer_simbolos():
    """Lee la tabla de simbolos: por seccion, (direccion, tamano, paquete, nombre).

    Se usa `readelf -sW` y NO `nm -S`: nm no dice en que SECCION vive el simbolo, y esa es justo la
    pregunta (de que seccion sale el peso). `-sW` trae la columna Ndx (el indice de la seccion) y el
    tamano, asi que el mapa sale del propio ELF y no de suponer por el nombre del simbolo.
    """
    salida = subprocess.run([READELF, "-sW", ELF], capture_output=True, text=True).stdout
    cab = subprocess.run([READELF, "-SW", ELF], capture_output=True, text=True).stdout
    indices = {}
    for linea in cab.splitlines():
        p = linea.replace("[", " ").replace("]", " ").split()
        if len(p) >= 7 and p[0].isdigit():
            indices[p[0]] = p[1]

    crudos = defaultdict(list)
    for linea in salida.splitlines():
        if ":" not in linea:
            continue
        p = linea.split(maxsplit=7)
        if len(p) < 8:
            continue
        ndx, nombre = p[6], p[7].strip()
        if not ndx.isdigit() or not nombre:
            continue
        try:
            direccion = int(p[1], 16)
            tam = int(p[2], 16)
        except ValueError:
            continue
        if tam == 0:
            continue
        crudos[indices.get(ndx, f"??{ndx}")].append((direccion, tam, paquete(nombre), nombre))
    return crudos


def solapes(crudos, tam_real):
    """Secciones donde la suma de tamanos pasa el tamano real: los tamanos NO son exclusivos."""
    out = []
    for seccion, filas in crudos.items():
        real = tam_real.get(seccion, 0)
        suma = sum(t for _, t, _, _ in filas)
        if real and suma > real:
            out.append((seccion, suma, real))
    return sorted(out, key=lambda x: -x[1])


def aviso_solape(mentirosas):
    """Explica el solape y devuelve el exit code del tool (2 = no hay atribucion que dar)."""
    print("AVISO: los tamanos de estos simbolos SE SOLAPAN; sumarlos por paquete NO atribuye bytes.")
    print("       Medido en el ELF enlazado SIN -ffunction-sections: .text suma 2,86x su seccion.")
    for seccion, suma, real in mentirosas:
        print(f"  {seccion}: suma={suma} > seccion={real}  ({suma / real:.2f}x)")
    print()
    print("       La atribucion exacta necesita el ELF PARTIDO por funcion:")
    print("         bash scripts/console/recorte-214.sh          (arma los .gox con")
    print("           -ffunction-sections -fdata-sections y los enlaza con -Wl,--gc-sections)")
    print("         python3 scripts/console/pesar-motor.py /tmp/dcbuild/motor-214/prog.elf --secciones")
    return 2


def por_seccion(crudos, tam_real):
    peso = defaultdict(lambda: defaultdict(int))
    nsimb = defaultdict(lambda: defaultdict(int))
    for seccion, filas in crudos.items():
        for _, tam, etiqueta, _ in filas:
            peso[seccion][etiqueta] += tam
            nsimb[seccion][etiqueta] += 1

    for seccion, paquetes in sorted(peso.items(), key=lambda x: -sum(x[1].values())):
        total = sum(paquetes.values())
        real = tam_real.get(seccion, 0)
        print(f"== {seccion}: {total} bytes en simbolos  (la seccion mide {real} bytes;"
              f" {100 * total / real:.1f}% de la seccion)" if real else f"== {seccion}: {total} bytes")
        print(f"{'bytes':>10}  {'%':>5}  {'simb':>6}  paquete")
        for etiqueta, b in sorted(paquetes.items(), key=lambda x: -x[1])[:14]:
            print(f"{b:>10}  {100 * b / total:5.1f}  {nsimb[seccion][etiqueta]:>6}  {etiqueta}")
        print()


def por_paquete(crudos):
    peso = defaultdict(int)
    nsimb = defaultdict(int)
    total = 0
    for filas in crudos.values():
        for _, tam, etiqueta, _ in filas:
            peso[etiqueta] += tam
            nsimb[etiqueta] += 1
            total += tam
    print(f"peso atribuido a simbolos con tamano: {total} bytes ({total / 1048576:.2f} MB)")
    print()
    print(f"{'bytes':>10}  {'%':>5}  {'simb':>6}  paquete")
    for etiqueta, b in sorted(peso.items(), key=lambda x: -x[1]):
        print(f"{b:>10}  {100 * b / total:5.1f}  {nsimb[etiqueta]:>6}  {etiqueta}")


def main():
    crudos = leer_simbolos()
    tam_real = tamano_de_secciones()
    cargadas = sum(t for s, t in tam_real.items()
                   if s not in (".debug_info", ".go_export", ".symtab", ".strtab", ".comment"))
    print(f"ELF: {ELF}   ({cargadas} bytes de secciones en el archivo)")

    mentirosas = solapes(crudos, tam_real)
    if mentirosas:
        sys.exit(aviso_solape(mentirosas))

    if POR_SECCION:
        print("peso por SECCION (tamano real de la seccion | suma de simbolos con tamano)")
        print()
        por_seccion(crudos, tam_real)
    else:
        por_paquete(crudos)


if __name__ == "__main__":
    main()
