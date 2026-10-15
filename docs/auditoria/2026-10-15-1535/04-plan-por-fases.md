# 04 — Plan por fases (con criterio de aceptación medible en cada una)

Regla del plan: **cada fase termina con un número**, no con una sensación. Nada de "quedó andando": o hay un
número, o no está terminada. Y ninguna fase se da por buena sin correr en la consola (no solo en el emulador).

## Arquitectura objetivo (una sola frase)

**Un motor, dos backends.** El código de juego (287.443 líneas) no se toca en su lógica; se agrega un perfil de
consola (arte paletizado, cachés chicas, datos horneados, cero asignación por frame) y un backend de Dreamcast
detrás de las interfaces que ya existen (`d2common/d2interface`). El host x86-64 sigue usando el backend Ebiten y
sigue siendo el lugar donde se corren los tests y los 291 escenarios: **el host es el laboratorio, la consola es
la cancha.**

Esto también protege el trabajo del fork original (macOS arm64): el backend nuevo entra como paquetes nuevos
(`d2core/d2render/kos`, `d2core/d2audio/kos`, `d2core/d2input/kos`) seleccionados por etiquetas de build, sin
tocar el backend de Ebiten que hoy funciona.

## F0 — Línea base en el host (ya hecho en esta auditoría)

* Hecho: `go build ./...` exit 0; motor corriendo con los MPQ reales, screenshots, y perfil (RSS, churn, pprof).
* Falta cerrarlo con un **script de perfil reproducible** versionado en el repo (`scripts/console/`) que corra
  pueblo y Blood Moor y escriba los números a un archivo: es la vara con la que se mide todo lo demás.
* Criterio de aceptación: dos corridas, sin errores fatales, con `VmHWM`, `heap_alloc`, `mallocs/frame` y
  `alloc_kb_per_frame` anotados en un archivo del repo. Hoy: 522-578 MB (pueblo), 692-812 MB (moor), 925 KB/frame.

## F1 — Spike de Dreamcast: ¿existe el runtime y cuánto queda libre? (go/no-go del proyecto)

Es el experimento más importante del plan, chico y binario. Consiste en cuatro pruebas, todas en el emulador y
después en la consola real:

1. **Hola mundo**: compilar y correr `println()` con `sh-elf-gccgo` + KOS + libgodc.
2. **Cabe el motor**: compilar *la mayor cantidad posible* del motor (sin networking/video/replay) con
   `sh-elf-gccgo -ffunction-sections -fdata-sections -Wl,--gc-sections`, y **leer el tamaño del ELF**: ¿cuánto
   de los 16 MB se lleva el binario? ¿Cuánto queda para heap? Este número decide si el proyecto es viable tal
   cual o si hay que podar paquetes y empaquetar `d2records` en binario.
3. **Presupuesto de video**: dibujar con el PVR un tile DT1 paletizado real (sacar uno del MPQ con el pipeline de
   F3) a 320×240 y a 640×480, con 300 sprites en pantalla, y **medir el frame en ciclos** (no en reloj del
   emulador): ¿entra un frame de 60 Hz? ¿de 30 Hz?
4. **Presupuesto de memoria**: cargar un nivel horneado (un mapa DS1 con sus tiles y un puñado de monstruos),
   medir el heap con `gc_stats()`, y ver si el GC no pasa de 2-4 ms.

* Criterio de aceptación: las cuatro pruebas corren **en un Dreamcast real** (con GDEMU) y escriben sus números
  por el puerto serie/IP. Si el binario no deja ~8 MB libres o el frame de 30 Hz no entra, se corta acá y se
  vuelve a decidir (con datos, no con opiniones).
* Costo estimado: días, no semanas. Es tiempo de instalación de toolchain y de leer logs de `println()`.

## F2 — Subconjunto de biblioteca estándar para la consola

* Objetivo: que el motor enlace contra paquetes propios (`d2core/d2platform/…`) que cumplan los contratos de:
  `fmt, strings, errors, strconv, os, math, sort, sync, time, image/color, path/filepath, io, encoding/binary,
  math/rand, bytes, log, bufio, hash/fnv, sync/atomic, encoding/csv` (y `container/heap` en un par de lugares).
  Fuera de consola: `net`, `encoding/json` (queda solo en tests), `runtime/pprof`, `os/exec`, `regexp` si se
  puede evitar.
* Criterio de aceptación: cada paquete tiene tests que corren **en el host** (y pasa la suite del host) **y
  enlaza** en `sh-elf-gccgo` sin símbolos faltantes. Meta de azúcar: `fmt` reemplazado o prohibido en el camino
  caliente (una llamada a `fmt` por frame es una asignación por frame).
* Costo estimado: semanas. Es trabajo mecánico y aburrido, pero acotado y verificable.

## F3 — Pipeline de horneado de datos (host → medio de la consola)

* Objetivo: un comando del host (Go, reutilizando `d2mpq`, `d2dc6`, `d2dcc`, `d2dt1`, `d2ds1`, `d2drlg`, `d2txt`)
  que convierta los MPQ del dueño en un paquete plano para la consola: tiles y sprites **paletizados** (índices +
  CLUT), niveles **pre-generados** (el generador ya coincide con el original en 427 niveles), tablas `.txt` en
  binario compacto, y audio convertido a ADPCM de AICA.
* Números ya medidos que sostienen esta fase: `d2data.mpq` = 11.082 archivos / 343,7 MB descomprimidos; los
  tiles del Acto 1 son **29,5 MB** en 662 archivos; los monstruos de todos los actos 128,2 MB; UI 49,7 MB; DT1
  totales 103,3 MB, DCC 139,5 MB, DC6 75,2 MB; los cuatro MPQ medidos (`d2data` 343,7 + `patch_D2` 7,8 +
  `d2exp` 291,6 + `d2char` 255,6) suman **898,7 MB** descomprimidos, de los cuales **DCC = 573,0 MB** (animaciones
  de personajes y monstruos: el grueso del arte) y **DT1 = 150,7 MB** (tiles). Los otros siete MPQ (música,
  sonido, voces, video, ~1 GB comprimido) no los medí archivo por archivo. Con GDEMU
  (SD) eso no es un problema de espacio: es un problema de streaming y de caché de texturas (8 MB de VRAM).
* Criterio de aceptación: el paquete del Acto 1 se hornea en minutos, pesa lo previsto y **el motor del host
  puede leerlo** (modo consola en el host: mismo formato de datos, backend Ebiten). Ese modo es la clave para
  depurar sin consola.
* Costo estimado: semanas.

## F4 — Backends de Dreamcast

* `d2render/kos`: superficies 320×240 (y 640×480) sobre el PVR, texturas PAL8 + CLUT, blending, el "tinte" del
  día/noche y el ciclo de colores como cambio de paleta; `ReplacePixels` como subida de textura; `Screenshot()`
  por lectura de VRAM. Sin shaders.
* `d2input/kos`: alimentar el `Source` de `d2gamepad` (que ya traduce mando → mouse/teclado virtual).
* `d2audio/kos`: efectos por `snd_sfx`/AICA y música streameada o ADPCM.
* Criterio de aceptación: en consola, el pueblo con el héroe caminando y el panel de inventario abriéndose, con
  el cursor movido por el stick, a 30 FPS sostenidos y sin tirones de GC.
* Costo estimado: semanas.

## F5 — Motor completo en consola, ampliando por contenido

* Orden: pueblo (nivel 1) → Blood Moor con monstruos y combate → cuevas del Acto 1 → el Acto 1 completo → actos 2-5.
* Cada ampliación mide: heap máximo, pausa máxima de GC, frame promedio y p95, y bytes de VRAM en uso.
* Criterio de aceptación por hito: **30 FPS sostenidos en consola real** (no en emulador) con p95 de frame bajo
  el presupuesto, y ninguna pausa de GC que se vea a ojo.

## F6 — PS2 (y después PSP), cuando DC esté andando

* Primero el spike de runtime (gccgo mips-elf vs TinyGo, elegido con números), después los backends, después el
  mismo motor. Los datos horneados de F3 y el endurecimiento de F5 se reusan.

## Hitos, en orden, con la vara de medir

| # | Hito | Criterio | Riesgo principal |
|---|---|---|---|
| 1 | Línea base del host versionada | script + archivo de números en el repo | ninguno |
| 2 | Spike DC (F1) | binario del motor compilado; MB libres; frame en ciclos; GC < 4 ms | **el tamaño del binario** |
| 3 | Stdlib mínima (F2) | tests verdes en host + enlace en sh-elf | volumen de trabajo |
| 4 | Paquete del Acto 1 (F3) | horneado en minutos; el host lo lee | formato de datos |
| 5 | Pueblo jugable en DC (F4) | 30 FPS en consola, cursor con stick | VRAM/velocidad real |
| 6 | Acto 1 jugable (F5) | 30 FPS en consola con combate | memoria y churn |
| 7 | PS2/PSP (F6) | spike de runtime decidido | **no hay runtime** |

## Poda recomendada (para el binario y para la cabeza)

En consola no tienen sentido y hay que sacarlos del build por etiquetas: `d2networking` (25.068 líneas, sin
Battle.net en la consola y sin red), `d2video` (Bink), `d2script` (otto/JS), `d2replay`, `d2thread`,
`d2app/d2setup` (diálogos de escritorio), `d2core/d2hero` en su parte de importación de `.d2s` del usuario si no
se usa, y las tablas de `d2records` como literales de Go (3,64 MB de fuente) pasadas a binario empaquetado.

## Decisiones que necesito del dueño antes de escribir código

1. **Dreamcast primero** (mi recomendación, por lo del runtime de Go) — ¿confirmas, o preferís PS2 por los 32 MB?
2. **Toolchain de Dreamcast en contenedor** (recomendado: no ensucia el host y yo no uso `sudo`) — ¿o preferís
   instalarlo en el host con tu `sudo`?
3. **Primer hito a la vista**: pueblo caminable en consola con el cursor por stick (mi recomendación: así se
   valida todo el camino de datos+runtime+video+input con el mínimo contenido) — ¿o querés apuntar directo a
   "Acto 1 jugable"?
4. **Rama**: sigo la convención del fork (`feat/wNN-*` → `land/*`). Propongo `feat/console-spike-dc`,
   `feat/console-hal`, `feat/asset-bake`. ¿Te sirve?

## LO QUE ESTÁ BIEN del plan

1. Todo lo riesgoso está al principio y es barato: F1 cuesta días y decide el proyecto antes de comprometer meses.
2. El host conserva el banco de pruebas completo durante todo el port: se puede validar lógica sin consola.
3. Los datos horneados y el endurecimiento del motor son reutilizables para PS2 y PSP.
4. El trabajo no rompe el fork de macOS: backends nuevos detrás de etiquetas de build.

## LO QUE ENTORPECE

1. F2 (stdlib) es la fase larga y poco vistosa, y es inevitable en la ruta de Go.
2. El proyecto es de meses, no de semanas; el primer "se ve el juego en la tele" puede tardar.
3. La falta de depurador en consola obliga a un ida y vuelta host/consola para cada bug nuevo.
4. La verificación real necesita la consola conectada y los MPQ: no es un proyecto que se pueda validar en CI.

## NO VERIFICADO

* Las estimaciones de costo son órdenes de magnitud, no compromisos: no ejecuté ninguna fase.
* No sé si el motor completo enlaza con `sh-elf-gccgo` (diferencias del compilador `gc` vs gccgo): es la
  incógnita de F1.2 y puede agregar trabajo a F2.
* No medí el horneado real del Acto 1 (no escribí el pipeline): los números de arte son el volumen descomprimido
  de los MPQ, leído con un medidor propio, no el tamaño del paquete final.
* No verifiqué el límite de tamaño de un ejecutable de KOS/GDEMU ni el tiempo de carga desde GDEMU/SD.
