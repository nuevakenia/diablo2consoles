#!/bin/bash
# CORPUS DE DC-002.17: el motor que ya llega a `main` y lo frenaba el ENTORNO (el romdisk de solo lectura y
# el video que pedia 1 MiB que no habia).
#
# Lo que mide, con el arranque del motor entero en Flycast (el mismo binario de build-motor-arranque.sh):
#
#  CONTROL 1 (video): el backend del PVR arranca y el log NO tiene "codigo -3" ni "Out of memory". Antes
#    de esta tarjeta el arranque decia "el video no arranco: ... (codigo -3)" con un pedido de 1.048.576
#    bytes al monton que no entraba en los 16 MB.
#  CONTROL 2 (config del motor): el motor PUDO crear el directorio y el archivo de su configuracion
#    (marcas OS-RAMFS mkdir-ok /rom/OpenDiablo2 y create-ok .../config.json) y el log NO tiene
#    "read-only filesystem". Antes de esta tarjeta lo unico que habia era
#    "OS-ESCRITURA MkdirAll path= /rd/OpenDiablo2" y el Run() muriendo ahi.
#  CONTROL 3 (escritura de verdad): la prueba del propio arranque escribe 67 bytes en el overlay de la RAM,
#    los lee de vuelta, el contenido es IGUAL y despues de borrar quedan 0 archivos. Este control existe
#    porque el motor NO alcanza a escribir: su `d2config.Save()` se cae antes, en `json.MarshalIndent` (la
#    consola no tiene reflexion), asi que sin esta prueba la escritura quedaria probada por nadie.
#
#  MUTANTE 'sin-overlay': se deshace el sistema de archivos escribible (d2c_mkdir y d2c_abrir_crear vuelven
#    a fallar como antes) y los CONTROLES 2 y 3 se tienen que poner ROJOS. Es el mutante del defecto de
#    esta tarjeta: si el overlay no esta, el motor vuelve a morir en el mkdir de su config.
#  MUTANTE 'calloc-1mib': vuelve el `calloc` de 1 MiB del buffer del cuadro (el defecto del video) y el
#    CONTROL 1 se tiene que poner ROJO con "codigo -3".
#
#  CONTROL SANO DE VUELTA: con las dos cosas repuestas, los tres controles vuelven a verde.
#
# Uso:  bash scripts/console/checar-entorno-217.sh [dir_gox] [dir_build] [segundos]
#       GODC_GOX tiene que estar EXPORTADO (los terceros de la gox, ver README 32).
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-${GODC_GOX:-/tmp/gox-00216}}"
BUILD="${2:-/tmp/dcbuild/motor-217}"
SEGS="${3:-20}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
OSDC="$REPO/console/dc/std/csrc/os_dc.c"
PVRC="$REPO/console/dc/std/csrc/pvr_pantalla.c"
FIJO_OS=/tmp/checar-217-os_dc.c
FIJO_PVR=/tmp/checar-217-pvr_pantalla.c
CAZADOS=0
TOTAL=2

[ -f "$OSDC" ] || { echo "falta $OSDC" >&2; exit 1; }
[ -f "$PVRC" ] || { echo "falta $PVRC" >&2; exit 1; }
# si el overlay no esta en la fuente, la corrida mediria otra cosa: mejor no correr que mentir
grep -q 'd2c_ramfs_resumen' "$OSDC" || {
    echo "os_dc.c NO tiene el overlay de la RAM (DC-002.17): no hay que medir." >&2
    exit 1
}

# ---------------------------------------------------------------- la corrida y su veredicto
correr() { # $1 = dir de la corrida; deja el log en $1/flycast.log
    local dir="$1"
    if ! bash "$HERE/build-motor-arranque.sh" "$BUILD" "$GEN" > "$dir.build.log" 2>&1; then
        echo "   no-enlaza (ver $dir.build.log)"
        return 1
    fi
    rm -rf "$dir"
    CAPTURE=none bash "$HERE/run-flycast.sh" "$BUILD/prog-emu.elf" "$SEGS" "$dir" > "$dir.salida" 2>&1
    [ -f "$dir/flycast.log" ]
}

# control 1: el video arranca y el monton no se queda sin el MiB
ctrl_video() {
    local log="$1/flycast.log"
    grep -qa 'MOTOR-ARRANQUE video listo' "$log" || return 1
    grep -qa 'codigo -3' "$log" && return 1
    grep -qa 'Out of memory' "$log" && return 1
    return 0
}

# control 2: el motor pudo con su configuracion (mkdir + create) y no hay error de solo lectura
ctrl_config() {
    local log="$1/flycast.log"
    grep -qa 'OS-RAMFS mkdir-ok path= /rd/OpenDiablo2' "$log" || return 1
    grep -qa 'OS-RAMFS create-ok path= /rd/OpenDiablo2/config.json' "$log" || return 1
    grep -qa 'read-only filesystem' "$log" && return 1
    return 0
}

# control 3: la escritura y la lectura de vuelta del overlay (la prueba del propio arranque)
ctrl_escritura() {
    local log="$1/flycast.log"
    grep -qaE 'OS-RAMFS prueba: escribi [0-9]+ bytes, lei [0-9]+ igual= ?true' "$log" || return 1
    grep -qa 'OS-RAMFS prueba: despues de borrar -> archivos= 0 bytes= 0' "$log" || return 1
    return 0
}

controles() { # $1 = dir de la corrida; imprime los tres y devuelve cuantos dan bien
    local dir="$1" bien=0
    if ctrl_video "$dir"; then echo "   control 1 (video):            VERDE"; bien=$((bien + 1));
    else echo "   control 1 (video):            ROJO"; fi
    if ctrl_config "$dir"; then echo "   control 2 (config del motor): VERDE"; bien=$((bien + 1));
    else echo "   control 2 (config del motor): ROJO"; fi
    if ctrl_escritura "$dir"; then echo "   control 3 (escritura):        VERDE"; bien=$((bien + 1));
    else echo "   control 3 (escritura):        ROJO"; fi
    echo "$bien"
}

echo "== 1. control sano: el video arranca y el motor puede escribir su configuracion"
correr /tmp/checar-217-sano || exit 1
BIEN="$(controles /tmp/checar-217-sano | sed -n '$p')"
controles /tmp/checar-217-sano | sed -n '1,3p'
if [ "$BIEN" -ne 3 ]; then
    echo "   EL CONTROL SANO NO DA ($BIEN/3): no se sigue, primero hay que ver por que" >&2
    grep -a 'MOTOR-ARRANQUE\|OS-' /tmp/checar-217-sano/flycast.log | tr -d '\r' | sed 's/^/     /' >&2
    exit 1
fi
echo "   OK: 3/3 controles verdes con el codigo de ahora"
grep -a 'MOTOR-ARRANQUE\|OS-' /tmp/checar-217-sano/flycast.log | tr -d '\r' | sed 's/^/     /'

# ---------------------------------------------------------------- MUTANTE 1: sin overlay
echo
echo "== mutante 'sin-overlay' (el sistema de archivos escribible vuelve a fallar como en DC-002.16)"
cp "$OSDC" "$FIJO_OS"
python3 - "$OSDC" <<'PY'
import pathlib, sys
p = pathlib.Path(sys.argv[1])
t = p.read_text()
# d2c_mkdir vuelve a decir que no, y d2c_abrir_crear vuelve a no dar descriptor: es exactamente lo que
# hacian los stubs antes de esta tarjeta (ErrNoSePuede en el Go, -1 en el C).
a = "int d2c_mkdir(const char *ruta) {\n    int i;"
if a not in t:
    sys.exit("no encuentro el cuerpo de d2c_mkdir: el mutante no se aplico")
t = t.replace(a, "int d2c_mkdir(const char *ruta) {\n    int i;\n    return -1;   /* MUTANTE sin-overlay */", 1)
b = "int d2c_abrir_crear(const char *ruta) {\n    int i = ramfs_crear(ruta);"
if b not in t:
    sys.exit("no encuentro el cuerpo de d2c_abrir_crear: el mutante no se aplico")
t = t.replace(b, "int d2c_abrir_crear(const char *ruta) {\n    int i = ramfs_crear(ruta);\n    return D2C_NO_VALIDO;   /* MUTANTE sin-overlay */", 1)
p.write_text(t)
print("   os_dc.c mutado: d2c_mkdir -> -1, d2c_abrir_crear -> invalido")
PY
[ $? -eq 0 ] || { cp "$FIJO_OS" "$OSDC"; echo "   el mutante no se pudo aplicar" >&2; exit 1; }
if correr /tmp/checar-217-mut-sin-overlay; then
    controles /tmp/checar-217-mut-sin-overlay | sed -n '1,3p'
    if ctrl_config /tmp/checar-217-mut-sin-overlay || ctrl_escritura /tmp/checar-217-mut-sin-overlay; then
        echo "   NO LO CAZO: el overlay no esta y los controles 2/3 siguen verdes" >&2
    else
        echo "   cazado: sin el overlay el motor no puede con su config y la escritura no existe"
        CAZADOS=$((CAZADOS + 1))
    fi
else
    echo "   cazado de otra forma: ni siquiera enlaza/corre sin el overlay"
    CAZADOS=$((CAZADOS + 1))
fi
cp "$FIJO_OS" "$OSDC"

# ---------------------------------------------------------------- MUTANTE 2: el calloc de 1 MiB
echo
echo "== mutante 'calloc-1mib' (vuelve el buffer del cuadro entero en el monton: el defecto del video)"
cp "$PVRC" "$FIJO_PVR"
python3 - "$PVRC" <<'PY'
import pathlib, sys
p = pathlib.Path(sys.argv[1])
t = p.read_text()
a = "    pant_tex = pvr_mem_malloc((size_t)PANT_PW * (size_t)PANT_PH * 2);\n    if (pant_tex == NULL) {\n        return -2;\n    }\n"
if a not in t:
    sys.exit("no encuentro la reserva de VRAM: el mutante no se aplico")
# el pedido viejo: el cuadro entero convertido, 1024x512x2 = 1.048.576 bytes del monton
m = a + ("\n    {   /* MUTANTE calloc-1mib: el buffer del cuadro entero, como antes de DC-002.17 */\n"
         "        void *mut_conv = calloc((size_t)PANT_PW * (size_t)PANT_PH, sizeof(uint16_t));\n"
         "        if (mut_conv == NULL) {\n"
         "            return -3;\n"
         "        }\n"
         "    }\n")
t = t.replace(a, m, 1)
p.write_text(t)
print("   pvr_pantalla.c mutado: vuelve el calloc de 1.048.576 bytes")
PY
[ $? -eq 0 ] || { cp "$FIJO_PVR" "$PVRC"; echo "   el mutante no se pudo aplicar" >&2; exit 1; }
if correr /tmp/checar-217-mut-calloc; then
    controles /tmp/checar-217-mut-calloc | sed -n '1,3p'
    if ctrl_video /tmp/checar-217-mut-calloc; then
        echo "   NO LO CAZO: con el calloc de 1 MiB el video sigue arrancando" >&2
    else
        echo "   cazado: el video vuelve a caer (codigo -3 / Out of memory)"
        grep -a 'codigo -3\|Out of memory' /tmp/checar-217-mut-calloc/flycast.log | tr -d '\r' | sed 's/^/     /'
        CAZADOS=$((CAZADOS + 1))
    fi
else
    echo "   cazado de otra forma: ni siquiera enlaza/corre con el buffer entero"
    CAZADOS=$((CAZADOS + 1))
fi
cp "$FIJO_PVR" "$PVRC"

# ---------------------------------------------------------------- control sano de vuelta
echo
echo "== control sano de vuelta (el overlay y el buffer por bloques repuestos)"
if correr /tmp/checar-217-vuelta; then
    BIEN="$(controles /tmp/checar-217-vuelta | sed -n '$p')"
    controles /tmp/checar-217-vuelta | sed -n '1,3p'
    [ "$BIEN" -eq 3 ] || { echo "   NO VOLVIERON LOS TRES CONTROLES ($BIEN/3)" >&2; exit 1; }
    echo "   control sano VERDE"
else
    echo "   NO VOLVIO EL CONTROL SANO" >&2
    exit 1
fi

echo
echo "== $CAZADOS/$TOTAL mutantes cazados =="
[ "$CAZADOS" -eq "$TOTAL" ]
