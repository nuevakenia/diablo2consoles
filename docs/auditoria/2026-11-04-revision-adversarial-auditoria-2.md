# Revisión adversarial de la auditoría 2 — 2026-11-04

Revisor: par hostil (Go, reingeniería inversa, gamedev, hardware Dreamcast).
Objeto: `docs/auditoria/2026-11-04-auditoria-2-ruta.md`, contrastado con la auditoría 1,
`informe-continuidad.md`, `docs/PLAN-JUEGO.md` y el código/evidencia del repo.
Método: leer el código y **medir**. Todo número de acá sale de una corrida o de una evidencia del
propio repo; cuando es medición mía en el host, se dice.

---

## Veredicto en cinco líneas

1. **La dirección es correcta**: el POC no puede crecer hasta ser Diablo II, y el juego tiene que
   escribirse una sola vez. Eso no lo discuto.
2. **La conclusión operativa es falsa**: "montar el motor" no es un paso, es un programa de 3-6 meses,
   porque el motor **no cabe en 16 MB tal como está**: sus tablas son 15-16,7 MB y sus superficies
   cuestan 4 B/píxel, contra un montón de C de 2,45 MB.
3. **La auditoría 2 no leyó las mediciones de su propio repo.** Fase D ya se intentó (DC-002.93,
   2026-10-31) y la pared está medida. El documento la declara "NO EMPEZADA" y "no medida".
4. **El POC actual ya monta el motor** (31 paquetes, incluidos `d2mapengine` y `d2maprenderer`). El
   "desvío" que la auditoría denuncia describe al POC de 2026-10-29 (1.788 líneas), no al de 36.879.
5. **El plan de tres pistas es viable como dirección y fantasía como secuencia**: pone como paso 2 lo
   que está bloqueado por RAM, y deja como "deuda" lo que es el trabajo real (modelo de datos y de
   superficies).

---

## Lo que está bien (y hay que decirlo)

- **El diagnóstico de los seis defectos de la demo es correcto y está bien atribuido** (dirección a
  mano, reloj por cuadro, spawn sembrado, estela por no restaurar la caja). No es un problema de
  arquitectura del POC: son caminos concretos.
- **La crítica a la métrica es impecable**: 189/191 tarjetas no es un juego. El tablero mide actividad,
  no producto, y eso hay que arreglarlo sí o sí.
- **El POC pagó el riesgo de plataforma**: libgodc, GC, PVR, GD-ROM, AICA, maple, instrumentación.
  Sin eso no hay nada que montar. Es real y se reusa.
- **"El juego se escribe una sola vez"** es la regla correcta. El problema es cómo se ejecuta.

---

## Hallazgos

### H1 · Fase D no está "NO EMPEZADA": está empezada, medida y falló por RAM

`console/dc/evidencia/dc-002.93-el-hueco-de-gc-init.md` (2026-10-31, encabezado: *"tablero
dreamcast, fase D"*) corre **`d2app.Create().Run()` en Flycast** — no un POC, la app del motor:

```
1ST_READ.BIN         11.650.536 B
semiespacios          4.194.368 B (2 x 2048 KB)
montón de C después     481.056 B  -> muere en config.json
con 1536 KB + .eh_frame fuera:
  montón de C         2.450.496 B
  180 s               1 arranque, sin OOM, 5 tablas completas
  donde se queda      leyendo weapons.txt
  pantalla            677 px = sólo el borde de la ventana
```

La auditoría dice *"nadie lo ha arrancado. El juego del motor nunca se ha visto en pantalla"* (§2) y
*"Fase D == NO EMPEZADA: ningún main de consola importa d2gamescreen"* (§2). Lo segundo es un tecnicismo:
`d2app/initialization.go:16` importa `d2game/d2gamescreen` y `:53` lo llama, y el main de consola
`console/dc/poc/motor-disco/main.go` importa `d2app`. El paquete ya está enlazado. Lo que falta no es un
`import`: es memoria. Declarar "0 %" por el grafo de imports es exactamente el vicio que la propia
auditoría denuncia (medir tarjetas, no juego).

### H2 · El presupuesto que decide: las tablas son 15-16,7 MB

Medido hoy en el host, cargando las 85 tablas de `d2resource.DataDictionaries()` con el
`AssetManager` real:

```
inicio                          0,61 MB
MPQ abiertos                    1,68 MB
85 diccionarios                16,66 MB   <- HeapAlloc vivo
  skills.txt        +1,65 MB     monstats.txt     +1,92 MB
  Missiles.txt      +1,55 MB     Sounds.txt       +1,24 MB
  weapons.txt       +1,04 MB     skilldesc.txt    +0,80 MB
  armor.txt         +0,69 MB     misc.txt         +0,52 MB
```

DC-002.82/93 ya habían medido **14,95 MB**. La consola, con el motor arrancado, tiene **2,45 MB** de
montón de C. La brecha es de **6x**. (La medición es en host de 64 bits: en SH-4 los punteros son de
4 B y el número baja — digamos a 10-11 MB—, pero no 6x; es el mismo orden de magnitud. La cifra de
14,95 MB que usó DC-002.93 también es de host.) La auditoría escribe en §9: *"Cuánta RAM pide d2gamescreen en
SH-4: es el riesgo real de la Fase D y no está medido"*. Está medido, está en `docs/PLAN.md`, y el
resultado es que no entra. Ese es el hallazgo que invierte el veredicto.

### H3 · El modelo de superficies del motor cuesta 4 B/píxel; el del POC, 1 B/píxel

`d2core/d2render/pvr/imagen.go:66`:

```go
return &Imagen{pixeles: make([]byte, ancho*alto*4), ...}
```

y `d2core/d2asset/dc6_animation.go:130-138` expande **todas** las direcciones y cuadros al construir la
animación. Medido sobre los MPQ reales:

| asset | frames | expandido a RGBA |
|---|---|---|
| `invchar6.DC6` (inventario, el que carga el motor) | 8 (256x256 máx) | **1.105.920 B** |
| `buysell.DC6` (mercader) | 4 | 552.960 B |
| `ctrlpnl7.DC6` (HUD 640x480) | 6 | 193.704 B |
| héroe AM `NU` (8 capas, 16 dirs) | 16x8 | **1.310.320 B** (≈80 KB por dirección) |
| lienzo de pantalla 640x480 | 1 | **1.228.800 B** |

El POC evita esto por diseño: el panel del inventario son **75.392 B** de índices
(`PLAN-JUEGO.md`, E3). Montar el motor sin cambiar el contrato de `Surface` multiplica por 4-15 la RAM
de todo lo que se ve. No es un detalle de optimización: es la diferencia entre abrir el inventario y no
abrirlo.

### H4 · El mapa: `MapTile` = 328 B; Blood Moor son 2,1 MB de una sola vez

`d2core/d2map/d2mapengine/map_tile.go:14-18` (5 rebanadas + `[25]SubTileFlags`); medido con
`unsafe.Sizeof`: **328 B**. Contra `levels.txt` real:

```
pueblo (Act 1 Town)      56x40  =  2.240 tiles ->   734.720 B
Wilderness 1 (Blood Moor) 80x80 =  6.400 tiles -> 2.099.200 B
Cave/Crypt/Jail (200x200)        = 40.000 tiles -> 13.120.000 B
```

El POC usa **262.144 B** de índices para el mapa entero (512x512, un solo buffer). El motor pide 2,1 MB
contiguos *antes* de una sola entidad, y 328 B por casilla con hasta 5 rebanadas cada una (decenas de
miles de objetos pequeños para el GC de libgodc).

### H5 · La aritmética del arranque no cierra, y la auditoría nunca la hace

Con las mejores palancas medidas (DC-002.93):

```
imagen 1ST_READ            10.729.672 B   (11.650.536 sin .eh_frame fuera)
semiespacios 2 x 1536 KB    3.145.792 B
montón de C libre           2.450.496 B
                          -------------
total                      16.325.960 B   de 16.777.216 B
```

Quedan **451 KB** para KOS, pilas y todo lo demás, y **2,45 MB** de montón de C para todo el juego.
El lienzo de 640x480 (`console/dc/engine/ebiten/v2/ebiten.go:650-657`) **sí entra**: 1.228.800 B, y
deja 1.221.696. El problema es lo que viene después: para dibujar **el pueblo** hacen falta, además,
`MapTile` 734.720 B + cache de tiles en índices **2.723.840 B** (DC-002.101) + tablas del compositor
(409.816 B el subconjunto medido en consola, DC-002.86). Sólo pantalla + mapa + cache = **4.687.360 B**
contra 2.450.496 disponibles. El pueblo no entra con la cache en RAM: habría que mandarla a VRAM, donde
compite con el cuadro y el split (DC-002.116 dejó **30.024 B** libres con cuatro jugadores).

La auditoría contrapone "357 ms en Linux contra 93 s en consola" (§4) como si el problema fuera el ciclo
de desarrollo; el problema es que en la consola **no cabe el nivel**, y eso no se arregla desarrollando
en Linux.

### H6 · Los "tres backends" no son los que usa el motor

- `d2app/app.go:172` hace `ebiten.CreateRenderer(a.config)`, hardcodeado. En consola eso es el shim de
  **2.802 líneas** en `console/dc/engine/ebiten/v2/`, que presenta por su **propio** shim de C/PVR
  (`console/dc/engine/ebiten/v2/pvr.go:29-34`, `d2c_pvr_pantalla_*`), no por `d2core/d2render/pvr`
  (4.858 líneas). O sea: hay **dos implementaciones de PVR**, y la que corre el motor no es la que el
  informe llama "el backend".
- `d2core/d2input/input_manager.go:26` hace `ebiten_input.NewGamepadService()`, también hardcodeado.
- `d2core/d2input/maple` son **787 líneas** (`control.go`, `traduccion.go`, `inyeccion.go`): lee el
  bus y traduce botones. **No implementa `d2interface.InputManager`**, no tiene `BindHandler`, no
  sintetiza mouse.

O sea: la Fase C del informe ("maple detrás de d2input, la interfaz está limpia, es la más fácil") no
es una conexión: hay que escribir el adaptador. Y el juego es **mouse-céntrico**: `GameControls` vive
de `OnMouseButtonRepeat` (`d2game/d2player/game_controls.go:615`) y `OnMouseMove` (`:745`), con
tooltips y regiones clickeables. Un juego de mouse con cruceta no es "la interfaz limpia": es una
decisión de producto (que el POC ya tomó y que la auditoría tira como si fuera backend).

Además el shim **no implementa `ExpandirIndices`** (sólo lo implementa `d2render/pvr`), así que el
camino del motor usa `d2util.ImgIndexToRGBA` + `ReplacePixels` (`d2core/d2asset/cuadro_paletizado.go`):
buffer temporal de 4x por cuadro **además** del buffer final. La optimización que el fork construyó
para el POC no está en el camino del motor.

### H7 · El "33:1" es contabilidad creativa

- Omite **`d2game/d2player`: 31.355 líneas** (HUD, panel de inventario, menú de NPC, skill tree,
  waypoints). Es donde vive la UI de verdad, y es el código más acoplado al contrato de `Surface` que
  hay que revalidar en PVR. La tabla de §1 ni lo nombra.
- Cuenta `d2networking` como *"no se usa: el objetivo es local"* (25.082 líneas). Falso: el juego local
  corre por `d2localclient` + `d2server` (`d2networking/d2client/game_client.go:170-171`).
- Cuenta `d2ui` (3.834) como juego gratis, cuando es la capa que más depende del backend.
- Dice *"`d2core/d2render/pvr` == backend PVR escrito (6.191 líneas)"*. Medido: PVR = **4.858**;
  ebiten = **1.333**; 4.858 + 1.333 = 6.191 exacto. Sumó los dos backends y se lo atribuyó al de PVR.
  En `informe-continuidad.md` el mismo backend son **2.202** líneas. Dos cifras para el mismo número
  que sostiene la misma tesis.

El ratio real, contando lo que hay que ejecutar en consola, no es 33:1 ni de lejos; y el tamaño del
motor no es una medida de trabajo ahorrado, sino de código **no verificado en la consola**.

### H8 · El POC actual ya monta el motor: el "desvío" describe un POC que ya no existe

`console/dc/poc/juego-131` importa **31 paquetes del motor**, entre ellos:

```
d2core/d2map/d2mapengine     d2core/d2map/d2maprenderer   d2core/d2asset
d2core/d2inventory           d2common/d2quest            d2common/d2npc
d2common/d2monster           d2common/d2skill            d2common/d2statlist
d2common/d2automap           d2core/d2records            d2common/d2path
```

y los usa de verdad: `d2mapengine.CreateMapEngine` (`main.go:619`), `ReusarLasCasillasEnSuLugar`
(`:631`), `d2maprenderer.ComoConsola` / `CacheDeTilesEnIndices` / `CacheDeTilesEnTramo`
(`main.go:788-797`), `ResetMapParaTilesExactos` (`moor_147.go:168`). También `d2inventory`, `d2quest`,
`d2npc` y las tablas de monstruo.

La frase *"importa 6 paquetes de formato… no importa `d2core/d2map`"* (§3) describe al
`personaje-mapa` de 1.788 líneas del 29/10, no al POC de 36.879. Consecuencia: el pivote no es
"empezar a usar el motor"; es **reemplazar el main loop del POC por `d2gamescreen`**, conservando el
resto. Es una tarea mucho más chica y mucho más concreta que la que la auditoría propone — y también
mucho menos gloriosa.

### H9 · El POC es una copia por tarjeta: 20 snapshots, no un programa

`console/dc/poc/juego-112` … `juego-131` son **20 directorios completos**; `juego-131` es
`juego-130` + 22 archivos (23 → 45), con `personaje.go` **byte-idéntico** (md5 `729642558a496fa378a5811e872afd04`
en los dos). "45 archivos, 36.879 líneas" es **una copia**, no el proyecto. Ese modelo —una copia por
tarjeta— es la causa mecánica del desvío que la auditoría atribuye a "inercia": cada tarjeta arranca
del snapshot anterior, los arreglos no se propagan, y el build de 49 s es el precio de recompilar una
copia entera.

La auditoría no lo menciona, y su métrica no puede verlo. Es lo primero que hay que matar.

### H10 · Los hitos H1-H8 no tienen un solo número, y meten H8 en la misma lista que H1

`PLAN-JUEGO.md` §0 dice: *"no se cuenta un verde sin número en la consola"*. La tabla de §5 de la
auditoría dice "0 %" ocho veces, sin presupuesto de MB, ms ni pantallas. Es el mismo vicio que critica.

Peor: **H7** (5 actos, 27 quests) exige las tablas, los DT1 y los DS1 de los 5 actos en el mismo
montón de 2,45 MB; **H8** (cooperativo 1-4, "feature nueva") multiplica por 4 el problema de
superficies (4 inventarios, 4 héroes, split). H8 no es un hito: es un segundo proyecto. Ponerlo en la
misma lista que "mostrar el menú" hace que la lista parezca un plan.

### H11 · Riesgos de Fase D que la auditoría no menciona (y que están medidos)

1. **Semiespacio por binario**: DC-002.93 §4 — *"una corrida del motor y una del compositor no se
   pueden tener con el mismo `libgodc.a`"*; hay que rearmar el runtime con `fix-libgodc-gc.py <KB>`
   antes de cada corrida. Un main de Fase D necesita **las dos cosas** (componer el nivel y correr el
   juego) en el mismo binario.
2. **`otto` (JavaScript) entra por la puerta de atrás**: `d2client.Create` → `GameClient.Open` llama
   `scriptEngine.AllowEval()` (`game_client.go:190-191`), y `d2script/engine.go:21` hace `otto.New()`.
   Medido: **1.305.784 B** de heap sólo por crear la VM. Con `nil` hay nil-deref. Es el 53 % del montón
   de C para un autoscript que el jugador no usa.
3. **Audio**: AICA sólo acepta WAV **PCM**; rechaza ADPCM explícitamente
   (`d2core/d2audio/aica/wav.go:150`). Sin ADPCM no hay música ni voces de D2. El POC sólo hizo sonar
   "dos PCM" (DC-002.62).
4. **Guardado**: no hay VMU. El shim de `os` escribe en un **ramfs** (`console/dc/std/os/os.go`,
   `d2c_ramfs_*`). H6 (`.d2s` con el motor) necesita un backend de almacenamiento persistente que nadie
   escribió.
5. **UI a 800x600 en pantalla de 640x480**: `d2app/app.go:479` — *"la interfaz vive en una columna de
   800x600"*. El backend PVR escala con NEAREST (`PushFilter` se ignora), así que 800→640 aliasa el
   texto. El POC usa el asset de 640 (`ctrlpnl7.DC6`) y midió las posiciones; el motor usa el de 800
   (`800ctrlpnl7.DC6`, medido: 42.008 B).
6. **Reloj/tick**: la lógica del motor es a 25 Hz con `Advance(elapsed)` real; a 20-30 ms por cuadro
   corre 1-2 ticks por cuadro, pero un cuadro de 150 ms dispara 4 ticks de IA/pathfinding. No hay
   presupuesto de cuadro ni análisis de death spiral.

### H12 · Las dos auditorías se contradicen y ninguna reconcilia

La auditoría 1 ordena arreglar **en el POC**: D1 estela, D2 crash, D4 reloj, D3 direcciones, D5 cinturón,
D6 spawn, después arranque a 15 s y 320x240 (7+ tarjetas de trabajo en `console/`). La auditoría 2, 24
horas después, dice *"no más features de juego en `console/`"*. Si la 2 tiene razón, la lista de la 1 es
trabajo tirado; si la 1 tiene razón, el pivote es prematuro. No hay conciliación: hay dos documentos
canónicos.

Mi lectura: de la 1 se salvan **D2 (crash) y D4 (reloj)** como *tests* de las reglas del motor —el
reloj es exactamente lo que hay que verificar que el motor hace bien en consola—, no como fixes del POC.
D3 (dirección) es una tabla del exe: se verifica con un test, no con una captura.

### H13 · El POC no se "degrada" a backend por decreto

`informe-continuidad.md` y la auditoría 2 dicen *"el POC deja de ser el juego y pasa a ser el backend"*.
Un programa de 36.879 líneas con su propio loop, su propio cache, su propio HUD y 24 archivos que tocan
PVR directo **no se convierte en biblioteca porque lo diga un documento**. Lo que hay de backend real
son ~4.858 líneas (`d2render/pvr`) + 787 (maple) + el proveedor AICA; el resto es una vertical slice que
hay que apagar archivo por archivo. Seis días de deriva después del informe original son la prueba
empírica de que el decreto no funciona.

---

## Qué está mal priorizado

1. **Se prioriza "correr el motor en Linux" (hecho) y "Fase D" (bloqueada) sobre el modelo de datos
   (15-16,7 MB), que es lo único que hoy impide avanzar.** Es limpiar la cocina mientras se quema la casa.
2. **Se deja como "deuda" al final lo que es el trabajo real**: tablas, superficies, mapa, input.
3. **Se pone H7/H8 en la lista de hitos** sin presupuesto, cuando H8 cuadruplica el problema que hace
   inviable H1.
4. **Se ordena seguir arreglando el POC (auditoría 1) y dejar de arreglarlo (auditoría 2)** en el mismo
   día, sin decidir cuál manda.

---

## Qué haría distinto (y por qué)

1. **Gate 0, antes de cualquier pantalla**: un binario de consola que arranque el motor con **sólo las
   tablas del pueblo** (el camino de DC-002.86: 409.816 B medidos en consola, contra 4,04 MB de las 12),
   componga el pueblo y **pinte un cuadro real** de `d2gamescreen`. Criterio: MB de montón, ms de cuadro,
   y captura. Si no pasa, no se toca ninguna pantalla más. Esto convierte DC-002.93 en línea base en vez
   de en un cadáver.
2. **Modelo de datos primero**: bajar las 85 tablas a subconjuntos por nivel/acto (el pueblo usa el 0,9 %
   de las 12 del compositor: 40.040 B), con índices en vez de structs donde se pueda. Ese es el port real;
   "montar la pantalla" es lo último, no lo primero.
3. **Superficies a 8 bits en el camino del motor**: implementar `ExpandirIndices` en el shim de ebiten y
   superficies indexadas en `d2ui`/`d2player`, para igualar el 1 B/píxel del POC. Sin esto, `invchar.DC6`
   solo (1,1 MB) se come medio montón.
4. **Conservar el esquema de control del POC como contrato de producto** y escribir un
   `d2interface.InputManager` que sintetice mouse desde la cruceta (o adaptar `GameControls`). No tirar la
   única decisión de diseño que el usuario ya validó jugando.
5. **Un solo directorio de consola**, no 20 snapshots: el boot de `juego-131/main.go` (2.372 líneas, ~15
   palancas en orden: `ArrancarComoConsola`, `UsarLiberadorDeExternos`, `ReservarTramoDeLaCache`,
   `ComoConsola`, `CacheDeTilesEnIndices`, …) + `d2render/pvr` como backend + `d2gamescreen` arriba.
   Borrar `juego-112..130`. Sin esto, el próximo arreglo tampoco se propaga.
6. **Métrica nueva**: pantallas montadas × MB de montón × ms de cuadro, medidas en Flycast. Cero tarjetas
   contadas como progreso.
7. **Presupuesto escrito y respetado** (medido, no aspiracional): imagen ≤ 10,7 MB; montón de C ≤ 2,45 MB;
   superficies ≤ 1,2 MB; mapa ≤ 1 MB; VRAM ≤ 8 MB; cuadro ≤ 33 ms. Cualquier etapa que no entre, se recorta
   antes de escribirla.

---

## Respuestas directas

**a) ¿"Montar el motor y dejar de crecer el POC" es correcto?** Como dirección, sí. Como conclusión
técnica, no: el motor **no puede correr en 16 MB tal como está**, por tres estructuras concretas y
medidas — 15-16,7 MB de tablas, `MapTile` de 328 B (2,1 MB Blood Moor, 13,1 MB un nivel 200x200) y
superficies de 4 B/píxel (1,23 MB la pantalla, 1,1 MB el inventario) — contra un montón de 2,45 MB. La
frase correcta es: *"el motor tiene las reglas; la consola necesita su propio modelo de datos y de
dibujo, y ese modelo es el que el POC ya insinuó"*.

**b) Riesgos de Fase D que el documento no menciona**: los seis de H11, más el costo de `otto` y la
dualidad de backends (H6). Todos medidos o medibles con el repo.

**c) ¿El plan de tres pistas es realista?** La pista juego (Linux) es real y sirve para reglas/quests;
la pista plataforma es la única que puede validar RAM/ms y está bloqueada por H2; la pista validación con
capturas es razonable pero no valida *feel* ni control. Son **1,5 pistas**, no tres, y el orden está
invertido: la validación humana tiene que empezar por el control y la cámara, no por comparar píxeles.

**d) ¿Qué está mal priorizado?** Lo de la sección anterior: datos > superficies > boot > pantallas; hoy
está pantallas > boot > datos.

**e) ¿Viable o fantasía?** El pivote es viable como **programa de 3-6 meses con Gates medidos**. El plan
tal como está escrito —Fase D como paso 2, H1-H8 como checklist, "el motor nos regala 73.400 líneas"— es
una fantasía de tablero: no porque el motor no sirva, sino porque el documento confunde *tener el código*
con *que el código quepa*, y no leyó las mediciones que ya dicen que no cabe.
