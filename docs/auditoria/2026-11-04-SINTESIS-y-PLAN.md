# Síntesis y plan corregido — 2026-11-04

Reconcilia tres documentos: `2026-11-04-auditoria-demo.md` (auditoría 1, los defectos de la demo),
`2026-11-04-auditoria-2-ruta.md` (auditoría 2, la ruta) y `2026-11-04-revision-adversarial-auditoria-2.md`
(el par hostil). **Donde los tres se contradicen, manda la revisión adversarial: se verificó contra el código y
las evidencias, y la auditoría 2 estaba equivocada en dos puntos centrales.**

---

## 1. Lo que la revisión adversarial derribó (y hay que admitirlo)

### 1.1 «La Fase D no ha empezado» — FALSO

Se intentó el **2026-10-31** y está medida en `console/dc/evidencia/dc-002.93-el-hueco-de-gc-init.md` (tarjeta
`t_011baf18`, encabezado: *«tablero dreamcast, fase D»*). Se corre `d2app.Create().Run()` —la app del motor, no
un POC— y esto es lo que pasa:

```
1ST_READ.BIN            11.650.536 B
semiespacios             4.194.368 B (2 x 2048 KB)
montón de C                   481.056 B   -> muere en config.json
con 1536 KB + .eh_frame fuera:
  montón de C             2.450.496 B
  180 s                   1 arranque, sin OOM, 5 tablas completas
  donde se queda          leyendo weapons.txt
  pantalla                677 px = sólo el borde de la ventana
```

O sea: el motor **arranca en la consola**, carga config y tablas, y **muere por memoria**. No es «no empezó»:
es «empezó y se estrelló contra el muro de los 16 MB», que es un dato muchísimo más útil.

### 1.2 «El POC importa 6 paquetes de formato» — FALSO

`console/dc/poc/juego-131` importa **32 paquetes del motor**, entre ellos `d2mapengine`, `d2maprenderer`,
`d2asset`, `d2inventory`, `d2quest`, `d2npc`, `d2monster`, `d2skill`, `d2statlist`, `d2automap`, `d2records`.
Y los usa: `CreateMapEngine` (`main.go:619`), `ComoConsola`/`CacheDeTilesEnIndices` (`main.go:788-797`).

El «POC que no usa el motor» era el de **1.788 líneas del 29 de octubre**. El actual ya está montado encima.
La consecuencia cambia el plan: el pivote **no** es «empezar a usar el motor», es **reemplazar el main loop del
POC por `d2gamescreen`** conservando el resto. Mucho más chico y concreto.

### 1.3 «El motor nos regala 73.400 líneas» — contabilidad creativa

- Omite `d2game/d2player`: **31.355 líneas** (HUD, inventario, skill tree, waypoints), que es la UI de verdad y
  la más acoplada al contrato de `Surface`.
- Cuenta `d2networking` (25.082) como «no se usa»: falso, el juego local corre por `d2localclient` + `d2server`.
- «6.191 líneas de PVR» = PVR (4.858) + ebiten (1.333). Sumó los dos backends.
- El mismo backend son «2.202 líneas» en el informe de continuidad y «6.191» en la auditoría 2.

Tener el código **no** es tener el trabajo hecho: es tener código **no verificado en la consola**.

---

## 2. El veredicto corregido

**El POC no puede crecer hasta ser Diablo II** (eso sigue en pie). **Pero «montar el motor» no es un paso: es un
programa de 3-6 meses con gates**, porque el motor **no cabe en 16 MB tal como está**. Tres estructuras lo
impiden, todas medidas:

| estructura | costo | contra |
|---|---|---|
| **85 tablas de `d2resource`** | **15-16,7 MB** (host; en SH-4 ~10-11 MB) | **2,45 MB** de montón de C |
| **`MapTile` = 328 B** (`map_tile.go:14-18`) | pueblo 734.720 · **Blood Moor 2.099.200** · nivel 200x200 **13.120.000** | el POC usa **262.144 B** para todo |
| **superficies a 4 B/píxel** (`pvr/imagen.go:66`) | lienzo 640x480 = 1.228.800 · `invchar6.DC6` = 1.105.920 · héroe 16 dirs = 1.310.320 | el POC: **75.392 B** de índices |

La aritmética del arranque, con las mejores palancas ya medidas:

```
imagen 1ST_READ ......... 10.729.672
semiespacios (2x1536 KB) .. 3.145.792
montón de C .............. 2.450.496
                         -----------
total ................... 16.325.960  de 16.777.216  ->  451 KB para KOS y pilas
```

Y para dibujar **el pueblo** hacen falta además `MapTile` 734.720 + caché de tiles 2.723.840 + tablas del
compositor 409.816 = **3.868.376 B** contra 2.450.496. **El pueblo no entra con la caché en RAM.**

Esa es la frase honesta: *el motor tiene las reglas; la consola necesita su propio modelo de datos y de dibujo,
y ese modelo es el que el POC ya insinuó.*

---

## 3. El ciclo de desarrollo (esto sí quedó probado)

```
JUEGO en Linux (motor + Ebiten) .... go build 5 s, first-frame 357 ms   -> reglas, quests, UI
CONSOLA (Flycast) ................. build 49 s + arranque 93 s          -> memoria, PVR, ms
```

El motor corre en Linux (medido). Las reglas del juego (giro, reloj, alcance, daño, spawn) se arreglan ahí, en
segundos, y se verifican contra las tablas que el fork tradujo del `Game.exe` 1.14b con Ghidra.

Pero ojo: **desarrollar en Linux no arregla la RAM de la consola.** Son dos ciclos distintos y hay que tratar­los
como tales.

---

## 4. El plan corregido: gates, no checklist

### Gate 0 — el motor dibuja un cuadro del pueblo en la consola (lo único que importa ahora)

Un binario de consola que arranque el motor con **sólo las tablas del pueblo** (el camino de DC-002.86 midió
**409.816 B** en consola contra 4,04 MB de las 12), componga el pueblo y pinte **un cuadro real** de
`d2gamescreen`.

Criterio de cierre: MB de montón, ms de cuadro, y captura. **Si no pasa, no se toca ninguna pantalla más.**

Esto convierte DC-002.93 en línea base en vez de cadáver.

### Gate 1 — el modelo de datos (el port de verdad)

Bajar las 85 tablas a **subconjuntos por nivel/acto**, con índices en vez de structs donde se pueda. El pueblo
usa el **0,9 %** de las 12 tablas del compositor (40.040 B). Ese es el trabajo real; «montar la pantalla» es lo
último.

### Gate 2 — las superficies a 8 bits en el camino del motor

Implementar `ExpandirIndices` en el shim de ebiten y superficies indexadas en `d2ui`/`d2player`, para igualar el
1 B/píxel del POC. Sin esto, `invchar.DC6` solo (1,1 MB) se come medio montón.

### Gate 3 — un solo directorio de consola

`juego-112..130` son **20 snapshots**: `juego-131` es `juego-130` + 22 archivos, con `personaje.go`
**byte-idéntico**. Eso es la causa mecánica del desvío: cada tarjeta arranca del snapshot anterior y los
arreglos no se propagan. Hay que borrarlos y quedarse con **un** directorio: el boot de `juego-131/main.go`
(2.372 líneas, ~15 palancas) + `d2render/pvr` + `d2gamescreen` arriba.

### Gate 4 — el control

Conservar el esquema del POC (cruceta, 8 direcciones, botones) como **contrato de producto** y escribir un
`d2interface.InputManager` que sintetice mouse desde la cruceta. El juego del motor es mouse-céntrico
(`game_controls.go:615,745`): un juego de mouse con cruceta es una decisión de producto, y el POC ya la tomó y
Papu ya la validó jugando.

### Gate 5 — la métrica

**pantallas montadas × MB de montón × ms de cuadro**, medidas en Flycast. Cero tarjetas contadas como progreso.
Presupuesto escrito y respetado: imagen ≤ 10,7 MB · montón ≤ 2,45 MB · superficies ≤ 1,2 MB · mapa ≤ 1 MB ·
VRAM ≤ 8 MB · cuadro ≤ 33 ms. Lo que no entre, se recorta antes de escribirlo.

---

## 5. Lo que hay que dejar de hacer (y es incómodo)

- **No más `juego-NNN`.** Un directorio. Los snapshots se acabaron.
- **No más features de juego en `console/`** hasta que el Gate 0 pase.
- **No más demo del POC**: la que Papu jugó es la última hasta que el motor dibuje.
- **No más documentos que no leyeron las mediciones del propio repo.** Esta auditoría 2 lo hizo y la
  revisión la agarró: la lección es que **antes de declarar «no medido» hay que buscar en `console/dc/evidencia/`**.

---

## 6. Las tres cosas que necesito de Papu (y no más)

1. **El Gate 0 y su captura**: cuando el motor pinte el pueblo en la consola, una mirada suya de 10 segundos
   dice si el camino va bien.
2. **El control**: probar el esquema de la cruceta en el motor y decir si se siente manejable. Eso ya lo validó
   en el POC; hay que revalidarlo con el motor.
3. **La prioridad cuando haya que elegir**: actos vs cooperativo vs pulido. Hoy el cooperativo (H8) cuadruplica
   el problema de memoria; conviene saber si es requisito del día 1 o del final.

---

## 7. Veredicto final, sin adornos

El proyecto **no está al 97 %**. El backend de plataforma está maduro (libgodc, GC, PVR, GD-ROM, AICA, maple,
instrumentación) y eso es un activo real. El **juego del motor está al 0 % en la consola** y su primer intento
murió por RAM con la pared medida. El POC es una vertical slice de 36.879 líneas que ya usa 32 paquetes del
motor, pero que no es el juego.

El port es **viable como programa de 3-6 meses con gates medidos**. No es viable como «montamos el motor y
listo», y el tablero tiene que dejar de decir lo contrario.
