#!/bin/bash
# DC-002.29: EL INDICE DEL `init` DE PAQUETE QUE MATA EL ARRANQUE REARMADO.
#
# QUE MIDE
#   1. HOST/consola: la camada de REFERENCIA (una que ARRANCA) reenlazada tal cual: sus ~118 `init` de
#      paquete corren TODOS y el arranque llega a `main.main` (control sano del instrumento).
#   2. CONSOLA: la MISMA camada con UN SOLO paquete recompilado (`d2common/d2fileformats/d2mpq`): el
#      instrumento dice el INDICE del `init` que no vuelve (la tarjeta DC-002.29 pide la linea exacta).
#
# COMO (el instrumento ya existe: scripts/console/marcas-init-elf.py)
#   Se parchea el ELF ENLAZADO, sin tocar una linea de Go: cada llamada de `___go_init_main` se repunta a un
#   trozo que imprime su indice (`ARQ-INIT idx = n`) y salta al destino original. El arranque del runtime ya
#   tiene sus marcas (`fix-libgodc-marcas-arranque.py`), asi que en el mismo log se lee hasta donde llego.
#
# EL ELF TIENE QUE ENTRAR EN LOS 16 MB DE FLYCAST (medido: reios_loadElf rechaza >16 MB), asi que despues de
# instrumentar se le saca el debug y el .go_export con el MISMO paso que build-motor-arranque.sh.
#
# Uso: bash scripts/console/medir-init-muerto-229.sh [camada_referencia] [segundos]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REF="${1:-/tmp/gox-240}"
SEGS="${2:-30}"
SAL="${SAL:-$HOME/.hermes/cache/scratch/corpus-229}"
GOX_SICK="$SAL/gox-rearme"
PAQ=d2common/d2fileformats/d2mpq
PREFIJO=github.com/OpenDiablo2/OpenDiablo2

mkdir -p "$SAL"
nota() { echo "   $*"; }

corre() {   # $1 etiqueta  $2 camada
    local nom="$1" gox="$2" out="$SAL/$1"
    echo "== build $nom (camada $gox)"
    POC=motor-disco bash "$HERE/build-motor-arranque.sh" "$out" "$gox" > "$out.build.log" 2>&1
    [ $? -eq 0 ] || { echo "   el POC no compila/enlaza (ver $out.build.log)"; return 1; }

    # instrumento: los indices de `init`
    python3 "$HERE/marcas-init-elf.py" "$out/prog.elf" "$out/prog-instr.elf" > "$out.instr.log" 2>&1
    if [ $? -ne 0 ]; then echo "   el instrumento FALLO:"; sed 's/^/     /' "$out.instr.log" | tail -8; return 1; fi

    sh-elf-objcopy -R .go_export -R .comment --strip-debug "$out/prog-instr.elf" "$out/prog-instr-emu.elf"
    sh-elf-strip -s "$out/prog-instr-emu.elf"
    nota "instrumentado: $(stat -c%s "$out/prog-instr-emu.elf") bytes (Flycast corta en 16 MB)"

    CAPTURE=none bash "$HERE/run-flycast.sh" "$out/prog-instr-emu.elf" "$SEGS" "$out-run" > "$out-run.out" 2>&1

    local T; T=$(tr -d '\r' < "$out-run/flycast.log")
    local INITS ULT
    INITS=$(printf '%s' "$T" | grep -ac 'ARQ-INIT')
    ULT=$(printf '%s' "$T" | grep -a 'ARQ-INIT' | tail -1)
    echo "   (el que anda) marcas: enteros=$(printf '%s' "$T" | grep -ac 'main-entra') main.main=$(printf '%s' "$T" | grep -ac 'main.main-entra') cuadros=$(printf '%s' "$T" | grep -ac 'EBITEN-PERFIL')"
    echo "   $nom: ARQ-INIT vistos = $INITS ; ULTIMO = $ULT"
    printf '%s' "$T" | grep -a 'ARQ-INIT' > "$out.inits.log"
    printf '%s' "$T" > "$out.serie.log"
    return 0
}

echo "== 0. la camada de referencia $REF"
[ -f "$REF/orden.txt" ] && [ -f "$REF/orden-motor.txt" ] || { echo "   falta la camada en $REF"; exit 1; }
nota "orden.txt $(wc -l < "$REF/orden.txt") objetos | orden-motor.txt $(wc -l < "$REF/orden-motor.txt") paquetes"
nota "d2mpq.gox de la referencia: $(stat -c%s "$REF/$PREFIJO/$PAQ.gox") bytes sha $(sha256sum "$REF/$PREFIJO/$PAQ.gox" | cut -c1-16)"

# la camada rearmada: copia de la de referencia + SOLO d2mpq recompilado (las fuentes del arbol de trabajo)
echo "== 1. la camada REARMADA (copia de la referencia con d2mpq recompilado)"
rm -rf "$GOX_SICK"; cp -a "$REF" "$GOX_SICK"
sed -i "s#$REF#$GOX_SICK#g" "$GOX_SICK/orden.txt" "$GOX_SICK/orden-motor.txt"
bash "$HERE/recompilar-paquete-motor.sh" "$PAQ" "$GOX_SICK" > "$SAL/rearme-d2mpq.log" 2>&1 ||
    { echo "   el rearme fallo (ver $SAL/rearme-d2mpq.log)"; tail -5 "$SAL/rearme-d2mpq.log"; exit 1; }
nota "$(grep '^RE-MOTOR' "$SAL/rearme-d2mpq.log")"
nota "sha256 nuevo $(sha256sum "$GOX_SICK/$PREFIJO/$PAQ.gox" | cut -c1-16) contra referencia $(sha256sum "$REF/$PREFIJO/$PAQ.gox" | cut -c1-16)"

# la tabla indice -> paquete (la usa la lectura del resultado)
corre ref "$REF" || exit 1
python3 "$HERE/marcas-init-elf.py" --lista "$SAL/ref/prog.elf" > "$SAL/tabla-inits.txt" 2>&1 || true
corre rearme "$GOX_SICK"

echo
echo "== LA TABLA (indice -> paquete), del binario de referencia:"
if [ -f "$SAL/ref.inits.log" ] && [ -s "$SAL/ref.inits.log" ]; then
    for n in $(grep -a -o 'ARQ-INIT idx = [0-9]*' "$SAL/ref.inits.log" | grep -o '[0-9]*$'); do
        paq=$(awk -v k="$n" '$1==k {print $2}' "$SAL/tabla-inits.txt")
        printf '   %3s  %s\n' "$n" "$paq"
    done | tail -3
fi
echo "== evidencia: $SAL"
