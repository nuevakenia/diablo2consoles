#!/bin/bash
# Arreglos al toolchain precompilado de Dreamcast (drpaneas/dreamcast-toolchain-builds).
# Son dos cosas que vienen mal en el tarball y que hay que aplicar una vez:
#
#   1) kos-ports/lib/*.a son symlinks rotos a /opt/toolchains/dc (la ruta de la maquina donde
#      se compilo el tarball). Se repuntan a los archivos reales del propio tarball.
#   2) el Makefile de los bindings de Go (libgodc/kos/Makefile) no incluye perf.go en
#      GO_SOURCES, asi que el kos.gox que viene compilado NO trae los contadores de
#      rendimiento (PerfCntrStart/PerfCntrCycles). Se agrega y se recompila el paquete.
#
# Idempotente: si ya esta aplicado, no hace nada.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DC_ROOT="${DC_ROOT:-$HOME/dreamcast}"
# el Makefile de los bindings necesita sh-elf-gccgo, sh-elf-ar, sh-elf-objcopy y kos-cc en el PATH
export PATH="$DC_ROOT/sh-elf/bin:$DC_ROOT/kos/utils/build_wrappers:$PATH"
# kos-cc es un wrapper que hace `exec $KOS_CC ...`: sin KOS_CC exportado falla con
# "exec: -O: opción inválida", asi que se carga el entorno de KOS (apagando -u, que el
# environ.sh de KOS referencia variables sin definir).
if [ -f "$DC_ROOT/kos/environ.sh" ]; then
  set +u
  # shellcheck disable=SC1091
  . "$DC_ROOT/kos/environ.sh" >/dev/null 2>&1
  set -u
fi

echo "== 1) symlinks de kos-ports =="
KOS_PORTS="${KOS_PORTS:-$DC_ROOT/kos-ports}" bash "$HERE/fix-ports-symlinks.sh"

echo "== 2) perf.go en los bindings de Go de libgodc =="
MK="$DC_ROOT/libgodc/kos/Makefile"
if [ ! -f "$MK" ]; then
  echo "no existe $MK"; exit 1
fi
if grep -q "perf.go" "$MK"; then
  echo "  ya incluye perf.go"
else
  cp "$MK" "$MK.orig"
  python3 - "$MK" <<'EOF'
import sys, pathlib
p = pathlib.Path(sys.argv[1])
t = p.read_text()
if "./vmu.go" in t:
    t = t.replace("fs.go \\\n             vmu.go", "fs.go \\\n             vmu.go \\\n             perf.go")
elif "vmu.go" in t:
    t = t.replace("vmu.go", "vmu.go \\\n             perf.go", 1)
p.write_text(t)
print("  agregado perf.go a GO_SOURCES")
EOF
fi

cd "$DC_ROOT/libgodc/kos" || exit 1
[ -f kos.gox.orig ] || cp kos.gox kos.gox.orig
[ -f libkos.a.orig ] || cp libkos.a libkos.a.orig
echo "== recompilando el paquete kos =="
make -B all 2>&1 | tail -4
if strings kos.gox | grep -q "PerfCntrStart"; then
  echo "OK: los bindings de Go ya traen los contadores de rendimiento"
else
  echo "AVISO: no se ve PerfCntrStart en kos.gox"
fi
