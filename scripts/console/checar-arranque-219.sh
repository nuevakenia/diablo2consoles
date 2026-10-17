#!/bin/bash
# CORPUS DE DC-002.19: EL ARRANQUE DEL MOTOR PASA LA SELECCION DE LOS ARCHIVOS DEL JUEGO.
#
# QUE SE MIDE (todo en Flycast, con el MOTOR DE VERDAD: el POC console/dc/poc/motor-arranque llama a
# d2app.Create().Run(), o sea el mismo entry point de la consola):
#
#  CONTROL 1 (la config del arranque): el motor lee `/rd/OpenDiablo2/config.json` -- escrita por el MOTOR
#    mismo (d2config.Save, el MarshalJSON de DC-002.18) y horneada en el romdisk (build-motor-arranque.sh) --
#    y queda con `MpqPath= /rd/juego` y los 7 archivos del orden del motor. Sin esta config el motor
#    pregunta por la carpeta y en la consola no hay quien conteste.
#  CONTROL 2 (la seleccion): `Complete(MpqPath, orden)` = true Y `Validate(MpqPath).OK()` = true con
#    7 archivos vistos y 0 faltantes. Validate es el camino que MIRA EL CONTENIDO del directorio: es el que
#    en DC-002.19 daba `false archivos que ve= 0` sobre una carpeta con siete archivos.
#  CONTROL 3 (el ReadDir del OVERLAY en RAM): el directorio que el arranque crea en la RAM
#    (/rd/prueba-seleccion, con un archivo de 12 bytes) se lista con su entrada, y el overlay dice 1 archivo
#    y 12 bytes. Es el defecto que esta tarjeta arreglo: antes el listado de un directorio del overlay
#    volvia VACIO (medido en la evidencia, `Validate.OK()= false archivos que ve= 0`).
#  CONTROL 4 (la lectura de la config por el motor): `[App][INFO] loaded configuration file from
#    /rd/OpenDiablo2/config.json`. Es el camino real de lectura del arranque (d2app/app.go), no un POC
#    paralelo: lo que DC-002.18 dejo declarado como hueco.
#  CONTROL 5 (la FRONTERA declarada): despues de la seleccion el arranque sigue y muere en
#    `panic: assets: bmp.Decode failed: gzip: invalid header` -- el gzip de un asset EMBEBIDO, ANTES de
#    cualquier lectura del MPQ. Eso NO es de esta tarjeta (es DC-002.20, ya abierta con este defecto
#    medido): el control exige que el panic ESTE, para que el dia que DC-002.20 lo arregle este script se
#    ponga ROJO y haya que mover la frontera a mano en vez de creerle a un verde viejo.
#  CONTROL 6 (las marcas): el numero EXACTO de marcas `MOTOR-ARRANQUE` de la corrida entera.
#
# MUTANTES (5): cada uno tiene que poner ROJO el control que le toca. Si queda VERDE, el hueco es del corpus.
#   'sin-romdisk'      DC_SIN_ROMDISK=1  -> reproduce el estado del arranque ANTES del arreglo (en DC-002.18
#                      no habia romdisk): /rd no existe y el motor pide la carpeta -> caen 1, 2 y 4.
#   'sin-config'       DC_SIN_CONFIG=1   -> romdisk con el directorio de datos pero SIN config: el motor
#                      arma su configuracion por defecto (MpqPath vacio) -> caen 1 y 2.
#   'sin-sellos'       DC_SIN_SELLOS=1   -> config puesta pero SIN los archivos del juego: la seleccion se
#                      vuelve a frenar -> cae 2 (y el 1 sigue verde: la config esta).
#   'overlay-no-lista' mutacion en Go (os.go): os.ReadDir NO mezcla el listado del overlay -> cae 3.
#   'ramfs-no-matchea' mutacion en C (os_dc.c): el listado del overlay no matchea ninguna ruta -> cae 3.
#
# LA GOX: hay que tener una con la stdlib minima Y el motor entero. Se arma asi (una vez):
#     bash scripts/console/build-std-gox.sh   /tmp/gox-00219
#     bash scripts/console/build-motor-all.sh /tmp/gox-00219
# El script REHACE os.gox ahi mismo desde el arbol (compilar-paquete-std.sh os) y NO confia en que este al
# dia: si el arbol de .gox se COPIA de otro lado, su orden.txt trae las rutas ABSOLUTAS del original y el
# enlace toma los .gox VIEJOS sin decir nada (medido: el ReadDir del overlay corria con el os.gox viejo y
# volvia vacio con el arreglo ya puesto).
#
# Uso:  bash scripts/console/checar-arranque-219.sh [dir_gox] [dir_build] [segundos]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-00219}"
BUILD="${2:-/tmp/dcbuild/arranque-219}"
SEGS="${3:-25}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"

OS="$REPO/console/dc/std/os/os.go"
OSDC="$REPO/console/dc/std/csrc/os_dc.c"
ARR="$HERE/build-motor-arranque.sh"
POC=motor-arranque
MARCAS=10

TOTAL_CTRL=6
TOTAL_MUT=5
CAZADOS=0

for f in "$GEN/orden.txt" "$GEN/orden-motor.txt" "$OS" "$OSDC" "$ARR"; do
    [ -f "$f" ] || { echo "falta $f" >&2; exit 1; }
done

# si el arreglo no esta en el arbol, lo que se mediria es otra cosa: mejor no correr que mentir
grep -q 'd2c_ramfs_manifiesto' "$OS" || {
    echo "$OS no lista el overlay de la RAM: el arreglo de DC-002.19 no esta, no hay que medir." >&2; exit 1; }
grep -q 'd2c_ramfs_manifiesto' "$OSDC" || {
    echo "$OSDC no tiene el listado del overlay: el arreglo de DC-002.19 no esta, no hay que medir." >&2; exit 1; }
grep -q 'romdisk' "$ARR" || {
    echo "$ARR no hornea el romdisk del arranque: el arreglo de DC-002.19 no esta, no hay que medir." >&2; exit 1; }

# ---------------------------------------------------------------- compilar y correr
MUT_ENV=""   # lo usa compilar() para los mutantes que se hacen con variables de entorno del build
compilar() { # $1 = dir de salida
    local out="$1"
    if ! bash "$HERE/compilar-paquete-std.sh" os "$GEN" > "$out.std.log" 2>&1; then
        echo "   os.gox NO COMPILA (ver $out.std.log)"; return 1
    fi
    # shellcheck disable=SC2086
    if ! env $MUT_ENV POC="$POC" bash "$ARR" "$out" "$GEN" > "$out.build.log" 2>&1; then
        echo "   EL ARRANQUE NO ENLAZA (ver $out.build.log)"; return 1
    fi
}

correr() { # $1 = dir del build, $2 = dir de la corrida
    rm -rf "$2"
    (cd "$REPO" && CAPTURE=none bash "$HERE/run-flycast.sh" "$1/prog-emu.elf" "$SEGS" "$2") > "$2.salida" 2>&1
    [ -f "$2/flycast.log" ]
}

# ---------------------------------------------------------------- los seis controles
ctrl_config() {
    grep -qaF 'MOTOR-ARRANQUE config del arranque: MpqPath= /rd/juego archivos en el orden= 7' "$1"
}
ctrl_seleccion() {
    grep -qaF 'MOTOR-ARRANQUE seleccion: Complete= true Validate.OK()= true archivos que ve= 7 faltan= 0' "$1"
}
ctrl_overlay() {
    grep -qaF 'MOTOR-ARRANQUE overlay del listado: archivos= 1 bytes= 12 dirs= 2' "$1" &&
        grep -qaF 'MOTOR-ARRANQUE seleccion overlay: ReadDir de /rd/prueba-seleccion -> 1 entradas: uno.txt' "$1"
}
ctrl_lectura() {
    # OJO: el log del motor trae los colores ANSI pegados al prefijo
    # (`ESC[35m[App]ESC[32m[INFO]ESC[0m loaded configuration file from ...`), asi que se busca el texto
    # que va DESPUES del prefijo: buscar "[App][INFO] loaded..." da 0 matches y el control se pone rojo
    # con la linea presente (medido).
    grep -qaF 'loaded configuration file from /rd/OpenDiablo2/config.json' "$1"
}
ctrl_frontera() {
    grep -qaF 'panic: assets: bmp.Decode failed: gzip: invalid header' "$1"
}
ctrl_marcas() {
    [ "$(grep -ac 'MOTOR-ARRANQUE' "$1")" = "$MARCAS" ]
}

controles() { # $1 = log; imprime los seis y devuelve cuantos dan bien (en la ultima linea)
    local log="$1" bien=0
    if ctrl_config "$log";    then echo "   control 1 (la config del arranque):     VERDE"; bien=$((bien + 1));
    else echo "   control 1 (la config del arranque):     ROJO"; fi
    if ctrl_seleccion "$log"; then echo "   control 2 (la seleccion de archivos):   VERDE"; bien=$((bien + 1));
    else echo "   control 2 (la seleccion de archivos):   ROJO"; fi
    if ctrl_overlay "$log";   then echo "   control 3 (el ReadDir del overlay):     VERDE"; bien=$((bien + 1));
    else echo "   control 3 (el ReadDir del overlay):     ROJO"; fi
    if ctrl_lectura "$log";   then echo "   control 4 (la lectura del motor):       VERDE"; bien=$((bien + 1));
    else echo "   control 4 (la lectura del motor):       ROJO"; fi
    if ctrl_frontera "$log";  then echo "   control 5 (la frontera declarada):      VERDE"; bien=$((bien + 1));
    else echo "   control 5 (la frontera declarada):      ROJO"; fi
    if ctrl_marcas "$log";    then echo "   control 6 (las $MARCAS marcas exactas):       VERDE"; bien=$((bien + 1));
    else echo "   control 6 (las $MARCAS marcas exactas):       ROJO ($(grep -ac 'MOTOR-ARRANQUE' "$log"))"; fi
    echo "$bien"
}

mostrar() { # $1 = log
    grep -a 'MOTOR-ARRANQUE\|loaded configuration\|^panic\|Memory:' "$1" | tr -d '\r' | sed 's/^/     /'
    echo "     (marcas MOTOR-ARRANQUE de la corrida: $(grep -ac 'MOTOR-ARRANQUE' "$1"))"
}

# ---------------------------------------------------------------- mutacion de archivos
# El respaldo es una copia propia (nunca `git checkout --`): en medio de la tarjeta hay arreglos SIN
# COMMITEAR en esos mismos archivos. Despues de cada mutante se verifica el sha256.
mutar() { # <archivo> <buscar> <poner>; devuelve 1 si el patron no es unico
    local arch="$1" buscar="$2" poner="$3"
    python3 - "$arch" "$buscar" "$poner" <<'PY' || return 1
import sys
ruta, buscar, poner = sys.argv[1], sys.argv[2], sys.argv[3]
t = open(ruta).read()
n = t.count(buscar)
if n != 1:
    print("el texto a mutar aparece %d veces (tiene que ser 1)" % n, file=sys.stderr)
    sys.exit(2)
open(ruta, "w").write(t.replace(buscar, poner))
PY
}

exportar() { # <archivo>; imprime "<copia> <sha256>"
    local copia
    copia="$(mktemp /tmp/checar-arranque-219-XXXXXX)"
    cp "$1" "$copia"
    echo "$copia $(sha256sum "$1" | awk '{print $1}')"
}

restaurar() { # <archivo> <copia> <sha>
    cp "$2" "$1"
    if [ "$(sha256sum "$1" | awk '{print $1}')" != "$3" ]; then
        echo "   EL ARBOL QUEDO MUTADO: no coincide el sha256 de $1" >&2
        return 1
    fi
    echo "   arbol restaurado ($(basename "$1"))"
}

# ---------------------------------------------------------------- los mutantes
probar_mutante() { # <nombre> <env del build> <archivo a mutar|-> <buscar> <poner> <control que tiene que caer>
    local nombre="$1" envmut="$2" arch="$3" buscar="${4:-}" poner="${5:-}" tiene_que_caer="$6"
    local out="/tmp/checar-219-mut-$nombre"
    local log="$out/run/flycast.log"
    echo
    echo "== mutante '$nombre'"
    local copia="" sha=""
    if [ "$arch" != "-" ]; then
        read -r copia sha <<<"$(exportar "$arch")"
        if ! mutar "$arch" "$buscar" "$poner"; then
            echo "   NO SE APLICO LA MUTACION (patron sin match unico): el mutante no prueba nada" >&2
            restaurar "$arch" "$copia" "$sha" || exit 1
            rm -f "$copia"
            return
        fi
    fi
    MUT_ENV="$envmut"
    if compilar "$out" && correr "$out" "$out/run" && [ -f "$log" ]; then
        controles "$log" | sed -n '1,6p'
        if "$tiene_que_caer" "$log"; then
            echo "   NO LO CAZO: el control que tenia que caer sigue VERDE con el mutante puesto" >&2
        else
            echo "   cazado: el control que tenia que caer cayo"
            CAZADOS=$((CAZADOS + 1))
        fi
    else
        # OJO: si no hay log, los controles dan ROJO por AUSENCIA, no porque el mutante haga lo suyo. Eso
        # no es cazar nada: se declara y NO se cuenta (medido: la primera version de este script miraba el
        # log en $out/ y la corrida lo escribia en $out/run/, asi que dos mutantes "cayeron" sin haber
        # corrido el POC).
        echo "   NO LO CAZO: con el mutante no compila/enlaza/corre (o no dejo log), y un mutante que no corre no prueba nada" >&2
    fi
    MUT_ENV=""
    if [ "$arch" != "-" ]; then
        restaurar "$arch" "$copia" "$sha" || exit 1
        rm -f "$copia"
    fi
}

# ---------------------------------------------------------------- 1. control sano
echo "== 1. control sano: con el arreglo puesto, los $TOTAL_CTRL controles"
compilar "$BUILD" || exit 1
correr "$BUILD" /tmp/checar-219-sano || exit 1
BIEN="$(controles /tmp/checar-219-sano/flycast.log | sed -n '$p')"
controles /tmp/checar-219-sano/flycast.log | sed -n '1,6p'
if [ "$BIEN" -ne "$TOTAL_CTRL" ]; then
    echo "   EL CONTROL SANO NO DA ($BIEN/$TOTAL_CTRL): no se sigue, primero hay que ver por que" >&2
    mostrar /tmp/checar-219-sano/flycast.log >&2
    exit 1
fi
echo "   OK: $BIEN/$TOTAL_CTRL controles verdes con el codigo de ahora"
mostrar /tmp/checar-219-sano/flycast.log

# ---------------------------------------------------------------- 2. los mutantes
# El primero es el ESTADO DEL ARBOL ANTES DEL ARREGLO (DC-002.18, commit 8bf54810): sin romdisk.
probar_mutante sin-romdisk "DC_SIN_ROMDISK=1" - "" "" ctrl_config
probar_mutante sin-config "DC_SIN_CONFIG=1" - "" "" ctrl_config
probar_mutante sin-sellos "DC_SIN_SELLOS=1" - "" "" ctrl_seleccion

probar_mutante overlay-no-lista "" "$OS" \
    '	// El overlay TAPA al romdisk con el mismo nombre (la misma regla que os.Open y d2c_hay).
	for _, nueva := range deRAM {' \
    '	// MUTANTE overlay-no-lista
	for _, nueva := range []DirEntry(nil) {' \
    ctrl_overlay

probar_mutante ramfs-no-matchea "" "$OSDC" \
    'if (l <= n + 1 || strncmp(nombre, prefijo, n + 1) != 0) {' \
    'if (l <= n + 1 || 1) {   /* MUTANTE ramfs-no-matchea */' \
    ctrl_overlay

# ---------------------------------------------------------------- 3. control sano de vuelta
echo
echo "== 3. control sano de vuelta (el arbol entero, sin mutar)"
if compilar "$BUILD" && correr "$BUILD" /tmp/checar-219-vuelta; then
    BIEN="$(controles /tmp/checar-219-vuelta/flycast.log | sed -n '$p')"
    controles /tmp/checar-219-vuelta/flycast.log | sed -n '1,6p'
    [ "$BIEN" -eq "$TOTAL_CTRL" ] || { echo "   NO VOLVIERON LOS $TOTAL_CTRL CONTROLES ($BIEN/$TOTAL_CTRL)" >&2; exit 1; }
    echo "   control sano VERDE"
else
    echo "   NO VOLVIO EL CONTROL SANO" >&2
    exit 1
fi

echo
echo "== $CAZADOS/$TOTAL_MUT mutantes cazados =="
[ "$CAZADOS" -eq "$TOTAL_MUT" ]
