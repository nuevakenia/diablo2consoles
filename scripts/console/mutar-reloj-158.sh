#!/bin/bash
# Mutadores del RELOJ de la consola (DC-002.158): cada mutante rompe UNA cosa que mide el POC del reloj
# (console/dc/poc/reloj-158). Si el POC sigue VERDE con el mutante puesto, el hueco es del POC.
#
# POR QUE HACE FALTA. El reloj de la consola no se puede probar en el host: habla con la RTC y con el timer de
# KOS. Su unica prueba es este POC en Flycast, y una prueba que nunca vio un defecto no prueba nada: el defecto
# de la tarjeta (la RTC leida en cada Now, que hacia adelantar el reloj 2x) vivio sin que nadie lo viera hasta
# DC-002.157, con todo el mundo mirando `CUADRO n ms=`.
#
# LOS TRES MUTANTES:
#   1. rtc-en-cada-now        el defecto de la tarjeta: la RTC se vuelve a leer en cada llamada. Tiene que dar
#                             ~2000 por mil en las esperas que cruzan un segundo (1000, 700 y 2000 ms).
#   2. unixnano-sin-nsec      UnixNano devuelve solo los segundos: las esperas de 150, 300 y 700 ms salen con
#                             delta cero (o de un segundo redondo).
#   3. nsec-en-microsegundos  el nsec se guarda en microsegundos en vez de nanosegundos (la escala de la linea
#                             que arreglo esta tarjeta): el delta de 1 s sale ~1000 veces mas chico.
#
# Restaurar NO se hace con git checkout --: se respalda el archivo en /tmp y se verifica con sha256sum (regla
# del proyecto: el arbol del hito puede tener arreglos sin commitear).
#
# Uso:  scripts/console/mutar-reloj-158.sh
# Sale 0 solo si el control da verde y los TRES mutantes son cazados.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
cd "$HERE/../.." || exit 1
# shellcheck disable=SC1091
source scripts/console/env.sh

RELOJ=console/dc/std/time/time.go
POC=console/dc/poc/reloj-158
ELF=/tmp/dcbuild/reloj-158/prog.elf      # lo que deja build-dc.sh por defecto para ese POC
SAL=/tmp/mutar-reloj-158
RESP="$SAL/time.go.orig"
mkdir -p "$SAL"
cp "$RELOJ" "$RESP"
SHA0="$(sha256sum "$RELOJ" | cut -d' ' -f1)"
echo "time.go de partida: sha256 $SHA0"

muta() {  # $1 = buscar, $2 = poner
    python3 - "$RELOJ" "$1" "$2" <<'PY'
import sys
ruta, buscar, poner = sys.argv[1], sys.argv[2], sys.argv[3]
texto = open(ruta).read()
if buscar not in texto:
    print("  no encontre el fragmento a mutar", file=sys.stderr)
    sys.exit(3)
open(ruta, "w").write(texto.replace(buscar, poner, 1))
PY
}

# corre_poc: compila la stdlib + el POC y lo corre 14 s en Flycast; deja el RESUMEN en $SAL/resultado
corre_poc() {
    : > "$SAL/resultado"
    if ! bash scripts/console/build-std-gox.sh >"$SAL/build-std.log" 2>&1; then
        echo "la stdlib no compila" > "$SAL/resultado"
        return
    fi
    if ! bash scripts/console/build-dc.sh "$POC" >"$SAL/build-poc.log" 2>&1; then
        echo "el POC no compila" > "$SAL/resultado"
        return
    fi
    rm -rf "$SAL/run"
    CAPTURE=none bash scripts/console/run-flycast.sh "$ELF" 14 "$SAL/run" >/dev/null 2>&1
    tr -d '\r' < "$SAL/run/flycast.log" 2>/dev/null | grep -a 'RELOJ158: RESUMEN' | tail -1 > "$SAL/resultado"
    if [ ! -s "$SAL/resultado" ]; then
        echo "(el POC no imprimio el resumen)" > "$SAL/resultado"
    fi
}

verde() { grep -q 'malas= 0' "$1"; }

echo
echo "== control: sin mutar tiene que salir VERDE =="
corre_poc
echo "  $(cat "$SAL/resultado")"
if ! verde "$SAL/resultado"; then
    echo "CONTROL MAL: con el codigo sin tocar el POC no da verde. Se corta."
    exit 1
fi

cazados=0
total=0
probar() {  # $1 nombre, $2 buscar, $3 poner, $4 que tiene que romper
    total=$((total + 1))
    echo
    echo "== mutante $total: $1 ($4) =="
    cp "$RESP" "$RELOJ"
    if ! muta "$2" "$3"; then
        echo "  NO SE PUDO APLICAR (se saltea)"
        cp "$RESP" "$RELOJ"
        total=$((total - 1))
        return
    fi
    corre_poc
    echo "  $(cat "$SAL/resultado")"
    if verde "$SAL/resultado"; then
        echo "  VERDE: el mutante NO fue cazado. Hueco del POC."
    else
        echo "  ROJO: cazado."
        cazados=$((cazados + 1))
    fi
    cp "$RESP" "$RELOJ"
}

# 1. EL DEFECTO DE LA TARJETA: la RTC leida en CADA llamada, con el tiempo transcurrido sumado encima.
probar "la RTC en cada Now (el defecto de DC-002.158)" \
    "	desde := us - arranqueUs
	return Time{sec: baseUnix + int64(desde/1000000), nsec: int32(desde%1000000) * 1000}" \
    "	desde := us - arranqueUs
	return Time{sec: d2c_reloj_unix() + int64(desde/1000000), nsec: int32(desde%1000000) * 1000}" \
    "las esperas que cruzan un segundo dan 2000 por mil"

# 2. los subsegundos fuera de UnixNano (la conversion a float64 de d2util pierde la parte chica)
probar "UnixNano sin los nanosegundos" \
    "func (t Time) UnixNano() int64 { return t.sec*1000000000 + int64(t.nsec) }" \
    "func (t Time) UnixNano() int64 { return t.sec * 1000000000 }" \
    "las esperas de 150, 300 y 700 ms dan delta cero"

# 3. la escala del nsec (microsegundos guardados como si fueran nanosegundos): el segundo entero sigue bien,
#    pero la fraccion queda 1000 veces mas chica, asi que las esperas que NO cruzan el borde del segundo dan
#    delta ~0.
probar "el nsec en microsegundos" \
    "nsec: int32(desde%1000000) * 1000}" \
    "nsec: int32(desde%1000000)}" \
    "las esperas de 150, 300 y 700 ms dan delta ~0"

echo
echo "== restauro y verifico =="
cp "$RESP" "$RELOJ"
SHA1="$(sha256sum "$RELOJ" | cut -d' ' -f1)"
if [ "$SHA0" != "$SHA1" ]; then
    echo "EL ARCHIVO QUEDO DISTINTO: $SHA0 -> $SHA1 (revisar $RESP)" >&2
    exit 1
fi
echo "  time.go quedo byte por byte igual (sha256 $SHA1)"

# Y EL ARBOL COMPILADO SE REARMA. El ultimo corre_poc dejo la stdlib compilada con el MUTANTE puesto: si el
# script terminara aca, el proximo build que enlace $DC_ROOT/gox (el motor, el Gate 0) se llevaria el reloj
# roto sin avisar. Este control final compila con el archivo bueno y lo mide, o sea que deja el arbol en el
# estado que el resto del proyecto espera.
echo
echo "== control final: la stdlib se rearma con el archivo bueno =="
corre_poc
echo "  $(cat "$SAL/resultado")"
if ! verde "$SAL/resultado"; then
    echo "EL CONTROL FINAL NO DA VERDE: el arbol de la stdlib quedo con el mutante." >&2
    exit 1
fi

echo
echo "== resultado: $cazados de $total mutantes cazados =="
if [ "$cazados" -eq "$total" ] && [ "$total" -gt 0 ]; then
    echo "VERDE: el POC del reloj caza los $total mutantes."
    exit 0
fi
echo "ROJO: hay mutantes que el POC no caza." >&2
exit 1
