#!/bin/bash
# CORPUS DE DC-002.20: EL GZIP DE LA CONSOLA CONTRA EL ASSET EMBEBIDO DEL MOTOR.
#
# QUE SE MIDE (POC console/dc/poc/gzip-asset, que corre EN Flycast e importa el paquete del MOTOR que
# paniqueaba, `d2common/d2util/assets`, o sea el mismo `CompressedDebugText` del binario):
#
#  CONTROL 0 (el host, como referencia): console/dc/gen-asset-info lee el asset con el `compress/gzip` y el
#    `hash/crc32` DE VERDAD de Go y dice 2.209 bytes comprimidos (crc32 aa973211), 98.358 descomprimidos
#    (crc32 5dce3e98) y un BMP de 256 x 128 x 24. Si el host no da eso, no hay con que comparar.
#  CONTROL 1 (los bytes embebidos): la consola ve los MISMOS 2.209 bytes, con el mismo crc32. Si no, el
#    problema estaria en el binario y no en el gzip.
#  CONTROL 2 (el gzip EN STREAMING): 98.358 bytes con crc32 5dce3e98, leidos de a 512 bytes en 193
#    lecturas. Es el arreglo: el camino viejo no podia (ver control 5).
#  CONTROL 3 (la cabecera del resultado): los cuatro primeros bytes son 66 77 54 128, o sea "BM": es el BMP
#    descomprimido, no basura.
#  CONTROL 4 (el camino del MOTOR, tal cual): `gzip.NewReader` + `bmp.Decode(s)` sobre el asset embebido
#    decodifica un 256 x 128. Es la linea que paniqueaba con `assets: bmp.Decode failed: gzip: invalid
#    header`.
#  CONTROL 5 (el defecto declarado): el camino VIEJO (una sola vez, destino = 4 x comprimido + 64 KB =
#    74.372 bytes) sigue devolviendo -1, porque el resultado real son 98.358 bytes y no entra. Se exige que
#    SIGA fallando: es el limite conocido de `d2c_zlib_inflate`, y el dia que alguien cambie la regla este
#    control se pone ROJO y hay que venir a leer esta nota.
#  CONTROL 6 (las marcas): el numero EXACTO de lineas `GZIP-ASSET` de la corrida.
#
# MUTANTES (3), cada uno tiene que poner ROJO el control que le toca:
#   'pos-de-un-tiro'  (Go) `g.pos += consumido` -> `g.pos = len(g.comprimido)`: el lector se come el
#                     comprimido entero en el primer paso y la zlib se queda sin entrada.
#   'consumido-cero'  (C)  `d2c_zs_consumido[slot] = ...` -> 0: un paso no avanza y el lector no puede
#                     saber cuanto avanzo (es el ABI del puente, del lado del C).
#   'ventana-sin-gzip' (C) `inflateInit2(15 + 32)` -> `inflateInit2(15)`: la zlib deja de reconocer la
#                     cabecera gzip. Es, literalmente, el `invalid header` del defecto.
#
# Uso:  bash scripts/console/checar-gzip-220.sh [dir_gox] [segundos]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-00219}"
SEGS="${2:-20}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
POC=console/dc/poc/gzip-asset
BUILD=/tmp/dcbuild/gzip-asset

GZ="$REPO/console/dc/std/compress/gzip/gzip.go"
ZC="$REPO/console/dc/std/csrc/zlib_dc.c"

MARCAS=6
TOTAL_CTRL=7
TOTAL_MUT=3
CAZADOS=0

# OJO: build-dc.sh toma el arbol de .gox de GODC_GOX, NO del argumento de este script. Sin esto compilaba
# contra $DC_ROOT/gox (la gox compartida) y el POC no encontraba el paquete del MOTOR: "import file
# github.com/OpenDiablo2/.../assets not found" (medido).
export GODC_GOX="$GEN"

for f in "$GEN/orden.txt" "$GZ" "$ZC" "$POC/main.go"; do
    [ -f "$f" ] || { echo "falta $f" >&2; exit 1; }
done

# ---------------------------------------------------------------- compilar y correr
compilar() { # deja los logs en /tmp/gzip-220.std.log y /tmp/gzip-220.build.log
    if ! bash "$HERE/compilar-paquete-std.sh" compress/gzip "$GEN" > /tmp/gzip-220.std.log 2>&1; then
        echo "   el gzip no compila (ver /tmp/gzip-220.std.log)"; return 1
    fi
    if ! bash "$HERE/build-dc.sh" "$POC" > /tmp/gzip-220.build.log 2>&1; then
        echo "   el POC no enlaza (ver /tmp/gzip-220.build.log)"; return 1
    fi
}

correr() { # $1 = dir de la corrida
    rm -rf "$1"
    (cd "$REPO" && CAPTURE=none bash "$HERE/run-flycast.sh" "$BUILD/prog.elf" "$SEGS" "$1") > "$1.salida" 2>&1
    [ -f "$1/flycast.log" ]
}

# ---------------------------------------------------------------- los siete controles
ctrl_host() {
    (cd "$REPO" && go run ./console/dc/gen-asset-info) 2>/dev/null > /tmp/gzip-220.host.txt
    grep -qaF 'embebido bytes=2209' /tmp/gzip-220.host.txt &&
        grep -qaF 'descomprimido bytes=98358' /tmp/gzip-220.host.txt &&
        grep -qaF 'descomprimido crc32=5dce3e98' /tmp/gzip-220.host.txt &&
        grep -qaF 'bmp ancho=256 alto=128 bits_por_pixel=24' /tmp/gzip-220.host.txt
}
ctrl_embebido() {
    # 2862035473 es 0xAA973211 en decimal: `println` de la consola imprime sin signo.
    grep -qaF 'GZIP-ASSET embebido bytes= 2209 crc32= 2862035473' "$1"
}
ctrl_streaming() {
    grep -qaF 'GZIP-ASSET nuevo bytes= 98358 crc32= 1573797528 lecturas_de_512= 193' "$1"
}
ctrl_cabecera() {
    grep -qaF 'GZIP-ASSET nuevo cabecera= 66 77 54 128' "$1"
}
ctrl_motor() {
    grep -qaF 'GZIP-ASSET motor bmp.Decode OK ancho= 256 alto= 128' "$1"
}
ctrl_viejo_falla() {
    grep -qaF 'GZIP-ASSET viejo cota= 74372 devuelto= -1 entra_el_real= false' "$1"
}
ctrl_marcas() {
    [ "$(grep -ac 'GZIP-ASSET' "$1")" = "$MARCAS" ]
}

controles() { # $1 = log; imprime los siete y devuelve cuantos dan bien (en la ultima linea)
    local log="$1" bien=0
    if ctrl_host;        then echo "   control 0 (el host de referencia):      VERDE"; bien=$((bien + 1));
    else echo "   control 0 (el host de referencia):      ROJO"; fi
    if ctrl_embebido "$log";    then echo "   control 1 (los bytes embebidos):        VERDE"; bien=$((bien + 1));
    else echo "   control 1 (los bytes embebidos):        ROJO"; fi
    if ctrl_streaming "$log";   then echo "   control 2 (el gzip en streaming):       VERDE"; bien=$((bien + 1));
    else echo "   control 2 (el gzip en streaming):       ROJO"; fi
    if ctrl_cabecera "$log";    then echo "   control 3 (la cabecera BM):             VERDE"; bien=$((bien + 1));
    else echo "   control 3 (la cabecera BM):             ROJO"; fi
    if ctrl_motor "$log";       then echo "   control 4 (el camino del motor):        VERDE"; bien=$((bien + 1));
    else echo "   control 4 (el camino del motor):        ROJO"; fi
    if ctrl_viejo_falla "$log"; then echo "   control 5 (el defecto declarado):       VERDE"; bien=$((bien + 1));
    else echo "   control 5 (el defecto declarado):       ROJO"; fi
    if ctrl_marcas "$log";      then echo "   control 6 (las $MARCAS marcas exactas):         VERDE"; bien=$((bien + 1));
    else echo "   control 6 (las $MARCAS marcas exactas):         ROJO ($(grep -ac 'GZIP-ASSET' "$log"))"; fi
    echo "$bien"
}

mostrar() {
    grep -a 'GZIP-ASSET' "$1" | tr -d '\r' | sed 's/^/     /'
    echo "     (lineas GZIP-ASSET de la corrida: $(grep -ac 'GZIP-ASSET' "$1"))"
}

# ---------------------------------------------------------------- mutacion de archivos
# El respaldo es una copia propia (nunca `git checkout --`): en medio de la tarjeta hay arreglos SIN
# COMMITEAR en esos mismos archivos. Despues de cada mutante se verifica el sha256.
mutar() { # <archivo> <buscar> <poner>
    local arch="$1" buscar="$2" poner="$3"
    python3 - "$arch" "$buscar" "$poner" <<'PY' || return 1
import sys
ruta, buscar, poner = sys.argv[1], sys.argv[2], sys.argv[3]
t = open(ruta).read()
n = t.count(buscar)
if n != 1:
    print("el texto a mutar aparece %d veces (tiene que ser 1)" % n, file=sys.stderr)
    sys.exit(2)
open(ruta, "w").write(t.replace(buscar, poner))
PY
}

exportar() { # <archivo>; imprime "<copia> <sha256>"
    local copia
    copia="$(mktemp /tmp/checar-gzip-220-XXXXXX)"
    cp "$1" "$copia"
    echo "$copia $(sha256sum "$1" | awk '{print $1}')"
}

restaurar() { # <archivo> <copia> <sha>
    cp "$2" "$1"
    if [ "$(sha256sum "$1" | awk '{print $1}')" != "$3" ]; then
        echo "   EL ARBOL QUEDO MUTADO: no coincide el sha256 de $1" >&2
        return 1
    fi
    echo "   arbol restaurado ($(basename "$1"))"
}

probar_mutante() { # <nombre> <archivo> <buscar> <poner> <control que tiene que caer>
    local nombre="$1" arch="$2" buscar="$3" poner="$4" tiene_que_caer="$5"
    local run="/tmp/checar-gzip-220-mut-$nombre"
    echo
    echo "== mutante '$nombre'"
    local copia="" sha=""
    read -r copia sha <<<"$(exportar "$arch")"
    if ! mutar "$arch" "$buscar" "$poner"; then
        echo "   NO SE APLICO LA MUTACION (patron sin match unico): el mutante no prueba nada" >&2
        restaurar "$arch" "$copia" "$sha" || exit 1
        rm -f "$copia"
        return
    fi
    if compilar && correr "$run"; then
        controles "$run/flycast.log" | sed -n '1,7p'
        if "$tiene_que_caer" "$run/flycast.log"; then
            echo "   NO LO CAZO: el control que tenia que caer sigue VERDE con el mutante puesto" >&2
        else
            echo "   cazado: el control que tenia que caer cayo"
            CAZADOS=$((CAZADOS + 1))
        fi
    else
        echo "   NO LO CAZO: con el mutante no compila/enlaza/corre (o no dejo log), y un mutante que no corre no prueba nada" >&2
    fi
    restaurar "$arch" "$copia" "$sha" || exit 1
    rm -f "$copia"
}

# ---------------------------------------------------------------- 1. control sano
echo "== 1. control sano: con el arreglo puesto, los $TOTAL_CTRL controles"
compilar || exit 1
correr /tmp/checar-gzip-220-sano || exit 1
BIEN="$(controles /tmp/checar-gzip-220-sano/flycast.log | sed -n '$p')"
controles /tmp/checar-gzip-220-sano/flycast.log | sed -n '1,7p'
if [ "$BIEN" -ne "$TOTAL_CTRL" ]; then
    echo "   EL CONTROL SANO NO DA ($BIEN/$TOTAL_CTRL): no se sigue, primero hay que ver por que" >&2
    mostrar /tmp/checar-gzip-220-sano/flycast.log >&2
    exit 1
fi
echo "   OK: $BIEN/$TOTAL_CTRL controles verdes con el codigo de ahora"
mostrar /tmp/checar-gzip-220-sano/flycast.log

# ---------------------------------------------------------------- 2. los mutantes
probar_mutante pos-de-un-tiro "$GZ" \
    'g.pos += consumido' \
    'g.pos = len(g.comprimido)   // MUTANTE pos-de-un-tiro' \
    ctrl_streaming

probar_mutante consumido-cero "$ZC" \
    'd2c_zs_consumido[slot] = src_len - (int)d2c_zs[slot].avail_in;' \
    'd2c_zs_consumido[slot] = 0;   /* MUTANTE consumido-cero */' \
    ctrl_streaming

probar_mutante ventana-sin-gzip "$ZC" \
    'if (inflateInit2(&d2c_zs[i], 15 + 32) != Z_OK) return -1;' \
    'if (inflateInit2(&d2c_zs[i], 15) != Z_OK) return -1;   /* MUTANTE ventana-sin-gzip */' \
    ctrl_streaming

# ---------------------------------------------------------------- 3. control sano de vuelta
echo
echo "== 3. control sano de vuelta (el arbol entero, sin mutar)"
if compilar && correr /tmp/checar-gzip-220-vuelta; then
    BIEN="$(controles /tmp/checar-gzip-220-vuelta/flycast.log | sed -n '$p')"
    controles /tmp/checar-gzip-220-vuelta/flycast.log | sed -n '1,7p'
    [ "$BIEN" -eq "$TOTAL_CTRL" ] || { echo "   NO VOLVIERON LOS $TOTAL_CTRL CONTROLES ($BIEN/$TOTAL_CTRL)" >&2; exit 1; }
    echo "   control sano VERDE"
else
    echo "   NO VOLVIO EL CONTROL SANO" >&2
    exit 1
fi

echo
echo "== $CAZADOS/$TOTAL_MUT mutantes cazados =="
[ "$CAZADOS" -eq "$TOTAL_MUT" ]
