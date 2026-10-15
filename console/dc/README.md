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
| imagen de disco arrancable | **falla** | el `.cue` arranca (REIOS lee el IP.BIN) pero el SH-4 tira excepción a los ~2,5 s |

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

## 5. Ciclo de desarrollo

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

## 6. POCs incluidos

| POC | qué prueba |
|---|---|
| `console/dc/poc/hello` | Go importando `kos` (arranque + PVR) |
| `console/dc/poc/goruntime` | Go pelado, sin tocar KOS: aísla el runtime de libgodc |
| `console/dc/poc/pipeline-c` | C puro: toolchain + imagen + PVR + serie (fondo morado + cuadro) |
| `console/dc/poc/serialcheck` | deja el resultado de `dbgio_dev_select("scif")` en el color de fondo |
| `console/dc/poc/goquad` | **POC principal**: Go + PVR + sprite + mando + ciclos por serie |

## 7. Pendientes (con hipótesis)

1. **Imagen de disco arrancable** (bloquea probar en la Dreamcast real con GDEMU).
   El `.cue`/`.iso` con IP.BIN de `makeip` arranca pero el SH-4 tira excepción a los ~2,5 s,
   justo cuando REIOS salta al bootstrap del IP.BIN (`reios_setup_state(0xac008300)`), en
   cambio el camino ELF salta a `0x8C010000` y funciona. Hipótesis: el bootstrap que genera
   `makeip` no es el que espera el reios, o el `.cue` MODE1/2048 no entrega los sectores que
   el bootstrap pide. Próximo paso: desensamblar el IP.BIN en el offset `0x300` y comparar
   con un IP.BIN conocido, y probar `scramble` sobre el `1ST_READ.BIN`.
2. **Mando**: confirmar que Flycast expone el control en el puerto 0 y que el POC lo lee.
3. **Textura paletizada real (PAL8 + CLUT)** desde un asset de Diablo II horneado
   (DC-001.4): el objetivo del port depende de dibujar el arte con paleta, no RGBA.
4. **Los tres números del go/no-go** (DC-001.5): tamaño del binario del motor,
   RAM libre con el motor cargado, ciclos por frame con ~300 sprites.
5. **Compilar el motor con `sh-elf-gccgo`** (DC-001.6): se espera que falte la biblioteca
   estándar; hay que listar qué paquetes fallan y ordenar por volumen de trabajo.
