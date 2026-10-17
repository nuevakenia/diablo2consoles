#!/bin/bash
# CORPUS DE DC-002.27: LOS ASSETS DEL JUEGO EN EL DISCO (la carpeta /JUEGO por ISO9660, leida en /cd).
#
# QUE MIDE (y por que en estos dos niveles):
#
#   HOST:  el MPQ que viaja DENTRO de la ISO es el mismo archivo del PC (sha256, los tres), y la config
#          del arranque que va al romdisk dice /cd/juego con los tres nombres.
#   C:     el camino pelado de KOS (POC disco-c): el drive contesta GD-ROM y `fs_open` de la ruta del MPQ
#          abre con su tamano exacto. Es el nivel del driver, sin Go en el medio.
#   GO:    el mismo disco visto por el `os` de la consola (POC motor-disco): `os.Stat` (el respaldo por
#          apertura que arregla esta tarjeta) y la seleccion de archivos del MOTOR (`d2setup.Complete`).
#   MOTOR: el arranque de verdad: el cartel "game files not found at /cd/juego" NO aparece.
#
# CONTROLES (la imagen CON los MPQ):
#  1. los 3 MPQ leidos de la ISO dan el sha256 del archivo del PC
#  2. la config del arranque dice /cd/juego y los 3 nombres (y el generador lo verifica con el json de Go)
#  3. la sonda del driver (C) abre /cd/JUEGO/PATCH_D2.MPQ y dice su tamano: 5.022.588 bytes
#  4. la sonda de Go: os.Stat de /cd/juego/patch_d2.mpq da 5.022.588 bytes y Complete=true con 3 archivos
#  5. el motor NO dice "game files not found at /cd/juego" y ENTRA a su arranque
#
# MUTANTE (tiene que CAZARSE):
#  A. DC_SIN_JUEGO=1: la ISO sin la carpeta /JUEGO -> la config nombra archivos que no estan y el motor
#     TIENE que volver a su cartel "game files not found at /cd/juego" (es el estado de DC-002.26).
#
# Uso: bash scripts/console/checar-disco-227.sh [dir_gox] [dir_mpq] [segundos_motor]
#      dir_mpq: donde estan los MPQ del juego en el PC (por defecto, el prefijo de wine del port)
#      (la carpeta que se hornea se arma con ENLACES DUROS a los MPQ: no copia 515 MB)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-222}"
MPQ="${2:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
SEGS="${3:-75}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
SAL="${SAL:-/home/dgalli/.hermes/cache/scratch/corpus-227}"

export GODC_GOX="$GEN"
mkdir -p "$SAL"

OK=0
FALLOS=0
nota() { echo "   $*"; }
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }

# ================================================================================================
# 0. LA CARPETA QUE SE HORNEA: los MPQ que la config del arranque nombra, enlazados (no copiados)
# ================================================================================================
echo "== 0. la carpeta del juego que se hornea (enlaces duros a los MPQ del PC) =="
CONF_TMP="$SAL/config-tmp.json"
mkdir -p "$SAL/juego"

if ! go run ./console/dc/gen-config-consola -datos "$SAL/juego-vacio" -config "$CONF_TMP" \
        -ruta-consola /cd/juego -mpq patch_d2.mpq,d2data.mpq,d2char.mpq -sin-sellos > "$SAL/config-tmp.log" 2>&1; then
    echo "   el generador de la configuracion FALLO (ver $SAL/config-tmp.log)"; exit 1
fi

for n in $("$DC_PYTHON" -c 'import json,sys; print(" ".join(json.load(open(sys.argv[1]))["MpqLoadOrder"]))' "$CONF_TMP"); do
    ln -f "$MPQ/$n" "$SAL/juego/$n" 2>/dev/null || cp "$MPQ/$n" "$SAL/juego/$n"
done

nota "MPQ horneados: $(ls "$SAL/juego" | tr '\n' ' ')"
nota "bytes: $(du -sb "$SAL/juego" | cut -f1)"

# ================================================================================================
# 1. LA IMAGEN CON EL JUEGO (el POC del motor) y el control del HOST
# ================================================================================================
echo "== 1. la imagen CON los MPQ (POC motor-disco) =="
A="$SAL/con-juego"
MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO=patch_d2.mpq,d2data.mpq,d2char.mpq \
    DC_SIN_SELLOS=1 DC_DISCO_JUEGO="$SAL/juego" POC=motor-disco \
    bash "$HERE/build-motor-arranque.sh" "$A" "$GEN" > "$A.build.log" 2>&1 ||
    { echo "   EL POC motor-disco NO COMPILA/ARMA (ver $A.build.log)"; exit 1; }

nota "ISO: $(stat -c%s "$A/disc.iso") bytes  ($(grep -a 'DISCO: ' "$A.build.log" | tail -1))"

# control 1: la suma del MPQ que viaja en la ISO contra el del PC
for n in patch_D2.mpq d2data.mpq d2char.mpq; do
    MAYUS="$(echo "$n" | tr '[:lower:]' '[:upper:]')"
    dentro=$("$DC_PYTHON" "$HERE/leer-iso.py" "$A/disc.iso" "/JUEGO/$MAYUS;1" | awk '{print $4}')
    fuera=$(sha256sum "$MPQ/$n" | awk '{print $1}')

    if [ -n "$dentro" ] && [ "$dentro" = "$fuera" ]; then
        verde "control 1: $n dentro de la ISO = el del PC ($dentro)"
    else
        rojo "control 1: $n dentro=$dentro  PC=$fuera"
    fi
done

# control 2: la config del arranque (el generador, con el json de Go, exige ruta y nombres)
if go run ./console/dc/gen-config-consola -datos "$SAL/juego" -config "$A/romdisk/OpenDiablo2/config.json" \
        -ruta-consola /cd/juego -mpq patch_d2.mpq,d2data.mpq,d2char.mpq -verificar > "$SAL/config-verif.log" 2>&1; then
    verde "control 2: la config del arranque dice /cd/juego con los 3 nombres"
else
    rojo "control 2: la config del arranque no cierra (ver $SAL/config-verif.log)"
fi

nota "$(grep -a 'CONFIG-CONSOLA verificacion:' "$SAL/config-verif.log" | tail -1)"

# ================================================================================================
# 2. LA SONDA DEL DRIVER (C puro) y LA DEL OS/MOTOR (Go), sobre el MISMO disco
# ================================================================================================
echo "== 2. las sondas de la consola sobre el disco con juego =="
C="$SAL/disco-c"
bash "$HERE/build-c.sh" "$REPO/console/dc/poc/disco-c" "$C" > "$C.build.log" 2>&1 ||
    { echo "   el POC disco-c NO COMPILA (ver $C.build.log)"; exit 1; }
MKISO_JOLIET=0 bash "$HERE/mkdisco.sh" "$C" "$SAL/juego" >> "$C.build.log" 2>&1
CAPTURE=none bash "$HERE/run-flycast.sh" "$C/disc.gdi" 25 "$SAL/run-c" >/dev/null 2>&1
LC="$SAL/run-c/flycast.log"

if grep -aq 'DISCO2-C cdrom_get_status -> 0 errno=0 status=0x01 disc=0x80' "$LC" &&
   grep -aq 'DISCO2-C fs_open(/cd/JUEGO/PATCH_D2.MPQ) -> [0-9]' "$LC" &&
   grep -aq 'DISCO2-C fs_total(/cd/JUEGO/PATCH_D2.MPQ) -> 5022588' "$LC"; then
    verde "control 3: el driver ve el GD-ROM y abre el MPQ del disco con su tamano (5.022.588)"
else
    rojo "control 3: el driver no abre el MPQ del disco (ver $LC)"
fi

nota "$(grep -a 'DISCO2-C fs_open(/cd/JUEGO/PATCH_D2.MPQ)' "$LC" | tail -1 | tr -d '\r')"

CAPTURE=none bash "$HERE/run-flycast.sh" "$A/disc.gdi" "$SEGS" "$SAL/run-motor" >/dev/null 2>&1
LM="$SAL/run-motor/flycast.log"

n_stat=$(grep -ac 'DISCO-CONSOLA stat /cd/juego/patch_d2.mpq bytes= 5022588' "$LM")
n_complete=$(grep -ac 'MOTOR-DISCO seleccion: Complete= true .*archivos que ve= 3' "$LM")

if [ "$n_stat" -ge 1 ]; then
    verde "control 4a: os.Stat del MPQ del disco da 5.022.588 bytes (el respaldo por apertura)"
else
    rojo "control 4a: os.Stat del MPQ del disco no dio el tamano (medi $n_stat)"
fi

if [ "$n_complete" -ge 1 ]; then
    verde "control 4b: la seleccion del MOTOR sobre /cd/juego esta completa (3 archivos)"
else
    rojo "control 4b: la seleccion del MOTOR sobre /cd/juego no esta completa (medi $n_complete)"
fi

nota "$(grep -a 'MOTOR-DISCO seleccion:' "$LM" | tail -1 | tr -d '\r')"
nota "$(grep -a 'DISCO-CONSOLA suma principio' "$LM" | head -1 | tr -d '\r')"

n_cartel=$(grep -ac 'game files not found at "/cd/juego"' "$LM")
n_file_err=$(grep -ac 'file not found: "/cd/juego' "$LM")

if [ "$n_cartel" -eq 0 ] && [ "$n_file_err" -eq 0 ]; then
    verde "control 5: el motor NO dice que no encuentra los archivos del juego (se fue del cartel)"
else
    rojo "control 5: el motor sigue sin ver los archivos (cartel=$n_cartel file-not-found=$n_file_err)"
fi

nota "$(grep -a 'MOTOR-DISCO Run() termino con error\|MOTOR-DISCO Run() volvio' "$LM" | tail -1 | tr -d '\r')"

# ================================================================================================
# 3. EL MUTANTE: la ISO SIN la carpeta del juego -> el cartel tiene que VOLVER
# ================================================================================================
echo "== 3. el mutante: la misma imagen SIN /JUEGO =="
B="$SAL/sin-juego"
MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO=patch_d2.mpq,d2data.mpq,d2char.mpq \
    DC_SIN_SELLOS=1 DC_SIN_JUEGO=1 POC=motor-disco \
    bash "$HERE/build-motor-arranque.sh" "$B" "$GEN" > "$B.build.log" 2>&1 ||
    { echo "   el mutante NO COMPILA/ARMA (ver $B.build.log)"; exit 1; }

CAPTURE=none bash "$HERE/run-flycast.sh" "$B/disc.gdi" 40 "$SAL/run-mutante" >/dev/null 2>&1
LB="$SAL/run-mutante/flycast.log"

n_cartel_mut=$(grep -ac 'game files not found at "/cd/juego"' "$LB")
n_inyectado=$(grep -ac 'DISCO-CONSOLA stat /cd/juego/patch_d2.mpq bytes= 5022588' "$LB")

if [ "$n_cartel_mut" -ge 1 ] && [ "$n_inyectado" -eq 0 ]; then
    verde "mutante A: sin /JUEGO el motor vuelve al cartel de los archivos ($n_cartel_mut) y el Stat ya no da el tamano"
else
    rojo "mutante A NO cazado: cartel=$n_cartel_mut stat-con-tamano=$n_inyectado (el corpus no discrimina)"
fi

echo
echo "== RESULTADO: $OK controles/mutantes OK, $FALLOS con fallo =="
[ "$FALLOS" -eq 0 ]
