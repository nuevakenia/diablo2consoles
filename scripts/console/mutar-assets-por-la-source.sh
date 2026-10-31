#!/bin/bash
# MUTADOR de la regla de los assets en el camino de consola (DC-002.95).
#
# POR QUÉ EXISTE. Una regla que no caza su propio caso no prueba nada: si el chequeo saliera verde siempre,
# toda esta tarjeta sería un verde mentiroso. Acá se le ponen a propósito las seis cosas que la regla dice
# distinguir, sobre una COPIA del árbol en /tmp (el repo no se toca ni con un byte):
#
#   1. CONTROL LIMPIO      la copia sin tocar tiene que dar VERDE (si esto falla, no se midió nada)
#   2. os.ReadFile         una lectura NUEVA en un archivo que no tenía ninguna          -> ROJO
#   3. io.ReadAll          la otra forma de la misma falta (la del buffer que se duplica) -> ROJO
#   4. mpq.ReadFile        la lectura del MPQ en una función NUEVA: la deuda no la tapa  -> ROJO
#   5. mpq.ReadFileInto    el MISMO sitio por el camino con dueño: NO es una falta       -> VERDE
#   6. PROSA               un comentario que NOMBRA io.ReadAll y os.ReadFile             -> VERDE
#   7. DEUDA PAGADA        una entrada de la deuda que ya no está en el árbol             -> VERDE + aviso
#
# El 5, el 6 y el 7 son los que importan tanto como los rojos: prueban que la regla distingue el camino bueno
# del malo (no se pone roja con cualquier cosa) y que la deuda se puede pagar sin dejar el chequeo en rojo.
#
# Uso:  scripts/console/mutar-assets-por-la-source.sh        (sale 0 si los 7 casos dan lo esperado)
#       DEST=/tmp/otro scripts/console/mutar-assets-por-la-source.sh
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
DEST="${DEST:-/tmp/regla-mutante-95}"
DEUDA="$REPO/scripts/console/assets-fuera-de-la-source.deuda.txt"
SALIDA="$DEST/salida.txt"

[ -f "$DEUDA" ] || { echo "no encuentro la deuda en $DEUDA" >&2; exit 2; }
command -v go >/dev/null 2>&1 || { echo "no hay 'go' en el PATH (la regla corre en el host)" >&2; exit 2; }

echo "== mutador de la regla: copia del alcance en $DEST (el repo no se toca) =="
rm -rf "$DEST"
mkdir -p "$DEST"

# La lista del alcance sale de la PROPIA regla (-alcance): si se escribiera acá también, habría dos listas y se
# separarían (la lección de DC-002.90).
mapfile -t DIRS < <(cd "$REPO" && go run ./scripts/console/regla-assets-por-la-source -alcance)
[ "${#DIRS[@]}" -ge 1 ] || { echo "la regla no devolvió alcance" >&2; exit 2; }
for d in "${DIRS[@]}"; do
    mkdir -p "$DEST/$d"
    cp -a "$REPO/$d/." "$DEST/$d/" || { echo "no pude copiar $d" >&2; exit 2; }
done
echo "   alcance copiado: ${#DIRS[@]} directorios"
echo

correr() { # desde el repo, contra la copia y la deuda real (o la que venga en $1)
    local deuda="${1:-$DEUDA}"
    ( cd "$REPO" && go run ./scripts/console/regla-assets-por-la-source -raiz "$DEST" -deuda "$deuda" ) \
        > "$SALIDA" 2>&1
}

# Los archivos de los casos. Se restauran TODOS antes de cada caso: el mutante del caso anterior tiene que
# desaparecer, o el rojo del caso 2 contamina el 5 (medido: así falló la primera corrida de este mutador).
ARCHIVOS=(
    console/dc/poc/mapa-101/main.go
    console/dc/poc/fuente-consola/main.go
    console/dc/poc/personaje-mapa/main.go
    d2core/d2audio/aica/proveedor.go
)

restaurar() {
    local a
    for a in "${ARCHIVOS[@]}"; do
        cp -a "$REPO/$a" "$DEST/$a" || { echo "no pude restaurar $a" >&2; exit 2; }
    done
}

CASOS=0
BIEN=0
MAL=""
probar() { # nombre  esperado(verde|rojo)  archivo(relativo)  inyeccion
    local nombre="$1" esperado="$2" archivo="$3" inyeccion="$4" codigo obtenido
    restaurar
    printf '%s\n' "$inyeccion" >> "$DEST/$archivo"
    correr
    codigo=$?
    case $codigo in
        0) obtenido=verde ;;
        1) obtenido=rojo ;;
        *) obtenido="error($codigo)" ;;
    esac
    CASOS=$((CASOS + 1))
    if [ "$obtenido" = "$esperado" ]; then
        BIEN=$((BIEN + 1))
        printf '   OK     %-46s esperado %-5s dio %-5s\n' "$nombre" "$esperado" "$obtenido"
    else
        MAL="$MAL $nombre"
        printf '   FALLO  %-46s esperado %-5s dio %-5s\n' "$nombre" "$esperado" "$obtenido"
        sed -n '1,30p' "$SALIDA" | sed 's/^/            /'
    fi
}

# ---- 1) CONTROL: la copia limpia tiene que estar verde -----------------------------------------------
restaurar
correr
codigo=$?
CASOS=$((CASOS + 1))
if [ "$codigo" -eq 0 ]; then
    BIEN=$((BIEN + 1))
    printf '   OK     %-46s esperado %-5s dio %-5s\n' "CONTROL: la copia limpia" "verde" "verde"
    grep -E '^(archivos .go revisados|lecturas fuera de la Source|deuda declarada)' "$SALIDA" | sed 's/^/            /'
else
    MAL="$MAL CONTROL-LIMPIO"
    printf '   FALLO  %-46s esperado %-5s\n' "CONTROL: la copia limpia" "verde"
    sed -n '1,30p' "$SALIDA" | sed 's/^/            /'
    echo
    echo "ROJO: sin árbol limpio no se puede medir nada. Se corta acá." >&2
    exit 1
fi
echo

# ---- 2..6) los casos de la regla --------------------------------------------------------------------
probar "os.ReadFile en un archivo limpio" rojo console/dc/poc/mapa-101/main.go \
'// MUTANTE (DC-002.95): una lectura nueva, sin dueño, en un archivo que estaba limpio.
func mutanteUno() ([]byte, error) { return os.ReadFile("/rd/x") }'

probar "io.ReadAll en un archivo limpio" rojo console/dc/poc/fuente-consola/main.go \
'// MUTANTE (DC-002.95): el io.ReadAll que este árbol NO usa, en un archivo limpio.
func mutanteDos(r io.Reader) ([]byte, error) { return io.ReadAll(r) }'

probar "mpq.ReadFile en una función nueva" rojo console/dc/poc/personaje-mapa/main.go \
'// MUTANTE (DC-002.95): el camino viejo en una función que la deuda no conoce.
func mutanteTres() { _, _ = mpq.ReadFile("x.dt1") }'

probar "mpq.ReadFileInto (camino con dueño)" verde console/dc/poc/personaje-mapa/main.go \
'// CONTROL (DC-002.95): el MISMO sitio, por el camino con dueño -- la regla no lo castiga.
func mutanteConDueno() { var dst []byte; _, _ = mpq.ReadFileInto("x.dt1", dst, nil) }'

probar "prosa que nombra el camino viejo" verde console/dc/poc/mapa-101/main.go \
'// CONTROL (DC-002.95): la prosa NOMBRA el camino viejo y no es una llamada.
// El camino viejo era io.ReadAll(stream) y mpq.ReadFile(nombre); os.ReadFile tampoco va en la consola.'

echo

# ---- 7) la deuda pagada avisa y no deja el chequeo en rojo -------------------------------------------
# Se simula el arreglo SIN borrar la línea de la deuda (el caso real: uno arregla y se olvida de borrarla). En la
# copia, el `ReadFile` del audio pasa al camino con dueño (`ReadFileInto`). Tiene que dar VERDE y AVISAR que esa
# entrada ya no aparece en el árbol. Al revés también es cierto y es a propósito: dejar la violación y borrar la
# línea da ROJO, porque la deuda declara lo que EXISTE, no es un permiso.
restaurar
sed -i 's/p\.mpq\.ReadFile(clave)/p.mpq.ReadFileInto(clave, nil, nil)/' "$DEST/d2core/d2audio/aica/proveedor.go"
correr
codigo=$?
CASOS=$((CASOS + 1))
if [ $codigo -eq 0 ] && grep -q '^AVISO:' "$SALIDA"; then
    BIEN=$((BIEN + 1))
    printf '   OK     %-46s esperado %-5s dio %-5s\n' "DEUDA PAGADA avisa y no rompe" "verde" "verde"
    grep -A2 '^AVISO:' "$SALIDA" | sed 's/^/            /'
else
    MAL="$MAL DEUDA-PAGADA"
    printf '   FALLO  %-46s esperado %-5s dio %s\n' "DEUDA PAGADA avisa y no rompe" "verde+aviso" "$codigo"
    sed -n '1,30p' "$SALIDA" | sed 's/^/            /'
fi

echo
echo "== casos: $CASOS   con el resultado esperado: $BIEN =="
if [ -n "$MAL" ]; then
    echo "ROJO: la regla NO caza (o caza de más) en:$MAL"
    exit 1
fi
echo "VERDE: la regla caza las cuatro formas de leer un asset por fuera de la Source, deja pasar el camino"
echo "       con dueño y la prosa, y la deuda se puede pagar sin dejar el chequeo en rojo."
exit 0
