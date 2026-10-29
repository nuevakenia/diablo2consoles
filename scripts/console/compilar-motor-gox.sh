#!/bin/bash
# DC-002.66 (Fase A): compila la LOGICA DEL JUEGO del motor para SH-4, en formato .gox.
#
# Pregunta que responde: el motor de OpenDiablo2 (2160 paquetes, actos, quests, NPC, items, skills) compila
# para el SH-4 de la Dreamcast? Si compila, el pivote del informe-continuidad es viable y el trabajo grande
# es solo el backend de render. Si no compila, hay que saber exactamente que paquete y por que ANTES de
# invertir semanas en el backend.
#
# Como funciona: no hay orden de dependencias escrito a mano (lo hay, pero cambia). Se usa punto fijo:
# se intenta compilar cada paquete; si falla SOLO porque le falta el .gox de otro paquete interno, se
# reintenta en la vuelta siguiente. Cuando una vuelta entera no agrega ningun paquete nuevo, se para.
# Lo que queda sin compilar se clasifica por la causa (ebiten = necesita backend; otra = problema real).
#
# Uso:  scripts/console/compilar-motor-gox.sh [dir_salida]
#       (por defecto /tmp/motor-gox: son .gox descartables, no van al repo)
set -uo pipefail
AQUI="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$AQUI/../.." && pwd)"
. "$AQUI/env.sh" >/dev/null 2>&1

GEN="${GODC_GOX:-$DC_ROOT/gox}"
OUT="${1:-/tmp/motor-gox}"
PAQUETES="${PAQUETES_OVERRIDE:-./d2common/... ./d2core/... ./d2game/... ./d2script/...}"
MINUTOS_POR_PAQUETE="${MINUTOS_POR_PAQUETE:-3}"

mkdir -p "$OUT"
echo "== 0/3 inventario =="
[ -d "$GEN" ] || { echo "FALTA el arbol de la stdlib en $GEN (correr scripts/console/build-std-gox.sh)"; exit 1; }
echo "   stdlib .gox: $(find "$GEN" -name '*.gox' | wc -l)"

cd "$REPO"
mapfile -t TODOS < <(go list $PAQUETES 2>/dev/null | sort)
if [ "${#TODOS[@]}" -eq 0 ]; then
    echo "go list no devolvio paquetes: revisar el arbol"
    exit 1
fi
echo "   paquetes del motor: ${#TODOS[@]}"

# ---------------------------------------------------------------------------------------------
# El .gox sale en una ruta que sigue el import path: gccgo busca "<dir>/<importpath>.gox".
# ---------------------------------------------------------------------------------------------
compilar() {
    local p="$1" dir="$2" salida="$OUT/$1.gox" err
    # ya compilado y sin fuentes mas nuevas: no recompilar (permite pasadas sucesivas)
    if [ -s "$salida" ] && [ -z "$(find "$dir" -name '*.go' -newer "$salida" -print -quit 2>/dev/null)" ]; then
        return 0
    fi
    local -a archivos=()
    # Filtrar lo que no es de esta plataforma (medido: sin esto, d2loader no compila porque junta la
    # variante host y la godc y da "redefinition of Marca", y eso corta d2asset -> d2mapentity -> d2game).
    local f
    for f in "$dir"/*.go; do
        [ -e "$f" ] || continue
        case "$f" in
            *_test.go|*_windows.go|*_darwin.go|*_js.go|*_freebsd.go|*_android.go) continue ;;
        esac
        if head -3 "$f" 2>/dev/null | grep -q 'go:build !dreamcast'; then continue; fi
        archivos+=("$f")
    done
    [ "${#archivos[@]}" -eq 0 ] && return 2
    mkdir -p "$(dirname "$salida")"
    err=$(timeout "$((MINUTOS_POR_PAQUETE*60))" sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -I "$OUT" \
            -fgo-pkgpath="$p" -c "${archivos[@]}" -o "$salida" 2>&1)
    local rc=$?
    if [ $rc -eq 0 ] && [ -s "$salida" ]; then
        return 0
    fi
    printf '%s\n' "$err"
    return 1
}

echo
echo "== 1/3 compilando por punto fijo =="
pendientes=("${TODOS[@]}")
fallos=""          # p<tab>motivo
verdes=0
for ((vuelta=1; vuelta<=12; vuelta++)); do
    nuevos=0
    siguiente=()
    for p in "${pendientes[@]}"; do
        rel="${p#github.com/OpenDiablo2/OpenDiablo2/}"
        dir="$REPO/$rel"
        if [ ! -d "$dir" ] || ! ls "$dir"/*.go >/dev/null 2>&1; then
            continue
        fi
        # los _test.go no van: el paquete de prueba importa testing y no es parte del juego
        if ! ls "$dir"/*.go 2>/dev/null | grep -qv "_test.go"; then
            continue
        fi
        if compilar "$p" "$dir" >/tmp/gox-err.$$ 2>&1; then
            verdes=$((verdes+1)); nuevos=$((nuevos+1))
            rm -f /tmp/gox-err.$$
        else
            err=$(cat /tmp/gox-err.$$ 2>/dev/null); rm -f /tmp/gox-err.$$
            # falta de un .gox interno o de la stdlib -> todavia puede entrar mas adelante
            if printf '%s' "$err" | grep -q "import file .* not found"; then
                siguiente+=("$p")
                fallos+="$p"$'\t'"$(printf '%s' "$err" | head -1 | sed 's/.*error: //')"$'\n'
            elif printf '%s' "$err" | grep -q "hajimehoshi/ebiten"; then
                fallos+="$p"$'\t'"EBITEN (necesita backend de plataforma)"$'\n'
            else
                fallos+="$p"$'\t'"$(printf '%s' "$err" | head -1 | sed 's/.*error: //')"$'\n'
            fi
        fi
    done
    echo "   vuelta $vuelta: +$nuevos compilados (total $verdes), quedan ${#siguiente[@]}"
    pendientes=("${siguiente[@]}")
    [ "$nuevos" -eq 0 ] && break
    [ "${#pendientes[@]}" -eq 0 ] && break
done

# ---------------------------------------------------------------------------------------------
echo
echo "== 2/3 resultado =="
echo "   VERDES (compilan a .gox para sh-elf): $verdes de ${#TODOS[@]}"

# clasificar lo que quedo sin compilar, por causa, contando por paquete unico
printf '%s' "$fallos" | awk -F'\t' 'NF==2 && $1!="" {print $2}' | sort | uniq -c | sort -rn | head -12 \
    | sed 's/^/   /'

echo
echo "== 3/3 lo que no compilo, por paquete =="
printf '%s' "$fallos" | awk -F'\t' 'NF==2 && $1!=""' | sort -u | awk -F'\t' '{printf "   %-70s %s\n", $1, $2}' | head -40

echo
echo "   .gox escritos en $OUT ($(du -sh "$OUT" 2>/dev/null | cut -f1))"
