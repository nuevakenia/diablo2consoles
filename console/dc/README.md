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
4. **Los tres números del go/no-go** (DC-001.5): tamaño del binario del motor,
   RAM libre con el motor cargado, ciclos por frame con ~300 sprites.
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
