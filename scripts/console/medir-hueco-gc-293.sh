#!/bin/bash
# DC-002.93 -- EL HUECO DE `gc_init`, MEDIDO: lo que pide el GC, lo que hay, y donde esta la pared.
#
# POR QUE EXISTE. DC-002.87c midio sobre el binario del motor de DC-002.28 (`0a9c182d`, 13.775.848 B):
#
#     Out of memory. Requested sbrk_base 0x8d182000, was 0x8cf82000, diff 2097152
#     fatal error: gc_init: alloc failed
#
# 2 MB pedidos para el SEGUNDO semiespacio con 450.560 B de hueco. Ese numero quedo escrito como si fuera
# del motor de HOY, y no lo es: el mismo motor rearmado con el arbol de hoy mide 11.650.536 B (-2.125.312 B
# del recorte de DC-002.68 en adelante), pasa los DOS semiespacios y muere 50 s despues, en otro lado. Un
# numero de hace seis tarjetas no sirve para decidir: este script mide el hueco del binario que se le pasa.
#
# LO QUE MIDE, EN ORDEN (el "paso a paso" de la tarjeta):
#   1. la imagen (1ST_READ.BIN: los bytes que el cargador pone en RAM) y su techo,
#   2. donde cayeron los dos semiespacios del GC (`gc_init-semiespacio0/1`, direccion y separacion),
#   3. lo que `gc_init` pide de una vez: 2 x semiespacio (runtime/gc_heap.c),
#   4. el monton de C que queda despues de los dos: techo - (semiespacio1 + semiespacio),
#   5. donde murio la corrida y con cuanto margen (`Out of memory` / `fatal error` / `Memory:`).
#
# USO
#   bash scripts/console/medir-hueco-gc-293.sh leer <log-de-flycast> [bytes-del-1ST_READ]
#       Solo lee una corrida ya hecha. No toca nada.
#
#   bash scripts/console/medir-hueco-gc-293.sh escalera <dir_build_arranque> [KB ...]
#       Para cada semiespacio: rehace libgodc.a con ESE valor, reenlaza el mismo build (build-motor-arranque)
#       y corre 60 s en Flycast, y da el veredicto con los numeros de arriba. Por defecto: 2048 1536 1024 768.
#       Pide el arbol de .gox (BASE_GOX, por defecto /tmp/gox-293 -- UN arbol PROPIO: build-tablas-282.sh
#       recompila 19 paquetes A TRAVES de los enlaces de su gox-enlace, asi que si comparte el arbol con este
#       escalera, el d2records.gox del motor queda con la tabla de lookups adentro y el binario sale 1,67 MB
#       mas grande -- medido, costo una medicion en DC-002.93), el POC (POC=motor-disco) y los MPQ del
#       juego en $HOME/.wine-d2/drive_c/Games/Diablo II. Las corridas van a /tmp/dc293/escalera-293.
#
# EL TECHO es 0x8cff0000 (fin de los 16 MB que ve el programa; lo imprime el runtime como `techo=`). Si el
# log trae su propia linea `Memory: arena=... used=... free=...`, el techo de ESA corrida se deriva de ahi.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"

MODO="${1:-leer}"

# el paso a paso, en python: la aritmetica hexadecimal no se hace de cabeza
informe() {
    local log="$1" bin="${2:-0}"
    BIN="$bin" python3 - "$log" <<'PY'
import os, re, sys

log = sys.argv[1]
bin_bytes = int(os.environ.get("BIN", "0") or 0)
texto = open(log, "rb").read().decode("utf-8", "replace").replace("\r", "")

TECHO = 0x8CFF0000          # fin de los 16 MB de RAM del programa (el `techo=` del runtime)
BASE_IMG = 0x8C010000       # donde arranca la imagen

def num(pat, cast=int):
    m = re.findall(pat, texto)
    return cast(m[-1]) if m else None

sb0 = num(r"gc_init-semiespacio0 v=(\d+)")
sb1 = num(r"gc_init-semiespacio1 v=(\d+)")
dur = sb1 - sb0 if (sb0 and sb1) else None
mem = re.findall(r"Memory: arena=(\d+) used=(\d+) free=(\d+)", texto)
oom = re.findall(r"Out of memory\. Requested sbrk_base (0x[0-9a-f]+), was (0x[0-9a-f]+), diff (\d+)", texto)
fat = re.findall(r"fatal error: ([^\n]*)", texto)
exc = "SH4 exception" in texto
arranques = len(re.findall(r"REIOS: Booting up", texto))

print("== imagen")
if bin_bytes:
    print("   1ST_READ.BIN          %d B" % bin_bytes)
    print("   fin de la imagen      0x%08X  (0x%08X + %d)" % (BASE_IMG + bin_bytes, BASE_IMG, bin_bytes))
else:
    print("   1ST_READ.BIN          (no informado: pasalo como 2do argumento)")
print("   techo de los 16 MB    0x%08X" % TECHO)

print("== el GC")
if sb0 and sb1:
    print("   semiespacio 0         0x%08X" % sb0)
    print("   semiespacio 1         0x%08X" % sb1)
    print("   separacion            %d B (%.0f KB)" % (dur, dur / 1024.0))
    print("   lo que pide gc_init   2 x %d = %d B  (de UNA vez, gc_heap.c)" % (dur, 2 * dur))
    fin_gc = sb1 + dur
    print("   monton de C despues   0x%08X -> 0x%08X = %d B libres para TODO lo demas" % (fin_gc, TECHO, TECHO - fin_gc))
else:
    print("   el log no trae las dos marcas gc_init-semiespacio0/1")

print("== donde murio")
if oom:
    pedido, habia, diff = oom[-1]
    libre = TECHO - int(habia, 16)
    print("   Out of memory         pidio %s, habia %s (diff %s B); al techo le quedaban %d B"
          % (pedido, habia, diff, libre))
if fat:
    print("   fatal error           %s" % fat[-1].strip())
if mem:
    a, u, f = mem[-1]
    print("   Memory                arena=%s used=%s free=%s" % (a, u, f))
print("   arranques de consola  %d%s" % (arranques, "  <- el programa REINICIO" if arranques > 1 else ""))
if exc:
    print("   el emulador se detuvo con SH4 exception")
PY
}

case "$MODO" in
leer)
    LOG="${2:?uso: medir-hueco-gc-293.sh leer <log-de-flycast> [bytes-del-1ST_READ]}"
    [ -f "$LOG" ] || { echo "no existe $LOG" >&2; exit 1; }
    informe "$LOG" "${3:-0}"
    ;;
escalera)
    DIR="${2:?uso: medir-hueco-gc-293.sh escalera <dir_build_arranque> [KB ...]}"
    shift 2
    KBS="${*:-2048 1536 1024 768}"
    BASE_GOX="${BASE_GOX:-/tmp/gox-293}"
    POC="${POC:-motor-disco}"
    SEGS="${SEGS:-60}"
    SAL="${SAL:-/tmp/dc293/escalera-293}"
    mkdir -p "$SAL"
    printf 'KB\t1ST_READ\tpedido\tmontonC\tdesenlace\n' > "$SAL/tabla.tsv"
    for KB in $KBS; do
        OUT="$SAL/kb$KB"
        mkdir -p "$OUT"
        echo "== semiespacio $KB KB"
        python3 "$HERE/fix-libgodc-gc.py" "$KB" --build > "$OUT/vendor.log" 2>&1 \
            || { echo "   el runtime NO se rehizo (ver $OUT/vendor.log)" >&2; exit 1; }
        env MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO=patch_d2.mpq,d2data.mpq,d2char.mpq \
            DC_SIN_SELLOS=1 DC_DISCO_JUEGO="$HOME/.wine-d2/drive_c/Games/Diablo II" POC="$POC" \
            bash "$HERE/build-motor-arranque.sh" "$DIR" "$BASE_GOX" > "$OUT/build.log" 2>&1 \
            || { echo "   el build NO salio (ver $OUT/build.log)" >&2; exit 1; }
        BIN="$(stat -c%s "$DIR/1ST_READ.BIN")"
        rm -rf "$OUT/run"
        CAPTURE=none bash "$HERE/run-flycast.sh" "$DIR/disc.gdi" "$SEGS" "$OUT/run" > "$OUT/run.out" 2>&1
        informe "$OUT/run/flycast.log" "$BIN" | tee "$OUT/informe.txt" | sed 's/^/   /'
        # la fila de la tabla: el numero que decide, no el relato
        PEDIDO=$((2 * KB * 1024))
        MONTON=$(sed -n 's/.*monton de C despues   0x[0-9A-F]* -> 0x[0-9A-F]* = \([0-9]*\) B.*/\1/p' "$OUT/informe.txt")
        if grep -qa "gc_init: alloc failed" "$OUT/run/flycast.log"; then DES="muere en gc_init"
        elif grep -qa "gc_external_alloc" "$OUT/run/flycast.log"; then DES="pasa gc_init, muere en gc_external_alloc"
        elif grep -qa "fatal error: out of memory" "$OUT/run/flycast.log"; then DES="pasa gc_init, muere en gc_alloc (semiespacio chico)"
        elif grep -qa "SH4 exception" "$OUT/run/flycast.log"; then DES="pasa gc_init, muere por excepcion"
        else DES="vive los $SEGS s"; fi
        printf '%s\t%s\t%s\t%s\t%s\n' "$KB" "$BIN" "$PEDIDO" "${MONTON:-?}" "$DES" >> "$SAL/tabla.tsv"
    done
    echo
    echo "== TABLA ($SAL/tabla.tsv)"
    column -t -s "$(printf '\t')" "$SAL/tabla.tsv" 2>/dev/null || cat "$SAL/tabla.tsv"
    ;;
*)
    echo "uso: $0 leer <log> [bytes] | $0 escalera <dir_build> [KB ...]" >&2
    exit 1
    ;;
esac
