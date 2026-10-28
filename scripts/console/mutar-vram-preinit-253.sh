#!/bin/bash
# DC-002.56: MUTADOR del control `checar-vram-preinit-253.sh`.
#
# Un control que solo da verde no prueba nada: este script mete los DOS defectos que el control tiene que cazar
# y exige que se ponga ROJO con cada uno. Nunca toca el arbol del repo: el mutante de fuente vive en una COPIA
# del POC (se le pasa por POC_DIR, igual que las otras herramientas de la casa) y el mutante de log es un
# flycast.log sintetico en /tmp.
#
#   [1/4] arbol del repo                                      -> VERDE
#   [2/4] mutante de FUENTE: la lectura devuelta en MAPA-IMAGEN -> ROJO (cazado)
#          (es exactamente el defecto que la tarjeta saca: `vram_libre=`, int64(d2cVramLibre()) antes del video)
#   [3/4] mutante de LOG: una marca previa con `vram_libre= 0`  -> ROJO (cazado)
#   [4/4] log sano: las marcas dicen `sin-init`                 -> VERDE (el control no da rojo por todo)
#
# Uso:  bash scripts/console/mutar-vram-preinit-253.sh
# Sale: 0 si cazo todos los mutantes y el arbol sano dio verde; 1 si alguno paso desapercibido.
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

# ---- [1/4] el arbol del repo tiene que dar verde
exigir_verde "[1/4] arbol del repo" "$TMP/sano.txt" bash "$CONTROL"

# ---- [2/4] mutante de FUENTE: la lectura devuelta al camino previo al video
cp -a "$POC" "$TMP/poc-mutante"
sed -i '/MAPA-IMAGEN/s|"vram_libre= sin-init")|"vram_libre=", int64(d2cVramLibre()))|' "$TMP/poc-mutante/main.go"
if grep -q 'MAPA-IMAGEN.*d2cVramLibre()' "$TMP/poc-mutante/main.go"; then
    echo "  [2/4] el mutante de fuente quedo puesto ($(grep -c 'd2cVramLibre()' "$TMP/poc-mutante/main.go") lecturas en total)"
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
    printf 'HITO250: VRAM-POSTINIT vram_libre= 8123456\n'
} > "$TMP/log-mutante.txt"
exigir_rojo "[3/4] mutante de log (vram_libre= 0 pre-video)" "$TMP/mutante-log.txt" \
    bash "$CONTROL" "$TMP/log-mutante.txt"

# ---- [4/4] el mismo log, sano: no puede dar rojo
#       OJO con el nombre del archivo de salida: si coincide con el log que se le pasa al control, el `>` de la
#       redireccion lo trunca ANTES de leerlo y el control mira un log vacio (medido: daba "no hay marcas").
sed 's|MAPA-IMAGEN bytes= 1048576 vram_libre= 0|MAPA-IMAGEN bytes= 1048576 vram_libre= sin-init|' \
    "$TMP/log-mutante.txt" > "$TMP/log-sano.txt"
exigir_verde "[4/4] log sano (las tres marcas dicen sin-init)" "$TMP/salida-4.txt" \
    bash "$CONTROL" "$TMP/log-sano.txt"

echo
echo "== resumen: verdes=$verde rojos=$rojo fallos=$fallos"
if [ "$fallos" -eq 0 ] && [ "$rojo" -eq 2 ]; then
    echo "== MUTADOR OK: el control caza $rojo/$rojo defectos y no da rojo sobre el arbol sano"
    exit 0
fi
echo "== MUTADOR FALLA: algun mutante paso desapercibido o el arbol sano no dio verde"
exit 1
