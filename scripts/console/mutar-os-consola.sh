#!/bin/bash
# Mutadores del paquete os de la consola (DC-002.6): cada uno rompe UNA cosa que el POC del VFS mide.
# Si el POC sigue VERDE con el mutante puesto, el hueco es del POC y hay que taparlo.
#
# Es el equivalente en la CONSOLA de mutar-std-host.sh: os no se puede probar en el host, porque su
# implementacion habla con el VFS de KOS. Cada mutante cuesta compilar la stdlib + el POC (gccgo y
# kos-cc) y ~30 s de Flycast.
#
# Restaurar NO se hace con git checkout --: se respalda el archivo en /tmp y se verifica con sha256sum
# (regla del proyecto: el arbol del hito puede tener arreglos sin commitear).
#
# Uso:  scripts/console/mutar-os-consola.sh
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
cd "$HERE/../.." || exit 1
source scripts/console/env.sh

OS=console/dc/std/os/os.go
RESP=/tmp/mutar-os/os.go
mkdir -p /tmp/mutar-os
cp "$OS" "$RESP"
SHA0="$(sha256sum "$OS" | cut -d' ' -f1)"
echo "os.go de partida: sha256 $SHA0"

muta() {  # $1 = nombre, $2 = buscar, $3 = reemplazar
    python3 - "$OS" "$2" "$3" <<'PY'
import sys
ruta, buscar, poner = sys.argv[1], sys.argv[2], sys.argv[3]
texto = open(ruta).read()
if buscar not in texto:
    print("  no encontre el fragmento a mutar", file=sys.stderr)
    sys.exit(3)
open(ruta, "w").write(texto.replace(buscar, poner, 1))
PY
}

corre_poc() {  # compila y corre el POC; deja la linea de resultado en /tmp/mutar-os/resultado
    if ! bash scripts/console/build-std-gox.sh >/tmp/mutar-os/build.log 2>&1; then
        echo "stdlib no compila" > /tmp/mutar-os/resultado
        return
    fi
    if ! bash scripts/console/build-dc.sh console/dc/poc/vfs >>/tmp/mutar-os/build.log 2>&1; then
        echo "el POC no compila" > /tmp/mutar-os/resultado
        return
    fi
    rm -rf /tmp/mutar-os/run
    bash scripts/console/run-flycast.sh /tmp/dcbuild/vfs/prog.elf 30 /tmp/mutar-os/run >/dev/null 2>&1
    grep -a 'VFS: casos' /tmp/mutar-os/run/flycast.log 2>/dev/null | tr -d '\r' | tail -1 > /tmp/mutar-os/resultado
    if [ ! -s /tmp/mutar-os/resultado ]; then
        echo "(el POC no imprimio nada)" > /tmp/mutar-os/resultado
    fi
}

echo
echo "== control: sin mutar tiene que salir VERDE =="
corre_poc
echo "  $(cat /tmp/mutar-os/resultado)"
if ! grep -q 'mal 0' /tmp/mutar-os/resultado; then
    echo "CONTROL MAL: con el codigo sin tocar el POC no da verde. Se corta."
    exit 1
fi

cazados=0
total=0
probar() {  # $1 nombre, $2 buscar, $3 poner, $4 que tiene que romper
    total=$((total + 1))
    echo
    echo "== mutante $total: $1 ($4) =="
    cp "$RESP" "$OS"
    if ! muta "$1" "$2" "$3"; then
        echo "  NO SE PUDO APLICAR (se saltea)"
        cp "$RESP" "$OS"
        return
    fi
    corre_poc
    echo "  $(cat /tmp/mutar-os/resultado)"
    if grep -q 'mal 0' /tmp/mutar-os/resultado; then
        echo "  VERDE: el mutante NO fue cazado. Hueco del POC."
    else
        echo "  ROJO: cazado."
        cazados=$((cazados + 1))
    fi
    cp "$RESP" "$OS"
}

# 1. ReadFile que se queda con la primera vuelta (el archivo grande no entra en el buffer de 4096)
probar "ReadFile de una sola vuelta" \
    "		if err != nil {
			break
		}
	}
	if tam > 0" \
    "		if err != nil {
			break
		}
		break
	}
	if tam > 0" \
    "grande.txt de 6000 bytes (mas que el buffer de 4096)"

# 2. ReadDir que no trae el tamano del manifiesto
probar "ReadDir sin tamanos" \
    "salida = append(salida, DirEntry{nombre: campos[0], tam: int64(tam), tipo: tipo})" \
    "salida = append(salida, DirEntry{nombre: campos[0], tam: 0, tipo: tipo})" \
    "los tamanos del manifiesto"

# 3. Stat que miente con el tamano
probar "Stat con tamano cero" \
    "return infoArchivo{nombre: nombre, tam: int64(tam), dir: d2c_stat_es_dir(nombre) == 1}, nil" \
    "return infoArchivo{nombre: nombre, tam: 0, dir: d2c_stat_es_dir(nombre) == 1}, nil" \
    "Stat de hola.txt (37 bytes)"

# 4. IsNotExist que dice siempre que no falta
probar "IsNotExist siempre falso" \
    "return err == ErrNotExist || err == ErrSoloLectura" \
    "return false" \
    "IsNotExist de un archivo que no esta"

echo
echo "== restauro y verifico =="
cp "$RESP" "$OS"
SHA1="$(sha256sum "$OS" | cut -d' ' -f1)"
if [ "$SHA0" != "$SHA1" ]; then
    echo "MAL: os.go no quedo igual (sha256 $SHA1, era $SHA0)"
    exit 1
fi
echo "os.go restaurado, sha256 $SHA1 (igual al de partida)"
echo
echo "== mutantes cazados $cazados/$total =="
[ "$cazados" = "$total" ] || exit 1
