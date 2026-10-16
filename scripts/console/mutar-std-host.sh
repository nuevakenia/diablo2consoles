#!/bin/bash
# Mutador de la prueba diferencial de la stdlib minima (console/dc/std).
#
# Una suite verde no prueba nada si no caza sus propias regresiones: este script mete a mano cada
# defecto que ya aparecio una vez (y que el corpus del host no cubria) y exige que la suite se ponga
# ROJA. Si un mutante pasa desapercibido, el hueco sigue abierto.
#
# Uso:  scripts/console/mutar-std-host.sh
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
STD="${STD:-$REPO/console/dc/std}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

rc=0
contador=0
cazados=0

# El respaldo es una copia propia y no `git checkout --` a proposito: en medio del hito hay
# arreglos SIN COMMITEAR en los mismos archivos, y un checkout los borraria. La restauracion se
# verifica con sha256 al final de cada mutante (el arbol no puede quedar mutado). Corre un solo
# agente a la vez sobre este repo, asi que no hay dos mutadores pisandose.
aplicar() { # <archivo> <buscar> <poner>
    local arch="$1" buscar="$2" poner="$3"
    cp "$STD/$arch" "$TMP/orig"
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
    return 0
}

mutacion() { # <nombre> <archivo> <buscar> <poner> <goarch>
    local nombre="$1" arch="$2" buscar="$3" poner="$4" arch_go="$5"
    contador=$((contador + 1))
    echo "== mutante $contador: $nombre  (GOARCH=$arch_go)"
    if ! aplicar "$arch" "$buscar" "$poner"; then
        echo "   NO SE APLICO LA MUTACION (patron sin match unico): el mutante no prueba nada"
        restaurar "$arch"
        rc=1
        return
    fi
    if (cd "$STD" && GOARCH="$arch_go" go test ./difftest/ >"$TMP/salida" 2>&1); then
        echo "   LA SUITE NO LO CAZO: paso igual con el mutante puesto"
        rc=1
    else
        echo "   cazado: la suite se puso rojo con el aserto:"
        grep -m1 -- "--- FAIL" "$TMP/salida" | sed 's/^/     /'
        grep -m1 -A1 -- "difftest_test.go:" "$TMP/salida" | tail -1 | cut -c1-150 | sed 's/^/     /'
        cazados=$((cazados + 1))
    fi
    restaurar "$arch" || rc=1
    echo
}

echo "== control sano: la suite tiene que estar VERDE antes de mutar nada =="
for arch in amd64 386; do
    if (cd "$STD" && GOARCH="$arch" go test ./difftest/ >"$TMP/sano-$arch" 2>&1); then
        echo "   GOARCH=$arch verde"
    else
        echo "   GOARCH=$arch ROJO sin mutar nada: arreglar eso primero (no se puede medir el mutador)"
        sed -n '1,10p' "$TMP/sano-$arch" | sed 's/^/     /'
        rc=1
    fi
done
if [ "$rc" -ne 0 ]; then
    echo "== el control no esta verde: se corta aca =="
    exit 1
fi
echo

echo "== 1. la saturacion al desbordar tiene que salir del ancho pedido, no de 64 bits =="
mutacion "strconv: al desbordar devuelve los topes de 64 bits" \
    "strconv/strconv.go" \
    '		if n > (lim-v)/uint64(baseEf) {
			// al desbordar, Go satura al tope DEL ANCHO pedido (con su signo), no al de 64 bits
			return maxInt64For(neg, bitSize), rangeError(fn, s)
		}' \
    '		if n > (lim-v)/uint64(baseEf) {
			if neg {
				return minInt64, rangeError(fn, s)
			}
			return maxInt64, rangeError(fn, s)
		}' \
    amd64

echo "== 2. bitSize 0 tiene que ser el ancho de int de la plataforma (esto solo lo ve el int de 32) =="
mutacion "strconv: bitSize 0 tratado siempre como 64 bits" \
    "strconv/strconv.go" \
    '	if bitSize == 0 {
		bitSize = intSize
	}
	lim := maxForBitSize(neg, bitSize)' \
    '	if bitSize == 0 {
		bitSize = 64
	}
	lim := maxForBitSize(neg, bitSize)' \
    386

echo "== 3. %c de un valor que no es un caracter valido tiene que dar U+FFFD =="
mutacion "fmt: %c confia en la conversion rune -> texto" \
    "fmt/fmt.go" \
    '			r := rune(n)
			if uint64(n) > utf8.MaxRune {
				r = utf8.RuneError
			}
			return string(r), false' \
    '			return string(rune(n)), false' \
    amd64

echo "== 4. un verbo que no aplica al elemento de un arreglo recorre el arreglo =="
mutacion "fmt: el aviso del verbo malo se arma para el arreglo entero" \
    "fmt/fmt.go" \
    '	if s, ok := f.listaDeElementos(arg, verb, fl, f.w, prec); ok {
		f.armado = true // el ancho ya se aplico a cada elemento
		return s, false
	}' \
    '	// mutante: la lista de elementos, desactivada' \
    amd64

echo "== 5. el corpus de la consola tiene que declarar la semantica de int de la consola (32) =="
contador=$((contador + 1))
if (cd "$STD" && go run ./cmd/gen-casos) > "$TMP/casos.go" 2>"$TMP/gen.err"; then
    if grep -q '{"9223372036854775807", 2147483647, true}' "$TMP/casos.go"; then
        echo "   bien: Atoi del tope de int64 sale saturado a MaxInt32 con error (semantica de la consola)"
        cazados=$((cazados + 1))
    else
        echo "   MAL: el corpus no declara el tope de Atoi como lo hace un int de 32 bits:"
        grep '"9223372036854775807"' "$TMP/casos.go" | sed 's/^/     /'
        rc=1
    fi
else
    echo "   MAL: el generador de casos no corre"
    sed -n '1,5p' "$TMP/gen.err" | sed 's/^/     /'
    rc=1
fi

echo "== 6. math: los ceros llevan el signo del argumento (Floor(-0.5) es -0, no +0) =="
mutacion "math: Trunc devuelve float64(i) y pierde el signo del cero" \
    "math/math.go" \
    '	i, ok := truncAInt(x)
	if !ok {
		return x
	}
	return Copysign(float64(i), x) // el cero con el signo del argumento (ver Floor)' \
    '	i, ok := truncAInt(x)
	if !ok {
		return x
	}
	return float64(i)' \
    amd64

echo "== 7. math: MaxFloat64 tiene que ser la constante hexadecimal, no el decimal =="
mutacion "math: MaxFloat64 escrito como literal decimal" \
    "math/math.go" \
    '	MaxFloat64 = 0x1.fffffffffffffp1023' \
    '	MaxFloat64 = 1.7976931348623157e308' \
    amd64

echo "== 8. math: el cuadrante del coseno no se puede equivocar =="
mutacion "math: Cos con los cuadrantes 1 y 3 intercambiados" \
    "math/transcend.go" \
    '	case 1:
		return -sinPoly(r)
	case 2:
		return -cosPoly(r)
	}
	return sinPoly(r)
}' \
    '	case 1:
		return sinPoly(r)
	case 2:
		return -cosPoly(r)
	}
	return -sinPoly(r)
}' \
    amd64

echo "== 9. math: la serie del logaritmo suma el termino 1 =="
mutacion "math: Log sin el termino constante de la serie" \
    "math/transcend.go" \
    '	for j := 12; j >= 0; j-- {' \
    '	for j := 12; j >= 1; j-- {' \
    amd64

echo "== 10. hash/fnv: el primo es parte del algoritmo =="
mutacion "hash/fnv: primo de 32 bits equivocado" \
    "hash/fnv/fnv.go" \
    '	prime32  = 16777619' \
    '	prime32  = 16777617' \
    amd64

echo "== 11. encoding/binary: el orden de bytes no se puede cambiar =="
mutacion "encoding/binary: Uint32 leido como big endian" \
    "encoding/binary/binary.go" \
    '	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24' \
    '	return uint32(b[3]) | uint32(b[2])<<8 | uint32(b[1])<<16 | uint32(b[0])<<24' \
    386


echo "== 12. sort: el ancho del elemento sale del descriptor del tipo =="
mutacion "sort: no se usa el descriptor del elemento" \
    "sort/sort.go" \
    '	if elemPtr {' \
    '	if false {' \
    amd64

echo "== 13. sort: SliceStable tiene que ser estable =="
mutacion "sort: SliceStable usa el quicksort" \
    "sort/sort.go" \
    '	stable(n, less, swapDe(base, ancho))' \
    '	quickSort(0, n, less, swapDe(base, ancho), pdqDepth(n))' \
    amd64

echo "== 14. strconv: la forma g pasa a cientifica con el umbral de Go (6, no 21) =="
mutacion "strconv: FormatFloat g con umbral 21" \
    "strconv/strconv.go" \
    '			return shortest(f, 6, upper)' \
    '			return shortest(f, 21, upper)' \
    amd64

echo "== 15. strconv: la mantisa cientifica arranca en el primer digito significativo =="
mutacion "strconv: mantisa con ceros de relleno (0.00001e-05)" \
    "strconv/strconv.go" \
    '	digits = strings.TrimLeft(digits, "0") // los ceros delanteros no son digitos significativos
' \
    '' \
    amd64

echo
echo "== $cazados/$contador chequeos cazados =="
if [ "$rc" -eq 0 ]; then
    echo "== OK: la suite caza todos los defectos conocidos y el arbol quedo sano =="
else
    echo "== HAY HUECOS (arriba) =="
fi
exit "$rc"
