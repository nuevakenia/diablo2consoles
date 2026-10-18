#!/bin/bash
# CORPUS DE DC-002.30: LA RESOLUCION DEL MOTOR -- palanca de RAM o de VRAM?
#
# QUE MIDE (host + consola). La tarjeta nacio de una hipotesis: "la resolucion de la pantalla logica del
# motor (800x600 -> 320x240 = -84%) como palanca de RAM para el arranque". Lo que este corpus exige:
#
#  1. HOST: la tabla de lo que el MOTOR resuelve (`d2common/d2display.Resolve`, el mismo codigo del motor)
#     para cada configuracion (1512x982 / 800x600 / 640x480 / 320x240) y cada pantalla del renderer
#     (640x480, la del POC, y 320x240): TODAS tienen que resolver a 800x600. Es el PISO de `Clamp`
#     (BaseW/BaseH): el campo `Display` del config no puede bajar la pantalla del motor.
#  2. HOST: `gen-config-consola -pantalla WxH` SI escribe la resolucion pedida en el config.json del
#     romdisk (`Display`), y el sha256 del archivo CAMBIA respecto del de siempre (declarado); el
#     `-verificar` con los sellos puestos tiene que quedar verde.
#  3. CONSOLA: el arranque del motor con la config en 800x600 y el instrumento de DC-002.27/28 (el libre
#     del monton de C, las marcas, la ultima marca, el PC de la excepcion, las recolecciones del GC):
#     el punto donde muere, medido.
#  4. CONSOLA: lo MISMO con la config en 320x240: el display resuelto y el punto de RAM tienen que ser
#     IDENTICOS a los del control 3. Si el punto se mueve, la resolucion es palanca de RAM (la hipotesis
#     de la tarjeta); si no se mueve, no lo es y hay que decirlo.
#  5. CONSOLA: el SHIM con la pantalla del PVR en 320x240 (una copia del POC con `IniciarPantalla(320,240)`)
#     contra la de 640x480: el cuadro del PVR SI baja en la VRAM (el numero), y el punto de RAM NO se mueve
#     (la pantalla vive en la VRAM desde DC-002.22).
#  7. LA PREGUNTA DE FONDO, medida (no exigida): con el piso de Clamp SACADO (el mutante puesto) y la
#     config en 320x240, el motor SI resuelve 320x240 -- y su punto de RAM se compara con el de 800x600.
#     Responde "¿y si el motor pudiera bajar la pantalla, seria palanca de RAM?".
#
# MUTANTE (tiene que CAZARSE): `scripts/console/mutar-pantalla-230.py --sin-piso` saca el piso de
# `d2common/d2display.Clamp` y el 320x240 SI aparece resuelto -> el control 1 tiene que ponerse ROJO.
# El corpus exige que el archivo quede con el mismo sha256 con el que empezo.
#
# OJO: la copia del POC del control 5 (`console/dc/poc/motor-disco-p320`) la crea ESTE script y la borra al
# terminar: no es una POC del repo, es el instrumento del corpus.
#
# Uso: bash scripts/console/checar-pantalla-230.sh [dir_gox] [segundos]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-330}"
SEGS="${2:-30}"
MPQ="${MPQ:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
REPO="$(cd "$HERE/../.." && pwd)"
SAL="${SAL:-/home/dgalli/.hermes/cache/scratch/corpus-230}"
JUEGO="${DC_DISCO_JUEGO:-/home/dgalli/.hermes/cache/scratch/corpus-227/juego}"
MUT="$HERE/mutar-pantalla-230.py"
DISPLAY_GO="$REPO/d2common/d2display/display.go"
POC320="$REPO/console/dc/poc/motor-disco-p320"
POC320_SRC="$REPO/console/dc/poc/motor-disco"

mkdir -p "$SAL"
OK=0
FALLOS=0
nota() { echo "   $*"; }
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }

# OJO (DC-002.30, medido): aca el mutante del piso se PONE Y SE LEVANTA varias veces. Si la corrida muere en
# el medio (pasa: la corrida anterior de esta tarjeta murio por credenciales con el mutante pegado), el arbol
# queda mutado para la proxima. El trap lo levanta SIEMPRE, sea cual sea la salida.
MUT_LEVANTADO=0
levanta_mutante() {
    if [ "$MUT_LEVANTADO" = "1" ]; then
        "$DC_PYTHON" "$MUT" --restaurar > /dev/null 2>&1 || true
        MUT_LEVANTADO=0
    fi
}
trap levanta_mutante EXIT

sha_archivo() { sha256sum "$1" | awk '{print $1}'; }

corre() {   # $1 nombre  $2 pantalla_del_config ("-" = la del motor)  $3 poc  $4 segundos  $5 layer (por defecto $GEN)
    local nom="$1" pant="$2" poc="$3" segs="$4" layer="${5:-$GEN}" out="$SAL/$1"
    local args=()
    [ "$pant" != "-" ] && args=(DC_PANTALLA="$pant")
    mkdir -p "$out"
    # la corrida vieja se borra ANTES del build: si el build falla, `mide` no puede leer un log de la corrida
    # anterior y dar por bueno un numero que es de otro binario (leccion de DC-002.29/230).
    rm -rf "$out-run"
    env MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO=patch_d2.mpq,d2data.mpq,d2char.mpq \
        DC_SIN_SELLOS=1 DC_DISCO_JUEGO="$JUEGO" POC="$poc" "${args[@]}" \
        bash "$HERE/build-motor-arranque.sh" "$out" "$layer" > "$out.build.log" 2>&1 || {
        echo "   el POC $poc no compila/enlaza (ver $out.build.log)"; return 1; }
    CAPTURE=none bash "$HERE/run-flycast.sh" "$out/disc.gdi" "$segs" "$out-run" > "$out-run.out" 2>&1
}

mide() {    # $1 nombre -> CFG_ DISPLAY_ VRAM_ FB_ RAM_ MARCAS_ MAIN_ GC_ ULTIMA_ PC_
    local T
    if [ ! -f "$SAL/$1-run/flycast.log" ]; then
        rojo "$1: no hay flycast.log (la corrida no salio)"
    elif [ "$SAL/$1-run/flycast.log" -ot "$SAL/$1.build.log" ]; then
        # el log es MAS VIEJO que el build: seria el de una corrida anterior (otro binario). Se dice, no se usa.
        rojo "$1: el flycast.log es MAS VIEJO que el build (numero de otra corrida)"
    fi
    T=$(tr -d '\r' < "$SAL/$1-run/flycast.log")
    CFG_=$(grep -ao 'display=[0-9]*x[0-9]*' "$SAL/$1/config-consola.log" 2>/dev/null | tail -1)
    DISPLAY_=$(printf '%s' "$T" | grep -ao 'display [0-9]*x[0-9]*' | tail -1)
    VRAM_=$(printf '%s' "$T" | grep -ao 'VRAM de la pantalla [0-9]*' | tail -1)
    FB_=$(printf '%s' "$T" | grep -ao 'framebuffer de la pantalla en la VRAM: [0-9]*' | tail -1)
    RAM_=$(printf '%s' "$T" | grep -ao 'monton de C antes del motor: [0-9]* bytes libres' | tail -1)
    MARCAS_=$(printf '%s' "$T" | grep -ac 'ARQ-MARCA')
    MAIN_=$(printf '%s' "$T" | grep -ac 'main.main-entra')
    GC_=$(printf '%s' "$T" | grep -ac 'gc_collect')
    ULTIMA_=$(printf '%s' "$T" | grep -a 'ARQ-MARCA' | tail -1 | sed 's/^ *//')
    PC_=$(printf '%s' "$T" | grep -ao 'Unhandled exception: PC [0-9a-f]*' | tail -1)
}

resumen() {     # $1 nombre
    echo "   [$1] config:$CFG_  display_resuelto:${DISPLAY_:-?}  $VRAM_  $FB_"
    echo "   [$1] $RAM_  marcas=$MARCAS_ main.main=$MAIN_ gc_collect=$GC_  $PC_"
    echo "   [$1] ultima marca: $ULTIMA_"
}

# ================================================================================================
# 0. la carpeta del juego (enlaces duros, como el corpus de DC-002.28) y el estado del mutante
# ================================================================================================
echo "== 0. la carpeta del juego y el estado del mutante =="
mkdir -p "$SAL/juego"
for n in patch_d2.mpq d2data.mpq d2char.mpq; do
    real="$(ls -1 "$MPQ" 2>/dev/null | grep -ix "$n" | head -1)"
    [ -n "$real" ] || { echo "   no encuentro $n en $MPQ"; exit 1; }
    [ -f "$SAL/juego/$n" ] || { ln -f "$MPQ/$real" "$SAL/juego/$n" 2>/dev/null || cp "$MPQ/$real" "$SAL/juego/$n"; }
done
nota "MPQ horneados: $(ls "$SAL/juego" | tr '\n' ' ')"
if [ "$("$DC_PYTHON" "$MUT" --estado | grep -c 'SACADO')" = "0" ]; then
    verde "control 0: el piso de Clamp esta puesto (el mutante NO esta pegado)"
else
    rojo "control 0: el mutante del piso esta PEGADO: correr $MUT --restaurar antes de medir"
fi
SHA_ANTES="$(sha256sum "$DISPLAY_GO" | awk '{print $1}')"

# ================================================================================================
# 1. HOST: lo que el MOTOR resuelve (la tabla de la resolucion)
# ================================================================================================
echo "== 1. host: lo que resuelve el MOTOR (d2common/d2display), config x pantalla del renderer =="
# el instrumento del host, con el MISMO codigo del motor. Se escribe aca (el corpus se arma solo) y se
# corre DENTRO del modulo del repo para que el import resuelva.
cat > "$SAL/host-resolucion.go" <<'EOF'
// instrumento del corpus de DC-002.30: lo que el MOTOR resuelve para la pantalla logica.
package main

import (
	"fmt"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2display"
)

func main() {
	for _, scr := range []d2display.Size{{W: 640, H: 480}, {W: 320, H: 240}} {
		for _, cfg := range []d2display.Size{{W: 1512, H: 982}, {W: 800, H: 600}, {W: 640, H: 480}, {W: 320, H: 240}} {
			s, err := d2display.Resolve("", cfg, scr)
			if err != nil {
				panic(err)
			}

			fmt.Printf("RESOLUCION  config=%dx%d  pantalla=%dx%d  ->  resuelto=%dx%d\n",
				cfg.W, cfg.H, scr.W, scr.H, s.W, s.H)
		}
	}
}
EOF
if (cd "$REPO" && go run "$SAL/host-resolucion.go") > "$SAL/host-resolucion.txt" 2>&1; then
    sed 's/^/     /' "$SAL/host-resolucion.txt"
    RESUELTOS=$(grep -c 'resuelto=800x600' "$SAL/host-resolucion.txt")
    TOTAL=$(grep -c 'RESOLUCION' "$SAL/host-resolucion.txt")
    [ "$RESUELTOS" = "$TOTAL" ] &&
        verde "control 1: las $TOTAL combinaciones (4 configs x 2 pantallas) resuelven a 800x600: el campo Display del config NO baja la pantalla del motor (el PISO de Clamp)" ||
        rojo "control 1: $((TOTAL - RESUELTOS)) de $TOTAL combinaciones NO resuelven a 800x600 (el piso no esta haciendo su trabajo)"
else
    rojo "control 1: no pude correr el instrumento del host (ver $SAL/host-resolucion.txt)"
fi

# ================================================================================================
# 2. HOST: el generador ESCRIBE la resolucion pedida (y el sha256 del config cambia)
# ================================================================================================
echo "== 2. host: gen-config-consola -pantalla escribe el Display (y el config cambia de sha256) =="
for p in 1512x982 800x600 320x240; do
    mkdir -p "$SAL/gen-$p/juego"
    (cd "$REPO" && go run ./console/dc/gen-config-consola -datos "$SAL/gen-$p/juego" -config "$SAL/gen-$p/config.json" \
        -ruta-consola /cd/juego -mpq patch_d2.mpq,d2data.mpq,d2char.mpq -sin-sellos -pantalla "$p") \
        > "$SAL/gen-$p.log" 2>&1
    SHA_P="$(grep -ao 'sha256=[0-9a-f]*' "$SAL/gen-$p.log" | tail -1 | cut -d= -f2)"
    DICHO="$(grep -ao 'display=[0-9]*x[0-9]*' "$SAL/gen-$p.log" | tail -1 | cut -d= -f2)"
    nota "pantalla pedida $p -> config display=$DICHO sha256=${SHA_P:0:16}"
    eval "SHA_$(echo "$p" | tr x _)=$SHA_P"
    [ "$DICHO" = "$p" ] || rojo "control 2: el config NO dice la pantalla pedida ($p -> $DICHO)"
done
if [ "${SHA_1512_982:-a}" != "${SHA_800_600:-b}" ] && [ "${SHA_800_600:-b}" != "${SHA_320_240:-c}" ]; then
    verde "control 2: la resolucion SI se escribe en el config y el sha256 cambia con ella (1512x982 ${SHA_1512_982:0:16} / 800x600 ${SHA_800_600:0:16} / 320x240 ${SHA_320_240:0:16})"
else
    rojo "control 2: el sha256 del config NO cambia con la resolucion"
fi
# y el -verificar con los sellos puestos: verde, con el Display que se pidio
mkdir -p "$SAL/gen-verif/juego"
(cd "$REPO" && go run ./console/dc/gen-config-consola -datos "$SAL/gen-verif/juego" -config "$SAL/gen-verif/config.json" \
    -ruta-consola /cd/juego -mpq patch_d2.mpq,d2data.mpq,d2char.mpq -pantalla 320x240) > "$SAL/gen-verif.log" 2>&1
(cd "$REPO" && go run ./console/dc/gen-config-consola -datos "$SAL/gen-verif/juego" -config "$SAL/gen-verif/config.json" \
    -ruta-consola /cd/juego -mpq patch_d2.mpq,d2data.mpq,d2char.mpq -pantalla 320x240 -verificar) > "$SAL/gen-verificar.log" 2>&1
if [ "$(grep -c 'verificacion OK' "$SAL/gen-verificar.log")" = "1" ] &&
    [ "$(grep -ao 'display=[0-9]*x[0-9]*' "$SAL/gen-verificar.log" | tail -1)" = "display=320x240" ]; then
    verde "control 2b: el -verificar del host acepta el Display pedido ($(grep -ao 'display=[0-9]*x[0-9]*' "$SAL/gen-verificar.log" | tail -1))"
else
    rojo "control 2b: el -verificar no acepta el Display pedido (ver $SAL/gen-verificar.log)"
fi

# ================================================================================================
# 3 y 4. CONSOLA: el arranque con la config en 800x600 y en 320x240
# ================================================================================================
# el "punto de RAM" de una corrida: lo que NO tiene que moverse con la pantalla. Se arma con los campos
# medidos, SIN el config (que si cambia): display resuelto, libre del monton de C, las marcas, main.main,
# las recolecciones del GC, la ultima marca y el PC de la excepcion.
punto() { printf '%s|%s|%s|%s|%s|%s|%s' "$DISPLAY_" "$RAM_" "$MARCAS_" "$MAIN_" "$GC_" "$ULTIMA_" "$PC_"; }

# punto_ram: el MISMO punto SIN el display resuelto. Hace falta para el control 7: ahi el display ES distinto a
# proposito (es lo que se mide), y comparar con `punto` seria comparar el numero que tiene que cambiar. Lo que
# NO tiene que moverse es la RAM: el libre del monton de C y el punto donde muere el arranque.
punto_ram() { printf '%s|%s|%s|%s|%s|%s' "$RAM_" "$MARCAS_" "$MAIN_" "$GC_" "$ULTIMA_" "$PC_"; }

echo "== 3. consola: el arranque con la config en 800x600 (el instrumento de DC-002.27/28) =="
corre c3-800x600 800x600 motor-disco "$SEGS"
mide c3-800x600
resumen c3-800x600
P800="$(punto)"
PR800="$(punto_ram)"
CFG800="$CFG_"

# EL PEDIDO DE PANTALLA (criterio 2 de la tarjeta): la consola tiene que imprimir el numero nuevo. Desde
# DC-002.22 la pantalla del motor vive en la VRAM, asi que el numero es de VRAM y NO hay pedido al monton de
# C: el 1.916.928 de DC-002.21/22 (800x600x4 = 1.920.000, menos los 3.072 del sbrk) no tiene que aparecer.
L3="$(tr -d '\r' < "$SAL/c3-800x600-run/flycast.log")"
N_VIEJO="$(printf '%s' "$L3" | grep -c '1916928')"
N_OOM="$(printf '%s' "$L3" | grep -c 'out of memory')"
N_VRAM="$(printf '%s' "$L3" | grep -c 'VRAM de la pantalla')"
[ "$N_VIEJO" = "0" ] && [ "$N_OOM" = "0" ] && [ "$N_VRAM" = "1" ] &&
    verde "control 3d: la consola imprime el pedido de pantalla como VRAM ($VRAM_) y NO como monton de C: 0 lineas con el 1.916.928 de DC-002.21/22 y 0 OOM" ||
    rojo "control 3d: el pedido de pantalla del arranque no es el nuevo (1916928=$N_VIEJO out-of-memory=$N_OOM VRAM-de-la-pantalla=$N_VRAM)"

echo "== 4. consola: lo mismo con la config en 320x240 =="
corre c4-320x240 320x240 motor-disco "$SEGS"
mide c4-320x240
resumen c4-320x240
P320="$(punto)"
CFG320="$CFG_"

echo "== 3b. consola: y con la resolucion del motor (sin -pantalla: 1512x982, la de siempre) =="
corre c3b-1512x982 - motor-disco "$SEGS"
mide c3b-1512x982
resumen c3b-1512x982
P1512="$(punto)"
CFG1512="$CFG_"

# los CUATRO puntos de RAM comparados entre si (el config SI cambia; el punto no). El 640x480 es el que pide
# el PLAN de la tarjeta ("medir el arranque con 640x480"), y el 1512x982 es la resolucion del motor sin
# -pantalla (la de siempre): los cuatro tienen que dar el mismo punto.
echo "== 3c. consola: el arranque con 640x480 (el del PLAN) =="
corre c3c-640x480 640x480 motor-disco "$SEGS"
mide c3c-640x480
resumen c3c-640x480
P640="$(punto)"
CFG640="$CFG_"

if [ "$P800" = "$P320" ] && [ "$P800" = "$P1512" ] && [ "$P800" = "$P640" ]; then
    verde "control 3+4: el punto de RAM NO se mueve con la resolucion del config (800x600 / 640x480 / 320x240 / 1512x982 dan LO MISMO: '$P800')"
else
    rojo "control 3+4: el punto de RAM CAMBIA con la resolucion del config (800x600='$P800' 640x480='$P640' 320x240='$P320' 1512x982='$P1512')"
fi
# y los cuatro configs DICEN su numero (el instrumento prueba que el config SI se horneo distinto)
[ "$CFG800" = "display=800x600" ] && [ "$CFG640" = "display=640x480" ] && [ "$CFG320" = "display=320x240" ] &&
    [ "$CFG1512" = "display=1512x982" ] &&
    verde "control 3+4b: los cuatro configs horneados dicen lo que se les pidio ($CFG800 / $CFG640 / $CFG320 / $CFG1512): el unico que no cambia es el punto de RAM" ||
    rojo "control 3+4b: algun config no dice lo que se pidio ($CFG800 / $CFG640 / $CFG320 / $CFG1512)"
[ "$CFG320" = "display=320x240" ] && [ "$(printf '%s' "$P320" | grep -c '^display 800x600|')" = "1" ] &&
    verde "control 4b: el motor resuelve 800x600 CON LA CONFIG EN 320x240 ($CFG320 escrito en el romdisk -> '$P320')" ||
    rojo "control 4b: el motor NO resolvio 800x600 con la config en 320x240 (config $CFG320, punto '$P320')"

# ================================================================================================
# 5. CONSOLA: el SHIM con la pantalla del PVR en 320x240 (¿y ahi si se mueve algo?)
# ================================================================================================
echo "== 5. consola: el shim con la pantalla del PVR en 320x240 (copia del POC, se borra al final) =="
rm -rf "$POC320"
cp -a "$POC320_SRC" "$POC320"
sed -i 's/ebiten.IniciarPantalla(640, 480)/ebiten.IniciarPantalla(320, 240)/' "$POC320"/main.go
nota "la copia cambia UNA linea: $(grep -n 'IniciarPantalla(' "$POC320"/main.go | head -1 | sed 's/^ *//')"
corre c5-shim320 800x600 motor-disco-p320 "$SEGS"
mide c5-shim320
resumen c5-shim320
VRAM640="$(tr -d '\r' < "$SAL/c3-800x600-run/flycast.log" | grep -ao 'VRAM de la pantalla [0-9]*' | tail -1)"
if [ "$VRAM_" != "$VRAM640" ]; then
    verde "control 5: el cuadro del PVR SI baja con la pantalla del shim ($VRAM640 con 640x480 -> $VRAM_ con 320x240): la palanca de la pantalla es de VRAM"
else
    rojo "control 5: el cuadro del PVR NO cambio con la pantalla del shim ($VRAM_)"
fi
[ "$(punto)" = "$P800" ] &&
    verde "control 5b: el punto de RAM tampoco se mueve con la pantalla del shim (sigue '$P800')" ||
    rojo "control 5b: el punto de RAM SE MUEVE con la pantalla del shim (640x480='$P800' 320x240='$(punto)')"
rm -rf "$POC320"
nota "la copia del POC quedo borrada ($POC320)"

# ================================================================================================
# 6. EL MUTANTE: el piso de Clamp
# ================================================================================================
echo "== 6. mutante: sin el piso de Clamp el 320x240 SI se resuelve (el control 1 tiene que ponerse ROJO) =="
"$DC_PYTHON" "$MUT" --sin-piso > "$SAL/mutante-sin-piso.log" 2>&1
MUT_LEVANTADO=1
nota "$(sed 's/^/     /' "$SAL/mutante-sin-piso.log")"
if (cd "$REPO" && go run "$SAL/host-resolucion.go") > "$SAL/host-resolucion-mut.txt" 2>&1; then
    R_MUT=$(grep -c 'resuelto=800x600' "$SAL/host-resolucion-mut.txt")
    T_MUT=$(grep -c 'RESOLUCION' "$SAL/host-resolucion-mut.txt")
    if [ "$R_MUT" != "$T_MUT" ]; then
        verde "mutante CAZADO: sin el piso solo $R_MUT de $T_MUT combinaciones resuelven a 800x600 ($(grep -a 'resuelto=320x240' "$SAL/host-resolucion-mut.txt" | head -1 | sed 's/^ *//'))"
    else
        rojo "mutante NO cazado: con el piso sacado las $T_MUT combinaciones siguen resolviendo a 800x600 (el hueco es del corpus)"
    fi
else
    rojo "mutante: el instrumento del host no corrio con el mutante puesto (ver $SAL/host-resolucion-mut.txt)"
fi
levanta_mutante
SHA_DESPUES="$(sha_archivo "$DISPLAY_GO")"
[ "$SHA_ANTES" = "$SHA_DESPUES" ] && verde "el archivo quedo con el mismo sha256 (mutante levantado: ${SHA_ANTES:0:16})" ||
    rojo "el archivo NO quedo como estaba ($SHA_ANTES contra $SHA_DESPUES)"

# ================================================================================================
# 7. EL FONDO: ¿y si el motor PUDIERA bajar la pantalla? (con el piso sacado, la config en 320x240)
#    Es la pregunta de fondo de la tarjeta: si la resolucion NO fuera palanca por el piso, ¿seria
#    palanca si el motor la dejara bajar? Se mide con el mutante puesto: el motor resuelve 320x240 de
#    verdad y se compara su punto de RAM con el de 800x600 (control 3). El corpus NO lo exige en un
#    sentido ni en otro: lo declara (es una medicion, no un control).
# ================================================================================================
echo "== 7. el fondo: con el piso SACADO y d2display REARMADO, ¿el motor resuelve 320x240 y gana RAM? =="
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"

# DEFECTO DEL CORPUS ANTERIOR (medido, 2026-10-18): `--sin-piso` solo cambia la FUENTE de `d2display`, y el
# binario de la consola se enlaza contra el `.gox` PRE-COMPILADO del layer. Sin rearmar el paquete, la consola
# seguia resolviendo 800x600 con el mutante puesto: el control 7 comparaba el mismo numero dos veces y daba un
# verde FALSO. Aca el mutante se rearma en su PROPIO layer (una copia del coherente, que no se toca) y el
# control exige el numero nuevo EN LA CONSOLA: sin "display 320x240" no mide nada.
GOX_REF="$GEN/$PREFIJO/d2common/d2display.gox"
LAYER_MUT="$SAL/gox-sin-piso"
rm -rf "$LAYER_MUT"
cp -a "$GEN" "$LAYER_MUT"
# SEGUNDO DEFECTO, MEDIDO EN LA PRIMERA CORRIDA DE ESTE CORPUS (2026-10-18 07:31): `orden.txt` y
# `orden-motor.txt` llevan rutas ABSOLUTAS del layer de origen, asi que `cp -a` NO alcanza -- el enlace
# siguio leyendo los `.gox` de $GEN y el control 7 quedo ROJO con "display 800x600" mientras el layer
# COPIADO quedaba intacto (asi se cazo). Es la trampa que DC-002.22 seccion 6 ya tenia anotada. Las rutas se
# reescriben al layer copiado y se CONTROLA que TODAS apunten adentro: si no, el rearme no llega al binario.
sed -i "s|^$GEN/|$LAYER_MUT/|" "$LAYER_MUT/orden.txt" "$LAYER_MUT/orden-motor.txt"
N_ORD="$(wc -l < "$LAYER_MUT/orden.txt")"
N_ORDM="$(wc -l < "$LAYER_MUT/orden-motor.txt")"
FUERA="$(cat "$LAYER_MUT/orden.txt" "$LAYER_MUT/orden-motor.txt" | grep -cv "^$LAYER_MUT/")"
[ "$FUERA" = "0" ] &&
    verde "control 7a: las $N_ORD+$N_ORDM entradas del layer copiado apuntan ADENTRO del copy, asi que el enlace usa el d2display.gox REARMADO (con las rutas viejas el control 7 daba un ROJO falso... que era verdadero)" ||
    rojo "control 7a: $FUERA entradas del layer copiado siguen apuntando al layer de origen: el rearme NO llegaria al binario"
GOX_MUT="$LAYER_MUT/$PREFIJO/d2common/d2display.gox"
# shellcheck disable=SC2086
SRCS_DISPLAY="$(ls "$REPO"/d2common/d2display/*.go | grep -v _test.go)"
SHA_GOX_REF="$(sha_archivo "$GOX_REF")"

"$DC_PYTHON" "$MUT" --sin-piso > /dev/null 2>&1
MUT_LEVANTADO=1
if sh-elf-gccgo $GODC_GOFLAGS -I "$LAYER_MUT" -fgo-pkgpath="$PREFIJO/d2common/d2display" -c $SRCS_DISPLAY \
    -o "$GOX_MUT" > "$SAL/rearma-display-sin-piso.log" 2>&1; then
    nota "d2display.gox REARMADO con el piso sacado: $(stat -c%s "$GOX_REF") -> $(stat -c%s "$GOX_MUT") bytes"
    nota "el .text del paquete (lo que se enlaza): $(sh-elf-size -A "$GOX_REF" | awk '$1==".text"{print $2}') con piso -> $(sh-elf-size -A "$GOX_MUT" | awk '$1==".text"{print $2}') sin piso"
    rearmado=1
else
    rojo "control 7: no pude rearmar d2display.gox con el mutante (ver $SAL/rearma-display-sin-piso.log)"
    rearmado=0
fi

# EL A/B DE VERDAD: el MISMO binario (el layer rearmado) con dos configs. Cambiar la config no cambia una
# linea de codigo, y las dos pantallas llevan el mismo numero de digitos ("800x600" y "320x240"), asi que el
# config.json del romdisk tampoco cambia de tamano: la diferencia entre los dos puntos es de la RESOLUCION y
# de nada mas. El control 7b es la referencia del binario rearmado con la pantalla VIEJA: sin el, un
# 1176 -> 544 se leeria como "la pantalla cuesta RAM" cuando puede ser el layout del paquete rearmado (que
# cambia 88 bytes de .text).
if [ "$rearmado" = "1" ]; then
    echo "== 7b. consola: el MISMO binario rearmado, con la config en 800x600 (la referencia del A/B) =="
    corre c7b-rearmado-800x600 800x600 motor-disco "$SEGS" "$LAYER_MUT"
    mide c7b-rearmado-800x600
    resumen c7b-rearmado-800x600
    PR_MUT800="$(punto_ram)"
    D_REARMADO="$DISPLAY_"
    # EXPECTATIVA CORREGIDA (medido en la corrida de las 07:4x): sin el piso, el config de 800x600 NO resuelve
    # 800x600. `Resolve` limita la config a la pantalla del renderer, y la del PVR en este POC es 640x480
    # menos el alto del marco de la ventana (el motor le deja lugar al menu y a la barra de titulo): 640x400.
    # O sea que el binario rearmado resuelve 640x400 con la config vieja y 320x240 con la nueva -- DOS
    # pantallas distintas en el MISMO binario, que es lo que hace valido el A/B del 7d.
    [ "$DISPLAY_" = "display 640x400" ] ||
        rojo "control 7b: el binario rearmado con la config en 800x600 tiene que resolver 640x400, la config limitada a la pantalla del PVR sin el piso (dice '${DISPLAY_:-(vacio)}')"
fi

echo "== 7c. consola: el MISMO binario rearmado, con la config en 320x240 (el A/B de la resolucion) =="
corre c7-sin-piso 320x240 motor-disco "$SEGS" "$LAYER_MUT"
mide c7-sin-piso
resumen c7-sin-piso
if [ "$rearmado" = "1" ]; then
    if [ "$DISPLAY_" != "display 320x240" ]; then
        rojo "control 7: la consola NO resolvio 320x240 con el piso sacado y el paquete rearmado (dice '${DISPLAY_:-(vacio)}'): el mutante no llego al binario"
    else
        verde "control 7: con el piso SACADO y d2display.gox REARMADO la consola SI resuelve 320x240 (\"$DISPLAY_\"): el mutante llego al binario y el motor puede bajar la pantalla"
        [ "$DISPLAY_" != "$D_REARMADO" ] &&
            verde "control 7c: el MISMO binario resuelve DOS pantallas distintas segun la config ($D_REARMADO con la config en 800x600 y $DISPLAY_ con la de 320x240): el A/B del 7d compara la resolucion y nada mas" ||
            rojo "control 7c: las dos configs del A/B resolvieron LA MISMA pantalla ($DISPLAY_): el A/B no dice nada"
        if [ "$(punto_ram)" = "$PR_MUT800" ]; then
            verde "control 7d: con el MISMO binario, la pantalla en 320x240 da la MISMA RAM que en 800x600 ($PR_MUT800): la resolucion NO es palanca de RAM ni siquiera cuando el motor la baja de verdad"
        else
            nota "control 7d: con el MISMO binario la RAM cambia al bajar la pantalla: '$(punto_ram)' con 320x240 contra '$PR_MUT800' con 800x600"
        fi
        if [ "$PR_MUT800" = "$PR800" ]; then
            verde "control 7e: el binario rearmado tiene la MISMA RAM que el layer coherente ($PR800): el rearme del paquete no mueve el punto"
        else
            nota "control 7e: el binario rearmado NO tiene la misma RAM que el coherente: '$PR_MUT800' contra '$PR800' (la diferencia es del LAYOUT del paquete rearmado, no de la pantalla: el .text cambia 88 bytes)"
        fi
    fi
fi
[ "$(sha_archivo "$GOX_REF")" = "$SHA_GOX_REF" ] &&
    verde "el layer coherente quedo intacto (d2display.gox sha256 ${SHA_GOX_REF:0:16}): el rearme fue en su copia" ||
    rojo "el layer coherente NO quedo intacto ($SHA_GOX_REF contra $(sha_archivo "$GOX_REF"))"
levanta_mutante
SHA_DESPUES2="$(sha_archivo "$DISPLAY_GO")"
[ "$SHA_ANTES" = "$SHA_DESPUES2" ] && verde "el archivo quedo con el mismo sha256 despues del control 7 (${SHA_ANTES:0:16})" ||
    rojo "el archivo NO quedo como estaba despues del control 7 ($SHA_ANTES contra $SHA_DESPUES2)"

"$DC_PYTHON" "$MUT" --estado | sed 's/^/   /'

echo
echo "== RESULTADO: $OK controles OK, $FALLOS con fallo"
echo "== la hipotesis de la tarjeta (la resolucion como palanca de RAM) se mide en los controles 3+4 y 5b."
echo "== evidencia: console/dc/evidencia/dc-002.30-resolucion.txt"
[ "$FALLOS" -eq 0 ]
