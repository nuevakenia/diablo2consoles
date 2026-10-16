#!/bin/bash
# Mutadores de DC-002.7: los candados, los atomicos y el runtime de la consola.
#
# El metodo: un mutante es un cambio chico que ROMPE la regla que la prueba tiene que defender. Si el
# POC de goroutines (console/dc/poc/goroutines) sigue VERDE con el mutante puesto, el hueco es de la
# prueba y hay que agregar el caso que falta. El control sano va primero: si con el codigo sin tocar no
# da verde, se corta (no se mide nada).
#
# Cada mutante se aplica sobre el disco, se compila la stdlib, se compila el POC, se corre Flycast y se
# mira la consola serie. Al final se restauran los archivos y se verifica su sha256.
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.." || exit 1
source scripts/console/env.sh

STD="console/dc/std"
POC="console/dc/poc/goroutines"
OUT=/tmp/mutar-goro
mkdir -p "$OUT"
LOG="$OUT/flycast.log"

ARCHIVOS=("$STD/sync/sync.go" "$STD/sync/atomic/atomic.go" "$STD/runtime/runtime.go" "$STD/csrc/atomic_dc.c")
for f in "${ARCHIVOS[@]}"; do
    sha256sum "$f" | cut -d' ' -f1 > "$OUT/sha-$(echo "$f" | tr / -).orig"
done

restaurar() {
    for f in "${ARCHIVOS[@]}"; do
        cp "$OUT/resp-$(basename "$f")" "$f" 2>/dev/null || true
    done
}
trap restaurar EXIT

# correr: compila y corre el POC; deja la consola serie en $LOG y devuelve 0 si la corrida llego al final
correr() {
    if ! bash scripts/console/build-std-gox.sh >/dev/null 2>&1; then
        echo "  (la stdlib no compila: cuenta como cazado)"
        return 1
    fi
    if ! bash scripts/console/build-dc.sh "$POC" >/dev/null 2>&1; then
        echo "  (el POC no compila: cuenta como cazado)"
        return 1
    fi
    # 45 s de tope: si un mutante deja un candado trabado, la consola no llega al final y eso es ROJO
    timeout 120 bash scripts/console/run-flycast.sh /tmp/dcbuild/goroutines/prog.elf 45 "$OUT" >/dev/null 2>&1
    tr -d '\r' < "$LOG" > "$LOG.limpio" 2>/dev/null
    if grep -q 'GORO RESULTADO OK' "$LOG.limpio"; then
        return 0
    fi
    return 1
}

echo "== control: sin mutar tiene que salir VERDE =="
if ! correr; then
    echo "CONTROL MAL: con el codigo sin tocar el POC no da verde. Se corta."
    exit 1
fi
echo "  control OK"

total=0
cazados=0

# probar: $1 nombre, $2 archivo, $3 buscar (python), $4 poner (python), $5 que tiene que romper
probar() {
    total=$((total + 1))
    local nombre="$1" arch="$2" buscar="$3" poner="$4" que="$5"
    echo
    echo "== mutante $total: $nombre ($que) =="
    cp "$arch" "$OUT/resp-$(basename "$arch")"
    if ! python3 - "$arch" "$buscar" "$poner" <<'PY'
import sys
p, buscar, poner = sys.argv[1], sys.argv[2], sys.argv[3]
s = open(p, encoding="utf-8").read()
if buscar not in s:
    print("NO ENCONTRE el texto a mutar")
    sys.exit(1)
open(p, "w", encoding="utf-8").write(s.replace(buscar, poner, 1))
PY
    then
        echo "  no se pudo aplicar el mutante (revisar el patron)"
        cp "$OUT/resp-$(basename "$arch")" "$arch"
        return
    fi
    if correr; then
        echo "  VERDE: el mutante NO fue cazado. Hueco del POC."
    else
        echo "  ROJO: cazado."
        cazados=$((cazados + 1))
    fi
    cp "$OUT/resp-$(basename "$arch")" "$arch"
}

# 1. sync: si Unlock no suelta el candado, el contador protegido no puede dar exacto (y el POC se traba)
probar "sync: Unlock que no suelta el candado" \
    "$STD/sync/sync.go" \
    '	if m.libre == nil {
		return
	}
	<-m.libre' \
    '	if m.libre == nil {
		return
	}' \
    "el contador con Mutex da 800 exacto"

# 2. sync: si WaitGroup.Done no cierra el canal, el Wait se queda esperando para siempre
probar "sync: WaitGroup.Done que no despierta al que espera" \
    "$STD/sync/sync.go" \
    '	if wg.n == 0 && wg.aviso != nil {
		close(wg.aviso)
		wg.aviso = nil
	}' \
    '	if wg.n == 0 && wg.aviso != nil {
		_ = wg.aviso
	}' \
    "WaitGroup.Wait espera a las cuatro"

# 3. sync/atomic: si AddInt64 devuelve el valor viejo y no suma, el contador no llega a 800
probar "sync/atomic: AddInt64 que no suma" \
    "$STD/csrc/atomic_dc.c" \
    'int64_t d2c_at_add64(int64_t *p, int64_t d) {
    int irq = irq_disable();
    int64_t nuevo = *p + d;
    *p = nuevo;
    irq_restore(irq);
    return nuevo;
}' \
    'int64_t d2c_at_add64(int64_t *p, int64_t d) {
    int irq = irq_disable();
    int64_t viejo = *p;
    irq_restore(irq);
    (void)d;
    return viejo;
}' \
    "atomic.AddInt64 da 800 exacto"

# 4. sync/atomic: si CompareAndSwap cambia aunque no coincida, el caso del CAS que falla se cae
probar "sync/atomic: CAS que cambia sin coincidir" \
    "$STD/csrc/atomic_dc.c" \
    '    if (*p == *esperado) {
        *p = nuevo;
        ok = 1;
    } else {
        *esperado = *p;
    }
    irq_restore(irq);
    return ok;
}

/* ------------------------------------------------ 64 bits' \
    '    *p = nuevo;
    ok = 1;
    irq_restore(irq);
    return ok;
}

/* ------------------------------------------------ 64 bits' \
    "el CAS que no coincide deja el valor"

# 5. runtime: si GOOS dice otra cosa, el caso del runtime lo caza
probar "runtime: GOOS que no es dreamcast" \
    "$STD/runtime/runtime.go" \
    '	GOOS   = "dreamcast"' \
    '	GOOS   = "linux"' \
    "runtime.GOOS = dreamcast"

echo
echo "== restauro y verifico =="
for f in "${ARCHIVOS[@]}"; do
    sha="$(sha256sum "$f" | cut -d' ' -f1)"
    guardado="$OUT/sha-$(echo "$f" | tr / -).orig"
    if [ "$sha" != "$(cat "$guardado")" ]; then
        echo "MAL: $f no quedo igual (sha256 $sha, era $(cat "$guardado"))"
        exit 1
    fi
    echo "$f restaurado, sha256 $sha"
done
echo
echo "== mutantes cazados $cazados/$total =="
[ "$cazados" -eq "$total" ] || exit 1
