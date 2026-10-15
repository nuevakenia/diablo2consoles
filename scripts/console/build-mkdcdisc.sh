#!/bin/bash
# Compila mkdcdisc (la herramienta que arma la imagen .cdi autoarrancable para el GDEMU).
# Va todo a $DC_ROOT/deps: nada al sistema, nada de sudo.
#
# Dos cosas que hay que saber antes de correrlo:
#
#  1. mkdcdisc necesita libisofs (la biblioteca de libburnia). No esta en el sistema y no se
#     puede instalar sin root, asi que se compila aca. OJO: se compila COMPARTIDA, porque la
#     estatica queda con una referencia a libacl sin resolver al enlazar.
#
#  2. mkdcdisc tiene un bug real: en ElfSection hace
#         m_Data.resize(shdr.sh_size); memcpy(&m_Data[0], ...);
#     y para la seccion NULL de todo ELF sh_size es 0, asi que memcpy de 0 bytes sobre un
#     vector vacio. En Arch/CachyOS las flags por defecto traen _GLIBCXX_ASSERTIONS, que lo
#     aborta con "Assertion '__n < this->size()' failed". Se compila con
#     -U_GLIBCXX_ASSERTIONS y listo (el memcpy de 0 bytes es inofensivo).
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DC_ROOT="${DC_ROOT:-$HOME/dreamcast}"
DEPS="$DC_ROOT/deps"
WORK="${WORK:-/tmp/dcsrc}"
LIBISOFS_VER="${LIBISOFS_VER:-1.5.8}"

mkdir -p "$DEPS" "$WORK"

echo "== 1/3 libisofs $LIBISOFS_VER (compartida, en $DEPS) =="
if [ ! -f "$DEPS/lib/libisofs.so" ]; then
    cd "$WORK"
    [ -f libisofs.tar.gz ] || curl -sL -o libisofs.tar.gz \
        "https://files.libburnia-project.org/releases/libisofs-$LIBISOFS_VER.pl02.tar.gz"
    rm -rf "libisofs-$LIBISOFS_VER"
    tar xzf libisofs.tar.gz
    cd "libisofs-$LIBISOFS_VER"
    ./configure --prefix="$DEPS" --disable-versioned-libs >/dev/null
    make -j"$(nproc)" >/dev/null
    make install >/dev/null
else
    echo "   ya esta instalada"
fi

echo "== 2/3 mkdcdisc =="
if [ ! -x "$DEPS/bin/mkdcdisc" ]; then
    cd "$WORK"
    [ -d mkdcdisc ] || git clone -q --depth 1 https://gitlab.com/cepawiel/mkdcdisc.git mkdcdisc
    cd mkdcdisc
    # venv: si no hay meson en el sistema, se usa el del venv del toolchain
    export PATH="$DC_ROOT/venv/bin:$PATH"
    export PKG_CONFIG_PATH="$DEPS/lib/pkgconfig"
    export LD_LIBRARY_PATH="$DEPS/lib"
    command -v meson >/dev/null || "$DC_ROOT/venv/bin/pip" install meson ninja
    rm -rf build
    CXXFLAGS="-U_GLIBCXX_ASSERTIONS -DNDEBUG -O2" meson setup build --buildtype=release >/dev/null
    ninja -C build >/dev/null
    install -Dm755 build/mkdcdisc "$DEPS/bin/mkdcdisc"
else
    echo "   ya esta compilado"
fi

echo "== 3/3 prueba =="
export LD_LIBRARY_PATH="$DEPS/lib"
"$DEPS/bin/mkdcdisc" --help 2>&1 | head -2 || true   # head cierra el pipe: mkdcdisc muere con SIGPIPE
echo
echo "mkdcdisc listo en $DEPS/bin/mkdcdisc"
echo "OJO: para correrlo hace falta  export LD_LIBRARY_PATH=$DEPS/lib"
