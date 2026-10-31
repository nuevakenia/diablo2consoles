#!/bin/bash
# DC-002.91 -- EL ARBOL DE .gox VIEJO NO PUEDE FALLAR COMO SI FUERA EL CODIGO.
#
# QUE HACE. Antes de compilar nada, compara la fecha de cada `.gox` del arbol (BASE_GOX) contra la de la
# fuente mas nueva del paquete que le corresponde:
#   * stdlib de la consola   console/dc/std/<pkg>/*.go            -> <pkg>.gox
#   * terceros reescritos    console/dc/engine/<pkgpath>/*.go     -> <pkgpath>.gox
#   * el motor               d2common|d2core|d2game|d2app|...     -> $PREFIJO/<pkg>.gox
# Si un .gox es mas viejo que su fuente (o no existe), imprime un RECUADRO con el paquete, las dos fechas
# y la receta de rehacer el arbol, y aborta el build.
#
# POR QUE (medido en DC-002.90, commit a02487b8). `zlib.NuevoInflador` entro al repo en DC-002.80
# (97a5c183, 30/10 19:24) y el arbol /tmp/gox-med se habia armado el 30/10 a las 13:22. El build de las
# tablas murio con:
#   /d2common/d2fileformats/d2mpq/prestamo_de_zlib_consola.go:26:21:
#      error: reference to undefined identifier 'zlib.NuevoInflador'
# El simbolo EXISTE en el repo: lo que estaba viejo era el objeto compilado. El mensaje apuntaba al codigo
# y se fue a buscar el problema al lugar equivocado. Este chequeo lo dice ANTES y con la receta.
#
# QUE NO HACE. No mide nada, no recompila nada, no toca el arbol: solo lee fechas (dos `find`, un `awk`).
# Tampoco reemplaza los paquetes que cada build recompila a proposito: esos se dejan pasar, porque el
# propio build los rehace (se informan como "se recompila aca" y no abortan).
#
# Uso:  se llama desde los build-*.sh que usan BASE_GOX:
#         BASE_GOX="$BASE_GOX" RECOMPILA="$PAQUETES_MOTOR" "$HERE/revisar-arbol-gox.sh"
#       BASE_GOX=...          el arbol de .gox a revisar (por defecto /tmp/gox-med)
#       RECOMPILA="p1 p2"     paquetes del motor que el build recompila solo (esos no abortan)
#       GOX_VIEJO_SEGUIR=1    avisa lo mismo pero NO aborta (para medir a proposito con un arbol viejo)
#       GOX_CHEQUEO=0         se salta el chequeo entero
#
# Sale con 1 si hay algo que rehacer (y no se pidio seguir), 0 si el arbol esta al dia.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"

BASE_GOX="${BASE_GOX:-/tmp/gox-med}"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"
RECOMPILA="${RECOMPILA:-}"
# la lista puede venir con saltos de linea (los build-*.sh la escriben en varias lineas): aca se
# normaliza a UNA linea de palabras, porque se busca por " $paquete "
RECOMPILA="$(printf '%s' "$RECOMPILA" | tr '\n\t' '  ' | tr -s ' ')"
# los siete directorios del motor, los mismos que recorre build-motor-all.sh
DIRS_MOTOR="d2common d2core d2game d2app d2networking d2script d2thread"
# gccgo no filtra por sufijo de plataforma (lo hace a mano build-motor-all.sh) y hay dos archivos del
# host que no entran al binario de la consola: si se miraran, el chequeo lloraria por fuentes que el
# arbol no tiene por que haber compilado.
EXCLUIR_MOTOR='(_darwin|_windows|_js|_wasm|_plan9|_freebsd|_netbsd|_openbsd|_linux|_android|_ios|_solaris|_aix)[.]go$'
EXCLUIR_MOTOR="$EXCLUIR_MOTOR"'|_(386|amd64|arm|arm64|ppc64|ppc64le|mips|mipsle|mips64|mips64le|riscv64|s390x|loong64|wasm)[.]go$'
EXCLUIR_MOTOR="$EXCLUIR_MOTOR"'|/marca_carga_host[.]go$|/object_lookup_record_data[.]go$|/object_lookups_host[.]go$'

if [ "${GOX_CHEQUEO:-1}" = "0" ]; then
    echo "== arbol de .gox: chequeo saltado (GOX_CHEQUEO=0)"
    exit 0
fi
if [ ! -d "$BASE_GOX" ]; then
    echo "== arbol de .gox: NO EXISTE $BASE_GOX (corre build-std-gox.sh y build-motor-all.sh)" >&2
    exit 1
fi

TMP="$(mktemp -d "${TMPDIR:-/tmp}/gox-viejo.XXXXXX")"
trap 'rm -rf "$TMP"' EXIT

# ---------------------------- el inventario del arbol (una sola lectura) ----------------------------
find "$BASE_GOX" -name '*.gox' -printf '%T@\t%P\n' |
    awk -F'\t' '{ p = $2; sub(/\.gox$/, "", p); print p "\t" $1 }' > "$TMP/gox.txt"

# ---------------------------- las fuentes, mapeadas a su .gox ----------------------------
# Cada linea que sale es "clave<TAB>fecha<TAB>ruta de la fuente", con la clave igual a la ruta del .gox
# sin el .gox (o sea: el nombre del paquete dentro del arbol).
fuentes() {  # $1 raiz, $2 prefijo a recortar, $3 prefijo a pegar en la clave, [$4 regex a excluir]
    local raiz="$1" quitar="$2" pegar="$3" excluir="${4:-}"
    [ -d "$raiz" ] || return 0
    find "$raiz" -name '*.go' -printf '%T@ %p\n' |
        awk -v q="$quitar" -v pe="$pegar" -v ex="$excluir" '
            {
                t = $1
                sub(/^[^ ]+ /, ""); p = $0
                if (p ~ /_test\.go$/) next
                if (ex != "" && p ~ ex) next
                rel = p
                if (substr(rel, 1, length(q)) == q) rel = substr(rel, length(q) + 1)
                n = split(rel, a, "/")
                if (n < 2) next                     # un .go suelto en la raiz no es un paquete
                pkg = a[1]
                for (i = 2; i < n; i++) pkg = pkg "/" a[i]
                print pe pkg "\t" t "\t" p
            }'
}

fuentes_paquete() {  # $1 dir del paquete, $2 su clave en el arbol. Sirve para los terceros, cuyo
                    # nombre en el arbol es la RUTA DE IMPORTACION y no el camino de la fuente
    local dir="$1" clave="$2"
    [ -d "$dir" ] || return 0
    find "$dir" -maxdepth 1 -name '*.go' ! -name '*_test.go' -printf '%T@ %p\n' |
        awk -v k="$clave" '{ t = $1; sub(/^[^ ]+ /, ""); print k "\t" t "\t" $0 }'
}

{
    # la stdlib minima: `cmd/` son herramientas del host (gen-casos, medir-math), no van al arbol
    fuentes "$REPO/console/dc/std" "$REPO/console/dc/std/" "" '/cmd/'
} > "$TMP/fuentes.txt"

# los terceros reescritos. La tabla NO se escribe aca: sale de los `compilar_tercero "<ruta de
# importacion>" "<dir>"` de build-std-gox.sh, que es donde se declara de verdad que se compila y con que
# nombre. Asi el chequeo no inventa un mapeo que el build no usa.
while read -r pkgpath dir; do
    dir="${dir//\$HERE/$HERE}"
    [ -d "$dir" ] || continue
    fuentes_paquete "$dir" "$pkgpath" >> "$TMP/fuentes.txt"
done < <(awk '/^[[:space:]]*compilar_tercero /{ print $2, $3 }' "$HERE/build-std-gox.sh" | tr -d '"')

for d in $DIRS_MOTOR; do
    fuentes "$REPO/$d" "$REPO/" "$PREFIJO/" "$EXCLUIR_MOTOR" >> "$TMP/fuentes.txt"
done

# ---------------------------- el cruce: por paquete, la fuente mas nueva contra su .gox ----------------------------
awk -F'\t' '
    NR == FNR { gox[$1] = $2; next }
    {
        c[$1] = 1
        if (!(($1) in mt) || $2 + 0 > mt[$1] + 0) { mt[$1] = $2 + 0; src[$1] = $3 }
    }
    END {
        for (k in c) {
            if (!(k in gox)) { print "FALTA\t" k "\t-\t-\t" src[k]; continue }
            if (gox[k] + 0 < mt[k] + 0)
                printf "VIEJO\t%s\t%s\t%s\t%s\n", k, gox[k], sprintf("%.3f", mt[k]), src[k]
        }
    }' "$TMP/gox.txt" "$TMP/fuentes.txt" | sort > "$TMP/hallazgos.txt"

NOBJS="$(wc -l < "$TMP/gox.txt")"
NHALL="$(wc -l < "$TMP/hallazgos.txt")"

if [ "$NHALL" -eq 0 ]; then
    echo "== arbol de .gox: $BASE_GOX -- $NOBJS objetos, ninguno mas viejo que su fuente"
    exit 0
fi

# ---------------------------- la clasificacion y el recuadro ----------------------------
fecha() { date -d "@$1" '+%d/%m %H:%M'; }   # $1 = epoch (con decimales: date los acepta)

FILAS_STD=(); FILAS_TERC=(); FILAS_MOTOR=(); FILAS_FALTA=(); FILAS_RECOMP=()
ABORTA=0
while IFS=$'\t' read -r tipo clave tg ts ruta; do
    case "$clave" in
        "$PREFIJO"/*)
            p="${clave#"$PREFIJO/"}"
            if [ "$tipo" = "FALTA" ]; then
                FILAS_FALTA+=("$(printf '   %-38s %-13s %-13s %s' "$p" "(no existe)" "-" "d2*/$p/*.go")")
                ABORTA=1
            elif case " $RECOMPILA " in *" $p "*) true ;; *) false ;; esac; then
                # este build lo recompila solo: no hay nada que rehacer en el arbol
                FILAS_RECOMP+=("$(printf '   %-38s %-13s %s' "$p" "$(fecha "$tg")" \
                    "se recompila aca (la fuente mas nueva es ${ruta#"$REPO/"})")")
            else
                FILAS_MOTOR+=("$(printf '   %-38s %-13s %-13s %s' "$p" "$(fecha "$tg")" "$(fecha "$ts")" \
                    "${ruta#"$REPO/"}")")
                ABORTA=1
            fi ;;
        *)
            if [ "$tipo" = "FALTA" ]; then
                FILAS_FALTA+=("$(printf '   %-38s %-13s %-13s %s' "$clave" "(no existe)" "-" "${ruta#"$REPO/"}")")
            elif [ -d "$REPO/console/dc/std/$clave" ]; then
                FILAS_STD+=("$(printf '   %-38s %-13s %-13s %s' "$clave" "$(fecha "$tg")" "$(fecha "$ts")" \
                    "${ruta#"$REPO/"}")")
            else
                FILAS_TERC+=("$(printf '   %-38s %-13s %-13s %s' "$clave" "$(fecha "$tg")" "$(fecha "$ts")" \
                    "${ruta#"$REPO/"}")")
            fi
            ABORTA=1 ;;
    esac
done < "$TMP/hallazgos.txt"

cabecera() { printf '   %-38s %-13s %-13s %s\n' "paquete" ".gox (arbol)" "fuente mas nueva" "la fuente"; }

{
    echo ""
    echo "###########################################################################################"
    echo "#  OJO: EL ARBOL DE .gox ES MAS VIEJO QUE SUS FUENTES -- NO ES UN ERROR DEL CODIGO"
    echo "#  arbol: $BASE_GOX"
    if [ "${#FILAS_STD[@]}" -gt 0 ]; then
        echo "#"
        echo "#  LA STDLIB DE LA CONSOLA (console/dc/std -> \$BASE_GOX/<pkg>.gox)"
        cabecera
        printf '%s\n' "${FILAS_STD[@]}"
    fi
    if [ "${#FILAS_TERC[@]}" -gt 0 ]; then
        echo "#"
        echo "#  LOS TERCEROS REESCRITOS (console/dc/engine -> \$BASE_GOX/<ruta de importacion>.gox)"
        cabecera
        printf '%s\n' "${FILAS_TERC[@]}"
    fi
    if [ "${#FILAS_MOTOR[@]}" -gt 0 ]; then
        echo "#"
        echo "#  EL MOTOR (d2* -> \$BASE_GOX/$PREFIJO/<pkg>.gox)"
        cabecera
        printf '%s\n' "${FILAS_MOTOR[@]}"
    fi
    if [ "${#FILAS_FALTA[@]}" -gt 0 ]; then
        echo "#"
        echo "#  SIN OBJETO EN EL ARBOL (el .gox no existe)"
        cabecera
        printf '%s\n' "${FILAS_FALTA[@]}"
    fi
    if [ "${#FILAS_RECOMP[@]}" -gt 0 ]; then
        echo "#"
        echo "#  VIEJOS PERO NO IMPORTA: este build los recompila antes de enlazar (no hace falta rehacer nada)"
        printf '%s\n' "${FILAS_RECOMP[@]}"
    fi
    echo "#"
    echo "#  gccgo va a decir algo como"
    echo "#     error: reference to undefined identifier 'X.Y'"
    echo "#  y el simbolo SI existe en el repo: lo que falta es el OBJETO COMPILADO. La receta (~6 min):"
    echo "#"
    echo "#     bash scripts/console/build-std-gox.sh    /tmp/gox-nuevo            # stdlib + terceros"
    echo "#     bash scripts/console/build-motor-all.sh  /tmp/gox-nuevo /tmp/log-nuevo"
    echo "#     BASE_GOX=/tmp/gox-nuevo bash scripts/console/build-XXXX.sh          # el build de la tarjeta"
    echo "#"
    echo "#  (medido en DC-002.90: stdlib 63 objetos, motor 145 de 145, bloqueados 0)"
    if [ "$ABORTA" = "1" ] && [ "${GOX_VIEJO_SEGUIR:-0}" != "1" ]; then
        echo "#  -> ABORTA ACA, sin compilar. Para seguir igual a proposito: GOX_VIEJO_SEGUIR=1"
    elif [ "$ABORTA" = "1" ]; then
        echo "#  -> GOX_VIEJO_SEGUIR=1: se avisa y se SIGUE igual (el build puede morir por el arbol, no"
        echo "#     por el codigo: si falla con 'undefined identifier', es esto)"
    else
        echo "#  -> no aborta: todo lo viejo lo recompila este mismo build"
    fi
    echo "###########################################################################################"
    echo ""
} >&2

[ "$ABORTA" = "1" ] && [ "${GOX_VIEJO_SEGUIR:-0}" != "1" ] && exit 1
exit 0
