# DC-001 — primer POC: Go corriendo en Dreamcast

Esta rama es la primera del port a consolas. El objetivo no es "el juego andando", es
**probar que la cadena existe y medirla**: compilar Go para el SH-4, arrancarlo en el
emulador, dibujar con el PVR, leer el mando y sacar números por texto. Todo lo de acá se
midió, no se dedujo.

Base: `master` (motor OpenDiablo2 en Go + Ebiten, x86-64, intacto).
El motor del host no se toca: lo nuevo vive en `console/` y `scripts/console/`.

---

## 1. Lo que ya funciona (medido)

| pieza | resultado | cómo se comprobó |
|---|---|---|
| toolchain cruzado | `sh-elf-gccgo (GCC) 15.1.0` + KOS 2.2.1 + libgodc | `sh-elf-gccgo --version` |
| Go compila para SH-4 | OK | `prog.elf`: `ELF 32-bit LSB, Renesas SH, statically linked` |
| enlace con KOS + libgodc | OK | `kos-cc` enlaza con `libgodcbegin` + `libkos` + `libgodc` |
| arranque en el emulador | OK | Flycast levanta el ELF con su loader y corre 20 s sin excepción |
| PVR dibuja | OK | captura de pantalla: fondo `(89,0,140)` = el color pedido, + cuadro blanco |
| consola serie (texto) | OK | `SERIAL-CHECK: rc=0 dev=scif` dentro del log del emulador |
| mando (maple) | pendiente de verificar | el POC imprime el estado del puerto 0 |
| imagen de disco arrancable | **OK con GDI** (en el emulador) | arranca y el POC corre desde el disco: misma salida serie y mismo asset pixel-perfect. Ver sección 9 |

Números del binario (POC mínimo, con `println` de Go y dibujo por PVR): ver
`evidencia/` de la corrida; el `prog.elf` con debug ronda los 2,7 MB y el
`1ST_READ.BIN` los 0,3 MB. **Ese es el número que manda para el presupuesto de 16 MB**
(el binario del motor completo se medirá en DC-001.5/DC-001.6).

## 2. Entorno

- Host: CachyOS x86-64, sesión **Wayland con KWin (KDE)**.
- Toolchain en `~/dreamcast` (un usuario, sin sudo):
  `sh-elf/`, `kos/`, `kos-ports/`, `libgodc/` desde el tarball
  `dreamcast-toolchain-gcc15.1.0-kos2.2.1-linux-x86_64-ubuntu22.tar.gz`.
- Emulador: `~/dreamcast/dl/flycast.AppImage` (Flycast 2.7).
- `~/dreamcast/venv`: python con `pycdlib` (para armar el ISO).

## 3. Dos arreglos obligatorios del toolchain

Los aplica `scripts/console/fix-toolchain.sh` (idempotente):

1. **`kos-ports/lib/*.a` son symlinks rotos** a `/opt/toolchains/dc/...` (la ruta de la
   máquina donde se compiló el tarball). Se repuntan a los archivos reales del tarball
   (`kos-ports/<port>/inst/lib/*.a`). Los 49 estaban rotos.
2. **Falta `perf.go` en `GO_SOURCES`** del `libgodc/kos/Makefile`: el `kos.gox` que viene
   compilado no trae `PerfCntrStart/PerfCntrCycles`, así que desde Go no se pueden leer
   los contadores de rendimiento del SH-4. Se agrega y se recompila el paquete.

## 4. Cosas que se aprendieron a golpes

- **gccgo resuelve `import "kos"` buscando `<dir>/kos.gox`**, no el directorio del paquete:
  el `-I` correcto es `$LIBGODC/kos`.
- **`libkos.a` ya trae los stubs en C** (`pvr_dr.c`, `plx_c_stub.c`, `vram_stub.c`).
  Compilarlos aparte da "multiple definition".
- **PLX (el modo inmediato de dibujo) vive en `kos-ports/libparallax.a`**, no en
  `libkallisti`. Y `kos_img_free` viene de `libkosutils` (addons de KOS).
- **`libtremor` y `libogg` definen los mismos símbolos**: hay que elegir uno (acá `libogg`).
- **Flycast arranca ELF directamente** (`reios_loadElf`), sin IP.BIN ni ISO. Ese es el
  ciclo rápido de desarrollo. El arranque por disco usa otro camino y hoy falla.
- **Flycast no abre un `.iso` suelto** (lo manda a libcdio y falla). Envolviéndolo en un
  `.cue` con `TRACK 01 MODE1/2048` sí lo lee `cue_parse`.
- **La consola serie del emulador se activa con una clave contraintuitiva**: la opción está
  declarada `Option<bool> SerialConsole("Debug.SerialConsoleEnabled")` **sin pasar sección**,
  así que en `emu.cfg` va bajo `[config]`, no bajo `[Debug]`:
  ```
  [config]
  Debug.SerialConsoleEnabled = yes
  Debug.SerialPTY = no
  ```
  Con `SerialPTY = yes` el texto se va a `/dev/pts/N` (y ahí no se lee). Y del lado de la
  consola hay que seleccionar el dispositivo: `dbgio_dev_select("scif")` — sin eso, KOS deja
  el handler en `NULL` y `printf`/`println` no salen a ninguna parte.
- **En KDE/Wayland `ffmpeg x11grab` da negro** (XWayland no tiene respaldo del root window):
  las capturas se hacen con `spectacle -b -n -f -o archivo.png`.
- El `environ.sh` de KOS referencia variables sin definir: con `set -u` activo el `source`
  aborta en silencio. En `scripts/console/env.sh` se apaga `-u` al entrar.

## 5. Puesta en marcha desde cero (4 comandos)

```bash
scripts/console/dl-toolchain.sh          # baja toolchain (471 MB) + Flycast a ~/dreamcast/dl
tar -xzf ~/dreamcast/dl/dreamcast-toolchain-*.tar.gz -C ~/dreamcast    # deja sh-elf/ kos/ kos-ports/ libgodc/
uv venv ~/dreamcast/venv && uv pip install --python ~/dreamcast/venv/bin/python pycdlib
scripts/console/fix-toolchain.sh         # los dos arreglos del tarball (symlinks + perf.go)
```

Y para activar la consola serie del emulador, una sola vez, en `~/.config/flycast/emu.cfg`
(fijarse que va bajo `[config]`, no bajo `[Debug]` — ver sección 4):

```ini
[config]
Debug.SerialConsoleEnabled = yes
Debug.SerialPTY = no
```

## 6. Ciclo de desarrollo

```
source scripts/console/env.sh                 # toolchain en el PATH
bash scripts/console/build-dc.sh console/dc/poc/goquad     # Go -> prog.elf (+ disc.cue)
scripts/console/run-flycast.sh /tmp/dcbuild/goquad/prog.elf 20 /tmp/dc-run
```

`run-flycast.sh` levanta el emulador, deja capturas escalonadas, mata el proceso al final y
**vuelca el texto de la consola serie**. Ese texto es la evidencia medible: el POC imprime
frames, ciclos y estado del mando.

Para el POC de C (sin Go) se usa `scripts/console/build-c.sh`, que es la prueba de que el
pipeline (toolchain + imagen + emulador) está sano cuando algo falla en el lado de Go.

## 7. POCs incluidos

| POC | qué prueba |
|---|---|
| `console/dc/poc/hello` | Go importando `kos` (arranque + PVR) |
| `console/dc/poc/goruntime` | Go pelado, sin tocar KOS: aísla el runtime de libgodc |
| `console/dc/poc/pipeline-c` | C puro: toolchain + imagen + PVR + serie (fondo morado + cuadro) |
| `console/dc/poc/serialcheck` | deja el resultado de `dbgio_dev_select("scif")` en el color de fondo |
| `console/dc/poc/goquad` | **POC principal**: Go + PVR + sprite + mando + ciclos por serie |

## 8. DC-001.4: un asset REAL de Diablo II en 8 bits (PAL8 + paleta por hardware)

Este es el paso que decide el port. El POC `console/dc/poc/pal8` toma un DC6 del MPQ
(el logo de Diablo II), lo hornea **en tiempo de build** a indices de 8 bits + una paleta de
256 colores ARGB1555, y lo dibuja con el PVR: textura paletizada + la paleta en hardware.

Medido en Flycast (evidencia en `console/dc/evidencia/`):

| que | numero |
|---|---|
| pixeles del arte identicos a la referencia del host | **4051/4051 (100%)**, diferencia maxima 7 por canal (lo que pierde ARGB1555 al guardar 5 bits) |
| pixeles transparentes con el fondo visible | **2085/2085 (100%)** (indice 0 = transparente, la convencion de Diablo II) |
| VRAM del asset | 6.136 bytes de indices (118x52 en 16 bits habrian sido 12.272) + la textura rellenada de 128x64 = 8.192 |
| orientacion | identidad 100% (la primera version salia espejada vertical: ver abajo) |
| rendimiento | 16.434-16.723 us/frame = 59-60 fps con el asset dibujado dos veces (1:1 y 4x) |

Por que importa: el arte de Diablo II ya **es** paletizado (8 bits + CLUT). Mantenerlo asi
cuesta la mitad de VRAM que convertirlo a 16 bits, y el PowerVR2 hace el blending por
hardware. La cuenta grande: los 898,7 MB de arte descomprimido, si fueran 16 bits, no entran
ni en el disco; en 8 bits son la mitad.

### Como se corre

```
scripts/console/build-dc.sh console/dc/poc/pal8        # hornea el asset y compila
scripts/console/run-flycast.sh /tmp/dcbuild/pal8/prog.elf 15 /tmp/dc-pal8
```

Y la verificacion no es a ojo: el horneador deja un PNG de referencia y
`scripts/console/compare-capture.py` recorta la captura del emulador en la posicion exacta
donde el POC dibujo el asset y compara pixel a pixel:

```
scripts/console/compare-capture.py --captura /tmp/dc-pal8/f10s.png \
    --referencia /tmp/dcbuild/pal8/gen/d2logo.png --pos 48 64 --fondo 89 0 140
```

La herramienta ubica sola la pantalla de la consola dentro de la captura buscando el color de
fondo (KDE abre el emulador donde quiere: la primera vez estaba en 2752,456 y despues en
2850,461) y ademas informa **en cual de las cuatro orientaciones** coincide la captura.

### Los datos del juego no entran al repo

`scripts/check_repo_hygiene.sh` prohibe `.dc6/.dt1/.dcc/.ds1/...` y archivos grandes. Por eso
el horneado corre **en tiempo de build**: `console/dc/bake` (herramienta del host, en Go) lee
el MPQ de la instalacion de Diablo II y escribe el `.h` en el directorio de build
(`/tmp/dcbuild/<poc>/gen`), que es descartable. El repo solo guarda el **horneador** y el
descriptor `console/dc/poc/<poc>/bake.conf`, nunca el arte.

### El volteo del sprite, y por que la comparacion no lo veia

La primera version salia **espejada verticalmente** (me lo cazo el usuario mirando la pantalla:
"el logo se ve volteado"), y la comparacion pixel a pixel daba 100% igual. El agujero de metodo:
la referencia del host sale del **mismo** horneado que la textura, asi que un espejado se
comparaba espejado contra espejado y daba perfecto. La herramienta ahora compara la captura
contra la referencia en las cuatro orientaciones y avisa cual gano:

```
  orientacion de la captura contra la referencia:
    identidad             17.8%
    espejada horizontal   12.8%
    espejada vertical    100.0%     <- asi se delata
```

Causa: `dc6.DecodeFrame()` arranca en `y = height-1` y va bajando porque el formato DC6 guarda
las lineas de abajo hacia arriba: lo que devuelve **ya esta en orden de despliegue** (es lo que
el motor mete en la superficie con `ImgIndexToRGBA` + `ReplacePixels`). Yo le daba una vuelta
extra "para el PVR" y eso era exactamente el espejado. Moraleja para el horneado: no
"arreglar" el orden de los pixeles que devuelve un decodificador del motor; usarlo tal cual.

### Tres piedras con las que me tope (van a volver a aparecer)

1. **El PVR (y KOS) exigen texturas de lado potencia de dos.** El logo es 118x52: compilar el
   poligono con ese ancho hace `pvr_poly_compile` abortar con
   `Invalid texture U size`. Solucion: subir la textura rellenada a 128x64 (con indice 0, que
   es transparente) y dibujar solo el arte visible usando las UV.
2. **Las UV de las texturas twiddled van NORMALIZADAS (0..1), no en pixeles.** Si se pasan
   en pixeles (118, 52) el PVR repite el patron y se ve cualquier cosa: en mi primera corrida
   salieron dos manchas naranjas. La referencia esta en el propio KOS
   (`examples/dreamcast/sound/ghettoplay-vorbis/bkg.c`: usa `u` de 0.0 a 1.0). Se agrega medio
   texel de recorte para no muestrear el relleno del borde.
3. **El binding de Go de libgodc no expone `pvr_txr_load_ex` ni `pvr_set_pal_entry`**, que son
   justo las dos piezas de la textura paletizada. Va por el puente de C (`csrc/cshim.c`), el
   mismo patron que la consola serie. Es ~90 lineas y es lo que hay que "portear" al backend
   definitivo.

### Piezas nuevas de esta etapa

| archivo | que hace |
|---|---|
| `console/dc/bake/bake.go` | horneador: MPQ + DC6 + PL2 -> indices de 8 bits + paleta ARGB1555 + PNG de referencia |
| `console/dc/poc/pal8/bake.conf` | que asset hornear (mpq, dc6, pal, frame, nombre) |
| `console/dc/poc/pal8/main.go` | POC en Go: sube y dibuja el asset (1:1 y 4x) |
| `console/dc/poc/pal8/csrc/cshim.c` | textura PAL8 + paleta por hardware + quad texturado |
| `scripts/console/bake-asset.sh` | corre el horneador buscando solo la carpeta de Diablo II |
| `scripts/console/compare-capture.py` | comparacion pixel a pixel contra la referencia |

## 9. Arrancar desde el disco (imagen de GD-ROM)

Al principio el `.cue`/`.iso` no arrancaba: a los ~2,5 s el SH-4 tiraba
`Fatal: SH4 exception when blocked`. La causa estaba en el propio emulador, no en la imagen:

```c
// core/reios/reios.cpp
descrambl = disc->type != GdRom;
...
if (descrambl) { bootFile->read(buf.data(), size, offset);
                 descrambl_buffer(buf.data(), dest, size); }   // DESCRAMBLA el binario
```

Un `.cue`/`.iso` se trata como CD-R (`type != GdRom`) y en ese camino reios **descrambla** el
binario de arranque antes de ejecutarlo. Nuestro `1ST_READ.BIN` va sin scramble (como el de
cualquier homebrew moderno), asi que quedaba descramblado = basura ejecutandose. Con un **GDI
el disco es GD-ROM, no hay descrambling** y ademas es el formato que lee el GDEMU.

`scripts/console/mkgdi.py` arma el GDI. Lo que espera el emulador (`core/imgread/gdi.cpp`):
al menos 3 pistas, pista 1 datos, pista 2 audio, pista 3 datos **empezando en FAD 45000**; el
sector de arranque se lee del FAD fijo **45150** (= pista 3 + 150), o sea el principio de la
pista 3, donde va el IP.BIN. Segundo detalle del emulador (`core/imgread/isofs.cpp`): el PVD
lo lee en `baseFad + 16`, pero **los directorios y los archivos los lee en `150 + lba` fijo**,
o sea la pista 1 (en un GD-ROM real ahi esta la zona de baja densidad, con el ISO y el binario
de arranque). Por eso las pistas 1 y 3 llevan el mismo ISO.

Resultado medido: el POC arranca desde el GDI, imprime lo mismo por la consola serie
(60 fps, VRAM, asset 118x52) y el asset sigue dando **4051/4051 pixeles identicos** y
**2085/2085 transparentes correctos** contra la referencia del host. El arranque por `.elf`
sigue siendo el ciclo rapido de desarrollo; el GDI es para validar el arranque real.

Pendiente para la consola: el GDEMU lee sectores crudos de **2352** y este GDI usa 2048.

## 10. DC-001.6: la medida del muro (la biblioteca estandar)

El motor son 2.713 archivos .go en 146 paquetes. Compilarlo para SH-4 con `sh-elf-gccgo` choca
contra tres cosas, en este orden:

```
d2common/d2util/debug_print.go:4:14: error: import file 'image' not found
d2common/d2util/debug_print.go:6:66: error: import file '.../d2common/d2util/assets' not found
d2common/d2util/debug_print.go:8:41: error: import file 'github.com/hajimehoshi/ebiten/v2' not found
```

1. **No hay biblioteca estandar para sh-elf.** libgodc no la trae (sus ejemplos solo importan
   `kos` y `unsafe`): hay que escribirla.
2. **Los paquetes del proyecto hay que compilarlos a `.gox` en orden de dependencias** (es como
   resuelve los imports gccgo). Es mecanico, no hay que escribir nada.
3. **Ebitengine no existe en la consola**: se reemplaza por el backend nuevo.

La medida del punto 1 (herramienta `scripts/console/audit-stdlib.py`, tabla completa en
`evidencia/stdlib-que-falta.txt`):

| que | numero |
|---|---|
| paquetes estandar que el motor usa **directo** | **51** |
| simbolos distintos de esos paquetes que se tocan | **337** (proxy) |
| los que mas archivos tocan | fmt 344, strings 260, errors 144, strconv 137, os 114, math 88, path/filepath 186, sort 115 |
| paquetes estandar **transitivos** (entran por Ebitengine) | 185 (no cuentan para el porteo) |
| archivos que tocan Ebiten (lo que hay que reemplazar) | **12**, en 4 paquetes: d2render/ebiten 6, d2audio/ebiten 3, d2input/ebiten 2, d2common/d2util 1 |

Las dos buenas noticias de la medicion: el aislamiento del backend aguanta (12 archivos, no
200) y 337 simbolos es un numero finito y acotado, no "hay que portear Go entero". Las malas:
`os` (35 simbolos) y `syscall`/`net` no tienen sentido tal cual en la consola y hay que
sustituirlos por un VFS minimo sobre el disco; y `reflect` + `sync` + `runtime` son los que
dependen del runtime de Go, que en gccgo para sh-elf hay que ver cuanto aguanta.

Triage para la fase siguiente (rama DC-002):

| bucket | paquetes | que implica |
|---|---|---|
| puro calculo (sin sistema operativo) | fmt, strings, strconv, sort, math, errors, bytes, unicode/utf8, container/heap, hash/fnv, encoding/binary, encoding/hex, math/rand, image/color | se escriben en Go y se compilan a .gox: es trabajo acotado y verificable contra el host |
| necesita sistema operativo (VFS minimo) | os, path/filepath, io, io/ioutil, bufio, log, time, flag, os/exec | se implementa contra el lector de disco de KOS; `os/exec` se tira |
| depende del runtime o no aplica | reflect, sync, sync/atomic, runtime, net, syscall, os/signal, context, encoding/json, regexp, compress/gzip, crypto/sha256, encoding/csv, image/png, image | a medir: cuanto se puede reemplazar por el runtime de gccgo y cuanto hay que reescribir |

## 11. Pendientes (con hipótesis)

1. **Imagen para la consola real (GDEMU)** — DC-001.2b, segunda mitad. El arranque por disco
   ya funciona en el emulador con `.gdi` (ver sección 9), pero el GDI que usa el emulador lleva
   sectores de 2048 y el GDEMU lee **sectores crudos de 2352** (12 de sync + 4 de cabecera +
   2048 de datos + 4 de EDC + 276 de ECC). Falta esa conversión (o armar el `.cdi` con una
   herramienta hecha para eso, tipo `mkdcdisc`) y probar en la consola.
2. **Mando**: confirmar que Flycast expone el control en el puerto 0 y que el POC lo lee.
3. **Textura paletizada real (PAL8 + CLUT)** desde un asset de Diablo II horneado
   (DC-001.4): el objetivo del port depende de dibujar el arte con paleta, no RGBA.
4. **Los tres números del go/no-go** (DC-001.5): tamaño del binario del motor,
   RAM libre con el motor cargado, ciclos por frame con ~300 sprites.
5. **Biblioteca estandar para SH-4** (DC-002): 51 paquetes y 337 simbolos medidos (seccion 10).
   Se empieza por los de puro calculo (fmt, strings, strconv, math, errors) para poder compilar
   y correr un paquete real del motor en la consola.
