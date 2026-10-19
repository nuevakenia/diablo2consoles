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
  **OJO (DC-002.26): el twiddle propio que se escribio aca quedó con los bits de X y de Y AL REVES** (la
  convencion del PVR, la de KOS `pvr_texture.c:34`, pone la X en los bits IMPARES) **y la tabla era de 512**
  cuando el motor pide una textura 1024x1024. Eso hizo que el cuadro del motor se viera ROTADO 90 grados
  hasta la seccion 42. El numero de arriba (302 us -> 2.624 us) no esta en duda: lo que estaba mal era la
  orientacion del cuadro y el tamano de la tabla.
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

**OJO — RE-MEDIDO EN DC-002.25 (seccion 41): las filas de 512 y 256 de esa tabla se midieron con la copia
por Store Queue del GC ROTA** (DC-002.24 la encontro: el area de la Store Queue salia 0 = la BIOS, las
escrituras se descartaban y el conjunto vivo del GC era practicamente VACIO, no se copiaba nada). Con la
copia arreglada la escalera **se da vuelta**: con el semiespacio en **256 KB el arranque ya NO alcanza**
(`fatal error: out of memory`, 10 marcas de 11, 0 cuadros del loop) y con **512 KB completa cuadros** (3,
11 marcas, sin OOM). La tabla se deja tal como se midio, con su fecha; la medicion corregida esta en la
seccion 41.

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
contiguos y sin repetir**. **OJO (DC-002.26): esa cuenta compara la conversion del cuadro entero contra la
conversion por bloques y las DOS usan la misma convencion, asi que no puede ver una traspuesta** (los bits de
X y de Y estaban al reves y las dos puntas coincidian igual: el cuadro salia rotado 90 grados; ver la seccion
42, que agrega `verificar-twiddle-226.py`, que compara contra la convencion de KOS). El costo, medido con el
`EBITEN-PERFIL` del POC del cuadro:

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

## 35. DC-002.19: el arranque PASA la seleccion de los archivos del juego (`os.ReadDir` del overlay y el romdisk que hornea la config)

Cerrada DC-002.18, el error que queda es `reported to the user: no Diablo II game files were selected`
(`d2app/firstrun.go:32` -> `d2setup.EnsureGameFiles`). La seccion tiene las tres partes: lo MEDIDO antes
de tocar nada (35.1), el ARREGLO (35.2) y lo medido despues, con los controles y los mutantes (35.3).

### 35.1 Lo medido ANTES del arreglo (no se toco nada todavia)

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

Ejecutable (la medicion): `bash scripts/console/medir-setup-219.sh /tmp/gox-00218 /tmp/dcbuild/setup-219 25`
(POC `console/dc/poc/setup-archivos`). Evidencia cruda: `evidencia/dc-002.19-seleccion-de-archivos.txt`.

### 35.2 El arreglo: dos defectos, uno de la consola y uno del arranque

**(a) `os.ReadDir` del OVERLAY de la RAM.** El `ReadDir` de la consola salia SOLO del `.manifiesto` que el
build hornea en el romdisk, asi que un directorio creado o escrito por el programa (el overlay en RAM de
DC-002.17) se veia VACIO. Ahora hay dos fuentes y se juntan:

- en C, `d2c_ramfs_manifiesto(buf, max, ruta)` (`console/dc/std/csrc/os_dc.c`) arma el MISMO formato del
  manifiesto (`nombre TAB bytes TAB archivo|directorio`) con las entradas DIRECTAS del directorio del
  overlay, sin bajar a los subdirectorios, sin listar `.manifiesto` y devolviendo NEGATIVOS distintos por
  cada salida (`-1` no entra, `-2` argumentos, `-3` ruta larga, `-5` la ruta no llega como cadena, y
  `-(100 + 10*archivos + dirs)` como instrumento cuando no matcheo nada). Ojo con la FIRMA: la ruta va
  ULTIMA (escalares primero) -- con la ruta primero el C recibia los argumentos corridos y devolvia `-2`.
- en Go, `os.ReadDir` (`console/dc/std/os/os.go`) junta el listado del overlay con el del romdisk: el
  overlay TAPA al romdisk por nombre (la misma regla de `os.Open`/`d2c_hay`) y el resultado se ordena por
  nombre (Go lo promete). Un directorio que no esta en ninguno de los dos da `ErrNotExist`, no lista vacia.

**(b) La configuracion del arranque, horneada.** `scripts/console/build-motor-arranque.sh` arma un romdisk
con `/rd/OpenDiablo2/config.json` y `/rd/juego`, y el `config.json` lo escribe `console/dc/gen-config-consola`
con el `d2config.Save` DEL MOTOR (el `MarshalJSON` de DC-002.18) usando la tabla del motor
(`d2setup.CoreMPQs`) como unica fuente de los nombres. Con eso `Complete(MpqPath, MpqLoadOrder)` = true y
`EnsureGameFiles` no pregunta nada. Los siete MPQ se crean VACIOS: es el instrumento que corre la frontera
hacia adelante (los MPQ reales no entran en el romdisk; el camino de lectura real es el disco).

El generador tiene tres salidas mas: `-sin-sellos` (config sin archivos: el mutante de la seleccion),
`-verificar` (EL CONTROL DEL HOST: lee el config con el `json` de verdad y exige ruta, orden y archivos).
Y `build-motor-arranque.sh` acepta `DC_SIN_ROMDISK=1` / `DC_SIN_CONFIG=1` para reproducir el estado
anterior al arreglo.

### 35.3 Lo medido DESPUES: 6/6 controles, 5/5 mutantes, 87/87 casos y la barrera igual

    bash scripts/console/checar-arranque-219.sh /tmp/gox-00219 /tmp/dcbuild/arranque-219 25

       OK: 6/6 controles verdes con el codigo de ahora
         MOTOR-ARRANQUE config del arranque: MpqPath= /rd/juego archivos en el orden= 7
         MOTOR-ARRANQUE seleccion: Complete= true Validate.OK()= true archivos que ve= 7 faltan= 0
         MOTOR-ARRANQUE overlay del listado: archivos= 1 bytes= 12 dirs= 2
         MOTOR-ARRANQUE seleccion overlay: ReadDir de /rd/prueba-seleccion -> 1 entradas: uno.txt
         [App][INFO] loaded configuration file from /rd/OpenDiablo2/config.json
         panic: assets: bmp.Decode failed: gzip: invalid header
         (marcas MOTOR-ARRANQUE de la corrida: 10)
    == 5/5 mutantes cazados ==

Los seis controles son: 1 la config del arranque, 2 la seleccion (`Complete` **y** `Validate`, que es el
que mira el CONTENIDO del directorio), 3 el `ReadDir` del overlay, 4 la lectura del config POR EL MOTOR
(`loaded configuration file from ...`), 5 la FRONTERA declarada (el `gzip` del asset embebido, que es
DC-002.20: el control se pone ROJO a proposito el dia que se arregle) y 6 el numero exacto de marcas.

Los cinco mutantes: `DC_SIN_ROMDISK=1` (el estado del arbol antes del arreglo: caen 1, 2, 4 y 6),
`DC_SIN_CONFIG=1` (caen 1, 2, 4 y 6), `DC_SIN_SELLOS=1` (cae 2 y 6; el 1 sigue verde, la config esta),
`overlay-no-lista` en Go (cae 3) y `ramfs-no-matchea` en C (cae 3). Los dos ultimos son el mismo defecto
de los dos lados del puente con KOS.

El corpus del paquete (`console/dc/poc/vfs`, caso 4m) queda en **87/87** casos y
`scripts/console/mutar-os-consola.sh` en **12/12** mutantes cazados (los dos nuevos bajan el POC a
`81/87` y `81/83`), con `os.go` (sha256 `99e3041f…`) y `os_dc.c` (`b7afa0c6…`) restaurados.

Las barreras del host siguen verde: `test-std-host.sh` en amd64 y 386 (13/13 excluidos que no compilan en
el host) y `mutar-std-host.sh` con **39/39** chequeos cazados. Y la del motor, `compare-motor-dc6.sh`:
**31 lineas identicas, 30 frames**.

Evidencia cruda del arreglo: `evidencia/dc-002.19-arreglo-seleccion.txt`.

**Lo que queda, declarado.** La frontera nueva es el `compress/gzip` de la consola sobre un asset
EMBEBIDO (`d2common/d2util/assets.CreateTextImage`), antes de cualquier lectura del MPQ: es DC-002.20. Los
MPQ del romdisk estan vacios: la lectura de los datos del juego (disco u horneada) es otra tarjeta. El
`os.Getenv` de la consola sigue siendo un stub; no se toco porque el arranque ya no lo necesita para la
seleccion.

## 36. DC-002.20: el gzip de la consola contra el asset EMBEBIDO (el tamano que se hacia pasar por cabecera)

El arranque ya pasaba la seleccion de los archivos del juego y moria con `panic: assets: bmp.Decode failed:
gzip: invalid header`. Esta tarjeta midio ese `invalid header`, lo arreglo y re-midio el arranque: el panic
no esta mas y la frontera nueva es la RAM.

**1. La causa no era la cabecera, era el TAMANO, y esta medido en las dos puntas.** El asset embebido del
motor (`assets.CompressedDebugText`) son **2.209 bytes** (crc32 `aa973211`) que descomprimen a **98.358**
(crc32 `5dce3e98`, un BMP de 256 x 128 x 24 bits, el atlas de texto). El `gzip` de la consola descomprimia
de UNA sola vez, con el destino que calcula `d2c_zlib_inflate_bound` = **4 x comprimido + 64 KB = 74.372**
bytes. Es una REGLA, no una cota: el resultado no entraba, la zlib devolvia error, el puente lo traducia a
`-1` y el paquete a `ErrHeader` -> `gzip: invalid header`. Medido tambien del lado de la consola:

    GZIP-ASSET embebido bytes= 2209 crc32= 2862035473          (0xAA973211, igual al host)
    GZIP-ASSET viejo cota= 74372 devuelto= -1 entra_el_real= false

**2. El arreglo: inflate EN STREAMING, con el destino en el buffer del que se lee.**
`console/dc/std/csrc/zlib_dc.c` gana cuatro `z_stream` en `.bss` prestados por INDICE (no se guardan
punteros a estado de C en el monton de Go; es la regla de los portadores `uintptr` del proyecto) con
`d2c_zlib_stream_init` / `_step` / `_consumido` / `_fin` / `_end`, y el mismo `inflateInit2(15 + 32)` que
decide entre zlib y gzip. El resultado del paso se lee con funciones APARTE en vez de punteros de salida: un
puntero de salida mal pasado no se nota, un contador que no avanza si. `console/dc/std/compress/gzip`
reescribe su `Reader.Read` para pedir bytes en el `p` del que lee, con el bucle que cubre el caso normal del
inflate (un paso que consume todo el comprimido y escribe CERO, otro que devuelve menos de lo que entra) y
error declarado si un paso no avanza por ningun lado (mejor que colgar el motor en un bucle). `Close()`
devuelve el slot. El camino de una sola vez se queda para `compress/zlib`, con su limite declarado en el
control 5 del corpus.

Medido en Flycast con el destino de **512 bytes** (193 lecturas), para probar que entrega por partes y no de
un golpe:

    GZIP-ASSET nuevo bytes= 98358 crc32= 1573797528 lecturas_de_512= 193
    GZIP-ASSET nuevo cabecera= 66 77 54 128   (BM)
    GZIP-ASSET motor bmp.Decode OK ancho= 256 alto= 128

**3. El arranque re-medido: la frontera se movio a la RAM.** Las mismas 10 marcas `MOTOR-ARRANQUE`, la
seleccion igual de verde, el config leido por el motor, y donde antes estaba el panic del gzip ahora:

    Out of memory. Requested sbrk_base 0x8cff6000, was 0x8cfde000, diff 98304
    fatal error: gc_external_alloc: OOM
    Memory: arena=2579164 used=2576068 free=3096

El atlas de texto YA se descomprime y lo que no entra es su imagen: 98.304 bytes contiguos contra 3.096
libres de arena. Es DC-002.21, abierta con ese numero.

**4. El corpus (`scripts/console/checar-gzip-220.sh`) y su referencia del host.** Siete controles: el 0 corre
`go run ./console/dc/gen-asset-info` (el `compress/gzip` y el `crc32` DE VERDAD de Go) y exige 2.209 /
98.358 / `5dce3e98` / 256x128; el 1 que la consola vea los MISMOS bytes embebidos; el 2 los 98.358 bytes con
el CRC32 del host leidos de a 512; el 3 la cabecera `BM`; el 4 el camino del MOTOR (`gzip.NewReader` +
`bmp.Decode`) decodificando; el 5 que el camino viejo SIGA fallando (limite declarado, se pone rojo a
proposito el dia que la regla cambie); el 6 las 6 marcas exactas. Los tres mutantes (lector de Go, contador
del puente C, ventana de la zlib) ponen rojo el control 2. Resultado: **7/7 controles y 3/3 mutantes**.

Barreras: `test-std-host.sh` verde en amd64 y 386 (13/13 excluidos), `mutar-std-host.sh` con **39/39**
chequeos cazados y `compare-motor-dc6.sh` con **31 lineas identicas y 30 frames**. Evidencia cruda:
`evidencia/dc-002.20-gzip-asset-embebido.txt`.

## 37. DC-002.21: el atlas de texto ENTRA — el semiespacio del GC (512 -> 256 KB) y las dos paredes de RAM

El arranque ya descomprimia el atlas de texto (DC-002.20) y moria pidiendo **98.304 bytes contiguos** para su
imagen (un BMP de 256 x 128 x 24 = 98.358 bytes), con **3.096 libres** en el monton de C. Esta tarjeta midio de
donde sale ese presupuesto, movio la unica palanca que lo paga y **re-midio**: esa pared desaparecio y la que
queda es un pedido del MOTOR de **1.916.928 bytes**.

**1. La palanca y por que se paga doble.** `gc_init` (`runtime/gc_heap.c`) pide **DOS** bloques del MISMO
monton (`memalign(32, GC_SEMISPACE_SIZE)` x 2), asi que el semiespacio se paga dos veces: 512 -> 256 KB libera
**524.288** bytes de verdad. Medido con el motor entero en Flycast (las dos lineas son la re-medicion de esta
tarjeta; la corrida de DC-002.20 con 512 KB dio `arena=2579164 used=2576068 free=3096` — el `used` es
identico):

    KB=512   Out of memory ... diff 98304     Memory: arena=2579324 used=2576068 free=3256
    KB=256   Out of memory ... diff 1916928   Memory: arena=2415484 used=2372348 free=43136

**OJO — RE-MEDIDO EN DC-002.25 (seccion 41): las DOS lineas de arriba se midieron con el defecto de la copia
por Store Queue del GC PUESTO** (las escrituras de la copia se descartaban, el area salia siempre 0 = la
BIOS: ver la seccion 40). Con la copia rota el conjunto vivo del GC era practicamente VACIO y por eso "256 KB
alcanzaba". Con el GC SANO la escalera se da vuelta: **256 KB ya no alcanza** (`fatal error: out of memory`
durante `Create()`, 10 marcas de 11, 0 cuadros) y **512 KB completa cuadros** (3, 11 marcas, sin OOM, monton
de C: 3968 libres antes del motor y 40.776 al crear la pantalla). El resto de esta seccion (la palanca, el
corpus, las paredes declaradas) sigue valiendo; lo que cambia es el VALOR, y el corpus de la seccion 41 ya
no deja pasar un 256 KB como verde.

La pared de 98.304 **desaparece** (la imagen del atlas de texto entra) y la arena queda 163.840 bytes mas
chica, que NO es lo que libera el semiespacio: con 256 KB el arranque llega mas lejos y vuelve a consumir (el
atlas entra y el motor sigue cargando). Se declara tal cual, sin repartir esa diferencia a mano.

**2. La pared nueva esta declarada, NO atribuida.** Los 1.916.928 bytes (0x1D4000) son del motor pidiendo
memoria para sus datos, y **no esta medido que estructura es** (candidato medido en DC-002.14: la tabla de
`d2core/d2records`, 1.672.892 bytes, el 95,0% del `.data`). Tampoco es cosa de seguir bajando el semiespacio:
el ELF enlazado carga **13.992.908** bytes (0xd583cc) de los 16.777.216 — `.text` 4.997.464, `.rodata`
5.916.360, `.eh_frame` 1.073.276, `.gcc_except_table` 20.600, `.data` 1.760.444, `.bss` 224.640 — asi que la
palanca grande sigue siendo recortar el binario (la linea de DC-002.14) o leer los datos del disco en vez de
tenerlos en `.data`. 256 KB es el piso razonable *para esta etapa*: mas abajo el GC copia cada vez menos
memoria y ademas no alcanza (faltan 1,9 MB).

**3. Un hecho nuevo, medido: despues del OOM la consola REARRANCA.** El log con 256 KB tiene la secuencia del
arranque **cinco veces** (50 marcas `MOTOR-ARRANQUE` y 5 lineas `MOTOR-ARRANQUE arranca`); con 512 KB, una
(10 marcas). Por eso el corpus cuenta las marcas por ARRANQUE COMPLETO (total = 10 x arranques) y no "hasta el
primer OOM", que mediria el bucle de reinicio.

**4. El corpus (`scripts/console/checar-gc-221.sh`): 6 controles y su mutante.** El mutante de una tarjeta
cuyo defecto es un PRESUPUESTO es el **valor viejo del vendor** (512 KB), aplicado con el script idempotente
`fix-libgodc-gc.py 512 --build`. Los controles: (1) las 10 marcas por arranque completo; (2) la seleccion de
archivos sigue verde (`Complete= true Validate.OK()= true archivos que ve= 7 faltan= 0`); (3) el config lo lee
el motor; (4) **NO** esta la pared de 98.304; (5) **SI** esta la de 1.916.928; (6) el monton de C medido (arena
2.415.484 con 43.136 libres). El par 4+5 es el que no se puede pasar por verde solo: uno exige que la pared
vieja no este y el otro que la nueva si, con su numero. Resultado: **6/6 controles verdes y 1/1 mutante
cazado** (`fix-libgodc-gc.py` deja el vendor en 256 KB y rehace `libgodc.a`, que es lo que el Makefile no hace
solo: `%.o: %.c` no tiene dependencias de cabeceras). **DC-002.25 movio esa aguja: con el GC sano el vendor
queda en 512 KB y el mutante del corpus es ahora el 256 KB de esta seccion (ver la 41).**

**5. El corpus de DC-002.14 que quedo muerto, alineado.** `checar-recorte-214.sh` clavaba `KB_OK=512` y
salia con error si el vendor no estaba en 512: con el valor nuevo el corpus **no arrancaba**. Se alineo
(`KB_OK=256`, y el control 3 acepta las dos formas de "el arranque paso las paredes del runtime", con los
numeros de las dos etapas declarados en el propio script). Ademas sus dos reenlaces compilan el POC de HOY y
el gox de DC-002.14 ya no lo puede compilar (le faltan `os.RamfsResumen` de DC-002.17 y `json.Instrumento` de
DC-002.18), asi que usan el gox de la rama (`GOX_ACTUAL`). Corrida de esta tarjeta: **4 controles sanos y 2/2
mutantes cazados**. El mutante `gc-2048` (el valor de fabrica del vendor) es el comodin que vigila esta
tarjeta y sigue cazando: **con el GC en 2048 KB el arranque de hoy vuelve a morir en `gc_init` (diff
2097152)**. Tambien se corrigio el defecto del script del vendor que seguia imprimiendo un numero de DC-002.14
("quedan libres 610.304") y su valor por defecto paso de 512 a **256 KB** (el medido para el arranque de hoy).

Barreras: `test-std-host.sh` verde en amd64 y 386 (13/13 excluidos), `mutar-std-host.sh` con **39/39**
chequeos cazados y el arbol sano, `compare-motor-dc6.sh` **IGUAL** (31 lineas identicas y 30 frames) y
`checar-gzip-220.sh` con el control sano 7/7 y 3/3 mutantes (el gzip de DC-002.20 no se toca de rebote).
Evidencia cruda: `evidencia/dc-002.21-monton-semiespacio.txt`.

## 38. DC-002.22: la pantalla del motor deja de pagarse en el monton — el framebuffer en la VRAM

El pedido de **1.916.928 bytes** de DC-002.21 quedo medido (era el punto abierto del plan de esa tarjeta):
es **la pantalla logica del motor**, **800x600x4 = 1.920.000 bytes** exactos, que el `RunGame` del shim crea
con el tamaño del `Layout` del motor. El numero del OOM cuadra al byte: `1.920.000 - 3.072` (lo que el sbrk
ya tenia) `= 1.916.928 = 0x1D4000`. **No** era la tabla de `d2core/d2records` (el candidato por parecido de
numeros de DC-002.14). Esta tarjeta lo saca del monton: el framebuffer se reserva **contiguo en la VRAM del
PVR** (8 MB, con ~1 MB usado por el PVR) y el shim lo usa igual, porque memoria es memoria.

**1. Las tres partes del arreglo (dos en el shim, ninguna en el vendor).** (a) `d2c_pvr_pantalla_fb_init`
reserva `w*h*4` con `pvr_mem_malloc` y lo pasa a Go con `unsafe`; (b) `IniciarPantalla` pasa a ser
re-dimensionable — el POC arranca el video en 640x480 y el motor despues en 800x600, y eso dejaba la textura
de 640x480 contra un framebuffer de 800x600 (la conversion devolvia -2: defecto propio de DC-002.20/21 que
esta tarjeta arreglo); (c) `pvr_init_defaults()` se llama **UNA vez** (`pvr_listo`) porque el
re-dimensionable lo llamaba dos ("pvr_init called twice!"). Queda anotado que **`vid_set_mode` esta
PROHIBIDO** despues de inicializar el PVR: deja al chip esperando el vsync de otro framebuffer y
`pvr_wait_ready()` no vuelve nunca (cuelgue sin error). Del lado del paquete, `var pantalla =
NewImage(640, 480)` creaba **1.228.800 bytes** para una imagen que el motor **no usa** (nadie llama
`ebiten.Screen()` en todo el arbol: el `Draw` de la interfaz `Game` recibe la pantalla por parametro); ahora
es perezosa.

**2. Lo medido despues: el OOM se va y el motor ENTRA AL LOOP.** El arranque sano ya no tiene la pared y
llega al PVR de verdad:

    PVR-FRAMEBUFFER la pantalla vive en la VRAM: 1920000 bytes
    PVR-FRAMEBUFFER monton de C: libre 40936 bytes
    PVR-FRAMEBUFFER ida y vuelta: 65536 bytes, distintos: 0
    EBITEN-LOOP cuadro 0 esperando el cuadro del PVR
    EBITEN-LOOP cuadro 0 el PVR dio el cuadro, abro la lista

Contra el defecto: antes `diff 1916928` / `free=43136` y **5 arranques** (el OOM reiniciaba la consola, sin
llegar nunca al loop); ahora **sin OOM**, **1 arranque** y el cuadro 0 pedido al PVR. El monton de C esta al
tope igual (**4.128 libres** antes del motor, **40.936** al crear la pantalla) y no importa: el pedido
grande ya no sale de ahi.

**3. Dos cosas que estaban SUPUESTAS y ahora estan medidas.** La VRAM se escribe **de a un byte**: la ida y
vuelta de 64 KB da **0 distintos** (el compositor del shim escribe pixel a pixel y el PVR guarda su VRAM
como memoria de bloques: si no aguantara, el sintoma seria una excepcion del SH-4 dentro del motor, sin
pila que leer). Y el PVR se inicializa una sola vez. Las dos quedan como instrumento y como control.

**4. El error que queda, declarado con su direccion (el tramo siguiente).** El motor entra a
`updateInitError` (no tiene assets, asi que muestra su cuadro de error) y muere ahi: `target.Clear` se
completa (medido con una sonda en el shim: `Fill` entra y sale, y despues **ninguna** operacion mas del
shim, o sea no llega ni a `Size()`), `PushTranslation` pasa, y en `target.DrawTextf(...)`
(`d2app/app.go:718` -> `ebiten_surface.go:404` -> `s.Renderer().PrintAt` -> `d2util/debug_print.go:48`) el
SH-4 salta a basura: `Unhandled exception: PC 0000000b` / `kernel panic`, con `PR=8c445a90` que
`sh-elf-addr2line` resuelve a `d2app.App.updateInitError`. **No es un OOM**: es un salto a una direccion
invalida en el camino de la interfaz del renderer, y no esta medido cual de los dos lados es. Va como el
defecto abierto de la tarjeta siguiente, ya no tapado por el OOM.

**5. El corpus (`checar-pantalla-222.sh`): 7 controles y sus mutantes — y un mutante que NO cae.** Los
controles: la pantalla en la VRAM con sus 1.920.000; sin OOM; el motor entra al loop; abre la lista del
cuadro 0; la VRAM byte a byte; el PVR una sola vez; la seleccion de archivos sigue verde. El mutante del
defecto (el framebuffer de vuelta al monton, `mutar-pantalla-222.py --pantalla-al-monton`) **CAZA**: vuelve
el OOM. El mutante de la pantalla del paquete (`--paquete-al-arranque`) **NO CAE y es el hallazgo mas
util de la tarjeta**: con la gemela puesta el arranque **llega mas lejos** (completa los cuadros 0, 1 y 2
del loop y NO se cae) con el monton **200 bytes** mas chico (3.928 contra 4.128: el monton crece por sbrk
cuando se le pide, no paga 1,2 MB). O sea: **el crash del cuadro 0 no es falta de RAM, es el layout del
monton** (algo se lee distinto segun donde caiga todo). Un mutante que no cae es informacion: el corpus lo
imprime y lo declara en vez de exigir un verde. El mutador es idempotente y el corpus verifica que los dos
archivos queden con el **mismo sha256** del principio (para que no se commitee un mutante puesto).

**5b. El tercer dato del mismo patron, en el corpus de DC-002.21.** `checar-gc-221.sh` se **alineo** (su
control 5 exigia la pared de 1.916.928 que esta tarjeta quita, y el POC de hoy no enlazaba contra el gox de
DC-002.20: `reference to undefined identifier 'ebiten.MemLibreDelMonton'`): `GEN` es el gox de la rama, las
marcas por arranque pasan de 10 a 11 (la marca nueva del monton), el control 5 es "el motor ENTRA al loop" y
el 6 los numeros de hoy (4.128 / 40.936). Corrida: **6/6 controles verdes**. Y **su mutante (el semiespacio
en 512 KB, el valor viejo) queda NO CAZADO, con su razon medida**: con el framebuffer ya fuera del monton,
512 KB no ahoga nada y el arranque **completa 7 cuadros** del loop. Tres layouts, tres resultados del mismo
cuadro 0: **0 cuadros** (sano), **3** (la pantalla del paquete puesta), **7** (el semiespacio en 512 KB). El
presupuesto del GC lo sigue vigilando el comodin de `checar-recorte-214.sh` (con el GC en 2048 KB el arranque
muere en `gc_init`).

**6. Un defecto de instrumento, medido y anotado para no repetirlo.** El gox de esta tarjeta es propio
(`/tmp/gox-222`, porque `build-std-gox.sh` borra el directorio entero) y copiarlo arrastra las rutas
ABSOLUTAS de `orden.txt`/`orden-motor.txt`: la primera corrida enlazo los `.gox` **viejos** y dio el
defecto como si el arreglo no existiera (0 lineas `PVR-FRAMEBUFFER`). El corpus lo chequea antes de arrancar.

Barreras: `test-std-host.sh` verde en amd64 y 386, `mutar-std-host.sh` con 39/39 chequeos cazados,
`compare-motor-dc6.sh` **IGUAL** y `checar-gzip-220.sh` / `checar-gc-221.sh` siguen verdes (ni el gzip ni
el atlas se tocan de rebote). Evidencia cruda: `evidencia/dc-002.22-pantalla-en-la-vram.txt`.

## 39. DC-002.23: de que lado se rompe el error del motor — no es el texto, es el campo `errorMessage` del App

DC-002.22 dejo abierta una pregunta con direccion: el cuadro 0 del motor se caia con
`Unhandled exception: PC 0000000b` pintando el texto de su cuadro de error, y habia que medir si el salto
estaba en el **itab del metodo `PrintAt`** del renderer (la familia de DC-002.2b/2c/2f) o en el estado del
**`GlyphPrinter`** (`glyphImageTable`). Esta tarjeta lo mide y **la respuesta es: ninguno de los dos**.

**1. Lo medido, con la direccion del salto y sin deducir el offset.** El `jsr @r0` que se cae es el de la
**evaluacion del argumento** `a.errorMessage.Error()` de `d2app.App.updateInitError`: el motor ni llega a
`DrawTextf`. La sonda temporal dentro de `d2app` (que se quito al cerrar; ver
`evidencia/dc-002.23-error-del-motor.txt`) imprime las dos palabras del campo en los tres puntos del camino:

    loadEngine: errorMessage NO es nil   itab= 0x9         data= 0x8c912590  m0= 0x0
    updateInitError entra                itab= 0x9         data= 0x8c912590  m0= 0x0
    updateInitError antes de DrawTextf    itab= 0x9         data= 0x8c912590  m0= 0x0

Los **tres** puntos leen **lo mismo**: el campo ya viene mal, no se pisa entre que se guarda y se usa. El
`itab= 0x9` no es una tabla de metodos (un valor chico) y `m0 = *(itab+4) = 0`, asi que el salto del `jsr`
no es de ninguna funcion. Y el offset **no** es una deduccion del desensamblado: medido con
`unsafe.Offsetof` dentro del paquete, `errorMessage= 156`, `renderer= 124`, `config= 148`, `logger= 152`,
`options= 164`, `size= 168`. La lectura por offset del POC (`156`) y la del compilador coinciden.

**2. El valor que aparece en el campo dice que NO es un `error`.** En el layout que lo muestra, el campo
vale `itab= 0x8c4f3c60` / `data= 0xff000000`, y `sh-elf-nm` resuelve esa direccion contra el binario:

    8c4f3c60 R _image_1color.RGBA..d

O sea el **descriptor de tipo de `image/color.RGBA`**, con un color opaco (alfa 0xff) al lado: el par de
palabras de una **interfaz con un color**, no una tabla de metodos de `error`. El defecto no es "el error
del motor": es **memoria del `App` corrompida**, y por eso el sintoma cambia de forma segun el layout.

**3. La interfaz `error` de la consola esta BIEN, y por eso el defecto no esta ahi.** El POC nuevo
`console/dc/poc/iface-error` (sin el motor) mide **14 formas** de toparse con un `error` (multi-retorno con
literal nil, con variable nil, por dos niveles, con error real, error como primer valor, campo de struct,
tipo local, nil pelado y la **asignacion multiple a CAMPOS DE STRUCT** que es literalmente la forma de
`app.go:134`). Las nueve nil dan `itab= 0x0 data= 0x0`, las seis con error real dan itab en `.rodata` y
`m0` en `.text`, y el despacho devuelve los textos (`-> uno`, `-> guardado en un campo`). Los tres
constructores del arranque (`d2loader`, `d2records`, `d2asset`) devuelven error nil, medido con el motor.

**4. El mismo codigo, CUATRO sintomas: es el layout del monton.** Corriendo el motor entero en Flycast y
moviendo solo el layout (sondas de distinto tamano), el cuadro 0 se cae de cuatro formas distintas: el
`PC 0000000b` de DC-002.22 (binario de la rama, 0 cuadros completos, monton de C con 4.128 libres antes del
motor y 40.936 al crear la pantalla), `panic: growslice: cap out of range`, `panic: nil pointer dereference`
(cuando el campo queda nil, el motor **pasa** `loadEngine` y muere mas adelante) y `fatal error: concurrent
map read and map write`. Es el hallazgo de DC-002.22 (200 bytes de diferencia cambian el resultado) medido
ahora desde el otro lado: **no es el texto ni la interfaz del renderer, es memoria pisada**.

**5. El corpus (`scripts/console/checar-error-223.sh`) y su instrumento.** Tres controles (las 14 formas
del `error`; el campo del `App` con el motor: nil en los tres puntos y los tres constructores; y el
**defecto vigilado** del binario de la rama: llega al loop, abre el cuadro 0 y queda con **0 cuadros** y
`PC 0000000b` -- si algun dia deja de caerse asi, el control FALLA y hay que actualizar la medicion) y un
mutante: `mutar-error-223.py --offset-vecino` mueve el offset del POC de 156 a 152 (el campo `logger`, un
puntero no nil) y exige que las lecturas dejen de dar NIL -- si el POC siguiera diciendo NIL, el
instrumento no mediria lo que dice medir (la leccion de DC-002.8b).

**6. Lo que queda ABIERTO, declarado.** Esta tarjeta cierra la pregunta que la abrio y **no** cierra el
defecto: no esta identificado quien escribe en el campo entre `Create` y `loadEngine`. Lo medido para el
que siga: el valor escrito es una interfaz con `color.RGBA`, el `App` vive en el monton (`0x8cd8a218`,
`0x8cd91288`, `0x8cdc4490` en las tres corridas) y la escritura cae en su offset 156. El arreglo va como
tarjeta siguiente (por el flujo acordado: el defecto que aparece al cerrar una tarjeta se abre como tarjeta
nueva).

Barreras: `test-std-host.sh` verde en amd64 y 386, `compare-motor-dc6.sh` **IGUAL** (31 lineas identicas,
30 frames) y `checar-pantalla-222.sh` con sus **7/7** controles verdes y el mutante del OOM cazado (el
mutante B sigue declarado NO cazado, con su hallazgo). Evidencia cruda:
`evidencia/dc-002.23-error-del-motor.txt`.

## 40. DC-002.24: quien escribia en `App+156` — la copia por **Store Queue** del GC (y el arranque que vuelve a dar cuadros)

DC-002.23 midio **que** el campo `errorMessage` del `App` (offset 156 de 168) amanece con basura de otro
objeto y dejo abierto **quien** lo escribe. Esta tarjeta lo mide en el **runtime** (no en el codigo Go: una
sonda en Go mueve el layout del monton, que es justo lo que mueve el defecto de forma) y **lo arregla**.

**1. Quien escribe: el GC, no el motor — y escribe OTRO objeto encima del App.** La traza del runtime
(`scripts/console/fix-libgodc-campo-traza.py`, que reconoce el `App` por el **nombre de su descriptor de
tipo** y no por una direccion fija) muestra la cadena completa:

    el App quedo vigilado en 8cdd2778 (168 bytes, tipo App)
    pila sp=8ce102a4 hi=8ce10e48 usado=2980 puntero-al-App: dentro=4 fuera=0
    copia: el GC llega al App (8cd85028)  ->  el App SE COPIA (176 bytes)
    sq entrada:   dst=8cdc5040 src=8cd85020 tam=176 via_sq=1 dst_cab= 00000000 00000000
    sq salida-SQ: dst=8cdc5040 src=8cd85020 tam=176 via_sq=1 dst_cab= 00000000 00000000   <- el destino QUEDA EN CERO
    rechazo: el encabezado del App (8cdc5040) NO paso la validacion: tamano fuera de rango
    PISADO: el GC escribe un bloque de 288 bytes en 8cdc5048 que CUBRE el App

El App **si** esta referenciado por la pila (el GC lo alcanza, lo copia y actualiza el puntero), pero como
su **encabezado** quedo en cero el GC siguiente lo **rechaza** y copia otro objeto **encima**: ese bloque es
el que ensucia el offset 156 y el que hace saltar `a.errorMessage.Error()` a `PC 0000000b`.

**2. La causa de fondo: la formula del AREA de la Store Queue.** `gc_sq_copy()` (runtime/gc_copy.c) elige
el area de la Store Queue con `*QACR0 = (dst_addr >> 26) & 0x1C`. El KOS de este mismo toolchain usa
`QACR_EXTERN_BITS(dest) = ((dest) >> 24) & 0x1c` (`kernel/arch/dreamcast/hardware/sq.c`). Con `>> 26` sobre
una direccion del monton (`0x8C......`) el area sale **siempre 0**, y el area 0 es la **BIOS**: solo
lectura. Las escrituras de la Store Queue se **descartan en silencio** (por eso el destino queda como
estaba, en cero la primera vuelta de cada semiespacio, y no hay excepcion que lo delate): la via rapida
estaba rota para **todo** el monton.

**3. El arreglo: 2 lineas** (`scripts/console/fix-libgodc-sq.py`, ya **encadenado** dentro de
`fix-libgodc-gc.py`, asi que cualquier corrida de la cadena del runtime lo deja puesto):

    *QACR0 = (dst_addr >> 24) & 0x1C;    *QACR1 = (dst_addr >> 24) & 0x1C;

y ademas la invalidacion del destino (`dcache_inval_range`) antes del remanente que va por `memcpy`:
la Store Queue escribe **sin pasar por la cache D**. Declarado: Flycast **no emula la cache D**, asi que en
el emulador invalidad o no da igual (medido: con `--sin-inval` tambien da los mismos cuadros); se deja
puesta por el hardware real. `--sin-sq` apaga la via rapida entera (todo `memcpy`), la salida conservadora.

**4. Antes y despues (mismo instrumento, POC motor-arranque, 30 s de Flycast).** Antes: **0 cuadros**
completos del loop y `Unhandled exception: PC 0000000b, code 1`, con el destino de la copia en cero.
Despues: **3 cuadros completos** (`cuadro 0/1/2 listo`, 306.951 us el cuadro 2) y la copia del App con su
encabezado sano a los dos lados (`dst_cab= 190000b0 8ca5f680 src_cab= 190000b0 8ca5f680`). El `jsr @0x0b`
desaparece.

**5. La pared siguiente, que el defecto tapaba: el SEMIESPACIO del GC.** Con las copias sanas se puede
medir el conjunto vivo del arranque (antes no: no se copiaba nada):

    GC-TRAZA n=1  copiado=183776 antes=262144        <- semiespacio de 256 KB (DC-002.21)
    GC-TRAZA n=16 copiado=260288 antes=262144
    GC-TRAZA OOM: pedido=4008 usado=260288 limite=8cdc0980

Con 256 KB el conjunto vivo (260 KB) no entra y el motor muere **durante `Create()`** con
`fatal error: out of memory`. Con 384 KB y con 512 KB el arranque completa cuadros (con 512 KB el conjunto
vivo queda en el 51% del semiespacio: `copiado=269784 antes=524288`, 4 recolecciones contra 12 con 384 KB y
el **mismo** tiempo por cuadro: el cuello de botella es el dibujado del emulador, no el GC). O sea:
**la medicion de DC-002.21 (512 -> 256 KB) se hizo con el defecto de la Store Queue puesto**, cuando el
conjunto vivo del GC era practicamente vacio; con el GC sano el valor que anda es el **512 KB** de
DC-002.14. Quedan por re-medir con el GC sano la escalera de DC-002.21 (`checar-gc-221.sh`) y el corpus del
recorte (`checar-recorte-214.sh`).

**6. El corpus y sus DOS mutantes.** `scripts/console/checar-error-223.sh` queda con 4 controles (las 14
formas del `error`; el campo del `App`: 3 NIL y **0 BASURA** con el arreglo; el **arranque con el arreglo**:
entra al loop y completa **3 cuadros**; y el mutante del instrumento del offset, que ya existia) y un
**mutante nuevo del arreglo**: `mutar-error-223.py --sq-roto 256` le devuelve la formula vieja a
`gc_sq_copy` y rehace libgodc.a con el semiespacio de DC-002.21, y exige que el arranque **vuelva a 0
cuadros** (medido: `panic: growslice: cap out of range` — la forma de la caida cambia con el layout, como
en DC-002.23, asi que lo que se exige es el 0 cuadros). El `--volver` rehace la cadena y deja el arreglo
puesto. En el mismo pase se aprieto el grep del control 1, que contaba los `itab= 0x0` (los nil) como
"basura": ahora cuenta solo los `esnil= no` con el itab fuera de `.rodata`.

**7. Lo que queda ABIERTO, declarado.** Despues de los 3 cuadros aparece **otra** caida
(`Unhandled exception: PC 64800008`) en los dos estados (con y sin el arreglo) y de forma intermitente:
**no es el campo del `App`** y va como tarjeta siguiente. Y el monton de C queda con ~4 KB libres antes del
motor (`monton de C antes del motor: 3968 bytes libres`), la pared de RAM de DC-002.21/22.

Barreras: `test-std-host.sh` **verde en amd64 y 386** y `compare-motor-dc6.sh` **IGUAL** (31 lineas
identicas, 30 frames comparados). Evidencia cruda:
`evidencia/dc-002.24-copia-por-store-queue.txt`.

## 41. DC-002.25: la escalera del GC re-medida con el **GC sano** (512 KB es el valor, 256 ya no alcanza) y la cadena del vendor que **no era idempotente**

DC-002.24 cerro dejando dos cosas pendientes por escrito (§40, punto 5): con la copia por Store Queue del GC
**arreglada**, el conjunto vivo del GC ya no es vacio (~270 KB) y **toda** medicion hecha con el defecto
puesto hay que re-medir. Esta tarjeta re-mide la escalera del semiespacio (DC-002.21) y el corpus del recorte
(DC-002.14), corrige los numeros que quedaron mal en las secciones 30 y 37, y de paso encuentra **un defecto
nuevo**: la cadena de arreglos del vendor **no era idempotente**.

**1. La escalera, medida con el GC sano (mismo POC `motor-arranque`, mismo gox, 30 s; solo cambia
`GC_SEMISPACE_SIZE_KB`).** El escenario completo es 512 -> 384 -> 256 KB:

    KB     cuadros del loop   marcas MOTOR-ARRANQUE   OOM                          monton de C antes del motor   PVR-FRAMEBUFFER   cuadro
    512    3                 11 (1 arranque)         no                           3968 bytes libres             40776 libres      306.939 us
    384    3                 11 (1 arranque)         no                           3968 bytes libres             40776 libres      309.410 us
    256    0                 10 (1 arranque)         fatal error: out of memory   3968 bytes libres             --                --

Con 256 KB el arranque muere **dentro de `d2app.Create()`** (despues de la marca 1 y del `[App][INFO]`, antes
de la marca 2) y el OOM de newlib lo imprime con su cuenta: `Memory: arena=688700 used=684732 free=3968`. Es
**determinista**: 3 corridas del mismo binario dieron las tres veces 0 cuadros, 10 marcas y el mismo
`Memory:`. Con 384 KB y 512 KB el arranque entra al loop y completa **3 cuadros** (el corte silencioso
despues del cuadro 2 es el defecto ABIERTO de DC-002.24b, no de esta tarjeta: el arranque llega y pinta).

**2. Lo que hay que corregir en el registro (la historia se deja, la medicion se corrige).** La seccion 37
(DC-002.21) y la seccion 30 (DC-002.14) dicen que **256 KB alcanza** y que 512 KB muere en la imagen del
atlas (`diff 98304`). Las dos mediciones se hicieron **con el defecto de la copia por Store Queue puesto**:
con la copia rota no se copiaba nada, el conjunto vivo del GC era practicamente vacio y por eso un semiespacio
chico "alcanzaba". Con el GC sano **la escalera se da vuelta**: 512 KB es el valor (llega al loop, y el
conjunto vivo queda al 51%: `copiado=269784 antes=524288`, DC-002.24), 384 KB arranca sin margen medido y
256 KB **no alcanza**. Las dos secciones llevan ahora una nota en el lugar donde estaba el numero viejo,
apuntando aca; **no se borro ninguna medicion**.

**3. El corpus de la escalera, re-escrito (`scripts/console/checar-gc-221.sh`).** Los controles ahora corren
sobre el valor que anda (512 KB) y el **mutante es el valor viejo (256 KB)**, que tiene que caer:

    control 1 (las 11 marcas por arranque):        VERDE
    control 2 (la seleccion de archivos):          VERDE
    control 3 (el config lo lee el motor):         VERDE
    control 4 (NO esta la pared de 98.304):        VERDE
    control 5 (el motor ENTRA al loop):            VERDE
    control 6 (el monton de C medido):             VERDE   <- los numeros se re-midieron: 3968 y 40.776
    mutante 256 KB:                                CAZADO  <- `fatal error: out of memory`, 0 cuadros, 10 marcas
    resultado: 6/6 controles verdes, 1/1 mutante cazado

El corpus es ademas **el vigia del GC**: el par *512-arranca / 256-no-alcanza* solo se sostiene si la copia por
Store Queue esta sana. Si alguien la vuelve a romper, el conjunto vivo se vacia, 256 KB vuelve a "alcanzar" y
el mutante queda **verde** -> el corpus sale ROJO por mutante no cazado (no se puede pasar por verde con un GC
que no copia). Se declara en el encabezado del propio script.

**4. UN DEFECTO NUEVO: la cadena del vendor no era idempotente.** La regla del proyecto (y la promesa escrita
de `fix-libgodc-gc.py`, "correrlo dos veces deja el mismo sha256") no se cumplia: `reemplazar()` de
`fix-libgodc-sq.py` miraba **primero el texto viejo**, y `VIEJO_FIN` (el `if (remainder > 0) memcpy(...)`) es
una **SUBCADENA** de `NUEVO_FIN`, asi que despues de aplicar el arreglo el conteo del texto viejo seguia dando
1 (el de adentro del nuevo) y **cada corrida insertaba otra invalidacion de cache**. Medido:

    gc_copy.c pristino (git show HEAD:)                43749 bytes, 1 `dcache_inval_range`
    cadena canonica (una corrida sobre el pristino)    44509 bytes, 2
    gc_copy.c que habia en el vendor (acumulado)       46699 bytes, 8   <- 2.190 bytes de invalidaciones repetidas
    libgodc.a por corrida de la cadena (acumulado)     206364 -> 206372 -> 206380 -> 206384 -> 206392 ... (crecia)

El arreglo es **una comprobacion en el orden correcto** (mirar primero el RESULTADO: si el texto nuevo ya
esta, la cadena ya paso por aca). Re-medido con su corpus nuevo
(`scripts/console/checar-idempotencia-vendor.sh`, que arranca de un reset pristino por LECTURA -- `git show
HEAD:runtime/gc_copy.c`, no un `git checkout`):

    control 1 (dos corridas dejan el mismo gc_copy.c):        VERDE  (44509 bytes, sha 379271ee57d1dd61, 2)
    mutante (el orden viejo del guard):                        CAZADO (44509 -> 44874 bytes, 2 -> 3)
    control 2 (de vuelta: la cadena vuelve al mismo estado):   VERDE
    resultado: 2/2 controles verdes, 1/1 mutante cazado

Declarado: el `sha256` de `libgodc.a` **no** es reproducible (su tamano sigue al del fuente -- crecio con las
invalidaciones acumuladas --, pero `ar` guarda el mtime de cada objeto, asi que dos builds del MISMO fuente
dan sha distinto con el mismo tamano); por eso el corpus compara `gc_copy.c`, que es la fuente.

**5. El corpus del recorte (DC-002.14): lo que se corrigio y lo que quedo declarado.**
`checar-recorte-214.sh` tenia clavado el veredicto del arranque en las dos formas de la era del defecto
(`pasa-el-techo-de-RAM panic=...` y `cae-gc-external diff 1916928`). Con el GC sano el veredicto del arranque
de hoy con 512 KB es **`arranca 11 marcas`** (llega al motor), asi que ese caso se agrego al control 3 y a la
vuelta, y el encabezado se re-escribio con la historia corregida. **No se pudo correr de punta a punta**: sus
dos artefactos viven en `/tmp` y no sobrevivieron (faltaban `/tmp/dcbuild/motor-214` y
`/tmp/dcbuild/motor-arranque`), y el gox del recorte (`/tmp/gox-214-gc`, de DC-002.14) **no enlaza el POC de
hoy**:

    console/dc/poc/motor-arranque/main.go:54:57: error: reference to undefined identifier 'os.RamfsTam'
    main.go:199:71: error: reference to undefined identifier 'ebiten.MemLibreDelMonton'
    main.go:226:81: error: reference to undefined identifier 'ebiten.VramDelFramebuffer'

Para re-correrlo hay que rehacer el gox del recorte con sus flags (`recorte-214.sh` sin `SALTAR_GOX`), que es
trabajo de esa tarjeta y no de esta. Lo que **si** se re-midio es la parte que depende del GC: que el arranque
pase las paredes de RAM con el semiespacio en 512 KB es el mismo hecho que mide el control sano de
`checar-gc-221.sh` (11 marcas, 3 cuadros). Los controles 1, 2 y 4 de ese corpus (el instrumento que se niega a
mentir, el peso del dato por dos caminos y el registro del linker) no dependen del semiespacio.

**6. Los valores por defecto que estaban al reves.** `fix-libgodc-gc.py` tenia **256 KB por defecto** (el valor
del defecto) y su texto lo justificaba con las mediciones malas: el default pasa a **512 KB** y el encabezado
dice de donde sale. Declarado, y **no** re-medido aca: los dos instrumentos de traza del GC
(`fix-libgodc-alloc-traza.py`, `fix-libgodc-campo-traza.py`) siguen clavando 256 KB en sus llamadas -- son de
la era del defecto y re-medirlos es tarjeta aparte.

Barreras: `test-std-host.sh` **verde en amd64 y 386** (13/13 excluidos), `mutar-std-host.sh` **39/39 chequeos
cazados** (el arbol queda sano) y `compare-motor-dc6.sh` **IGUAL** (31 lineas identicas, 30 frames comparados).
Evidencia cruda: `evidencia/dc-002.25-escalera-gc-sano.txt`.

## 42. DC-002.26: el cuadro del motor salia **ROTADO 90 grados** — el twiddle de la pantalla tenia los bits de x e y **al reves** (y la tabla corta para el motor)

El sintoma lo trajo la captura de Papu (2026-10-17 18:56): el arranque del motor llega al loop y dibuja su
cuadro de error (`updateInitError`, texto blanco sobre rojo), pero **las lineas de texto corren de arriba
abajo**. Esta tarjeta lo mide, encuentra **dos** defectos en la misma funcion (`armar_twiddle`/`indice_tile`
de `console/dc/std/csrc/pvr_pantalla.c`) y deja los dos con corpus y mutante.

**1. El instrumento (captura contra el cuadro, no contra el ojo).** `scripts/console/medir-volteo-226.py`
ubica la pantalla de la consola dentro de la captura por el rojo del cuadro de error, cuenta la TINTA (los
pixeles que no son ese fondo) por fila y por columna, y reporta las dos varianzas. Un texto horizontal da
**picos por filas** (varianza grande) y columnas parejas; uno rotado 90 grados, al reves. Medido:

    captura con el defecto (f10s, 483x453):   tinta 5604 px   var_filas    27.5   var_columnas 1351.1  -> ROTADO
    captura arreglada    (f6s,  604x453):     tinta 3196 px   var_filas   788.0   var_columnas   43.5  -> DERECHO
    texto de la captura arreglada (transcripto):  file not found: "/rd/juego/patch_d2.mpq" / Please check
    your config file at "/rd/OpenDiablo2/config.json" / Also, verify that the MPQ files exist at /rd/juego /
    Capitalization in the file name matters.

**2. Defecto A: el lugar de los bits estaba al reves.** La convencion del PVR es la del propio KOS
(`kernel/arch/dreamcast/hardware/pvr/pvr_texture.c:34`): `TWIDOUT(x, y) = TWIDTAB(y) | (TWIDTAB(x) << 1)`,
o sea **los bits de x van a los IMPARES**. En `pvr_pantalla.c` estaban al reves (la y a los impares), y eso no
es una diferencia de estilo: intercambiar el lugar de los bits del indice es escribir la textura
**traspuesta** — el hardware lee el pixel que espera en (x, y) y se encuentra el de (y, x). De ahi la rotacion
de 90 grados. Como se midio, con el instrumento nuevo
`scripts/console/verificar-twiddle-226.py` (compara el indice de **cada** pixel visible contra la convencion
de KOS, en las dos texturas del proyecto):

    POC 640x480 (textura 1024x512):  indice nuestro contra el del PVR: 0 distintos de 307.200
                                     la variante vieja (y en los impares): 306.592 distintos (99,8%)
    MOTOR 800x600 (textura 1024x1024): indice nuestro: 0 distintos de 480.000
                                     la variante vieja: 479.400 distintos (99,9%)

La progresion medida en la consola, corrigiendo de a una cosa (misma captura, mismo instrumento):

    sin arreglo (y en los impares, tabla 512):      var_filas   27.5  var_columnas 1351.1  ROTADO
    solo la tabla a 1024 (sigue al reves):          var_filas   47.0  var_columnas  605.1  ROTADO
    bits bien + tabla 1024:                         var_filas  788.0  var_columnas   43.5  DERECHO

**3. Defecto B: la tabla del twiddle no cubria el lado menor.** El indice del bloque sale de `x0 & (lado_min-1)`
y `y0 & (lado_min-1)`, asi que la tabla necesita `lado_min` entradas; con **512** estaba justo para el POC
(1024x512) pero no para el motor (1024x1024, `twid_mask` = 1023): las lecturas con x0 o y0 >= 512 **salian de
la tabla** (leian el .bss de atras) y el bloque se escribia en cualquier parte. Medido: **219 de los 475
bloques de 32x32** del cuadro del motor. La tabla pasa a **1024** entradas (el macro `TWIDTAB` ya cubre los 10
bits). Declarado: este defecto **no** se caza mirando la rotacion — con la tabla corta y los bits bien el
texto sale **derecho pero con artefactos** (medido en el mutante: tinta **5831** px contra **3196** del sano, y
la pantalla roja que ubica el instrumento mide **483** px de ancho contra **604**). Lo caza el instrumento de
host, que compara la tabla contra el requisito del tamano (`1024 >= 1024`).

**4. El corpus (`scripts/console/checar-twiddle-226.sh`) y su mutador (`mutar-twiddle-226.py`).** Cinco
controles (indice = el del PVR; la tabla cubre el lado menor; la captura sale DERECHA; hay tinta; el motor
llega al loop) y **dos mutantes**: `--al-reves` (el lugar de los bits vuelve atras -> el instrumento de host
dice NO y la captura vuelve a salir ROTADA) y `--tabla-512` (la tabla vuelve a 512 -> el instrumento de host
dice NO). El mutador es idempotente y el corpus termina comprobando que `pvr_pantalla.c` queda con el mismo
`sha256` que al empezar. Resultado de la corrida: **2/2 mutantes cazados y 5/5 controles verdes**.

**5. Lo que este defecto esconde del registro viejo.** La formula se escribio en **DC-002.10** y nunca se
comparo en la orientacion: las comparaciones que dieron **identidad 100%** (DC-001.3 y DC-002.12) son de
**sprites**, que no pasan por esta funcion; y `medir-twiddle-217.py` compara la conversion del cuadro entero
contra la conversion **por bloques**, las dos con la misma convencion — nuestra contra nuestra no puede ver una
traspuesta. Es la leccion de siempre (DC-002.8b, DC-002.10): el instrumento tiene que tener una referencia de
afuera, y aca esa referencia es la convencion de KOS y la captura del emulador.

Barreras: `test-std-host.sh` **verde en amd64 y 386** (13/13 excluidos), `mutar-std-host.sh` **39/39 chequeos
cazados** (el arbol queda sano) y `compare-motor-dc6.sh` **IGUAL** (31 lineas identicas, 30 frames comparados).
Evidencia cruda: `evidencia/dc-002.26-cuadro-rotado.txt`.

## 43. DC-002.24b: el "corte despues de los cuadros" era el **instrumento** (y la sonda del PC del SH-4)

DC-002.24 dejo el arranque en 3 cuadros y dos hilos abiertos: un "corte silencioso" despues de los cuadros y
una `Unhandled exception: PC 64800008, code 1` **intermitente**. Esta tarjeta midio los dos con una **sonda del
PC** nueva — un `.c` del runtime, nada de Go (una sonda en Go mueve el monton, que es lo que mueve el defecto de
forma: la leccion de DC-002.22/23/24) — y el resultado es que **el corte no existe**: el loop sigue dando
cuadros.

**1. La sonda (`runtime/sonda_pc.c` + `scripts/console/fix-libgodc-sonda-pc.py`, idempotente).** Engancha el
manejador **global** de excepciones de KOS (`irq_set_global_handler`) y muestrea el contexto del hilo
**interrumpido** en el tick del sistema de hilos (TMU0, `THD_SCHED_HZ` = 100 Hz, que KOS ya tiene armado: no se
toca ningun timer). Cada 1 s imprime los 4 PC que mas se repitieron en la ventana y el `pc`/`pr`/`fp`/`sp` del
ultimo muestreo; grita `RACHA` (el mismo PC 0,3 s seguidos) y `FUERA` (PC fuera de la RAM, 0x8c000000-0x8d000000).
Cuesta 1.095 bytes de `.text` y 552 de `.bss` y **no mueve la aguja**: el cuadro 11 del loop mide
`cuadro_us 309308` sin la sonda y `309326` con ella (**+18 us de 309 ms**). Los PC se resuelven con
`scripts/console/resolver-pc.py` contra **`prog.elf`** — el `prog-emu.elf` que corre Flycast esta **sin
simbolos** (`nm` dice "no symbols" y `addr2line` contesta `?? ??:0`).

**2. Medido: el corte no es un corte.** El loop solo imprime los cuadros 0-2, el PERFIL del 11 y un heartbeat
cada 300 (`ebiten.go`), asi que entre el cuadro 12 y el 299 el log queda mudo **por diseno**:

    corrida de 140 s:  EBITEN-LOOP, cuadros 300 huella 2011487124 dibujos 58394    (entre las ventanas t=94 y t=95)
    corrida de 250 s:  EBITEN-LOOP, cuadros 600 huella 2011487124 dibujos 116594   (entre las ventanas t=187 y t=188)

600 cuadros en 187,5 s = **3,20 cuadros/s**, que es exactamente lo que el loop mide por dentro (`cuadro_us
309326` = 3,23 cuadros/s). Y la huella **no cambia** entre el cuadro 300 y el 600: es la misma pantalla
redibujada.

**3. Medido: que hace el motor ahi (a que altura del arranque esta).** No esta cargando el mapa ni el atlas: el
motor **fallo su `initialize()` y esta sentado en su pantalla de error** (`d2app/app.go:328` ->
`d2core/d2render/ebiten/ebiten_renderer.go:271` -> `PanicScreen.Draw`: `Clear()` + `Fill(Darkred)` +
`DebugPrint(errorMessage)`, cuadro tras cuadro). El **censo** de la sonda sobre 118 ventanas: **`Image.SetRGBA`
50,4%**, **`d2c_pvr_pantalla_convertir` 28,4%** (el twiddle del cuadro), **`Image.Fill`/`LlenarDeRGBA` 20,9%**,
y **0** muestras fuera de la RAM y **0** rachas clavadas. El presupuesto del cuadro que mide el propio loop
dice lo mismo desde el otro lado: `dibujar_us 251796` (81,4%) + `convertir_us 57527` (18,6%) = `cuadro_us
309326`. O sea: el cuadro del arranque se paga **pixel a pixel en Go** (`LlenarDeRGBA` llama a `SetRGBA` por
pixel, y `SetRGBA` chequea bordes y calcula el indice por pixel) mas la conversion del twiddle por bloques de
32x32.

**4. La excepcion intermitente `PC 64800008` no aparecio.** 7 corridas, **~11 minutos de emulacion (~2.000
cuadros)**: 0 `Unhandled exception`, 0 `SONDA-PC FUERA`, 0 `RACHA`. Declarado: las "2 de 3 corridas" de
DC-002.24 salieron con la cadena del vendor **no idempotente** (el defecto de DC-002.25: el arbol tenia ~2.190
bytes de invalidaciones duplicadas), un estado que ya no existe; y **un corpus de corridas fijas no prueba que
un defecto intermitente no exista**. La receta para cazarlo queda escrita (sonda puesta + corrida larga + el
volcado completo de KOS, que imprime PC/registros/PR y la plantilla de `KOS_ADDR2LINE`).

**5. El defecto de instrumento que esta tarjeta arregla.** El tope de la corrida en `run-flycast.sh` era la
**lista de capturas** (t=3..40), no `$SECS`: `run-flycast.sh <elf> 150` se cortaba a los ~41 s, y una corrida
"de 30 s" duraba **26 s** (el dato que DC-002.25 habia escrito —"la corrida dura los 30 s completos"— no era
cierto). Con ese tope el heartbeat del cuadro 300 (t~95 s) no podia aparecer **nunca**. La lista ahora llega a
600 s.

Corpus: `scripts/console/checar-cuadros-24b.sh` — **6/6 controles verdes, 1/1 mutante cazado** (el mutante es la
misma corrida con el tope viejo de 40 s: no llega al cuadro 300 y el control 1 se pone ROJO).
Barreras: `test-std-host.sh` **verde en amd64 y 386** (13/13 excluidos), `mutar-std-host.sh` **39/39 chequeos
cazados** y `compare-motor-dc6.sh` **IGUAL** (31 lineas identicas, 30 frames comparados).
Evidencia cruda: `evidencia/dc-002.24b-el-corte-era-el-instrumento.txt`.

## 44. DC-002.27: los assets del juego en el DISCO — el ISO9660 sin Joliet, el `stat` que no contesta el disco, y la barrera que queda

El arranque del motor buscaba sus MPQ en `/rd/juego` (el romdisk, dentro del ELF) y ahí no pueden estar: el
binario ya carga **13.992.908 de los 16.777.216** bytes de RAM. Los datos van al **disco** y el motor los lee
**por partes** (`Seek` + `ReadAt`: `d2common/d2fileformats/d2mpq/`), así que no hay pared de RAM.

**Los números del juego (medidos con `stat`, la tarjeta citaba 515 MB):** `patch_D2.mpq` **5.022.588**,
`d2data.mpq` **267.330.019**, `d2char.mpq` **267.704.443** = **540.057.050** bytes (515,0 MB). El ISO con los
tres adentro sale de **553.893.888** bytes con el árbol Joliet y de **540.428.288** bytes (263.881 sectores)
sin él — el que anda, ver el defecto 1. Entra en un CD de 700 MB y en un GD-ROM.

**Lo que se agregó (todo apagado por defecto, el camino de DC-002.19 sale byte por byte igual — el
`config.json` del romdisk sigue dando el mismo sha256 `8305595f…`):** en `gen-config-consola`, `-ruta-consola
/cd/juego` y `-mpq a,b,c` (los nombres se filtran contra `d2setup.CoreMPQs`: uno que el motor no conoce se
rechaza en el build); en `mkiso.py`, `origen:ruta_interna` con creación de carpetas (pycdlib contesta
`Could not find path` sin el padre: medido) y `MKISO_JOLIET=0`; `mkdisco.sh` (ISO + GDI con la carpeta del
juego en `/JUEGO`), `leer-iso.py` (sha256 de un archivo de adentro del ISO) y el POC `motor-disco`.

**Defecto 1 — el driver ISO9660 de KOS no encuentra ningún nombre si el ISO trae árbol Joliet.** Con el
disco de 540.440.576 bytes: `nmmgr_lookup("/cd")` da el handler, y **todo** lo que se abra da `errno=2`
(ENOENT). No es el montaje: `init_percd` falla con `ENODEV` (19) y acá da 2, o sea que el driver monta y lo
que no encuentra es el nombre. Con `MKISO_JOLIET=0`, el mismo POC: `fs_open("/cd/JUEGO/PATCH_D2.MPQ") -> 3`
y `fs_total -> 5.022.588` (el tamaño exacto del PC). El driver compara sin distinguir mayúsculas
(`fncompare` usa `tolower`), así que el ISO va en mayúsculas (lo único que acepta el nivel 3) y el motor
pide en minúsculas: da igual.

**Defecto 2 — `stat()` de KOS no contesta ninguna ruta del disco, y el motor decide con `os.Stat`.** Sin
arreglo, el arranque imprimía `game files not found at "/cd/juego", starting first-run setup` (lo que mira
`d2setup.Complete` es `os.Stat`). Arreglo en `console/dc/std/csrc/os_dc.c`: `d2c_stat_tam` cae a abrir y
preguntar el tamaño (`fs_open` + `fs_total`), porque el driver ISO9660 sí implementa `fs_total` mientras que
`stat` da -1. Después: `stat /cd/juego/patch_d2.mpq -> 5022588` y `Complete= true` (**el control negativo
`/cd/juego/no-existe.mpq` sigue fallando**). Declarado: el `stat` de un **directorio** y el `ReadDir` del
disco siguen por el camino del `.manifiesto` (0 entradas), por eso `d2setup.Validate.Ok()` sigue en false y
el arranque usa `Complete`.

**Defecto 3 — el `binary` de la consola no lee *structs*, y el MPQ se lee con *structs*.** La sonda de los
cargadores del motor devolvió `d2mpq.New FALLO: failed to read reader: encoding/binary: Read no soporta ese
tipo de dato (ver DC-002.5)`: `readHeader` hace `binary.Read(&mpq.header)` y el `binary` de la consola es la
stdlib mínima (escalares y `[]byte`; un *struct* necesitaría reflector, que está fuera del port). Arreglo en
`d2mpq`: la cabecera (32 bytes, sin relleno) y la tabla de posiciones del bloque se leen como bytes y se
arman campo por campo con `binary.LittleEndian`. `go test ./d2common/d2fileformats/d2mpq/` verde
(`TestSynthMPQ` arma MPQs de verdad, así que el camino del host queda cubierto). Después: `d2mpq.New OK`,
`d2mpq.FromFile OK`, `mpq.NewSource OK`.

**Defecto 4 — el driver montaba la sesión de la zona de BAJA densidad y leía los datos corridos 45.000
sectores** (el que más costó: dos veces parecía «el emulador miente»). Medido con el `.gdi` de siempre
(pista 1 = el ISO, pista 2 audio, pista 3 = el ISO en el LBA 45.000): el drive contesta GD-ROM (`disc=0x80`)
y el mapa de FADs está bien —el sector 0 del ISO (el IP.BIN) en el FAD **150** y el PVD (`\x01CD001`) en el
FAD **166**, justo donde el driver los busca—, y `fs_open` + `fs_total` dan **5.022.588** (el tamaño sale del
directorio), pero `fs_read` de `/cd/JUEGO/PATCH_D2.MPQ` volvía `41 41 85 86 …` en vez de
`4d 50 51 1a 20 00 00 00 7c a3 4c 00 00 00 03 00`, mientras que el **mismo** `fs_read` de `/1ST_READ.BIN`
(LBA 25) daba `37 d0 02 01 …` = exacto lo que hay en el ISO en ese LBA (`mapa-extents.py`). La causa: la zona
de **baja** densidad de un GD-ROM son los primeros 45.000 sectores (92.006.400 bytes), el árbol con los MPQ
no entra ahí (el MPQ está en el LBA 261.428) y toda lectura más allá del FAD 45.150 la contesta la zona de
**alta** (la pista 3, que vive en el LBA 45.000): el driver sumaba solo el 150 de siempre y leía 45.000
sectores más allá (el LBA 216.428 del ISO, adentro de otro MPQ). Arreglo:
`scripts/console/fix-kos-iso9660-sesion.py` (parche idempotente al KOS vendorizado, `--ver` / `--revertir`):
la sesión de la zona alta manda cuando trae pista de datos y su base (`session_lba` = 45.000) se suma en las
lecturas del árbol (directorios, datos por los dos caminos —DMA y caché— y readdir). Con la sesión de baja el
resultado es byte por byte el de antes (`session_lba` = 0). Después: `fs_read(2048) -> 4d 50 51 1a` y
`fs_read(4096)` igual (los dos caminos coinciden en 2.048 de 2.048 bytes). Medido de paso: Flycast **exige**
que la pista 1 sea de datos («Track 1 must be a data track» → «Invalid GDI file»), así que no se puede dejar
la zona de baja sólo con audio.

**Los controles que dicen que el disco está bien (sin grises):** los tres MPQ leídos **de adentro** de la ISO
dan el sha256 del archivo del PC (`patch_D2` `eb7db78b…`, `d2data` `97c452b1…`, `d2char` `29078a10…`) y la
**consola lee los mismos bytes**: la suma de los primeros y los últimos 256 KiB de cada MPQ, leídos del disco
con el `os` del motor, da lo mismo que el archivo del PC (`patch_D2` 3885502911/1899281040, `d2data`
1306660740/1102600186, `d2char` 2160699462/2078461507). Y el motor ya **no** dice `game files not found` ni
`file not found` (0 ocurrencias). Velocidad del GD-ROM **emulado**: 4 MiB en 34.245 ms = **122 KB/s** (cota
del emulador, declarada). `runtime.GOOS` en la consola es `dreamcast` (no `linux`): el camino `openIgnoreCase`
de `d2mpq`, que lista el directorio si falla el `os.Open`, no se usa allá.

**La pared que queda (el tramo siguiente, medido):** el motor pasa la selección, carga las fuentes y muere al
empezar a leer el *contenido* de los MPQ: `fatal error: gc_external_alloc: OOM`. La pantalla queda **negra** y
en la captura de los 90 s el rojo oscuro del cuadro de error es **0,0 %** (o sea: el cartel de los MPQ ya no
está, pero tampoco hay menú dibujado todavía). El tramo que falta es de **RAM**: la tabla de hash de
`d2data.mpq` son **1.916.928** bytes contiguos (el número de DC-002.22) y el montón de C de la consola queda
en **696 bytes libres** justo antes de entrar al motor. Candidato medido para el tramo siguiente: leer las
tablas del MPQ **por trozos** (la de hash y la de bloques se descifran de a entradas de 16 bytes, sin una
reserva única de 1,9 MB).

Evidencia cruda: `evidencia/dc-002.27-disco-iso9660.txt` (incluye la corrección de la primera versión, que
daba por bueno que la consola leía el contenido del MPQ cuando leía bien el tamaño y el contenido corrido).

## 45. DC-002.28: las tablas del MPQ **por trozos** — la reserva única se fue, la pared cambió de lado, y el arranque del motor se cuelga **mudo** cuando el binario se rearma

**La demanda, medida (la tarjeta la estimaba en 1.916.928 bytes).** Las dos tablas del MPQ tienen 16 bytes por
entrada, y la cabecera dice cuántas (sonda del host `cabeceras-mpq.py`, la misma que imprime la consola):

| MPQ | tabla de hash | tabla de bloques | **pedido de un tirón** (crudo + descifrado) |
|---|---|---|---|
| `patch_d2.mpq` | 16.384 entradas = **262.144** B | 219 = **3.504** B | 524.288 + 7.008 |
| `d2data.mpq` | **65.536** = **1.048.576** B | 11.083 = **177.328** B | **2.097.152** + 354.656 |
| `d2char.mpq` | 16.384 = **262.144** B | 10.010 = **160.160** B | 524.288 + 320.320 |

La tarjeta sacaba sus 1.916.928 de 119.808 entradas; la cabecera dice 65.536, así que el pedido de un tirón
son **2.097.152** bytes contiguos. Ese es el que moría: en libgodc, toda petición de más de **64 KiB**
(`GC_LARGE_OBJECT_THRESHOLD`) se va al montón de C (`gc_alloc` → `gc_external_alloc`, `runtime/gc_heap.c`).

**El arreglo: descifrar por trozos y entregarlos en el acto.** El descifrado de una tabla es una **cadena**
(cada palabra depende del `seed` y el `seed2` que dejó la anterior), así que no se puede leer "por partes
sueltas", pero sí se puede descifrar por trozos y darle cada trozo ya descifrado al consumidor: el estado se
hereda. En `d2common/d2fileformats/d2mpq/`: `decryptTablePorTrozos` con **`tablaTrozoEntradas = 1.024`**
(16 KiB crudos + 4 KiB descifrados, por debajo del umbral de 64 KiB y dentro del semiespacio de 512 KiB del
GC), `readHashTable` llenando el mapa por trozos y `readBlockTable` con `append` de `*Block`. El camino de un
tirón quedó como referencia (`decryptTable`) y lo usan los tests. `go test ./d2common/d2fileformats/d2mpq/`
**verde** (0,053 s), `go vet` limpio, `FuzzArchive` 20 s sin fallos.

**Lo medido en la consola (POC `motor-reservas`, el mismo disco de DC-002.27).** Los instrumentos del montón
de C son nuevos y el número que la evidencia anterior imprimía (`mallinfo().fordblks`) **no** es "la RAM que
queda": el arena crece por `sbrk`. Al arrancar el POC:

    sbrk=0x8c3b7000 techo=0x8cff0000 arena=1211840 libre=2808 usado=1209032 bloqueMax=12814336

o sea **12.814.336 bytes contiguos disponibles** (`bloqueMax`, medido pidiéndolo y devolviéndolo) con 2.808
bytes "libres" dentro del arena. Después: `patch_d2.mpq` (hash de 262.144 B en trozos) abre **OK** y el
montón **no se mueve** (`sbrk` y arena iguales); `d2data.mpq` llega a la cabecera y entonces:

    fatal error: out of memory
    Memory: arena=1543616 used=1541480 free=2136

**La pared cambió de lado y ese es el resultado de la tarjeta:** el `gc_external_alloc: OOM` de DC-002.27 ya
no está (el montón de C no se toca en este tramo y le quedan 12,8 MB contiguos); la pared nueva es el
**semiespacio del GC** (512 KiB, `gc_heap.c:147`, después de `gc_collect`): lo que lo llena es un objeto de Go
**por entrada** (los `*Hash` del mapa y los `*Block` del arreglo). Control de que no hay ninguna reserva
grande escondida: con la traza mínima de libgodc (`fix-libgodc-alloc-traza.py --solo-grande`, que registra
toda petición > 256 KiB y reproduce el mismo camino) la corrida imprime **cero** líneas `ALLOC-TRAZA`.
El tramo siguiente es del lado del **motor**: guardar las tablas compactas (los bytes, no un objeto por
entrada) o abrir cada tabla al usarla.

**El arranque del motor (el POC `motor-disco`) NO se pudo medir, y no por las tablas.** Con los gox de
DC-002.27 el motor corre hasta el **cuadro 2** (`EBITEN-PERFIL cuadro_us 311807`); con el `d2mpq` por trozos
el mismo build muere **mudo**, dos corridas: ninguna línea del programa. Instrumentado (un constructor de C
que abre la consola serie antes de `main` y una variable de paquete que imprime en el arranque de los `init`)
queda localizado: el ELF **carga**, los constructores de C **corren** y la muerte está **adentro del arranque
del runtime de Go** (entre los constructores y los `init` del paquete). Discriminadores, cada uno un
build+corrida:

| qué se enlazó | resultado |
|---|---|
| gox de DC-002.27 (los dos paquetes viejos) | **anda**, hasta el cuadro 2 |
| `d2mpq` por trozos recompilado (el flujo corto) | muere mudo |
| `d2loader/mpq` recompilado, `d2mpq` por trozos | muere mudo |
| **`d2mpq` vuelto a HEAD** (el camino de un tirón) y recompilado con el mismo flujo | **muere mudo igual** |
| los dos `.gox` viejos dentro del directorio nuevo | **anda** |

O sea: **no es el código por trozos** (el mismo `.gox` recompilado de las fuentes de HEAD mata el arranque
igual, con la misma firma). Es la **fragilidad de layout** que la rama ya tiene medida (DC-002.22: "el fallo
del cuadro 0 es de LAYOUT"): el binario rearmado (13.771.784 contra 13.708.888 bytes, +62.896) cambia el
arranque del runtime y lo deja colgado. Dato del mismo par: en la corrida que muere el constructor de C
**corre dos veces** y en la que anda una. Queda como tarjeta hija con esta medición; para medir el motor con
las tablas nuevas hay que resolver eso primero.

**Controles y mutantes (`scripts/console/checar-reservas-228.sh`, el corpus de la tarjeta).** El host exige el
descifrado por trozos palabra por palabra igual al de un tirón y que **ningún pedido pase de 64 KiB** —el tope
del control está **fijo** en 64 KiB a propósito: si se atara a la constante del trozo, un trozo de 128 KiB
pasaría el control—; la consola exige los mismos números de cabecera que el host, `mpq.NewSource OK` y la
**ausencia** del OOM del montón de C. Los dos mutantes se cazan: **A** (la tabla entera de un tirón:
`mutar-reservas-228.py --una-sola-reserva`) hace fallar el host ("llegó en 1 trozos, esperaba 2") y en la
consola se caza por la **huella en el montón de C** —con la reserva única el `sbrk` se mueve al abrir el MPQ
chico (`0x8c3b7000` → `0x8c437000`, +524.288 = las dos reservas viejas) y por el camino por trozos **no se
mueve**—, que es un control que muerde aunque la reserva entre; **B** (el trozo de 128 KiB) hace fallar el
host con "pidió 131056 bytes de una vez: el tope es 65536", y el corpus verifica que el archivo queda con el
**mismo sha256** con el que empezó. Declarado: en el POC la reserva única **no** da OOM (tiene 12,8 MB
contiguos y la de `d2data` son 2 MB); el que murió con ese OOM fue el motor (DC-002.27), que hoy no arranca
por el cuelgue de arriba.

Evidencia cruda: `evidencia/dc-002.28-tablas-mpq-por-trozos.txt`.

---

## 46. DC-002.29: el arranque del motor rearmado — no era la fragilidad de layout, era el **codigo nuevo de `d2mpq` contra la camada vieja**

DC-002.28 dejo el arranque **colgado mudo** cuando el binario se **rearma**, y lo dejo anotado como
"fragilidad de layout": la camada de `.gox` con la que andaba el motor es de la fuente del commit
`6e85010b` (DC-002.20, mtime 10-17 11:10), y cuando la tarjeta recompilaba `d2mpq` desde el arbol de
trabajo y volvia a enlazar, el binario moria adentro de los `init` sin imprimir nada. Esta tarjeta
fue a medir cual de las dos cosas es el disparador: **rearmar** o **el codigo nuevo**.

La medicion es un A/B de verdad: se copia la camada de referencia (`/tmp/gox-240`, sha256 y mtime de
`gox-00220`), se le reemplaza **solo** `d2mpq.gox`, se enlaza el POC `motor-disco` con el mismo
`build-motor-arranque.sh` y el mismo disco, y se corren 30 s de Flycast.

| corrida | `d2mpq` compilado de | `.gox` | resultado |
|---|---|---|---|
| E1 | `6e85010b` (la fuente de la camada) | 204.544 B (`.text` 14.020) | **ARRANCA**: `marcas=27 main.main=1 cuadros=1 arranques=1 gc_collect=4` |
| E2 | `7ef2ff16` (DC-002.27) | 206.468 B (`.text` 14.604) | **MUERE**: `marcas=16 main.main=0 cuadros=0 gc_collect=0`, ultima marca `main_wrapper-entra` |
| sick | HEAD (DC-002.28, lo de la tarjeta) | 207.352 B (`.text` 14.704) | igual que E2 |
| la camada misma | — | 204.656 B (`.text` 14.064) | (es la que anda) |

O sea: **rearmar no rompe nada** — E1 es un rearme (misma invocacion, mismo lugar, misma camada) y
llega al cuadro. Lo que rompe es el **codigo nuevo**. Y la "fragilidad de layout" queda tirada por el
suelo por partida doble: en E1 el binario tambien cambia (el `.gox` es 112 bytes mas chico y todas las
direcciones de atras se corren) y arranca igual, y el **bisect** de DC-002.27 muestra que los dos
archivos del cambio, **por separado**, matan el arranque:

    header viejo + stream nuevo   `.gox` 204.640 B (16 bytes MENOS que la camada)  MUERE (arranques=10)
    header nuevo + stream viejo   `.gox` 206.576 B                                MUERE (arranques=10)

Y lo que queda **descartado con numeros**, no con opiniones:

- **no es la lista ni el orden de los `init`**: `___go_init_main` tiene **119** llamadas y la lista
  (indice -> destino) es **identica** en la referencia, en E1 y en E2;
- **no es la tabla de raices del GC** que el `init` registra: la de `d2reward` (`_go..C0`) tiene en los
  dos binarios `count=4`, las mismas formas (56/56/56/264) y todos los punteros dentro de RAM;
- **no es una colision de nombre de tipo**: el tipo que agrega el codigo nuevo (`_type.._632_7uint8`,
  descriptor de 48 B, `__size`=32, el `[32]byte` del buffer del `readHeader` nuevo) lo define **un solo**
  `.gox` de los 206 de la camada;
- **no es el compilador**: `d2mpq` recompilado dos veces da el mismo `.gox` byte a byte
  (sha256 `25f97728c10b51af`, 207.392 B las dos);
- **no son las rutas relativas/absolutas**: recompilando 4 paquetes cuya fuente no cambio, el sha256
  del `.gox` **cambia** (los caminos del debug) pero el `.text` queda **identico** (`d2hero`
  116400/116400, `d2loader/mpq` 1640/1640, `d2netpacket` 23760/23760, `d2player` 489792/489792). El
  control 1 que heredaba esta tarjeta exigia **sha256 igual**: estaba mal escrito y queda corregido.

**Donde muere, con la direccion.** Del volcado de KOS: `PC 8c0e5002, code 1, evt 0180`,
`PR 8c01225a`, `R1 8c0e49e0`, `R15 ffffe078`. Leido contra el ELF: `PR 8c01225a` es la instruccion
siguiente a la llamada de `___go_init_main` al `init` de **`d2reward`** (o sea: ese `init` no volvio);
`R1 = 8c0e49e0` es `d2reward..import`, que son 16 bytes de codigo cuyo unico trabajo es
`jsr` a **`_runtime.registerGCRoots(_go..C0)`** (`gc_heap.c:192`, cuatro lineas:
`roots->next = gc_global_roots; gc_global_roots = roots;`); y `PC 8c0e5002` **no es una instruccion**:
cae adentro del **charco de literales** de `d2skilldesc.sprintfD` (la palabra que esta en `8c0e5000` es
`mov.w <pool>,r15 ! e078`, que explica el `R15 = 0xffffe078` del volcado: el CPU **entro al pool**).
El hilo queda abierto ahi: la medicion dice **donde** muere y donde **no** esta la causa (la tabla, la
lista, el tamano, las rutas, el compilador), no cual es el mecanismo exacto del salto.

**Lo que el codigo nuevo mete en el `.gox`** (`nm -S`, viejo -> nuevo): agrega
`_type.._632_7uint8` (48 B, debil), su `..eq`, y **20 raices del GC** (`_go..C108`..`_go..C354`); saca
`_type.._2_6_7uint32` (40 B); `d2mpq..import` 600 -> 556 B, `MPQ.readH*` 208 -> 536 B, `Stream.lo*`
692 -> 928 B.

**La hipotesis que queda viva** (y su prueba): el `.gox` de gccgo **no es autocontenido** (lleva los
cuerpos inlineados y los numeros de tipo de sus imports, con la camada en la que se compilo), asi que
mezclar un `.gox` de un arbol con los del otro cambia el binario de una forma que el arranque no
aguanta — y no la aguanta **ningun** cambio de codigo de `d2mpq`, ni su mitad.

**Y la prueba, hecha: la camada COHERENTE.** Se armo la camada entera del arbol de trabajo
(`build-std-gox.sh` + `build-motor-all.sh`: **142 de 142** paquetes, 206 `.gox`) en `/tmp/gox-330` y
con ella:

- el rearme de `d2mpq` **no cambia nada**: el binario enlazado queda con `.text` (5.001.880 B, sha256
  `21fc2196ce7297f1`) y `.rodata` (5.914.272 B, sha256 `48698cc94679f4d2`) **byte a byte** iguales a
  los de la camada sin rearmar (lo unico que difiere del `prog.elf` entero son 32 bytes de debug:
  44.043.776 contra 44.043.808);
- y el **motor arranca**: dos corridas de 30 s identicas dan `marcas=41 main.main=1 arranques=2
  gc_collect=11`, con la ultima marca en `gc-por-umbral-bytes-usados v=480736` y la muerte en
  `PC 0000085b`.

O sea: pasa los **119** `init`, entra a `main.main` y corre **11 recolecciones del GC** hasta la
**pared de 480.736 bytes** usados del semiespacio de 512 KiB — muere antes del primer cuadro
(`cuadros=0`). Esa pared es la de DC-002.28 (un objeto de Go por entrada de la tabla del MPQ) y es
otra tarjeta; lo que estaba roto aca era el **arranque**, y era **mezclar la camada**.

**Corpus.** `scripts/console/checar-rearme-229.sh` (host + consola, se arma solo: saca la fuente vieja
con `git archive`): control 1 = recompilar un paquete sin cambios conserva el `.text`; control 2 = el
rearme es determinista; control 3 = la camada de referencia llega al cuadro; control 4 = la camada +
`d2mpq` de **su propia fuente** tambien llega al cuadro (el que tira abajo la hipotesis "rearmar
rompe"); control 5 = la camada + `d2mpq` del arbol de trabajo **muere en los `init`** (el defecto,
vigilado: si algun dia deja de morir, el corpus se pone rojo para que se revise el limite); control 6 =
la lista de los `init` es la misma; **control 7 (el ARREGLO)** = sobre la camada coherente del arbol
(`COHERENTE=/tmp/gox-330`) el rearme no cambia el `.text` y el motor ARRANCA (`main.main=1` y las
recolecciones del GC). Mutante: `scripts/console/mutar-rearme-229.py --fuente-tocada` rompe la
coherencia de la camada y el control 1 tiene que ponerse **rojo** (es exactamente el estado que mata
el arranque).

**El corpus, corrido de punta a punta: 8/8 controles verdes y 1/1 mutante cazado — y TRES defectos del
propio corpus, medidos y arreglados.** El corpus de arriba estaba escrito pero **nunca habia corrido hasta
el final**: moria en el control 4. (1) ese control sacaba la fuente vieja con `--strip-components=4`, y con
4 el `tar` **no extrae nada** (directorio vacio, medido) -> `gccgo` cortaba el script y los controles 5, 6 y
7 no corrian nunca; iba con 3 y sin filtrar los `_test.go` (que gccgo no filtra y piden `testing`). Ahora
compila **10 archivos**, deja `d2mpq.gox` de **204.640 B** con `.text 0x36f0` — **el mismo `.text` de la
camada de referencia** — y si no compila se marca rojo **sin correr** (correr la copia daria un verde
falso). (2) el control 6 comparaba las **direcciones** de los `init`, que se corren con el layout (el
`d2mpq` nuevo agranda el binario): daba rojo con la lista siendo la misma — **119 entradas identicas** en
indice y simbolo, **71 lineas** que difieren solo en la direccion (`8c45992c` -> `8c459bd8`); ahora compara
`(indice, simbolo)`. (3) el corpus de DC-002.28 dejaba su **mutante A pegado** en `crypto.go` cuando no
corria la parte de consola (sha256 `c38e234f545b30a0` con el mutante contra `4a59a9cd8d99e87e` limpio): el
arbol quedaba mutado para la corrida siguiente; se agrego el levantamiento + su control de sha256, y un
`control 0b` al empezar que dice si el mutante ya estaba puesto.

Los numeros de la corrida (Flycast 30 s por corrida, `COHERENTE=/tmp/gox-330`): control 1 los 4 `.text`
iguales; control 2 determinista (sha256 `25f97728c10b51af`); control 3 la camada de referencia ARRANCA
(`marcas=27 main.main=1 cuadros=1 gc_collect=4`); control 4 con el `d2mpq` de su propia fuente da
**exactamente lo mismo** (`marcas=27 main.main=1 cuadros=1 gc_collect=4`) — rearmar no rompe nada; control 5
el rearme corto contra la camada vieja **muere adentro de los `init`** (`main.main=0 cuadros=0
gc_collect=0`, ultima marca `main_wrapper-entra v=0`); control 6 la lista es la misma; control 7 sobre la
camada coherente el rearme **no cambia el `.text`** (`0000399c`) y el motor **arranca**
(`marcas=41 main.main=1 gc_collect=11`). El mutante sobre la camada coherente: control sano `.text 0x399c`
identico (la camada **es** del arbol) y con la linea del mutante `0x39d4` -> **cazado**.

**Y el control 4 del corpus de DC-002.28, otra vez con las marcas** (`CAMADA_MOTOR=/tmp/gox-330`,
`GEN=/tmp/gox-228`, 75 s): **8 controles verdes**, con
`main.main-entra=1` y el motor pasando `Create()`, leyendo `config.json` del romdisk (`display 800x600`) y
empezando a cargar `/cd/juego/patch_d2.mpq` con **1.176 bytes libres** en el monton de C antes del motor.
Si la camada del motor no es la del arbol, el control se pone **rojo diciendolo**.

**El estado de los criterios, declarado:** el criterio 1 (el rearmado arranca, imprime marcas y **llega al
cuadro 2**) queda **parcial**: arranca y las marcas salen, pero **`cuadros=0`** porque muere en la **pared
del semiespacio del GC** (480.736 bytes usados de 512 KiB, `PC 0000085b`) — la pared de DC-002.28, con sus
palancas en las tarjetas abiertas DC-002.30 (resolucion), DC-002.31 (assets sueltos) y DC-002.32 (peso del
binario). Los criterios 2 y 3 quedan medidos (secciones 9 y 10 de la evidencia).

Evidencia cruda: `evidencia/dc-002.29-arranque-rearmado.txt`.

## 47. DC-002.30: la resolucion como palanca — es de **VRAM** (-786.432 B), no de RAM; y el motor ni la baja (el piso de 800x600)

La tarjeta nacio de una hipotesis de Papu: la pantalla logica del motor es **800x600x4 = 1.920.000
bytes** (exactamente el pedido de **1.916.928** de DC-002.21/22, `1.920.000 - 3.072` de sbrk), asi que
bajarla a 320x240 deberia liberar **-84%** y destrabar el arranque, que hoy muere contra el semiespacio del
GC (DC-002.28/29). El corpus (`scripts/console/checar-pantalla-230.sh`, host **y** consola, **18/18**
controles verdes) mide tres cosas por separado y la hipotesis queda **refutada con numeros**: el campo
`Display` del config **no puede bajar** la pantalla del motor, la pantalla **no se paga en RAM** (se paga
en la VRAM desde DC-002.22), y aun forzando al motor a resolver 320x240 de verdad el punto de RAM **no se
mueve ni un byte**.

**1. El generador ya escribe la resolucion, y el config cambia.** `gen-config-consola -pantalla WxH`
(`console/dc/gen-config-consola/main.go`, nuevo flag) escribe el `Display` del config.json del romdisk y el
sha256 del archivo **cambia con ella**: `1512x982 → 24eaf44bfa7afd2b`, `800x600 → 7c893c5b134086c5`,
`320x240 → 5588275180436273` (el `-verificar` del host acepta el Display pedido). El que **no** cambia es lo
que el motor hace con eso:

| config | pantalla del renderer | lo que el MOTOR resuelve |
|---|---|---|
| 1512x982 / 800x600 / 640x480 / 320x240 | 640x480 | **800x600** (las cuatro) |
| 1512x982 / 800x600 / 640x480 / 320x240 | 320x240 | **800x600** (las cuatro) |

8 de 8 combinaciones resuelven a 800x600: `d2display.Resolve` limita la config a la pantalla y despues
pasa por `Clamp`, que **sube** a `BaseW/BaseH` (800x600). O sea que el campo `Display` del config no es una
palanca de nada mientras el piso este puesto. (El instrumento del host usa el **mismo** codigo del motor,
`d2display.Resolve`, no una copia.)

**2. El arranque con las cuatro configs: el MISMO punto, medido con el instrumento de DC-002.27/28.** POC
`motor-disco` (el motor entero leyendo los MPQ del disco, 3 MPQ de verdad en la ISO), Flycast 30 s por
corrida, layer coherente `/tmp/gox-330`:

    config     display resuelto   VRAM        monton de C     marcas  main.main  gc_collect  muerte
    800x600    800x600            1.048.576   1.176 libres    41      1          11          PC 0000085b
    640x480    800x600            1.048.576   1.176 libres    41      1          11          PC 0000085b
    320x240    800x600            1.048.576   1.176 libres    41      1          11          PC 0000085b
    1512x982   800x600            1.048.576   1.176 libres    41      1          11          PC 0000085b

Los cuatro config.json **son distintos** (punto 1) y el punto de RAM es **el mismo**: ni el libre del monton
de C, ni las marcas, ni las 11 recolecciones del GC, ni el PC de la muerte. La pared sigue donde la dejo
DC-002.29 (el semiespacio del GC en 480.736 de 524.288 bytes) y la config no la corre ni un pixel. Y el
pedido de pantalla **ya no es del monton**: en toda la corrida hay **0** lineas con `1.916.928` y **0**
`out of memory`, y **1** `VRAM de la pantalla 1048576` (criterio 2 de la tarjeta: el numero nuevo es de
VRAM).

**3. La pantalla SI es palanca, pero de VRAM.** El shim del POC con el PVR en 320x240 (una copia del POC que
cambia **una** linea, `IniciarPantalla(320, 240)`, y se borra al terminar) baja el cuadro de **1.048.576 a
262.144 bytes = -786.432 (-75%)** y el punto de RAM **no se mueve** (sigue 1.176 libres, marcas=41,
gc_collect=11, PC 0000085b). La VRAM de la consola son 8 MB con ~1 MB en uso, asi que esos 768 KiB no son
una pared: la pared medida es el semiespacio del GC (480.736/524.288) y el monton de C (1.176 bytes libres).

**4. El A/B del fondo, dentro del MISMO binario: ni forzando la pantalla abajo aparece RAM.** Con el piso de
`Clamp` **sacado** (mutante `mutar-pantalla-230.py --sin-piso`, que solo cambia la fuente) el binario de la
consola **seguia** resolviendo 800x600, porque el `.gox` de `d2display` esta **pre-compilado** en el layer.
El corpus lo rearma en su **propia copia** del layer (`d2display.gox` 65508 → 65388 bytes, `.text` 7084 →
6996) y entonces el **mismo binario** resuelve dos pantallas distintas segun la config, con la RAM
**identica**:

| config | display resuelto | monton de C | marcas | main.main | gc_collect | muerte |
|---|---|---|---|---|---|---|
| 800x600 | **640x400** | 1.240 libres | 41 | 1 | 11 | PC 0000085b |
| 320x240 | **320x240** | 1.240 libres | 41 | 1 | 11 | PC 0000085b |

Es el A/B que la tarjeta pedia, y ahora **cierra**: cambiar la config no cambia una linea de codigo y las
dos pantallas llevan el mismo numero de digitos (el config.json del romdisk no cambia de tamano), asi que la
unica diferencia entre las dos corridas es la resolucion — y la RAM no se mueve. (El 640x400 del config de
800x600 es el motor limitando la config a la **pantalla del renderer**, 640x480 menos el marco de la
ventana; con el piso puesto ese 800x600 se subia a 800x600 y no se veia.) Lo unico que si se mueve en ese
A/B es el **binario**: el rearmado deja 1.240 bytes libres contra los 1.176 del layer coherente (**64
bytes**), que es el **layout** del paquete rearmado (88 bytes de `.text`) y no la pantalla. Declarado como
medicion, no como control.

**5. El veredicto, y que NO se toca.** La resolucion **no** es una palanca de RAM: libera VRAM (-786.432
bytes = -75% del cuadro del PVR) y ni siquiera eso es una pared hoy. La palanca de RAM sigue siendo el peso
del binario y las tablas que lo llenan (DC-002.32). El **piso de 800x600 se queda**: la interfaz del motor
esta disenada como una columna de 800x600 (`docs/DISPLAY.md`), y el generador ahora puede escribir cualquier
resolucion sin que el motor deje de clampearla. Queda como instrumento (y como el control que prueba el
piso).

**6. Tres defectos del propio corpus, medidos antes de creerle un verde.** (a) El control 7 ponia el
mutante y **no rearmaba el `.gox`**: la consola seguia resolviendo 800x600 y el control comparaba el mismo
numero dos veces — un **verde falso**; ahora rearma el paquete en su copia y **exige** `display 320x240` en
la consola. (b) `cp -a` **no alcanza** para copiar el layer: `orden.txt`/`orden-motor.txt` llevan rutas
**absolutas**, asi que el enlace seguia leyendo los `.gox` de origen — el control 7 quedo **ROJO** con el
layer copiado intacto (asi se cazo, y es la trampa que DC-002.22 §6 ya tenia anotada); ahora las rutas se
reescriben y hay un control de que **las 74+142 entradas apunten adentro del copy**. (c) `corre` no borraba
la corrida vieja antes del build y `mide` podia leer un `flycast.log` de **otro binario**: ahora se borra
antes y se rechaza un log mas viejo que el build. Ademas: el corpus se corrio dos veces de punta a punta — la
primera dejo **16 OK / 1 fallo**, y ese fallo era **expectativa mal escrita** del control 7b (esperaba
800x600 y el motor resuelve 640x400 sin el piso); corregida la expectativa y agregado el control 7c, la
corrida final da **18/18**. Los controles 0 a 6 dieron los mismos numeros en las dos corridas. El mutante
(0 de 8 combinaciones resuelven a 800x600) se caza, y el script tiene un `trap` que lo **levanta en
cualquier salida**: la corrida anterior de esta tarjeta murio por credenciales y dejo el mutante pegado en el
arbol (`display.go` sha256 `81eaa783aae51331` contra `9c33f062f3324a4d` de HEAD), que es exactamente lo que
el trap impide.

Barreras: `test-std-host.sh` **verde** en amd64 y 386, `mutar-std-host.sh` **39/39** chequeos cazados con el
arbol sano, `compare-motor-dc6.sh` **IGUAL** (31 lineas identicas, 30 frames).

Evidencia cruda: `evidencia/dc-002.30-resolucion.txt`.

## 48. DC-002.31: los assets SIN MPQ (el proveedor de sistema de archivos del motor) y el techo de tamano real (el GDEMU no tiene el techo del GD-ROM)

La tarjeta nacio de dos preguntas de Papu (2026-10-17): **(a)** se pueden disponer los assets del juego
**sin MPQ**, como archivos sueltos (para poder procesarlos: bajar colores, reescalar, tirar lo que no se
usa)? **(b)** aguanta una imagen mas grande que un GD-ROM (1,1 GB)? Las dos quedan contestadas con numeros.
El corpus es `scripts/console/checar-sueltos-231.sh` (host **y** consola: el host mide el **mismo codigo**
del motor y la consola lo corre en Flycast, layer coherente `/tmp/gox-330`, 25 s por corrida): **29
controles, 0 fallos**. Las herramientas que entran al corpus: `console/dc/mpq-extraer` (MPQ -> arbol
suelto), `console/dc/sonda-sueltos` (el proveedor en el host), `console/dc/sonda-isexit` (la tabla de
`os.IsExist`/`os.IsNotExist` de Go), `console/dc/sonda-seleccion` (la **seleccion** del motor contra el
arbol suelto) y `console/dc/poc/motor-sueltos` (la consola, **por el camino del motor**:
`d2common/d2loader.Loader`, el mismo que usa `d2core/d2asset`).

**(a) LOS SUELTOS ANDAN, y el motor YA los soportaba.** El registro de proveedores del motor
(`d2common/d2loader/loader.go:41-42`) ya tiene `LoaderProviders[types.AssetSourceFileSystem] =
filesystem.OnAddSource`, y ese Source abre con `os.Open` y verifica con `os.Stat` — **las dos funciones que
este port ya habia arreglado para leer el disco en DC-002.27**. Pero el camino suelto tenia **tres
defectos** (ninguno del motor Go: dos del Source vendoreado y uno de la stdlib de la consola), y los tres se
vieron **EN ROJO** antes de arreglarlos:

1. **El NOMBRE.** El motor pide sus assets con **barras invertidas** (la convencion de Windows:
   `data\global\ui\frontend\cinematicselectexp.dc6`) y `filepath.Join`/`Clean` en Linux **no** las tratan
   como separador: la ruta quedaba con un `\` adentro. ROJO (host y consola): el `Loader.Load` del motor
   contestaba `file not found: data\global\ui\frontend\cinematicselectexp.dc6` **con el arbol suelto
   completo en el disco**. Arreglo en `Source.fullPath` (`strings.ReplaceAll(subPath, "\\", "/")` +
   `filepath.FromSlash`, la misma normalizacion que el `cleanName` del Source del MPQ), aplicado con el
   script idempotente `scripts/console/fix-d2loader-fs.py`.
2. **El `Exists`.** El Source respondia `os.IsExist(err)` donde `err` es el de `os.Stat`: **con el archivo
   presente ese `err` es `nil`, y `os.IsExist(nil)` es FALSE**. ROJO: `Source.Exists=false` con el archivo
   presente (y `Loader.Exists=false`), o sea que el motor habria contestado "no esta" para todo el arbol
   suelto. Arreglo: `err == nil`.
3. **La STDLIB DE LA CONSOLA** (`console/dc/std/os/os.go`). `IsExist(err)` era `!IsNotExist(err)`: con
   `err == nil` eso da **TRUE** (nil no es `ErrNotExist`), donde el `os.IsExist` de Go da **false**. Medido
   con el MISMO codigo compilado en los dos lados: el `Exists` del motor contestaba **true** en la consola y
   **false** en el host. Arreglo: `IsExist(err) == (err == ErrExist)`, con `ErrExist` nuevo en el paquete.
   ROJO antes: **96 bien / 3 mal** en `console/dc/poc/vfs`; DESPUES: **99 bien / 0 mal**. Y era la causa de
   un **agujero de la suite**: el mutante del defecto 2 quedaba **VERDE** en la consola (el mutante volvia a
   `os.IsExist(err)` y el `os.IsExist` de la consola contestaba true con `err == nil`), o sea que el corpus
   de la consola no estaba midiendo lo que creia.

Con los tres arreglos, el asset `data\global\ui\frontend\cinematicselectexp.dc6` sale del arbol suelto con
los **mismos bytes** que del MPQ, en las dos plataformas:

    HOST (sonda-sueltos, arbol del patch_D2 extraido: 156 archivos)
      barras_invertidas_OK=true   barras_normales_OK=true bytes=141944 suma=1831109629
      source_exists=true          mpq_OK=true bytes=141944 suma=1831109629
      loader_addsource_OK=true    loader_load_OK=true bytes=141944 suma=1831109629   <- EL CAMINO DEL MOTOR
      loader_exists=true

    CONSOLA (poc/motor-sueltos, disco con el arbol suelto + el MPQ, .gdi SIN Joliet)
      SUELTOS-CONSOLA stat /cd/juego FALLO: file does not exist        <- limite de DC-002.27 (FS ROM de KOS)
      SUELTOS-CONSOLA ReadDir /cd/juego entradas= 0                    <- idem (el driver no lista directorios)
      SUELTOS-CONSOLA OnAddSource OK: /cd/juego
      SUELTOS-CONSOLA Source.Open con el nombre del MOTOR (barras invertidas) OK bytes= 141944 suma= 1831109629
      SUELTOS-CONSOLA Source.Exists con el nombre del MOTOR= true
      SUELTOS-CONSOLA ms del Source= 1255                              <- el primer Open, en el arranque
      SUELTOS-CONSOLA Loader.AddSource= true
      SUELTOS-CONSOLA Loader.Load con el nombre del MOTOR OK bytes= 141944 suma= 1831109629
      SUELTOS-CONSOLA Loader.Exists con el nombre del MOTOR= true
      SUELTOS-CONSOLA MPQ ReadFile OK bytes= 141944 suma= 1831109629    <- la referencia, en la MISMA corrida

**El control negativo (2g), que es lo que prueba que el verde es del arbol y no de otra cosa:** con el
arbol extraido **con la caja del MPQ** (las carpetas como vienen: `data/global/ui/FrontEnd/`) el
`Loader.Load` **FALLA** (`file not found: ...`) mientras el MISMO asset del MPQ sale igual (141.944 /
1.831.109.629): el arbol suelto tiene que ir con los nombres que el motor pide, en **minusculas**. **El
mutante del disco (3g):** el mismo disco **sin** el arbol suelto -> el `Source.Open` y el `Loader.Load`
fallan y el MPQ se lee igual (1 linea OK).

**(a-2) EL CRITERIO 1, CON EL CODIGO DEL MOTOR: la SELECCION del motor es MPQ-CENTRICA y NO ve el arbol
suelto.** El arranque del motor decide si tiene los archivos del juego con `d2setup.Complete`/`Validate`, y
despues le pide sus fuentes al `Loader` **por NOMBRE de MPQ** (`d2app/initialization.go:82`:
`a.asset.AddSource(srcPath, types.AssetSourceMPQ)`, una por entrada de `MpqLoadOrder`). Eso se mide con el
**mismo** codigo del motor (`console/dc/sonda-seleccion`), y el resultado es el que importa antes de elegir
el empaquetado:

    SONDA-SELECCION 1-solo-sueltos  sueltos en el dir: 156 archivos en 24 carpetas
    SONDA-SELECCION 1-solo-sueltos  Complete=false Validate.OK()=false archivos_que_ve_en_el_orden=0 faltan_core=7
    SONDA-SELECCION 1-solo-sueltos  Le faltan (core): patch_d2.mpq d2data.mpq d2char.mpq d2music.mpq d2sfx.mpq d2speech.mpq d2video.mpq
    SONDA-SELECCION 2-solo-mpq      Complete=false Validate.OK()=false archivos_que_ve_en_el_orden=1 faltan_core=6
    SONDA-SELECCION 2-solo-mpq      Ve (en el orden): patch_d2.mpq
    SONDA-SELECCION 2-solo-mpq      Complete=true   (con la orden que pide el patch: el control positivo)

El arbol suelto tiene sus **156 archivos en 24 carpetas** y la seleccion **no los mira**: pide los **7
nombres** de MPQ del nucleo. Con **solo** sueltos, `Complete=false`, y el motor se iria al camino de
primera ejecucion (`d2setup.EnsureGameFiles`: **dialogos**, que la consola no tiene — la pared de
DC-002.19). O sea: **el empaquetado suelto anda para LEER** (el camino del `Loader` de arriba, medido en la
consola byte a byte) **pero el arranque del motor necesita el MPQ al lado o un cambio en su seleccion**;
por eso el disco del corpus de esta tarjeta lleva **las dos cosas** (arbol suelto + `PATCH_D2.MPQ`) y su
arranque pasa la seleccion con el MPQ. Es el dato que decide (ver la decision, abajo), y queda como el
primer renglon de la tarjeta siguiente.

**(b) UN MPQ DESCOMPRIMIDO PESA MAS QUE EL MPQ.** `console/dc/mpq-extraer -minusculas`, los tres MPQ del
arranque:

| MPQ | entradas | suelto en disco | del MPQ | suelto/MPQ | entradas ilegibles | colisiones de nombre |
|---|---|---|---|---|---|---|
| d2char.mpq | 10.009 | 268.026.569 | 267.704.443 | 1,0012 | 0 | 0 |
| d2data.mpq | 10.811 | 360.353.103 | 267.330.019 | **1,3480** | 271 | 0 |
| patch_d2.mpq | 156 | 8.178.526 | 5.022.588 | 1,6283 | 6 | 0 |
| **TOTAL** | **20.976** | **636.558.198** | **540.057.050** | **1,1787** | 277 | 0 |

`d2char` (los sprites) ya viene **sin comprimir** y suelto pesa casi lo mismo (x1,0012); `d2data` (las
tablas) es el que se infla (x1,3480), y `patch_d2` (las pantallas del arranque) x1,6283. Las **colisiones
de nombre son 0** en los tres (dos entradas que solo difieran en mayusculas se pisarian al extraer: no hay
ninguna). Los **277 archivos ilegibles** son entradas que el lector de MPQ **del motor** no pudo leer
(0,06% de d2data): limite del lector, declarado abajo.

**(c) EL TECHO NO ES EL GD-ROM, y lo que rompe el montaje es el JOLIET (no el tamano).** Discos dummy con
el `.gdi` de sectores de 2352:

| relleno | ISO del `.gdi` | la consola arranca | lee el disco | tope del GD-ROM |
|---|---|---|---|---|
| 700 MB | 1.275.639.808 = 1.216 MB | si (2 marcas) | si (1) | 1,1 GB |
| 1516 MB | 2.131.277.824 = **2.032 MB** | si (2 marcas) | si (1) | 1,1 GB |

Un disco de **2.032 MB** (casi el doble del tope del GD-ROM) **arranca y se lee**. Con GDEMU la consola lee
de una **tarjeta SD** (el limite real de FAT32 es 4 GB **por archivo**, y el juego completo son 1,8 GB). Lo
que **si** rompe el montaje es otra cosa: el MISMO ISO (6,6 MB) armado con el **arbol Joliet** que
`mkiso.py` pone por defecto -> **el binario arranca (2 marcas) pero el disco NO monta (2 marcas de "no
lee", 0 lecturas)**; con `MKISO_JOLIET=0` monta (1 lectura). El que rompe es el **descriptor Joliet**, no el
tamano. (De ahi que todo el corpus de discos vaya con `MKISO_JOLIET=0`.)

**El arbol suelto del JUEGO ENTERO en un disco, y las dos paredes del ISO9660.** Con los 20.976 archivos
sueltos, el armado choco con **dos paredes**, las dos con el build muriendo y la regla leida en pycdlib
(medidas, no deducidas):

| pared | error de pycdlib | afuera |
|---|---|---|
| NOMBRES (solo A-Z, 0-9, _ y .) | `ISO9660 filenames must consist of characters A-Z, 0-9, and _` | **9** |
| PROFUNDIDAD (7 niveles) | `Directory levels too deep (maximum is 7)` | **7** |
| entran en el ISO9660 | | **20.960** |

Los 9 del nombre: **7 con ESPACIO** (`IZUAL DEATH.DCC`, `IZUAL SPELL.DCC`, `SLUG MISSILE EXPLODE.DCC`,
`ZOMBIE HOLE.DCC`, `REDEMPTIONGHOST BIG|MEDIUM|SMALL.DCC`) y **2 con GUION** (`SUB-HAR.DT1`,
`SUB-HARFLR.DT1`). Los 7 de la profundidad: `DATA/GLOBAL/MONSTERS/VM/COF/VM/VM{A1,D,DTH,GH,NU,S1,WL}HTH.COF`
(8 niveles). **Renombrarlos no sirve** (el motor los busca por su nombre), asi que el corpus los **salta y
los cuenta**. Y habia un tercer tope que no es del ISO9660: la **lista de argumentos del kernel** — con
20.976 archivos en `argv` el build moria con **"La lista de argumentos es demasiado larga" (E2BIG)**; ahora
`mkiso.py` acepta **`@lista.txt`** (una linea por archivo) y con eso arma el disco. **Medido en la misma
pasada: pycdlib ACEPTA los nombres largos** del arbol suelto
(`/JUEGO/DATA/GLOBAL/UI/FRONTEND/CINEMATICSELECTEXP.DC6;1`, 23 caracteres de nombre y 5 niveles, queda
escrito con ese nombre). El disco de las 20.960 entradas que entran: la consola **arranca**, lee un asset
del arbol suelto con la **misma suma** que el MPQ (**1.831.109.629**), y la ISO mide **670.253.056 bytes =
639 MB** contra el tope de un CD-R de 700 MB (734.003.200: **sobran 63.750.144**).

**Los dos predicados de error de Go, medidos** (`go run ./console/dc/sonda-isexit`, exigidos por el corpus y
por `poc/vfs` 4d-bis):

    SONDA-ISEXIT nil                   exist=false notexist=false
    SONDA-ISEXIT ErrExist              exist=true  notexist=false
    SONDA-ISEXIT ErrNotExist           exist=false notexist=true
    SONDA-ISEXIT ErrPermission         exist=false notexist=false
    SONDA-ISEXIT otro                  exist=false notexist=false
    SONDA-ISEXIT envuelto_ErrNotExist  exist=false notexist=false   <- NO camina los Unwrap
    SONDA-ISEXIT error_de_Open         exist=false notexist=true
    SONDA-ISEXIT nil_de_Stat           exist=false notexist=false   <- el caso del defecto 3

El de la consola compara por identidad **a proposito**: el `os.IsNotExist` de Go **no camina los `Unwrap`**
(solo mira `*PathError`/`*LinkError`/`*SyscallError`, medido arriba con el caso `envuelto_ErrNotExist`).
Eso **no era un defecto**: era la misma regla.

**LOS MUTANTES.** `scripts/console/mutar-sueltos-231.sh` saca uno por vez y **recompila el `.gox` del
paquete mutado EN EL ARBOL MUTADO** (la trampa de la camada de DC-002.29: sin eso la consola seguia midiendo
el codigo viejo), y despues corre las dos mitades del corpus:

    mutante 'nombre'  -> 8 controles en rojo: 2a (sha del arbol), 2b (Source.Open), 2e (Loader.Load),
                         2f (Loader.Exists), 3b y 3d (la consola: el Source y el Loader del motor), 3c y 3e (los dos Exists)
    mutante 'exists'  -> 5 controles en rojo: 2a, 2d (Source.Exists), 2f, 3c y 3e
    control sano      -> 14 OK / 0 fallos
    resultado         -> 2/2 mutantes cazados

`scripts/console/mutar-os-consola.sh` suma el mutante **13** (`IsExist` como la negacion de `IsNotExist`),
que exige que `poc/vfs` se ponga rojo por los tres casos de 4d-bis.

**LA DECISION DEL EMPAQUETADO, con los numeros de arriba.** (1) **GDEMU**: el techo es la tarjeta (FAT32,
4 GB por archivo) y el juego completo son 1,8 GB -> entra entero si se quiere, con los **853 MB** de
audio/video incluidos; el arbol suelto de los tres MPQ del arranque mide **636.558.198** y procesarlo no
tiene techo ahi. (2) **CD-R / GD-ROM** (700 MB / 1,1 GB): el arbol suelto de los mismos tres MPQ (515 MB
comprimidos) mide una ISO de **639 MB**, o sea que **entra en un CD-R** con 63,75 MB de sobra, pero
**pierde las 16 entradas** que el ISO9660 no puede escribir (9 por el nombre, 7 por la profundidad) y
**pesa 1,1787x** mas. (3) Consecuencia medida en **(a-2)**: un arbol suelto **solo** no pasa la seleccion
del motor (pide nombres de MPQ), asi que hoy el unico empaquetado que arranca sin tocar el motor es **el
MPQ**; el suelto sirve para **leer** (medido) y para **procesar los assets** (que es lo que un MPQ no deja
hacer), y para arrancar con el hace falta **cambiar la seleccion del motor** (o un sistema de archivos con
nombres largos, que en la consola no esta medido: ver limites).

**LIMITES MEDIDOS (declarados, no escondidos)**

* El `stat` del **DIRECTORIO** del disco falla en la consola y su `ReadDir` da 0 entradas (el FS ROM/ISO9660
  de KOS no lo sabe dar: el mismo limite que DC-002.27). Abrir y leer **archivos** del disco anda. El
  control 3f **vigila** que siga siendo asi: si algun dia el driver contesta, el corpus avisa.
* Los nombres van en **MAYUSCULAS** en el ISO9660 y el motor los pide en minusculas: anda porque el driver
  ISO9660 de KOS compara con `tolower` (`kernel/arch/dreamcast/fs/fs_iso9660.c:411`).
* **271 entradas de `d2data.mpq` y 6 de `patch_d2.mpq`** no se pudieron leer con el lector de MPQ **del
  motor** (`-detalle-errores`): el arbol suelto del juego queda sin esas entradas (0,06% de d2data).
* Un **ISO9660 NO puede ser el medio** de un arbol suelto con los nombres del MPQ (las dos paredes de
  arriba). Haria falta un medio con nombres largos y sin tope de 7 niveles (ext2/FAT32), y **NO esta medido
  en esta tarjeta que KOS pueda montarlo**: el KOS del toolchain trae `fs_iso9660`, `fs_vmu` y `fs_dcload`,
  y **ningun `fs_ext2`** (medido en el arbol). Es lo primero de la tarjeta siguiente.
* **La SELECCION del motor es MPQ-centrica** (medida en (a-2)): con solo sueltos el arranque se va al
  camino de primera ejecucion, que necesita dialogos. Hoy el disco lleva el MPQ **al lado** del arbol.
* Los POC de esta tarjeta **no corren el `App` completo** con los assets sueltos: el motor no entra en RAM
  (es lo que mide **DC-002.32**) y su arranque muere antes, en la pared del semiespacio del GC
  (DC-002.28/29/30). Lo que queda medido del camino del motor es su **seleccion** (a-2, host) y su
  **`Loader`** (host **y** consola, byte a byte).

Barreras: `test-std-host.sh` **verde** en amd64 y 386, `mutar-std-host.sh` **39/39** chequeos cazados con el
arbol sano, `compare-motor-dc6.sh` **IGUAL** (31 lineas identicas, 30 frames).

Evidencia cruda: `evidencia/dc-002.31-empaquetado.txt`.

## 49. DC-002.32: el peso del binario en RAM — la tabla de d2records **fuera** (1.788.960 B de imagen y +1.788.928 de aire), el `.eh_frame` que **no baja por donde parece** (el hueco del linker) y el panic que **no** pierde su traza

La tarjeta sale de la pregunta de Papu (2026-10-17) por un recorte de assets: esa respuesta ya estaba medida
(el disco no es el problema, DC-002.27) y lo que quedaba era el **peso del binario**, que en la consola se
copia ENTERO a RAM (no hay memoria virtual ni mapeo de ROM). Todo lo de abajo está medido sobre una **capa
coherente propia armada desde HEAD `af89439b`** (copia de fuentes `/tmp/dc232-src` + capa `/tmp/gox-232p`,
**142/142** paquetes, 4m32s) porque el repo lo estaba usando otro agente (DC-002.33). El corpus es
`scripts/console/checar-peso-232.sh` — **15 controles, 0 fallos**, con **8 mutantes cazados** — y la
evidencia cruda está en `evidencia/dc-002.32-peso.txt`. Los instrumentos nuevos: `console/dc/poc/sonda-peso-232`
(el peso y el montón), `console/dc/poc/sonda-panic-232` (el panic de verdad), `fix-d2records-tabla-232.py`
(la palanca 1, idempotente y reversible), `gen-ldscript-eh-232.py` (la palanca 2) y `armar-peso-232.sh`.

**1. EL PESO, SECCIÓN POR SECCIÓN** (`sh-elf-size`, `medir-cargado.py`; base = el motor tal cual, variante =
la tabla fuera del binario). El número del cargador es la suma de las secciones **ALLOC**, que son las únicas
que el arranque copia; el tamaño del archivo (43,9 MB) y el `1ST_READ.BIN` no son el peso en RAM:

    seccion                  base      variante      delta
    .text                5.002.552    5.002.552          0
    .rodata              5.918.652    5.802.604    116.048   <- las cadenas de la tabla
    .eh_frame            1.074.504    1.074.504          0
    .gcc_except_table       20.600       20.600          0
    .data                1.760.444       87.548  1.672.896   <- LA TABLA (y 4 bytes de alineacion)
    .got                        12           12          0
    .bss                   226.748      226.748          0
    ------------------------------------------------------------------
    TOTAL ALLOC         14.003.628   12.214.684  1.788.944   = 83,46% -> 72,80% de la RAM
    1ST_READ.BIN        13.777.032   11.988.072  1.788.960

La tarjeta traia **13.998.980** (medido sobre el ELF de `/tmp/dcbuild/dc24b-corpus`): la diferencia
(**4.648** bytes) es el código nuevo de DC-002.29/31. De los 16.777.216 bytes, al motor le quedan
**2.773.588** para el montón de C, los dos semiespacios del GC (512 KB cada uno) y los ROMDISK.

**2. LA TABLA, POR DOS CAMINOS QUE COINCIDEN.** El símbolo más gordo del binario es **un solo** símbolo de
`.data`: `nm -S --size-sort` da `8cb94fc4 001986bc d _go..C3111` = **1.672.892** bytes, y la cuenta desde la
fuente (`grep -c '^[[:space:]]*{Act:'` sobre `object_lookup_record_data.go`, 1.942.429 bytes de archivo) da
**7.891 elementos × 212 bytes** (el `unsafe.Sizeof` medido ADENTRO de la consola por el POC, no en el host):
**1.672.892**. Los dos caminos son el mismo número, y el corpus exige que sigan coincidiendo.

**3. LA PALANCA 1 (la tabla fuera del binario), con sus TRES números y su PRECIO.** La palanca se aplica con
`fix-d2records-tabla-232.py --fuera` (deja `var objectLookups = []ObjectLookupRecord{}`), se rearma la capa
del paquete y se reenlaza; `--dentro` la revierte. Medido:

* **la imagen plana baja 1.788.960 bytes** (13.777.032 -> 11.988.072) y el ELF cargado 1.788.944;
* **el aire del montón de C crece 1.788.928**: el bloque contiguo más grande **antes del motor** pasa de
  **1.428.480** a **3.217.408** bytes (`bloqueMax`, con `sbrk`/`techo`/`arena`/`usado` del instrumento de
  DC-002.28);
* y el hallazgo duro: **en la base la tabla ya NO ENTRA en el montón** (`la reserva de 1672892 bytes NO
  ENTRO`, con 1.428.480 de bloque máximo) y en la variante **sí entra** (`ptr=0x8ccdd528`, arena que sube de
  1.211.840 a 2.883.008). O sea que el binario de hoy vive más allá de lo que el montón de C puede pedir.

El **precio** está medido con el mismo instrumento: devolver la tabla al montón de C cuesta
**1.671.168** bytes (`sbrk` y `arena`, contra 1.672.892 del símbolo: los 1.724 de diferencia son la
contabilidad del `malloc`). Conclusión honesta: la palanca **no regala RAM por sí sola**, la **mueve** del
binario al montón; es un ahorro NETO sólo si los 7.891 registros se leen **bajo demanda** en vez de
materializarse (eso es una decisión de `d2records`, no de esta tarjeta). Lo que sí mide esta tarjeta es que
los 1,79 MB de imagen son **tan reales como el aire**: el `bloqueMax` del montón sube exactamente lo que baja
la imagen (1.788.928 vs 1.788.960, con 32 bytes de alineación de diferencia).

Las DOS corridas del arranque (Flycast, 30 s cada una, con las marcas y el instrumento de DC-002.27/28):
**no se mueve el punto donde muere** — las dos llegan al loop del motor (12 líneas `EBITEN-LOOP`, mismo
`cuadro_us` ~309.336 y las mismas 26 marcas `ARQ-MARCA`, última `gc_collect v=4`), porque **la pared del GC no
es del tamaño del binario**: es el semiespacio (el umbral se dispara a los **471.888/471.864** bytes usados en
las dos). Lo que cambia es el aire, y con él que la tabla entre o no entre. Las dos paredes son distintas y
las dos hay que bajarlas (DC-002.28/29/30).

**4. LA PALANCA 2 (el `.eh_frame`, 1.074.504 bytes) — EL HUECO.** Hay un detalle que cambia la palanca entera:
**sacarlo con `objcopy -R .eh_frame` NO baja la imagen plana NI UN BYTE**. Medido: 13.777.032 con el
`.eh_frame` y 13.777.032 sin él. La razón es el script de KOS (`utils/ldscripts/shlelf.xc`): pone el
`.eh_frame` **en medio del mapa** (antes de `.gcc_except_table`, `.ctors`, `.dtors`, `.data` y `.got`), y
`objcopy -O binary` escribe por **dirección**, así que el hueco se rellena para no mover lo que viene después.
La palanca, entonces, no es "sacarlo": es **que el linker lo ponga AL FINAL** (el último PROGBITS antes del
`__bss_start`). `gen-ldscript-eh-232.py` escribe una copia del script de KOS con las dos reglas del
`.eh_frame` movidas ahí **sin tocar el KOS vendorizado**, y el reenlace la usa con el último `-T` (medido: el
último manda; dos scripts se concatenan y rompen el enlace). Con eso:

    script de KOS:      imagen CON .eh_frame 13.777.032   SIN 13.777.032   -> 0 bytes
    .eh_frame AL FINAL: imagen CON .eh_frame 13.777.040   SIN 12.702.536   -> 1.074.504 bytes

y el `.bss` (226.748) se corre para abajo con el `.eh_frame`, así que esos 1,07 MB son **RAM libre de verdad**
(y no un hueco: el `.bss` NOBITS que queda delante se rellena de ceros si se lo deja detrás).

**5. Y EL PANIC DE VERDAD NO PIERDE NADA.** El miedo de la tarjeta era correcto de plantear: gccgo puede
necesitar el `.eh_frame` para desenrollar un `panic`/`recover`, y el motor SÍ usa panics. `sonda-panic-232`
arma una pila de 4 niveles y paniquea al fondo; se corrió **tres veces**: (A) el `.elf` de siempre, (B) la
imagen **con** `.eh_frame` por el **camino del disco** (control del camino) y (C) la imagen **sin** él. Las
tres imprimen **lo mismo** (`evidencia/dc-002.32-peso.txt`): `panic: ...`, `goroutine 1 [running]:`,
`Stack Trace: frame pointers not enabled!` y `Memory: arena=1212212 used=1209004 free=3208`. La razón está
medida y no es un supuesto: **la traza de este runtime no camina el `.eh_frame`** — el mensaje lo dice
(`frame pointers not enabled!`, el aviso que este port ya tenía fichado) —, o sea que hoy el `.eh_frame` es
**peso muerto para el panic**. Si algún día el runtime imprime una traza de verdad, esta palanca se vuelve a
medir (queda como límite declarado, no como verdad eterna). El camino A/B/C tiene otra lección: Flycast, con
un `.elf`, carga por **PROGRAM HEADERS** (`core/reios/reios_elf.cpp` usa `PT_LOAD`), así que sacar la sección
del ELF **no** cambia lo que el emulador carga: la medición de esta palanca **tiene que ir por el disco**
(imagen plana), que es además lo que hace la consola.

**6. LO QUE NO CUESTA NADA, MEDIDO.** KOS ya pasa `-Wl,--gc-sections`: medido **quitándolo** del enlace, la
imagen crece **524.852** bytes (y el `.gcc_except_table` aporta 20.600 de esos), así que esa bandera ya está
regalando medio mega. Y una corrección a la premisa de la tarjeta ("los 142 paquetes están todos enlazados"):
**no lo están** — el linker sólo se lleva lo que alguien referencia. El binario del panic (que sólo necesita
la serial y el panic) mide **340.396** bytes de imagen con la **misma** capa de 142 paquetes: el "código no
usado" ya lo saca el linker, y por eso la palanca 5 vale lo que valga cada referencia nueva, no 142 paquetes.

**LOS MUTANTES Y LA REVERSIBILIDAD (criterio 4).** `checar-peso-232.sh` corre con **--mutar=<nombre>**, que
hace mentir UN camino de medición y exige que su control se ponga **ROJO**: `secciones-suma` (la suma de las
secciones), `tabla-simbolo` (el símbolo del ELF contra la fuente), `palanca-imagen` (lo que baja la imagen),
`palanca-aire` (lo que sube el montón), `palanca-precio` (lo que cuesta devolver la tabla), `eh-hueco` (el
recorte sin mover la sección), `eh-movido` (el recorte con la sección al final) y `vuelta` (el peso viejo).
**8 de 8 cazados** (cada uno deja el corpus en 14/15). Y la vuelta de la palanca es una medición, no una
promesa: con `--dentro` la fuente vuelve con el **mismo sha256** (`1e29e1f8...c516`, contra la copia
`.original`) y el motor reenlazado pesa otra vez **14.003.628**.

**LIMITES MEDIDOS (declarados, no escondidos)**

* La palanca 1 mueve RAM, no la regala (el precio medido de 1.671.168 en el montón): el ahorro neto depende
  de que `d2records` lea los registros bajo demanda. Eso **no** está hecho acá.
* La palanca 2 llega hasta el **enlace** (script propio): el script de KOS sigue siendo vendorizado y no se
  tocó; el `Makefile`/`build-motor-arranque.sh` no lo usan por defecto (se pide con `DC_LDEXTRA`).
* El corpus trabaja sobre una copia de fuentes cuando se le pasa `DC232_SRC` (así se hizo acá, por el otro
  agente); por defecto trabaja sobre el repo y **deja la fuente como la encontró** (lo verifica con sha256).
* Las corridas del arranque van **sin el disco del juego** (el POC `sonda-peso-232`, igual que el
  `motor-arranque`): llegan al loop con las dos variantes. El arranque CON los MPQ (que es el que muere en el
  semiespacio, DC-002.28/29) no se re-corrió acá porque su pared no es del tamaño del binario (medido: el
  umbral del GC se dispara al mismo valor en las dos corridas).
* El `.eh_frame` "al final" mueve el `.bss` con él: si algún día se retira la sección del enlace **sin** ese
  script, la imagen plana **no** baja (el hueco) y el `.bss` queda relleno de ceros delante (226.748 bytes
  pagados al pedo): el corpus vigila los dos casos.
* Lo que no se midió: **qué** símbolos saca el `--gc-sections` (y si alguno se llama por reflexión).

Barreras: corpus `checar-peso-232.sh` **15/15** con **8/8** mutantes cazados, la capa propia **142/142**, y
`test-std-host.sh` sin cambios (esta tarjeta no toca la stdlib).

Evidencia cruda: `evidencia/dc-002.32-peso.txt`.

## 50. DC-002.33: las tablas del MPQ **compactas** (los bytes, no un objeto de Go por entrada) — la tabla entera da el mismo checksum en el host y en la consola, el cargador ya abre `d2data.mpq`, y la pared que mata es **otra** (el montón de C)

La tarjeta sale de donde quedó DC-002.28: con el descifrado **por trozos** el `gc_external_alloc: OOM` del montón
de C ya no estaba y la pared nueva era el **semiespacio del GC** (512 KiB), porque lo que lo llenaba era **un
objeto de Go por entrada** (los `*Hash` del mapa y los `*Block` del arreglo): `d2data.mpq` tiene **65.536
ranuras de hash y 11.083 de bloque** (16 bytes cada entrada), y ese conjunto vivo no entra en 512 KiB. Lo que
hace esta tarjeta: las dos tablas de cada MPQ van **como bytes contiguos** (16 por ranura: la de hash de
`d2data.mpq` es **UNA reserva de 1.048.576 bytes**, más grande que el umbral de "objeto grande" de libgodc, o
sea del **montón de C**, no del semiespacio) y el que consulta lo hace **por índice**, con el sondeo del
formato MPQ. La lectura sigue siendo **por trozos** (DC-002.28) y cada trozo se copia derecho a los bytes
(`mpq_hash.go`, `mpq_block.go`).

Todo lo de abajo está medido con una **capa coherente propia** (`/tmp/gox-233`, 142/142 paquetes) y el corpus
`scripts/console/checar-tablas-233.sh`, que va contra los **3 MPQ del arranque** (`patch_d2`, `d2data`,
`d2char`) horneados en la ISO (`/cd/juego`, DC-002.27). Los instrumentos son un **par host/consola** que
imprime **los mismos seis números por archivo**: cuántas entradas tiene cada tabla, el **checksum de sus
BYTES** (`suma = suma*31 + byte`) y la **consulta** (cuántos nombres encuentra el sondeo por índice):

* host: `console/dc/sonda-tablas-mpq` (`go run ./console/dc/sonda-tablas-mpq -mpqdir <dir> -mpq ...`);
* consola: `console/dc/poc/tablas-mpq` (+ `csrc/cshim.c`).

**1. LAS TABLAS: LOS MISMOS NÚMEROS DE LOS DOS LADOS.** Con el mismo código del motor, archivo por archivo
(control 2 del corpus = las dos tablas de los 3 MPQ; control 2b = la consulta):

    archivo        entradasHash  entradasBloque  checksumHash  checksumBloque  nombres  contenidos
    patch_d2.mpq      16.384          219         2035763179     1179232150        162        162
    d2data.mpq        65.536       11.083         1897791535     4131808245     11.082     11.082
    d2char.mpq        16.384       10.010         3931321293      965121384     10.009     10.009
    ----------------------------------------------------------------------------------------------
    TOTAL                                                                         21.253     21.253

Los seis números son **idénticos** en el host y en la consola, y en el host el test
`TestBuscarHashIgualQueElMapaViejo` (nombre por nombre del `(listfile)`, los 3 MPQ) da **21.253 nombres
iguales y 0 distintos** contra el mapa de objetos viejo: el que consulta por índice no cambió ninguna
respuesta.

**2. EL CARGADOR DEL MOTOR YA ABRE `d2data.mpq`.** El POC de DC-002.28 (mismo código, mismo disco) **empieza**
`d2data.mpq` — el archivo donde moría — y lo **abre con sus dos tablas**, sin una sola línea de `fatal error`
ni `out of memory`: la reserva de 1 MB ya no pasa por el semiespacio. Con el **mutante** (un objeto de Go por
entrada) el mismo POC **no pasa de ahí**: el pedido de la tabla de hash como objetos no entra, y muere **mudo**
(sin el mensaje en la serie, familia de DC-002.28/29). Ese es el control 5b medido por **cuántas aperturas**
hay: 1 con el mutante contra 3 con las tablas como bytes. Ojo con un detalle que **no** es de esta tarjeta: el
**tercero** de ese POC es el que queda del lado del defecto del **nombre que se corrompe** (DC-002.39,
intermitente: en una corrida salió con el nombre **vacío** —`/cd/JUEGO/`— y en la corrida del corpus llegó con
su nombre), y no es una pared de memoria.

**3. EL ARRANQUE ENTERO, ANTES Y DESPUÉS (y la lectura honesta).** Con el instrumento de DC-002.27/28
(`motor-disco`, el motor de verdad sobre el disco con los 3 MPQ):

                                          ANTES (objeto por entrada)   DESPUÉS (bytes)
    semiespacio usado (última recolección)        409.432                  393.224
    libre del semiespacio (de 524.288)            114.856                  131.064
    marcas `ARQ-MARCA`                              57                       31
    recolecciones (`gc_collect`)                    19                        6
    `main.main-entra`                                1                        1
    cuadros (`EBITEN-LOOP`)                          0                        0
    donde muere                                    mudo          `fatal error: gc_external_alloc: OOM`
                                                         `Memory: arena=2496864 used=2455304 free=41560`

La pared vieja medida en DC-002.29/30 con las tablas como objetos era **480.736** usados de los 524.288. La
lectura honesta de la tabla: **la pared del semiespacio de esta tarjeta ya no es la que mata** (ni los 393.224
de después ni los 409.432 del mutante llegan a 480.736), pero el arranque **todavía no termina**: muere
**después** de `d2app.Create()`, en **otra** pared — el **montón de C** (`arena=2496864`, 41.560 libres), que
ya venía declarada desde DC-002.28 — y por eso los cuadros siguen en 0. Esa segunda pared es de DC-002.34, no
de acá; lo que sí queda medido es que el arranque **pasa el cargador** (los 3 MPQ del disco, con `d2data.mpq`
entre ellos) y que el semiespacio dejó de ser el que lo frenaba.

**4. EL MUTANTE (criterio 3).** Con el árbol **pristino** (un objeto por entrada): el POC `tablas-mpq` **no
compila** (los instrumentos `TableEntries`/`TableChecksum` no existen en el código viejo => el mutante llegó de
verdad a la representación anterior, control 5a); el cargador **no pasa de las tablas de `d2data.mpq`**
(control 5b); el motor muere **antes** de los cuadros y por el camino del **semiespacio** (mudo, sin
`gc_external_alloc`), en **409.432** usados, contra el de las tablas como bytes que muere en el **montón de C**
(control 5c, o sea que el sitio de la muerte no es el mismo); y el árbol vuelve con el **mismo sha256** del
`d2mpq` (control 6).

**5. LO QUE APARECIÓ EN EL CAMINO (no es de esta tarjeta, y tiene tarjeta propia).**

* **DC-002.39**: en una **misma corrida**, el **nombre del tercero** del `MpqLoadOrder` se corrompe. Medido:
  la vuelta 3 del POC sale con `FALLO: file does not exist largoNombre= -1795502946 ruta= ` (un `len`
  **negativo**: el `string` está vivo pero su cabecera no) después de **11 recolecciones**; y en el POC
  `motor-reservas` el tercero salió con el nombre **vacío** en una de las corridas (es **intermitente**). Es un
  dato **vivo** que se vuelve basura (raíces del GC o el port de `encoding/json`, a decidir con un POC
  chiquito). Por eso el corpus mide el **tercero en su propia corrida** (sección 2b, una sola vuelta) y ahí
  `d2char.mpq` da exactamente los números del host.
* **DC-002.40**: la consola **no puede leer un archivo comprimido del MPQ**: `ReadFile("(listfile)")` da
  `zlib: invalid header` en los **tres** MPQ y el host los lee bien. Por eso los nombres de la **consulta**
  viajan en un `.txt` al lado de los MPQ (`nombres_<mpq>.txt`, los escribe la sonda del host con
  `-nombres-dir` y `mkdisco.sh` los hornea en el `/JUEGO` de la consola).
* **Defecto del instrumento, ya arreglado**: el POC de la consola **sin importar `ebiten`** arranca y muere
  con `Unhandled exception: PC 8c1f8a70` — un PC **6,6 KiB más allá** del `_end` de su propia imagen, antes de
  la primera marca del runtime. `ebiten` es el que **arrastra el paquete `kos`** de libgodc: el binario que
  anda tiene **274 símbolos `_kos.`** y el que moría, **cero**.

Barreras: corpus `checar-tablas-233.sh` **14/14 controles OK y 0 fallos**, la capa **142/142** y, en el host,
`go test ./d2common/d2fileformats/d2mpq/` verde con los 3 MPQ del juego.

Evidencia cruda: `evidencia/dc-002.33-tablas-mpq.txt`.

## 51. DC-002.35: el primer hito visible, MEDIDO — el DC6 del menú sale **idéntico** en el host y en la consola (8/8 trozos) y la captura del emulador da el asset con **39,49%** de píxeles y **90,57%** de coincidencia

La tarjeta queda de la corrida anterior con el criterio 1 en **parcial y con un defecto medido**: la consola leía
`data\global\ui\BIGMENU\waitingroombkgd.dc6` del `patch_D2.mpq` del disco con `crc32=42502455` y el host el mismo
asset con `crc32=820453653` — **mismo tamaño (488.892 bytes), contenido distinto**. Esta corrida vuelve a medir las
dos puntas con el arreglo de las **tablas compactas del MPQ** (DC-002.33) puesto, y mide el criterio 2 (la captura)
con un instrumento que da números.

**1. EL HASH: COINCIDE EXACTO (criterio 1, cerrado).** El POC de la consola ahora imprime los **16 primeros bytes** y
el **CRC-32 de cada trozo de 64 KiB**, y `console/dc/verif-dc6-235` imprime los mismos del lado del host:

    trozo  desde    bytes    consola       host
      0        0    65536    1201590758 == 1201590758
      1    65536    65536    2375366611 == 2375366611
      2   131072    65536    2398266364 == 2398266364
      3   196608    65536    3671128252 == 3671128252
      4   262144    65536    2010774418 == 2010774418
      5   327680    65536    3859150012 == 3859150012
      6   393216    65536    1780613363 == 1780613363
      7   458752    30140    1087561145 == 1087561145
    ------------------------------------------------------------------
    archivo entero          820453653  == 820453653   (16 primeros bytes iguales)
    65.536 índices compuestos 1306731525 == 1306731525
    paleta PL2 (443.175 B)    3508331570 == 3508331570

O sea: **8 de 8 trozos, el archivo entero, los índices compuestos y la paleta** dan lo mismo en las dos
plataformas. El "mismo tamaño, otro contenido" de la evidencia anterior **no era el asset**: era el **lector del
MPQ viejo** (antes de las tablas compactas). El archivo está **comprimido con IMPLODE (PKWARE)** en el MPQ —lo midió
la sonda `sonda-dc6-mpq-235.py`, independiente del motor— y con el lector arreglado el `explode` da los mismos bytes
en SH-4 y en amd64.

**2. LA CAPTURA DEL MENÚ, CON NÚMEROS (criterio 2).** `spectacle -f` captura la **pantalla completa** (3584x1080:
dos monitores), así que la ventana del emulador hay que **ubicarla**. Tres intentos, dos medidos en rojo:

* recorte "a ojo" (210,210 640x480): midió el **escritorio** — color dominante 1,1,1 y 14,91% de píxeles no negros;
* `spectacle` de la corrida completa a los **40 s**: 99,11% de píxeles no negros y **0,98%** de coincidencia — había
  **otra ventana** encima (el run vecino de DC-002.34 tenía su Flycast vivo: `flycast-adelante.sh` mueve **toda**
  ventana de clase `flycast` y las dos terminaron en el mismo lugar).

El instrumento que queda (`scripts/console/medir-captura-235.py`) ubica la pantalla por la **banda de negro puro**
que el POC deja al lado del quad, toma el bloque del asset (escala 1875/1000 => el asset ocupa el **alto entero**),
lo baja a 256x256 y lo compara con la referencia PNG del host. La geometría se **verifica**, no se supone: la
informa KWin para la ventana viva y esta tarjeta agrega `run-flycast-235.sh`, que corre el disco copiado a
`dc235.gdi` (título "Flycast - dc235"), pide la posición con `-config window:left/top`, la deja `keepAbove` y
**reafirma** posición en cada captura (medido: un script de KWin del run vecino movió mi ventana de 1400,92 a
210,210 a los 15 s).

    KWin: Flycast - dc235 | flycast | geo=1400,92,682,481  =>  cliente 1400,120 682x453

Medición, misma ventana y mismo instrumento en las dos corridas:

                                        SANA (asset real)      MUTANTE (DC6_INEXISTENTE)
    píxeles del bloque que no son negro   81.047 de 205.209      0 de 205.209
                                          (39,49%)               (0,00%)
    pantalla completa en negro puro       192.565 de 273.612     273.612 de 273.612 (100,00%)
                                          (70,38%)
    coincidencia con la referencia ±4     81,68%                 55,04%
    coincidencia con la referencia ±8     90,57%                 56,94%
    coincidencia con la referencia ±16    94,56%                 65,42%
    error medio por píxel                 6,13                   17,75

El **gate** de la tarjeta es: píxeles del asset **≥ 20%** del bloque **y** coincidencia **±8 ≥ 80%**. La corrida
sana pasa (39,49% y 90,57%); el mutante **no** (0,00% y 56,94% — ese 56% es el piso de comparar negro contra una
referencia que es 61% casi negra, y se ve en el **error medio**: 17,75 contra 6,13). La diferencia entre las dos
corridas es **81.047 píxeles contra 0**.

**3. EL CONTROL CRUZADO CONTRA EL HOST.** La referencia del host tiene **39.991 de 65.536** píxeles casi negros
(61,02%), o sea **38,98%** útiles; la captura de la consola da **39,49%** — **0,51 puntos** de diferencia, que es lo
que agrega el escalado bilineal 453/256 del emulador y el ARGB1555. Y a los **6 s** la misma corrida da **0,00%**
(el POC tarda ~3,5 s: 2.302 ms de MPQ + 1.105 ms del DC6 + composición), así que el instrumento también distingue
"todavía no pintó" de "pintó".

**4. LO QUE SE ARREGLÓ EN LOS INSTRUMENTOS** (cada uno es un defecto medido):

* `build-dc6-menu-disco.sh` le pasaba a `mkiso.py` el nombre del MPQ de la config (`patch_d2.mpq`) pero en el PC el
  archivo es **`patch_D2.mpq`**: el MPQ no entraba al disco y el POC moría con `MPQ-ABRIR FALLO: file does not
  exist`, que **parece** un defecto del MPQ. Ahora resuelve el nombre real del archivo.
* La **paleta** vive en `d2data.mpq` y el DC6 en `patch_d2.mpq`: el horneado de la paleta pedía el DC6 al MPQ grande
  y abortaba el build. Se agregó **`-solo-pal`** a `console/dc/bake-dc6`.
* El paso de "números del host" llamaba a `crc-dc6-235.py` con el nombre del POC, que no era el de los archivos
  horneados (`pal235.*`): comparaba **otro archivo**. Ahora corre `console/dc/verif-dc6-235`, que además escribe la
  referencia PNG.
* `mkdisco-235.sh` tenía la paleta fija en `host/pal235.pal`; ahora autodetecta la `*.pal` que dejó el horneador.

Barrera del motor: **no se corrió y no hace falta** — el diff de esta tarjeta no toca una línea del motor
(`console/dc/bake-dc6`, `console/dc/poc/dc6-menu-disco`, `console/dc/verif-dc6-235` y los scripts de build/corrida;
ningún archivo de `d2*`, `os`, `video` ni `audio`). La comparación consola-vs-host de esta tarjeta es la del POC del
DC6 y es **exacta**.

Evidencia cruda: `evidencia/dc-002.35b-dc6-hash-y-captura.txt` (y la anterior, `dc-002.35-dc6-menu-disco.txt`).

## 52. DC-002.34: hasta dónde llega el motor **entero** con los MPQ del disco — la palanca de DC-002.33 lo deja pidiendo **1 MiB contiguo** al montón de C (39.536 libres), la de DC-002.32 lo corre **un MPQ entero** (lee `d2data.mpq` y dos assets, y muere mudo) y **cada una sola no alcanza**: **0 cuadros** en los cinco estados

La tarjeta de DC-002.33 cerró con una frase que había que cobrar: *«el arranque todavía no termina: muere
después de `d2app.Create()`, en otra pared, el montón de C»*. Lo que hace esta tarjeta es **medir ese punto
con el motor entero** (el POC `motor-arranque`, que llama al entry point de verdad: `d2app.Create(...)` +
`instance.Run()`, lo único que agrega es la consola serie y el video), con los **3 MPQ del juego horneados en
la ISO** (`/JUEGO`, el camino de DC-002.27) y **una palanca por vez**:

* **BASE** — las tablas del MPQ como **objetos** de Go (el «antes» de DC-002.33) y la tabla de `d2records`
  **adentro** del binario (el «antes» de DC-002.32);
* **S233** — la palanca de DC-002.33 (las tablas como **bytes contiguos**), que es el árbol de HEAD;
* **S232SOLO** — sólo la de DC-002.32 (`--fuera`), con las tablas del MPQ todavía como objetos;
* **S232** — las dos;
* **MUTANTE** — la misma imagen **sin** la carpeta `/JUEGO`.

**1. EL NÚMERO, ESTADO POR ESTADO.** Todo con la capa `/tmp/gox-233` (control: `d2mpq.gox` recompilado de este
árbol sale **byte a byte igual**, `cmp`), `libgodc.a` del 18-10 13:20:35 y el árbol como el arreglo de 233 (7/7):

    estado                        1ST_READ.BIN   montón de C   semiespacio máx.   dónde muere
    BASE  (sin 233, sin 232)      13.771.272 B    1.344 libres  393.240 (131.048)   Data address error (PC 0000085b)
    S233  (solo 233)              13.773.320 B    3.392 libres  393.232 (131.056)   gc_external_alloc: OOM
    S232SOLO (solo 232)           11.982.344 B      320 libres  401.264 (123.024)   Data address error (PC 0000085b)
    S232  (233 + 232)             11.984.360 B    2.400 libres  409.472 (114.816)   MUDA (la consola reinicia)
    MUTANTE (ISO sin /JUEGO)      13.773.320 B    3.392 libres  395.616 (128.672)   MUDA (la consola reinicia)

La muerte de **S233** es la que da el número que faltaba, y es limpia:

    [File Loader][INFO] Adding source: '/cd/juego/patch_d2.mpq'
    Out of memory. Requested sbrk_base 0x8d04b000, was 0x8cf4b000, diff 1048576
    fatal error: gc_external_alloc: OOM
    Memory: arena=1970208 used=1930672 free=39536

O sea: **la pared ya no es el semiespacio del GC** (le sobran **131.056 B** de 524.288) sino el **montón de C**,
que tiene **39.536 B** cuando el arranque le pide **UN MEGA CONTIGUO**. Las **5 vueltas** de la corrida dan el
mismo número: no es azar. (Y ojo con la lectura del «antes»: con las tablas como objetos el arranque **no**
imprime `OOM`, se cae con una excepción —`Unhandled exception: PC 0000085b`, un PC que no es código del
arranque— y el emulador **se detiene**; en S233 muere con el mensaje.)

**2. CADA PALANCA SOLA NO ALCANZA; LAS DOS JUNTAS VALEN UN MPQ.** Con **S232SOLO** (la tabla de `d2records`
afuera pero las tablas del MPQ como objetos) el arranque **muere igual que BASE**, con la **misma excepción** y
sin pasar del primer MPQ: el que mata primero es el **GC** (401.264 usados). La RAM que gana la tabla de 232 no
sirve hasta que la de 233 corre el techo al montón de C. Con **las dos** (S232) el arranque **abre los dos
primeros MPQ del GD-ROM**, arranca el **Asset Manager** y **lee dos assets de verdad** de adentro del MPQ:

    [File Loader][INFO] Adding source: '/cd/juego/patch_d2.mpq'
    [File Loader][INFO] Adding source: '/cd/juego/d2data.mpq'
    [Asset Manager][INFO] Language: enUS (ENG), font dir LATIN
    [App][INFO] Initializing asset manager
    [File Loader][INFO] Loaded /cd/juego/patch_d2.mpq -> /data/global/excel/LvlTypes.txt
    [File Loader][INFO] Loaded /cd/juego/patch_d2.mpq -> /data/global/excel/LvlPrest.txt
    (y ahí: 00:19:429 REIOS: Booting up — la consola reinicia sin imprimir NADA)

Eso es la prueba de que **el tamaño del binario en RAM es la palanca de esta pared** (la tesis de DC-002.32,
medida ahora sobre el arranque y no sobre una sonda). La muerte pasó a ser **muda** justo después de
`LvlPrest.txt`: sin mensaje no se puede decir si falta RAM o si es un defecto, y por eso va a su tarjeta.

Un número que **no** se comporta como uno esperaría y queda anotado tal como salió: el hueco de C **no** crece
con la imagen más chica (S232SOLO tiene 1.788.928 B menos de imagen y le quedan **320 B** contra los 1.344 B de
BASE). Lo que el binario deja libre se lo lleva el runtime (el semiespacio/las arenas) antes de que el POC mida.

**3. LA PANTALLA (criterio 2), CON NÚMEROS.** No se usa el recorte fijo de DC-002.35 (`210,210 640x480`): en
este host la captura de pantalla completa es **3584x1080** (dos monitores) y ahí la ventana del emulador **no
cae**. Se captura **la ventana** por su id de X11 (`_NET_CLIENT_LIST` + `import -window`), que con
`window:fullscreen=no` mide **682x453** (308.946 píxeles), y se clasifica cada cuadro por color dominante:

    estado    capturas   "negra" (la pantalla del motor)   "arranque-sega" (licencia de la consola)
    BASE         6              4  (67%)                          2  (33%)
    S233        25             16  (64%)                          9  (36%)
    S232SOLO     5              3  (60%)                          2  (40%)
    S232        25             17  (68%)                          8  (32%)
    MUTANTE     25              3  (12%)                         22  (88%)

    «negra» = 100,0% de los píxeles con los tres canales < 16; «arranque-sega» = 91,9% en un gris uniforme.
    El reparto se mueve entre corridas (S233 14-23 de 25, S232 16-21, MUTANTE 3-10): el ciclo del motor dura
    ~5 s y la captura es 1 por segundo. Lo que NO se mueve: **cuadros dibujados por el motor = 0** en los
    cinco estados (cero líneas `EBITEN-PERFIL`/`cuadros del loop`): el motor **no llega al bucle de dibujo**.
    Lo que se ve en pantalla no es su cartel ni nada del juego: o negro (su pantalla, sin pintar) o la
    licencia de arranque de la consola entre reinicio y reinicio.

**4. EL MUTANTE (criterio 3).** La misma imagen **sin la carpeta `/JUEGO`**: 9 vueltas en 25 s y en las 9 la
consola dice `game files not found at "/cd/juego", starting first-run setup`; en las tres corridas con juego,
**0 veces**. El control de la ISO, además, compara el `sha256` de los tres MPQ **de adentro del ISO9660** con
el del archivo del PC (los tres iguales) y confirma que la ISO del mutante **no** tiene `/JUEGO`.

**5. LO QUE APARECIÓ EN EL CAMINO (cada uno con su tarjeta y con este número).**

* **DC-002.41**: la pared es el **montón de C** (39.536 libres contra un pedido de 1.048.576). Falta medir
  **quién** pide ese mega (la tabla de hash de `d2data.mpq` son 65.536 x 16 = **1.048.576 B** justos: el
  arreglo de DC-002.33 la pasó al montón de C a propósito, porque supera el umbral de «objeto grande»; hay que
  partirla).
* **DC-002.42**: la palanca de DC-002.32 **no está en el árbol** (el instrumento la vacía). Aplicada de verdad
  vale **un MPQ entero de arranque**… siempre que la de 233 esté abajo (arriba medido: sola, no alcanza).
* **DC-002.43**: la muerte **muda** después de `LvlPrest.txt` (semiespacio en 409.472, sin `fatal error`, sin
  `Out of memory`, sin excepción del SH-4 del lado del emulador).
* **DC-002.44**: el motor **ve 0 archivos** en `/cd/juego` — `Complete= true Validate.OK()= false archivos que
  ve= 0 faltan= 7` en las tres corridas con juego: el `Stat` por nombre anda (por eso el MPQ se abre y se lee
  entero) y el **listado** del mismo directorio no. El `ReadDir` del overlay sí anda (`/rd/prueba-seleccion`,
  1 entrada): el defecto está del lado del iso9660.

Barreras: corpus `checar-motor-disco-234.sh` **8/8 controles OK y 0 fallos** con los **cinco estados**, los
números de la tabla salieron idénticos en dos corridas completas separadas (los únicos que se mueven son los
del reparto negro/licencia de la pantalla, por el instante de la captura), y la capa `/tmp/gox-233` es la de
este árbol (control byte a byte del `d2mpq.gox`).

Evidencia cruda: `evidencia/dc-002.34-motor-disco.txt` (y los logs de Flycast de los cinco estados, con las
capturas, en `~/.hermes/cache/scratch/corpus-234/`).

## 53. DC-002.36: el MENU PRINCIPAL **dibujado y navegable** en la Dreamcast — los nombres de las texturas ya no se caen con el recolector (la cadena que apunta al **MEDIO** de otro objeto), el disco que **no arranca** con carpeta y el romdisk que lo saca de la ecuación

La tarjeta pedía el menú principal del juego en pantalla y navegable. Lo que la bloqueaba no era el menú: era el
**recolector de basura de la consola con los punteros interiores** — las cadenas que son un **pedazo** de un objeto
más grande (el `strings.Fields(string(datos))` del plan del menú, o el `strings.Split` del decodificador de json).
Medido, el POC del menú cargaba **3 de 11** texturas: los ids 0 y 1 salían con su `crc32` y desde el id 2 los
**nombres** salían `<basura>` y el POC pedía archivos que no existen (`faltan= 8`). Los nombres son los que se caen,
y los nombres son cadenas que apuntan al medio del buffer del plan.

**El arreglo**, con la sonda como instrumento (`console/dc/poc/sonda-cadenas`, tres casos y un **control sano**):

* **A**: una cadena que es un pedazo de una reserva grande (el `strings.Fields` del plan) — huella crc32 antes y
  después de las recolecciones: **2992558440 → 2644168089** sin el arreglo (ROJO) y **2992558440 == 2992558440** con el
  arreglo (VERDE).
* **B**: una cadena guardada en un campo de una caja del heap (el campo `nombre` de cada `textura`) — lo mismo:
  **ROJO → VERDE**.
* **C**: el control sano de la sonda — **VERDE siempre** (si C se cae, el que está mal es el instrumento).

Las recolecciones son **deterministas** (`runtime.GC()`, que el paquete `runtime` de la consola expone como
`//extern d2c_gc`): `tamBuffer= 1200`, `recolecciones= 5`, `tamSobra= 2048` y **sin** `GC to-space overflow`. (Con
`tamBuffer= 20000` el instrumento medía otra cosa: 567 recolecciones y el desborde del semiespacio de 512 KiB a
487.960 B.) En `~/dreamcast/libgodc/runtime/gc_copy.c` el arreglo tiene dos partes: el barrido conservador ya no
descarta las direcciones que caen adentro de un objeto (`gc_is_valid_object_start` pedía principio exacto) y el
copiado resuelve el puntero al medio moviéndolo con su objeto. Se aplica con
`python3 scripts/console/fix-libgodc-interior.py --build` (idempotente, deja pristino el vendor y encadena
`fix-libgodc-sq.py`).

**La suite y su mutante** (`bash scripts/console/checar-cadenas-236.sh 25`): **15 controles OK, 0 con fallo**.
El mutante es `fix-libgodc-interior.py --quitar --build` y con él **A y B salen ROJOS** (2992558440 → 2644168089) y
**C sigue verde**: la suite caza el defecto y no se cae por el instrumento. Al final el corpus deja el árbol como
estaba.

**El menú, en pantalla y con números** (`bash scripts/console/checar-menu-236.sh 45`):

* **10 de 11** texturas subidas a la VRAM (`bytes= 1196032`, `vram_libre= 4593064`), `faltan= 1` y
  **281 quads por cuadro** dibujados: el menú está en pantalla.
* criterio 1 (captura a los 10 s, foco -1): **49.542 de 307.200** píxeles que **no** son el fondo (16,13%); los
  dominantes son los grises de las fuentes y los botones (80,80,80 / 64,64,64 / 96,96,96). Contra la referencia que
  arma el **host con el código del motor**: ±4 **31,61%**, ±8 **38,43%**, ±16 **48,89%**, error medio **35,41** — la
  coincidencia es parcial porque la referencia incluye el **fondo** y la corrida va sin él.
* criterio 2 (navegación): con `abajo` por el **teclado virtual de `/dev/uinput`** antes de las capturas de 15 s y
  25 s (42 marcas de FOCO en la corrida), las capturas **f10s vs f15s cambian 6.059 de 307.200 píxeles (1,97%)** en
  la caja `x=211..428 y=352..380 (218x29)` y **f15s vs f25s cambian 11.921 de 307.200 píxeles (3,88%)** en
  `x=211..428 y=320..380 (218x61)` = `x=263..535 y=400..475` en el lienzo lógico del motor: es la región de los
  botones. La selección se mueve y **se ve**.
* criterio 3 (el mutante, sin `PLAN.TXT`): `PLAN FALLO: file does not exist`, **0 texturas, 0 quads** y la pantalla
  **queda como estaba**: 0 de 307.200 píxeles que no sean el fondo del PVR (negro), contra los 49.542 del menú con
  el plan.

**Para que la tecla llegue** hay que ACTIVAR la ventana: el teclado virtual manda a la ventana activa, no a la de
arriba. Con la ventana sólo `keepAbove` la consola serie se quedaba en `FOCO inicial= -1` y las dos capturas salían
idénticas (**0 píxeles** de diferencia). La regla de KWin del runner ahora hace `workspace.activeWindow = w`, informa
cuál quedó activa y el runner **insiste** hasta 5 veces antes de cada tecla (medido: con un run vecino vivo la
activación no siempre prende al primer intento).

**Para que las 11 texturas entren en la RAM** el POC decodifica, sube a la VRAM y **suelta el lienzo** de cada
textura antes de la siguiente (`t.idx = nil`). Antes armaba los 11 lienzos y los subía después: con los ~2,3 MB de
lienzos más los ~1,4 MB de los DC6 crudos vivos a la vez, el arena de C de KOS quedaba lleno
(`Memory: arena=4148384 used=4144792 free=3592`) y el POC moría con `panic: index out of range` al cargar la 9.ª
textura (`FUENTE_FORMAL10`).

**El defecto que queda declarado: el FONDO.** Con los 11 assets el POC **muere al subir la textura 0** a la VRAM. La
medición lo ubica: recién leído el plan la textura 0 está **limpia** (`PLAN-LEIDO id= 0 w= 800 h= 600 pw= 1024
ph= 1024 banco= 0`) y al salir de `armarDC6` sale con `ph= -2147482624` y `banco= -1942452840` (valores con pinta de
puntero) mientras los otros 10 elementos salen perfectos y con `crc32` estable entre corridas: **algo escribe fuera
de su buffer al decodificar los frames del fondo** y lo que pisa es el struct de la textura 0. Al pegado de la grilla
del fondo se le puso un **tope explícito** que cuenta los píxeles que no entran y **no se dispara**: el que se sale es
el decodificador del DC6 (`d2dc6.DecodeFrame`). Es el mismo síntoma de fondo de **DC-002.39** (la consola corrompe un
dato vivo): se declara acá y sigue por esa tarjeta. La corrida «el menú se ve» va sin el fondo
(`SIN_ASSET=FONDO.DC6`) y por eso trae los otros 10.

**El disco no arranca con carpeta (y el romdisk lo saca de la ecuación).** Con el mismo `1ST_READ.BIN`
(sha256 `c48f24624e8fa48705077c3d`) y el mismo IP.BIN:

* el ISO con los 20 archivos adentro de la carpeta `/MENU` **no arranca**: la corrida no imprime **ninguna** marca del
  arranque (ni el constructor de C de libgodc, `ARQ-MARCA ctor-kos_startup v=0`, que sale antes de `main`) y la
  pantalla queda negra; el mismo binario con los 20 archivos en la **raíz** del ISO arranca.
* el bisect del contenido: con **un** archivo chico en la carpeta (`PAL_SKY.PAL`, 768 B) ya no arranca, con nombres
  cortos 8.3 tampoco, con `PLAN.TXT` (10 sectores) tampoco → es la **carpeta**, no los nombres ni el tamaño.
* el **cruce**: el disco de DC-002.35 (carpeta `/JUEGO`) arranca con **su** binario pero **no** con el del menú, y el
  binario del menú con `/JUEGO` tampoco → no es el nombre de la carpeta ni el ISO solo: depende del **binario**. Y el
  arreglo «sin carpetas» tampoco es estable: el mismo ISO de raíz arrancó con el binario de antes y **no** con el
  binario de 352 bytes más grande. El arranque depende de una combinación (layout del ISO × layout del binario) que
  **no** está medida: queda declarado, no resuelto.

La salida medida es el **romdisk**: `build-dc.sh` empaqueta `<poc>/romdisk` con `genromfs` + `bin2o` cuando ese
directorio existe y `~/dreamcast/libgodc/runtime/romdisk_init.c` lo monta en `/rd` (símbolos débiles: no hay que
tocar el `csrc` del POC). El POC busca sus assets en `/rd/`, después en `/cd/MENU/` y después en `/cd/`, y se queda
con el primero donde lea el `PLAN.TXT`. Con eso el menú se mide igual y el disco sale de la ecuación.

**De paso, el bug del `.gox`** que rompía el enlace de los POC: `multiple definition of
golang_0org_1x_1image_1bmp.Decode` (y 11 más). La causa era `~/dreamcast/gox/orden.txt` con **87 líneas** = 75 de la
stdlib más **12 `.gox` de terceros duplicados**: se escribían en otro árbol y se anotaban en la orden compartida, así
que se acumulaban en cada corrida. En `build-std-gox.sh` los terceros van a `$GEN` (el mismo árbol que la stdlib): la
orden quedó en **63 líneas, 0 duplicados** y el árbol con **65 `.gox`** (51 stdlib + 12 terceros + 2 del motor) todos
de la misma corrida = la camada coherente de DC-002.29.

Barreras: corpus `checar-cadenas-236.sh` **15/15 controles OK** con el mutante cazado (A y B rojos, C verde),
`checar-menu-236.sh` con los tres criterios medidos (**49.542** píxeles de elementos, **6.059** y **11.921** píxeles de
cambio al navegar, **0** quads y **0** píxeles con el mutante) y la barrera del motor (`compare-motor-dc6.sh`) igual.

Evidencia cruda: `evidencia/dc-002.36-menu-consola.txt` (con las capturas en
`~/.hermes/cache/scratch/corpus-menu-236/menuseve/`).

## 54. DC-002.39: el nombre del `MpqLoadOrder` que se caia con el recolector — el valor que el bucle tiene **en la mano** queda viejo si la recoleccion cae ahi (medido con POC chiquito y con el vigia dentro del GC), y los 3 MPQ del arranque ya salen en **UNA sola pasada** con `d2char.mpq` intacto

La tarjeta pedia decir **cual de las dos** cosas corrompia el nombre del tercero del `MpqLoadOrder` (el `range`
de `d2app/initialization.go` y del POC `tablas-mpq`), el **decodificador json** o el **recolector**, y
arreglarlo. La respuesta, medida: **ninguna de las dos "por si sola"** — el decodificador entrega las tres
cadenas y el recolector mantiene la configuracion impecable (el `struct`, su cabecera de rebanada y el arreglo)
en **todas** las recolecciones. Lo que se cae es el **valor que el bucle tiene en la mano** en la vuelta en que
el cuerpo (abrir y leer el MPQ) dispara una recoleccion: ese valor queda viejo y el bucle sigue leyendo con el.

**Lo que se midio** (crudo en `~/.hermes/cache/scratch/p39/`):

* **El POC chiquito** (`console/dc/poc/config-viva`, el que pedia la tarjeta): lee la config igual que el motor
  y fuerza recolecciones en 406 rondas, **sin** trabajo de MPQ adentro. Nunca se corrompe (`rotosJ 0` en las
  406). O sea: con el buffer de la config vivo alcanza — el defecto necesita que el recolector se lleve el
  buffer, y eso lo trae el trabajo del motor.
* **El POC con el trabajo del motor adentro** (`console/dc/poc/tablas-mpq-viva`, el vigia puesto DENTRO del
  bucle): en la vuelta 3 el bucle tiene `leidoPtr= 2351579416 leidoLen= 49` y el **mismo** elemento releido por
  indice del arreglo vivo, en ese mismo instante, tiene `vivoPtr= 2351579416 vivoLen= 10`. El arreglo esta
  bien; el valor del bucle NO.
* **El vigia en el runtime** (copia privada de libgodc con un `vigia_viva.c`): `largos= 12 10 10` en **todas**
  las recolecciones, la cabecera del `struct` se actualiza y los **8** lugares que tenian la direccion del
  arreglo (7 en el stack + 1 en el monton) se actualizaron todos a la direccion nueva. Las **cadenas** no
  tienen "un lugar" que apunte a ellas: apuntan ellas al medio del buffer que leyo el decodificador.
* **La memoria reusada**: el par {puntero leido, 49} se busco palabra por palabra en los **16 MB enteros** de
  RAM de la consola y **no existe**. El largo no salio de ninguna copia: se leyo de memoria que ya es de otro
  objeto, a la que la cadena sigue apuntando.
* **El instrumento del segundo intento** (dentro del POC del corpus): los nombres que fallan son nombres
  **bien formados** (no basura) y el **mismo** nombre consultado de nuevo **si** se encuentra
  (`segundo intento contiene= true`). Lo que fallo fue el **valor** de la primera llamada.
* **Descartado midiendo**: punteros interiores dentro del arreglo (`RECHAZA= 0`), los bancos de registros
  r8..r13 (con el derrame/recarga alrededor de `gc_collect` el defecto no cambia), el tope de 64 KiB del
  barrido del stack (`usado= 5728`, no se recorta) y el arreglo de **punteros interiores** de DC-002.36: esta
  aplicado en el runtime y el defecto **igual** pasa (se corrio el POC en HEAD contra el runtime con la cadena
  canonica completa: la vuelta 3 volvio a salir con basura), o sea que el agujero es el valor que vive donde la
  recoleccion **no lo puede ver**.

**El arreglo, en los dos bucles del arranque** (motor y sonda del corpus): recorrer por **indice** y releer el
nombre del arreglo **vivo** en cada vuelta, en vez de `for i, x := range cfg.MpqLoadOrder`. Es lo que se midio
sano en la misma corrida (el recorrido por indice del POC, perfecto en todas las vueltas, con los mismos datos
y el mismo recolector que rompian el `range`). No se toco el runtime: el arreglo es del codigo del arranque.

**Los numeros del cierre** (runtime con la cadena canonica completa, POC `tablas-mpq` con el arreglo, los 3 MPQ
del disco en **una** pasada):

| | antes (corpus de DC-002.33) | con el arreglo |
|---|---|---|
| vuelta 3 (`d2char.mpq`) | `TABLAS-MPQ 3  FALLO: file does not exist largoNombre= -1795502946 ruta= ` + volcado de ~400 KB, **sin** `TOTAL` | `nombres= 10009 contenidos= 10009`, las dos tablas `16384/10010` y `3931321293/965121384` (identicas al host) |
| las 3 vueltas | se median en dos corridas (seccion 2b del corpus) | las tres en **una** pasada: `TABLAS-MPQ-TOTAL archivos= 3 nombres= 21253` |

El **mutante** es el POC en HEAD (sin el arreglo, mismo runtime canonico, mismo disco): la vuelta 3 vuelve a
salir con el nombre en basura y el volcado (log de 1.126.577 B contra los 8.207 B de la corrida con arreglo).

**Lo que queda abierto y se dice con su numero**: la **consulta** de los 21.253 nombres pierde entre 0 y 6
nombres por corrida (medido en la corrida del cierre: `10008` de `10009` en d2char y `11082` de `11082` en
d2data => `TOTAL contenidos= 21252`; en otra corrida del mismo binario fue `21250`), siempre con los mismos
sintomas (nombre bien formado + `segundo intento contiene= true`) y siempre en la vuelta en la que cae una
recoleccion. Es el mismo agujero del **valor que la recoleccion no puede ver** (un temporal en un registro en
el momento de recolectar, que en este runtime no se barre) y por eso la **seccion 2b del corpus de DC-002.33 se
borro** (la corrida extra del tercero ya no hace falta) y su **control 2b** -- que ahora se calcula con los
numeros de la seccion 2 -- queda como el **vigia** de ese residuo: verde cuando la consulta cierra con los
21.253, rojo cuando pierde alguno. El arreglo de fondo es del runtime (un
punto de recoleccion que vuelque **todos** los registros, como el derrame de bancos que se probo para r8..r13
y no alcanzo), no del codigo del motor. Cada POC que recorre el `MpqLoadOrder` con `range` y trabajo pesado
adentro tiene el mismo patron (`console/dc/poc/motor-reservas`, `d2app/d2setup/discover.go`).

Barreras: el corpus de DC-002.33 con la corrida de los 3 MPQ en una pasada (**el `TOTAL` sale**, las tres vueltas
en la misma corrida y el control 2b vigilando el residuo de la consulta) y `checar-cadenas-236.sh` (el arreglo de
punteros interiores del runtime sigue aplicado).

Evidencia cruda: `evidencia/dc-002.39-nombre-corrompido.txt`.

## 55. DC-002.40: el `(listfile)` que la consola no podia leer — no era el dato ni el camino encriptado, era el puente de `compress/zlib` (el de una sola vez devolvia `rc_zlib= -5`, la zlib **sin entrada**); y lo que queda trabado (`d2data`/`d2char`) **es el monton de Go**, medido con los mismos bytes en `.bss` (**200/200** contra el fallo en la vuelta **58**)

La tarjeta pedia decir **que compresion** tiene el `(listfile)` y **donde se rompe** la consola, y arreglarlo.
Las dos respuestas, medidas con el host (`scripts/console/sonda-mpq-compr.py`, que lee el MPQ a mano y
descomprime con la zlib de Python, a proposito **sin** codigo del motor) y con la consola
(`console/dc/poc/compr-40` en Flycast, por el camino del motor: `d2mpq.FromFile` + `ReadFile`):

* **La compresion**: los TRES `(listfile)` son `Compress | Encrypted | FixKey` (`0x80030200`), con los
  sectores en **zlib** (primer byte `0x02`) y el **bloque encriptado** (offset con `FixKey`: los offsets
  descifrados de `patch_d2.mpq` son `[12, 674, 1088]`). Numeros del host: `patch_d2` **5502** bytes / crc32
  **502100774** / **162** nombres; `d2data` **466867** / **923032874** / **11082**; `d2char` **418279** /
  **1367999602** / **10009**. En el host los tres se leen y `go test ./d2common/d2fileformats/d2mpq/` esta
  verde: el MPQ no era el problema.
* **Donde se rompia**: en el **puente**, no en el dato. Los 662 bytes del sector 0 llegaban al monton de la
  consola **enteros** (crc32 **3898719478**, el del host) y el crudo del disco tambien (**2166895218**). Lo
  que fallaba era el puente **de una sola vez** que usaba `d2std/compress/zlib`: `d2c_zlib_inflate` devuelve
  **-1** con `rc_zlib= -5` = `Z_BUF_ERROR`, o sea la zlib **sin entrada** (`avail_in` llegaba 0: el slice de
  Go en la firma corria los argumentos). Tampoco era tamano: la cota era **68180** bytes para 4096 de salida.
  El puente en **streaming** (buffers como `uintptr` y el resultado del paso por funciones aparte: la
  convencion que el proyecto pago cara en DC-002.19/2.20), con los MISMOS bytes, da **4096 / crc32
  2274402335**, los del host.
* **El arreglo**: `compress/zlib` de la consola pasa al puente en streaming (el slot del inflate se suelta
  apenas termina el flujo, no solo en `Close`, para no quedarse sin los 4 slots si un camino se olvida de
  cerrar). Con eso **el motor** lee el `(listfile)` de `patch_d2.mpq`: `bytes= 5502 nombres= 162
  crc32= 502100774` (ms 2198), y el mismo paquete leyendo **de a 512 bytes** (9 pasos: el unico control del
  avance de `z.pos` entre pasos) tambien da **4096 / crc32 2274402335**. El control sano de la tarjeta (el DC6
  con implode, que nunca estuvo encriptado) sigue **igual**: **141944** bytes, suma **1831109629**.

**Lo que queda trabado, y NO es la zlib** (`d2data.mpq` y `d2char.mpq`: 11082 y 10009 nombres). Con los mismos
662 bytes y el mismo puente, **200 vueltas con los buffers en el monton de Go** (como los tiene el motor:
`bytes.Reader` + `bytes.Buffer`) fallan en la **vuelta 58** con `rc_zlib= -3` (`Z_DATA_ERROR`, no `-4` de
memoria) y con el crc32 de la entrada **intacto**; las mismas 200 vueltas **con los buffers en `.bss`** dan
**200/200**. O sea: lo que se pisa son los buffers del monton de Go. Y ese monton son dos
`memalign(32, 512 KB)` **pedidos al monton de C** (`libgodc/runtime/gc_heap.c:39-40`: en el log del arranque,
`gc_init-semiespacio0/1` caen DENTRO del arena), que queda con **~1 KB** libre al arrancar y **~34 KB** cuando
falla, con la zlib pidiendo 32 KB de ventana + ~7 KB de estado por flujo; despues de que el motor lee el
`(listfile)` el monton de C ya crecio a **1.961.088** B de arena con **10264** libres. La lectura de `d2data`
ademas **muere muda** y la consola se rearma (39 arranques en 120 s en una de las corridas). La causa de fondo
—el agujero del **valor que la recoleccion no puede ver** de la seccion 54, o el copiado del recolector encima
de un bloque de C— **no queda medida**: es lo primero que hay que medir en DC-002.41 (`t_d2725794`), que es de
donde cuelga el ultimo tramo de esta tarjeta. El corpus lo deja a la vista en su **control 11**, que se pone
ROJO el dia que el monton se arregle (para que nadie se olvide de mudarlo).

Barreras: corpus `checar-compr-240.sh` **13/13** controles con el control sano y **2/2** mutantes cazados
(`avail-in-cero` en el C — `avail_in = 0`, literalmente el defecto medido, que voltea todo lo que descomprima
— y `pos-de-un-tiro` en el Go, al que lo caza el **control 5b**: `M4b` es el mismo paquete con un lector de a
512 bytes, porque el camino del motor pide de a 4096 y descomprime todo en UN paso, y sin ese control el
mutante quedaba verde; medido, y por eso existe), `bash scripts/console/test-std-host.sh` verde en los dos
anchos (13 de 13 de los que hablan con KOS siguen sin compilar en el host) y
`go test ./d2common/d2fileformats/d2mpq/` verde.

Evidencia cruda: `evidencia/dc-002.40-mpq-comprimido.txt`.

## 56. DC-002.41: el mega contiguo del monton de C era la **tabla de hash de `d2data.mpq`** (1.048.576 B pedidos de una sola vez con 671.744 de credito) — el arreglo guarda solo las ranuras **OCUPADAS** (la reserva baja a **185.776 B** y el ahorro en las tres tablas es **1.219.200 B**), y con eso el arranque abre las **TRES** fuentes del disco

La tarjeta pedia decir **quien** pide los 1 MiB contiguos del monton de C y **bajar esa pared**. El
instrumento (`scripts/console/sonda-alloc-241.py`, enganchado a las reservas grandes de
`libgodc/runtime/gc_heap.c` y `gc_runtime.c`) imprime por cada reserva >= `--umbral` el sitio, el tamano, el
**pc** del Go que la pide, el **credito de `sbrk`** (`_arch_mem_top - THD_KERNEL_STACK_SIZE - sbrk_base`) y
el estado del monton. Con `--umbral 16384`, la corrida de la pared dice:

    107  ALLOC-241 makeslice size=262144  pc=8c1af410 credito=933888 arena=1711168 used=1668192
    113  [File Loader][INFO] Adding source: '/cd/juego/patch_d2.mpq'
    114  ALLOC-241 makeslice size=1048576 pc=8c1af410 credito=671744 arena=1973312 used=1930672 free=42640
    116  Out of memory. Requested sbrk_base 0x8d04c000, was 0x8cf4c000, diff 1048576

`pc=8c1af410` resuelto sobre `prog.elf` da **`MPQ.readHashTable` (`d2common/d2fileformats/d2mpq/mpq_hash.go:102`)**,
y el MISMO pc pidio 262.144 bytes siete lineas antes: era la tabla de `patch_d2.mpq` (16.384 ranuras x 16,
que si entra). El mega es la tabla de **`d2data.mpq`** (65.536 x 16 = 1.048.576 exactos), la primera reserva
grande de la SEGUNDA fuente: la corrida muere antes de que salga su linea `Adding source`.

**El arreglo.** La tabla del formato es casi toda **vacia** (en `d2data.mpq` hay 11.083 entradas de bloque: a
lo sumo 11.083 ranuras ocupadas, el **17%** de las 65.536) y las vacias son `0xFFFFFFFF` en las cuatro
palabras: guardarlas cuesta 871.248 B que no hacen falta. Ahora la tabla de hash se guarda como **un bit por
ranura** + las entradas **ocupadas** (16 B cada una, en orden de ranura) + un acumulado **por bloque de 512
ranuras** para convertir numero de ranura en indice compacto. La consulta sigue siendo EL MISMO sondeo del
formato MPQ de DC-002.33 y el checksum de la tabla sigue siendo el de los bytes de la tabla **logica**
(ranura por ranura), que es lo que lo mantiene sirviendo de barrera. Ademas el arreglo se **reserva de una
vez** con la cota de entradas de bloque del header: la primera version crecia con `append` y esa cadena dejo
**905.400 B** de monton de C para un arreglo final de 185.441 (medido), o sea se comia lo que habia ganado.

| archivo | ranuras | ocupadas | reserva antes | reserva ahora |
| --- | --- | --- | --- | --- |
| `patch_D2.mpq` | 16.384 | 216 | 262.144 | **5.616** |
| `d2data.mpq` | 65.536 | 11.083 | 1.048.576 | **185.776** |
| `d2char.mpq` | 16.384 | 10.010 | 262.144 | **162.272** |

**El resultado, medido en la consola.** Con el arreglo el arranque **no pide ningun mega** (ninguna reserva
de 1.048.576 en toda la corrida), abre **las tres** fuentes del disco (`patch_d2.mpq`, `d2data.mpq` — lo que
pedia el criterio 2, **sin** la palanca de DC-002.32 — y ademas `d2char.mpq`, que ninguna corrida habia
alcanzado), inicializa el Asset Manager y lee su primer asset del MPQ. La pared de esta tarjeta queda atras:
el monton de C llega al final con decenas de miles de bytes libres y **258.048** de credito de `sbrk`. Lo que
viene despues ya no es memoria: `panic: index out of range` al cargar el asset **siguiente** — el tramo que
DC-002.43 tiene que diagnosticar (es donde S232 moria MUDA, sin mensaje).

**Barreras.** `TestBuscarHashIgualQueElMapaViejo` compara el sondeo nuevo contra el mapa de objetos de antes
con la tabla **cruda** del archivo como referencia: **21.253 nombres, 0 distintos, 0 fuera del (listfile)** en
los tres MPQ (o sea que cubre TODAS las ranuras ocupadas, no una muestra); los dos checksums de cada tabla son
los mismos numeros de DC-002.33/DC-002.39 (`checksumHash` 2035763179 / 1897791535 / 3931321293); el corpus de
DC-002.34 vuelve a correr los cuatro estados **8 controles OK, 0 fallos** (y su fila S233 pasa de "OOM pidiendo
1 MiB con 39.536 libres, 1 fuente" a "las tres fuentes abiertas, 1 asset leido, sin OOM"); y el corpus de
DC-002.40 queda **13/13** con el control 7 (el motor leyendo el `(listfile)` comprimido de `patch_d2.mpq`, con
el crc32 del host) intacto — de paso, `d2data.mpq` ahora **abre** su tabla (`FromFile` OK) y su `(listfile)`
arranca a streamear 65.536 B antes de morir en el defecto que esa tarjeta ya declaro (el monton de Go), asi que
su control 11 **no** hay que mudarlo. El SANO de `fix-mpq-tablas-233.py` se actualizo al texto nuevo (es el
script que declara cual es el estado sano del arbol: si no, el control 0 del corpus se ponia rojo), y
`checar-motor-disco-234.sh` lleva un arreglo de una linea: el mutante sin `/JUEGO` se pasa con `env` (la
asignacion expandida no es asignacion para bash: el build del mutante moria con "orden no encontrada" y no se
habia visto porque en DC-002.34 la imagen del mutante ya estaba construida).

**Lo que queda anotado.** (a) El arranque se lleva **458.752 B** en CUATRO reservas antes de tocar un MPQ
(2 x 98.304 de `image/bmp.decodeConfig`, 131.072 de `image.NewRGBA` y 131.072 de `ebiten/v2.NewImage`): es la
palanca que le queda a la pared del monton de C, y vive en el camino de la pantalla de arranque, no en el MPQ.
(b) El estado S232 (la palanca de 232, que **vacia** la tabla de d2records: es un instrumento de peso, no un
motor jugable) es el que mas memoria viva pide (4.356.112 B) y con el arreglo muere con un OOM chico
(diff 323.584): es el estado que DC-002.42 tiene que aplicar de verdad.

Evidencia cruda: `evidencia/dc-002.41-monton-de-c.txt`.

## 57. DC-002.39b: el residuo de la consulta **no era un registro** — era el **portero** del barrido conservador, que rechazaba los punteros interiores que solo viven en la pila (184 de 2.560 comparaciones a **0**), y la consulta del corpus **sigue en rojo por otro valor** (`cfg`), medido

**Lo que la tarjeta suponía, y lo que se midió.** DC-002.39 dejó dicho que el valor que se caía vivía "en un
registro" y que había que volcarlos todos. Se probó primero eso: `fix-libgodc-bancos.py` mete
`__asm__ volatile("" ::: "r8","r9","r10","r11","r12","r13")` en la entrada de `gc_alloc()` y de
`runtime_GC()` (el compilador derrama esos bancos en el prólogo y los recarga en el epílogo: verificado con
`sh-elf-objdump`). Medido con el POC chiquito, **los 184 fallos de 2.560 siguen ahí, en los MISMOS índices**
(`ronda= 1 i= 56`, `ronda= 2 i= 42`, `ronda= 3 i= 21`, ...). O sea: no está en esos bancos; lo que lo pierde
es el **portero**, no dónde viva.

**El POC que lo ata (`console/dc/poc/banco-vivo`, 191 líneas, 30 s de Flycast).** Arma un buffer con nombres
y 64 `string` que son **pedazos** suyos (punteros interiores, y lo único que los mantiene vivos), y recorre
el arreglo con `range` haciendo **reservas adentro** del cuerpo (el trabajo del motor). En cada vuelta
compara el valor del bucle contra el mismo elemento releído por índice:

    BANCO-VIVO-FALLO ronda= 1 i= 56 leidoPtr= 2350049328 leidoLen= 13 vivoPtr= 2350615136 vivoLen= 13
    BANCO-VIVO-TOTAL rondas= 40 comparaciones= 2560 fallos= 184 suma= 15974400

y clasificando los dos punteros contra los semiespacios (que el runtime imprime al arrancar) se ve que el
que queda en el semiespacio **abandonado** es **siempre el valor del bucle**: **12 de 12** contra **0 de 12**
del releído por índice. La `suma` (1.597.440) es la misma con y sin el arreglo: se pierde la dirección, no el
dato — el mismo síntoma del `largoNombre= -1795502946` del corpus.

**La causa, en el código.** El barrido conservador de la pila (`gc_scan_range_conservative`) pedía DOS cosas
antes de actualizar una palabra: que la dirección cayera en el from-space (`gc_might_be_heap_ptr`, línea
1082) y que fuera un **comienzo de objeto** (`gc_is_valid_object_start`, línea 976). Y esa segunda rechaza
los interiores por dos lados: exige alineación a 8 bytes (línea 992) y valida el "encabezado" que hay 4 bytes
más abajo (líneas 996-999). El resolutor que **sí** sabe de interiores (`gc_relocate`, línea 474, con
`gc_interior_base`, línea 434: el arreglo de DC-002.36) quedaba detrás del portero, así que al candidato
nunca le llegaba la pelota y su ranura de la pila se quedaba apuntando al semiespacio abandonado.

**El arreglo (`fix-libgodc-candidato.py`).** En los 12 puntos del barrido conservador el portero pasa a ser
`gc_candidato_del_barrido()`, que sólo exige lo mismo que ya se pedía (que la dirección caiga en el
from-space) y deja decidir al resolutor: comienzo de objeto (lo copia), interior (copia el objeto que lo
**contiene** y devuelve `nuevo_comienzo + desplazamiento`) o nada. El único `gc_is_valid_object_start` que no
se toca es el del camino rápido del propio resolutor. El script es idempotente (dos corridas, mismo sha256),
tiene `--quitar` (el mutante) y `--estado` (para que el corpus sepa qué runtime mide). Medido: **184 de 2.560
→ 0**, y **más rápido** (1.578 ms → 1.473 ms en la misma ventana): ahora los pedazos del mismo buffer los
resuelve el cursor de `gc_interior_base` en vez de descartarse con una validación que lee encabezados.

**Lo que NO quedó cerrado (y es la tarjeta siguiente).** El control 2b del corpus de DC-002.33 **sigue en
rojo**, y no por este valor: medido con el corpus entero y con `checar-consulta-233.sh` (la sección 2 sola,
repetible) sobre un runtime privado y un clon del árbol en `f2bab3af`, la consulta da **162 de 21.253**
(`d2data.mpq` 0 de 11.082 y `d2char.mpq` sin línea) **igual con el arreglo y sin él**, y el POC imprime el
camino del config ya corrompido: `sin lista de nombres en ata\globa/nombres_d2data.txt` (un pedazo de
`data\global` donde tenía que decir `/cd/juego`), y en la tercera vuelta el nombre del archivo sale en NULOS.
Los dos valores que se corrompen se leen **a través del puntero `cfg`**, así que el que queda viejo es `cfg`
mismo (un local, barrido conservadoramente, que apunta al dato del config: no alineado a 8). Con este arreglo
debería resolverlo `gc_interior_base` con desplazamiento 0, y no lo resuelve: **eso es DC-002.39c**, con los
instrumentos ya puestos (el número con `checar-consulta-233.sh`, el caso chico con `banco-vivo`, y el POC
`tablas-mpq-viva` de DC-002.39, que hoy no enlaza porque le faltan sus `d2c_vigia_*` en `csrc/`).

**Barreras.** `checar-cadenas-236.sh` lleva ahora los controles **1d** (con la cadena puesta: banco-vivo
`fallos= 0` y la suma de 15.974.400) y **2d** (con el mutante: `fallos > 0` y la misma suma), así que si
alguien saca este arreglo de la cadena el corpus se pone rojo. El mutante se pidió con
`fix-libgodc-candidato.py --quitar --build`: banco-vivo **184 de 2.560** contra **0**, y la consulta **162**
en los dos casos (no es este defecto).

Evidencia cruda: `evidencia/dc-002.39b-candidato-interior.txt`.

## 58. DC-002.42: la tabla de `objectLookups` **DE VERDAD** fuera del binario — viaja en el **disco** (`/JUEGO/OBJECTLOOKUPS.PACK`, 773.869 B) y el motor la lee **por registro** (el indice son 31.644 B y la ventana 4 KiB, contra 1.672.892 B de `.data`): **1.797.792 B** menos de imagen, el simbolo `_go..C3111` **desaparecido** y los 7.891 registros iguales a los del host, leidos en la consola con el crc cerrando

La tarjeta pedia aplicar DE VERDAD la palanca que DC-002.32 solo midio como instrumento (vaciando la tabla,
que deja al motor sin los 7.891 registros) y decidir con medicion DONDE vive la tabla. Ahora vive en el
**disco** y el motor la lee **por registro** cuando la usa; y el host sigue verde.

* **Los dos archivos del host NO entran al binario** (por NOMBRE, no por etiqueta): `sh-elf-gccgo` **ignora**
  `//go:build` — medido con dos sondas en `/tmp/probe-tags` (un archivo marcado `dreamcast` y otro marcado
  `!dreamcast` compilan los dos, exit 0). `build-motor-all.sh` y `recompilar-paquete-motor.sh` excluyen
  `object_lookup_record_data.go` (el volcado) y `object_lookups_host.go` (el registro de la tabla como fuente
  y el escritor del archivo). El MUTANTE de la tarjeta es `DC_TABLA_LOOKUPS=dentro`.
* **El peso**: el `.gox` de `d2records` para SH-4 baja de **4.758.772** a **2.278.484** B (-2.480.288) y el
  `1ST_READ.BIN` del POC `motor-arranque` (los 3 MPQ + el archivo de la tabla en la ISO) de **13.808.328** a
  **12.010.536** B: **1.797.792 B** (1,71 MiB, el **10,7%** de los 16.777.216 B de la consola). El simbolo mas
  gordo del mutante es `_go..C3111` (**1.672.892** = 7.891 x 212 B, `sh-elf-nm -S --size-sort prog.elf`) y en
  el arreglo **no existe**: el mas gordo pasa a ser `_aligned_vertex_pool` (**131.072** B).
* **El archivo**: cabecera de 32 B (`"D2OL" | version | flags | n | idxOff | recOff | nGrupos | crc |
  maxLen`), indice `n x 4` y los registros (act u8, tipo u8, id u16, las **23** cadenas con su largo u8 y 4
  i16). Lo arma `console/dc/gen-lookups-pack` desde `d2records.ObjectLookupsCompilados()` (MISMA tabla, sin
  segunda copia) y lo lee `d2core/d2records/object_lookups_store.go`. El corpus comprueba que el archivo de
  dentro del ISO9660 es el del host (`sha256 0255463368aa6513...`, via `leer-iso.py`).
* **La consola lo lee** (POC `motor-arranque`, antes de arrancar el motor): `bytes= 773869 registros= 7891
  grupos= 10 maxRegistro= 165`, `crc del archivo= 2553790715 crc leido= 2553790715` (**el lector verifica el
  crc recorriendo el archivo entero al abrirlo**), `indice en RAM= 31644 bytes (4 x 7891) + ventana de 4096`.
  Y los numeros que el MOTOR pide salen iguales a los del host: la tabla del DS1 `12 37 39 35 36` y sus dos
  overrides `0`, `342`; `FindObject(1,2,0)` = `rogue fountain (12) FN NU HTH 12 -1` y `FindObject(1,1,0)` =
  `gheed-ACT 1 TABLE GH NU HTH`. En RAM, en vez de 1.672.892 B (+116.048 de cadenas) vivos desde el arranque:
  **31.644 + 4.096 = 35.740 B**. El `init` de `RecordManager` ya **no** arma `Object.Lookup` con los 7.891
  registros (eso era, medido en DC-002.32, lo que obligaba a tener la tabla entera desde el arranque): las
  filas se llenan a demanda (`guardaObjectRecord`) y `DS1ObjectClass` lee solo los grupos de tipo Item y solo
  el campo `ObjectsTxtId`.
* **El viaje de ida y vuelta** (el guardian del formato): `TestPackObjectLookups` escribe el `.PACK` desde la
  tabla compilada, lo reabre con el LECTOR DE LA CONSOLA y compara los **7.891** registros campo por campo
  (las 23 cadenas + Act/Type/Id + los 4 numericos). El orden de las cadenas lo fija
  `ordenDeLosCamposDeTexto` (23 entradas): cambiar el struct o ese orden **cae** la prueba. `go test
  ./d2core/d2records/...` verde.
* **El arranque de la consola**: en la corrida buena, el estado de este arreglo **abre** `/cd/juego/patch_d2.mpq`
  y `/cd/juego/d2data.mpq`, inicializa el Asset Manager y **lee su primer asset** del disco
  (`Loaded /cd/juego/patch_d2.mpq -> /data/global/excel/LvlTypes.txt`), con **736 B** libres en el monton de C
  y el semiespacio del GC en **401.328** B; el mutante (la tabla adentro) llega al mismo tramo y muere con
  `Out of memory ... diff 7630848`; y el control negativo (la MISMA capa **sin** el archivo en el disco) llega
  a **3** fuentes + 1 asset y muere con el OOM chico de DC-002.41 (`diff 319488`). O sea: el muro del tramo
  siguiente esta **igual** con y sin la tabla.
* **Dos numeros del enunciado que DC-002.41 movio** (y se reportan tal cual, no se maquillan): el mutante ya
  **no** muere en el primer MPQ (muere pidiendo 7.630.848 B contiguos al monton de C) y el instrumento ya no
  llega a `LvlPrest.txt`: con el arreglo de la tabla de hash de `d2data.mpq` en HEAD, los dos se corren un MPQ
  entero. El corpus igual **caza al mutante**: por la imagen (+1.797.792 B) y por el simbolo de la tabla.
* **La flakiness del arranque, medida y NO de esta tarjeta**: en las corridas de esta sesion los TRES estados
  (incluido el que **no** tiene la tabla ni en el binario ni en el disco) se quedan en el primer tramo: la
  ultima corrida completa del corpus dio **15 controles OK y 5 con fallo**, y los 5 son los del arranque (el
  FIX y el SIN-ARCHIVO agotaron **6 intentos** cada uno sin abrir una sola fuente). El log dice `could not load
  file stream config.json (file not found: config.json)` (con una marca `ARQ-MARCA gc_collect` justo antes) y
  despues `game files not found at "C:/Program Files (x86)/Diablo II"` — la familia del VALOR QUE LA
  RECOLECCION SE LLEVA (DC-002.39 y DC-002.40), aca con la cadena del camino de `config.json`; la MISMA lectura
  hecha por el POC **antes** del motor funciona. Y una vez que la vuelta 1 falla, el motor **crea** un config
  por defecto en `/rd/OpenDiablo2/config.json` (con los caminos de Windows) y ninguna vuelta posterior ve el
  disco: el sintoma se auto-sostiene dentro de la sesion del emulador (por eso el corpus abre un emulador nuevo
  en cada intento). Y no es una sospecha: DC-002.39b (commit e6a63acf, en HEAD) midio el MISMO valor ("el
  camino del config corrompido en `ata\globa`") y lo paso a DC-002.39c ("el puntero `cfg`, un local barrido
  conservadoramente"). El corpus no la maquilla: corre cada estado hasta 6 veces, reporta el intento y da la
  tabla de vueltas (`scripts/console/vueltas-242.py`) con las fuentes y los assets de cada una.

El corpus es `scripts/console/checar-lookups-242.sh` (estados FIX / MUTANTE / SIN-ARCHIVO, con los controles
del peso, del simbolo, del archivo dentro del ISO y de la consola). Evidencia cruda:
`evidencia/dc-002.42-lookups-disco.txt`.

## 58. DC-002.39c: el barrido del GC se **CORTA en una cabecera en CERO** y deja 63.024 B sin barrer — el arreglo de los nombres queda en el from-space, y la cabecera en cero la deja la **copia por Store Queue** con la fórmula vieja (`>>26`)

Lo que DC-002.39b dejó anotado como "queda viejo el puntero `cfg`" se midió y **no era eso**: el `cfg` local no
queda viejo (el GC lo actualiza a la dirección nueva del objeto), y el JSON tampoco (el testigo de literales,
que vive en `.rodata`, no se rompe nunca). Lo que se rompe es **la memoria a la que `cfg` apunta**.

El eslabón, medido con el POC `tablas-mpq-viva` (el de DC-002.39, que **hoy ya enlaza**: le faltaba su
`d2c_fallo`, que vivía en el `gc_copy.c` de la copia privada; se portó a `vigia_fallo.c` y se le sumaron tres
ganchos privados al `gc_copy.c` — `vigia_imprime` al entrar y al salir de cada recolección, `vigia_obj` al
barrer el objeto del tipo de la config y `vigia_barrido`/`vigia_corte` con el resultado del barrido de Cheney):

1. en la **recolección 1** (fase 17, dentro del cuerpo de `d2data.mpq`) el **barrido de Cheney se corta**:
   `objetos= 47 cursor= 8c3278e0 alloc= 8c336f10 corto= 1 queda-sin-barrer= 63024`, y la cabecera que lo corta
   está **en cero** (`corte_datos= 0`, con 136 palabras en cero seguidas); las 4 palabras de antes son texto
   (`lite` / `dhth` / `.dcc` / `\r\nda`), o sea el cursor cayó en el **medio** de un objeto medio copiado;
2. la copia del objeto de la **config** (`hdr MUDADO 8c2a5890 -> 8c335398`) cae **dentro** de ese tramo sin
   barrer: su campo `[]string` no se sigue (`OBJN ... barridos= 0`), el arreglo de los 3 nombres **no se
   copia** y queda en el from-space;
3. la recolección 2 usa ese espacio como destino y **lo pisa**: `cfg` fase 17 gc 2 lee `6f4d5c6c 6574736e …`
   ("l\Monste…"), `largos= 1702130542,1381260368,1414089810` y el nombre sale `LITDTHTH.D`;
4. el número: `d2data.mpq **nombres= 0** contenidos= 0` de 11.082 (el mismo 162 de 21.253 de DC-002.39b).

La cabecera en cero la deja **la copia por Store Queue con la fórmula vieja**: `~/dreamcast/libgodc` tenía
`*QACR0/1 = (dst_addr >> 26) & 0x1C`, que para el montón (`0x8C……`) da área 0 → las escrituras van a la BIOS y
se descartan, o sea **el defecto DC-002.24 estaba VIVO en el vendor**. El script que corrió último en la cadena
fue el de DC-002.39b y **no re-aplica** el de la SQ: la cadena de `fix-libgodc-*.py` no se sostiene sola
(tarjeta DC-002.39d).

La **contra-prueba**, con el mismo POC, el mismo disco y el mismo gox (lo único distinto es lo que aplica
`fix-libgodc-sq.py`, `>> 24` + la invalidación de caché del destino):

| | con `>>26` (el vendor vivo) | con `>>24` (la SQ arreglada) |
|---|---|---|
| barrido de la recolección 1 | 47 objetos, `corto=1`, **63.024 B sin barrer** | 138 objetos, `corto=0`, `queda-sin-barrer=0` |
| `patch_d2.mpq` | 162 de 162 (2.421 ms) | 162 de 162 (2.421 ms) |
| `d2data.mpq` | **0 de 11.082** (10.815 ms) | **11.082 de 11.082**, `largas= 0` (21.122 ms) |
| `(listfile)` de `d2data` | `FALLO: zlib: invalid header` | `bytes= 466867` |
| tablas de `d2char.mpq` | no llega | `16384 / 10010 / 3931321293 / 965121384` (las del host) |
| `banco-vivo` (control 1d) | — | 40 rondas, 2.560 comparaciones, **fallos= 0**, suma 15.974.400 |

Los mismos números en dos corridas independientes (y repetidos dentro de cada corrida tras el reinicio de la
consola). El **TOTAL de 21.253 sigue sin imprimirse** porque con los 3 MPQ la consola **se reinicia** al entrar
en `d2char` (el síntoma de DC-002.43, no de la consulta).

**El método, para que no se repita:** medir siempre sobre una copia privada del runtime (`cp -a` + los
`fix-libgodc-*.py` con la ruta como argumento) y **comprobar en qué estado está el vendor antes de creer
cualquier número**: `grep -n 'dst_addr >> 2' ~/dreamcast/libgodc/runtime/gc_copy.c` tiene que decir `>> 24`.

**Ambiente (costó dos corridas):** el `flycast.AppImage` muere con **SIGBUS** si `/tmp` (tmpfs, 16 GB) está
casi lleno; el log no trae una sola línea de la consola y parece un fallo del POC. Con el AppImage ya extraído
(`FLYCAST=$HOME/dreamcast/dl/squashfs-root/AppRun`) anda. Evidencia:
`console/dc/evidencia/dc-002.39c-barrido-cortado.txt`.

## 59. DC-002.43: el instrumento que dice EN QUÉ LLAMADA se muere el arranque (y con cuánta memoria)

Evidencia completa, con los logs crudos: `console/dc/evidencia/dc-002.43-marcas-de-carga.txt`.

Las líneas `[File Loader][INFO] Loaded ...` salen **después** de que la carga salió bien: no dicen nada del
pedido que estaba en curso cuando el motor murió. Lo que se agregó es una marca al **entrar** y al **volver**
de los dos puntos por los que el Asset Manager pide un archivo, y **cada marca lleva la memoria de ese
instante** (los mismos dos números de la línea `Memory: arena= used= free=` del runtime, sacados de
`mallinfo()`, más el semiespacio del GC de Go y cuántas recolecciones van):

    Loader.Load    ENTRAR (antes de tocar ninguna fuente) y VOLVER (con el stream abierto)
    LoadFile       ENTRAR y VOLVER de la lectura COMPLETA (`ioutil.ReadAll`, la que se lleva el archivo entero)

| archivo | qué es |
| --- | --- |
| `d2common/d2loader/marca_carga.go` | las constantes y el porqué; sin `//go:build` (lo compilan los dos lados) |
| `d2common/d2loader/marca_carga_host.go` | `!dreamcast`: en el host `Marca()` NO HACE NADA |
| `d2common/d2loader/marca_carga_godc.go` | `dreamcast`: llama al C de la consola |
| `console/dc/std/csrc/marcas_carga.c` | el que imprime: va en el C de la STDLIB, que enlazan TODOS los binarios de la consola (ningún POC cambió) |
| `scripts/console/build-motor-all.sh`, `recompilar-paquete-motor.sh` | sacan `marca_carga_host.go` del build de la consola **por NOMBRE** (gccgo NO respeta `//go:build`, medido en DC-002.42) |

Ojo con el `init()` de `marca_carga_godc.go`: **medido, NO se ejecuta** en la consola (nadie referencia
`d2loader..import` en `prog.elf`: los inits de los paquetes del motor no entran en la cadena de arranque). El
control que vale es el de la tabla del corpus: **si no hay NINGUNA marca, la corrida no trae el instrumento**.

### lo medido (estado S232 del corpus, 25 s de consola serie)

    ARQ-CARGA entra     /data/local/use                        cfree=40104 cused=2344184 gsemi=370232/524288 gc=8
    ARQ-CARGA entra     /data/global/excel/LvlTypes.txt        cfree=40104 cused=2344184 gsemi=385072/524288 gc=8
    [File Loader][INFO] Loaded /cd/juego/patch_d2.mpq -> /data/global/excel/LvlTypes.txt
    ARQ-CARGA volvio    /data/global/excel/LvlTypes.txt        cfree=40104 cused=2344184 gsemi=390512/524288 gc=8
    ARQ-CARGA lee-entra /data/global/excel/LvlTypes.txt        cfree=40104 cused=2344184 gsemi=390512/524288 gc=8
    ARQ-CARGA lee-volvio /data/global/excel/LvlTypes.txt       cfree=40104 cused=2344184 gsemi=397688/524288 gc=9
    ARQ-CARGA entra     /data/global/excel/LvlPrest.txt        cfree=40104 cused=2344184 gsemi=392792/524288 gc=13
    [File Loader][INFO] Loaded /cd/juego/patch_d2.mpq -> /data/global/excel/LvlPrest.txt
    ARQ-CARGA volvio    /data/global/excel/LvlPrest.txt        cfree=40104 cused=2344184 gsemi=392184/524288 gc=16
    ARQ-CARGA lee-entra /data/global/excel/LvlPrest.txt        cfree=40104 cused=2344184 gsemi=392184/524288 gc=16
    ARQ-MARCA gc-por-umbral-bytes-usados v=396312 ... v=521024   <-- de acá para abajo, DENTRO de la lectura
    fatal error: out of memory
    Memory: arena=2384288 used=2344184 free=40104

**La última marca** es `lee-entra /data/global/excel/LvlPrest.txt`, con el montón de C en **40.104 libres**
(2.344.184 usados) y el semiespacio en **392.184 de 524.288 (132.104 libres)**, 16 recolecciones. El número
que no existía. Y el que la cierra (del propio archivo, sin usar el código del motor: `sonda-mpq-compr.py`):

    patch_d2.mpq  data\global\excel\LvlPrest.txt  hashIdx=7534 blockIdx=15
      CompressedFileSize=22259  UncompressedFileSize=134937  (Implode, 33 sectores de 4096)

=> la lectura entera son **134.937 B** y en el semiespacio quedaban **132.104**: el archivo **no entra**. La
lectura crece a saltos y la consola se muere cuando el semiespacio llega a **521.024 de 524.288**. La pared
**NO es el montón de C**: mientras todo eso pasa, `cfree`/`cused` **no se mueven** (40.104 / 2.344.184 desde
el primer asset hasta la muerte).

**Respuesta a la pregunta de la tarjeta: es RAM, y es el semiespacio del GC de Go**, y quien lo llena es la
lectura COMPLETA de un asset de 134.937 B. Lo que queda para la tarjeta siguiente es la decisión (leer por
trozos en vez de `ReadAll`, o subir el semiespacio), y se toma con estos números sin otra corrida a ciegas.

### hallazgo 0: `libgodc.a` VIEJO (y el control nuevo del corpus)

`libgodc.a` se arma aparte (`make -C ~/dreamcast/libgodc`) y las fuentes del runtime las cambian los
`fix-libgodc-*.py`. Medido hoy: `libgodc.a` de las **01:40** contra `runtime/gc_copy.c` de las **02:54** (el
arreglo de la Store Queue de DC-002.39c). Con ese `.a`, el arranque con los TRES MPQ del disco **no llega ni a
abrir un MPQ**: lee su config, entra en la pantalla de error del motor y **muere dibujándola** (`game files not
found at "C:/Program Files (x86)/Diablo II"` + `Unhandled exception` con `PR 8c448dac` = `d2app.App.updateInitError
app.go:718`), con CERO líneas `Adding source:`. Rearmado el `.a`, **el MISMO binario** llega a los tres MPQ y
lee dos assets. Por eso `checar-motor-disco-234.sh` trae el **control 0b**: rojo si hay algún `runtime/*.c` más
nuevo que `libgodc.a`, con el `make` escrito en el mensaje.

### el corpus de DC-002.34, tres veces (criterio 2)

    corpus-ins   (instrumentado, primera corrida)     9 controles OK, 0 con fallo
    corpus-ins2  (instrumentado, segunda corrida)     9 controles OK, 0 con fallo
    corpus-plain (SIN el instrumento, la MISMA capa)  9 controles OK, 0 con fallo

Los nueve controles son los ocho de DC-002.34 más el **0b** (el `libgodc.a` al día). Lo que cambia y lo que
no, con las tres tablas en la mano:

| estado | instrumentado | sin instrumento | qué no cambia |
| --- | --- | --- | --- |
| BASE | 1 vuelta, muda, 0 assets, 1 fuente, semiespacio máx **410.760** (817 marcas) | 1 vuelta, muda, 0 assets, 1 fuente, máx **424.192** (910 marcas) | las cinco columnas de la izquierda |
| S233 | 1 vuelta, `fatal error: out of memory`, 2 assets, 3 fuentes, máx **521.024** | 1 vuelta, `GC to-space overflow - your live data doesn't fit in the heap`, 2 assets, 3 fuentes, máx **509.624** | toda la clasificación |
| S232 | igual que S233 (1 vuelta, mensaje, 2 assets, máx **521.024**) | 1 vuelta, muda, 2 assets, 3 fuentes, máx **487.656** | 2 assets, 3 fuentes, 1 vuelta |
| S232SOLO | 1 vuelta, muda, 0 assets, máx **410.760** | 1 vuelta, muda, 0 assets, máx **424.192** | las cinco columnas |
| MUTANTE | 2 vueltas, el cartel, 0 fuentes | 2 vueltas, el cartel, 0 fuentes | todo |

**Los cinco estados siguen siendo los mismos cinco** (vueltas, fuentes, assets, cartel y familia de muerte), y
**el instrumento no mueve la muerte**: la corrida sin el instrumento llega al mismo punto y hasta dice la pared
con sus propias palabras (`GC to-space overflow - your live data doesn't fit in the heap`). Lo que **no** es
comparable corrida a corrida es el semiespacio máximo: medido con la MISMA imagen sin instrumento, dos corridas
de 25 s dan **487.656** y **408.088** (80 KB de diferencia, 19%), así que las diferencias instrumentado / sin
instrumento (10 a 33 KB) caen **dentro** de la dispersión de la corrida. Los dos corpus instrumentados, en
cambio, dieron los cinco números **idénticos** entre sí: con el instrumento la muerte queda clavada en la pared
(521.024) o en la tormenta de recolecciones, y por eso se repite.

Dos cosas más que la tabla deja ver (son de DC-002.41/DC-002.42, no del instrumento): `BASE` y `S232SOLO`
mueren **mudas pero con 835/828 recolecciones** y su última marca de carga es `volvio config.json` (o sea que
mueren ANTES de pedir el primer asset: es la tormenta del objeto-por-entrada), y `S232` y `S233` dan hoy el
MISMO `1ST_READ.BIN` y los mismos números (la palanca de DC-002.32 ya no cambia el peso de la imagen, porque
desde DC-002.42 la tabla vive en el disco).

### cómo se repite

    make -C ~/dreamcast/libgodc -j8            # 0) si el control 0b sale rojo
    cd ~/Proyectos/diablo2consoles
    go build ./d2common/d2loader/... ./d2core/d2asset/...   # el lado HOST no cambia de comportamiento
    SAL=<dir> SRC=<copia> GEN_HEAD=<capa con el instrumento> \
        bash scripts/console/checar-motor-disco-234.sh --medir 25     # 25 s: con 20 s el último mensaje no sale
    grep -a "ARQ-CARGA" <SAL>/run-s232-con-juego/flycast.log | tail

El estado `S232` es el que importa acá; los cinco salen en `<SAL>/tabla.txt`.

### lo que esta tarjeta NO mide (declarado)

* **No arregla la muerte**: la deja dicha. El arreglo (leer por trozos / subir el semiespacio) es la tarjeta
  siguiente; los números de arriba son los que la deciden.
* **No mide qué pasa después de pasar `LvlPrest.txt`**: el arranque muere ahí.
* **No toca el `ReadAll` de `LoadFile`**: la marca lo señala, no lo cambia.
* La lectura de DC-002.41 (`panic: index out of range` justo después de `LvlTypes.txt`) **no se reprodujo** hoy
  con el runtime al día: hoy la muerte es DENTRO de la lectura de `LvlPrest.txt`. Queda anotado para la tarjeta
  siguiente.

## 60. DC-002.39c, la segunda mitad: la consulta de los **21.253 cierra** — el que se llevaba la corrida puesta era el `ReadFile("(listfile)")` del TERCER MPQ (salta a una **dirección basura**), no la consulta

La sección 58 dejó el defecto medido en los dos sentidos y el `TOTAL de 21.253` sin imprimir, con la razón
declarada ("la consola se reinicia al entrar en `d2char`"). Lo que faltaba era **qué** la tira abajo. Se midió
con una **sonda de paso** en el POC (el montón de C y el reloj en cada tramo) y con el `ReadFile` del
`(listfile)` bajo una palanca, y el resultado cambia el diagnóstico: **el que mata la corrida es el
`(listfile)`, no la consulta.**

1. **El rojo de partida**, con el runtime bueno (la copia privada con `>>24` y el candidato interior de
   DC-002.39b): `11244 de 21253` — `patch_d2.mpq` 162/162, `d2data.mpq` 11082/11082 y `d2char.mpq` **sin
   línea**, porque después de sus tablas (`16384 / 10010 / 3931321293 / 965121384`) la corrida se reinicia.
2. **Dónde se va**: con la sonda puesta, la consola lo dice ella misma, dentro del `(listfile)` de `d2data`:

       TABLAS-MPQ-SONDA listfile-d2data.mpq-antes libreC= 3488 ms= 12579
       Unhandled exception: PC 656a626f, code 1, evt 00e0
        R0-R7: 00000000 656a626f 8c3b29a8 …      SR 40000101 PR 8c0159aa
       Encountered Data address error (read)      kernel panic: unhandled IRQ/Exception

   `0x656a626f` leído byte a byte es **texto** (`"obje"`, un pedazo de las rutas del `(listfile)`): el salto se
   fue a un pedazo de **dato**, el mismo género de defecto que DC-002.23/DC-002.24. Y el `PR` dice dónde
   estaba el que corrió: `sh-elf-addr2line -f -C -i -e prog.elf 8c0159aa` → **`bytes.Buffer.ReadFrom`**
   (`bytes/bytes.go:186`), que es el camino de lectura del MPQ. Sin la sonda, el mismo fallo se ve como un
   reinicio mudo (`REIOS: Booting up`), que es el síntoma que la tarjeta tenía anotado como de DC-002.43.
3. **La contra-prueba** (un solo cambio: `leerListfile = false`, mismo binario, mismo disco, mismo gox):

       TABLAS-MPQ-TOTAL archivos= 3 nombres= 21253 contenidos= 21253 libreFinal= 3448

   y la corrida **no muere**: en 150 s hace tres pasadas de los 3 MPQ. El montón de C no se mueve durante la
   consulta de `d2char` (**2792 B libres** de punta a punta, 20 marcas): la consulta no es la pared.
4. **El número de la tarjeta**, con el instrumento del corpus:

       checar-consulta-233.sh 3 120  ->  3 de 3 vueltas con los 21253
       (patch_d2.mpq:162/162  d2data.mpq:11082/11082  d2char.mpq:10009/10009)

5. **El mutante**, que es una palabra (`>> 24` → `>> 26` en los dos QACR de la copia privada, con
   `scripts/console/mutar-libgodc-sq.py`): la MISMA suite da `162 de 21253` (`d2data.mpq` sin línea). El
   mutante caza la regresión y el arreglo de la SQ es el que hace la diferencia.
6. **Lo que no se rompe**: `banco-vivo` sigue en `rondas= 40 comparaciones= 2560 fallos= 0 suma= 15974400`
   (control 1d de DC-002.39b).

**Decisión de instrumento (no es esconder nada):** el POC `tablas-mpq` mide **la consulta**; el `(listfile)`
de `patch_d2.mpq` lo mide su corpus (`checar-compr-240.sh`, control 7, contra el crc32 del host), y el defecto
del `(listfile)` del tercer MPQ queda **medido acá** y abierto en su tarjeta. El POC imprime
`(listfile) NO SE LEE (DC-002.39c: dispara un salto a basura)` en cada vuelta, para que el recorte se vea en el
log y no se confunda con una lectura que sí funcionó.

**Lo que queda abierto (declarado):** (a) el defecto del `(listfile)` del tercer MPQ, con su mutante natural
(volver a leerlo deja el corpus en 11244 de 21253); (b) el vendor sigue en `>>26`, o sea **el defecto
DC-002.24 sigue VIVO** y la cadena de `fix-libgodc-*.py` no se re-aplica entera (DC-002.39d): ningún número
sacado del vendor es limpio mientras eso siga así; (c) `checar-cadenas-236.sh` no se corrió porque su sección
del mutante toca el **vendor compartido** y había otro agente vivo sobre el repo. Evidencia:
`console/dc/evidencia/dc-002.39c-consulta-21253.txt`.

## 61. DC-002.39d: la cadena del runtime **paso por paso** — el que corre último manda, el paso que se llevaba la Store Queue queda identificado, y el vendor de hoy **ya está completo** (verificado por sha256)

La tarjeta nació de una medición: a las 02:45 `~/dreamcast/libgodc/runtime/gc_copy.c` estaba en `>>26` (la
fórmula vieja del QACR: el defecto DC-002.24 **vivo**) aunque `fix-libgodc-sq.py` existe y promete
idempotencia. Lo que faltaba era **quién pisa a quién**. Medido sobre una copia privada, con el sha256, los
bytes y el estado de la Store Queue después de cada paso:

| paso | sha256(16) | bytes | SQ |
|---|---|---|---|
| pristino (`git show HEAD`, una lectura: la regla del proyecto) | `130de736` | 43749 | `>>26` |
| `fix-libgodc-gc.py 512` (pristino + ABI + interfaces) | `379271ee` | 44509 | `>>24` |
| `fix-libgodc-candidato.py` (modo normal: reset + interiores + candidato + marcas) | `fec6ef13` | 50074 | **`>>26`** |
| `fix-libgodc-sq.py --sin-pristino` (solo la Store Queue) | `988e80fc` | 50834 | `>>24` |

O sea: **el último que corre es el que manda**, y el candidato en su modo normal (el que promete
idempotencia) hace `git checkout` de `gc_copy.c` y **se lleva la Store Queue**. Es exactamente el estado en el
que DC-002.39c encontró el vendor (había corrido el candidato a las 01:04) y sobre el que midió el barrido
cortado.

**El vendor de HOY ya está completo** (alguien corrió la cadena entera: DC-002.43 rearmó el `.a` a las 02:54):
`gc_copy.c` `988e80fca09df879`, 50.834 B, con los interiores (DC-002.36), el candidato (DC-002.39b), las
marcas ARQ-MARCA (DC-002.29) y `>> 24` en los dos QACR; `libgodc.a` 210.002 B `54bd85eb3ce768bb`, más nuevo
que todos los `runtime/*.c` (control 0b verde). Y no es casualidad: es **el mismo sha256** que produce la
cadena ordenada.

**El arreglo:** un solo punto de entrada, `scripts/console/fix-libgodc-cadena.sh`, que corre la cadena entera
en el orden que importa y **con la Store Queue al final** (después del último script que resetea
`gc_copy.c`), desde cualquier estado de partida, más `--build` (el `.a` al día) y `--verificar` (el control
que se le puede correr a cualquier árbol sin modificarlo):

    desde el PRISTINO   130de736  ->  punto de entrada  ->  988e80fc   (50834 B)
    desde el SUCIO      fec6ef13  ->  punto de entrada  ->  988e80fc   (50834 B)
    --verificar: completo -> 0 · el del candidato -> 1 ("FALTA la Store Queue (>>24)") · mutado -> 1 · vendor -> 0

**El mutante** (`>> 24` → `>> 26`, una palabra, `mutar-libgodc-sq.py`) deja la suite en `162 de 21253`
(contra `3 de 3 vueltas con 21253` del arreglado): cazado. Y **el número con el VENDOR** (el criterio de
cerrar con el árbol bueno): `checar-consulta-233.sh 3 100` con `DC_ROOT=$HOME/dreamcast` da **3 de 3 vueltas
con los 21.253** (`patch_d2.mpq` 162/162, `d2data.mpq` 11082/11082, `d2char.mpq` 10009/10009).

**Lo que queda abierto (declarado):** la regla **no está forzada en el código** — los `fix-libgodc-*.py`
sueltos siguen pudiendo resetear el árbol y dejar el estado a medias; lo que hay es el punto de entrada y
`--verificar` (que sirve de control 0 de cualquier corpus). Y `checar-idempotencia-vendor.sh` sigue midiendo y
mutando el **vendor compartido**: no se corrió con otro agente midiendo. Evidencia:
`console/dc/evidencia/dc-002.39d-cadena.txt`.

## 62. DC-002.44: el motor **VE** los archivos del disco (`os.ReadDir("/cd/juego")`: **0 → 3**) — el que no listaba era el **puente** de `os`, y la medición que decía «el driver no sabe listar» era el **Joliet** de la ISO

El defecto: con los MPQ en el disco, el motor veía **cero** archivos en `/cd/juego` (`Validate` daba
`OK()= false faltan= 7`) mientras `Complete` (un `os.Stat` por nombre) contestaba `true` y los MPQ se abrían y
se leían enteros. Dos caminos sobre el mismo directorio, uno ve todo y el otro nada.

**Dónde se perdía (criterio 1), con la salida a la vista:** en el **puente**. `console/dc/std/os/os.go`
(`ReadDir`) tenía dos fuentes y ninguna era el disco — el `.manifiesto` del romdisk y el manifiesto del
overlay de la RAM — y para `/cd/juego` las dos fallan y el código devolvía **lista vacía con error nil** (el
`d2c_stat_es_dir(base) == 1` hacía que no devolviera `ErrNotExist`). De ahí el `archivos que ve= 0`.

**La corrección medida (1-BIS), que vale más que el arreglo:** la primera vuelta había concluido que el
driver `iso9660` de KOS **no** sabe listar (su `fs_readdir` devolvía solo `.` y `..`). Eso era **el Joliet de
la ISO**, no el driver: la misma sonda de C (`console/dc/poc/disco-c`), el mismo driver, sobre la ISO **sin**
el árbol Joliet devuelve **6 de 6** entradas de `/cd/JUEGO` (`d2char.mpq` 267.704.443, `d2data.mpq`
267.330.019, `nombres_*.txt`, `patch_d2.mpq` 5.022.588) con nombre, bytes y attr, y también por la ruta en
minúsculas. Con Joliet, en cambio, **ni los archivos abren** (`/cd/1ST_READ.BIN` y `/cd/JUEGO/PATCH_D2.MPQ`
dan ENOENT) y el `readdir` del root devuelve punto y punto-punto.

**El arreglo:** el disco se arma con su propio manifiesto, como el romdisk. `scripts/console/mkdisco.sh`
—que es el que **sabe** qué quedó dentro de la ISO— escribe `/JUEGO/MANIFIESTO.TXT` (nombre en minúsculas,
TAB, bytes, TAB, tipo; el nombre no puede ser `.manifiesto` porque el ISO9660 de nivel 3 sólo acepta A-Z, 0-9
y `_`) y `os.ReadDir` lo busca cuando no hay `.manifiesto` de punto. Va en **mayúsculas** dentro del ISO (el
driver compara sin distinguir mayúsculas) y **sin árbol Joliet por defecto** (`MKISO_JOLIET="${MKISO_JOLIET:-0}"`,
la misma decisión que ya tenían `mkdisco-235/236.sh`): con Joliet el disco no servía para leer el juego.

**El número del motor** (POC `motor-arranque`, `d2app.Create().Run()` de verdad, 45 s por corrida, con la
misma ISO y el mismo binario; el config del arranque es el instrumento de DC-002.34, `MpqPath= /cd/juego`,
`MpqLoadOrder= 3`):

| disco | la línea del motor |
|---|---|
| 3 MPQ en `/JUEGO` | `Complete= true  Validate.OK()= false  archivos que ve= 3  faltan= 4` |
| 3 MPQ + las 4 **marcas** de 0 B del núcleo que esta máquina no tiene | `Complete= true  Validate.OK()= true  archivos que ve= 7  faltan= 0` |
| `DC_SIN_JUEGO=1` (sin `/JUEGO`) | `Complete= false Validate.OK()= false archivos que ve= 0  faltan= 7` |

`ve= 3` es el número de la tarjeta (era **0** con el mismo instrumento) y el `faltan` bajó de 7 a 4: los 4
del núcleo que esta máquina no tiene. **`ve= 3` con `faltan= 0` no puede darse nunca**: `OK()` es
`len(MissingCore) == 0` sobre la tabla **fija** de los 7 `CoreMPQs` (`d2app/d2setup/discover.go:14` y `:44`),
así que con 3 MPQ faltan 4 y con los 7 nombres completos `ve` es 7. Los dos extremos quedaron medidos. Y con
el listado andando el arranque **llega más lejos**: `Adding source: '/cd/juego/patch_d2.mpq'` (+`d2data`,
`d2char`) y `[Asset Manager][INFO] Language: enUS (ENG), font dir LATIN`, muriendo después con `Illegal
instruction` (eso es DC-002.43).

**El defecto más fácil de leer mal de la tarjeta:** `build-motor-arranque.sh` **no compila** la stdlib, la
enlaza de la capa de `.gox`; con la capa de DC-002.34 (`/tmp/gox-233`) su `os.gox` (139.580 B, del 18 oct
12:15) **no tenía el arreglo** y el motor imprimía el defecto otra vez (`ve= 0`) con el `os.go` del árbol ya
arreglado. Con la capa rearmada desde HEAD (75 + 142, `142/142`) el `os.gox` es 140.824 B y la misma corrida
da `ve= 3`. Queda como control del corpus.

**El overlay no se rompió** (criterio 4): `/rd/prueba-seleccion` sigue listando en las tres corridas
(`ReadDir de /rd/prueba-seleccion -> 1 entradas: uno.txt`). **El corpus**
`scripts/console/checar-lista-disco-244.sh` da **16 controles OK, 0 fallos** (sha256 de los 3 MPQ contra el
PC, las 4 marcas declaradas, el control de la capa, el manifiesto horneado igual al del build, los 4
criterios). Evidencia: `console/dc/evidencia/dc-002.44-lista-del-disco.txt`.

**Lo que queda declarado:** el arreglo de **fondo** es que `os.ReadDir` de `/cd` hable con el `fs_readdir`
del driver —que medido funciona—, y eso va como tarjeta aparte: el manifiesto es del **horneador** (lista los
`*.mpq` y `*.txt` que ese script hornea), no del sistema de archivos de la consola.


---

## 63. DC-002.39e: el cache de itabs guarda punteros del monton de Go en `.bss` de C y el recolector **no barre `.bss`** — por eso el `ReadFile("(listfile)")` del tercer MPQ saltaba a un pedazo de DATO, y la consulta queda en **21.253** con la lectura puesta

`get_itab` (`runtime/interface_dreamcast.c`) arma los `Itab *` de las conversiones **entre interfaces** en el
montón de Go y los guarda en `static struct { Itab *itabs[32]; int count; } itab_cache`, un array en **`.bss`
de C**. El recolector de la consola es copiativo y sus raíces son **solo** `gc_root_table`, las listas de
gccgo y las pilas (`gc_scan_roots` de `gc_copy.c`, con el comentario explícito *"not conservative .data/.bss
scan"*). Un puntero guardado en `.bss` es **invisible**: cuando el recolector **copia** el itab al otro
semiespacio la ranura sigue apuntando al viejo, y en la recolección siguiente ese puntero se reubica como
**interior** (DC-002.36) — la tabla de métodos termina siendo la de **otro** objeto y la llamada al método
salta a un pedazo de **dato**. Eso es exactamente `PC 656a626f` (`"obje"` leído como dirección) con el `PR` en
`bytes.Buffer.ReadFrom`.

### el defecto, como tal (POC nuevo `console/dc/poc/itab-raiz`)

El POC arma la **misma figura** que el motor: `porCache(entra saludador) otro` es una conversión entre **dos**
interfaces (no se puede resolver en compilación: va al runtime y al cache), `porEstatico(u *uno) otro` es el
**control** (conversión concreto→interfaz, itab **estático** en `.rodata`). Las interfaces que salen del cache
quedan **vivas** en un slice a través de las recolecciones, y se vuelven a llamar después.

Primero se mide que el camino se **ejerce** (si no, la hipótesis no vale nada): `itabLlamadas= 40`,
`itabArmados= 10`, `itabEnSemi= true` — o sea el itab cae **adentro** del semiespacio activo, el recolector lo
**puede** mover. Con la llamada **inmediata** (sin recolección en el medio) todo da bien: **120/120**.

Con la figura V2 (viva a través de la recolección), medido con `bash scripts/console/checar-itab-39e.sh`:

| runtime | `libgodc.a` | arranques | TOTAL | resultado |
|---|---|---|---|---|
| **mutante** (`fix-libgodc-itab.py --sin-raices`) | 210728 B `d89f2752e07f36a5` | **70** | **0** | reinicio mudo en la **vuelta 4** |
| **arreglo** (paso 4 de `fix-libgodc-cadena.sh`) | 210932 B `58e6567d424051a7` | **1** | **1** | 40 vueltas, `v2mal= 0` |

El mutante imprime `ITAB-RAIZ armados a= 1007 b= 2009 gc= 0` y **nada más**: 70 arranques × 3 líneas de
progreso (`v=1..3`) = 210 líneas y **cero** TOTAL. Muere en la vuelta 4, después de la primera recolección
forzada: las 3 primeras van bien. Ese es el dato que separa *"el cache devuelve una tabla rota"* (sería malo
desde la vuelta 1) de *"el cache devuelve una tabla que se rompe **después**, cuando el recolector copia el
itab"*.

Con el arreglo, los cuatro números que importan:

* `v2mal= 0` con `v2bien= 820` — 820 llamadas por el camino del cache **después** de las recolecciones, todas
  correctas (`gc= 32` recolecciones, ninguna línea `ITAB-RAIZ-MALO-*`);
* `itabArmados= 1` contra **10** del defecto, con las **mismas** 120 llamadas: con las ranuras registradas el
  cache **acierta**; sin ellas **falla** después de cada recolección y vuelve a reservar;
* `v4bien= 820 v4mal= 0` — el **control** con itab estático sobrevive igual con y sin arreglo, así que lo que
  rompe **no** es "una interfaz viva en un slice" (el patrón del motor en `ReadFrom`), es **el itab del cache**.

### el arreglo (paso 4 de la cadena)

`scripts/console/fix-libgodc-itab.py`: registra las **32 ranuras** del cache con `gc_add_root` una sola vez, al
guardar el primer itab. Con eso el recolector las barre como una variable de Go más y las **actualiza** al
copiar el itab. Se registran las 32 de una vez (no la que se acaba de guardar) porque `gc_add_root` no se puede
llamar por cada itab: la tabla de raíces del runtime tiene tope **256** y el cache son 32 ranuras. El paso va
**último** en `fix-libgodc-cadena.sh` por la misma razón que la Store Queue (DC-002.39d): el script resetea
`interface_dreamcast.c` y después reaplica el ABI, así que deja su archivo en el estado final sin llevarse nada
de los pasos anteriores (que tocan `gc_copy.c`). La cadena lo **verifica** (`--verificar` exige el marcador
`d2c_itab_registrar_raices`).

### el corpus del motor: los 21.253 **con** la lectura puesta

POC nuevo `console/dc/poc/tablas-mpq-listfile` (el mismo `tablas-mpq` de DC-002.33 con `leerListfile = true`,
que es lo que estaba apagado), sobre un **árbol limpio de HEAD** (`719d7b82`) para que no se mezcle el trabajo
en curso de otra tarjeta. `bash scripts/console/checar-consulta-233.sh 1 75` con el gox del motor (140
paquetes), un solo cambio entre las dos corridas:

```
ARREGLO   21253 de 21253   ( patch_d2.mpq:162/162  d2data.mpq:11082/11082  d2char.mpq:10009/10009)
MUTANTE   11244 de 21253   ( patch_d2.mpq:162/162  d2data.mpq:11082/11082  d2char.mpq:/10009)
```

`162 + 11082 + 10009 = 21253`. El que se cae es el **tercer** MPQ (`d2char.mpq`), exactamente como decía la
tarjeta: el mutante ni siquiera llega a contar los nombres de ese MPQ. Y con el arreglo se cumple el criterio 4
de la tarjeta (`checar-consulta-233.sh 3 120`, mismo árbol y mismo gox): **3 de 3 vueltas con los 21253**
(`totales: 21253 21253 21253`) — no es un cierre por suerte de una vuelta.

### cómo se repite

```
source scripts/console/env.sh
bash scripts/console/checar-itab-39e.sh --preparar           # arma los dos árboles y mide los dos estados
POC=tablas-mpq-listfile DC_ROOT=<copia con el arreglo> bash scripts/console/checar-consulta-233.sh 3 120
```

Evidencia: `console/dc/evidencia/dc-002.39e-itab-raiz.txt`.

## 65. DC-002.45: el listado de `/cd/juego` deja de ser el del **horneador** — `os.ReadDir` le pregunta al `fs_readdir` del driver y un archivo que el `MANIFIESTO.TXT` no conoce **sale igual** (78 bytes medidos)

DC-002.44 cerró el síntoma con el `MANIFIESTO.TXT` que hornea `mkdisco.sh`: el motor pasó de ver **0** a ver
**3** archivos. Pero ese listado es del **horneador**, no del sistema de archivos — la lista la escribe el
script que arma el disco, así que un archivo que esté en la ISO y el script no conozca **no puede salir
nunca**. Esta tarjeta lleva el listado al camino de fondo: `fs_open(ruta, O_RDONLY|O_DIR)` + `fs_readdir` en un
bucle, que es lo que el driver del ISO9660 ya sabía hacer desde DC-002.44 (sección 1-BIS de su evidencia) y
que el puente de `os` nunca llamaba.

**El instrumento es un archivo que el horneador no conoce.** `mkdisco.sh` acepta ahora
`DC_MARCA_SUELTA=<archivo>`: lo hornea como `/JUEGO/MARCA_SUELTA.DAT` y lo deja **fuera** del manifiesto a
propósito. Son **78 bytes** con sha256 `67f895f06df68181…`, y el corpus exige que el listado los traiga. Sin
ese archivo el defecto no se puede ver: los 3 MPQ están en las dos listas y los dos caminos darían lo mismo.

**El orden de las fuentes es la decisión.** `os.ReadDir` prueba **(1) el driver primero** y deja el manifiesto
de **respaldo** (el del romdisk, `/rd/.manifiesto`, y el del horneador, `MANIFIESTO.TXT`); el overlay de la
RAM sigue encima, tapando por nombre. Al revés —manifiesto primero— el defecto quedaría **tapado**: un disco
con manifiesto viejo o incompleto daría la lista del horneador y al driver no se le preguntaría nunca. El
manifiesto queda para lo que el driver **no** puede listar: `/rd`, donde el FS ROM de KOS no deja abrir un
directorio (medido en DC-002.19).

Los números, medidos en Flycast (`~/.hermes/cache/scratch/c245/`):

```
POC lista-disco, disco con los 3 MPQ + la marca suelta
  /cd/juego entradas= 4
   [0] d2char.mpq       dir= false bytes= 267704443
   [1] d2data.mpq       dir= false bytes= 267330019
   [2] marca_suelta.dat dir= false bytes= 78        <- el archivo que el horneador NO conoce
   [3] patch_d2.mpq     dir= false bytes= 5022588
  /cd entradas= 2   -> [1] juego dir= true bytes= 0  <- el subdirectorio sale como DIRECTORIO
  y por os.Stat los mismos bytes: 267704443 / 267330019 / 5022588  (el driver no inventa tamaños)

el mismo disco, con el puente forzado al manifiesto (mutante 1)
  /cd/juego entradas= 3                              <- la marca suelta desaparece: el defecto, en rojo

motor entero (45 s), disco con marca suelta
  seleccion: Complete= true Validate.OK()= false archivos que ve= 3 faltan= 4   <- el número de DC-002.44
  seleccion overlay: ReadDir de /rd/prueba-seleccion -> 1 entradas: uno.txt      <- lo que ya andaba
motor entero (45 s), disco SIN /JUEGO
  seleccion: Complete= false Validate.OK()= false archivos que ve= 0 faltan= 7  <- el mutante, igual que antes
```

`ve= 3` no se mueve porque cuenta los nombres de la tabla `LoadOrder`, no las entradas del directorio: lo que
cambia con esta tarjeta es **qué archivos puede ver** el listado, no cuántos de la tabla están.

**La memoria, medida y no prometida.** El listado se arma en un buffer **LOCAL** de 1024 bytes (`var bufISO`),
como el del overlay: en el arranque del motor el montón de C tiene **2.496 bytes** libres y un listado no le
puede pedir memoria. 40 vueltas del listado dejan el montón **igual** (`cfree= 2232` y `cused= 1208572` antes y
después). El semiespacio del GC **sí** sube (14.160 → 236.880, ~5.568 por vuelta): eso es el **resultado** — el
`[]DirEntry` con sus cadenas — y el camino del manifiesto paga exactamente lo mismo.

**Lo que salió mal en el camino y quedó cazado:** en la primera corrida el listado daba 4 entradas, pero la
cuarta era `manifiesto.txt` — el **índice** del directorio saliendo como si fuera un archivo del juego. El
driver del ISO9660 pasa los nombres a **minúsculas** (`fs_iso9660.c:fn_postprocess`), así que comparar contra
`"MANIFIESTO.TXT"` no matcheaba; se compara sin distinguir mayúsculas y el mutante 3 exige que, si vuelve a
pasar, el corpus se ponga rojo.

```bash
bash scripts/console/checar-lista-disco-245.sh    # 20 controles OK, 0 fallos
bash scripts/console/mutar-lista-disco-245.sh     # 4 de 4 mutantes cazados
```

Los 4 mutantes, y qué caza cada uno: **1** el puente no consulta al driver (contesta `-4` y cae al
manifiesto) → el listado baja a 3 y la marca suelta desaparece — *es el defecto de la tarjeta*; **2** el driver
no salta `.` y `..` → 6 entradas; **3** el driver lista el `MANIFIESTO.TXT` → el índice como archivo; **4** el
driver no trae `de->size` → los 3 MPQ con `bytes= 0`. Los mutantes corren sobre una **copia** de la capa de
`.gox` y el script verifica con sha256 que `os.go` y `os_dc.c` quedan iguales al terminar.

La barrera del motor se rearmó con la capa nueva (para no comparar contra un ELF viejo):
`compare-motor-dc6.sh` → **31 líneas idénticas, 30 frames**, host x86-64 y SH-4.

Lo que esta tarjeta **no** arregla, declarado: `/rd` sigue listándose por su `.manifiesto` (el FS ROM de KOS no
deja abrir un directorio), el `Validate.OK()= false` del motor es el estado de DC-002.44 y depende de que el
disco traiga el juego completo, y el listado tiene tope de 64 entradas por directorio (una lista truncada se
avisa, no se inventa).

Evidencia: `console/dc/evidencia/dc-002.45-lista-del-disco-por-el-driver.txt`.

## 66. DC-002.46: el corpus de DC-002.40 **vuelve a 16/16** — los cuatro rojos (M6/M7/M8/M10) eran el **runtime vendorizado**, no el corpus, y ahora el corpus trae los tres controles que lo dicen

La tarjeta reportaba `bash scripts/console/checar-compr-240.sh <capa limpia de HEAD>` en **9/13** con los
cuatro controles del MOTOR en rojo, y el log imprimiendo M1..M5 y **rearmándose** (M6/M7/M8/M10 nunca salen).
Ya estaba descartado el instrumento, la capa y el `OBJECTLOOKUPS.PACK`. Lo que quedaba por mirar era lo que el
corpus **no** medía: el runtime vendorizado, que es un `.a` que se arma **aparte** (`make -C ~/dreamcast/libgodc`)
y no viene del repo.

Los 9/13 se midieron con un `libgodc.a` de las **03:09**. Entre esa corrida y esta el runtime cambió dos veces:
`runtime/interface_dreamcast.c` (05:50, el arreglo de DC-002.39e: el itab sale del montón **movible** y se pide
al montón de C con `gc_external_alloc`) y `runtime/gc_copy.c` + `libgodc.a` rearmado (06:03). Con el `.a` de
**hoy**, la capa del árbol de HEAD limpio (`/tmp/gox-246`, 205 `.gox`) y el mismo disco, el corpus da:

```
control 0  (el host, patch_d2):          VERDE      control 7  (M6 el MOTOR, 162 nombres):   VERDE
control 0b (libgodc.a no esta viejo):    VERDE      control 8  (M7 el control sano DC6):     VERDE
control 0c (el itab FUERA del monton):   VERDE      control 9  (M8 200/200 sin fallos):      VERDE
control 0d (la capa es la de este arbol):VERDE      control 10 (M10 aislado con buffers C):  VERDE
control 1  (el host, los otros dos):     VERDE      control 11 (M6 d2data Y d2char al host): VERDE
control 2  (M1 los 662 bytes):           VERDE
control 3  (M2 el defecto declarado):    VERDE      mutante 'avail-in-cero'  -> cae el 5   -> CAZADO
control 4  (M3 el puente streaming):     VERDE      mutante 'pos-de-un-tiro' -> cae el 5b  -> CAZADO
control 5  (M4 compress/zlib ARREGLADO): VERDE      vuelta sano                          -> 16/16
control 5b (M4b el paquete de a 512):    VERDE                                  == 2 verdes, 0 fallos ==
control 6  (M5 el crudo del disco):      VERDE
```

Los números del motor, los tres MPQ por el camino del motor (`d2mpq.FromFile` + `ReadFile`): patch_d2
**5502 / 162 nombres / crc32 502100774**, d2data **466867 / 11082 / 923032874**, d2char **418279 / 10009 /
1367999602** — los tres del host. `M8` 200 vueltas con `fallos= 0` y `M10` 200/200.

**La prueba en contra** (para que el verde no sea de palabra): `scripts/console/mutar-runtime-itab-246.sh`
copia el árbol de Dreamcast a un `DC_ROOT` propio y cambia **una línea** del runtime vendorizado —
`gc_external_alloc(total_size)` por `gc_alloc(total_size, NULL)`, o sea el itab de vuelta en el montón
**movible** — y rearma el `.a` **en ese árbol** (sin rearmarlo el corpus enlaza el runtime sano y el mutante
es invisible). Con eso, mismo árbol de fuentes, misma capa, mismo disco: **9/16**, con `0c`, `7`, `8`, `9`,
`10` y `11` en ROJO y el log rearmándose 33 veces sin llegar a M6. Es el síntoma exacto que reportaba la
tarjeta, y ahora tiene nombre y apellido en la primera línea de la salida.

Lo que se mudó en el corpus (13 → **16** controles):

* **0b (nuevo)** — `libgodc.a` no está viejo: ninguna fuente del runtime es más nueva que el `.a`. Ya existía
  en el corpus de DC-002.34; acá faltaba, y es justo la herramienta con la que se estaba midiendo.
* **0c (nuevo)** — el `get_itab` pide el itab al montón de C. Es un control de **fuente**: dice el nombre del
  defecto (DC-002.39e) en vez de dejar que el síntoma se lea como "el motor no avanza".
* **0d (nuevo)** — la capa de `.gox` es la de **este** árbol: cada paquete de `orden-motor.txt` tiene que
  estar y ningún `.go` suyo puede ser más nuevo que su `.gox`. El corpus tomaba por **defecto** la capa
  `/tmp/gox-240` (la de la época de DC-002.40) y nada avisaba: medir el motor de hoy con la capa de ayer da
  números que no son de hoy. Medido: `/tmp/gox-246` contra HEAD → 0 paquetes con problema; `/tmp/gox-240`
  contra HEAD → 6 (d2mpq, d2records, d2loader, d2asset, d2app y uno más).
* **9 (mudado)** — M8 pedía `fallos>0` con `rc_zlib= -3` (el defecto del montón de Go de DC-002.40, arreglado
  en DC-002.41). Ahora pide `fallos= 0`: es el defecto que **ya no queda**, y si vuelve, se pone ROJO.
* **11 (mudado)** — pedía lo contrario de lo que pasa: "d2data/d2char NO llegan a los números del host" era el
  bloqueo declarado. Desde DC-002.39e **sí** llegan, así que ahora el control pide que los dos lleguen.

Ojo con lo que esta tarjeta **no** arregló: el defecto del itab ya estaba arreglado en el vendor (05:50). Lo
que se hizo fue medirlo, nombrarlo y dejar el corpus con los controles que lo cazan.

Evidencia: `console/dc/evidencia/dc-002.46-corpus-16.txt`.

## 67. DC-002.45: la tabla se **consume por trozos** (34 pedidos de 4.096 B por `LvlPrest.txt`) — la lectura deja de ser la pared, y **el que llena el semiespacio es el DATO VIVO: 514.832 de 524.288 (98,1 %)**

DC-002.43 dejó medido que el arranque moría **adentro** de la lectura completa (`ioutil.ReadAll` de
`LoadFile`) de `data/global/excel/LvlPrest.txt`: **134.937 B** de archivo contra **132.104 B** libres en el
semiespacio del GC al entrar (`gsemi=392184/524288`). Lo que se hizo acá es que la tabla **no se materialice**:
el `csv.Reader` la consume del stream del asset (el MPQ ya entrega por **sector de 4.096**), y las filas que
quedan vivas son las parseadas, no el archivo.

**Lo que cambió** (`d2core/d2asset/asset_manager.go`, `d2common/d2fileformats/d2txt/data_dictionary.go`):

```
LoadDataDictionary:  antes  LoadFile(path)  (ReadAll del asset entero) + LoadDataDictionary(bytes)
                     ahora  LoadAsset(path) (el stream tal cual)      + LoadDataDictionaryReader(stream)
LoadRecords:         las marcas lee-entra/lee-volvio envuelven el CONSUMO de la tabla, que es donde se lee
```

Las dos versiones (arreglo y mutante) salen del **mismo texto reversible**:
`scripts/console/fix-tablas-trozos-245.py --sano | --pristino | --estado`.

**Host, con la tabla REAL** (`console/dc/std/difftest/dc00245_test.go`, 134.937 B sacados del MPQ por el
`d2mpq` del árbol): **1.093 filas**, el lector recibe **34 pedidos**, el mayor de **4.096 B**, y las 1.093
filas son idénticas a las de `encoding/csv` de Go (campos, largos y final). La tabla entera y por trozos dan
lo mismo: el arreglo no cambia lo que sale.

**Consola (Flycast, 50 s, capa del árbol de hoy)** — la marca de la lectura del arreglo **crece 3.752 B**
(372.568 → 376.320) con un archivo de 134.937 B, y el motor **sigue**:

```
ARQ-CARGA lee-entra /data/global/excel/LvlPrest.txt  gsemi=372568/524288 gc=7
ARQ-CARGA lee-volvio /data/global/excel/LvlPrest.txt gsemi=376320/524288 gc=1121
ARQ-CARGA entra     /data/global/excel/LvlWarp.txt   gsemi=376584/524288 gc=1121
ARQ-CARGA lee-volvio /data/global/excel/LvlWarp.txt  gsemi=371992/524288 gc=1122
```

15 marcas de carga, 3 assets del MPQ, ninguna muerte en la ventana. `scripts/console/checar-trozos-245.sh 50`
→ **12 controles OK, 0 con fallo**. (Con 25 s el `lee-volvio` **no alcanza a salir**: la tabla por trozos hace
~1.114 recolecciones, ver abajo. Medir con 50 s.)

**El número que decide la tarjeta siguiente** (marca `gc-vivo` de `instrumentar-vivo-245.py`, lo que sobrevive
a la copia): **514.832 B de 524.288 = 98,1 %**, quedan **9.456 B**. Ya no es el archivo el que llena el
semiespacio: son las filas parseadas que el `RecordManager` se queda. Con el dato vivo al 98 %, cada
recolección copia ~500 KB y el arranque se arrastra: **1.114 recolecciones** entre `lee-entra` y `lee-volvio`
de una sola tabla.

**La otra salida de la tarjeta quedó cerrada por medición**: "subir el semiespacio" no es gratis ni posible
hoy — del otro lado está el **montón de C con 42.120 B libres de 4.374.528** (99 % usado). El mutante muere
justo ahí, ya lejos de la lectura:

```
fatal error: gc_external_alloc: OOM
Memory: arena=4374528 used=4332408 free=42120
```

**El criterio 3 de la tarjeta cambió de forma, y lo cambia un número**: el mutante con el `ReadAll` puesto
**entra** a la lectura (175.912 B libres para 134.937 B: le sobran **40.975 B**) y **no** muere adentro —
avanza **más** que el arreglo en la misma ventana (5 tablas contra 3, 24 marcas contra 15). Un control que se
pone rojo porque el defecto ya no alcanza a matar no habla del arreglo, así que el corpus mide la **propiedad**
(la marca de la lectura no crece en el tamaño del archivo) y **informa el margen** del mutante con números.

**El corpus de DC-002.34 no se mueve** (`GEN_HEAD=<capa de hoy> bash scripts/console/checar-motor-disco-234.sh 25`
→ **9/9**): los cinco estados dan lo mismo que en DC-002.43 — `S233`/`S232` siguen con **10 marcas**, **2
assets** y la última marca `lee-entra .../LvlPrest.txt` (con 25 s la tabla por trozos todavía se está
consumiendo; con 50 s termina y pasa a `LvlWarp.txt`).

**Controles nuevos del corpus** (`checar-trozos-245.sh`, 12 en verde): `0e` el runtime pide los itabs al
montón de C (sin eso el camino por trozos salta a basura dentro de un `gc_collect`, PC `6c000000`);
`0f` `orden-motor.txt` sin repetidos (medido: **171 líneas, 29 repetidas, 142 únicas** → el linker muere con
`multiple definition of ...`); `2`/`2b` la capa del mutante **no** es el mismo binario (`d2asset.gox`
1.099.948 B contra 1.101.628 B) y la copia estaba en `pristino` al compilarla; `7` la marca de la lectura del
arreglo no crece en el tamaño del archivo. El control `2` anterior miraba `LoadDataDictionaryReader` en
`d2txt.gox`: esa función existe en **los dos** árboles (el que se muta es el **llamador**) y daba `4 y 4`, que
se lee como "el mutante no se recompiló" cuando sí.

**Lo que esta tarjeta NO deja arreglado**: el arranque no avanza más que antes (3 tablas contra 5 del mutante
en 50 s) porque el dato vivo está al 98 % del semiespacio; el montón de C está al 99 %; y la cadena del
runtime ahora incluye el paso 5 (`fix-libgodc-itab-245.py`, el itab fuera del montón movible) para que un
runtime rearmado desde cero no pierda el arreglo.

Evidencia: `console/dc/evidencia/dc-002.45-lectura-por-trozos.txt`.

## 68. DC-002.47: el arranque **no** se quedaba en 3 tablas por memoria — `d2std/bufio` tiraba el último trozo: cuando el stream del MPQ devuelve **bytes y `io.EOF` en la misma llamada**, una tabla que entra en un solo pedido se leía **VACÍA** (`LvlWarp.txt`, 3.935 B contra 4.096); arreglado el lector el arranque **pide la 6ª tabla**, y el `.eh_frame` **fuera del mapa** corre el fin del `.bss` **1.076.448 B** para abajo

DC-002.45 dejó el corpus en verde y el diagnóstico escrito: «el que llena el semiespacio es el dato vivo
(98,1 %)». Y el arranque seguía pidiendo **3 tablas**. Lo que faltaba medir era **dónde** se quedaba el cuarto
pedido, y la respuesta no estaba en la memoria: estaba en **dos líneas de `ReadBytes`**.

**El defecto** (`console/dc/std/bufio/bufio.go`). `ReadBytes` acumulaba los trozos y, cuando `rellenar`
devolvía error, hacía `return salida, err` — **tirando los bytes que `rellenar` acababa de dejar en el buffer**.
El stream del MPQ entrega los bytes del último pedido **junto con el fin** (todo lo que entra en un pedido
viene con `io.EOF` pegado), así que el caso no es exótico: le pasa a **cualquier** tabla que entre en un solo
pedido y a cualquier tabla en su último trozo.

```
LvlWarp.txt  3.935 B  <  buffer 4.096 B  ->  0 filas  ->  el arranque no pide la tabla siguiente
```

El arreglo es la guarda que usa el `ReadSlice` de Go: el fin se entrega cuando **el buffer quedó vacío**
(`if b.Buffered() == 0 && b.err != nil { return salida, b.err }`) y `rellenar` sólo se llama si no hay error
pendiente. Reversible: `python3 scripts/console/mutar-bufio-247.py --sano | --mutante | --estado`.

**El instrumento tenía el mismo tipo de defecto, y por eso daba verde.** `d2std/io.EOF` es un valor **propio**
(`errors.New("EOF")`), distinto del `io.EOF` del host, y el csv de la consola compara por **identidad**. Las
pruebas de DC-002.45 le inyectaban a la consola el EOF **de Go**: ninguna rama de fin se disparaba (medido con
una traza temporal: `readLine bytes=5 err=SI` con el EOF de Go, `err=no` con el de la consola) y la prueba
medía otra cosa. Ahora cada lado lee con **su** EOF (`fin error` en los lectores de prueba) y los finales se
comparan por el **texto** del error (`esEOF`). Las dos pruebas de DC-002.45 siguen verdes con el instrumento
arreglado:

```
filas=1093  pedidos=34  pedido mayor=4096 B  (la tabla son 134937 B)
filas iguales por los dos caminos: 1093
```

**El mutante lo caza la difftest, no el relato** (`go test ./difftest/ -run DC00247`, el mutante es el lector
de antes):

```
arreglo  3.935 B entra entero     Go=270 registros  d2std=270 registros
arreglo  11 KB, no entra entero   Go=780 registros  d2std=780 registros
arreglo  3.935 B sin salto final  Go=270 registros  d2std=270 registros
arreglo  una fila sin salto final Go=1   registro   d2std=1   registro
MUTANTE 3.935 B entra entero      Go=270 registros  d2std=0   registros      <- FAIL
MUTANTE 11 KB, no entra entero    Go=780 registros  d2std=781 registros      <- FAIL
```

**Lo que cambió en el arranque** (Flycast, 50 s por corrida, mismas capas, lo único que cambia es el lector;
`bash scripts/console/checar-techo-247.sh 50` → **11 controles OK, 0 con fallo**):

```
                       assets del MPQ pedidos   tablas LEÍDAS                     dato vivo máx        gc
lector VIEJO (mutante)         3                LvlTypes, LvlPrest, LvlWarp      406.992 (38,8 %)      3
lector ARREGLADO (sano)        6                + Objects, + objtype           1.037.096 (98,9 %)   911
                                                 (y pide la 7ª: weapons.txt)
```

El mutante del lector — el MISMO binario con el `ReadBytes` de antes y la MISMA partición — vuelve a quedarse
en las 3 tablas de DC-002.45, con el semiespacio al **38,8 %** y **3 recolecciones** en toda la ventana: la
pared de las 3 tablas no era memoria, era el último trozo que se tiraba.

**La partición, medida con las dos particiones y el MISMO lector** (`checar-techo-247.sh`, el control 8
compara **al mismo tramo**):

```
partición                          assets   al terminar LvlTypes.txt (mismo tramo)   gc_collect
1024 KB por lado, .eh_frame fuera     6     408.312 de 1.048.576  (38,9 %)              911
 512 KB por lado, .eh_frame adentro   2     389.560 de   524.288  (74,3 %)              726
```

Al MISMO trabajo hecho (LvlTypes.txt terminada) la partición nueva usa **38,9 %** del semiespacio contra
**74,3 %** de la vieja: eso es lo que corre la pared. En el máximo del recorrido los dos números quedan
parecidos (**98,9 %** contra **97,9 %**) porque **el motor llena el semiespacio que le den** — el sano llega a
6 assets y la vieja se queda en 2. Comparar máximos fue el primer control de esta tarjeta y daba ROJO con el
arranque yendo tres veces más lejos; el control que sirve compara el mismo tramo.

**Los cuatro binarios de la exploración** (mismo árbol, 50 s cada uno; se corrieron para ELEGIR la partición,
y de ahí salió que el `.eh_frame` fuera del mapa se queda):

```
fin del .bss   1ST_READ.BIN   semiespacio  lector   assets       dato vivo máx        gc_collect  final
0x8CBBBF5C     12.011.848 B   512 KB       arreglo  5            515.344 / 524.288    1.258       pide Objects
                                                                  (98,3 %: 8.944 B)               (en curso)
0x8CBBBF5C     12.011.848 B   1024 KB      arreglo  6            981.592 / 1.048.576  1.087       pide weapons
                                                                  (93,6 %)                        (en curso)
0x8CAB527C     10.935.400 B   1024 KB      arreglo  11 (2 vidas) 828.808 / 1.048.576    517       panic: index
                                                                  (79,0 %)                        out of range
0x8CAB527C     10.935.400 B   1024 KB      VIEJO    3            380.104 / 1.048.576      3       (se queda en 3)
                                                                  (36,2 %)
```

El panic de la tercera fila (517 recolecciones) **no se reprodujo** en las corridas del corpus, que pasaron de
911 y 929 recolecciones sin morir: queda como una observación a vigilar, no como una pared medida.

**Las dos palancas.**

* **El semiespacio** lo fija el `godc_config.h` del runtime **vendorizado** (`GC_SEMISPACE_SIZE_KB`, hoy
  **1024 KB por lado**): `python3 scripts/console/fix-libgodc-semiespacio-245.py --kb 1024` (y `--estado` para
  leerlo — el corpus lo lee de ahí, **no está escrito a mano** en ninguna parte). El script **rehace
  `libgodc.a`**: el Makefile del vendor es `%.o: %.c` **sin** dependencias de cabeceras y tocar el header no
  recompila nada (DC-002.45). Nunca sube a ciegas: si falta `sh-elf-gcc` en el `PATH` **aborta antes** de
  borrar los `.o`, porque el modo de fallar era quedarse sin biblioteca y con un `make` roto.
* **El `.eh_frame` fuera del mapa** (`scripts/console/gen-ldscript-eh-247.py`, `DC_EH_FINAL=1`): saca las dos
  reglas del script de enlace de KOS y las manda al `/DISCARD/` — **sin tocar el KOS vendorizado** (el script
  se lee de `$KOS_BASE` y el nuevo se deja en el directorio de build). El `.eh_frame` pasa de **1.076.440 B** a
  **4 B** y todo lo que venía detrás se corre para abajo: el fin del `.bss` baja **1.076.448 B**
  (2.360.887.648 → 2.359.811.200) y la imagen plana baja lo mismo (**12.011.848 → 10.935.400**). El montón de
  C **empieza en el fin del `.bss`**, así que gana exactamente ese techo.
* Los dos semiespacios salen del **montón de C** (`memalign(32, 1 MB)` en `gc_heap.c`), de modo que **subirlos
  cuesta techo del montón**: por eso las dos palancas son compañeras y no alternativas.

**El techo del montón de C, medido por el lado del que se choca**: en el corpus de DC-002.45 el mutante del
`ReadAll` muere con `Out of memory. Requested sbrk_base 0x8d00a000, was 0x8cfe1000, diff 167936` seguido de
`fatal error: gc_external_alloc: OOM` (arena=5.422.432 used=5.380.984 **free=41.448**): el que no entra es el
**montón**, no el semiespacio. Los `gc_external_alloc` (los itabs que desde DC-002.45 no viven en el montón
movible) son parte de eso.

**Los corpus** (`scripts/console/checar-trozos-245.sh 50` → **12 controles OK, 0 con fallo**, exit 0;
`checar-techo-247.sh 50` → **11 controles OK, 0 con fallo**; `checar-motor-disco-234.sh 25` → **9/9**). El
corpus de DC-002.45 ahora **recompila del snapshot del árbol** los paquetes de la stdlib que esta tarjeta tocó
(`STD_PACKS=bufio`): la capa de partida traía los `.gox` de la stdlib de otro día y la corrida medía el árbol
de ayer **sin que nada lo dijera** — el mismo modo de fallar que el corpus ya castiga del lado del motor.

Los números del sano en el corpus de DC-002.45, con la partición nueva: **6 assets** del MPQ, **5 tablas
leídas** (la última pedida es `weapons.txt`), dato vivo máximo **1.037.096 B de 1.048.576 (98,9 %)** y **929**
recolecciones; el mutante del `ReadAll` completa 4 lecturas y 5 assets con el dato vivo en 566.088 B (53,9 %).
Tres cosas del instrumento que el corpus ahora exige, porque las tres se midieron rotas en esta tarjeta:

* `checar-trozos-245.sh 50` **son 50 segundos**: el parseo viejo dejaba 25 (`SEGS="${2:-25}"` y los segundos en
  `$1`) y las corridas se cortaban a los 25 s sin que nada lo dijera (los `win-025s.png` de todas las corridas).
* el control 0e exige **la marca `gc-vivo` en `gc_copy.c`**: la cadena del runtime resetea el archivo y se
  lleva el instrumento, con lo que la corrida sigue pero los números no existen (medido: el sano corrió sin
  una sola marca y el corpus solo podía dejar una nota).
* el corpus **fija la partición que mide** (`SEMI_KB=1024`): el runtime vendorizado es uno solo y lo comparten
  todas las tarjetas; entre dos corridas otra tarjeta lo dejó en el semiespacio viejo y los controles de la
  lectura se pusieron rojos **sin que el arreglo hubiera cambiado** (marcas `gsemi=373472/524288` contra un
  techo de 1.048.576 leído al empezar).

**Lo que esta tarjeta NO cierra, con el número**: el arranque **no termina** — llega a la 6ª tabla
(`weapons.txt`) y ahí se cortan los 50 s — y con la partición de 1 MB el dato vivo vuelve a tocar el techo
(**1.037.096 de 1.048.576 = 98,9 %**): el motor llena el semiespacio que le den, así que la tarjeta siguiente
es la misma pregunta un piso más arriba (bajar el dato vivo o mover la partición otra vez, ahora con el montón
de C medido). El `panic: index out of range` que apareció en una corrida de la exploración (517 recolecciones,
dato vivo 787.176 B, 261.400 B libres) **no se reprodujo** en las corridas del corpus (911 y 929 recolecciones,
sin muerte) y queda como observación a vigilar, no como pared medida.

**Cómo se repite**:

```
bash scripts/console/checar-trozos-245.sh 50        # 12 controles (el corpus de DC-002.45, actualizado)
bash scripts/console/checar-techo-247.sh 50         # los de esta tarjeta: la partición, el lector y el mutante
bash scripts/console/checar-motor-disco-234.sh 25   # 9/9 (el corpus del motor)
python3 scripts/console/mutar-bufio-247.py --estado # sano | mutante del lector
```

Evidencia: `console/dc/evidencia/dc-002.47-techo-del-arranque.txt`.

### DC-002.47, cierre medido (2026-10-19 09:4x): el corpus queda **11/11** y el **control 8 estaba mal**

El defecto de raíz ya está arriba; lo que faltaba era la corrida con el instrumento sano. `bash
scripts/console/checar-techo-247.sh 50` → **11 controles OK, 0 con fallo (exit 0)**.

**El control 8 mezclaba tramos y por eso daba rojo con la partición buena.** Comparaba el MÁXIMO del dato vivo
del recorrido: **98,9 %** (sano, semiespacio 1.048.576 B) contra **97,9 %** (partición vieja, 524.288 B), o sea
"la partición nueva no baja la ocupación" — y el que se muere antes (2 assets) no alcanza a llenarse. Al
**mismo tramo** (`lee-volvio` de la primera tabla del MPQ) la partición nueva usa **408.312 B de 1.048.576
(38,9 %)** y la vieja **389.560 B de 524.288 (74,3 %)**, con **2 contra 11** recolecciones en ese punto. El
control ahora mide eso; los máximos quedan como nota. Lección: una comparación entre corridas que recorren
distancias distintas no discrimina, por más números que tenga.

**Los tres estados, 50 s cada uno**: sano (**lector arreglado + partición nueva**) = **6 assets / 5 tablas**
(LvlTypes, LvlPrest, LvlWarp, Objects, objtype), dato vivo máx **1.037.096 de 1.048.576 (98,9 %)**, `gc_collect`
máx **905**; mutante (**lector viejo**) = **3 assets**, **406.992 (38,8 %)**, gc máx **4**; partición vieja
(**`.eh_frame` dentro del mapa**, 512 KB) = **2 assets**, **513.448 (97,9 %)**, gc máx **726**.

**Declarado, no tapado**: el arranque sigue muriendo más allá de las 5 tablas en `fatal error:
gc_external_alloc: OOM` (el que no entra es el **montón de C**: `arena=5.422.432 used=5.380.984 free=41.448`),
y el arranque es **flaky** (en una corrida intermedia el lado del mutante no llegó ni al primer MPQ: 0 assets,
mismo binario que en la anterior daba 3 — es el agujero de DC-002.39c/243, no esta palanca).

Evidencia: `console/dc/evidencia/dc-002.47-techo-del-arranque.txt` (sección 8).

## 69. DC-002.42 (cierre medido): el corpus de la tabla en el disco queda **21/21** — el rojo del criterio 2 era el **runtime sin reenlazar**, y al re-medir apareció que el horneador había **perdido el bucle del `.pack`**

La entrega de DC-002.42 (la tabla de `objectLookups` fuera del binario, leída del disco **por registro**) quedó
en `review` con el criterio 2 parcial y 5 controles en rojo. Re-medido hoy, el criterio 2 **sí** cierra, y en el
camino apareció un defecto propio que el corpus no estaba cazando.

**1. El freno del criterio 2 no era la palanca: era el `.a`.** `fix-libgodc-sq.py --estado` lo decía en su paso
1/2: el arreglo de la copia del GC por Store Queue (DC-002.39c) estaba en el árbol (`gc_copy.c`, sha256
`daf54657ec72fc56`) y **no** en la biblioteca (`libgodc.a` 210.054 B, sha256 `13683f04…`, 08:41). Reenlazado con
`fix-libgodc-sq.py /home/dgalli/dreamcast/libgodc --build --semispace 1024`, el arranque pasa de morir antes de
los assets a **abrir `patch_d2.mpq` y `d2data.mpq` del disco y leer 2 assets** (controles 11/12/14/15, criterio 2).

**2. El defecto que apareció al re-medir.** Con el runtime sano, los controles 4 a 10 cayeron en rojo **y los del
arranque quedaron verdes** (13 OK / 7 fallo): la consola contestaba

```
LOOKUPS-242 FALLO al abrir la tabla del disco: objectLookups: no pude abrir /cd/juego/objectlookups.pack: file does not exist
```

y el `MANIFIESTO.TXT` del disco listaba **3** archivos (los tres MPQ) y ninguno era el `.pack`. Causa en el
código: `mkdisco.sh` hornea `$JUEGO/*.mpq` y `$JUEGO/*.txt`, y el bucle de `*.pack` que agregó esta tarjeta
(commit `3a381c5e`) **no sobrevivió** a la reescritura del horneador de DC-002.44. No se notó porque el horneador
no se queja de lo que no hornea: el disco se armaba igual, sólo que sin el archivo. Es la misma lección que la
sección 66: un instrumento compartido que se reescribe se lleva puestos los caminos de las otras tarjetas.

**3. El arreglo, con su mutante.** Vuelve el bucle de `*.pack` en `mkdisco.sh` (lo hornea en `/JUEGO` y lo anota
en el manifiesto, como los MPQ) y se agrega el mutante del camino, `DC_SIN_PACK=1` (no lo hornea). El corpus
`checar-lookups-242.sh` gana el **control 4b**: arma una ISO mínima (el `.pack` y nada más, 773.869 B, sin los
540 MB de MPQ) y exige que con el bucle salga con el sha del host y con el mutante **no** esté.

**4. Medido.** `bash scripts/console/checar-lookups-242.sh 25` → **21 controles OK, 0 con fallo**. Los números de
la tarjeta siguen ahí: `PACK-242 bytes= 773869 registros= 7891 grupos= 10 maxRegistro= 165 crc= 2553790715`, índice
en RAM **31.644 B** (4 × 7.891) + ventana de 4 KiB, `LOOKUPS-242 bytes= 773869 registros= 7891` en la consola con
el crc cerrando, la tabla del DS1 con los números del host (`12 37 39 35 36`) y los overrides `0`/`342`, y
`FindObject(1,2,0)` = `rogue fountain` por el camino del motor. El peso no se movió: `1ST_READ.BIN` 12.010.632 B
contra 13.808.424 B del mutante (**1.797.792 B**, el símbolo `_go..C3111` desaparecido).

**Lo que sigue abierto y no es de esta tarjeta**: el arranque muere mudo después de leer los assets (DC-002.43)
con el montón de C sin aire (`arena=4372096 used=4333072 free=39024`, `diff 86016`), el arranque es flaky (el
control 11 reintenta), y el mutante ya no muere en el primer MPQ porque esa pared la movió DC-002.41 — el corpus
lo caza por la imagen y por el símbolo.

**Cómo se repite**:

```
python3 scripts/console/fix-libgodc-sq.py /home/dgalli/dreamcast/libgodc --build --semispace 1024
bash scripts/console/fix-libgodc-cadena.sh --verificar --kb 1024     # OK: la cadena esta completa
bash scripts/console/checar-lookups-242.sh 25                       # 21 controles OK, 0 con fallo
DC_SIN_PACK=1 bash scripts/console/mkdisco.sh <dir_build> <dir_juego>  # el mutante del horneador
```

Evidencia: `console/dc/evidencia/dc-002.42-corpus-verificado.txt`.

## 70. DC-002.48: el dato vivo del arranque baja de 1.037.032 a 788.488 B (98,8 % → 75,1 % del semiespacio) y el arranque llega al MISMO tramo — con un mutante que lo devuelve a 1.037.032

El techo de DC-002.47 era el **dato vivo**, no la lectura: la tabla se consume por trozos (34 pedidos de
4.096 B por `LvlPrest.txt`) y aun así el `gc-vivo` llegaba a **1.037.032 de 1.048.576 (98,8 %)** con 5 tablas
leídas y `weapons.txt` como 6ª pedida. Esta tarjeta mide **de dónde salen esos bytes** y los baja con dos
palancas que no cambian lo que el motor ve: el arranque sigue llegando a los mismos 6 assets / 5 tablas.

### 70.1 De dónde salen los bytes (medido en el HOST, `console/dc/peso-arranque-248`)

El runtime de la consola no tiene un contador de «vivo por tabla», así que el reparto se mide en el host con
el **mismo camino del motor** (lector de MPQ → `d2txt.LoadDataDictionary` → loader de `d2records`), corrido
dos veces con el mismo binario del instrumento: sobre `HEAD` y sobre el árbol con las palancas. Mismas 6
tablas, mismo orden que el arranque:

```
tabla             archivo B      vivo HEAD B     vivo palancas B     delta
LvlTypes.txt         10.719            79.904             90.872      +10.968
LvlPrest.txt        134.937           371.760            332.952      -38.808
LvlWarp.txt           3.935            14.920             14.952          +32
objtype.txt           7.003            26.184             27.776       +1.592
Objects.txt         211.970           654.536            408.976     -245.560
(6ª) weapons.txt    118.029           984.104          1.042.288      +58.184

acumulado de las 5 que el arranque COMPLETA:  1.147.304  ->  875.528   =  -271.776 B (-23,7 %)
sizeof ObjectDetailRecord:                          768 B   ->     632 B
campos del struct:                                   77     ->      41   (36 de ellos muertos)
```

Las dos cosas que el arranque se queda de más, medidas y no deducidas:

  a) `Objects.txt` son 573 records: 573 × 768 B = 440.064 B, pero el delta vivo de esa tabla era
     **654.536 B**. Los **214.472 B** que sobran son el **TEXTO** del archivo (211.970 B): los campos que
     guarda cada record (`Name`, `Description`, `token`) son **subslices del buffer interno del `csv.Reader`**,
     y una sola subslice viva se lleva el buffer entero.
  b) `ObjectDetailRecord` tiene 77 campos y **36 no tienen una sola lectura** en los 142 paquetes del motor
     (barrido de `.Campo` en todo el árbol, fuera del loader y del archivo del struct:
     `scripts/console/campos-muertos-248.py`). Son 136 B por fila que el GC **copia en cada recolección**.

### 70.2 Las dos palancas

1. **El record de Objects no guarda los 36 campos muertos** (`campos-muertos-248.py --podar`): el struct pasa
   de 768 a **632 B** en el host (77 → 41 campos) y la tabla de Objects de 654.536 a **408.976 B** vivos con
   el mismo instrumento. La lista está en el script y se puede volver a medir: si algún día el motor lee uno,
   el barrido lo devuelve a la lista de vivos.
2. **`d2txt.DataDictionary.String` COPIA el valor fuera del buffer del csv** (el `strings.Clone` que este Go
   no trae, escrito a mano). Medido **a favor y en contra**: la copia CUESTA en las tablas con muchas cadenas
   chicas (LvlTypes 79.904 → **90.872 B**, +10.968; LvlWarp +32; objtype +1.592) y PAGA en las que guardan
   pocas cadenas sobre archivos grandes (Objects **-245.560 B**, LvlPrest **-38.808 B**). El neto de las 5
   tablas es **-271.776 B**.

Ninguna de las dos toca el comportamiento: los 142 paquetes del motor compilan igual (control 0c del corpus,
**142 de 142**) y el arranque pide exactamente los mismos assets.

### 70.3 Los números de la CONSOLA (Flycast, 50 s, POC `motor-arranque`, 3 MPQ del disco)

Mismo instrumento (`gc-vivo` = lo que sobrevive a una copia, `instrumentar-vivo-245.py` sobre el runtime
vendorizado), misma partición (**1.024 KB por lado**, `DC_EH_FINAL=1`), mismo arranque; lo único que cambia es
el árbol:

```
                                  HEAD (mutante)        DC-002.48 (palancas)
dato VIVO máximo (gc-vivo)            1.037.032 B             788.488 B    -> -248.544 B (-24,0 %)
del semiespacio                            98,8 %                75,1 %
libre del semiespacio en el máximo         11.544 B             260.088 B   -> x22,5
marcas gc-vivo en la ventana                  264                 770      (no bajaron las recolecciones)
assets del MPQ pedidos                          6                   6
lecturas de tabla terminadas                    5                   5
última tabla pedida                   weapons.txt         weapons.txt
muerte                                  MUDA (corte)        MUDA (corte)
```

Es el criterio 1 de la tarjeta: el dato vivo baja **con números medidos** y el arranque **llega igual de
lejos** (6 assets, 5 tablas, `weapons.txt` como 6ª pedida).

Los tramos, marca por marca (`ARQ-CARGA lee-volvio`, el `gsemi` de ese instante):

```
tabla leída          HEAD          DC-002.48
LvlTypes.txt        408.312         459.808     <- la palanca 2 CUESTA 51.496 B acá
LvlPrest.txt        668.976         507.952
LvlWarp.txt         705.552         581.528
objtype.txt         429.296         717.672     <- el gsemi depende de dónde caiga la recolección
Objects.txt         832.144         716.792
máximo del recorrido 1.037.032       788.488
```

**Ojo al leer los tramos como escalera**: `gsemi` es el semiespacio USADO en ese instante, así que una
recolección antes o después lo mueve decenas de KB (el contador `gc=` de la marca lo delata: 270 en el
`lee-volvio` de Objects de HEAD contra **24** en el de esta tarjeta). El número que no depende de eso es el
**máximo de `gc-vivo`** (lo que sobrevive a una copia).

**Corrida de contraste independiente** (`checar-techo-247.sh 50`, la capa de esta tarjeta): el sano da
**788.616 B de 1.048.576 (75,2 %)**, `gc_collect` máximo **804** y 6 assets; el mutante del lector 536.808 B
(51,1 %), gc 7, 3 assets; la partición vieja 434.848 de 524.288 (82,9 %), gc 1.408, 2 assets. Dos corridas
distintas del mismo árbol dan **788.488 y 788.616 B** (128 B de diferencia: la ventana de 50 s).

### 70.4 El mutante (el que exige que el control se ponga ROJO)

`scripts/console/mutar-vivo-248.py --pristino <dir>` devuelve los **tres archivos** al estado de `HEAD` en un
árbol de trabajo aparte (verificado con `cmp`: los 3 distintos del checkout e **iguales** a `git show HEAD:`),
y ese árbol pasa por el **MISMO corpus**:

```
control checar-vivo-248.sh (techo declarado 950.000 B)   palancas        pristino
dato vivo máximo                                          788.488 B     1.037.032 B
del semiespacio                                              75,1 %          98,8 %
resultado del control                                 4 OK / 0 fallo   3 OK / 1 FALLO (control 2)
exit                                                             0               1
```

Sin ese control, una corrida con el dato vivo otra vez al 98,8 % se lee igual que una que lo bajó: los dos
números son «verde» si el número no es un control. El corpus del árbol pristino da **12/12** con el mismo
motor de DC-002.47 (10 assets / 9 lecturas sumadas de dos pasadas: el arranque es flaky, es de otra tarjeta).

### 70.5 Los corpus

* `bash scripts/console/checar-trozos-245.sh 50` → **12 controles OK, 0 con fallo** (exit 0). El sano llega a
  **6 assets / 5 lecturas** con dato vivo máximo **788.488 B**; el mutante del LECTOR (el `ReadAll`, el de
  DC-002.47) sigue muriendo donde tiene que morir: **5 assets / 4 lecturas** y **574.088 B**.
* `bash scripts/console/checar-vivo-248.sh <dir>` → **4 controles OK, 0 con fallo** con las palancas y
  **3 OK / 1 FALLO (el control 2)** con el árbol pristino.
* `bash scripts/console/checar-techo-247.sh 50` (BASE = la capa de esta tarjeta) → **11 controles OK, 0 con
  fallo**.
* `bash scripts/console/checar-motor-disco-234.sh 25` → **9 controles OK, 0 con fallo** con
  `GEN_HEAD=<capa de hoy>` (los cinco estados siguen igual: el mutante sin `/JUEGO` deja el cartel del motor
  **2 veces** y con los MPQ **0**, y `S233`/`S232` leen **3 assets** del MPQ).

### 70.6 El límite, declarado y no tapado

El arranque sigue muriendo **más allá de las 5 tablas** en `fatal error: gc_external_alloc: OOM`, y el que se
agota ahora es el **montón de C**: `arena=5.424.000 used=5.381.312 free=42.688` con `sbrk 0x8d00a000` contra
`0x8cfe1000` (medido en la misma corrida). Bajar el dato vivo **no** mueve ese techo: son dos montones
distintos, y el arranque es **flaky** (una corrida del pristino hizo DOS pasadas: 10 assets / 9 lecturas
sumadas de las dos). Las dos cosas son de otras tarjetas y quedan dichas para que nadie las lea como «esto no
sirvió».

### 70.7 Los defectos de instrumento que aparecieron en esta tarjeta

1. `checar-trozos-245.sh` rehacía la capa base solo si `$SAL/motor-all.log` **existía**. Una corrida **matada a
   mitad** deja ese log en **0 bytes**, así que la corrida siguiente se saltaba la recompilación, heredaba el
   `orden-motor.txt` de la capa de PARTIDA (111 de 142 paquetes, **sin `d2records`**) y el enlace moría con los
   ~1.500 símbolos de `d2records` indefinidos: se lee como «el árbol no compila» cuando el que no compiló nunca
   fue invocado. Ahora la condición exige la línea `compilados:` en el log (y el control 0c dice cuántos compiló:
   **142 de 142**).
2. `checar-motor-disco-234.sh` toma su capa de partida de `/tmp/gox-233`, que es de **antes** de DC-002.42: no
   trae `d2records.RutaDeLosLookups` (el POC del arranque no compila: `reference to undefined identifier`) y su
   `d2mpq.gox` no coincide con un recompilado de hoy porque lleva rutas embebidas de otro día (control 3 en
   rojo). La capa «de hoy» hay que armarla: **copiar `/tmp/gox-233`, reescribirle las rutas
   (`reescribe-rutas-gox.py`) y recomputar **encima** los paquetes del árbol de hoy con la MISMA herramienta
   que usa el corpus (`recompilar-paquete-motor.sh`: `d2txt`, `d2loader`, `d2asset`, `d2records`)**, y pasarla
   con `GEN_HEAD=<capa>`. Es lo que ya decía el README de DC-002.47 («`GEN_HEAD=<capa de hoy>`»); con la capa
   armada así el corpus queda **9/9** (medido: `GEN_HEAD=/home/dgalli/.hermes/cache/scratch/gox-248head-b
   bash scripts/console/checar-motor-disco-234.sh 25`, y el aviso de `capa()` que reusa el directorio viejo:
   el `SAL` de un intento anterior **no** se recicla, hay que darle uno limpio).
3. `mutar-vivo-248.py --pristino` necesita un árbol con `.git`: los archivos se sacan con `git show HEAD:` del
   árbol donde vive el script. La copia congelada se hace con `rsync --exclude .git`, así que el script se corre
   **desde el checkout** apuntando a la copia; corrido desde la copia, `git show` falla a la mitad y deja el
   primero de los tres archivos escrito y los otros dos como estaban (un «pristino» a medias).

### 70.8 Cómo se repite

```
# el instrumento del host (peso vivo por tabla, en el host)
go run ./console/dc/peso-arranque-248 -mpq "$HOME/.wine-d2/drive_c/Games/Diablo II" -n 6

# la suite de la consola (12 controles) y el control del dato vivo (4 controles)
SRC=<dir-src> SAL=<dir-sal> bash scripts/console/checar-trozos-245.sh 50
bash scripts/console/checar-vivo-248.sh <dir-sal>

# el mutante: los 3 archivos al estado de HEAD en un árbol aparte y el mismo corpus + el control
rsync -a --exclude .git ~/Proyectos/diablo2consoles/ <copia>/
python3 ~/Proyectos/diablo2consoles/scripts/console/mutar-vivo-248.py --pristino <copia>
cd <copia> && SRC=<copia-src> SAL=<copia-sal> bash scripts/console/checar-trozos-245.sh 50
bash scripts/console/checar-vivo-248.sh <copia-sal>        # -> ROJO (control 2)

# las otras dos barreras
BASE=<dir-sal> SAL=<dir-techo> bash scripts/console/checar-techo-247.sh 50
GEN_HEAD=<capa de hoy> bash scripts/console/checar-motor-disco-234.sh 25
```

**Nota de aislamiento**: las corridas se hacen sobre **copias congeladas** del checkout (`rsync` a un
directorio de trabajo) porque hay otro agente del tablero trabajando el mismo repo (DC-002.38): el corpus
sincroniza el árbol al vuelo y una copia congelada es la única forma de que lo medido sea lo que se dice.

Evidencia: `console/dc/evidencia/dc-002.48-dato-vivo-bajo-el-techo.txt`.

