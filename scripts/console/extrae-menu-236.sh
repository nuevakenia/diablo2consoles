#!/bin/bash
# DC-002.36: EXTRAER los assets del MENU PRINCIPAL y MEDIRLOS (que MPQ los tiene, tamano, frames, lienzo).
#
# El motor ya trae el menu escrito (d2game/d2gamescreen/main_menu.go): lo que dibuja y donde esta medido
# de su propio codigo (constantes backgroundX/Y, diabloLogoX/Y, singlePlayerBtnX/Y, ...). Este script no
# adivina nada: para cada asset prueba los MPQ del juego EN ORDEN, se queda con el que lo tiene, lo deja
# crudo en $OUT/MENU y despues lo mide con el mismo verificador que usa DC-002.35 (verif-dc6-235).
#
# Uso:  extrae-menu-236.sh [dir_salida]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
OUT="${1:-$HOME/.hermes/cache/scratch/dc35/menu-236}"
D2DIR="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
PY="${DC_PYTHON:-$HOME/dreamcast/venv/bin/python}"

mkdir -p "$OUT/MENU" "$OUT/host"

# Los MPQ donde pueden vivir los assets de UI, en orden de probabilidad. MEDIDO: el FONDO del menu vive
# en d2exp.mpq (no en d2data.mpq), y los botones/logos/fuentes en d2data.mpq.
MPQS="d2data.mpq d2exp.mpq patch_D2.mpq d2char.mpq d2Sfx.mpq"

# nombre_logico|ruta_dentro_del_MPQ|paleta (la del codigo del motor)
ASSETS="
fondo|data/global/ui/FrontEnd/gameselectscreenEXP.dc6|data/global/palette/sky/pal.dat
logo_negro|data/global/ui/FrontEnd/D2logoBlackLeft.DC6|data/global/palette/units/pal.dat
logo_fuego|data/global/ui/FrontEnd/D2logoFireLeft.DC6|data/global/palette/units/pal.dat
boton_ancho|data/global/ui/FrontEnd/WideButtonBlank.dc6|data/global/palette/units/pal.dat
boton_corto|data/global/ui/CharSelect/ShortButtonBlank.dc6|data/global/palette/units/pal.dat
boton_medio|data/global/ui/FrontEnd/MediumButtonBlank.dc6|data/global/palette/units/pal.dat
fuente_exocet|data/local/FONT/LATIN/fontexocet10.dc6|data/global/palette/units/pal.dat
"

echo "== DC-002.36: assets del MENU PRINCIPAL (extraidos del MPQ del juego y medidos)"
printf '%-14s %-52s %-18s %s\n' ASSET ARCHIVO "MPQ" "BYTES"

while IFS='|' read -r nombre ruta paleta; do
    [ -z "${nombre:-}" ] && continue
    # el motor arma las rutas con '/' inicial y las pasa al MPQ con '\' (asi las guarda el MPQ)
    ruta_mpq="$(printf '%s' "$ruta" | sed 's|/|\\|g')"
    hallado=""
    for m in $MPQS; do
        real="$(ls -1 "$D2DIR" | grep -ix "$m" | head -1)"
        [ -n "$real" ] || continue
        if ( cd "$REPO" && go run ./console/dc/bake-dc6 -mpq "$D2DIR/$real" -dc6 "$ruta_mpq" \
                -out "$OUT/host/$nombre" -name "$nombre" >/dev/null 2>&1 ); then
            hallado="$real"; break
        fi
    done
    if [ -z "$hallado" ]; then
        printf '%-14s %-52s %-18s %s\n' "$nombre" "$(basename "$ruta")" "NO ESTA" "-"
        continue
    fi
    cp -f "$OUT/host/$nombre/$nombre.dc6" "$OUT/MENU/$nombre.dc6"
    bytes="$(stat -c %s "$OUT/MENU/$nombre.dc6")"
    printf '%-14s %-52s %-18s %s\n' "$nombre" "$(basename "$ruta")" "$hallado" "$bytes"
    # la paleta, aparte: puede vivir en OTRO MPQ (como en DC-002.35)
    ruta_pal="$(printf '%s' "$paleta" | sed 's|/|\\|g')"
    if [ ! -f "$OUT/MENU/$nombre.pal" ]; then
        for m in $MPQS; do
            real="$(ls -1 "$D2DIR" | grep -ix "$m" | head -1)"
            [ -n "$real" ] || continue
            if ( cd "$REPO" && go run ./console/dc/bake-dc6 -mpq "$D2DIR/$real" -pal "$ruta_pal" \
                    -out "$OUT/host/$nombre" -name "$nombre" -solo-pal >/dev/null 2>&1 ); then
                cp -f "$OUT/host/$nombre/$nombre.pal" "$OUT/MENU/$nombre.pal" 2>/dev/null
                break
            fi
        done
    fi
done <<EOF
$ASSETS
EOF

echo
echo "== LA MEDIDA DE CADA UNO (mismo verificador que DC-002.35)"
for f in "$OUT"/MENU/*.dc6; do
    [ -f "$f" ] || continue
    base="$(basename "$f" .dc6)"
    pal="$OUT/MENU/$base.pal"
    [ -f "$pal" ] || pal=""
    if [ -n "$pal" ]; then
        ( cd "$REPO" && go run ./console/dc/verif-dc6-235 -dc6 "$f" -pal "$pal" -out "$OUT/host/$base" -name "$base" ) 2>&1 |
            grep -E 'HOST DC6 bytes|HOST DC6 frames|HOST DC6 compuesto|HOST PALETA' | sed "s/^/   [$base] /"
    else
        ( cd "$REPO" && go run ./console/dc/verif-dc6-235 -dc6 "$f" -out "$OUT/host/$base" -name "$base" ) 2>&1 |
            grep -E 'HOST DC6 bytes|HOST DC6 frames' | sed "s/^/   [$base] /"
    fi
done

echo
echo "== lo que queda en $OUT/MENU (esto es lo que se hornea en el disco)"
ls -la "$OUT/MENU" | sed 's/^/   /'
