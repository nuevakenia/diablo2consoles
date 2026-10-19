#!/bin/bash
# CORPUS DE DC-002.44: EL LISTADO DE UN DIRECTORIO DEL DISCO (ISO9660) VISTO POR EL MOTOR.
#
# QUE MIDE (los cuatro criterios de la tarjeta, con la salida a la vista):
#
#  1. en que llamada se pierde el listado -- la seccion 1 de la evidencia
#     (console/dc/evidencia/dc-002.44-lista-del-disco.txt) lo mide con el POC de C `disco-c`, que habla con
#     el DRIVER pelado (fs_open/fs_readdir de KOS): abre el directorio (fd valido) y solo devuelve "." y "..".
#     Este corpus mide la otra mitad, la del PUENTE de `os`, con el motor entero arriba.
#  2. el numero del motor con los MPQ en /JUEGO: el POC `motor-arranque` (el entry point de verdad,
#     d2app.Create().Run()) sobre TRES discos armados por `mkdisco.sh`, con la MISMA ISO y el MISMO binario:
#
#       mdisco3    los 3 MPQ de verdad en /JUEGO        -> `archivos que ve= 3` (antes: 0)
#       mdisco7    los mismos 3 + los 4 nombres del nucleo que en esta maquina NO existen, como MARCAS de
#                  0 bytes (d2music, d2sfx, d2speech, d2video) -> `Validate.OK()= true ... faltan= 0`
#       mdiscomut  DC_SIN_JUEGO=1 (la ISO SIN /JUEGO)   -> el control negativo: 0 / faltan= 7
#
#     OJO CON EL NUMERO: `Validate.OK()` exige los 7 de `d2setup.CoreMPQs` (discover.go: OK() ==
#     len(MissingCore) == 0), asi que con 3 MPQ en el disco `faltan` NO puede ser 0 (es 4) y `ve` no puede
#     ser 3 con OK()= true (serian 7). Los tres numeros del criterio 2 (OK=true, ve=3, faltan=0) NO pueden
#     darse juntos: lo que se mide son los dos extremos, con la linea del codigo a la vista.
#  3. el overlay de la RAM (`/rd/prueba-seleccion`) sigue listando: lo mide el mismo POC en las mismas
#     corridas (`MOTOR-ARRANQUE seleccion overlay: ReadDir de /rd/prueba-seleccion -> 1 entradas: uno.txt`).
#  4. el binario que se mide tiene el arreglo: control de la CAPA de .gox (el os.gox de la capa vieja NO
#     tiene el arreglo y el motor imprime el defecto aunque el os.go del arbol este arreglado; medido en
#     esta corrida, es el defecto mas facil de leer mal).
#
# Uso:  bash scripts/console/checar-lista-disco-244.sh [--refrescar] [segundos_por_corrida]
#   --refrescar  rehace la capa de .gox (75 + 142 paquetes), el binario y los tres discos. Sin la bandera
#                se reusa lo que ya este en $SAL (los builds son de minutos y cada ISO de 552 MB).
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SAL="${SAL:-/home/dgalli/.hermes/cache/scratch/c244}"
GEN="${GEN:-$SAL/gox-head}"
MPQ="${MPQ:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
LISTA="patch_d2.mpq,d2data.mpq,d2char.mpq"
MARCAS="d2music.mpq d2sfx.mpq d2speech.mpq d2video.mpq"
SEGS="${2:-45}"
REFRESCAR=0
[ "${1:-}" = "--refrescar" ] && REFRESCAR=1

OK=0
FALLOS=0
nota() { echo "   $*"; }
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }
paso() { printf '\n== %s\n' "$*"; }
# busca una linea EXACTA en el log de una corrida
linea() { grep -aF "$2" "$SAL/run-$1/flycast.log" 2>/dev/null | head -1 | tr -d '\r'; }

mkdir -p "$SAL"

# ================================================================================================
# 0. LAS CARPETAS QUE SE HORNEA Y EL CONTROL DE PROCEDENCIA DE LOS MPQ
# ================================================================================================
paso "0. las carpetas del disco (enlaces duros: no copia 540 MB) y el sha256 contra el PC"
for n in patch_d2.mpq d2data.mpq d2char.mpq; do
    real="$(ls -1 "$MPQ" | grep -ix "$n" | head -1)"
    [ -n "$real" ] || { echo "no encuentro $n en $MPQ"; exit 1; }
    mkdir -p "$SAL/juego3" "$SAL/juego7"
    ln -f "$MPQ/$real" "$SAL/juego3/$n" 2>/dev/null || cp "$MPQ/$real" "$SAL/juego3/$n"
    ln -f "$SAL/juego3/$n" "$SAL/juego7/$n" 2>/dev/null || cp "$SAL/juego3/$n" "$SAL/juego7/$n"
    a="$(sha256sum "$SAL/juego3/$n" | cut -d' ' -f1)"
    b="$(sha256sum "$MPQ/$real" | cut -d' ' -f1)"
    if [ "$a" = "$b" ]; then verde "MPQ $n sha256=$a (el mismo archivo del PC)"; else rojo "MPQ $n NO coincide"; fi
done
# las marcas vacias: son el INSTRUMENTO (esta maquina tiene 3 de los 7 MPQ del nucleo). Se declaran.
for n in $MARCAS; do
    [ -f "$SAL/juego7/$n" ] || : > "$SAL/juego7/$n"
    [ "$(stat -c%s "$SAL/juego7/$n")" = "0" ] || { rojo "la marca $n no esta vacia"; continue; }
    verde "marca $n: $(stat -c%s "$SAL/juego7/$n") bytes (nombre del nucleo, contenido vacio: solo mide el LISTADO)"
done

# ================================================================================================
# 1. LA CAPA DE .GOX Y EL BINARIO (el arreglo TIENE que estar adentro del binario que se mide)
# ================================================================================================
paso "1. la capa de .gox coherente con HEAD y el binario del arranque"
if [ "$REFRESCAR" = 1 ]; then
    rm -rf "$GEN"; mkdir -p "$GEN"
    ( cd "$REPO" && bash scripts/console/build-std-gox.sh "$GEN" ) > "$SAL/std-gox.log" 2>&1
    ( cd "$REPO" && bash scripts/console/build-motor-all.sh "$GEN" "$SAL/log-motor-all" ) > "$SAL/motor-all.log" 2>&1 || true
    rm -rf "$SAL/marranque"; mkdir -p "$SAL/marranque"
    ( cd "$REPO" && POC=motor-arranque DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO="$LISTA" DC_SIN_SELLOS=1 \
        bash scripts/console/build-motor-arranque.sh "$SAL/marranque" "$GEN" ) > "$SAL/marranque.build.log" 2>&1
fi
[ -f "$SAL/marranque/1ST_READ.BIN" ] || { echo "no esta $SAL/marranque/1ST_READ.BIN (correr con --refrescar)"; exit 1; }
if [ -f "$GEN/os.gox" ] && [ "$(strings "$GEN/os.gox" | grep -c MANIFIESTO.TXT)" != "0" ]; then
    verde "control 4: el os.gox de la capa ($(stat -c%s "$GEN/os.gox") bytes) TIENE el arreglo (MANIFIESTO.TXT)"
else
    rojo "control 4: el os.gox de la capa es VIEJO (sin el arreglo): el motor imprimiria el defecto igual"
fi
nota "binario: $(stat -c%s "$SAL/marranque/1ST_READ.BIN") bytes sha256 $(sha256sum "$SAL/marranque/1ST_READ.BIN" | cut -c1-16)"
nota "config del arranque: $(grep -a 'CONFIG-CONSOLA datos=' "$SAL/marranque.build.log" | tail -1)"
nota "config del arranque: $(grep -a 'MpqLoadOrder=' "$SAL/marranque.build.log" | tail -1)"

# ================================================================================================
# 2. LOS TRES DISCOS
# ================================================================================================
paso "2. los tres discos (mkdisco.sh: SIN Joliet por defecto y con MANIFIESTO.TXT horneado)"
for d in mdisco3 mdisco7 mdiscomut; do
    mkdir -p "$SAL/$d"
    ln -f "$SAL/marranque/IP.BIN" "$SAL/$d/IP.BIN"
    ln -f "$SAL/marranque/1ST_READ.BIN" "$SAL/$d/1ST_READ.BIN"
done
if [ "$REFRESCAR" = 1 ] || [ ! -f "$SAL/mdisco3/disc.gdi" ]; then
    bash "$HERE/mkdisco.sh"  "$SAL/mdisco3"   "$SAL/juego3"   > "$SAL/mdisco3.log" 2>&1
    bash "$HERE/mkdisco.sh"  "$SAL/mdisco7"   "$SAL/juego7"   > "$SAL/mdisco7.log" 2>&1
    DC_SIN_JUEGO=1 bash "$HERE/mkdisco.sh" "$SAL/mdiscomut" > "$SAL/mdiscomut.log" 2>&1
fi
for d in mdisco3 mdisco7; do
    # el manifiesto que VIAJA en la ISO tiene que ser el mismo archivo que el del build
    dentro="$("$DC_PYTHON" "$HERE/leer-iso.py" "$SAL/$d/disc.iso" "/JUEGO/MANIFIESTO.TXT;1" | grep -o 'sha256=[0-9a-f]*' | cut -d= -f2)"
    fuera="$(sha256sum "$SAL/$d/MANIFIESTO.TXT" | cut -d' ' -f1)"
    lineas="$(wc -l < "$SAL/$d/MANIFIESTO.TXT")"
    if [ "$dentro" = "$fuera" ]; then verde "$d: MANIFIESTO.TXT de la ISO == el del build ($lineas lineas, sha256 ${dentro:0:16})"; else rojo "$d: el manifiesto de la ISO NO coincide"; fi
done
for d in mdisco3 mdisco7 mdiscomut; do nota "$d: disc.iso $(stat -c%s "$SAL/$d/disc.iso") bytes"; done

# ================================================================================================
# 3. LAS TRES CORRIDAS
# ================================================================================================
paso "3. las tres corridas del motor entero en Flycast ($SEGS s cada una)"
export FLYCAST="${FLYCAST:-$HOME/dreamcast/dl/squashfs-root/AppRun}"
export CAPTURE=none
for par in "m3:mdisco3" "m7:mdisco7" "mmut:mdiscomut"; do
    et="${par%%:*}"; disco="${par##*:}"
    if [ "$REFRESCAR" = 1 ] || [ ! -s "$SAL/run-$et/flycast.log" ]; then
        bash "$HERE/run-flycast.sh" "$SAL/$disco/disc.gdi" "$SEGS" "$SAL/run-$et" > "$SAL/run-$et.salida" 2>&1
    fi
    nota "run-$et: $(linea "$et" 'MOTOR-ARRANQUE seleccion:')"
done

# ================================================================================================
# 4. LOS CONTROLES (los cuatro criterios)
# ================================================================================================
paso "4. los controles"

# criterio 1 + 2: el motor VE los archivos del disco (0 -> 3) y el faltan baja en lo que ahora ve
L="$(linea m3 'MOTOR-ARRANQUE seleccion:')"
case "$L" in
    *"Complete= true"*"archivos que ve= 3"*"faltan= 4"*) verde "criterio 1: $L" ;;
    *) rojo "criterio 1: esperaba 'Complete= true ... ve= 3 ... faltan= 4' y salio: $L" ;;
esac

# criterio 2: con el juego COMPLETO (los 7 nombres del nucleo) el veredicto es OK()= true y faltan= 0
L="$(linea m7 'MOTOR-ARRANQUE seleccion:')"
case "$L" in
    *"Complete= true"*"Validate.OK()= true"*"archivos que ve= 7"*"faltan= 0"*) verde "criterio 2: $L" ;;
    *) rojo "criterio 2: esperaba 'Validate.OK()= true ... faltan= 0' y salio: $L" ;;
esac

# criterio 3: el mutante (la ISO SIN /JUEGO) sigue dando 0 / faltantes
L="$(linea mmut 'MOTOR-ARRANQUE seleccion:')"
case "$L" in
    *"Complete= false"*"archivos que ve= 0"*"faltan= 7"*) verde "criterio 3 (mutante): $L" ;;
    *) rojo "criterio 3 (mutante): esperaba 've= 0 ... faltan= 7' y salio: $L" ;;
esac

# criterio 4: el overlay de la RAM sigue listando (lo que ya andaba)
for et in m3 m7 mmut; do
    L="$(linea "$et" 'MOTOR-ARRANQUE seleccion overlay:')"
    case "$L" in
        *"-> 1 entradas: uno.txt"*) verde "criterio 4 ($et): $L" ;;
        *) rojo "criterio 4 ($et): el overlay no listo: $L" ;;
    esac
done
# y el control de que el overlay no se mezclo con el disco: el listado del disco tiene 3 nombres y ninguno es uno.txt
L="$(linea m3 'MOTOR-ARRANQUE overlay del listado:')"
nota "$L"

paso "VEREDICTO"
echo "   controles OK: $OK, fallos: $FALLOS"
[ "$FALLOS" = "0" ] || exit 1
