#!/bin/bash
# CORPUS DE DC-002.27: LOS ASSETS DEL JUEGO EN EL DISCO (la carpeta /JUEGO por ISO9660, leidos en /cd).
#
# QUE MIDE, EN LOS TRES NIVELES QUE HACEN FALTA (host, consola y motor):
#   HOST:  el MPQ que viaja DENTRO de la ISO es el mismo archivo del PC (sha256) y la config del arranque
#          dice /cd/juego con los tres nombres. Ademas deja los numeros de referencia de la suma de bytes.
#   C:     el camino pelado de KOS (POC disco-c): el drive ve el GD-ROM, `fs_open` abre el MPQ del disco y
#          los primeros bytes son los del MPQ (4d 50 51 1a), no otra cosa.
#   GO:    el mismo disco visto por el `os` de la consola y por los CARGADORES DEL MOTOR (POC motor-disco):
#          `os.Stat` (el respaldo por apertura de esta tarjeta), `d2setup.Complete`, `d2mpq.New`,
#          `d2mpq.FromFile` y `mpq.NewSource`; y la suma de los bytes leidos del disco contra la del PC.
#   MOTOR: el arranque de verdad ya no se queda en el cartel de los MPQ.
#
# CONTROLES (la imagen CON los MPQ):
#  1. los 3 MPQ leidos de la ISO dan el sha256 del archivo del PC
#  2. la config del arranque dice /cd/juego y los 3 nombres
#  3. la consola lee los 3 MPQ: la suma de los primeros y los ultimos 256 KiB coincide con la del PC
#  4. la sonda del driver (C) abre el MPQ y sus primeros bytes son 4d 50 51 1a
#  5. `os.Stat` da el tamano y `d2setup.Complete` da true con los 3 archivos
#  6. los cargadores del motor (d2mpq.New / FromFile / mpq.NewSource) abren el MPQ del disco
#  7. el motor NO dice "game files not found" ni "file not found" (la pared de los MPQ se corrio)
#
# MUTANTES (tienen que CAZARSE):
#  A. DC_SIN_JUEGO=1: la ISO sin la carpeta /JUEGO -> el cartel "game files not found at /cd/juego" vuelve.
#  B. MKISO_JOLIET=3: la ISO con el arbol Joliet -> TODO /cd contesta ENOENT (el defecto 1 de la tarjeta).
#  C. (aparte, MUTANTE_KOS=1) scripts/console/fix-kos-iso9660-sesion.py --revertir + recompilar KOS ->
#     el MPQ se lee corrido 45.000 sectores (41 41 85 86...). Toca el KOS vendorizado: va con bandera.
#
# Uso: bash scripts/console/checar-disco-227.sh [dir_gox] [dir_mpq] [segundos_motor]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-222}"
MPQ="${2:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
SEGS="${3:-60}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
SAL="${SAL:-/home/dgalli/.hermes/cache/scratch/corpus-227}"

export GODC_GOX="$GEN"
mkdir -p "$SAL"

OK=0
FALLOS=0
nota() { echo "   $*"; }
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }

sucio() {  # (archivo, patron) -> cuantas veces aparece
    grep -ac "$2" "$1" 2>/dev/null | head -1
}

# ================================================================================================
# 0. LA CARPETA QUE SE HORNEA Y EL ESTADO DE LOS PARCHES DE LOS QUE DEPENDE
# ================================================================================================
echo "== 0. la carpeta del juego (enlaces duros: no copia 515 MB) y los parches =="
CONF_TMP="$SAL/config-tmp.json"
mkdir -p "$SAL/juego"

if ! (cd "$REPO" && go run ./console/dc/gen-config-consola -datos "$SAL/juego-vacio" -config "$CONF_TMP" \
        -ruta-consola /cd/juego -mpq patch_d2.mpq,d2data.mpq,d2char.mpq -sin-sellos) > "$SAL/config-tmp.log" 2>&1; then
    echo "   el generador de la configuracion FALLO (ver $SAL/config-tmp.log)"; exit 1
fi

for n in $("$DC_PYTHON" -c 'import json,sys; print(" ".join(json.load(open(sys.argv[1]))["MpqLoadOrder"]))' "$CONF_TMP"); do
    # OJO: el nombre de la config va en minusculas (patch_d2.mpq) y en el PC el archivo puede estar con
    # otra caja (patch_D2.mpq): se busca sin distinguir mayusculas y se enlaza con el nombre de la config.
    real="$(ls -1 "$MPQ" | grep -ix "$n" | head -1)"
    [ -n "$real" ] || { echo "   no encuentro $n en $MPQ"; exit 1; }

    ln -f "$MPQ/$real" "$SAL/juego/$n" 2>/dev/null || cp "$MPQ/$real" "$SAL/juego/$n"
done

nota "MPQ horneados: $(ls "$SAL/juego" | tr '\n' ' ') ($(du -sb "$SAL/juego" | cut -f1) bytes)"

if [ "$("$DC_PYTHON" "$HERE/fix-kos-iso9660-sesion.py" --ver | grep -c PARCHEADO)" = "1" ]; then
    verde "control 0: el driver iso9660 de KOS tiene el parche de la sesion de alta densidad"
else
    rojo "control 0: falta el parche de KOS (scripts/console/fix-kos-iso9660-sesion.py + make -C \$KOS_BASE)"
fi

# ================================================================================================
# 1. LA IMAGEN CON EL JUEGO (POC del motor) + los controles del HOST
# ================================================================================================
echo "== 1. la imagen CON los MPQ (POC motor-disco) y los controles del host =="
A="$SAL/con-juego"
MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO=patch_d2.mpq,d2data.mpq,d2char.mpq \
    DC_SIN_SELLOS=1 DC_DISCO_JUEGO="$SAL/juego" POC=motor-disco \
    bash "$HERE/build-motor-arranque.sh" "$A" "$GEN" > "$A.build.log" 2>&1 ||
    { echo "   EL POC motor-disco NO COMPILA/ARMA (ver $A.build.log)"; exit 1; }

nota "ISO: $(stat -c%s "$A/disc.iso") bytes  ($(grep -a 'DISCO: ' "$A.build.log" | tail -1))"

for n in patch_D2.mpq d2data.mpq d2char.mpq; do
    MAYUS="$(echo "$n" | tr '[:lower:]' '[:upper:]')"
    dentro=$("$DC_PYTHON" "$HERE/leer-iso.py" "$A/disc.iso" "/JUEGO/$MAYUS;1" | grep -o 'sha256=[0-9a-f]*' | cut -d= -f2)
    real="$(ls -1 "$MPQ" | grep -ix "$n" | head -1)"
    fuera=$(sha256sum "$MPQ/$real" | awk '{print $1}')

    if [ -n "$dentro" ] && [ "$dentro" = "$fuera" ]; then
        verde "control 1: $n dentro de la ISO = el del PC (${dentro:0:16}...)"
    else
        rojo "control 1: $n dentro=$dentro PC=$fuera"
    fi
done

if (cd "$REPO" && go run ./console/dc/gen-config-consola -datos "$SAL/juego" \
        -config "$A/romdisk/OpenDiablo2/config.json" -ruta-consola /cd/juego \
        -mpq patch_d2.mpq,d2data.mpq,d2char.mpq -verificar) > "$SAL/config-verif.log" 2>&1; then
    verde "control 2: la config del arranque dice /cd/juego con los 3 nombres"
else
    rojo "control 2: la config del arranque no cierra (ver $SAL/config-verif.log)"
fi

# ================================================================================================
# 2. LAS SONDAS DE LA CONSOLA (C puro y Go) sobre el mismo disco
# ================================================================================================
echo "== 2. las sondas de la consola (driver, os y cargadores del motor) =="
C="$SAL/disco-c"
bash "$HERE/build-c.sh" "$REPO/console/dc/poc/disco-c" "$C" > "$C.build.log" 2>&1 ||
    { echo "   el POC disco-c NO COMPILA (ver $C.build.log)"; exit 1; }
MKISO_JOLIET=0 bash "$HERE/mkdisco.sh" "$C" "$SAL/juego" >> "$C.build.log" 2>&1
CAPTURE=none bash "$HERE/run-flycast.sh" "$C/disc.gdi" 25 "$SAL/run-c" >/dev/null 2>&1
LC="$SAL/run-c/flycast.log"

if grep -aq 'DISCO2-C fs_total(/cd/JUEGO/PATCH_D2.MPQ) -> 5022588' "$LC" &&
   grep -aq 'DISCO2-C fs_read(2048) -> 2048 errno=16 bytes: 4d 50 51 1a' "$LC"; then
    verde "control 4: el driver abre el MPQ del disco y lee su cabecera (4d 50 51 1a)"
else
    rojo "control 4: el driver no lee el MPQ del disco (ver $LC)"
fi

CAPTURE=none bash "$HERE/run-flycast.sh" "$A/disc.gdi" "$SEGS" "$SAL/run-motor" >/dev/null 2>&1
LM="$SAL/run-motor/flycast.log"

# control 3: la suma de los bytes leidos del disco contra la del PC (los 3 MPQ, los dos extremos)
suma_ok=0
for caso in PATCH_D2 D2DATA D2CHAR; do
    pc="$(echo "$caso" | tr '[:upper:]' '[:lower:]')"
    [ "$pc" = "patch_d2" ] && pc="patch_D2"

    host=$("$DC_PYTHON" "$HERE/suma-mpq.py" "$MPQ/$pc.mpq" 262144)
    h1=$(echo "$host" | awk 'NR==1{print $NF}' | cut -d= -f2)
    h2=$(echo "$host" | awk 'NR==2{print $NF}' | cut -d= -f2)
    # head -1: el arranque de la consola puede repetir el bloque en el log (dos vueltas del motor), asi que
    # se toma la PRIMERA linea que aparece
    c1=$(grep -a "DISCO-CONSOLA suma principio /cd/JUEGO/$caso.MPQ" "$LM" | tr -d '\r' | grep -o 'suma= [0-9]*' | awk '{print $2}' | head -1)
    c2=$(grep -a "DISCO-CONSOLA suma final /cd/JUEGO/$caso.MPQ" "$LM" | tr -d '\r' | grep -o 'suma= [0-9]*' | awk '{print $2}' | head -1)

    if [ -n "$c1" ] && [ "$c1" = "$h1" ] && [ "$c2" = "$h2" ]; then
        suma_ok=$((suma_ok + 1))
        nota "$caso: consola $c1/$c2 = host $h1/$h2"
    else
        nota "$caso: consola ${c1:-?}/${c2:-?}  host $h1/$h2  (no coincide)"
    fi
done

if [ "$suma_ok" = "3" ]; then
    verde "control 3: los 3 MPQ leidos del disco dan la misma suma que los del PC (los dos extremos)"
else
    rojo "control 3: solo $suma_ok de 3 MPQ coinciden con el PC"
fi

if grep -aq 'DISCO-CONSOLA stat /cd/juego/patch_d2.mpq bytes= 5022588' "$LM" &&
   grep -aq 'MOTOR-DISCO seleccion: Complete= true' "$LM" &&
   grep -aq 'DISCO-CONSOLA stat /cd/juego/no-existe.mpq FALLO' "$LM"; then
    verde "control 5: os.Stat del MPQ da 5.022.588 y Complete=true (el control negativo sigue fallando)"
else
    rojo "control 5: os.Stat o Complete no dan lo medido (ver $LM)"
fi

if grep -aq 'CARGADOR-CONSOLA d2mpq.New OK' "$LM" &&
   grep -aq 'CARGADOR-CONSOLA d2mpq.FromFile OK' "$LM" &&
   grep -aq 'CARGADOR-CONSOLA mpq.NewSource OK' "$LM"; then
    verde "control 6: los cargadores del MOTOR abren el MPQ del disco (New, FromFile y NewSource)"
else
    rojo "control 6: algun cargador del motor NO abre el MPQ del disco (ver $LM)"
fi

# El motor tiene DOS mensajes distintos que empiezan con "file not found": el suyo (d2loader, "Checked <ruta>,
# file not found", que es un asset que no esta adentro del MPQ) y el del arranque sin las fuentes del juego
# (d2app/initialization.go, "file not found: <ruta>" con la ruta del MPQ). Aca se exige el de la RUTA DEL MPQ:
# el otro es del tramo siguiente (un pedido de asset adentro del MPQ) y no dice nada de /cd.
n_cartel=$(sucio "$LM" 'game files not found at "/cd/juego"')
n_err=$(sucio "$LM" 'file not found: "/cd/juego')

if [ "$n_cartel" -eq 0 ] && [ "$n_err" -eq 0 ]; then
    verde "control 7: el motor no se queda en el cartel de los MPQ (0 ocurrencias)"
else
    rojo "control 7: el motor sigue sin ver el juego (cartel=$n_cartel file-not-found=$n_err)"
fi

nota "el tramo siguiente, medido: $(grep -a 'fatal error' "$LM" | tail -1 | tr -d '\r')"

# ================================================================================================
# 3. LOS MUTANTES
# ================================================================================================
echo "== 3. mutante A: la misma imagen SIN la carpeta /JUEGO =="
B="$SAL/sin-juego"
MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO=patch_d2.mpq,d2data.mpq,d2char.mpq \
    DC_SIN_SELLOS=1 DC_SIN_JUEGO=1 POC=motor-disco \
    bash "$HERE/build-motor-arranque.sh" "$B" "$GEN" > "$B.build.log" 2>&1 ||
    { echo "   el mutante A NO COMPILA/ARMA (ver $B.build.log)"; exit 1; }

CAPTURE=none bash "$HERE/run-flycast.sh" "$B/disc.gdi" 40 "$SAL/run-mutante-a" >/dev/null 2>&1
LB="$SAL/run-mutante-a/flycast.log"

if [ "$(sucio "$LB" 'game files not found at "/cd/juego"')" -ge 1 ] &&
   [ "$(sucio "$LB" 'DISCO-CONSOLA stat /cd/juego/patch_d2.mpq bytes= 5022588')" -eq 0 ]; then
    verde "mutante A cazado: sin /JUEGO vuelve el cartel y el Stat ya no da el tamano"
else
    rojo "mutante A NO cazado (el corpus no discrimina)"
fi

echo "== 3b. mutante B: la ISO con el arbol Joliet (el defecto 1 de la tarjeta) =="
J="$SAL/con-joliet"
MKISO_JOLIET=3 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO=patch_d2.mpq,d2data.mpq,d2char.mpq \
    DC_SIN_SELLOS=1 POC=motor-disco \
    bash "$HERE/build-motor-arranque.sh" "$J" "$GEN" > "$J.build.log" 2>&1 ||
    { echo "   el mutante B NO COMPILA/ARMA (ver $J.build.log)"; exit 1; }

CAPTURE=none bash "$HERE/run-flycast.sh" "$J/disc.gdi" 25 "$SAL/run-mutante-b" >/dev/null 2>&1
LJ="$SAL/run-mutante-b/flycast.log"

if [ "$(sucio "$LJ" 'DISCO-CONSOLA stat /cd/juego/patch_d2.mpq bytes= 5022588')" -eq 0 ] &&
   [ "$(sucio "$LJ" 'DISCO-CONSOLA stat /cd/juego/patch_d2.mpq FALLO')" -ge 1 ]; then
    verde "mutante B cazado: con Joliet el driver no encuentra NINGUNA ruta de /cd"
else
    rojo "mutante B NO cazado (el corpus no discrimina el arbol Joliet)"
fi

if [ "${MUTANTE_KOS:-0}" = "1" ]; then
    echo "== 3c. mutante C: el driver de KOS SIN el parche de la sesion =="
    "$DC_PYTHON" "$HERE/fix-kos-iso9660-sesion.py" --revertir
    (cd "$KOS_BASE" && . ./environ.sh > /dev/null 2>&1; make -j10) > "$SAL/kos-revert.log" 2>&1

    K="$SAL/sin-parche-kos"
    MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO=patch_d2.mpq,d2data.mpq,d2char.mpq \
        DC_SIN_SELLOS=1 POC=motor-disco \
        bash "$HERE/build-motor-arranque.sh" "$K" "$GEN" > "$K.build.log" 2>&1

    CAPTURE=none bash "$HERE/run-flycast.sh" "$K/disc.gdi" 30 "$SAL/run-mutante-c" >/dev/null 2>&1
    LK="$SAL/run-mutante-c/flycast.log"

    if grep -aq 'CARGADOR-CONSOLA primeros 16 bytes: 41 41 85 86' "$LK"; then
        verde "mutante C cazado: sin el parche el MPQ se lee corrido (41 41 85 86)"
    else
        rojo "mutante C NO cazado (ver $LK)"
    fi

    "$DC_PYTHON" "$HERE/fix-kos-iso9660-sesion.py"
    (cd "$KOS_BASE" && . ./environ.sh > /dev/null 2>&1; make -j10) > "$SAL/kos-restaurar.log" 2>&1
else
    nota "mutante C no corrido (MUTANTE_KOS=1 lo corre: revierte el parche de KOS y recompila; medido aparte: 41 41 85 86)"
fi

echo
echo "== RESULTADO: $OK controles OK, $FALLOS con fallo =="
[ "$FALLOS" -eq 0 ]
