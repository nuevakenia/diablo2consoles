# 02 — Viabilidad en Dreamcast (el candidato correcto para empezar)

## El veredicto corto

**Sí, el Dreamcast es la consola por donde hay que empezar, y por una razón que no es de gusto: es la única de
las tres candidatas donde existe un runtime de Go.** El motor es Go y su valor son las 287.443 líneas de
ingeniería inversa; tirarlas para escribir un motor en C sería empezar el proyecto de nuevo. En Dreamcast hay un
camino publicado, con código y con consola: `libgodc` (runtime de Go para Dreamcast con gccgo + KallistiOS).

El riesgo no es "¿se puede?" sino "¿cabe en 16 MB y a qué velocidad?" — y eso se responde con un experimento
chico (el spike F1 del informe 04), no con un plan de meses.

## Hardware, contra los requisitos reales del juego

| | Dreamcast | Diablo II 1.14b (requisitos oficiales de Blizzard) |
|---|---|---|
| CPU | SH-4 @ 200 MHz, FPU **solo precisión simple** (`-m4-single`; los `float64` se emulan por software) | Pentium 233 MHz |
| RAM | **16 MB** principales (0x8C000000–0x8D000000) | **32 MB** mínimo (64 MB para multijugador/gráficos mejorados) |
| VRAM | **8 MB** PowerVR2 (sin shaders, T&L en CPU, PAL8 + CLUT y blending en hardware) | **8 MB** de "texture RAM" para gráficos mejorados |
| Almacenamiento | GD-ROM 1,2 GB (acá: **GDEMU + SD**, que es mucho más rápido) | 650 MB de disco + CD 4× |
| Sonido | AICA: 2 MB de sample RAM, 64 canales ADPCM | tarjeta DirectX 6.1 |
| Input | mando con stick analógico (existe mouse oficial, pero no es lo que hay en la mesa) | teclado + mouse |

Dos cosas de esa tabla importan más que todo lo demás: **la RAM de texturas que el juego original pedía (8 MB)
es exactamente la VRAM del Dreamcast**, y **el arte del juego ya es de 8 bits indexado con paleta** — el formato
que el PowerVR2 dibuja nativamente (PAL8 + CLUT). No es casualidad que el Dreamcast sea buena plaza para un
diabloide 2D: es su tipo de juego.

## El estado de Go en Dreamcast (verificado en las fuentes, no de oídas)

Existe `github.com/drpaneas/libgodc`: *"Go runtime for Dreamcast using gccgo and KOS"* — reemplaza el runtime
estándar de Go por uno hecho para 16 MB, un SH-4 de un solo núcleo y sin sistema operativo; provee recolección
de basura, goroutines, canales y las funciones core. Hay toolchain precompilada con GCC 15.1.0 (C, C++, **Go**),
KallistiOS 2.2.1 y `kos-ports` (SDL, libGL, libpng, jpeg, zlib, freetype, lua).

Su mapa de memoria declarado, de los 16 MB:

```
0x8D000000  stack del hilo principal de KOS (128 KB) + scheduler
            semispace GC 1 (2 MB)
            semispace GC 0 (2 MB)          <- 4 MB de los 16 se van en el GC por defecto
            stacks de goroutine (64 KB c/u, no crecen)
            objetos grandes (> 64 KB, van por malloc, fuera del heap GC)
            .bss / .data / .rodata / .text
0x8C010000  KOS + dcload (64 KB)
0x8C000000
```

Límites que cambian el diseño (todos de la documentación de libgodc, y todos importan):

| Límite | Valor | Consecuencia para el port |
|---|---|---|
| Pausa del GC | 1-20 ms según heap vivo (10-20 ms con 1 MB vivo) | A 60 FPS hay 16,6 ms por frame: **el churn actual (925 KB/frame) mata la consola** |
| Semispace GC | 2 MB × 2 por defecto | El heap útil es de un puñado de MB, no de cientos |
| Stack de goroutine | 64 KB fijos, **no crecen** | Prohibida la recursión profunda y los buffers grandes en stack |
| Scheduler | M:1, sin paralelismo, **sin preempción** | Un bucle apretado cuelga todo: hay que ceder (`runtime.Gosched`) |
| Debug | Sin gdb/delve, sin core dumps, sin profiling; `println()` por puerto serie | Se depura con `println()` y paciencia: **más razón para depurar en el host** |
| cgo | No hay; se llama a C con `//extern` | Los bindings a KOS se escriben a mano (como en `libgodc/kos/*`) |
| Señales | No hay | Nada de `os/signal`, ni UI de consola como en escritorio |
| Red | Requiere adaptador banda ancha (poco común) | El multijugador TCP del repo queda fuera de la consola |

## El hallazgo que más pesa: la biblioteca estándar de Go no está

Los ejemplos oficiales de libgodc **solo importan `kos` y `unsafe`**; la salida es el `println()` del lenguaje.
El runtime está escrito en C + ensamblador SH-4 (`gc_heap.c`, `chan.c`, `scheduler.c`, `splitstack.c`,
`map_dreamcast.c`, `interface_dreamcast.c`, `defer_dreamcast.c`, `string_dreamcast.c`, `type_registry.c`): eso
implementa las **primitivas del lenguaje**, no los paquetes.

Lo que el código de envío del motor necesita (medido, archivos que la importan, sin tests):

```
fmt 344 · strings 260 · errors 144 · strconv 137 · os 114 · math 88 · sort 86 · sync 54 · time 52
image/color 40 · encoding/json 34 · path/filepath 32 · io 31 · encoding/binary 28 · math/rand 28
bytes 23 · log 22 · image 17 · bufio 13 · runtime 12 · net 10 · syscall 9 · hash/fnv 9
sync/atomic 8 · encoding/csv 7 · regexp 7 · reflect: 0 directo
```

Traducción honesta: **"cambiar tres backends y compilar" no alcanza.** Antes de ver un solo píxel en la consola
hay que darle al motor un subconjunto de la biblioteca estándar. La buena noticia es que ese subconjunto es
enumerable, chico en su mayoría y **no es investigación**: `strings`, `sort`, `strconv`, `bytes`, `bufio`, `io`,
`errors`, `hash/fnv`, `math` y `encoding/binary` son cálculo puro (se portan o se copian de las fuentes de Go,
licencia BSD compatible con la GPLv3 del repo); lo que sí necesita trabajo son los que tocan el sistema
(`fmt`→KOS, `os`→KOS fs, `time`→reloj de KOS, `log`→`println`), y `encoding/csv` (que en D2 lee las tablas `.txt`
y bien puede reemplazarse por un lector binario propio, que además conviene). `encoding/json` (34 archivos, 20
de ellos en `d2networking/d2netpacket`) y `net` (10) se caen solos si la consola es de un jugador.

Es una obra de semanas, medible, con un criterio de aceptación claro: *el paquete compila y pasa sus tests en el
host y además enlaza con `sh-elf-gccgo`*.

## Dónde está el espacio, y dónde no

Medido en este host, corriendo el motor real con los datos reales:

| Medición | Pueblo (nivel 1) | Blood Moor (nivel 2) |
|---|---|---|
| RSS pico (`VmHWM`) | 522-578 MB | 692-812 MB |
| Heap Go en vivo (`heap_alloc`) | 466 MB | 300 MB |
| Memoria de imágenes de Ebiten (in-use) | 44,8% de 206 MB ≈ 92 MB | **54% de 247 MB ≈ 135 MB** |
| Churn de asignación | — | **2.242 MB en ~15 s** (78 MB/s, 4.286 mallocs/frame, 925 KB/frame) |

Quién se lleva la memoria (in-use, Blood Moor): imágenes de Ebiten 135,5 MB · `io.ReadAll` 39,3 MB · frames DCC
13,5 MB. Quién ensucia el heap (alloc, Blood Moor): `d2dcc.fillPixelBuffer` 251 MB ·
`d2mapstamp.LoadStampPath` 245 MB · imágenes de Ebiten 243 MB · **`d2util.ImgIndexToRGBA` 239 MB** ·
`MapEngine.GetTiles` 211 MB · `io.ReadAll` 200 MB · `d2path.longSearch` 93 MB · `d2mpq.sectorBytes+explode` 95 MB.

Las seis palancas, todas identificadas por nombre y archivo:

1. **Paletas** — `d2util.ImgIndexToRGBA` aparece en **5 lugares exactos**: `d2core/d2asset/dc6_animation.go:171`,
   `d2core/d2asset/dcc_animation.go:190` y `d2core/d2map/d2maprenderer/tile_cache.go:127,171,250`. Ahí el motor
   convierte índices de 8 bits a RGBA (4 bytes/píxel). En consola hay que **no convertir**: subir el búfer de
   índices como textura PAL8 con su CLUT. Cuatro veces menos memoria de textura, y el ciclo de colores y el
   tinte día/noche del juego salen de cambiar la paleta — que es justo lo que el hardware hace gratis.
2. **Cachés** — `d2core/d2asset/asset_manager.go:40-47`: animaciones 128 MB, DT1/DS1/COF/DCC 1 GB. Bajarlos a
   2-4 MB con desalojo LRU es un cambio de constantes (más el trabajo de que el desalojo no rompa las pantallas).
3. **Lecturas completas** — `io.ReadAll` (39 MB en vivo, 200 MB de churn): los archivos se leen enteros a
   memoria. En consola: leer por bloques contra `kos/fs.go`, y mejor aún servir de un paquete horneado en el
   medio (ver 04, F3).
4. **Generación en vivo** — `d2mapstamp.LoadStampPath` (245 MB de churn) y `MapEngine.GetTiles` (211 MB): los
   niveles y sus bytes se arman en cada carga. Como el generador del repo **coincide registro a registro con el
   original en 427 niveles**, los niveles se pueden **hornear fuera de línea** y el SH-4 solo lee bytes.
5. **Pathfinding** — `d2path.longSearch.add`, 93 MB de churn: reusar los buffers (pool) en vez de asignar.
6. **MPQ en la consola** — `sectorBytes` + `explode` (95 MB de churn) y leer bloques comprimidos del MPQ con
   descompresión en el SH-4 son un desperdicio: **los MPQ no viajan a la consola**. Viaja un formato plano.

## El riesgo que hay que medir antes de prometer nada: el binario

En x86-64, el binario sin símbolos de depuración pesa **27,1 MB**, y sus secciones de código son enormes:

```
.text        10.979.628 B   (código)
.gopclntab    8.837.510 B   (tabla de PC→línea, para trazas de pánico: prescindible)
.go.type      2.456.728 B   (descriptores de tipo)
.rodata         910.576 B
```

Sobre los símbolos: ~39,6 MB corresponden a biblioteca estándar y terceros (Ebiten ~0,85 MB, y el resto a
`fmt`/`unicode`/`reflect`/etc.), ~9,4 MB al código y datos propios del motor — de los cuales **3,64 MB son las
tablas de `d2core/d2records` escritas como literales de Go en el fuente**. En un SH-4 el código suele quedar más
grande. Si el binario final se come 6-8 MB de los 16, el proyecto no entra; si queda en 3-4 MB, entra con
holgura. **Eso no se puede saber sin compilar con `sh-elf-gccgo`, y es la primera cosa que hay que hacer.**

Palancas para el binario, ya conocidas: `-ffunction-sections -fdata-sections --gc-sections` (libgodc ya compila
así), poda de paquetes que en consola no existen (`d2networking`, `d2video`/Bink, `d2script`/otto, `d2replay`,
`d2thread`, `d2app/d2setup`), y **reemplazar las tablas de `d2records` por datos binarios empaquetados** (de
3,64 MB de literales a unos cientos de KB leídos de un bloque comprimido: además de tamaño, ahorra RAM de
`.rodata` con punteros de 4 bytes).

## Herramientas y ciclo de trabajo

* Toolchain precompilada (GCC 15.1.0 C/C++/Go + KOS 2.2.1 + kos-ports) para Linux: se instala sin compilar nada.
  Recomiendo meterla en un contenedor para no ensuciar el host (yo no uso `sudo`).
* Emulador: **Flycast** (el más usado para homebrew, con `dc-tool-ip` sobre red para subir el ejecutable y leer
  el `println()` por serie) o Redream para pruebas rápidas.
* Hardware real: el dueño tiene **Dreamcast con GDEMU**, o sea pruebas en consola a un `cp` de distancia.
* Alternativa de código abierto para leer en el host: KallistiOS trae su propio `dc-tool` y el `libgodc` tiene
  ejemplos (`hello_screen`, `input`, `platformer`, `filesystem`, `bfont`) que son las plantillas de partida.

## Prior art que conviene mirar (no copiar, mirar)

* **DevilutionX en Dreamcast** (`nexus382/devilutionX`): Diablo 1 corriendo en la consola. Precedente directo de
  "un motor de diabloide en 16 MB", en C++/SDL.
* **OpenLara** (`XProger/OpenLara`, port DC en `histat/OpenLara-dc`): motor de Tomb Raider en C++ con separación
  núcleo/plataforma, corriendo en Dreamcast, PS2, PSP y más. El modelo de arquitectura a imitar.
* **SNESticle**: mismo juego, con backends Win32/PS2/Dreamcast detrás de una abstracción ("Gep"). Confirma el
  patrón: un núcleo portable + backends por consola.
* **Ebitengine en consolas**: los únicos backends oficiales son Nintendo Switch y PS5, con SDK propietario bajo
  NDA (paquete externo `uwagaki`), y usan `-overlay` para reescribir las llamadas al sistema del runtime de Go.
  Eso confirma dos cosas: (a) Ebitengine en consolas viejas **no existe y no va a existir**; (b) la técnica
  viable en esas consolas es tocar el runtime y el backend, que es lo que hace libgodc por otro camino.

## LO QUE ESTÁ BIEN

1. La única consola de las tres con un runtime de Go publicado, con ejemplos, mapa de memoria y límites escritos.
2. El hardware encaja con el juego: arte de 8 bits indexado, sprite 2D, 8 MB de "texture RAM" en la VRAM y
   blending/CLUT en hardware. Nada del motor necesita shaders.
3. El juego se puede reducir de tamaño de forma agresiva sin tocar una línea de lógica: niveles horneados,
   cachés de 2-4 MB, texturas paletizadas, MPQ fuera.
4. El banco de pruebas del host (tests + 291 escenarios + oráculos) sigue siendo válido durante todo el port.
5. El dueño tiene la consola con GDEMU: la verdad final está a mano, no hay que comprar nada.

## LO QUE ENTORPECE

1. **La biblioteca estándar ausente** es el trabajo más grande y el menos divertido. Sin ella no hay ni `fmt`.
   Estimación honesta: varias semanas de un subconjunto acotado, con tests en el host y enlace en `sh-elf-gccgo`.
2. **El tamaño del binario (riesgo #1)**: `.text` 11 MB + `gopclntab` 8,8 MB + tipos 2,5 MB en x86-64. Se mide
   recién con el compilador de SH-4 y decide el proyecto.
3. **El churn de memoria (riesgo #2)**: 925 KB/frame con un GC que para el mundo 1-20 ms. Hay que llevarlo a
   ~0 por frame (pools, slices reusados, cero `fmt` en el camino caliente) — o el juego va a tironear.
4. **Sin depurador**: ni gdb ni profiling en la consola. Los bugs de consola se cazan con `println()` y con
   paciencia; de ahí que el host deba seguir siendo el lugar donde se corre la suite.
5. **`float64` por todos lados** (2.260 usos): el SH-4 no tiene FPU doble. Los caminos calientes (movimiento,
   pathfinding, distancias) hay que pasarlos a `float32`/punto fijo, y eso toca `d2map` (297 usos) y
   `d2game/d2player` (74) — con cuidado de no cambiar la lógica del juego.
6. **Audio**: el motor reproduce PCM a 44.100 Hz y la música es de 22.050 Hz estéreo. En DC hay que reencodear a
   ADPCM de AICA (2 MB de sample RAM) o streamear; no es difícil, pero no sale gratis.

## NO VERIFICADO

* **No instalé ni corrí el toolchain de Dreamcast** (ni compilé una línea con `sh-elf-gccgo`): todo lo que dice
  este informe sobre libgodc viene de su repositorio y su documentación, leídos hoy, no de una prueba propia.
* No medí si el motor **compila** con `sh-elf-gccgo` (gccgo no es el compilador `gc`: puede haber diferencias de
  sintaxis/semántica en 287.443 líneas — por ejemplo uso de características que gccgo no soporte). Es parte del
  spike F1 y es una incógnita real, no un detalle.
* No verifiqué cuánto ocupa el binario con `sh-elf-gccgo` ni cuánto de `libgodc` queda disponible para el juego
  una vez cargado el motor. Es el número que decide el proyecto.
* No medí la velocidad del SH-4 para este motor: no hay manera honesta de estimarla sin la consola.
* No probé la compatibilidad de licencias en detalle (GPLv3 del repo vs licencias de libgodc/GCC/libgo); hay que
  revisarla antes de mezclar código, aunque libgodc se presenta como licencia permisiva.
