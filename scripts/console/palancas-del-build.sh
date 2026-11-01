#!/bin/bash
# DC-002.107 -- LAS PALANCAS DEL BUILD DEL MOTOR, EN UN SOLO LUGAR.
#
# POR QUE EXISTE. Las palancas que deciden QUE archivos entran a la imagen de la consola estaban COPIADAS
# en cada script que compila Go a mano: `build-motor-all.sh` (el camino del motor), `revisar-arbol-gox.sh`
# (el que compara fechas de .gox) y los builds de los POC del pueblo (`build-mapa-101/102/103/294.sh`,
# `build-tablas-282.sh`). Cada copia es una fuente de verdad mas, y una de ellas se quedo atras: los builds
# de los POC de DC-002.101/.102 no traian la exclusion de la tabla de objectLookups (DC-002.42) y cada
# imagen salia con 1.788.960 B de regalo (medido en DC-002.103: 1ST_READ.BIN de 7.044.392 B en vez de
# 5.251.656 B; y en DC-002.107 el .gox de `d2core/d2records` mide 4.663.692 B sin la exclusion y
# 2.185.576 B con ella, con UN simbolo de 1.672.892 B que se va entero).
# Es el mismo defecto que DC-002.90 (las palancas de knobs.go con dos fuentes de verdad) y DC-002.91
# (el arbol de .gox viejo): el numero se escribe dos veces y una de las dos se queda vieja.
#
# QUE HACE. Este archivo es LA fuente de verdad de las exclusiones por NOMBRE (gccgo NO respeta
# `//go:build`: un archivo con `//go:build dreamcast` se compila igual -- medido en DC-002.42). El que
# compila Go para la consola no arma la lista de fuentes: la PIDE.
#
# Uso (siempre desde un script de build, despues de `env.sh`):
#
#     . "$HERE/palancas-del-build.sh"
#     mapfile -t SRC_P < <(fuentes_del_paquete "$REPO" "$p")
#     sh-elf-gccgo $GODC_GOFLAGS -I "$BASE_GOX" -fgo-pkgpath="$PREFIJO/$p" -c "${SRC_P[@]}" -o "$out"
#     lleva_la_tabla_de_lookups "$OUT/prog.elf" && { echo "FALLA: ..."; exit 1; }
#
# API:
#   fuentes_del_paquete <repo> <paquete>   las fuentes que van al build, una por linea (con TODAS las palancas)
#   exclusiones_del_build                  el regex completo (para quien tenga que mirar fechas o listas)
#   lookups_fuera_de_la_imagen             true si la tabla va FUERA (lo normal)
#   lleva_la_tabla_de_lookups <elf|gox>    0 si el binario lleva el volcado de la tabla (lo que NO queremos)
#   palancas_del_build                     una linea para el log del build (que palancas se usaron)
#
# LA PALANCA DEL MUTANTE. `DC_TABLA_LOOKUPS=dentro` (DC-002.42, criterio 3) vuelve a meter los dos
# archivos: es como se mide cuanto pesa la tabla y como se prueba que un chequeo la caza. El binario que
# sale con el mutante NO es el de la referencia y el build lo dice en grande.

if [ -n "${PALANCAS_DEL_BUILD_CARGADAS:-}" ]; then
    return 0 2>/dev/null || exit 0
fi
PALANCAS_DEL_BUILD_CARGADAS=1

# ---- las cuatro palancas (regex de grep -vE, sobre la ruta completa) ----
# 1. los archivos por plataforma: gccgo no los filtra solo, y los _linux.go se intentan compilar.
PALANCA_PLATAFORMA='_(darwin|windows|js|wasm|plan9|freebsd|netbsd|openbsd|linux|android|ios|solaris|aix)[.]go$'
# 2. los archivos por arquitectura (mismo motivo).
PALANCA_ARQUITECTURA='_(386|amd64|arm|arm64|ppc64|ppc64le|mips|mipsle|mips64|mips64le|riscv64|s390x|loong64|wasm)[.]go$'
# 3. el par mutuamente excluyente de las marcas de carga (DC-002.43): el de la consola es
#    `marca_carga_godc.go`, el del host no entra.
PALANCA_MARCA_HOST='/(marca_carga_host)[.]go$'
# 4. la tabla de objectLookups (DC-002.42): su lugar es el DISCO (`/JUEGO/OBJECTLOOKUPS.PACK`, la arma
#    `console/dc/gen-lookups-pack`) y el motor la lee POR REGISTRO cuando la usa
#    (`d2core/d2records/object_lookups_store.go`). El volcado de `object_lookup_record_data.go` es UN
#    simbolo de .data de 1.672.892 B mas 116.048 B de sus cadenas = 1.788.960 B de imagen; y
#    `object_lookups_host.go` es el lado del host (registro de la tabla como fuente y escritor del .pack).
PALANCA_LOOKUPS='/(object_lookup_record_data|object_lookups_host)[.]go$'
# lo que pesa la tabla en la imagen (medido en DC-002.42 y DC-002.103; escrito UNA vez, aca)
PALANCA_LOOKUPS_BYTES=1788960
# el simbolo unico de .data con el volcado, medido con `sh-elf-nm -S` en DC-002.107
PALANCA_LOOKUPS_SIMBOLO_BYTES=1672892
# un simbolo de datos de 1 MB o mas en un binario de la consola es, hoy, la tabla (medido: en el .gox de
# `d2records` con la exclusion el mayor simbolo no llega a 200 KB, y en `prog.elf` a 300 KB)
PALANCA_UMBRAL_SIMBOLO=1000000

lookups_fuera_de_la_imagen() { [ "${DC_TABLA_LOOKUPS:-fuera}" = "fuera" ]; }

# el regex con las palancas que estan activas (las cuatro, o tres con el mutante puesto)
exclusiones_del_build() {
    local e="$PALANCA_PLATAFORMA|$PALANCA_ARQUITECTURA|$PALANCA_MARCA_HOST"
    if lookups_fuera_de_la_imagen; then
        e="$e|$PALANCA_LOOKUPS"
    fi
    printf '%s\n' "$e"
}

# las fuentes de UN paquete que van al build de la consola
fuentes_del_paquete() {  # <repo> <paquete>
    local repo="$1" paq="$2" patron
    patron="$(exclusiones_del_build)"
    # `ls` de un paquete que no existe no es un error del build: el llamador decide si la lista vacia
    # es un bloqueo (`[ -n "$srcs" ] || continue` en build-motor-all.sh).
    ls "$repo/$paq"/*.go 2>/dev/null | grep -v '_test\.go$' | grep -vE "$patron"
    return 0
}

# 0 (verdadero) si el binario lleva el volcado de la tabla: sirve de guardia DENTRO del build y como
# veredicto del chequeo. Acepta el .gox del paquete o el prog.elf ya enlazado.
lleva_la_tabla_de_lookups() {  # <archivo elf/gox>
    local f="$1"
    [ -f "$f" ] || return 2
    sh-elf-nm -S "$f" 2>/dev/null |
        awk -v umbral="$PALANCA_UMBRAL_SIMBOLO" '
            NF >= 4 && $2 ~ /^[0-9a-f]+$/ && strtonum("0x" $2) >= umbral { print; encontrado = 1 }
            END { exit !encontrado }'
}

# la linea que cada build deja en su log: con cual palanca se compilo
palancas_del_build() {
    if lookups_fuera_de_la_imagen; then
        printf 'PALANCAS DEL BUILD: objectLookups FUERA de la imagen (-%d B); marca de carga: la de la consola\n' \
            "$PALANCA_LOOKUPS_BYTES"
    else
        printf 'PALANCAS DEL BUILD: DC_TABLA_LOOKUPS=dentro -- la tabla de objectLookups VA EN LA IMAGEN (+%d B): NO es el binario de la referencia\n' \
            "$PALANCA_LOOKUPS_BYTES"
    fi
}

# el aviso EN GRANDE del mutante (mismo formato que las pisadas de knobs.go)
aviso_mutante_lookups() {
    lookups_fuera_de_la_imagen && return 0
    printf '%s\n' \
        '   ****************************************************************' \
        '   * MUTANTE: DC_TABLA_LOOKUPS=dentro                              *' \
        '   * la tabla de objectLookups VA EN LA IMAGEN (+1.788.960 B)      *' \
        '   * este binario NO es el de la referencia del POC                *' \
        '   ****************************************************************'
}
