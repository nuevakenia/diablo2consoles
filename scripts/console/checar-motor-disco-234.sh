#!/bin/bash
# CORPUS DE DC-002.34: HASTA DONDE LLEGA EL MOTOR COMPLETO CON LOS ASSETS DEL DISCO.
#
# QUE MIDE (criterio 1 de la tarjeta): el arranque del MOTOR ENTERO (POC `motor-arranque`, que llama al
# entry point de verdad: d2app.Create() + Run()) con los TRES MPQ del disco horneados en la ISO (/JUEGO, el
# camino de DC-002.27), corrido en Flycast con la consola serie capturada, en los TRES estados que hacen falta
# para ver el efecto de cada palanca POR SEPARADO:
#
#   BASE   las tablas del MPQ como OBJETOS de Go (el "antes" de DC-002.33) y la tabla de d2records ADENTRO
#          del binario (el "antes" de DC-002.32);
#   S233   la palanca de DC-002.33 (las tablas como BYTES contiguos) -- el arbol de HEAD tal cual;
#   S232   la palanca de DC-002.32 (la tabla de d2records FUERA del binario) + DC-002.33.
#
# Y el MUTANTE del criterio 3: la misma imagen SIN la carpeta /JUEGO -> tiene que volver el cartel del motor.
#
# LA PANTALLA (criterio 2) se mide con `captura-ventana-234.sh`: captura LA VENTANA del emulador (682x453 en
# este Flycast, NO el recorte fijo 210,210 640x480 de DC-002.35 que medía el escritorio) y clasifica cada
# cuadro por su color dominante. Ademas cuenta los cuadros que dibujo el motor (EBITEN-PERFIL: 0 = murio
# antes del bucle).
#
# Se trabaja sobre una COPIA de las fuentes ($SRC): las dos palancas se aplican mutando el arbol, y el
# checkout principal lo puede estar usando otro agente del tablero.
#
# Uso:  bash scripts/console/checar-motor-disco-234.sh [--refrescar] [segundos_por_corrida]
#   --refrescar   rehace las capas de .gox y las cuatro imagenes desde cero (si no, lo que ya esta en disco
#                 se reusa: los builds son de minutos y las imagenes de 553 MB)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SRC="${SRC:-/home/dgalli/.hermes/cache/scratch/dc234-src}"
SAL="${SAL:-/home/dgalli/.hermes/cache/scratch/corpus-234}"
MPQ="${MPQ:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
GEN_HEAD="${GEN_HEAD:-/tmp/gox-233}"     # la capa coherente con HEAD (control 3 la verifica)
GOX_BASE="$SAL/gox-base"
GOX_S232="$SAL/gox-s232"
GOX_S232SOLO="$SAL/gox-s232solo"
LISTA="patch_d2.mpq,d2data.mpq,d2char.mpq"
SEGS="${2:-25}"
REFRESCAR=0
[ "${1:-}" = "--refrescar" ] && REFRESCAR=1

OK=0
FALLOS=0
nota() { echo "   $*"; }
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }
paso() { printf '\n== %s\n' "$*"; }

mkdir -p "$SAL/juego"

# ================================================================================================
# 0. LA CARPETA QUE SE HORNEA Y LOS CONTROLES DE PROCEDENCIA
# ================================================================================================
paso "0. la carpeta del juego (enlaces duros: no copia 540 MB) y el estado de las fuentes"
for n in patch_d2.mpq d2data.mpq d2char.mpq; do
    real="$(ls -1 "$MPQ" | grep -ix "$n" | head -1)"
    [ -n "$real" ] || { echo "no encuentro $n en $MPQ"; exit 1; }
    ln -f "$MPQ/$real" "$SAL/juego/$n" 2>/dev/null || cp "$MPQ/$real" "$SAL/juego/$n"
done
nota "MPQ horneados: $(ls "$SAL/juego" | tr '\n' ' ') ($(du -sb "$SAL/juego" | cut -f1) bytes)"

if [ "$(cd "$REPO" && "$DC_PYTHON" scripts/console/fix-mpq-tablas-233.py --estado | tail -1 | grep -c '7/7 como el arreglo')" = "1" ]; then
    verde "control 0: el arbol del checkout esta como el arreglo de DC-002.33 (7/7)"
else
    rojo "control 0: el arbol NO esta como el arreglo (correr fix-mpq-tablas-233.py --sano)"
fi

nota "libgodc: $(stat -c '%y %s bytes' "$LIBGODC/libgodc.a") sha256 $(sha256sum "$LIBGODC/libgodc.a" | cut -d' ' -f1)"

# control 3: la capa de .gox es la de este arbol (recompilar d2mpq tiene que dar el MISMO archivo)
cp "$GEN_HEAD/github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq.gox" "$SAL/d2mpq-antes.gox" 2>/dev/null
if (cd "$REPO" && bash scripts/console/recompilar-paquete-motor.sh d2common/d2fileformats/d2mpq "$GEN_HEAD") > "$SAL/recompilar-d2mpq.log" 2>&1 &&
   cmp -s "$SAL/d2mpq-antes.gox" "$GEN_HEAD/github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq.gox"; then
    verde "control 3: la capa $GEN_HEAD es la de este arbol (d2mpq.gox byte a byte igual al recompilarlo)"
else
    rojo "control 3: d2mpq.gox de $GEN_HEAD NO coincide con el recompilado (ver $SAL/recompilar-d2mpq.log)"
fi

# --- la copia de fuentes ---
if [ "$REFRESCAR" = 1 ] || [ ! -d "$SRC/d2core" ]; then
    paso "copia de fuentes: $REPO -> $SRC (rsync, sin .git)"
    mkdir -p "$SRC"
    rsync -a --exclude '.git' "$REPO/" "$SRC/"
fi

# ================================================================================================
# 1. LAS CUATRO IMAGENES (los tres estados y el mutante)
# ================================================================================================
revisa_rutas() { # $1 = capa de .gox: no puede quedar ni una ruta absoluta vieja /tmp/gox-* adentro
    local n
    n=$(grep -c '/tmp/gox' "$1/orden-motor.txt" 2>/dev/null || true)
    if [ "${n:-0}" != "0" ]; then
        echo "   FALLO: $1 conserva $n rutas /tmp/gox en orden-motor.txt (correr reescribe-rutas-gox.py)"
        return 1
    fi
    return 0
}

armar() { # $1 = nombre (dir), $2 = dir de .gox, $3 = "sin-juego" para el mutante
    local dir="$SAL/$1"
    [ "$REFRESCAR" = 0 ] && [ -f "$dir/1ST_READ.BIN" ] && { nota "ya construido: $1"; return 0; }
    revisa_rutas "$2" || exit 1
    local -a extra=()
    [ "${3:-}" = "sin-juego" ] && extra=(env DC_SIN_JUEGO=1)
    # DC-002.41: el mutante se pasa con `env`, NO como palabra expandida. Con `extra="DC_SIN_JUEGO=1"` y
    # `... $extra bash ...` (o el arreglo expandido entre comillas) bash NO reconoce la asignacion -- una
    # palabra que sale de una expansion no es una asignacion -- e intenta ejecutar el comando
    # `DC_SIN_JUEGO=1`: el build del mutante muere con "orden no encontrada". No se habia visto porque en
    # DC-002.34 la imagen del mutante ya estaba construida en $SAL y `armar` la reusaba; con un $SAL nuevo se cae.
    ( cd "$SRC" && MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO="$LISTA" DC_SIN_SELLOS=1 \
        DC_DISCO_JUEGO="$SAL/juego" POC=motor-arranque "${extra[@]}" \
        bash scripts/console/build-motor-arranque.sh "$dir" "$2" ) > "$dir.build.log" 2>&1 ||
        { echo "   NO COMPILA/ARMA $1 (ver $1.build.log)"; exit 1; }
    nota "$1: 1ST_READ.BIN $(stat -c%s "$dir/1ST_READ.BIN") bytes, ISO $(stat -c%s "$dir/disc.iso") bytes"
}

capa() { # $1 = dir destino de la capa, $2 = paquete a recompilar (vacio = ninguna), $3 = "pristino"
    if [ "$REFRESCAR" = 0 ] && [ -f "$1/orden-motor.txt" ]; then nota "capa ya copiada: $1"; return 0; fi
    rm -rf "$1"; cp -a "$GEN_HEAD" "$1"
    # los orden*.txt guardan RUTAS ABSOLUTAS: sin reescribirlas el enlace mete los DOS juegos de .gox y el
    # linker muere con "multiple definition of ..._d2mpq.New..f" (medido)
    python3 "$HERE/reescribe-rutas-gox.py" "$GEN_HEAD" "$1" "$1"
    [ -n "${2:-}" ] || return 0
    ( cd "$SRC" && bash scripts/console/recompilar-paquete-motor.sh "$2" "$1" )
}

paso "1a. BASE: las tablas del MPQ como OBJETOS (el antes de DC-002.33)"
( cd "$SRC" && "$DC_PYTHON" scripts/console/fix-mpq-tablas-233.py --estado )
( cd "$SRC" && "$DC_PYTHON" scripts/console/fix-mpq-tablas-233.py --pristino ) || exit 1
capa "$GOX_BASE" d2common/d2fileformats/d2mpq
armar base-con-juego "$GOX_BASE"
( cd "$SRC" && "$DC_PYTHON" scripts/console/fix-mpq-tablas-233.py --sano )

paso "1b. S233: el arbol de HEAD (la palanca de DC-002.33 adentro)"
armar s233-con-juego "$GEN_HEAD"

paso "1c. S232: la tabla de d2records FUERA (la palanca de DC-002.32)"
( cd "$SRC" && python3 scripts/console/fix-d2records-tabla-232.py --fuera ) || exit 1
capa "$GOX_S232" d2core/d2records
armar s232-con-juego "$GOX_S232"
( cd "$SRC" && python3 scripts/console/fix-d2records-tabla-232.py --dentro )

paso "1d. S232SOLO: SOLO la palanca de DC-002.32 (la tabla afuera) y las tablas del MPQ como OBJETOS"
# Este estado es el que pide el criterio 1 al pie de la letra: el efecto de la palanca de DC-002.32 por
# separado, sin la de DC-002.33 debajo. Lo que se espera es que la pared del semiespacio (la de BASE) siga
# matando antes de que la RAM ganada por la tabla llegue a servir.
if [ "$REFRESCAR" = 1 ] || [ ! -f "$GOX_S232SOLO/orden-motor.txt" ]; then
    rm -rf "$GOX_S232SOLO"; cp -a "$GOX_S232" "$GOX_S232SOLO"
    python3 "$HERE/reescribe-rutas-gox.py" "$GOX_S232" "$GOX_S232SOLO" "$GOX_S232SOLO" || exit 1
fi
( cd "$SRC" && "$DC_PYTHON" scripts/console/fix-mpq-tablas-233.py --pristino ) || exit 1
capa_solo() { ( cd "$SRC" && bash scripts/console/recompilar-paquete-motor.sh d2common/d2fileformats/d2mpq "$GOX_S232SOLO" ); }
capa_solo || exit 1
armar s232solo-con-juego "$GOX_S232SOLO"
( cd "$SRC" && "$DC_PYTHON" scripts/console/fix-mpq-tablas-233.py --sano )
( cd "$SRC" && python3 scripts/console/fix-d2records-tabla-232.py --dentro )

paso "1e. MUTANTE: la misma imagen SIN la carpeta /JUEGO"
armar sin-juego "$GEN_HEAD" sin-juego

# ================================================================================================
# 2. LOS CONTROLES DE LA IMAGEN (el MPQ que viaja adentro es el del PC; el mutante no lo tiene)
# ================================================================================================
paso "2. controles de la ISO"
for n in PATCH_D2 D2DATA D2CHAR; do
    dentro=$("$DC_PYTHON" "$HERE/leer-iso.py" "$SAL/s233-con-juego/disc.iso" "/JUEGO/$n.MPQ;1" | grep -o 'sha256=[0-9a-f]*' | cut -d= -f2)
    case "$n" in PATCH_D2) pc=patch_D2.mpq ;; D2DATA) pc=d2data.mpq ;; D2CHAR) pc=d2char.mpq ;; esac
    fuera=$(sha256sum "$MPQ/$pc" | cut -d' ' -f1)
    if [ -n "$dentro" ] && [ "$dentro" = "$fuera" ]; then
        verde "control 1: $n dentro de la ISO = el del PC (${dentro:0:16}...)"
    else
        rojo "control 1: $n dentro=$dentro PC=$fuera"
    fi
done
# OJO: leer-iso.py sale con 1 cuando no encuentra la ruta (es su control), y con `set -o pipefail` el
# `... | grep -q FALLO` hereda ese 1 y el `if` se va por la rama del fallo. Por eso se mira la SALIDA.
sal_mutante=$("$DC_PYTHON" "$HERE/leer-iso.py" "$SAL/sin-juego/disc.iso" "/JUEGO/PATCH_D2.MPQ;1" 2>&1)
case "$sal_mutante" in
    *FALLO*) verde "control 2: la ISO del mutante NO tiene /JUEGO (leer-iso contesta FALLO)" ;;
    *)       rojo  "control 2: la ISO del mutante TIENE /JUEGO (el mutante no discrimina): $sal_mutante" ;;
esac
if (cd "$SRC" && go run ./console/dc/gen-config-consola -datos "$SAL/juego" \
        -config "$SAL/s233-con-juego/romdisk/OpenDiablo2/config.json" -ruta-consola /cd/juego \
        -mpq "$LISTA" -verificar) > "$SAL/config-verif.log" 2>&1; then
    verde "control 4: la config del arranque dice /cd/juego con los 3 nombres"
else
    rojo "control 4: la config del arranque no cierra (ver config-verif.log)"
fi

# ================================================================================================
# 3. LAS CUATRO CORRIDAS (consola serie + capturas de la VENTANA del emulador)
# ================================================================================================
corrida() { # $1 = nombre, $2 = segundos
    paso "3. corrida $1"
    bash "$HERE/captura-ventana-234.sh" "$SAL/$1/disc.gdi" "${2:-$SEGS}" "$SAL/run-$1"
}

corrida base-con-juego
corrida s233-con-juego
corrida s232-con-juego
corrida s232solo-con-juego
corrida sin-juego

# ================================================================================================
# 4. LA TABLA (criterio 1) Y LOS CONTROLES DE LA PANTALLA Y DEL MUTANTE (criterios 2 y 3)
# ================================================================================================
paso "4. la tabla de los cinco estados"
"$DC_PYTHON" "$HERE/tabla-estados-234.py" \
    "$SAL/run-base-con-juego/flycast.log" "$SAL/run-s233-con-juego/flycast.log" \
    "$SAL/run-s232-con-juego/flycast.log" "$SAL/run-s232solo-con-juego/flycast.log" \
    "$SAL/run-sin-juego/flycast.log" | tee "$SAL/tabla.txt"

paso "5. la pantalla, con numeros (criterio 2)"
for e in base-con-juego s233-con-juego s232-con-juego s232solo-con-juego sin-juego; do
    n_negras=$("$DC_PYTHON" "$HERE/clasifica-captura-234.py" "$SAL/run-$e"/win-*s.png 2>/dev/null | grep -c 'negra')
    n_total=$(ls "$SAL/run-$e"/win-*s.png 2>/dev/null | wc -l)
    nota "$e: $n_total capturas, $n_negras con la pantalla del motor en negro"
done

paso "6. el mutante (criterio 3)"
n_cartel=$(tr -d '\r' < "$SAL/run-sin-juego/flycast.log" | grep -ac 'game files not found at "/cd/juego"')
n_con=$(cat "$SAL"/run-base-con-juego/flycast.log "$SAL"/run-s233-con-juego/flycast.log "$SAL"/run-s232-con-juego/flycast.log |
    tr -d '\r' | grep -ac 'game files not found at "/cd/juego"')
if [ "$n_cartel" -ge 1 ] && [ "$n_con" -eq 0 ]; then
    verde "control 6: sin los MPQ en la ISO vuelve el cartel ($n_cartel veces) y con ellos no aparece ($n_con)"
else
    rojo "control 6: cartel sin MPQ=$n_cartel con MPQ=$n_con (el mutante no discrimina)"
fi

echo
echo "== RESULTADO: $OK controles OK, $FALLOS con fallo =="
[ "$FALLOS" -eq 0 ]
