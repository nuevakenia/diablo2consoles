# Auditoría — port de OpenDiablo2 (fork `diablo2consoles`) a Dreamcast / PS2 / PSP

**Fecha:** 2026-10-15 15:35 (-03) · **Máquina:** CachyOS, x86-64, 12 hilos, 31 GiB, go1.27.1, gcc 16.2.1
**Objeto:** `nuevakenia/diablo2consoles` (fork de `sermare/OpenDiablo2`, motor en Go + Ebitengine)
**Pregunta:** ¿se puede portear este motor a consolas, cuál primero, y qué camino conviene?

---

## Resumen ejecutivo (lo que hay que saber para decidir)

### El veredicto

**Sí se puede, y hay que empezar por Dreamcast.** No porque sea la consola más cómoda (no lo es: es la que menos
RAM tiene), sino porque **es la única de las tres donde existe un runtime de Go publicado** (`libgodc`, gccgo +
KallistiOS). El activo del proyecto son las 287.443 líneas de Go con la ingeniería inversa del juego; cualquier
camino que las tire para reescribir en C es empezar de nuevo. Y el trabajo real no es "recompilar para consola":
es **un backend de 2.202 líneas** detrás de las interfaces que ya existen, más **un subconjunto de la biblioteca
estándar de Go**, más **un pipeline de horneado de datos**.

### Los ocho hallazgos que importan (cada uno con su número medido)

1. **El motor ya está partido en dos por una interfaz, y el port lo aprovecha.**
   Solo **12 archivos** del repo importan Ebitengine; el backend de escritorio son **2.202 líneas** en tres
   paquetes (`d2render/ebiten` 1.333, `d2audio/ebiten` 424, `d2input/ebiten` 445) detrás de `d2common/d2interface`.
   La inyección está a la vista: `d2app/app.go:172` y `:188`. Un backend de consola no toca la lógica del juego.
   (Contra: Ebitengine exige shaders y ninguna de estas consolas los tiene, así que el backend se escribe contra
   el hardware, no contra Ebitengine; y el campo `Backend` del `config.json` es decorativo — nadie lo lee.)

2. **El repo ya compila y corre en este Linux x86-64, sin tocar una línea.** `go build ./...` → exit 0; el motor
   levantó el pueblo y Blood Moor con los MPQ reales, generó screenshots y terminó en 0 con 3 líneas de error no
   fatal. **No hay ningún "paso a 64 bits" pendiente** (Go ya es de 64 bits): el trabajo es al revés, 32 bits y
   precisión simple. El host es el laboratorio y ya está montado (tests, 291 escenarios, oráculos, pprof).

3. **La memoria de la versión de escritorio no tiene techo: 522-578 MB de RSS en el pueblo y 692-812 MB en Blood
   Moor.** El desglose dice de dónde sacar: **54% de la memoria viva son imágenes RGBA de Ebiten** (135 MB),
   `io.ReadAll` 39 MB, frames DCC 13,5 MB. Y los presupuestos de caché son de escritorio:
   `d2core/d2asset/asset_manager.go:40-47` → 128 MB de animaciones y **1 GB** por cada caché de DT1/DS1/COF/DCC.

4. **La palanca más grande está en cinco líneas.** `d2util.ImgIndexToRGBA` convierte índices de 8 bits a RGBA en
   `dc6_animation.go:171`, `dcc_animation.go:190` y `tile_cache.go:127,171,250`. En consola hay que **no
   convertir**: el arte del juego ya es 8 bits con paleta, y el PowerVR2 del Dreamcast dibuja PAL8 + CLUT con
   blending en hardware — 4× menos memoria de textura y el tinte día/noche y el ciclo de colores salen gratis
   cambiando la paleta.

5. **El enemigo real es el churn, no el tamaño.** En Blood Moor el motor asigna **2.242 MB en ~15 segundos**
   (78 MB/s, **4.286 mallocs/frame**, 925 KB/frame). El GC de libgodc para el mundo 1-20 ms según heap vivo, y a
   60 FPS el frame dura 16,6 ms: así como está, en consola eso son tirones garantizados. El camino: pools, slices
   reusados, cero `fmt` en el bucle de juego y GC manual en las cargas (que es lo que recomienda libgodc).

6. **El obstáculo más grande de la ruta Go: en Dreamcast no existe la biblioteca estándar.** Los ejemplos de
   libgodc solo importan `kos` y `unsafe`; el motor usa `fmt` en 344 archivos, `strings` en 260, `errors` 144,
   `strconv` 137, `os` 114, `sort` 86, `time` 52, `encoding/csv` 7 (sin `reflect` directo). Es un subconjunto
   enumerable, en su mayoría cálculo puro (se porta o se copia de las fuentes de Go), pero es trabajo de semanas
   antes del primer píxel.

7. **Riesgo #1, el que decide el proyecto: ¿cabe el binario en 16 MB?** En x86-64 el ejecutable sin símbolos
   pesa 27,1 MB: `.text` 11,0 MB, `.gopclntab` 8,8 MB, tipos 2,5 MB, `.rodata` 0,9 MB; de los 16 MB hay que
   restar KOS (~1 MB) y 4 MB de semispaces del GC. Se mide compilando con `sh-elf-gccgo` (días de trabajo) y de
   ahí salen las palancas: poda de `d2networking`/`d2video`/`d2script`/`d2replay`, `--gc-sections`, y las tablas
   de `d2records` (hoy **3,64 MB de literales de Go**) pasadas a binario empaquetado.

8. **El contenido no es un problema de espacio, es de streaming.** Leí los MPQ: `d2data.mpq` son 11.082 archivos
   / **343,7 MB** descomprimidos; los tiles del **Acto 1 son 29,5 MB** (662 archivos); los monstruos de todos los
   actos 128,2 MB; la UI 49,7 MB; `d2char.mpq` 255,6 MB de DCC; los cuatro MPQ medidos (`d2data`,
   `patch_D2`, `d2exp`, `d2char`) suman **898,7 MB** descomprimidos. Con GDEMU
   (SD) eso cabe de sobra; el límite son los 8 MB de VRAM y la caché, no el medio.

### Y sobre tu preocupación del emulador

Los emuladores **no pueden mentir** en RAM ni en VRAM (Flycast emula los 16 MB y los 8 MB exactos: si te pasás,
revienta igual que en la consola), pero **sí mienten en velocidad** (son recompiladores corriendo en un PC de 12
hilos: no emulan un SH-4 a 200 MHz en tiempo real) y en el costo de relleno del PowerVR2. La política: medir
**ciclos** (presupuesto de 6,7 M de ciclos por frame a 30 FPS), usar el modo intérprete como cota pesimista, y
hacer que **la prueba de aceptación de cada hito corra en la consola real** (GDEMU y HDD del PS2), no en la
ventana del emulador. Detalle en `05-emulador-contra-consola.md`.

---

## Recomendación en tres líneas

1. **Conservar el motor Go** y agregarle un perfil de consola (arte paletizado, cachés de 2-4 MB, datos
   horneados, cero asignación por frame) + backends nuevos detrás de `d2interface`.
2. **Seguir desarrollando y probando en Linux x86-64** (ya funciona): el host es donde se corre la suite y se
   depura; la consola es donde se mide de verdad. Los dos backends conviven por etiquetas de build.
3. **Primero Dreamcast, después PS2, después PSP** — y antes de escribir código, un spike de días (F1 del informe
   04) que conteste tres cosas binarias: ¿enlaza el motor con `sh-elf-gccgo`? ¿cuántos MB de los 16 quedan libres?
   ¿entra un frame de 30 Hz con 300 sprites paletizados? Si eso sale bien, el resto es trabajo; si no, se decide
   con números en la mano y no con un plan de meses.

---

## Índice

| Informe | Contenido |
|---|---|
| [`01-el-repo-medido.md`](01-el-repo-medido.md) | qué es el repo, tamaño y reparto del esfuerzo, el HAL, lo que ya está resuelto, la medición propia en Linux |
| [`02-dreamcast-viabilidad.md`](02-dreamcast-viabilidad.md) | hardware vs requisitos del juego, el runtime `libgodc`, la stdlib ausente, la memoria medida, el riesgo del binario, prior art |
| [`03-ps2-y-psp.md`](03-ps2-y-psp.md) | por qué no empezar por PS2/PSP, qué falta, qué se hereda, qué dice el único antecedente de Go en PS2 |
| [`04-plan-por-fases.md`](04-plan-por-fases.md) | F0-F6 con criterio de aceptación medible, poda recomendada, decisiones pendientes del dueño |
| [`05-emulador-contra-consola.md`](05-emulador-contra-consola.md) | lo que el emulador puede y no puede mentir, política de medición, ficha de datos por corrida |
| [`06-riesgos-y-no-verificado.md`](06-riesgos-y-no-verificado.md) | 10 riesgos ordenados por impacto, fuera de alcance decidido, y la lista completa de lo NO verificado |
| [`evidencia/`](evidencia/) | scripts y salidas crudas: pprof, perf, tamaño de binario, arte de los MPQ, screenshots |

## Decisiones que necesito para arrancar

1. **Dreamcast primero** (mi recomendación por el runtime de Go) — ¿lo confirmas?
2. **Toolchain de Dreamcast en contenedor** (no ensucio el host y yo no uso `sudo`) — ¿o lo instalás vos en el host?
3. **Primer hito visible**: pueblo caminable en la consola con el cursor por stick, o apuntar directo a "Acto 1
   jugable".
4. **Ramas**: seguir la convención del fork (`feat/wNN-*` → `land/*`): propongo `feat/console-spike-dc`,
   `feat/console-hal`, `feat/asset-bake`.

## Cómo se hizo esta auditoría (y qué no se hizo)

Se midió **antes** de leer la narrativa del proyecto: estado de git, tamaños, reparto, costo del harness propio
del repo, memoria real del motor corriendo, perfiles de heap, arte de los MPQ, tamaño del binario, y las fuentes
externas (libgodc, ps2dev, DevilutionX-DC, OpenLara, SNeSticle). **No se tocó ni una línea de código** (los
únicos archivos agregados son los de esta carpeta de documentación) y **no se corrió la suite del repo** (el
reglamento del proyecto lo prohíbe en una auditoría): los "813 verdes / 291 escenarios" son declaración del repo,
no medición mía. Todo lo que no se probó está listado como NO VERIFICADO en el informe 06.
