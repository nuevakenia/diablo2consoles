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

### La imagen para la consola real (GDEMU)

El `.gdi` que uso para el ciclo rapido lleva sectores de 2048; el GDEMU (y la Dreamcast real)
leen sectores crudos de **2352** y ademas el binario de arranque va scramblado. De eso se encarga
**mkdcdisc**, que hace el camino entero de una: saca el binario del ELF, lo scrambla, arma el
IP.BIN con el `1ST_READ.BIN`, arma el ISO y lo empaqueta en un `.cdi` de DiscJuggler.

```
scripts/console/build-mkdcdisc.sh                      # compila la herramienta (en ~/dreamcast/deps)
scripts/console/mkcdi.sh <prog.elf> <salida.cdi>       # arma la imagen
```

Dos cosas que cuestan un rato si no se saben:

1. **mkdcdisc necesita libisofs** y no hay forma de instalarla sin root: se compila en
   `~/dreamcast/deps`. Tiene que ser **compartida** (`--disable-shared` NO): la estatica queda
   con `acl_set_file`/`acl_free` sin resolver al enlazar.
2. **mkdcdisc tiene un bug con las flags de Arch/CachyOS**: en `ElfSection` hace
   `m_Data.resize(shdr.sh_size); memcpy(&m_Data[0], ...)` y para la seccion NULL de todo ELF
   `sh_size` es 0, o sea `memcpy` de 0 bytes sobre un vector vacio. Con `_GLIBCXX_ASSERTIONS`
   (que Arch/CachyOS traen por defecto) eso aborta con
   `Assertion '__n < this->size()' failed`. Se compila con `-U_GLIBCXX_ASSERTIONS` y listo.
   Y para correrlo hace falta `LD_LIBRARY_PATH=~/dreamcast/deps/lib`.

Resultado medido: el `.cdi` de **2.486.473 bytes** (con `-N`, sin relleno; con relleno pasa de
700 MB sin ninguna ventaja) arranca en el emulador y el asset da otra vez **100% de coincidencia**
en las cuatro orientaciones. El IP.BIN sale con region **JUE** (arranca en cualquier consola) y
el nombre del disco es el que se le pasa con `-n`.

Para el GDEMU: copiar el `.cdi` a la SD en una carpeta numerada (el menu del GDEMU muestra una
entrada por carpeta: `01/`, `02/`, ...) y encender. El POC no necesita ningun archivo aparte.

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
4. **Los tres números del go/no-go** (DC-001.5): **medidos** — 1,39 MB de binario cargado, 10,00 MB
   libres de los 16 con el motor cargado, y 300 sprites paletizados a 60,0 fps gastando 683 µs de CPU
   por frame. El detalle y el veredicto están en la sección 25.
5. **Biblioteca estandar para SH-4** (DC-002): 51 paquetes y 337 simbolos medidos (seccion 10).
   Se empieza por los de puro calculo (fmt, strings, strconv, math, errors) para poder compilar
   y correr un paquete real del motor en la consola.

## 11. DC-002.2: la biblioteca estandar minima en la consola (hasta donde llega hoy)

La stdlib de la seccion 10 **compila para SH-4 y enlaza**: `scripts/console/build-std-gox.sh` la
deja en `$DC_ROOT/gox` en ocho archivos `.gox` (406 KB en total; el `.gox` de gccgo es a la vez la
cabecera de importacion y el objeto con el codigo, por eso tambien va al enlace). El POC
`console/dc/poc/stdlib` arma con eso un ELF de 3.086.004 bytes que Flycast arranca.

El POC no lleva respuestas escritas a mano: `scripts/console/gen-std-casos.sh` corre
`console/dc/std/cmd/gen-casos` **en el host**, donde esta la stdlib de verdad, y hornea la tabla de
casos (25.999 bytes) con el resultado que dio la biblioteca estandar real. La consola solo comprueba
que su implementacion da lo mismo.

**Estado: arranca, escribe por la consola serie y se cae en el despacho de interfaces.**

Lo que se midio con sondas (`console/dc/poc/sonda`, que imprime una marca antes de cada paso):

| sonda | que probaba | resultado |
| --- | --- | --- |
| float64 y float32, `unsafe` | calculo con punto flotante | anda |
| `strconv.Atoi("42")` | numero a texto | anda |
| `errors.New("x")` y `.Error()` | error creado y usado en su propio paquete | anda |
| `strconv.Atoi("abc")` y `.Error()` | error devuelto como interfaz | anda |
| `fmt.Sprintf("%v", err)` | asercion `interface{}` -> `error` dentro de fmt | **no**: imprime `%!v(tipo no soportado=...)` |
| `fmt.Errorf("ctx: %w", base)` | envolver un error | **no**: salta a la direccion 5 y muere |
| `strconv.FormatFloat(3.5,'f',2,64)` | flotante a texto | **no**: da `350` (en el host da `3.50`) |

El volcado del emulador dice donde: `Unhandled exception: PC 8c05e622` y esa direccion cae **dentro
del descriptor de tipo** `_type.._2_2main_0miError` (8c05e60c) — o sea, una llamada por tabla de
metodos fue a parar a los datos. Antes, el mismo programa habia muerto dentro de
`runtime.ifaceI2T2P` (la conversion de interfaces del runtime).

La causa **no** esta en nuestra stdlib ni en el SH-4: el control es la propia suite de libgodc, que
pasa en el emulador (`test_types.elf`: "PASS: type switch", "PASS: method interface", "PASS: empty
interface"). Pero sus pruebas son de **un solo archivo**: nunca cruzan paquetes por archivos `.gox`,
que es justo lo que hace una biblioteca estandar. El sospechoso es entonces el camino
inter-paquetes de gccgo/libgodc (descriptores de tipo e itabs de metodos entre objetos compilados
por separado).

### 11.1 La causa: la convencion de llamada de gccgo 15 no es la que libgodc espera

Se aislo con un caso minimo de **dos** paquetes (`console/dc/poc/xpkg`: uno define el tipo y su
metodo, el otro hace la asercion). Resultado:

| prueba | antes | despues del arreglo |
| --- | --- | --- |
| llamar al metodo del otro paquete | bien | bien |
| asercion `interface{}` -> `error` | **falla** (ok=false) | bien |
| asercion con puntero | **falla** | bien |
| asercion a tipo concreto | bien | bien |

Solo falla la asercion de interfaz vacia a interfaz con metodos (E2I) sobre un tipo de otro paquete.
Las sondas dentro del runtime dijeron que `implements_interface` da verdadero, los nombres de los
metodos coinciden y la memoria del itab se consigue: la funcion terminaba bien, pero **escribia el
resultado en basura**.

gccgo 15 devuelve los dos valores (interfaz + ok) **por memoria**: el llamador reserva 12 bytes y
pasa la direccion del buffer en **r2** (medido en el desensamblado de `main.main`). libgodc declaraba
esos ayudantes con punteros de salida (`Iface *, bool *`), que el compilador de C deja en **r7 y la
pila**: nunca coinciden. Queda comprobado compilando la misma llamada en C con las dos formas: la
forma "devolver por valor" usa r2, la forma con punteros de salida usa r7 y pila. De ahi tambien los
`Illegal instruction` con el PC dentro de un descriptor de tipo cuando corre la stdlib.

El arreglo es mecanico y esta en `scripts/console/fix-libgodc-abi.py` (reescribe la definicion en
`runtime/interface_dreamcast.c` y la declaracion en `runtime/runtime.h` para que `ifaceE2I2`
devuelva `{Iface, bool}` por valor), y despues `make -C <libgodc> libgodc.a`. Con eso las cuatro
pruebas del caso minimo pasan.

**Falta la misma cirugia en los otros ayudantes** que libgodc declara con punteros de salida y que
gccgo 15 tambien llama: `ifaceE2T2`, `ifaceI2T2`, `ifaceI2T2P`, `assertitab`, `ifaceT2Ip`. El metodo
es el mismo: mirar el desensamblado de una llamada real para saber la forma que espera gccgo.
Evidencia cruda en `console/dc/evidencia/dc-002.2b-abi-libgodc.txt`.

Control que no alcanzaba: la suite de libgodc pasa en el emulador, pero sus pruebas son de un solo
archivo y nunca hacen una asercion de interfaz sobre un tipo de otro paquete.

### 11.2 Los ayudantes que devuelven dos valores (DC-002.2c)

La misma cirugia se aplico a los otros tres que fallaban igual, cada uno verificado en el
desensamblado de una llamada real: `ifaceI2I2` (buffer de 12 bytes en r2), `ifaceI2T2P` (resultado
de 8 bytes en r0/r1) y `assertitab` (devolvia el Itab crudo cuando el llamador espera el puntero al
arreglo de metodos). Quedan mirados y sin tocar `ifaceE2T2`, `ifaceeq`, `ifaceT2Ip` e `ifaceE2T2P`,
que ya coincidian. Todo en `scripts/console/fix-libgodc-abi.py`.

Con eso **el autotest de la biblioteca estandar corre de punta a punta en la consola**: 560 casos,
404 coinciden y 156 no (la linea `STD-TEST: casos A bien B mal C` lleva tres numeros: total, los que
coinciden y los que no). De esos 156, 139 son de una sola forma: en la ruta de los flotantes falta el
punto decimal (`"%.2f"` de 1.5 da `150` en vez de `1.50`); los otros 17 incluyen un error del
generador de casos que ya se corrigio. Con las dos cosas arregladas quedan **7 de 560** (ver 11.3).

El defecto quedo acotado con una sonda (`console/dc/poc/concat`): con el mismo codigo copiado a mano
dentro del POC el resultado es correcto (`insertLocal(150, 2)` da `1.50`), y con una impresion
temporal dentro de `strconv.insertPoint` se ve que **`n` llega bien pero `prec` llega 0**, asi que la
funcion toma la rama sin parte decimal. Es una sola funcion en un solo paquete, no la convencion
`(uint64, int)` en general. Detalle y caminos para seguir en
`console/dc/evidencia/dc-002.2c-abi-ayudantes.txt`.

### 11.3 El punto decimal de los flotantes (DC-002.2d)

La causa esta en el objeto compilado, no en el codigo: `sh-elf-objdump -d strconv.gox` muestra que
gccgo 15.1 (sh-elf) compila `insertPoint` como una llamada pelada a `FormatUint` y tira todo el resto
del cuerpo (`if prec == 0`, el relleno y la insercion del punto), sin leer `prec`. Pasa igual con
`-O2` y con `-O0`, y la fuente que se compila es la correcta. La copia de la sonda no existe como
simbolo en el binario: se inlino. Inlineado el cuerpo esta bien; como funcion propia, el compilador lo
rompe. El llamador si pasa `prec` en r6 (visto en el desensamblado).

El arreglo fue escribir la insercion del punto **en el lugar**, dentro de `fixedDigits`, con una sola
salida y una bandera `exacto`. Los 139 casos de flotantes pasaron. Contando bien la linea del
autotest (total, bien, mal): 156 fallos → 17 (10 de ellos eran un error del generador de casos, ya
corregido) → **7 de 560**, que son diferencias reales de la biblioteca que el corpus del host no
cubria: `%c` con caracter no ASCII, `%t` sobre `[]uint8` (Go formatea elemento por elemento), `Atoi`
en los bordes de `int64` y un caso de `errors.Is`. Tarjeta DC-002.2e; el camino es llevarlos al
corpus de la prueba diferencial del host, verlos en rojo, arreglar y volver. Detalle en
`console/dc/evidencia/dc-002.2d-punto-flotantes.txt`.

### 11.4 Los casos que quedaban (DC-002.2e): 6 arreglados, 1 que no es de la stdlib

El autotest de la consola queda en **590 casos, 589 coinciden, 1 no** (los 590 son porque el corpus
sumo casos nuevos). Los 6 se arreglaron con el metodo de siempre: llevados al corpus del host, vistos
en rojo, arreglado el codigo y medido otra vez en la consola. El que falta no es de nuestra
biblioteca: es un defecto del runtime de libgodc (**DC-002.2f**).

| caso | donde estaba el defecto | arreglo |
|---|---|---|
| `%c` de `int64(min)` → byte nulo | nuestra `fmt`: `string(rune(n))` sin mirar el rango | `convert()` mira el valor sin signo y usa `utf8.RuneError` si se sale de `MaxRune`, como Go |
| `%t` (y `%d`, `%c`) sobre `[]byte` → aviso del arreglo entero | nuestra `fmt`: el verbo malo se armaba para el arreglo | `listaDeElementos`: el verbo cae sobre cada elemento (`[%!t(uint8=97) ...]`), con ancho/banderas/precision por elemento |
| `Atoi` en los bordes: `-1`/`0` con error | `strconv`: `bitSize 0` se trataba como 64 bits **y** al desbordar devolvia los topes de 64 bits | `bitSize 0` = `intSize` de la plataforma; al desbordar satura al tope **del ancho pedido** |
| lo mismo, pero en el **corpus** | `gen-casos` calculaba el esperado de `Atoi` con el `int` de 64 bits del host | el corpus usa el ancho de int de la consola (32) y lo dice |

Dos cosas que quedaron como parte del metodo (y que antes no existian):

- **`scripts/console/test-std-host.sh`**: la prueba diferencial del host corre en los **dos anchos**
  de `int` (`GOARCH=amd64` y `GOARCH=386`, que es el ancho del SH-4). Los dos defectos de `strconv` no
  se ven en el host de 64 bits: con una sola corrida el hueco queda abierto.
- **`scripts/console/mutar-std-host.sh`**: control sano verde y **5/5 mutantes cazados**. Un mutante
  por arreglo (el de `bitSize 0` se juzga en 386, porque en amd64 no cambia nada) y uno que exige que
  el corpus declare la semantica de int de la consola.

El que queda (**`errors.Is`**) es del runtime: `runtime/interface_dreamcast.c` (`implements_interface`)
compara el **nombre** del metodo y no la **firma**, asi que `*wrapErrors` —cuyo `Unwrap()` devuelve
`[]error`— "implementa" tambien `interface{ Unwrap() error }`: el itab queda con el metodo equivocado y
la llamada devuelve basura. Medido con la sonda `console/dc/poc/sonda-e` (E29-E32): las dos aserciones
dicen `ok`, el type switch elige el case equivocado y la comparacion de interfaces, en cambio, anda
bien (E28). Es el mismo linaje que DC-002.2b/2c, pero del otro lado: alla era la **convencion de
llamada**, aca el **chequeo de implementacion**. Ese caso queda **rojo y a la vista** en el autotest
hasta que se arregle el runtime: sacarlo del corpus seria esconder un defecto que el motor puede
pisar. Detalle y evidencia cruda en `console/dc/evidencia/dc-002.2e-casos-restantes.txt`.

### 11.5 El runtime tenia DOS defectos de interfaces (DC-002.2f)

El caso que quedaba en rojo (`errors.Is` no encontraba el error envuelto) resulto no ser uno sino dos
defectos del runtime, y el primero **tapaba** al segundo:

1. **`implements_interface` comparaba el nombre del metodo y no la firma**
   (`~/dreamcast/libgodc/runtime/interface_dreamcast.c`). Un tipo con `Unwrap() []error`
   "implementaba" `interface{ Unwrap() error }`: el itab quedaba armado con la funcion equivocada y la
   asercion decia `ok`. Arreglo: `method_types_equal` compara el tipo del metodo (`__typ` del imethod
   contra `__mtyp` del metodo, medido como el campo correcto: el header de `type_descriptors.h` lo
   documentaba **al reves** y ahora lo dice bien), por puntero y, si no coinciden, por forma.
2. **`runtime_ifaceeq` comparaba los PUNTEROS de itab.** El mismo par (interfaz, tipo concreto) puede
   tener dos tablas: la que arma el compilador para una conversion estatica y la que arma el runtime
   en el heap. Medido con la sonda `sonda-g`: `e2 == base` daba `false` con los dos datos apuntando al
   mismo `*errorString` (itabs `8c0991ec` y `8c0676e4`, con el mismo tipo en su palabra 0 y la misma
   funcion en la palabra 1). Arreglo: comparar el tipo concreto (lo que hace el runtime de Go), no la
   direccion de la tabla.

Por que el arreglo (1) solo no cambiaba nada: el envoltorio de `%w` con UN error es un `wrapError`
con la firma correcta, asi que la cadena se cortaba en (2). Y por que el corpus no cazaba (1) hasta
ahora: no habia ningun caso con **el mismo nombre de metodo y otra firma**. Ahora hay tres casos
nuevos (`poc/stdlib/main.go`, `correrErrores`), con el mismo esperado que da Go: la asercion de
`[]error` da `true`, la de `Unwrap() error` da `false`, y `Is` con dos envueltos encuentra el error.

Medido, todo en la consola:

| | antes | despues |
|---|---|---|
| corpus | 590 casos, 589 bien, 1 mal | **593 casos, 593 bien, 0 mal** |
| asercion con firma equivocada (sonda-f F13) | `ok=true` | `ok=false` |
| asercion con la firma justa (sonda-f F16) | `ok=true` | `ok=true` |
| `errors.Is(envuelto, base)` (sonda-g G9) | `false` | `true` |
| `e2 == base` (sonda-g G12) | `false` | `true` (los itabs siguen siendo dos: G15) |

El mutador `scripts/console/mutar-libgodc-2f.sh` saca cada arreglo por separado y exige que el corpus
se ponga rojo: **2/2 cazados**, con control sano `593/593` verde. Los dos arreglos se aplican con
`scripts/console/fix-libgodc-iface.py` (idempotente: deja el arbol pristino y vuelve a aplicar, primero
el arreglo de ABI y despues estos dos; dos corridas seguidas dan el mismo sha256). Sigue pendiente,
declarado en el codigo: `implements_interface` no compara el `pkgPath` de los metodos no exportados.

## 12. DC-002.3: el decodificador DC6 del motor en la consola (medido)

El primer paquete **real** del motor corriendo en la Dreamcast emulada. No hay codigo del motor
copiado al lado de la consola: el POC importa `d2common/d2fileformats/d2dc6` **tal como esta en el
repo** (con su `pkgpath` de verdad) y lo unico que aporta la consola son los bytes del DC6,
horneados del MPQ en el build porque en la Dreamcast no hay sistema de archivos.

```bash
scripts/console/build-motor-dc6.sh      # DC6 crudo + stdlib + paquetes del motor + ELF  (2,6 s)
scripts/console/compare-motor-dc6.sh    # el host y la consola, con diff                   (17,2 s)
```

Resultado: **31 lineas de resumen identicas, 30 frames** decodificados del logo de Battle.net, cada
uno con su tamano, offsets, cantidad de pixeles opacos y un hash FNV-1a de 32 bits de los indices
decodificados (`frame 0: 118x52 off=(-63,11) bloque=4618 pixeles=6136 opacos=4051 hash=2884d1c0`).

| pieza | lo que mide |
|---|---|
| `console/dc/bake-dc6/` | hornea el DC6 **crudo** del MPQ: el `.dc6` (para el host) y su var de Go (para la consola) |
| `console/dc/motorresumen/` | el resumen medible, en **un solo paquete**: lo importan los dos lados, asi que una diferencia entre los lados es de plataforma y no de dos copias del instrumento |
| `console/dc/motorcmp/` | el mismo codigo del motor corriendo en el host (x86-64) |
| `console/dc/poc/motor-dc6/` | el POC de la consola; `asset.conf` dice que DC6 hornear |
| `scripts/console/build-motor-gox.sh` | compila los paquetes del motor a `.gox` para SH-4 (saltea los `_test.go`: gccgo no filtra y piden `testing`) |

Lo que **faltaba en la stdlib minima** (medido compilando, no adivinado): `d2datautils` pide
`io.EOF`/`io.Reader`, `bytes.Buffer` y `log.Print`/`log.Panic`. Se agregaron los tres paquetes con lo
justo (`io` paso a compilarse despues de `errors` porque `io.EOF` es un `errors.New`), y quedaron
declarados sus limites: `bytes` no tiene las funciones de busqueda/comparacion y `log` no tiene fecha
ni prefijos (en la consola no hay reloj). (En ese momento la stdlib minima eran 8 paquetes: hoy son
16, ver la seccion 13.) El ELF del POC pesa
**3.259.236 bytes** (el autotest de la stdlib sola era 3.094.432: el motor + el resumen agregan
~165 KB). Detalle en `console/dc/evidencia/dc-002.3-motor-dc6.txt`.

## 13. DC-002.4: los paquetes de puro calculo (math y compañia)

Los paquetes del muro que no dependen de nada del sistema operativo, medidos con
`scripts/console/audit-stdlib.py` (cuantos archivos del motor los importan y que simbolos tocan):

| paquete | archivos | tiene que dar |
|---|---|---|
| `math` | 88 | exactas **bit a bit**; las trascendentales a 12 cifras |
| `math/rand` | 28 | la secuencia es propia: se prueba determinismo y rango |
| `encoding/binary` | 28 | igual a Go (escalares; las estructuras van en DC-002.5) |
| `hash/fnv` | 9 | igual a Go (mismo algoritmo y mismos primos) |
| `math/bits` | 2 | igual a Go |
| `encoding/hex`, `container/ring`, `hash` | 1 cada uno | igual a Go |

La biblioteca minima pasa de **8 a 16 paquetes** (`math`, `math/bits`, `math/rand`, `hash`,
`hash/fnv`, `encoding/binary`, `encoding/hex`, `container/ring`) y de 406 KB a **621 KB** de `.gox`.
Ademas se adelanto el `bytes.Reader` de DC-002.5, porque `encoding/binary.Read` lo necesita para
leer de un bloque en memoria.

```bash
scripts/console/build-std-gox.sh          # los 16 paquetes a .gox para SH-4
scripts/console/build-dc.sh console/dc/poc/stdlib
scripts/console/run-flycast.sh /tmp/dcbuild/stdlib/prog.elf 40 /tmp/dc-run-24f
```

Resultado medido en la consola: **1515 casos, 1515 bien, 0 mal** (eran 593). Los 922 nuevos son
math (426), math/bits (394), `hash/fnv`, `encoding/hex`, `encoding/binary` (incluido `Read` desde un
`bytes.Reader`), `container/ring`, las clases de `Inf`/`NaN`/cero con signo y las propiedades de
`math/rand`. Los resultados de math se comparan **por bits** (`math.Float64bits`), no por texto: asi
el corpus no depende de `fmt` y un ultimo bit distinto se ve. Detalle en
`console/dc/evidencia/dc-002.4-math-y-compania.txt`.

Instrumento propio: `go run ./cmd/medir-math` compara cada funcion contra la de Go y reporta el
error por funcion. Es lo que encontro los dos primeros defectos de abajo.

**Los tres defectos que aparecieron midiendo** (los dos primeros mios, el tercero lo cazo el corpus
de la consola):

1. `MaxFloat64` escrito en decimal (`1.7976931348623157e308`) **no es igual** a la constante de Go:
   Go compara las constantes de punto flotante de forma **exacta**, no redondeada, asi que el literal
   decimal tiene los mismos bits al pasar a `float64` pero no es el mismo numero racional (medido con
   un programa minimo: el mismo `%x` y `==` en false). Va en hexadecimal, que es exacto.
2. Cuatro polinomios escritos a mano mal: la serie del log **sin el termino 1** (el factor que
   multiplica), el seno y el coseno **sin la mitad de los terminos**, y el arcotangente **sin la
   alternancia de signos**. Errores de 1e-3 que el instrumento mostro en la primera corrida. Ahora
   los coeficientes se generan de los factoriales.
3. **El signo del cero**: `Floor`/`Ceil`/`Trunc`/`Round`/`RoundToEven` devolvian `float64(i)`, o sea
   `+0`, donde Go devuelve el cero **con el signo del argumento** (`Trunc(-0.1)` es `-0`). Lo cazo el
   corpus en su primera corrida completa: **7 casos en rojo**. El arreglo es `Copysign(float64(i), x)`
   y el mutante 6 lo vuelve a cazar.

**Costo de la aritmetica en la consola** (lo imprime el autotest, con el reloj de KOS):

| medicion | tiempo | por operacion |
|---|---|---|
| 1e6 multiplicacion+suma `float64` | 20 ms | ~20 ns |
| 1e6 multiplicacion+suma `float32` | 20 ms | ~20 ns |
| 1e5 `Sqrt` | 132 ms | ~1,3 us |
| 1e4 `Exp`+`Log` | 51 ms | ~2,5 us por funcion |
| 1e4 `Sin`+`Cos` | 70 ms | ~3,5 us por funcion |
| 1e4 `Atan2`+`Pow` | 71 ms | ~3,5 us por operacion |

O sea: la aritmetica basica de `float64` **no** es el problema (lo mismo que `float32`: la hace la
FPU), y lo caro son las trascendentales de este paquete, que son polinomios en Go. Dato para el
motor: no llamar `Sin`/`Cos`/`Sqrt` adentro de un bucle por frame.

Verificacion: `scripts/console/test-std-host.sh` **verde en amd64 y 386** (mismo criterio que la
consola mas pruebas de propiedad para lo no comparable), y `scripts/console/mutar-std-host.sh`
**11/11 cazados** con control sano. Los seis mutantes nuevos: el cero sin signo, `MaxFloat64` en
decimal, el cuadrante del coseno, la serie del log sin el termino constante, el primo de fnv y el
orden de bytes de `encoding/binary`.

**Lo que queda declarado**: `encoding/binary.Read` de **estructuras** necesita el descriptor del
tipo y devuelve un error explicito en vez de cualquier cosa (es la tarjeta DC-002.5); y de `math` no
estan `Log2`/`Log10`/`Modf`/`Frexp`/`Cbrt`/`Asin`/`Tan` porque el motor no toca esos simbolos (se
agregan si algun paquete del motor los pide).

## 14. DC-002.5: ordenar y recorrer (sort sin reflect, bytes y strings completos)

`sort` (86 archivos del motor), `bytes` y `strings` completos, mas `utf8.DecodeRune` sobre bytes y
`strconv.AppendInt`. La biblioteca minima pasa de 16 a 17 paquetes (674 KB de `.gox`).

**El problema interesante: `sort.Slice` sin `reflect`.** En Go, `Slice`/`SliceStable` arman su
intercambio con `reflect`, y portarlo es inviable: medido, `reflect` de Go depende de 43 paquetes, 32
de ellos del runtime del compilador, que la consola no tiene. La solucion es leer el descriptor de
tipo del argumento con `unsafe`, y eso se MIDIO antes de escribirlo con una sonda
(`console/dc/poc/sonda-h`): un slice pasado a `interface{}` tiene como dato un puntero a su cabecera
`{ptr, len, cap}`, y el descriptor del tipo del slice tiene el TAMANO DEL ELEMENTO en su campo 10
(offset 40), probado con 7 tipos incluidos tamanos que no son potencias de dos (4, 8, 12, 20).

Tres cosas que la sonda no habia mostrado y que salieron en el camino:

1. **El layout no es el mismo en los dos compiladores**: en gccgo el campo tiene el tamano del
   elemento; en gc ese campo es un puntero al descriptor del elemento. Y el offset depende del ancho
   de palabra (40 en la consola, 48 en el host de 64 bits). Solucion: el layout se detecta una vez al
   arrancar, midiendo con un tipo de 20 bytes (un tamano que no se confunde con ningun campo del
   descriptor: los que se repiten son 0, 1, 4, 8, 12, 16, 24 y los punteros). Si no lo reconoce,
   `panic` en vez de ordenar con el ancho equivocado (que corromperia memoria).
2. **El runtime de la consola no tiene los descriptores del tipo `unsafe.Pointer`**: una clausura o un
   campo con un `unsafe.Pointer` obliga a referenciar los simbolos `unsafe.Pointer..p`/`..d` y el link
   falla. Por eso la base del slice, los campos de la cabecera y los parametros de la funcion de
   intercambio son `uintptr`, y la conversion a puntero se hace solo al leer o escribir memoria.
3. **La estabilidad**: `Slice` y `Sort` de Go no son estables (dos ordenes distintos son validos), asi
   que el corpus usa claves distintas en esos casos y claves repetidas con la posicion original de
   cada elemento en los de `SliceStable`.

Medido en la consola: **1850 casos, 1850 bien, 0 mal** (eran 1515). El host sigue verde en amd64 y
386 y el mutador queda en **15/15** con control sano.

**Los tres defectos de `strconv` que destapo el corpus** (la forma `%g` no estaba cubierta: hueco de
la suite de DC-002.2):

1. `'g'` (la forma mas corta) pasaba a notacion cientifica con el umbral 21 en vez del 6 de Go: 1e10
   salia `10000000000` donde Go escribe `1e+10`.
2. La mantisa en notacion cientifica arrancaba con ceros de relleno: 1e-5 daba `0.00001e-05`.
3. Los denormales no salian: 5e-324 daba `0.00000000000000000`.

**Limite declarado**: la forma `'f'` con pocos decimales sobre valores mas chicos que el ultimo digito
redondea mal (1e-5 con 0 o 2 decimales da `1` y `0.01` donde Go da `0` y `0.00`). El motor no la usa y
queda registrado en `TestFormatFloatLimitesConocidos` (que falla si algun dia se arregla, para que no
quede una lista muerta).

Detalle completo en `console/dc/evidencia/dc-002.5-sort-bytes-strings.txt`. `flag` y `encoding/csv`
pasan a una tarjeta propia (son paquetes de formato, no de ordenar ni de recorrer).

## 15. DC-002.6: el sistema de archivos de la consola (os contra el VFS de KOS)

El motor usa `os` en 114 archivos y 35 simbolos. Este hito escribe ese paquete contra el VFS de KOS y
lo prueba en la consola, porque `os` no se puede probar en el host: su implementacion habla con el FS
de la Dreamcast. Evidencia completa: `evidencia/dc-002.6-vfs-os.txt`.

**Los archivos viajan adentro del ELF.** No hay disco al alcance: el romdisk se hornea en el binario.

    genromfs (3 archivos + manifiesto) -> romdisk.img (7168 bytes)
    bin2o                              -> romdisk.o (simbolos romdisk / romdisk_end)
    libgodc                            -> runtime/romdisk_init.c: "extern const uint8 romdisk[]
                                          __attribute__((weak))" + __kos_romdisk: si el programa
                                          enlaza romdisk.o, KOS monta ESA imagen en /rd al arrancar
    os.ReadFile("/rd/x")               -> fs_open + fs_read + fs_total

No hay que poner `KOS_INIT_ROMDISK` en el C: libgodc ya lo trae y ponerlo otra vez da
"multiple definition of __kos_romdisk" (medido).

**El romdisk no se puede listar.** Medido con la sonda-i (C puro) en Flycast, con el romdisk montado y
la ruta de todas las formas: `fs_open("/rd")`, `fs_open("/rd/")`, `fs_open("/")`, `fs_open("rd:/")` dan
INVALID; `stat("/rd")` da -1; `fs_readdir` da NULL. Los ARCHIVOS de adentro se abren y se leen bien.
El handler del FS ROM no contesta pedidos de directorio.

Solucion: `os.ReadDir` sale de un `.manifiesto` que hornea el MISMO build que arma el romdisk (nombre
TAB bytes TAB tipo por linea). Al generarlos juntos no se pueden desincronizar. El GD-ROM (`/cd`, por
ISO9660) queda para cuando se llegue: es otro FS y otro camino.

**Medido en la consola (Flycast, consola serie):**

    VFS: casos 18 bien 18 mal 0
    VFS: ReadDir /rd -> binario.bin grande.txt hola.txt  | tipos: 8 8 8
    VFS: RESULTADO OK - el sistema de archivos de la consola existe y lee

Los 18 casos: leer un texto, un binario (tamano Y suma de bytes), algo que no esta, Open/Size/Read en
trozos/Close, un archivo de 6000 bytes (mas grande que el buffer interno de 4096: varias vueltas),
ReadDir con tamanos, `Stat` + `IsNotExist`, `Existe`, `Getenv`, `Args` y la salida por
`os.Stdout`/`os.Stderr`.

**Mutadores** (`scripts/console/mutar-os-consola.sh`, 4/4 cazados, control sano y sha256 restaurado):
ReadFile que se queda con la primera vuelta, ReadDir sin tamanos, Stat que miente con el tamano e
IsNotExist siempre falso. Es el equivalente en la consola de `mutar-std-host.sh`: `os` no entra en el
corpus del host.

**Limites declarados en el codigo** (no fingidos): no se puede escribir (el romdisk es de solo lectura y
no hay otro FS enganchado: `MkdirAll`/`Remove`/`Rename`/`Create` devuelven error, y el motor las usa
para los saves); `Getenv` devuelve vacio siempre (es la llamada mas usada del motor, 461: CASCAL y los
paths de datos van a llegar vacios); `Args` es una lista fija; `FileInfo` no tiene `ModTime` (eso pide
`time`, que es la parte que sigue de esta tarjeta).

La barrera del hito sigue igual con la stdlib nueva: `compare-motor-dc6.sh` da 31 lineas identicas y 30
frames.

**Alcance:** esta tarjeta hizo `os` y el VFS. `time`, `path/filepath`, `bufio` y `log` son de formato y
de reloj, no del sistema de archivos, y pasan a la tarjeta dc-002.6b (recorte declarado).

## 16. DC-002.6b: path/filepath (los paths del motor)

`Join` 317 veces, `Base` 26, `Clean` 21, `Dir` 16, `Ext` 12: el paquete de paths es el mas usado de la
stdlib minima despues de `fmt` y `strings`. Es PURO TEXTO, asi que se puede probar de la manera mas
fuerte: comparando contra el `path/filepath` de Go, caso por caso, en el host. Evidencia completa:
`evidencia/dc-002.6b-filepath.txt`.

    Host:    prueba diferencial verde en amd64 y en 386
    Consola: VFS: casos 34 bien 34 mal 0   (18 del sistema de archivos + 16 de filepath)
    Mutador: 19/19 cazados, control sano

**Dos defectos mios que cazo la prueba diferencial:** `Dir` y `Split` sacaban las barras del final
antes de cortar, y Go corta en la ULTIMA barra (y `Split` no limpia nada). Medido: `Dir("/a/b/")` daba
`"a"` donde Go da `"a/b"`; `Split("a/b/")` daba `("a/", "b/")` donde Go da `("a/b/", "")`. Los dos
estan ahora en el mutador (16 y 17).

**Lo que queda afuera:** `Glob` (13) y `Walk` (2) necesitan listar directorios (o sea el paquete `os`),
y si este archivo importara `os` el paquete entero dejaria de poder probarse en el host; van con el
bucket de datos (DC-002.9). Tampoco estan `EvalSymlinks`, `Rel`, `VolumeName` ni `Match`: en la consola
no hay enlaces, ni directorio actual, ni volumenes.

**Dos cosas del mutador que valen solas:** (1) ahora "cazado" exige que la suite se ponga ROJA de
verdad — un `--- FAIL` o que el proceso se caiga (SIGSEGV/panic) — porque antes bastaba con que
`go test` saliera distinto de cero y eso daba por buenos mutantes que solo rompian la compilacion; (2)
al endurecerlo se vio que el mutante del ancho del elemento de `sort` SI se caza, pero reventando con
SIGSEGV en vez de fallar un test. Las dos formas de rojo son validas y quedaron aceptadas en el script.
Los patrones de los mutantes que llevan barras entre comillas van entre `$'...'` (con `'...'` el string
de bash se cierra y el mutante llega roto a python).

**Alcance:** esta tarjeta hizo `path/filepath`. `time`, `bufio` y `log` pasan a dc-002.6c (recorte
declarado).

## 17. DC-002.6c: bufio (el Scanner y el Reader sobre cualquier io.Reader)

`NewScanner` 18 veces y `NewReader` 10 en el motor: es como lee los textos de tablas. Evidencia completa
en `evidencia/dc-002.6c-bufio.txt`.

    Host:    prueba diferencial verde en amd64 y en 386
    Consola: VFS: casos 39 bien 39 mal 0   (34 + 5 de bufio, 84 KB de .gox)
    Mutador: 22/22 cazados, control sano

**Como se prueba un Scanner:** el contrato es el de Go (`Scan()` da false al final O por un error, y
despues hay que mirar `Err()`), asi que se compara token por token contra el Scanner de Go — pero al mio
se le da un Reader propio que sirve el archivo DE A POCO (de a 1, 3, 7 y 13 bytes). Con el archivo de a
un byte, cualquier error de "cuando corto el token" aparece enseguida.

**Dos defectos mios que cazo la prueba:** (1) `ScanLines` dejaba el `\r` de los finales de Windows (Go
tiene un `dropCR`: si el `\r` esta pegado al `\n`, se saca) — medido: Go daba `"con retorno"` y el mio
`"con retorno\r"`; (2) `ScanWords` cortaba la palabra sin esperar el blanco o el final, asi que con el
archivo de a un byte `"una linea"` salia como 8 tokens (`"u","n","a",...`) donde Go da 2. Los dos estan
en el mutador (20 y 21).

**Dos cosas del mutador que salieron de aca:** (1) un mutante puede COLGAR el test, no solo fallarlo o
hacerlo caer — el mutante "el Reader no rellena su buffer" deja al Scanner pidiendo datos para siempre,
asi que los tests del mutador ahora corren con `-timeout 25s` y un cuelgue cuenta como rojo; (2) y lo
mas importante: si el mutador muere a la mitad, el archivo queda MUTADO (paso: lo mato un timeout con el
mutante 22 aplicado y despues el control sano se colgaba sin que se entendiera por que). El script ahora
tiene un trap que restaura el archivo mutado cuando muere.

**Alcance:** esta tarjeta hizo `bufio`. `time` (el reloj de KOS) y `log` pasan a dc-002.6d (recorte
declarado).

## 18. DC-002.6d: time (el reloj de KOS) y log

Con esto se cierra el bucket de sistema operativo de DC-002: `os` (y su VFS), `path/filepath`, `bufio`,
`time` y `log`. Evidencia completa: `evidencia/dc-002.6d-time-log.txt`.

    Consola: VFS: casos 60 bien 60 mal 0
    Mutador de consola: 7/7 cazados, control sano, los tres archivos restaurados con sha256

**El reloj, medido en las cabeceras de KOS:** `rtc_unix_secs()` (los segundos Unix de la RTC del
hardware: el doc avisa que puede fallar y devolver 0), `timer_us_gettime64()` (microsegundos desde que
arranco la consola) y `thd_sleep(ms)` (el sleep de hilos, en milisegundos). `Now` combina los dos
primeros: avanza con el timer y arranca en la fecha de la RTC. **En la consola no hay zona horaria**: la
hora es la de la RTC, que va en UTC, y eso queda declarado (no se inventa un offset).

**Un defecto mio, cazado en la consola:** `Duration.String(1h)` daba `"1h0s"` y Go da `"1h0m0s"` (los
minutos van aunque sean 0 si ya hay horas). La fraccion tambien estaba mal al principio (dividia por una
escala: 1500ms daba `"1.5ms"`); se reescribio con el algoritmo de Go (`fmtFrac`/`fmtInt`): es los
ultimos `prec` digitos de los NANOSEGUNDOS, sin los ceros del final.

**`log`** ya existia de DC-002.3; se le agrego `Fatal`/`Fatalf`/`Fatalln` y `SetOutput`/`SetPrefix`. El
`Fatal` imprime y despues `os.Exit` (que en la consola se queda en un lazo): medido, es la ultima linea
que sale. El ENCABEZADO de Go (`2009/01/23 01:23:23 `) NO esta: necesita las conversiones de calendario,
que `time` todavia no tiene (la libc de KOS las da con `gmtime_r`). Queda declarado.

**Lo que NO hay en `time`, y esta dicho en el paquete:** `Format`/`Parse`, las conversiones de calendario
(`Year`/`Month`/`Day`/...) y `LoadLocation`/`UTC`: no hay zona horaria en la consola.

## 19. DC-002.7: sync, sync/atomic y runtime (goroutines y atomicos en la consola)

Con esto la consola tiene la capa de concurrencia. Evidencia completa:
`evidencia/dc-002.7-goroutines-sync.txt`.

    Consola: GORO casos 35 bien 35 mal 0
    Prueba en contra: sin candado y cediendo en medio, el contador pierde 600 de 800 incrementos
    Con sync.Mutex: 800 exacto. Con atomic.AddInt64: 800 exacto.
    Host: difftest de d2std/sync contra el sync de Go, verde en amd64 y 386.

**Primero se midio como cede una goroutine** (sonda en `console/dc/poc/goroutines`), y el resultado
manda el diseño: el scheduler de libgodc es **cooperativo sobre un hilo de KOS** (`thd_pass` +
`gopark`/`goready`, sin `thd_create`). Medido: cuatro goroutines sumando sin candado dan **8000
exacto** (no hay preempcion, ni el timer interrumpe), `thd_pass()` **no** releva goroutines, un canal
**con** buffer nunca se llena solo (no es punto de cesion) y uno **sin** buffer si lo es. Por eso el
candado es un canal con buffer 1: libre no cede, tomado se bloquea y ahi el scheduler releva.

**Lo que ya traia libgodc** (medido en su codigo): los atomicos de GCC para SH-4 en
`runtime/dreamcast_support.c`, de 1/2/4 bytes, todos con `irq_disable` — y **ninguno de 64 bits**. El
SH-4 no tiene atomicos de hardware y el runtime compila con `-matomic-model=soft-imask`: la unica
primitiva es apagar interrupciones, y alcanza porque la Dreamcast tiene un solo procesador. Los de 64
bits se agregaron en C. Tampoco hay semaforos ni locks en el runtime: por eso `sync` se escribe aca.

**Un hallazgo que rompio el enlace:** definir `GC` y `NumGoroutine` en el paquete `runtime` de la
consola da "multiple definition", porque gccgo nombra sus simbolos `runtime.GC` y libgodc ya los define.
Se declaran sin cuerpo (`//extern`) y un puente de C los resuelve contra el runtime: el motor puede
seguir llamando `runtime.GC()` como en Go.

**La prueba en contra** (la aceptacion): el mismo lazo de cuatro goroutines, sin candado y cediendo en
medio, pierde 600 de 800 incrementos; con `sync.Mutex` da 800 exacto.

**Cuatro defectos que cazo la prueba del host**, todos mios: la declaracion del `RWMutex` (el `RLock`
recursivo SI se traba: es un canal con buffer 1), `d2std/time` y `d2std/log` sin compilar en el host
desde DC-002.6d (nadie habia corrido la prueba despues), el canal del candado creado en el primer uso
(con goroutines paralelas del host dos crean canales distintos; en la consola no puede pasar) y el
`Unlock` de un candado libre, que en Go **aborta el proceso** y no se puede probar con `recover`.

**El mutador cazo un hallazgo que queda abierto:** de los cinco mutantes
(`scripts/console/mutar-goroutines.sh`) se cazan cuatro. El del CAS no, y no es culpa del mutador (se
comprobo a mano, con el patron por nombre de funcion y el log limpio): **el `CompareAndSwapInt32` del
POC no pasa por el shim `d2c_at_cas32`** — mutado para devolver siempre `false`, el POC sigue diciendo
`true`. `AddInt64` en cambio si pasa (ese mutante se caza). Hay que averiguar por donde va el CAS
(probablemente el runtime de libgodc ya trae su implementacion de los atomicos y el enlace la prefiere)
antes de dar por buena la capa de atomicos.

**Dos defectos del propio mutador** que salieron en el camino, y que importan porque un mutador que
miente es peor que no tenerlo: no borraba el ELF antes de compilar (si el build fallaba, corria el
binario viejo y daba VERDE) y no borraba el log de Flycast (el `RESULTADO OK` de la corrida anterior
hacia que el grep lo encontrara).

## 20. DC-002.5b: flag y encoding/csv (los paquetes de formato)

Los dos paquetes que quedaron afuera de DC-002.5 (no son de ordenar ni de recorrer) y que el motor
usa de verdad: `flag` en el arranque (`d2app/app.go`: String, Bool, Int, Usage, PrintDefaults, Parse) y
`encoding/csv` en las tablas de texto (8 usos de `NewReader`, con tabulador, filas de largo distinto y
comillas). Evidencia completa: `evidencia/dc-002.5b-flag-csv.txt`.

    Host:    prueba diferencial verde en amd64 Y 386   (flag 45/45, csv 462/462, Writer 18/18)
    Consola: STD-TEST: casos 2354 bien 2354 mal 0      (eran 1850: +504 = 42 de flag + 462 de csv)
    Mutador: 26/26 cazados (los 22 de antes + 4 nuevos), control sano y arbol restaurado
    Barrera: compare-motor-dc6.sh — 31 lineas identicas, 30 frames

Los dos paquetes son Go PURO (no hablan con KOS), asi que la prueba diferencial del host es el juez
mas fuerte que se les puede dar: la misma entrada por los dos lados y la salida TEXTO A TEXTO. En
`flag` se compara el valor de cada bandera, los argumentos que quedan, cuantas se pusieron, el texto
del ERROR y la salida de la ayuda; en `csv`, las filas, el texto del error (con linea y columna) y el
avance de la entrada. Los dos anchos de int importan: los casos que no entran en 32 bits viven en la
corrida de 386, que es el ancho de la consola.

**Los dos paquetes ya estaban escritos y no estaban atados** (una corrida anterior los dejo a medias):
`flag` no estaba en el `ORDEN` del build (la consola no lo tenia) y del `csv` el archivo decia —falso—
que no compilaba en SH-4. Medido: los dos compilan (198.020 y 95.676 bytes de `.gox`), el comentario
viejo se corrige, y la stdlib minima pasa a 37 archivos / 2.094.200 bytes.

**Tres defectos del propio test** (la suite tambien se equivoca, y hay que medirla a ella):

1. Las expectativas del texto de ayuda estaban escritas a mano y eran falsas. La salida real del flag
   de Go dice que las booleanas de UNA letra llevan la ayuda en la MISMA linea con un tabulador
   (`  -v\tShow version`) y que un default que es el valor CERO no se imprime (no hay `(default 0)`;
   si hay `(default 4)`). Ahora las expectativas son las medidas y se le exigen a los dos textos.
2. El caso "global con banderas" traia una bandera de nombre INEXISTENTE y los dos conjuntos de esa
   prueba son `ExitOnError`: el flag de Go terminaba el proceso con `os.Exit(2)` **adentro** de la
   prueba, asi que la suite se caia sin imprimir un solo `--- FAIL` (medido: exit status 2).
3. A nuestro `csv` se le daba el `strings.Reader` de la biblioteca estandar de Go. El `io.EOF` de Go
   **no es** el `io.EOF` de `d2std`, asi que el Reader nuestro veia un fin de entrada ajeno: 255 de
   429 combinaciones en rojo con `err=EOF` donde Go da `err=` (vacio). Ahora lee de `bytes.NewReader`
   de la consola, que es ademas el montaje fiel.

**Dos defectos reales de los paquetes**, los dos cazados por esta suite:

- `csv.ReadAll` devolvia el fin de la entrada como error (aqui la traduccion a `nil` existia, pero no
  habia nada que la probara hasta que el lector del test fue el de la consola).
- `bufio.Writer.Flush` **no detectaba el short write**: si la salida de abajo acepta una parte del
  buffer y no dice nada (`n < len`, `err == nil`), Go marca `io.ErrShortWrite` y guarda el error; el
  nuestro daba por buena la escritura, tiraba los bytes que no salieron y **perdia el error** (medido:
  `csv.Writer.Error()` daba `nil` donde el de Go da error). Se copio la semantica de Go y se agrego
  `io.ErrShortWrite` al paquete `io`.

**Un hueco de la suite del hito anterior**: `test-std-host.sh` estaba en ROJO desde el commit
`4bc01457` (DC-002.8) con "missing function body" de `compress/zlib` y `compress/gzip`, que son
puentes `//extern` a la zlib de KOS y no compilan en el host (el mismo caso de `os`/`time`/`log`).
Nadie habia corrido esa prueba desde entonces. Los dos entran ahora a la lista de excluidos.

**Y un defecto del corpus que cazo la propia consola**: el caso `-n=123456789012` no puede estar en el
corpus. El corpus se calcula con el `int` de 64 bits del host y en la consola el `int` es de 32:
medido, la consola dio `value out of range` y quedo 1 caso en rojo (2355 casos, 1 mal). El difftest
tampoco lo puede cazar (compara Go contra nosotros con el MISMO ancho): vive solo en la corrida de 386,
que es el ancho de la consola.

**Limites declarados**: `flag.Duration`/`DurationVar` no estan (pedirian `time.ParseDuration` y el
motor las usa solo en `cmd/od2server` y `cmd/od2mpbot`); `flag` no importa `os` ni `time` —los
argumentos y el terminar el proceso son variables que engancha el arranque, y eso es lo que le permite
compararse contra el flag de Go en el host—; el Writer de `csv` esta y se prueba en el host (18 casos)
pero no va al corpus de la consola porque el motor solo LEE tablas; y `TrimLeadingSpace` no saltea los
espacios no-ASCII (no hay tablas de Unicode), declarado en la cabecera del paquete con su prueba.

## 21. DC-002.8: el motor compilando por capas — 92 de 142, y los bloqueos reales

La tarjeta no pide escribir paquetes: pide MEDIR cuanto del motor compila para SH-4 y con que se traba
cada paquete que no compila. La herramienta es `scripts/console/build-motor-all.sh`: intenta compilar
los 142 paquetes del motor (los directorios con `.go`, sin los de prueba) y reintenta los que fallaron
en capas, porque lo mas probable es que fallaran por una dependencia que si compilo en la pasada
anterior. Se corta cuando una capa no desbloquea nada. Evidencia completa:
`evidencia/dc-002.8-motor-por-capas.txt`.

    ANTES de esta tarjeta:  90 de 142   (52 bloqueados)
    DESPUES:                93 de 142   (capas: 57 -> 75 -> 87 -> 91 -> 93 -> 93; 49 bloqueados)

Y lo que importa no es el numero: es QUE falta. Leyendo los 49 bloqueos por causa (no por paquete) sale
que casi todos son **cuatro cosas**, y tres de ellas no son nuestra stdlib:

  - 9 paquetes por `encoding/json` (y por ahi arrastran los `d2config`, `d2replay`, `d2gamepad`, ...):
    la decision ya esta tomada y medida (commit `48912a00`): NO se escribe json.
  - `net` (2), `regexp` (1), `os/exec` (1), `embed` (1): stdlib de Go que no tiene sentido portar.
  - `github.com/robertkrimen/otto` (d2script: un interprete de JavaScript entero) y `restruct` (d2pl2).
  - y el resto son paquetes INTERNOS que importan a los de arriba (cascada).

Lo unico que SI era nuestro y bloqueaba paquetes del motor se saco de los errores exactos de
compilacion, se probo contra Go y se agrego:

    encoding/binary.AppendUint16 / AppendUint32 / AppendUint64   -> desbloqueo d2networking/d2mp
    bytes.Buffer.Read (Buffer es un io.Reader)                   -> desbloqueo d2core/d2term
    image.Uniform                                                -> d2common/d2display
    image.Rectangle.Union / Inset / In                           -> d2common/d2display
    ebiten: 37 teclas que el motor nombra (4 son alias de las Arrow, como en Ebitengine)

Cada simbolo nace de un error de compilacion con archivo y linea (`d2mp/events.go:89`,
`terminal_logger.go:22`, `shotcheck.go:34`), no de una lista de deseos. `AppendByte` no se agrego
porque **Go no lo tiene en `ByteOrder`**: lo cazo la prueba diferencial contra Go en el primer intento.

**Los tres defectos que aparecieron al medir el avance** (y que son de la stdlib, no del motor):

- `color.RGBA.RGBA()` **premultiplicaba por el alfa**: era una copia de `NRGBA`. En Go, `RGBA` es un
  color YA premultiplicado y su `RGBA()` solo escala a 16 bits. Medido: `RGBA{1,2,3,128}` daba
  `(129,258,387,32896)` donde Go da `(257,514,771,32896)`, y con alfa 0 daba negro donde Go da el
  color. Se veia en el dibujado: `Over` con `RGBA{0x11,0x22,0x33,0x80}` sobre `0xc0` daba
  `(50,59,61)` donde Go da `(71,82,93)`. `color.RGBA64.RGBA()` tenia el mismo defecto, en 16 bits.
  Lo cazo la prueba de `image.Uniform` **dibujado** (el dibujado de Go usa `RGBA()`), no una prueba de
  colores: la prueba de los ocho colores que faltaba ahora esta (`TestColorTodosContraGo`) y es la que
  lo habria cazado antes.
- `image/draw` (y `image`) importaban `"image"` y `"image/color"` **de Go** en vez de `"d2std/..."`: en
  la consola compilaba igual (el build reescribe `d2std/` -> `""`), pero en el HOST traia los tipos de
  Go, asi que la prueba diferencial del host **no estaba probando lo nuestro**. Es exactamente lo que
  dice la cabecera de `build-std-gox.sh` que hay que hacer, y estos dos archivos no lo cumplian.
- `Rectangle.Inset` **no era el de Go**: medido, cuando el inset invertiria el lado (`Min+n > Max-n`)
  Go COLAPSA ese eje en el punto medio (`(Min+Max)/2`, division truncada hacia cero). Nuestro `Inset`
  solo sumaba y restaba y dejaba el rectangulo invertido: `Inset((-3,-3)-(0,0), 2)` da `(-1,-1)-(-1,-1)`
  en Go (el medio de `(-3,0)` es `-1`, no `-2`) y nosotros dabamos `(-1,-1)-(-2,-2)`.
- `Rectangle.In` (la contencion de RECTANGULOS, que no es el `In` de `Point` que ya estaba) **no
  existia**: lo pide `d2display/shotcheck.go:53`. Medido contra Go: un rectangulo VACIO esta dentro de
  cualquiera (incluso del vacio) y uno no vacio no esta dentro de uno vacio. Con eso, `d2display` cerro y
  el motor paso de 92 a 93.

**Y dos defectos de la propia suite**, los dos de la clase que mas envenena: los que hacen parecer que
todo esta probado.

- `cmd/gen-casos` emitia el resumen de flag/csv terminando con `\\n` (DOS caracteres) en vez de un
  salto de linea, asi que ese comentario se **comia la linea siguiente**. Estaba latente (se comia otro
  comentario) y se destapo al insertar el bloque de DC-002.8 justo despues: el `type casoAppend struct`
  quedaba comentado y el compilador de la consola se quejaba con "expected declaration" lejos de la
  causa.
- `TestWaitGroupContraGo` estaba **flaky** y ademas **no comparaba nada**: el contador lo incrementaban
  DIEZ goroutines con `n++` sin sincronizar, y la funcion devolvia `n, n` (el mismo contador dos veces,
  con el parametro `usarConsola` ignorado por un `_ = usarConsola`). Medido:
  `-count=30` daba fallos con "Go 29, consola 29" (2 de 30). Un test asi no solo hace fallar la suite al
  azar: hace que el MUTADOR cace mutantes por casualidad — el mutante del `RGBA` "cazo" por esto la
  primera vez. Arreglado: un contador por lado, con `sync/atomic`, y 15 en cada uno. Despues del
  arreglo: `-run TestWaitGroupContraGo -count=50` verde, y la suite entera `-count=20` verde en amd64 y
  en 386 (3 rondas).

Cierre medido de la suite y del corpus con todo esto adentro:

    host:    prueba diferencial verde en amd64 Y 386 (y -count=20 verde en los dos)
    consola: STD-TEST: casos 2746 bien 2746 mal 0   (eran 2354: +392)
    mutador: 32/32 cazados, control sano y arbol restaurado
    barrera: compare-motor-dc6.sh — 31 lineas identicas, 30 frames

**Los bloqueos que quedan, declarados** (van como paso propio, ninguno se deja en silencio):

- `os` y `io/fs` tienen TIPOS DISTINTOS para lo mismo (`FileInfo`, `DirEntry`, `FileMode`) y en Go son
  el MISMO tipo. Sintoma: `d2mpq/mpq.go:205` "cannot use `[]os.FileInfo` as `[]fs.FileInfo`". El
  arreglo es alinear `io/fs` como dueno y que `os` reuse por alias (~150 lineas): medido a mitad de
  camino, el motor **baja a 40 de 142** porque `os` deja de compilar. Por eso va como paso propio y no
  como parche de pasada.
- `io/ioutil`, `os`, `compress/zlib` y `compress/gzip` tienen el mismo defecto de import que tenian
  `image` y `image/draw` (`"io"`, `"os"`, `"sort"` en vez de `"d2std/..."`). En zlib/gzip da igual (son
  `//extern` a KOS y no compilan en el host), pero en `ioutil` y `os` no: la prueba del host de `ioutil`
  escribe con el `os` de Go. Se arregla junto con lo anterior, porque `os` es el que no compila en el
  host.
- El control de costo del POC (`bits64(suma)&1`, "no tiene que ser 0") **siempre da 0**: un control que
  no discrimina no es un control.
- Un cuelgue **intermitente** de la suite en 386: medido 1 vez en 80 corridas (una corrida de
  `-count=20` en 386 se quedo en el `-timeout` de 300 s; las otras tres dieron verde en 3,2 s). Es de la
  familia de los tests de goroutines y canales de DC-002.7, que es lo que el propio archivo advierte que
  puede colgarse con el scheduler del host.

**Y dos defectos del propio mutador**, que salieron al revisar los resultados (un mutador que miente es
peor que no tenerlo):

- El test del mutador corria con `-timeout 25s`: con la maquina cargada el rojo podia ser por TIEMPO y
  no por un aserto, y el mutante se contaba como cazado igual. Ahora es `-timeout 60s` y el script
  **imprime el motivo** cuando el rojo es por tiempo o caida, para que se vea cual de los dos es.
- El mutante de `Uniform.Opaque` estaba escrito `return true`, que **ni compila** (la variable `a` queda
  sin usar): el script lo reportaba como "LA MUTACION NO COMPILA (no prueba nada de la suite)" y eso
  hundia el conteo. Se corrigio a `return a != 0xffff`.

## 22. DC-002.8a: los tipos de `os` alineados con `io/fs`, y los imports de la stdlib minima

La tarjeta nacia de dos pendientes de DC-002.8. Medido antes de tocar nada, los dos habia que
reescribirlos:

- **A ("os y io/fs tienen tipos distintos") ya NO era cierto**: el alias (`os.FileInfo = fs.FileInfo`,
  etc.) esta hecho y commiteado en `95e0ef74`. Lo que quedaba era el COMENTARIO "PENDIENTE MEDIDO" en
  `build-std-gox.sh` —el propio mensaje del commit decia haberlo borrado: "El pendiente que estaba
  anotado en build-std-gox.sh queda hecho; se borra de ahi"— y, sobre todo, un defecto grande que ese
  commit dejo y nadie vio.
- **B ("4 archivos importan la biblioteca de Go") era cierto y mas grande**: eran **8 archivos con 12
  imports** (`bytes`, `compress/gzip`, `compress/zlib`, `container/heap`, `io/fs`, `io/ioutil`, `math/cmplx`,
  `os`, `strings`).

**El defecto grande (y por que nadie lo veia): el corpus de `os` en la consola NO COMPILABA.**

    $ bash scripts/console/build-dc.sh console/dc/poc/vfs
    console/dc/poc/vfs/main.go:79:41: error: reference to undefined field or method 'Size'
    console/dc/poc/vfs/main.go:191:48: error: reference to undefined field or method 'TipoCrudo'

El POC del VFS (60 casos de DC-002.6d) usaba un `os.DirEntry` propio mas ancho, con `Size()` y
`TipoCrudo()`, que el alias elimino al convertirlo en `fs.DirEntry`. Desde `95e0ef74` el corpus de `os`
estaba muerto y **las dos cosas verdes que se miraban no podian verlo**: la prueba del host excluye `os`
(no compila contra KOS) y la barrera del motor usa `ReadFile`, no `ReadDir`. Aparecio por el camino mas
directo: ir a correr el corpus de `os`.

    ANTES                                         DESPUES
    checar-imports-d2std.sh (nuevo)  12 viol./8   0 (VERDE)
    test-std-host.sh (amd64 + 386)   verde        verde + "excluidos que NO compilan: 9 de 9"
    .gox de la consola               --           tamanos IDENTICOS byte a byte
    build-motor-all.sh               93/142       93/142 (los MISMOS 142, mismo estado: diff vacio)
    poc/vfs (el corpus de os)        NO COMPILA   77 casos, 77 bien, 0 mal (eran 60)
    mutar-os-consola.sh              no se podia  10/10 cazados (eran 7)
    mutar-tipos-os.sh (nuevo)        no existia   4/4 cazados
    mutar-std-host.sh                32/32        32/32
    compare-motor-dc6.sh             31 lineas    31 lineas identicas, 30 frames
    poc/stdlib                       2746         2746 casos, 2746 bien, 0 mal

**Por que los imports hay que escribirlos como `"d2std/..."`** (y por que el defecto era invisible): en
la consola, el nombre desnudo `"io"` resuelve a NUESTRO `io.gox` porque el build reescribe `"d2std/` ->
`"`. En el host, el mismo texto resuelve a la biblioteca de Go: la prueba diferencial compilaba `ioutil`
contra el `os` de Go **y no lo decia**. Con el prefijo explicito el nombre ya no puede significar dos
cosas. El ajuste lo exige ahora `scripts/console/checar-imports-d2std.sh` (37 paquetes, 41 archivos;
`unsafe` es lo unico de afuera que se permite, `difftest/` y `cmd/` quedan afuera porque son las
herramientas del host y comparar contra la biblioteca real es su trabajo).

Consecuencia declarada: `io/fs` (por `d2std/time`) y `io/ioutil` (por `d2std/os`) **dejan de compilar en
el host**. No se pierde ningun test (los dos solo compilaban: no tienen archivos de prueba); el corpus
de `ioutil` pasa a la consola, sobre NUESTRO `os`, y el limite queda **declarado y verificado** en
`test-std-host.sh`, que ahora tiene dos mitades: los no excluidos corren la prueba diferencial y los
excluidos tienen que NO compilar (el comentario lo prometia desde DC-002.5b; el codigo solo los
excluia). Con un paquete que si compila metido a mano en la lista, la prueba se pone roja sola.

**Los defectos que aparecieron midiendo** (los tres ultimos no estaban en la tarjeta; salieron del
camino):

- `os.ReadDir` **no ordenaba por nombre** y el de Go si (`os.ReadDir`: *sorted by filename*). Medido con
  un fixture nuevo, `/rd/desorden/.manifiesto`, escrito a proposito como `zulu, alfa, mike`: nuestro
  `ReadDir` daba `"zulu.txt alfa.txt mike.txt "` donde Go da `"alfa.txt mike.txt zulu.txt "`. Antes de
  ese fixture el orden **no estaba probado**: el manifiesto de `/rd` sale alfabetico de casualidad (asi
  lo expande el glob del build), asi que un mutante "sin ordenar" quedaba verde. Se ordena a mano en
  `os.go` (sin importar `sort`: `os` es el paquete mas bajo y no importa nada que no sea imprescindible,
  la misma razon por la que existe `partirTab`).
- El **manifiesto no distinguia directorios**: `build-dc.sh` escribia "archivo" para todo, asi que la
  rama `campos[2] == "directorio"` de `os.ReadDir` era **codigo muerto** y `DirEntry.IsDir()` /
  `Type() == fs.ModeDir` no tenian ningun caso. Ahora marca "directorio" (tamano 0: el de un directorio
  lo pone el sistema de archivos que lo hospeda —medido 4096 en `/tmp`— y no es un dato del contenido) y
  el romdisk del POC tiene ese subdirectorio: `/rd` paso de 3 a 4 entradas.
- El **mutador de la consola estaba roto en silencio**: su mutante 2 buscaba `DirEntry{nombre: ...}` y el
  tipo concreto se llama `dirEntryOS` desde `95e0ef74`, asi que se reportaba "NO SE PUDO APLICAR (se
  saltea)" y el conteo final se caia. Arreglado.
- `os.TipoCrudo()` era **inalcanzable**: `os.DirEntry` ES `fs.DirEntry`, esa interfaz no lo declara, y
  el tipo concreto no se puede nombrar desde afuera. Se fue; quedan `Type()` e `IsDir()`, que son los de
  Go. La seccion 4k/4l del POC del VFS ahora mide `ioutil` sobre el `os` de la consola (incluida la
  asignacion exacta del motor, `var files []fs.FileInfo; files, err = ioutil.ReadDir(dir)` de
  `d2mpq/mpq.go:204`, que es el sintoma del tablero) y las identidades de tipo que exige el compilador.

**Los mutantes nuevos** de la alineacion viven en `scripts/console/mutar-tipos-os.sh` (solo compilacion,
sin Flycast) y los tres de ioutil/orden en `scripts/console/mutar-os-consola.sh` (8, 9 y 10). El
control del mutador de tipos exige, sin mutar, que compilen los tres: la stdlib, `d2mpq` y el POC del
VFS. Y el mutante mas útil es el que reproduce el estado **"a mitad de camino"** que el tablero
advierte: con `os.FileInfo` propio y `os` parcheado para que compile, el que se cae es el motor, con el
error exacto de DC-002.8:

    d2common/d2fileformats/d2mpq/mpq.go:205:28: error: incompatible types in assignment
    (cannot use type []os.FileInfo as type []fs.FileInfo)

Nota de arbol: al medir la barrera, el WIP de DC-002.9 que quedo sin commitear (motorresumen,
motorcmp, bake-dc6 y los dos scripts del motor) **no compila** (`motorresumen` usa
`d2util.ImgIndexToRGBA` y `build-motor-gox.sh` todavia no incluye `d2common/d2util` en su lista de
paquetes). La barrera se midio apartando ese WIP con `git stash` (el pipeline commiteado + los cambios
de esta tarjeta) y despues se restauro; el WIP queda declarado en la tarjeta `t_1677ebe0`, que es la que
lo tiene que terminar. Evidencia completa: `evidencia/dc-002.8a-os-iofs-imports.txt`.

## 23. DC-002.9: el motor pintando en la consola (DC6 + paleta del MPQ por el PVR)

El paso visual del port: que el **codigo del motor** —no un horneador nuestro— decodifique un DC6 con
su paleta y lo dibuje el PVR de la Dreamcast. El DC6 y el `pal.dat` entran **crudos** del MPQ (140.740
y 768 bytes), los parsea `d2dc6.Load` + `DecodeFrame`, `d2dat.Load` y `d2util.ImgIndexToRGBA`
compilados con `sh-elf-gccgo`, y lo unico que aporta la consola es el PVR: textura de 1 byte por pixel
(PAL8) con la paleta del motor en el banco 0 del hardware.

Se mide **dos veces y por caminos distintos**, porque ninguna de las dos alcanza sola:

- **El resumen de texto**: el POC imprime los indices decodificados y los RGBA finales de cada frame con
  su hash FNV-1a. Medido: **61 lineas identicas** entre x86-64 y SH-4 (la linea de version + 30 frames
  por indices + 30 frames por RGBA). La de indices ya la media DC-002.3; la de RGBA es lo nuevo y dice
  que el COLOR salio igual: la paleta se decodifico igual en las dos plataformas y el indice 0 quedo
  transparente en las dos. (Frame 0: indices `hash=2884d1c0`, RGBA `hash=85c68d20`.)
- **La captura**: el comparador del host deja la referencia del frame 0 en PNG (`motorcmp -png`, el
  mismo camino `DecodeFrame` + `ImgIndexToRGBA`) y la captura del emulador se compara **pixel a pixel**
  contra ella, incluida la transparencia y en las cuatro orientaciones. Medido: **4051/4051 pixeles de
  arte (100%)**, diferencia maxima **7 por canal** (lo que pierde ARGB1555), **2085/2085 pixeles
  transparentes** con el fondo violeta a la vista y **identidad 100%** contra 17,5% espejada
  horizontal, 16,7% espejada vertical y 11,9% rotada 180 (o sea: no hay volteo). La captura recortada
  esta en `evidencia/dc-002.9-consola.png`.

**Tres huecos que aparecieron en el camino y quedaron cerrados:**

- `build-motor-gox.sh` tenia 3 paquetes y el color necesita 11. La lista se armo **midiendo** la
  clausura con `go list -deps` (no adivinando): `d2enum`, `d2math`, `d2math/d2vector`,
  `d2clientconnectiontype`, `d2interface`, `d2util/assets`, `d2util`, `d2datautils`, `d2dc6`, `d2dat` y
  `motorresumen`. El ELF quedo de 6.899.820 bytes.
- Los **paquetes de terceros no iban al enlace**: `build-std-gox.sh` los compila y los deja en el `-I`,
  pero no los anota en `orden.txt`, y eso alcanzaba mientras nadie los usara. Medido: `d2util` llama a
  ebiten (`debug_print.go`) y `d2util/assets` a `x/image/bmp` (`zipbmp.go`); sin esos `.gox` en el
  enlace el linker se cae por simbolos UNDEFINED. Ahora se anotan (y una sola vez: correr el script dos
  veces le pasaba el mismo objeto dos veces al linker).
- **La captura no salia**, y este es el defecto que vale contar. Las primeras cuatro capturas salieron
  con **cero pixeles del POC**: en la pantalla estaba Diablo II. Medido en el apilado de KWin:
  `gamescope | Diablo II | fullScreen=true activa=true` arriba de `flycast | Flycast - prog |
  keepAbove=true`; el emulador abre su ventana en la pantalla de 1024x768 donde el juego corre en
  fullscreen, y el juego **activo** gana aunque la ventana del emulador tenga keep-above. Lo que se
  probo, medido: `keepAbove` se aplica pero no alcanza; activar la ventana funciona pero le roba el foco
  al juego; `SDL_VIDEO_WINDOW_POS` no tiene efecto; `w.move()` no existe, `w.output` es de solo lectura
  y `Qt` no existe en el motor de scripts de KWin 6; **`w.frameGeometry = {x,y,width,height}` si** (la
  ventana paso de 2830,385 a 210,210 con su mismo tamano, asi que el emulador sigue pintando 1:1). El
  arreglo es `scripts/console/flycast-adelante.sh`, que `run-flycast.sh` llama solo cuando va a sacar
  capturas. Re-medido: pantalla encontrada en 210,238 (ancho 640, o sea 1:1) y 4051/4051 pixeles.

**Los mutantes** viven en `scripts/console/mutar-motor-pvr.sh` y son los del camino del color: `alpha0`
(el indice 0 deja de ser transparente), `uv1` (la UV derecha pasa a 1.0 y muestrea el relleno de la
textura), `pal1` (la textura usa el banco 1 de la paleta, que esta vacio) y `frame1` (se dibuja el
frame 1 con la referencia del frame 0). Medido: **4/4 cazados** y control sano verde
(`arte=4051/4051 transparentes=2085/2085 veredicto=OK`). El detalle que dejo el primer intento:
`alpha0` daba `arte=4051/4051` y quedaba **verde** —sacar la transparencia del indice 0 no mueve NI UN
pixel de arte, solo los transparentes— asi que el mutador ahora lee las dos lineas y el veredicto. El
hueco estaba en el mutador, no en la comparacion (que ya daba NO COINCIDE).

```
source scripts/console/env.sh
bash scripts/console/build-std-gox.sh                    # media: 7,3 s (44 paquetes)
bash scripts/console/build-motor-pvr.sh /tmp/dcbuild/motor-pvr   # media: 3,1 s
bash scripts/console/compare-motor-pvr.sh /tmp/dcbuild/motor-pvr # media: 20,0 s (texto + captura)
bash scripts/console/mutar-motor-pvr.sh                  # media: 1 m 25 s (4 mutantes + control)
```

Evidencia con los numeros crudos: `evidencia/dc-002.9-motor-pvr.txt`, y los dos lados del resumen
(`...-host.txt` y `...-consola.txt`, identicos).

## 24. DC-002.8b: los dos defectos de la propia suite, y el mutador que no mordia

La tarjeta no era del motor ni de la stdlib: eran **dos defectos de la suite**, de la clase que hace
parecer que algo esta probado cuando no lo esta. Salieron medidos en DC-002.8 (seccion 21) y quedaron
declarados sin arreglar. Los dos estan cerrados. Evidencia cruda:
`evidencia/dc-002.8b-suite.txt`.

**1. El control de costo de float64 del POC miraba un bit que no cambia.** El control imprimia
`bits64(suma)&1`, y la suma de los seis bucles termina **par**: el bit bajo es siempre 0. Medido, el
control daba `0` en todas las corridas y no se ponia en rojo ni sacando un bucle entero, o sea que las
mediciones de costo no estaban controladas por nada. Ahora el control compara la suma contra un **valor
conocido**: los 1e5 terminos de la raiz dan ~2,1e7 por si solos, asi que con el piso en **1e7** cualquier
bucle que falte deja la suma corta y el control da 0; y ademas se imprime el **digest** de la suma, para
lo que el piso tapa (un resultado distinto que sigue por encima del piso). Medido en la consola:
`control=1 (piso 1e7; suma_bits=4174392dddcd686a)` con los mismos 2746 casos de siempre
(`2746 bien / 2746 mal 0`) y los mismos ms de costo (20/20/132/51/70/71 ms): el control cambio su
capacidad de cazar, no la medicion. El mutador nuevo `scripts/console/mutar-std-consola.sh` mete los dos
defectos posibles —el bucle que no acumula (**lo caza el piso**: control 1 → 0) y un termino con otro
valor (**lo caza el digest**: `4174399a3d683b3a` ≠ `4174392dddcd686a`)— y da **2/2**.

**2. El cuelgue intermitente de 386: `TestRWMutexContraGo`, 1 cada 10 corridas.** El test que se colgaba
es el bloque de **dos lectores** (dos goroutines con `RLock`/`RUnlock` sobre el mismo `RWMutex` de la
consola). El canal de un candado de la consola se crea en el **primer uso** (limite declarado en
`d2std/sync`): en el host, dos goroutines que lo usan en paralelo por primera vez pueden crear **dos
canales** a la vez, y cada una se queda con su testigo. Ese bloque usaba un candado (`mDC`) al que nadie
le habia creado el canal. El volcado completo lo dice sin adivinar: la goroutine queda clavada en
`RWMutex.RUnlock -> Mutex.Unlock -> <-m.libre` (un testigo que nadie va a soltar) mientras el test espera
en el `WaitGroup`. Medido con el candado sin preparar: **4 cuelgues en 40 corridas** de
`-count=20 -timeout 60s` (= 800 ejecuciones, tope de 60 s cada uno; la referencia que traia la tarjeta era
1 en 40 corridas). Con el arreglo: **0 cuelgues en 80 corridas** (= 1600 ejecuciones).

**3. El hueco que estaba debajo: el aserto del mutador vivia adentro de lo que el mutante sacaba.** Este
es el hallazgo que no estaba en la tarjeta y que salio de correr su propio mutador: `mutar-cuelgue-386.sh`
terminaba en **1/2** con la suite arreglada — el mutante que quita el `preparar()` del candado quedaba
**VERDE**. Por que: la exigencia de `Preparado()` (que es lo que caza un candado sin preparar) vivia
**adentro** de `preparar()`, y el mutante saca la llamada a `preparar()`. Sacaba la llamada y con ella el
aserto. Y el aserto viejo del bloque tenia el otro problema: con **un** incremento por goroutine, dos
goroutines que suman 1 cada una dan 2 aunque el candado no excluya, asi que la corrupcion no se podia ver.
Arreglado en `console/dc/std/difftest/dc0027_test.go`:

- el aserto de "el candado esta preparado" se mudo al ayudante `dosLectoresConsola()`, que es quien usa el
  candado en paralelo y **que el mutante no toca**: la exigencia ya no vive en la misma llamada que la
  prepara;
- los dos lectores suman **2000 veces cada uno** y el total de la consola se exige **exacto (4000)**, con
  una **barrera** para que los dos entren a la vez (sin barrera, la segunda goroutine puede arrancar
  despues de que la primera termino y el bloque no probaba paralelismo). El lado de Go suma con
  `sync/atomic`: ahi los dos `RLock` entran juntos y un `n++` pelado seria carrera de verdad.

Medido, 20 invocaciones separadas (sin cache) del test en 386: **con el mutante 20 rojas / 0 verdes** (el
mensaje es el del candado sin preparar, no un fallo de casualidad) y **sin el mutante 20 verdes / 0
rojas**. El mutador completo: **2/2, rc=0**.

**Como se midio esto (y por que importa).** Todo lo de 386 corrio sobre **una copia de `console/dc/std`
sacada de git** (verificada archivo por archivo contra los blobs de HEAD: 77 archivos, 0 distintos), no
sobre el arbol de trabajo. La razon es un defecto medido, no una precaucion teorica: la primera corrida se
**contamino**, porque habia otra sesion de Hermes corriendo el mutador de la stdlib sobre el mismo arbol y
mi corrida compilo `d2std/bytes` con un mutante puesto. El test que giraba 100 s entonces **no era el del
candado**, era `TestBytesBufferReadContraGo`. Ese dato queda declarado: **un test que gira en vez de fallar
se come el tope entero del `go test`** (la otra sesion tardo 61 s en esa misma corrida), y es exactamente
por lo que esta tarjeta insiste con asertos que fallan en el acto y con nombres propios.

Cierre medido de la tarjeta:

```
control del POC (consola)   control=1 (piso 1e7; suma_bits=4174392dddcd686a), casos 2746 bien 2746 mal 0
mutador del control         2/2 (piso y digest)
el cuelgue de 386           antes: 4 cuelgues en 40 corridas (800 ejecuciones), TestRWMutexContraGo
                            despues: 0 en 80 corridas (1600 ejecuciones), peor corrida 4.476 ms
mutador del cuelgue         2/2, y el mutante del candado sin preparar: 20/20 rojo (antes quedaba verde)
suite del host              verde en amd64 y 386; mutar-std-host.sh 34/34 y arbol sano
barrera del motor           compare-motor-dc6.sh: 31 lineas identicas, 30 frames
```

```
bash scripts/console/mutar-std-consola.sh   # el control del POC: 2/2, 4 corridas de Flycast (medido: 1 m 59 s)
bash scripts/console/mutar-cuelgue-386.sh   # el cuelgue: 5x -count=20 + 2 mutantes (medido: 19,5 s)
bash scripts/console/test-std-host.sh       # los dos anchos del host (medido: 0,7 s con el cache de go caliente)
bash scripts/console/mutar-std-host.sh      # el general: 34/34 y arbol sano (medido: 2 m 11 s)
bash scripts/console/compare-motor-dc6.sh   # la barrera (medido: 17,1 s, con el build del POC aparte)
```

## 25. DC-001.5: los tres numeros del go/no-go, medidos (y el veredicto)

Los tres numeros que el plan por fases pedia para decidir si el port se hace o se corta estaban
pendientes desde la fase DC-001 y son los que mas pesan de todo el trabajo: sin ellos, "se puede" es una
opinion. Se midieron con un POC nuevo, `console/dc/poc/motor-300`, que carga el motor de verdad (los
bytes crudos del MPQ los decodifica `d2dc6.Load` + `DecodeFrame` y `d2dat.Load` compilados para SH-4, con
la textura PAL8 y la paleta del motor en el banco 0 del PVR) y despues dibuja **300 sprites por frame**
en una rejilla de 20x15 (paso de 15 px, escala 0,13: los 300 entran en pantalla tambien a 320x240).

Los tres numeros, en una linea cada uno:

    (a) EL BINARIO        1.390.948 bytes es lo que SE CARGA en los 16 MB (11 secciones: .text 642.032,
                          .rodata 362.740, .data 156.156, .bss 183.648, .eh_frame 45.844). El .elf en
                          disco pesa 7.025.836 bytes, pero 5.485.147 son el debug y el .go_export, que
                          no se cargan: lo que viaja al disco es el 1ST_READ.BIN, 1.207.432 bytes.
    (b) LA RAM LIBRE       10.485.760 bytes = 10,00 MB de los 16 MB, medidos CON EL MOTOR CARGADO y el
                          PVR andando, pidiendo bloques de 1 MB a malloc hasta que uno falla. La VRAM
                          (8 MB aparte) queda en 5.781.192 libres = 5,51 MB con la textura arriba.
    (c) EL FRAME CON       60,0 fps sostenidos: 302 frames en 5.011.680 us (16.594 us por frame) a
        300 SPRITES        640x480 y 301 en 5.010.117 us (16.644) a 320x240. El costo de CPU de SOMETER
                          los 300 sprites es 683 us por frame en las cuatro fases: 136.600 ciclos del
                          SH-4, un 4,1% del presupuesto de un frame de 60 Hz.

Tres cosas del metodo, porque son las que hacen que los numeros signifiquen algo:

- **(b) no se responde con el "libre" del heap de KOS.** `mallinfo().fordblks` marca 1.288 bytes en esa
  misma corrida, porque el runtime de Go se queda la arena de malloc de una sola vez (4.355.580 bytes) y
  deja el heap de newlib sin nada. Ese dato NO es la RAM que le queda al programa: el numero que
  contesta la tarjeta se mide pidiendo RAM hasta que falla, y por eso el POC mide las dos cosas e imprime
  las dos. Las dos lecturas de `mallinfo` (antes de cargar el motor y despues) dan lo mismo, que es otro
  dato: el DC6 de esta fase va compilado en el `.rodata` del ELF, no en el heap.
- **El frame se mide a fondo y sincronizado, y los dos dan igual.** El emulador TOPEA a 60 fps: la fase
  sin esperar el vblank da 16.624 y 16.657 us por frame, o sea lo mismo que la sincronizada. Se puede
  afirmar que el frame **no se cae** de 60 Hz; medir el frame libre necesita la consola real.
- **Los ciclos NO son un numero medido.** Los contadores del SH-4 (`PerfCntr*`) devuelven **0** en Flycast
  (medido en las cuatro fases), asi que el POC imprime los ciclos aparte, **derivados** del reloj (us x
  200 MHz). El numero medido es el tiempo. Los ciclos por frame se cierran en la tarjeta DC-001.2d, que
  es la que esta esperando la consola real: el mismo binario imprime las lineas `MOTOR-300` por el
  puerto serie y ahi los contadores existen.

El veredicto es **GO**: el codigo se lleva 1,39 MB de los 16 (8,7%) y deja 10,00 MB libres con el PVR
andando (el plan pedia ~8 MB), y un frame de 300 sprites paletizados gasta 683 us de CPU de los 16.667
que hay en 60 Hz. Ni la RAM ni la VRAM son las que aprietan.

Evidencia cruda y los comandos exactos: `evidencia/dc-001.5-go-no-go.txt`. El resumen para el repo (el
que pedia la tarjeta "en docs/"): `docs/dreamcast-go-no-go.md`.

```
POC=motor-300 bash scripts/console/build-motor-dc6.sh /tmp/dcbuild/8b-motor-300
CAPTURE=none bash scripts/console/run-flycast.sh /tmp/dcbuild/8b-motor-300/prog.elf 32 /tmp/dc-run-8b-300d
grep -a 'MOTOR-300' /tmp/dc-run-8b-300d/flycast.log | tr -d '\r'
source scripts/console/env.sh && sh-elf-size -A /tmp/dcbuild/8b-motor-300/prog.elf   # (a)
```

## 26. DC-002.10: el cuadro del motor en la pantalla de la Dreamcast (y 7,4 veces mas rapido)

> Esta seccion se escribe en DC-002.12: la tarjeta DC-002.10 se cerro sin su seccion del README (y lo
> mismo paso con DC-002.11), que es lo que el metodo pide. Los numeros son los de su evidencia
> (`evidencia/dc-002.10-motor-pantalla.txt` y `dc-002.10-pantalla.png`) y el resumen de cierre de la
> tarjeta; no se midio nada nuevo para escribirla.

El POC `console/dc/poc/motor-pantalla` dibuja **por el camino del motor completo**: una imagen del motor
(`ebiten.Image`) -> `DrawImage` sobre la pantalla -> la pantalla a la VRAM del PVR -> el PVR la pinta. Es
el paso que cierra la pregunta "¿el motor puede pintar en la consola?" con un cuadro en pantalla, y queda
claro por que hizo falta: el camino tiene tres costos distintos y los tres se midieron por separado.

Lo que imprime la consola serie (crudo en la evidencia):

    MOTOR-PANTALLA arranca (DC-002.10: el cuadro del motor en la consola)
    MOTOR-PANTALLA runtime dreamcast sh4
    MOTOR-PANTALLA asset 256 x 256
    MOTOR-PANTALLA imagen del motor armada, bytes de RGBA 262144
    MOTOR-PANTALLA la pantalla del juego ocupa en la VRAM, bytes 1048576
    EBITEN-LOOP cuadro 0 esperando el cuadro del PVR / ... subo el cuadro a la VRAM / cuadro 0 listo
    EBITEN-PERFIL dibujar_us 27376 convertir_us 47807 subir_us 2624 pintar_us 3 cuadro_us 77810

**El cuadro bajo de 590.087 us a 77.810 us** (de 1,7 a 12,8 cuadros por segundo): **7,4 veces**, por tres
causas distintas, cada una medida:

- **La FPU del SH-4 es de 32 bits y el bucle de dibujado usaba `float64`**, que en la consola se hace por
  software: 260 ms -> 59 ms -> 27 ms al pasar a `float32` con un caso rapido para solo mover.
- **`pvr_txr_load_ex` twiddlea la textura con una DIVISION POR PIXEL** y el SH-4 no tiene instruccion de
  division: el twiddle paso a una tabla con corrimientos y la subida a copia plana por Store Queues.
  Medido: **302.549 us -> 2.624 us (115 veces)**.
- **Pintar: 2 us.** O sea que el PVR no era el problema en ningun momento.

Los flags `PVR_TXRLOAD_SQ` y `PVR_TXRLOAD_DMA` del header de KOS **no hacen nada**: mismo tiempo exacto
con los dos (probado).

**Tres defectos que solo aparecieron midiendo** (dos de ellos MIRANDO la pantalla, porque la huella se
calcula antes del twiddle y no los veia): `vid_set_mode` despues de `pvr_init_defaults()` **cuelga**
`pvr_wait_ready()` para siempre (se ve un cuadro pintado y despues silencio, indistinguible de "anda
bien"); `a | b + c` en C suma antes del OR; y la tabla del twiddle en `uint16` **trunca** (los bits se
esparcen: `TWIDTAB(511) = 87.381` no entra en 16 bits).

Arreglos de fondo de la misma tarjeta, porque el instrumento tambien estaba mal: `build-dc.sh` nunca
linkeaba los terceros (los compilaba y no entraba a `orden.txt`) y `preflight.sh` usaba
`-fsyntax-only`, bandera con la que **gccgo se cae** cuando el paquete tiene un struct con un campo slice
(`internal compiler error in fill_in_array`): la herramienta contestaba "no compila" por un defecto del
compilador. Commits 565987e3 y 457c6cfc.

**Lo que quedo pendiente y se cerro en DC-002.12**: al lado de la evidencia se dejo escrito
`poc/motor-pvr/ESTADO.md` diciendo que el POC `motor-pvr` **no compilaba** y culpando al horneador. Era
un diagnostico falso (se habia usado el script equivocado): el POC compila y corre desde DC-002.9 y sigue
dando las mismas 61 lineas. Esta contado en la seccion 28.

## 27. DC-002.11: el control de la Dreamcast (el bus maple) y el instrumento que media otra cosa

> Misma aclaracion que la seccion 26: la seccion se escribe en DC-002.12 con los numeros que ya estaban
> medidos en `evidencia/dc-002.11-control.txt` y `dc-002.11-control.png` (commits 63372cb3 y 6e317504).

El control de la Dreamcast se lee del **bus maple** (`maple_enum_type` + `maple_dev_status`) y se traduce
al motor: la palanca y la cruceta mueven el **cursor**, A es el clic izquierdo, B el derecho, la cruceta
son las **flechas** del teclado (es lo que hace que los menus se puedan navegar), Start es Enter, los
gatillos son Q y E, y los mismos botones se llenan como botones de joystick.

**Cuando se lee la entrada importa, y se midio**: va **entre `CerrarCuadro()` y `Update()`**. Antes de
`CerrarCuadro` el estado del cuadro anterior queda guardado ya con la tecla nueva y el flanco
(`just pressed`, lo que mueve los MENUS) **nunca es verdad**; despues de `Update` el motor ya pregunto.

Lo que da la corrida (crudo en la evidencia):

    MOTOR-PANTALLA hay control en el puerto 0
    MOTOR-CONTROL botones 128 cursor 332 240        (128/64/16/32 = cruceta, un bit por direccion)
    MOTOR-CONTROL botones 4   cursor ...            (4 = boton A; el flanco "A se apreto en este cuadro" anda)
    MOTOR-PANTALLA cuadros 300 x 216 cursor 344 240 dibujos 299 huella 1996575903 centro 304 174
    MOTOR-PANTALLA cuadros 480 x 511 cursor 639 479 dibujos 479 huella -1487941904 centro 560 291

o sea que la **huella** cambia (el cuadro se movio) y el **centro de masa del contenido** se fue de
293,174 a 560,291: **267 px a la derecha y 117 abajo**, movidos por el control. Mapeo del teclado del
emulador, tambien medido: flechas -> cruceta, `Z` -> A (4), `X` -> B (2), `C` -> Start (8), `V` -> X
(1024). Y como se le mandan las teclas: en KDE/Wayland **no hay xdotool** y `wtype` **no sirve** ("el
compositor no tiene el protocolo de teclado virtual"), pero `/dev/uinput` es del kernel y esta
escribible: `scripts/console/teclas-virtuales.py` crea un teclado virtual y lo **destruye al terminar**
(si no, queda un teclado fantasma). La prueba completa es `scripts/console/probar-control.sh`.

**El defecto de fondo que tapaba toda la evidencia** (y que vale mas que el control): en `RunGame` decia
`pantalla := NewImage(...)`. El `:=` crea una imagen **local** que tapa a la del paquete, asi que el
dibujado y la VRAM usaban esa (la pantalla SE VEIA bien y la captura mostraba el logo), pero `Screen()` y
las cuentas de la evidencia leian la **GLOBAL, vacia**: el instrumento media otra cosa que la que se ve.
Se desperto porque la huella daba **el mismo numero con el logo en x=192 y en x=511**; se confirmo con
`Image.CentroDeMasa()` (promedio de las posiciones de los pixeles claros), que daba 0 0 con el logo
dibujado y visible en la captura. Leccion: **la huella dice SI el cuadro cambio, no DONDE**.

## 28. DC-002.12: el POC del motor que "no compilaba" (el diagnostico falso) y el aviso que faltaba

Cuando arranco esta tarjeta el tablero **no tenia ninguna tarjeta ready**: la tanda DC-002.5b..DC-002.9
estaba cerrada y DC-002.10/DC-002.11 tambien. Lo que quedaba abierto no estaba en el tablero sino en el
repo: (a) `console/dc/poc/motor-pvr/ESTADO.md` afirmaba que ese POC **no compila**, y (b) el README no
tenia las secciones de DC-002.10 ni DC-002.11, que es lo que el metodo pide para cerrar una tarjeta. Las
dos cosas se cerraron midiendo.

**1. El POC si compila, y da lo mismo que el host.** El camino existe desde DC-002.9 y no se habia usado
al diagnosticar:

    $ bash scripts/console/build-motor-pvr.sh /tmp/dcbuild/motor-pvr
      ELF: /tmp/dcbuild/motor-pvr/prog.elf   7.198.516 bytes   1ST_READ.BIN  1.300.936 bytes
      (hornea el DC6 crudo de 140.740 B y el pal.dat de 768 B, compila la stdlib minima y 11 paquetes
       del motor para SH-4)
    $ bash scripts/console/compare-motor-pvr.sh
      == IGUAL: 61 lineas identicas (mismo DC6, mismo codigo del motor, x86-64 y SH-4)
         frames comparados: 30
      pixeles de arte: 4051/4051 (100.0%)   diferencia maxima: 7 por canal
      pixeles transparentes: 2085/2085 (100.0%) con el fondo visible
      identidad 100.0% (espejada horizontal 17.5%, vertical 16.7%, rotada 180 11.9%)
      RESULTADO: OK      (18,58 s el ciclo entero, emulador adentro)

Asi que los dos "huecos" que el ESTADO.md declaraba no existian: `asset.conf` **si** lo lee un script
(`build-motor-dc6.sh`, que hornea los bytes crudos con `console/dc/bake-dc6`) y `motorresumen` **si**
estaba en el `.gox` (lista PAQUETES de `build-motor-gox.sh` desde DC-002.9; el archivo queda en
`<gox>/github.com/OpenDiablo2/OpenDiablo2/console/dc/motorresumen.gox`, 23.760 bytes).

**2. El defecto de verdad: el error ponia la culpa donde no era.** `build-dc.sh` no decia nada y el
compilador contestaba lo suyo por dos variables que ese script no puede generar:

    $ bash scripts/console/build-dc.sh console/dc/poc/motor-pvr /tmp/dcbuild/motor-pvr-check
    console/dc/poc/motor-pvr/main.go:53:30: error: reference to undefined name ‘assetDC6’
    console/dc/poc/motor-pvr/main.go:59:35: error: reference to undefined name ‘assetPAL’

Ese mensaje manda a buscar el problema en el horneador (y es exactamente el diagnostico falso que quedo
escrito). Ahora **`build-dc.sh` avisa antes de compilar** (exit 2, con el nombre del script correcto)
cuando el POC trae `asset.conf`, y `build-motor-dc6.sh` se lo saltea con `POC_ASSET_CRUDO=1`. De paso,
`bake-asset.sh` distingue los dos casos al no encontrar `bake.conf`: dice que el asset es CRUDO y quien
lo hornea, en vez del "no hay assets que hornear" que se lee como si al POC le faltara algo.

**La prueba es un corpus con su mutante**, `scripts/console/checar-poc-crudo.sh`:

    == control sano (el aviso puesto)
       build-dc.sh sobre console/dc/poc/motor-pvr: avisado (exit 2)
         ERROR: ... es un POC de asset CRUDO (trae asset.conf), no de un .h horneado.
                Se arma con:  scripts/console/build-motor-dc6.sh [dir_salida]
                              scripts/console/build-motor-pvr.sh [dir_salida]   (para motor-pvr)
    == mutante 'sin-aviso' (se saca la condicion del aviso en scripts/console/build-dc.sh)
       build-dc.sh con el aviso sacado: crudo (exit 1)
       cazado: sin el aviso el error vuelve a ser el que manda a buscar donde no esta
         .../main.go:53:30: error: reference to undefined name ‘assetDC6’
    == control sano de vuelta (el aviso repuesto)  ->  avisado (exit 2)
    == 1/1 mutantes cazados ==        (0,167 s todo)

El mutante se aplica por reemplazo EXACTO de texto sobre `build-dc.sh` con respaldo propio y `sha256`
(no `git checkout --`: el arbol puede tener trabajo de otra tarjeta encima) y se verifica que el archivo
vuelva al original.

**3. Lo que no se movio** (nada de esto podia romper el resto, y se midio): la barrera siguio IGUAL,
**31 lineas identicas** entre host y consola y 30 frames comparados (`compare-motor-dc6.sh`, 17,110 s);
el POC del color siguio verde con la captura 4051/4051 (18,58 s); y los **4/4 mutantes** del camino del
color se cazaron otra vez con el control sano verde (`mutar-motor-pvr.sh`, 1 m 19 s):

    alpha0  arte=4051/4051 transparentes=0/2085    veredicto=NO
    uv1     arte=572/4051  transparentes=1492/2085 veredicto=NO
    pal1    arte=0/4051    transparentes=2085/2085 veredicto=NO
    frame1  arte=1437/4051 transparentes=1837/2085 veredicto=NO
    control sano (restaurado) arte=4051/4051 transparentes=2085/2085 veredicto=OK

(y `alpha0` se sigue viendo en la TRANSPARENCIA y no en el arte: sacarle el indice 0 transparente no
mueve ni un pixel de arte, que fue el hueco del primer mutador).

```bash
# El POC que "no compilaba", medido de punta a punta
bash scripts/console/build-motor-pvr.sh /tmp/dcbuild/motor-pvr    # media: 3,1 s con SALTAR_STDLIB=1
bash scripts/console/compare-motor-pvr.sh /tmp/dcbuild/motor-pvr  # 18,58 s: 61 lineas + 4051/4051 px
bash scripts/console/checar-poc-crudo.sh                          # 0,167 s: control sano + 1 mutante
bash scripts/console/compare-motor-dc6.sh                         # la barrera: 17,110 s, 31 lineas igual
bash scripts/console/mutar-motor-pvr.sh                           # 1 m 19 s: 4/4 cazados + control sano
```

Evidencia con los numeros crudos: `evidencia/dc-002.12-motor-pvr-y-el-aviso.txt`.

## 29. DC-002.13: el arranque del motor de verdad (compila, enlaza, y se cae en los 16 MB)

Lo que faltaba saber despues de DC-002.12: el motor compila paquete por paquete (142/142 en DC-002.8) y
pinta (DC-002.9/.10) y lee el control (DC-002.11), pero **nadie habia probado el entry point de verdad**:
`main.go` de la raiz -> `d2app.Create(...).Run()` -> `d2app/d2setup`, con el motor entero enlazado. Esta
tarjeta lo hizo, y el resultado tiene tres numeros y una caida con nombre.

**1. Compila** (el paquete del entry point, con el `.gox` completo del motor):

    sh-elf-gccgo $GODC_GOFLAGS -fmax-errors=100 -I <gox> -fgo-pkgpath=main \
        -c main.go doc.go bundle_other.go -o main.o      ->  0,119 s, main.o = 55.216 bytes

**2. Enlaza, y el ultimo escalon fueron 5 simbolos del runtime.** El enlace del motor entero
(`scripts/console/build-motor-arranque.sh`: los **205 `.gox`** — 142 del motor + 63 de stdlib/terceros) se
cae con **15 referencias en 5 simbolos**: `runtime.memhash` (6), `memhash32` (3), `f64hash` (3),
`strhash` (1), `memhash8` (1). Los cinco son los hash que gccgo pone en el **descriptor de tipo** de las
claves de mapa que no son de los tipos rapidos. libgodc SI los tiene, pero con el sufijo `..f` (el nombre
de cuando se la LLAMA); el nombre **pelado** no estaba definido para los hash.

Las firmas se leyeron del envoltorio que arma el compilador (`sh-elf-objdump -dr`), no se adivinaron: para
una clave `struct{string;uint32;int}` llama `strhash(clave,h)` y despues `memhash32(campo,h)` —**dos**
argumentos—; para una `struct{int;int;int}` llama `memhash(clave,h)` con `mov #12,r6` en el slot de
retardo, o sea **tres** argumentos con el tamano. `console/dc/std/csrc/hash_dc.c` define los 5 nombres
pelados: los de dos argumentos delegan en lo que libgodc ya tiene y el de tres en `map_memhash` — **el
mismo algoritmo que libgodc usa en sus caminos rapidos de mapa**, que es lo que hace que la clave se
encuentre. Se hizo en el C de la stdlib minima (repo) y no en el runtime vendorizado: los
`fix-libgodc-*.py` empiezan dejando el arbol pristino y un cambio a mano ahi se pierde. Re-medido: enlaza,
exit 0, en **1,458 s**.

**3. El instrumento tenia un limite que no estaba escrito: Flycast rechaza todo ELF de mas de 16 MB.** Lo
unico que contesta es `Flycast has stopped: Failed to open ELF <ruta>`, y en su fuente esta el porque
(`core/reios/reios_elf.cpp:18`: `if (size == 0 || size > 16_MB) return false;`). Con el motor entero el ELF
sale de **43.876.988 bytes** (11.963.527 son `.go_export` y ~14 MB de `.debug_*`, que no se cargan), asi que
`build-motor-arranque.sh` deja `prog-emu.elf` sin ellos: **13.707.552 bytes**. Lo que se carga de verdad
son las secciones ALLOC: **13.910.992 bytes = 13,27 MB de los 16** (82,9%).

**4. Y ahi se cae, con el numero exacto**: KOS 2.2.1 arranca, monta el maple, y el runtime de Go de la
consola muere **antes de ejecutar una sola linea del motor** (no hay ni una linea `MOTOR-ARRANQUE`):

    Out of memory. Requested sbrk_base 0x8d16b000, was 0x8cf6b000, diff 2097152
    fatal error: gc_init: alloc failed

O sea: el binario deja el break en 0x8CF6B000 (16.166.912 bytes = **96,36% de la RAM**), al GC le quedan
610.304 bytes (0,58 MB) y pide su semiespacio de **2 MB** (`gc_semispace.h:21`): **faltan 1.486.848 bytes
= 1,42 MB**. No es cosa de agrandar o achicar el heap: con un semiespacio de 1 MB seguirian faltando
438.272 bytes, y el semiespacio mas grande que entra son 610.304 bytes (**596 KB**). Lo que no entra es el
**binario**, no el GC.

**El veredicto**: el motor entero compila, enlaza y **casi arranca**: le faltan 1,42 MB de los 16, y lo que
hay que recortar se puede medir en el ELF enlazado (recortar el binario o arrancar por partes). Mientras
tanto, las dos piezas que ya andan en la consola siguen andando: el cuadro del motor (DC-002.10/.12) y el
control (DC-002.11) no enlazan el motor entero.

```bash
# El arranque del motor de verdad, de punta a punta (medido: 1,5 s el enlace, 27,0 s la corrida)
bash scripts/console/build-motor-arranque.sh /tmp/dcbuild/motor-arranque /tmp/gox-0028
CAPTURE=none bash scripts/console/run-flycast.sh /tmp/dcbuild/motor-arranque/prog-emu.elf 20 /tmp/dc-run-arranque2
grep -a 'fatal error\|Out of memory\|Memory:' /tmp/dc-run-arranque2/flycast.log
```

Evidencia con los numeros crudos, las firmas leidas del binario y la aritmetica:
`evidencia/dc-002.13-arranque-del-motor.txt`.

## 30. DC-002.14: donde se van los 13,27 MB y que recorte hace arrancar (el semiespacio del GC)

DC-002.13 cerro con un numero y una promesa: el motor entero enlaza, arranca y muere en el `gc_init` porque
**faltan 1.486.848 bytes** de los 16 MB, y "lo que hay que recortar se puede medir en el ELF enlazado". Esta
tarjeta midio de donde sale el peso, probo las palancas del compilador y del enlazador, y encontro que **el
recorte que hace arrancar no esta en el binario: esta en lo que el GC reserva antes de la primera linea del
motor**.

**1. La herramienta con la que se iba a recortar media mal, y lo dijo su propio control.** `pesar-motor.py`
atribuia el peso por paquete sumando el tamano de los simbolos del ELF enlazado. Medido en modo
`--secciones`:

    seccion   suma de tamanos   tamano REAL de la seccion   suma/seccion
    .text          14.215.564            4.962.328               2,86x
    .data           1.918.399            1.760.156               1,09x
    .bss              412.208              204.256               2,02x
    .rodata         4.959.484            5.896.816               0,84x
    simbolos de .text: 21.266;  los que ARRANCAN DENTRO del anterior: 21.242
    UNION de los intervalos de .text: 4.971.334 = 100,2% de la seccion

Los tamanos son ciertos pero **no son exclusivos** (el ultimo del grupo se lleva el resto de la seccion), asi
que sumarlos por paquete reparte bytes ajenos. **La tabla "por paquete" de DC-002.8/.13/.14 (datos del
compilador 3.964.602, d2game 1.098.252, d2common 996.244, ...; total 9.555.553) queda retirada.** Ahora el
tool detecta el solape y **sale con exit 2 sin imprimir tabla**, y el peso por paquete se mide con
`pesar-gox.py`: las secciones ALLOC de cada `.gox`, donde cada byte se cuenta una sola vez (y sin los
`.debug_*` ni el `.go_export`, que engañan: un `.gox` de 4,6 MB puede tener 2,28 MB de codigo y datos).

**2. El peso honesto, y la sorpresa: lo mas pesado es UN DATO, no codigo.** 205 `.gox`, **13.604.164 bytes**
(el ELF enlazado carga 13.910.992: los 306.828 de diferencia son libc/KOS/libgodc y alineacion).

    bytes      gox  paquete
    2281930   6033  d2core/d2records        <- de esos, 1.672.892 son UN dato
    1326100  12841  d2game/d2gamescreen
    1248929  14729  d2game/d2player
     555773   6001  d2core/d2monsters
     378544   3902  d2common/d2monster
     362657   3778  d2core/d2skills
     349228   3327  d2common/d2skill
     247392   2539  d2core/d2hero
     237757   2456  d2common/d2quest
     227221   2281  d2common/d2drlg/drlgoutdoor
     ... (los 25 primeros en la evidencia)

El dato de 1.672.892 bytes (**95,0% del `.data` del motor entero**) es la tabla `var objectLookups =
[]ObjectLookupRecord{...}` de `d2core/d2records/object_lookup_record_data.go` (1.942.429 bytes de fuente,
7.902 lineas, 7.892 elementos: el volcado de `objectLookups.txt`). Se usa de verdad --
`record_manager.go:285` llama `r.initObjectRecords(objectLookups)` en el init -- asi que **no es codigo
muerto que se pueda tirar**: es el tipo de dato que en la consola no deberia estar en `.data` sino leerse del
disco. Queda anotado para mas adelante.

**3. Las dos palancas del compilador/enlazador: medidas y muertas.**

| palanca | resultado medido |
|---|---|
| `-ffunction-sections -fdata-sections` + `-Wl,--gc-sections` | cargado 13.910.992 -> **13.913.308 (+2.316, no baja)**. El linker SI tira 38.491 secciones, todas diminutas (stubs `..eq..f`, `.data` vacias de `crtbegin.o`). gccgo no parte la salida por funcion: el ELF tiene 0 secciones `.text.*` |
| `-fno-asynchronous-unwind-tables` (el `.eh_frame` son 1.066.804 bytes = 7,67%) | en un paquete real el `.eh_frame` queda **identico** (3.544 con y sin la bandera); en el motor entero sale 480 bytes **mas grande** |
| sacar paquetes que el entry point no usa | 142 enlaces, uno sin cada paquete: **8 MUERTO** (d2common, d2common/d2data, d2common/d2replay, d2thread, ...) y **134 NECESARIO**. Los MUERTO son paquetes sin codigo y el ELF sale del MISMO tamano (43.888.556): **no ahorran RAM** |

**4. La cuenta corregida: `gc_init` pide DOS semiespacios, no uno.** La cuenta de DC-002.13 media solo la
primera peticion. En el runtime vendorizado (`libgodc/runtime/gc_heap.c:37-38`) el init hace dos
`memalign(32, GC_SEMISPACE_SIZE)`, asi que lo que necesita para arrancar es `2 x GC_SEMISPACE_SIZE`. Con la
misma imagen (13.910.992 cargados, 610.304 libres) la escalera medida es:

    GC_SEMISPACE_SIZE_KB   el GC pide   el arranque
    2048 (default)            4 MB      muere en gc_init: alloc failed (diff 2097152)
    1024                      2 MB      pasa gc_init, muere despues en gc_external_alloc: OOM (diff 1228800)
    512                       1 MB      pasa las DOS paredes de RAM y llega al panic de Go del init
    256                     512 KB      igual que 512 (con arena mas chica: 1.915.772 contra 2.440.060)

El recorte es entonces de configuracion del runtime, con el script idempotente que ya usa el proyecto para el
vendor: **`scripts/console/fix-libgodc-gc.py 512 --build`** (por defecto 512 KB, el semiespacio mas grande
que llega hasta el fondo). Costo declarado: un heap de 512 KB GC-ea mucho mas seguido; sirve para el arranque
y para seguir midiendo, no es la configuracion de un juego corriendo.

**5. El arranque, entero, con el semiespacio en 512 KB.** El MISMO binario de DC-002.13 (sin recortarle un
byte) ahora deja atras las dos paredes de RAM y muere mas adelante, con un defecto **distinto**:

    panic: assignment to entry in nil map

    goroutine 1 [running]:
    Stack Trace: frame pointers not enabled!

    Memory: arena=2440060 used=2436340 free=3720

Ni una linea `Out of memory` ni `gc_init: alloc failed`, y tampoco ni una linea `MOTOR-ARRANQUE`: el panic es
**antes de `main`**, en el init del runtime o de los paquetes. Eso ya no es RAM (es el defecto siguiente) y va
como tarjeta nueva.

**6. El corpus y la barrera.** `scripts/console/checar-recorte-214.sh` deja 4 controles sanos (el instrumento
que se niega a mentir, el dato medido por los dos caminos, el arranque que pasa el techo de RAM, y el registro
del limite del linker) y caza **2/2 mutantes**: `sin-guard` (se saca el aviso de solape: la tabla solapada
vuelve y el control 1 se cae) y `gc-2048` (el semiespacio vuelve al default del vendor: el arranque vuelve a
morir en `gc_init` con diff 2097152). La barrera no se movio: **31 lineas identicas** entre host y consola
(`compare-motor-dc6.sh`), con el runtime nuevo adentro.

```bash
# Donde esta el peso y que recorte hace arrancar (todo medido)
bash scripts/console/recorte-214.sh                    # los .gox con los flags + el enlace (medido: no baja)
python3 scripts/console/pesar-motor.py <elf> --secciones # el instrumento que se niega a mentir (exit 2)
python3 scripts/console/pesar-gox.py /tmp/gox-214-gc   # el peso por paquete, sin solapes
bash scripts/console/medir-alcance-motor.sh /tmp/gox-214-gc   # 8 MUERTO / 134 NECESARIO (142 enlaces)
bash scripts/console/medir-gc-semiespacio-214.sh       # la escalera 2048/1024/512/256 en Flycast
python3 scripts/console/fix-libgodc-gc.py 512 --build  # el recorte del GC que hace arrancar
bash scripts/console/checar-recorte-214.sh             # el corpus: 4 controles + 2/2 mutantes
bash scripts/console/compare-motor-dc6.sh              # la barrera: 31 lineas iguales
```

Evidencia con todas las salidas crudas (incluida la del corpus y la de la barrera):
`evidencia/dc-002.14-peso-y-recorte.txt`.

## 31. DC-002.15: el init que escribia en un mapa nil (y el regexp que habia detras)

DC-002.14 dejo el arranque del motor pasando las paredes de RAM y muriendo en un panic de Go ANTES de
`main`. Esta tarjeta encontro QUIEN lo disparaba y lo arreglo, y en el camino dejo identificado el defecto
siguiente.

**1. Quien, sin pila que leer.** El mensaje lo imprime libgodc y el runtime minimo no sabe armar la pila
(`Stack Trace: frame pointers not enabled!`), asi que no habia a quien preguntarle. El instrumento es una
traza en los TRES sitios que paniquean por mapa nil (`runtime/map_dreamcast.c` y el macro de
`map_fast_internal.h`) y en la entrada del panic (`runtime_gopanic_impl`), con el script idempotente
`scripts/console/fix-libgodc-map-traza.py`: imprimen por la consola serie el **pc de retorno** y el tipo del
dato. El pc se resuelve contra el ELF enlazado (que lleva simbolos), descontando 4 (en SH-4 el registro de
retorno apunta a la instruccion siguiente, y las instrucciones son de 2 bytes):

    MAP-NIL-TRAZA runtime_mapassign_fast32 pc=8c148c14 t=8c666058
    -> sh-elf-addr2line 8c148c10: github_..._d2common_1d2skill..init0   d2common/d2skill/class_abc.go:17
    -> sh-elf-nm 8c666058: _type..map_6int_7..._d2skill_0doFn

**2. La causa, y por que el codigo se veia bien.** `d2common/d2skill/class.go` declaraba
`var doTable map[int]doFn` (nil) y lo creaba **dentro de su `init()`**; `class_abc.go` escribia
`doTable[15] = ...` **desde su propio `init()`**, con este comentario: *"Go runs the init functions of a
package in file name order and "class.go" comes before this file"*. Eso ultimo es comportamiento de la
**herramienta** de Go, no del lenguaje: el spec solo garantiza que la inicializacion de las **variables**
termina antes de que corra **cualquier** `init()`. Medido: la linea de compilacion presenta los archivos en
orden (`ls *.go`), pero gccgo numeró el init de `class_abc.go` como `..init0`, o sea **primero** — el orden
de los `init()` de archivo no es el orden de la linea de compilacion.

**3. El arreglo (en el motor, no en el vendor).** El mapa pasa a crearse en la inicializacion de la
variable, que es lo que el spec si garantiza:

    var doTable = map[int]doFn{ ... }     // antes: var ... + func init() { doTable = ... }

    $ python3 scripts/console/fix-d2skill-dotable-215.py        # idempotente
    $ git diff --stat d2common/d2skill/class.go                 # 101 inserciones, 98 borrados (el dedent)
    $ gofmt -l d2common/d2skill/                                # vacio

Efecto medido, recompilando solo ese paquete y reenlazando: **0** lineas `nil map` y **0** `MAP-NIL-TRAZA`
(el panic del mapa nil quedo atras).

**4. El defecto siguiente, ya identificado: un `regexp` que no compila.** Con la traza tambien en la
entrada del panic, el arranque arreglado ahora dice:

    PANIC-TRAZA pc=8c44b4ba tipo=8c4ef00c kind=54 dato=8cda6680
    panic: (value of kind 22 at 0x8cda6680)

`kind 22` = `GO_PTR` y el tipo en 8c4ef00c esta declarado en el ELF como `_regexp.Error..p`: el panic es con
un `*regexp.Error`. Sigue siendo antes de `main`, asi que lo dispara un `regexp.MustCompile` de **nivel de
paquete** (el motor tiene nueve). La sonda `console/dc/poc/sonda-regexp` compila los nueve y dice cual se la
lleva: los cuatro sin bandera compilan, y el quinto —`d2core/d2map/d2mapgen/real_maze_gen.go:33`,
`(?i)(pre|prev|up|ent|exit)\d*\.ds1$`— paniquea. Aislado en una segunda corrida: **`(?i)a` sola ya rompe el
regexp de la consola**, o sea la bandera de plegado de mayusculas. Es el unico patron del motor que la usa.
El veredicto fino (parser de banderas, tablas de unicode o el motor de ejecucion) va como tarjeta propia.

**5. El corpus.** `scripts/console/checar-init-215.sh`: control sano (el panic del mapa nil no vuelve y el
que queda viene identificado por la traza) + **mutante `init-de-archivo`** (se deshace el arreglo: `doTable`
vuelve a crearse en el `init()` de `class.go` y el arranque **vuelve a morir en el mapa nil** → cazado
1/1). El corpus no pide `MOTOR-ARRANQUE > 0` a proposito: el defecto del regexp esta delante y es de otra
tarjeta; lo que se mide aca es que el mapa nil dejo de ser el que corta. La barrera no se movio:
`compare-motor-dc6.sh`, 31 lineas identicas host/consola.

```bash
# Quien paniquea, en la consola (sin pila que leer)
python3 scripts/console/fix-libgodc-map-traza.py --build       # la traza en libgodc (idempotente)
bash scripts/console/build-motor-arranque.sh /tmp/dcbuild/motor-traza /tmp/gox-0028
CAPTURE=none bash scripts/console/run-flycast.sh /tmp/dcbuild/motor-traza/prog-emu.elf 15 /tmp/dc-run-215-traza
grep -a 'MAP-NIL-TRAZA\|PANIC-TRAZA' /tmp/dc-run-215-traza/flycast.log | tr -d '\r'
sh-elf-addr2line -f -e /tmp/dcbuild/motor-traza/prog.elf 8c148c10    # el llamado esta en pc-4

# El arreglo del motor y su corpus
python3 scripts/console/fix-d2skill-dotable-215.py             # doTable como variable, no en init()
bash scripts/console/checar-init-215.sh                        # 1 control + 1/1 mutantes

# El defecto siguiente (regexp), medido
bash scripts/console/build-dc.sh console/dc/poc/sonda-regexp
CAPTURE=none bash scripts/console/run-flycast.sh /tmp/dcbuild/sonda-regexp/prog.elf 15 /tmp/dc-run-sonda-regexp
grep -a 'SONDA-REGEXP\|panic' /tmp/dc-run-sonda-regexp/flycast.log | tr -d '\r'
```

Evidencia con las salidas crudas (traza, addr2line, la sonda y el corpus):
`evidencia/dc-002.15-init-mapa-nil.txt`.

## 32. DC-002.16: la bandera (?i) del regexp de la consola — dos defectos, y el arranque que llega a main

DC-002.15 dejo identificado el defecto siguiente: con el mapa nil arreglado, el arranque moria **antes de
`main`** con un panic cuyo valor era un `*regexp.Error` (0 lineas `MOTOR-ARRANQUE`), o sea un
`regexp.MustCompile` de nivel de paquete. El unico patron del motor con bandera es el de
`d2core/d2map/d2mapgen/real_maze_gen.go:33`. Esta tarjeta lo midio y lo cerro — y el arranque del motor
ahora **llega a `main`**.

**1. La causa: un off-by-one en el corte del prefijo.** El regexp de la consola es codigo propio
(`console/dc/std/regexp`). `Compile` sacaba la bandera con `p[3:]`, pero `"(?i)"` mide **cuatro** bytes
(`( ? i )`): quedaba `"i)a"` y el parser se plantaba en el `)`. Medido — el error exacto, y el mismo en el
host y en la consola porque el `.gox` sale de este archivo:

    regexp: sobra )a en (?i)a
    regexp: sobra )(pre|prev|up|ent|exit)\d*\.ds1$ en (?i)(pre|prev|up|ent|exit)\d*\.ds1$

No era el parser de banderas, ni las tablas de plegado de `unicode`, ni el motor de ejecucion: era el
corte del prefijo. Los 9 patrones de nivel de paquete del motor pasaron de **8 de 9** a **9 de 9**.

**2. El segundo defecto: la bandera estaba MUERTA.** Con el prefijo ya cortado bien, `(?i)a` compilaba
pero **no encontraba la "A"**: `igual` comparaba con `alternarMayus` en los dos lados, y esa funcion es un
**intercambio** de caso (a->A, A->a), biyectivo: `alternarMayus(x) == alternarMayus(y)` solo si `x == y`.
Medido contra el regexp de Go: `MatchString("(?i)a")` en `"BANANA"` daba `false` (Go: `true`) y
`FindAllString` en `"aAa"` daba 2 de 3. El arreglo agrega `plegarMayus` (a minuscula) para comparar; en
`claseChar.contiene` el intercambio **si** es lo correcto (ahi la pregunta es simetrica, "¿el otro caso
cae en el rango?", y es lo que hace que `[a-z]` acepte `"Z"`), por eso no se toco.

**3. Por que ninguna suite lo veia.** Los 13 casos del corpus diferencial del regexp eran **sin bandera**:
la suite estaba verde y el motor paniqueaba igual. El hueco era del corpus. Ahora hay 6 casos nuevos con
el patron real del motor (`(?i)a`, `(?i)^abc$`, `(?i)[a-z]\d*\.ds1$`, `(?i)(pre|prev|up|ent|exit)\d*\.ds1$`
con 11 textos: `PRE1.ds1`, `Up.ds1`, `EXIT34.ds1`, `data/global/tiles/PRE1.ds1`, `x.ds1`, `exit.ds2`, ...),
el aserto paso a `Compile` (con `MustCompile`, el panic se llevaba puesto el proceso de test y el mutador
leia un desborde en vez del aserto), y el patron **vacio** se midio contra Go: **no es un limite**, va al
corpus. El limite que queda declarado y vigilado (`TestLimitesConocidos`) es que el plegado es **solo
ASCII** (para `(?i)` el signo Kelvin U+212A es una "k" en Go; la consola no tiene esas tablas).

**4. La consola, medida con la sonda** `console/dc/poc/sonda-regexp` (los 9 patrones del motor con
`Compile` — el error, no el panic — y despues el plegado del patron real, 6 de 6 y 0 de 2):

    ROJO  (gox con el defecto, sha256 d5ca2cb1…): 8 de 9 compilan; el de la bandera: el error de arriba;
                                                  y el MustCompile final paniquea (mismo sintoma del motor)
    VERDE (gox de cierre,    sha256 f990b831…):   9 de 9, plegado 6 de 6 y 0 de 2, sin panic

**5. El arranque del motor, re-medido.** `checar-init-215.sh` se **aprieta** en esta tarjeta: su control
ya no acepta "cualquier panic que no sea el del mapa nil", ahora **exige llegar a `main`** (N marcas
`MOTOR-ARRANQUE`) y 0 lineas con `panic:`; el panic del regexp es un ROJO explicito. Medido: **9 marcas y
0 panics** (antes: 0 marcas + panic), 1/1 en el mutante del mapa nil. Las 9 marcas dicen ademas **donde
llega**: `d2app.Create()`, `Run()`, y `Run() termino con error: operation not supported on read-only
filesystem`, con el video sin arrancar (`no se pudo arrancar la pantalla del PVR (codigo -3)`). Ese es el
defecto siguiente, de otra tarjeta: lo que frena al motor ya no es el codigo ni la stdlib, es el entorno.

**6. Los mutantes.** `mutar-std-host.sh`: **36/36** (antes 34/34); los dos nuevos son el corte del prefijo
y el plegado (`--- FAIL: TestContraElRegexpDeGo`). Y uno nuevo, `mutar-regexp-216.sh`, que mide la otra
punta: recompila **solo** `regexp.gox`, reenlaza el motor-arranque y corre Flycast — control sano (9
marcas, 0 panics), mutante `p[3:]` (**0 marcas + el panic del regexp, cazado 1/1**) y control de vuelta
con el **mismo sha256** de la gox. La barrera no se movio: `compare-motor-dc6.sh`, **31 lineas identicas,
30 frames**.

**Nota del arnes (dos trampas medidas, que vuelven a aparecer):** (a) `build-std-gox.sh` arranca con un
`rm -rf` de su directorio de salida, asi que **rehacer la stdlib borra la gox del motor** (los 142
paquetes + `orden-motor.txt`) que vive en el mismo directorio: el enlace se cae con "falta
orden-motor.txt" y hay que recomputar el motor (5 min). El mutador de esta tarjeta recompila **un solo**
`.gox` (copia las fuentes a un temporal, reescribe `d2std/` -> `""` y compila con su pkgpath), igual que
hace `checar-init-215.sh` con `d2common/d2skill`. (b) Los paquetes de **terceros** (ebiten, otto, akara,
colornames, uuid, bmp, wav) van a `${GODC_GOX:-$DC_ROOT/gox}`, no al directorio que se pasa como
argumento: hay que **exportar `GODC_GOX`** o quedan en la gox compartida. Medido: sin exportarlo el motor
se corta en **100 de 142**; con `export GODC_GOX=/tmp/gox-00216`, **142 de 142**.

```bash
# La causa y el corpus, en el host
cd console/dc/std && GOARCH=amd64 go test ./regexp/     # y GOARCH=386
bash scripts/console/test-std-host.sh                   # los dos anchos, verde
bash scripts/console/mutar-std-host.sh                  # 36/36 cazados

# La consola: la sonda (9 patrones del motor + el plegado)
export GODC_GOX=/tmp/gox-00216
bash scripts/console/build-dc.sh console/dc/poc/sonda-regexp
CAPTURE=none bash scripts/console/run-flycast.sh /tmp/dcbuild/sonda-regexp/prog.elf 15 /tmp/dc-run-regexp
grep -a 'SONDA-REGEXP' /tmp/dc-run-regexp/flycast.log | tr -d '\r'

# El mutante de la consola (control sano + p[3:] + control de vuelta) y el corpus del arranque
bash scripts/console/mutar-regexp-216.sh /tmp/gox-00216 /tmp/dcbuild/motor-216-mut 15
bash scripts/console/checar-init-215.sh /tmp/gox-00216 /tmp/dcbuild/motor-216 15

# La barrera
GODC_GOX=/tmp/gox-00216 SALTAR_STDLIB=1 bash scripts/console/build-motor-dc6.sh /tmp/dcbuild/motor-dc6-216
bash scripts/console/compare-motor-dc6.sh /tmp/dcbuild/motor-dc6-216 d2logo
```

Evidencia con las salidas crudas (los dos errores del host, la sonda roja y verde, los tres mutadores y
las 9 marcas del arranque): `evidencia/dc-002.16-regexp-bandera-i.txt`.

## 33. DC-002.17: el motor ya llega a `main` y lo frenaba el ENTORNO — el video pedia 1 MiB que no habia y el romdisk no dejaba escribir

DC-002.16 dejo el arranque del motor llegando a `main` con **9 marcas `MOTOR-ARRANQUE` y 0 panics**. Esta
tarjeta cerro los dos defectos del ENTORNO que quedaban delante (no del codigo del motor): el video que no
arrancaba porque el backend pedia 1 MiB del monton que no existen, y el `Run()` que moria porque el romdisk
de KOS es de solo lectura y el motor escribe su configuracion. El resultado medido: **10 marcas, video
arrancado, el motor creando y guardando su configuracion**, y el defecto siguiente identificado con nombre y
apellido (`json.MarshalIndent` no puede sin reflexion).

**1. El pedido, medido.** El arranque llega a `main`, llama a `d2app.Create().Run()` y muere con
`operation not supported on read-only filesystem`. Con el medidor puesto en los stubs de escritura de
`console/dc/std/os`, el log dice cual es, y es UNO:

    OS-ESCRITURA MkdirAll path= /rd/OpenDiablo2
    MOTOR-ARRANQUE Run() termino con error: operation not supported on read-only filesystem

La cadena: `d2app/app.go:264` no encuentra `config.json`, arma el default y lo guarda
(`creating default configuration file at /rd/OpenDiablo2/config.json...`), y `d2core/d2config/d2config.go:52`
lo primero que hace en `Save()` es `os.MkdirAll(filepath.Dir(c.path), 0750)`. Ese `MkdirAll` es el unico
error de esa funcion: **el motor no arranca porque no puede crear el directorio donde guarda su
configuracion**. De la misma corrida: `Rom disk image at 0x0 is not a ROMFS image` (el romdisk del ELF no
es una imagen ROMFS valida).

**2. El video no arranca por MEMORIA, no por el PVR.** Dos lineas antes del `codigo -3`:

    Out of memory. Requested sbrk_base 0x8d0a9000, was 0x8cfa9000, diff 1048576

1 MiB exactos que el monton pide y no hay; lo dice el runtime de KOS, no nuestro codigo. El `-3` del PVR
es el sintoma. Esto ata los puntos 2 y 3 de la tarjeta: cualquier sistema de archivos en RAM tiene que
salir de la MISMA memoria que ya falta. Para dimensionarlo: el ELF que abre Flycast mide 13.710.848 bytes
y la Dreamcast tiene 16 MiB.

**3. Por que el medidor no se veia (dos causas, las dos medidas; costo 3 corridas).**

  a) **La llamada estaba INLINADA.** gccgo compila con `-O2` (`env.sh`), asi que `os.MkdirAll` (una linea)
     quedo copiada adentro de `d2config.gox`, compilada ANTES del medidor. Medido con `sh-elf-nm`:
     `d2config.gox` tiene `U _os.ErrNoSePuede` y **no** tiene `U _os.MkdirAll`. Recompilar solo `os.gox`
     no alcanza: hay que recompilar tambien el paquete del motor que la tenga inlinada (lo hace
     `compilar_paquete_motor()` del medidor, misma receta que `build-motor-gox.sh`).
  b) **El `println` no puede concatenar aca.** `println("a" + que + "b")` arma una cadena nueva en el
     monton, y el monton no tiene 1 MiB ([2]): la version que concatenaba no imprimia NADA. Con
     argumentos sueltos (`println("OS-ESCRITURA", que, "path=", donde)`) sale. Regla para los medidores de
     aca en adelante: **imprimir con argumentos sueltos, nunca concatenando**.

**4. Una trampa del arnes (bloqueaba la medicion, arreglada).** `build-motor-arranque.sh` no podia
enlazar: `multiple definition of ..._d2dat.Load..f` con el MISMO `d2dat.gox` como "first defined here".
Causa: `orden.txt` tenia **74 lineas** (63 de stdlib + 11 del motor) porque `build-motor-gox.sh` anota a
proposito los paquetes del motor en `orden.txt`, y el enlace del arranque usa orden.txt **y**
orden-motor.txt. `grep -vxF -f orden-motor.txt orden.txt` deja **64 lineas** con la interseccion vacia, y
el script ahora filtra (no-op cuando orden.txt esta limpio). El mismo patron esta en
`checar-recorte-214.sh:146` y `medir-alcance-motor.sh:34`, anotado y sin tocar. Se contamino sin que nadie
lo viera porque el ultimo enlace bueno de DC-002.16 fue ANTES de la barrera `build-motor-dc6.sh`.

```bash
# La medicion completa (recompila os.gox con el medidor + d2config, reenlaza y corre Flycast)
export GODC_GOX=/tmp/gox-00216
bash scripts/console/medir-escritura-217.sh /tmp/gox-00216 /tmp/dcbuild/motor-217 20
```

Evidencia cruda (log, la cadena del codigo y los `sh-elf-nm`): `evidencia/dc-002.17-escritura-config.txt`.

**5. El video: 1.048.576 bytes del MONTON a 2.048 bytes de .bss y 300 subidas de 2 KiB.** El `-3` no era el
PVR: era nuestro `calloc` del cuadro entero (`1024x512x2 = 1.048.576`, exactamente el `diff` del mensaje de
KOS). La cuenta de la memoria, medida: los segmentos que carga el ELF son **13.915.392 B**, el `.bss` termina
en `0x8cd5b500`, el tope del `sbrk` es `0x8cff0000` (16 MiB menos los 64 KiB de pila del kernel, `mm.c`), o
sea **2.706.176 B de monton** de los que el motor tenia **266.240 libres** y pedia **1.048.576** (3,94 veces
lo libre). El arreglo: un bloque de 32x32 SI es contiguo en la textura twiddleada (los 5 bits bajos de cada
coordenada dan los 10 bits bajos del indice), asi que el cuadro se arma **de a un bloque** en `conv_tile`
(2.048 B de `.bss`, alineado a 32 para las Store Queues) y cada bloque se sube con `pvr_txr_load`. El relleno
se deja en cero una vez en el arranque (512 bloques).

**6. La colocacion de los pixeles, probada con una cuenta y no con un ojo.** 300 subidas por cuadro mueven el
cuadro: si un bloque fuera al indice equivocado, la pantalla se veria mal y el arranque no se enteraria (el
video "arranca" igual). `scripts/console/medir-twiddle-217.py` compara el indice de **cada** pixel visible
calculado por la formula vieja contra la nueva: **307.200/307.200 iguales y 300/300 bloques con 1.024 indices
contiguos y sin repetir**. El costo, medido con el `EBITEN-PERFIL` del POC del cuadro:

| | dibujar_us | convertir_us | subir_us | cuadro_us |
|---|---|---|---|---|
| con el `pvr_pantalla.c` viejo | 27415 | 47806 | 2625 | 77848 |
| con el de esta tarjeta | 27418 | 35100 | 0 | 62521 |

La conversion baja 26,6% (ya no recorre el relleno: 217.088 de los 524.288 pixeles, el 41%) y la subida queda
en 0 porque va dentro de la conversion. **El cuadro queda 1,24x mas rapido** y la huella de la pantalla es la
misma (`-1192069352`).

**7. El sistema de archivos escribible: un overlay en RAM de 18.020 B de `.bss`, cero del monton.** En
`os_dc.c`: 8 archivos x 2.048 B + 8 directorios, con crear/truncar, escribir, leer con seek, tamano, borrar,
renombrar, truncar y "existe"; lo que se escribe en la RAM **tapa** al romdisk con el mismo nombre y lo que no
esta en RAM se lee de KOS. **No usa el monton a proposito**: quedan 266.240 B y el GC se lleva 1 MB de
semiespacios, asi que un tmpfs con `malloc` volveria a pelear por la memoria que falta; con presupuesto
estatico el limite se dice con un numero. Medido en el arranque del motor: `OS-RAMFS mkdir-ok path=
/rd/OpenDiablo2` y `create-ok .../config.json` (antes moria en el `MkdirAll`).

**8. La escritura probada de verdad, porque el motor no llega a escribir.** `d2config.Save()` se cae en
`json.MarshalIndent` (el `encoding/json` de la consola no tiene reflexion) **antes** de escribir un byte, asi
que el arranque solo prueba el mkdir y el create. El propio arranque corre una prueba directa con el mismo
paquete `os`: mkdir, create, escribir 67 bytes, cerrar, leer de vuelta, comparar y borrar -> `escribi 67 bytes,
lei 67 igual= true` y `archivos= 0 bytes= 0` despues de borrar. **El defecto que queda, con el texto exacto**:
`json: en la consola no hay reflexion: solo se pueden serializar los tipos que saben hacerlo ellos mismos
(MarshalJSON/UnmarshalJSON)` -> tarjeta nueva: no es del sistema de archivos ni del motor, es del paquete
`encoding/json` de la consola.

**9. El corpus y la barrera.** `scripts/console/checar-entorno-217.sh`: **3 controles** (video sin `-3` ni
`Out of memory`; el motor pudo con su config y no hay `read-only filesystem`; la prueba de escritura y lectura
de vuelta) y **2/2 mutantes cazados** — `sin-overlay` (se deshace el overlay en el C y los controles 2 y 3 se
ponen ROJOS: vuelve el "operation not supported on read-only filesystem") y `calloc-1mib` (vuelve el buffer
del cuadro entero y el control 1 se pone ROJO con el `diff 1048576` y el `codigo -3`). Los dos mutantes son de
C, asi que cada corrida no recompila los `.gox`. La barrera **no se movio: 31 lineas identicas** entre host y
consola (`compare-motor-dc6.sh`, 30 frames), y del lado del host `test-std-host.sh` quedo **verde en amd64 y en
386** (los 13 paquetes que hablan con KOS siguen sin compilar en el host: `os` incluido) con
`mutar-std-host.sh` en **36/36 chequeos cazados**.

```bash
# El arreglo y su corpus: 3 controles + 2 mutantes + control sano de vuelta
export GODC_GOX=/tmp/gox-00216
bash scripts/console/checar-entorno-217.sh /tmp/gox-00216 /tmp/dcbuild/motor-217 20
# La prueba de que los pixeles caen donde caian (307.200/307.200)
python3 scripts/console/medir-twiddle-217.py
# La barrera
bash scripts/console/build-motor-dc6.sh /tmp/dcbuild/motor-dc6-217
bash scripts/console/compare-motor-dc6.sh /tmp/dcbuild/motor-dc6-217 d2logo
```

**Una trampa de instrumento, medida con A/B (no es del arreglo).** La captura del POC `motor-pantalla` daba
el borde de arriba del marco amarillo completo (446/446 px) y el de abajo en **0/446**: la ventana de Flycast
en esta sesion mide 446x334 (escala 0,6969 sobre 640x480) y el marco de 1 pixel queda mezclado por el
reescalado. Con el C VIEJO (`git show HEAD:...`) la misma captura da exactamente lo mismo, asi que el defecto
es del instrumento y no del cambio: por eso la colocacion se prueba con la cuenta (307.200/307.200)
y no con los pixeles reescalados.

Evidencia cruda (los logs, las cuentas de la memoria, el corpus y la barrera):
`evidencia/dc-002.17-video-y-overlay.txt` (el arreglo) y `evidencia/dc-002.17-escritura-config.txt` (la
medicion que lo abrio).


## 34. DC-002.18: el motor arranca su entorno y lo frenaba el SERIALIZADOR (el `config.json` sin reflexion)

DC-002.17 dejo el arranque creando su `config.json` y muriendo al ESCRIBIRLO: `json.MarshalIndent` del
`d2config.Configuration` devolvia `ErrSinReflexion`. Esta tarjeta puso el instrumento, escribio el
serializador a mano y volvio a medir. El resultado: **el config se guarda (el overlay pasa de 0 a 549
bytes) y el error que queda ya NO es de serializacion sino la falta de los archivos del juego**.

**1. Los dos numeros, medidos (el "antes" es el arreglo sacado, no una deduccion).**
`scripts/console/medir-arranque-218.sh` corre el mismo POC con el `MarshalJSON` puesto y con el
`MarshalJSON` desactivado (el mutante `sin-marshal` = el estado del arbol en DC-002.17, commit 34e35a36):

    marcas MOTOR-ARRANQUE .............. 10 ANTES -> 10 DESPUES
    serializaciones (JSON-CONSOLA) ..... 1 ANTES  -> 1 DESPUES   (MarshalIndent sobre *d2config.Configuration)
    ese tipo sabe serializarse? ........ sabe= false -> sabe= true
    el overlay del motor ............... archivos= 1, bytes= 0 -> archivos= 1, bytes= 549

    ANTES:   Run() termino con error: json: en la consola no hay reflexion: solo se pueden serializar los
             tipos que saben hacerlo ellos mismos (MarshalJSON/UnmarshalJSON)
    DESPUES: Run() termino con error: reported to the user: no Diablo II game files were selected

Contar marcas NO alcanzaba para ver el arreglo (son las mismas 10): lo que lo ve son el TEXTO del error y
los BYTES del overlay. El error que queda es el de la tarjeta siguiente (los MPQ y el mapa), no un defecto
de serializacion.

**2. El arreglo, y por que esa opcion.** `MarshalJSON`/`UnmarshalJSON` escritos a mano en el paquete del
motor (`d2core/d2config/d2config_json.go`, opcion (a) de la tarjeta), aplicados con el script idempotente
`scripts/console/fix-d2config-json-218.py` (no pisa un archivo distinto y reproduce el archivo con el mismo
sha256 desde cero). SE ELIGIO MIDIENDO: los dos caminos del motor (el `Save` y el `Decode`) pasan por el
`Marshaler`/`Unmarshaler` del tipo, asi que con los dos metodos alcanza y no hay que enseñarle al json de la
consola ningun tipo del motor. Lo unico que el json de la consola necesitaba era el `Indent`, que si se
puede hacer sin reflexion porque trabaja sobre el TEXTO json.

**3. El defecto ESCONDIDO del `Decoder` (el que la tarjeta anotaba, confirmado).** El `Decoder.Decode` del
json de la consola devolvia `ErrSinReflexion` SIEMPRE: el motor se caia igual al LEER un config que
existiera. Con el mutante `sin-decode` se mide el antes y el despues:

    JSON-CONFIG prueba FALLO: Decode: json: en la consola no hay reflexion: ...

**4. El corpus de la consola, byte a byte contra el host.** `scripts/console/checar-json-218.sh` (POC
`console/dc/poc/json-config`) usa el `Configuration` del motor de verdad y compara lo que queda en el
archivo contra los bytes que HORNEA el `json` de Go en el host sobre una struct espejo sin metodos
(`console/dc/gen-config-json`), que es la unica referencia honesta (comparar contra `json.Marshal(c)` seria
compararse consigo mismo). Cuatro controles, los cuatro VERDES: guardar (`574 = 574`), leerlo con el
`Decoder` y reescribirlo (574 iguales), el `Encoder` ((422 bytes, y lo escrito se lee de vuelta igual) y la
variante de bordes ((441 = 441): lista `nil`, campos vacios, negativos, comillas y `<html>`, una lista
VACIA y una clave que se omite. En la corrida, 12 serializaciones, todas sobre `*d2config.Configuration` y
con `sabe= true`.

**5. Los mutantes (una suite verde no prueba nada).** En la consola, 4/4 cazados: `sin-marshal` (los cuatro
controles ROJOS con el error del defecto original), `sin-decode` (control 1 verde, 2, 3 y 4 rojos),
`sin-indent` (`bytes= 421 esperados= 574`, primera diferencia en el byte 1) y `sin-campo` (sin `D2SDir`:
`bytes= 558`, diferencia en el byte 385 — es el que dice que la comparacion byte a byte discrimina). En el
host, el mutador quedo `39/39` con los tres nuevos de esta tarjeta (37 `MarshalIndent` sin reacomodar, 38
`Decode` sin probar el `Unmarshaler`, 39 el instrumento mudo). Dos trampas anotadas: un mutante que NO
compila no prueba nada (el script ahora exige que el control caiga compilando), y una lista de limites que
no es la real es una mentira con formato de dato (el limite de UTF-8 del `Indent` se midio y NO era limite:
el `Indent` de Go tampoco valida).

**6. La barrera y el motor.** El motor compila **142 de 142, 0 bloqueados**. La barrera del hito, con el
`d2config.gox` NUEVO: `compare-motor-dc6.sh` -> **IGUAL, 31 lineas identicas, 30 frames**. `test-std-host.sh`
-> verde en amd64 y 386 (los 13 paquetes que hablan con KOS siguen sin compilar en el host, que es parte de
la prueba).

**7. El instrumento, para las proximas tarjetas.** `json.Instrumento` es un punto de enganche en el json de
la consola (no un `println`: el NOMBRE del tipo no se puede sacar sin reflexion) y cubre los cinco puntos
(`Marshal`, `MarshalIndent`, `Unmarshal`, `Encode`, `Decode`); los POC de la consola lo enchufan y leen el
descriptor de tipo de gccgo con el layout de las sondas de DC-002.2f (`sonda-g`), que es lo que permite
imprimir `tipo=` y `sabe=`. En el host queda en `nil` y no imprime nada.

Evidencia cruda (los logs de las dos corridas del arranque, el del corpus con sus mutantes y el de la
barrera): `evidencia/dc-002.18-serializador-config-sin-reflexion.txt`.

## 35. DC-002.19 (MEDICION, tarjeta abierta): la seleccion de los archivos del juego en la consola

Cerrada DC-002.18, el error que queda es `reported to the user: no Diablo II game files were selected`
(`d2app/firstrun.go:32` -> `d2setup.EnsureGameFiles`). Esta seccion es la MEDICION de esa frontera hecha
ANTES de tocar nada; el arreglo todavia no esta.

**1. Los caminos de seleccion que el motor ya tiene NO SIRVEN en la consola, y esta medido por que.**
`d2setup.DefaultUI()` elige entre `ScriptedUI` (sin dialogos, con la carpeta de `OD2_SETUP_AUTOPICK`) y
`OsaUI` (dialogos nativos por `osascript`; `ui_native_other.go`). Medido: `os.Getenv("OD2_SETUP_AUTOPICK")`
devuelve **""** (el `os.Getenv` de la stdlib minima es un stub), y `OsaUI.ChooseFolder` devuelve
**`cancelled by user`**. O sea que en la consola no hay quien elija la carpeta, ni por env ni por dialogo.

**2. Un defecto NUEVO del `os` de la consola, medido contra el motor de verdad.** Sobre la MISMA carpeta con
los siete MPQ creados:

    Validate(dir).OK() = false   archivos que ve = 0     <- os.ReadDir del overlay de RAM
    Complete(dir, orden) = true                          <- os.Stat de cada nombre

No es cosmetico: el camino de la seleccion del motor (`Validate` -> `Resolve`) pregunta justamente por el
contenido del directorio, asi que la carpeta buena se veria VACIA aunque hubiera quien la eligiera.

**3. La frontera se movio: el motor PASA la seleccion y el error nuevo es de otro paquete.** Con el config
escrito por el motor (`DefaultConfig` + `MpqPath` + `MpqLoadOrder` + `Save`, o sea lo que dejaria una
seleccion exitosa) y los MPQ presentes (vacios: `Complete` mira nombres, no contenido):

    [File Loader][INFO] Loaded /rd/OpenDiablo2 -> config.json
    [App][INFO] loaded configuration file from /rd/OpenDiablo2/config.json
    panic: assets: bmp.Decode failed: gzip: invalid header

Dos cosas para quedarse: **el camino de LECTURA del arranque quedo medido en el motor de verdad** (era el
hueco que DC-002.18 dejo declarado: el `Decode` lee el config y el motor lo dice), y la frontera nueva es el
`compress/gzip` de la consola sobre un asset EMBEBIDO (`d2common/d2util/assets.CreateTextImage`), ANTES de
cualquier lectura del MPQ.

Ejecutable: `bash scripts/console/medir-setup-219.sh /tmp/gox-00218 /tmp/dcbuild/setup-219 25`
(POC `console/dc/poc/setup-archivos`). Evidencia cruda: `evidencia/dc-002.19-seleccion-de-archivos.txt`.
