#!/bin/bash
# DC-002.107 -- CHEQUEO: NINGUN BUILD DE POC SALE SIN LAS PALANCAS DEL BUILD DEL MOTOR.
#
# POR QUE EXISTE. DC-002.103 midio el defecto: los builds de los POC del pueblo (DC-002.101/.102 y el 103
# que salio de ellos) NO aplicaban la exclusion de la tabla de objectLookups (DC-002.42) porque cada script
# tenia su COPIA de los `grep -vE` de build-motor-all.sh, y una copia se quedo vieja. Cada imagen salia con
# 1.788.960 B de regalo (`object_lookup_record_data.go` es UN simbolo de .data de 1.672.892 B mas 116.048 B
# de sus cadenas) y el monton de C arrancaba 1,79 MB mas abajo. Es el defecto de DC-002.90/.91: el mismo
# numero escrito en dos lados.
#
# QUE PRUEBA (cada cosa tiene que poder ponerse en ROJO sola):
#   1/5 las palancas estan en UN SOLO LUGAR: ningun build de POC escribe su propia lista de exclusiones, y
#       todos piden la lista al camino unico (`palancas-del-build.sh`).
#   2/5 el camino unico deja la tabla AFUERA y con el mutante la mete: se compila `d2core/d2records` de las
#       dos maneras y se comparan BYTES y el simbolo de 1.672.892 B (el numero, no la bandera).
#   3/5 el binario de cada POC construido no lleva la tabla: `prog.elf` sin simbolos de datos de 1 MB o mas,
#       `.data+.bss` por debajo del tope y `1ST_READ.BIN` con su tamano y su sha256 a la vista.
#   4/5 el mutante PONE EL CHEQUEO EN ROJO: el mismo verificador de 3/5, aplicado al binario con la tabla
#       adentro, tiene que decir que NO. Si dijera que si, el chequeo no caza nada (prueba en contra).
#   5/5 (CONSTRUIR=1) construye cada POC con su script y le corre el verificador: de punta a punta.
#
# Uso:  scripts/console/checar-palancas-poc-107.sh [dir_salida]
#       CONSTRUIR=1     construye cada POC de la lista con su script (tarda: son cinco builds de ~1,5 min)
#       BASE_GOX=...    arbol de .gox para esos builds (por defecto $HOME/gox108; /tmp puede estar lleno)
#       DIRS="a b c"    en vez de construir, verifica estos directorios de build (los cinco por defecto)
#       MUTANTE=1       se salta 1/5 y 5/5 y corre solo el control del mutante (2/5 y 4/5)
#
# LO QUE MIDIO EN DC-002.107 (1ST_READ.BIN de cada POC: con la tabla adentro -> por el camino unico):
#   mapa-101    7.052.520 -> 5.259.240 B   sha256 fe1efdbc1a9bd1d5...   -1.793.280
#   mapa-102    7.069.896 -> 5.276.456 B   sha256 7aa7481b84077cbc...   -1.793.440
#   mapa-103    7.078.984 -> 5.285.672 B   sha256 adcbb79acb8f55d7...   -1.793.312
#   mapa-294    7.050.760 -> 5.257.512 B   sha256 a997ddafb4a82a2e...   -1.793.248
#   tablas-282  6.877.096 -> 5.083.784 B   sha256 3edaf6d5d5dcc2d6...   -1.793.312
# El "antes" NO es un numero copiado: es el build de hoy con `DC_TABLA_LOOKUPS=dentro`, y los cuatro de
# mapa-101/102/294 y tablas-282 salen con el MISMO sha256 que los builds viejos de DC-002.103. La tabla de
# evidencia (logs, los .CON-LA-TABLA, el mutante POC por POC) esta en
# console/dc/evidencia/dc-002.107-las-palancas-de-los-poc.txt.
# sale: 0 = todo en verde; 1 = alguna prueba en rojo; 3 = no se pudo correr
set -o pipefail

REPO="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=env.sh
. "$REPO/scripts/console/env.sh" >/dev/null 2>&1 || true
# shellcheck source=palancas-del-build.sh
. "$REPO/scripts/console/palancas-del-build.sh"

SALIDA="${1:-$HOME/dcbuild107/chequeo}"
mkdir -p "$SALIDA" || { echo "FALLA: no puedo crear $SALIDA"; exit 3; }
BASE_GOX="${BASE_GOX:-$HOME/gox108}"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"

# el tope de la imagen de un POC: con la tabla adentro mide 7.044.392 B (medido en DC-002.103); sin ella
# ninguno de estos POC pasa de 5,3 MB. El numero esta escrito UNA vez, aca.
TOPE_1ST_READ=6000000
# el tope de datos de un ejecutable de la consola: con la tabla son 1.933.848 B; sin ella, ~261.000 B.
TOPE_DATOS=1000000

# los cinco builds de POC que compilan `d2core/d2records` a mano (los que se pueden quedar sin la palanca).
# nombre:script:directorio por defecto
POCS="mapa-101:build-mapa-101.sh:/tmp/dcbuild/mapa101
mapa-102:build-mapa-102.sh:/tmp/dcbuild/mapa102
mapa-103:build-mapa-103.sh:/tmp/dcbuild/mapa103
mapa-294:build-mapa-294.sh:/tmp/dcbuild/mapa294
tablas-282:build-tablas-282.sh:/tmp/dcbuild/tablas282"

# los scripts que tienen que pedir la lista al camino unico (los dos del motor y los cinco de POC)
DEL_CAMINO_UNICO="build-motor-all.sh revisar-arbol-gox.sh build-mapa-101.sh build-mapa-102.sh build-mapa-103.sh build-mapa-294.sh build-tablas-282.sh"

verdes=0
rojas=0
ok()  { echo "   OK    $*"; verdes=$((verdes + 1)); }
mal() { echo "   ROJO  $*"; rojas=$((rojas + 1)); }

# las lineas EJECUTABLES (sin comentarios) de un script que tocan la lista de exclusiones: si hay alguna,
# ese script tiene su propia fuente de verdad
copias_de_la_lista() {  # <script>
    awk '
        /^[[:space:]]*#/ { next }
        /object_lookup_record_data|object_lookups_host|marca_carga_host/ { print NR": "$0; next }
        /grep[[:space:]]+-vE.*_\(darwin/ { print NR": "$0 }
    ' "$1"
}

# ---- el verificador: un directorio de build de POC, veredicto unico (0 = sin la tabla, 1 = con la tabla)
# se usa en 3/5 (tiene que dar 0) y en 4/5 (el mutante tiene que dar 1). Es el MISMO codigo: si el mutante
# no lo pone en rojo, el chequeo no sirve.
verificar_poc() {  # <dir> [silencio]
    local dir="$1" callar="${2:-0}" elf="$1/prog.elf" bin="$1/1ST_READ.BIN" datos
    [ -f "$elf" ] || { [ "$callar" = 1 ] || echo "      sin prog.elf en $dir"; return 2; }
    if lleva_la_tabla_de_lookups "$elf" > "$dir/.tabla-encontrada" 2>/dev/null; then
        [ "$callar" = 1 ] || { echo "      prog.elf LLEVA la tabla de objectLookups:"; sed 's/^/         /' "$dir/.tabla-encontrada"; }
        return 1
    fi
    datos="$(sh-elf-size -A "$elf" 2>/dev/null | awk '$1==".data"||$1==".bss"{s+=$2} END{print s+0}')"
    if [ "${datos:-0}" -ge "$TOPE_DATOS" ]; then
        [ "$callar" = 1 ] || echo "      .data+.bss = $datos B (tope $TOPE_DATOS): la tabla esta adentro"
        return 1
    fi
    if [ -f "$bin" ]; then
        local tam
        tam="$(stat -c%s "$bin")"
        if [ "$tam" -ge "$TOPE_1ST_READ" ]; then
            [ "$callar" = 1 ] || echo "      1ST_READ.BIN = $tam B (tope $TOPE_1ST_READ): la imagen es la gorda"
            return 1
        fi
        [ "$callar" = 1 ] || printf '      1ST_READ.BIN = %s B  sha256 %s  .data+.bss %s B\n' \
            "$tam" "$(sha256sum "$bin" | cut -c1-16)..." "$datos"
    fi
    return 0
}

echo "== 1/5 las palancas del build estan en UN SOLO LUGAR (nada copiado)"

if [ "${MUTANTE:-0}" != "1" ]; then
    for s in $DEL_CAMINO_UNICO; do
        f="$REPO/scripts/console/$s"
        if [ ! -f "$f" ]; then mal "no esta $s"; continue; fi
        if grep -q 'palancas-del-build.sh' "$f"; then
            ok "$s pide las palancas al camino unico"
        else
            mal "$s NO pide las palancas a palancas-del-build.sh"
        fi
        if [ "$s" = "revisar-arbol-gox.sh" ]; then
            grep -q 'exclusiones_del_build' "$f" && ok "$s usa exclusiones_del_build" \
                || mal "$s no usa exclusiones_del_build"
        else
            grep -q 'fuentes_del_paquete' "$f" && ok "$s usa fuentes_del_paquete" \
                || mal "$s no usa fuentes_del_paquete"
        fi
        linea="$(copias_de_la_lista "$f")"
        if [ -z "$linea" ]; then
            ok "$s no tiene copia propia de la lista de exclusiones"
        else
            mal "$s tiene su propia copia de la lista (esa es la segunda fuente de verdad):"
            printf '         %s\n' "$linea"
        fi
    done
    # el camino unico tiene que declarar las cuatro palancas
    for p in PALANCA_PLATAFORMA PALANCA_ARQUITECTURA PALANCA_MARCA_HOST PALANCA_LOOKUPS; do
        grep -q "^$p=" "$REPO/scripts/console/palancas-del-build.sh" \
            && ok "palancas-del-build.sh declara $p" \
            || mal "palancas-del-build.sh NO declara $p"
    done
fi

echo "== 2/5 el camino unico deja la tabla AFUERA, y con el mutante la mete (bytes y simbolo)"
GOX_FUERA="$SALIDA/d2records-fuera.gox"
GOX_DENTRO="$SALIDA/d2records-dentro.gox"
if [ ! -d "$BASE_GOX" ]; then
    mal "no esta el arbol de .gox en $BASE_GOX (corre build-std-gox.sh y build-motor-all.sh)"
else
    mapfile -t FUERA < <(fuentes_del_paquete "$REPO" d2core/d2records)
    mapfile -t DENTRO < <(DC_TABLA_LOOKUPS=dentro fuentes_del_paquete "$REPO" d2core/d2records)
    if [ "${#FUERA[@]}" -gt 0 ] && sh-elf-gccgo $GODC_GOFLAGS -I "$BASE_GOX" \
            -fgo-pkgpath="$PREFIJO/d2core/d2records" -c "${FUERA[@]}" -o "$GOX_FUERA" 2> "$SALIDA/gox-fuera.err"; then
        T_FUERA="$(stat -c%s "$GOX_FUERA")"
        if lleva_la_tabla_de_lookups "$GOX_FUERA" > "$SALIDA/simbolo-fuera.txt"; then
            mal "el .gox SIN el mutante lleva un simbolo de datos de 1 MB o mas (la tabla se colo)"
            sed 's/^/         /' "$SALIDA/simbolo-fuera.txt"
        else
            ok "el .gox de d2records con el camino unico NO lleva el volcado ($T_FUERA B, ${#FUERA[@]} fuentes)"
        fi
    else
        mal "no pude compilar d2records por el camino unico (ver $SALIDA/gox-fuera.err)"
        T_FUERA=0
    fi
    if [ "${#DENTRO[@]}" -gt 0 ] && sh-elf-gccgo $GODC_GOFLAGS -I "$BASE_GOX" \
            -fgo-pkgpath="$PREFIJO/d2core/d2records" -c "${DENTRO[@]}" -o "$GOX_DENTRO" 2> "$SALIDA/gox-dentro.err"; then
        T_DENTRO="$(stat -c%s "$GOX_DENTRO")"
        if lleva_la_tabla_de_lookups "$GOX_DENTRO" > "$SALIDA/simbolo-dentro.txt"; then
            simbolo="$(awk '$2 ~ /^[0-9a-f]+$/ {print strtonum("0x" $2)}' "$SALIDA/simbolo-dentro.txt" | sort -rn | head -1)"
            ok "con DC_TABLA_LOOKUPS=dentro el .gox SI lleva el volcado: simbolo de $simbolo B ($T_DENTRO B, ${#DENTRO[@]} fuentes)"
        else
            mal "con DC_TABLA_LOOKUPS=dentro el .gox NO lleva el volcado: el mutante no muta y el control no vale"
            simbolo=0
        fi
        if [ "${T_DENTRO:-0}" -gt "${T_FUERA:-0}" ]; then
            ok "el mutante engorda el .gox en $((T_DENTRO - T_FUERA)) B ($T_FUERA -> $T_DENTRO)"
        else
            mal "el mutante no engorda el .gox ($T_FUERA -> $T_DENTRO): no esta metiendo nada"
        fi
        if [ "${simbolo:-0}" -eq "$PALANCA_LOOKUPS_SIMBOLO_BYTES" ]; then
            ok "el simbolo mide exactamente $PALANCA_LOOKUPS_SIMBOLO_BYTES B: es el volcado de la tabla"
        else
            mal "el simbolo del volcado mide '${simbolo:-0} B' y la referencia es $PALANCA_LOOKUPS_SIMBOLO_BYTES B"
        fi
    else
        mal "no pude compilar d2records con el mutante (ver $SALIDA/gox-dentro.err)"
    fi
fi

echo "== 3/5 cada POC construido: el binario NO lleva la tabla y se dicen tamano y sha256"
if [ -n "${DIRS:-}" ]; then
    for d in $DIRS; do
        [ -d "$d" ] || { mal "no existe el directorio de build $d"; continue; }
        salida="$(verificar_poc "$d")"
        if [ $? -eq 0 ]; then
            ok "POC $d: sin la tabla"
            printf '%s\n' "$salida"
        else
            mal "POC $d: la imagen NO es la de la referencia"
            [ -n "$salida" ] && printf '%s\n' "$salida"
        fi
    done
else
    for linea in $POCS; do
        nombre="${linea%%:*}"; resto="${linea#*:}"; dir="${resto#*:}"
        if [ ! -d "$dir" ]; then
            echo "   --    POC $nombre: sin build en $dir (no se verifica; CONSTRUIR=1 lo hace)"
            continue
        fi
        salida="$(verificar_poc "$dir")"
        if [ $? -eq 0 ]; then
            ok "POC $nombre: sin la tabla"
            printf '%s\n' "$salida"
        else
            mal "POC $nombre: la imagen lleva la tabla (o esta sobre el tope)"
            [ -n "$salida" ] && printf '%s\n' "$salida"
        fi
    done
fi

echo "== 4/5 el mutante PONE EL CHEQUEO EN ROJO (la prueba en contra: si esto no caza, nada caza)"
if [ -f "$GOX_DENTRO" ]; then
    mkdir -p "$SALIDA/mutante"
    cp -f "$GOX_DENTRO" "$SALIDA/mutante/prog.elf"
    if verificar_poc "$SALIDA/mutante" > /dev/null 2>&1; then
        mal "el verificador APROBO un binario que lleva la tabla: no caza la regresion"
    else
        ok "el verificador RECHAZA el binario con la tabla adentro (mismo codigo de 3/5)"
    fi
else
    mal "no hay binario mutante para probar el verificador"
fi

echo "== 5/5 (CONSTRUIR=1) los cinco POC de punta a punta, con su script"
if [ "${CONSTRUIR:-0}" = "1" ] && [ "${MUTANTE:-0}" != "1" ]; then
    for linea in $POCS; do
        nombre="${linea%%:*}"; resto="${linea#*:}"; script="${resto%%:*}"; dir="${resto#*:}"
        sal="$SALIDA/poc-$nombre"
        echo "   --    $nombre: bash $script $sal (BASE_GOX=$BASE_GOX)"
        if BASE_GOX="$BASE_GOX" bash "$REPO/scripts/console/$script" "$sal" > "$SALIDA/build-$nombre.log" 2>&1; then
            ok "$nombre: el build termino (log en $SALIDA/build-$nombre.log)"
            if verificar_poc "$sal" > "$SALIDA/verifica-$nombre.txt"; then
                ok "$nombre: el binario no lleva la tabla"
                sed 's/^/         /' "$SALIDA/verifica-$nombre.txt"
            else
                mal "$nombre: el binario NO paso el verificador"
                sed 's/^/         /' "$SALIDA/verifica-$nombre.txt"
            fi
        else
            mal "$nombre: el build fallo (ver $SALIDA/build-$nombre.log)"
            tail -6 "$SALIDA/build-$nombre.log" | sed 's/^/         /'
        fi
    done
else
    echo "   --    no pedido (CONSTRUIR=1 para construir los cinco)"
fi

echo
echo "== resumen: $verdes en verde, $rojas en rojo"
if [ "$rojas" -eq 0 ]; then
    echo "== VERDE: ningun build de POC sale sin las palancas, y el chequeo caza el mutante"
    exit 0
fi
echo "== ROJO: algun build de POC puede salir con la tabla de objectLookups adentro (1.788.960 B de regalo)"
exit 1
