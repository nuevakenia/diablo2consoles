#!/usr/bin/env python3
"""DC-002.47: EL `.eh_frame` FUERA DEL MAPA -- el script del linker que le da AIRE AL MONTON DE C.

POR QUE EXISTE (y en que se diferencia del de DC-002.32, que ya esta en el repo)
    DC-002.32 midio dos cosas con el `.eh_frame` (1.076.512 bytes de tablas de desenrollado):
      a) sacarlo con `objcopy -R .eh_frame` NO baja la imagen plana, porque el script de KOS lo pone EN MEDIO
         del mapa y el hueco se rellena para conservar las direcciones de lo que viene despues;
      b) moverlo AL FINAL con un script propio (`gen-ldscript-eh-232.py`) + `objcopy -R` SI baja la imagen
         plana exactamente su tamano.
    Lo que DC-002.32 NO midio es lo que importa aca: **el `.bss` NO se corre para abajo**. Medido en esta
    tarjeta con las dos tablas de secciones: el `.bss` queda en la MISMA direccion (0x8cbbE080) con y sin la
    palanca de DC-002.32, porque el linker le asigna su direccion con el `.eh_frame` ADENTRO del mapa (que
    el `.eh_frame` no vaya en la imagen plana no cambia el mapa). O sea: la palanca de DC-002.32 ahorra
    bytes de DISCO (la imagen que se lee), no bytes de RAM.

    Esta es la otra: que el `.eh_frame` no exista en el mapa. Se sacan sus dos reglas del script de KOS y se
    agrega al `/DISCARD/` (el linker lo tira como tira el `.note.GNU-stack`). Asi TODO lo que viene despues
    (`.gcc_except_table`, `.data`, `.got`, `.bss`) se corre para abajo sus 1.076.796 bytes, el `_end` del
    programa baja con ellos, y el monton de C -- que en KOS empieza justo en el `_end` (medido: `sbrk(0) -
    arena` == fin del `.bss`, exacto) -- GANA ese techo.

POR QUE NO CUESTA NADA (medido, no supuesto)
    * La traza del panic de la consola la hace KOS con el frame pointer (`arch_stk_trace`), no libgcc: la
      prueba de DC-002.32 (POC sonda-panic-232) da la MISMA traza con y sin `.eh_frame`.
    * El `__gccgo_personality_v0` de libgodc es un stub que devuelve siempre `_URC_CONTINUE_UNWIND`
      (runtime/runtime_stubs.c): no hay ningun handler de Go que se instale por `.eh_frame`.
    * `libgodc` y KOS son C: no hay excepciones de C++ en el binario de la consola.

NO se toca el KOS vendorizado: el script se lee de `$KOS_BASE` y la copia va al directorio de build.

Uso:  python3 scripts/console/gen-ldscript-eh-247.py <salida.xc>
"""
import os
import pathlib
import sys

SALIDA = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else "/tmp/dcbuild/dc247-ld/shlelf-sin-eh.xc")
KOS = pathlib.Path(os.environ.get("KOS_BASE", os.path.expanduser("~/dreamcast/kos")))
ORIGEN = KOS / "utils/ldscripts/shlelf.xc"

RO = "  .eh_frame       : ONLY_IF_RO { KEEP (*(.eh_frame)) }\n"
RW = "  .eh_frame       : ONLY_IF_RW { KEEP (*(.eh_frame)) }\n"
HDR = "  .eh_frame_hdr : { *(.eh_frame_hdr) }\n"
# DONDE VA EL REEMPLAZO: en el lugar de la regla RO (arriba del `.gcc_except_table`), con CUATRO BYTES
# CERO. Por que no se puede tirar la seccion sin mas (medido): `crtbegin.o` tiene relocaciones R_SH_DIR32
# contra el SIMBOLO DE SECCION `.eh_frame` (su `.text` le pasa el puntero a `__register_frame_info`), y el
# linker muere con
#   «.eh_frame» referenciado en la seccion «.text» de crtbegin.o: se definio en la seccion descartada «.eh_frame»
# Una seccion de 4 bytes ceros conserva el simbolo y cuesta 4 bytes: los 4 ceros son, ademas, el TERMINADOR
# de una tabla de FDE (`longitud = 0`), asi que `__register_frame_info` la recorre y la deja vacia sin leer
# nada mas alla.
CUATRO = ("  /* DC-002.47: del .eh_frame REAL no queda nada (ver /DISCARD/); lo unico que se queda es el de\n"
          "     crtbegin.o (0 bytes: es solo la MARCA del principio) y el de crtend.o (4 bytes: el TERMINADOR\n"
          "     de tabla, `longitud = 0`). Esos dos no se pueden tirar: la relocacion R_SH_DIR32 de crtbegin.o\n"
          "     va contra el simbolo de SU seccion `.eh_frame`, y sin ella el linker muere con\n"
          "       «.eh_frame» referenciado en la seccion «.text» de crtbegin.o: se definio en la seccion\n"
          "       descartada «.eh_frame» de crtbegin.o.                          (medido en DC-002.47)\n"
          "     Costo: 4 bytes en vez de 1.076.796, y `__register_frame_info` recorre el terminador y deja la\n"
          "     tabla vacia. */\n"
          "  .eh_frame       : { KEEP(*crtbegin.o(.eh_frame)) KEEP(*crtend.o(.eh_frame)) }\n")
# el /DISCARD/ que trae KOS: ahi se suman las dos secciones de desenrollado
ANCLA = "  /DISCARD/ : { *(.note.GNU-stack) *(.gnu_debuglink) }"
NUEVO = ("  /* DC-002.47: el .eh_frame NO EXISTE en el mapa (no solo fuera de la imagen plana): lo que viene\n"
         "     despues se corre para abajo sus 1.076.796 bytes y el monton de C gana ese techo. */\n"
         "  /DISCARD/ : { *(.note.GNU-stack) *(.gnu_debuglink) *(.eh_frame) *(.eh_frame_hdr) }")


def main() -> int:
    if not ORIGEN.exists():
        print(f"no encuentro el script de KOS: {ORIGEN}", file=sys.stderr)
        return 1

    texto = ORIGEN.read_text()

    faltan = [n for n, regla in (("ONLY_IF_RO", RO), ("ONLY_IF_RW", RW)) if regla not in texto]
    if faltan:
        print(f"el script de KOS no tiene las reglas del .eh_frame que esperaba: {faltan}", file=sys.stderr)
        return 1

    if texto.count(ANCLA) != 1:
        print("no encuentro el /DISCARD/ de KOS", file=sys.stderr)
        return 1

    nuevo = texto.replace(RO, CUATRO).replace(RW, "").replace(HDR, "")
    nuevo = nuevo.replace(ANCLA, NUEVO)

    SALIDA.parent.mkdir(parents=True, exist_ok=True)
    SALIDA.write_text(nuevo)
    print(f"LDScript-247: {ORIGEN} -> {SALIDA}")
    print("LDScript-247: .eh_frame y .eh_frame_hdr fuera del mapa (al /DISCARD/); "
          f"{len(nuevo)} bytes de script")
    return 0


if __name__ == "__main__":
    sys.exit(main())
