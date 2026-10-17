#!/bin/bash
# EL CORPUS DE DC-002.14: de donde sale el peso del motor y que recorte hace arrancar.
#
# Lo que mide, en orden:
#  1. EL INSTRUMENTO NO SE DEJA MENTIR. En el ELF enlazado los tamanos de .text se SOLAPAN (21.242 de
#     21.266 simbolos arrancan dentro del anterior; la suma da 14.215.564 bytes en una seccion de
#     4.962.328 = 2,86x, mientras la UNION de los intervalos da 4.971.334 = 100,2% de la seccion). Los
#     tamanos son de fiar pero NO son exclusivos, y sumarlos por paquete reparte bytes ajenos. pesar-motor.py
#     tiene que darse cuenta y salir con 2 en vez de imprimir esa tabla.
#     (La tabla "por paquete" de DC-002.8/.13/.14 -- d2game 1.098.252, etc. -- salia de ahi: es un artefacto
#     del solape y queda retirada. Esta tarjeta lo midio DESPUES de haberla publicado.)
#  2. EL MISMO DATO, MEDIDO POR DOS CAMINOS. El peso mas grande del motor es UN dato de d2core/d2records:
#     el []ObjectLookupRecord de d2core/d2records/object_lookup_record_data.go (1.942.429 bytes de fuente,
#     7.902 lineas). Se mide en su .gox (seccion .data.go..C3111) y en el ELF enlazado, y los dos numeros
#     tienen que coincidir (el .data entero del ELF lo confirma).
#  3. EL ARRANQUE PASA LAS PAREDES DEL RUNTIME. Con el semiespacio del GC chico el binario del motor arranca,
#     pasa gc_init (2 x semiespacio) y lo que lo frena despues ya no es RAM.
#     RE-MEDIDO EN DC-002.25, CON EL GC SANO (la copia por Store Queue arreglada en DC-002.24): las dos
#     afirmaciones que eran de DC-002.21 en esta parte -- "256 KB alcanza" y "512 KB muere en la imagen del
#     atlas (diff 98304)" -- se midieron con el DEFECTO de la copia puesto, cuando el conjunto vivo del GC era
#     practicamente vacio. Con el GC sano el arranque de hoy, con el semiespacio en 512 KB, no muere en ninguna
#     pared de RAM: llega al motor y completa cuadros (`arranca 11 marcas`, 3 cuadros del loop; ver
#     checar-gc-221.sh y la seccion 41 del README). El veredicto que este control acepta es entonces `arranca*`
#     ademas de las dos formas historicas (el panic de Go de DC-002.14 y el pedido del motor de 1.916.928), y
#     sigue ROJO si el arranque muere en gc_init o si vuelve la pared del atlas (98.304). El comodin que vigila
#     el PRESUPUESTO sigue siendo el mutante 'gc-2048':
#
#         KB=2048 ->  gc_init: alloc failed (el defecto que DC-002.14 encontro, sigue cazado)
#         KB=512  ->  arranca: 11 marcas y cuadros del loop (el valor, con el GC sano)
#         KB=256  ->  fatal error: out of memory (el valor de DC-002.21 ya no alcanza; corpus: checar-gc-221.sh)
#
#     OJO, ARTEFACTOS: los dos binarios que este corpus compara viven en /tmp y NO sobreviven a un reinicio
#     (medido al re-correrlo en DC-002.25: faltaban /tmp/dcbuild/motor-214 y /tmp/dcbuild/motor-arranque, y el
#     gox del recorte /tmp/gox-214-gc es de antes de DC-002.18 -- no tiene `json`, asi que el POC de HOY no
#     enlaza contra el). Para re-correrlo: `SALTAR_GOX=1 bash scripts/console/recorte-214.sh /tmp/gox-214-gc
#     /tmp/dcbuild/motor-214 A` + `bash scripts/console/build-motor-arranque.sh /tmp/dcbuild/motor-arranque
#     /tmp/gox-222`; si el gox del recorte no enlaza, hay que rehacerlo con los flags (recorte-214.sh sin
#     SALTAR_GOX). Los controles 1, 2 y 4 y el mutante no dependen del semiespacio; el control 3 con el GC sano
#     se re-midio por separado (checar-gc-221.sh: 6/6 verdes con 512 KB, 11 marcas y 3 cuadros).
#  4. EL RECORTE DEL BINARIO NO TIENE PALANCA (registro del limite conocido). El enlace con
#     -Wl,--gc-sections no tira NI UNA seccion y el binario sale 2.316 bytes MAS GRANDE por alineacion: no
#     hay codigo muerto que sacar. La palanca es lo que el GC pide, no el .text.
#
# Mutantes (cada uno exige que el corpus se ponga ROJO):
#   sin-guard  se saca el aviso de solape de pesar-motor.py         -> cae el control 1
#   gc-2048    el semiespacio del GC vuelve al default del vendor   -> cae el control 3 (gc_init)
#
# Uso:  bash scripts/console/checar-recorte-214.sh [dir_gox] [dir_elf] [dir_baseline]
#         SALTAR_ARRANQUE=1   no corre Flycast (deja el instrumento y el registro del linker)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-214-gc}"               # el gox del "recorte" (con los flags que gccgo ignora)
REC="${2:-/tmp/dcbuild/motor-214}"        # el ELF enlazado de ese gox
BASE="${3:-/tmp/dcbuild/motor-arranque}"  # el ELF del baseline (DC-002.13)
SEGS="${SEGS:-15}"
KB_OK=512                                  # el semiespacio del arranque con el GC SANO (DC-002.24). DC-002.14
                                           # tambien uso 512; DC-002.21 lo recorto a 256 MIDIENDO con el
                                           # defecto de la copia por Store Queue puesto (con la copia sana el
                                           # conjunto vivo del arranque es ~270 KB y 256 no alcanza: ver
                                           # scripts/console/fix-libgodc-sq.py).
# DC-002.21: los dos reenlaces de este corpus (el mutante y la vuelta) compilan el POC `motor-arranque` de
# HOY, y el gox de DC-002.14 ($GEN) ya no lo puede compilar: le faltan paquetes que se agregaron despues
# (`os.RamfsResumen`, DC-002.17; `json.Instrumento`, DC-002.18). Medido: "undefined identifier os.RamfsResumen".
# Esos dos reenlaces solo necesitan una `libgodc` rehecha con el valor bajo prueba, asi que usan el gox de la
# rama (el mismo que el resto de los corpus); el gox del recorte se sigue usando para el control 2 (pesar-gox)
# y su ELF para el control 4.
GOX_ACTUAL="${GOX_ACTUAL:-${GODC_GOX:-/tmp/gox-00220}}"
PESAR="$HERE/pesar-motor.py"
PESAR_GOX="$HERE/pesar-gox.py"
MEDIR="$HERE/medir-cargado.py"
BACKUP=/tmp/checar-214-pesar-motor-original.py
CAZADOS=0
TOTAL=2

for f in "$GEN/orden-motor.txt" "$GOX_ACTUAL/orden-motor.txt" "$BASE/prog.elf" "$REC/prog.elf" "$PESAR" "$PESAR_GOX" "$MEDIR"; do
    [ -f "$f" ] || { echo "falta $f (corre scripts/console/recorte-214.sh)" >&2; exit 1; }
done
[ "$(grep -o 'GC_SEMISPACE_SIZE_KB [0-9]*' "$LIBGODC/runtime/godc_config.h" | awk '{print $2}')" = "$KB_OK" ] || {
    echo "el semiespacio del GC no esta en $KB_OK KB: corre python3 scripts/console/fix-libgodc-gc.py $KB_OK --build" >&2
    exit 1
}
[ -f "$LIBGODC/libgodcbegin.a" ] || { echo "falta $LIBGODC/libgodcbegin.a: corre fix-libgodc-gc.py <KB> --build" >&2; exit 1; }

cargado() { python3 "$MEDIR" "$1" 2>/dev/null | head -1 | sed 's/.*: \([0-9]*\) bytes.*/\1/'; }

# veredicto del arranque, 4 estados (los mismos que mide medir-gc-semiespacio-214.sh)
arranque() { # $1 = dir de la corrida
    if grep -qa 'gc_init: alloc failed' "$1/flycast.log"; then
        echo "cae-gc-init diff $(grep -a -o 'diff [0-9]*' "$1/flycast.log" | tail -1 | cut -d' ' -f2)"
    elif grep -qa 'gc_external_alloc' "$1/flycast.log"; then
        echo "cae-gc-external diff $(grep -a -o 'diff [0-9]*' "$1/flycast.log" | tail -1 | cut -d' ' -f2)"
    elif grep -qa 'panic:' "$1/flycast.log"; then
        echo "pasa-el-techo-de-RAM panic='$(grep -a -m1 'panic:' "$1/flycast.log" | tr -d '\r' | sed 's/panic: //')'"
    else
        echo "arranca $(grep -ac 'MOTOR-ARRANQUE' "$1/flycast.log") marcas"
    fi
}

correr() { # $1 = elf, $2 = dir de salida
    rm -rf "$2"
    CAPTURE=none bash "$HERE/run-flycast.sh" "$1" "$SEGS" "$2" > "$2.salida" 2>&1
    arranque "$2"
}

mutar() {
    python3 - "$1" "$2" "$3" <<'PY'
import pathlib, sys
p = pathlib.Path(sys.argv[1]); viejo = sys.argv[2]; nuevo = sys.argv[3]
t = p.read_text(); n = t.count(viejo)
if n != 1:
    sys.exit("el texto a mutar aparece %d veces (esperaba 1)" % n)
p.write_text(t.replace(viejo, nuevo, 1))
PY
}

echo "== 1. control sano: el instrumento NO imprime la tabla solapada"
python3 "$PESAR" "$BASE/prog.elf" > /tmp/checar-214-inst.log 2>&1
rc=$?
echo "   pesar-motor.py sobre $BASE/prog.elf: exit $rc"
[ "$rc" = "2" ] || {
    echo "   EL CONTROL 1 NO DA (esperaba exit 2):" >&2
    sed -n '1,12p' /tmp/checar-214-inst.log | sed 's/^/     /' >&2
    exit 1
}
grep -m1 '\.text: suma' /tmp/checar-214-inst.log | sed 's/^/     /'
grep -m1 '\.bss: suma' /tmp/checar-214-inst.log | sed 's/^/     /'

echo
echo "== 2. control sano: el dato mas pesado del motor, medido por dos caminos"
python3 "$PESAR_GOX" "$GEN" 40 > /tmp/checar-214-gox.log 2>&1
dato_gox="$(grep -m1 '\.data\.go\.\.C3111' /tmp/checar-214-gox.log | awk '{print $1}')"
paquete="$(grep -m1 'github.com/OpenDiablo2/OpenDiablo2/d2core/d2records$' /tmp/checar-214-gox.log | awk '{print $1}')"
dato_elf="$(sh-elf-nm -S "$BASE/prog.elf" | awk '$4=="_go..C3111" && strtonum("0x" $2)>max {max=strtonum("0x" $2)} END{print max}')"
data_elf="$(sh-elf-size -A "$BASE/prog.elf" | awk '$1==".data"{print $2}')"
echo "   el .gox de d2core/d2records pesa $paquete bytes (secciones ALLOC)"
echo "   el dato mas grande de ese paquete (.data.go..C3111): $dato_gox bytes en el .gox, $dato_elf en el ELF"
echo "   .data entero del ELF: $data_elf bytes"
[ -n "$dato_gox" ] && [ "$dato_gox" = "$dato_elf" ] || {
    echo "   LOS DOS CAMINOS NO COINCIDEN ($dato_gox vs $dato_elf)" >&2
    exit 1
}
echo "   coinciden, y son el $(python3 -c "print(f'{$dato_gox * 100 / $data_elf:.1f}')")% del .data del motor"

echo
echo "== 3. control sano: el arranque del motor pasa TODAS las paredes de RAM"
# El ELF que se corre es el BASELINE (el de DC-002.13, sin recortar un byte): asi el numero dice que lo que
# hizo arrancar fue el recorte del GC y no un binario distinto.
if [ -z "${SALTAR_ARRANQUE:-}" ]; then
    v="$(correr "$BASE/prog-emu.elf" /tmp/checar-214-sano)"
    echo "   cargado: $(cargado "$BASE/prog.elf") bytes   el GC pide: $((2 * KB_OK * 1024)) (2 x $KB_OK KB)"
    echo "   arranque: $v"
    case "$v" in
    arranca*)
        # DC-002.25 (GC SANO, 512 KB): el arranque de HOY no muere en ninguna pared de RAM -- llega al motor y
        # completa cuadros. Es el mismo hecho que mide el control sano de checar-gc-221.sh (11 marcas, 3
        # cuadros). Los dos casos de abajo son la HISTORIA (medidos con el defecto de la copia por Store Queue
        # puesto) y se dejan para que se vea de donde venia el numero.
        echo "   OK (DC-002.25, GC sano): el arranque pasa las paredes de RAM y llega al motor ($v)"
        ;;
    pasa-el-techo-de-RAM*)
        echo "   OK: gc_init y gc_external_alloc quedaron atras; lo que sigue ya no es RAM"
        ;;
    "cae-gc-external diff 1916928")
        # DC-002.21: el estado de HOY con el semiespacio de esta rama. El runtime pasa sus paredes (no hay
        # gc_init) y lo que no entra es un pedido del MOTOR de 1.916.928 bytes. Se acepta DECLARADO.
        echo "   OK (DC-002.21): el runtime pasa sus paredes; la pared que queda es del MOTOR (1.916.928)"
        ;;
    *)
        echo "   EL ARRANQUE NO PASA EL TECHO DE RAM ($v)" >&2
        grep -a -E 'Out of memory|fatal error|panic:|Memory:' /tmp/checar-214-sano/flycast.log |
            tr -d '\r' | head -5 | sed 's/^/     /' >&2
        exit 1
        ;;
    esac
else
    echo "   (SALTAR_ARRANQUE: no se corre Flycast)"
fi

echo
echo "== 4. control sano (registro del limite): gc-sections NO baja el binario"
# OJO con el idioma: ld imprime el aviso en el idioma del sistema ("eliminando la seccion ... no
# utilizada"), asi que un grep en ingles cuenta 0 y hace creer que no tira nada. Medido: con LC_ALL=C
# tira 38.491 secciones... y el binario NO baja ni un byte util (sale 2.316 MAS GRANDE, por alineacion):
# lo que tira son secciones diminutas (stubs ..eq..f, .data vacias). El control es el DELTA, no el aviso.
# shellcheck disable=SC2046,SC2086
LC_ALL=C kos-cc -o /tmp/checar-214-gc.elf "$REC/main.o" "$REC/cshim.o" "$REC"/stdlib-*.o \
    $(tr '\n' ' ' < "$GEN/orden.txt") $(tr '\n' ' ' < "$GEN/orden-motor.txt") \
    -Wl,--gc-sections -Wl,--print-gc-sections $DC_LDPATHS $DC_LIBS_PRE $DC_LIBS_POST > /tmp/checar-214-gc.log 2>&1
tiradas="$(grep -ciE 'removing unused section|no utilizada' /tmp/checar-214-gc.log)"
nuevo="$(cargado "$REC/prog.elf")"
base="$(cargado "$BASE/prog.elf")"
delta=$((base - nuevo))
echo "   secciones que el linker tira: $tiradas"
echo "   cargado baseline: $base   cargado del recorte: $nuevo   delta: $delta bytes"
if [ "$delta" -ge 10240 ]; then
    echo "   EL LIMITE CAMBIO: el recorte del binario ahora baja $delta bytes: hay palanca" >&2
    exit 1
fi
echo "   el recorte NO baja: sale $((-delta)) bytes MAS GRANDE, y las $tiradas secciones que tira son"
echo "   diminutas (stubs ..eq..f, .data vacias de crtbegin.o)"
echo "   no hay codigo muerto que sacar con el linker: la palanca es lo que el GC PIDE, no el .text"

echo
echo "== mutante 'sin-guard' (se saca el aviso de solape de scripts/console/pesar-motor.py)"
cp "$PESAR" "$BACKUP"
if mutar "$PESAR" 'mentirosas = solapes(crudos, tam_real)' 'mentirosas = []'; then
    python3 "$PESAR" "$BASE/prog.elf" > /tmp/checar-214-inst-mut.log 2>&1
    rc=$?
    echo "   sin el control, sobre el ELF enlazado: exit $rc"
    # el modo "por paquete" (que no se llama con --secciones) imprime su tabla con este encabezado: si el
    # control se saca, la tabla solapada vuelve y el corpus se tiene que caer (control 1 pide exit 2).
    if [ "$rc" = "0" ] && grep -q 'peso atribuido a simbolos con tamano' /tmp/checar-214-inst-mut.log; then
        echo "   cazado: imprime la tabla solapada como si fuera una atribucion"
        grep -m1 'd2game' /tmp/checar-214-inst-mut.log | sed 's/^/     /'
        CAZADOS=$((CAZADOS + 1))
    else
        echo "   NO LO CAZO (exit $rc)" >&2
    fi
    cp "$BACKUP" "$PESAR"
else
    echo "   NO SE PUDO APLICAR EL MUTANTE" >&2
fi

echo
echo "== mutante 'gc-2048' (el semiespacio del GC vuelve al default del vendor)"
# OJO: el reenlace del mutante va a un directorio PROPIO (/tmp/dcbuild/motor-2048). Escribirlo encima del
# baseline (/tmp/dcbuild/motor-arranque) fue un defecto de este corpus: despues de esa corrida el
# "baseline" quedaba siendo el binario del recorte (13.913.308 en vez de 13.910.992) y los controles
# siguientes median otra cosa. Un mutante no puede pisar el artefacto que los controles comparan.
MUT=/tmp/dcbuild/motor-2048
if python3 "$HERE/fix-libgodc-gc.py" 2048 --build > /tmp/checar-214-gc-2048.log 2>&1 &&
    bash "$HERE/build-motor-arranque.sh" "$MUT" "$GOX_ACTUAL" > /tmp/checar-214-relink-2048.log 2>&1; then
    if [ -z "${SALTAR_ARRANQUE:-}" ]; then
        v="$(correr "$MUT/prog-emu.elf" /tmp/checar-214-mutante)"
        echo "   con el GC en 2048 KB (pide 4 MB): $v"
        case "$v" in
        cae-gc-init*)
            echo "   cazado: vuelve a morir en gc_init, que es el defecto que esta tarjeta encontro"
            CAZADOS=$((CAZADOS + 1))
            ;;
        *) echo "   NO LO CAZO ($v)" >&2 ;;
        esac
    else
        echo "   (SALTAR_ARRANQUE: este mutante necesita Flycast)" >&2
    fi
else
    echo "   NO SE PUDO APLICAR EL MUTANTE (ver /tmp/checar-214-gc-2048.log)" >&2
fi

echo
echo "== control sano de vuelta: el semiespacio en $KB_OK KB"
python3 "$HERE/fix-libgodc-gc.py" "$KB_OK" --build > /tmp/checar-214-vuelta.log 2>&1 || exit 1
bash "$HERE/build-motor-arranque.sh" /tmp/dcbuild/motor-512 "$GOX_ACTUAL" > /tmp/checar-214-relink-vuelta.log 2>&1 || exit 1
if [ -z "${SALTAR_ARRANQUE:-}" ]; then
    v="$(correr /tmp/dcbuild/motor-512/prog-emu.elf /tmp/checar-214-vuelta)"
    echo "   arranque con $KB_OK KB: $v"
    case "$v" in
    arranca*) echo "   control sano VERDE (DC-002.25: con el GC sano llega al motor)" ;;
    pasa-el-techo-de-RAM*) echo "   control sano VERDE" ;;
    "cae-gc-external diff 1916928")
        # DC-002.21: el arranque de HOY con el semiespacio de la rama. El runtime pasa sus paredes y lo que
        # no entra es un pedido del MOTOR (1.916.928 bytes): se acepta DECLARADO (ver el encabezado).
        echo "   control sano VERDE (DC-002.21: la pared que queda es del MOTOR, 1.916.928)"
        ;;
    *)
        echo "   NO VOLVIO EL CONTROL 3 ($v)" >&2
        exit 1
        ;;
    esac
fi
python3 "$PESAR" "$BASE/prog.elf" > /dev/null 2>&1
rc=$?
echo "   instrumento sobre el ELF enlazado: exit $rc (tiene que ser 2)"
[ "$rc" = "2" ] || { echo "   NO VOLVIO EL CONTROL 1" >&2; exit 1; }

echo
echo "== $CAZADOS/$TOTAL mutantes cazados =="
echo "   (si un mutante queda verde, el hueco es del corpus)"
[ "$CAZADOS" -eq "$TOTAL" ]
