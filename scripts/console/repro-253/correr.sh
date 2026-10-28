#!/bin/bash
# Escalera de reproduccion de DC-002.53 (2026-10-25).
#
# PARA QUE: separar de una vez "KOS/Flycast roto", "Go/libgodc roto", "el POC hace algo antes del video
# que rompe pvr_init_defaults" y "el tamano/forma del binario del POC". Cada eslabon es un programa chiquito
# que se compila y se corre en Flycast en ~3-4 minutos, en vez de los 15 de un build del POC entero.
#
# USO:
#   scripts/console/repro-253/correr.sh            # corre TODOS los eslabones
#   scripts/console/repro-253/correr.sh kos        # solo el de KOS pelado
#   scripts/console/repro-253/correr.sh minimo     # solo el Go minimo
#   scripts/console/repro-253/correr.sh churn      # solo el Go con churn de monton
#   scripts/console/repro-253/correr.sh nivel      # solo el del NIVEL (mpq + DS1 + PVR al final)
#
# RESULTADO MEDIDO EL 2026-10-25 (con KOS y libgodc pristinos, Flycast):
#   kos    -> arranca, pvr_init_defaults OK, 64 KB de VRAM, 60 escenas, sale limpio ("arch: shutting down")
#   minimo -> arranca, pvr_init_defaults OK, 60 pintadas por el shim del POC, sale limpio
#   churn  -> igual que minimo pero antes pide 1 MB, 40 temporales de 96 KB y 2 GC: TAMBIEN anda
# Los tres pasan. O sea: ni KOS, ni Flycast, ni libgodc, ni el estado del monton. El problema esta en lo que
# el POC hace antes de encender el video, o en el tamano/forma de SU binario (ver evidencia dc-002.53).
#
# RESULTADO MEDIDO EL 2026-10-27 (eslabon nuevo, el del NIVEL):
#   nivel  -> PASA Y PINTA. Las cinco marcas del POC con los MISMOS numeros (DS1 act=1 w=57 h=41 objetos=41,
#             COMPUESTO pisos=1982/1982 paredes=330/330, LIENZO pixelesNoVacios=193538 de 262144) y despues el
#             PVR arranca y pinta 60 cuadros (dibujando=1) + latidos. Veredicto medido sobre la captura:
#             area de video del emulador 54,4% de pixeles no negros. Detalle en
#             console/dc/evidencia/dc-002.58-escalon-nivel.txt
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
. "$REPO/scripts/console/env.sh"

QUE="${1:-todos}"

kos() {
    local out=/tmp/repro253-kos
    rm -rf "$out"; mkdir -p "$out"
    kos-cc -c "$HERE/kos-pelado/main.c" -o "$out/main.o" || return 1
    kos-cc -o "$out/prog.elf" "$out/main.o" $DC_LDPATHS -lkos $DC_LIBS_POST || return 1
    empaquetar "$out"
    correr "$out"
}

minimo() {
    local out=/tmp/repro253-minimo
    rm -rf "$out"
    SALTAR_STDLIB=1 bash "$REPO/scripts/console/build-dc.sh" "$HERE/go-minimo" "$out" >/dev/null 2>&1 || { echo "FALLO el build"; return 1; }
    correr "$out"
}

churn() {
    local out=/tmp/repro253-churn
    rm -rf "$out"
    SALTAR_STDLIB=1 bash "$REPO/scripts/console/build-dc.sh" "$HERE/go-churn" "$out" >/dev/null 2>&1 || { echo "FALLO el build"; return 1; }
    correr "$out"
}

# nivel (DC-002.58): el eslabon del TRABAJO PREVIO AL VIDEO. Abre el d2data.mpq del disco, lee el DS1 del acto
# 1, compone el nivel con el compositor del POC (`nivel.go`, copiado TAL CUAL) y recien despues enciende el PVR
# y pinta. Necesita el MPQ en la ISO (los otros eslabones no leen el disco) y una corrida LARGA: el MPQ tarda
# ~10 s en abrir y el nivel ~20 s en componerse, asi que el video arranca bien despues del minuto.
nivel() {
    local out=/tmp/repro253-nivel
    rm -rf "$out"; mkdir -p "$out"

    # el compositor del eslabon TIENE que ser el mismo archivo que compila el POC: si no, los numeros del
    # LIENZO no son comparables y el eslabon no mide el mismo camino (misma guarda que build-monstruo-250.sh).
    if ! cmp -s "$HERE/go-nivel/nivel.go" "$REPO/console/dc/poc/personaje-mapa/nivel.go"; then
        echo "FALLO: go-nivel/nivel.go NO es el mismo archivo que el del POC"
        return 1
    fi
    echo "   nivel.go del eslabon == nivel.go del POC ($(sha256sum "$HERE/go-nivel/nivel.go" | cut -c1-16))"

    echo "== build del eslabon"
    SALTAR_STDLIB=1 bash "$REPO/scripts/console/build-dc.sh" "$HERE/go-nivel" "$out" > "$out/build.log" 2>&1 || {
        echo "FALLO el build (ver $out/build.log)"; tail -8 "$out/build.log"; return 1; }
    tail -4 "$out/build.log"

    # la ISO con el D2DATA.MPQ en /JUEGO (la MISMA ruta que usa el POC: /cd/JUEGO/D2DATA.MPQ)
    local D2DIR="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
    local MPQ_DATA
    MPQ_DATA="$(ls "$D2DIR" 2>/dev/null | grep -ix 'd2data.mpq' | head -1)"
    [ -n "$MPQ_DATA" ] || { echo "no encuentro d2data.mpq en $D2DIR"; return 1; }
    printf '%s:/JUEGO/D2DATA.MPQ;1\n' "$D2DIR/$MPQ_DATA" > "$out/iso-lista.txt"
    echo "== ISO con $(stat -c%s "$D2DIR/$MPQ_DATA") bytes de d2data.mpq en /JUEGO"
    MKISO_JOLIET=0 "$DC_PYTHON" "$REPO/scripts/console/mkiso.py" "$out/disc.iso" "$out/IP.BIN" \
        "$out/1ST_READ.BIN" "@$out/iso-lista.txt" | tail -2
    "$DC_PYTHON" "$REPO/scripts/console/mkgdi.py" "$out" | tail -2

    # --- la corrida se dirige por MARCAS, no por reloj: se espera "CUADRO n= 60" (los 60 cuadros del criterio)
    # y recien ahi se captura. Con capturas por tiempo (run-flycast.sh) la ultima cae antes de que el nivel
    # este compuesto y no hay nada que medir.
    local log="$out/corrida/flycast.log"
    local FLY="${FLYCAST:-$HOME/dreamcast/dl/flycast.AppImage}"
    local ESPERA="${ESPERA_NIVEL:-300}"
    mkdir -p "$out/corrida"
    rm -f "$out"/corrida/*.png
    echo "== Flycast (esperando las marcas, hasta ${ESPERA}s)"
    bash "$REPO/scripts/console/flycast-adelante.sh" arriba
    setsid env DISPLAY="${DISPLAY:-:0}" LIBGL_ALWAYS_SOFTWARE=1 "$FLY" \
        -config "window:fullscreen=no" -config "Debug:SerialConsoleEnabled=true" \
        "$out/disc.gdi" > "$log" 2>&1 &
    local PID=$!

    local t=0 listo=0
    while [ "$t" -lt "$ESPERA" ]; do
        if grep -qa "HITO258: CUADRO n= 60 " "$log" 2>/dev/null; then listo=1; break; fi
        if ! kill -0 "$PID" 2>/dev/null; then echo "  el emulador se detuvo a los ${t}s"; break; fi
        sleep 2; t=$(( t + 2 ))
    done
    echo "  marcas: $(grep -ac 'HITO258: CUADRO' "$log" 2>/dev/null) cuadros marcados, 60 llegado=$listo a los ${t}s"

    # EL VEREDICTO: la captura es de la pantalla entera (spectacle -f) y se mide SOLO el area de video del
    # emulador. La geometria la da el gestor de ventanas, no un recorte fijo (medido: un recorte fijo en
    # 210,210 mide el fondo de pantalla en las 11 capturas, con el nivel pintado en otra parte).
    local geo cli
    geo="$(bash "$REPO/scripts/console/flycast-adelante.sh" geo)"
    echo "  ventana: ${geo:-NO la encuentro (sin KWin no hay geometria)}"
    cli="$(printf '%s' "$geo" | sed -n 's/.*cli=\([0-9-]*,[0-9-]*,[0-9]*,[0-9]*\).*/\1/p')"
    sleep 1
    spectacle -b -n -f -o "$out/corrida/nivel-258.png" >/dev/null 2>&1
    if [ -n "$cli" ]; then
        python3 "$HERE/pixeles-258.py" "$out/corrida/nivel-258.png" "$cli" | tee "$out/corrida/pixeles.txt"
        python3 - "$out/corrida/nivel-258.png" "$cli" "$out/corrida/nivel-258-recorte.png" <<'PYFIN'
import sys

from PIL import Image

x, y, w, h = (int(v) for v in sys.argv[2].split(","))
area = Image.open(sys.argv[1]).convert("RGB").crop((x, y, x + w, y + h))
area.save(sys.argv[3])
# el emulador de este host muestra el area de video a 320x240: se guarda tambien al doble (nearest, sin
# interpolar) para que el nivel se vea en la evidencia. El conteo de pixeles se hace sobre el recorte 1:1.
area.resize((w * 2, h * 2), Image.NEAREST).save(sys.argv[3].replace(".png", "-x2.png"))
print("RECORTE:", sys.argv[3], "%dx%d (+ x2)" % (w, h))
PYFIN
    else
        echo "VEREDICTO: sin geometria de la ventana no hay captura que medir (flycast-adelante.sh geo)"
    fi

    kill -TERM -"$PID" 2>/dev/null || kill -TERM "$PID" 2>/dev/null
    sleep 1
    kill -KILL -"$PID" 2>/dev/null
    bash "$REPO/scripts/console/flycast-adelante.sh" sacar

    echo
    echo "--- $log (las marcas del eslabon) ---"
    tr -d '\r' < "$log" | grep -a "HITO258\|HITO250\|Unhandled\|Fatal" | grep -av "LATIDO\|CUADRO n= "
    tr -d '\r' < "$log" | grep -a "HITO258: CUADRO n= 1 \|HITO258: CUADRO n= 60 "
    echo "--- latidos: $(tr -d '\r' < "$log" | grep -ac 'HITO258: LATIDO')"
}

empaquetar() {
    local out=$1
    sh-elf-objcopy -R .stack -O binary "$out/prog.elf" "$out/1ST_READ.BIN"
    "$DC_MAKEIP" -f "$out/IP.BIN" >/dev/null
    "$DC_PYTHON" "$REPO/scripts/console/mkiso.py" "$out/disc.iso" "$out/IP.BIN" "$out/1ST_READ.BIN"
    "$DC_PYTHON" "$REPO/scripts/console/mkgdi.py" "$out"
}

correr() {
    local out=$1
    LIBGL_ALWAYS_SOFTWARE=1 CAPTURAS=1 INTERVALO=1.0 ESPERA_ARRANQUE=80 \
        bash "$REPO/scripts/console/run-flycast-249.sh" "$out/disc.gdi" "$out/corrida" >/dev/null 2>&1
    echo "--- $out ---"
    grep -a "PRUEBA\|MINIMO\|CHURN\|HITO250\|Unhandled" "$out/corrida/flycast.log" 2>/dev/null | tr -d '\r' | head -12
}

case "$QUE" in
    kos)    kos ;;
    minimo) minimo ;;
    churn)  churn ;;
    nivel)  nivel ;;
    todos)  kos; minimo; churn; nivel ;;
    *) echo "uso: $0 [kos|minimo|churn|nivel|todos]"; exit 2 ;;
esac
