#!/bin/bash
# Compila la biblioteca estandar minima (console/dc/std) para SH-4, en formato .gox.
#
# Los paquetes se importan entre si como "d2std/xxx" para que la prueba diferencial del host
# funcione contra el modulo d2std. En la consola, en cambio, el motor pide "fmt", "strings", etc. a
# secas: si se compilaran las dos formas habria DOS juegos de paquetes distintos y los tipos no
# encajarian entre si (un io.Writer de cada lado no es el mismo tipo). Por eso el build copia las
# fuentes a un directorio de trabajo y reescribe "d2std/" -> "": un solo juego de paquetes, con los
# nombres que el motor espera encontrar.
#
# Uso:  scripts/console/build-std-gox.sh [dir_salida]      (por defecto $DC_ROOT/gox)
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

STD="${STD:-$HERE/../../console/dc/std}"
GEN="${1:-${GODC_GOX:-$DC_ROOT/gox}}"

# orden de dependencias: cada paquete necesita los .gox de los anteriores
# (io va despues de errors porque io.EOF es un errors.New, DC-002.3)
# Orden de dependencias. ojo: heap importa sort, y csv importa encoding.
# flag va despues de io/strconv/strings (importa los tres) y antes de encoding/csv (que no lo usa).
# NOTA (DC-002.5b): hasta DC-002.5b este archivo decia que encoding/csv NO podia entrar al orden
# porque "no compila en SH-4 (non-integer len argument to make en csv.go:344)" y que al entrar "el
# build se cortaba ahi". Las DOS cosas eran falsas cuando se escribieron: medido, csv compila solo
# (95.628 bytes de .gox) y el corte era un problema de orden, arreglado en DC-002.8 (commit
# 4420033d). El comentario quedo viejo y confundia: se corrige aca.
# PENDIENTE QUE YA NO ESTA (DC-002.8a): hasta DC-002.8 este archivo decia que os y io/fs tenian TIPOS
# DISTINTOS para lo mismo (FileInfo, DirEntry, FileMode) y que alinearlos bajaba el motor a 40 de 142.
# Las dos cosas eran ciertas ENTONCES y dejaron de serlo en 95e0ef74 (DC-002.8), que hizo el alias; el
# comentario quedo y confundia, porque el tablero lo leia como trabajo pendiente. Medido en DC-002.8a:
# os y io/fs ya comparten los tipos (os los reusa por alias), el motor esta en 93 de 142 antes y despues
# de los cambios de esta tarjeta, y el que vigila la alineacion es scripts/console/mutar-tipos-os.sh.
# Lo que SI se agrego aca (DC-002.8a): los imports de los paquetes de d2std van como "d2std/..." y no
# con el nombre desnudo; el que lo exige es scripts/console/checar-imports-d2std.sh.
ORDEN="${ORDEN:-unicode unicode/utf8 unicode/utf16 errors io bytes strings strconv fmt math math/bits math/rand math/cmplx hash hash/fnv hash/crc32 crypto/sha256 encoding/json encoding/binary encoding/hex compress/zlib compress/gzip container/ring sort container/heap image/color image image/draw image/png image/gif regexp time io/fs embed os os/user os/exec os/signal syscall log bufio flag encoding/csv io/ioutil path/filepath runtime runtime/pprof sync/atomic sync context net}"

if [ ! -d "$STD/${ORDEN%% *}" ]; then
    echo "no encuentro el primer paquete ($STD/${ORDEN%% *})" >&2
    exit 1
fi

rm -rf "$GEN"
mkdir -p "$GEN/src" "$GEN/d2std"
for pkg in $ORDEN; do
    dst="$GEN/src/$pkg"
    mkdir -p "$dst"
    for f in "$STD/$pkg"/*.go; do
        [ -f "$f" ] || continue
        # los _test.go van SOLO al host (son la prueba contra la biblioteca de verdad): si entran al
        # build de la consola, gccgo intenta compilar un paquete de prueba que importa al de go.
        case "$f" in *_test.go) continue ;; esac
        sed 's|"d2std/|"|g' "$f" > "$dst/$(basename "$f")"
    done
    mkdir -p "$(dirname "$GEN/$pkg")"
    # el archivo .gox que escribe gccgo es a la vez la cabecera de importacion Y el objeto con el
    # codigo (ELF reubicable): sirve para importar y hay que pasar el mismo archivo al enlazador
    sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -fgo-pkgpath="$pkg" -c "$dst"/*.go -o "$GEN/$pkg.gox"
    printf "  %-16s %8s bytes\n" "$pkg.gox" "$(stat -c%s "$GEN/$pkg.gox")"
    echo "$GEN/$pkg.gox" >> "$GEN/orden.txt"
done

echo "stdlib minima compilada para SH-4 en $GEN"

# ============================================================================
# Los paquetes de TERCEROS que el motor importa y que se reescribieron para la consola. No son stdlib,
# pero van al mismo directorio de .gox porque para gccgo da lo mismo: son paquetes con su ruta.
#
# Va aca (y no en un script aparte) para que cualquier build que arme la stdlib los tenga: si faltan,
# el motor se cae en d2util y en los paquetes que lo usan, y el error se ve lejos de la causa.
# ============================================================================
compilar_tercero() {
    local pkgpath="$1" fuente="$2"
    # El destino SALE DE GODC_GOX, igual que la stdlib de arriba. Antes estaba fijo en $DC_ROOT/gox y eso
    # rompia medir con un gox aparte: la stdlib se compilaba en el gox propio pero los paquetes de
    # terceros se iban al compartido, asi que el gox propio quedaba a medias y el motor acusaba
    # "import file ... not found" por paquetes que si estaban compilados (medido: 99 de 142 en vez de 127).
    # Con GODC_GOX puesto, los dos agentes pueden medir sin pisarse la gox del otro.
    local GOX_TERCEROS="${GODC_GOX:-$DC_ROOT/gox}"
    local salida="$GOX_TERCEROS/${pkgpath}.gox"
    mkdir -p "$(dirname "$salida")"

    # Si lo que se pasa es un DIRECTORIO, se compilan todos los .go de adentro. Hacia falta: un paquete de
    # verdad tiene varios archivos (ebiten: ebiten.go + entrada.go), y compilando de a uno los simbolos de
    # los otros no existen ("undefined type CursorMode").
    local archivos=()
    if [ -d "$fuente" ]; then
        for f in "$fuente"/*.go; do
            [ -f "$f" ] || continue
            case "$f" in *_test.go) continue ;; esac
            archivos+=("$f")
        done
    else
        archivos=("$fuente")
    fi

    if sh-elf-gccgo $GODC_GOFLAGS -I "$GOX_TERCEROS" -fgo-pkgpath="$pkgpath" -c "${archivos[@]}" -o "$salida" 2>"$salida.log"; then
        echo "  ${pkgpath##*/}.gox  $(stat -c%s "$salida") bytes  (${#archivos[@]} archivo(s))"
        rm -f "$salida.log"
        # Y AL ORDEN DE ENLACE. Esto faltaba, y era un defecto que no se veia: los terceros se compilaban
        # (quedaban como .gox) pero NO entraban a orden.txt, que es la unica lista que build-dc.sh usa para
        # pasarle objetos al enlazador. Un POC que usara ebiten compilaba y despues moria en el enlace con
        # "undefined reference to ...ebiten_1v2.RunGame": como si el paquete no existiera, cuando existia y
        # estaba compilado. Se ve recien cuando un POC usa un tercero -- o sea, justo ahora que el motor
        # grafico entra en juego.
        echo "$salida" >> "$GOX_TERCEROS/orden.txt"
    else
        echo "  ERROR en $pkgpath:"
        head -4 "$salida.log"
    fi
}

echo "== los paquetes de terceros reescritos para la consola =="
# ebiten: el esqueleto del backend grafico/input/audio (d2core/d2render, d2input, d2audio y d2util)
compilar_tercero "github.com/hajimehoshi/ebiten/v2" "$HERE/../../console/dc/engine/ebiten/v2"
# ebitenutil: las ayudas de dibujo (d2render/ebiten: DebugPrint, DrawLine, DrawRect). Despues de ebiten.
compilar_tercero "github.com/hajimehoshi/ebiten/v2/ebitenutil" "$HERE/../../console/dc/engine/ebiten/v2/ebitenutil"
# audio: el sonido (d2audio/ebiten). Compila y lleva estado, pero el AICA todavia no esta conectado.
compilar_tercero "github.com/hajimehoshi/ebiten/v2/audio" "$HERE/../../console/dc/engine/ebiten/v2/audio"
# inpututil: la parte de input (teclas y textos). Va despues de ebiten, que es de quien depende.
compilar_tercero "github.com/hajimehoshi/ebiten/v2/inpututil" "$HERE/../../console/dc/engine/ebiten/v2/inpututil"
# akara: el BitSet (d2records/monster_unique_affix_*). Medido: es lo unico que se usa de esa libreria.
compilar_tercero "github.com/gravestench/akara" "$HERE/../../console/dc/engine/akara/akara"
# x/image/colornames: los nombres de los colores (d2render/ebiten pinta la pantalla de error de rojo).
# El archivo esta GENERADO desde la tabla real de Go (cmd/gen-colornames), no copiado a mano.
compilar_tercero "golang.org/x/image/colornames" "$HERE/../../console/dc/engine/x/image/colornames"
# audio/wav: el decodificador de WAV (d2audio/ebiten). Este SI decodifica de verdad; lo que falta es el
# chip de audio (el AICA), que es trabajo aparte del video.
compilar_tercero "github.com/hajimehoshi/ebiten/v2/audio/wav" "$HERE/../../console/dc/engine/ebiten/v2/audio/wav"
# pkg/profile: el perfilador de CPU/memoria (d2app). Es un stub: un perfil se escribe a un archivo, y en
# la consola no se puede escribir (romdisk de solo lectura). Para medir tiempos ya esta la consola serie.
compilar_tercero "github.com/pkg/profile" "$HERE/../../console/dc/engine/pkg/profile"
# otto: el interprete de JavaScript (d2script y d2networking/d2server). Es un stub DECLARADO: el scripting
# no es parte del juego de un jugador, y un interprete de JavaScript entero es otro trabajo (el camino, el
# dia que haga falta, es compilar otto tal cual, que es Go puro). Y ojo: d2server importa otto, asi que sin
# esto NO compila d2networking, y sin d2networking no compila d2app: el juego entero.
compilar_tercero "github.com/robertkrimen/otto" "$HERE/../../console/dc/engine/otto"
# otto/underscore: el agregado de underscore.js. d2script lo importa con un guion bajo, o sea que el solo
# hecho de importarlo es el uso (registraria las funciones al arrancar el interprete). Sin interprete no
# tiene nada adentro, y esta dicho por que.
compilar_tercero "github.com/robertkrimen/otto/underscore" "$HERE/../../console/dc/engine/otto/underscore"
# google/uuid: los identificadores de las entidades del mapa (d2mapentity/factory.go). Medido: solo se
# usa uuid.New.
compilar_tercero "github.com/google/uuid" "$HERE/../../console/dc/engine/uuid"
# x/image/bmp: el BMP con los glifos de la fuente del juego (d2util/assets)
compilar_tercero "golang.org/x/image/bmp" "$HERE/../../console/dc/engine/x/image/bmp"
