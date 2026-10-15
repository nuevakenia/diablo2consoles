#!/bin/bash
# Genera la tabla de casos del autotest de la consola.
#
# La idea: en la Dreamcast no esta la stdlib de Go, asi que no hay contra que comparar. Las
# respuestas esperadas las calcula la biblioteca estandar REAL aca en el host y quedan horneadas
# en el archivo que despues compila el POC de la consola (console/dc/poc/stdlib).
#
# Uso:  scripts/console/gen-std-casos.sh <dir_de_build>
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STD="${STD:-$HERE/../../console/dc/std}"
OUT="${1:?uso: gen-std-casos.sh <dir_de_build>}"

mkdir -p "$OUT/gen"
(cd "$STD" && go run ./cmd/gen-casos) > "$OUT/gen/casos_gen.go"
echo "casos generados: $OUT/gen/casos_gen.go ($(stat -c%s "$OUT/gen/casos_gen.go") bytes)"
