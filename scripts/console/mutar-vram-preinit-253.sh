#!/bin/bash
# DC-002.56: MUTADOR del control `checar-vram-preinit-253.sh`.
#
# Un control que solo da verde no prueba nada: este script mete los DOS defectos que el control tiene que cazar
# y exige que se ponga ROJO con cada uno. Nunca toca el arbol del repo.
#
# OJO CON EL ESTADO DEL ARBOL: la limpieza que este control vigila ("vram_libre= sin-init" en las tres marcas
# previas al video) YA ATERRIZO en el POC (DC-002.57), una vez que DC-002.61 encontro que el que tumbaba
# BESTIA=0 no era el layout del binario sino la pista de audio del .gdi. El mutador igual NO usa el arbol del
# repo como referencia sana: arma una COPIA con la limpieza aplicada (la misma transformacion que aplico
# DC-002.57 a mano, las dos formas de texto) y trabaja sobre ella. Asi el mutador no depende de que nadie
# vuelva a tocar esas tres lineas: si el POC cambia, la copia se arma igual y el paso [1/4] lo delata.
#
#   [1/4] copia con la limpieza aplicada                      -> VERDE
#   [2/4] mutante de FUENTE: la lectura devuelta en MAPA-IMAGEN -> ROJO (cazado)
#   [3/4] mutante de LOG: una marca previa con `vram_libre= 0` -> ROJO (cazado)
#   [4/4] log sano: las marcas dicen `sin-init`               -> VERDE (el control no da rojo por todo)
#
# Uso:  bash scripts/console/mutar-vram-preinit-253.sh
# Sale: 0 si el control da verde sobre la copia limpia y caza los 2 mutantes; 1 si algo pasa desapercibido.
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
POC="$REPO/console/dc/poc/personaje-mapa"
CONTROL="$HERE/checar-vram-preinit-253.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

verde=0
rojo=0
fallos=0

exigir_verde() {
    local etiqueta="$1" salida="$2"
    shift 2
    if "$@" > "$salida" 2>&1; then
        echo "  $etiqueta: VERDE"
        verde=$((verde + 1))
    else
        echo "  $etiqueta: esperaba VERDE y dio ROJO"
        sed 's/^/        /' "$salida" | grep -i "rojo" | head -3
        fallos=$((fallos + 1))
    fi
}

exigir_rojo() {
    local etiqueta="$1" salida="$2"
    shift 2
    if "$@" > "$salida" 2>&1; then
        echo "  $etiqueta: NO CAZADO -- el control dio VERDE con el defecto adentro"
        fallos=$((fallos + 1))
    else
        echo "  $etiqueta: cazado  ($(grep -m1 'ROJO:' "$salida" | sed 's/^ *//'))"
        rojo=$((rojo + 1))
    fi
}

# ---- la copia con la limpieza aplicada (la misma transformacion que pide DC-002.56)
cp -a "$POC" "$TMP/poc-limpio"
# OJO: son DOS formas de texto. En MAPA-IMAGEN y PERSONAJE-IMAGEN el literal va aparte (", "vram_libre=",");
# en MPQ-CERRADOS va PEGADO a la etiqueta ("... MPQ-CERRADOS vram_libre="). Con una sola regla quedaba una de
# las tres sin limpiar y el control la cazaba (medido: es justo lo que tiene que hacer).
sed -i 's|"vram_libre=", int64(d2cVramLibre())|"vram_libre= sin-init"|g' "$TMP/poc-limpio/main.go"
sed -i 's|vram_libre=", int64(d2cVramLibre())|vram_libre= sin-init"|g' "$TMP/poc-limpio/main.go"
quedan=$(grep -c 'println.*vram_libre= sin-init' "$TMP/poc-limpio/main.go")
echo "== copia de trabajo: $TMP/poc-limpio ($quedan marcas sin-init)"
if [ "$quedan" -ne 3 ]; then
    echo "   FALLA: la limpieza no quedo aplicada en las tres marcas (cambio el texto del POC?)"
    exit 1
fi

# el arbol del repo, informativo: desde DC-002.57 deberia estar VERDE (la limpieza ya aterrizo)
if bash "$CONTROL" > "$TMP/repo.txt" 2>&1; then
    echo "== arbol del repo: VERDE (la limpieza ya aterrizo)"
else
    echo "== arbol del repo: ROJO -- alguien devolvio una lectura pre-video ($(grep -c 'ANTES del encendido' "$TMP/repo.txt") lecturas); no cuenta como fallo del mutador"
fi

# ---- [1/4] la copia limpia tiene que dar verde
exigir_verde "[1/4] copia con la limpieza aplicada" "$TMP/sano.txt" \
    env POC_DIR="$TMP/poc-limpio" bash "$CONTROL"

# ---- [2/4] mutante de FUENTE: la lectura devuelta al camino previo al video
cp -a "$TMP/poc-limpio" "$TMP/poc-mutante"
sed -i '/MAPA-IMAGEN/s|"vram_libre= sin-init")|"vram_libre=", int64(d2cVramLibre()))|' "$TMP/poc-mutante/main.go"
if grep -q 'MAPA-IMAGEN.*d2cVramLibre()' "$TMP/poc-mutante/main.go"; then
    echo "  [2/4] el mutante de fuente quedo puesto"
else
    echo "  [2/4] NO pude poner el mutante de fuente (el texto de MAPA-IMAGEN cambio de forma)"
    fallos=$((fallos + 1))
fi
exigir_rojo "[2/4] mutante de fuente (MAPA-IMAGEN pre-video)" "$TMP/mutante-fuente.txt" \
    env POC_DIR="$TMP/poc-mutante" bash "$CONTROL"

# ---- [3/4] mutante de LOG: una marca previa al video con un numero
{
    printf 'HITO238: MPQ-ABRIR OK /cd/JUEGO/D2DATA.MPQ\n'
    printf 'HITO238: MAPA-IMAGEN bytes= 1048576 vram_libre= 0\n'
    printf 'HITO238: PERSONAJE-IMAGEN bytes= 262144 vram_libre= sin-init\n'
    printf 'HITO250: MPQ-CERRADOS vram_libre= sin-init\n'
    printf 'HITO250: PVR antes-de-pvr_init_defaults\n'
} > "$TMP/log-mutante.txt"
exigir_rojo "[3/4] mutante de log (vram_libre= 0 pre-video)" "$TMP/mutante-log.txt" \
    env POC_DIR="$TMP/poc-limpio" bash "$CONTROL" "$TMP/log-mutante.txt"

# ---- [4/4] el mismo log, sano: no puede dar rojo
#       OJO con el nombre del archivo de salida: si coincide con el log que se le pasa al control, el `>` de la
#       redireccion lo trunca ANTES de leerlo y el control mira un log vacio (medido: daba "no hay marcas").
sed 's|MAPA-IMAGEN bytes= 1048576 vram_libre= 0|MAPA-IMAGEN bytes= 1048576 vram_libre= sin-init|' \
    "$TMP/log-mutante.txt" > "$TMP/log-sano.txt"
exigir_verde "[4/4] log sano (las tres marcas dicen sin-init)" "$TMP/salida-4.txt" \
    env POC_DIR="$TMP/poc-limpio" bash "$CONTROL" "$TMP/log-sano.txt"

echo
echo "== resumen: verdes=$verde rojos=$rojo fallos=$fallos"
if [ "$fallos" -eq 0 ] && [ "$rojo" -eq 2 ] && [ "$verde" -eq 2 ]; then
    echo "== MUTADOR OK: el control caza $rojo/$rojo defectos y no da rojo sobre la copia sana"
    exit 0
fi
echo "== MUTADOR FALLA: algun mutante paso desapercibido o la copia sana no dio verde"
exit 1
