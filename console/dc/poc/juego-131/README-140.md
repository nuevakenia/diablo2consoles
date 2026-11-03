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
# 110 s de juego con capturas cada 3 s. El guion de teclas va en GUION.
SEGUNDOS=110 INTERVALO=3 GUION='nada 6 derecha 1.2 nada 0.5 s 0.4 nada 2 abajo+izquierda 1.0 nada 0.5
  d 0.4 nada 0.5 d 0.4 nada 0.5 d 0.4 nada 0.5 d 0.4 nada 1 arriba+derecha 1.2 s 0.4 nada 1' \
  bash scripts/console/run-demo-140.sh /tmp/dcbuild/demo140/disc.gdi /tmp/capturas-demo140
```

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
- `demoMoor` (`DEMO_MOOR=si|no`): si cruzar el borde del campamento compone el **Blood Moor** como
  nivel nuevo (el orden de DC-002.109: soltar la caché y las casillas del nivel que se deja antes de
  componer el siguiente). Por omisión `no`: no se probó todavía.

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

## Lo que NO (y por eso hay tarjetas aparte)

- El **segundo bicho** quedó vivo (1 de 2 bajas): la Den no quedó limpia y el camino
  COBRAR → punto de habilidad no se recorrió con un jugador.
- El **texto del panel** de Akara no se pinta: el panel abre (356x396 y 500x81, con el texto del motor
  en el log) y en pantalla queda el recuadro negro con el borde.
- **8 fps no son los 18-20** del criterio: el cuadro jugable es de 124-131 ms.
- La **salida al Blood Moor** (`DEMO_MOOR=si`) no se probó.
