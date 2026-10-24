#!/bin/bash
# Mutador de la regla de codegen de DC-002.52 (`checar-codegen-252.py`).
#
# Una regla que solo "da verde" no prueba nada: este script mete en una COPIA del arbol los dos patrones que
# ya rompieron el arranque de la consola y exige que el verificador se ponga ROJO. Si un mutante pasa
# desapercibido, la regla tiene un hueco.
#
# Nunca toca el arbol: trabaja sobre $TMP/console/dc/poc/... y el verificador se corre con --root.
#
# Uso:  scripts/console/mutar-codegen-252.sh
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
VERIF="$HERE/checar-codegen-252.py"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

verde=0
rojo=0
fallos=0

# el control sano: el arbol del repo tiene que dar verde
if python3 "$VERIF" > "$TMP/sano.txt" 2>&1; then
    echo "  [1/3] arbol sano: VERDE  ($(grep -o '\.go mirados: [0-9]*' "$TMP/sano.txt"))"
    verde=$((verde + 1))
else
    echo "  [1/3] arbol sano: ROJO -- el control falla sobre el arbol limpio"
    sed 's/^/        /' "$TMP/sano.txt" | tail -5
    fallos=$((fallos + 1))
fi

# la copia donde se inyectan los mutantes
mkdir -p "$TMP/console/dc/poc/prueba"
cat > "$TMP/console/dc/poc/prueba/limpio.go" <<'EOF'
package prueba

type modo struct {
	nombre string
	ms     int
}

func modos() []modo {
	return []modo{{"quieto", 128}, {"camina", 40}}
}
EOF

mutar() {
    local nombre="$1" archivo="$2" contenido="$3"
    printf '%s\n' "$contenido" > "$TMP/console/dc/poc/prueba/$archivo"

    if python3 "$VERIF" --root "$TMP/console/dc" > "$TMP/salida.txt" 2>&1; then
        echo "  $nombre: NO CAZADO -- el verificador dio verde con el defecto adentro"
        fallos=$((fallos + 1))
    else
        echo "  $nombre: cazado  ($(grep -m1 'VIOLACIONES:' "$TMP/salida.txt" | sed 's/^ *//'))"
        rojo=$((rojo + 1))
    fi

    rm -f "$TMP/console/dc/poc/prueba/$archivo"
}

# A) el append de un struct ANONIMO -- el que costo ocho intentos en DC-002.50
mutar "  [2/3] mutante A (append de struct anonimo)" mutante_a.go 'package prueba

func conAppend() []struct {
	nombre string
	ms     int
} {
	lista := []struct {
		nombre string
		ms     int
	}{{"quieto", 128}}

	return append(lista, struct {
		nombre string
		ms     int
	}{"camina", 40})
}'

# B) el recorte de una rebanada de struct ANONIMO -- la variante que murio despues del init del PVR
mutar "  [3/3] mutante B (recorte de struct anonimo)" mutante_b.go 'package prueba

func conRecorte(bestia bool) []struct {
	nombre string
	ms     int
} {
	modos := []struct {
		nombre string
		ms     int
	}{{"quieto", 128}, {"camina", 40}}

	if bestia {
		modos = modos[:1]
	}

	return modos
}'

echo
echo "== controles verdes: $verde · mutantes cazados: $rojo/2 · fallos: $fallos =="
if [ "$fallos" -ne 0 ] || [ "$rojo" -ne 2 ]; then
    echo "FALLO: el control no esta vigilando lo que dice vigilar"
    exit 1
fi
echo "OK: el control caza las dos formas del defecto y el arbol queda limpio"
