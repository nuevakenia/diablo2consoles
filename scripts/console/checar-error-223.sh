#!/bin/bash
# CORPUS DE DC-002.23: DE QUE LADO SE ROMPE EL ERROR DEL MOTOR EN EL CUADRO 0.
#
# La pregunta de la tarjeta: en DC-002.22 el cuadro 0 del motor se caia con
#   Unhandled exception: PC 0000000b, code 1, evt 00e0
# pintando el texto de su cuadro de error, y quedo abierto si el salto era el itab del metodo `PrintAt`
# del renderer o el estado del `GlyphPrinter`.
#
# LA RESPUESTA MEDIDA (esta tarjeta): NO es ninguno de los dos. El `jsr @r0` que se cae es el de la
# EVALUACION DEL ARGUMENTO `a.errorMessage.Error()` de `d2app.App.updateInitError`, y lo que esta mal es
# el CAMPO `errorMessage` del `App`, que no es una tabla de metodos valida:
#
#   itab= 0x8c4f3c60 data= 0xff000000        (medido con una sonda temporal dentro de d2app)
#   0x8c4f3c60 = _image_1color.RGBA..d       (sh-elf-nm: el DESCRIPTOR DE TIPO de image/color.RGBA)
#
# El offset del campo NO es deducido del desensamblado: `unsafe.Offsetof` dentro del paquete da
# errorMessage=156, renderer=124, config=148, logger=152, options=164, size=168.
#
# CONTROLES (corrida sana):
#  1. (POC iface-error, sin el motor) las 14 formas con las que la consola se topa con un `error` dan el
#     layout sano: los nil con itab=0x0 data=0x0, y los no-nil con itab en .rodata y m0 en .text; y el
#     despacho (`e.Error()`) devuelve los textos. O sea: las interfaces `error` de la consola estan BIEN,
#     y por eso el defecto no es "la interfaz del error" sino MEMORIA del `App`.
#  2. (POC motor-error, con el motor y sin tocarle una linea) el campo `errorMessage` del `App`, leido en
#     el offset 156, esta NIL tras `Create`, tras `LoadConfig` y tras arrancar el video; y los tres
#     constructores que usa el arranque devuelven error nil.
#  3. (POC motor-arranque, el binario de la rama) EL ARRANQUE CON EL ARREGLO (DC-002.24): el motor entra al
#     loop, completa **3 cuadros** y NO hay excepcion del SH-4. ANTES del arreglo este control era el defecto
#     vigilado: 0 cuadros y `Unhandled exception: PC 0000000b, code 1` (la evaluacion de
#     `a.errorMessage.Error()` con el campo del App pisado por el GC).
#
# MUTANTES:
#  A. --offset-vecino: el POC motor-error lee el offset 152 (el campo `logger`, un puntero NO nil) en vez
#     de 156. Si las lecturas siguieran dando NIL, el instrumento no mediria lo que dice medir. CAZADO.
#  B. --sq-roto (DC-002.24): le devuelve a `gc_sq_copy` la formula vieja del area de la Store Queue
#     (`>> 26`) y rehace libgodc.a. Con el defecto puesto el campo del App del control 2 tiene que volver
#     a dar BASURA (medicion directa del arreglo): si siguiera dando NIL, el arreglo no seria el que
#     arregla. CAZADO.
#     (De yapa, medido: con el defecto y el semiespacio de 512 KB el arranque SI completa cuadros y muere
#     despues con otra excepcion del SH-4 -- el defecto cambia de forma con el layout, como en DC-002.22. En
#     la evidencia queda el detalle: `Unhandled exception: PC 64800008`.)
#
# Uso:  bash scripts/console/checar-error-223.sh [dir_gox] [segundos_arranque]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-222}"
SEGS="${2:-30}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
SAL="${SAL:-/tmp/corpus-223}"

export GODC_GOX="$GEN"
mkdir -p "$SAL"
OK=0
FALLOS=0

nota() { echo "   $*"; }
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }

echo "== 0. el estado del runtime (la cadena + el arreglo de la Store Queue, con 512 KB) =="
"$DC_PYTHON" "$HERE/fix-libgodc-gc.py" 512 --build >"$SAL/runtime.log" 2>&1 || {
    echo "   la cadena del runtime FALLO (ver $SAL/runtime.log)"; exit 1; }
nota "$(grep -a 'libgodc.a' "$SAL/runtime.log" | tail -1)"

echo "== 1. el POC del error sin el motor (14 formas de interfaz) =="
bash "$HERE/build-dc.sh" "$REPO/console/dc/poc/iface-error" "$SAL/iface" >"$SAL/iface.build.log" 2>&1 ||
    { echo "   EL POC iface-error NO COMPILA (ver $SAL/iface.build.log)"; exit 1; }
CAPTURE=none bash "$HERE/run-flycast.sh" "$SAL/iface/prog.elf" 22 "$SAL/run-iface" >/dev/null 2>&1
L="$SAL/run-iface/flycast.log"

n_nil=$(grep -ac 'IFACE-ERROR .*esnil= si itab= 0x0 data= 0x0' "$L")
n_nil2=$(grep -ac 'IFACE-FORMA .*esnil= si itab= 0x0 data= 0x0' "$L")
n_nil=$((n_nil + n_nil2))
n_sano=$(grep -aE 'IFACE-(ERROR|FORMA) .*esnil= no itab= 0x8[0-9a-f]* data= 0x8[0-9a-f]* m0= 0x8[0-9a-f]*' "$L" | wc -l)
n_basura=$(grep -aE 'IFACE-(ERROR|FORMA).*esnil= no' "$L" | grep -vcE 'itab= 0x8[0-9a-f]{7}')
despacho=$(grep -ac 'IFACE-ERROR 1 -> uno' "$L")
campo=$(grep -ac 'IFACE-ERROR 3 -> guardado en un campo' "$L")

nota "formas nil (itab=0x0/data=0x0): $n_nil (esperado 8: 4 multi-retorno + 4 campos de struct)"
nota "formas con error real (itab y m0 en el binario): $n_sano (esperado 6)"
nota "formas con error real y el itab FUERA de .rodata (basura): $n_basura (esperado 0)"
[ "$n_nil" -eq 8 ] && verde "los 8 caminos nil dan una interfaz error NIL de verdad" ||
    rojo "esperaba 8 formas nil y medi $n_nil"
[ "$n_sano" -eq 6 ] && verde "los 6 caminos con error real dan itab en .rodata y m0 en .text" ||
    rojo "esperaba 6 formas sanas y medi $n_sano"
[ "$n_basura" -eq 0 ] && verde "ningun itab chico (basura)" || rojo "hay $n_basura itabs chicos"
{ [ "$despacho" -ge 1 ] && [ "$campo" -ge 1 ]; } &&
    verde "el despacho (e.Error()) devuelve los textos" || rojo "el despacho no devolvio los textos"

echo "== 2. el campo errorMessage del App, con el motor (POC motor-error) =="
POC=motor-error bash "$HERE/build-motor-arranque.sh" "$SAL/motor-error" "$GEN" >"$SAL/motorerr.build.log" 2>&1 ||
    { echo "   EL POC motor-error NO COMPILA (ver $SAL/motorerr.build.log)"; exit 1; }
CAPTURE=none bash "$HERE/run-flycast.sh" "$SAL/motor-error/prog-emu.elf" 20 "$SAL/run-motorerr" >/dev/null 2>&1
M="$SAL/run-motorerr/flycast.log"

n_campo_nil=$(grep -ac 'MOTOR-ERROR .*App.errorMessage.* estado= NIL' "$M")
n_constructor=$(grep -ac 'MOTOR-ERROR [123] d2.*esnil= si itab= 0x0' "$M")
n_campo_basura=$(grep -ac 'MOTOR-ERROR .*App.errorMessage.* estado= BASURA' "$M")

nota "lecturas del campo (offset 156) que dan NIL: $n_campo_nil (esperado 3: Create, LoadConfig, video)"
nota "constructores del arranque con error nil: $n_constructor (esperado 3)"
nota "lecturas del campo que dan BASURA: $n_campo_basura (esperado 0 con el arreglo del GC)"
[ "$n_campo_nil" -eq 3 ] && verde "el campo se guarda NIL en los tres puntos medidos" ||
    rojo "esperaba 3 lecturas NIL y medi $n_campo_nil"
[ "$n_constructor" -eq 3 ] && verde "d2loader/d2records/d2asset devuelven error nil" ||
    rojo "esperaba 3 constructores con error nil y medi $n_constructor"
[ "$n_campo_basura" -eq 0 ] && verde "el campo del App no tiene la basura del GC (la copia esta arreglada)" ||
    rojo "el campo del App tiene basura en $n_campo_basura lecturas (el GC lo vuelve a pisar)"

echo "== 3. el binario de la rama: EL ARRANQUE CON EL ARREGLO (cuadros completos) =="
POC=motor-arranque bash "$HERE/build-motor-arranque.sh" "$SAL/arranque" "$GEN" >"$SAL/arranque.build.log" 2>&1 ||
    { echo "   EL POC motor-arranque NO COMPILA (ver $SAL/arranque.build.log)"; exit 1; }
CAPTURE=none bash "$HERE/run-flycast.sh" "$SAL/arranque/prog-emu.elf" "$SEGS" "$SAL/run-arranque" >/dev/null 2>&1
A="$SAL/run-arranque/flycast.log"

loop=$(grep -ac 'EBITEN-LOOP cuadro 0 esperando el cuadro del PVR' "$A")
# los cuadros COMPLETOS del loop los cuenta el corpus: el POC no imprime ese resumen (lo imprime
# checar-pantalla-222.sh con su propia cuenta). Se cuentan las marcas del PVR por cuadro.
cuadros=$(grep -ac 'EBITEN-LOOP cuadro [0-9]* el PVR dio el cuadro' "$A")
excepcion=$(grep -aoE 'Unhandled exception: PC [0-9a-f]{8}, code [0-9]' "$A" | head -1)
arranques=$(grep -ac 'MOTOR-ARRANQUE arranca' "$A")
monton=$(grep -aoE 'monton de C antes del motor: [0-9]+ bytes libres' "$A" | head -1 | grep -oE '[0-9]+' | head -1)
vram=$(grep -aoE 'VRAM de la pantalla [0-9]+ bytes' "$A" | head -1 | grep -oE '[0-9]+' | head -1)

nota "el motor llega al loop del cuadro 0: $loop vez/veces"
nota "cuadros completos del loop: ${cuadros:-?}"
nota "lo que se cae: ${excepcion:-?}"
nota "arranques de la consola: ${arranques:-?}; monton de C antes del motor: ${monton:-?} bytes libres; VRAM de la pantalla: ${vram:-?} bytes"

[ "${loop:-0}" -ge 1 ] && verde "el motor entra al loop y abre el cuadro 0" ||
    rojo "el motor no llego al cuadro 0"
if [ "${cuadros:-0}" -ge 1 ]; then
    verde "EL ARREGLO ANDA: $cuadros cuadros completos del loop (antes del arreglo: 0 y PC 0000000b)"
    [ -n "${excepcion:-}" ] && nota "(queda OTRA caida despues de los cuadros: ${excepcion} -- otro defecto, no el campo del App)"
else
    rojo "el arranque no completa cuadros (cuadros=${cuadros:-?}, excepcion=${excepcion:-?}): el arreglo del GC no esta puesto?"
fi

echo "== 4. el mutante del instrumento (el offset del campo) =="
"$DC_PYTHON" "$HERE/mutar-error-223.py" --offset-vecino >/dev/null 2>&1
POC=motor-error bash "$HERE/build-motor-arranque.sh" "$SAL/motor-error-mut" "$GEN" >"$SAL/mut.build.log" 2>&1
CAPTURE=none bash "$HERE/run-flycast.sh" "$SAL/motor-error-mut/prog-emu.elf" 20 "$SAL/run-mut" >/dev/null 2>&1
MM="$SAL/run-mut/flycast.log"
nil_mut=$(grep -ac 'MOTOR-ERROR .*App.errorMessage.* estado= NIL' "$MM")
basura_mut=$(grep -ac 'MOTOR-ERROR .*App.errorMessage.* estado= BASURA' "$MM")
nota "con el offset 152 (el campo logger): NIL=$nil_mut BASURA=$basura_mut"
if [ "$nil_mut" -eq 0 ] && [ "$basura_mut" -ge 1 ]; then
    verde "MUTANTE CAZADO: con el offset vecino el POC ya NO dice NIL (el instrumento discrimina)"
else
    rojo "MUTANTE NO CAZADO: con el offset mal el POC igual dice NIL ($nil_mut): el control no discrimina"
fi
"$DC_PYTHON" "$HERE/mutar-error-223.py" --volver >/dev/null 2>&1

echo "== 5. el mutante del arreglo (la copia por Store Queue del GC) =="
# El mutante le devuelve a gc_sq_copy la formula vieja del area de la Store Queue y rehace libgodc.a con el
# semiespacio de DC-002.21 (256 KB), que es donde se midio el rojo original: el arranque tiene que volver a
# **0 cuadros**. (Con el arreglo y 512 KB completa cuadros -- control 3.) La FORMA de la caida cambia con el
# layout (DC-002.23: PC 0000000b, growslice, nil deref, map concurrente), asi que lo que se exige es el 0
# cuadros y el texto de la caida va solo como nota.
"$DC_PYTHON" "$HERE/mutar-error-223.py" --sq-roto 256 >"$SAL/mutsq.log" 2>&1
POC=motor-arranque bash "$HERE/build-motor-arranque.sh" "$SAL/arranque-mut-sq" "$GEN" >"$SAL/mutsq.build.log" 2>&1 ||
    { echo "   EL POC motor-arranque NO COMPILA con el mutante (ver $SAL/mutsq.build.log)"; exit 1; }
CAPTURE=none bash "$HERE/run-flycast.sh" "$SAL/arranque-mut-sq/prog-emu.elf" "$SEGS" "$SAL/run-mut-sq" >/dev/null 2>&1
MS="$SAL/run-mut-sq/flycast.log"
cuadros_mut=$(grep -ac 'EBITEN-LOOP cuadro [0-9]* listo' "$MS")
excepcion_mut=$(grep -aoE 'Unhandled exception: PC [0-9a-f]{8}, code [0-9]' "$MS" | head -1)
panic_mut=$(grep -aoE 'panic: [a-zA-Z: ]+' "$MS" | head -1)
nota "con la Store Queue rota (256 KB): cuadros completos=$cuadros_mut, cae con: ${excepcion_mut:-${panic_mut:-?}}"
if [ "${cuadros_mut:-1}" -eq 0 ]; then
    verde "MUTANTE CAZADO: sin el arreglo el arranque vuelve a 0 cuadros completos (con el arreglo: 3)"
else
    rojo "MUTANTE NO CAZADO: con la Store Queue rota el arranque igual completa $cuadros_mut cuadros"
fi
"$DC_PYTHON" "$HERE/mutar-error-223.py" --volver >"$SAL/mutsq.volver.log" 2>&1

echo
echo "== $OK controles verdes, $FALLOS fallos =="
[ "$FALLOS" -eq 0 ] || exit 1
