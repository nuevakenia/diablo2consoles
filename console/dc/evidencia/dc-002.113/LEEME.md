# DC-002.113 — El control MUEVE al jugador: la cruceta, la animación de caminar y la cámara

**La medida, en una línea:** en la Dreamcast, con el POC andando, la cruceta del emulador mueve a la amazona por el
pueblo en **las 8 direcciones** con la animación de caminar del motor (`TW`, `AMTWhth.COF`), a **1,875 celdas por
segundo** — el número de la tabla del exe, medido tramo por tramo entre **1,812 y 1,875** — y la cámara la sigue
manteniendo el ancla del motor en **(320, 283)** en todos los cuadros. El cuadro del pueblo **no se movió**: el
mapa solo sigue dando `pixelesNoVacios= 307.196 de 307.200` y `crc32= 1437022778` (el del host) y **el cuadro 1 con
el jugador quieto sigue dando `crc32= 1363871674`**, el mismo de DC-002.112.

**Total caminado medido:** 12,20 s con la cruceta apretada, **114,375 subtiles = 22,875 celdas**, 8 tramos, sin un
recorte del reloj de 25 Hz más allá de 2 (`RESUMEN cuadrosPintados= 25 tramos= 8 ... celdasPorSegundo= 1.875`).

---

## 1. La cadena, sin inventar nada

| paso | qué | de dónde sale |
|---|---|---|
| 1 | el **maple** del puerto 0, leído cada cuadro por `d2interface.InputService` | `d2core/d2input/maple` (DC-002.71: 9 botones, 12 pruebas, 8 mutantes) |
| 2 | bits del control → **teclas del motor** (la tabla de traducción del backend) | `d2core/d2input/maple/traduccion.go` |
| 3 | teclas → **ejes de pantalla** (−1/0/1) | `movimiento.go`, `ejesDelControl` |
| 4 | ejes → **dirección del piso** (la que usa el motor para el sprite) | `DireccionDelPiso` → `d2vector.Vector.DirectionTo` → `d2cof.Dir64ToCof` (la cadena medida en DC-002.49b) |
| 5 | dirección → **ciclo COF** ya compuesto (`TW` dir 0·1·4·7·8·9·12·15) | `jugador_carga.go` |
| 6 | **desplazamiento** con la velocidad del exe y el reloj de 25 Hz | `personaje.go` (compartido con el host): 9,375 subtiles/s |
| 7 | **cámara** en el jugador | `rendererDelMapa.WorldToOrtho` + `Camera.MoveTo`, lo mismo que `d2gamescreen` con el héroe |

**Lo que NO se hizo:** no hay tabla propia de direcciones ni de velocidades. Las 8 direcciones salen de
`DireccionesDelControl()`, que pregunta a la cadena del motor por los 8 ejes de la cruceta y devuelve
`[12 4 8 0 9 1 7 15]`; la velocidad sale de `d2path` por `personaje.go`. El POC no pregunta por el bus maple ni por
Ebitengine: pregunta por **teclas**, que es el contrato.

## 2. La corrida del criterio (consola, Flycast)

`camara= siguiendo` (default de `knobs.go`), `cuadros= 22`, guion de la cruceta: las 8 direcciones, 1,5 s cada una y
**en parejas que vuelven** (derecha/izquierda, arriba/abajo...). Log completo en
`consola-criterio-movimiento.log`; la salida del script, en `consola-criterio-movimiento-salida.txt`.

```
JUEGO113: JUGADOR CICLOS quieto= NU quietoDir= 0 quietoCuadros= 8 caminando= 8 direcciones= 8 indicesTotales= 165152 jugadorOK= true
JUEGO113: UMBRAL DEL RECOLECTOR antes= 100 ahora= 95 semiespacio= 1835008 umbralBytes= 1743257 usado= 662504 colecciones= 75
JUEGO113: MAPA tiles= 2337 pisos= 2337 paredes= 338 crcGrilla= -1596781041 ms= 24433 w= 57 h= 41
JUEGO113: MOTOR velocidad=subtilesPorSegundo= 9.375 celdasPorSegundo= 1.875 ticksPorSegundo= 25 segundosPorTick= 0.04
JUEGO113: DIRECCIONES de la cruceta 8 lista= 8
JUEGO113: CONTROL hay= true servicio= true inyectarGuion= false
JUEGO113: CUADRO 1 SOLO-MAPA     pixelesNoVacios= 307196 de 307200 crc32= 1437022778 igualAlHost= true
JUEGO113: CUADRO 1 CON-JUGADOR  pixelesNoVacios= 307196 de 307200 crc32= 1363871674 pixelesDelJugador= 896 ancla= 320 283 dir= 0 modo= NU
JUEGO113: CUADRO 1 REFERENCIA   igualAlDeDC002112= true crc32= 1363871674 ancla= 320 283
```

Los **8 tramos**, uno por dirección de la cruceta (la celda de arranque es 28,20):

| tramo | dir COF | cuadros | segundos | subtiles | celdas/s | celda inicial | celda final |
|---|---|---|---|---|---|---|---|
| DERECHA | 12 | 1 | 0,920 | 8,625 | **1,875** | 28 · 20 | 29,22 · 18,78 |
| IZQUIERDA | 4 | 2 | 1,825 | 16,875 | **1,849** | 29,22 · 18,78 | 25,51 · 19,84 |
| ARRIBA | 8 | 2 | 1,984 | 18,000 | **1,815** | 26,83 · 21,17 | 25,45 · 19,79 |
| ABAJO | 0 | 2 | 1,917 | 17,625 | **1,839** | 24,29 · 18,62 | 26,21 · 19,41 |
| ARRIBA+DERECHA | 9 | 2 | 1,988 | 18,375 | **1,849** | 26,78 · 21,11 | 25,62 · 17,63 |
| ABAJO+IZQUIERDA | 1 | 1 | 0,938 | 8,625 | **1,839** | 25,62 · 17,63 | 26,16 · 19,26 |
| ARRIBA+IZQUIERDA | 7 | 1 | 0,993 | 9,000 | **1,813** | 26,16 · 19,26 | 24,46 · 18,69 |
| ABAJO+DERECHA | 15 | 2 | 1,862 | 17,250 | **1,853** | 24,46 · 18,69 | 27,73 · 19,79 |

Teórico: 9,375 subtiles/s × 0,04 s por tick = 0,375 subtiles por tick; 5 subtiles por celda → **1,875 celdas/s**.
Los tramos dan 96,7–100 % del teórico y el **total da exacto** (`RESUMEN ... celdasPorSegundo= 1.875
teoricoCeldasPorSegundo= 1.875`): la diferencia de cada tramo es la cuantización del reloj de 25 Hz (un tramo de 1
cuadro cobra 23 o 25 ticks enteros), y en el total se compensa.

```
JUEGO113: RESUMEN cuadrosPintados= 25 tramos= 8 tramosReportados= 8 segundosCaminados= 12.2 subtiles= 114.375
          celdas= 22.875 celdasPorSegundo= 1.875 teoricoCeldasPorSegundo= 1.875 ticks= 305 recortes= 2
          pasoMayor= 1 celdasFinales= 27.7289 19.7855 direcciones= 8 camara= siguiendo reloj= 23.204
JUEGO113: REPETIBILIDAD cuadro= 24 camara= siguiendo crcMapa= 960892321 crcMapaDelCuadroAnterior= 960892321
          igualAlCuadroAnterior= true crcConJugador= 1034563617 igualConJugador= true ancla= 320 283
          celda= 28 20 llenos= 307200 llenosDelCuadroAnterior= 307200
          crcMapaDelCuadro1= 1437022778 igualAlCuadro1= false llenosDelCuadro1= 307196 cachePerezosa= true
```

**La repetibilidad se mide contra el cuadro anterior y no contra el cuadro 1, y eso es un hallazgo, no una
comodidad:** la cache de tiles del renderer es **perezosa** y de **solo agregar**. Al cuadro 1 hay tiles que
todavía no se compusieron (307.196 píxeles no vacíos); al volver al origen, después de caminar, esos tiles ya están
cacheados y el mismo encuadre da 307.200. Con la cámara **fija** el fenómeno no aparece (ver §3) porque el viewport
nunca cambia. Lo que el POC sostiene, y mide, es lo otro: **en la misma celda (28,20), dos cuadros seguidos dan el
mismo `crc32`**, mapa y jugador.

## 3. Las dos cámaras (el A/B que pide la tarjeta)

| | `camara= siguiendo` (criterio) | `camara= fija` |
|---|---|---|
| ancla del jugador mientras camina | **(320, 283) en todos los cuadros** | 320 → **124** → 320: el jugador **se mueve en pantalla** |
| desplazamiento en pantalla medido | 0 px (la cámara lo persigue) | **196 px** con 1,725 celdas caminadas |
| celdas/s | **1,875** | **1,875** |
| cuadro 1 contra DC-002.112 | `1363871674` ✔ | `1363871674` ✔ |
| repetibilidad contra el cuadro 1 | no (`igualAlCuadro1= false`, cache perezosa) | **sí** (`igualAlCuadro1= true`, 307.196 px) |
| derrames de la cache | crecen con la caminata | **0** |

La corrida con cámara fija está en `camara-fija/` (log, salida y las capturas `c00-quieto`, `c1`, `c2`, `c3`):
`RESUMEN cuadrosPintados= 13 tramos= 2 celdas= 5.175 celdasPorSegundo= 1.875` y
`REPETIBILIDAD ... igualAlCuadroAnterior= true igualAlCuadro1= true`.

**Con la cámara fija el jugador se va del cuadro rápido y eso está medido:** una celda son 160×160 px, así que la
pantalla de 640×480 muestra **4×3 celdas**; a 1,875 celdas/s el jugador cruza el ancho en ~2 s. Por eso el default
es `siguiendo`.

## 4. Las tres paredes que aparecieron, con su número

1. **El umbral del recolector (1,843 MB).** Con el default de DC-002.112 (`umbralDelRecolector= 100`, que
   `gc_heap.c` traduce a "recolecta al 75 %") el POC **no pasa de componer el pueblo**: el jugador de esta tarjeta
   deja **148.720 B de índices de las 8 direcciones** vivos y el conjunto queda por encima del 75 %. El recolector
   empieza a correr **en cada asignación**: medido, **2.333 recolecciones seguidas** (`ARQ-MARCA gc_collect`) y la
   composición clavada justo después de leer el `.ds1`. Con `umbralDelRecolector= 95` (umbral 1.743.257 B) la misma
   corrida compone en 24 s. El conjunto vivo, medido con una recolección a pedido después de componer:
   **1.328.120 B de 1.835.008 (72 %)**.
2. **La recolección pedida desde el cuadro clava el ciclo del PVR.** Se probó pedirle un `runtime.GC()` al
   recolector cada 8 cuadros: dos corridas quedaron **en el cuadro 7** y no volvieron a presentar (el emulador vivo,
   sin imagen nueva). La palanca `gcCadaCuadros` quedó en **0** con el porqué escrito en `knobs.go`.
3. **La cache de tiles es de solo agregar y sus derrames son VIVOS.** Con la cámara siguiendo, cada celda nueva pide
   piezas que no estaban (~26 KB cada una: 160×160 px a 1 B/píxel) y lo que no entra en el tramo de la VRAM
   **derrama al montón de Go**, donde el recolector no lo puede liberar. Medido en la corrida larga (5 s por
   dirección): **27 derrames, 732.160 B**, y el emulador se cayó con `fatal error: GC to-space overflow`. Las
   defensas de esta tarjeta: el tramo de la cache subió de 2.723.840 B a **4.177.920 B** (12 bloques de 348.160 B de
   la VRAM servible) y el guion va en **parejas que vuelven** para que la cámara regrese a las celdas ya cacheadas.
   **La deuda queda anotada:** un paseo largo con la cámara siguiendo necesita **desalojo en la cache** (o la
   composición al vuelo cuando el tramo está lleno), y eso es tarjeta aparte.

## 5. Lo que el jugador cuesta (medido antes y después)

| | NU dir 0 (DC-002.112) | 8 direcciones de TW + NU |
|---|---|---|
| índices compuestos | 16.576 B | **165.152 B** |
| arena del decodificador, por dirección | 133 pedidos / 86.208 B | 133 pedidos / **70.560 – 97.744 B** |
| montón de C libre al terminar | 138.008 B | **138.008 B** (la arena se devuelve entera) |
| semiespacio vivo, después de componer | — | **1.328.120 B (72 %)** |

Cada dirección se compone con la arena prestada del montón de C y **se devuelve en el acto**: el montón de C queda
igual que antes de tocar al jugador. Lo que queda vivo son los índices (1 B/píxel, índice 0 transparente).

## 6. Las capturas

`capturas/` tiene el cuadro recortado a 640×480 (la ventana del emulador, `crop=640:480:1400:120`):

* `c00-quieto.png` — el jugador **quieto** (`NU` dir 0): el cuadro de DC-002.112.
* `c02.png` … `c05.png` — caminando: `dir= 4`, `dir= 0`, `dir= 1`, `dir= 15`, con el ancla clavada en **320,283**
  (la cámara lo sigue) y el cuadro del ciclo de `TW` avanzando.
* `c99-final.png` — el cuadro que queda en pantalla.
* `zoom-c*.png` — el recorte 120×140 del área del jugador, escalado ×3: la amazona de cuerpo entero y con la
  armadura azul, en pose de caminata.

La etiqueta de cada captura **no sale del reloj**: sale del log (`geo.txt`), que en el mismo instante dice la celda,
la dirección, el cuadro del ciclo y el `crc32` del compuesto.

## 7. Cómo se reproduce

```bash
# consola: el build (ELF + imagen con los TRES MPQ) y la corrida del criterio
bash scripts/console/build-juego-113.sh /tmp/dcbuild/juego113
bash scripts/console/run-flycast-113.sh /tmp/dcbuild/juego113/disc.gdi /tmp/dcbuild/juego113/corrida

# las líneas del criterio
grep -a 'JUEGO113: TRAMO\|JUEGO113: RESUMEN\|JUEGO113: REPETIBILIDAD' /tmp/dcbuild/juego113/corrida/flycast.log

# el A/B de la cámara fija (otro binario: la palanca se pisa y el build lo avisa)
CAMARA=fija CUADROS=10 GUION="derecha 1.5 izquierda 1.5" \
    bash scripts/console/build-juego-113.sh /tmp/dcbuild/juego113-fija
CAMARA=fija CUADROS=10 GUION="derecha 1.5 izquierda 1.5" \
    bash scripts/console/run-flycast-113.sh /tmp/dcbuild/juego113-fija/disc.gdi /tmp/dcbuild/juego113-fi/corrida
```

Tiempos medidos en este host (sin KVM): ~60 s hasta el primer cuadro (10 s abrir el `D2DATA.MPQ`, 24 s componer el
pueblo, ~1 s la carga de las 8 direcciones), **~670–760 ms por cuadro** de PVR (1,3 cuadros/s), 22 cuadros del guion
+ 3 del cierre ≈ 30 s.

## 8. Archivos de esta evidencia

| archivo | qué es |
|---|---|
| `consola-criterio-movimiento.log` | la corrida del criterio completa (arranque, jugador, pueblo, los 25 cuadros, los 8 tramos, el resumen) |
| `consola-criterio-movimiento-salida.txt` | la salida del script `run-flycast-113.sh`: capturas con su estado y las líneas del criterio juntas |
| `geo.txt` | una línea por captura: celda, dirección, cuadro del ciclo, `crc32` y geometría de la ventana |
| `capturas/` | el cuadro recortado a 640×480 y los zooms del área del jugador |
| `camara-fija/` | el A/B con `CAMARA=fija`: log, salida y 4 capturas |
