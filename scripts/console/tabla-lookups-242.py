#!/usr/bin/env python3
"""Los numeros de las dos corridas del corpus de DC-002.42, sacados del log de Flycast (sin interpretacion).

Uso:  tabla-lookups-242.py <log_fix> <log_mutante>
"""
import re
import sys


def analiza(nombre, path):
    with open(path, "rb") as fh:
        ls = fh.read().decode("utf-8", "replace").replace("\r", "").split("\n")

    print("== %s" % nombre)
    print("   vueltas del arranque        : %d" % sum("MOTOR-ARRANQUE arranca" in l for l in ls))

    for etiqueta, filtro in (
        ("ruta de la tabla", "LOOKUPS-242 ruta:"),
        ("la tabla del disco", "LOOKUPS-242 bytes="),
        ("crc e indice", "LOOKUPS-242 crc del archivo="),
        ("monton al leer la tabla", "LOOKUPS-242 monton de C despues de leer la tabla:"),
        ("la tabla del DS1", "LOOKUPS-242 DS1ObjectClass(0,0..4):"),
        ("los overrides del DS1", "LOOKUPS-242 DS1ObjectClass overrides:"),
        ("FindObject(1,2,0)", "LOOKUPS-242 FindObject(1,2,0):"),
        ("FindObject(1,1,0)", "LOOKUPS-242 FindObject(1,1,0):"),
        ("FALLO de la sonda", "LOOKUPS-242 FALLO"),
    ):
        vistos = [l.strip() for l in ls if filtro in l]
        if vistos:
            print("   %-28s: %s" % (etiqueta, vistos[0].split(":", 1)[1].strip() if ":" in vistos[0] else vistos[0]))

    m = [l for l in ls if "monton de C antes del motor" in l]
    if m:
        print("   monton de C antes del motor : %s" % m[0].split(":", 2)[-1].strip())

    semi = [int(x.group(1)) for l in ls for x in [re.search(r"gc-por-umbral-bytes-usados v=(\d+)", l)] if x]
    if semi:
        print("   semiespacio usado maximo    : %d -> libre minimo %d (%d marcas)"
              % (max(semi), 524288 - max(semi), len(semi)))

    fuentes = [l.strip() for l in ls if "Adding source: '/cd/juego" in l]
    print("   fuentes del disco abiertas  : %d (%s)" % (len(fuentes), ", ".join(
        sorted({l.split("/cd/juego/")[-1][:-1] for l in fuentes}))))

    cargados = [l.strip() for l in ls if "Loaded /cd/juego" in l]
    print("   assets leidos del MPQ       : %d" % len(cargados))
    for l in cargados[:4]:
        print("      %s" % l)

    print("   cuadros dibujados           : %d" % sum(("EBITEN-PERFIL" in l or "cuadros del loop" in l) for l in ls))
    print("   cartel del motor            : %d" % sum('game files not found at "/cd/juego"' in l for l in ls))

    ped = [l.strip() for l in ls if "Out of memory. Requested" in l]
    oom = [l.strip() for l in ls if l.startswith("Memory: arena=")]
    panic = [l.strip() for l in ls if l.startswith("panic:")]
    exc = [l.strip() for l in ls if "Unhandled exception" in l or "Data address error" in l]
    if ped:
        print("   muerte                      : %s" % ped[0])
        print("                                 %s" % (oom[-1] if oom else "(sin Memory:)"))
    elif panic:
        print("   muerte                      : %s" % panic[0])
    elif exc:
        print("   muerte                      : %s" % " / ".join(exc[:2]))
    else:
        print("   muerte                      : MUDA (la consola reinicia sin mensaje)")

    ult = [l.strip() for l in ls if l.strip() and "reios" not in l.lower()]
    if ult:
        print("   ULTIMA linea de la consola  : %s" % ult[-1])
    print()


def main():
    if len(sys.argv) != 3:
        print(__doc__)
        return 2
    analiza("FIX     (la tabla FUERA: se lee del disco)", sys.argv[1])
    analiza("MUTANTE (la tabla ADENTRO del binario)", sys.argv[2])
    return 0


if __name__ == "__main__":
    sys.exit(main())
