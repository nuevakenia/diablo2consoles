#!/bin/bash
# DC-002.27: LA IMAGEN DE DISCO CON LOS ASSETS DEL JUEGO ADENTRO.
#
# Arma el ISO (y el GDI que arranca Flycast y lee el GDEMU) de un POC YA CONSTRUIDO, con la CARPETA DEL
# JUEGO en `/JUEGO` adentro del ISO9660. Es el otro camino que el romdisk para los datos del juego: el
# romdisk son 16 MB de RAM y el binario ya se lleva 13,9 MB, mientras que el disco se lee POR PARTES (el
# MPQ tiene su indice al final del archivo y el motor hace Seek + ReadAt: no hace falta que entre en RAM).
#
# Uso:  scripts/console/mkdisco.sh <dir_build> [<dir_juego>]
#   <dir_build>  trae IP.BIN y 1ST_READ.BIN (los deja build-motor-arranque.sh o build-dc.sh)
#   <dir_juego>  los MPQ del juego TAL COMO ESTAN en el PC (p.ej. ~/.wine-d2/drive_c/Games/Diablo II).
#                Se hornean todos los *.mpq que haya ahi, en /JUEGO.
#
# POR QUE LOS NOMBRES VAN EN MAYUSCULAS (y por que eso no rompe nada): el ISO9660 de nivel 3 que arma
# pycdlib solo acepta A-Z, 0-9 y _ (medido: "patch_D2.mpq" da PyCdlibInvalidInput "ISO9660 filenames must
# consist of characters A-Z, 0-9, and _"). El motor los pide en minusculas, y el iso9660 de KOS compara los
# nombres sin distinguir mayusculas. Eso NO se supone: la sonda del POC motor-disco abre las DOS formas
# ("/cd/JUEGO/PATCH_D2.MPQ" y "/cd/juego/patch_d2.mpq") y el corpus lo exige.
#
# MUTANTE: DC_SIN_JUEGO=1 -> la ISO se arma SIN la carpeta /JUEGO. Es el mutante de la tarjeta: el cuadro de
# error del motor ("file not found: /cd/juego/patch_d2.mpq") TIENE que volver.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

DIR="${1:?uso: mkdisco.sh <dir_build> [<dir_juego>]}"
JUEGO="${2:-}"
ISO="$DIR/disc.iso"

[ -f "$DIR/IP.BIN" ] || { echo "no encuentro $DIR/IP.BIN (lo deja el build del POC)" >&2; exit 1; }
[ -f "$DIR/1ST_READ.BIN" ] || { echo "no encuentro $DIR/1ST_READ.BIN (lo deja el build del POC)" >&2; exit 1; }

ARGS=("$DIR/IP.BIN" "$DIR/1ST_READ.BIN")
BAKED=0
BYTES=0

if [ "${DC_SIN_JUEGO:-0}" = "1" ]; then
    echo "== la ISO SIN la carpeta del juego (mutante DC_SIN_JUEGO=1) =="
elif [ -z "$JUEGO" ]; then
    echo "AVISO: sin <dir_juego>: la ISO va SIN los MPQ (es el mutante de la tarjeta)" >&2
else
    # DC-002.44: EL MANIFIESTO DEL DIRECTORIO DEL DISCO.
    #
    # El `os.ReadDir` de la consola NO puede listar /cd: el driver iso9660 de KOS abre el directorio
    # (`fs_open("/cd/JUEGO", O_RDONLY|O_DIR)` da un fd valido, medido con el POC disco-c) pero su
    # `iso_readdir` no devuelve las entradas (solo "." y ".."), y ademas el puente de `console/dc/std/os`
    # nunca lo llamaba. Lo que SI anda, y esta medido desde DC-002.19, es el ".manifiesto" que el build
    # hornea junto al contenido (es como lista el romdisk). El disco se arma igual: como este script es el
    # que SABE que quedo adentro, escribe el manifiesto con lo que horneo, con los nombres en minusculas
    # (que es como los pide el motor: `d2setup.Validate` compara el listado contra `patch_d2.mpq`...).
    #
    # El nombre no puede ser ".manifiesto": el ISO9660 de nivel 3 solo acepta A-Z, 0-9 y _ (medido en
    # DC-002.27). Va como MANIFIESTO.TXT y `os.ReadDir` lo busca con ese nombre cuando el de punto no esta.
    MANIF="$DIR/MANIFIESTO.TXT"
    : > "$MANIF"

    for f in "$JUEGO"/*.mpq; do
        [ -f "$f" ] || continue
        # ojo: el nombre interno va en MAYUSCULAS (lo exige el ISO9660 de nivel 3) y con `;1` (la version de
        # ISO9660; el iso9660 de KOS la ignora al comparar, y el motor pide el nombre sin version)
        nombre="$(basename "$f" | tr '[:lower:]' '[:upper:]')"
        ARGS+=("$f:/JUEGO/$nombre;1")
        printf '%s\t%s\tarchivo\n' "$(basename "$f" | tr '[:upper:]' '[:lower:]')" "$(stat -c%s "$f")" >> "$MANIF"
        BAKED=$((BAKED + 1))
        BYTES=$((BYTES + $(stat -c%s "$f")))
    done

    # DC-002.33: ademas de los MPQ, los .txt del directorio. ES UN INSTRUMENTO, y solo hay uno: la lista de
    # nombres de DC-002.33 (`nombres_<mpq>.txt`, la escribe `console/dc/sonda-tablas-mpq -nombres-dir`), que
    # es por donde la CONSOLA recibe los nombres para consultarlos -- el `(listfile)` del MPQ NO se puede leer
    # alla (medido: `zlib: invalid header`, defecto aparte). Sin esto la consulta de la consola no se puede
    # medir: los nombres no llegan al disco.
    for f in "$JUEGO"/*.txt; do
        [ -f "$f" ] || continue
        nombre="$(basename "$f" | tr '[:lower:]' '[:upper:]')"
        ARGS+=("$f:/JUEGO/$nombre;1")
        printf '%s\t%s\tarchivo\n' "$(basename "$f" | tr '[:upper:]' '[:lower:]')" "$(stat -c%s "$f")" >> "$MANIF"
        BAKED=$((BAKED + 1))
        BYTES=$((BYTES + $(stat -c%s "$f")))
    done

    # DC-002.42 (reparado): ADEMAS DE LOS MPQ Y LOS .txt, LOS `.pack` -- el archivo de la TABLA de
    # objectLookups que el motor lee del disco POR REGISTRO (`objectlookups.pack`, lo arma
    # `console/dc/gen-lookups-pack`). Es el mismo camino de datos del juego, asi que va al lado de los MPQ.
    #
    # POR QUE VUELVE A ESTAR: este bucle lo agrego DC-002.42 y la reescritura del horneador de DC-002.44
    # (el MANIFIESTO.TXT) lo dejo afuera sin que nadie lo notara, porque el horneador no se queja de lo que
    # no hornea: el disco se armaba igual, solo que SIN el archivo. Medido (corrida del corpus
    # `checar-lookups-242.sh` con el .a de hoy, 09:0x): la consola contestaba
    #     LOOKUPS-242 FALLO al abrir la tabla del disco: no pude abrir /cd/juego/objectlookups.pack
    # y los controles 4 a 10 caian en rojo MIENTRAS los del arranque (11/12/14/15) daban verde: eso es lo
    # que delata que el que falta es el archivo del disco y no la palanca del binario.
    for f in "$JUEGO"/*.pack; do
        [ -f "$f" ] || continue
        if [ "${DC_SIN_PACK:-0}" = "1" ]; then
            # EL MUTANTE de este camino (DC-002.42): el horneador NO hornea el .pack. Es lo que pasaba de
            # verdad cuando la reescritura de DC-002.44 se lo llevo puesto: el disco se armaba igual y la
            # consola contestaba "no pude abrir /cd/juego/objectlookups.pack". El corpus lo caza.
            echo "== MUTANTE DC_SIN_PACK=1: NO horneo $(basename "$f") (queda fuera de la ISO y del manifiesto) =="
            continue
        fi
        nombre="$(basename "$f" | tr '[:lower:]' '[:upper:]')"
        ARGS+=("$f:/JUEGO/$nombre;1")
        printf '%s\t%s\tarchivo\n' "$(basename "$f" | tr '[:upper:]' '[:lower:]')" "$(stat -c%s "$f")" >> "$MANIF"
        BAKED=$((BAKED + 1))
        BYTES=$((BYTES + $(stat -c%s "$f")))
    done

    ARGS+=("$MANIF:/JUEGO/MANIFIESTO.TXT;1")
fi

# DC-002.45: LA MARCA SUELTA -- un archivo que se hornea en /JUEGO y que el MANIFIESTO.TXT NO lista.
#
# Es el INSTRUMENTO del control que el manifiesto no puede dar: el listado del disco salia del manifiesto
# (o sea del HORNEADOR), asi que un archivo que este en la ISO pero que el horneador no conozca no podia
# salir nunca. Con el listado por el `fs_readdir` del driver, SI sale. El corpus
# (scripts/console/checar-lista-disco-245.sh) exige que salga; el mutante que fuerza el manifiesto exige
# que NO salga. Sin este archivo el defecto no se puede ver: los 3 MPQ estan en las dos listas.
#
# Uso:  DC_MARCA_SUELTA=<archivo>   (se hornea como /JUEGO/MARCA_SUELTA.DAT y NO va al manifiesto)
if [ -n "${DC_MARCA_SUELTA:-}" ]; then
    [ -f "$DC_MARCA_SUELTA" ] || { echo "no existe la marca suelta $DC_MARCA_SUELTA" >&2; exit 1; }
    ARGS+=("$DC_MARCA_SUELTA:/JUEGO/MARCA_SUELTA.DAT;1")
    echo "== marca suelta: $DC_MARCA_SUELTA -> /JUEGO/MARCA_SUELTA.DAT ($(stat -c%s "$DC_MARCA_SUELTA") bytes, FUERA del manifiesto) =="
fi

# DC-002.44: SIN EL ARBOL JOLIET POR DEFECTO (MKISO_JOLIET=0). Medido (seccion 1 de la evidencia de esta
# tarjeta, POC disco-c): con el arbol Joliet (el default de mkiso.py, nivel 3) el driver iso9660 de KOS
# dice "joliet level 3 extensions detected" y monta, pero NINGUN ARCHIVO del arbol se abre --
# `/cd/JUEGO/PATCH_D2.MPQ` y `/cd/1ST_READ.BIN` dan ENOENT -- asi que un disco "real" armado por aca no
# servia para leer el juego (el listado andaba igual, por el manifiesto). El arbol Joliet se sigue pudiendo
# pedir con MKISO_JOLIET=3 (lo hace scripts/console/checar-disco-227.sh, que mide ese defecto a proposito).
# Los otros armadores de disco (mkdisco-235.sh, mkdisco-236.sh) ya venian con 0 desde DC-002.35.
MKISO_JOLIET="${MKISO_JOLIET:-0}" "$DC_PYTHON" "$HERE/mkiso.py" "$ISO" "${ARGS[@]}" | sed 's/^/   /'
"$DC_PYTHON" "$HERE/mkgdi.py" "$DIR" | sed 's/^/   /'

echo "DISCO: $BAKED MPQ en /JUEGO ($BYTES bytes) + el binario de arranque"
echo "DISCO: $DIR/disc.iso ($(stat -c%s "$ISO") bytes) y $DIR/disc.gdi"
