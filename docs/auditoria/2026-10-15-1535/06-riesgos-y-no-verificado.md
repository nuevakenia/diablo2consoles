# 06 — Riesgos ordenados y lo que NO está verificado

## Riesgos, por impacto sobre el proyecto (no por dificultad)

| # | Riesgo | Impacto | Por qué | Cómo se cierra |
|---|---|---|---|---|
| 1 | **El binario no cabe** en los 16 MB del Dreamcast | **mata el proyecto tal cual está planteado** | En x86-64 el binario sin símbolos pesa 27,1 MB: `.text` 11,0 MB + `.gopclntab` 8,8 MB + tipos 2,5 MB + `.rodata` 0,9 MB, y en SH-4 el código suele quedar más grande. De los 16 MB hay que restar KOS (~1 MB) y 4 MB de semispaces del GC | Compilar con `sh-elf-gccgo` y **medir el ELF** (F1.2). Si no cabe: poda de paquetes, `--gc-sections`, tablas de `d2records` a binario empaquetado (hoy son 3,64 MB de literales de Go) |
| 2 | **El churn de memoria y las pausas de GC** | alto: se ve como tirones y, peor, consumos que no caben | Medido: 2.242 MB asignados en 15 s, 4.286 mallocs/frame, 925 KB/frame en Blood Moor; el GC de libgodc para el mundo 1-20 ms según heap vivo (a 60 FPS el frame dura 16,6 ms) | Pools y slices reusados, cero `fmt` en el camino caliente, `debug.SetGCPercent(-1)` con GC manual en las cargas (lo recomienda libgodc y encaja con el ciclo de carga de niveles) |
| 3 | **La biblioteca estándar de Go no existe en Dreamcast** | alto: sin ella no compila nada | Los ejemplos de libgodc solo importan `kos` y `unsafe`; el motor usa `fmt` en 344 archivos, `strings` en 260, etc. | F2: subconjunto propio de paquetes, con tests en host y enlace en `sh-elf-gccgo` |
| 4 | **`float64` masivo con FPU de precisión simple** | medio-alto: rendimiento | 2.260 usos de `float64` contra 22 de `float32`; en `-m4-single` los dobles se emulan por software. `d2core/d2map` 297, `d2game/d2player` 74, `d2common/d2math` 60 | Auditoría por camino caliente (movimiento, pathfinding, distancias) y paso a `float32`/punto fijo, verificando que la lógica no cambie (los oráculos del repo sirven para eso) |
| 5 | **Cachés de 128 MB–1 GB** en el motor de escritorio | medio: memoria | `d2core/d2asset/asset_manager.go:40-47` | Bajar a 2-4 MB con desalojo y probar que ninguna pantalla se rompe (los escenarios del repo lo detectan) |
| 6 | **Sin depurador en la consola** | medio: velocidad de desarrollo | libgodc no tiene gdb/delve/core dumps/profiling; `println()` por serie | Host como laboratorio (tests y escenarios), consola solo para lo que solo la consola puede decir; instrumentación propia dentro del motor |
| 7 | **Coherencia de caché y DMA** | medio: bugs fantasma | El emulador no se queja si falta un flush; la consola sí. libgodc además avisa que el GC puede mover un buffer chico en medio de una transferencia | `dcache_flush_range` explícito y buffers no movibles (memoria de KOS, no del heap GC) para DMA |
| 8 | **Licencias** | medio: legal | El repo es GPLv3; libgodc/KOS/GCC traen las suyas | Revisar antes de mezclar código; el port es un fork GPLv3 y no distribuye datos del juego |
| 9 | **El fork de macOS se rompe** | medio: trabajo perdido | El port agrega backends y poda código | Backends nuevos en paquetes propios detrás de etiquetas de build; el backend Ebiten no se toca; los tests del host siguen corriendo en cada cambio |
| 10 | **Audio** | bajo-medio | AICA: 2 MB de sample RAM, ADPCM; el motor usa PCM 44,1 kHz y música 22,05 kHz estéreo | Reencodear en el horneado (F3) y streamear la música |

## Fuera de alcance (decidido, no olvidado)

* **Multijugador**: el TCP del repo no viaja a la consola (el Dreamcast necesita adaptador de banda ancha para
  red, y no hay Battle.net). Se queda en el host.
* **Cinematográficas Bink**: pesan 380 MB entre `d2video` y `d2xvideo` y ya están apagadas por defecto. En
  consola, fuera del build.
* **Editor y herramientas de escritorio** (`d2app/d2setup`, diálogos de primer arranque de macOS): no van.
* **Windows/macOS/arm64**: siguen siendo el objetivo del fork original; este port no los toca.

## NO VERIFICADO (todo lo que este informe NO probó)

1. **No compilé nada para Dreamcast.** No instalé el toolchain, no ejecuté `sh-elf-gccgo`, no corrí un emulador de
   Dreamcast. Todo lo que dice 02 sobre libgodc viene de leer su repositorio, su documentación y sus ejemplos hoy
   (2026-10-15), no de una prueba propia. El spike F1 existe precisamente para convertir esa lectura en medición.
2. **No verifiqué que el motor compile con gccgo.** gccgo no es el compilador `gc`: 287.443 líneas de Go pueden
   tener construcciones que gccgo no acepte, y eso no se sabe hasta compilar.
3. **No verifiqué el tamaño del binario de SH-4** ni cuánto queda libre de los 16 MB con el motor cargado. Es el
   dato que decide el proyecto y es lo primero que hay que medir.
4. **No corrí la suite del repo** (`scripts/verify.sh`, 291 escenarios): la auditoría no ejecuta la barrera
   completa (el reglamento del proyecto lo prohíbe durante una auditoría). Los "813 verdes" son declaración del
   repo en su último commit, no medición mía.
5. **No probé el save propio del dueño** (`.d2s`): las corridas usaron el save de prueba del repo
   (`.verifycfg/Saves/0.od2`); su carpeta `save/` está vacía. No verifiqué importar un `.d2s` suyo.
6. **No medí el juego en modo turbo** (`OD2_AUTOSPEED`) ni jugué a mano: las corridas fueron automatizadas
   (pueblo y Blood Moor, ~12-35 s cada una).
7. **No medí el PS2 ni el PSP** en ninguna forma: no hay toolchain instalado, ni emulador, ni prueba. Todo lo del
   informe 03 es de fuentes citadas.
8. **No medí el horneado real** (F3): los números de arte (Acto 1 = 29,5 MB de tiles; `d2data.mpq` 343,7 MB;
   `d2data` 343,7 MB; los cuatro MPQ medidos 898,7 MB descomprimidos) salen de leer los MPQ con un medidor propio,
   no del paquete final.
   Tampoco sé cuánto comprimen ni cuánto pesa el índice.
9. **No verifiqué los tiempos de carga desde GDEMU/SD** ni el límite de tamaño de un ejecutable de KOS.
10. **No revisé a fondo** `d2networking`, `d2video`, `d2script` (otto) ni `d2thread`: los marco como candidatos a
    poda por tamaño y por sentido, no por un análisis línea por línea.
11. **No verifiqué el estado actual (2026) de TinyGo para MIPS/ps2sdk** ni si el experimento del blog tuvo
    continuación con código publicado.
