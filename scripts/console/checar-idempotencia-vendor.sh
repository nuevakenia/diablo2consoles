#!/bin/bash
# DC-002.25: LA IDEMPOTENCIA DE LA CADENA DEL VENDOR (medida, con su mutante).
#
# EL DEFECTO (encontrado re-midiendo la escalera del semiespacio en DC-002.25)
#   La regla del proyecto es que los archivos de ~/dreamcast/libgodc (vendorizado, fuera del repo) se tocan
#   con scripts IDEMPOTENTES, y `fix-libgodc-gc.py` lo promete por escrito ("correrlo dos veces deja el
#   mismo sha256"). No era cierto: `reemplazar()` de `fix-libgodc-sq.py` miraba PRIMERO el texto viejo, y
#   `VIEJO_FIN` (el `if (remainder > 0) memcpy(...)`) es una SUBCADENA de `NUEVO_FIN`, asi que despues de
#   aplicar el arreglo el conteo del texto viejo seguia dando 1 (el de adentro del nuevo) y la cadena
#   insertaba OTRA invalidacion de cache en cada corrida. Medido (gc_copy.c por corrida de la cadena, 512
#   KB): 46699 -> 47064 bytes y 8 -> 9 invalidaciones; con eso el `arena` del arranque se movia 32 bytes
#   entre dos builds identicos con el MISMO KB (688700 -> 688668), o sea: los numeros de la tarjeta no eran
#   reproducibles.
#
# EL ARREGLO (scripts/console/fix-libgodc-sq.py)
#   `reemplazar()` mira primero el RESULTADO: si el texto nuevo ya esta, la cadena ya paso por aca y no toca
#   nada. Medido: dos corridas seguidas dejan gc_copy.c con el MISMO sha256 y el MISMO tamano.
#
# ESTE CORPUS (4 corridas de la cadena)
#   0. el reset pristino: gc_copy.c se saca del vendor con `git show HEAD:runtime/gc_copy.c` -- una LECTURA
#      del repo vendorizado, no un `git checkout` -- asi el corpus arranca siempre del mismo punto
#   1. control 1: dos corridas de la cadena -> gc_copy.c identico (sha256, bytes y cuenta de invalidaciones)
#   2. mutante: se le devuelve el ORDEN VIEJO al guard -> la corrida tiene que HACER CRECER el archivo
#      (si no crece, este corpus no vigila nada y sale ROJO)
#   3. control 2 (de vuelta): se restaura el guard Y el archivo del estado sano (bit a bit) -> la cadena
#      vuelve a dejar exactamente el mismo sha256 y tamano del control 1
#
# Uso:  bash scripts/console/checar-idempotencia-vendor.sh
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

SQ="$HERE/fix-libgodc-sq.py"
GC="$HERE/fix-libgodc-gc.py"
C="$LIBGODC/runtime/gc_copy.c"
LOG=/tmp/dc225-idem
BK_SQ=/tmp/dc225-idem-fix-libgodc-sq.py
BK_C=/tmp/dc225-idem-gc_copy.c
CAZADOS=0
TOTAL_MUT=1
BIEN=0
TOTAL_CTRL=2

datos() { printf '%s %s %s\n' "$(sha256sum "$C" | cut -c1-16)" "$(wc -c < "$C")" "$(grep -c 'dcache_inval_range' "$C")"; }
cadena() { python3 "$GC" 512 --build > "$LOG-cadena.txt" 2>&1 || { echo "la cadena fallo (ver $LOG-cadena.txt)" >&2; exit 1; }; }

echo "== 0. el reset pristino (una LECTURA del vendor, no un git checkout)"
git -C "$LIBGODC" show HEAD:runtime/gc_copy.c > "$C" || { echo "no pude sacar el gc_copy.c pristino" >&2; exit 1; }
read -r s0 b0 i0 <<<"$(datos)"
echo "   pristino: sha $s0  $b0 bytes  invalidaciones $i0"

echo
echo "== 1. control 1: dos corridas de la cadena dejan el mismo gc_copy.c"
cadena
read -r s1 b1 i1 <<<"$(datos)"
echo "   corrida 1: sha $s1  $b1 bytes  invalidaciones $i1"
cp "$C" "$BK_C"
cadena
read -r s2 b2 i2 <<<"$(datos)"
echo "   corrida 2: sha $s2  $b2 bytes  invalidaciones $i2"
if [ "$s1" = "$s2" ] && [ "$b1" = "$b2" ]; then
    echo "   control 1 (la cadena es idempotente):        VERDE"
    BIEN=$((BIEN + 1))
else
    echo "   control 1 (la cadena es idempotente):        ROJO ($b1 -> $b2 bytes)"
fi
echo "   (libgodc.a: $(wc -c < "$LIBGODC/libgodc.a") bytes -- su sha256 NO es reproducible porque 'ar'"
echo "    guarda el mtime de cada objeto; lo que se compara es el TAMANO y la fuente)"

echo
echo "== 2. mutante: se le devuelve el ORDEN VIEJO al guard de reemplazar()"
cp "$SQ" "$BK_SQ"
python3 - "$SQ" <<'PY' || exit 1
import pathlib, sys
p = pathlib.Path(sys.argv[1])
t = p.read_text()
viejo = '''    if nuevo in t:
        print("  ya estaba %s" % path.name)
        return
    n = t.count(viejo)
'''
nuevo = '''    n = t.count(viejo)
    if n == 0 and nuevo in t:
        print("  ya estaba %s" % path.name)
        return
'''
if t.count(viejo) != 1:
    print("no encontre el guard nuevo (%d veces)" % t.count(viejo))
    raise SystemExit(1)
p.write_text(t.replace(viejo, nuevo, 1))
print("   mutante aplicado: se mira el texto viejo primero")
PY
cadena
read -r s3 b3 i3 <<<"$(datos)"
echo "   corrida con el orden viejo: sha $s3  $b3 bytes  invalidaciones $i3"
if [ "$b3" -gt "$b2" ]; then
    echo "   CAZADO: con el orden viejo el archivo CRECE ($b2 -> $b3 bytes, invalidaciones $i2 -> $i3)"
    echo "   y por eso los numeros del arranque se movian entre builds identicos: el defecto es real"
    CAZADOS=1
else
    echo "   NO LO CAZO: con el orden viejo el archivo NO cambio ($b2 -> $b3): revisar el corpus" >&2
fi

echo
echo "== 3. control 2 (de vuelta): se restaura el guard y el archivo sano, y la cadena vuelve al mismo sha"
cp "$BK_SQ" "$SQ"
cp "$BK_C" "$C"
cmp -s "$BK_SQ" "$SQ" && echo "   guard restaurado: OK" || echo "   guard restaurado: DISTINTO" >&2
read -r sr br ir <<<"$(datos)"
echo "   gc_copy.c restaurado del estado sano: sha $sr  $br bytes"
cadena
read -r s4 b4 i4 <<<"$(datos)"
echo "   la cadena de vuelta: sha $s4  $b4 bytes  invalidaciones $i4"
if [ "$s4" = "$s1" ] && [ "$b4" = "$b1" ]; then
    echo "   control 2 (la cadena vuelve al mismo estado): VERDE"
    BIEN=$((BIEN + 1))
else
    echo "   control 2 (la cadena vuelve al mismo estado): ROJO ($s1 -> $s4)"
fi
git -C "$LIBGODC" diff --stat -- runtime/gc_copy.c | sed 's/^/   /'

echo
echo "== $CAZADOS/$TOTAL_MUT mutantes cazados, $BIEN/$TOTAL_CTRL controles verdes =="
[ "$CAZADOS" -eq "$TOTAL_MUT" ] && [ "$BIEN" -eq "$TOTAL_CTRL" ]
