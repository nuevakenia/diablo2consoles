#!/bin/bash
# Mutadores del paquete os de la consola (DC-002.6, ampliado en DC-002.8a con ioutil y el orden de
# ReadDir): cada uno rompe UNA cosa que el POC del VFS mide. Si el POC sigue VERDE con el mutante
# puesto, el hueco es del POC y hay que taparlo.
#
# Es el equivalente en la CONSOLA de mutar-std-host.sh: os y ioutil no se pueden probar en el host,
# porque ioutil camina sobre el os de la consola y el os habla con el VFS de KOS. Cada mutante cuesta
# compilar la stdlib + el POC (gccgo y kos-cc) y ~30 s de Flycast.
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
IUTIL=console/dc/std/io/ioutil/ioutil.go
RESP=/tmp/mutar-os/os.go
# los archivos que un mutante toca y que hay que dejar iguales al terminar
ARCHIVOS="$OS console/dc/std/time/time.go console/dc/std/log/log.go $IUTIL console/dc/std/csrc/os_dc.c"
mkdir -p /tmp/mutar-os
cp "$OS" "$RESP"
SHA0="$(sha256sum "$OS" | cut -d' ' -f1)"
echo "os.go de partida: sha256 $SHA0"
for f in $ARCHIVOS; do
    sha256sum "$f" | cut -d' ' -f1 > "/tmp/mutar-os/sha-$(echo "$f" | tr / -).orig"
done

muta() {  # $1 = nombre, $2 = buscar, $3 = reemplazar, $4 = archivo (por defecto os.go)
    local arch="${4:-$OS}"
    python3 - "$arch" "$2" "$3" <<'PY'
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
probar() {  # $1 nombre, $2 archivo, $3 buscar, $4 poner, $5 que tiene que romper
    total=$((total + 1))
    local arch="$2"
    echo
    echo "== mutante $total: $1 ($5) =="
    cp "$OS" /tmp/mutar-os/os.go.resp
    cp "$arch" "/tmp/mutar-os/resp-$(basename "$arch")"
    if ! muta "$1" "$3" "$4" "$arch"; then
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
    cp "/tmp/mutar-os/resp-$(basename "$arch")" "$arch"
}

# 1. ReadFile que se queda con la primera vuelta (el archivo grande no entra en el buffer de 4096)
probar "ReadFile de una sola vuelta" \
    "console/dc/std/os/os.go" \
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
    "console/dc/std/os/os.go" \
    "salida = append(salida, dirEntryOS{nombre: campos[0], tam: int64(tam), tipo: tipo})" \
    "salida = append(salida, dirEntryOS{nombre: campos[0], tam: 0, tipo: tipo})" \
    "los tamanos del manifiesto"

# 3. Stat que miente con el tamano
probar "Stat con tamano cero" \
    "console/dc/std/os/os.go" \
    "return infoArchivo{nombre: nombre, tam: int64(tam), dir: d2c_stat_es_dir(nombre) == 1}, nil" \
    "return infoArchivo{nombre: nombre, tam: 0, dir: d2c_stat_es_dir(nombre) == 1}, nil" \
    "Stat de hola.txt (37 bytes)"

# 4. IsNotExist que dice siempre que no falta
probar "IsNotExist siempre falso" \
    "console/dc/std/os/os.go" \
    "return err == ErrNotExist || err == ErrSoloLectura" \
    "return false" \
    "IsNotExist de un archivo que no esta"

# 5. time: Duration.String tiene que imprimir los minutos aunque sean 0 si hay horas (Go da "1h0m0s")
probar "time: Duration sin el 0m de las horas" \
    "console/dc/std/time/time.go" \
    "	if d >= Minute || s != \"\" {" \
    "	if d >= Minute {" \
    "Duration.String(1h) = 1h0m0s"

# 6. time: Sub tiene que mirar los nanosegundos tambien, no solo los segundos
probar "time: Sub que devuelve solo los segundos" \
    "console/dc/std/time/time.go" \
    "	return Duration((t.sec-u.sec)*1000000000 + int64(t.nsec-u.nsec))" \
    "	return Duration(t.sec - u.sec)" \
    "time.Sub(1s) = 1s (no 1ns)"

# 7. log: el prefijo tiene que salir delante del mensaje
probar "log: el prefijo no sale en el mensaje" \
    "console/dc/std/log/log.go" \
    "	fmt.Fprint(fmt.Output, prefijo+s)" \
    "	fmt.Fprint(fmt.Output, s)" \
    "el texto exacto del log con prefijo"

# 8. ReadDir SIN ORDENAR (DC-002.8a): Go ordena por nombre y el manifiesto sale en el orden del build.
#    Lo caza el fixture /rd/desorden, que esta escrito zulu-alfa-mike a proposito: si el manifiesto de
#    /rd alcanzara (sale alfabetico de casualidad, por el glob del build) este mutante quedaria VERDE.
probar "ReadDir sin ordenar (el manifiesto sale como viene)" \
    "console/dc/std/os/os.go" \
    "	for i := 1; i < len(salida); i++ {
		for j := i; j > 0 && salida[j].Name() < salida[j-1].Name(); j-- {
			salida[j], salida[j-1] = salida[j-1], salida[j]
		}
	}
" \
    "" \
    "el orden por nombre del fixture /rd/desorden"

# 9. el manifiesto trata los DIRECTORIOS como archivos (el tipo es lo unico que los distingue)
probar "el directorio del manifiesto como archivo" \
    "console/dc/std/os/os.go" \
    "if campos[2] == \"directorio\" {" \
    "if campos[2] == \"nunca\" {" \
    "la entrada \"desorden\" es un directorio (IsDir/Type)"

# 10. ioutil.ReadDir con el comparador al reves (sigue ordenando, pero mal)
probar "ioutil.ReadDir con el comparador invertido" \
    "console/dc/std/io/ioutil/ioutil.go" \
    "sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })" \
    "sort.Slice(list, func(i, j int) bool { return list[i].Name() > list[j].Name() })" \
    "el orden por nombre de ioutil.ReadDir"

# 11. ReadDir que NO mezcla el listado del OVERLAY de la RAM (DC-002.19): es el defecto de la tarjeta.
#     Lo caza el caso 4m del POC (el directorio que el programa crea en la RAM, con sus entradas) y, en el
#     motor de verdad, el control 3 de checar-arranque-219.sh.
probar "ReadDir sin el overlay de la RAM" \
    "console/dc/std/os/os.go" \
    "	// El overlay TAPA al romdisk con el mismo nombre (la misma regla que os.Open y d2c_hay).
	for _, nueva := range deRAM {" \
    "	// MUTANTE overlay-no-lista
	for _, nueva := range []DirEntry(nil) {" \
    "las entradas del overlay en el caso 4m"

# 12. el listado del overlay del lado del C que no matchea ninguna ruta (DC-002.19): el MISMO defecto, pero
#     del otro lado del puente con KOS (d2c_ramfs_manifiesto). El POC tiene que caer igual: si quedara verde,
#     el puente C no estaria cubierto y el arreglo de Go solo probaria la mitad.
probar "el listado del overlay que no matchea (C)" \
    "console/dc/std/csrc/os_dc.c" \
    "if (l <= n + 1 || strncmp(nombre, prefijo, n + 1) != 0) {" \
    "if (l <= n + 1 || 1) {   /* MUTANTE: no matchea */" \
    "las entradas del overlay en el caso 4m"

# 13. IsExist como la NEGACION de IsNotExist (DC-002.31): con err == nil -- el os.Stat de un archivo que SI
#     esta, que es lo que mira el Exists del proveedor de sistema de archivos del motor -- eso da true donde
#     el os.IsExist de Go da false. Lo cazan los casos 4d-bis del POC (IsExist(nil), IsExist(otro),
#     IsExist(err de Stat de un archivo que si esta)) y, en el motor, el mutante 'exists' de
#     scripts/console/mutar-sueltos-231.sh.
probar "IsExist como la negacion de IsNotExist" \
    "console/dc/std/os/os.go" \
    "func IsExist(err error) bool { return err == ErrExist }" \
    "func IsExist(err error) bool { return !IsNotExist(err) }" \
    "IsExist(nil) en la consola (Go da false, la negacion da true)"

echo
echo "== restauro y verifico =="
cp "$RESP" "$OS"
mal=""
for f in $ARCHIVOS; do
    sha="$(sha256sum "$f" | cut -d' ' -f1)"
    guardado="/tmp/mutar-os/sha-$(echo "$f" | tr / -).orig"
    if [ -f "$guardado" ]; then
        if [ "$sha" != "$(cat "$guardado")" ]; then
            echo "MAL: $f no quedo igual (sha256 $sha, era $(cat "$guardado"))"
            mal="si"
        else
            echo "$f restaurado, sha256 $sha"
        fi
    fi
done
[ -z "$mal" ] || exit 1
echo
echo "== mutantes cazados $cazados/$total =="
[ "$cazados" = "$total" ] || exit 1
