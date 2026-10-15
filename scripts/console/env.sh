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
export GODC_GOFLAGS="-O2 -ml -m4-single -fno-split-stack -mfsrra -mfsca -I$LIBGODC/kos -I$LIBGODC -I$KOS_BASE/include"

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
