#!/usr/bin/env python3
"""DC-002.32: EL SCRIPT DEL LINKER CON EL `.eh_frame` AL FINAL (el que hace real la palanca).

POR QUE EXISTE (medido en esta tarjeta, no supuesto): sacar el `.eh_frame` con `objcopy -R` baja las
secciones ALLOC del ELF 1.074.504 bytes, pero la IMAGEN PLANA que la consola copia a RAM (`1ST_READ.BIN`,
`objcopy -O binary`) NO baja NI UN BYTE: el script de KOS (`utils/ldscripts/shlelf.xc`) pone el `.eh_frame`
EN MEDIO del mapa (antes de `.gcc_except_table`, `.ctors`, `.dtors`, `.data` y `.got`), asi que al sacarlo
queda un HUECO que objcopy rellena para conservar las direcciones de lo que viene despues. Medido: 13.777.032
bytes de 1ST_READ.BIN con y sin el `.eh_frame`.

La palanca, entonces, no es el `objcopy`: es DONDE EL LINKER LO PONE. Este generador escribe una copia del
script de KOS con las dos reglas del `.eh_frame` (`ONLY_IF_RO` / `ONLY_IF_RW`) MOVIDAS al final del mapa, justo
antes del `.ocram` (que vive en una direccion fija de la SH-4: 0x7c001000, asi que nada puede ir despues de
el). Con eso, lo que sigue al `.eh_frame` es solo `.bss` (NOBITS: no ocupa bytes en la imagen), y sacarlo baja
la imagen plana exactamente su tamano.

NO se toca el KOS vendorizado: el script se lee de `$KOS_BASE` y la copia modificada se escribe en el
directorio de build (descartable), con la ruta que se le pase. El corpus lo usa con
`DC_LDEXTRA="-Wl,-T,<copia>"` (el ultimo `-T` manda, medido).

Uso:  python3 scripts/console/gen-ldscript-eh-232.py <salida.xc>
"""
import os
import pathlib
import sys

SALIDA = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else "/tmp/dcbuild/dc232-ld/shlelf-eh-al-final.xc")
KOS = pathlib.Path(os.environ.get("KOS_BASE", os.path.expanduser("~/dreamcast/kos")))
ORIGEN = KOS / "utils/ldscripts/shlelf.xc"

RO = "  .eh_frame       : ONLY_IF_RO { KEEP (*(.eh_frame)) }\n"
RW = "  .eh_frame       : ONLY_IF_RW { KEEP (*(.eh_frame)) }\n"
MARCA = "  .eh_frame       : { KEEP (*(.eh_frame)) }   /* DC-002.32: al FINAL */\n"
# donde va la regla nueva: despues del .sdata y ANTES del __bss_start, o sea el ULTIMO PROGBITS del mapa
# (asi el .eh_frame es lo unico que sobra al final: sacarlo baja la imagen exactamente su tamano, y el .bss
# se corre para abajo con ella, que es RAM libre de verdad)
ANCLA = "  _edata = .; PROVIDE (edata = .);\n"


def main() -> int:
    if not ORIGEN.exists():
        print(f"no encuentro el script de KOS: {ORIGEN}", file=sys.stderr)
        return 1

    texto = ORIGEN.read_text()
    if RO not in texto or RW not in texto:
        print("el script de KOS no tiene las dos reglas del .eh_frame que esperaba", file=sys.stderr)
        return 1
    if texto.count(ANCLA) != 1:
        print("no encuentro el ancla del .ocram", file=sys.stderr)
        return 1

    # 1) sacar las dos reglas de donde estaban
    nuevo = texto.replace(RO, "").replace(RW, "")
    # 2) ponerla al final, antes del .ocram
    nuevo = nuevo.replace(ANCLA, MARCA + ANCLA)

    SALIDA.parent.mkdir(parents=True, exist_ok=True)
    SALIDA.write_text(nuevo)
    print(f"LDScript-232: {ORIGEN} -> {SALIDA}")
    print(f"LDScript-232: .eh_frame movido al final (antes del .ocram); {len(nuevo)} bytes de script")
    return 0


if __name__ == "__main__":
    sys.exit(main())
