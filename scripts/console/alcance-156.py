#!/usr/bin/env python3
"""DC-002.156: QUE PAQUETES QUEDAN EN EL ENLACE, por reachabilidad de imports.

POR QUE EXISTE. El enlace del Gate 0 pasa de 145 paquetes a lo que el pueblo usa. Para saber QUE se
cae (y cuanto pesa) antes de compilar nada, se arma el grafo de imports del repo -- con las mismas
palancas por NOMBRE del build (`palancas-del-build.sh`) -- y se calcula el alcance desde las raices.

Uso:
    python3 scripts/console/alcance-156.py <raiz...> [--fuera pkg ...]

`<raiz>` es una ruta de importacion completa o una carpeta del repo (d2app, d2game/d2gamescreen).
Con `--fuera` se simulan los paquetes que el build de la consola NO compila (los stubs).
Las fuentes de un paquete son las mismas que elige el build (plataforma/arquitectura/marca/lookups);
con `--solo-archivos <pkg>=<archivo,archivo>` se simula una lista blanca (las pantallas del Gate 0).
"""
import os
import re
import sys
from collections import defaultdict

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
PREFIJO = "github.com/OpenDiablo2/OpenDiablo2"

PALANCA_PLATAFORMA = r"_(?:darwin|windows|js|wasm|plan9|freebsd|netbsd|openbsd|linux|android|ios|solaris|aix)\.go$"
PALANCA_ARQ = r"_(?:386|amd64|arm|arm64|ppc64|ppc64le|mips|mipsle|mips64|mips64le|riscv64|s390x|loong64|wasm)\.go$"
PALANCA_MARCA = r"/marca_carga_host\.go$"
PALANCA_LOOKUPS = r"/(?:object_lookup_record_data|object_lookups_host)\.go$"
EXCLUIR = re.compile("|".join([PALANCA_PLATAFORMA, PALANCA_ARQ, PALANCA_MARCA, PALANCA_LOOKUPS]))

IMPORT_RE = re.compile(r'^\s*(?:[\w.]+\s+)?"([^"]+)"', re.M)


def fuentes(directorio, solo=None):
    archivos = []
    for nombre in sorted(os.listdir(directorio)):
        if not nombre.endswith(".go") or nombre.endswith("_test.go"):
            continue
        if EXCLUIR.search("/" + nombre):
            continue
        if solo is not None and nombre not in solo:
            continue
        archivos.append(os.path.join(directorio, nombre))
    return archivos


def imports_de(archivo):
    with open(archivo, encoding="utf-8", errors="replace") as f:
        texto = f.read()
    # solo el/los bloques import; buscar el cuerpo entre 'import (' y ')' y las lineas 'import "x"'
    salida = []
    for bloque in re.finditer(r"import\s*\(([^)]*)\)", texto, re.S):
        salida += IMPORT_RE.findall(bloque.group(1))
    salida += re.findall(r'^import\s+"([^"]+)"', texto, re.M)
    return salida


def paquetes_del_repo():
    """ruta de importacion -> directorio, para cada paquete Go del motor."""
    mapa = {}
    for raiz in ("d2common", "d2core", "d2game", "d2app", "d2networking", "d2script", "d2thread"):
        base = os.path.join(REPO, raiz)
        for actual, _, archivos in os.walk(base):
            if any(a.endswith(".go") and not a.endswith("_test.go") for a in archivos):
                rel = os.path.relpath(actual, REPO)
                mapa["%s/%s" % (PREFIJO, rel)] = actual
    return mapa


def main():
    args = sys.argv[1:]
    fuera, solo_archivos, raices = set(), {}, []
    lista_salida, gox_dir = None, "/tmp/dc155/gox"
    i = 0
    while i < len(args):
        if args[i] == "--fuera":
            i += 1
            while i < len(args) and not args[i].startswith("--"):
                fuera.add(args[i])
                i += 1
        elif args[i] == "--solo-archivos":
            i += 1
            while i < len(args) and not args[i].startswith("--"):
                pkg, lista = args[i].split("=", 1)
                solo_archivos[pkg] = set(x for x in lista.split(",") if x)
                i += 1
        elif args[i] == "--lista":
            i += 1
            lista_salida = args[i]
            i += 1
        elif args[i] == "--gox":
            i += 1
            gox_dir = args[i]
            i += 1
        else:
            raices.append(args[i])
            i += 1

    repo = paquetes_del_repo()

    def normaliza(raiz):
        if raiz.startswith(PREFIJO):
            return raiz
        return "%s/%s" % (PREFIJO, raiz.rstrip("/"))

    fuera = {normaliza(f) for f in fuera}
    solo_archivos = {normaliza(k): v for k, v in solo_archivos.items()}

    pendientes = [normaliza(r) for r in raices]
    alcanzados = set()
    podados = set()
    while pendientes:
        pkg = pendientes.pop()
        if pkg in alcanzados or pkg in podados:
            continue
        if pkg in fuera:
            # el build de la consola NO compila este paquete: entra un stub declarado en su lugar, asi que
            # sus imports no se siguen (lo que el stub necesita lo declara el stub, no el paquete real)
            podados.add(pkg)
            continue
        alcanzados.add(pkg)
        directorio = repo.get(pkg)
        if directorio is None:
            continue  # stdlib o tercero: no entra en esta cuenta
        for archivo in fuentes(directorio, solo_archivos.get(pkg)):
            for imp in imports_de(archivo):
                if imp.startswith(PREFIJO):
                    pendientes.append(imp)

    pesos = {}
    try:
        with open(os.environ.get("PESOS", "/tmp/dc156-pesos.tsv"), encoding="utf-8") as f:
            for linea in f:
                try:
                    pkg, valor = linea.rstrip("\n").split("\t")
                except ValueError:
                    continue
                pesos[pkg] = int(valor)
    except OSError:
        print("AVISO: no encuentro los pesos por paquete (corre antes scripts/console/tasar-paquetes-156.sh)",
              file=sys.stderr)

    def tamano(pkg):
        # OJO: el tamano del ARCHIVO .gox no es lo que entra a la imagen (trae el .go_export, que no se
        # enlaza). Lo que se cuenta es text+data, que lo mide tasar-paquetes-156.sh.
        return pesos.get(pkg, 0)

    todos = set(repo)
    stubs = sorted(podados & todos, key=tamano, reverse=True)
    caen = sorted(todos - alcanzados - podados, key=tamano, reverse=True)

    # --lista: la lista de enlace del build (una ruta de .gox por linea, en el orden en que se enlazan)
    if lista_salida:
        with open(lista_salida, "w", encoding="utf-8") as f:
            for pkg in sorted(alcanzados & todos):
                f.write("%s/%s.gox\n" % (gox_dir.rstrip("/"), pkg))
        print("== lista de enlace escrita en %s (%d paquetes)" % (lista_salida, len(alcanzados & todos)))

    print("== ALCANCE DEL ENLACE (raices: %s)" % ", ".join(sorted(raices)))
    print("   quedan %d paquetes, caen %d, se reemplazan por un stub %d" %
          (len(alcanzados & todos), len(caen), len(stubs)))
    print()
    print("   los que CAEN de la imagen (bytes de text+data del .gox):")
    total = 0
    for pkg in caen:
        print("   %9d  %s" % (tamano(pkg), pkg.replace(PREFIJO + "/", "")))
        total += tamano(pkg)
    print("   %9d  TOTAL que cae" % total)
    print()
    print("   los que se reemplazan por un STUB (ahorra su text+data, cuesta el stub):")
    total_stub = 0
    for pkg in stubs:
        print("   %9d  %s" % (tamano(pkg), pkg.replace(PREFIJO + "/", "")))
        total_stub += tamano(pkg)
    print("   %9d  TOTAL de los stubs" % total_stub)
    print()
    print("   AHORRO en la imagen: %d B (caen %d + stubs %d)" % (total + total_stub, total, total_stub))
    print()
    print("   los que QUEDAN:")
    total_queda = 0
    for pkg in sorted(alcanzados & todos, key=tamano, reverse=True):
        print("   %9d  %s" % (tamano(pkg), pkg.replace(PREFIJO + "/", "")))
        total_queda += tamano(pkg)
    print("   %9d  TOTAL que queda" % total_queda)


if __name__ == "__main__":
    main()
