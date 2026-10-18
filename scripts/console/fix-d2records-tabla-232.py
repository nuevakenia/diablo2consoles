#!/usr/bin/env python3
"""DC-002.32: LA TABLA DE d2records FUERA DEL BINARIO (y de vuelta), con script idempotente.

QUE HACE: el peso mas grande del binario del motor es UN SOLO simbolo de `.data`, medido con `nm` sobre el
ELF enlazado: `_go..C3111` = 1.672.892 bytes, la tabla `objectLookups` de
`d2core/d2records/object_lookup_record_data.go` (7.902 lineas de fuente, 1.942.429 bytes: un volcado de
datos, no codigo).

Este script reemplaza ese archivo por una declaracion VACIA del mismo tipo:

    var objectLookups = []ObjectLookupRecord{}

con lo que la tabla (y sus cadenas) dejan de estar en la imagen, y revierte el cambio para volver a medir con
la tabla adentro. Es el instrumento de la palanca, no el arreglo: la tabla vacia significa que el motor
arranca SIN los 7.891 registros de `objectLookups` (el motor solo los usa para poblar `Object.Lookup`, que a
su vez se usa al generar un nivel), asi que lo que se mide con esto es el PESO y el punto donde muere el
arranque, no un motor jugable. La decision de donde entra la tabla (formato y momento del arranque) queda
medida en la evidencia de la tarjeta, con el numero que la decide: los bytes no desaparecen, se mudan.

IDEMPOTENTE Y VERIFICABLE: deja el arbol pristino con `--dentro` (restaura el original guardado y, si no
esta, lo saca del propio git) y reporta sha256 y tamano en cada paso.

Uso:
  python3 scripts/console/fix-d2records-tabla-232.py --ver       (dice en que estado esta)
  python3 scripts/console/fix-d2records-tabla-232.py --fuera     (la tabla AFUERA del binario)
  python3 scripts/console/fix-d2records-tabla-232.py --dentro    (la tabla ADENTRO: el mutante / la vuelta)
  --copia <archivo>   donde guardar el original (por defecto /tmp/dcbuild/dc232-tabla/<nombre>.original)
"""
import hashlib
import pathlib
import shutil
import subprocess
import sys

MARCA = "DC-002.32: LA TABLA FUERA DEL BINARIO"
REPO = pathlib.Path(__file__).resolve().parents[2]
ARCH = REPO / "d2core/d2records/object_lookup_record_data.go"
COPIA = pathlib.Path("/tmp/dcbuild/dc232-tabla/object_lookup_record_data.go.original")

STUB = f"""package d2records

// {MARCA}
//
// LO QUE ESTE ARCHIVO DECLARA, medido en el ELF enlazado: el volcado `objectLookups` de
// object_lookup_record_data.go pesa 1.672.892 bytes de `.data` (es UN solo simbolo del compilador,
// `_go..C3111`; 7.891 elementos de 212 bytes) y sus cadenas otros cientos de KB de `.rodata` -- de los
// 13.998.980 bytes que el binario del motor se lleva de los 16.777.216 de la consola (83,4%).
//
// Esta es la variante que mide la palanca de la tarjeta DC-002.32: la tabla FUERA del binario. El motor
// arranca igual (el resto esta entero) pero con `Object.Lookup` vacio: el motor solo lo usa al generar un
// nivel, asi que el arranque que se mide sigue siendo el de verdad y lo que cambia es el peso.
//
// Lo escribe y lo revierte scripts/console/fix-d2records-tabla-232.py (idempotente, con sha256).
var objectLookups = []ObjectLookupRecord{{}}
"""


def sha256(p: pathlib.Path) -> str:
    return hashlib.sha256(p.read_bytes()).hexdigest()


def estado() -> str:
    if not ARCH.exists():
        return "falta"
    return "fuera" if MARCA in ARCH.read_text(errors="replace")[:2000] else "dentro"


def ver() -> int:
    e = estado()
    print(f"TABLA-232 estado={e}")
    print(f"TABLA-232 archivo={ARCH}")
    if ARCH.exists():
        print(f"TABLA-232 bytes={ARCH.stat().st_size} sha256={sha256(ARCH)}")
    print(f"TABLA-232 copia={COPIA} existe={COPIA.exists()}")
    return 0


def fuera() -> int:
    if estado() == "fuera":
        print("TABLA-232 ya estaba AFUERA (no hay nada que hacer)")
        return ver()
    COPIA.parent.mkdir(parents=True, exist_ok=True)
    if not COPIA.exists():
        shutil.copy2(ARCH, COPIA)
        print(f"TABLA-232 original guardado en {COPIA} ({COPIA.stat().st_size} bytes)")
    else:
        print(f"TABLA-232 ya habia una copia en {COPIA} ({COPIA.stat().st_size} bytes): se usa esa")
    antes = ARCH.stat().st_size
    ARCH.write_text(STUB)
    print(f"TABLA-232 la tabla quedo AFUERA: {antes} -> {ARCH.stat().st_size} bytes de fuente")
    return ver()


def dentro() -> int:
    if estado() == "dentro":
        print("TABLA-232 la tabla ya estaba ADENTRO (no hay nada que hacer)")
        return ver()
    if COPIA.exists():
        shutil.copy2(COPIA, ARCH)
        print(f"TABLA-232 original restaurado desde {COPIA}")
    else:
        print("TABLA-232 sin copia guardada: se restaura del propio git")
        subprocess.run(["git", "-C", str(REPO), "checkout", "--", str(ARCH)], check=True)
    return ver()


def main() -> int:
    if "--ver" in sys.argv:
        return ver()
    if "--fuera" in sys.argv:
        return fuera()
    if "--dentro" in sys.argv:
        return dentro()
    print(__doc__)
    return 2


if __name__ == "__main__":
    sys.exit(main())
