#!/bin/bash
# LOS BUILDS DE LA CONSOLA NO VAN EN /tmp (DC-002.64)
#
# Cada build deja una ISO de ~540 MB (los tres MPQ del juego adentro) mas el ELF. /tmp en este host es
# un tmpfs de 16 GB, o sea RAM: con 30 builds se llena y el siguiente build muere con
# "Failed to locate bootfile" o "No space left on device", que se lee igual que un defecto del codigo
# (paso dos veces en DC-002.50 y DC-002.53). Y peor: esa RAM la pierde el resto del sistema.
#
# Lo que hace este script:
#   1. deja la base de builds en DISCO (~/.cache/dcbuild, 552 GB libres) y pone /tmp/dcbuild como
#      symlink, asi TODOS los scripts que ya escriben en /tmp/dcbuild siguen funcionando sin tocarse;
#   2. rota: conserva los N builds mas recientes y borra el resto;
#   3. nunca borra un build tocado en los ultimos MINUTOS_PROTEGIDO minutos (uno que se este armando o
#      corriendo ahora mismo) salvo con --todos --forzar.
#
# Uso:
#   limpiar-builds.sh                 rotar dejando los 3 mas recientes
#   limpiar-builds.sh 5               dejar 5
#   limpiar-builds.sh --todos         vaciar todo (respeta la proteccion por minutos)
#   limpiar-builds.sh --todos --forzar  vaciar todo sin preguntar
#   limpiar-builds.sh --estado        solo informar
#
# Variables: DCBUILD (base, default ~/.cache/dcbuild), MINUTOS_PROTEGIDO (default 10).
set -uo pipefail

BASE="${DCBUILD:-$HOME/.cache/dcbuild}"
LEGACY="${LEGACY:-/tmp/dcbuild}"
MINUTOS_PROTEGIDO="${MINUTOS_PROTEGIDO:-10}"
QUEDAN=3
MODO="rotar"

case "${1:-}" in
    --estado) MODO="estado" ;;
    --todos)  MODO="todos" ;;
    ''|*[!0-9]*) ;;
    *) QUEDAN="$1" ;;
esac
[ "${2:-}" = "--forzar" ] && MINUTOS_PROTEGIDO=0
[ "${1:-}" = "--forzar" ] && MINUTOS_PROTEGIDO=0

decir() { printf '%s\n' "$*"; }

# 1) la base en disco
mkdir -p "$BASE" || { decir "no puedo crear $BASE"; exit 1; }

# No se mueve la base si hay un build o una corrida en curso: mover 1,6 GB de tmpfs a disco mientras
# flycast tiene la ISO abierta es pedir un archivo a medias. Se rota igual y el symlink espera.
en_curso=0
if pgrep -x flycast >/dev/null 2>&1 || pgrep -x sh-elf-gcc >/dev/null 2>&1; then
    en_curso=1
fi

if [ -e "$LEGACY" ] && [ ! -L "$LEGACY" ]; then
    if [ "$en_curso" = "1" ]; then
        decir "hay un build o una corrida en curso: se rota, pero la base NO se mueve a disco todavia"
        BASE="$LEGACY"
    else
        decir "moviendo $LEGACY a $BASE (son ~540 MB por build: no van en tmpfs)"
        for d in "$LEGACY"/*; do
            [ -e "$d" ] || continue
            mv -n "$d" "$BASE/" 2>/dev/null
        done
        rmdir "$LEGACY" 2>/dev/null
    fi
fi
if [ ! -e "$LEGACY" ] && [ "$en_curso" = "0" ]; then
    ln -s "$BASE" "$LEGACY" && decir "$LEGACY -> $BASE (en disco)"
fi

# 2) inventario
mapfile -t TODOS < <(find "$BASE" -mindepth 1 -maxdepth 1 -type d -printf '%T@ %p\n' 2>/dev/null | sort -rn)
total=${#TODOS[@]}
decir "builds en $BASE: $total"
decir "espacio: $(df -h "$BASE" | awk 'NR==2 {print $4" libres de "$2}')  |  /tmp: $(df -h /tmp | awk 'NR==2 {print $4" libres de "$2}')"

if [ "$MODO" = "estado" ]; then
    i=0
    for t in "${TODOS[@]}"; do
        i=$((i+1))
        p="${t#* }"
        printf '  %2d. %-42s %s\n' "$i" "$(basename "$p")" "$(du -sh "$p" 2>/dev/null | cut -f1)"
    done
    exit 0
fi

# 3) que se borra
limite=0
[ "$MODO" = "rotar" ] && limite="$QUEDAN"
ahora=$(date +%s)
borrados=0
liberado=0
i=0
for t in "${TODOS[@]}"; do
    i=$((i+1))
    mt="${t%% *}"
    p="${t#* }"
    [ "$i" -le "$limite" ] && continue
    edad=$(( (ahora - ${mt%.*}) / 60 ))
    if [ "$edad" -lt "$MINUTOS_PROTEGIDO" ]; then
        decir "  protegido (tocado hace ${edad} min): $(basename "$p")"
        continue
    fi
    tam=$(du -sk "$p" 2>/dev/null | cut -f1)
    rm -rf "$p" && { borrados=$((borrados+1)); liberado=$((liberado+tam)); decir "  borrado: $(basename "$p")"; }
done

decir "borrados=$borrados liberado=$((liberado/1024)) MB"
decir "espacio ahora -> $(df -h "$BASE" | awk 'NR==2 {print $4" libres"}')  |  /tmp: $(df -h /tmp | awk 'NR==2 {print $4" libres"}')"
