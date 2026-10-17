#!/bin/bash
# Mutador del cuelgue intermitente de la prueba diferencial en 386 (DC-002.8b).
#
# EL DEFECTO (medido antes de arreglarlo): "GOARCH=386 go test -count=20 ./difftest/" se colgo 1 vez en
# 40 corridas (una corrida = 20 ejecuciones del test), con la goroutine clavada en
# d2std/sync.(*RWMutex).RUnlock -> (*Mutex).Unlock -> <-m.libre, o sea esperando un testigo que ya no
# estaba. El culpable era el bloque de dos goroutines de TestRWMutexContraGo: usa un candado `mDC` que
# NADIE preparaba (el preparar() de ese test era para otro candado, `rwDC`), asi que las dos goroutines
# creaban el canal del candado a la vez y se quedaban con DOS testigos distintos.
#
# POR QUE EL MUTADOR NO PUEDE SER "sacar el preparar y esperar el cuelgue": el cuelgue es RARISIMO
# (medido con una copia instrumentada: 2 en 300 rondas de 16 goroutines, 0 en 300 con 2, 4 u 8), asi que
# un mutante probabilistico queda verde casi siempre y no prueba nada. Lo que se vigila es la CAUSA:
#   mutante 1: un candado usado en paralelo SIN preparar -> lo caza el aserto de Preparado(), en el acto.
#   mutante 2: el candado que crea un canal NUEVO en cada uso -> lo caza la comparacion de valores (es el
#              mismo estrago que los dos testigos: el candado deja de excluir).
#
# Uso:  scripts/console/mutar-cuelgue-386.sh        (solo host, no usa Flycast: es rapido)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
STD="${STD:-$REPO/console/dc/std}"
TMP="$(mktemp -d)"
EN_CURSO=""
trap 'restaurar_al_morir' EXIT

restaurar_al_morir() {
    if [ -n "$EN_CURSO" ] && [ -f "$TMP/orig" ]; then
        cp "$TMP/orig" "$STD/$EN_CURSO"
        echo "== el script murio a la mitad: $EN_CURSO restaurado (revisar el sha256) =="
    fi
    rm -rf "$TMP"
}

rc=0
contador=0
cazados=0

# El respaldo es una copia propia, nunca `git checkout --`: en medio del hito hay arreglos SIN COMMITEAR
# en los mismos archivos y un checkout los borraria. Se verifica con sha256 al restaurar.
aplicar() { # <archivo-relativo-a-STD> <buscar> <poner>
    local arch="$1" buscar="$2" poner="$3"
    cp "$STD/$arch" "$TMP/orig"
    EN_CURSO="$arch"
    sha256sum "$TMP/orig" | awk '{print $1}' > "$TMP/orig.sha"
    python3 - "$STD/$arch" "$buscar" "$poner" <<'PY' || return 1
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

restaurar() { # <archivo>
    local arch="$1"
    cp "$TMP/orig" "$STD/$arch"
    if [ "$(sha256sum "$STD/$arch" | awk '{print $1}')" != "$(cat "$TMP/orig.sha")" ]; then
        echo "   EL ARBOL QUEDO MUTADO: no coinciden los sha256 de $arch"
        return 1
    fi
    EN_CURSO=""
    return 0
}

mutacion() { # <nombre> <archivo> <buscar> <poner>
    local nombre="$1" arch="$2" buscar="$3" poner="$4"
    contador=$((contador + 1))
    echo "== mutante $contador: $nombre"
    if ! aplicar "$arch" "$buscar" "$poner"; then
        echo "   NO SE APLICO LA MUTACION (patron sin match unico): el mutante no prueba nada"
        restaurar "$arch"
        rc=1
        return
    fi
    (cd "$STD" && GOARCH=386 go test -timeout 60s ./difftest/ >"$TMP/salida" 2>&1)
    if grep -qaE -- "--- FAIL|^fatal error|SIGSEGV|^panic:" "$TMP/salida"; then
        echo "   cazado: la suite se puso roja:"
        grep -am1 -- "--- FAIL" "$TMP/salida" | sed 's/^/     /'
        grep -am1 -- "el candado quedo SIN preparar\|_test.go:[0-9]*:" "$TMP/salida" | cut -c1-160 | sed 's/^/     /'
        grep -am1 -E "test timed out|^panic:|^fatal error" "$TMP/salida" | cut -c1-160 | sed 's/^/     (por tiempo o caida) /'
        cazados=$((cazados + 1))
    elif grep -qaE "build failed|cannot|undefined|syntax error" "$TMP/salida"; then
        echo "   LA MUTACION NO COMPILA (no prueba nada de la suite):"
        grep -am1 -E "build failed|cannot|undefined|syntax error" "$TMP/salida" | cut -c1-160 | sed 's/^/     /'
        rc=1
    else
        echo "   LA SUITE NO LO CAZO: paso igual con el mutante puesto (compilando)"
        rc=1
    fi
    restaurar "$arch" || rc=1
    echo
}

echo "== control sano: la suite tiene que estar VERDE antes de mutar nada =="
for arch in amd64 386; do
    if (cd "$STD" && GOARCH="$arch" go test -timeout 60s ./difftest/ >"$TMP/sano-$arch" 2>&1); then
        echo "   GOARCH=$arch verde"
    else
        echo "   GOARCH=$arch ROJO sin mutar nada: arreglar eso primero"
        sed -n '1,10p' "$TMP/sano-$arch" | sed 's/^/     /'
        rc=1
    fi
done
if [ "$rc" -ne 0 ]; then
    echo "== el control no esta verde: se corta aca =="
    exit 1
fi

# El control del DEFECTO: es el que dice si el arreglo sirvio. 5 corridas de -count=20 en 386 son 100
# ejecuciones del test del candado; antes del arreglo, 1 cuelgue en 40 corridas; con el candado
# preparado, 0. Si alguna se cuelga, el arreglo no esta y el mutador se corta.
echo
echo "== control del cuelgue: 5 corridas de -count=20 en 386 (100 ejecuciones), 0 cuelgues esperados =="
colgados=0
for i in 1 2 3 4 5; do
    if (cd "$STD" && GOARCH=386 go test -count=20 -timeout 60s ./difftest/ >"$TMP/c20-$i" 2>&1); then
        echo "   corrida $i: verde"
    else
        echo "   corrida $i: ROJA (¿se colgo?)"
        grep -am1 -E "test timed out|^panic:|--- FAIL" "$TMP/c20-$i" | cut -c1-160 | sed 's/^/     /'
        colgados=$((colgados + 1))
    fi
done
if [ "$colgados" -ne 0 ]; then
    echo "== $colgados de 5 corridas se cayeron: el cuelgue intermitente esta de vuelta =="
    exit 1
fi
echo

echo "== 1. un candado usado en paralelo SIN preparar tiene que cazarse (era el defecto) =="
# Este es EXACTAMENTE el defecto de DC-002.8b: el bloque de dos goroutines usa un candado al que nadie
# le creo el canal. Antes se colgaba 1 vez cada 40 corridas; ahora la exigencia de Preparado() lo caza
# en el acto, todas las veces.
mutacion "difftest: candado del bloque de 2 goroutines sin preparar" \
    "difftest/dc0027_test.go" \
    $'\tpreparar(t, &mDC)\n\twgGo.Add(2)' \
    $'\twgGo.Add(2)'

echo "== 2. un candado que crea un canal nuevo en cada uso deja de excluir =="
# Es el estrago de los dos testigos, pero deterministico: con un canal nuevo por uso, el Lock nunca
# espera y las 8 goroutines del escenario suman sin candado (el contador deja de dar 16000 exacto).
mutacion "sync: el candado crea el canal en cada uso" \
    "sync/sync.go" \
    $'\tif m.libre == nil {\n\t\tm.libre = make(chan struct{}, 1)\n\t}\n\treturn m.libre' \
    $'\treturn make(chan struct{}, 1)'

echo
echo "== $cazados/$contador chequeos cazados =="
if [ "$rc" -eq 0 ]; then
    echo "== OK: la suite caza el candado sin preparar y el candado que no excluye, y el arbol quedo sano =="
else
    echo "== HAY HUECOS (arriba) =="
fi
exit "$rc"
