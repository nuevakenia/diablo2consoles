# 01 — El repo medido (qué hay, de dónde viene, qué corre)

Fecha: 2026-10-15 15:35 (-03). Máquina: CachyOS, x86-64, 12 hilos, 31 GiB, go1.27.1, gcc 16.2.1.
Todos los números salen de comandos corridos en este host; el comando va al lado de cada uno.
Nada de este informe se apoya en lo que los documentos del repo dicen de sí mismos.

## Qué es

`nuevakenia/diablo2consoles` es un fork de `sermare/OpenDiablo2`, que a su vez es un fork del motor
[OpenDiablo2](https://github.com/OpenDiablo2/OpenDiablo2) (Go + Ebitengine) convertido en un **Diablo II: LoD
1.14b nativo de macOS Apple Silicon**, desarrollado de forma autónoma por un agente (Claude) para el dueño
del repo. Licencia GPLv3 y `scripts/check_repo_hygiene.sh` vigila que no haya datos del juego adentro.

Comprobaciones de partida:

```
git remote -v            -> origin  https://github.com/nuevakenia/diablo2consoles.git
git branch -a            -> solo master (al día con origin/master)
git status --short       -> limpio
git log -1 --date=short  -> 2026-10-12  20a31960  Docs: wave 36-43 landed, full verify run 813 green (291 scenarios x6 jobs)
```

Es decir: no es un proyecto abandonado ni un motor a medio hacer. Tiene **2.286 commits sin merge** (3.065 en
total), de los cuales **1.575 son posteriores al 2025-01-01**; los 713 primeros son el upstream de 2018.
El trabajo del fork macOS está vivo y se hace por "waves" (w36…w43, ramas `feat/wNN-*`, `land/w38`).

## Tamaño y reparto del esfuerzo

```
find . -name '*.go' | wc -l                        -> 2.708 archivos
find . -name '*.go' -exec cat {} + | wc -l          -> 423.172 líneas
   de envío (sin _test.go): 1.667 archivos / 287.443 líneas
   de test  (_test.go):     1.041 archivos / 135.729 líneas
cat docs/*.md *.md | wc -l                          -> 6.481 líneas de documentación
git log --oneline -- docs/ | wc -l                  -> 492 commits tocan docs/ (21% de los no-merge)
du -sh .git -> 55M     du -sh . (sin .git) -> 66M
```

Por componente (código de envío + tests juntos):

| Componente | Archivos | Líneas |
|---|---|---|
| `d2common` (formatos, tablas, lógica de juego compartida) | 1.186 | 186.809 |
| `d2core` (asset, mapa, render, ui, registros, audio) | 995 | 143.259 |
| `d2game` (pantallas: juego, menús, quests) | 342 | 63.673 |
| `d2networking` (TCP directo entre instancias) | 142 | 25.068 |
| `d2app` (arranque, configuración, harness) | 25 | 3.149 |

Las cuatro quintas partes del código son lógica de juego y datos: eso es lo que hay que preservar en un port.
Los tests son un tercio de las líneas y **no viajan a la consola** (corren en el host). La documentación es el
1,5% del código: no hay andamio desproporcionado, al contrario de lo que suele verse en proyectos así.

## Hallazgo central: el motor ya está partido en dos por una interfaz

Esto es lo que cambia por completo la conversación del port:

```
grep -rl hajimehoshi/ebiten --include=*.go . | wc -l   -> 12 archivos en todo el repo
   d2core/d2render/ebiten (6)  d2core/d2audio/ebiten (3)  d2core/d2input/ebiten (2)  d2common/d2util (1)
```

Las interfaces viven en `d2common/d2interface` (`renderer.go`, `surface.go`, `input_manager.go`,
`audio_provider.go`, `palette.go`, …) y el backend de escritorio está aislado en tres paquetes:

| Backend a reemplazar | Archivos | Líneas |
|---|---|---|
| `d2core/d2render/ebiten` | 15 | 1.333 |
| `d2core/d2audio/ebiten` | 4 | 424 |
| `d2core/d2input/ebiten` | 5 | 445 |
| **Total** | **24** | **2.202** |

Y la inyección es explícita, sin magia: `d2app/app.go:172` (`ebiten.CreateRenderer(a.config)`) y
`d2app/app.go:188` (`ebiten2.CreateAudio(...)`). El bucle principal es `a.renderer.Run(update, advance, w, h, title)`
(`d2app/app.go:410`).

Un backend de consola **no tiene que tocar el motor**: implementa `Renderer`, `Surface`, `InputManager` y las
fuentes de audio, y reemplaza esas dos líneas. La superficie de trabajo medida es de 2.202 líneas, no de 287.443.

## Lo que ya está resuelto y no hay que inventar

* **Input de mando**: `d2core/d2input/d2gamepad` es Go puro (sin ebiten), con perfiles por familia de mando
  (Xbox/PlayStation/Switch) y un `Controller` que convierte el mando en el teclado y mouse virtuales que el
  juego ya entiende: el stick izquierdo mueve/apunta, el derecho maneja un cursor para menús e inventario, los
  botones disparan skills, pociones, paneles y el automapa. `Config` trae `Deadzone 0.25`, `CursorSpeed 520`,
  `MoveRadius 150`, `ScreenW 800`, `ScreenH 600` (`d2core/d2input/d2gamepad/controller.go:24`). El input es,
  normalmente, un tercio del trabajo de un port a consola; acá está hecho y con tests sin mando físico.
* **Verificación**: `scripts/verify.sh` + 291 escenarios en `scripts/verify.d/` + unitarios, con oráculos
  (generador de niveles comparado registro a registro con el original, RNG, ida y vuelta del `.d2s`), y un
  harness por variables `OD2_AUTO*` que corre el motor de verdad. El último landing declara 813 escenarios
  verdes (291 × 6 jobs). Esto es el banco de pruebas que valida cualquier cambio del port **en el host**.
* **Perfilado listo**: `OD2_AUTOPERF` (metros de frame, CPU, heap), `OD2_PPROF_CPU/HEAP` y `OD2_AUTOSHOT`
  (`d2app/autoperf.go`). Con eso medí memoria y tiempos reales sin escribir nada.

## Medición propia: el motor corre en este Linux x86-64, sin tocar una línea

```
go build ./...                 -> exit 0, sin errores (go1.27.1, linux/amd64, CGO on)
go build -o od2 .              -> 37.425.176 B en 1,7 s (caché caliente)
go vet ./d2core/d2render/... ./d2core/d2audio/... ./d2core/d2input/...   -> sin hallazgos
```

Corrida real contra los MPQ del dueño (`/home/dgalli/.wine-d2/drive_c/Games/Diablo II/`, 11 archivos, 1,8 GB):
pueblo (nivel 1) y Blood Moor (nivel 2), con `OD2_AUTOGAME` + `OD2_AUTOLEVEL` + `OD2_AUTOPERF` + `OD2_AUTOSHOT`.

* `exit 0` en las cuatro corridas; screenshots generados (2560×1052 y 1024×688, PNG RGB).
* Errores en el log: **3 líneas** en la corrida del pueblo, ninguna fatal — un asset ausente
  (`/data/local/use`) y el `fallback` esperado de `drlgmaze` sobre un nivel que no es laberinto.
* Marcas de tiempo medidas: `assets-ready t_ms=222`, `first-frame t_ms=979`, `game-playable t_ms=1724`;
  carga de nivel `drlg-outdoor ms=185` (PC), `populate ms=92,5`.

**Conclusión de 01: no hay ningún "paso a 64 bits" pendiente.** El repo ya es de 64 bits por defecto — Go
compila a amd64 sin cambios — y ya corre en este Linux. La premisa del port no es esa: el trabajo real es
hacia *abajo* (32 bits, 4 bytes de puntero, FPU de un solo precisión).

## LO QUE ESTÁ BIEN

1. La separación motor/backend existe y es chica (2.202 líneas detrás de `d2interface`), así que un backend de
   consola es un trabajo acotado y verificable, no una reescritura.
2. El repo compila y **corre** en este Linux x86-64 sin cambios (medido), con harness de medición y suite de
   291 escenarios: hay banco de pruebas antes de empezar.
3. El input de mando ya está resuelto y probado en Go puro, incluido el cursor virtual para menús (D2 es un
   juego de mouse; esto suele ser el dolor de cabeza de un port).
4. El arte del juego es de 8 bits con índices y paletas intercambiables (DC6/DCC + PL2 + palshift): el formato
   ideal para el hardware de estas consolas, con 4× menos memoria que el RGBA que usa la versión de escritorio.
5. El esfuerzo está concentrado donde debe estar (juego y datos), con los tests afuera del envío.

## LO QUE ENTORPECE

1. **[alta] La consola elegida no puede reusar Ebitengine.** Ebitengine exige GL/GLES con shaders; el PowerVR2
   del Dreamcast y el GS del PS2 no tienen shaders. El backend hay que escribirlo contra el hardware (PVR del
   Dreamcast), no contra GL. Consecuencia: el nuevo backend no puede reusar el código del backend de ebiten
   (1.333 líneas de referencia, no de base), aunque sí las interfaces.
2. **[alta] La versión de escritorio está calibrada como 64-bit sin techo**: los presupuestos de caché de
   assets son 128 MB para animaciones y **1 GB** para DT1/DS1/COF/DCC (`d2core/d2asset/asset_manager.go:40-47`).
   Con eso el proceso llega a 812 MB de RSS en Blood Moor (medido). No es un bug: es una decisión de escritorio
   que en consola hay que invertir (cachés de 2-4 MB con desalojo).
3. **[media] El campo `Backend` del `config.json` es decorativo**: se escribe en `d2core/d2config/defaults.go:24`
   pero **nadie lo lee** (el único `Backend` que se usa es otro: el de las opciones del menú de escape).
   El HAL existe, el registro de backends no; hay que cablearlo (poco trabajo, pero hay que hacerlo).
4. **[media] 2.260 usos de `float64` contra 22 de `float32`**, concentrados en `d2core/d2map` (297),
   `d2networking` (190), `d2game/d2player` (74), `d2common/d2math` (60), `d2core/d2asset` (31). El SH-4 solo
   tiene FPU de precisión simple: las operaciones `float64` se emulan por software. Hay que auditarlos y pasar
   los caminos calientes a `float32` o punto fijo.
5. **[baja] El repo no tiene ninguna rama ni carpeta de consola**: cero rastros de `dreamcast`, `ps2`,
   `kallisti`, `sh-4` en código, docs o scripts (`grep -rniE …` → solo nombres de mandos PlayStation en
   `d2gamepad`). O sea: el port arranca de cero, aunque con el terreno preparado.

## NO VERIFICADO

* No corrí la suite del repo (`scripts/verify.sh`) ni los escenarios: la auditoría no ejecuta la barrera
  completa, y el skill del proyecto lo prohíbe expresamente. Los 813 verdes son declaración del repo, no
  medición mía.
* No probé el `.d2s` propio del dueño (la corrida usó el save de prueba `.verifycfg/Saves/0.od2`, que ya viene
  en el repo); su carpeta `save/` está vacía.
* No medí con `OD2_AUTOSPEED` ni en modo turbo, ni con el juego a 60 FPS reales sostenidos.
* No revisé la calidad del port en `d2networking`, `d2video` (Bink), `d2script` (otto/JS) ni `d2thread`: son
  candidatos naturales a poda en consola (ver 04) pero no los medí uno por uno.
