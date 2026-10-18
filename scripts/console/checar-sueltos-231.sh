#!/bin/bash
# DC-002.31 -- EL EMPAQUETADO DE LOS ASSETS SIN MPQ (y el limite de tamano real de la imagen).
#
# Que mide, con numeros exactos:
#   1. lo que PESA un MPQ descomprimido en archivos sueltos, contra el MPQ (un MPQ comprime: lo suelto
#      puede pesar MAS, y eso hay que saberlo ANTES de elegir el empaquetado);
#   2. si el proveedor de SISTEMA DE ARCHIVOS del motor -- que ya existe -- lee esos sueltos, y con QUE
#      FORMA del nombre, en el HOST y en la CONSOLA. La consola mide EL CAMINO DEL MOTOR (su `Loader`:
#      el que usa `d2asset.AssetManager.LoadFile`), no una sonda a mano: un proveedor que solo anda con
#      la sonda no sirve para el empaquetado;
#   3. hasta que tamano de imagen llega Flycast: el GDEMU no tiene el techo del GD-ROM, el emulador si;
#   4. el arbol suelto del JUEGO ENTERO (los tres MPQ del arranque) en un disco, con su tamano de ISO y
#      las dos paredes del ISO9660 que aparecieron al armarlo;
#   5. (seccion 6) LA TABLA DE os.IsExist/os.IsNotExist de Go, medida con una herramienta del host: los
#      valores esperados del corpus de la consola no salen de la memoria de nadie;
#   6. (seccion 7) LA SELECCION DE ARCHIVOS DEL MOTOR (`d2setup.Complete`/`Validate`) contra el arbol
#      SUELTO: el criterio 1 de la tarjeta, con el MISMO codigo del motor.
#
# Los controles llevan el numero medido, no un "si/no": cuando uno se cae, se ve el numero que lo cayo.
# Los MUTANTES viven aca adentro cuando son de instrumento (el disco sin los sueltos, el Joliet), y en
# scripts/console/mutar-sueltos-231.sh cuando hay que RECOMPILAR la consola para sacarlos (los dos arreglos
# del Source de sistema de archivos).
#
# Uso:  bash scripts/console/checar-sueltos-231.sh [dir-de-trabajo] [layer-gox] [segundos]
#       SUELTOS_SOLO="2 3" para correr solo esas secciones (lo usa el mutador, que necesita la vuelta corta)
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
# shellcheck disable=SC1091
. "$HERE/env.sh"

cd "$REPO"

SAL="${1:-$HOME/.hermes/cache/scratch/corpus-231}"
GEN="${2:-/tmp/gox-330}"
SEGS="${3:-25}"
SECCIONES="${SUELTOS_SOLO:-1 2 3 4 5 6 7}"
JUEGO_REAL="$HOME/.wine-d2/drive_c/Games/Diablo II"
# el MPQ chico: el unico que se puede descomprimir dos veces en una corrida sin gastar 700 MB de disco
MPQ_CHICO="$JUEGO_REAL/patch_D2.mpq"
MPQ_MEDIO="$JUEGO_REAL/d2char.mpq"
MPQ_GRANDE="$JUEGO_REAL/d2data.mpq"
ASSET='data\global\ui\frontend\cinematicselectexp.dc6'
POC="$SAL/poc-231"
ETIQ_SOURCE='SUELTOS-CONSOLA Source.Open con el nombre del MOTOR (barras invertidas)'
ETIQ_LOADER='SUELTOS-CONSOLA Loader.Load con el nombre del MOTOR'

mkdir -p "$SAL"
# el directorio del juego en el disco: los MPQ (el camino de DC-002.27) y, al lado, el arbol suelto
JUEGO="$SAL/juego"
mkdir -p "$JUEGO"
[ -f "$JUEGO/patch_d2.mpq" ] || ln "$MPQ_CHICO" "$JUEGO/patch_d2.mpq"
OK=0
MAL=0
ok() { echo "OK: $1"; OK=$((OK + 1)); }
mal() { echo "MAL: $1"; MAL=$((MAL + 1)); }
# campo: el valor de `clave=valor` -- vale para numeros Y para true/false (el primer defecto del corpus
# fue este: el extractor solo aceptaba digitos y los booleanos salian vacios, con lo que el control se
# ponia ROJO por el instrumento y no por el codigo)
campo() { printf '%s' "$1" | sed -n "s/.*$2=\([^ ]*\).*/\1/p" | head -1; }
proporcion() { awk -v a="$1" -v b="$2" 'BEGIN { printf "%.4f", a / b }'; }
corre() { case " $SECCIONES " in *" $1 "*) return 0 ;; *) return 1 ;; esac; }
# cuenta las lineas de una salida que llevan una etiqueta LITERAL (las etiquetas tienen parentesis)
cuenta() { printf '%s' "$1" | grep -acF "$2" || true; }
# el POC armado: lo necesitan las secciones 3, 4 y 5 (la 3 lo compila y lo deja)
asegura_poc() {
    [ -f "$POC/IP.BIN" ] && return 0
    env DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO=patch_d2.mpq DC_SIN_SELLOS=1 POC=motor-sueltos \
        bash "$HERE/build-motor-arranque.sh" "$POC" "$GEN" > "$SAL/poc.build.log" 2>&1
}
# arma el disco: el binario + el MPQ del juego + (si se le pasa el arbol) los sueltos, en /JUEGO
arma_disco() {
    local dir="$1" arbol="$2"
    mkdir -p "$dir"
    cp -f "$POC/IP.BIN" "$POC/1ST_READ.BIN" "$dir/"
    local args=("$dir/IP.BIN" "$dir/1ST_READ.BIN" "$JUEGO/patch_d2.mpq:/JUEGO/PATCH_D2.MPQ;1")
    if [ -n "$arbol" ]; then
        local f
        while IFS= read -r f; do
            args+=("$f:/JUEGO/$(echo "${f#"$arbol"/}" | tr '[:lower:]' '[:upper:]');1")
        done < <(find "$arbol" -type f | sort)
    fi
    MKISO_JOLIET=0 "$DC_PYTHON" "$HERE/mkiso.py" "$dir/disc.iso" "${args[@]}" > "$dir/iso.log" 2>&1
    "$DC_PYTHON" "$HERE/mkgdi.py" "$dir" >> "$dir/iso.log" 2>&1
}

echo "== secciones: $SECCIONES (dir de trabajo $SAL, layer $GEN, $SEGS s por corrida)"
# el arreglo que la consola y el host tienen que estar midiendo: el Source de sistema de archivos de
# DC-002.31. Si no esta puesto, TODO lo de abajo mide otro codigo (y el mutador lo saca a proposito).
FIX_ESTADO="$(python3 "$HERE/fix-d2loader-fs.py" --ver 2>&1)"
SHA_SOURCE="$(echo "$FIX_ESTADO" | sed -n 's/.*en el arbol:  sha256 \([0-9a-f]*\)/\1/p')"
SHA_SANO="$(echo "$FIX_ESTADO" | sed -n 's/.*sano:         sha256 \([0-9a-f]*\)/\1/p')"
echo "   source.go sha256=$SHA_SOURCE (sano=$SHA_SANO)"

# =================================================================================================
if corre 1; then
    echo "== 1. CUANTO PESA UN MPQ DESCOMPRIMIDO EN ARCHIVOS SUELTOS =="
    for par in "patch:$MPQ_CHICO" "d2char:$MPQ_MEDIO" "d2data:$MPQ_GRANDE"; do
        NOMBRE="${par%%:*}"
        RUTA="${par#*:}"
        DEST="$SAL/sueltos-$NOMBRE"
        LINEA="$(go run ./console/dc/mpq-extraer -mpq "$RUTA" -dest "$DEST" -minusculas 2>&1 | grep -a 'MPQ-EXTRAER archivos=')"
        echo "   $NOMBRE: $LINEA"
        echo "$LINEA" > "$SAL/sueltos-$NOMBRE.txt"

        ARCH="$(campo "$LINEA" archivos)"
        SUELTO="$(campo "$LINEA" bytes_sueltos)"
        COLIS="$(campo "$LINEA" colisiones_de_nombre)"
        MPQB="$(stat -c%s "$RUTA")"
        printf '%s' "$LINEA" | grep -a 'MPQ-EXTRAER proporcion' >> "$SAL/sueltos-$NOMBRE.txt"

        if [ "${ARCH:-0}" -gt 0 ] && [ "${SUELTO:-0}" -gt "$MPQB" ]; then
            ok "1.$NOMBRE $ARCH archivos: $SUELTO bytes sueltos contra $MPQB del MPQ (proporcion $(proporcion "$SUELTO" "$MPQB")), colisiones=$COLIS"
        else
            mal "1.$NOMBRE sueltos=$SUELTO MPQ=$MPQB (el MPQ NO comprime: el suelto es mas chico o igual)"
        fi
    done

    # el arbol de la CAJA del MPQ (sin -minusculas): lo usa el control negativo del punto 2. Ademas mide
    # las colisiones de nombre, que en el disco son choques de verdad y en el MPQ no existen.
    CAJA_LINEA="$(go run ./console/dc/mpq-extraer -mpq "$MPQ_CHICO" -dest "$SAL/sueltos-patch-caja" 2>&1 | grep -a 'MPQ-EXTRAER archivos=')"
    echo "   patch (caja del MPQ): $CAJA_LINEA"
    COLIS_CAJA="$(campo "$CAJA_LINEA" colisiones_de_nombre)"
    ok "1.colisiones el arbol de la caja del MPQ tiene $COLIS_CAJA colisiones de nombre en el disco (dos entradas que solo difieren en mayusculas se pisan)"
fi

# =================================================================================================
if corre 2; then
    echo "== 2. EL PROVEEDOR DE SISTEMA DE ARCHIVOS DEL MOTOR (host) =="
    HOST="$(go run ./console/dc/sonda-sueltos -mpq "$MPQ_CHICO" -raiz "$SAL/sueltos-patch" -asset "$ASSET" 2>&1)"
    echo "$HOST" | sed 's/^/   /'
    echo "$HOST" > "$SAL/sonda-host.txt"

    INV="$(campo "$HOST" barras_invertidas_OK)"
    NOR="$(campo "$HOST" barras_normales_OK)"
    SRC_EXISTS="$(campo "$HOST" source_exists)"
    LOADER_ADD="$(campo "$HOST" loader_addsource_OK)"
    LOADER_LOAD="$(campo "$HOST" loader_load_OK)"
    LOADER_EXISTS="$(campo "$HOST" loader_exists)"
    BYTES_SUELTO="$(printf '%s' "$HOST" | sed -n 's/.*barras_normales_OK=true bytes=\([0-9]*\).*/\1/p')"
    SUMA_SUELTO="$(printf '%s' "$HOST" | sed -n 's/.*barras_normales_OK=true bytes=[0-9]* suma=\([0-9]*\).*/\1/p')"
    SUMA_MPQ="$(printf '%s' "$HOST" | sed -n 's/.*mpq_OK=true bytes=[0-9]* suma=\([0-9]*\).*/\1/p')"
    SUMA_LOADER="$(printf '%s' "$HOST" | sed -n 's/.*loader_load_OK=true bytes=[0-9]* suma=\([0-9]*\).*/\1/p')"

    if [ "$SHA_SOURCE" = "$SHA_SANO" ]; then
        ok "2a el arbol tiene los dos arreglos del Source de DC-002.31 (source.go sha256=$SHA_SOURCE)"
    else
        mal "2a el arbol NO tiene los arreglos del Source: sha256=$SHA_SOURCE contra el sano $SHA_SANO (corre scripts/console/fix-d2loader-fs.py)"
    fi

    [ "$INV" = "true" ] && ok "2b el Source abre con EL NOMBRE DEL MOTOR (barras invertidas): $BYTES_SUELTO bytes" ||
        mal "2b con el nombre del motor el Source FALLA (barras_invertidas_OK=$INV): es el defecto del nombre"

    if [ "$NOR" = "true" ] && [ -n "$SUMA_SUELTO" ] && [ "$SUMA_SUELTO" = "$SUMA_MPQ" ]; then
        ok "2c el arbol suelto entrega el MISMO asset que el MPQ: $BYTES_SUELTO bytes, suma $SUMA_SUELTO en los dos"
    else
        mal "2c sueltos OK=$NOR suma=$SUMA_SUELTO contra MPQ $SUMA_MPQ"
    fi

    [ "$SRC_EXISTS" = "true" ] && ok "2d Source.Exists=true con el archivo presente (el os.IsExist(err) con err==nil daba false)" ||
        mal "2d Source.Exists=$SRC_EXISTS con el archivo presente: es el defecto del Exists"

    if [ "$LOADER_ADD" = "true" ] && [ "$LOADER_LOAD" = "true" ] && [ "$SUMA_LOADER" = "$SUMA_MPQ" ]; then
        ok "2e EL CAMINO DEL MOTOR: Loader.AddSource + Loader.Load dan el asset (suma $SUMA_LOADER = la del MPQ)"
    else
        mal "2e el Loader del motor no carga el suelto: addsource=$LOADER_ADD load=$LOADER_LOAD suma=$SUMA_LOADER contra $SUMA_MPQ"
    fi

    [ "$LOADER_EXISTS" = "true" ] && ok "2f Loader.Exists=true (lo que responde el asset.FileExists del motor con un proveedor de sistema de archivos)" ||
        mal "2f Loader.Exists=$LOADER_EXISTS: el motor contestaria 'no esta' con el archivo presente"

    # control negativo: el arbol con la CAJA del MPQ (FrontEnd) -- el disco distingue mayusculas, el MPQ no
    CAJA="$(go run ./console/dc/sonda-sueltos -mpq "$MPQ_CHICO" -raiz "$SAL/sueltos-patch-caja" -asset "$ASSET" 2>&1)"
    echo "$CAJA" | sed 's/^/   (caja) /'
    CAJA_LOAD="$(campo "$CAJA" loader_load_OK)"
    CAJA_MPQ="$(campo "$CAJA" mpq_OK)"
    if [ "$CAJA_LOAD" = "false" ] && [ "$CAJA_MPQ" = "true" ]; then
        ok "2g control negativo: con las carpetas en la caja del MPQ (FrontEnd) el Loader FALLA y el mismo asset del MPQ sale igual"
    else
        mal "2g control negativo flojo: caja load=$CAJA_LOAD mpq=$CAJA_MPQ (el arbol suelto tiene que ir en minusculas)"
    fi
fi

# =================================================================================================
if corre 3; then
    echo "== 3. EL ARBOL SUELTO EN LA CONSOLA (el POC motor-sueltos, EL CAMINO DEL MOTOR) =="
    # el arbol suelto de patch: si la seccion 1 no corrio, se extrae aca (es el chico)
    [ -d "$SAL/sueltos-patch" ] || go run ./console/dc/mpq-extraer -mpq "$MPQ_CHICO" -dest "$SAL/sueltos-patch" -minusculas > /dev/null

    # el .gox del paquete arreglado tiene que ser el del arbol de ahora: si no, la consola mediria el
    # codigo viejo (es la trampa de la camada, DC-002.29). Son DOS: el del Source de sistema de archivos
    # (el motor) y el de la stdlib `os` -- el Exists que el mutante del Source llama es el `os.IsExist` de
    # la consola, y sin recompilarlo la consola seguia midiendo el IsExist viejo (medido: el mutante
    # 'exists' quedaba VERDE en la consola y ROJO en el host por esto mismo).
    bash "$HERE/recompilar-paquete-motor.sh" d2common/d2loader/filesystem "$GEN" > "$SAL/recompilar-fs.log" 2>&1
    echo "   $(tail -1 "$SAL/recompilar-fs.log")"
    bash "$HERE/compilar-paquete-std.sh" os "$GEN" >> "$SAL/recompilar-fs.log" 2>&1
    echo "   $(tail -1 "$SAL/recompilar-fs.log") (std)"

    if env DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO=patch_d2.mpq DC_SIN_SELLOS=1 POC=motor-sueltos \
        bash "$HERE/build-motor-arranque.sh" "$POC" "$GEN" > "$SAL/poc.build.log" 2>&1; then
        ok "3a el POC motor-sueltos compila y enlaza ($(grep -ac . "$SAL/poc.build.log") lineas de log)"
    else
        mal "3a el POC NO compila: $(tail -3 "$SAL/poc.build.log")"
    fi

    arma_disco "$SAL/poc-con-sueltos" "$SAL/sueltos-patch"
    CAPTURE=none bash "$HERE/run-flycast.sh" "$SAL/poc-con-sueltos/disc.gdi" "$SEGS" "$SAL/poc-con-sueltos-run" > "$SAL/poc-con-sueltos-run.out" 2>&1
    LOG="$(tr -d '\r' < "$SAL/poc-con-sueltos-run/flycast.log")"
    echo "$LOG" | grep -a 'SUELTOS-CONSOLA' | sed 's/^/   /' | head -14

    C_SOURCE_OK="$(cuenta "$LOG" "$ETIQ_SOURCE OK")"
    C_SOURCE_FALLO="$(cuenta "$LOG" "$ETIQ_SOURCE FALLO")"
    C_EXISTS="$(cuenta "$LOG" 'SUELTOS-CONSOLA Source.Exists con el nombre del MOTOR= true')"
    C_LOADER_OK="$(cuenta "$LOG" "$ETIQ_LOADER OK")"
    C_LOADER_FALLO="$(cuenta "$LOG" "$ETIQ_LOADER FALLO")"
    C_LOADER_EXISTS="$(cuenta "$LOG" 'SUELTOS-CONSOLA Loader.Exists con el nombre del MOTOR= true')"
    N_SUELTOS="$(find "$SAL/sueltos-patch" -type f 2>/dev/null | wc -l)"
    C_SUMA="$(printf '%s' "$LOG" | sed -n "s/.*Loader.Load con el nombre del MOTOR OK bytes= [0-9]* suma= \([0-9]*\).*/\1/p" | head -1)"
    C_MPQ_SUMA="$(printf '%s' "$LOG" | sed -n 's/.*MPQ ReadFile OK bytes= [0-9]* suma= \([0-9]*\).*/\1/p' | head -1)"
    C_READDIR="$(printf '%s' "$LOG" | sed -n 's/.*ReadDir \/cd\/juego entradas= \([0-9]*\).*/\1/p' | head -1)"
    C_STAT_FALLO="$(printf '%s' "$LOG" | grep -acF 'SUELTOS-CONSOLA stat /cd/juego FALLO' || true)"

    [ "${C_SOURCE_OK:-0}" -gt 0 ] && [ "${C_SOURCE_FALLO:-0}" -eq 0 ] &&
        ok "3b la consola abre el asset con EL NOMBRE DEL MOTOR desde el arbol suelto ($C_SOURCE_OK lineas OK)" ||
        mal "3b consola: Source con el nombre del motor OK=$C_SOURCE_OK FALLO=$C_SOURCE_FALLO"

    [ "${C_EXISTS:-0}" -gt 0 ] && ok "3c la consola: Source.Exists=true con el archivo presente ($C_EXISTS)" ||
        mal "3c la consola: Source.Exists no dio true ($C_EXISTS)"

    if [ "${C_LOADER_OK:-0}" -gt 0 ] && [ -n "$C_SUMA" ] && [ "$C_SUMA" = "$C_MPQ_SUMA" ]; then
        ok "3d EL CAMINO DEL MOTOR en la consola: Loader.Load da el asset (suma $C_SUMA = la del MPQ) sobre el arbol suelto de $N_SUELTOS archivos"
    else
        mal "3d consola: Loader.Load OK=$C_LOADER_OK FALLO=$C_LOADER_FALLO suma=$C_SUMA contra MPQ $C_MPQ_SUMA"
    fi

    [ "${C_LOADER_EXISTS:-0}" -gt 0 ] && ok "3e la consola: Loader.Exists=true con el archivo presente" ||
        mal "3e la consola: Loader.Exists no dio true ($C_LOADER_EXISTS)"

    # LIMITE DECLARADO (DC-002.27): el driver ISO9660 de KOS no contesta el stat de un DIRECTORIO ni su
    # readdir. Este control vigila que siga siendo asi: si algun dia los contesta, el control avisa y el
    # limite se revisa (0 entradas y el stat del directorio en FALLO es lo que obliga al .manifiesto).
    if [ "${C_READDIR:-x}" = "0" ] && [ "${C_STAT_FALLO:-0}" -gt 0 ]; then
        ok "3f limite conocido y vigente: el stat del DIRECTORIO del disco falla ($C_STAT_FALLO) y su ReadDir da 0 entradas"
    else
        mal "3f el limite cambio: ReadDir entradas=$C_READDIR stat en FALLO=$C_STAT_FALLO (si el driver ya contesta, revisar el .manifiesto del romdisk)"
    fi

    echo "== 3g-MUTANTE. El MISMO disco SIN el arbol suelto: el Loader tiene que fallar y el MPQ leer igual =="
    arma_disco "$SAL/poc-sin-sueltos" ""
    CAPTURE=none bash "$HERE/run-flycast.sh" "$SAL/poc-sin-sueltos/disc.gdi" "$SEGS" "$SAL/poc-sin-sueltos-run" > "$SAL/poc-sin-sueltos-run.out" 2>&1
    LOGM="$(tr -d '\r' < "$SAL/poc-sin-sueltos-run/flycast.log")"
    echo "$LOGM" | grep -a 'SUELTOS-CONSOLA' | sed 's/^/   /' | head -8
    M_MOTOR="$(cuenta "$LOGM" "$ETIQ_LOADER FALLO")"
    M_MPQ="$(cuenta "$LOGM" 'SUELTOS-CONSOLA MPQ ReadFile OK')"
    if [ "${M_MOTOR:-0}" -gt 0 ] && [ "${M_MPQ:-0}" -gt 0 ]; then
        ok "3g MUTANTE cazado: sin los sueltos en el disco el Loader del motor falla ($M_MOTOR) y el MPQ se lee igual ($M_MPQ)"
    else
        mal "3g el MUTANTE quedo verde: Loader FALLO=$M_MOTOR MPQ OK=$M_MPQ"
    fi
fi

# =================================================================================================
if corre 4; then
    echo "== 4. EL TAMANO DE LA IMAGEN Y LO QUE DE VERDAD ROMPE EL MONTAJE =="
    # Medido con las mismas recetas que se reproducen aca:
    #   dummy 700 MB  (ISO 1.275.086.848 bytes = 1,19 GB) -> arranca y el disco SE LEE (el techo del GD-ROM no es el techo)
    #   dummy 1516 MB (ISO 2.130.724.864 bytes = 2,03 GB) -> tambien
    #   y un ISO de 2.130.724.864 bytes se leia igual: el tamano no era la causa de que un disco grande no
    #   montara. La causa aislada: el arbol JOLIET que pycdlib agrega por defecto (4c/4d).
    JUEGO3="$SAL/juego3"
    mkdir -p "$JUEGO3"
    for m in patch_D2 d2char d2data; do
        [ -f "$JUEGO3/$m.mpq" ] || ln "$JUEGO_REAL/$m.mpq" "$JUEGO3/$m.mpq"
    done

    POC_BASE="$POC"
    asegura_poc
    for MB in 700 1516; do
        OUT="$SAL/grande-$MB"
        mkdir -p "$OUT"
        cp -f "$POC_BASE/IP.BIN" "$POC_BASE/1ST_READ.BIN" "$OUT/"
        truncate -s "${MB}M" "$OUT/grande.bin"
        MKISO_JOLIET=0 "$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "$OUT/1ST_READ.BIN" \
            "$JUEGO3/patch_D2.mpq:/JUEGO/PATCH_D2.MPQ;1" \
            "$JUEGO3/d2char.mpq:/JUEGO/D2CHAR.MPQ;1" \
            "$JUEGO3/d2data.mpq:/JUEGO/D2DATA.MPQ;1" \
            "$OUT/grande.bin:/GRANDE.BIN;1" > "$SAL/grande-$MB.log" 2>&1
        "$DC_PYTHON" "$HERE/mkgdi.py" "$OUT" >> "$SAL/grande-$MB.log" 2>&1
        CAPTURE=none bash "$HERE/run-flycast.sh" "$OUT/disc.gdi" 22 "$OUT-run" > "$OUT-run.out" 2>&1
        LOGG="$(tr -d '\r' < "$OUT-run/flycast.log")"
        BYTES="$(stat -c%s "$OUT/disc.iso")"
        MONTA="$(printf '%s' "$LOGG" | grep -acF 'SUELTOS-CONSOLA MPQ ReadFile OK' || true)"
        ARRANCA="$(printf '%s' "$LOGG" | grep -acF 'SUELTOS-CONSOLA arranca' || true)"
        echo "   dummy ${MB}MB -> ISO $BYTES bytes = $((BYTES / 1024 / 1024)) MB; arranca=$ARRANCA; lee_el_disco=$MONTA"

        if [ "${ARRANCA:-0}" -gt 0 ] && [ "${MONTA:-0}" -gt 0 ]; then
            ok "4.${MB}MB la consola arranca y LEE un disco de $((BYTES / 1024 / 1024)) MB (el techo del GD-ROM es 1,1 GB)"
        else
            mal "4.${MB}MB con $((BYTES / 1024 / 1024)) MB no arranco/leyo (arranca=$ARRANCA lee=$MONTA)"
        fi
    done

    echo "   -- 4c: el Joliet (el DEFAULT de mkiso.py) contra el mismo ISO sin Joliet --"
    for MODO in con_joliet sin_joliet; do
        OUT="$SAL/joliet-$MODO"
        mkdir -p "$OUT"
        cp -f "$POC_BASE/IP.BIN" "$POC_BASE/1ST_READ.BIN" "$OUT/"

        if [ "$MODO" = "con_joliet" ]; then
            "$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "$OUT/1ST_READ.BIN" \
                "$JUEGO/patch_d2.mpq:/JUEGO/PATCH_D2.MPQ;1" > "$SAL/joliet-$MODO.log" 2>&1
        else
            MKISO_JOLIET=0 "$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "$OUT/1ST_READ.BIN" \
                "$JUEGO/patch_d2.mpq:/JUEGO/PATCH_D2.MPQ;1" > "$SAL/joliet-$MODO.log" 2>&1
        fi

        "$DC_PYTHON" "$HERE/mkgdi.py" "$OUT" >> "$SAL/joliet-$MODO.log" 2>&1
        CAPTURE=none bash "$HERE/run-flycast.sh" "$OUT/disc.gdi" 22 "$OUT-run" > "$OUT-run.out" 2>&1
        LOGJ="$(tr -d '\r' < "$OUT-run/flycast.log")"
        LEE="$(printf '%s' "$LOGJ" | grep -acF 'SUELTOS-CONSOLA MPQ ReadFile OK' || true)"
        NOLEE="$(printf '%s' "$LOGJ" | grep -acF 'SUELTOS-CONSOLA el MPQ del disco FALLO' || true)"
        ARR="$(printf '%s' "$LOGJ" | grep -acF 'SUELTOS-CONSOLA arranca' || true)"
        echo "      $MODO: ISO $(stat -c%s "$OUT/disc.iso") bytes; arranca=$ARR lee=$LEE no_lee=$NOLEE"

        if [ "$MODO" = "con_joliet" ]; then
            { [ "$ARR" -gt 0 ] && [ "$LEE" -eq 0 ]; } && ok "4c MUTANTE cazado: con el Joliet por defecto el binario arranca pero el disco NO monta ($NOLEE marcas)" ||
                mal "4c con Joliet el disco se leyo: el defecto del Joliet no se reprodujo (lee=$LEE)"
        else
            [ "$LEE" -gt 0 ] && ok "4d el MISMO ISO sin Joliet monta: el que rompe es el descriptor JOLIET, no el tamano" ||
                mal "4d sin Joliet tampoco leyo: el instrumento no distingue (lee=$LEE)"
        fi
    done
fi

# =================================================================================================
if corre 5; then
    echo "== 5. EL ARBOL SUELTO DEL JUEGO ENTERO (los tres MPQ del arranque) EN UN DISCO =="
    # el empaquetado de la tarjeta, medido: los tres MPQ que el motor necesita para arrancar, sueltos y
    # en el disco, con la suma de los tres arboles y el tamano REAL de la ISO. El techo de un CD-R es
    # 700 MB; el del GDEMU es la tarjeta.
    SUELTOS=""
    TOTAL_SUELTO=0
    TOTAL_MPQ=0
    for m in patch d2char d2data; do
        case "$m" in
            patch) ruta="$MPQ_CHICO" ;; d2char) ruta="$MPQ_MEDIO" ;; *) ruta="$MPQ_GRANDE" ;;
        esac
        [ -d "$SAL/sueltos-$m" ] || go run ./console/dc/mpq-extraer -mpq "$ruta" -dest "$SAL/sueltos-$m" -minusculas > /dev/null
        n="$(find "$SAL/sueltos-$m" -type f | wc -l)"
        b="$(find "$SAL/sueltos-$m" -type f -printf '%s\n' | awk '{s+=$1} END {print s+0}')"
        mb="$(stat -c%s "$ruta")"
        echo "   $m: $n archivos, $b bytes en el disco contra $mb del MPQ"
        TOTAL_SUELTO=$((TOTAL_SUELTO + b))
        TOTAL_MPQ=$((TOTAL_MPQ + mb))
    done
    echo "   TOTAL suelto=$TOTAL_SUELTO bytes contra $TOTAL_MPQ de los tres MPQ (proporcion $(proporcion "$TOTAL_SUELTO" "$TOTAL_MPQ"))"

    POC_BASE="$POC"
    asegura_poc
    OUT="$SAL/juego-suelto"
    mkdir -p "$OUT"
    cp -f "$POC_BASE/IP.BIN" "$POC_BASE/1ST_READ.BIN" "$OUT/"
    # DC-002.31: 20.976 archivos NO entran en la lista de argumentos (E2BIG: "La lista de argumentos es
    # demasiado larga", medido). Se pasan por un archivo con la forma @lista (ver mkiso.py).
    #
    # Y el arbol suelto del juego NO entra entero en un ISO9660. Dos paredes, las dos MEDIDAS con el
    # build muriendo y con la regla leida en pycdlib:
    #   (1) los NOMBRES: solo admite A-Z 0-9 _ y . (`PyCdlibInvalidInput: ISO9660 filenames must consist of
    #       characters A-Z, 0-9, and _`): 9 entradas del juego tienen ESPACIO (7) o GUION (2)
    #       (`IZUAL DEATH.DCC`, `SUB-HAR.DT1`, ...).
    #   (2) la PROFUNDIDAD: 7 niveles (Ecma-119 6.8.2.1, 8 con la raiz; pycdlib `len(split_path(...)) > 7`)
    #       (`PyCdlibInvalidInput: Directory levels too deep (maximum is 7)`): 7 entradas del juego tienen
    #       8 niveles (`/JUEGO/DATA/GLOBAL/MONSTERS/VM/COF/VM/VMA1HTH.COF;1`).
    # Las que no entran se SALTAN y se CUENTAN (renombrarlas no sirve: el motor las busca por su nombre).
    LISTA="$OUT/lista.txt"
    FUERA_NOMBRE=0
    FUERA_PROF=0
    {
        printf '%s\n' "$OUT/IP.BIN" "$OUT/1ST_READ.BIN"
        printf '%s\n' "$JUEGO/patch_d2.mpq:/JUEGO/PATCH_D2.MPQ;1"
        for m in patch d2char d2data; do
            while IFS= read -r f; do
                rel="${f#"$SAL/sueltos-$m"/}"
                origen="$(printf '%s' "$rel" | tr '[:lower:]' '[:upper:]')"
                if printf '%s' "$origen" | grep -q '[^A-Z0-9_./]'; then
                    FUERA_NOMBRE=$((FUERA_NOMBRE + 1))
                    # al stderr: el stdout de este bloque ES la lista
                    [ "$FUERA_NOMBRE" -le 12 ] && echo "   FUERA por el NOMBRE: /JUEGO/$origen" >&2
                    continue
                fi
                # los niveles se cuentan sobre la ruta INTERNA completa (/JUEGO/DATA/...), que es la que ve
                # pycdlib: los slashes de `origen` + 2 (uno por la barra de adelante y otro por JUEGO). Dos
                # intentos fallidos, medidos: con los slashes de `origen` y con +1 los 7 casos de 8 niveles
                # pasaban y el build moria igual en pycdlib.
                NIVELES="$(printf '%s' "$origen" | tr -cd '/' | wc -c)"
                if [ "$((NIVELES + 2))" -gt 7 ]; then
                    FUERA_PROF=$((FUERA_PROF + 1))
                    [ "$FUERA_PROF" -le 12 ] && echo "   FUERA por PROFUNDIDAD ($((NIVELES + 2)) niveles): /JUEGO/$origen" >&2
                    continue
                fi
                printf '%s:/JUEGO/%s;1\n' "$f" "$origen"
            done < <(find "$SAL/sueltos-$m" -type f | sort)
        done
    } > "$LISTA"
    N_ARCH="$(wc -l < "$LISTA")"
    echo "   lista: $N_ARCH entradas = 3 (IP.BIN, 1ST_READ.BIN, el MPQ) + $((N_ARCH - 3)) del arbol suelto del juego"
    echo "   el ISO9660 deja afuera $FUERA_NOMBRE por el NOMBRE y $FUERA_PROF por la PROFUNDIDAD (de $((N_ARCH - 3 + FUERA_NOMBRE + FUERA_PROF)))"
    MKISO_JOLIET=0 "$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "@$LISTA" > "$OUT/iso.log" 2>&1
    "$DC_PYTHON" "$HERE/mkgdi.py" "$OUT" >> "$OUT/iso.log" 2>&1
    ISO_BYTES="$(stat -c%s "$OUT/disc.iso" 2>/dev/null || echo 0)"
    echo "   ISO con $((N_ARCH - 3)) archivos sueltos: $ISO_BYTES bytes = $((ISO_BYTES / 1024 / 1024)) MB"

    CAPTURE=none bash "$HERE/run-flycast.sh" "$OUT/disc.gdi" 22 "$OUT-run" > "$OUT-run.out" 2>&1
    LOG5="$(tr -d '\r' < "$OUT-run/flycast.log")"
    ARR5="$(printf '%s' "$LOG5" | grep -acF 'SUELTOS-CONSOLA arranca' || true)"
    OK5="$(printf '%s' "$LOG5" | grep -acF "$ETIQ_LOADER OK" || true)"
    SUMA5="$(printf '%s' "$LOG5" | sed -n "s/.*Loader.Load con el nombre del MOTOR OK bytes= [0-9]* suma= \([0-9]*\).*/\1/p" | head -1)"
    MPQ5="$(printf '%s' "$LOG5" | sed -n 's/.*MPQ ReadFile OK bytes= [0-9]* suma= \([0-9]*\).*/\1/p' | head -1)"

    if [ "${ARR5:-0}" -gt 0 ] && [ "${OK5:-0}" -gt 0 ] && [ -n "$SUMA5" ] && [ "$SUMA5" = "$MPQ5" ]; then
        ok "5a la consola lee un asset del arbol suelto del JUEGO ENTERO ($((N_ARCH - 3)) archivos en el disco), suma $SUMA5 = la del MPQ"
    else
        mal "5a arranca=$ARR5 loadOK=$OK5 suma=$SUMA5 contra MPQ $MPQ5"
    fi

    if [ "$FUERA_NOMBRE" -eq 9 ] && [ "$FUERA_PROF" -eq 7 ] && [ "$((N_ARCH - 3))" -eq 20960 ]; then
        ok "5b el arbol suelto NO entra entero en un ISO9660: de 20976 entradas del juego entran 20960, quedan afuera 9 por el NOMBRE (espacio o guion) y 7 por la PROFUNDIDAD (8 niveles)"
    else
        mal "5b el ISO9660 dejo afuera $FUERA_NOMBRE por nombre y $FUERA_PROF por profundidad, y entraron $((N_ARCH - 3)) (medido antes: 9, 7 y 20960)"
    fi

    if [ "$ISO_BYTES" -gt 0 ] && [ "$ISO_BYTES" -lt 734003200 ]; then
        ok "5c las 20960 entradas que SI entran pesan una ISO de $ISO_BYTES bytes contra 734.003.200 del tope de 700 MB (sobran $((734003200 - ISO_BYTES)))"
    else
        mal "5c la ISO con las entradas que entran mide $ISO_BYTES bytes"
    fi
fi

# =================================================================================================
if corre 6; then
    echo "== 6. LA TABLA DE REFERENCIA DE os.IsExist/os.IsNotExist (la biblioteca de Go, medida) =="
    # El paquete os de la consola no compila en el host (habla con KOS por //extern), asi que los valores
    # esperados del corpus de la consola (poc/vfs, seccion 4d-bis) salen de esta tabla: aca se mide la
    # biblioteca REAL y se exige, linea por linea, que siga diciendo lo mismo. El defecto de DC-002.31 fue
    # `IsExist(err) = !IsNotExist(err)`: con err == nil daba true donde Go da false.
    TABLA="$(go run ./console/dc/sonda-isexit 2>&1)"
    echo "$TABLA" | sed 's/^/   /'
    echo "$TABLA" > "$SAL/sonda-isexit.txt"

    ESPERADO="SONDA-ISEXIT nil exist=false notexist=false
SONDA-ISEXIT ErrExist exist=true notexist=false
SONDA-ISEXIT ErrNotExist exist=false notexist=true
SONDA-ISEXIT ErrPermission exist=false notexist=false
SONDA-ISEXIT otro exist=false notexist=false
SONDA-ISEXIT envuelto_ErrNotExist exist=false notexist=false
SONDA-ISEXIT error_de_Open exist=false notexist=true
SONDA-ISEXIT nil_de_Stat exist=false notexist=false"

    if [ "$TABLA" = "$ESPERADO" ]; then
        ok "6a la tabla de os.IsExist/os.IsNotExist de Go es la documentada (8 casos): nil -> false/false, ErrExist -> true/false, un envuelto NO se desenvuelve"
    else
        mal "6a la tabla de Go cambio (el corpus de la consola espera la de antes): $(echo "$TABLA" | diff - <(echo "$ESPERADO") | head -3 | tr '\n' ' ')"
    fi
fi

# =================================================================================================
if corre 7; then
    echo "== 7. LA SELECCION DE ARCHIVOS DEL MOTOR (d2setup) contra el arbol SUELTO (el criterio 1) =="
    # El arranque del motor decide con `d2setup.Complete`/`Validate` si tiene los archivos del juego, y
    # despues le pide sus fuentes al Loader por NOMBRE de MPQ (`d2app/initialization.go:82`). Con el arbol
    # suelto el motor YA tiene el proveedor (medido en 2 y 3), pero la SELECCION es la que decide si el
    # arranque sigue o se va al camino de primera ejecucion (`EnsureGameFiles`: los dialogos que la consola
    # no tiene, la pared de DC-002.19). Por eso este control es el que contesta el criterio 1 con numeros.
    [ -d "$SAL/sueltos-patch" ] || go run ./console/dc/mpq-extraer -mpq "$MPQ_CHICO" -dest "$SAL/sueltos-patch" -minusculas > /dev/null

    SEL="$(go run ./console/dc/sonda-seleccion -sueltos "$SAL/sueltos-patch" -mpqdir "$JUEGO" -orden "patch_d2.mpq,d2data.mpq,d2char.mpq" 2>&1)"
    echo "$SEL" | sed 's/^/   /'
    echo "$SEL" > "$SAL/sonda-seleccion.txt"

    # el control positivo: con SOLO el MPQ y la orden que lo pide, la seleccion del motor SI pasa
    SELPOS="$(go run ./console/dc/sonda-seleccion -sueltos "$SAL/sueltos-patch" -mpqdir "$JUEGO" -orden "patch_d2.mpq" 2>&1)"
    echo "$SELPOS" | grep -a '2-solo-mpq Complete' | sed 's/^/   (positivo) /'
    echo "$SELPOS" >> "$SAL/sonda-seleccion.txt"

    SUELTOS_VE="$(printf '%s' "$SEL" | sed -n 's/.*1-solo-sueltos sueltos en el dir: \([0-9]*\) archivos.*/\1/p' | head -1)"
    C1_COMPLETE="$(printf '%s' "$SEL" | sed -n 's/.*1-solo-sueltos Complete=\([^ ]*\).*/\1/p' | head -1)"
    C1_VE="$(printf '%s' "$SEL" | sed -n 's/.*1-solo-sueltos Complete=[^ ]* Validate.OK()=[^ ]* archivos_que_ve_en_el_orden=\([0-9]*\).*/\1/p' | head -1)"
    C2_VE="$(printf '%s' "$SEL" | sed -n 's/.*2-solo-mpq Complete=[^ ]* Validate.OK()=[^ ]* archivos_que_ve_en_el_orden=\([0-9]*\).*/\1/p' | head -1)"
    CP_COMPLETE="$(printf '%s' "$SELPOS" | sed -n 's/.*2-solo-mpq Complete=\([^ ]*\).*/\1/p' | head -1)"

    if [ "${SUELTOS_VE:-0}" -gt 0 ] && [ "$C1_COMPLETE" = "false" ] && [ "${C1_VE:-x}" = "0" ]; then
        ok "7a la seleccion del motor NO ve el arbol suelto: $SUELTOS_VE archivos en el directorio y Complete=false con 0 archivos del orden (la seleccion pide NOMBRES de MPQ)"
    else
        mal "7a con SOLO sueltos: sueltos=$SUELTOS_VE Complete=$C1_COMPLETE archivos_que_ve=$C1_VE"
    fi

    if [ "${C2_VE:-x}" = "1" ]; then
        ok "7b con el MPQ de verdad la seleccion ve $C2_VE archivo del orden (el patch) y Complete sigue false con la orden de 3: el que manda es el NOMBRE del MPQ"
    else
        mal "7b la seleccion con el MPQ ve $C2_VE archivos del orden (medido antes: 1)"
    fi

    if [ "$CP_COMPLETE" = "true" ]; then
        ok "7c control positivo: con el MPQ presente y la orden que lo pide, Complete=true (la seleccion del motor pasa)"
    else
        mal "7c el control positivo quedo flojo: Complete=$CP_COMPLETE"
    fi
fi

echo
echo "== RESULTADO: $OK controles OK, $MAL con fallo"
[ "$MAL" -eq 0 ]
