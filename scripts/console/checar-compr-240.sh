#!/bin/bash
# CORPUS DE DC-002.40: POR QUE LA CONSOLA NO PODIA LEER UN ARCHIVO COMPRIMIDO DE UN MPQ.
#
# QUE SE MIDE. POC `console/dc/poc/compr-40` (corre EN Flycast, monta un disco con los MPQ de verdad y
# llama al MOTOR por `d2mpq.FromFile` + `ReadFile`, o sea el mismo camino que fallaba). Los numeros de
# referencia los da el HOST con `scripts/console/sonda-mpq-compr.py` (lectura cruda del MPQ con la zlib de
# Python, a proposito SIN codigo del motor) y quedan en $SAL/gen/host-*.txt.
#
# EL DEFECTO (lo que la tarjeta viene a arreglar): el `(listfile)` de patch_d2.mpq es `Compress | Encrypted
# | FixKey` (0x80030200): el bloque esta encriptado y sus sectores son zlib. La consola lo leia con
# `zlib: invalid header`. Medido en la consola: el puente de `d2std/compress/zlib` era el de UNA SOLA VEZ
# (`d2c_zlib_inflate`, que devuelve -1 con `rc_zlib= -5`, o sea la zlib SIN ENTRADA: `avail_in` llegaba 0
# por como viajaba el slice de Go a C) y el dato estaba BIEN (los 662 bytes del sector y su crc32 son los
# del host). El arreglo es que `d2std/compress/zlib` pase al puente EN STREAMING (el mismo del gzip).
#
# CONTROLES
#  0 HOST      la sonda sobre patch_d2.mpq: Flags=0x80030200, sector 0 guardado=662 crc32=3898719478,
#              inflado=4096 crc32=2274402335, (listfile) 5502 bytes crc32=502100774 con 162 nombres.
#  1 HOST      la referencia de los OTROS DOS: d2data 466867 crc32=923032874 (11082 nombres) y d2char
#              418279 crc32=1367999602 (10009 nombres). Es contra esto que se compara la consola.
#  2 CONSOLA M1  los 662 bytes del sector llegan ENTEROS al monton de la consola (crc32 3898719478).
#  3 CONSOLA M2  el puente de UNA SOLA VEZ sigue devolviendo -1 con rc_zlib=-5. Es el defecto DECLARADO:
#              el dia que alguien lo arregle este control se pone ROJO y hay que venir a leer esta nota.
#  4 CONSOLA M3  el puente EN STREAMING da 4096 bytes crc32=2274402335 (los del host).
#  5 CONSOLA M4  `d2std/compress/zlib` (el paquete ARREGLADO) da 4096 crc32=2274402335. <-- EL ARREGLO
# 5b CONSOLA M4b el MISMO paquete con un lector de a 512 bytes: 4096 crc32=2274402335 en 8 pasos. Es el
#              unico control del avance de `z.pos` entre pasos (los demas piden de a 4096 y descomprimen todo
#              en un paso: medido, sin este control el mutante `pos-de-un-tiro` quedaba VERDE).
#  6 CONSOLA M5  los mismos 662 bytes leidos CRUDOS del disco (crc32 2166895218: el sector encriptado).
#  7 CONSOLA M6  el camino del MOTOR lee el (listfile) de patch_d2.mpq: 5502 bytes, 162 nombres,
#              crc32=502100774. Es el criterio 2 de la tarjeta, cumplido con los numeros del host.
#  8 CONSOLA M7  el control sano de la tarjeta: el DC6 con implode (SIN encriptar) sigue igual
#              (141944 bytes, suma 1831109629). Si esto cambia, el disco o el motor cambiaron.
#  9 CONSOLA M8  el defecto que QUEDA, medido: 200 vueltas del MISMO camino con los buffers en el monton
#              de Go y falla (rc_zlib=-3, Z_DATA_ERROR) con el dato de entrada intacto. No es la zlib: es
#              el monton (ver 10).
# 10 CONSOLA M10 los mismos 662 bytes y el mismo puente, pero con los buffers en .bss: 200/200 vueltas
#              OK. Aisla el defecto 9: lo que se pisa es el monton de Go, no falta memoria en el de C.
# 11 CONSOLA M6  d2data.mpq y d2char.mpq NO llegan a los numeros del host (ni 466867 ni 418279 bytes): en
#              esta camada la lectura de d2data ademas muere MUDA (sin la linea del FALLO) y la consola se
#              rearma. Es el BLOQUEO DECLARADO de la tarjeta: sin arreglar el monton (DC-002.41) no se llega
#              a los 11082/10009 nombres. Cuando eso se arregle, este control se pone ROJO y hay que mudarlo.
#
# MUTANTES (2), cada uno tiene que poner ROJO el control que le toca:
#  'avail-in-cero'  (C, csrc/zlib_dc.c)  `d2c_zs[slot].avail_in = (uInt)src_len;` -> `= 0;`: es,
#                   literalmente, el defecto medido (la zlib sin entrada). Tiene que caer el control 5.
#  'pos-de-un-tiro' (Go, compress/zlib)  `z.pos += consumido` -> `z.pos = len(z.comprimido)`: el lector se
#                   come el comprimido en el primer paso y el segundo paso se queda sin entrada. Cae el
#                   control 5b (M4b, el del lector de a 512) y NO el 5: el 5 lee de a 4096 y descomprime todo
#                   en un paso, asi que por ahi ese defecto no se ve (medido: con el control 5 solo, este
#                   mutante quedaba VERDE).
#  'paso-de-un-byte' NO sirve como mutante: pedir UN byte de destino por paso es mas lento pero da lo mismo,
#                   y quedo verde a proposito. Se dejo anotado abajo en vez de borrarlo, para que nadie lo
#                   vuelva a escribir creyendo que prueba algo.
#
# Uso: bash scripts/console/checar-compr-240.sh [dir_gox] [dir_mpq] [segundos] [modo]
#      modo = todo (por defecto) | sano | mut1 | mut2 | vuelta
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-${GODC_GOX:-/tmp/gox-240}}"
MPQ="${2:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
SEGS="${3:-90}"
# MODO: 'todo' (por defecto), 'sano', 'mut1', 'mut2' o 'vuelta'. Sirve para correr UNA parte: cada corrida
# del corpus son ~4 min de Flycast + 4 compilaciones del motor, y asi se puede re-verificar un mutante solo.
MODO="${4:-todo}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
SAL="${SAL:-/home/dgalli/.hermes/cache/scratch/corpus-240}"
POC=console/dc/poc/compr-40
BUILD=/tmp/dcbuild/compr-40
SONDA="$HERE/sonda-mpq-compr.py"
PY="${DC_PYTHON:-$HOME/dreamcast/venv/bin/python}"
[ -x "$PY" ] || PY=python3

ZLIBGO="$REPO/console/dc/std/compress/zlib/zlib.go"
ZC="$REPO/console/dc/std/csrc/zlib_dc.c"

# Los numeros del host (control 0 y 1). Son los mismos que trae el POC en sus constantes: si aca cambian,
# el control 0 lo dice antes de mirar la consola.
S0_BYTES=662; S0_CRC=3898719478
INF_BYTES=4096; INF_CRC=2274402335
LF_BYTES=5502; LF_CRC=502100774; LF_NOMBRES=162
RAW_CRC=2166895218

export GODC_GOX="$GEN"

OK=0
FALLOS=0
nota() { echo "   $*"; }
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }

# ---------------------------------------------------------------- el corpus en disco
# Los MPQ de verdad y el sector que deja el host. OJO con el nombre: en la instalacion el archivo es
# `patch_D2.mpq` (con D mayuscula) y el motor lo busca como `patch_d2.mpq`, asi que el corpus lo enlaza con
# el nombre que usa el motor. Los MPQ se enlazan (duros), no se copian: son 540 MB.
arma_corpus() {
    mkdir -p "$SAL/juego" "$SAL/gen" || return 1

    ln -f "$MPQ/patch_D2.mpq" "$SAL/juego/patch_d2.mpq" 2>/dev/null || {
        echo "falta $MPQ/patch_D2.mpq" >&2; return 1; }
    for m in d2data.mpq d2char.mpq; do
        ln -f "$MPQ/$m" "$SAL/juego/$m" 2>/dev/null || { echo "falta $MPQ/$m" >&2; return 1; }
    done

    # el host: la sonda lee los 3 MPQ crudos y deja los bytes del sector 0 (descifrado) y sus numeros.
    # Se la corre contra el CORPUS y no contra la instalacion: ahi el archivo se llama patch_d2.mpq (el
    # nombre que usa el motor) y en la instalacion es patch_D2.mpq.
    "$PY" "$SONDA" --mpqdir "$SAL/juego" --mpq patch_d2.mpq,d2data.mpq,d2char.mpq --archivo "(listfile)" \
        --extraer "$SAL/gen" > "$SAL/gen/sonda.txt" 2>&1 || return 1
    cp "$SAL/gen/patch_d2_s0.bin" "$SAL/juego/sector0_compr40.txt" || return 1
}

compilar() {
    if ! bash "$HERE/compilar-paquete-std.sh" compress/zlib "$GEN" > "$SAL/std.log" 2>&1; then
        echo "   compress/zlib no compila (ver $SAL/std.log)"; return 1
    fi
    if ! (cd "$REPO" && POC=compr-40 MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego \
            DC_MPQ_JUEGO=patch_d2.mpq,d2data.mpq,d2char.mpq \
            DC_SIN_SELLOS=1 DC_DISCO_JUEGO="$SAL/juego" \
            bash "$HERE/build-motor-arranque.sh" "$BUILD" "$GEN") > "$SAL/build.log" 2>&1; then
        echo "   el POC no enlaza (ver $SAL/build.log)"; return 1
    fi
    # guarda contra el error tonto que ya paso: sin `POC=compr-40` el script enlaza OTRO POC y la corrida
    # sale con 0 marcas COMPR-40 (y el control sano se cae sin decir por que)
    grep -qaF 'compilando el arranque (POC compr-40)' "$SAL/build.log" || {
        echo "   el build NO compilo el POC compr-40 (ver $SAL/build.log)"; return 1; }
}

correr() { # $1 = dir de la corrida
    rm -rf "$1"
    (cd "$REPO" && CAPTURE=none bash "$HERE/run-flycast.sh" "$BUILD/disc.gdi" "$SEGS" "$1") > "$1.salida" 2>&1
    [ -f "$1/flycast.log" ]
}

# ---------------------------------------------------------------- los controles
campo() { # <archivo host-x.txt> <campo>
    awk -F'\t' -v k="$2" '$1 == k {print $2}' "$1" 2>/dev/null | head -1
}

ctrl_host() { # 0: los numeros del host para patch_d2 (la sonda, no el motor)
    local h="$SAL/gen/host-patch_d2.txt"
    [ "$(campo "$h" sector0_stored_bytes)" = "$S0_BYTES" ] &&
        [ "$(campo "$h" sector0_stored_crc32)" = "$S0_CRC" ] &&
        [ "$(campo "$h" sector0_inflado_bytes)" = "$INF_BYTES" ] &&
        [ "$(campo "$h" sector0_inflado_crc32)" = "$INF_CRC" ] &&
        grep -qaF 'DESCOMPRIMIDO (zlib de Python): 5502 bytes de 5502 esperados, 0 fallos, 0 sectores crudos, 162 nombres, crc32=502100774' "$SAL/gen/sonda.txt"
}

ctrl_host3() { # 1: la referencia de los otros dos (tamano y crc32 del (listfile) entero)
    local hd="$SAL/gen/host-d2data.txt" hc="$SAL/gen/host-d2char.txt"
    [ -n "$(campo "$hd" Flags)" ] && [ -n "$(campo "$hc" Flags)" ] &&
        grep -qaF 'DESCOMPRIMIDO (zlib de Python): 466867 bytes de 466867 esperados, 0 fallos, 0 sectores crudos, 11082 nombres, crc32=923032874' "$SAL/gen/sonda.txt" &&
        grep -qaF 'DESCOMPRIMIDO (zlib de Python): 418279 bytes de 418279 esperados, 0 fallos, 0 sectores crudos, 10009 nombres, crc32=1367999602' "$SAL/gen/sonda.txt"
}

ctrl_m1() { grep -qaF "COMPR-40 M1 sector del host bytes= $S0_BYTES crc32= $S0_CRC" "$1"; }
ctrl_m2() { grep -qaF 'COMPR-40 M2 puente de una sola vez FALLO (devolvio -1 ) cota= 68180 ms= 0 rc_zlib= -5' "$1"; }
ctrl_m3() { grep -qaF "COMPR-40 M3 puente en streaming bytes= $INF_BYTES crc32= $INF_CRC" "$1"; }
ctrl_m4() { grep -qaF "COMPR-40 M4 compress/zlib bytes= $INF_BYTES crc32= $INF_CRC" "$1"; }
ctrl_m4b() { # el paquete con un lector de a 512: el UNICO control del avance de `z.pos` entre pasos
    grep -qaF "COMPR-40 M4b compress/zlib de a 512 bytes= $INF_BYTES crc32= $INF_CRC" "$1" &&
        ! grep -qaF 'COMPR-40 M4b compress/zlib de a 512 FALLO' "$1"
}
ctrl_m5() { grep -qaF "COMPR-40 M5 crudo del disco bytes= $S0_BYTES crc32= $RAW_CRC" "$1"; }
ctrl_m6() { grep -qaF "COMPR-40 M6 patch_d2.mpq (listfile) bytes= $LF_BYTES nombres= $LF_NOMBRES crc32= $LF_CRC" "$1"; }
ctrl_m7() { grep -qaF 'COMPR-40 M7 control bytes= 141944 suma= 1831109629' "$1"; }
ctrl_m8() { # el defecto que QUEDA: 200 vueltas y falla (con rc -3), con el dato de entrada intacto
    local f
    f="$(tr -d '\r' < "$1" | grep -ao 'M8 200 vueltas del camino del motor, fallos= [0-9]*' | head -1 | awk '{print $NF}')"
    [ -n "$f" ] && [ "$f" -gt 0 ] && grep -qaF 'rc_zlib= -3' "$1"
}
ctrl_m10() { grep -qaF 'COMPR-40 M10 200 vueltas con los buffers en C: vueltas_ok= 200 fallas= 0' "$1"; }
ctrl_bloqueo() { # d2data/d2char NO llegan a los numeros del host: o fallan con el error, o la corrida muere
    # ahi mismo (medido: en esta camada la lectura de d2data muere MUDA, sin la linea del FALLO, y la consola
    # se rearma). El control es el mismo en los dos casos: NO se llega al (listfile) de esos dos.
    ! grep -qaF 'COMPR-40 M6 d2data.mpq (listfile) bytes=' "$1" &&
        ! grep -qaF 'COMPR-40 M6 d2char.mpq (listfile) bytes=' "$1"
}

controles() { # $1 = log; imprime los doce y devuelve cuantos dan bien (en la ultima linea)
    local log="$1" bien=0
    if ctrl_host;     then echo "   control 0  (el host, patch_d2):          VERDE"; bien=$((bien + 1));
    else echo "   control 0  (el host, patch_d2):          ROJO"; fi
    if ctrl_host3;    then echo "   control 1  (el host, los otros dos):     VERDE"; bien=$((bien + 1));
    else echo "   control 1  (el host, los otros dos):     ROJO"; fi
    if ctrl_m1 "$log"; then echo "   control 2  (M1 los 662 bytes):           VERDE"; bien=$((bien + 1));
    else echo "   control 2  (M1 los 662 bytes):           ROJO"; fi
    if ctrl_m2 "$log"; then echo "   control 3  (M2 el defecto declarado):    VERDE"; bien=$((bien + 1));
    else echo "   control 3  (M2 el defecto declarado):    ROJO"; fi
    if ctrl_m3 "$log"; then echo "   control 4  (M3 el puente streaming):     VERDE"; bien=$((bien + 1));
    else echo "   control 4  (M3 el puente streaming):     ROJO"; fi
    if ctrl_m4 "$log"; then echo "   control 5  (M4 compress/zlib ARREGLADO): VERDE"; bien=$((bien + 1));
    else echo "   control 5  (M4 compress/zlib ARREGLADO): ROJO"; fi
    if ctrl_m4b "$log"; then echo "   control 5b (M4b el paquete de a 512):    VERDE"; bien=$((bien + 1));
    else echo "   control 5b (M4b el paquete de a 512):    ROJO"; fi
    if ctrl_m5 "$log"; then echo "   control 6  (M5 el crudo del disco):      VERDE"; bien=$((bien + 1));
    else echo "   control 6  (M5 el crudo del disco):      ROJO"; fi
    if ctrl_m6 "$log"; then echo "   control 7  (M6 el MOTOR, 162 nombres):   VERDE"; bien=$((bien + 1));
    else echo "   control 7  (M6 el MOTOR, 162 nombres):   ROJO"; fi
    if ctrl_m7 "$log"; then echo "   control 8  (M7 el control sano DC6):     VERDE"; bien=$((bien + 1));
    else echo "   control 8  (M7 el control sano DC6):     ROJO"; fi
    if ctrl_m8 "$log"; then echo "   control 9  (M8 el defecto que QUEDA):    VERDE"; bien=$((bien + 1));
    else echo "   control 9  (M8 el defecto que QUEDA):    ROJO"; fi
    if ctrl_m10 "$log"; then echo "   control 10 (M10 aislado con buffers C):  VERDE"; bien=$((bien + 1));
    else echo "   control 10 (M10 aislado con buffers C):  ROJO"; fi
    if ctrl_bloqueo "$log"; then echo "   control 11 (el bloqueo declarado d2data): VERDE"; bien=$((bien + 1));
    else echo "   control 11 (el bloqueo declarado d2data): ROJO"; fi
    echo "$bien"
}

mostrar() {
    tr -d '\r' < "$1" | grep -a 'COMPR-40' | grep -av 'M8 vuelta\|M9 vuelta\|M0 ' | sed 's/^/     /' | head -30
    echo "     (lineas COMPR-40 de la corrida: $(grep -ac 'COMPR-40' "$1"))"
}

# ---------------------------------------------------------------- mutacion de archivos
# El respaldo es una copia propia (nunca `git checkout --`): en medio de la tarjeta puede haber arreglos SIN
# COMMITEAR en esos mismos archivos. Despues de cada mutante se verifica el sha256.
mutar() { # <archivo> <buscar> <poner>
    local arch="$1" buscar="$2" poner="$3"
    "$PY" - "$arch" "$buscar" "$poner" <<'PY' || return 1
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

exportar() {
    local copia
    copia="$(mktemp /tmp/checar-compr-240-XXXXXX)"
    cp "$1" "$copia"
    echo "$copia $(sha256sum "$1" | awk '{print $1}')"
}

restaurar() {
    cp "$2" "$1"
    if [ "$(sha256sum "$1" | awk '{print $1}')" != "$3" ]; then
        echo "   EL ARBOL QUEDO MUTADO: no coincide el sha256 de $1" >&2
        return 1
    fi
    echo "   arbol restaurado ($(basename "$1"))"
}

probar_mutante() { # <nombre> <archivo> <buscar> <poner> <control que tiene que caer>
    local nombre="$1" arch="$2" buscar="$3" poner="$4" tiene_que_caer="$5"
    local run="$SAL/mut-$nombre"
    echo
    echo "== mutante '$nombre'"
    local copia="" sha=""
    read -r copia sha <<<"$(exportar "$arch")"
    if ! mutar "$arch" "$buscar" "$poner"; then
        echo "   NO SE APLICO LA MUTACION (patron sin match unico): el mutante no prueba nada" >&2
        restaurar "$arch" "$copia" "$sha" || exit 1
        rm -f "$copia"
        rojo "mutante $nombre: no se pudo mutar"
        return
    fi
    if compilar && correr "$run"; then
        controles "$run/flycast.log" | sed -n '1,13p'
        if "$tiene_que_caer" "$run/flycast.log"; then
            echo "   NO LO CAZO: el control que tenia que caer sigue VERDE con el mutante puesto" >&2
            rojo "mutante $nombre: NO cazado"
        else
            echo "   cazado: el control que tenia que caer cayo"
            verde "mutante $nombre cazado"
        fi
    else
        echo "   NO LO CAZO: con el mutante no compila/enlaza/corre (o no dejo log)" >&2
        rojo "mutante $nombre: no corrio"
    fi
    restaurar "$arch" "$copia" "$sha" || exit 1
    rm -f "$copia"
}

# ---------------------------------------------------------------- 1. el corpus y el host
echo "== 1. el corpus en disco y los numeros del HOST (la sonda, sin el motor)"
arma_corpus || { echo "no se armo el corpus" >&2; exit 1; }
ls "$SAL/juego" | tr '\n' ' '; echo
nota "sonda del host: $(grep -ac 'DESCOMPRIMIDO' "$SAL/gen/sonda.txt") archivo(s) descomprimido(s) por la zlib de Python"

# ---------------------------------------------------------------- 2. control sano
if [ "$MODO" = todo ] || [ "$MODO" = sano ]; then
echo
echo "== 2. control sano: con el arreglo puesto, los 12 controles"
compilar || exit 1
correr "$SAL/sano" || exit 1
BIEN="$(controles "$SAL/sano/flycast.log" | sed -n '$p')"
controles "$SAL/sano/flycast.log" | sed -n '1,13p'
if [ "$BIEN" -ne 13 ]; then
    echo "   EL CONTROL SANO NO DA ($BIEN/13): no se sigue, primero hay que ver por que" >&2
    mostrar "$SAL/sano/flycast.log" >&2
    exit 1
fi
echo "   OK: 13/13 controles verdes con el codigo de ahora"
mostrar "$SAL/sano/flycast.log"
fi

# ---------------------------------------------------------------- 3. los mutantes
if [ "$MODO" = todo ] || [ "$MODO" = mut1 ] || [ "$MODO" = mut2 ]; then
echo
echo "== 3. los mutantes (cada uno tiene que poner ROJO el control 5)"
if [ "$MODO" = todo ] || [ "$MODO" = mut1 ]; then
probar_mutante avail-in-cero "$ZC" \
    'd2c_zs[slot].avail_in = (uInt)src_len;' \
    'd2c_zs[slot].avail_in = 0;   /* MUTANTE avail-in-cero */' \
    ctrl_m4
fi

if [ "$MODO" = todo ] || [ "$MODO" = mut2 ]; then
probar_mutante pos-de-un-tiro "$ZLIBGO" \
    'z.pos += consumido' \
    'z.pos = len(z.comprimido)   // MUTANTE pos-de-un-tiro' \
    ctrl_m4b
fi
fi

# ---------------------------------------------------------------- 4. control sano de vuelta
if [ "$MODO" = todo ] || [ "$MODO" = vuelta ]; then
echo
echo "== 4. control sano de vuelta (el arbol entero, sin mutar)"
if compilar && correr "$SAL/vuelta"; then
    BIEN="$(controles "$SAL/vuelta/flycast.log" | sed -n '$p')"
    controles "$SAL/vuelta/flycast.log" | sed -n '1,13p'
    [ "$BIEN" -eq 13 ] || { echo "   NO VOLVIERON LOS 13 CONTROLES ($BIEN/13)" >&2; exit 1; }
    echo "   control sano VERDE"
else
    echo "   NO VOLVIO EL CONTROL SANO" >&2
    exit 1
fi
fi

echo
echo "== $OK verdes, $FALLOS fallos =="
[ "$FALLOS" -eq 0 ]
