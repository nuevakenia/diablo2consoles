#!/usr/bin/env python3
"""La tabla de los estados de DC-002.34, sacada de los logs de Flycast (sin interpretacion).

Los logs son los del corpus `scripts/console/checar-motor-disco-234.sh`, en este orden:
  BASE (sin la palanca de DC-002.33 y sin la de DC-002.32), S233 (solo la de 233), S232 (las dos),
  S232SOLO (solo la de 232, con las tablas del MPQ como objetos) y el MUTANTE (la ISO sin /JUEGO).

Uso:  tabla-estados-234.py <log_base> <log_s233> <log_s232> <log_s232solo> <log_mutante>
      (con 4 logs tambien anda: se saltan los estados que no se pases, en el orden de arriba)
"""
import re
import sys

NOMBRES = [
    "BASE    (sin 233, sin 232)",
    "S233    (solo 233)",
    "S232    (233 y 232)",
    "S232SOLO (solo 232, tablas como objetos)",
    "MUTANTE (ISO sin /JUEGO)",
]


def analiza(nombre, path):
    with open(path, "rb") as fh:
        ls = fh.read().decode("utf-8", "replace").replace("\r", "").split("\n")

    print("== %s" % nombre)
    print("   vueltas del arranque        : %d" % sum("MOTOR-ARRANQUE arranca" in l for l in ls))

    m = [l for l in ls if "monton de C antes del motor" in l]
    if m:
        print("   monton de C antes del motor : %s" % m[0].split(":", 2)[-1].strip())

    semi = [int(x.group(1)) for l in ls for x in [re.search(r"gc-por-umbral-bytes-usados v=(\d+)", l)] if x]
    if semi:
        print("   semiespacio usado maximo    : %d -> libre minimo %d (%d marcas)"
              % (max(semi), 524288 - max(semi), len(semi)))

    fuentes = [l.strip() for l in ls if "Adding source: '/cd/juego" in l]
    print("   fuentes del disco abiertas  : %d" % len(fuentes))
    for l in fuentes[:3]:
        print("      %s" % l)

    cargados = [l.strip() for l in ls if "Loaded /cd/juego" in l]
    print("   assets leidos del MPQ       : %d" % len(cargados))
    for l in cargados[:4]:
        print("      %s" % l)

    # DC-002.43: LAS MARCAS DE CARGA (el instrumento de la muerte muda). La ultima dice la llamada exacta en
    # la que estaba el motor cuando murio, con la memoria de ese instante. Si no hay ninguna, la corrida NO
    # trae el instrumento (y entonces no se puede leer donde murio, no es que no haya cargado nada).
    marcas = [l.strip() for l in ls if "ARQ-CARGA" in l]
    if marcas:
        print("   marcas de carga             : %d" % len(marcas))
        print("      ultima : %s" % marcas[-1])
        # ojo: NO hay marca de control que se imprima sola (el `init` del paquete no entra en la cadena de
        # arranque de la consola: ver marca_carga_godc.go). El control es HABER marcas o no haberlas.
        print("      primera: %s" % marcas[0])
    else:
        print("   marcas de carga             : 0 -- corrida SIN el instrumento de DC-002.43")

    print("   cuadros dibujados           : %d" % sum(("EBITEN-PERFIL" in l or "cuadros del loop" in l) for l in ls))
    print("   cartel del motor            : %d" % sum('game files not found at "/cd/juego"' in l for l in ls))

    ped = [l.strip() for l in ls if "Out of memory. Requested" in l]
    oom = [l.strip() for l in ls if l.startswith("Memory: arena=")]
    exc = [l.strip() for l in ls if "Unhandled exception" in l or "Data address error" in l]
    # DC-002.43: el `panic:` de Go TAMBIEN es una muerte con mensaje (DC-002.41 lo dejo anotado: el
    # clasificador lo rotulaba MUDA aunque la linea estaba en el log). Y el `fatal error:` tambien.
    pan = [l.strip() for l in ls if l.startswith("panic:")]
    fat = [l.strip() for l in ls if l.startswith("fatal error:")]
    if ped:
        print("   muerte                      : %s" % ped[0])
        print("                                 %s" % (oom[0] if oom else "(sin Memory:)"))
    elif pan or fat:
        print("   muerte                      : %s" % (pan[0] if pan else fat[0]))
        if oom:
            print("                                 %s" % oom[0])
    elif exc:
        print("   muerte                      : %s" % " / ".join(exc[:2]))
    else:
        print("   muerte                      : MUDA (la consola reinicia sin mensaje)")

    paradas = [l.strip() for l in ls if "Flycast has stopped" in l]
    if paradas:
        print("   el emulador                 : %s" % paradas[0])

    ult = [l.strip() for l in ls if l.strip() and "reios" not in l.lower()]
    print("   ULTIMA linea de la consola  : %s" % (ult[-1] if ult else "(vacia)"))
    print()


def main():
    args = sys.argv[1:]
    if len(args) == 4:
        # sin el estado S232SOLO: se mantiene el orden de los cinco y se saltea ese
        args = args[:3] + [None] + [args[3]]
    if len(args) != 5:
        print(__doc__)
        return 2
    for nombre, path in zip(NOMBRES, args):
        if path is None:
            continue
        analiza(nombre, path)
    return 0


if __name__ == "__main__":
    sys.exit(main())
