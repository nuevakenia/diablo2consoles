#!/bin/bash
# Entorno del toolchain de Dreamcast (KOS 2.2.1 + gccgo 15.1 + libgodc).
# Uso:  source scripts/console/env.sh
#
# Notas aprendidas a golpes (ver docs/auditoria y la rama DC-001):
#  * gccgo resuelve `import "kos"` buscando <dir>/kos.gox en cada -I: hay que apuntar a
#    $LIBGODC/kos (no a $LIBGODC).
#  * libkos.a ya trae los stubs C (pvr_dr.c, plx_c_stub.c): NO compilarlos aparte.
#  * PLX vive en kos-ports (libparallax.a), no en libkallisti.
#  * libtremor y libogg definen los mismos simbolos: usar solo uno (aca, libogg).
#  * kos_img_free viene de libkosutils (addons de KOS).

export DC_ROOT="${DC_ROOT:-$HOME/dreamcast}"
export KOS_BASE="$DC_ROOT/kos"
export KOS_PORTS="$DC_ROOT/kos-ports"
export LIBGODC="$DC_ROOT/libgodc"
export PATH="$DC_ROOT/sh-elf/bin:$KOS_BASE/utils/build_wrappers:$PATH"

# KOS define KOS_CC, KOS_CFLAGS, KOS_LDFLAGS, etc.
# OJO: el environ.sh de KOS referencia variables sin definir, asi que con `set -u` activo
# (build-dc.sh usa set -euo pipefail) el `source` aborta sin decir nada. Se apaga -u al entrar.
if [ -f "$KOS_BASE/environ.sh" ]; then
  case $- in *u*) _dc_had_u=1 ;; *) _dc_had_u= ;; esac
  set +u
  . "$KOS_BASE/environ.sh" >/dev/null 2>&1
  [ -n "$_dc_had_u" ] && set -u
  unset _dc_had_u
fi

# ---- compilacion de Go ----
# DC-002.68 (el recorte): el binario del motor entero no dejaba RAM para la pantalla (DC-002.67: 1ST_READ.BIN
# de 11.664.488 B y malloc(1.228.800) FALLA en la consola). Dos flags atacan eso desde la compilacion:
#   -g0                  los .gox traian DWARF (11,3 MB de los 40,6 del prog.elf). No va al BIN, pero engorda
#                        los objetos y el enlace. Medido en d2common/d2act3: 4.340 B de .debug -> 0.
#   -ffunction-sections  una seccion .text.<simbolo> por funcion (medido en el mismo paquete: 2 -> 40
#                        secciones). Sin esto el linker NO puede tirar nada: entra el .text del paquete entero.
#   -fdata-sections      lo mismo con los datos (tablas, cadenas, descriptores de tipo): sin esto el .rodata
#                        del paquete es un bloque unico y --gc-sections se lo tiene que quedar completo.
# Los tres viajan por GODC_GOFLAGS a TODOS los caminos que compilan Go para la consola (la stdlib minima,
# los terceros, los .gox del motor y el main del POC), que es lo que hace que el enlace sea homogeneo.
# El enlace con --gc-sections esta en build-dc.sh (ahi se tira lo que no se alcanza).
export GODC_GOFLAGS="-O2 -g0 -ffunction-sections -fdata-sections -ml -m4-single -fno-split-stack -mfsrra -mfsca -I$LIBGODC/kos -I$LIBGODC -I$KOS_BASE/include"

# ---- enlace ----
export DC_LDPATHS="-L$LIBGODC -L$LIBGODC/kos -L$KOS_PORTS/lib -L$KOS_BASE/lib/dreamcast -L$KOS_BASE/addons/lib/dreamcast"
# --whole-archive en libgodcbegin: contiene el arranque del runtime Go (kos.Main / main).
export DC_LIBS_PRE="-Wl,--whole-archive -lgodcbegin -Wl,--no-whole-archive -lkos -lgodc"
export DC_LIBS_POST="-Wl,--start-group -lparallax -limageload -lkmg -lpng -ljpeg -lz -lwav -loggvorbisplay -lvorbis -logg -lkosutils -lkallisti -lc -lgcc -lm -Wl,--end-group"

# ---- herramientas de imagen ----
# python con pycdlib (para armar la imagen): uv venv ~/dreamcast/venv && uv pip install pycdlib
export DC_PYTHON="${DC_PYTHON:-$HOME/dreamcast/venv/bin/python}"
[ -x "$DC_PYTHON" ] || export DC_PYTHON=python3

export DC_MAKEIP="$KOS_BASE/utils/makeip/makeip"
export DC_SCRAMBLE="$KOS_BASE/utils/scramble/scramble"
