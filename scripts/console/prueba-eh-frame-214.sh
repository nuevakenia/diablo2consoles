#!/bin/bash
# DC-002.14: prueba de la palanca .eh_frame (tabla de desenrollado, 1.066.804 bytes en el ELF del motor).
#
# Pregunta: gccgo honra -fno-asynchronous-unwind-tables? (En el mismo ELF se midio que IGNORA
# -ffunction-sections: 0 secciones .text.* y el binario salio 2.316 bytes MAS GRANDE, no mas chico.)
# Se compila UN paquete de dos maneras y se comparan sus secciones: si .eh_frame desaparece, la palanca es
# real y se puede usar en los 205 .gox.
set -uo pipefail
cd /home/dgalli/Proyectos/diablo2consoles
# shellcheck source=/dev/null
source scripts/console/env.sh

GEN="${1:-/tmp/gox-214-gc}"
PKG="${2:-d2common}"
S="$PKG"
srcs=$(ls "$S"/*.go 2>/dev/null | grep -v _test.go |
    grep -vE '_(darwin|windows|js|wasm|plan9|freebsd|netbsd|openbsd|linux|android|ios|solaris|aix)\.go$' |
    grep -vE '_(386|amd64|arm|arm64|ppc64|ppc64le|mips|mipsle|mips64|mips64le|riscv64|s390x|loong64|wasm)\.go$')

compilar() {
    local salida="$1"
    shift
    # shellcheck disable=SC2086
    sh-elf-gccgo $GODC_GOFLAGS "$@" -I "$GEN" \
        -fgo-pkgpath="github.com/OpenDiablo2/OpenDiablo2/$PKG" -c $srcs -o "$salida" 2>&1 | head -3
}

secciones() {
    sh-elf-size -A "$1" | grep -E '^\.(text|rodata|data|bss|eh_frame|gcc_except_table)' | sed 's/^/      /'
}

echo "== $PKG sin la bandera"
compilar /tmp/eh-a.gox
secciones /tmp/eh-a.gox
echo "== $PKG con -fno-asynchronous-unwind-tables"
compilar /tmp/eh-b.gox -fno-asynchronous-unwind-tables
secciones /tmp/eh-b.gox
echo
echo "== el .eh_frame del motor entero, para saber a que escala se mueve"
sh-elf-size -A /tmp/dcbuild/motor-arranque/prog.elf | grep -E 'eh_frame|gcc_except'
