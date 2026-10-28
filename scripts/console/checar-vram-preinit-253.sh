#!/bin/bash
# DC-002.56: CONTROL -- ninguna lectura del ALLOCATOR DEL PVR antes de encender el video.
#
# PARA QUE SIRVE: el POC leia `d2cVramLibre()` (-> `pvr_mem_available()`) TRES veces en el camino previo al
# video (MAPA-IMAGEN, PERSONAJE-IMAGEN, MPQ-CERRADOS), con el PVR apagado. El valor era siempre 0 y no ensuciaba
# nada (medido: KOS pvr_mem.c:176 corta con `pvr_mem_base == NULL`), pero un instrumento que imprime un 0 fijo
# no informa y ademas deja la puerta abierta a que alguien mida VRAM con el allocator sin inicializar. Este
# control lo impide, en las dos caras que se pueden medir:
#
#   A) LA FUENTE (siempre): en `main.go` ninguna llamada a `d2cVramLibre(` puede estar en una linea ANTERIOR a
#      `ebiten.IniciarPantalla(` (la que dispara `pvr_init_defaults`), y las tres marcas previas al video tienen
#      que imprimir `vram_libre= sin-init`. En `csrc/` tiene que haber UNA sola aparicion de `pvr_mem_available(`
#      (la de `d2c_vram_libre`).
#   B) EL LOG (si se le pasa un flycast.log): ninguna marca previa a `HITO250: PVR antes-de-pvr_init_defaults`
#      puede traer un `vram_libre=` NUMERICO.
#
# Uso:  bash scripts/console/checar-vram-preinit-253.sh [flycast.log]
#       POC_DIR=<copia del poc>   -> mide otra copia del arbol (lo usa el mutador)
# Sale: 0 = VERDE (pasa), 1 = ROJO (hay una lectura pre-init).
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
POC="${POC_DIR:-$REPO/console/dc/poc/personaje-mapa}"
MAIN="$POC/main.go"
CSRC="$POC/csrc"
LOG="${1:-}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

fallos=0
malo() { echo "   ROJO: $*"; fallos=$((fallos + 1)); }
bien() { echo "   OK:   $*"; }

echo "== control VRAM pre-init (DC-002.56) sobre $POC"

if [ ! -f "$MAIN" ]; then
    echo "   ROJO: no existe $MAIN"
    exit 1
fi

# ---- A.1 la marca del encendido del video, y su linea
VIDEO=$(grep -n 'ebiten\.IniciarPantalla(' "$MAIN" | head -1 | cut -d: -f1)
if [ -z "$VIDEO" ]; then
    malo "no encuentro la marca del encendido del video ('ebiten.IniciarPantalla(') en main.go"
    VIDEO=999999
else
    bien "la marca del encendido del video esta en main.go:$VIDEO ('ebiten.IniciarPantalla(')"
fi

# ---- A.2 ninguna llamada a d2cVramLibre antes de esa linea
#       (los comentarios se borran ANTES de mirar: una nota que nombra la funcion no es una llamada; las lineas
#        se conservan, asi el numero de linea sigue siendo el del archivo)
sed 's://.*::' "$MAIN" > "$TMP/main.sin-comentarios.go"
ANTES=$(grep -n 'd2cVramLibre(' "$TMP/main.sin-comentarios.go" | grep -v ':[[:space:]]*func ' || true)
if [ -z "$ANTES" ]; then
    bien "ninguna llamada a d2cVramLibre() en main.go (solo la declaracion extern)"
else
    hay_pre=0
    while IFS= read -r linea; do
        n=${linea%%:*}
        if [ "$n" -lt "$VIDEO" ]; then
            malo "d2cVramLibre() en main.go:$n, ANTES del encendido del video (linea $VIDEO): ${linea#*:}"
            hay_pre=1
        else
            bien "d2cVramLibre() en main.go:$n, con el video ya encendido (linea $VIDEO)"
        fi
    done <<< "$ANTES"
    [ "$hay_pre" = "1" ] || true
fi

# ---- A.3 las tres marcas del camino previo al video dicen 'sin-init'
PRE_INIT=$(grep -n 'MAPA-IMAGEN\|PERSONAJE-IMAGEN\|MPQ-CERRADOS' "$TMP/main.sin-comentarios.go" | cut -d: -f1)
CUENTA_INIT=0
CUENTA_MARCA=0
while IFS= read -r n; do
    [ -z "$n" ] && continue
    CUENTA_MARCA=$((CUENTA_MARCA + 1))
    if [ "$n" -lt "$VIDEO" ]; then
        if sed -n "${n}p" "$TMP/main.sin-comentarios.go" | grep -q 'vram_libre= sin-init'; then
            CUENTA_INIT=$((CUENTA_INIT + 1))
        else
            malo "la marca previa al video main.go:$n no imprime 'vram_libre= sin-init': $(sed -n "${n}p" "$TMP/main.sin-comentarios.go")"
        fi
    fi
done <<< "$PRE_INIT"
if [ "$CUENTA_MARCA" -lt 3 ]; then
    malo "esperaba TRES marcas previas al video (MAPA-IMAGEN / PERSONAJE-IMAGEN / MPQ-CERRADOS) y encontre $CUENTA_MARCA"
else
    bien "las $CUENTA_INIT de $CUENTA_MARCA marcas previas al video dicen 'vram_libre= sin-init'"
fi

# ---- A.4 en csrc/ hay UNA sola aparicion de pvr_mem_available( : la de d2c_vram_libre
#       (tambien sin comentarios: una nota que explica la funcion no es una llamada; en C los comentarios de
#        bloque /* */ y los // se borran, y las lineas se conservan para que el numero siga sirviendo)
quitar_comentarios_c() {
    awk '
    {
        linea = $0; salida = ""
        while (length(linea) > 0) {
            if (en_bloque) {
                i = index(linea, "*/")
                if (i == 0) { linea = "" }
                else { linea = substr(linea, i + 2); en_bloque = 0 }
            } else {
                i = index(linea, "/*"); j = index(linea, "//")
                if (j > 0 && (i == 0 || j < i)) { salida = salida substr(linea, 1, j - 1); linea = "" }
                else if (i > 0) { salida = salida substr(linea, 1, i - 1); linea = substr(linea, i + 2); en_bloque = 1 }
                else { salida = salida linea; linea = "" }
            }
        }
        print salida
    }' "$1"
}

if [ -d "$CSRC" ]; then
    mkdir -p "$TMP/csrc"
    for f in "$CSRC"/*.c "$CSRC"/*.h; do
        [ -f "$f" ] || continue
        quitar_comentarios_c "$f" > "$TMP/csrc/$(basename "$f")"
    done
    N_SHIM=$(grep -c 'pvr_mem_available(' "$TMP/csrc/cshim.c" 2>/dev/null || true)
    N_OTROS=$(grep -rn 'pvr_mem_available(' "$TMP/csrc" 2>/dev/null | grep -v 'cshim\.c:' | wc -l)
    if [ "$N_SHIM" -eq 1 ] && [ "$N_OTROS" -eq 0 ]; then
        bien "csrc/: una sola lectura de pvr_mem_available() (la de d2c_vram_libre en cshim.c)"
    else
        malo "csrc/: $N_SHIM apariciones en cshim.c (esperaba 1) y $N_OTROS en el resto (esperaba 0)"
    fi
else
    malo "no existe $CSRC"
fi

# ---- B. el LOG de una corrida, si se paso
if [ -n "$LOG" ]; then
    if [ ! -f "$LOG" ]; then
        malo "me pasaron un log que no existe: $LOG"
    else
        SALIDA=$(awk '
            /HITO250: PVR antes-de-pvr_init_defaults/ { video = 1 }
            !video && /(MAPA-IMAGEN|PERSONAJE-IMAGEN|MPQ-CERRADOS)/ {
                marcas++
                if (/vram_libre= sin-init/) { bien++ } else { malas[++n] = $0 }
            }
            END {
                if (marcas == 0) { print "sin-marcas"; exit }
                for (i = 1; i <= n; i++) print "MALA " malas[i]
                print "CUENTA " bien " " marcas
            }' < <(tr -d '\r' < "$LOG"))
        case "$SALIDA" in
            sin-marcas)
                malo "en el log no hay ninguna marca previa al video (MAPA-IMAGEN / PERSONAJE-IMAGEN / MPQ-CERRADOS)"
                ;;
            *)
                while IFS= read -r l; do
                    case "$l" in
                        MALA*) malo "log: marca previa al video con vram_libre NUMERICO: ${l#MALA }" ;;
                        CUENTA*)
                            set -- $l
                            if [ "$2" -eq "$3" ]; then
                                bien "log: las $2 de $3 marcas previas al video dicen 'vram_libre= sin-init'"
                            else
                                malo "log: $(( $3 - $2 )) de $3 marcas previas al video NO dicen 'sin-init'"
                            fi
                            ;;
                    esac
                done <<< "$SALIDA"
                ;;
        esac
        if tr -d '\r' < "$LOG" | grep -q 'HITO250: VRAM-POSTINIT vram_libre='; then
            bien "log: hay lectura post-init ($(tr -d '\r' < "$LOG" | grep -m1 'VRAM-POSTINIT' | sed 's/^ *//'))"
        else
            echo "   aviso: el log no trae 'HITO250: VRAM-POSTINIT' (puede que la corrida no llegara al video)"
        fi
    fi
else
    echo "   (sin log: solo se mira la fuente; pasale el flycast.log para revisar la corrida)"
fi

echo
if [ "$fallos" -eq 0 ]; then
    echo "== VERDE: ninguna lectura del allocator del PVR antes de encender el video"
    exit 0
fi
echo "== ROJO: $fallos problema(s) de lectura pre-init"
exit 1
