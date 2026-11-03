# DC-002.140 — La demo jugable

El POC `juego-131` con un jugador de verdad al mando: se camina con la cruceta, se habla con **X**,
se golpea con **Y**, y la Den of Evil avanza porque el **motor** (`d2common/d2quest`) ve lo que el
jugador hizo. No hay guion que mueva al héroe ni que dispare la quest por número de cuadro.

## Compilar

```bash
cd ~/Proyectos/diablo2consoles

# la demo (queda en /tmp/dcbuild/demo140/disc.gdi, ~30 s)
bash scripts/console/build-demo-140.sh /tmp/dcbuild/demo140

# sin los instrumentos por cuadro: es la que se JUEGA (8 fps). Con MEDIR_CUADRO=si el cuadro
# cuesta 2260 ms = 0,45 fps y no se puede jugar.
MEDIR_CUADRO=no MEDIR_VIVO=no bash scripts/console/build-demo-140.sh /tmp/dcbuild/demo140
```

## Correr

```bash
# 72 s de juego con capturas cada 2 s, con el guion del criterio de DC-002.141 (ver abajo).
SEGUNDOS=72 INTERVALO=2 \
  bash scripts/console/run-demo-140.sh /tmp/dcbuild/demo140/disc.gdi /tmp/capturas-demo140
```

El guion por omisión del runner es **el del criterio de DC-002.141**: a Akara con X (`s`), a la calle del
primer bicho con `abajo+izquierda`, cuatro golpes con Y (`d`), a la calle del segundo bicho con `abajo`,
once golpes, y de vuelta a Akara con `arriba+derecha` a cobrar con X (`s`). Las teclas `x` e `y` **no
producen botón**: el guion viejo (que las usaba) no movía nada.

El emulador tarda **80 s** desde el disco hasta `JUEGO121: PRIMER-CUADRO`; el runner espera esa marca
antes de mandar la primera tecla.

## Las teclas del emulador (medido en DC-002.71, tecla por tecla)

| tecla del teclado | botón del control | para qué en la demo |
|---|---|---|
| flechas | cruceta | caminar (1,87 celdas/s) |
| `s` | **X** (bit 1024) | hablar con el NPC / avanzar la quest |
| `d` | **Y** (bit 512) | golpear |
| `x` | A (bit 4) | recoger / acción |
| `c` | B (bit 2) | cerrar panel |
| `enter` | START | — |

`z`, `y`, `a`, `q`, `w`, `e` y espacio **no producen ningún botón** en esta versión de Flycast.

## Las palancas de esta tarjeta

- `demoKnob` (`DEMO=si|no`): la demo. Con `no` el POC se comporta como siempre (se corta en `cuadros+3`,
  el guion de la quest por cuadro corre, etc.). Por omisión `no`.
- `demoMoor` (`DEMO_MOOR=si|no`): si cruzar el borde del campamento compone el **nivel de afuera** como
  nivel nuevo (el orden de DC-002.109: soltar la caché y las casillas del nivel que se deja antes de
  componer el siguiente). Por omisión `no`. **DC-002.144b: con `si` YA ENTRA** (`CRUZA OK nivel=BloodMoor
  tiles= 289 libre= 1.946.320`) — ver la sección "El Blood Moor (`DEMO_MOOR`)" al final y
  `console/dc/evidencia/dc-002.144b-el-orden-de-los-pedidos.txt`. Lo que entra es la pieza de afuera de 17x17
  y el minimapa/HUD siguen siendo los del pueblo, por eso el defecto sigue en `no`.
- `moorScratchPrimero` (`DEMO_MOOR_SCRATCH=si|no`, **DC-002.144b**): el orden de los dos pedidos grandes al
  cruzar. Con `si` (el defecto) el buffer de lectura del `.dt1` **más grande** del nivel (1.008.003 B) se pide
  antes de los tramos del arena de bloques —`UsarScratchPorArchivo(false)`, el camino de DC-002.101— y cae en el
  hueco entero que deja el arena del pueblo (2.621.440 B). Con `no` (el camino de DC-002.111) cada `.dt1` pide su
  propio buffer después de los tramos, el hueco se parte y el pedido de 1.008.003 B muere en
  `gc_external_alloc: OOM`. Solo actúa con `demoMoor = "si"`.
- `combateCercaKnob` (`COMBATE_CERCA=si|no`, **DC-002.141**): con `si` el botón Y le pega al **bicho vivo
  más cercano** de cada jugador y los relojes del modo (GH/DT/DD) y de la muerte corren para **todos** los
  bichos. Con `no` (el defecto, la corrida de DC-002.126 bit a bit) el bicho de pelea es siempre
  `bichos[0]`: **en cuanto el primero muere no arranca ni un golpe más** y el segundo bicho de la demo no
  se puede matar de ninguna manera. El build de la demo (`build-demo-140.sh`) lo enciende.

Además, con `demoKnob = "si"` el build de la demo apaga todo lo que aprieta solo: `GUION_POC=nada`,
`INYECTAR_P1=0`, `GOLPE_AUTO=no`, `HABLAR_AUTO=no`, `RECOGER_AUTO=no`, `INVENTARIO_AUTO=no`,
`ALIJO_AUTO=no`, `CERRAR_PANELES=no`, `CERRAR_DIALOGO=no`, `MUERTE_POR_CUADRO=no`, `MVP=no`,
`FASE=apagado`.

## Qué deja en el log

```
DEMO140: ARRANCA ...            el escenario y la celda de arranque
DEMO140: BOTON ...              lo que el jugador aprieta, cuando cambia (mas el cuadro y la celda)
DEMO140: HABLA CON AKARA ...    el boton X entro y habia un NPC en rango
DEMO140: QUEST ACEPTADA ...     el motor marco el slot: estado, pagina, bichos vivos
DEMO140: MATA bicho= ... bajas= ... quedaSegunElMotor= ...    una baja, contada por el motor
DEMO140: FPS ...                el ritmo real cada 30 cuadros (msPorCuadro, fps, celda, nivel)
DEMO140: CIERRE ...             el resumen: cuadros, segundos, fps medio/min/max, quest, mundo, RAM
```

## Lo que está medido (ver `console/dc/evidencia/dc-002.140-demo-jugable.txt`)

- **8 fps** con el cuadro completo en **124-131 ms** jugando de verdad (con los instrumentos por
  cuadro: 2260 ms = 0,45 fps — el crc32 del lienzo se llevaba el 91 % del cuadro).
- La cruceta mueve al héroe a **1,82-1,87 celdas/s** (el teórico del motor es 1,875): 2,1 celdas en
  `derecha 1,2 s`.
- El botón X abre el panel de Akara (`distancia= 1,18 radio= 2`) y el motor marca `FlagStarted`
  (`antes= 0 despues= 4`, `estado= aceptada`, página 1 de la Den of Evil).
- Tres golpes con Y bajan la vida del bicho **3 → 2 → 1 → muerto**, y el contador del motor avanza
  (`bajas= 1 de 2`, `LogUpdate(4)`, página 4).
- **DC-002.141**: con `COMBATE_CERCA=si` la corrida del criterio mata **los dos bichos** (3 golpes cada
  uno, `quedaSegunElMotor= 2 → 1 → 0`), la Den queda limpia (`estado= cueva-limpia`, página 5,
  `recompensaPendiente= true`) y el cobro en Akara paga **`SkillPoint(1)`** (`bits 8198 → 8193`,
  `estado= cobrada`, `log= COMPLETADA`). Cierre: `bajas= 2 limpias= true cobrada= true puntoDeHabilidad= 1`.
  Ver `console/dc/evidencia/dc-002.141-den-limpia-y-cobro.txt`.

## Lo que NO (y por eso hay tarjetas aparte)

- El **texto del panel** de Akara no se pinta: el panel abre (356x396 y 500x81, más las páginas del log
  de 276x81 y 252x81, con el texto del motor en el log) y en pantalla queda el recuadro negro con el borde.
- **8 fps no son los 18-20** del criterio: el cuadro jugable es de 124-131 ms.
- La **salida al Blood Moor** (`DEMO_MOOR=si`) **ya entra por RAM** (DC-002.144b: `CRUZA OK nivel=BloodMoor
  tiles= 289 libre= 1.946.320 semiespacio= 1.485.976`, 6 fps), pero el nivel que entra es la pieza de afuera de
  17x17 y **el minimapa y el HUD siguen siendo los del pueblo** (el minimapa suma las celdas nuevas a las del
  pueblo: 701 = 680 + 21). El detalle, abajo.

## El Blood Moor (`DEMO_MOOR`) — DC-002.144: el muro, y DC-002.144b: cómo se rompió

`demoMoor = "no"` **sigue siendo el defecto** (el build de la demo también lo deja en `no`): cruzar ya anda, pero
lo que entra es la pieza de afuera de 17x17 con el minimapa y el HUD del pueblo. Encenderla es un build:
`DEMO_MOOR=si bash scripts/console/build-demo-140.sh /tmp/dcbuild/demo140-moor`.

### DC-002.144 — el muro (medido, 19:06)

Lo que se midió corriéndolo en Flycast con el teclado virtual (`/tmp/dcbuild/demo140-moor`, 19:06, evidencia
completa en `console/dc/evidencia/dc-002.144-el-blood-moor.txt`):

1. **El camino no llegaba ni a intentarlo.** Tres defectos, los tres arreglados dentro de la palanca:
   las tablas (`LvlPrest`/`LvlTypes`) ya estaban **soltadas** por la carga (`TABLAS SOLTADAS ... bytes= 216064`)
   y `presetDelMoor` buscaba el preset en una tabla vacía; **el Blood Moor no tiene preset** (medido en el host:
   el único preset con `LevelID > 0` es el del pueblo — el afuera es DRLG de 56x96 armado con piezas, no un
   preset); y las **fuentes** también estaban soltadas, así que el `.ds1` del nivel nuevo contestaba
   `file not found` (sin tablas de MPQ no hay archivo que buscar) y `LoadStampPath` hacía `panic`.
2. **Con eso arreglado, el nivel nuevo se lee y se empieza a componer**: caché del pueblo devuelta a la VRAM
   (3.133.440 B), casillas soltadas (semiespacio 87,6 % → 68,4 %), `Act1/Outdoors/Wild1.ds1` (17x17, preset 32
   «Act 1 - Fence Fill 1», `outdoors= true`) leído del MPQ.
3. **Y ahí se muere**: los `.dt1` del afuera piden **4.368.642 B** de arenas de bloques (7 tramos de
   318.414 a 524.288, más el de 1.008.003 que falla) contra **2.995.872 B** libres en el montón de C.
   `Out of memory. Requested sbrk_base 0x8d088000, was 0x8cf92000, diff 1007616` →
   `fatal error: gc_external_alloc: OOM`. El semiespacio **no** es la pared (68,4 % usado).
4. **La palanca apagada no cambia la demo**: con `DEMO_MOOR=no` la corrida es la de DC-002.141 (tablas y
   fuentes soltadas, cero líneas de cruce, la quest se acepta con X, 145-149 ms por cuadro).

### DC-002.144b — lo que entró, y por qué era el ORDEN (medido, 19:31 y 19:38)

**El afuera entra.** `DEMO140: CRUZA OK nivel=BloodMoor tiles= 289 jugador= 8.5 8.5 ms= 15117 libre= 1946320
semiespacio= 1485976 vramLibre= 3511976`, la quest queda **en curso** por `EvAreaChanged`
(`DEMO140: AREA pueblo->exterior`), el cruce tarda 15,1 s y después el nivel corre a 6 fps (164 ms por cuadro)
con el descampado dibujado en pantalla. Evidencia completa en
`console/dc/evidencia/dc-002.144b-el-orden-de-los-pedidos.txt` (logs y captura incluidos).

Lo que cambió, y es **una sola palanca** (`moorScratchPrimero`, `DEMO_MOOR_SCRATCH=si|no`): el buffer de lectura
del `.dt1` más grande del nivel (1.008.003 B) se pide **antes** de los tramos del arena de bloques
(`d2asset.UsarScratchPorArchivo(false)`), no después. La pared no era falta de bytes —el montón podía servir ese
bloque (`servible= 1008003`)— sino que los tramos del arena del afuera (2 x 524.288) y los buffers chicos
(318.414, 505.504, 439.569) se metían dentro del hueco fresco que deja el arena del pueblo (2.621.440 B) y lo
partían. Con la palanca encendida el bloque grande cae en el hueco entero; con la palanca apagada, el mismo
código y el mismo guion mueren en `gc_external_alloc: OOM` (A/B corrido: las dos corridas terminan con
`free= 1.946.320`).

Lo que todavía falta para tener **el Blood Moor de verdad**: el nivel que entra es `Wild1.ds1` (17x17, "Act 1 -
Fence Fill 1"), no el afuera de 56x96, porque el Blood Moor no tiene preset y hay que componerlo por regiones con
el DRLG; y el **minimapa y el HUD no siguen el nivel nuevo** (el minimapa mezcla 680 celdas del pueblo con 21 del
afuera y el HUD sigue diciendo «Rogue Encampment»). Las tres palancas de memoria de la tarjeta quedaron con su
número: la 1 (soltar el pueblo entero) no hizo falta —el pico del cruce es 2.056.579 B contra 2.994.912 B
libres—, la 2 (VRAM) no hizo falta y tiene `vramLibre= 3.511.976` B de hueco, y la 3 (DRLG) sigue pendiente.
