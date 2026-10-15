#!/bin/bash
# El toolchain precompilado (drpaneas/dreamcast-toolchain-builds) se armó en /opt/toolchains/dc,
# asi que kos-ports/lib/*.a quedaron como symlinks rotos a esa ruta. Este script los repunta a los
# archivos reales que SI vienen dentro del tarball (kos-ports/<port>/inst/lib/*.a).
# Es idempotente: si ya estan sanos, no hace nada.
set -u
KOS_PORTS="${KOS_PORTS:-$HOME/dreamcast/kos-ports}"
LIB="$KOS_PORTS/lib"
[ -d "$LIB" ] || { echo "no existe $LIB"; exit 1; }
rep=0; rotos=0
for f in "$LIB"/*.a; do
  [ -e "$f" ] && continue
  rotos=$((rotos+1))
  base=$(basename "$f")
  found=$(find "$KOS_PORTS" -path "*/inst/lib/$base" 2>/dev/null | head -1)
  if [ -n "$found" ]; then ln -sf "$found" "$f"; rep=$((rep+1)); fi
done
echo "symlinks rotos: $rotos | reparados: $rep"
for f in "$LIB"/*.a; do [ -e "$f" ] || echo "SIGUE ROTO: $(basename "$f")"; done
